package api

import (
	"net/http"
	"os"
	"strings"
)

// Mesin AI GLOBAL (DESIGN-Clipper-Lanjutan.md §1, keputusan pemilik 9 Oktober
// 2026): satu mesin & model untuk semua alat, plus pengecualian per alat.
// Sebelumnya tiap halaman punya pemilih mesinnya sendiri (notes/39) — empat
// tempat untuk satu pertanyaan, dan keempatnya bisa tidak sinkron.
//
// Disimpan di .env lewat setEnv, sama seperti kunci API: berlaku seketika,
// bertahan setelah aplikasi ditutup, dan di mode web dipakai bersama satu tim.
// Permintaan kerja (job, tulis, caption) tetap membawa engine/model sendiri;
// GUI mengisinya dari sini, jadi tidak ada jalur lama yang patah.

// aiTools = alat yang boleh punya pengecualian.
var aiTools = []string{"clips", "news", "writer", "captions"}

type aiChoice struct {
	Engine string `json:"engine"`
	Model  string `json:"model"`
}

func aiEnvNames(tool string) (engine, model string) {
	if tool == "" {
		return "CLIPPER_AI_ENGINE", "CLIPPER_AI_MODEL"
	}
	t := strings.ToUpper(tool)
	return "CLIPPER_AI_ENGINE_" + t, "CLIPPER_AI_MODEL_" + t
}

func readAIChoice(tool string) *aiChoice {
	en, mn := aiEnvNames(tool)
	e := strings.TrimSpace(os.Getenv(en))
	if e == "" {
		return nil
	}
	return &aiChoice{Engine: e, Model: strings.TrimSpace(os.Getenv(mn))}
}

// effectiveAI: pengecualian alat bila ada, kalau tidak pilihan global, kalau
// belum pernah diisi juga: Ollama lokal (bawaan lama GUI). Model kosong =
// model bawaan mesin itu (yang disimpan di halaman Pengaturan).
func effectiveAI(tool string) aiChoice {
	c := readAIChoice(tool)
	if c == nil {
		c = readAIChoice("")
	}
	if c == nil {
		c = &aiChoice{Engine: "ollama"}
	}
	out := *c
	if out.Model == "" && out.Engine != "heuristic" {
		if d, ok := engineByID(out.Engine); ok {
			out.Model = resolve(d).Model
		}
	}
	return out
}

func aiSettingsView() map[string]any {
	overrides := map[string]*aiChoice{}
	effective := map[string]aiChoice{}
	for _, t := range aiTools {
		overrides[t] = readAIChoice(t)
		effective[t] = effectiveAI(t)
	}
	global := readAIChoice("")
	if global == nil {
		g := effectiveAI("")
		global = &g
	}
	return map[string]any{"global": global, "overrides": overrides, "effective": effective, "tools": aiTools}
}

func (s *Server) getAISettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, aiSettingsView())
}

// saveAISettings: {tool?: "", engine, model}. tool kosong = pilihan global;
// tool + engine kosong = hapus pengecualian alat itu (kembali ikut global).
func (s *Server) saveAISettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tool   string `json:"tool"`
		Engine string `json:"engine"`
		Model  string `json:"model"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.Tool != "" && !contains(aiTools, req.Tool) {
		writeErr(w, 400, "unknown tool "+req.Tool+": choose one of "+strings.Join(aiTools, ", "))
		return
	}
	engine := strings.TrimSpace(req.Engine)
	if engine != "" && engine != "heuristic" {
		if _, ok := engineByID(engine); !ok {
			writeErr(w, 400, "unknown engine "+engine+": choose one of: "+engineIDs())
			return
		}
	}
	if engine == "" && req.Tool == "" {
		writeErr(w, 400, "the shared engine cannot be empty")
		return
	}
	en, mn := aiEnvNames(req.Tool)
	s.setEnv(en, engine)
	s.setEnv(mn, strings.TrimSpace(req.Model))
	writeJSON(w, 200, aiSettingsView())
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
