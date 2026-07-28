package pipeline

import (
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/score/llm"
	"github.com/gemgum/clipper/engine/internal/types"
)

// transkrip sintetis: segmen 5 detik sepanjang total detik.
func transkrip(total float64) types.Transcript {
	var tr types.Transcript
	for t := 0.0; t < total; t += 5 {
		tr.Segments = append(tr.Segments, types.TranscriptSegment{
			Start: t, End: t + 5, Text: "kalimat.",
		})
	}
	return tr
}

func TestChunkPendekTidakDipecah(t *testing.T) {
	parts := chunkTranscript(transkrip(600), 720, 120) // 10 mnt, batas 12 mnt
	if len(parts) != 1 {
		t.Fatalf("dapat %d potongan, ingin 1", len(parts))
	}
	if parts[0].info.Total != 1 {
		t.Errorf("info.Total = %d, ingin 1", parts[0].info.Total)
	}
}

func TestChunkPanjangBertumpangTindih(t *testing.T) {
	parts := chunkTranscript(transkrip(3600), 720, 120) // 60 mnt, potongan 12 mnt
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

func TestMomenTerbelahDisambung(t *testing.T) {
	// Momen berakhir tepat di batas potongan & ditandai berlanjut,
	// lalu potongan berikutnya melanjutkannya.
	ms := []llm.Moment{
		{Start: 600, End: 720, Score: 80, Title: "bagian A", Berlanjut: true},
		{Start: 720, End: 790, Score: 70, Title: "bagian B"},
	}
	out := mergeMoments(ms)
	if len(out) != 1 {
		t.Fatalf("dapat %d klip, ingin 1 (tersambung)", len(out))
	}
	if out[0].Start != 600 || out[0].End != 790 {
		t.Errorf("rentang tersambung %.0f-%.0f, ingin 600-790", out[0].Start, out[0].End)
	}
}

func TestDuplikatDariTumpangTindihDibuang(t *testing.T) {
	// Momen sama muncul di dua potongan (area tumpang tindih).
	ms := []llm.Moment{
		{Start: 640, End: 700, Score: 60, Title: "versi potongan 1"},
		{Start: 640, End: 700, Score: 75, Title: "versi potongan 2"},
	}
	out := mergeMoments(ms)
	if len(out) != 1 {
		t.Fatalf("dapat %d klip, ingin 1 (duplikat dibuang)", len(out))
	}
	if out[0].Score != 75 {
		t.Errorf("skor %.0f, ingin 75 (yang tertinggi dipertahankan)", out[0].Score)
	}
}

func TestMomenTerpisahTidakDigabung(t *testing.T) {
	ms := []llm.Moment{
		{Start: 0, End: 60, Score: 80},
		{Start: 200, End: 260, Score: 70},
	}
	if out := mergeMoments(ms); len(out) != 2 {
		t.Fatalf("dapat %d klip, ingin 2 (terpisah jauh)", len(out))
	}
}

func TestValidasiMenolakBatasNgawur(t *testing.T) {
	tr := transkrip(600)
	ms := []llm.Moment{
		{Start: 100, End: 60},                             // terbalik
		{Start: 50, End: 52},                              // terlalu pendek
		{Start: 900, End: 1000},                           // di luar durasi video
		{Start: 100, End: 160, Score: 80, Title: "judul"}, // valid
	}
	ok, ditolak, err := validateMoments(ms, tr, "mesin uji")
	if err != nil {
		t.Fatalf("tak seharusnya gagal, masih ada 1 momen valid: %v", err)
	}
	if len(ok) != 1 || len(ditolak) != 3 {
		t.Errorf("valid=%d ditolak=%d, ingin 1 dan 3", len(ok), len(ditolak))
	}
}

func TestValidasiGagalBilaSemuaNgawur(t *testing.T) {
	tr := transkrip(600)
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
