// Package ollama memakai LLM lokal via Ollama (mis. Qwen) untuk scoring offline.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/score/llm"
	"github.com/gemgum/clipper/engine/internal/types"
)

const defaultURL = "http://localhost:11434"

// numCtx = jendela konteks yang diminta engine per potongan transkrip. Dipakai
// juga saat menilai model terpasang: model dengan konteks maksimum di bawah ini
// akan memotong prompt diam-diam.
const numCtx = 8192

// minParams = batas bawah ukuran model (miliar parameter) yang masih sanggup
// mengerjakan prompt pemilihan momen. Dari uji di notes/12: model 4B membalas
// JSON valid tapi isian kosong, sedangkan 7B mengerjakannya dengan benar.
const minParams = 6.5

// Client memanggil Ollama.
type Client struct {
	URL   string
	Model string
	HTTP  *http.Client
	// Temperature menimpa nilai bawaan bila > 0. Tugas menyalin-ulang seperti
	// koreksi transkrip butuh angka jauh lebih rendah daripada tugas kreatif
	// seperti memilih momen — pada 0.4 keluarannya berbeda-beda tiap kali.
	Temperature float64
}

// defaultTemperature dipakai bila Client.Temperature tidak diisi.
const defaultTemperature = 0.4

func New(url, model string) *Client {
	if url == "" {
		url = defaultURL
	}
	if model == "" {
		model = "qwen2.5"
	}
	return &Client{URL: strings.TrimRight(url, "/"), Model: model, HTTP: &http.Client{Timeout: 5 * time.Minute}}
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type chatReq struct {
	Model    string         `json:"model"`
	Messages []chatMsg      `json:"messages"`
	Stream   bool           `json:"stream"`
	Format   any            `json:"format,omitempty"` // "json" atau JSON Schema
	Options  map[string]any `json:"options,omitempty"`
}
type chatResp struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Error string `json:"error"`
}

// SelectMoments meminta model lokal memilih momen dari satu potongan transkrip.
// Prompt-nya SAMA DETAILNYA dengan Claude: bentuk balasan dijamin oleh JSON
// Schema di parameter "format", sehingga prompt panjang tidak lagi membuat
// model lokal membalas JSON rusak (masalah lama yang memaksa prompt disederhanakan).
func (c *Client) SelectMoments(ctx context.Context, tr types.Transcript, maxClips int, targetMin, targetMax float64, ch llm.Chunk) ([]llm.Moment, error) {
	content, err := c.Complete(ctx, llm.SystemPrompt(targetMin, targetMax, ch, tr.Language), llm.UserPrompt(tr, maxClips),
		llm.ResponseSchema(), 3072)
	if err != nil {
		return nil, err
	}
	var wrap llm.MomentsWrapper
	if err := json.Unmarshal([]byte(content), &wrap); err != nil {
		return nil, fmt.Errorf("local model %s returned JSON that could not be read: %w — reply: %s",
			c.Model, err, trunc(content, 300))
	}
	return wrap.Moments, nil
}

// Complete mengirim satu pasang prompt ke Ollama dan mengembalikan isi balasan.
//
// schema (boleh nil) diteruskan ke parameter "format" Ollama sehingga bentuk
// balasan dijamin di sisi server — inilah yang membuat model lokal tetap
// membalas JSON rapi walau promptnya panjang.
func (c *Client) Complete(ctx context.Context, system, user string, schema any, numPredict int) (string, error) {
	if numPredict <= 0 {
		numPredict = 2048
	}
	temperature := c.Temperature
	if temperature <= 0 {
		temperature = defaultTemperature
	}
	reqBody := chatReq{
		Model: c.Model,
		Messages: []chatMsg{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Stream:  false,
		Format:  schema,
		Options: map[string]any{"temperature": temperature, "num_ctx": numCtx, "num_predict": numPredict},
	}
	buf, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/api/chat", bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ollama is unreachable at %s — make sure it is installed and run `ollama serve`: %w", c.URL, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var cr chatResp
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("the Ollama response could not be read (status %d): %s", resp.StatusCode, trunc(string(raw), 200))
	}
	if cr.Error != "" {
		return "", ollamaError(c.Model, cr.Error)
	}
	return cr.Message.Content, nil
}

// ollamaError menerjemahkan pesan Ollama jadi petunjuk yang bisa ditindaklanjuti.
func ollamaError(model, msg string) error {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "not found") || strings.Contains(low, "no such model"):
		return fmt.Errorf("model %q is not installed in Ollama — click \"download model\" in the GUI or run `ollama pull %s`", model, model)
	case strings.Contains(low, "format"):
		return fmt.Errorf("Ollama rejected the JSON schema — update Ollama to a version that supports structured output (%s)", msg)
	case strings.Contains(low, "memory") || strings.Contains(low, "out of"):
		return fmt.Errorf("not enough RAM/VRAM for model %q — use a smaller model (%s)", model, msg)
	}
	return fmt.Errorf("Ollama failed to process the request: %s", msg)
}

