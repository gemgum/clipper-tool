package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/news"
	"github.com/gemgum/clipper/engine/internal/pipeline"
	"github.com/gemgum/clipper/engine/internal/score/llm"
	"github.com/gemgum/clipper/engine/internal/score/ollama"
	"github.com/gemgum/clipper/engine/internal/writer"
)

// Mesin LLM, satu daftar untuk seluruh aplikasi (notes/39).
//
// Sebelumnya tiap tab merakit pilihannya sendiri dan kunci Claude hanya bisa
// diisi dari tab klip. Menambah satu penyedia berarti menyentuh tiga tempat.
// Sekarang: satu tabel di sini, satu halaman setelan, satu pemilih di GUI.
//
// Yang TIDAK ada di tabel ini disengaja — temperature ditentukan tugasnya
// (0,1 untuk mengekstrak fakta, 0,4 untuk memilih momen), max tokens dipatok
// engine sesudah 4096 terbukti memotong JSON di tengah (notes/38), dan nama
// header auth sama untuk semua endpoint OpenAI-compatible. Isian yang tidak
// dibutuhkan hanya menambah cara untuk salah.

// Jenis mesin. Menentukan klien mana yang dipakai, bukan siapa penyedianya.
const (
	kindLocal     = "local"     // server di komputer ini; ditemukan sendiri
	kindAnthropic = "anthropic" // API Claude
	kindOpenAI    = "openai"    // apa pun yang bicara /chat/completions
)

// engineDef = satu mesin yang dikenal aplikasi.
type engineDef struct {
	ID   string
	Name string
	Kind string
	// Path = awalan jalur di bawah BaseURL, hanya untuk kindOpenAI. Gemini
	// memakai "/v1beta/openai", sisanya "/v1".
	Path string
	// Base, Model = nilai bawaan pabrik. Keduanya boleh ditimpa pengguna lewat
	// env, karena penyedia memindahkan endpoint dan menerbitkan model baru
	// jauh lebih sering daripada aplikasi ini dirilis ulang.
	Base  string
	Model string
	// EnvKey/EnvBase/EnvModel = nama variabel di .env.
	EnvKey   string
	EnvBase  string
	EnvModel string
	// KeysURL = halaman tempat pengguna mengambil kuncinya.
	KeysURL string
}

// engineDefs = daftar tertutup. Ditulis di satu tempat supaya menambah penyedia
// berikutnya benar-benar satu baris.
var engineDefs = []engineDef{
	{
		ID: "ollama", Name: "Local AI", Kind: kindLocal,
		EnvBase: "OLLAMA_HOST", EnvKey: "LLM_API_KEY",
	},
	{
		ID: "claude", Name: "Claude", Kind: kindAnthropic,
		Base: "https://api.anthropic.com", Model: "claude-haiku-4-5",
		EnvKey: "ANTHROPIC_API_KEY", EnvBase: "ANTHROPIC_BASE_URL", EnvModel: "ANTHROPIC_MODEL",
		KeysURL: "https://console.anthropic.com/settings/keys",
	},
	{
		ID: "openai", Name: "ChatGPT (OpenAI)", Kind: kindOpenAI, Path: "/v1",
		Base: "https://api.openai.com", Model: "gpt-5.6",
		EnvKey: "OPENAI_API_KEY", EnvBase: "OPENAI_BASE_URL", EnvModel: "OPENAI_MODEL",
		KeysURL: "https://platform.openai.com/api-keys",
	},
	{
		ID: "gemini", Name: "Gemini", Kind: kindOpenAI, Path: "/v1beta/openai",
		Base: "https://generativelanguage.googleapis.com", Model: "gemini-3.7-flash",
		EnvKey: "GEMINI_API_KEY", EnvBase: "GEMINI_BASE_URL", EnvModel: "GEMINI_MODEL",
		KeysURL: "https://aistudio.google.com/apikey",
	},
	{
		ID: "deepseek", Name: "DeepSeek", Kind: kindOpenAI, Path: "/v1",
		Base: "https://api.deepseek.com", Model: "deepseek-v4-pro",
		EnvKey: "DEEPSEEK_API_KEY", EnvBase: "DEEPSEEK_BASE_URL", EnvModel: "DEEPSEEK_MODEL",
		KeysURL: "https://platform.deepseek.com/api_keys",
	},
}

func engineByID(id string) (engineDef, bool) {
	for _, d := range engineDefs {
		if d.ID == id {
			return d, true
		}
	}
	return engineDef{}, false
}

