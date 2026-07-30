package capture

import (
	"context"
	"os"
	"testing"
	"time"
)

// Uji manual: jalankan dengan
//
//	CLIPPER_UJI_URL="https://..." go test ./internal/capture/ -run TestDumpDOMManual -v
//
// Dilewati bila variabel itu tidak diset, supaya tidak ikut di CI.
func TestDumpDOMManual(t *testing.T) {
	url := os.Getenv("CLIPPER_UJI_URL")
	if url == "" {
		t.Skip("set CLIPPER_UJI_URL untuk menjalankan uji manual ini")
	}
	bin := Cari()
	t.Log("browser:", bin)
	c := New(bin)
	ctx, batal := context.WithTimeout(context.Background(), 120*time.Second)
	defer batal()
	mulai := time.Now()
	dom, err := c.DumpDOM(ctx, url, 15000)
	t.Logf("lama: %.1fs", time.Since(mulai).Seconds())
	if err != nil {
		t.Fatal("GALAT:", err)
	}
	t.Log("panjang DOM:", len(dom))
}
