// Package config memuat opsi runtime dan default engine.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Mode operasi engine.
type Mode string

const (
	ModeOffline Mode = "offline"
	ModeHybrid  Mode = "hybrid"
	ModeOnline  Mode = "online"
)

// Reframe menentukan cara membuat rasio 9:16.
type Reframe string

const (
	ReframeCenter     Reframe = "center"      // isi penuh (crop/zoom)
	ReframeFit        Reframe = "fit"         // muat utuh + latar blur (tanpa zoom)
	ReframeFaceFollow Reframe = "face_follow" // ikut wajah (tahap lanjut)
)

// Pilihan berkas keluaran klip.
const (
	OutputBurn  = "burn"  // 1 file: video dengan subtitle dibakar
	OutputClean = "clean" // 1 file: video polos tanpa subtitle
	OutputBoth  = "both"  // 2 file: polos + bersubtitle
)

// ScoreEngine menentukan mesin penilaian.
type ScoreEngine string

const (
	ScoreHeuristic    ScoreEngine = "heuristic"
	ScoreHeuristicLLM ScoreEngine = "heuristic_llm"
	ScoreLLM          ScoreEngine = "llm"
)

// Paths ke binary & folder yang dipakai engine.
type Paths struct {
	FFmpeg   string
	FFprobe  string
	Whisper  string
	Model    string
	Worker   string
	DataDir  string
	FontsDir string
	APIKey   string
}

// Subtitle mengatur tampilan subtitle. Koordinat memakai ruang PlayRes
// 1080x1920 (di-scale otomatis ke resolusi output oleh libass).
type Subtitle struct {
	Font         string `json:"font"`  // nama family, mis. "Montserrat"
	Size         int    `json:"size"`  // ukuran font di ruang 1080x1920
	X            int    `json:"x"`     // titik jangkar (tengah teks) 0..1080
	Y            int    `json:"y"`     // 0..1920
	Color        string `json:"color"` // "white" | "yellow" | "#RRGGBB"
	Bold         bool   `json:"bold"`
	Outline      int    `json:"outline"`       // ketebalan garis tepi (0=tanpa)
	OutlineColor string `json:"outline_color"` // warna tepi (default hitam)
	Box          bool   `json:"box"`           // latar kotak di belakang teks

	// Mode tampilan: "normal" (kalimat utuh), "karaoke" (kata aktif disorot,
	// sisa kalimat tetap terlihat), "word" (satu kata per layar).
	Mode           string `json:"mode"`
	HighlightColor string `json:"highlight_color"` // warna sorot untuk karaoke/word
	Speed          string `json:"speed"`           // lambat | normal | padat

	// Karaoke: field lama, dipertahankan agar preset tersimpan tidak rusak.
	// Diterjemahkan ke Mode="karaoke" saat Validate.
	Karaoke bool `json:"karaoke"`
}

// Mode subtitle yang dikenali.
const (
	SubNormal  = "normal"
	SubKaraoke = "karaoke"
	SubWord    = "word"
)

// Pacing menerjemahkan Speed ke durasi minimum satu tampilan (detik) dan
// maksimum baris per tampilan. Makin lambat = makin sedikit teks sekaligus.
func (s Subtitle) Pacing() (minDur float64, maxLines int) {
	switch s.Speed {
	case "lambat":
		return 1.6, 2
	case "padat":
		return 0.9, 3
	default: // normal
		return 1.2, 2
	}
}

// Options untuk satu job clipping.
type Options struct {
	Mode           Mode        `json:"mode"`
	Language       string      `json:"language"`
	WhisperModel   string      `json:"whisper_model"`
	Device         string      `json:"device"`
	Aspect         string      `json:"aspect"`
	Resolution     string      `json:"resolution"` // 720p | 1080p | 1440p
	Quality        string      `json:"quality"`    // draft | hd | max
	FPS            int         `json:"fps"`        // 0 = ikut sumber
	Reframe        Reframe     `json:"reframe"`
	SubtitleStyle  string      `json:"subtitle_style"` // plain | viral (fallback warna)
	Subtitle       Subtitle    `json:"subtitle"`
	SubtitleOutput string      `json:"subtitle_output"` // burn | clean | both
	MaxClips       int         `json:"max_clips"`
	DurationPreset string      `json:"duration_preset"` // auto | 30 | 60 | 90 | 120 | 180
	TargetMin      float64     `json:"target_min"`
	TargetMax      float64     `json:"target_max"`
	ScoreEngine    ScoreEngine `json:"score_engine"`
	Provider       string      `json:"provider"`     // claude | ollama (mesin scoring)
	LLMModel       string      `json:"llm_model"`    // model Claude (mode hybrid)
	OllamaModel    string      `json:"ollama_model"` // model lokal (mode offline)
	OllamaURL      string      `json:"ollama_url"`   // default http://localhost:11434
	MinScore       int         `json:"min_score"`
	OutputDir      string      `json:"output_dir"`
}

