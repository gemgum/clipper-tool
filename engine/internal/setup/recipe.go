package setup

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// recipe = satu unduhan yang menghasilkan biner siap pakai.
//
// Semua isi arsip diratakan ke satu folder (ToolsDir): whisper.cpp membawa
// pustaka bersama di sebelah binernya, dan engine memang menunjuk
// LD_LIBRARY_PATH ke folder biner itu. Struktur folder di dalam arsip tidak
// membawa arti apa pun bagi kita.
type recipe struct {
	url string
	// kind: "zip" atau "tar.gz". Keduanya ada di pustaka standar Go — itu
	// sebabnya rilis .tar.xz (mis. ffmpeg statis untuk Linux) tidak dipakai:
	// menambah dependensi eksternal hanya untuk membongkarnya melanggar aturan
	// "standard library saja" proyek ini.
	kind string
	// want: nama berkas (tanpa folder) yang diambil. Kosong = ambil semua.
	// Dipakai untuk ffmpeg, yang arsipnya berisi ratusan berkas dokumentasi.
	want []string
}

func (r *recipe) ext() string {
	if r.kind == "zip" {
		return ".zip"
	}
	return ".tar.gz"
}

// wanted melaporkan apakah satu berkas di dalam arsip perlu diambil.
func (r *recipe) wanted(name string) bool {
	base := path.Base(strings.ReplaceAll(name, `\`, "/"))
	if base == "" || strings.HasPrefix(base, ".") {
		return false
	}
	if len(r.want) == 0 {
		return true
	}
	for _, w := range r.want {
		if strings.EqualFold(base, w) {
			return true
		}
	}
	return false
}

// --- resep per komponen & per OS ---
//
// Versinya dipaku, tidak mengikuti "latest": rilis baru bisa mengubah nama
// berkas di dalam arsip, dan yang gagal karenanya adalah pemasangan di mesin
// pengguna — tempat paling buruk untuk menemukan kejutan. Menaikkan versi
// adalah keputusan sadar, bukan efek samping waktu.
const whisperVersion = "v1.9.1"

func whisperRecipe() *recipe {
	base := "https://github.com/ggml-org/whisper.cpp/releases/download/" + whisperVersion + "/"
	switch runtime.GOOS {
	case "windows":
		// Berisi whisper-cli.exe + DLL pendampingnya.
		return &recipe{url: base + "whisper-bin-x64.zip", kind: "zip"}
	case "linux":
		if runtime.GOARCH == "arm64" {
			return &recipe{url: base + "whisper-bin-ubuntu-arm64.tar.gz", kind: "tar.gz"}
		}
		return &recipe{url: base + "whisper-bin-ubuntu-x64.tar.gz", kind: "tar.gz"}
	}
	// macOS: rilis resminya hanya xcframework, bukan biner baris perintah.
	return nil
}

func whisperSize() string {
	switch runtime.GOOS {
	case "windows":
		return "~8 MB"
	case "linux":
		return "~9 MB"
	}
	return ""
}

func whisperHint() string {
	if runtime.GOOS == "darwin" {
		return "On macOS, install it with: brew install whisper-cpp"
	}
	return "Or build it yourself with ./setup.sh"
}

func ffmpegRecipe() *recipe {
	switch runtime.GOOS {
	case "windows":
		return &recipe{
			url:  "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip",
			kind: "zip",
			want: []string{"ffmpeg.exe", "ffprobe.exe"},
		}
	case "darwin":
		// evermeet.cx mengemas satu biner per zip, jadi ffprobe ikut terpasang
		// lewat resep kedua di bawah.
		return &recipe{
			url:  "https://evermeet.cx/ffmpeg/getrelease/ffmpeg/zip",
			kind: "zip",
			want: []string{"ffmpeg"},
		}
	}
	// Linux: rilis statis resminya .tar.xz, dan xz tidak ada di pustaka standar.
	return nil
}

func ffmpegSize() string {
	switch runtime.GOOS {
	case "windows":
		return "~110 MB"
	case "darwin":
		return "~26 MB"
	}
	return ""
}

func ffmpegHint() string {
	switch runtime.GOOS {
	case "linux":
		return "Install it with your package manager, e.g. sudo apt install ffmpeg"
	case "darwin":
		return "Or install it with: brew install ffmpeg"
	}
	return "Download the release build and put ffmpeg.exe next to the app"
}

// --- unduh ---

// download menyimpan url ke dest sambil melaporkan kemajuannya.
//
// Ditulis ke berkas .part lalu diganti nama di akhir: unduhan model bisa
// memakan menit, dan berkas setengah jadi yang bernama seperti berkas jadi akan
// dianggap "sudah terpasang" pada percobaan berikutnya.
func download(ctx context.Context, url, dest string, onProgress func(Progress)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 6 * time.Hour} // model besar di sambungan lambat
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s said %s", url, res.Status)
	}

	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	total := res.ContentLength
	pr := &progressReader{r: res.Body, total: total, onProgress: onProgress}
	_, err = io.Copy(f, pr)
	closeErr := f.Close()
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("download interrupted: %w", err)
	}
	if closeErr != nil {
		os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, dest)
}

// progressReader melaporkan kemajuan salinan, dibatasi sekali per 200 ms supaya
// aliran SSE tidak dibanjiri ribuan pesan yang isinya nyaris sama.
type progressReader struct {
	r          io.Reader
	total      int64
	read       int64
	last       time.Time
	onProgress func(Progress)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if time.Since(p.last) > 200*time.Millisecond || err == io.EOF {
		p.last = time.Now()
		value := -1.0
		if p.total > 0 {
			value = float64(p.read) / float64(p.total)
		}
		p.onProgress(Progress{
			Value:   value,
			Message: fmt.Sprintf("%s of %s", human(p.read), human(p.total)),
			Bytes:   p.read,
			Total:   p.total,
		})
	}
	return n, err
}

func human(b int64) string {
	switch {
	case b <= 0:
		return "?"
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(b)/(1<<20))
	default:
		return fmt.Sprintf("%.0f KB", float64(b)/(1<<10))
	}
}

// --- bongkar ---

// extract membongkar arsip ke dir dan mengembalikan jumlah berkas yang diambil.
func (r *recipe) extract(archivePath, dir string) (int, error) {
	if r.kind == "zip" {
		return r.extractZip(archivePath, dir)
	}
	return r.extractTarGz(archivePath, dir)
}

func (r *recipe) extractZip(archivePath, dir string) (int, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0, err
	}
	defer zr.Close()
	n := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !r.wanted(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return n, err
		}
		err = writeFile(filepath.Join(dir, path.Base(f.Name)), rc, f.FileInfo().Mode())
		rc.Close()
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (r *recipe) extractTarGz(archivePath, dir string) (int, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	n := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return n, err
		}
		if !r.wanted(h.Name) {
			continue
		}
		target := filepath.Join(dir, path.Base(h.Name))
		switch h.Typeflag {
		case tar.TypeReg:
			if err := writeFile(target, tr, os.FileMode(h.Mode)); err != nil {
				return n, err
			}
			n++
		case tar.TypeSymlink:
			// Pustaka whisper datang sebagai rantai symlink
			// (libwhisper.so → libwhisper.so.1 → libwhisper.so.1.9.1). Karena
			// semuanya diratakan ke satu folder, tujuannya cukup nama berkasnya.
			os.Remove(target)
			if err := os.Symlink(path.Base(h.Linkname), target); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

// writeFile menulis satu berkas hasil bongkaran, menimpa yang lama.
//
// Berkas lama dihapus lebih dulu, tidak sekadar ditimpa: menimpa biner yang
// sedang berjalan gagal di Windows, dan menghapus-lalu-membuat memberi pesan
// yang lebih jelas ketimbang "access denied" di tengah penyalinan.
func writeFile(dest string, src io.Reader, mode os.FileMode) error {
	os.Remove(dest)
	if mode == 0 {
		mode = 0o644
	}
	// Biner harus bisa dieksekusi. Rilis Windows tidak membawa bit itu, dan
	// tanpa ini whisper-cli hasil unduhan tidak bisa dijalankan sama sekali.
	if isExecutableName(dest) {
		mode |= 0o755
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, src); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// isExecutableName menebak apakah sebuah berkas hasil bongkaran adalah program
// atau pustaka — keduanya perlu bit x di Unix.
func isExecutableName(p string) bool {
	base := strings.ToLower(filepath.Base(p))
	if strings.HasSuffix(base, ".exe") || strings.HasSuffix(base, ".dll") {
		return true
	}
	if strings.Contains(base, ".so") || strings.HasSuffix(base, ".dylib") {
		return true
	}
	return !strings.Contains(base, ".") // biner Unix umumnya tanpa akhiran
}
