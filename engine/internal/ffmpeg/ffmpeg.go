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
		return 0, fmt.Errorf("ffprobe duration: %w", err)
	}
	s := strings.TrimSpace(string(out))
	d, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", s, err)
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
		return 0, 0, fmt.Errorf("ffprobe dimensions: %w", err)
	}
	var parsed struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return 0, 0, fmt.Errorf("parse dimensions: %w", err)
	}
	if len(parsed.Streams) == 0 {
		return 0, 0, fmt.Errorf("no video stream found")
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
	return c.run(ctx, "extract audio", args)
}

// EncodeOpts parameter encoding & subtitle.
type EncodeOpts struct {
	CRF          string
	Preset       string
	AssPath      string // subtitle .ass; kosong = tanpa subtitle
	FontsDir     string // dir font untuk libass
	Mode         string // center | face_follow (di mana bingkai duduk)
	Background   string // blur | black — isi ruang kosong yang tersisa
	FrameVisible int    // 100 = frame asli utuh, 0 = memenuhi kotaknya
	PictureSize  int    // 100 = gambar memenuhi bingkai, <100 = mengecil di tengah
	FPS          int    // 0 = ikut sumber
}

// Layout menempatkan video ke bingkai target.
//
// Dipakai bersama oleh render klip DAN preview satu frame — sengaja satu
// sumber, sebab dulu keduanya menyusun filter sendiri-sendiri dan preview
// tertinggal ketika perilaku render berubah.
//
// Ada DUA sumbu yang berdiri sendiri. Keduanya sengaja dipisah: dulu satu
// kendali mencampur keduanya, dan itulah yang membuat zoom terasa salah.
//
// FrameVisible = berapa banyak frame ASLI yang tersisa terlihat.
//
//	100 = seluruh frame asli terlihat (contain) — tidak ada yang terpotong;
//	  0 = gambar memenuhi kotaknya (cover) — tepi yang berlebih dipotong.
//
// PictureSize = seberapa besar gambar itu duduk di dalam bingkai.
//
//	 100 = gambar memenuhi bingkai dari tepi ke tepi;
//	<100 = gambar mengecil di tengah, latar mengelilingi keempat sisinya.
//
// Contoh: FrameVisible 0 + PictureSize 50 memberi potongan penuh yang mengecil
// di tengah — persis tampilan yang dulu dihasilkan zoom 50.
type Layout struct {
	Mode         string // center | face_follow (di mana bingkai duduk)
	Background   string // blur | black — mengisi ruang kosong yang tersisa
	FrameVisible int    // 0..100
	PictureSize  int    // 5..100; <=0 dianggap 100
}

// ReframeFilter menyusun rantai filter -vf untuk menempatkan video ke bingkai
// target.
func ReframeFilter(l Layout, targetW, targetH int) string {
	visible := clampPercent(l.FrameVisible)
	size := l.PictureSize
	if size <= 0 {
		size = 100
	}
	size = clampPercent(size)

	// Kotak tempat gambar diletakkan. Dibulatkan genap: encoder h264 menolak
	// dimensi ganjil.
	boxW := evenBox(targetW * size / 100)
	boxH := evenBox(targetH * size / 100)

	foreground := fitChain(visible, boxW, boxH)

	// Latar hanya tak terlihat bila kotaknya sebesar bingkai DAN isinya memenuhi
	// kotak itu. Selain itu selalu ada ruang yang harus diisi.
	if size >= 100 && visible <= 0 {
		return foreground
	}

	if l.Background == "black" {
		// Tidak perlu menggandakan aliran: cukup beri bantalan hitam di sekeliling.
		return foreground + fmt.Sprintf(",pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black", targetW, targetH)
	}

	// Blur: video dipakai dua kali — satu jadi latar yang dibesarkan & diburamkan,
	// satu lagi jadi gambar depan yang ditumpuk di tengahnya.
	return fmt.Sprintf(
		"split=2[bg][fg];"+
			"[bg]scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d,gblur=sigma=20[bgb];"+
			"[fg]%s[fgb];"+
			"[bgb][fgb]overlay=(W-w)/2:(H-h)/2",
		targetW, targetH, targetW, targetH, foreground)
}

// fitChain memasukkan gambar ke kotak boxW x boxH menurut berapa banyak frame
// asli yang harus tetap terlihat.
//
// Kedua ujungnya memakai bentuk sederhana yang dihitung ffmpeg sendiri; hanya
// nilai di antaranya yang butuh ekspresi. Selain lebih mudah dibaca, ini juga
// menjamin ujungnya tepat: pembulatan ekspresi bisa meleset satu piksel, dan
// pada isi-penuh satu piksel meleset berarti segaris latar terlihat di tepi.
func fitChain(visible, boxW, boxH int) string {
	switch {
	case visible >= 100:
		// Contain: seluruh frame asli muat, menyisakan ruang di satu sumbu.
		return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos", boxW, boxH)
	case visible <= 0:
		// Cover: kotak penuh, kelebihannya dipotong di tengah.
		return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d",
			boxW, boxH, boxW, boxH)
	}

	// Di antaranya: skala diinterpolasi linear dari contain ke cover. Makin
	// KECIL FrameVisible, makin dekat ke cover — jadi bobotnya dibalik.
	//
	//	contain = min(BW/iw, BH/ih)      cover = max(BW/iw, BH/ih)
	//	skala   = contain + (cover-contain) * (100-visible)/100
	//
	// Dihitung ffmpeg lewat ekspresi supaya dimensi sumber tidak perlu diprobe
	// lebih dulu. Ekspresinya dikutip tunggal agar koma di dalam min()/max()
	// tidak dibaca sebagai pemisah filter — di dalam kutip, koma tidak
	// ditafsirkan, jadi TIDAK boleh di-escape dengan backslash.
	toward := float64(100-visible) / 100
	contain := fmt.Sprintf("min(%d/iw,%d/ih)", boxW, boxH)
	cover := fmt.Sprintf("max(%d/iw,%d/ih)", boxW, boxH)
	scale := fmt.Sprintf("(%s+(%s-%s)*%.4f)", contain, cover, contain, toward)

	return fmt.Sprintf(
		"scale=w='trunc(iw*%s/2)*2':h='trunc(ih*%s/2)*2':flags=lanczos"+
			",crop='min(iw,%d)':'min(ih,%d)'",
		scale, scale, boxW, boxH)
}

