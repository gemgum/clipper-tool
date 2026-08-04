// Package setup memeriksa dan memasang komponen yang dibutuhkan engine:
// ffmpeg, whisper.cpp, dan model whisper.
//
// Sampai kini pemasangan adalah pekerjaan pengembang — setup.sh, cmake, g++.
// Tidak satu pun boleh dituntut dari pengguna aplikasi desktop, jadi paket ini
// mengerjakannya: unduh rilis biner resmi, bongkar ke ToolsDir/ModelsDir, dan
// laporkan kemajuannya selagi berjalan.
//
// Yang TIDAK dipasang di sini: Ollama dan Chrome. Keduanya aplikasi utuh milik
// orang lain dengan pemasangnya sendiri; yang bisa dilakukan hanyalah mendeteksi
// keberadaannya dan menunjukkan halaman unduhnya.
package setup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gemgum/clipper/engine/internal/capture"
	"github.com/gemgum/clipper/engine/internal/config"
)

// Kind komponen — menentukan bentuk barisnya di halaman Requirements.
const (
	KindTool  = "tool"  // biner yang dipasang engine
	KindModel = "model" // model whisper
	KindApp   = "app"   // aplikasi terpisah, hanya dideteksi
)

// Component adalah satu baris di halaman Requirements.
type Component struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Required    bool   `json:"required"`
	Installed   bool   `json:"installed"`
	Path        string `json:"path"`   // di mana ia ditemukan
	Detail      string `json:"detail"` // keterangan singkat untuk pengguna
	Size        string `json:"size"`   // perkiraan besar unduhan
	Installable bool   `json:"installable"`
	Hint        string `json:"hint"` // cara memasang manual, bila engine tak bisa
	URL         string `json:"url"`  // halaman unduh untuk pemasangan manual
}

// Model whisper yang dikenal, beserta perkiraan ukurannya. Satu-satunya daftar
// model di engine — /api/models ikut membacanya dari sini.
var Models = []struct {
	Name string
	Size string
}{
	{"tiny", "~75 MB"},
	{"base", "~142 MB"},
	{"small", "~466 MB"},
	{"medium", "~1.5 GB"},
	{"large-v3", "~2.9 GB"},
	{"large-v3-turbo", "~1.5 GB"},
}

// ModelID membentuk id komponen untuk sebuah model.
func ModelID(name string) string { return "model:" + name }

// ModelPath mengembalikan letak berkas model di dalam layout.
func ModelPath(l config.Layout, name string) string {
	return filepath.Join(l.ModelsDir, "ggml-"+name+".bin")
}

// HasModel melaporkan apakah sebuah model sudah terunduh.
func HasModel(l config.Layout, name string) bool {
	_, err := os.Stat(ModelPath(l, name))
	return err == nil
}

