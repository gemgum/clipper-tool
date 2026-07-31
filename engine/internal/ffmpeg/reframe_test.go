package ffmpeg

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// Rantai filter dipakai bersama render klip & preview satu frame. Tes ini
// menjaga keduanya tetap satu sumber — kalau salah satu ujung diubah, yang lain
// ikut berubah dengan sendirinya.
func TestReframeFilterEnds(t *testing.T) {
	// Isi penuh: perbesar lalu potong tengah, tanpa pekerjaan latar sama sekali.
	full := ReframeFilter(Layout{Mode: "center", FrameVisible: 0, PictureSize: 100}, 1080, 1920)
	if !strings.Contains(full, "force_original_aspect_ratio=increase") || !strings.Contains(full, "crop=1080:1920") {
		t.Errorf("isi penuh harus memperbesar lalu crop tengah, dapat: %s", full)
	}
	for _, forbidden := range []string{"gblur", "split=2", "pad="} {
		if strings.Contains(full, forbidden) {
			t.Errorf("isi penuh tidak boleh memakai %q: %s", forbidden, full)
		}
	}

	// Frame utuh: seluruh gambar asli di atas latar blur.
	none := ReframeFilter(Layout{Mode: "center", Background: "blur", FrameVisible: 100, PictureSize: 100}, 1080, 1920)
	for _, part := range []string{
		"split=2[bg][fg]",                      // frame dipakai dua kali
		"gblur=sigma=20",                       // latar di-blur
		"force_original_aspect_ratio=decrease", // depan muat utuh, tidak terpotong
		"overlay=(W-w)/2:(H-h)/2",              // ditaruh persis di tengah
	} {
		if !strings.Contains(none, part) {
			t.Errorf("frame utuh kehilangan %q, dapat: %s", part, none)
		}
	}
}

// "Isi penuh 9:16" adalah keluaran yang paling sering dipakai, dan pernah
// sekali berubah tanpa disadari saat sumbu zoom ditulis ulang. Rantainya
// dikunci persis di sini: kalau ada yang menyentuhnya, tes ini yang berteriak
// lebih dulu, bukan pengguna yang melihat hasil rendernya berbeda.
func TestFullFrameFilterIsLockedDown(t *testing.T) {
	const want = "scale=1080:1920:force_original_aspect_ratio=increase:flags=lanczos,crop=1080:1920"
	for _, l := range []Layout{
		{Mode: "center", FrameVisible: 0, PictureSize: 100},                      // tanpa latar
		{Mode: "center", Background: "blur", FrameVisible: 0, PictureSize: 100},  // latar diabaikan
		{Mode: "center", Background: "black", FrameVisible: 0, PictureSize: 100}, // latar diabaikan
		{Mode: "center", FrameVisible: 0},                                        // PictureSize kosong = 100
	} {
		if got := ReframeFilter(l, 1080, 1920); got != want {
			t.Errorf("layout %+v:\n dapat: %s\n ingin: %s", l, got, want)
		}
	}
}

func TestReframeFilterBlackBackgroundDoesNotSplitStream(t *testing.T) {
	got := ReframeFilter(Layout{Mode: "center", Background: "black", FrameVisible: 100, PictureSize: 100}, 1080, 1920)
	if !strings.Contains(got, "pad=1080:1920") {
		t.Errorf("latar hitam harus memakai pad, dapat: %s", got)
	}
	// Latar hitam tidak perlu menyalin aliran video — split/overlay hanya
	// dibutuhkan untuk blur, dan memakainya di sini membuang waktu encode.
	for _, forbidden := range []string{"split=2", "gblur", "overlay="} {
		if strings.Contains(got, forbidden) {
			t.Errorf("latar hitam tidak boleh memakai %q: %s", forbidden, got)
		}
	}
}

