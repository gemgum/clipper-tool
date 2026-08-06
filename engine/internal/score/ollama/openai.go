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
	} `json:"choices"`
	Error any `json:"error"`
}

// completeOpenAI mengirim satu pasang prompt ke server bergaya OpenAI.
func (c *Client) completeOpenAI(ctx context.Context, system, user string, schema any, numPredict int) (string, error) {
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
		// json_schema kalau servernya sanggup; kalau tidak, ia mengabaikan
		// field ini dan promptnya yang menjaga bentuk balasan. json_object
		// tidak dipakai sebagai cadangan otomatis karena sebagian server
		// MENOLAK permintaan yang memuat keduanya.
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
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.URL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("content-type", "application/json")
	// Sebagian server (LiteLLM, vLLM di belakang gateway) mensyaratkan header
	// ini walau kuncinya tidak diperiksa. Isinya dari LLM_API_KEY bila diisi.
	httpReq.Header.Set("authorization", "Bearer "+apiKey())

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return "", dialError(c.URL, c.Model, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var parsed openAIResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("the reply from %s could not be read (status %d): %s", c.URL, resp.StatusCode, trunc(string(raw), 200))
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("%s refused the request: %s", c.URL, trunc(fmt.Sprint(parsed.Error), 200))
	}
	if len(parsed.Choices) == 0 {
		return "", nil // balasan kosong: pemanggil yang memutuskan artinya
	}
	return parsed.Choices[0].Message.Content, nil
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"/v1/models", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("authorization", "Bearer "+apiKey())
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
