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
	Stage    string      `json:"stage"`
	Value    float64     `json:"progress"`
	Message  string      `json:"message,omitempty"`
	Clip     *types.Clip `json:"clip,omitempty"`
}

// ProgressFunc callback progres (boleh nil).
type ProgressFunc func(Progress)

// Pipeline menyimpan dependensi & konfigurasi.
type Pipeline struct {
	Paths  config.Paths
	Opts   config.Options
	ff     *ffmpeg.Client
	wh     *transcribe.Whisper
	wk     *worker.Client
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
	// File sementara (audio/transkrip) di workDir; klip final di outDir.
	if outDir == "" {
		outDir = workDir
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("folder output %q: %w", outDir, err)
	}

	// 1. Ekstrak audio.
	emit(onProgress, Progress{Stage: "extracting", Value: 0.05, Message: "Mengekstrak audio"})
	wav := filepath.Join(workDir, "audio.wav")
	if err := p.ff.ExtractAudioWAV(ctx, input, wav); err != nil {
		return nil, err
	}

	// 2. Fitur audio (opsional, dari worker C++).
	var rms worker.FeaturesResult
	if p.wk.Available() {
		emit(onProgress, Progress{Stage: "extracting", Value: 0.12, Message: "Analisis energi audio (worker C++)"})
		if r, err := p.wk.Features(ctx, wav, 100); err == nil {
			rms = r
		}
	}

	// 3. Transkripsi (bagian paling lama; progress diparse dari whisper).
	emit(onProgress, Progress{Stage: "transcribing", Value: 0.2, Message: "Transkripsi (whisper.cpp)"})
	outBase := filepath.Join(workDir, "transcript")
	tr, err := p.wh.Transcribe(ctx, wav, outBase, p.Opts.Language, runtime.NumCPU(), func(f float64) {
		// Petakan 0..1 transkripsi ke pita 0.20..0.53 dari total.
		emit(onProgress, Progress{
			Stage:   "transcribing",
			Value:   0.20 + 0.33*f,
			Message: fmt.Sprintf("Transkripsi %.0f%%", f*100),
		})
	})
	if err != nil {
		return nil, err
	}
	if len(tr.Segments) == 0 {
		return nil, fmt.Errorf("transkrip kosong — cek audio/bahasa")
	}

	// 4-5. Pemilihan momen. Bila LLM tersedia, LLM menentukan batas momen
	// (start/end) sehingga durasi bervariasi sesuai isi. Jika tidak, heuristik.
	var selected []types.Clip
	var sel momentSelector
	label := ""
	switch p.Opts.Provider {
	case "claude":
		if p.Paths.APIKey != "" {
			sel = llm.New(p.Paths.APIKey, p.Opts.LLMModel)
			label = "AI (Claude) memilih momen dari transkrip"
		}
	case "ollama":
		sel = ollama.New(p.Opts.OllamaURL, p.Opts.OllamaModel)
		label = "AI lokal (Ollama: " + p.Opts.OllamaModel + ") memilih momen"
	}
	if sel != nil {
		emit(onProgress, Progress{Stage: "scoring", Value: 0.58, Message: label})
		clips, err := p.selectWith(ctx, tr, sel)
		if err != nil {
			emit(onProgress, Progress{Stage: "scoring", Value: 0.6,
				Message: "LLM gagal (" + err.Error() + "), memakai heuristik"})
			selected = p.heuristicSelect(tr, rms)
		} else {
			selected = clips
		}
	} else {
		emit(onProgress, Progress{Stage: "segmenting", Value: 0.55, Message: "Memilih klip (heuristik)"})
		selected = p.heuristicSelect(tr, rms)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("tidak ada klip terpilih — cek transkrip/opsi")
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
			Message: fmt.Sprintf("Render %s (skor %d)", cl.ID, cl.Score)})

		assPath := filepath.Join(workDir, cl.ID+".ass") // subtitle = file sementara
		segs := segmentsInRange(tr, cl.Start, cl.End)
		if err := subtitle.WriteASS(assPath, segs, cl.Start, p.Opts.Subtitle); err != nil {
			return nil, err
		}
		outMP4 := filepath.Join(outDir, cl.ID+".mp4") // klip final ke folder output
		enc := ffmpeg.EncodeOpts{
			CRF: crf, Preset: preset, AssPath: assPath, FontsDir: p.Paths.FontsDir,
			Mode: string(p.Opts.Reframe), FPS: p.Opts.FPS,
		}
		if err := p.ff.ClipReframe(ctx, input, cl.Start, cl.End, tw, th, enc, outMP4); err != nil {
			return nil, err
		}
		cl.SubtitlePath = assPath
		cl.VideoPath = outMP4
		cl.Status = "rendered"
		emit(onProgress, Progress{Stage: "rendering", Value: frac, Clip: cl})
	}

	emit(onProgress, Progress{Stage: "done", Value: 1.0, Message: fmt.Sprintf("Selesai: %d klip", len(selected))})
	return selected, nil
}

// momentSelector adalah mesin yang memilih momen dari transkrip (Claude/Ollama).
type momentSelector interface {
	SelectMoments(ctx context.Context, tr types.Transcript, maxClips int, targetMin, targetMax float64) ([]llm.Moment, error)
}

// selectWith menjalankan selektor → klip (batas dirapikan & di-topN).
func (p *Pipeline) selectWith(ctx context.Context, tr types.Transcript, sel momentSelector) ([]types.Clip, error) {
	moments, err := sel.SelectMoments(ctx, tr, p.Opts.MaxClips, p.Opts.TargetMin, p.Opts.TargetMax)
	if err != nil {
		return nil, err
	}
	return topN(momentsToClips(moments, tr), p.Opts.MaxClips, p.Opts.MinScore), nil
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
