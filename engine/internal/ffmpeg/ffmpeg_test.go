package ffmpeg

import "strings"

import "testing"

// Kasus nyata: render gagal dengan exit 254 tapi pesannya hanya berisi
// statistik x264, karena dulu yang diambil 500 karakter TERAKHIR.
func TestRingkasGalatAmbilBarisPenting(t *testing.T) {
	stderr := `frame= 1234 fps=45 q=28.0 size=    3131kB
[libx264 @ 0x55] frame I:12    Avg QP:18.44  size: 41830
Error opening output file /home/x/data/job_0001/clip_14.mp4.
Error opening output files: No such file or directory
[libx264 @ 0x55] ref B L1: 96.0%  4.0%
[libx264 @ 0x55] kb/s:1336.65
[aac @ 0x66] Qavg: 1167.742
Conversion failed!
`
	got := ringkasGalat(stderr)
	if !strings.Contains(got, "No such file or directory") {
		t.Errorf("sebab sebenarnya hilang dari pesan: %s", got)
	}
	if strings.Contains(got, "kb/s") || strings.Contains(got, "Qavg") {
		t.Errorf("statistik encoder seharusnya tidak ikut: %s", got)
	}
	if !strings.Contains(got, "terhapus/dipindah saat render") {
		t.Errorf("petunjuk tindak lanjut tidak muncul: %s", got)
	}
}

func TestRingkasGalatTanpaBarisCocok(t *testing.T) {
	// Tidak ada baris bertanda galat: jatuh ke ekor keluaran, jangan kosong.
	if got := ringkasGalat("frame= 10 fps=25\nframe= 20 fps=25\n"); got == "" {
		t.Error("pesan tidak boleh kosong saat tak ada baris yang cocok")
	}
	// "Conversion failed!" sendirian tidak menjelaskan apa pun, tapi tetap
	// lebih baik daripada pesan kosong.
	if got := ringkasGalat("Conversion failed!\n"); got == "" {
		t.Error("pesan tidak boleh kosong")
	}
}

func TestPetunjukGalat(t *testing.T) {
	cases := map[string]string{
		"Permission denied":                        "izin tulis",
		"No space left on device":                  "disk habis",
		"Invalid data found when processing input": "rusak",
	}
	for in, mau := range cases {
		if got := petunjukGalat(in); !strings.Contains(got, mau) {
			t.Errorf("petunjukGalat(%q) = %q, harus menyebut %q", in, got, mau)
		}
	}
	if got := petunjukGalat("galat yang tidak dikenali"); got != "" {
		t.Errorf("galat tak dikenal seharusnya tanpa petunjuk, dapat %q", got)
	}
}

// Rantai filter dipakai bersama render klip & preview satu frame. Tes ini
// menjaga keduanya tetap satu sumber — kalau salah satu mode diubah, yang lain
// ikut berubah dengan sendirinya.
func TestReframeFilter(t *testing.T) {
	center := ReframeFilter("center", 1080, 1920)
	if !strings.Contains(center, "force_original_aspect_ratio=increase") || !strings.Contains(center, "crop=1080:1920") {
		t.Errorf("center harus memperbesar lalu crop tengah, dapat: %s", center)
	}
	if strings.Contains(center, "gblur") {
		t.Errorf("center tidak boleh memakai latar blur: %s", center)
	}

	fit := ReframeFilter("fit", 1080, 1920)
	for _, bagian := range []string{
		"split=2[bg][fg]",                      // frame dipakai dua kali
		"gblur=sigma=20",                       // latar di-blur
		"force_original_aspect_ratio=decrease", // depan muat utuh, tanpa zoom
		"overlay=(W-w)/2:(H-h)/2",              // ditaruh persis di tengah
	} {
		if !strings.Contains(fit, bagian) {
			t.Errorf("fit kehilangan %q, dapat: %s", bagian, fit)
		}
	}

	// Mode tak dikenal jatuh ke center; penolakannya ditangani config.Reframe.Cek
	// supaya pesan errornya muncul sebelum ffmpeg dipanggil.
	if ReframeFilter("apa saja", 720, 1280) != ReframeFilter("center", 720, 1280) {
		t.Error("mode tak dikenal seharusnya sama dengan center")
	}
}
