package setup

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
)

// makeZip menulis arsip zip berisi nama→isi.
func makeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

// Arsip rilis membungkus binernya dalam satu folder ("Release/", "bin/").
// Struktur itu tidak berarti apa-apa bagi engine — yang penting semua berkas
// mendarat di satu folder, sebab di situlah engine mencari pustaka pendamping.
func TestZipIsFlattenedIntoOneFolder(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.zip")
	makeZip(t, archive, map[string]string{
		"Release/whisper-cli.exe": "bin",
		"Release/whisper.dll":     "lib",
	})
	dest := t.TempDir()

	n, err := (&recipe{kind: "zip"}).extract(archive, dest)
	if err != nil {
		t.Fatal(err)
	}

	if n != 2 {
		t.Errorf("berkas terambil = %d, mau 2", n)
	}
	for _, want := range []string{"whisper-cli.exe", "whisper.dll"} {
		if _, err := os.Stat(filepath.Join(dest, want)); err != nil {
			t.Errorf("%s tidak ada di folder tujuan", want)
		}
	}
}

// Arsip ffmpeg berisi ratusan berkas dokumentasi; hanya dua yang dipakai.
func TestZipTakesOnlyTheWantedFiles(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.zip")
	makeZip(t, archive, map[string]string{
		"build/bin/ffmpeg.exe":   "x",
		"build/bin/ffprobe.exe":  "x",
		"build/bin/ffplay.exe":   "x",
		"build/doc/ffmpeg.html":  "x",
		"build/doc/general.html": "x",
	})
	dest := t.TempDir()

	n, err := (&recipe{kind: "zip", want: []string{"ffmpeg.exe", "ffprobe.exe"}}).extract(archive, dest)
	if err != nil {
		t.Fatal(err)
	}

	if n != 2 {
		t.Fatalf("berkas terambil = %d, mau 2", n)
	}
	if _, err := os.Stat(filepath.Join(dest, "ffplay.exe")); err == nil {
		t.Error("ffplay ikut terambil padahal tidak diminta")
	}
}

// Rilis Linux whisper.cpp memakai rantai symlink
// (libwhisper.so → libwhisper.so.1 → libwhisper.so.1.9.1). Tanpa dukungan ini,
// binernya terpasang tapi tidak bisa dijalankan sama sekali.
func TestTarGzKeepsSymlinks(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := "biner"
	if err := tw.WriteHeader(&tar.Header{
		Name: "whisper-bin/whisper-cli", Mode: 0o755,
		Size: int64(len(body)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{
		Name: "whisper-bin/libwhisper.so", Linkname: "libwhisper.so.1",
		Typeflag: tar.TypeSymlink, Mode: 0o777,
	}); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()
	f.Close()
	dest := t.TempDir()

	if _, err := (&recipe{kind: "tar.gz"}).extract(archive, dest); err != nil {
		t.Fatal(err)
	}

	link, err := os.Readlink(filepath.Join(dest, "libwhisper.so"))
	if err != nil {
		t.Fatalf("symlink tidak dibuat: %v", err)
	}
	if link != "libwhisper.so.1" {
		t.Errorf("tujuan symlink = %q, mau %q", link, "libwhisper.so.1")
	}
	// Biner harus bisa dieksekusi; rilis Windows tidak membawa bit itu, jadi
	// engine yang memasangnya.
	info, err := os.Stat(filepath.Join(dest, "whisper-cli"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o100 == 0 {
		t.Error("whisper-cli tidak bisa dieksekusi")
	}
}

// Unduhan yang terputus tidak boleh meninggalkan berkas yang tampak jadi:
// pemanggilan berikutnya akan menganggapnya "sudah terpasang".
func TestAFailedDownloadLeavesNothingBehind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "model.bin")

	err := download(context.Background(), srv.URL, dest, nil)

	if err == nil {
		t.Fatal("unduhan gagal tidak dilaporkan")
	}
	if _, err := os.Stat(dest); err == nil {
		t.Error("berkas tujuan tetap dibuat padahal unduhan gagal")
	}
	if _, err := os.Stat(dest + ".part"); err == nil {
		t.Error("berkas .part ditinggalkan")
	}
}

// Progres dilaporkan, dan totalnya ikut supaya GUI bisa menggambar bilah.
func TestDownloadReportsProgress(t *testing.T) {
	payload := strings.Repeat("x", 4096)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "4096")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "model.bin")

	var last Progress
	seen := 0
	if err := download(context.Background(), srv.URL, dest, func(p Progress) {
		last = p
		seen++
	}); err != nil {
		t.Fatal(err)
	}

	if seen == 0 {
		t.Fatal("tidak ada laporan progres")
	}
	if last.Total != 4096 || last.Bytes != 4096 {
		t.Errorf("progres terakhir = %d/%d, mau 4096/4096", last.Bytes, last.Total)
	}
	raw, err := os.ReadFile(dest)
	if err != nil || len(raw) != 4096 {
		t.Errorf("isi berkas = %d byte (err %v), mau 4096", len(raw), err)
	}
}

// Status membaca folder dari Layout, bukan dari sekitar biner.
func TestStatusFindsAnInstalledModel(t *testing.T) {
	l := config.Layout{ModelsDir: t.TempDir(), ToolsDir: t.TempDir()}
	if err := os.WriteFile(ModelPath(l, "small"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var small Component
	for _, c := range Status(l, false, "") {
		if c.ID == ModelID("small") {
			small = c
		}
	}

	if !small.Installed {
		t.Fatal("model small tidak terdeteksi")
	}
	if small.Path != ModelPath(l, "small") {
		t.Errorf("path = %q, mau %q", small.Path, ModelPath(l, "small"))
	}
}

// Komponen wajib yang belum ada harus bisa disebut namanya — itu yang dipakai
// untuk menolak job dengan pesan yang bisa ditindaklanjuti.
func TestMissingNamesTheRequiredComponents(t *testing.T) {
	l := config.Layout{ModelsDir: t.TempDir(), ToolsDir: t.TempDir()}

	missing := Missing(Status(l, false, ""))

	found := false
	for _, m := range missing {
		if m == "whisper.cpp" {
			found = true
		}
	}
	if !found {
		t.Errorf("kurang = %v, mau memuat whisper.cpp", missing)
	}
}

func TestInstallRejectsAnUnknownComponent(t *testing.T) {
	l := config.Layout{ModelsDir: t.TempDir(), ToolsDir: t.TempDir()}

	if err := Install(context.Background(), l, "sesuatu", nil); err == nil {
		t.Fatal("komponen tak dikenal tidak ditolak")
	}
	if err := Install(context.Background(), l, "model:tidak-ada", nil); err == nil {
		t.Fatal("model tak dikenal tidak ditolak")
	}
}

// Menghapus model membebaskan ruang; menghapus yang tidak ada bukan error.
func TestRemoveModel(t *testing.T) {
	l := config.Layout{ModelsDir: t.TempDir()}
	if err := os.WriteFile(ModelPath(l, "tiny"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RemoveModel(l, "tiny"); err != nil {
		t.Fatal(err)
	}
	if HasModel(l, "tiny") {
		t.Error("model masih ada setelah dihapus")
	}
	if err := RemoveModel(l, "tiny"); err != nil {
		t.Errorf("menghapus yang sudah tidak ada dilaporkan sebagai galat: %v", err)
	}
	if err := RemoveModel(l, "bukan-model"); err == nil {
		t.Error("nama model asing tidak ditolak")
	}
}
