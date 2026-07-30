// Package capture menangkap foto layar halaman web memakai Chrome headless.
//
// Mengikuti pola paket ffmpeg/transcribe: engine tidak memasang pustaka browser,
// melainkan memanggil biner yang sudah ada di sistem. Chrome/Edge praktis selalu
// tersedia (Edge bawaan Windows), jadi tidak menambah beban pemasangan.
package capture

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client menjalankan Chrome headless.
type Client struct {
	Bin string // path ke chrome/chromium/msedge
}

func New(bin string) *Client { return &Client{Bin: bin} }

// kandidat biner yang dicari bila CLIPPER_CHROME tidak diset. Urutannya:
// Chromium/Chrome Linux dulu (paling cocok dengan engine), baru .exe Windows
// lewat WSL sebagai cadangan.
var kandidatLinux = []string{
	"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
	"microsoft-edge", "microsoft-edge-stable", "brave-browser",
}

var kandidatWindows = []string{
	`/mnt/c/Program Files/Google/Chrome/Application/chrome.exe`,
	`/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe`,
	`/mnt/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe`,
	`/mnt/c/Program Files/Microsoft/Edge/Application/msedge.exe`,
}

// Cari menemukan biner browser yang bisa dipakai. Mengembalikan "" bila tidak
// ada — pemanggil yang memutuskan cara melaporkannya.
func Cari() string {
	if v := os.Getenv("CLIPPER_CHROME"); v != "" {
		return v
	}
	for _, nama := range kandidatLinux {
		if p, err := exec.LookPath(nama); err == nil {
			return p
		}
	}
	for _, p := range kandidatWindows {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// windows melaporkan apakah biner ini program Windows yang dijalankan lewat
// interop WSL. Program Windows tidak paham path Linux, jadi berkas masukan &
// keluaran harus lewat folder yang terlihat oleh kedua sisi.
func (c *Client) windows() bool {
	return strings.HasSuffix(strings.ToLower(c.Bin), ".exe")
}

// Opsi satu penangkapan.
type Opsi struct {
	Lebar  int     // lebar viewport (px CSS)
	Tinggi int     // tinggi viewport (px CSS)
	Skala  float64 // faktor skala perangkat; 2 = hasil 2x lebih tajam
	Tunggu int     // anggaran waktu virtual (ms) agar gambar & JS sempat termuat
}

func (o *Opsi) lengkapi() {
	if o.Lebar <= 0 {
		o.Lebar = 1080
	}
	if o.Tinggi <= 0 {
		o.Tinggi = 1920
	}
	if o.Skala <= 0 {
		o.Skala = 1
	}
	if o.Tunggu <= 0 {
		o.Tunggu = 12000
	}
}

// Tangkap merender url (boleh http/https atau file://) menjadi PNG di outPNG.
//
// Bila Chrome yang dipakai adalah program Windows, penangkapan dilakukan di
// folder temp Windows lalu hasilnya disalin ke outPNG — sebab chrome.exe tidak
// bisa menulis ke path Linux.
func (c *Client) Tangkap(ctx context.Context, url, outPNG string, o Opsi) error {
	if c.Bin == "" {
		return fmt.Errorf("browser tidak ditemukan — pasang Chrome/Chromium, atau set CLIPPER_CHROME ke path chrome.exe")
	}
	o.lengkapi()
	if err := os.MkdirAll(filepath.Dir(outPNG), 0o755); err != nil {
		return err
	}

	sasaran := outPNG // path yang ditulis Chrome
	var bersihkan func()
	if c.windows() {
		winOut, lxOut, err := tempWindows()
		if err != nil {
			return err
		}
		sasaran = winOut
		bersihkan = func() { os.Remove(lxOut) }
		defer func() {
			// Salin hasil dari temp Windows ke tujuan sebenarnya.
			if data, err := os.ReadFile(lxOut); err == nil {
				_ = os.WriteFile(outPNG, data, 0o644)
			}
			bersihkan()
		}()
	}

	// Profil sementara: mencegah bentrok dengan Chrome yang sedang dipakai
	// pengguna (Chrome menolak dua proses berbagi satu user-data-dir).
	//
	// Untuk chrome.exe profilnya WAJIB berada di disk Windows. Folder /tmp Linux
	// memang terlihat dari Windows lewat \\wsl.localhost, tapi jalur itu tidak
	// mendukung penguncian berkas — Chrome langsung mati dengan LockFileEx gagal.
	profilArg, hapusProfil, err := c.dirProfil()
	if err != nil {
		return err
	}
	defer hapusProfil()

	args := []string{
		"--headless=new",
		"--disable-gpu",
		"--hide-scrollbars",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--mute-audio",
		fmt.Sprintf("--window-size=%d,%d", o.Lebar, o.Tinggi),
		"--force-device-scale-factor=" + strconv.FormatFloat(o.Skala, 'f', -1, 64),
		fmt.Sprintf("--virtual-time-budget=%d", o.Tunggu),
		"--screenshot=" + sasaran,
	}
	if profilArg != "" {
		args = append(args, "--user-data-dir="+profilArg)
	}
	args = append(args, url)

	// Batas waktu keras: Chrome sesekali menggantung pada halaman bermasalah.
	ctx, batal := context.WithTimeout(ctx, 90*time.Second)
	defer batal()

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("browser tidak merespons dalam 90 detik saat membuka %s", url)
		}
		return fmt.Errorf("browser gagal: %v — %s", err, ringkas(stderr.String()))
	}

	// Chrome bisa keluar dengan status 0 tanpa menulis berkas (mis. URL ditolak).
	cek := sasaran
	if c.windows() {
		if lx, err := keLinux(sasaran); err == nil {
			cek = lx
		}
	}
	if fi, err := os.Stat(cek); err != nil || fi.Size() == 0 {
		return fmt.Errorf("browser tidak menghasilkan gambar untuk %s — %s", url, ringkas(stderr.String()))
	}
	return nil
}

