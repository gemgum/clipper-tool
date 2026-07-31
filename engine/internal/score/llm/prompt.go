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

// languageNames memetakan kode bahasa transkrip ke nama yang dimengerti model.
// Dipakai untuk memberi tahu model dalam bahasa apa judul & tagar harus ditulis:
// judul mengikuti bahasa VIDEO, bukan bahasa antarmuka.
var languageNames = map[string]string{
	"id": "Indonesian",
	"en": "English",
	"ms": "Malay",
	"jv": "Javanese",
	"su": "Sundanese",
}

// LanguageName mengembalikan nama bahasa yang bisa dipakai di dalam prompt.
// Kode yang tidak dikenal dikembalikan apa adanya — model umumnya masih paham.
func LanguageName(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return "Indonesian"
	}
	if name, ok := languageNames[code]; ok {
		return name
	}
	return code
}

// SystemPrompt menyusun instruksi yang dipakai SAMA PERSIS oleh Claude maupun
// Ollama, supaya hasil kedua mesin bisa dibandingkan. Ditulis detail karena
// keluaran model kini dijamin bentuknya (JSON Schema di Ollama) sehingga prompt
// panjang tidak lagi merusak format balasan.
//
// contentLang = bahasa isi video; judul & tagar ditulis dalam bahasa itu,
// sedangkan instruksinya sendiri selalu bahasa Inggris.
func SystemPrompt(targetMin, targetMax float64, ch Chunk, contentLang string) string {
	lang := LanguageName(contentLang)

	var b strings.Builder
	b.WriteString(`You curate viral short-form video clips (TikTok, Reels, Shorts).
You are given a timestamped transcript (in seconds). Your job is to SELECT the best moments for short vertical clips.

TIME BOUNDARY RULES — the most important part:
- 'start' and 'end' MUST come from timestamp numbers that actually APPEAR in the transcript.
  'start' = the opening number of a line, 'end' = the closing number of a line. NEVER invent numbers.
- A moment must begin at the start of a sentence and end at the end of a sentence. Never cut mid-utterance.
- Moments must not overlap each other.
`)
	fmt.Fprintf(&b, "- Aim for %.0f-%.0f seconds. You may deviate to keep a moment whole, but never go below %.0f seconds.\n",
		targetMin, targetMax, targetMin*0.6)

	if ch.Total > 1 {
		fmt.Fprintf(&b, `
CHUNK: this is part %d of %d, covering seconds %.0f through %.0f of the full video.
- Only pick moments inside that time range.
- If the best moment still CONTINUES past second %.0f, set "continues": true and write "end" = %.0f.
  The rest of the moment will be taken from the next chunk and joined automatically.
- Otherwise set "continues": false.
`, ch.Index, ch.Total, ch.Start, ch.End, ch.End, ch.End)
	}

	b.WriteString(`
SCORING CRITERIA (0-100 per dimension):
- hook: do the first 3 seconds stop someone from scrolling?
- emotion: emotional charge (surprise, humour, anger, tenderness, inspiration)
- clarity: understandable without the rest of the video
- shareability: worth sharing, or likely to draw comments
- standalone: complete as a single story (hook -> body -> close)
- score: an overall judgement, not a raw average of the five dimensions

TITLE & HASHTAGS:
`)
	fmt.Fprintf(&b, "- title: a catchy title in %s, at most 60 characters, no quotation marks.\n", lang)
	fmt.Fprintf(&b, "- hashtags: 3-5 relevant hashtags in %s, each starting with #.\n", lang)

	b.WriteString(`
Reply with VALID JSON ONLY, no explanation, in exactly this shape:
{"moments":[{"start":<seconds>,"end":<seconds>,"score":<0-100>,"reasons":{"hook":<0-100>,"emotion":<0-100>,"clarity":<0-100>,"shareability":<0-100>,"standalone":<0-100>},"title":"<title>","hashtags":["#..","#.."],"continues":false}]}`)
	return b.String()
}

// UserPrompt merangkai transkrip bertimestamp + permintaan jumlah momen.
func UserPrompt(tr types.Transcript, maxClips int) string {
	var b strings.Builder
	b.WriteString("Transcript:\n")
	for _, s := range tr.Segments {
		fmt.Fprintf(&b, "[%.1f-%.1f] %s\n", s.Start, s.End, s.Text)
	}
	fmt.Fprintf(&b, "\nSelect at most %d of the best moments.", maxClips)
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
						"continues": map[string]any{"type": "boolean"},
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
