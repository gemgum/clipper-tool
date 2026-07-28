// Package ollama memakai LLM lokal via Ollama (mis. Qwen) untuk scoring offline.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/score/llm"
	"github.com/gemgum/clipper/engine/internal/types"
)

const defaultURL = "http://localhost:11434"

// Client memanggil Ollama.
type Client struct {
	URL   string
	Model string
	HTTP  *http.Client
}

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
	Model    string                 `json:"model"`
	Messages []chatMsg              `json:"messages"`
	Stream   bool                   `json:"stream"`
	Format   string                 `json:"format,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
}
type chatResp struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Error string `json:"error"`
}

// Prompt SEDERHANA (tanpa reasons/hashtags): model lokal lemah cenderung
// kebablasan/mengulang bila skema terlalu rumit. Cukup start/end/score/title.
const selectSystem = `Kamu kurator klip video viral konten Indonesia. Dari transkrip bertimestamp, PILIH momen terbaik untuk klip pendek.
Aturan: tiap momen berdiri sendiri (hook + isi + penutup, bukan potongan acak); tentukan 'start' & 'end' (detik) dari timestamp; durasi sekitar %.0f-%.0f detik (boleh menyimpang demi momen utuh).
Balas HANYA JSON objek ini, tanpa teks lain:
{"moments":[{"start":<detik>,"end":<detik>,"score":<0-100>,"title":"<judul singkat Indonesia>"}]}`

// SelectMoments meminta model lokal memilih momen dari transkrip.
func (c *Client) SelectMoments(ctx context.Context, tr types.Transcript, maxClips int, targetMin, targetMax float64) ([]llm.Moment, error) {
	var b strings.Builder
	for _, s := range tr.Segments {
		fmt.Fprintf(&b, "[%.1f-%.1f] %s\n", s.Start, s.End, s.Text)
	}
	system := fmt.Sprintf(selectSystem, targetMin, targetMax)
	user := fmt.Sprintf("Transkrip:\n%s\nPilih maksimal %d momen terbaik.", b.String(), maxClips)

	reqBody := chatReq{
		Model:    c.Model,
		Messages: []chatMsg{{Role: "system", Content: system}, {Role: "user", Content: user}},
		Stream:   false,
		Format:   "json",
		Options:  map[string]interface{}{"temperature": 0.4, "num_ctx": 8192, "num_predict": 1536},
	}
	buf, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/api/chat", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama tidak terjangkau di %s (jalan? `ollama serve`): %w", c.URL, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var cr chatResp
	if err := json.Unmarshal(raw, &cr); err != nil {
		return nil, fmt.Errorf("parse respons Ollama: %w", err)
	}
	if cr.Error != "" {
		return nil, fmt.Errorf("Ollama error: %s", cr.Error)
	}
	var wrap struct {
		Moments []llm.Moment `json:"moments"`
	}
	if err := json.Unmarshal([]byte(cr.Message.Content), &wrap); err != nil {
		return nil, fmt.Errorf("parse JSON momen dari model lokal: %w", err)
	}
	return wrap.Moments, nil
}

// StatusInfo hasil pengecekan Ollama.
type StatusInfo struct {
	Running bool     `json:"running"`
	Models  []string `json:"models"`
}

// Status memeriksa apakah Ollama jalan & daftar model terpasang.
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
			Name string `json:"name"`
		} `json:"models"`
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(raw, &parsed)
	info := StatusInfo{Running: true}
	for _, m := range parsed.Models {
		info.Models = append(info.Models, m.Name)
	}
	return info
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
		return fmt.Errorf("Ollama tidak terjangkau (unduh gagal): %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var r struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	_ = json.Unmarshal(raw, &r)
	if r.Error != "" {
		return fmt.Errorf("unduh model gagal: %s", r.Error)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("unduh model gagal (status %d)", resp.StatusCode)
	}
	return nil
}
