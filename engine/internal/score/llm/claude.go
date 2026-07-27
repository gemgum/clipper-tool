// Package llm memanggil Claude API untuk menilai & memberi judul klip.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/types"
)

const endpoint = "https://api.anthropic.com/v1/messages"

// Client Claude API.
type Client struct {
	APIKey string
	Model  string
	HTTP   *http.Client
}

func New(apiKey, model string) *Client {
	if model == "" {
		model = "claude-haiku-4-5"
	}
	return &Client{
		APIKey: apiKey,
		Model:  model,
		HTTP:   &http.Client{Timeout: 90 * time.Second},
	}
}

// Judgment hasil penilaian LLM untuk satu klip.
type Judgment struct {
	Score    int           `json:"score"`
	Reasons  types.Reasons `json:"reasons"`
	Title    string        `json:"title"`
	Hashtags []string      `json:"hashtags"`
}

// anthropic request/response shapes.
type msgReq struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system"`
	Messages  []anthMsg    `json:"messages"`
}
type anthMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type msgResp struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

const systemPrompt = `Kamu adalah kurator klip video viral untuk konten berbahasa Indonesia (TikTok, Reels, Shorts).
Nilai satu segmen transkrip. Balas HANYA JSON valid tanpa penjelasan, dengan bentuk persis:
{"score":<0-100>,"reasons":{"hook":<0-100>,"emotion":<0-100>,"clarity":<0-100>,"shareability":<0-100>,"standalone":<0-100>},"title":"<judul catchy bahasa Indonesia, maks 60 karakter>","hashtags":["#..","#.."]}
Kriteria: hook (3 detik pertama menarik?), emotion (muatan emosi), clarity (mudah dipahami), shareability (layak dibagikan), standalone (bisa dimengerti tanpa konteks).`

// Judge menilai satu kandidat via Claude.
func (c *Client) Judge(ctx context.Context, cand types.Candidate) (Judgment, error) {
	if c.APIKey == "" {
		return Judgment{}, fmt.Errorf("ANTHROPIC_API_KEY kosong")
	}
	user := fmt.Sprintf("Durasi: %.0f detik.\nTranskrip:\n%s", cand.Duration(), cand.Text)
	reqBody := msgReq{
		Model:     c.Model,
		MaxTokens: 400,
		System:    systemPrompt,
		Messages:  []anthMsg{{Role: "user", Content: user}},
	}
	buf, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return Judgment{}, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Judgment{}, fmt.Errorf("panggil Claude: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var parsed msgResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Judgment{}, fmt.Errorf("parse respons Claude: %w", err)
	}
	if parsed.Error != nil {
		return Judgment{}, fmt.Errorf("Claude error: %s", parsed.Error.Message)
	}
	if len(parsed.Content) == 0 {
		return Judgment{}, fmt.Errorf("respons Claude kosong (status %d)", resp.StatusCode)
	}

	text := parsed.Content[0].Text
	jsonStr := extractJSON(text)
	var j Judgment
	if err := json.Unmarshal([]byte(jsonStr), &j); err != nil {
		return Judgment{}, fmt.Errorf("parse JSON penilaian: %w (teks: %s)", err, truncate(text, 200))
	}
	return j, nil
}

// Moment adalah momen menarik yang dipilih LLM dari transkrip (batas ditentukan LLM).
type Moment struct {
	Start    float64       `json:"start"`
	End      float64       `json:"end"`
	Score    int           `json:"score"`
	Reasons  types.Reasons `json:"reasons"`
	Title    string        `json:"title"`
	Hashtags []string      `json:"hashtags"`
}

const selectSystem = `Kamu kurator klip video viral untuk konten berbahasa Indonesia (TikTok, Reels, Shorts).
Kamu diberi transkrip video LENGKAP dengan timestamp (detik). Tugasmu MEMILIH momen-momen terbaik untuk dijadikan klip pendek vertikal.
Aturan:
- Tiap momen harus BERDIRI SENDIRI: ada hook menarik di awal, isi yang jelas, dan penutup yang memuaskan. BUKAN potongan acak di tengah kalimat.
- Tentukan sendiri 'start' dan 'end' (detik) tiap momen dari timestamp transkrip — panjang boleh BERVARIASI sesuai isi.
- Incar durasi sekitar %.0f-%.0f detik, TAPI boleh menyimpang demi momen yang utuh & kuat.
- Pilih momen dengan potensi viral tertinggi (hook kuat, emosi, kejutan, nilai, layak dibagikan).
Balas HANYA JSON array valid tanpa penjelasan, bentuk persis:
[{"start":<detik>,"end":<detik>,"score":<0-100>,"reasons":{"hook":<0-100>,"emotion":<0-100>,"clarity":<0-100>,"shareability":<0-100>,"standalone":<0-100>},"title":"<judul catchy Indonesia, maks 60 karakter>","hashtags":["#..","#.."]}]`

// SelectMoments mengirim transkrip bertimestamp & meminta LLM memilih momen terbaik.
func (c *Client) SelectMoments(ctx context.Context, tr types.Transcript, maxClips int, targetMin, targetMax float64) ([]Moment, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY kosong")
	}
	var b strings.Builder
	for _, s := range tr.Segments {
		fmt.Fprintf(&b, "[%.1f-%.1f] %s\n", s.Start, s.End, s.Text)
	}
	user := fmt.Sprintf("Transkrip:\n%s\nPilih maksimal %d momen terbaik. Balas JSON array.", b.String(), maxClips)

	reqBody := msgReq{
		Model:     c.Model,
		MaxTokens: 4096,
		System:    fmt.Sprintf(selectSystem, targetMin, targetMax),
		Messages:  []anthMsg{{Role: "user", Content: user}},
	}
	buf, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("panggil Claude: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var parsed msgResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse respons Claude: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("Claude error: %s", parsed.Error.Message)
	}
	if len(parsed.Content) == 0 {
		return nil, fmt.Errorf("respons Claude kosong (status %d)", resp.StatusCode)
	}
	jsonStr := extractArray(parsed.Content[0].Text)
	var moments []Moment
	if err := json.Unmarshal([]byte(jsonStr), &moments); err != nil {
		return nil, fmt.Errorf("parse JSON momen: %w", err)
	}
	return moments, nil
}

// extractArray mengambil blok [...] pertama dari teks.
func extractArray(s string) string {
	i := strings.Index(s, "[")
	j := strings.LastIndex(s, "]")
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}

// extractJSON mengambil blok {...} pertama dari teks.
func extractJSON(s string) string {
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
