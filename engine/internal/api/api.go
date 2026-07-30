// Package api menyediakan HTTP API + SSE untuk GUI.
package api

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/capture"
	"github.com/gemgum/clipper/engine/internal/card"
	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/ffmpeg"
	"github.com/gemgum/clipper/engine/internal/job"
	"github.com/gemgum/clipper/engine/internal/news"
	"github.com/gemgum/clipper/engine/internal/score/llm"
	"github.com/gemgum/clipper/engine/internal/score/ollama"
)

// Server membungkus manager job.
type Server struct {
	mgr   *job.Manager
	root  string
	paths config.Paths
	ff    *ffmpeg.Client
	kartu *card.Builder
}

func NewServer(mgr *job.Manager, root string) *Server {
	paths := config.ResolvePaths(root, config.DefaultOptions())
	return &Server{
		mgr:   mgr,
		root:  root,
		paths: paths,
		ff:    ffmpeg.New(paths.FFmpeg, paths.FFprobe),
		kartu: card.New(capture.New(paths.Chrome), paths.FontsDir),
	}
}

// Handler membangun router.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/config", s.config)
	mux.HandleFunc("GET /api/models", s.listModels)
	mux.HandleFunc("GET /api/settings", s.getSettings)
	mux.HandleFunc("POST /api/settings", s.postSettings)
	mux.HandleFunc("GET /api/ollama/status", s.ollamaStatus)
	mux.HandleFunc("POST /api/ollama/pull", s.ollamaPull)
	mux.HandleFunc("GET /api/fonts", s.listFonts)
	mux.HandleFunc("GET /api/font-file", s.fontFile)
	mux.HandleFunc("GET /api/font-check", s.fontCheck)
	mux.HandleFunc("GET /api/probe", s.probe)
	mux.HandleFunc("GET /api/frame", s.frame)
	mux.HandleFunc("POST /api/upload", s.upload)
	mux.HandleFunc("POST /api/jobs", s.createJob)
	mux.HandleFunc("GET /api/jobs", s.listJobs)
	mux.HandleFunc("GET /api/jobs/{id}", s.getJob)
	mux.HandleFunc("GET /api/jobs/{id}/events", s.jobEvents)
	mux.HandleFunc("POST /api/jobs/{id}/cancel", s.cancelJob)
	mux.HandleFunc("GET /api/jobs/{id}/clips", s.jobClips)
	mux.HandleFunc("GET /api/jobs/{id}/clips/{clip}/file", s.clipFile)
	// Kartu berita (tab kedua di GUI).
	mux.HandleFunc("GET /api/news/feeds", s.newsFeeds)
	mux.HandleFunc("GET /api/news/list", s.newsList)
	mux.HandleFunc("POST /api/news/article", s.newsArticle)
	mux.HandleFunc("POST /api/news/resolve", s.newsResolve)
	mux.HandleFunc("POST /api/news/analisis", s.newsAnalisis)
	mux.HandleFunc("POST /api/card", s.makeCard)
	mux.HandleFunc("GET /api/card/{id}/file", s.cardFile)
	mux.HandleFunc("GET /api/card/{id}/zip", s.cardZip)
	return withCORS(mux)
}

// --- kartu berita ---

// newsFeeds melaporkan daftar sumber RSS bawaan + apakah browser tersedia.
// GUI memakai flag itu untuk menjelaskan sejak awal bila kartu tak bisa dibuat,
// bukan membiarkan pengguna menemukannya saat menekan tombol.
func (s *Server) newsFeeds(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"feeds":       news.SumberBawaan,
		"ada_browser": s.paths.Chrome != "",
		"browser":     filepath.Base(s.paths.Chrome),
		"gaya":        []string{card.GayaGelap, card.GayaTerang, card.GayaKutipan},
		"rasio":       []string{card.Rasio916, card.Rasio45, card.Rasio11},
		"rata":        []string{card.RataKiri, card.RataTengah, card.RataKanan, card.RataPenuh},
	})
}

