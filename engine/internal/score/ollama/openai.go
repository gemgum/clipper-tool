package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Server LLM lokal SELAIN Ollama.
//
// llama.cpp (server), LocalAI, llamafile, vLLM, Aphrodite, LiteLLM, Exo — semua
// menyediakan API yang sama: `/v1/chat/completions` bergaya OpenAI. Jadi yang
// dibutuhkan bukan tujuh integrasi, melainkan SATU: satu bentuk permintaan yang
// sudah disepakati semuanya, plus satu cara mengenali mana yang sedang jalan.
//
// Ollama tetap dipakai lewat API-nya sendiri (`/api/chat`), bukan lewat lapisan
// /v1-nya: hanya di sana bentuk balasan bisa dipaksa dengan JSON Schema penuh,
// dan justru itu yang membuat model lokal tetap membalas JSON rapi.

// Jenis server yang dikenali.
const (
	KindOllama = "ollama"
	KindOpenAI = "openai" // apa pun yang bicara /v1/chat/completions
)

// Endpoint = satu server LLM lokal yang menjawab.
type Endpoint struct {
	URL  string `json:"url"`
	Kind string `json:"kind"`
	// Name = nama yang dikenali pengguna ("Ollama", "LM Studio"). Ikut di sini
	// supaya daftar pilihan di GUI tidak perlu menerjemahkan port jadi nama
	// sendiri — dua tempat yang menebak hal yang sama pasti akan berbeda.
	Name string `json:"name"`
}

// openAIReq/openAIResp: bagian yang benar-benar dipakai saja. Server-server ini
// berbeda-beda di bagian lain (usage, logprobs, kolom tambahan), dan menuliskan
// semuanya berarti satu server yang menambah kolom bisa mematahkan parsing.
type openAIReq struct {
	Model          string         `json:"model"`
	Messages       []chatMsg      `json:"messages"`
	Temperature    float64        `json:"temperature"`
	MaxTokens      int            `json:"max_tokens,omitempty"`
	ResponseFormat map[string]any `json:"response_format,omitempty"`
}

type openAIResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		// FinishReason membedakan "modelnya memang tidak menjawab" dari
		// "jatahnya habis" — dan tanpa itu keduanya sampai ke pemanggil sebagai
		// balasan kosong yang sama.
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error any `json:"error"`
}

// completeOpenAI mengirim satu pasang prompt ke server bergaya OpenAI.
func (c *Client) completeOpenAI(ctx context.Context, system, user string, schema any, numPredict int) (string, error) {
	// Dua kelonggaran, dan keduanya harus bisa BERLAKU BERSAMA. Ditulis sebagai
	// dua penambalan berurutan, DeepSeek mematahkannya: ia menolak skema dulu,
	// lalu percobaan tanpa skema kehabisan jatah — dan penanganan jatah tidak
	// pernah terpakai karena percobaan itu sudah keburu dikembalikan.
	//
	// Yang dilonggarkan cuma CARA MEMINTA; mesin yang dipilih pengguna tetap
	// dipakai apa adanya (notes/12). Keduanya juga terbaca: tombol Test
	// melaporkan skema yang tidak dipaksakan.
	if schema != nil {
		if _, refuses := noSchema.Load(c.URL); refuses {
			schema = nil
		}
	}
	budget := numPredict
	if _, roomy := needsRoom.Load(c.roomKey()); roomy {
		budget = withRoom(numPredict)
	}

	// Paling banyak tiga percobaan: sekali untuk tiap kelonggaran, plus yang
	// pertama. Batasnya ada supaya server yang selalu menolak tidak dicoba
	// selamanya.
	for i := 0; i < 3; i++ {
		out, err := c.postChat(ctx, system, user, schema, budget)
		if err == nil {
			return out, nil
		}
		switch {
		case schema != nil && isSchemaRefusal(err):
			// DeepSeek membalas "This response_format type is unavailable now"
			// dan MENOLAK seluruh permintaan. Dugaan lama — "server yang tidak
			// sanggup akan mengabaikan field ini" — ternyata salah, dan
			// akibatnya bukan balasan longgar melainkan job yang tidak jalan.
			noSchema.Store(c.URL, true)
			schema = nil
		case budget == numPredict && isBudgetExhausted(err):
			// Model bernalar memakai jatah yang sama untuk berpikir dan
			// menjawab; pada masukan panjang ia habis sebelum satu huruf
			// jawaban keluar.
			needsRoom.Store(c.roomKey(), true)
			budget = withRoom(numPredict)
		default:
			return "", err
		}
	}
	return "", fmt.Errorf("%s kept refusing the request even after loosening the reply format and the token budget (model %s)", c.URL, c.Model)
}

// noSchema mengingat server yang menolak json_schema, per alamat.
var noSchema sync.Map

// needsRoom mengingat model yang jatahnya perlu dilipatkan, per alamat+model.
// Per MODEL, bukan per alamat: satu penyedia bisa menyajikan model bernalar dan
// model biasa sekaligus.
var needsRoom sync.Map

func (c *Client) roomKey() string { return c.URL + "|" + c.Model }

