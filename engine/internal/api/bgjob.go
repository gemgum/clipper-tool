package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Job latar yang BUKAN job klip: pembuat berita (posts.go) dan pembuat caption
// (captions.go).
//
// Keduanya berbentuk sama persis — mulai, catat kemajuannya, siarkan ke halaman
// yang sedang menonton, simpan hasilnya — jadi bentuk itu hidup di satu tempat
// dan dibedakan hanya oleh tipe hasilnya. Sengaja tidak digabung dengan
// job.Manager: job klip punya antrian, riwayat di disk, dan folder kerja per
// job, sementara ini cuma jendela ke pekerjaan yang sedang berjalan.
//
// Disimpan di memori saja: hasil kerjanya sudah ada di foldernya sendiri.

// bgJob keadaan satu job latar.
type bgJob[T any] struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"` // running | done | error | canceled
	Stage     string    `json:"stage"`
	Progress  float64   `json:"progress"`
	Log       []string  `json:"log"`
	Error     string    `json:"error,omitempty"`
	Result    *T        `json:"result,omitempty"`
	CreatedAt time.Time `json:"created_at"`

	// cancel menghentikan job yang sedang jalan. Tidak ikut ke JSON — ia fungsi,
	// dan lagipula pemanggilnya cukup tahu Status.
	cancel context.CancelFunc `json:"-"`
}

// maxBGLog membatasi baris log yang disimpan per job. Satu job menghasilkan
// puluhan baris, bukan ribuan; batas ini menjaga job yang mengamuk tidak
// menghabiskan memori.
const maxBGLog = 500

// bgStore menyimpan job latar sejenis beserta pelanggan SSE-nya. Nilai nolnya
// langsung bisa dipakai.
type bgStore[T any] struct {
	mu     sync.RWMutex
	seq    int
	prefix string // awalan id, mis. "post" → post_0001
	jobs   map[string]*bgJob[T]
	subs   map[chan bgJob[T]]struct{}
}

func (p *bgStore[T]) init() {
	if p.jobs == nil {
		p.jobs = map[string]*bgJob[T]{}
		p.subs = map[chan bgJob[T]]struct{}{}
	}
}

func (p *bgStore[T]) create(prefix string, cancel context.CancelFunc) *bgJob[T] {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	p.seq++
	j := &bgJob[T]{
		ID:        fmt.Sprintf("%s_%04d", prefix, p.seq),
		Status:    "running",
		Stage:     "queued",
		CreatedAt: time.Now(),
		cancel:    cancel,
	}
	p.jobs[j.ID] = j
	return j
}

// update menyimpan perubahan lalu menyiarkannya.
func (p *bgStore[T]) update(id string, fn func(*bgJob[T])) {
	p.mu.Lock()
	p.init()
	j, ok := p.jobs[id]
	if !ok {
		p.mu.Unlock()
		return
	}
	fn(j)
	if len(j.Log) > maxBGLog {
		j.Log = j.Log[len(j.Log)-maxBGLog:]
	}
	snapshot := *j
	subs := make([]chan bgJob[T], 0, len(p.subs))
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

// stop menghentikan job yang sedang berjalan. Mengembalikan false bila job-nya
// tidak ada; job yang sudah selesai dianggap berhasil dibatalkan supaya tombol
// di GUI tidak melaporkan galat untuk keadaan yang tidak salah.
func (p *bgStore[T]) stop(id string) bool {
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

func (p *bgStore[T]) get(id string) (bgJob[T], bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	j, ok := p.jobs[id]
	if !ok {
		var zero bgJob[T]
		return zero, false
	}
	return *j, true
}

// snapshot mengembalikan seluruh job, terbaru dulu.
func (p *bgStore[T]) snapshot() []bgJob[T] {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]bgJob[T], 0, len(p.jobs))
	for _, j := range p.jobs {
		out = append(out, *j)
	}
	for i, k := 0, len(out)-1; i < k; i, k = i+1, k-1 {
		out[i], out[k] = out[k], out[i]
	}
	return out
}

func (p *bgStore[T]) subscribe() chan bgJob[T] {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	c := make(chan bgJob[T], 64)
	p.subs[c] = struct{}{}
	return c
}

func (p *bgStore[T]) unsubscribe(c chan bgJob[T]) {
	p.mu.Lock()
	delete(p.subs, c)
	p.mu.Unlock()
	close(c)
}

// finish menutup satu job: galat, dibatalkan, atau selesai dengan hasilnya.
//
// Dipakai bersama kedua jenis job supaya keduanya memperlakukan pembatalan
// dengan cara yang sama — dibatalkan pengguna BUKAN galat, dan menampilkannya
// merah membuat tombol yang baru saja ditekan terlihat rusak.
func (p *bgStore[T]) finish(id string, ctx context.Context, res T, err error) {
	p.update(id, func(j *bgJob[T]) {
		switch {
		case err != nil && ctx.Err() != nil:
			j.Status, j.Stage, j.Error = "canceled", "canceled", ""
		case err != nil:
			j.Status, j.Stage, j.Error = "error", "error", err.Error()
		default:
			j.Status, j.Stage, j.Progress = "done", "done", 1
			j.Result = &res
		}
	})
}

// stream mengalirkan kemajuan SELURUH job dalam satu store lewat SSE.
//
// Boleh disambung ulang kapan saja: pesan pertama berisi keadaan terkini, jadi
// halaman yang baru dibuka langsung tahu apa yang sedang berjalan.
func (p *bgStore[T]) stream(w http.ResponseWriter, r *http.Request, event string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming is not supported by this connection")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)

	for _, j := range p.snapshot() {
		writeSSE(w, event, j)
	}
	writeSSE(w, "ready", map[string]any{"ok": true})
	flusher.Flush()

	ch := p.subscribe()
	defer p.unsubscribe(ch)

	tick := time.NewTicker(20 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case j := <-ch:
			writeSSE(w, event, j)
			flusher.Flush()
		case <-tick.C:
			writeSSE(w, "ping", map[string]any{})
			flusher.Flush()
		}
	}
}