// newsList mengambil isi satu feed. Param 'feed' boleh berupa ID sumber bawaan
// atau URL feed apa pun.
func (s *Server) newsList(w http.ResponseWriter, r *http.Request) {
	maksCari, _ := strconv.Atoi(r.URL.Query().Get("maks"))
	// Kata kunci menang atas pilihan feed: kalau pengguna mengetik sesuatu, itu
	// yang dia mau lihat.
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		item, err := news.Cari(r.Context(), q, maksCari)
		if err != nil {
			writeErr(w, 502, err.Error())
			return
		}
		writeJSON(w, 200, item)
		return
	}

	feed := strings.TrimSpace(r.URL.Query().Get("feed"))
	if feed == "" {
		feed = news.SumberBawaan[0].ID
	}
	// Nama media diambil dari daftar kurasi bila feednya dikenal — judul channel
	// RSS terlalu berbeda-beda antar media untuk dipakai sebagai badge kartu.
	nama := ""
	for _, su := range news.SumberBawaan {
		if su.ID == feed {
			feed, nama = su.URL, su.Nama
			break
		}
	}
	if !strings.HasPrefix(feed, "http://") && !strings.HasPrefix(feed, "https://") {
		writeErr(w, 400, "feed tidak dikenal — pakai salah satu id bawaan, atau tempel URL feed lengkap")
		return
	}
	maks, _ := strconv.Atoi(r.URL.Query().Get("maks"))
	item, err := news.Daftar(r.Context(), feed, nama, maks)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, item)
}

// ramban menyediakan pembuka halaman berbasis browser untuk paket news.
// Mengembalikan nil bila browser tidak ada — news yang menerjemahkannya jadi
// pesan yang bisa ditindaklanjuti pengguna.
func (s *Server) ramban() news.Perambah {
	if s.paths.Chrome == "" {
		return nil
	}
	cap := capture.New(s.paths.Chrome)
	return func(ctx context.Context, url string) (string, error) {
		return cap.DumpDOM(ctx, url, 15000)
	}
}

// newsArticle membaca metadata satu artikel dari URL yang ditempel pengguna.
func (s *Server) newsArticle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "body JSON tidak valid")
		return
	}
	a, err := news.Ambil(r.Context(), req.URL)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, a)
}

// newsResolve menerjemahkan pengalih hasil pencarian jadi alamat artikel yang
// sebenarnya, supaya pengguna bisa menyalin tautan yang benar-benar menunjuk ke
// medianya — bukan alamat panjang milik Google.
func (s *Server) newsResolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "body JSON tidak valid")
		return
	}
	asli, err := news.Resolusi(r.Context(), req.URL, s.ramban(), s.paths.DataDir)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"url": asli})
}

