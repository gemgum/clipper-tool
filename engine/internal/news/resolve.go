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

// Resolve menerjemahkan pengalih Google News jadi alamat artikel yang
// sebenarnya. URL yang sudah berupa alamat artikel dikembalikan apa adanya.
//
// Hasilnya disimpan sebagai cache karena resolusi menuntut satu peluncuran
// browser (~2–3 detik). Cache ini juga menghemat di tempat lain: begitu sebuah
// tautan pernah diresolusi, FetchContent tidak perlu memanggil browser lagi —
// ia cukup mengunduh artikelnya lewat HTTP biasa, yang jauh lebih cepat.
func Resolve(ctx context.Context, link string, browse Browser, cacheDir string) (string, error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return "", fmt.Errorf("the link is empty")
	}
	if !IsGoogleNewsLink(link) {
		return link, nil
	}
	if original, ok := loadResolved(cacheDir, link); ok {
		return original, nil
	}
	if browse == nil {
		return "", fmt.Errorf("search-result links must be opened in a browser, but no browser is available — install Chrome/Chromium, or open the link yourself")
	}
	dom, err := browse(ctx, link)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(link)
	if err != nil {
		return "", err
	}
	art, err := parseArticle(dom, u, "")
	if err != nil {
		return "", err
	}
	if IsGoogleNewsLink(art.URL) {
		return "", fmt.Errorf("the link has not reached the original article yet — try again in a moment")
	}
	saveResolved(cacheDir, link, art.URL)
	return art.URL, nil
}

// --- cache resolusi ---

func resolvedPath(dir, link string) string {
	h := sha256.Sum256([]byte(link))
	return filepath.Join(dir, "cache", "resolved", hex.EncodeToString(h[:16])+".txt")
}

func loadResolved(dir, link string) (string, bool) {
	if dir == "" {
		return "", false
	}
	raw, err := os.ReadFile(resolvedPath(dir, link))
	if err != nil {
		return "", false
	}
	original := strings.TrimSpace(string(raw))
	if original == "" || IsGoogleNewsLink(original) {
		return "", false
	}
	return original, true
}

func saveResolved(dir, link, original string) {
	if dir == "" {
		return
	}
	path := resolvedPath(dir, link)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return // cache itu percepatan, bukan keharusan
	}
	_ = os.WriteFile(path, []byte(original), 0o644)
}
