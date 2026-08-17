package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gemgum/clipper/engine/internal/score/ollama"
	"github.com/gemgum/clipper/engine/internal/writer"
)

// PostJob keadaan satu job pembuat berita.
//
// Disimpan di memori saja, seperti pemasangan komponen (installs.go) dan tidak
// seperti job klip yang punya riwayat di disk. Alasannya: hasil kerjanya sudah
// ada di folder artikelnya sendiri — daftar job cuma jendela ke pekerjaan yang
// sedang berjalan.
type PostJob struct {
	ID        string         `json:"id"`
	Status    string         `json:"status"` // running | done | error
	Stage     string         `json:"stage"`
	Progress  float64        `json:"progress"`
	Log       []string       `json:"log"`
	Error     string         `json:"error,omitempty"`
	Result    *writer.Result `json:"result,omitempty"`
	CreatedAt time.Time      `json:"created_at"`

	// cancel menghentikan job yang sedang jalan. Tidak ikut ke JSON — ia fungsi,
	// dan lagipula pemanggilnya cukup tahu Status.
	cancel context.CancelFunc `json:"-"`
}

// maxPostLog membatasi baris log yang disimpan per job. Satu job menghasilkan
// puluhan baris, bukan ribuan; batas ini menjaga job yang mengamuk tidak
// menghabiskan memori.
const maxPostLog = 500

type posts struct {
	mu   sync.RWMutex
	seq  int
	jobs map[string]*PostJob
	subs map[chan PostJob]struct{}
}

func (p *posts) init() {
	if p.jobs == nil {
		p.jobs = map[string]*PostJob{}
		p.subs = map[chan PostJob]struct{}{}
	}
}

func (p *posts) create(cancel context.CancelFunc) *PostJob {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	p.seq++
	j := &PostJob{
		ID:        fmt.Sprintf("post_%04d", p.seq),
		Status:    "running",
		Stage:     "queued",
		CreatedAt: time.Now(),
		cancel:    cancel,
	}
	p.jobs[j.ID] = j
	return j
}

// update menyimpan perubahan lalu menyiarkannya.
func (p *posts) update(id string, fn func(*PostJob)) {
	p.mu.Lock()
	p.init()
	j, ok := p.jobs[id]
	if !ok {
		p.mu.Unlock()
		return
	}
	fn(j)
	if len(j.Log) > maxPostLog {
		j.Log = j.Log[len(j.Log)-maxPostLog:]
	}
	snapshot := *j
	subs := make([]chan PostJob, 0, len(p.subs))
	for c := range p.subs {
		subs = append(subs, c)
	}
	p.mu.Unlock()

	for _, c := range subs {
		// Pelanggan yang lambat dilewati, bukan ditunggu — satu halaman yang
		// membeku tidak boleh menghentikan job yang sedang berjalan.
		select {
		case c <- snapshot:
		default:
		}
	}
}

// cancel menghentikan job yang sedang berjalan. Mengembalikan false bila
// job-nya tidak ada; job yang sudah selesai dianggap berhasil dibatalkan supaya
// tombol di GUI tidak melaporkan galat untuk keadaan yang tidak salah.
func (p *posts) stop(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	j, ok := p.jobs[id]
	if !ok {
		return false
	}
	if j.Status == "running" && j.cancel != nil {
		j.cancel()
	}
	return true
}

func (p *posts) get(id string) (PostJob, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	j, ok := p.jobs[id]
	if !ok {
		return PostJob{}, false
	}
	return *j, true
}

// snapshot mengembalikan seluruh job, terbaru dulu.
func (p *posts) snapshot() []PostJob {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]PostJob, 0, len(p.jobs))
	for _, j := range p.jobs {
		out = append(out, *j)
	}
	for i, k := 0, len(out)-1; i < k; i, k = i+1, k-1 {
		out[i], out[k] = out[k], out[i]
	}
	return out
}

func (p *posts) subscribe() chan PostJob {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	c := make(chan PostJob, 64)
	p.subs[c] = struct{}{}
	return c
}

func (p *posts) unsubscribe(c chan PostJob) {
	p.mu.Lock()
	delete(p.subs, c)
	p.mu.Unlock()
	close(c)
}

// postsDir = tempat artikel disimpan. Selalu di bawah DataDir, tidak pernah
// disusun dari akar proyek (CLAUDE.md).
func (s *Server) postsDir() string { return filepath.Join(s.paths.DataDir, "posts") }

