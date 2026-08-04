package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Kunci sesi & asal yang diizinkan.
//
// Engine mendengar di 127.0.0.1, tapi di mesin pengguna itu tidak cukup: SETIAP
// program di komputer itu — dan setiap halaman web yang sedang dibuka —  bisa
// menghubunginya. Sejak ada /api/browse dan /api/locate, yang bisa dilakukannya
// bukan cuma memerintah engine, tapi juga menelusuri berkas pengguna.
//
// Karena itu dua lapis:
//
//	token   setiap permintaan harus membawa kunci yang dibuat baru tiap engine
//	        dijalankan, dan hanya ditulis ke berkas yang bisa dibaca pemiliknya;
//	origin  halaman web dari internet ditolak di tingkat CORS, jadi browser
//	        tidak pernah mengirimkan permintaannya sejak awal.

// NewToken membuat kunci sesi acak.
func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// rand.Read tidak pernah gagal di OS yang didukung Go; kalau toh gagal,
		// berhenti lebih baik daripada jalan dengan kunci yang bisa ditebak.
		panic("cannot generate a session token: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Handshake adalah isi berkas yang memberi tahu shell desktop cara menghubungi
// engine: port yang dipilih & kunci sesinya.
//
// Berkas, bukan argumen baris perintah: port baru diketahui SETELAH engine
// berhasil mendengar, dan kunci di baris perintah terlihat oleh setiap program
// lain lewat daftar proses.
type Handshake struct {
	URL     string `json:"url"`
	Port    int    `json:"port"`
	Token   string `json:"token"`
	PID     int    `json:"pid"`
	Version string `json:"version"`
}

// HandshakePath mengembalikan letak berkas handshake di dalam folder data.
func HandshakePath(dataDir string) string {
	return filepath.Join(dataDir, "engine.json")
}

// WriteHandshake menulis berkas handshake dengan izin hanya-pemilik.
func WriteHandshake(dataDir string, h Handshake) (string, error) {
	path := HandshakePath(dataDir)
	raw, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return "", err
	}
	// 0600: kuncinya sama saja dengan kata sandi ke seluruh isi berkas
	// pengguna, jadi pengguna lain di mesin yang sama tidak boleh membacanya.
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// SetToken menyalakan pemeriksaan kunci. Token kosong = pemeriksaan mati.
func (s *Server) SetToken(t string) { s.token = t }

// openPaths = jalur yang boleh diakses tanpa kunci.
//
// Hanya /api/health: shell desktop memakainya untuk tahu engine sudah hidup
// sebelum ia sempat membaca berkas handshake, dan jawabannya tidak memuat apa
// pun selain kata "ok".
var openPaths = map[string]bool{"/api/health": true}

// requestToken mengambil kunci dari permintaan.
//
// Query ikut diterima karena tidak semua permintaan bisa membawa header:
// EventSource (progres job), <video src>, dan tautan unduh dibuat oleh browser,
// bukan oleh kode kita.
func requestToken(r *http.Request) string {
	if v := r.Header.Get("X-Clipper-Token"); v != "" {
		return v
	}
	if v := r.Header.Get("Authorization"); strings.HasPrefix(v, "Bearer ") {
		return strings.TrimPrefix(v, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

// withToken menolak permintaan tanpa kunci yang benar.
func (s *Server) withToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" || openPaths[r.URL.Path] || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		got := requestToken(r)
		// Perbandingan waktu-tetap: token dibandingkan pada setiap permintaan
		// dari sumber mana pun, jadi lama perbandingannya tidak boleh
		// membocorkan berapa karakter awal yang sudah benar.
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
			writeErr(w, 401, "missing or wrong session key — open the app window again")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// localOrigin melaporkan apakah sebuah Origin datang dari mesin ini sendiri.
//
// Halaman web mana pun bisa menyuruh browser pengunjungnya menghubungi
// 127.0.0.1; yang menghentikannya adalah browser menolak MEMBACA jawabannya
// tanpa izin CORS. Jadi izin itu hanya diberikan kepada halaman lokal —
// GUI Next.js di :3000 saat pengembangan, dan jendela aplikasi nanti.
func localOrigin(origin string) bool {
	if origin == "" || origin == "null" {
		return true // permintaan bukan-browser, atau halaman dari berkas
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "http", "https":
		host := u.Hostname()
		return host == "127.0.0.1" || host == "localhost" || host == "::1"
	default:
		// Skema milik shell desktop (tauri://, app://, file://).
		return true
	}
}

// withCORS memberi izin baca hanya kepada halaman lokal.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if !localOrigin(origin) {
			writeErr(w, 403, "this engine only serves apps running on this computer")
			return
		}
		if origin != "" {
			// Origin dipantulkan, bukan "*": dengan "*" browser menolak
			// mengirim header kunci pada permintaan yang butuh preflight.
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Clipper-Token, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
