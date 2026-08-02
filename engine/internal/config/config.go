// Package config memuat opsi runtime dan default engine.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gemgum/clipper/engine/internal/capture"
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

// Tiga cara memasangkan video ke bingkai 9:16. Ketiganya pilihan yang berdiri
// sendiri — bukan titik pada satu sumbu.
const (
	// ReframeCenter: potong tengah sampai memenuhi bingkai. "Center of the
	// Picture" di antarmuka.
	ReframeCenter Reframe = "center"

	// ReframeFit: ambil seluruh resolusi video tanpa crop, sehingga SELURUH
	// gambar masuk. Sisa ruangnya diisi Background — inilah alasan pilihan blur
	// & hitam ada. "Whole Picture" di antarmuka.
	ReframeFit Reframe = "fit"

	// ReframeFaceFollow: potong mengikuti wajah. "Follow Face" di antarmuka.
	ReframeFaceFollow Reframe = "face_follow" // BELUM TERSEDIA
)

// Check melaporkan apakah mode reframe ini sudah bisa dipakai.
//
// face_follow sudah punya nama dan rencana, tapi deteksi wajahnya belum ada di
// worker C++. Tanpa pemeriksaan ini ffmpeg diam-diam merendernya sebagai center
// — persis jenis penggantian senyap yang dilarang di notes/12.
func (r Reframe) Check() error {
	switch r {
	case ReframeCenter, ReframeFit:
		return nil
	case ReframeFaceFollow:
		return fmt.Errorf("reframe mode %q is not available yet (face detection in the C++ worker is not built) — use %q or %q",
			r, ReframeCenter, ReframeFit)
	default:
		return fmt.Errorf("unknown reframe mode %q — choose %q or %q", r, ReframeCenter, ReframeFit)
	}
}

// Background = isi ruang kosong saat video tidak memenuhi bingkai 9:16 —
// terjadi di mode "fit", atau di mode "center" ketika Zoom di bawah 100%.
const (
	BackgroundBlur  = "blur"
	BackgroundBlack = "black"
)

// Zoom = seberapa besar video duduk di dalam bingkai, dalam persen dari ukuran
// ALAMI modenya (100 = ukuran alami). Terpisah dari Reframe.
//
// Di atas 100 gambar diperbesar dan bagian yang melewati bingkai dipotong —
// itulah satu-satunya cara memperbesar video di mode Whole Picture, yang pada
// 100 sudah pas di bingkai. Kelipatan 5 supaya pilihannya terbatas dan hasilnya
// bisa diulang.
const (
	// 100 = gambar memenuhi bingkai. Itu batas atas keduanya: memperbesar lagi
	// tidak menambah apa pun selain memotong lebih banyak.
	ZoomMax  = 100
	ZoomStep = 5

	// Batas bawah berbeda per mode, sebab titik awalnya ada di ujung yang
	// berlawanan:
	//   fit    → mulai di 0 (seluruh video masuk), hanya bisa NAIK;
	//   center → mulai di 100 (memenuhi bingkai), hanya bisa TURUN. Kotak
	//            potongannya tidak boleh nol, jadi berhenti di 5.
	ZoomWholeMin  = 0
	ZoomCenterMin = 5

	// Nilai alami tiap mode — dipakai sebagai titik awal & saat zoom tak diisi.
	// Sengaja BUKAN ZoomMax: keduanya sempat kebetulan sama, dan begitu batas
	// atas dinaikkan, nilai "belum diisi" ikut melompat tanpa ada yang meminta.
	ZoomWholeNatural  = 0
	ZoomCenterNatural = 100
)

// NaturalZoom mengembalikan titik awal zoom untuk sebuah mode.
func NaturalZoom(r Reframe) int {
	if r == ReframeFit {
		return ZoomWholeNatural
	}
	return ZoomCenterNatural
}

// Pilihan berkas keluaran klip.
const (
	OutputBurn  = "burn"  // 1 file: video dengan subtitle dibakar
	OutputClean = "clean" // 1 file: video polos tanpa subtitle
	OutputBoth  = "both"  // 2 file: polos + bersubtitle
)