// newsAnalisis mengambil badan artikel lalu meminta LLM menilai paragraf mana
// yang paling layak jadi isi kartu & caption.
//
// LLM di sini hanya MEMILIH NOMOR paragraf — teksnya diambil engine dari
// artikel. Jadi tidak ada peluang mengarang kalimat, dan hasilnya selalu
// verbatim dari sumbernya.
func (s *Server) newsAnalisis(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL         string `json:"url"`
		Provider    string `json:"provider"`     // claude | ollama
		LLMModel    string `json:"llm_model"`    // model Claude
		OllamaModel string `json:"ollama_model"` // model lokal
		OllamaURL   string `json:"ollama_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "body JSON tidak valid")
		return
	}
	isi, err := news.AmbilIsi(r.Context(), req.URL, s.ramban(), s.paths.DataDir)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}

	jalan, nama, err := s.mesinLLM(req.Provider, req.LLMModel, req.OllamaModel, req.OllamaURL)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	pilihan, err := news.Pilih(r.Context(), isi, jalan, nama, s.paths.DataDir)
	if err != nil {
		// Mesin yang dipilih pengguna dipakai apa adanya — bila gagal, job
		// berhenti dengan pesan akar masalah (catatan/12). Tidak ada
		// perpindahan diam-diam ke mesin lain.
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"artikel":  isi.Artikel,
		"paragraf": isi.Paragraf,
		"pilihan":  pilihan,
	})
}

// mesinLLM merangkai satu fungsi pemanggil LLM sesuai mesin yang dipilih.
func (s *Server) mesinLLM(provider, modelClaude, modelOllama, urlOllama string) (news.Mesin, string, error) {
	switch provider {
	case "claude":
		key := s.mgr.APIKey()
		if key == "" {
			return nil, "", fmt.Errorf("API key Claude kosong — isi di tab Klip video, panel Mesin AI, atau ANTHROPIC_API_KEY di .env")
		}
		c := llm.New(key, modelClaude)
		nama := "Claude (" + c.Model + ")"
		return func(ctx context.Context, system, user string) (string, error) {
			return c.Lengkapi(ctx, system, user, 4096)
		}, nama, nil
	case "ollama", "":
		c := ollama.New(urlOllama, modelOllama)
		nama := "Ollama (" + c.Model + ")"
		return func(ctx context.Context, system, user string) (string, error) {
			return c.Lengkapi(ctx, system, user, news.SkemaPilih(), 2048)
		}, nama, nil
	}
	return nil, "", fmt.Errorf("mesin %q tidak dikenal — pilih \"claude\" atau \"ollama\"", provider)
}

// makeCard merender kartu jadi PNG dan membalas id-nya.
//
// Sengaja sinkron, tidak lewat antrian job: satu kartu selesai dalam hitungan
// detik, sementara mesin job dirancang untuk pekerjaan hitungan menit.
func (s *Server) makeCard(w http.ResponseWriter, r *http.Request) {
	if s.paths.Chrome == "" {
		writeErr(w, 503, "browser tidak ditemukan — pasang Chrome/Chromium, atau set CLIPPER_CHROME ke path chrome.exe")
		return
	}
	var p card.Permintaan
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeErr(w, 400, "body JSON tidak valid: "+err.Error())
		return
	}
	id := fmt.Sprintf("kartu-%d", time.Now().UnixNano())
	if err := s.kartu.Buat(r.Context(), p, s.dirKartu(id)); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	lebar, tinggi := card.Dims(p.Rasio)
	writeJSON(w, 201, map[string]interface{}{
		"id": id, "lebar": lebar, "tinggi": tinggi,
		"file": "/api/card/" + id + "/file",
		"zip":  "/api/card/" + id + "/zip",
	})
}

func (s *Server) dirKartu(id string) string {
	return filepath.Join(s.root, "data", "kartu", id)
}

// cardFile menyajikan PNG kartu. Id dibatasi pola yang kita buat sendiri
// supaya parameter dari luar tidak bisa menunjuk berkas lain.
func (s *Server) cardFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idKartuOK.MatchString(id) {
		writeErr(w, 400, "id kartu tidak valid")
		return
	}
	p := filepath.Join(s.dirKartu(id), card.BerkasPNG)
	if _, err := os.Stat(p); err != nil {
		writeErr(w, 404, "kartu tidak ditemukan")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, p)
}

// cardZip membungkus gambar + caption + keterangan sumber jadi satu berkas.
func (s *Server) cardZip(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idKartuOK.MatchString(id) {
		writeErr(w, 400, "id kartu tidak valid")
		return
	}
	dir := s.dirKartu(id)
	isi, err := os.ReadDir(dir)
	if err != nil || len(isi) == 0 {
		writeErr(w, 404, "kartu tidak ditemukan")
		return
	}
	// Header ditulis lebih dulu karena zip ditulis langsung ke koneksi
	// (tidak disusun di memori); setelah byte pertama terkirim, status HTTP
	// tidak bisa diubah lagi.
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+id+`.zip"`)

	zw := zip.NewWriter(w)
	defer zw.Close()
	for _, e := range isi {
		if e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		f, err := zw.Create(e.Name())
		if err != nil {
			return
		}
		if _, err := f.Write(raw); err != nil {
			return
		}
	}
}

var idKartuOK = regexp.MustCompile(`^kartu-[0-9]{1,25}$`)

// listModels melaporkan model whisper yang tersedia/terunduh.
func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {
	known := []struct {
		name string
		size string
	}{
		{"tiny", "~75 MB"}, {"base", "~142 MB"}, {"small", "~466 MB"},
		{"medium", "~1.5 GB"}, {"large-v3", "~2.9 GB"},
	}
	type modelInfo struct {
		Name       string `json:"name"`
		Size       string `json:"size"`
		Downloaded bool   `json:"downloaded"`
	}
	out := make([]modelInfo, 0, len(known))
	for _, m := range known {
		p := filepath.Join(s.root, "models", "ggml-"+m.name+".bin")
		_, err := os.Stat(p)
		out = append(out, modelInfo{Name: m.name, Size: m.size, Downloaded: err == nil})
	}
	writeJSON(w, 200, out)
}

