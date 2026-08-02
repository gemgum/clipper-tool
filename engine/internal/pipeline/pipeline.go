// Package pipeline mengorkestrasi proses end-to-end: audio → transkrip →
// segmentasi → scoring → render klip.
package pipeline

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/correct"
	"github.com/gemgum/clipper/engine/internal/ffmpeg"
	"github.com/gemgum/clipper/engine/internal/score/heuristic"
	"github.com/gemgum/clipper/engine/internal/score/llm"
	"github.com/gemgum/clipper/engine/internal/score/ollama"
	"github.com/gemgum/clipper/engine/internal/segment"
	"github.com/gemgum/clipper/engine/internal/subtitle"
	"github.com/gemgum/clipper/engine/internal/transcribe"
	"github.com/gemgum/clipper/engine/internal/types"
	"github.com/gemgum/clipper/engine/internal/worker"
)

// clipLeadIn adalah ancang-ancang (detik) sebelum awal ucapan saat memotong
// klip. Whisper menandai awal segmen pas di atau sesudah onset suara, sehingga
// memotong tepat di angka itu memenggal suku kata pertama.
const clipLeadIn = 0.3

// Progress dilaporkan selama proses.
type Progress struct {
	Stage   string      `json:"stage"`
	Value   float64     `json:"progress"`
	Message string      `json:"message,omitempty"`
	Clip    *types.Clip `json:"clip,omitempty"`
	// Summary hanya terisi pada peristiwa terakhir: tabel rincian waktu siap
	// tampil. Dikirim sebagai teks, bukan data mentah, karena kedua penampilnya
	// (terminal & kotak log GUI) sama-sama monospace.
	Summary string `json:"summary,omitempty"`
}

// ProgressFunc callback progres (boleh nil).
type ProgressFunc func(Progress)

// Pipeline menyimpan dependensi & konfigurasi.
type Pipeline struct {
	Paths config.Paths
	Opts  config.Options
	ff    *ffmpeg.Client
	wh    *transcribe.Whisper
	wk    *worker.Client
}

func New(paths config.Paths, opts config.Options) *Pipeline {
	return &Pipeline{
		Paths: paths,
		Opts:  opts,
		ff:    ffmpeg.New(paths.FFmpeg, paths.FFprobe),
		wh:    transcribe.New(paths.Whisper, paths.Model),
		wk:    worker.New(paths.Worker),
	}
}

func emit(fn ProgressFunc, p Progress) {
	if fn != nil {
		fn(p)
	}
}

