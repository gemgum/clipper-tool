package news

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Ambil membaca satu artikel dari URL yang ditempel pengguna dan mengembalikan
// bahan untuk kartu.
//
// Sumber datanya adalah tag Open Graph (og:title, og:image, …) — bukan isi
// badan halaman. Alasannya: og: memang dipasang media supaya tautannya tampil
// rapi saat dibagikan ke media sosial, jadi isinya sudah berupa judul, ringkasan
// dan gambar utama yang mereka pilih sendiri. Menebaknya dari badan HTML jauh
// lebih rapuh dan berbeda-beda tiap situs.
func Ambil(ctx context.Context, halaman string) (Artikel, error) {
	u, err := url.Parse(strings.TrimSpace(halaman))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Artikel{}, fmt.Errorf("URL tidak valid — tempel tautan lengkap yang diawali https://")
	}
	raw, err := ambil(ctx, u.String())
	if err != nil {
		return Artikel{}, err
	}
	htmlStr := string(raw)

	meta := bacaMeta(htmlStr)
	judul := firstNonEmpty(meta["og:title"], meta["twitter:title"], tagTitle(htmlStr))
	if judul == "" {
		return Artikel{}, fmt.Errorf("tidak menemukan judul di %s — halaman itu mungkin bukan artikel, atau memuat isinya lewat JavaScript", domain(u.String()))
	}
	ringkas := firstNonEmpty(meta["og:description"], meta["twitter:description"], meta["description"])
	gambar := firstNonEmpty(meta["og:image"], meta["og:image:url"], meta["twitter:image"])
	if gambar != "" {
		// Sebagian situs menulis og:image sebagai path relatif.
		if abs, err := u.Parse(gambar); err == nil {
			gambar = abs.String()
		}
	}
	terbit := firstNonEmpty(meta["article:published_time"], meta["og:updated_time"], meta["date"])

	return Artikel{
		Judul:   bersih(judul),
		Ringkas: potong(bersih(ringkas), 300),
		URL:     u.String(),
		Gambar:  gambar,
		Sumber:  firstNonEmpty(bersih(meta["og:site_name"]), domain(u.String())),
		Domain:  domain(u.String()),
		Tanggal: formatTanggal(terbit),
		Terbit:  rfc3339(terbit),
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

// bacaMeta mengumpulkan seluruh tag meta jadi peta kunci→isi.
func bacaMeta(h string) map[string]string {
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
		kunci := strings.ToLower(strings.TrimSpace(k[1]))
		if _, ada := out[kunci]; ada {
			continue // pakai yang pertama; duplikat biasanya kurang spesifik
		}
		out[kunci] = v[1]
	}
	return out
}

func tagTitle(h string) string {
	if m := reTitle.FindStringSubmatch(h); len(m) == 2 {
		return m[1]
	}
	return ""
}
