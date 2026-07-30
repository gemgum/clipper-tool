package news

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Pencarian berita memakai endpoint RSS milik Google News.
//
// Kenapa bukan menyisir halaman pencarian Google: halaman itu menolak akses
// otomatis. Diuji dari mesin biasa dengan Chrome asli yang menjalankan JS,
// balasannya CAPTCHA pada permintaan PERTAMA — nol hasil. Endpoint RSS ini
// sebaliknya memang disediakan untuk dibaca mesin, dan mengembalikan XML yang
// bisa diurai parser feed yang sudah ada.
//
// Operator pencarian tetap berlaku dan sudah diuji: "site:antaranews.com",
// tanda kutip untuk frasa persis, dan "when:7d" untuk membatasi rentang waktu.

// gnewsHost = awalan tautan yang dikembalikan Google News. Tautan ini BUKAN
// alamat artikelnya, melainkan pengalih yang hanya bisa dibuka browser.
const gnewsHost = "news.google.com"

// TautanGoogleNews melaporkan apakah sebuah URL masih berupa pengalih Google
// News, sehingga perlu diresolusi lebih dulu sebelum isinya bisa dibaca.
func TautanGoogleNews(u string) bool {
	return strings.Contains(u, gnewsHost+"/rss/articles/") ||
		strings.Contains(u, gnewsHost+"/articles/") ||
		strings.Contains(u, gnewsHost+"/read/")
}

// Cari mengambil berita menurut kata kunci.
//
// Perhatikan: Artikel.URL yang dikembalikan masih berupa pengalih Google News.
// Pemanggil wajib meresolusinya (lihat AmbilIsi) sebelum dipakai jadi kartu —
// kalau tidak, yang terbaca adalah halaman Google News, bukan artikelnya.
func Cari(ctx context.Context, kata string, maks int) ([]Artikel, error) {
	kata = strings.TrimSpace(kata)
	if kata == "" {
		return nil, fmt.Errorf("kata kunci kosong")
	}
	q := url.Values{}
	q.Set("q", kata)
	// hl/gl/ceid menentukan bahasa & negara hasil. Tanpa ini yang keluar berita
	// berbahasa Inggris.
	q.Set("hl", "id")
	q.Set("gl", "ID")
	q.Set("ceid", "ID:id")

	raw, err := ambil(ctx, "https://"+gnewsHost+"/rss/search?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("pencarian berita gagal: %w", err)
	}
	art, err := uraiFeed(raw, "", maks)
	if err != nil {
		return nil, fmt.Errorf("hasil pencarian tidak terbaca: %w", err)
	}
	return art, nil
}
