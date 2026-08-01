package segment

import (
	"fmt"
	"testing"

	"github.com/gemgum/clipper/engine/internal/types"
)

// build menyusun transkrip dari daftar (mulai, teks) berdurasi 5 detik.
func build(lines []string, step float64) types.Transcript {
	tr := types.Transcript{Language: "id"}
	for i, s := range lines {
		tr.Segments = append(tr.Segments, types.TranscriptSegment{
			Start: float64(i) * step, End: float64(i)*step + step - 0.1, Text: s,
		})
	}
	return tr
}

// Cuplikan = pratayang: hampir seluruh kosakatanya muncul lagi jauh di belakang,
// sebab ia memang dipotong dari sana.
func TestOpeningPreviewIsRecognised(t *testing.T) {
	lines := []string{
		"pemerintah membangun universitas kebangsaan",
		"koruptor ditangkap kejaksaan minggu kemarin",
		"wartawan menuliskan berita meninggalnya penjaga",
	}
	// Isi video mengulang seluruhnya, jauh setelah cuplikan.
	for i := 0; i < 40; i++ {
		lines = append(lines, "percakapan biasa tentang keseharian bersama keluarga")
	}
	lines = append(lines,
		"pemerintah membangun universitas kebangsaan itu keliru sekali",
		"koruptor ditangkap kejaksaan minggu kemarin memang benar",
		"wartawan menuliskan berita meninggalnya penjaga rumahnya",
	)
	tr := build(lines, 5)
	if !IsOpeningPreview(tr, 0, 15) {
		t.Error("cuplikan pembuka tidak dikenali")
	}
}

// Pembuka yang memang isi — kosakatanya tidak bergema di belakang — harus lolos.
func TestRealOpeningIsKept(t *testing.T) {
	lines := []string{
		"selamat datang pembahasan konstitusi negara republik",
		"konstitusi mengatur hubungan pemerintah dengan rakyatnya",
		"perjanjian masyarakat melahirkan kewenangan bernegara",
	}
	for i := 0; i < 40; i++ {
		lines = append(lines, fmt.Sprintf("pembicaraan lainnya seputar perkara berbeda nomor %d puluhan", i))
	}
	tr := build(lines, 5)
	if IsOpeningPreview(tr, 0, 15) {
		t.Error("pembuka yang memang isi ikut dibuang")
	}
}

// Pemeriksaan hanya berlaku di pembuka. Momen di tengah video tidak boleh
// tertuduh, berapa pun kosakatanya bergema — di sana pengulangan kata justru
// tanda pembicaraan masih di topik yang sama.
func TestOnlyTheOpeningIsChecked(t *testing.T) {
	// Kosakatanya harus cukup banyak (lihat previewMinWords) supaya yang diuji
	// benar-benar batas posisinya, bukan tersangkut penjaga jumlah kata.
	var lines []string
	for i := 0; i < 120; i++ {
		lines = append(lines, "pembahasan konstitusi kebangsaan tentang kewenangan pemerintah "+
			"bersama masyarakat menjalankan perjanjian kehidupan berbangsa")
	}
	tr := build(lines, 5) // 600 detik
	if IsOpeningPreview(tr, 300, 330) {
		t.Error("momen di tengah video ikut diperiksa")
	}
	if !IsOpeningPreview(tr, 0, 30) {
		t.Error("prasyarat uji tidak terpenuhi: pembuka seharusnya terdeteksi bergema")
	}
}

// Rentang tanpa kosakata yang cukup tidak boleh dinilai: satu dua kata sudah
// menggeser rasionya jauh, dan menuduh berdasarkan itu asal-asalan.
func TestTooFewWordsIsNeverFlagged(t *testing.T) {
	tr := build([]string{"iya", "oh ya", "begitu"}, 5)
	if IsOpeningPreview(tr, 0, 15) {
		t.Error("rentang tanpa kata isi ikut dituduh cuplikan")
	}
}

// Tidak ada "nanti" untuk dibandingkan → tidak ada dasar menuduh.
func TestNothingAfterMeansNoVerdict(t *testing.T) {
	tr := build([]string{
		"pemerintah membangun universitas kebangsaan bersama",
		"koruptor ditangkap kejaksaan kemarin sekali",
	}, 5)
	if IsOpeningPreview(tr, 0, 10) {
		t.Error("dituduh cuplikan padahal tidak ada isi setelahnya")
	}
}
