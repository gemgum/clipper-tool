package segment

import (
	"regexp"
	"strings"

	"github.com/gemgum/clipper/engine/internal/types"
)

// Cuplikan pembuka: banyak video dibuka dengan potongan-potongan pendek dari
// sepanjang video yang disambung jadi satu. Terdengar ramai — memang dirancang
// begitu — tapi berganti topik tiap beberapa detik dan tidak membahas apa pun
// sampai tuntas. Klip yang dipotong dari sana jadi trailer untuk video yang
// tidak akan ditonton siapa pun.
//
// Prompt saja tidak cukup. Diuji langsung: setelah aturan "jangan ambil dari
// cuplikan" ditambahkan ke prompt, qwen2.5 TETAP memilih momen 41-57 detik dari
// cuplikan. Model lokal tidak terikat instruksi panjang — sama seperti waktu ia
// membalas satu entri untuk 32 segmen sampai dipaksa JSON Schema.
//
// Yang membedakan cuplikan bukan ketidaknyambungannya. Itu sempat diukur dan
// GAGAL: kesinambungan kata antar-paruh di jendela cuplikan 0,22 dan 0,07,
// sementara banyak jendela isi justru 0,00. Percakapan spontan memang berpindah
// topik.
//
// Yang membedakannya: cuplikan adalah PRATAYANG. Hampir seluruh kosakatanya
// muncul lagi jauh di belakang, sebab ia memang dipotong dari sana. Terukur di
// tiga video:
//
//	cuplikan (video dengan cuplikan) : 0,97 dan 0,95
//	seluruh jendela isi, tiga video  : 0,33 - 0,81
//	90 detik pertama, dua video lain : 0,33 - 0,76 (memang tidak bercuplikan)
const (
	// previewThreshold = ambang "hampir semuanya muncul lagi nanti".
	//
	// 0,90 diambil di antara 0,81 (jendela isi tertinggi yang terukur) dan 0,95
	// (cuplikan terendah yang terukur), condong ke sisi aman: lebih baik satu
	// cuplikan lolos daripada satu momen isi yang bagus dibuang.
	previewThreshold = 0.90

	// previewOpening = sejauh mana dari awal video pemeriksaan ini berlaku.
	//
	// Cuplikan selalu di pembuka. Membatasi pemeriksaan ke sini membuat momen di
	// tengah video tidak mungkin salah tuduh, berapa pun kosakatanya bergema.
	previewOpening = 180.0

	// previewGap = jarak yang dianggap "jauh di belakang". Di bawah ini
	// pengulangan kata cuma tanda pembicaraan masih di topik yang sama.
	previewGap = 120.0

	// previewMinWords = kosakata terkecil yang layak dinilai. Di bawah ini satu
	// dua kata sudah menggeser rasionya jauh.
	previewMinWords = 8
)

// reWord memilih kata yang cukup panjang untuk membawa makna. Kata pendek di
// bahasa Indonesia hampir semuanya kata tugas (yang, itu, dan, ke) — ia muncul
// di mana saja dan tidak menandakan topik apa pun.
var reWord = regexp.MustCompile(`[a-zA-Z]{6,}`)

func vocab(tr types.Transcript, from, to float64) map[string]bool {
	out := map[string]bool{}
	for _, s := range tr.Segments {
		if s.Start < from || s.End > to {
			continue
		}
		for _, w := range reWord.FindAllString(strings.ToLower(s.Text), -1) {
			out[w] = true
		}
	}
	return out
}

// IsOpeningPreview menilai apakah rentang ini bagian dari cuplikan pembuka.
//
// Sengaja mengembalikan bool, bukan skor: pemanggilnya cuma perlu memutuskan
// pakai atau tidak, dan membocorkan angkanya mengundang pemanggil membuat
// ambangnya sendiri-sendiri.
func IsOpeningPreview(tr types.Transcript, start, end float64) bool {
	if start > previewOpening {
		return false
	}
	here := vocab(tr, start, end)
	if len(here) < previewMinWords {
		return false
	}
	var last float64
	if n := len(tr.Segments); n > 0 {
		last = tr.Segments[n-1].End
	}
	later := vocab(tr, end+previewGap, last)
	if len(later) == 0 {
		return false // tidak ada "nanti" untuk dibandingkan
	}
	echoed := 0
	for w := range here {
		if later[w] {
			echoed++
		}
	}
	return float64(echoed)/float64(len(here)) >= previewThreshold
}