func trunc(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ModelInfo satu model terpasang di Ollama, sudah dinilai kelayakannya untuk
// tugas pemilihan momen — supaya GUI tinggal menampilkan, bukan menebak.
type ModelInfo struct {
	Name    string `json:"name"`    // nama lengkap dengan tag, mis. "qwen2.5:latest"
	Base    string `json:"base"`    // nama tanpa tag, mis. "qwen2.5"
	Params  string `json:"params"`  // mis. "7.6B" (apa adanya dari Ollama)
	Quant   string `json:"quant"`   // mis. "Q4_K_M"
	Bytes   int64  `json:"bytes"`   // ukuran berkas model
	Context int    `json:"context"` // konteks maksimum yang didukung model
	Ready   bool   `json:"ready"`   // sanggup mengerjakan prompt pemilihan momen
	Note    string `json:"note"`    // alasan bila tidak siap
}

// StatusInfo hasil pengecekan Ollama.
type StatusInfo struct {
	Running bool `json:"running"`
	// Models = daftar nama lengkap; dipertahankan agar pemakai lama tetap jalan.
	Models    []string    `json:"models"`
	Installed []ModelInfo `json:"installed"`
}

// Status memeriksa apakah Ollama jalan & daftar model terpasang, lengkap dengan
// penilaian siap/tidaknya tiap model.
func Status(ctx context.Context, url string) StatusInfo {
	if url == "" {
		url = defaultURL
	}
	url = strings.TrimRight(url, "/")
	cl := &http.Client{Timeout: 3 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url+"/api/tags", nil)
	resp, err := cl.Do(req)
	if err != nil {
		return StatusInfo{Running: false}
	}
	defer resp.Body.Close()
	var parsed struct {
		Models []struct {
			Name    string `json:"name"`
			Size    int64  `json:"size"`
			Details struct {
				ParameterSize     string `json:"parameter_size"`
				QuantizationLevel string `json:"quantization_level"`
				ContextLength     int    `json:"context_length"`
			} `json:"details"`
			Capabilities []string `json:"capabilities"`
		} `json:"models"`
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(raw, &parsed)
	info := StatusInfo{Running: true}
	for _, m := range parsed.Models {
		mi := ModelInfo{
			Name:    m.Name,
			Base:    BaseName(m.Name),
			Params:  m.Details.ParameterSize,
			Quant:   m.Details.QuantizationLevel,
			Bytes:   m.Size,
			Context: m.Details.ContextLength,
		}
		mi.Ready, mi.Note = judge(m.Details.ParameterSize, m.Details.ContextLength, m.Capabilities)
		info.Models = append(info.Models, m.Name)
		info.Installed = append(info.Installed, mi)
	}
	return info
}

// BaseName membuang tag Ollama: "qwen2.5:latest" → "qwen2.5".
func BaseName(name string) string {
	if i := strings.IndexByte(name, ':'); i > 0 {
		return name[:i]
	}
	return name
}

// judge menilai apakah sebuah model terpasang layak dipakai memilih momen.
func judge(paramSize string, ctxLen int, caps []string) (bool, string) {
	if len(caps) > 0 && !slices.Contains(caps, "completion") {
		return false, "this model does not generate text (e.g. an embedding model)"
	}
	if ctxLen > 0 && ctxLen < numCtx {
		return false, fmt.Sprintf("maximum context is %d tokens, below the %d a transcript chunk needs", ctxLen, numCtx)
	}
	if b := parseParams(paramSize); b > 0 && b < minParams {
		return false, fmt.Sprintf("only %s parameters — in testing, models below %.0fB return empty fields for this prompt", paramSize, minParams)
	}
	return true, ""
}

// parseParams mengubah "7.6B" → 7.6 dan "270M" → 0.27. Bentuk MoE seperti
// "8x7B" diambil angka setelah "x" (yang aktif per token). 0 = tak diketahui.
func parseParams(s string) float64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	if i := strings.LastIndexByte(s, 'X'); i >= 0 {
		s = s[i+1:]
	}
	mult := 1.0
	switch {
	case strings.HasSuffix(s, "B"):
		s = strings.TrimSuffix(s, "B")
	case strings.HasSuffix(s, "M"):
		s, mult = strings.TrimSuffix(s, "M"), 0.001
	default:
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v * mult
}

// Pull mengunduh model lewat Ollama (bisa lama). Mengembalikan error bila gagal.
func Pull(ctx context.Context, url, model string) error {
	if url == "" {
		url = defaultURL
	}
	url = strings.TrimRight(url, "/")
	body, _ := json.Marshal(map[string]interface{}{"name": model, "stream": false})
	cl := &http.Client{Timeout: 60 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url+"/api/pull", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/json")
	resp, err := cl.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama is unreachable (download failed): %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var r struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	_ = json.Unmarshal(raw, &r)
	if r.Error != "" {
		return fmt.Errorf("model download failed: %s", r.Error)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("model download failed (status %d)", resp.StatusCode)
	}
	return nil
}