// Koreksi transkrip sebelum dipakai pipeline. Menyala secara default: keluaran
// mentah whisper membawa tanda hubung dialog, tanda baca salah tempat, dan kata
// salah dengar — semuanya ikut terbakar ke subtitle DAN menyesatkan segmentasi.
//
// Butuh LLM, termasuk saat mesin skornya heuristik. Bila mesinnya tak
// terjangkau job berhenti dengan pesan sebabnya, bukan diam-diam dilewati
// (notes/12); matikan lewat "off" bila memang ingin transkrip mentah.
const (
	TranscriptFixOn  = "on"
	TranscriptFixOff = "off"
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
	Chrome   string // browser untuk merender kartu berita (boleh kosong)
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
	Speed          string `json:"speed"`           // slow | normal | dense

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

// Kecepatan tampil subtitle yang dikenali.
const (
	SpeedSlow   = "slow"
	SpeedNormal = "normal"
	SpeedDense  = "dense"
)

// Pacing menerjemahkan Speed ke durasi minimum satu tampilan (detik) dan
// maksimum baris per tampilan. Makin lambat = makin sedikit teks sekaligus.
func (s Subtitle) Pacing() (minDur float64, maxLines int) {
	switch s.Speed {
	case SpeedSlow:
		return 1.6, 2
	case SpeedDense:
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
	Background     string      `json:"background"`     // blur | black
	Zoom           int         `json:"zoom"`           // 0..100, kelipatan 5
	SubtitleStyle  string      `json:"subtitle_style"` // plain | viral (fallback warna)
	Subtitle       Subtitle    `json:"subtitle"`
	SubtitleOutput string      `json:"subtitle_output"` // burn | clean | both
	MaxClips       int         `json:"max_clips"`
	DurationPreset string      `json:"duration_preset"` // auto | 30 | 60 | 90 | 120 | 180
	TargetMin      float64     `json:"target_min"`
	TargetMax      float64     `json:"target_max"`
	ScoreEngine    ScoreEngine `json:"score_engine"`
	TranscriptFix  string      `json:"transcript_fix"` // on | off (default on)
	// Terms adalah ejaan baku nama & istilah khas video ini, dipakai tahap
	// koreksi untuk menarik salah dengar ke ejaan yang benar. Whisper tidak
	// mengenal nama daerah atau istilah Jawa, jadi ia menuliskannya sebagai kata
	// Indonesia terdekat ("Londo Ireng" → "Londo Irang"), dan tanpa daftar ini
	// tahap koreksi justru diperintahkan membiarkan kata asing apa adanya.
	Terms       []string `json:"terms"`
	Provider    string   `json:"provider"`     // claude | ollama (mesin scoring)
	LLMModel    string   `json:"llm_model"`    // model Claude (mode hybrid)
	OllamaModel string   `json:"ollama_model"` // model lokal (mode offline)
	OllamaURL   string   `json:"ollama_url"`   // default http://localhost:11434
	MinScore    int      `json:"min_score"`
	OutputDir   string   `json:"output_dir"`
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
		Background:     BackgroundBlur,
		Zoom:           ZoomCenterNatural,
		SubtitleStyle:  "plain",
		Subtitle:       DefaultSubtitle(),
		SubtitleOutput: OutputBurn,
		MaxClips:       10,
		DurationPreset: "auto",
		ScoreEngine:    ScoreHeuristic,
		TranscriptFix:  TranscriptFixOn,
		// Provider sengaja kosong: Validate memilih menurut mode (offline →
		// ollama, hybrid → claude). Dulu diisi "claude" sehingga mode offline
		// pun ikut memanggil API Claude.
		Provider:    "",
		LLMModel:    "claude-haiku-4-5",
		OllamaModel: "qwen2.5",
		OllamaURL:   "http://localhost:11434",
		MinScore:    0,
	}
}

// DefaultSubtitle: Montserrat, besar, posisi tengah, garis tepi hitam.
func DefaultSubtitle() Subtitle {
	return Subtitle{
		Font:           "Montserrat",
		Size:           72,
		X:              540,
		Y:              960,
		Color:          "white",
		Bold:           true,
		Outline:        4,
		OutlineColor:   "black",
		Box:            false,
		Mode:           SubNormal,
		HighlightColor: "yellow",
		Speed:          SpeedNormal,
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
	if err := o.Reframe.Check(); err != nil {
		return err
	}
	switch o.Background {
	case BackgroundBlur, BackgroundBlack:
	default:
		o.Background = d.Background
	}
	// Zoom dijepit lalu dibulatkan ke kelipatan 5. Permintaan yang tidak
	// mengirim field ini tetap mendapat default, sebab api.createJob menyemai
	// DefaultOptions sebelum mendekode JSON.
	// Batas bawah zoom bergantung mode: di fit, 0 berarti "seluruh video masuk"
	// dan itu titik awalnya; di center, kotak potongan tidak boleh nol.
	min := ZoomCenterMin
	if o.Reframe == ReframeFit {
		min = ZoomWholeMin
	} else if o.Zoom <= 0 {
		o.Zoom = ZoomCenterNatural // belum diisi
	}
	if o.Zoom > ZoomMax {
		o.Zoom = ZoomMax
	}
	o.Zoom = (o.Zoom / ZoomStep) * ZoomStep
	if o.Zoom < min {
		o.Zoom = min
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
	switch o.TranscriptFix {
	case TranscriptFixOn, TranscriptFixOff:
	default:
		// Permintaan lama tidak mengenal field ini; koreksi menyala kecuali
		// dimatikan secara eksplisit.
		o.TranscriptFix = d.TranscriptFix
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
		FFmpeg:  env("CLIPPER_FFMPEG_BIN", "ffmpeg"),
		FFprobe: env("CLIPPER_FFPROBE_BIN", "ffprobe"),
		Whisper: whisper,
		Model:   model,
		Worker:  worker,
		// Browser dicari oleh paket capture (termasuk chrome.exe lewat WSL).
		// Boleh kosong: hanya fitur kartu berita yang membutuhkannya.
		Chrome:   capture.Find(),
		DataDir:  dataDir,
		FontsDir: env("CLIPPER_FONTS_DIR", filepath.Join(root, "assets", "fonts")),
		APIKey:   os.Getenv("ANTHROPIC_API_KEY"),
	}
}
