package api

import (
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Menjelajah berkas lewat engine, bukan lewat unggahan.
//
// Kotak unggah menyalin video ke data/uploads dulu — 3,84 GB digandakan sebelum
// pekerjaan dimulai. Di aplikasi desktop itu tidak masuk akal: berkasnya sudah
// ada di mesin yang sama, engine tinggal membacanya di tempat. Dua endpoint di
// berkas ini yang membuat penyalinan itu bisa dihindari:
//
//	/api/browse  daftar isi folder, supaya GUI punya pemilih berkas sendiri
//	/api/locate  mencocokkan berkas yang di-drop (nama + ukuran) ke path aslinya
//
// Keduanya hanya boleh dijangkau dari mesin ini sendiri — engine mendengar di
// 127.0.0.1, dan token sesi menjaga sisanya.

// videoExts = akhiran yang dianggap video. Dipakai untuk menyorot berkas yang
// relevan di pemilih; berkas lain tetap ditampilkan supaya pengguna tidak
// bingung mencari sesuatu yang dilihatnya ada di file manager.
var videoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".mov": true, ".avi": true, ".webm": true,
	".m4v": true, ".ts": true, ".flv": true, ".wmv": true, ".mpg": true,
	".mpeg": true, ".m2ts": true, ".mts": true, ".3gp": true, ".ogv": true,
}

// isVideo melaporkan apakah nama berkas berakhiran video yang dikenal.
func isVideo(name string) bool {
	return videoExts[strings.ToLower(filepath.Ext(name))]
}

// entry satu baris di pemilih berkas.
type entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Dir   bool   `json:"dir"`
	Video bool   `json:"video"`
	Size  int64  `json:"size"`
}

// place = pintasan folder yang ditawarkan di sisi pemilih.
type place struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// maxEntries membatasi isi satu folder yang dikirim. Folder berisi puluhan ribu
// berkas ada di mesin siapa pun (cache, dataset); tanpa batas, satu klik salah
// membekukan GUI sampai daftar itu selesai dirender.
const maxEntries = 2000

// browse melaporkan isi satu folder.
func (s *Server) browse(w http.ResponseWriter, r *http.Request) {
	dir := strings.TrimSpace(r.URL.Query().Get("dir"))
	if dir == "" {
		dir = homeDir()
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		writeErr(w, 400, "invalid folder: "+err.Error())
		return
	}
	items, err := os.ReadDir(abs)
	if err != nil {
		writeErr(w, 400, "cannot open the folder: "+err.Error())
		return
	}

	entries := make([]entry, 0, len(items))
	for _, it := range items {
		name := it.Name()
		// Berkas tersembunyi disaring: pengguna tidak menyimpan videonya di
		// ~/.cache, dan daftar jadi jauh lebih pendek tanpa itu.
		if strings.HasPrefix(name, ".") {
			continue
		}
		e := entry{Name: name, Path: filepath.Join(abs, name), Dir: it.IsDir()}
		if !e.Dir {
			e.Video = isVideo(name)
			if info, err := it.Info(); err == nil {
				e.Size = info.Size()
			}
		}
		entries = append(entries, e)
		if len(entries) >= maxEntries {
			break
		}
	}
	// Folder dulu, lalu berkas; masing-masing menurut abjad tanpa membedakan
	// besar-kecil huruf — urutan yang sama dengan file manager mana pun.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	parent := filepath.Dir(abs)
	if parent == abs { // sudah di akar
		parent = ""
	}
	writeJSON(w, 200, map[string]any{
		"dir":       abs,
		"parent":    parent,
		"entries":   entries,
		"places":    places(),
		"truncated": len(entries) >= maxEntries,
	})
}

// locate mencari path asli sebuah berkas dari nama & ukurannya.
//
// Browser tidak memberi tahu halaman web di mana berkas yang di-drop berada —
// hanya nama, ukuran, dan waktu ubahnya. Padahal berkas itu ada di mesin yang
// sama dengan engine. Jadi engine mencarinya sendiri di tempat-tempat yang
// wajar; kalau ketemu, penyalinan 3,84 GB tidak perlu terjadi sama sekali.
//
// Nama DAN ukuran harus sama persis. Kalau ada lebih dari satu yang cocok,
// jawabannya kosong: menebak berkas mana yang dimaksud lebih buruk daripada
// menyuruh pengguna memilih sendiri.
func (s *Server) locate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	req.Name = filepath.Base(strings.TrimSpace(req.Name))
	if req.Name == "" || req.Name == "." || req.Name == string(filepath.Separator) {
		writeErr(w, 400, "the 'name' field is required")
		return
	}
	if req.Size <= 0 {
		writeErr(w, 400, "the 'size' field is required")
		return
	}

	found := findFile(searchRoots(), req.Name, req.Size, time.Now().Add(locateBudget))
	if len(found) != 1 {
		// 404 juga untuk hasil ganda: GUI memperlakukan keduanya sama —
		// kembali ke unggahan biasa.
		writeJSON(w, 404, map[string]any{
			"error":   "the file was not found on this computer",
			"matches": len(found),
		})
		return
	}
	writeJSON(w, 200, map[string]any{"path": found[0]})
}

// locateBudget membatasi lama pencarian. Ini berjalan sementara pengguna
// menunggu setelah men-drop berkas, jadi ia harus selesai dalam hitungan
// kedipan mata; gagal menemukan bukan bencana, sebab unggahan tetap ada.
const locateBudget = 2500 * time.Millisecond

// locateDepth = kedalaman folder yang ditelusuri dari tiap titik awal.
// Cukup untuk "Videos/2026/Agustus/rekaman.mp4", tidak sampai menyisir seluruh
// disk.
const locateDepth = 4