// DefaultOptions mengembalikan opsi default (konten Indonesia, CPU, HD).
func DefaultOptions() Options {
	return Options{
		Mode:           ModeOffline,
		Language:       "id",
		WhisperModel:   "small",
		Device:         "cpu",
		Aspect:         "9:16",
		Resolution:     "1080p",
		Quality:        "hd",
		Reframe:        ReframeCenter,
		SubtitleStyle:  "plain",
		Subtitle:       DefaultSubtitle(),
		SubtitleOutput: OutputBurn,
		MaxClips:       10,
		DurationPreset: "auto",
		ScoreEngine:    ScoreHeuristic,
		// Provider sengaja kosong: Validate memilih menurut mode (offline →
		// ollama, hybrid → claude). Dulu diisi "claude" sehingga mode offline
		// pun ikut memanggil API Claude.
		Provider: "",
		LLMModel:       "claude-haiku-4-5",
		OllamaModel:    "qwen2.5",
		OllamaURL:      "http://localhost:11434",
		MinScore:       0,
	}
}

// DefaultSubtitle: Montserrat, besar, posisi tengah, garis tepi hitam.
func DefaultSubtitle() Subtitle {
	return Subtitle{
		Font:         "Montserrat",
		Size:         72,
		X:            540,
		Y:            960,
		Color:          "white",
		Bold:           true,
		Outline:        4,
		OutlineColor:   "black",
		Box:            false,
		Mode:           SubNormal,
		HighlightColor: "yellow",
		Speed:          "normal",
	}
}

// Validate mengisi nilai kosong dengan default & menerjemahkan preset.
func (o *Options) Validate() error {
	d := DefaultOptions()
	if o.Mode == "" {
		o.Mode = d.Mode
	}
	if o.Language == "" {
		o.Language = d.Language
	}
	if o.WhisperModel == "" {
		o.WhisperModel = d.WhisperModel
	}
	if o.Device == "" {
		o.Device = d.Device
	}
	if o.Aspect == "" {
		o.Aspect = d.Aspect
	}
	if o.Resolution == "" {
		o.Resolution = d.Resolution
	}
	if o.Quality == "" {
		o.Quality = d.Quality
	}
	if o.Reframe == "" {
		o.Reframe = d.Reframe
	}
	if o.SubtitleStyle == "" {
		o.SubtitleStyle = d.SubtitleStyle
	}
	// Lengkapi subtitle yang kosong.
	ds := DefaultSubtitle()
	if o.Subtitle.Font == "" {
		o.Subtitle.Font = ds.Font
	}
	if o.Subtitle.Size <= 0 {
		o.Subtitle.Size = ds.Size
	}
	if o.Subtitle.X <= 0 {
		o.Subtitle.X = ds.X
	}
	if o.Subtitle.Y <= 0 {
		o.Subtitle.Y = ds.Y
	}
	if o.Subtitle.Color == "" {
		if o.SubtitleStyle == "viral" {
			o.Subtitle.Color = "yellow"
		} else {
			o.Subtitle.Color = ds.Color
		}
	}
	if o.Subtitle.Outline < 0 {
		o.Subtitle.Outline = ds.Outline
	}
	if o.Subtitle.OutlineColor == "" {
		o.Subtitle.OutlineColor = ds.OutlineColor
	}
	// Mode: kosong → ikut field lama "karaoke" bila diset, selain itu normal.
	switch o.Subtitle.Mode {
	case SubNormal, SubKaraoke, SubWord:
	default:
		if o.Subtitle.Karaoke {
			o.Subtitle.Mode = SubKaraoke
		} else {
			o.Subtitle.Mode = SubNormal
		}
	}
	if o.Subtitle.HighlightColor == "" {
		o.Subtitle.HighlightColor = ds.HighlightColor
	}
	if o.Subtitle.Speed == "" {
		o.Subtitle.Speed = ds.Speed
	}
	switch o.SubtitleOutput {
	case OutputBurn, OutputClean, OutputBoth:
	default:
		o.SubtitleOutput = d.SubtitleOutput
	}
	if o.MaxClips <= 0 {
		o.MaxClips = d.MaxClips
	}
	if o.DurationPreset == "" {
		o.DurationPreset = d.DurationPreset
	}
	// Preset durasi → target min/max (bila target belum diisi manual).
	if o.TargetMin <= 0 || o.TargetMax <= 0 {
		o.TargetMin, o.TargetMax = durationRange(o.DurationPreset)
	}
	if o.LLMModel == "" {
		o.LLMModel = d.LLMModel
	}
	if o.OllamaModel == "" {
		o.OllamaModel = d.OllamaModel
	}
	if o.OllamaURL == "" {
		o.OllamaURL = d.OllamaURL
	}
	// Provider default menurut mode (pipeline yang memakainya).
	if o.Provider == "" {
		if o.Mode == ModeHybrid || o.Mode == ModeOnline {
			o.Provider = "claude"
		} else {
			o.Provider = "ollama"
		}
	}
	return nil
}

