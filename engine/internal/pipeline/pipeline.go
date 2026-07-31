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

// Progress dilaporkan selama proses.
type Progress struct {
	Stage   string      `json:"stage"`
	Value   float64     `json:"progress"`
	Message string      `json:"message,omitempty"`
	Clip    *types.Clip `json:"clip,omitempty"`
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
		if err := p.ff.ExtractAudioWAV(ctx, input, wav); err != nil {
			return nil, err
		}
		// 2. Fitur audio (opsional, dari worker C++).
		if p.wk.Available() {
			emit(onProgress, Progress{Stage: "extracting", Value: 0.12, Message: "Analysing audio energy (C++ worker)"})
			if r, err := p.wk.Features(ctx, wav, 100); err == nil {
				rms = r
			}
		}
	}

	// 3. Transkripsi (bagian paling lama; progress diparse dari whisper).
	if !cacheHit {
		emit(onProgress, Progress{Stage: "transcribing", Value: 0.2, Message: "Transcribing (whisper.cpp)"})
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
		if cachePath != "" {
			// Gagal menyimpan cache tidak boleh menggagalkan job.
			_ = saveTranscriptCache(cachePath, tr)
		}
	}
	if len(tr.Segments) == 0 {
		return nil, fmt.Errorf("the transcript is empty — check the audio/language")
	}

	// 3b. Koreksi transkrip. Dijalankan SEBELUM segmentasi & scoring, bukan
	// hanya sebelum subtitle: tanda baca yang salah tempat membuat
	// segment.BuildCandidates memotong klip di tengah kalimat.
	if p.Opts.TranscriptFix != config.TranscriptFixOff {
		fixed, err := p.correctTranscript(ctx, tr, cacheKey, onProgress)
		if err != nil {
			return nil, err
		}
		tr = fixed
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
	if len(selected) == 0 {
		return nil, fmt.Errorf("%s produced no clips — try loosening the duration preset or lowering the minimum score", engineName)
	}

	// 6. Render klip.
	tw, th := p.Opts.Dims()
	crf, preset := p.Opts.Encode()
	for i := range selected {
		cl := &selected[i]
		cl.ID = fmt.Sprintf("clip_%02d", i+1)
		cl.JobID = jobID
		frac := 0.75 + 0.24*float64(i+1)/float64(len(selected))
		emit(onProgress, Progress{Stage: "rendering", Value: frac,
			Message: fmt.Sprintf("Rendering %s (score %d)", cl.ID, cl.Score)})

		segs := segmentsInRange(tr, cl.Start, cl.End)

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

	emit(onProgress, Progress{Stage: "done", Value: 1.0, Message: fmt.Sprintf("Done: %d clips", len(selected))})
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
		cachePath = correctedCachePath(p.Paths.DataDir, cacheKey, engineName, correct.PromptVersion)
		if cached, ok := loadTranscriptCache(cachePath); ok {
			emit(onProgress, Progress{Stage: "correcting", Value: 0.58,
				Message: "Corrected transcript loaded from cache"})
			return cached, nil
		}
	}

	emit(onProgress, Progress{Stage: "correcting", Value: 0.48,
		Message: fmt.Sprintf("%s is correcting the transcript", engineName)})

	fixed, report, err := correct.Correct(ctx, tr, complete, engineName, func(done, total int) {
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

// momentSelector adalah mesin yang memilih momen dari transkrip (Claude/Ollama).
type momentSelector interface {
	SelectMoments(ctx context.Context, tr types.Transcript, maxClips int, targetMin, targetMax float64, ch llm.Chunk) ([]llm.Moment, error)
}

// selectWith menjalankan selektor per potongan transkrip lalu menggabungkan
// hasilnya. Kegagalan potongan mana pun menggagalkan seluruh job (tanpa
// fallback), agar pengguna tahu persis mesin mana yang bermasalah.
func (p *Pipeline) selectWith(ctx context.Context, tr types.Transcript, sel momentSelector, engineName string, onProgress ProgressFunc) ([]types.Clip, error) {
	parts := chunkTranscript(tr, chunkSeconds(p.Opts.Provider), chunkOverlap)
	if len(parts) == 0 {
		return nil, fmt.Errorf("the transcript is empty")
	}
	// Tiap potongan diminta secukupnya; penyaringan akhir tetap di topN.
	perChunk := p.Opts.MaxClips/len(parts) + 1
	if perChunk < 2 {
		perChunk = 2
	}

	var moments []llm.Moment
	for i, part := range parts {
		msg := fmt.Sprintf("%s is selecting moments", engineName)
		if len(parts) > 1 {
			msg = fmt.Sprintf("%s is selecting moments — part %d/%d (minute %.0f–%.0f)",
				engineName, i+1, len(parts), part.info.Start/60, part.info.End/60)
		}
		emit(onProgress, Progress{
			Stage:   "scoring",
			Value:   0.58 + 0.15*float64(i)/float64(len(parts)),
			Message: msg,
		})
		ms, err := sel.SelectMoments(ctx, part.tr, perChunk, p.Opts.TargetMin, p.Opts.TargetMax, part.info)
		if err != nil {
			if len(parts) > 1 {
				return nil, fmt.Errorf("part %d of %d (minute %.0f–%.0f): %w",
					i+1, len(parts), part.info.Start/60, part.info.End/60, err)
			}
			return nil, err
		}
		moments = append(moments, ms...)
	}

	merged := mergeMoments(moments)
	valid, rejected, err := validateMoments(merged, tr, engineName)
	if err != nil {
		return nil, err
	}
	if len(rejected) > 0 {
		emit(onProgress, Progress{Stage: "scoring", Value: 0.73,
			Message: fmt.Sprintf("%d moments dropped for invalid time boundaries", len(rejected))})
	}

	clips := momentsToClips(valid, tr)
	fitDuration(clips, tr, p.Opts.TargetMin, p.Opts.TargetMax)
	return topN(clips, p.Opts.MaxClips, p.Opts.MinScore), nil
}

// fitDuration merapikan momen pilihan LLM agar masuk rentang durasi: yang
// terlalu pendek diperpanjang ke batas segmen berikutnya (lalu ke belakang bila
// perlu), yang terlalu panjang dipangkas ke batas segmen sebelum targetMax.
// Model lokal sering mengabaikan instruksi durasi, jadi ini jaring pengaman.
func fitDuration(clips []types.Clip, tr types.Transcript, targetMin, targetMax float64) {
	if len(tr.Segments) == 0 {
		return
	}
	first := tr.Segments[0].Start
	last := tr.Segments[len(tr.Segments)-1].End

	for i := range clips {
		c := &clips[i]
		// Terlalu pendek → majukan akhir ke ujung segmen berikutnya.
		for c.End-c.Start < targetMin && c.End < last {
			next := last
			for _, s := range tr.Segments {
				if s.End > c.End {
					next = s.End
					break
				}
			}
			if next <= c.End {
				break
			}
			c.End = next
		}
		// Masih pendek → mundurkan awal ke pangkal segmen sebelumnya.
		for c.End-c.Start < targetMin && c.Start > first {
			prev := first
			for _, s := range tr.Segments {
				if s.Start < c.Start {
					prev = s.Start
				}
			}
			if prev >= c.Start {
				break
			}
			c.Start = prev
		}
		// Terlalu panjang → pangkas ke batas segmen terakhir sebelum targetMax.
		if c.End-c.Start > targetMax {
			limit := c.Start + targetMax
			cut := 0.0
			for _, s := range tr.Segments {
				if s.End > c.Start && s.End <= limit {
					cut = s.End
				}
			}
			if cut > c.Start+targetMin {
				c.End = cut
			}
		}
		c.Duration = c.End - c.Start
		c.Transcript = joinRange(tr, c.Start, c.End)
	}
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

// momentsToClips mengubah momen LLM jadi klip; batas dirapikan ke segmen terdekat.
func momentsToClips(moments []llm.Moment, tr types.Transcript) []types.Clip {
	var clips []types.Clip
	for _, m := range moments {
		start, end := snapToSegments(tr, m.Start, m.End)
		if end-start < 3 { // buang yang terlalu pendek/aneh
			continue
		}
		sc := int(math.Round(m.Score))
		reasons := types.Reasons{
			Hook:         int(math.Round(m.Reasons.Hook)),
			Emotion:      int(math.Round(m.Reasons.Emotion)),
			Clarity:      int(math.Round(m.Reasons.Clarity)),
			Shareability: int(math.Round(m.Reasons.Shareability)),
			Standalone:   int(math.Round(m.Reasons.Standalone)),
		}
		// Model lokal (prompt sederhana) tak mengisi reasons → ratakan dari skor.
		if reasons == (types.Reasons{}) && sc > 0 {
			reasons = types.Reasons{Hook: sc, Emotion: sc, Clarity: sc, Shareability: sc, Standalone: sc}
		}
		clips = append(clips, types.Clip{
			Start: start, End: end, Duration: end - start,
			Score: sc, Reasons: reasons, Title: m.Title, Hashtags: m.Hashtags,
			Transcript: joinRange(tr, start, end), Status: "scored",
		})
	}
	return clips
}

// snapToSegments merapikan start/end LLM ke batas segmen transkrip terdekat.
func snapToSegments(tr types.Transcript, s, e float64) (float64, float64) {
	ns, ne := s, e
	bestS, bestE := math.MaxFloat64, math.MaxFloat64
	for _, seg := range tr.Segments {
		if d := math.Abs(seg.Start - s); d < bestS {
			bestS, ns = d, seg.Start
		}
		if d := math.Abs(seg.End - e); d < bestE {
			bestE, ne = d, seg.End
		}
	}
	if ne <= ns {
		ne = e
	}
	return ns, ne
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