// upload menerima berkas video (multipart, streaming) → simpan → balas path.
// Untuk drag/drop di GUI. File di-stream ke disk (tidak dimuat ke RAM).
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	reader, err := r.MultipartReader()
	if err != nil {
		writeErr(w, 400, "bukan multipart: "+err.Error())
		return
	}
	uploadDir := filepath.Join(s.root, "data", "uploads")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeErr(w, 400, "baca part: "+err.Error())
			return
		}
		if part.FormName() != "file" {
			continue
		}
		name := filepath.Base(part.FileName())
		if name == "" || name == "." {
			name = "upload.bin"
		}
		dst := filepath.Join(uploadDir, name)
		f, err := os.Create(dst)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		if _, err := io.Copy(f, part); err != nil {
			f.Close()
			writeErr(w, 500, "simpan file: "+err.Error())
			return
		}
		f.Close()
		abs, _ := filepath.Abs(dst)
		writeJSON(w, 200, map[string]string{"path": abs, "name": name})
		return
	}
	writeErr(w, 400, "tidak ada field 'file'")
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"modes":            []string{"offline", "hybrid", "online"},
		"reframe":          []string{"center", "face_follow"},
		"subtitle_styles":  []string{"plain", "viral"},
		"resolutions":      []string{"720p", "1080p", "1440p"},
		"qualities":        []string{"draft", "hd", "max"},
		"duration_presets": []string{"auto", "30", "60", "90", "120", "180"},
		"defaults":         config.DefaultOptions(),
	})
}

// getSettings melaporkan status setelan (apakah API key sudah ada).
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{"has_key": s.mgr.HasAPIKey()})
}

// postSettings menyimpan API key Claude (dari GUI) — di memori + .env.
func (s *Server) postSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AnthropicAPIKey string `json:"anthropic_api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "body JSON tidak valid")
		return
	}
	key := strings.TrimSpace(req.AnthropicAPIKey)
	s.mgr.SetAPIKey(key)
	if key != "" {
		_ = writeEnvKey(filepath.Join(s.root, ".env"), "ANTHROPIC_API_KEY", key)
	}
	writeJSON(w, 200, map[string]interface{}{"ok": true, "has_key": s.mgr.HasAPIKey()})
}

// ollamaStatus memeriksa apakah Ollama jalan & model yang terpasang.
func (s *Server) ollamaStatus(w http.ResponseWriter, r *http.Request) {
	info := ollama.Status(r.Context(), r.URL.Query().Get("url"))
	writeJSON(w, 200, info)
}

// ollamaPull mengunduh model via Ollama (bisa lama). Notifikasi bila gagal.
func (s *Server) ollamaPull(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL   string `json:"url"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "body JSON tidak valid")
		return
	}
	if req.Model == "" {
		writeErr(w, 400, "field 'model' wajib")
		return
	}
	if err := ollama.Pull(r.Context(), req.URL, req.Model); err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]interface{}{"ok": true})
}

// writeEnvKey menambah/mengganti satu KEY=value di berkas .env.
func writeEnvKey(path, key, val string) error {
	var lines []string
	found := false
	if raw, err := os.ReadFile(path); err == nil {
		for _, ln := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(ln), key+"=") {
				lines = append(lines, key+"="+val)
				found = true
			} else {
				lines = append(lines, ln)
			}
		}
	}
	if !found {
		lines = append(lines, key+"="+val)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600)
}

// fontCatalog = font bawaan proyek. Nama harus cocok dengan family internal
// font, sebab nama itulah yang ditulis ke .ass dan dicari libass.
var fontCatalog = []struct{ file, name string }{
	{"Montserrat.ttf", "Montserrat"},
	{"Anton.ttf", "Anton"},
	{"BebasNeue.ttf", "Bebas Neue"},
}

// fontNameOK = format nama font yang diterima: diawali huruf/angka, lalu
// huruf, angka, spasi, titik, kutip satu, "&", atau "-". Maksimal 64 karakter.
// Sengaja ketat — nama ini masuk ke berkas .ass dan ke argumen fc-match.
var fontNameOK = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 .'&-]{0,63}$`)

// fontResult hasil pengecekan satu nama font.
type fontResult struct {
	Valid  bool   `json:"valid"`
	Name   string `json:"name"`   // nama yang diminta
	Family string `json:"family"` // family yang benar-benar akan dipakai libass
	Source string `json:"source"` // "bawaan" | "sistem"
	Error  string `json:"error"`  // alasan bila tidak valid
	file   string // lokasi berkas (tidak dikirim ke GUI)
}