// RemoveModel menghapus satu model yang sudah diunduh.
func RemoveModel(l config.Layout, name string) error {
	for _, m := range Models {
		if m.Name == name {
			if err := os.Remove(ModelPath(l, name)); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("unknown whisper model %q", name)
}

// Status melaporkan keadaan semua komponen.
//
// Urutannya sengaja: yang wajib dulu, lalu model, lalu yang opsional. Halaman
// Requirements menampilkannya apa adanya — pengguna membaca dari atas, dan yang
// paling menghalanginya harus muncul lebih dulu.
func Status(l config.Layout, ollamaOK bool, ollamaDetail string) []Component {
	out := []Component{
		toolStatus(l, "ffmpeg", "ffmpeg", "Cuts and renders video. Required."),
		toolStatus(l, "ffprobe", "ffprobe", "Reads video length and size. Comes with ffmpeg."),
		whisperStatus(l),
	}
	for _, m := range Models {
		p := ModelPath(l, m.Name)
		_, err := os.Stat(p)
		c := Component{
			ID: ModelID(m.Name), Name: "Whisper model: " + m.Name, Kind: KindModel,
			Size: m.Size, Installable: true,
			Detail: "Speech recognition. " + modelNote(m.Name),
		}
		if err == nil {
			c.Installed, c.Path = true, p
		}
		out = append(out, c)
	}

	ollama := Component{
		ID: "ollama", Name: "Ollama", Kind: KindApp,
		Detail:      "Local LLM, used by offline mode and by transcript correction.",
		Installed:   ollamaOK,
		Hint:        "Install Ollama, start it, then pull a model (llama3.1 is the best pick for the term list).",
		URL:         "https://ollama.com/download",
		Installable: false,
	}
	if ollamaDetail != "" {
		ollama.Detail = ollamaDetail
	}
	out = append(out, ollama)

	chrome := Component{
		ID: "chrome", Name: "Chrome / Edge", Kind: KindApp,
		Detail:      "Renders the news cards. Not needed for video clips.",
		Hint:        "Install Chrome or Chromium — the Edge that ships with Windows works too.",
		URL:         "https://www.google.com/chrome/",
		Installable: false,
	}
	if p := capture.Find(); p != "" {
		chrome.Installed, chrome.Path = true, p
	}
	out = append(out, chrome)

	return out
}

// modelNote menjelaskan untuk apa tiap ukuran model, dalam kalimat pendek.
func modelNote(name string) string {
	switch name {
	case "tiny", "base":
		return "Fast, for quick tests."
	case "small":
		return "The default — the balance this project is tuned for."
	default:
		return "More accurate, much slower on a CPU."
	}
}

// toolStatus memeriksa satu biner: yang dipasang engine lebih dulu, baru PATH.
func toolStatus(l config.Layout, id, bin, detail string) Component {
	c := Component{
		ID: id, Name: bin, Kind: KindTool, Required: true, Detail: detail,
		Size: ffmpegSize(), Installable: ffmpegRecipe() != nil,
		Hint: ffmpegHint(), URL: "https://ffmpeg.org/download.html",
	}
	local := filepath.Join(l.ToolsDir, exeName(bin))
	if _, err := os.Stat(local); err == nil {
		c.Installed, c.Path = true, local
		return c
	}
	if p, err := exec.LookPath(bin); err == nil {
		c.Installed, c.Path = true, p
		c.Detail = detail + " Found on this system."
		return c
	}
	return c
}

// whisperStatus memeriksa whisper-cli.
func whisperStatus(l config.Layout) Component {
	c := Component{
		ID: "whisper", Name: "whisper.cpp", Kind: KindTool, Required: true,
		Detail:      "Turns speech into a timed transcript.",
		Size:        whisperSize(),
		Installable: whisperRecipe() != nil,
		Hint:        whisperHint(),
		URL:         "https://github.com/ggml-org/whisper.cpp/releases",
	}
	for _, cand := range []string{exeName("whisper-cli"), exeName("main")} {
		p := filepath.Join(l.ToolsDir, cand)
		if _, err := os.Stat(p); err == nil {
			c.Installed, c.Path = true, p
			return c
		}
	}
	if p, err := exec.LookPath("whisper-cli"); err == nil {
		c.Installed, c.Path = true, p
		c.Detail += " Found on this system."
	}
	return c
}

// Missing melaporkan komponen wajib yang belum ada. Dipakai untuk menjawab
// "kenapa tombol mulai tidak bisa ditekan" sebelum pengguna menekannya.
func Missing(cs []Component) []string {
	// Bukan nil: slice nil menjadi "null" di JSON, dan klien yang menghitung
	// panjangnya berhenti dengan galat alih-alih membaca "tidak ada yang kurang".
	out := []string{}
	for _, c := range cs {
		if c.Required && !c.Installed {
			out = append(out, c.Name)
		}
	}
	return out
}

// Progress dilaporkan selama pemasangan.
type Progress struct {
	Value   float64 `json:"value"` // 0..1, -1 bila besarnya tidak diketahui
	Message string  `json:"message"`
	Bytes   int64   `json:"bytes"`
	Total   int64   `json:"total"`
}

// Install memasang satu komponen. Kemajuannya dilaporkan lewat onProgress.
//
// Tidak ada langkah "coba yang lain kalau gagal": bila unduhan gagal, pesannya
// disampaikan apa adanya. Diam-diam memasang versi lain adalah persis jenis
// penggantian senyap yang dilarang di notes/12.
func Install(ctx context.Context, l config.Layout, id string, onProgress func(Progress)) error {
	if onProgress == nil {
		onProgress = func(Progress) {}
	}
	if name, ok := strings.CutPrefix(id, "model:"); ok {
		return installModel(ctx, l, name, onProgress)
	}
	switch id {
	// Satu unduhan menghasilkan keduanya — memasang "ffprobe" berarti memasang
	// paket yang sama.
	case "ffmpeg", "ffprobe":
		return installRecipe(ctx, l, ffmpegRecipe(), "ffmpeg", ffmpegHint(), onProgress)
	case "whisper":
		return installRecipe(ctx, l, whisperRecipe(), "whisper.cpp", whisperHint(), onProgress)
	case "ollama", "chrome":
		return fmt.Errorf("%s is a separate application — install it yourself, then press Refresh", id)
	}
	return fmt.Errorf("unknown component %q", id)
}

// installModel mengunduh satu berkas model dari HuggingFace.
func installModel(ctx context.Context, l config.Layout, name string, onProgress func(Progress)) error {
	known := false
	for _, m := range Models {
		if m.Name == name {
			known = true
			break
		}
	}
	if !known {
		return fmt.Errorf("unknown whisper model %q", name)
	}
	if err := os.MkdirAll(l.ModelsDir, 0o755); err != nil {
		return err
	}
	dest := ModelPath(l, name)
	// Dua cermin: HuggingFace resmi, lalu cermin komunitas. Model "small" 466 MB
	// dan "large-v3" 2,9 GB — unduhan sebesar itu di sambungan yang tidak
	// stabil butuh lebih dari satu pintu.
	urls := []string{
		"https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-" + name + ".bin",
		"https://hf-mirror.com/ggerganov/whisper.cpp/resolve/main/ggml-" + name + ".bin",
	}
	onProgress(Progress{Message: "Downloading model " + name + "…"})
	if err := downloadAny(ctx, urls, dest, onProgress); err != nil {
		return err
	}
	onProgress(Progress{Value: 1, Message: "Model " + name + " is ready."})
	return nil
}

// installRecipe menjalankan satu resep unduh+bongkar.
func installRecipe(ctx context.Context, l config.Layout, r *recipe, what, hint string, onProgress func(Progress)) error {
	if r == nil {
		return fmt.Errorf("this build cannot install %s automatically on %s — %s", what, runtime.GOOS, hint)
	}
	if err := os.MkdirAll(l.ToolsDir, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "clipper-install-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	archive := filepath.Join(tmp, "download"+r.ext())
	onProgress(Progress{Message: "Downloading " + what + "…"})
	if err := downloadAny(ctx, r.urls, archive, onProgress); err != nil {
		return err
	}
	onProgress(Progress{Value: 1, Message: "Unpacking " + what + "…"})
	n, err := r.extract(archive, l.ToolsDir)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("the %s download did not contain the expected files — the release layout may have changed", what)
	}
	onProgress(Progress{Value: 1, Message: fmt.Sprintf("%s is ready (%d files).", what, n)})
	return nil
}

// exeName menambahkan akhiran .exe di Windows.
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
