package news

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// FetchArticle membaca satu artikel dari URL yang ditempel pengguna dan
// mengembalikan bahan untuk kartu.
//
// Sumber datanya adalah tag Open Graph (og:title, og:image, …) — bukan isi
// badan halaman. Alasannya: og: memang dipasang media supaya tautannya tampil
// rapi saat dibagikan ke media sosial, jadi isinya sudah berupa judul, ringkasan
// dan gambar utama yang mereka pilih sendiri. Menebaknya dari badan HTML jauh
// lebih rapuh dan berbeda-beda tiap situs.
func FetchArticle(ctx context.Context, page, lang string) (Article, error) {
	u, err := url.Parse(strings.TrimSpace(page))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Article{}, fmt.Errorf("invalid URL — paste a full link starting with https://")
	}
	raw, err := download(ctx, u.String())
	if err != nil {
		return Article{}, err
	}
	return parseArticle(string(raw), u, lang)
}

// parseArticle membaca metadata dari HTML yang sudah di tangan. Dipisah dari
// FetchArticle supaya bisa dipakai juga untuk DOM hasil browser (tautan Google
// News), yang tidak melewati pengunduhan biasa.
func parseArticle(htmlStr string, u *url.URL, lang string) (Article, error) {
	meta := readMeta(htmlStr)
	title := firstNonEmpty(meta["og:title"], meta["twitter:title"], tagTitle(htmlStr))
	if title == "" {
		return Article{}, fmt.Errorf("no title found at %s — that page may not be an article, or it loads its content via JavaScript", domain(u.String()))
	}
	// og:url menyebut alamat kanonik artikel. Untuk halaman hasil resolusi
	// Google News, inilah satu-satunya tempat alamat aslinya muncul.
	address := u
	// clean() wajib di sini: sebagian situs menulis og:url dengan entitas HTML
	// menempel di ujungnya (mis. "…/artikel&nbsp;&nbsp;"). Diteruskan mentah,
	// alamat yang dihasilkan ikut membawa sampah itu dan berujung 404.
	if canon := clean(firstNonEmpty(meta["og:url"], meta["twitter:url"])); canon != "" {
		if abs, err := u.Parse(canon); err == nil && abs.Host != "" {
			address = abs
		}
	}
	summary := firstNonEmpty(meta["og:description"], meta["twitter:description"], meta["description"])
	image := clean(firstNonEmpty(meta["og:image"], meta["og:image:url"], meta["twitter:image"]))
	if image != "" {
		// Sebagian situs menulis og:image sebagai path relatif.
		if abs, err := address.Parse(image); err == nil {
			image = abs.String()
		}
	}
	published := firstNonEmpty(meta["article:published_time"], meta["og:updated_time"], meta["date"])

	return Article{
		Title:     clean(title),
		Summary:   truncate(clean(summary), 300),
		URL:       address.String(),
		Image:     image,
		Source:    firstNonEmpty(clean(meta["og:site_name"]), domain(address.String())),
		Domain:    domain(address.String()),
		Date:      formatDate(published, lang),
		Published: rfc3339(published),
	}, nil
}

// reMeta menangkap satu tag <meta>. Atribut property/name bisa muncul sebelum
// atau sesudah content, jadi tagnya ditangkap utuh lalu dibedah terpisah.
var (
	reMeta     = regexp.MustCompile(`(?is)<meta\s+[^>]*>`)
	reProperty = regexp.MustCompile(`(?is)\b(?:property|name|itemprop)\s*=\s*["']([^"']+)["']`)
	reContent  = regexp.MustCompile(`(?is)\bcontent\s*=\s*["']([^"']*)["']`)
	reTitle    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reHead     = regexp.MustCompile(`(?is)</head>`)
)

// readMeta mengumpulkan seluruh tag meta jadi peta kunci→isi.
func readMeta(h string) map[string]string {
	// Cukup pindai bagian <head>; badan artikel bisa sangat panjang dan tidak
	// pernah berisi tag og:.
	if loc := reHead.FindStringIndex(h); loc != nil {
		h = h[:loc[1]]
	}
	out := map[string]string{}
	for _, tag := range reMeta.FindAllString(h, -1) {
		k := reProperty.FindStringSubmatch(tag)
		v := reContent.FindStringSubmatch(tag)
		if len(k) != 2 || len(v) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(k[1]))
		if _, exists := out[key]; exists {
			continue // pakai yang pertama; duplikat biasanya kurang spesifik
		}
		out[key] = v[1]
	}
	return out
}

func tagTitle(h string) string {
	if m := reTitle.FindStringSubmatch(h); len(m) == 2 {
		return m[1]
	}
	return ""
}