// Run menjalankan seluruh pipeline untuk satu video. workDir menampung file
// sementara & output klip.
func (p *Pipeline) Run(ctx context.Context, jobID, input, workDir, outDir string, onProgress ProgressFunc) ([]types.Clip, error) {
	if err := p.wh.Available(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, err
	}
	// Klip final di outDir. Bila pengguna tidak menentukan folder keluaran,
	// outDir = workDir — karena itu berkas kerja (audio, transkrip, subtitle
	// sementara) ditaruh di subfolder tmp/, supaya folder hasil hanya berisi
	// klip dan .srt, tidak tercampur berkas antara.
	if outDir == "" {
		outDir = workDir
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("output folder %q: %w", outDir, err)
	}
	tmpDir := filepath.Join(workDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("work folder %q: %w", tmpDir, err)
	}

	rec := newRecorder()
	// Durasi video dipakai menghitung rasio realtime di ringkasan. Gagal probe
	// tidak boleh menggagalkan job — ringkasannya cukup tampil tanpa rasio.
	videoSec, _ := p.ff.Duration(ctx, input)

	// 1. Cache transkrip: kunci dari isi video + model + bahasa. Transkripsi
	// adalah tahap termahal (bisa puluhan menit), jadi percobaan ulang setelah
	// job gagal tidak perlu mengulanginya.
	var tr types.Transcript
	cacheHit := false
	cachePath := ""
	// Kunci ini dipakai ulang oleh cache koreksi, jadi hidup di luar blok if.
	cacheKey := ""
	if key, err := transcriptCacheKey(input, p.Opts.WhisperModel, p.Opts.Language); err == nil {
		cacheKey = key
		cachePath = transcriptCachePath(p.Paths.DataDir, key)
		if cached, ok := loadTranscriptCache(cachePath); ok {
			tr, cacheHit = cached, true
			emit(onProgress, Progress{Stage: "transcribing", Value: 0.48,
				Message: "Transcript loaded from cache (no re-transcription needed)"})
		}
	}

	// Audio hanya perlu diekstrak bila masih harus transkripsi, atau bila
	// heuristik butuh fitur energi dari worker C++.
	needAudio := !cacheHit || p.Opts.Provider == "heuristic"
	wav := filepath.Join(tmpDir, "audio.wav")
	var rms worker.FeaturesResult
	if needAudio {
		emit(onProgress, Progress{Stage: "extracting", Value: 0.05, Message: "Extracting audio"})
		t0 := time.Now()
		if err := p.ff.ExtractAudioWAV(ctx, input, wav); err != nil {
			return nil, err
		}
		rec.since("Extract audio", t0, "ffmpeg")
		// 2. Fitur audio (opsional, dari worker C++).
		if p.wk.Available() {
			emit(onProgress, Progress{Stage: "extracting", Value: 0.12, Message: "Analysing audio energy (C++ worker)"})
			t0 := time.Now()
			r, err := p.wk.Features(ctx, wav, 100)
			note := "C++ worker"
			if err == nil {
				rms = r
			} else {
				note = "C++ worker (failed, ignored)"
			}
			rec.since("Audio features", t0, note)
		}
	}

	// 3. Transkripsi (bagian paling lama; progress diparse dari whisper).
	if !cacheHit {
		emit(onProgress, Progress{Stage: "transcribing", Value: 0.2, Message: "Transcribing (whisper.cpp)"})
		tTr := time.Now()
		outBase := filepath.Join(tmpDir, "transcript")
		got, err := p.wh.Transcribe(ctx, wav, outBase, p.Opts.Language, runtime.NumCPU(), func(f float64) {
			// Petakan 0..1 transkripsi ke pita 0.20..0.48 dari total.
			emit(onProgress, Progress{
				Stage:   "transcribing",
				Value:   0.20 + 0.28*f,
				Message: fmt.Sprintf("Transcribing %.0f%%", f*100),
			})
		})
		if err != nil {
			return nil, err
		}
		tr = got
		rec.since("Transcribe", tTr, "whisper "+p.Opts.WhisperModel)
		if cachePath != "" {
			// Gagal menyimpan cache tidak boleh menggagalkan job.
			_ = saveTranscriptCache(cachePath, tr)
		}
	} else {
		rec.add("Transcribe", 0, "whisper "+p.Opts.WhisperModel+" (from cache)")
	}
	if len(tr.Segments) == 0 {
		return nil, fmt.Errorf("the transcript is empty — check the audio/language")
	}
	// Dihentikan DI SINI, sebelum koreksi: melewatkan ribuan segmen halusinasi ke
	// LLM makan waktu sangat lama dan hasilnya tetap sampah. Diperiksa juga pada
	// cache hit — transkrip buruk yang terlanjur tersimpan tidak boleh lolos
	// hanya karena tidak dihitung ulang.
	if err := detectRepetitionLoop(tr); err != nil {
		return nil, err
	}

	// 3b. Koreksi transkrip. Dijalankan SEBELUM segmentasi & scoring, bukan
	// hanya sebelum subtitle: tanda baca yang salah tempat membuat
	// segment.BuildCandidates memotong klip di tengah kalimat.
	if p.Opts.TranscriptFix != config.TranscriptFixOff {
		t0 := time.Now()
		fixed, err := p.correctTranscript(ctx, tr, cacheKey, onProgress)
		if err != nil {
			return nil, err
		}
		tr = fixed
		rec.since("Correct transcript", t0, correctionNote(p.Opts))
	}

	// 4-5. Pemilihan momen. Mesin dipilih pengguna dan TIDAK diganti diam-diam:
	// bila mesin yang dipilih gagal, job ikut gagal dengan pesan akar masalah.
	var selected []types.Clip
	var sel momentSelector
	engineName := ""
	switch p.Opts.Provider {
	case "claude":
		sel = llm.New(p.Paths.APIKey, p.Opts.LLMModel)
		engineName = "Claude (" + p.Opts.LLMModel + ")"
	case "ollama":
		sel = ollama.New(p.Opts.OllamaURL, p.Opts.OllamaModel)
		engineName = "Ollama (" + p.Opts.OllamaModel + ")"
	case "heuristic":
		engineName = "heuristic"
	default:
		return nil, fmt.Errorf("unknown score engine %q — choose claude, ollama, or heuristic", p.Opts.Provider)
	}

	tSel := time.Now()
	if sel != nil {
		clips, err := p.selectWith(ctx, tr, sel, engineName, onProgress)
		if err != nil {
			return nil, fmt.Errorf("%s failed to select moments: %w", engineName, err)
		}
		selected = clips
	} else {
		emit(onProgress, Progress{Stage: "segmenting", Value: 0.58, Message: "Selecting clips (heuristic)"})
		selected = p.heuristicSelect(tr, rms)
	}
	rec.since("Select moments", tSel, engineName)
	// Buang momen yang jatuh di cuplikan pembuka. Berlaku untuk SEMUA mesin:
	// prompt sudah melarangnya, tapi diuji langsung qwen2.5 tetap memilihnya —
	// model lokal tidak terikat instruksi panjang.
	//
	// Tidak pernah membuang habis: kalau semua momen kena, yang tersisa hanya
	// pilihan buruk, dan pilihan buruk masih lebih baik daripada job gagal
	// dengan "tidak ada klip".
	if kept := dropOpeningPreview(tr, selected); len(kept) > 0 && len(kept) < len(selected) {
		emit(onProgress, Progress{Stage: "segmenting", Value: 0.60, Message: fmt.Sprintf(
			"Skipped %d moment(s) taken from the opening teaser", len(selected)-len(kept))})
		selected = kept
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("%s produced no clips — try loosening the duration preset or lowering the minimum score", engineName)
	}

	// 6. Render klip.
	tw, th := p.Opts.Dims()
	crf, preset := p.Opts.Encode()
	tRender := time.Now()
	for i := range selected {
		cl := &selected[i]
		cl.ID = fmt.Sprintf("clip_%02d", i+1)
		cl.JobID = jobID
		frac := 0.75 + 0.24*float64(i+1)/float64(len(selected))
		emit(onProgress, Progress{Stage: "rendering", Value: frac,
			Message: fmt.Sprintf("Rendering %s (score %d)", cl.ID, cl.Score)})

		segs := segmentsInRange(tr, cl.Start, cl.End)

		// Mundur sedikit dari awal ucapan: timestamp awal whisper jatuh pas atau
		// sesudah onset suara, jadi memotong tepat di angka itu memenggal suku
		// kata pertama. Dilakukan SETELAH segmentsInRange supaya ancang-ancang
		// ini tidak ikut menarik masuk segmen sebelumnya, dan cl.Start memang
		// diubah agar metadata klip cocok dengan berkas yang jadi — termasuk
		// titik acuan subtitle di bawah, supaya tetap sinkron.
		cl.Start -= clipLeadIn
		if cl.Start < 0 {
			cl.Start = 0
		}

		// Teks ucapan klip tanpa timestamp — bahan untuk dibuatkan caption oleh
		// LLM mana pun. Ditulis untuk SETIAP klip, tidak seperti .srt yang hanya
		// ada di mode clean/both: caption dibutuhkan saat memposting, dan orang
		// memposting klip bersubtitle juga.
		//
		// Kegagalannya tidak menggagalkan job: klipnya sudah jadi, dan berkas
		// pendamping yang hilang bukan alasan membuang pekerjaan berat itu.
		txt := filepath.Join(outDir, cl.ID+".txt")
		if err := subtitle.WriteText(txt, segs, cl.Start); err == nil {
			cl.TranscriptTXT = txt
		}

		enc := ffmpeg.EncodeOpts{
			CRF: crf, Preset: preset, FontsDir: p.Paths.FontsDir,
			Mode: string(p.Opts.Reframe), Background: p.Opts.Background, Zoom: p.Opts.Zoom,
			FPS: p.Opts.FPS,
		}
		// Varian polos (tanpa subtitle) — untuk mode clean & both.
		if p.Opts.SubtitleOutput == config.OutputClean || p.Opts.SubtitleOutput == config.OutputBoth {
			name := cl.ID + ".mp4"
			if p.Opts.SubtitleOutput == config.OutputBoth {
				name = cl.ID + "_clean.mp4"
			}
			raw := filepath.Join(outDir, name)
			if err := p.ff.ClipReframe(ctx, input, cl.Start, cl.End, tw, th, enc, raw); err != nil {
				return nil, err
			}
			cl.VideoPathRaw = raw
			cl.VideoPath = raw // dipakai GUI bila tidak ada varian bersubtitle
			// Sertakan .srt agar klip polos bisa disubtitle di editor lain.
			srt := filepath.Join(outDir, cl.ID+".srt")
			if err := subtitle.WriteSRT(srt, segs, cl.Start, p.Opts.Subtitle); err == nil {
				cl.SubtitleSRT = srt
			}
		}
		// Varian bersubtitle (dibakar) — untuk mode burn & both.
		//
		// .ass hanya dibuat bila memang akan dibakar, ditulis ke tmp/, lalu
		// dihapus setelah pembakaran berhasil: isinya sudah menyatu di video
		// dan tidak dipakai siapa pun sesudah itu. Bila render gagal, berkasnya
		// sengaja ditinggal supaya penyebabnya masih bisa ditelusuri.
		if p.Opts.SubtitleOutput != config.OutputClean {
			assPath := filepath.Join(tmpDir, cl.ID+".ass")
			if err := subtitle.WriteASS(assPath, segs, cl.Start, p.Opts.Subtitle); err != nil {
				return nil, err
			}
			enc.AssPath = assPath
			outMP4 := filepath.Join(outDir, cl.ID+".mp4")
			if err := p.ff.ClipReframe(ctx, input, cl.Start, cl.End, tw, th, enc, outMP4); err != nil {
				return nil, err
			}
			_ = os.Remove(assPath)
			cl.VideoPath = outMP4
		}
		cl.Status = "rendered"
		emit(onProgress, Progress{Stage: "rendering", Value: frac, Clip: cl})
	}

	rec.since(fmt.Sprintf("Render %d %s", len(selected), plural(len(selected), "clip", "clips")), tRender,
		fmt.Sprintf("%s, %s", p.Opts.Resolution, p.Opts.Quality))

	sum := rec.summary(jobID, videoSec, len(selected), cacheHit)
	emit(onProgress, Progress{Stage: "done", Value: 1.0,
		Message: fmt.Sprintf("Done: %d clips", len(selected)),
		Summary: sum.Format()})
	return selected, nil
}