// Engine = satu mesin sebagaimana BERLAKU sekarang: bawaan pabrik yang sudah
// ditimpa isi .env. Inilah yang dikirim ke GUI.
type Engine struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	HasKey  bool   `json:"has_key"`
	// Ready = boleh muncul di pemilih mesin. Mesin cloud tanpa kunci TIDAK
	// muncul — keputusan pemilik proyek; pintunya baris "Add an engine…" yang
	// mengarah ke setelan (notes/39).
	Ready   bool   `json:"ready"`
	KeysURL string `json:"keys_url,omitempty"`
}

// resolve membaca satu mesin beserta timpaan .env-nya.
func resolve(d engineDef) Engine {
	e := Engine{ID: d.ID, Name: d.Name, Kind: d.Kind, BaseURL: d.Base, Model: d.Model, KeysURL: d.KeysURL}
	if v := envOr(d.EnvBase, ""); v != "" {
		e.BaseURL = v
	}
	if v := envOr(d.EnvModel, ""); v != "" {
		e.Model = v
	}
	e.HasKey = engineKey(d) != ""

	if d.Kind == kindLocal {
		// Mesin lokal tidak butuh kunci; yang menentukan siap atau tidak adalah
		// ADA SERVERNYA. Alamat kosong berarti "cari sendiri" — dan penemuan
		// itu mahal, jadi hasilnya dibaca dari cache Discover.
		if e.BaseURL == "" {
			e.BaseURL = ollama.Discover(context.Background())
		}
		e.Ready = e.BaseURL != ""
		return e
	}
	e.Ready = e.HasKey
	return e
}

// engineKey mengambil kunci satu mesin dari lingkungan.
//
// .env adalah SATU-SATUNYA sumber kebenaran, dan itu disengaja: CLI membaca
// berkas yang sama, jadi kunci yang diisi lewat GUI langsung berlaku di sana
// juga. Karena itu setiap penyimpanan kunci wajib os.Setenv, bukan cuma menulis
// berkasnya (lihat setEnv).
func engineKey(d engineDef) string { return envOr(d.EnvKey, "") }

func envOr(name, fallback string) string {
	if name == "" {
		return fallback
	}
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

// Engines melaporkan seluruh mesin beserta keadaannya.
func Engines() []Engine {
	out := make([]Engine, 0, len(engineDefs))
	for _, d := range engineDefs {
		out = append(out, resolve(d))
	}
	return out
}

// EngineFor merakit pemanggil LLM untuk satu mesin.
//
// model kosong = model yang berlaku untuk mesin itu (bawaan pabrik atau
// timpaan pengguna). Tanpa fallback: mesin yang dipilih dipakai apa adanya,
// gagal ya gagal dengan pesan akar masalahnya (notes/12).
func EngineFor(id, model string) (writer.Completer, string, error) {
	if id == "" {
		id = "ollama"
	}
	d, ok := engineByID(id)
	if !ok {
		return nil, "", fmt.Errorf("unknown engine %q — choose one of: %s", id, engineIDs())
	}
	e := resolve(d)
	if model = strings.TrimSpace(model); model == "" {
		model = e.Model
	}
	key := engineKey(d)

	switch d.Kind {
	case kindAnthropic:
		if key == "" {
			return nil, "", fmt.Errorf("%s has no API key yet — add it on the Requirements page", d.Name)
		}
		c := llm.New(key, model)
		c.BaseURL = e.BaseURL
		return func(ctx context.Context, system, user string, _ any) (string, error) {
			// Claude tidak menerima skema: bentuk balasannya diminta lewat prompt.
			return c.Complete(ctx, system, user, llmMaxTokens)
		}, d.Name + " (" + c.Model + ")", nil

	case kindOpenAI:
		if key == "" {
			return nil, "", fmt.Errorf("%s has no API key yet — add it on the Requirements page", d.Name)
		}
		c := ollama.New(e.BaseURL, model)
		c.Kind, c.Path, c.APIKey = ollama.KindOpenAI, d.Path, key
		return func(ctx context.Context, system, user string, schema any) (string, error) {
			return c.Complete(ctx, system, user, schema, llmMaxTokens)
		}, d.Name + " (" + c.Model + ")", nil

	default: // kindLocal
		c := ollama.New(e.BaseURL, model)
		// Konteks maksimum model ditanyakan sekali di sini, bukan per panggilan:
		// tanpa itu jendelanya terkunci di 8192 dan permintaan keluaran besar —
		// artikel utuh — terpotong di tengah JSON tanpa pesan galat apa pun.
		c.NumCtx = ollama.ContextOf(context.Background(), c.URL, c.Model)
		return func(ctx context.Context, system, user string, schema any) (string, error) {
			return c.Complete(ctx, system, user, schema, llmMaxTokens)
		}, "Ollama (" + c.Model + ")", nil
	}
}

func engineIDs() string {
	ids := make([]string, 0, len(engineDefs))
	for _, d := range engineDefs {
		ids = append(ids, d.ID)
	}
	sort.Strings(ids)
	return strings.Join(ids, ", ")
}

// --- HTTP ---

func (s *Server) listEngines(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"engines": Engines()})
}