func clampPercent(n int) int {
	if n < 0 {
		return 0
	}
	if n > 100 {
		return 100
	}
	return n
}

func evenBox(n int) int {
	if n < 2 {
		return 2
	}
	if n%2 != 0 {
		n--
	}
	return n
}

// ClipReframe memotong [start,end], menyesuaikan ke rasio target, dan membakar
// subtitle .ass bila diisi. Penempatan gambarnya ditentukan
// EncodeOpts.FrameVisible & EncodeOpts.PictureSize.
func (c *Client) ClipReframe(ctx context.Context, input string, start, end float64, targetW, targetH int, enc EncodeOpts, out string) error {
	dur := end - start
	if dur <= 0 {
		return fmt.Errorf("invalid clip duration: %.2f", dur)
	}
	// Pastikan folder tujuan ada. Render satu job bisa berjalan puluhan menit;
	// bila foldernya sempat terhapus atau dipindah di tengah jalan, ffmpeg baru
	// gagal setelah selesai meng-encode (ENOENT, exit 254) dan seluruh kerjanya
	// terbuang. Ini folder kerja job sendiri, jadi membuatnya ulang aman.
	if dir := filepath.Dir(out); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("output folder %q: %w", dir, err)
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

	vf := ReframeFilter(Layout{Mode: enc.Mode, Background: enc.Background,
		FrameVisible: enc.FrameVisible, PictureSize: enc.PictureSize}, targetW, targetH)
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
func (c *Client) ExtractFrame(ctx context.Context, input string, t float64, targetW, targetH int, layout Layout) ([]byte, error) {
	vf := ReframeFilter(layout, targetW, targetH)
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
		return nil, fmt.Errorf("frame extraction failed: %w", err)
	}
	return out.Bytes(), nil
}

func (c *Client) run(ctx context.Context, label string, args []string) error {
	cmd := exec.CommandContext(ctx, c.FFmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w: %s", label, err, summarizeError(stderr.String()))
	}
	return nil
}

// errorKeywords = penanda baris stderr yang benar-benar menjelaskan kegagalan.
var errorKeywords = []string{
	"error", "invalid", "no such", "permission", "unable",
	"not found", "denied", "no space", "cannot", "failed",
}

// summarizeError menyaring baris stderr ffmpeg yang menjelaskan sebab kegagalan.
//
// Dulu bagian ini hanya memotong 500 karakter TERAKHIR — padahal ekor keluaran
// ffmpeg selalu berisi blok statistik x264, bagian yang paling tidak berguna,
// sementara baris sebabnya tercetak jauh lebih awal dan ikut terbuang. Kasus
// nyatanya: "exit status 254" yang ternyata "No such file or directory" karena
// folder keluaran hilang saat render, tapi pesan itu tidak pernah terlihat.
func summarizeError(stderr string) string {
	var important []string
	seen := map[string]bool{}
	for _, ln := range strings.Split(stderr, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || seen[ln] {
			continue
		}
		low := strings.ToLower(ln)
		// "Conversion failed!" selalu muncul dan tidak menjelaskan apa pun —
		// dipakai hanya kalau tak ada baris lain yang lebih berguna.
		if low == "conversion failed!" {
			continue
		}
		for _, k := range errorKeywords {
			if strings.Contains(low, k) {
				important = append(important, ln)
				seen[ln] = true
				break
			}
		}
	}
	if len(important) == 0 {
		// Tak ada baris yang cocok: kembali ke ekor keluaran seadanya.
		tail := strings.TrimSpace(stderr)
		if len(tail) > 300 {
			tail = "…" + tail[len(tail)-300:]
		}
		return tail
	}
	// Baris terakhir yang cocok biasanya sebab paling dekat dengan kegagalan;
	// dua baris sebelumnya ikut dibawa sebagai konteks.
	if len(important) > 3 {
		important = important[len(important)-3:]
	}
	message := strings.Join(important, " | ")
	if hint := errorHint(message); hint != "" {
		message += " — " + hint
	}
	return message
}

// errorHint menerjemahkan galat ffmpeg yang sering muncul jadi langkah yang
// bisa ditindaklanjuti pengguna.
func errorHint(msg string) string {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "no such file or directory"):
		return "the output folder or file is missing (deleted or moved during the render?)"
	case strings.Contains(low, "permission denied"):
		return "write permission denied for the output folder"
	case strings.Contains(low, "no space left"):
		return "the disk is full"
	case strings.Contains(low, "invalid data found"):
		return "the source file is corrupt or its format is unsupported"
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