// correctionTemperature sengaja rendah. Koreksi transkrip adalah tugas
// menyalin-ulang, bukan tugas kreatif: pada suhu bawaan (0.4) model lokal
// memberi hasil berbeda tiap kali dijalankan atas transkrip yang sama.
const correctionTemperature = 0.1

// correctionProvider memilih LLM untuk koreksi transkrip.
//
// Koreksi butuh LLM walau mesin skornya heuristik, jadi providernya diturunkan
// dari MODE, bukan dari Provider: hybrid/online → Claude, selain itu → Ollama.
func correctionProvider(o config.Options) string {
	if o.Mode == config.ModeHybrid || o.Mode == config.ModeOnline {
		return "claude"
	}
	return "ollama"
}

// correctionNote menyebut mesin & jumlah istilah yang dipakai tahap koreksi,
// untuk ditampilkan di ringkasan. Daftar istilah ikut disebut karena ia
// mengubah hasil DAN kunci cache — dua percobaan yang waktunya mirip tapi
// daftarnya berbeda bukan percobaan yang sama.
func correctionNote(o config.Options) string {
	name := "Ollama (" + o.OllamaModel + ")"
	if correctionProvider(o) == "claude" {
		name = "Claude (" + o.LLMModel + ")"
	}
	switch n := len(o.Terms); {
	case n == 1:
		name += ", 1 term"
	case n > 1:
		name += fmt.Sprintf(", %d terms", n)
	}
	return name
}

