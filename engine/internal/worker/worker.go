// Package worker memanggil worker C++ (native) via subprocess (stdin/NDJSON).
package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Client memegang path binary worker C++.
type Client struct {
	Bin string
}

func New(bin string) *Client {
	return &Client{Bin: bin}
}

// Available memeriksa binary worker ada.
func (c *Client) Available() bool {
	if _, err := exec.LookPath(c.Bin); err == nil {
		return true
	}
	_, err := os.Stat(c.Bin)
	return err == nil
}

// featuresReq permintaan analisis audio.
type featuresReq struct {
	Cmd   string `json:"cmd"`
	Input string `json:"input"`
	HopMS int    `json:"hop_ms"`
}

// ndjsonLine satu baris balasan worker.
type ndjsonLine struct {
	Type    string    `json:"type"`
	Value   float64   `json:"value"`
	RMS     []float64 `json:"rms"`
	HopMS   int       `json:"hop_ms"`
	Message string    `json:"message"`
}

// FeaturesResult hasil analisis energi audio.
type FeaturesResult struct {
	RMS   []float64
	HopMS int
}

// Features menjalankan cmd "features" pada WAV → deret RMS.
func (c *Client) Features(ctx context.Context, wavPath string, hopMS int) (FeaturesResult, error) {
	if hopMS <= 0 {
		hopMS = 100
	}
	req := featuresReq{Cmd: "features", Input: wavPath, HopMS: hopMS}
	body, _ := json.Marshal(req)

	cmd := exec.CommandContext(ctx, c.Bin)
	cmd.Stdin = strings.NewReader(string(body))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return FeaturesResult{}, err
	}
	if err := cmd.Start(); err != nil {
		return FeaturesResult{}, fmt.Errorf("mulai worker: %w", err)
	}

	var res FeaturesResult
	var workerErr string
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		var line ndjsonLine
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue
		}
		switch line.Type {
		case "result":
			res.RMS = line.RMS
			res.HopMS = line.HopMS
		case "error":
			workerErr = line.Message
		}
	}
	if err := cmd.Wait(); err != nil {
		if workerErr != "" {
			return FeaturesResult{}, fmt.Errorf("worker error: %s", workerErr)
		}
		return FeaturesResult{}, fmt.Errorf("worker gagal: %w", err)
	}
	return res, nil
}