// Zoom di antara kedua ujung tetap menyisakan ruang, jadi latar wajib ikut.
func TestReframeFilterMidZoomKeepsBackground(t *testing.T) {
	got := ReframeFilter(Layout{Mode: "center", Background: "black", FrameVisible: 50, PictureSize: 100}, 1080, 1920)
	if !strings.Contains(got, "pad=1080:1920") {
		t.Errorf("frame terlihat sebagian harus mengisi sisa bingkai, dapat: %s", got)
	}
	// Tidak boleh melebihi bingkai: sisi yang kelebihan dipotong lebih dulu.
	if !strings.Contains(got, "crop='min(iw,1080)':'min(ih,1920)'") {
		t.Errorf("nilai tengah harus memotong sisi yang melebihi bingkai, dapat: %s", got)
	}
}

// Koma di dalam min()/max() harus terlindung kutip tunggal. Di dalam kutip
// ffmpeg TIDAK menafsirkan backslash, jadi meng-escape koma justru merusaknya.
func TestReframeFilterQuotesExpressionsWithoutBackslash(t *testing.T) {
	got := ReframeFilter(Layout{Mode: "center", FrameVisible: 50, PictureSize: 100}, 1080, 1920)
	if strings.Contains(got, `\,`) {
		t.Errorf("koma tidak boleh di-escape di dalam kutip: %s", got)
	}
	for _, expr := range []string{`w='trunc(`, `h='trunc(`, `crop='min(`} {
		if !strings.Contains(got, expr) {
			t.Errorf("ekspresi %q tidak dikutip: %s", expr, got)
		}
	}
}

// --- geometri sungguhan lewat ffmpeg ---

// scaledSize menjalankan HANYA tahap penskalaan zoom pada sumber sintetis, lalu
// melaporkan ukuran hasilnya. Inilah yang membuktikan arah sumbu zoom benar —
// mencocokkan string filter saja tidak membuktikan apa pun soal geometri.
func scaledSize(t *testing.T, visible, srcW, srcH, targetW, targetH int) (int, int) {
	t.Helper()
	chain := fitChain(visible, targetW, targetH)
	// Tahap crop dibuang: yang diukur besar gambarnya, bukan hasil potongannya.
	if i := strings.Index(chain, ",crop="); i >= 0 {
		chain = chain[:i]
	}

	out := t.TempDir() + "/frame.png"
	cmd := exec.CommandContext(context.Background(), "ffmpeg", "-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc=size="+itoa(srcW)+"x"+itoa(srcH)+":duration=0.1:rate=1",
		"-frames:v", "1", "-vf", chain, out)
	if err := cmd.Run(); err != nil {
		t.Fatalf("ffmpeg menolak rantai filter %q: %v", chain, err)
	}

	probe, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-of", "json", out).Output()
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Streams []struct{ Width, Height int } `json:"streams"`
	}
	if err := json.Unmarshal(probe, &parsed); err != nil || len(parsed.Streams) == 0 {
		t.Fatalf("tidak bisa membaca dimensi hasil: %v", err)
	}
	return parsed.Streams[0].Width, parsed.Streams[0].Height
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// Inti perbaikan: FrameVisible 100 memperlihatkan frame asli utuh, dan
// menurunkannya memperbesar gambar secara monoton sampai memenuhi bingkai di 0.
//
// Sumber 1920x1080 (16:9) ke bingkai 1080x1920 (9:16):
//   - visible 100 → lebar pas 1080, tinggi 608  (seluruh frame terlihat)
//   - visible 0   → tinggi pas 1920, lebar 3413 (bingkai penuh, sisi terpotong)
func TestFrameVisibleAxisGrowsAsItFalls(t *testing.T) {
	requireFFmpeg(t)
	const tw, th = 1080, 1920
	const sw, sh = 1920, 1080

	wWhole, hWhole := scaledSize(t, 100, sw, sh, tw, th)
	// Contain: seluruh frame asli masuk, tidak ada sisi yang melebihi bingkai.
	if wWhole > tw || hWhole > th {
		t.Errorf("visible 100 = %dx%d, melebihi bingkai %dx%d — ada yang terpotong", wWhole, hWhole, tw, th)
	}
	if wWhole != tw {
		t.Errorf("visible 100 lebar = %d, ingin pas %d (sumber lanskap menyentuh sisi kiri-kanan)", wWhole, tw)
	}

	wFull, hFull := scaledSize(t, 0, sw, sh, tw, th)
	// Cover: bingkai tertutup penuh.
	if wFull < tw || hFull < th {
		t.Errorf("visible 0 = %dx%d, tidak menutupi bingkai %dx%d", wFull, hFull, tw, th)
	}

	// Monoton: MENURUNKAN visible harus memperbesar gambar, tidak pernah mengecil.
	prevW, prevH := wWhole, hWhole
	for _, v := range []int{75, 50, 25, 0} {
		w, h := scaledSize(t, v, sw, sh, tw, th)
		if w <= prevW || h <= prevH {
			t.Errorf("visible %d = %dx%d, tidak lebih besar dari langkah sebelumnya %dx%d", v, w, h, prevW, prevH)
		}
		prevW, prevH = w, h
	}

	// Dimensi selalu genap — h264 menolak yang ganjil.
	for _, v := range []int{5, 35, 65, 95} {
		w, h := scaledSize(t, v, sw, sh, tw, th)
		if w%2 != 0 || h%2 != 0 {
			t.Errorf("visible %d menghasilkan dimensi ganjil %dx%d", v, w, h)
		}
	}
}