// engineModels menjawab daftar model satu mesin.
//
// Dipisah dari /test karena harganya beda jauh: ini satu GET, sedangkan /test
// memanggil LLM sungguhan. Pemilih model di GUI memanggil yang ini tiap kali
// mesinnya diganti — itu tidak boleh berbiaya token.
//
// Balasan kosong BUKAN galat: Claude tidak menyediakan daftar lewat jalur ini,
// dan server lokal yang sedang mati juga tidak. Kotak modelnya tetap bisa
// diketik (notes/39).
func (s *Server) engineModels(w http.ResponseWriter, r *http.Request) {
	d, ok := engineByID(r.PathValue("id"))
	if !ok {
		writeErr(w, 400, "unknown engine "+r.PathValue("id"))
		return
	}
	e := resolve(d)
	var names []string
	if d.Kind != kindAnthropic && e.BaseURL != "" {
		for _, m := range ollama.Models(r.Context(), e.BaseURL, d.Path, engineKey(d)) {
			names = append(names, m.Name)
		}
	}
	writeJSON(w, 200, map[string]any{"models": names})
}

// saveEngine menyimpan kunci/alamat/model satu mesin ke .env.
//
// Field yang TIDAK dikirim tidak disentuh, dan string kosong berarti "kembali
// ke bawaan" — bukan "simpan kosong". Tanpa pembedaan itu, menyimpan model saja
// akan menghapus kunci yang sudah ada.
func (s *Server) saveEngine(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID      string  `json:"id"`
		APIKey  *string `json:"api_key"`
		BaseURL *string `json:"base_url"`
		Model   *string `json:"model"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	d, ok := engineByID(req.ID)
	if !ok {
		writeErr(w, 400, "unknown engine "+req.ID)
		return
	}
	if req.APIKey != nil {
		key := strings.TrimSpace(*req.APIKey)
		s.setEnv(d.EnvKey, key)
		if d.Kind == kindAnthropic {
			// Manager menyimpan kunci untuk job klip yang sudah berjalan.
			s.mgr.SetAPIKey(key)
		}
	}
	if req.BaseURL != nil {
		s.setEnv(d.EnvBase, normalizeBase(*req.BaseURL))
		if d.Kind == kindLocal {
			// Alamat kosong berarti kembali ke penemuan otomatis, dan itu harus
			// terasa SEKARANG, bukan setelah aplikasi dijalankan lagi.
			ollama.ResetCache()
		}
	}
	if req.Model != nil {
		s.setEnv(d.EnvModel, strings.TrimSpace(*req.Model))
	}
	writeJSON(w, 200, resolve(d))
}

// setEnv menulis ke proses ini DAN ke .env. Keduanya perlu: yang pertama supaya
// perubahannya berlaku tanpa dijalankan ulang, yang kedua supaya ia bertahan —
// dan supaya CLI, yang membaca .env yang sama, ikut kebagian.
func (s *Server) setEnv(name, value string) {
	if name == "" {
		return
	}
	_ = os.Setenv(name, value)
	_ = writeEnvKey(s.paths.EnvFile, name, value)
}

func normalizeBase(v string) string {
	v = strings.TrimRight(strings.TrimSpace(v), "/")
	if v != "" && !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
		v = "http://" + v
	}
	return v
}

// EngineTest = hasil uji satu mesin.
//
// Bukan sekadar lampu hijau. Tiap hal yang bisa DITANYAKAN ke servernya
// menghapus satu isian yang harus diketik pengguna (notes/39):
//
//   - Models  → pengguna memilih dari daftar, bukan mengetik dari ingatan;
//   - Schema  → apakah penyedia menghormati balasan berskema. Ini yang menopang
//     seluruh pagar fakta pembuat berita, dan penyedia yang membandel gejalanya
//     BUKAN galat melainkan job gagal parsing di tengah jalan.
type EngineTest struct {
	OK     bool     `json:"ok"`
	Engine string   `json:"engine,omitempty"`
	Models []string `json:"models,omitempty"`
	// Schema = balasannya BERBENTUK seperti yang diminta.
	Schema bool `json:"schema"`
	// Strict = servernya benar-benar MEMAKSAKAN bentuk itu. Keduanya dipisah
	// karena model bisa saja menuruti prompt pada balasan pendek lalu melenceng
	// pada yang panjang — dan yang panjang itulah tahap menulis.
	Strict bool   `json:"strict"`
	Reply  string `json:"reply,omitempty"`
	Error  string `json:"error,omitempty"`
}

// schemaProbe = skema balasan sekecil mungkin untuk menguji ketaatan penyedia.
var schemaProbe = map[string]any{
	"type":       "object",
	"properties": map[string]any{"ok": map[string]any{"type": "boolean"}},
	"required":   []string{"ok"},
}

func (s *Server) testEngine(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID    string `json:"id"`
		Model string `json:"model"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	d, ok := engineByID(req.ID)
	if !ok {
		writeErr(w, 400, "unknown engine "+req.ID)
		return
	}
	e := resolve(d)

	out := EngineTest{}
	if d.Kind != kindAnthropic && e.BaseURL != "" {
		for _, m := range ollama.Models(r.Context(), e.BaseURL, d.Path, engineKey(d)) {
			out.Models = append(out.Models, m.Name)
		}
	}

	// Mesin lokal diuji dengan uji DUA TAHAP yang sama persis dengan yang
	// dipakai job (koreksi transkrip + pemilihan momen), bukan sapaan "ok".
	//
	// Alasannya temuan lama: berkali-kali di komputer baru Ollama jalan, model
	// terpasang, sapaan berhasil — dan job klip tetap berhenti. Sapaan menguji
	// bahwa servernya menjawab, bukan bahwa modelnya sanggup.
	if d.Kind == kindLocal {
		model := strings.TrimSpace(req.Model)
		if model == "" {
			model = e.Model
		}
		name, steps := pipeline.SelfTest(r.Context(), e.BaseURL, model)
		out.Engine = name
		out.OK, out.Strict, out.Schema = true, true, true
		for _, st := range steps {
			if !st.OK {
				out.OK, out.Strict, out.Schema = false, false, false
				out.Error = st.Name + ": " + st.Error
				break
			}
		}
		writeJSON(w, 200, out)
		return
	}

	complete, name, err := EngineFor(req.ID, req.Model)
	if err != nil {
		out.Error = err.Error()
		writeJSON(w, 200, out)
		return
	}
	out.Engine = name

	reply, err := complete(r.Context(), `Reply with JSON only: {"ok": true}`, "ping", schemaProbe)
	if err != nil {
		out.Error = err.Error()
		writeJSON(w, 200, out)
		return
	}
	out.OK = true
	out.Reply = truncateReply(reply)
	out.Schema = strings.Contains(strings.ReplaceAll(reply, " ", ""), `"ok":true`)
	out.Strict = d.Kind != kindOpenAI || ollama.SchemaEnforced(e.BaseURL)
	writeJSON(w, 200, out)
}