// dirProfil membuat folder profil sementara di sisi yang benar: disk Windows
// bila browsernya chrome.exe, /tmp biasa bila browsernya asli Linux.
func (c *Client) dirProfil() (arg string, hapus func(), err error) {
	if !c.windows() {
		d, err := os.MkdirTemp("", "clipper-chrome-")
		if err != nil {
			return "", nil, err
		}
		return d, func() { os.RemoveAll(d) }, nil
	}
	base, err := tempWindowsDir()
	if err != nil {
		return "", nil, err
	}
	win := base + `\` + fmt.Sprintf("clipper-profil-%d", time.Now().UnixNano())
	lx, err := keLinux(win)
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(lx, 0o755); err != nil {
		return "", nil, err
	}
	return win, func() { os.RemoveAll(lx) }, nil
}

// tempWindowsDir membaca %TEMP% Windows. Hasilnya di-cache: memanggil cmd.exe
// memakan ratusan milidetik, sedangkan nilainya tidak berubah selama proses.
var (
	sekaliTemp sync.Once
	tempWin    string
	tempErr    error
)

func tempWindowsDir() (string, error) {
	sekaliTemp.Do(func() {
		out, err := exec.Command("cmd.exe", "/c", "echo %TEMP%").Output()
		if err != nil {
			tempErr = fmt.Errorf("tidak bisa membaca folder temp Windows (interop WSL mati?): %v", err)
			return
		}
		dir := strings.TrimSpace(strings.ReplaceAll(string(out), "\r", ""))
		if dir == "" || strings.Contains(dir, "%TEMP%") {
			tempErr = fmt.Errorf("folder temp Windows tidak terbaca")
			return
		}
		tempWin = dir
	})
	return tempWin, tempErr
}

// DumpDOM membuka url, menunggu skripnya selesai, lalu mengembalikan DOM akhir.
//
// Dipakai untuk halaman yang pindah alamat lewat JavaScript — Google News,
// misalnya, yang tautannya tidak bisa diikuti dengan redirect HTTP biasa dan
// kodenya tidak lagi memuat URL aslinya. Setelah skripnya jalan, yang terbaca
// di sini sudah berupa halaman artikel yang sebenarnya, lengkap dengan tag
// og: dan badan tulisannya — jadi satu panggilan cukup.
func (c *Client) DumpDOM(ctx context.Context, url string, tungguMS int) (string, error) {
	if c.Bin == "" {
		return "", fmt.Errorf("browser tidak ditemukan — pasang Chrome/Chromium, atau set CLIPPER_CHROME ke path chrome.exe")
	}
	if tungguMS <= 0 {
		tungguMS = 15000
	}
	// Dicoba dua kali. Halaman yang pindah alamat lewat JavaScript kadang belum
	// selesai saat anggaran waktu virtual habis — terutama pada peluncuran
	// Chrome pertama yang masih dingin. Gejalanya khas: keluar sukses tapi DOM
	// nyaris kosong. Percobaan kedua dengan anggaran dua kali lipat hampir
	// selalu berhasil, jadi lebih baik menunggu sebentar daripada melempar
	// galat yang sebenarnya cuma soal waktu.
	dom, err := c.dumpSekali(ctx, url, tungguMS)
	if err == nil {
		return dom, nil
	}
	return c.dumpSekali(ctx, url, tungguMS*2)
}

func (c *Client) dumpSekali(ctx context.Context, url string, tungguMS int) (string, error) {
	profil, hapus, err := c.dirProfil()
	if err != nil {
		return "", err
	}
	defer hapus()

	args := []string{
		"--headless=new",
		"--disable-gpu",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--mute-audio",
		"--user-data-dir=" + profil,
		fmt.Sprintf("--virtual-time-budget=%d", tungguMS),
		"--dump-dom",
		url,
	}

	ctx, batal := context.WithTimeout(ctx, 90*time.Second)
	defer batal()

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("browser tidak merespons dalam 90 detik saat membuka %s", url)
		}
		return "", fmt.Errorf("browser gagal membuka halaman: %v — %s", err, ringkas(stderr.String()))
	}
	dom := out.String()
	if len(dom) < 200 {
		return "", fmt.Errorf("halaman %s tidak menghasilkan isi — %s", url, ringkas(stderr.String()))
	}
	return dom, nil
}

// tempWindows menyiapkan nama berkas di folder temp Windows dan mengembalikan
// pasangan (path Windows, path Linux) untuk berkas yang sama.
func tempWindows() (win, lx string, err error) {
	dir, err := tempWindowsDir()
	if err != nil {
		return "", "", err
	}
	win = dir + `\` + fmt.Sprintf("clipper-%d.png", time.Now().UnixNano())
	lx, err = keLinux(win)
	if err != nil {
		return "", "", err
	}
	return win, lx, nil
}

func keLinux(win string) (string, error) {
	out, err := exec.Command("wslpath", "-u", win).Output()
	if err != nil {
		return "", fmt.Errorf("wslpath gagal menerjemahkan %q: %v", win, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func keWindows(lx string) (string, error) {
	out, err := exec.Command("wslpath", "-w", lx).Output()
	if err != nil {
		return "", fmt.Errorf("wslpath gagal menerjemahkan %q: %v", lx, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// URLBerkas mengubah path berkas lokal jadi URL file:// yang bisa dibuka Chrome.
// Untuk chrome.exe path-nya diterjemahkan dulu ke bentuk Windows.
func (c *Client) URLBerkas(path string) (string, error) {
	if c.windows() {
		w, err := keWindows(path)
		if err != nil {
			return "", err
		}
		return "file:///" + strings.ReplaceAll(w, `\`, "/"), nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return "file://" + abs, nil
}

// SiapkanBerkas menulis isi ke lokasi yang bisa dibaca browser, lalu
// mengembalikan URL-nya beserta fungsi pembersih.
//
// Untuk chrome.exe berkas ditaruh di temp Windows: berkas di /tmp Linux memang
// bisa dijangkau lewat \\wsl.localhost, tapi jalur itu lambat dan kadang
// diblokir kebijakan keamanan Windows.
func (c *Client) SiapkanBerkas(isi []byte, ext string) (url string, bersih func(), err error) {
	if c.windows() {
		dir, err := tempWindowsDir()
		if err != nil {
			return "", nil, err
		}
		win := dir + `\` + fmt.Sprintf("clipper-%d%s", time.Now().UnixNano(), ext)
		lx, err := keLinux(win)
		if err != nil {
			return "", nil, err
		}
		if err := os.WriteFile(lx, isi, 0o644); err != nil {
			return "", nil, err
		}
		return "file:///" + strings.ReplaceAll(win, `\`, "/"), func() { os.Remove(lx) }, nil
	}
	f, err := os.CreateTemp("", "clipper-*"+ext)
	if err != nil {
		return "", nil, err
	}
	if _, err := f.Write(isi); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	f.Close()
	return "file://" + f.Name(), func() { os.Remove(f.Name()) }, nil
}

// ringkas mengambil baris stderr yang berguna saja. Chrome membanjiri stderr
// dengan peringatan GPU/USB/ekstensi yang tidak ada kaitannya dengan kegagalan.
func ringkas(s string) string {
	var pakai []string
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		l := strings.ToLower(ln)
		if strings.Contains(l, "usb") || strings.Contains(l, "gpu") ||
			strings.Contains(l, "extension") || strings.Contains(l, "gcm") ||
			strings.Contains(l, "registration") || strings.Contains(l, "dbus") ||
			strings.Contains(l, "bluetooth") || strings.Contains(l, "voice") {
			continue
		}
		pakai = append(pakai, ln)
		if len(pakai) >= 3 {
			break
		}
	}
	if len(pakai) == 0 {
		return "(tidak ada pesan galat yang jelas)"
	}
	return strings.Join(pakai, "; ")
}
