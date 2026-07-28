package llm

import (
	"fmt"
	"strings"

	"github.com/gemgum/clipper/engine/internal/types"
)

// Chunk menjelaskan posisi satu potongan transkrip dalam video utuh. Transkrip
// panjang dipecah agar muat di jendela konteks model; timestamp yang dikirim
// tetap ABSOLUT (detik asli video) supaya model tak perlu menghitung offset.
type Chunk struct {
	Index int     // urutan potongan, mulai 1
	Total int     // jumlah potongan
	Start float64 // detik awal potongan
	End   float64 // detik akhir potongan
}

// SystemPrompt menyusun instruksi yang dipakai SAMA PERSIS oleh Claude maupun
// Ollama, supaya hasil kedua mesin bisa dibandingkan. Ditulis detail karena
// keluaran model kini dijamin bentuknya (JSON Schema di Ollama) sehingga prompt
// panjang tidak lagi merusak format balasan.
func SystemPrompt(targetMin, targetMax float64, ch Chunk) string {
	var b strings.Builder
	b.WriteString(`Kamu kurator klip video viral untuk konten berbahasa Indonesia (TikTok, Reels, Shorts).
Kamu diberi transkrip bertimestamp (detik). Tugasmu MEMILIH momen terbaik untuk klip pendek vertikal.

ATURAN BATAS WAKTU — paling penting:
- 'start' dan 'end' WAJIB diambil dari angka timestamp yang benar-benar ADA di transkrip.
  'start' = angka awal sebuah baris, 'end' = angka akhir sebuah baris. DILARANG mengarang angka.
- Momen harus mulai di awal kalimat dan berakhir di akhir kalimat. Jangan memotong di tengah ucapan.
- Momen tidak boleh saling tumpang tindih.
`)
	fmt.Fprintf(&b, "- Incar durasi %.0f-%.0f detik. Boleh menyimpang demi momen yang utuh, tapi jangan di bawah %.0f detik.\n",
		targetMin, targetMax, targetMin*0.6)

	if ch.Total > 1 {
		fmt.Fprintf(&b, `
POTONGAN: ini bagian %d dari %d, mencakup detik %.0f sampai %.0f dari video utuh.
- Pilih momen HANYA di dalam rentang detik itu.
- Bila momen terbaik masih BERLANJUT melewati detik %.0f, set "berlanjut": true dan tulis "end" = %.0f.
  Sisa momen akan diambil dari potongan berikutnya dan disambung otomatis.
- Selain itu "berlanjut": false.
`, ch.Index, ch.Total, ch.Start, ch.End, ch.End, ch.End)
	}

	b.WriteString(`
KRITERIA SKOR (0-100 tiap dimensi):
- hook: 3 detik pertama menahan orang agar tidak scroll?
- emotion: muatan emosi (kaget, lucu, marah, haru, menginspirasi)
- clarity: mudah dipahami tanpa konteks video lain
- shareability: layak dibagikan atau memancing komentar
- standalone: utuh sebagai satu cerita (hook -> isi -> penutup)
- score: penilaian keseluruhan, bukan rata-rata mentah kelima dimensi

JUDUL & TAGAR:
- title: judul catchy bahasa Indonesia, maksimal 60 karakter, tanpa tanda kutip.
- hashtags: 3-5 tagar relevan berbahasa Indonesia, tiap tagar diawali #.

Balas HANYA JSON valid tanpa penjelasan apa pun, bentuk persis:
{"moments":[{"start":<detik>,"end":<detik>,"score":<0-100>,"reasons":{"hook":<0-100>,"emotion":<0-100>,"clarity":<0-100>,"shareability":<0-100>,"standalone":<0-100>},"title":"<judul>","hashtags":["#..","#.."],"berlanjut":false}]}`)
	return b.String()
}

// UserPrompt merangkai transkrip bertimestamp + permintaan jumlah momen.
func UserPrompt(tr types.Transcript, maxClips int) string {
	var b strings.Builder
	b.WriteString("Transkrip:\n")
	for _, s := range tr.Segments {
		fmt.Fprintf(&b, "[%.1f-%.1f] %s\n", s.Start, s.End, s.Text)
	}
	fmt.Fprintf(&b, "\nPilih maksimal %d momen terbaik.", maxClips)
	return b.String()
}

// ResponseSchema adalah JSON Schema bentuk balasan. Dipakai Ollama lewat
// parameter "format" agar decoder dipaksa menghasilkan JSON yang valid — dulu
// prompt rumit sering membuat model lokal membalas JSON rusak.
func ResponseSchema() map[string]any {
	num := map[string]any{"type": "number"}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"moments": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"start": num,
						"end":   num,
						"score": num,
						"reasons": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"hook": num, "emotion": num, "clarity": num,
								"shareability": num, "standalone": num,
							},
						},
						"title":     map[string]any{"type": "string"},
						"hashtags":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"berlanjut": map[string]any{"type": "boolean"},
					},
					"required": []string{"start", "end", "score", "title"},
				},
			},
		},
		"required": []string{"moments"},
	}
}

// MomentsWrapper bentuk balasan kedua provider.
type MomentsWrapper struct {
	Moments []Moment `json:"moments"`
}
