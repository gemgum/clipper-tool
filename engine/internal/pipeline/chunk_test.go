package pipeline

import (
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/score/llm"
	"github.com/gemgum/clipper/engine/internal/types"
)

// transcript sintetis: segmen 5 detik sepanjang total detik.
func transcript(total float64) types.Transcript {
	var tr types.Transcript
	for t := 0.0; t < total; t += 5 {
		tr.Segments = append(tr.Segments, types.TranscriptSegment{
			Start: t, End: t + 5, Text: "kalimat.",
		})
	}
	return tr
}

func TestShortTranscriptIsNotChunked(t *testing.T) {
	parts := chunkTranscript(transcript(600), 720, 120) // 10 mnt, batas 12 mnt
	if len(parts) != 1 {
		t.Fatalf("dapat %d potongan, ingin 1", len(parts))
	}
	if parts[0].info.Total != 1 {
		t.Errorf("info.Total = %d, ingin 1", parts[0].info.Total)
	}
}

func TestLongTranscriptChunksOverlap(t *testing.T) {
	parts := chunkTranscript(transcript(3600), 720, 120) // 60 mnt, potongan 12 mnt
	if len(parts) < 5 {
		t.Fatalf("hanya %d potongan untuk 60 menit", len(parts))
	}
	for i, p := range parts {
		if p.info.Index != i+1 || p.info.Total != len(parts) {
			t.Errorf("penomoran potongan salah: %+v", p.info)
		}
		if len(p.tr.Segments) == 0 {
			t.Errorf("potongan %d kosong", i+1)
		}
	}
	// Potongan berikutnya harus mundur (tumpang tindih), bukan menyambung pas.
	for i := 1; i < len(parts); i++ {
		if parts[i].info.Start >= parts[i-1].info.End {
			t.Errorf("potongan %d mulai %.0f, tidak tumpang tindih dengan akhir %.0f",
				i+1, parts[i].info.Start, parts[i-1].info.End)
		}
	}
	// Seluruh durasi harus tercakup sampai ujung.
	if last := parts[len(parts)-1].info.End; last < 3595 {
		t.Errorf("potongan terakhir berakhir di %.0f, video 3600 detik", last)
	}
}

func TestSplitMomentIsRejoined(t *testing.T) {
	// Momen berakhir tepat di batas potongan & ditandai "continues",
	// lalu potongan berikutnya melanjutkannya.
	ms := []llm.Moment{
		{Start: 600, End: 720, Score: 80, Title: "part A", Continues: true},
		{Start: 720, End: 790, Score: 70, Title: "part B"},
	}
	out := mergeMoments(ms)
	if len(out) != 1 {
		t.Fatalf("dapat %d klip, ingin 1 (tersambung)", len(out))
	}
	if out[0].Start != 600 || out[0].End != 790 {
		t.Errorf("rentang tersambung %.0f-%.0f, ingin 600-790", out[0].Start, out[0].End)
	}
}

func TestDuplicatesFromOverlapAreDropped(t *testing.T) {
	// Momen sama muncul di dua potongan (area tumpang tindih).
	ms := []llm.Moment{
		{Start: 640, End: 700, Score: 60, Title: "chunk 1 version"},
		{Start: 640, End: 700, Score: 75, Title: "chunk 2 version"},
	}
	out := mergeMoments(ms)
	if len(out) != 1 {
		t.Fatalf("dapat %d klip, ingin 1 (duplikat dibuang)", len(out))
	}
	if out[0].Score != 75 {
		t.Errorf("skor %.0f, ingin 75 (yang tertinggi dipertahankan)", out[0].Score)
	}
}

func TestSeparateMomentsAreNotMerged(t *testing.T) {
	ms := []llm.Moment{
		{Start: 0, End: 60, Score: 80},
		{Start: 200, End: 260, Score: 70},
	}
	if out := mergeMoments(ms); len(out) != 2 {
		t.Fatalf("dapat %d klip, ingin 2 (terpisah jauh)", len(out))
	}
}

func TestValidateRejectsNonsenseBoundaries(t *testing.T) {
	tr := transcript(600)
	ms := []llm.Moment{
		{Start: 100, End: 60},                             // terbalik
		{Start: 50, End: 52},                              // terlalu pendek
		{Start: 900, End: 1000},                           // di luar durasi video
		{Start: 100, End: 160, Score: 80, Title: "title"}, // valid
	}
	ok, rejected, err := validateMoments(ms, tr, "test engine")
	if err != nil {
		t.Fatalf("tak seharusnya gagal, masih ada 1 momen valid: %v", err)
	}
	if len(ok) != 1 || len(rejected) != 3 {
		t.Errorf("valid=%d rejected=%d, ingin 1 dan 3", len(ok), len(rejected))
	}
}

func TestValidateFailsWhenEverythingIsNonsense(t *testing.T) {
	tr := transcript(600)
	ms := []llm.Moment{{Start: 5000, End: 5060}}
	_, _, err := validateMoments(ms, tr, "Ollama (qwen2.5)")
	if err == nil {
		t.Fatal("harus gagal — tidak boleh diam-diam beralih ke heuristik")
	}
	// Pesan harus menyebut mesinnya supaya pengguna tahu akar masalahnya.
	if got := err.Error(); !strings.Contains(got, "Ollama (qwen2.5)") {
		t.Errorf("pesan error tidak menyebut mesin: %s", got)
	}
}