func truncateReply(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}

// llmEngine merakit pemanggil untuk tab kartu berita, dengan skema pemilihan
// paragraf yang sudah dipaku.
func (s *Server) llmEngine(id, model string) (news.Completer, string, error) {
	llmFn, name, err := EngineFor(id, model)
	if err != nil {
		return nil, "", err
	}
	return func(ctx context.Context, system, user string) (string, error) {
		return llmFn(ctx, system, user, news.SelectionSchema())
	}, name, nil
}

// fillEngine menerjemahkan mesin yang dipilih pengguna jadi koordinat yang
// dipahami pipeline klip.
//
// "ollama" dan "heuristic" dibiarkan apa adanya: keduanya sudah punya jalannya
// sendiri di pipeline (penemuan server lokal, dan tanpa LLM sama sekali).
func fillEngine(o *config.Options) {
	d, ok := engineByID(o.Provider)
	if !ok || d.Kind == kindLocal {
		return
	}
	e := resolve(d)
	o.EngineName = d.Name + " (" + firstNonEmpty(o.LLMModel, e.Model) + ")"
	o.LLMBase, o.LLMPath, o.LLMKeyEnv = e.BaseURL, d.Path, d.EnvKey
	if o.LLMModel == "" {
		o.LLMModel = e.Model
	}
	// Claude punya klien sendiri di pipeline; yang perlu diteruskan hanya
	// alamatnya, sebab kuncinya sudah dibaca lewat Paths.APIKey.
	if d.Kind == kindAnthropic {
		o.Provider = "claude"
	}
}
