package api

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gemgum/clipper/engine/internal/score/ollama"
	"github.com/gemgum/clipper/engine/internal/setup"
)

// Halaman Requirements: status komponen + pemasangannya.
//
// Dua endpoint, sesuai rencana di notes/23:
//
//	GET  /api/requirements          daftar komponen + statusnya
//	POST /api/requirements/install  memasang satu komponen, mengalirkan progres
//
// Progresnya memakai format SSE yang sama dengan job — satu mekanisme aliran
// untuk seluruh engine, bukan dua.

// requirements melaporkan status semua komponen.
func (s *Server) requirements(w http.ResponseWriter, r *http.Request) {
	// Ollama diperiksa lewat jaringan; itu bagian paling lambat di halaman ini,
	// tapi tanpa hasilnya barisnya cuma bisa bilang "tidak tahu".
	oll := ollama.Status(r.Context(), r.URL.Query().Get("ollama_url"))
	detail := ""
	switch {
	case oll.Running && len(oll.Installed) > 0:
		var names []string
		for _, m := range oll.Installed {
			names = append(names, m.Base)
		}
		detail = "Running. Models: " + strings.Join(names, ", ")
	case oll.Running:
		detail = "Running, but no model is installed yet — pull one below."
	}

	comps := setup.Status(s.layout, oll.Running, detail)
	writeJSON(w, 200, map[string]any{
		"components": comps,
		"missing":    setup.Missing(comps),
		"data_dir":   s.layout.DataDir,
		"models_dir": s.layout.ModelsDir,
		"tools_dir":  s.layout.ToolsDir,
		"dev":        s.layout.Dev,
		"ollama":     oll,
	})
}

// installing menjaga satu komponen tidak dipasang dua kali sekaligus. Dua
// unduhan ke berkas tujuan yang sama akan saling menimpa dan menghasilkan
// berkas rusak yang terlihat seperti berhasil.
type installing struct {
	mu sync.Mutex
	on map[string]bool
}

func (i *installing) start(id string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.on == nil {
		i.on = map[string]bool{}
	}
	if i.on[id] {
		return false
	}
	i.on[id] = true
	return true
}

func (i *installing) done(id string) {
	i.mu.Lock()
	delete(i.on, id)
	i.mu.Unlock()
}

// installComponent memasang satu komponen sambil mengalirkan progresnya.
//
// POST, bukan GET, sebab ia mengubah keadaan mesin — jadi klien membacanya
// dengan fetch + reader, bukan EventSource. Bentuk aliran datanya tetap SSE.
func (s *Server) installComponent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		writeErr(w, 400, "the 'id' field is required")
		return
	}
	if !s.installs.start(req.ID) {
		writeErr(w, 409, "that component is already being installed")
		return
	}
	defer s.installs.done(req.ID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming is not supported by this connection")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	writeSSE(w, "progress", setup.Progress{Message: "Starting…"})
	flusher.Flush()

	err := setup.Install(r.Context(), s.layout, req.ID, func(p setup.Progress) {
		writeSSE(w, "progress", p)
		flusher.Flush()
	})
	if err != nil {
		writeSSE(w, "error", map[string]string{"message": err.Error()})
		flusher.Flush()
		return
	}
	// Status terbaru ikut dikirim supaya GUI tidak perlu bertanya lagi.
	writeSSE(w, "done", map[string]any{
		"id":         req.ID,
		"components": setup.Status(s.layout, false, ""),
	})
	flusher.Flush()
}

// removeComponent menghapus model yang sudah diunduh.
//
// Ada karena model besar memakan 2,9 GB dan pengguna yang mencobanya sekali
// harus punya cara membuangnya tanpa mencari-cari foldernya sendiri.
func (s *Server) removeComponent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	name, ok := strings.CutPrefix(strings.TrimSpace(req.ID), "model:")
	if !ok {
		writeErr(w, 400, "only whisper models can be removed from here")
		return
	}
	if err := setup.RemoveModel(s.layout, name); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"ok":         true,
		"components": setup.Status(s.layout, false, ""),
	})
}

// listModels melaporkan model whisper yang tersedia/terunduh.
//
// Bertahan demi GUI yang sudah ada; daftar modelnya sendiri sekarang tinggal
// satu, di paket setup.
func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {
	type modelInfo struct {
		Name       string `json:"name"`
		Size       string `json:"size"`
		Downloaded bool   `json:"downloaded"`
	}
	out := make([]modelInfo, 0, len(setup.Models))
	for _, m := range setup.Models {
		out = append(out, modelInfo{
			Name: m.Name, Size: m.Size,
			Downloaded: setup.HasModel(s.layout, m.Name),
		})
	}
	writeJSON(w, 200, out)
}

// requirementsGuard menolak job bila komponen wajib belum ada, dengan menyebut
// yang mana. Tanpa ini pesannya baru muncul dari ffmpeg belasan detik kemudian,
// dalam bahasa yang tidak menyebut apa yang harus dipasang.
func (s *Server) missingRequirement() error {
	missing := setup.Missing(setup.Status(s.layout, false, ""))
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("%s is not installed yet — open the Requirements page to install it",
		strings.Join(missing, " and "))
}
