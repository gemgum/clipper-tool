package news

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Resolusi menerjemahkan pengalih Google News jadi alamat artikel yang
// sebenarnya. URL yang sudah berupa alamat artikel dikembalikan apa adanya.
//
// Hasilnya disimpan sebagai cache karena resolusi menuntut satu peluncuran
// browser (~2–3 detik). Cache ini juga menghemat di tempat lain: begitu sebuah
// tautan pernah diresolusi, AmbilIsi tidak perlu memanggil browser lagi — ia
// cukup mengunduh artikelnya lewat HTTP biasa, yang jauh lebih cepat.
func Resolusi(ctx context.Context, tautan string, ramban Perambah, cacheDir string) (string, error) {
	tautan = strings.TrimSpace(tautan)
	if tautan == "" {
		return "", fmt.Errorf("tautan kosong")
	}
	if !TautanGoogleNews(tautan) {
		return tautan, nil
	}
	if asli, ok := muatResolusi(cacheDir, tautan); ok {
		return asli, nil
	}
	if ramban == nil {
		return "", fmt.Errorf("tautan hasil pencarian perlu dibuka lewat browser, tapi browser tidak tersedia — pasang Chrome/Chromium, atau buka tautannya sendiri")
	}
	dom, err := ramban(ctx, tautan)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(tautan)
	if err != nil {
		return "", err
	}
	art, err := uraiArtikel(dom, u)
	if err != nil {
		return "", err
	}
	if TautanGoogleNews(art.URL) {
		return "", fmt.Errorf("tautan belum sampai ke artikel aslinya — coba lagi sebentar")
	}
	simpanResolusi(cacheDir, tautan, art.URL)
	return art.URL, nil
}

// --- cache resolusi ---

func jalurResolusi(dir, tautan string) string {
	h := sha256.Sum256([]byte(tautan))
	return filepath.Join(dir, "cache", "resolusi", hex.EncodeToString(h[:16])+".txt")
}

func muatResolusi(dir, tautan string) (string, bool) {
	if dir == "" {
		return "", false
	}
	raw, err := os.ReadFile(jalurResolusi(dir, tautan))
	if err != nil {
		return "", false
	}
	asli := strings.TrimSpace(string(raw))
	if asli == "" || TautanGoogleNews(asli) {
		return "", false
	}
	return asli, true
}

func simpanResolusi(dir, tautan, asli string) {
	if dir == "" {
		return
	}
	jalur := jalurResolusi(dir, tautan)
	if err := os.MkdirAll(filepath.Dir(jalur), 0o755); err != nil {
		return // cache itu percepatan, bukan keharusan
	}
	_ = os.WriteFile(jalur, []byte(asli), 0o644)
}
