package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func soon() time.Time { return time.Now().Add(2 * time.Second) }

// Inti fiturnya: berkas yang di-drop dikenali dari nama + ukuran, jadi video
// 3,84 GB tidak perlu disalin ke data/uploads sama sekali.
func TestLocateFindsTheFileByNameAndSize(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "Videos", "2026", "podcast.mp4")
	write(t, want, 1234)
	write(t, filepath.Join(root, "Videos", "lain.mp4"), 1234)

	got := findFile([]string{root}, "podcast.mp4", 1234, soon())

	if len(got) != 1 || got[0] != want {
		t.Fatalf("hasil = %v, mau [%s]", got, want)
	}
}

// Nama sama tapi ukuran beda = berkas lain. Kalau ini lolos, engine memproses
// video yang salah tanpa ada yang sadar.
func TestLocateRejectsADifferentSize(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "podcast.mp4"), 999)

	if got := findFile([]string{root}, "podcast.mp4", 1234, soon()); len(got) != 0 {
		t.Fatalf("hasil = %v, mau kosong", got)
	}
}

// Dua kandidat = jawabannya tidak diketahui. Menebak salah satunya lebih buruk
// daripada menyuruh pengguna memilih sendiri.
func TestLocateStaysSilentWhenTwoFilesMatch(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a", "podcast.mp4"), 1234)
	write(t, filepath.Join(root, "b", "podcast.mp4"), 1234)

	if got := findFile([]string{root}, "podcast.mp4", 1234, soon()); len(got) < 2 {
		t.Fatalf("hasil = %v, mau dua kandidat", got)
	}
}

// Pencarian tidak boleh menggantung GUI: begitu anggaran waktu habis, ia
// berhenti walau belum ketemu.
func TestLocateStopsWhenTheBudgetIsSpent(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "deep", "podcast.mp4"), 1234)

	if got := findFile([]string{root}, "podcast.mp4", 1234, time.Now().Add(-time.Second)); len(got) != 0 {
		t.Fatalf("hasil = %v, mau kosong (waktu sudah lewat)", got)
	}
}

// Folder besar yang tak pernah berisi video pengguna dilewati.
func TestLocateSkipsNoiseFolders(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "node_modules", "podcast.mp4"), 1234)

	if got := findFile([]string{root}, "podcast.mp4", 1234, soon()); len(got) != 0 {
		t.Fatalf("hasil = %v, mau kosong", got)
	}
}

// Endpoint locate menjawab 404 saat tidak yakin — itu isyarat bagi GUI untuk
// kembali ke unggahan biasa.
func TestLocateEndpointAnswers404WhenUnsure(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest("POST", "/api/locate", strings.NewReader(`{"name":"tidak-ada-12345.mp4","size":7}`))
	rec := httptest.NewRecorder()

	s.locate(rec, req)

	if rec.Code != 404 {
		t.Fatalf("kode = %d, mau 404", rec.Code)
	}
}

func TestLocateRejectsAnEmptyName(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest("POST", "/api/locate", strings.NewReader(`{"name":"","size":7}`))
	rec := httptest.NewRecorder()

	s.locate(rec, req)

	if rec.Code != 400 {
		t.Fatalf("kode = %d, mau 400", rec.Code)
	}
}

// browse: folder dulu, lalu berkas, masing-masing menurut abjad — urutan yang
// sama dengan file manager mana pun.
func TestBrowseListsFoldersFirst(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "zeta.mp4"), 1)
	write(t, filepath.Join(root, "Alpha.txt"), 1)
	write(t, filepath.Join(root, ".tersembunyi"), 1)
	if err := os.MkdirAll(filepath.Join(root, "Video"), 0o755); err != nil {
		t.Fatal(err)
	}

	body := browseJSON(t, root)

	var names []string
	for _, e := range body.Entries {
		names = append(names, e.Name)
	}
	want := []string{"Video", "Alpha.txt", "zeta.mp4"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("urutan = %v, mau %v", names, want)
	}
	for _, e := range body.Entries {
		if e.Name == "zeta.mp4" && !e.Video {
			t.Error("zeta.mp4 tidak ditandai sebagai video")
		}
	}
}

// Berkas tersembunyi tidak ditampilkan: pengguna tidak menyimpan videonya di
// ~/.cache, dan daftarnya jadi jauh lebih pendek.
func TestBrowseHidesDotFiles(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".rahasia.mp4"), 1)

	if body := browseJSON(t, root); len(body.Entries) != 0 {
		t.Errorf("entri = %v, mau kosong", body.Entries)
	}
}

// Folder yang tidak ada dijawab 400 dengan pesan, bukan 500 atau panik.
func TestBrowseReportsAMissingFolder(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest("GET", "/api/browse?dir="+url.QueryEscape(filepath.Join(t.TempDir(), "tidak-ada")), nil)
	rec := httptest.NewRecorder()

	s.browse(rec, req)

	if rec.Code != 400 {
		t.Fatalf("kode = %d, mau 400", rec.Code)
	}
}

type browseBody struct {
	Dir     string  `json:"dir"`
	Parent  string  `json:"parent"`
	Entries []entry `json:"entries"`
	Places  []place `json:"places"`
}

func browseJSON(t *testing.T, dir string) browseBody {
	t.Helper()
	s := &Server{}
	req := httptest.NewRequest("GET", "/api/browse?dir="+url.QueryEscape(dir), nil)
	rec := httptest.NewRecorder()
	s.browse(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("kode = %d: %s", rec.Code, rec.Body.String())
	}
	var body browseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}
