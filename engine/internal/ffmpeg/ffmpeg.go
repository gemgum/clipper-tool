// Package ffmpeg membungkus pemanggilan ffmpeg/ffprobe sebagai subprocess.
package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	Mode     string // "fit" (frame utuh) | selain itu = center (crop/zoom)
	Latar    string // blur | hitam — isi ruang kosong di sekeliling video
	Zoom     int    // persen ukuran video dalam bingkai; 0/100 = isi penuh
	FPS      int    // 0 = ikut sumber
}

// ReframeFilter membangun rantai filter penyesuai rasio.
//
// Dipakai bersama oleh render klip DAN preview satu frame — sengaja satu
// sumber, sebab dulu keduanya menyusun filter sendiri-sendiri dan preview
// tertinggal (selalu crop tengah) ketika mode "fit" ditambahkan ke render.
//
// "fit"  : frame utuh, tanpa zoom (paling tajam).
// selain : crop tengah, isi penuh.
//
// Zoom mengecilkan video di dalam bingkai (100 = isi penuh). Ruang sisa diisi
// menurut Latar: blur dari videonya sendiri, atau hitam polos.
type Tata struct {
	Mode  string // center | fit
	Latar string // blur | hitam
	Zoom  int    // persen; <=0 dianggap 100
}

// ReframeFilter menyusun rantai filter -vf untuk menempatkan video ke bingkai
// target.
func ReframeFilter(t Tata, targetW, targetH int) string {
	zoom := t.Zoom
	if zoom <= 0 || zoom > 100 {
		zoom = 100
	}
	// Ukuran video di dalam bingkai. Dibulatkan genap: encoder h264 menolak
	// dimensi ganjil.
	fw := genap(targetW * zoom / 100)
	fh := genap(targetH * zoom / 100)

	// Depan: "fit" memuat frame utuh (decrease), selain itu menutupi lalu
	// dipotong tengah (increase + crop).
	var depan string
	if t.Mode == "fit" {
		depan = fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos", fw, fh)
	} else {
		depan = fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d", fw, fh, fw, fh)
	}

	// Isi penuh tanpa zoom tidak menyisakan ruang kosong sama sekali, jadi latar
	// apa pun tidak akan terlihat — rantainya dibiarkan sesederhana mungkin.
	if t.Mode != "fit" && zoom == 100 {
		return depan
	}

	if t.Latar == "hitam" {
		// Tidak perlu menggandakan aliran: cukup beri bantalan hitam di sekeliling.
		return depan + fmt.Sprintf(",pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black", targetW, targetH)
	}

	// Blur: video dipakai dua kali — satu jadi latar yang dibesarkan & diburamkan,
	// satu lagi jadi gambar depan yang ditumpuk di tengahnya.
	return fmt.Sprintf(
		"split=2[bg][fg];"+
			"[bg]scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d,gblur=sigma=20[bgb];"+
			"[fg]%s[fgb];"+
			"[bgb][fgb]overlay=(W-w)/2:(H-h)/2",
		targetW, targetH, targetW, targetH, depan)
}

func genap(n int) int {
	if n < 2 {
		return 2
	}
	if n%2 != 0 {
		n--
	}
	return n
}

// ClipReframe memotong [start,end], menyesuaikan ke rasio target, dan membakar
// subtitle .ass bila diisi. Mode "fit" menampilkan frame utuh di atas latar blur
// (tanpa zoom, paling tajam); selain itu crop tengah (isi penuh, ada zoom).
func (c *Client) ClipReframe(ctx context.Context, input string, start, end float64, targetW, targetH int, enc EncodeOpts, out string) error {
	dur := end - start
	if dur <= 0 {
		return fmt.Errorf("durasi klip tidak valid: %.2f", dur)
	}
	// Pastikan folder tujuan ada. Render satu job bisa berjalan puluhan menit;
	// bila foldernya sempat terhapus atau dipindah di tengah jalan, ffmpeg baru
	// gagal setelah selesai meng-encode (ENOENT, exit 254) dan seluruh kerjanya
	// terbuang. Ini folder kerja job sendiri, jadi membuatnya ulang aman.
	if dir := filepath.Dir(out); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("folder keluaran %q: %w", dir, err)
		}
	}
	// Filter subtitle (ditempel di akhir rantai).
	sub := ""
	if enc.AssPath != "" {
		sub = fmt.Sprintf("subtitles='%s'", escapeFilterPath(enc.AssPath))
		if enc.FontsDir != "" {
			sub += fmt.Sprintf(":fontsdir='%s'", escapeFilterPath(enc.FontsDir))
		}
	}

	vf := ReframeFilter(Tata{Mode: enc.Mode, Latar: enc.Latar, Zoom: enc.Zoom}, targetW, targetH)
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
		"-map", "0:v:0", // ambil video pertama
		"-map", "0:a:0?", // ambil audio pertama bila ada (? = opsional)
		"-vf", vf,
		"-c:v", "libx264",
		"-preset", preset,
		"-crf", crf,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "160k",
		"-ac", "2", // paksa stereo (kompatibel semua pemutar)
		"-movflags", "+faststart",
		out,
	}
	return c.run(ctx, "clip+reframe", args)
}