// findFile menelusuri beberapa folder awal sampai anggaran waktu habis.
func findFile(roots []string, name string, size int64, deadline time.Time) []string {
	var out []string
	seen := map[string]bool{}
	for _, root := range roots {
		if root == "" || seen[root] {
			continue
		}
		seen[root] = true
		walkFor(root, name, size, 0, deadline, &out)
		// Dua hasil sudah cukup untuk tahu jawabannya ambigu.
		if len(out) > 1 || time.Now().After(deadline) {
			break
		}
	}
	return out
}

// skipDirs = folder yang tidak pernah berisi video pengguna tapi isinya puluhan
// ribu berkas. Menyisirnya hanya menghabiskan anggaran waktu.
var skipDirs = map[string]bool{
	"node_modules": true, "AppData": true, "Library": true, "Windows": true,
	"Program Files": true, "Program Files (x86)": true, "$Recycle.Bin": true,
	"snap": true, "go": true, "venv": true, "__pycache__": true,
}

func walkFor(dir, name string, size int64, depth int, deadline time.Time, out *[]string) {
	if depth > locateDepth || time.Now().After(deadline) || len(*out) > 1 {
		return
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var subdirs []string
	for _, it := range items {
		if it.IsDir() {
			n := it.Name()
			if strings.HasPrefix(n, ".") || skipDirs[n] {
				continue
			}
			subdirs = append(subdirs, filepath.Join(dir, n))
			continue
		}
		if it.Name() != name {
			continue
		}
		info, err := it.Info()
		if err != nil || info.Size() != size {
			continue
		}
		*out = append(*out, filepath.Join(dir, name))
		if len(*out) > 1 {
			return
		}
	}
	// Berkas di folder ini diperiksa lebih dulu, baru turun — berkas yang dicari
	// hampir selalu ada di folder dangkal (Downloads, Videos).
	for _, sd := range subdirs {
		walkFor(sd, name, size, depth+1, deadline, out)
		if len(*out) > 1 || time.Now().After(deadline) {
			return
		}
	}
}

// homeDir mengembalikan folder rumah pengguna, atau folder kerja bila tak ada.
func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return string(filepath.Separator)
}

// userFolders = nama folder video yang lazim, dalam bahasa Inggris. Sengaja
// tidak diterjemahkan: yang dibaca adalah nama folder di disk, dan Windows
// maupun Linux menamainya dalam bahasa Inggris walau tampilannya lain.
var userFolders = []string{"Videos", "Movies", "Downloads", "Desktop", "Documents"}

// places mengembalikan pintasan yang ditampilkan di sisi pemilih berkas:
// rumah, folder video yang lazim, dan (di WSL) folder rumah Windows.
func places() []place {
	home := homeDir()
	out := []place{{Name: "Home", Path: home}}
	for _, n := range userFolders {
		p := filepath.Join(home, n)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			out = append(out, place{Name: n, Path: p})
		}
	}
	for _, p := range windowsHomes() {
		base := filepath.Base(p)
		out = append(out, place{Name: "Windows: " + base, Path: p})
		for _, n := range userFolders {
			sub := filepath.Join(p, n)
			if st, err := os.Stat(sub); err == nil && st.IsDir() {
				out = append(out, place{Name: base + " / " + n, Path: sub})
			}
		}
	}
	out = append(out, drives()...)
	return out
}

// drives menambahkan huruf drive di Windows.
//
// Tanpa ini, pengguna Windows yang menyimpan videonya di D: harus mengetik
// path-nya sendiri — folder rumah saja tidak cukup di sistem yang memang
// membagi disknya per huruf.
func drives() []place {
	if runtime.GOOS != "windows" {
		return nil
	}
	var out []place
	for c := 'C'; c <= 'Z'; c++ {
		root := string(c) + `:\`
		if _, err := os.Stat(root); err == nil {
			out = append(out, place{Name: string(c) + ":", Path: root})
		}
	}
	return out
}

// searchRoots = titik awal pencarian /api/locate, dari yang paling mungkin.
func searchRoots() []string {
	var out []string
	home := homeDir()
	for _, n := range userFolders {
		out = append(out, filepath.Join(home, n))
	}
	for _, p := range windowsHomes() {
		for _, n := range userFolders {
			out = append(out, filepath.Join(p, n))
		}
	}
	// Rumah paling akhir: ia mencakup semua di atas, jadi baru dipakai bila
	// yang spesifik tidak membuahkan hasil.
	out = append(out, home)
	return out
}

// windowsHomes mencari folder rumah Windows saat engine jalan di dalam WSL.
//
// Di WSL, video pengguna hampir selalu ada di sisi Windows (/mnt/c/Users/...),
// bukan di folder rumah Linux — tanpa ini pemilih berkas terbuka di tempat yang
// pasti kosong.
func windowsHomes() []string {
	if runtime.GOOS != "linux" {
		return nil
	}
	var out []string
	for _, drive := range []string{"c", "d"} {
		users := filepath.Join("/mnt", drive, "Users")
		items, err := os.ReadDir(users)
		if err != nil {
			continue
		}
		for _, it := range items {
			n := it.Name()
			if !it.IsDir() || n == "Public" || n == "Default" || n == "All Users" ||
				strings.HasPrefix(n, ".") || strings.HasPrefix(n, "Default") {
				continue
			}
			// Hanya folder rumah pengguna yang login — daftar lengkap isi
			// C:\Users membingungkan dan sebagian tidak bisa dibaca.
			if u, err := user.Current(); err == nil && !strings.EqualFold(u.Username, n) {
				if _, err := os.Stat(filepath.Join(users, n, "Desktop")); err != nil {
					continue
				}
			}
			out = append(out, filepath.Join(users, n))
		}
	}
	return out
}