// correctTranscript membenahi transkrip sebelum dipakai tahap berikutnya.
//
// Hasilnya di-cache terpisah dari transkrip mentah: mematikan koreksi tidak
// boleh memaksa transkripsi ulang, dan menyalakannya lagi tidak perlu memanggil
// LLM dua kali untuk video yang sama.
func (p *Pipeline) correctTranscript(ctx context.Context, tr types.Transcript, cacheKey string, onProgress ProgressFunc) (types.Transcript, error) {
	provider := correctionProvider(p.Opts)

	var complete correct.Completer
	var engineName string
	switch provider {
	case "claude":
		c := llm.New(p.Paths.APIKey, p.Opts.LLMModel)
		c.Temperature = correctionTemperature
		engineName = "Claude (" + c.Model + ")"
		// Claude tidak punya parameter skema; bentuk balasannya dijaga prompt.
		complete = func(ctx context.Context, system, user string, _ any) (string, error) {
			return c.Complete(ctx, system, user, 8192)
		}
	default:
		c := ollama.New(p.Opts.OllamaURL, p.Opts.OllamaModel)
		c.Temperature = correctionTemperature
		engineName = "Ollama (" + c.Model + ")"
		complete = func(ctx context.Context, system, user string, schema any) (string, error) {
			return c.Complete(ctx, system, user, schema, 4096)
		}
	}

	cachePath := ""
	if cacheKey != "" {
		cachePath = correctedCachePath(p.Paths.DataDir, cacheKey, engineName, correct.CacheVersion(p.Opts.Terms))
		if cached, ok := loadTranscriptCache(cachePath); ok {
			emit(onProgress, Progress{Stage: "correcting", Value: 0.58,
				Message: "Corrected transcript loaded from cache"})
			return cached, nil
		}
	}

	// Jumlah istilah ikut dilaporkan: daftar yang tidak terkirim gagal secara
	// senyap — hasilnya sekadar "tidak ada yang berubah", persis seperti daftar
	// yang terkirim tapi tidak dipakai model. Tanpa angka ini keduanya mustahil
	// dibedakan tanpa membongkar berkas cache.
	terms := ""
	switch n := len(p.Opts.Terms); {
	case n == 1:
		terms = " (1 term)"
	case n > 1:
		terms = fmt.Sprintf(" (%d terms)", n)
	}
	emit(onProgress, Progress{Stage: "correcting", Value: 0.48,
		Message: fmt.Sprintf("%s is correcting the transcript%s", engineName, terms)})

	fixed, report, err := correct.Correct(ctx, tr, p.Opts.Terms, complete, engineName, func(done, total int) {
		emit(onProgress, Progress{
			Stage:   "correcting",
			Value:   0.48 + 0.10*float64(done)/float64(total),
			Message: fmt.Sprintf("%s is correcting the transcript — part %d/%d", engineName, done, total),
		})
	})
	if err != nil {
		// Mesin yang dibutuhkan tidak boleh dilewati diam-diam (notes/12).
		// Pesannya menyebut cara mematikan koreksi, supaya pengguna tanpa LLM
		// tidak kehabisan jalan.
		return types.Transcript{}, fmt.Errorf(
			"transcript correction failed: %w — fix the engine above, or turn correction off (-transcript-fix off in the CLI, or the checkbox in the GUI)", err)
	}

	emit(onProgress, Progress{Stage: "correcting", Value: 0.58, Message: report.Summary()})
	if cachePath != "" {
		// Gagal menyimpan cache tidak boleh menggagalkan job.
		_ = saveTranscriptCache(cachePath, fixed)
	}
	return fixed, nil
}