// ExtractFrame mengambil 1 frame pada detik t, disesuaikan ke rasio target
// memakai mode reframe yang sama dengan render, dikembalikan sebagai JPEG
// (untuk preview subtitle di GUI). Hasilnya = satu frame dari klip jadi,
// jadi koordinat subtitle yang diatur di preview berlaku apa adanya.
func (c *Client) ExtractFrame(ctx context.Context, input string, t float64, targetW, targetH int, tata Tata) ([]byte, error) {
	vf := ReframeFilter(tata, targetW, targetH)
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
		return fmt.Errorf("%s gagal: %w: %s", label, err, ringkasGalat(stderr.String()))
	}
	return nil
}

// kataGalat = penanda baris stderr yang benar-benar menjelaskan kegagalan.
var kataGalat = []string{
	"error", "invalid", "no such", "permission", "unable",
	"not found", "denied", "no space", "cannot", "failed",
}

// ringkasGalat menyaring baris stderr ffmpeg yang menjelaskan sebab kegagalan.
//
// Dulu bagian ini hanya memotong 500 karakter TERAKHIR — padahal ekor keluaran
// ffmpeg selalu berisi blok statistik x264, bagian yang paling tidak berguna,
// sementara baris sebabnya tercetak jauh lebih awal dan ikut terbuang. Kasus
// nyatanya: "exit status 254" yang ternyata "No such file or directory" karena
// folder keluaran hilang saat render, tapi pesan itu tidak pernah terlihat.
func ringkasGalat(stderr string) string {
	var penting []string
	terlihat := map[string]bool{}
	for _, ln := range strings.Split(stderr, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || terlihat[ln] {
			continue
		}
		low := strings.ToLower(ln)
		// "Conversion failed!" selalu muncul dan tidak menjelaskan apa pun —
		// dipakai hanya kalau tak ada baris lain yang lebih berguna.
		if low == "conversion failed!" {
			continue
		}
		for _, k := range kataGalat {
			if strings.Contains(low, k) {
				penting = append(penting, ln)
				terlihat[ln] = true
				break
			}
		}
	}
	if len(penting) == 0 {
		// Tak ada baris yang cocok: kembali ke ekor keluaran seadanya.
		tail := strings.TrimSpace(stderr)
		if len(tail) > 300 {
			tail = "…" + tail[len(tail)-300:]
		}
		return tail
	}
	// Baris terakhir yang cocok biasanya sebab paling dekat dengan kegagalan;
	// dua baris sebelumnya ikut dibawa sebagai konteks.
	if len(penting) > 3 {
		penting = penting[len(penting)-3:]
	}
	pesan := strings.Join(penting, " | ")
	if p := petunjukGalat(pesan); p != "" {
		pesan += " — " + p
	}
	return pesan
}

// petunjukGalat menerjemahkan galat ffmpeg yang sering muncul jadi langkah
// yang bisa ditindaklanjuti pengguna.
func petunjukGalat(msg string) string {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "no such file or directory"):
		return "folder atau berkas keluaran tidak ada (mungkin terhapus/dipindah saat render)"
	case strings.Contains(low, "permission denied"):
		return "izin tulis ditolak untuk folder keluaran"
	case strings.Contains(low, "no space left"):
		return "ruang disk habis"
	case strings.Contains(low, "invalid data found"):
		return "berkas sumber rusak atau formatnya tidak didukung"
	}
	return ""
}

// escapeFilterPath meng-escape karakter khusus dalam path untuk filter ffmpeg.
func escapeFilterPath(p string) string {
	p = strings.ReplaceAll(p, `\`, `\\`)
	p = strings.ReplaceAll(p, `:`, `\:`)
	p = strings.ReplaceAll(p, `'`, `\'`)
	return p
}