// createPost memulai job penulisan lalu LANGSUNG menjawab.
//
// Satu job memanggil LLM (jumlah artikel + 1) kali; pada model lokal itu
// hitungan menit. Menunggunya di dalam permintaan HTTP berarti job mati saat
// pengguna pindah halaman — kesalahan yang pernah dibayar mahal (notes/25).
func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URLs []string `json:"urls"`
		// Engine + Model = bentuk baku pemilih mesin (notes/39). Nama lama
		// masih diterima supaya halaman yang belum diperbarui tetap jalan.
		Engine string `json:"engine"`
		Model  string `json:"model"`
		// Mesin tahap MENULIS, bila berbeda. Kosong = memakai yang di atas.
		WriteEngine string `json:"write_engine"`
		WriteModel  string `json:"write_model"`
		Provider    string `json:"provider"`
		OllamaModel string `json:"ollama_model"`
		LLMModel    string `json:"llm_model"`
		Lang        string `json:"lang"`
		MaxWords    int    `json:"max_words"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if len(req.URLs) == 0 {
		writeErr(w, 400, "the 'urls' field is required — add at least one source article")
		return
	}

	read, readName, err := EngineFor(
		firstNonEmpty(req.Engine, req.Provider),
		firstNonEmpty(req.Model, req.OllamaModel, req.LLMModel))
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	// Mesin tahap menulis dirakit HANYA bila diminta: kosong berarti kedua
	// tahap memakai mesin yang sama, dan itu tetap jalan yang paling umum.
	var write writer.Completer
	writeName := readName
	if req.WriteEngine != "" {
		write, writeName, err = EngineFor(req.WriteEngine, req.WriteModel)
		if err != nil {
			writeErr(w, 400, err.Error())
			return
		}
	}

	// Job berjalan di context.Background() TURUNAN — bukan r.Context(): ia harus
	// hidup setelah permintaan HTTP-nya selesai (notes/25), tapi tetap punya
	// kenop untuk dihentikan.
	ctx, cancel := context.WithCancel(context.Background())
	job := s.posts.create(cancel)
	deps := writer.Deps{
		Read:        read,
		ReadEngine:  readName,
		Write:       write,
		WriteEngine: writeName,
		Browse:      s.browser(),
		CacheDir:    s.paths.DataDir,
		OutDir:      s.postsDir(),
	}
	opts := writer.Options{URLs: req.URLs, Lang: req.Lang, MaxWords: req.MaxWords}

	go func() {
		defer cancel()
		res, err := writer.Run(ctx, opts, deps, func(p writer.Progress) {
			s.posts.update(job.ID, func(j *PostJob) {
				j.Stage, j.Progress = p.Stage, p.Value
				if p.Message != "" {
					j.Log = append(j.Log, p.Message)
				}
			})
		})
		s.posts.update(job.ID, func(j *PostJob) {
			if err != nil {
				// Dibatalkan pengguna BUKAN galat: menampilkannya sebagai galat
				// merah membuat tombol yang baru saja ditekan terlihat rusak.
				if ctx.Err() != nil {
					j.Status, j.Stage, j.Error = "canceled", "canceled", ""
					return
				}
				j.Status, j.Stage, j.Error = "error", "error", err.Error()
				return
			}
			j.Status, j.Stage, j.Progress = "done", "done", 1
			j.Result = &res
		})
	}()

	writeJSON(w, 202, map[string]any{"id": job.ID, "engine": readName, "write_engine": writeName, "started": true})
}

// postLimits menyebut berapa artikel sumber yang muat untuk mesin PENULIS yang
// dipilih.
//
// Ditanyakan sebelum job dimulai, bukan dilaporkan sebagai galat sesudah lima
// menit menunggu: batasnya bergantung pada jendela konteks model, dan itu hal
// yang engine tahu sementara GUI tidak. Mesin cloud tidak punya angka konteks
// yang bisa ditanyakan — dan memang tidak perlu, semuanya jauh di atas ambang.
func (s *Server) postLimits(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("engine")
	model := r.URL.Query().Get("model")
	d, ok := engineByID(id)
	if !ok {
		writeJSON(w, 200, map[string]any{"max_sources": writer.MaxSources})
		return
	}
	ctxTokens := 0
	if d.Kind == kindLocal {
		e := resolve(d)
		if model == "" {
			model = e.Model
		}
		ctxTokens = ollama.ContextOf(r.Context(), e.BaseURL, model)
	}
	writeJSON(w, 200, map[string]any{
		"max_sources": writer.MaxSourcesFor(ctxTokens),
		"context":     ctxTokens,
	})
}

func (s *Server) cancelPost(w http.ResponseWriter, r *http.Request) {
	if !s.posts.stop(r.PathValue("id")) {
		writeErr(w, 404, "no such post job")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "canceled"})
}

func (s *Server) listPosts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"posts": s.posts.snapshot()})
}

func (s *Server) getPost(w http.ResponseWriter, r *http.Request) {
	j, ok := s.posts.get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "no such post job")
		return
	}
	writeJSON(w, 200, j)
}

// postFile menyajikan satu berkas dari folder artikel.
//
// Nama berkasnya dari daftar tetap, bukan dari permintaan: apa pun yang berasal
// dari luar dan dipakai menyusun path adalah jalan keluar dari folder itu.
func (s *Server) postFile(w http.ResponseWriter, r *http.Request) {
	j, ok := s.posts.get(r.PathValue("id"))
	if !ok || j.Result == nil {
		writeErr(w, 404, "no such post job, or it has not finished yet")
		return
	}
	var path, mime string
	switch r.URL.Query().Get("name") {
	case "", "article":
		path, mime = j.Result.Post.Markdown, "text/markdown; charset=utf-8"
	case "sources":
		path, mime = j.Result.Post.Sources, "application/json"
	case "image":
		path, mime = j.Result.Post.Image, ""
	default:
		writeErr(w, 400, "unknown file — use name=article, name=sources or name=image")
		return
	}
	if path == "" {
		writeErr(w, 404, "that file was not written for this article")
		return
	}
	if _, err := os.Stat(path); err != nil {
		writeErr(w, 404, "file not found")
		return
	}
	if mime != "" {
		w.Header().Set("Content-Type", mime)
	}
	http.ServeFile(w, r, path)
}

// postEvents mengalirkan kemajuan SEMUA job penulisan.
//
// Boleh disambung ulang kapan saja: pesan pertama berisi keadaan terkini, jadi
// halaman yang baru dibuka langsung tahu apa yang sedang berjalan.
func (s *Server) postEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming is not supported by this connection")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)

	for _, j := range s.posts.snapshot() {
		writeSSE(w, "post", j)
	}
	writeSSE(w, "ready", map[string]any{"ok": true})
	flusher.Flush()

	ch := s.posts.subscribe()
	defer s.posts.unsubscribe(ch)

	tick := time.NewTicker(20 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case j := <-ch:
			writeSSE(w, "post", j)
			flusher.Flush()
		case <-tick.C:
			writeSSE(w, "ping", map[string]any{})
			flusher.Flush()
		}
	}
}
