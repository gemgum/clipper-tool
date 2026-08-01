package segment

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/gemgum/clipper/engine/internal/types"
)

// Uji terhadap transkrip sungguhan di cache. Dilewati bila tidak diset, sebab
// data/ tidak ikut di-commit.
//
//	CLIPPER_TEST_TRANSCRIPT=data/cache/transcripts/<kunci>.json \
//	  go test ./internal/segment/ -run Live -v
func TestOpeningPreviewOnRealTranscriptLive(t *testing.T) {
	path := os.Getenv("CLIPPER_TEST_TRANSCRIPT")
	if path == "" {
		t.Skip("set CLIPPER_TEST_TRANSCRIPT untuk menjalankan uji ini")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var tr types.Transcript
	if err := json.Unmarshal(raw, &tr); err != nil {
		t.Fatal(err)
	}
	last := tr.Segments[len(tr.Segments)-1].End
	t.Logf("%d segmen, %.0f detik", len(tr.Segments), last)
	for _, w := range []struct{ a, b float64 }{
		{0, 45}, {24, 73}, {45, 90}, {90, 135}, {180, 225}, {400, 445}, {900, 945},
	} {
		if w.b > last {
			continue
		}
		t.Logf("%4.0f-%4.0f cuplikan=%v", w.a, w.b, IsOpeningPreview(tr, w.a, w.b))
	}
}