// listFonts melaporkan font yang tersedia di folder assets/fonts.
func (s *Server) listFonts(w http.ResponseWriter, r *http.Request) {
	type fontInfo struct {
		Name string `json:"name"`
	}
	out := []fontInfo{}
	for _, f := range fontCatalog {
		if _, err := os.Stat(filepath.Join(s.paths.FontsDir, f.file)); err == nil {
			out = append(out, fontInfo{Name: f.name})
		}
	}
	if len(out) == 0 {
		out = append(out, fontInfo{Name: "Montserrat"}) // fallback
	}
	writeJSON(w, 200, out)
}

// fontCheck memvalidasi nama font yang diketik manual di GUI: formatnya benar
// dan fontnya betul-betul ada (bawaan proyek atau terpasang di sistem).
func (s *Server) fontCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.resolveFont(r.Context(), r.URL.Query().Get("name")))
}

// resolveFont mencari berkas font untuk sebuah nama family.
//
// Urutannya: font bawaan proyek dulu, baru font sistem lewat fontconfig —
// sama dengan urutan yang dipakai libass saat merender (fontsdir lebih dulu).
// fc-match SELALU mengembalikan sesuatu (jatuh ke font pengganti bila tak
// ketemu), jadi nama family hasilnya wajib dibandingkan dengan yang diminta;
// kalau berbeda berarti font itu tidak terpasang.
func (s *Server) resolveFont(ctx context.Context, name string) fontResult {
	name = strings.TrimSpace(name)
	res := fontResult{Name: name}
	if name == "" {
		res.Error = "nama font kosong"
		return res
	}
	if !fontNameOK.MatchString(name) {
		res.Error = "format nama tidak valid — pakai huruf/angka, spasi, titik, ' & atau -, maksimal 64 karakter (contoh: \"Poppins\", \"Bebas Neue\")"
		return res
	}
	for _, f := range fontCatalog {
		if strings.EqualFold(f.name, name) {
			p := filepath.Join(s.paths.FontsDir, f.file)
			if _, err := os.Stat(p); err == nil {
				return fontResult{Valid: true, Name: name, Family: f.name, Source: "bawaan", file: p}
			}
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "fc-match", "-f", "%{family}\t%{file}", name).Output()
	if err != nil {
		res.Error = "tidak bisa memeriksa font sistem (fontconfig/fc-match tidak tersedia) — pakai font bawaan"
		return res
	}
	bagian := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
	if len(bagian) < 2 {
		res.Error = "font tidak ditemukan di sistem"
		return res
	}
	// %{family} bisa berisi beberapa alias yang dipisah koma.
	for _, fam := range strings.Split(bagian[0], ",") {
		if strings.EqualFold(strings.TrimSpace(fam), name) {
			return fontResult{Valid: true, Name: name, Family: strings.TrimSpace(fam), Source: "sistem", file: bagian[1]}
		}
	}
	res.Family = strings.TrimSpace(strings.Split(bagian[0], ",")[0])
	res.Error = fmt.Sprintf("font %q tidak terpasang — subtitle akan dirender memakai %q sebagai gantinya", name, res.Family)
	return res
}

// fontFile menyajikan berkas font agar GUI bisa memuat font asli untuk preview.
// Melayani font bawaan maupun font sistem yang lolos pengecekan.
func (s *Server) fontFile(w http.ResponseWriter, r *http.Request) {
	res := s.resolveFont(r.Context(), r.URL.Query().Get("name"))
	if !res.Valid {
		writeErr(w, 404, res.Error)
		return
	}
	if ext := strings.ToLower(filepath.Ext(res.file)); ext == ".otf" {
		w.Header().Set("Content-Type", "font/otf")
	} else {
		w.Header().Set("Content-Type", "font/ttf")
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, res.file)
}

// probe mengembalikan durasi & dimensi video.
func (s *Server) probe(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeErr(w, 400, "param 'path' wajib")
		return
	}
	ctx := r.Context()
	dur, err := s.ff.Duration(ctx, path)
	if err != nil {
		writeErr(w, 400, "gagal membaca video: "+err.Error())
		return
	}
	vw, vh, _ := s.ff.Dimensions(ctx, path)
	writeJSON(w, 200, map[string]interface{}{"duration": dur, "width": vw, "height": vh})
}