// momentSelector adalah mesin yang MEMILIH dari kandidat bernomor (Claude/Ollama).
//
// Dulu antarmuka ini bernama SelectMoments dan menyerahkan penentuan batas waktu
// ke model. Diukur pada qwen2.5, tiga kali jalan pada permintaan yang sama
// mengembalikan rentang terbalik ("468-43") dan momen 8 detik padahal diminta
// 30-60 — sebagian besar dibuang validateMoments, dan sisa yang lolos itulah
// yang membuat klip terasa berubah-ubah tiap kali dijalankan.
//
// Sekarang batas waktunya dibangun engine dari batas kalimat, dan model hanya
// memilih nomor. Kelas kegagalan itu hilang karena angkanya tidak pernah ada di
// tangan model.
type momentSelector interface {
	PickMoments(ctx context.Context, cands []types.Candidate, offset, maxClips int, contentLang string) ([]llm.Pick, error)
}

// selectWith menjalankan selektor per potongan transkrip lalu menggabungkan
// hasilnya. Kegagalan potongan mana pun menggagalkan seluruh job (tanpa
// fallback), agar pengguna tahu persis mesin mana yang bermasalah.
func (p *Pipeline) selectWith(ctx context.Context, tr types.Transcript, sel momentSelector, engineName string, onProgress ProgressFunc) ([]types.Clip, error) {
	// Kandidat dibangun dari SELURUH transkrip sekaligus, bukan per potongan.
	// Batas kandidat sudah ditentukan batas kalimat, jadi tidak ada momen yang
	// terbelah di sambungan potongan — penggabungan lintas potongan yang dulu
	// diperlukan (Moment.Continues) jadi tidak relevan.
	cands := segment.BuildCandidates(tr, p.Opts.TargetMin, p.Opts.TargetMax)
	if len(cands) == 0 {
		return nil, fmt.Errorf("no candidate clips could be built from the transcript")
	}

	// Dikirim berbatch: yang membatasi bukan kemampuan model membaca melainkan
	// jendela konteksnya. Model menilai per batch, penyaringan akhir tetap di
	// topN — jadi batch tidak mengurangi mutu pilihan, hanya membaginya.
	batch := llm.MaxCandidatesPerRequest
	batches := (len(cands) + batch - 1) / batch
	perBatch := p.Opts.MaxClips/batches + 1
	if perBatch < 2 {
		perBatch = 2
	}

	var clips []types.Clip
	for b := 0; b < batches; b++ {
		lo := b * batch
		hi := lo + batch
		if hi > len(cands) {
			hi = len(cands)
		}
		msg := fmt.Sprintf("%s is choosing from %d candidate clips", engineName, len(cands))
		if batches > 1 {
			msg = fmt.Sprintf("%s is choosing clips — part %d/%d", engineName, b+1, batches)
		}
		emit(onProgress, Progress{Stage: "scoring", Value: 0.58 + 0.15*float64(b)/float64(batches), Message: msg})

		picks, err := sel.PickMoments(ctx, cands[lo:hi], lo, perBatch, tr.Language)
		if err != nil {
			if batches > 1 {
				return nil, fmt.Errorf("part %d of %d: %w", b+1, batches, err)
			}
			return nil, err
		}
		got, bad := picksToClips(picks, cands, lo, hi)
		if bad > 0 {
			emit(onProgress, Progress{Stage: "scoring", Value: 0.73, Message: fmt.Sprintf(
				"%d pick(s) dropped: the number was not in the list", bad)})
		}
		clips = append(clips, got...)
	}
	if len(clips) == 0 {
		return nil, fmt.Errorf("%s chose none of the %d candidate clips", engineName, len(cands))
	}
	return topN(clips, p.Opts.MaxClips, p.Opts.MinScore), nil
}