// Sumber potret ke bingkai potret: sumbunya harus tetap benar arahnya walau
// peran lebar & tinggi bertukar.
func TestFrameVisibleAxisWorksForPortraitSource(t *testing.T) {
	requireFFmpeg(t)
	const tw, th = 1080, 1920
	const sw, sh = 1080, 1350 // 4:5, lebih persegi daripada bingkainya

	wWhole, hWhole := scaledSize(t, 100, sw, sh, tw, th)
	if wWhole > tw || hWhole > th {
		t.Errorf("visible 100 = %dx%d, melebihi bingkai", wWhole, hWhole)
	}
	wFull, hFull := scaledSize(t, 0, sw, sh, tw, th)
	if wFull < tw || hFull < th {
		t.Errorf("visible 0 = %dx%d, tidak menutupi bingkai", wFull, hFull)
	}
	if wFull <= wWhole || hFull <= hWhole {
		t.Errorf("visible 0 (%dx%d) harus lebih besar dari visible 100 (%dx%d)", wFull, hFull, wWhole, hWhole)
	}
}

// PictureSize adalah sumbu KEDUA dan benar-benar berdiri sendiri: ia mengecilkan
// gambar di dalam bingkai TANPA mengubah apa yang terpotong. Inilah tampilan
// yang sempat hilang saat sumbu zoom ditulis ulang.
func TestPictureSizeShrinksWithoutChangingTheCrop(t *testing.T) {
	requireFFmpeg(t)
	const tw, th = 1080, 1920

	// Potongan penuh (visible 0) di kotak separuh bingkai.
	half := ReframeFilter(Layout{Mode: "center", Background: "black", FrameVisible: 0, PictureSize: 50}, tw, th)
	if !strings.Contains(half, "crop=540:960") {
		t.Errorf("PictureSize 50 harus memotong ke kotak 540x960, dapat: %s", half)
	}
	if !strings.Contains(half, "pad=1080:1920") {
		t.Errorf("gambar yang mengecil harus dibantali sampai ukuran bingkai, dapat: %s", half)
	}

	// Rasio potongannya sama dengan saat memenuhi bingkai — yang berubah cuma
	// ukurannya. 540:960 dan 1080:1920 sama-sama 9:16.
	full := ReframeFilter(Layout{Mode: "center", Background: "black", FrameVisible: 0, PictureSize: 100}, tw, th)
	if !strings.Contains(full, "crop=1080:1920") {
		t.Errorf("PictureSize 100 harus memotong ke ukuran bingkai, dapat: %s", full)
	}
}

