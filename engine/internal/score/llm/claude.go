// Package llm memanggil Claude API untuk memilih & menilai momen klip.
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
	// Temperature dikirim hanya bila > 0; selain itu dipakai bawaan API.
	// Tugas menyalin-ulang seperti koreksi transkrip butuh angka rendah.
	Temperature float64
}

func New(apiKey, model string) *Client {
	if model == "" {
		model = "claude-haiku-4-5"
	}
	return &Client{
		APIKey: apiKey,
		Model:  model,
		HTTP:   &http.Client{Timeout: 120 * time.Second},
	}
}

// anthropic request/response shapes.
type msgReq struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	System      string    `json:"system"`
	Messages    []anthMsg `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
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

// Moment adalah momen menarik yang dipilih LLM dari transkrip (batas ditentukan
// LLM). Angka memakai float64 agar toleran terhadap model lokal yang membalas
// desimal (mis. qwen membalas "score": 7.0). Dikonversi ke int saat jadi klip.
type Moment struct {
	Start    float64       `json:"start"`
	End      float64       `json:"end"`
	Score    float64       `json:"score"`
	Reasons  MomentReasons `json:"reasons"`
	Title    string        `json:"title"`
	Hashtags []string      `json:"hashtags"`
	// Continues menandai momen yang masih bersambung ke potongan transkrip
	// berikutnya (lihat penggabungan di paket pipeline).
	Continues bool `json:"continues"`
}

// MomentReasons rincian skor (float agar toleran output model lokal).
type MomentReasons struct {
	Hook         float64 `json:"hook"`
	Emotion      float64 `json:"emotion"`
	Clarity      float64 `json:"clarity"`
	Shareability float64 `json:"shareability"`
	Standalone   float64 `json:"standalone"`
}

// SelectMoments mengirim satu potongan transkrip bertimestamp & meminta Claude
// memilih momen terbaik. Semua kegagalan dikembalikan apa adanya — pemanggil
// TIDAK boleh diam-diam berpindah ke mesin lain.
func (c *Client) SelectMoments(ctx context.Context, tr types.Transcript, maxClips int, targetMin, targetMax float64, ch Chunk) ([]Moment, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("the Claude API key is empty — set it in the AI engine panel (GUI) or ANTHROPIC_API_KEY in .env")
	}
	text, err := c.Complete(ctx, SystemPrompt(targetMin, targetMax, ch, tr.Language), UserPrompt(tr, maxClips), 8192)
	if err != nil {
		return nil, err
	}
	moments, err := parseMoments(text)
	if err != nil {
		return nil, fmt.Errorf("Claude (%s) returned JSON that could not be read: %w — reply: %s",
			c.Model, err, truncate(text, 300))
	}
	return moments, nil
}

// Complete mengirim satu pasang prompt dan mengembalikan teks balasan apa
// adanya. Ini lapisan HTTP telanjang yang dipakai bersama oleh pemilihan momen
// klip dan pemilihan paragraf berita — keduanya cuma berbeda prompt.
//
// Seperti SelectMoments: setiap kegagalan dikembalikan apa adanya, TIDAK ada
// perpindahan diam-diam ke mesin lain (lihat notes/12).
func (c *Client) Complete(ctx context.Context, system, user string, maxTokens int) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("the Claude API key is empty — set it in the AI engine panel (GUI) or ANTHROPIC_API_KEY in .env")
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	reqBody := msgReq{
		Model:     c.Model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  []anthMsg{{Role: "user", Content: user}},
	}
	if c.Temperature > 0 {
		reqBody.Temperature = &c.Temperature
	}
	buf, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("Claude is unreachable (check your internet connection): %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var parsed msgResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("the Claude response could not be read (status %d): %s", resp.StatusCode, truncate(string(raw), 200))
	}
	if parsed.Error != nil {
		return "", claudeError(resp.StatusCode, parsed.Error.Message)
	}
	if len(parsed.Content) == 0 {
		return "", fmt.Errorf("Claude returned an empty reply (status %d)", resp.StatusCode)
	}
	return parsed.Content[0].Text, nil
}

// claudeError menerjemahkan pesan API jadi petunjuk yang bisa ditindaklanjuti.
func claudeError(status int, msg string) error {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "x-api-key") || status == 401:
		return fmt.Errorf("the Claude API key was rejected — update it in the AI engine panel (%s)", msg)
	case status == 429:
		return fmt.Errorf("Claude quota/rate limit exceeded — wait a moment and try again (%s)", msg)
	case strings.Contains(low, "model"):
		return fmt.Errorf("unknown Claude model — pick a different model in the GUI (%s)", msg)
	case status == 400 && strings.Contains(low, "credit"):
		return fmt.Errorf("the Claude API balance is exhausted (%s)", msg)
	}
	return fmt.Errorf("Claude rejected the request (status %d): %s", status, msg)
}

// parseMoments menerima bentuk {"moments":[...]} maupun array telanjang [...].
func parseMoments(text string) ([]Moment, error) {
	if obj := extractBlock(text, '{', '}'); obj != "" {
		var wrap MomentsWrapper
		if err := json.Unmarshal([]byte(obj), &wrap); err == nil && wrap.Moments != nil {
			return wrap.Moments, nil
		}
	}
	arr := extractBlock(text, '[', ']')
	if arr == "" {
		return nil, fmt.Errorf("no JSON block found in the reply")
	}
	var moments []Moment
	if err := json.Unmarshal([]byte(arr), &moments); err != nil {
		return nil, err
	}
	return moments, nil
}

// extractBlock mengambil blok pertama..terakhir di antara pembuka & penutup.
func extractBlock(s string, open, close byte) string {
	i := strings.IndexByte(s, open)
	j := strings.LastIndexByte(s, close)
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return ""
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