// picksToClips memetakan nomor pilihan model kembali ke kandidatnya.
//
// Nomor di luar daftar dibuang, bukan dijepit ke tetangga terdekat: menjepit
// akan mengubah pilihan model jadi klip yang tidak pernah ia lihat. Nomor
// ganda juga dibuang — model kadang memilih kandidat yang sama dua kali.
func picksToClips(picks []llm.Pick, cands []types.Candidate, lo, hi int) ([]types.Clip, int) {
	seen := map[int]bool{}
	var out []types.Clip
	bad := 0
	for _, pk := range picks {
		if pk.Index < lo || pk.Index >= hi || seen[pk.Index] {
			bad++
			continue
		}
		seen[pk.Index] = true
		c := cands[pk.Index]
		out = append(out, types.Clip{
			Start:      c.Start,
			End:        c.End,
			Duration:   c.Duration(),
			Score:      int(math.Round(pk.Score)),
			Reasons:    llm.ToReasons(pk.Reasons, pk.Score),
			Title:      strings.TrimSpace(pk.Title),
			Hashtags:   pk.Hashtags,
			Transcript: c.Text,
			Status:     "scored",
		})
	}
	return out, bad
}

// heuristicSelect memilih klip via segmentasi window + skor heuristik (fallback).
func (p *Pipeline) heuristicSelect(tr types.Transcript, rms worker.FeaturesResult) []types.Clip {
	cands := segment.BuildCandidates(tr, p.Opts.TargetMin, p.Opts.TargetMax)
	var clips []types.Clip
	for _, c := range cands {
		total, reasons := heuristic.Score(c, p.Opts.TargetMin, p.Opts.TargetMax, rmsSlice(rms, c))
		clips = append(clips, types.Clip{
			Start: c.Start, End: c.End, Duration: c.Duration(),
			Score: total, Reasons: reasons, Transcript: c.Text, Status: "scored",
		})
	}
	return topN(clips, p.Opts.MaxClips, p.Opts.MinScore)
}

