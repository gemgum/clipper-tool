package ffmpeg

import (
	"strings"
	"testing"
)

// Kasus nyata: render gagal dengan exit 254 tapi pesannya hanya berisi
// statistik x264, karena dulu yang diambil 500 karakter TERAKHIR.
func TestSummarizeErrorKeepsImportantLines(t *testing.T) {
	stderr := `frame= 1234 fps=45 q=28.0 size=    3131kB
[libx264 @ 0x55] frame I:12    Avg QP:18.44  size: 41830
Error opening output file /home/x/data/job_0001/clip_14.mp4.
Error opening output files: No such file or directory
[libx264 @ 0x55] ref B L1: 96.0%  4.0%
[libx264 @ 0x55] kb/s:1336.65
[aac @ 0x66] Qavg: 1167.742
Conversion failed!
`
	got := summarizeError(stderr)
	if !strings.Contains(got, "No such file or directory") {
		t.Errorf("sebab sebenarnya hilang dari pesan: %s", got)
	}
	if strings.Contains(got, "kb/s") || strings.Contains(got, "Qavg") {
		t.Errorf("statistik encoder seharusnya tidak ikut: %s", got)
	}
	if !strings.Contains(got, "deleted or moved during the render") {
		t.Errorf("petunjuk tindak lanjut tidak muncul: %s", got)
	}
}

func TestSummarizeErrorWithoutMatchingLine(t *testing.T) {
	// Tidak ada baris bertanda galat: jatuh ke ekor keluaran, jangan kosong.
	if got := summarizeError("frame= 10 fps=25\nframe= 20 fps=25\n"); got == "" {
		t.Error("pesan tidak boleh kosong saat tak ada baris yang cocok")
	}
	// "Conversion failed!" sendirian tidak menjelaskan apa pun, tapi tetap
	// lebih baik daripada pesan kosong.
	if got := summarizeError("Conversion failed!\n"); got == "" {
		t.Error("pesan tidak boleh kosong")
	}
}

func TestErrorHint(t *testing.T) {
	cases := map[string]string{
		"Permission denied":                        "write permission denied",
		"No space left on device":                  "disk is full",
		"Invalid data found when processing input": "corrupt",
	}
	for in, want := range cases {
		if got := errorHint(in); !strings.Contains(got, want) {
			t.Errorf("errorHint(%q) = %q, harus menyebut %q", in, got, want)
		}
	}
	if got := errorHint("some unrecognised failure"); got != "" {
		t.Errorf("galat tak dikenal seharusnya tanpa petunjuk, dapat %q", got)
	}
}
