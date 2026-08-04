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
	// urls dicoba berurutan. Cermin pertama yang menjawab dipakai.
	//
	// Bukan kemewahan: gyan.dev — sumber ffmpeg Windows yang lazim — adalah
	// satu server kecil yang dari sebagian jaringan tidak terjangkau sama
	// sekali (terbukti: TLS handshake timeout dari Indonesia). Satu alamat mati
	// berarti komponen wajib tidak bisa dipasang, dan pengguna tidak punya
	// jalan lain di dalam aplikasi.
	urls []string
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

// Versi ffmpeg yang dipaku di cermin GitHub. gyan.dev sendiri hanya menyediakan
// "release terbaru" tanpa versi di alamatnya, jadi cermin inilah yang membuat
// hasil unduhan bisa diulang persis.
const ffmpegVersion = "9.0"

func whisperRecipe() *recipe {
	base := "https://github.com/ggml-org/whisper.cpp/releases/download/" + whisperVersion + "/"
	switch runtime.GOOS {
	case "windows":
		// Berisi whisper-cli.exe + DLL pendampingnya.
		return &recipe{urls: []string{base + "whisper-bin-x64.zip"}, kind: "zip"}
	case "linux":
		if runtime.GOARCH == "arm64" {
			return &recipe{urls: []string{base + "whisper-bin-ubuntu-arm64.tar.gz"}, kind: "tar.gz"}
		}
		return &recipe{urls: []string{base + "whisper-bin-ubuntu-x64.tar.gz"}, kind: "tar.gz"}
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
			urls: []string{
				// Cermin GitHub dari build yang sama — CDN GitHub terjangkau
				// dari mana pun aplikasi ini bisa diunduh, jadi ia didahulukan.
				"https://github.com/GyanD/codexffmpeg/releases/download/" + ffmpegVersion +
					"/ffmpeg-" + ffmpegVersion + "-essentials_build.zip",
				// Sumber aslinya, sebagai cadangan.
				"https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip",
				// Build pihak lain, kalau dua di atas mati.
				"https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip",
			},
			kind: "zip",
			want: []string{"ffmpeg.exe", "ffprobe.exe"},
		}
	}
	// macOS: sumber yang lazim (evermeet.cx) mengemas SATU biner per zip,
	// sedangkan daftar urls di sini berarti "cermin" — dicoba sampai satu
	// berhasil, lalu berhenti. Memakainya untuk dua berkas berbeda akan
	// memasang ffmpeg saja dan meninggalkan ffprobe hilang tanpa pesan apa pun.
	// Lebih jujur menyerahkannya ke Homebrew, yang memasang keduanya sekaligus.
	//
	// Linux: rilis statis resminya .tar.xz, dan xz tidak ada di pustaka standar.
	return nil
}

func ffmpegSize() string {
	switch runtime.GOOS {
	case "windows":
		return "~110 MB"
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

// downloadAny mencoba tiap cermin sampai ada yang berhasil.
//
// Galat terakhir yang dilaporkan, bukan yang pertama: yang pertama biasanya
// cermin yang memang sudah lama mati, sedangkan yang terakhir lebih mungkin
// menggambarkan keadaan jaringan pengguna.
func downloadAny(ctx context.Context, urls []string, dest string, onProgress func(Progress)) error {
	onProgress = orNoop(onProgress)
	var last error
	for i, url := range urls {
		if i > 0 {
			onProgress(Progress{Message: fmt.Sprintf("Trying another source (%d of %d)…", i+1, len(urls))})
		}
		err := download(ctx, url, dest, onProgress)
		if err == nil {
			return nil
		}
		// Pembatalan oleh pengguna bukan kegagalan cermin — jangan pindah.
		if ctx.Err() != nil {
			return err
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("no download source is configured")
	}
	return last
}

// orNoop membuat pelapor progres selalu aman dipanggil. Penjaganya ada di sini,
// bukan hanya di Install: fungsi unduh dipanggil juga dari tempat lain (dan
// dari uji), dan "tanpa pelapor" adalah pemakaian yang sah — bukan alasan untuk
// panik di tengah unduhan 111 MB.
func orNoop(fn func(Progress)) func(Progress) {
	if fn == nil {
		return func(Progress) {}
	}
	return fn
}

// download menyimpan url ke dest sambil melaporkan kemajuannya.
//
// Ditulis ke berkas .part lalu diganti nama di akhir: berkas setengah jadi yang
// bernama seperti berkas jadi akan dianggap "sudah terpasang" pada percobaan
// berikutnya.
//
// Berkas .part TIDAK dihapus saat gagal — justru itu yang membuat percobaan
// berikutnya bisa meminta sisanya saja lewat header Range. Di sambungan yang
// putus-nyambung, gagal di 100 MB tidak lagi berarti mengulang 111 MB.
func download(ctx context.Context, url, dest string, onProgress func(Progress)) error {
	onProgress = orNoop(onProgress)
	tmp := dest + ".part"

	// Berapa byte yang sudah ada dari percobaan sebelumnya.
	var have int64
	if st, err := os.Stat(tmp); err == nil {
		have = st.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
	}
	client := &http.Client{Timeout: 6 * time.Hour} // model besar di sambungan lambat
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer res.Body.Close()

	// 206 = server menerima permintaan lanjutan. 200 = ia mengabaikannya dan
	// mengirim dari awal, jadi berkas lama harus dibuang supaya tidak tersambung
	// jadi berkas rusak yang ukurannya kelihatan benar.
	resume := res.StatusCode == http.StatusPartialContent
	switch {
	case resume:
	case res.StatusCode == http.StatusOK:
		have = 0
	default:
		return fmt.Errorf("download failed: %s said %s", url, res.Status)
	}

	flag := os.O_CREATE | os.O_WRONLY
	if resume {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	f, err := os.OpenFile(tmp, flag, 0o644)
	if err != nil {
		return err
	}

	total := res.ContentLength
	if total > 0 {
		total += have // ContentLength hanya menghitung sisanya saat melanjutkan
	}
	pr := &progressReader{r: res.Body, total: total, read: have, onProgress: onProgress}
	_, err = io.Copy(f, pr)
	closeErr := f.Close()
	if err != nil {
		return fmt.Errorf("download interrupted: %w", err)
	}
	if closeErr != nil {
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