// frame mengembalikan 1 frame (JPEG) 9:16 untuk preview subtitle, disesuaikan
// memakai mode reframe yang sama dengan render — param 'reframe' (default
// center). Mode yang belum tersedia ditolak, bukan diganti diam-diam.
func (s *Server) frame(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	path := q.Get("path")
	if path == "" {
		writeErr(w, 400, "param 'path' wajib")
		return
	}
	t, _ := strconv.ParseFloat(q.Get("t"), 64)
	// Preview ringan: paksa 720p 9:16.
	opts := config.DefaultOptions()
	opts.Resolution = "720p"
	opts.Aspect = "9:16"
	if rf := q.Get("reframe"); rf != "" {
		opts.Reframe = config.Reframe(rf)
	}
	// Latar & zoom ikut dikirim supaya preview memakai penempatan yang sama
	// persis dengan render — Validate yang menjepit nilainya.
	opts.Latar = q.Get("latar")
	opts.Zoom, _ = strconv.Atoi(q.Get("zoom"))
	if err := opts.Validate(); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	tw, th := opts.Dims()
	img, err := s.ff.ExtractFrame(r.Context(), path, t, tw, th, ffmpeg.Tata{
		Mode: string(opts.Reframe), Latar: opts.Latar, Zoom: opts.Zoom,
	})
	if err != nil || len(img) == 0 {
		writeErr(w, 400, "gagal ekstrak frame")
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(200)
	_, _ = w.Write(img)
}

func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Source struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"source"`
		Options config.Options `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "body JSON tidak valid: "+err.Error())
		return
	}
	if req.Source.Value == "" {
		writeErr(w, 400, "source.value (path video) wajib diisi")
		return
	}
	opts := req.Options
	if err := opts.Validate(); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	j := s.mgr.Create(req.Source.Value, opts)
	snap := j.Snapshot()
	writeJSON(w, 201, snap)
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	jobs := s.mgr.List()
	out := make([]job.JobView, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, j.Snapshot())
	}
	writeJSON(w, 200, out)
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	j, ok := s.mgr.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "job tidak ditemukan")
		return
	}
	snap := j.Snapshot()
	writeJSON(w, 200, snap)
}

func (s *Server) jobClips(w http.ResponseWriter, r *http.Request) {
	j, ok := s.mgr.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "job tidak ditemukan")
		return
	}
	snap := j.Snapshot()
	writeJSON(w, 200, snap.Clips)
}

func (s *Server) clipFile(w http.ResponseWriter, r *http.Request) {
	j, ok := s.mgr.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "job tidak ditemukan")
		return
	}
	clipID := r.PathValue("clip")
	// ?varian=polos → berkas tanpa subtitle (bila dibuat); ?varian=srt → subtitle.
	varian := r.URL.Query().Get("varian")
	snap := j.Snapshot()
	for _, cl := range snap.Clips {
		if cl.ID != clipID {
			continue
		}
		path := cl.VideoPath
		switch varian {
		case "polos":
			path = cl.VideoPathRaw
		case "srt":
			path = cl.SubtitleSRT
		}
		if path != "" {
			http.ServeFile(w, r, path)
			return
		}
	}
	writeErr(w, 404, "klip belum tersedia")
}

func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	if !s.mgr.Cancel(r.PathValue("id")) {
		writeErr(w, 404, "job tidak ditemukan")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "canceled"})
}

// jobEvents mengalirkan progres via Server-Sent Events.
func (s *Server) jobEvents(w http.ResponseWriter, r *http.Request) {
	j, ok := s.mgr.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "job tidak ditemukan")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming tidak didukung")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, unsub := j.Subscribe()
	defer unsub()

	// Kirim status awal.
	snap := j.Snapshot()
	writeSSE(w, "progress", snap)
	flusher.Flush()

	// Jika job sudah selesai/gagal/dibatalkan sebelum kita berlangganan,
	// sampaikan event terminalnya sekarang (agar GUI tidak menunggu selamanya).
	switch snap.Status {
	case job.StatusDone:
		for _, cl := range snap.Clips {
			c := cl
			writeSSE(w, "clip", c)
		}
		writeSSE(w, "done", map[string]interface{}{"job_id": snap.ID, "clips": len(snap.Clips)})
		flusher.Flush()
		return
	case job.StatusError:
		writeSSE(w, "error", map[string]string{"message": snap.Error})
		flusher.Flush()
		return
	case job.StatusCanceled:
		writeSSE(w, "error", map[string]string{"message": "Dibatalkan oleh pengguna"})
		flusher.Flush()
		return
	}

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			writeSSE(w, ev.Type, ev.Data)
			flusher.Flush()
			if ev.Type == "done" || ev.Type == "error" {
				return
			}
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// --- util ---

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeSSE(w http.ResponseWriter, event string, data interface{}) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