// Tampilan lama yang dilaporkan hilang harus bisa dihasilkan lagi PERSIS.
// Rantai ini disalin dari implementasi sebelum sumbu zoom ditulis ulang.
func TestOldShrunkLookIsReproducibleExactly(t *testing.T) {
	const wantOldZoom50 = "scale=540:960:force_original_aspect_ratio=increase:flags=lanczos,crop=540:960" +
		",pad=1080:1920:(ow-iw)/2:(oh-ih)/2:black"
	got := ReframeFilter(Layout{Mode: "center", Background: "black", FrameVisible: 0, PictureSize: 50}, 1080, 1920)
	if got != wantOldZoom50 {
		t.Errorf("tampilan lama tidak kembali persis:\n dapat: %s\n ingin: %s", got, wantOldZoom50)
	}

	// "fit" lama = frame utuh memenuhi bingkai.
	const wantOldFit = "scale=1080:1920:force_original_aspect_ratio=decrease:flags=lanczos" +
		",pad=1080:1920:(ow-iw)/2:(oh-ih)/2:black"
	got = ReframeFilter(Layout{Mode: "center", Background: "black", FrameVisible: 100, PictureSize: 100}, 1080, 1920)
	if got != wantOldFit {
		t.Errorf("\"fit\" lama tidak kembali persis:\n dapat: %s\n ingin: %s", got, wantOldFit)
	}
}

func requireFFmpeg(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skip(bin + " tidak tersedia")
		}
	}
}

// topCentrePixel merender satu frame dari klip lalu membaca piksel di dekat tepi
// ATAS-tengah — tempat yang jadi ruang kosong saat zoom rendah.
//
// format=rgb24 WAJIB mendahului crop: pada sumber yuv420p, chroma-nya
// disubsampel, dan memotong petak sekecil ini sebelum konversi membuat ffmpeg
// menghitung lebar 0 lalu gagal ("non positive size for width").
func topCentrePixel(t *testing.T, clip string) (r, g, b byte) {
	t.Helper()
	out, err := exec.Command("ffmpeg", "-v", "error", "-i", clip,
		"-frames:v", "1", "-vf", "format=rgb24,crop=2:2:540:10",
		"-f", "rawvideo", "-pix_fmt", "rgb24", "-").Output()
	if err != nil || len(out) < 3 {
		t.Fatalf("tidak bisa membaca piksel dari %s: %v", clip, err)
	}
	return out[0], out[1], out[2]
}

// Uji end-to-end lewat ClipReframe: rantai filter lengkap (skala + crop + pad)
// harus tetap menghasilkan bingkai berukuran tepat di SEMUA nilai zoom, dan
// ruang kosong di zoom rendah benar-benar terisi latar.
func TestClipReframeFillsFrameAtEverySetting(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	src := dir + "/src.mp4"
	if err := exec.Command("ffmpeg", "-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc=size=1920x1080:duration=1:rate=10",
		"-pix_fmt", "yuv420p", src).Run(); err != nil {
		t.Fatal(err)
	}

	c := New("ffmpeg", "ffprobe")
	const tw, th = 1080, 1920
	cases := []struct {
		name             string
		visible, picture int
		wantDarkTop      bool
	}{
		{"isi-penuh", 0, 100, false}, // menutupi bingkai → tepi atas bergambar
		{"separuh-frame", 50, 100, true},
		{"frame-utuh", 100, 100, true}, // ada pita latar di atas & bawah
		{"mengecil", 0, 50, true},      // potongan penuh, tapi mengecil di tengah
	}
	for _, tc := range cases {
		out := dir + "/clip_" + tc.name + ".mp4"
		enc := EncodeOpts{CRF: "30", Preset: "ultrafast", Mode: "center", Background: "black",
			FrameVisible: tc.visible, PictureSize: tc.picture}
		if err := c.ClipReframe(context.Background(), src, 0, 0.5, tw, th, enc, out); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		w, h, err := c.Dimensions(context.Background(), out)
		if err != nil {
			t.Fatal(err)
		}
		if w != tw || h != th {
			t.Errorf("%s menghasilkan %dx%d, ingin %dx%d", tc.name, w, h, tw, th)
		}

		// Tepi atas: hitam saat masih ada ruang kosong, bergambar saat penuh.
		r, g, b := topCentrePixel(t, out)
		dark := r < 24 && g < 24 && b < 24
		if dark != tc.wantDarkTop {
			t.Errorf("%s: tepi atas gelap=%v (%d,%d,%d), ingin gelap=%v",
				tc.name, dark, r, g, b, tc.wantDarkTop)
		}
	}
}
