package api

import (
	"context"
	"errors"
	"testing"
)

// Job yang selesai/gagal ditulis ke disk dan terbaca lagi oleh store baru;
// nomor id melanjutkan yang terbesar, dan daftar urut terbaru dulu.
func TestBGStorePersists(t *testing.T) {
	dir := t.TempDir()
	var a bgStore[string]
	a.persistTo(dir)
	j1 := a.create("post", func() {})
	a.finish(j1.ID, context.Background(), "hasil", nil)
	j2 := a.create("post", func() {})
	a.finish(j2.ID, context.Background(), "", errors.New("gagal di tahap baca"))

	var b bgStore[string]
	b.persistTo(dir)
	got := b.snapshot()
	if len(got) != 2 {
		t.Fatalf("terbaca %d job, mau 2", len(got))
	}
	if got[0].ID != j2.ID || got[0].Status != "error" || got[0].Error == "" {
		t.Fatalf("urutan/isi salah: %+v", got[0])
	}
	if got[1].Result == nil || *got[1].Result != "hasil" {
		t.Fatalf("hasil hilang: %+v", got[1])
	}
	if j3 := b.create("post", func() {}); j3.ID != "post_0003" {
		t.Fatalf("id baru %s menimpa yang lama", j3.ID)
	}
}
