// Package api menyediakan HTTP API + SSE untuk GUI.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/ffmpeg"
	"github.com/gemgum/clipper/engine/internal/job"
	"github.com/gemgum/clipper/engine/internal/score/ollama"
)

// Server membungkus manager job.
type Server struct {
	mgr   *job.Manager
	root  string
	paths config.Paths
	ff    *ffmpeg.Client
}

func NewServer(mgr *job.Manager, root string) *Server {
	paths := config.ResolvePaths(root, config.DefaultOptions())
	return &Server{
		mgr:   mgr,
		root:  root,
		paths: paths,
		ff:    ffmpeg.New(paths.FFmpeg, paths.FFprobe),
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
	return withCORS(mux)
}

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

// listFonts melaporkan font yang tersedia di folder assets/fonts.
func (s *Server) listFonts(w http.ResponseWriter, r *http.Request) {
	// Nama "name" harus cocok dengan family internal font (untuk libass).
	catalog := []struct{ file, name string }{
		{"Montserrat.ttf", "Montserrat"},
		{"Anton.ttf", "Anton"},
		{"BebasNeue.ttf", "Bebas Neue"},
	}
	type fontInfo struct {
		Name string `json:"name"`
	}
	out := []fontInfo{}
	for _, f := range catalog {
		if _, err := os.Stat(filepath.Join(s.paths.FontsDir, f.file)); err == nil {
			out = append(out, fontInfo{Name: f.name})
		}
	}
	if len(out) == 0 {
		out = append(out, fontInfo{Name: "Montserrat"}) // fallback
	}
	writeJSON(w, 200, out)
}

// fontFile menyajikan berkas .ttf agar GUI bisa memuat font asli untuk preview.
func (s *Server) fontFile(w http.ResponseWriter, r *http.Request) {
	byName := map[string]string{
		"Montserrat": "Montserrat.ttf",
		"Anton":      "Anton.ttf",
		"Bebas Neue": "BebasNeue.ttf",
	}
	file, ok := byName[r.URL.Query().Get("name")]
	if !ok {
		writeErr(w, 404, "font tidak dikenal")
		return
	}
	p := filepath.Join(s.paths.FontsDir, file)
	if _, err := os.Stat(p); err != nil {
		writeErr(w, 404, "berkas font tidak ada")
		return
	}
	w.Header().Set("Content-Type", "font/ttf")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, p)
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

// frame mengembalikan 1 frame (JPEG) yang sudah di-crop 9:16 untuk preview subtitle.
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
	tw, th := opts.Dims()
	img, err := s.ff.ExtractFrame(r.Context(), path, t, tw, th)
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