// joinRange menggabungkan teks segmen dalam rentang [start,end].
func joinRange(tr types.Transcript, start, end float64) string {
	var parts []string
	for _, s := range tr.Segments {
		if s.End > start && s.Start < end {
			parts = append(parts, s.Text)
		}
	}
	return strings.Join(parts, " ")
}

// topN mengurutkan menurun, menyaring skor minimum, mengambil n teratas.
func topN(clips []types.Clip, n, minScore int) []types.Clip {
	sort.Slice(clips, func(i, j int) bool { return clips[i].Score > clips[j].Score })
	var out []types.Clip
	for _, c := range clips {
		if c.Score < minScore {
			continue
		}
		out = append(out, c)
		if len(out) >= n {
			break
		}
	}
	if len(out) == 0 && len(clips) > 0 {
		out = clips[:min(n, len(clips))]
	}
	return out
}

// rmsSlice mengambil potongan RMS untuk jendela kandidat.
func rmsSlice(r worker.FeaturesResult, c types.Candidate) []float64 {
	if len(r.RMS) == 0 || r.HopMS <= 0 {
		return nil
	}
	hop := float64(r.HopMS) / 1000.0
	i := int(c.Start / hop)
	j := int(c.End / hop)
	if i < 0 {
		i = 0
	}
	if j > len(r.RMS) {
		j = len(r.RMS)
	}
	if i >= j {
		return nil
	}
	return r.RMS[i:j]
}

func segmentsInRange(tr types.Transcript, start, end float64) []types.TranscriptSegment {
	var out []types.TranscriptSegment
	for _, s := range tr.Segments {
		if s.End > start && s.Start < end {
			out = append(out, s)
		}
	}
	return out
}

// dropOpeningPreview menyaring momen yang berasal dari cuplikan pembuka.
//
// Dipisah dari pemanggilnya supaya bisa diuji tanpa menjalankan seluruh
// pipeline — dan supaya jelas bahwa ia hanya MENYARING, tidak mengubah momen.
func dropOpeningPreview(tr types.Transcript, clips []types.Clip) []types.Clip {
	// Ujung cuplikan dihitung sekali, bukan per klip: satu video punya satu
	// ujung cuplikan, dan menghitungnya ulang tiap klip hanya memindai seluruh
	// transkrip berkali-kali untuk jawaban yang sama.
	previewEnd := segment.OpeningPreviewEnd(tr)
	if previewEnd <= 0 {
		return clips
	}
	kept := make([]types.Clip, 0, len(clips))
	for _, c := range clips {
		// Cukup awalnya yang diperiksa. Klip yang MULAI di dalam cuplikan sudah
		// cacat betapa pun panjang ekornya — justru klip berkaki dua seperti
		// itulah yang dulu lolos.
		if c.Start < previewEnd {
			continue
		}
		kept = append(kept, c)
	}
	return kept
}
