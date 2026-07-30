// Package news mengambil bahan berita untuk kartu: daftar dari RSS, atau
// metadata satu artikel dari URL yang ditempel pengguna.
//
// Sengaja memakai RSS, bukan menyisir hasil mesin pencari. Feed itu memang
// disediakan untuk dibaca mesin, formatnya stabil, dan tidak akan memblokir
// kita — berbeda dengan scraping halaman pencarian yang cepat kena CAPTCHA.
package news

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Sumber satu feed berita.
type Sumber struct {
	ID    string `json:"id"`
	Nama  string `json:"nama"`
	URL   string `json:"url"`
	Topik string `json:"topik"`
}

// SumberBawaan = feed media Indonesia yang dipakai GUI sebagai titik awal.
// Pengguna tetap bisa menempel URL feed lain lewat parameter ?feed=.
var SumberBawaan = []Sumber{
	{"antara", "ANTARA", "https://www.antaranews.com/rss/terkini.xml", "Terkini"},
	{"antara-ekonomi", "ANTARA", "https://www.antaranews.com/rss/ekonomi.xml", "Ekonomi"},
	{"antara-olahraga", "ANTARA", "https://www.antaranews.com/rss/olahraga.xml", "Olahraga"},
	{"antara-tekno", "ANTARA", "https://www.antaranews.com/rss/tekno.xml", "Teknologi"},
	{"detik", "detikcom", "https://rss.detik.com/index.php/detiknews", "Terkini"},
	{"detik-finance", "detikcom", "https://rss.detik.com/index.php/finance", "Ekonomi"},
	{"detik-inet", "detikcom", "https://rss.detik.com/index.php/inet", "Teknologi"},
	{"cnn", "CNN Indonesia", "https://www.cnnindonesia.com/nasional/rss", "Nasional"},
	{"cnn-ekonomi", "CNN Indonesia", "https://www.cnnindonesia.com/ekonomi/rss", "Ekonomi"},
	{"tempo", "Tempo", "https://rss.tempo.co/nasional", "Nasional"},
	{"liputan6", "Liputan6", "https://feed.liputan6.com/rss", "Terkini"},
	{"kumparan", "kumparan", "https://kumparan.com/kumparannews/rss", "Terkini"},
}

// Artikel satu item berita — bentuk yang dipakai kartu.
type Artikel struct {
	Judul   string `json:"judul"`
	Ringkas string `json:"ringkas"`
	URL     string `json:"url"`
	Gambar  string `json:"gambar"`
	Sumber  string `json:"sumber"`  // nama media, mis. "ANTARA"
	Domain  string `json:"domain"`  // mis. "antaranews.com"
	Tanggal string `json:"tanggal"` // sudah diformat untuk manusia
	Terbit  string `json:"terbit"`  // RFC3339 bila terbaca
}