// roomFactor & roomCap: seberapa banyak ruang tambahan diberikan.
//
// Empat kali cukup untuk penalaran atas satu artikel berita; batas atasnya ada
// supaya angka masukan yang besar tidak berlipat jadi tagihan yang mengagetkan.
const (
	roomFactor = 4
	roomCap    = 32768
)

func withRoom(n int) int { return min(n*roomFactor, roomCap) }

// isBudgetExhausted mengenali balasan yang habis jatah SEBELUM menjawab.
func isBudgetExhausted(err error) bool {
	return strings.Contains(err.Error(), "before answering")
}

// SchemaEnforced melaporkan apakah server di alamat itu MENERIMA balasan
// berskema.
//
// Bedanya dengan "balasannya kebetulan berbentuk benar" nyata: tanpa skema,
// bentuk balasan cuma dijaga prompt, dan itu paling sering patah justru pada
// balasan panjang — persis yang diminta tahap menulis pembuat berita.
func SchemaEnforced(url string) bool {
	_, refuses := noSchema.Load(strings.TrimRight(url, "/"))
	return !refuses
}

// isSchemaRefusal membedakan penolakan KARENA response_format dari penolakan
// lain — kunci salah, model tidak ada, kuota habis. Menyamakan semuanya berarti
// mencoba ulang permintaan yang memang mustahil.
func isSchemaRefusal(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "response_format") || strings.Contains(msg, "json_schema")
}

// postChat mengirim satu permintaan chat.
func (c *Client) postChat(ctx context.Context, system, user string, schema any, numPredict int) (string, error) {
	req := openAIReq{
		Model: c.Model,
		Messages: []chatMsg{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: c.temperature(),
		MaxTokens:   numPredict,
	}
	if schema != nil {
		// json_object tidak dipakai sebagai cadangan otomatis: sebagian server
		// MENOLAK permintaan yang memuat keduanya, dan promptnya sendiri sudah
		// meminta JSON.
		req.ResponseFormat = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "clipper",
				"schema": schema,
				"strict": true,
			},
		}
	}
	body, _ := json.Marshal(req)
	url := c.URL + c.path() + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("content-type", "application/json")
	// Sebagian server (LiteLLM, vLLM di belakang gateway) mensyaratkan header
	// ini walau kuncinya tidak diperiksa. Isinya dari LLM_API_KEY bila diisi.
	httpReq.Header.Set("authorization", "Bearer "+c.key())

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return "", dialError(c.URL, c.Model, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var parsed openAIResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		// Alamat LENGKAP, bukan cuma base-nya: alamat yang salah isi (mis.
		// endpoint gaya Anthropic milik DeepSeek) membalas 404 berbadan kosong,
		// dan tanpa jalur penuh pesan itu tidak menunjukkan apa pun.
		return "", fmt.Errorf("the reply from %s could not be read (status %d): %s", url, resp.StatusCode, trunc(string(raw), 200))
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("%s refused the request: %s", url, trunc(errorMessage(parsed.Error), 200))
	}
	if len(parsed.Choices) == 0 {
		return "", nil // balasan kosong: pemanggil yang memutuskan artinya
	}
	out := parsed.Choices[0].Message.Content
	if parsed.Choices[0].FinishReason == "length" {
		// Jatah habis. Dilaporkan sebagai galat WALAU sebagian isinya sudah
		// keluar: yang diminta selalu JSON, dan JSON yang terpotong di tengah
		// sama tidak bergunanya dengan balasan kosong — bedanya cuma ia gagal
		// beberapa lapis kemudian, dengan pesan yang tidak menyebut sebabnya.
		//
		// Model BERNALAR (DeepSeek v4-pro, o-series, R1) memakai jatah yang sama
		// untuk berpikir dan untuk menjawab, jadi pada masukan panjang jatahnya
		// bisa habis sebelum satu huruf jawaban keluar.
		return "", fmt.Errorf("%s spent its whole %d-token budget before answering (model %s). Reasoning models need more room: pick a non-reasoning model, or send less text at a time",
			c.URL, numPredict, c.Model)
	}
	return out, nil
}

