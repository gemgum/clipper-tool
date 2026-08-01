package ollama

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/gemgum/clipper/engine/internal/score/llm"
	"github.com/gemgum/clipper/engine/internal/types"
)

// Video ini dibuka dengan cuplikan: potongan pendek dari sepanjang video yang
// disambung, berganti topik tiap beberapa detik. Klip yang dipotong dari sana
// terdengar ramai tapi tidak membahas apa pun.
//
//	CLIPPER_TEST_LIVE=1 go test ./internal/score/ollama/ -run Teaser -v
func TestTeaserIsNotPicked(t *testing.T) {
	if os.Getenv("CLIPPER_TEST_LIVE") == "" {
		t.Skip("set CLIPPER_TEST_LIVE=1 untuk menjalankan uji ini terhadap Ollama")
	}
	raw, err := os.ReadFile("../../../../data/cache/transcripts/d13983c7d31d87ee45ba52a0cc8db865.json")
	if err != nil {
		t.Skip("transkrip contoh tidak ada")
	}
	var full types.Transcript
	if err := json.Unmarshal(raw, &full); err != nil {
		t.Fatal(err)
	}
	tr := types.Transcript{Language: full.Language}
	for _, s := range full.Segments {
		if s.End <= 300 {
			tr.Segments = append(tr.Segments, s)
		}
	}
	t.Logf("%d segmen, 0-300 detik", len(tr.Segments))

	c := New("", "qwen2.5")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	got, err := c.SelectMoments(ctx, tr, 2, 30, 60, llm.Chunk{Index: 1, Total: 1, Start: 0, End: 300})
	if err != nil {
		t.Fatal(err)
	}
	teaser := 0
	for _, m := range got {
		t.Logf("momen %.0f-%.0f skor %.0f | %s", m.Start, m.End, m.Score, m.Title)
		if m.Start < 90 {
			teaser++
		}
	}
	if teaser > 0 {
		t.Errorf("%d dari %d momen masih diambil dari cuplikan pembuka (mulai <90 detik)", teaser, len(got))
	}
}