// bentuk XML RSS 2.0. Field yang tidak dipakai sengaja tidak didaftarkan.
type rssFeed struct {
	Channel struct {
		Title string    `xml:"title"`
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
	// Atom (beberapa media memakai ini).
	AtomTitle   string      `xml:"title"`
	AtomEntries []atomEntry `xml:"entry"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	// Gambar bisa datang lewat beberapa cara tergantung media.
	Enclosure struct {
		URL string `xml:"url,attr"`
	} `xml:"enclosure"`
	MediaContent struct {
		URL string `xml:"url,attr"`
	} `xml:"content"`
	MediaThumb struct {
		URL string `xml:"url,attr"`
	} `xml:"thumbnail"`
}

type atomEntry struct {
	Title   string `xml:"title"`
	Summary string `xml:"summary"`
	Updated string `xml:"updated"`
	Link    struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
}

var klien = &http.Client{Timeout: 25 * time.Second}

// batasUnduh membatasi besar respons agar satu URL nakal tidak menghabiskan RAM.
const batasUnduh = 4 << 20 // 4 MB

// ambil mengunduh URL sebagai teks dengan batas ukuran & User-Agent yang jelas.
func ambil(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("URL tidak valid: %v", err)
	}
	// Jujur menyebut diri; beberapa media memblokir User-Agent kosong.
	req.Header.Set("User-Agent", "Clipper/0.1 (+pembuat konten; pembaca RSS)")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9")
	res, err := klien.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi %s: %v", url, err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("%s membalas HTTP %d", url, res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, batasUnduh))
}

// Daftar mengambil isi satu feed dan mengubahnya jadi daftar artikel.
//
// nama = nama media untuk badge kartu. Diisi pemanggil bila feednya ada di
// SumberBawaan, sebab judul channel RSS tidak bisa dipercaya: ANTARA menulis
// "Berita Terkini - ANTARA News" (nama medianya di belakang) sedangkan CNN
// menulis "CNN Indonesia | ..." (di depan). Bila kosong, judul channel dipakai
// sebagai tebakan, lalu domain sebagai jaring terakhir.
func Daftar(ctx context.Context, feedURL, nama string, maks int) ([]Artikel, error) {
	raw, err := ambil(ctx, feedURL)
	if err != nil {
		return nil, err
	}
	return uraiFeed(raw, nama, maks)
}

// uraiFeed mengubah isi XML jadi daftar artikel. Dipisah dari Daftar supaya
// penguraiannya bisa diuji tanpa menyentuh jaringan.
func uraiFeed(raw []byte, nama string, maks int) ([]Artikel, error) {
	if maks <= 0 {
		maks = 20
	}
	var f rssFeed
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	// Feed Indonesia kadang memakai encoding non-UTF8 atau entitas HTML mentah;
	// tanpa dua setelan ini decoder berhenti di karakter pertama yang aneh.
	dec.Strict = false
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("isi feed tidak terbaca sebagai RSS/Atom: %v", err)
	}

	namaFeed := bersih(f.Channel.Title)
	if namaFeed == "" {
		namaFeed = bersih(f.AtomTitle)
	}

	var out []Artikel
	for _, it := range f.Channel.Items {
		if len(out) >= maks {
			break
		}
		link := bersih(it.Link)
		if link == "" {
			continue
		}
		gambar := firstNonEmpty(it.Enclosure.URL, it.MediaContent.URL, it.MediaThumb.URL, gambarDalam(it.Description))
		out = append(out, Artikel{
			Judul:   bersih(it.Title),
			Ringkas: potong(bersih(buangTag(it.Description)), 300),
			URL:     link,
			Gambar:  gambar,
			Sumber:  firstNonEmpty(nama, namaMedia(namaFeed, link)),
			Domain:  domain(link),
			Tanggal: formatTanggal(it.PubDate),
			Terbit:  rfc3339(it.PubDate),
		})
	}
	for _, e := range f.AtomEntries {
		if len(out) >= maks {
			break
		}
		link := bersih(e.Link.Href)
		if link == "" {
			continue
		}
		out = append(out, Artikel{
			Judul:   bersih(e.Title),
			Ringkas: potong(bersih(buangTag(e.Summary)), 300),
			URL:     link,
			Gambar:  gambarDalam(e.Summary),
			Sumber:  firstNonEmpty(nama, namaMedia(namaFeed, link)),
			Domain:  domain(link),
			Tanggal: formatTanggal(e.Updated),
			Terbit:  rfc3339(e.Updated),
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("feed terbaca tapi tidak berisi artikel — periksa URL feed")
	}
	return out, nil
}

// --- util teks ---

var (
	reTag = regexp.MustCompile(`(?s)<[^>]*>`)
	reImg = regexp.MustCompile(`(?i)<img[^>]+src\s*=\s*["']([^"']+)["']`)
	reWS  = regexp.MustCompile(`\s+`)
)

func buangTag(s string) string { return reTag.ReplaceAllString(s, " ") }

// gambarDalam mencari <img src> yang diselipkan di dalam deskripsi RSS —
// kebiasaan ANTARA dan beberapa media lain.
func gambarDalam(s string) string {
	if m := reImg.FindStringSubmatch(s); len(m) == 2 {
		return html.UnescapeString(m[1])
	}
	return ""
}

func bersih(s string) string {
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	return strings.TrimSpace(reWS.ReplaceAllString(s, " "))
}

// potong memangkas teks di batas kata terdekat agar tidak terputus di tengah.
func potong(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)[:n]
	if i := strings.LastIndex(string(r), " "); i > n/2 {
		return strings.TrimSpace(string(r)[:i]) + "…"
	}
	return strings.TrimSpace(string(r)) + "…"
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

func domain(u string) string {
	s := u
	for _, p := range []string{"https://", "http://"} {
		s = strings.TrimPrefix(s, p)
	}
	s = strings.TrimPrefix(s, "www.")
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return s
}

// namaMedia memilih nama yang enak dibaca untuk badge kartu. Judul channel RSS
// sering panjang ("ANTARA News - Berita Terkini..."), jadi dipangkas di
// pemisah umum; bila tetap tidak masuk akal, pakai domainnya.
func namaMedia(judulFeed, link string) string {
	j := judulFeed
	for _, sep := range []string{" - ", " | ", " – ", ":"} {
		if i := strings.Index(j, sep); i > 0 {
			j = j[:i]
		}
	}
	j = strings.TrimSpace(j)
	if j == "" || len([]rune(j)) > 24 {
		return domain(link)
	}
	return j
}

// formatWaktu = pola tanggal yang muncul di feed Indonesia.
var formatWaktu = []string{
	time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822,
	time.RFC3339, "Mon, 2 Jan 2006 15:04:05 -0700", "2006-01-02 15:04:05",
}

func parseWaktu(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, f := range formatWaktu {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

var namaBulan = []string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember"}

var namaHari = map[time.Weekday]string{
	time.Sunday: "Minggu", time.Monday: "Senin", time.Tuesday: "Selasa",
	time.Wednesday: "Rabu", time.Thursday: "Kamis", time.Friday: "Jumat",
	time.Saturday: "Sabtu",
}

// formatTanggal menulis tanggal dalam bahasa Indonesia untuk ditempel di kartu.
func formatTanggal(s string) string {
	t, ok := parseWaktu(s)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s, %d %s %d", namaHari[t.Weekday()], t.Day(), namaBulan[t.Month()], t.Year())
}

func rfc3339(s string) string {
	if t, ok := parseWaktu(s); ok {
		return t.Format(time.RFC3339)
	}
	return ""
}