// errorMessage mengambil kalimat yang bisa dibaca dari objek galat penyedia.
//
// Bentuknya `{"error": {"message": "...", "code": ...}}` di hampir semua
// server. Dicetak apa adanya, ia jadi sintaks map Go ("map[code:... message:...]")
// — dan yang membaca pesan itu adalah pengguna, bukan pemrogram.
func errorMessage(v any) string {
	if m, ok := v.(map[string]any); ok {
		if s, ok := m["message"].(string); ok && s != "" {
			return s
		}
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// openAIModels membaca daftar model dari /v1/models.
//
// Bentuk minimum yang dijamin standar cuma `data[].id` — nama, tanpa spesifikasi
// apa pun. Tapi sebagian server memberi lebih: llama.cpp menyertakan `meta`
// berisi jumlah parameter, panjang konteks, ukuran, dan kuantisasi yang
// SEBENARNYA (terukur 6 Agustus 2026 pada llama-server b10295). Yang ada dipakai
// apa adanya, dan nama cuma jadi cadangan bila `meta` tidak dikirim.
//
// `n_ctx` yang paling penting dan bukan sekadar hiasan: resolveOllama
// memakainya sebagai Client.NumCtx, dan nilai 0 membuat koreksi transkrip
// menebak sendiri seberapa besar potongan yang muat.
func openAIModels(ctx context.Context, url string) []ModelInfo {
	return openAIModelsAt(ctx, url, "/v1", "")
}

// Models membaca daftar model satu server OpenAI-compatible, dengan jalur dan
// kunci yang ditentukan pemanggil. Diekspor karena halaman setelan memakainya
// untuk mengisi sendiri pilihan model tiap penyedia — supaya nama model tidak
// perlu diketik dari ingatan (notes/39).
func Models(ctx context.Context, url, path, key string) []ModelInfo {
	if path == "" {
		path = "/v1"
	}
	return openAIModelsAt(ctx, url, path, key)
}

// openAIModelsAt sama, dengan jalur dan kunci yang ditentukan pemanggil —
// itulah yang dibutuhkan penyedia cloud.
func openAIModelsAt(ctx context.Context, url, path, key string) []ModelInfo {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+path+"/models", nil)
	if err != nil {
		return nil
	}
	if key == "" {
		key = apiKey()
	}
	req.Header.Set("authorization", "Bearer "+key)
	resp, err := (&http.Client{Timeout: probeTimeout}).Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var parsed struct {
		Data []struct {
			ID   string `json:"id"`
			Meta struct {
				NParams int64  `json:"n_params"`
				NCtx    int    `json:"n_ctx"`
				Size    int64  `json:"size"`
				FType   string `json:"ftype"`
			} `json:"meta"`
		} `json:"data"`
	}
	raw, _ := io.ReadAll(resp.Body)
	if json.Unmarshal(raw, &parsed) != nil {
		return nil
	}
	out := make([]ModelInfo, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if strings.TrimSpace(m.ID) == "" {
			continue
		}
		// Base = nama yang layak dilihat. Perlu karena llama.cpp memakai PATH
		// LENGKAP berkas .gguf sebagai id, dan path 60 karakter di kotak pilihan
		// tidak memberitahu apa pun selain melebarkan kolomnya.
		mi := ModelInfo{Name: m.ID, Base: shortModelName(m.ID), Ready: true,
			Note:    "served by an OpenAI-compatible server",
			Bytes:   m.Meta.Size,
			Context: m.Meta.NCtx,
			Quant:   m.Meta.FType,
		}
		// Ukuran dari metadata selalu menang atas tebakan dari nama.
		params := float64(m.Meta.NParams) / 1e9
		if params == 0 {
			params = paramsFromName(m.ID)
		}
		// Penilaiannya cuma memberi catatan, tidak pernah menolak: salah baca
		// ukuran tidak boleh membuat model yang sebenarnya baik jadi tidak bisa
		// dipilih.
		if params > 0 {
			mi.Params = formatParams(params)
			if params < minParams {
				mi.Note = fmt.Sprintf("%s model — fine for transcript correction, but small models often return empty fields when picking moments", mi.Params)
			}
		}
		out = append(out, mi)
	}
	return out
}

// shortModelName memangkas id menjadi nama yang layak dibaca:
// "/home/x/models/Qwen2.5-3B-Instruct-Q4_K_M.gguf" → "Qwen2.5-3B-Instruct-Q4_K_M".
// Id yang memang sudah pendek dikembalikan apa adanya.
func shortModelName(id string) string {
	s := id
	if i := strings.LastIndexAny(s, `/\`); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(s, ".gguf")
	if s == "" {
		return BaseName(id)
	}
	return s
}

// paramsFromName membaca ukuran model dari namanya: "Qwen2.5-1.5B-Instruct" →
// 1.5, "SmolLM2-360M" → 0.36, "Llama-3.2-3b" → 3. 0 = tidak ketemu.
//
// Hanya untuk server yang TIDAK memberi metadata (semua yang bergaya OpenAI).
// Ollama menyebutkan ukurannya sendiri, dan angka dari sana selalu menang.
func paramsFromName(name string) float64 {
	m := paramInName.FindStringSubmatch(name)
	if m == nil {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	if strings.EqualFold(m[2], "M") {
		return v / 1000
	}
	return v
}

// Angka + B/M yang berdiri sebagai potongan namanya sendiri: "-1.5b-", "_360m".
// Tanpa pembatas itu, "3.2" di "Llama-3.2" ikut terbaca sebagai 3,2 miliar.
var paramInName = regexp.MustCompile(`(?i)[-_.:/ ]([0-9]+(?:\.[0-9]+)?)\s*([bm])(?:[-_.:/ ]|$)`)

// formatParams mengubah 1.5 → "1.5B" dan 0.36 → "360M".
func formatParams(b float64) string {
	if b < 1 {
		return fmt.Sprintf("%.0fM", b*1000)
	}
	return strings.TrimSuffix(fmt.Sprintf("%.1f", b), ".0") + "B"
}