// durationRange menerjemahkan preset ke rentang detik.
func durationRange(preset string) (min, max float64) {
	switch preset {
	case "auto", "":
		// Rentang lebar sebelumnya (30–180) tak pernah terpakai: segmentasi
		// selalu berhenti di batas bawah. 45–120 memberi variasi yang wajar.
		return 45, 120
	case "30":
		return 24, 40
	case "60":
		return 48, 75
	case "90":
		return 72, 105
	case "120":
		return 100, 140
	case "180":
		return 150, 200
	default:
		return 30, 180
	}
}

// Dims mengembalikan lebar & tinggi output sesuai resolusi & rasio.
func (o Options) Dims() (int, int) {
	// Tinggi sisi panjang untuk rasio potret.
	long := 1920
	switch o.Resolution {
	case "720p":
		long = 1280
	case "1080p":
		long = 1920
	case "1440p":
		long = 2560
	}
	switch o.Aspect {
	case "9:16":
		return roundEven(long * 9 / 16), long
	case "16:9":
		return long, roundEven(long * 9 / 16)
	case "1:1":
		s := long * 9 / 16
		return roundEven(s), roundEven(s)
	default:
		return roundEven(long * 9 / 16), long
	}
}

// Encode mengembalikan parameter crf & preset x264 sesuai kualitas.
func (o Options) Encode() (crf string, preset string) {
	switch o.Quality {
	case "draft":
		return "28", "veryfast"
	case "max":
		return "18", "slow"
	default: // hd
		return "20", "medium"
	}
}

func roundEven(n int) int {
	if n%2 != 0 {
		n++
	}
	return n
}

func firstExisting(cands ...string) string {
	for _, c := range cands {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if len(cands) > 0 {
		return cands[len(cands)-1]
	}
	return ""
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ResolvePaths menemukan binary & folder relatif terhadap root proyek.
func ResolvePaths(root string, o Options) Paths {
	dataDir := env("CLIPPER_DATA_DIR", filepath.Join(root, "data"))
	model := env("CLIPPER_WHISPER_MODEL",
		filepath.Join(root, "models", fmt.Sprintf("ggml-%s.bin", o.WhisperModel)))
	whisper := env("CLIPPER_WHISPER_BIN",
		firstExisting(
			filepath.Join(root, "bin", "whisper-cli"),
			filepath.Join(root, "bin", "main"),
			"whisper-cli",
		))
	worker := env("CLIPPER_WORKER_BIN",
		firstExisting(filepath.Join(root, "bin", "clipper-worker"), "clipper-worker"))
	return Paths{
		FFmpeg:   env("CLIPPER_FFMPEG_BIN", "ffmpeg"),
		FFprobe:  env("CLIPPER_FFPROBE_BIN", "ffprobe"),
		Whisper:  whisper,
		Model:    model,
		Worker:   worker,
		DataDir:  dataDir,
		FontsDir: env("CLIPPER_FONTS_DIR", filepath.Join(root, "assets", "fonts")),
		APIKey:   os.Getenv("ANTHROPIC_API_KEY"),
	}
}
