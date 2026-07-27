// Package ffmpeg membungkus pemanggilan ffmpeg/ffprobe sebagai subprocess.
package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Client memegang path binary ffmpeg & ffprobe.
type Client struct {
	FFmpeg  string
	FFprobe string
}

func New(ffmpeg, ffprobe string) *Client {
	return &Client{FFmpeg: ffmpeg, FFprobe: ffprobe}
}

// Duration mengembalikan durasi video (detik) via ffprobe.
func (c *Client) Duration(ctx context.Context, input string) (float64, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		input,
	}
	out, err := exec.CommandContext(ctx, c.FFprobe, args...).Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe durasi: %w", err)
	}
	s := strings.TrimSpace(string(out))
	d, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse durasi %q: %w", s, err)
	}
	return d, nil
}

// Dimensions mengembalikan lebar & tinggi video.
func (c *Client) Dimensions(ctx context.Context, input string) (w, h int, err error) {
	args := []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "json",
		input,
	}
	out, err := exec.CommandContext(ctx, c.FFprobe, args...).Output()
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe dimensi: %w", err)
	}
	var parsed struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return 0, 0, fmt.Errorf("parse dimensi: %w", err)
	}
	if len(parsed.Streams) == 0 {
		return 0, 0, fmt.Errorf("tidak ada stream video")
	}
	return parsed.Streams[0].Width, parsed.Streams[0].Height, nil
}

// ExtractAudioWAV mengekstrak audio ke WAV 16kHz mono (format whisper.cpp).
// Streaming langsung ke file — video tidak dimuat ke RAM.
func (c *Client) ExtractAudioWAV(ctx context.Context, input, outWAV string) error {
	args := []string{
		"-y",
		"-i", input,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "pcm_s16le",
		outWAV,
	}
	return c.run(ctx, "ekstrak audio", args)
}

// EncodeOpts parameter encoding & subtitle.
type EncodeOpts struct {
	CRF      string
	Preset   string
	AssPath  string // subtitle .ass; kosong = tanpa subtitle
	FontsDir string // dir font untuk libass
	Mode     string // "fit" (latar blur) | selain itu = center (crop/zoom)
	FPS      int    // 0 = ikut sumber
}

// ClipReframe memotong [start,end], menyesuaikan ke rasio target, dan membakar
// subtitle .ass bila diisi. Mode "fit" menampilkan frame utuh di atas latar blur
// (tanpa zoom, paling tajam); selain itu crop tengah (isi penuh, ada zoom).
func (c *Client) ClipReframe(ctx context.Context, input string, start, end float64, targetW, targetH int, enc EncodeOpts, out string) error {
	dur := end - start
	if dur <= 0 {
		return fmt.Errorf("durasi klip tidak valid: %.2f", dur)
	}
	// Filter subtitle (ditempel di akhir rantai).
	sub := ""
	if enc.AssPath != "" {
		sub = fmt.Sprintf("subtitles='%s'", escapeFilterPath(enc.AssPath))
		if enc.FontsDir != "" {
			sub += fmt.Sprintf(":fontsdir='%s'", escapeFilterPath(enc.FontsDir))
		}
	}

	var vf string
	if enc.Mode == "fit" {
		// Latar: frame dibesarkan penuh lalu di-blur. Depan: frame utuh (fit).
		vf = fmt.Sprintf(
			"split=2[bg][fg];"+
				"[bg]scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d,gblur=sigma=20[bgb];"+
				"[fg]scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos[fgb];"+
				"[bgb][fgb]overlay=(W-w)/2:(H-h)/2",
			targetW, targetH, targetW, targetH, targetW, targetH)
	} else {
		// Center: cover lalu crop tengah (lanczos untuk pembesaran lebih tajam).
		vf = fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d",
			targetW, targetH, targetW, targetH)
	}
	if enc.FPS > 0 {
		vf += fmt.Sprintf(",fps=%d", enc.FPS)
	}
	if sub != "" {
		vf += "," + sub
	}

	crf := enc.CRF
	if crf == "" {
		crf = "20"
	}
	preset := enc.Preset
	if preset == "" {
		preset = "medium"
	}
	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", start),
		"-i", input,
		"-t", fmt.Sprintf("%.3f", dur),
		"-map", "0:v:0",   // ambil video pertama
		"-map", "0:a:0?",  // ambil audio pertama bila ada (? = opsional)
		"-vf", vf,
		"-c:v", "libx264",
		"-preset", preset,
		"-crf", crf,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "160k",
		"-ac", "2",        // paksa stereo (kompatibel semua pemutar)
		"-movflags", "+faststart",
		out,
	}
	return c.run(ctx, "clip+reframe", args)
}

// ExtractFrame mengambil 1 frame pada detik t, di-crop ke rasio target,
// dikembalikan sebagai JPEG (untuk preview subtitle di GUI).
func (c *Client) ExtractFrame(ctx context.Context, input string, t float64, targetW, targetH int) ([]byte, error) {
	vf := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d",
		targetW, targetH, targetW, targetH)
	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", t),
		"-i", input,
		"-frames:v", "1",
		"-vf", vf,
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"pipe:1",
	}
	cmd := exec.CommandContext(ctx, c.FFmpeg, args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ekstrak frame gagal: %w", err)
	}
	return out.Bytes(), nil
}

func (c *Client) run(ctx context.Context, label string, args []string) error {
	cmd := exec.CommandContext(ctx, c.FFmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		tail := stderr.String()
		if len(tail) > 500 {
			tail = tail[len(tail)-500:]
		}
		return fmt.Errorf("%s gagal: %w: %s", label, err, tail)
	}
	return nil
}

// escapeFilterPath meng-escape karakter khusus dalam path untuk filter ffmpeg.
func escapeFilterPath(p string) string {
	p = strings.ReplaceAll(p, `\`, `\\`)
	p = strings.ReplaceAll(p, `:`, `\:`)
	p = strings.ReplaceAll(p, `'`, `\'`)
	return p
}
