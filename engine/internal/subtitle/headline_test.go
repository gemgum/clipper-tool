package subtitle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/types"
)

func headlineWatermark() config.Watermark {
	b := config.DefaultWatermark()
	b.Image = "/tmp/banner.png"
	b.Width = 92
	b.Headline.Text = "RINZ KENA MENTAL?"
	return b
}

// Headline dipenggal memakai lebar BINGKAI, dan ukuran font yang menentukan
// berapa karakter yang muat. Acuannya BUKAN kotak watermark: sejak kotak itu
// berbawaan seperempat bingkai, memakainya menghasilkan empat karakter per baris.
func TestHeadlineLinesFollowFrameWidth(t *testing.T) {
	text := "BEGINI KATA MIDLANER ANDALAN RRQ HOSHI SOAL KEKALAHAN KEMARIN"
	small := headlineLines(text, 40)
	big := headlineLines(text, 96)
	if len(big) <= len(small) {
		t.Fatalf("font besar harus menghasilkan lebih banyak baris: %d vs %d", len(big), len(small))
	}
	// 1080 - 2*60 margin, huruf 0,6 x ukuran → 40 karakter pada ukuran 40.
	for _, ln := range small {
		if len(ln) > 40 {
			t.Fatalf("baris melewati lebar bingkai: %q", ln)
		}
	}
}

// Baris yang diketik pengguna sendiri dihormati: kalau ia menekan enter, di
// situlah barisnya patah.
func TestHeadlineLinesKeepManualBreaks(t *testing.T) {
	got := headlineLines("RINZ\nKENA MENTAL", 64)
	if len(got) != 2 || got[0] != "RINZ" {
		t.Fatalf("pemenggalan manual hilang: %#v", got)
	}
}

// Headline ditulis sebagai style KEDUA di .ass yang sama, pada Layer 1 supaya ia
// berada di atas subtitle bila keduanya bertumpang tindih.
func TestWriteASSAddsHeadline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.ass")
	segs := []types.TranscriptSegment{{Start: 0, End: 3, Text: "halo semuanya"}}
	if err := WriteASS(path, segs, 0, config.DefaultSubtitle(), headlineWatermark(), "RINZ KENA MENTAL?", 30); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	s := string(out)
	if !strings.Contains(s, "Style: Headline,") {
		t.Fatalf("style headline tidak ditulis:\n%s", s)
	}
	if !strings.Contains(s, "Dialogue: 1,") || !strings.Contains(s, ",Headline,,") {
		t.Fatalf("baris headline tidak ditulis di Layer 1:\n%s", s)
	}
	// For = 0 → sampai klip habis (30 detik), bukan sampai segmen terakhir.
	if !strings.Contains(s, "0:00:30.00") {
		t.Fatalf("headline tidak bertahan sampai akhir klip:\n%s", s)
	}
}

// Tanpa teks, .ass harus sama seperti sebelum fitur ini ada — tidak ada style
// menganggur, tidak ada baris kosong.
func TestWriteASSWithoutHeadlineStaysPlain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.ass")
	segs := []types.TranscriptSegment{{Start: 0, End: 3, Text: "halo semuanya"}}
	if err := WriteASS(path, segs, 0, config.DefaultSubtitle(), config.DefaultWatermark(), "", 30); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if strings.Contains(string(out), "Headline") {
		t.Fatalf("headline muncul padahal teksnya kosong:\n%s", out)
	}
}

// Rentang waktu: At + For jadi ujung akhirnya.
func TestWriteASSHeadlineWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.ass")
	b := headlineWatermark()
	b.At, b.For = 2, 5
	if err := WriteASS(path, nil, 0, config.DefaultSubtitle(), b, "HALO", 30); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "Dialogue: 1,0:00:02.00,0:00:07.00,Headline") {
		t.Fatalf("jendela waktu headline salah:\n%s", out)
	}
}
