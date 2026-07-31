package subtitle

import (
	"os"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/types"
)

// utterance membuat segmen dengan kata berdurasi tetap mulai detik start.
func utterance(start, per float64, words ...string) types.TranscriptSegment {
	seg := types.TranscriptSegment{Start: start, Text: strings.Join(words, " ")}
	t := start
	for _, w := range words {
		seg.Words = append(seg.Words, types.Word{Start: t, End: t + per, Text: w})
		t += per
	}
	seg.End = t
	return seg
}

func TestBuildPagesHasNoFlashPages(t *testing.T) {
	// Ucapan cepat (0,25 dtk/kata) — dulu menghasilkan tampilan <0,5 detik.
	segs := []types.TranscriptSegment{
		utterance(0, 0.25, "hari", "ini", "kita", "bahas", "soal", "uang", "dan", "cara", "mengaturnya", "dengan", "benar."),
		utterance(3.0, 0.25, "itu."),
	}
	sub := config.DefaultSubtitle()
	minDur, maxLines := sub.Pacing()
	pages := buildPages(collectWords(segs, 0), maxCharsPerLine(sub.Size), maxLines, minDur)

	if len(pages) == 0 {
		t.Fatal("tidak ada tampilan dihasilkan")
	}
	for i, p := range pages {
		if d := p.end - p.start; d < 0.9 {
			t.Errorf("tampilan %d hanya %.2f detik (terlalu cepat dibaca)", i, d)
		}
	}
	// Tampilan tidak boleh saling tumpang tindih.
	for i := 1; i < len(pages); i++ {
		if pages[i].start < pages[i-1].end {
			t.Errorf("tampilan %d mulai (%.2f) sebelum %d selesai (%.2f)",
				i, pages[i].start, i-1, pages[i-1].end)
		}
	}
}

func TestBuildPagesLosesNoWords(t *testing.T) {
	segs := []types.TranscriptSegment{
		utterance(0, 0.3, "satu", "dua", "tiga", "empat", "lima", "enam", "tujuh", "delapan", "sembilan", "sepuluh"),
	}
	pages := buildPages(collectWords(segs, 0), 20, 2, 1.2)
	var got []string
	for _, p := range pages {
		for _, ln := range p.lines {
			for _, w := range ln {
				got = append(got, w.Text)
			}
		}
	}
	want := "satu dua tiga empat lima enam tujuh delapan sembilan sepuluh"
	if strings.Join(got, " ") != want {
		t.Errorf("kata berubah/hilang:\n dapat: %s\n ingin: %s", strings.Join(got, " "), want)
	}
}

func TestTimingFollowsWordTimestamps(t *testing.T) {
	// Kata pertama mulai detik 5 dalam klip yang dimulai detik 3 → relatif 2 dtk.
	segs := []types.TranscriptSegment{utterance(5, 0.5, "halo", "dunia")}
	words := collectWords(segs, 3)
	if len(words) != 2 {
		t.Fatalf("dapat %d kata", len(words))
	}
	if words[0].Start < 1.99 || words[0].Start > 2.01 {
		t.Errorf("waktu mulai relatif salah: %.2f (ingin 2.00)", words[0].Start)
	}
}

func TestColorIsNotForcedToYellow(t *testing.T) {
	sub := config.DefaultSubtitle()
	sub.Color = "white"
	sub.Mode = config.SubKaraoke
	head := assHeader(sub)
	// PrimaryColour (kolom ke-4 baris Style) harus tetap putih.
	for _, line := range strings.Split(head, "\n") {
		if !strings.HasPrefix(line, "Style: Default,") {
			continue
		}
		cols := strings.Split(line, ",")
		if cols[3] != "&H00FFFFFF" {
			t.Errorf("warna dasar berubah jadi %s padahal pengguna memilih putih", cols[3])
		}
	}
	if hl := highlightColor(sub); hl != "&H0000FFFF" {
		t.Errorf("warna sorot = %s, ingin kuning", hl)
	}
}

func TestWordModeShowsOneWordPerPage(t *testing.T) {
	segs := []types.TranscriptSegment{utterance(0, 0.4, "satu", "dua", "tiga")}
	sub := config.DefaultSubtitle()
	sub.Mode = config.SubWord

	path := t.TempDir() + "/w.ass"
	if err := WriteASS(path, segs, 0, sub); err != nil {
		t.Fatal(err)
	}
	var dialogue []string
	for _, l := range strings.Split(readFile(t, path), "\n") {
		if strings.HasPrefix(l, "Dialogue:") {
			dialogue = append(dialogue, l)
		}
	}
	if len(dialogue) != 3 {
		t.Fatalf("dapat %d baris Dialogue, ingin 3 (satu per kata)", len(dialogue))
	}
	for i, want := range []string{"satu", "dua", "tiga"} {
		if !strings.HasSuffix(dialogue[i], want) {
			t.Errorf("baris %d: %q tidak berakhir dengan kata %q", i, dialogue[i], want)
		}
	}
}

func TestKaraokeModeHighlightsActiveWord(t *testing.T) {
	segs := []types.TranscriptSegment{utterance(0, 0.4, "satu", "dua", "tiga")}
	sub := config.DefaultSubtitle()
	sub.Mode = config.SubKaraoke

	path := t.TempDir() + "/k.ass"
	if err := WriteASS(path, segs, 0, sub); err != nil {
		t.Fatal(err)
	}
	body := readFile(t, path)
	// Tiap kata dapat gilirannya disorot, seluruh teks tetap terlihat.
	for _, w := range []string{"satu", "dua", "tiga"} {
		if !strings.Contains(body, "&}"+w+"{") {
			t.Errorf("kata %q tidak pernah disorot", w)
		}
	}
	n := strings.Count(body, "Dialogue:")
	if n != 3 {
		t.Errorf("dapat %d Dialogue, ingin 3 (satu per pergantian sorot)", n)
	}
}

// Pacing mengikuti nilai kecepatan bahasa Inggris; nilai lama bahasa Indonesia
// tidak lagi dikenali dan jatuh ke normal.
func TestPacingFollowsSpeed(t *testing.T) {
	sub := config.DefaultSubtitle()
	sub.Speed = config.SpeedSlow
	slowDur, slowLines := sub.Pacing()
	sub.Speed = config.SpeedDense
	denseDur, denseLines := sub.Pacing()
	if slowDur <= denseDur {
		t.Errorf("slow (%.1f) harus menahan tampilan lebih lama dari dense (%.1f)", slowDur, denseDur)
	}
	if slowLines >= denseLines {
		t.Errorf("slow (%d baris) harus lebih sedikit dari dense (%d baris)", slowLines, denseLines)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
