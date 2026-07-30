package news

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Paragraf satu blok teks dari badan artikel.
//
// Indeks-lah yang dikirim ke dan diterima dari LLM. Dengan begitu LLM tidak
// pernah memegang kesempatan menulis teks sendiri: ia hanya menunjuk nomor,
// dan engine yang mengambil kalimat aslinya. Verbatim terjamin oleh bentuk
// datanya, bukan oleh kepatuhan model pada instruksi.
type Paragraf struct {
	Indeks int    `json:"indeks"`
	Teks   string `json:"teks"`
}

// Isi = artikel beserta badan teksnya yang sudah dipecah jadi paragraf.
type Isi struct {
	Artikel   Artikel    `json:"artikel"`
	Paragraf  []Paragraf `json:"paragraf"`
	JumlahKat int        `json:"jumlah_kata"`
}

// minKataParagraf = ambang panjang sebuah blok agar dianggap paragraf berita.
// Di bawah ini biasanya menu, label, keterangan foto, atau tombol berbagi.
const minKataParagraf = 12

// AmbilIsi membaca artikel sekaligus badan teksnya.
//
// Metodenya: ambil isi <p> lebih dulu, sebab hampir semua media Indonesia
// menaruh badan berita di sana. Bila hasilnya terlalu sedikit (ada situs yang
// memakai <div> per paragraf), dicoba lagi dengan <div> berisi teks saja.
func AmbilIsi(ctx context.Context, halaman string) (Isi, error) {
	art, err := Ambil(ctx, halaman)
	if err != nil {
		return Isi{}, err
	}
	raw, err := ambil(ctx, art.URL)
	if err != nil {
		return Isi{}, err
	}
	par := uraiParagraf(string(raw))
	if len(par) == 0 {
		return Isi{}, fmt.Errorf(
			"badan artikel tidak terbaca dari %s — halaman mungkin berbayar, atau isinya dimuat lewat JavaScript. "+
				"Coba tempel artikel lain, atau salin sendiri paragrafnya", art.Domain)
	}
	kata := 0
	for _, p := range par {
		kata += len(strings.Fields(p.Teks))
	}
	return Isi{Artikel: art, Paragraf: par, JumlahKat: kata}, nil
}

var (
	reP    = regexp.MustCompile(`(?is)<p\b[^>]*>(.*?)</\s*p\s*>`)
	reDiv  = regexp.MustCompile(`(?is)<div\b[^>]*>([^<]{60,})</\s*div\s*>`)
	reBody = regexp.MustCompile(`(?is)<body\b[^>]*>(.*)</\s*body\s*>`)

	// Blok yang isinya tidak pernah jadi badan berita — dibuang lebih dulu
	// beserta isinya, supaya menu & skrip tidak terbaca sebagai paragraf.
	//
	// Satu regex per tag karena RE2 (mesin regexp Go) tidak mendukung
	// backreference, jadi `<(a|b)>...</\1>` tidak bisa dipakai.
	reBuang = buangTagBlok(
		"script", "style", "noscript", "nav", "header", "footer",
		"aside", "form", "figcaption", "iframe",
	)
)

func buangTagBlok(tag ...string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(tag))
	for _, t := range tag {
		out = append(out, regexp.MustCompile(`(?is)<`+t+`\b[^>]*>.*?</\s*`+t+`\s*>`))
	}
	return out
}

// frasaSampah = penanda blok yang jelas bukan isi berita meski panjangnya cukup.
var frasaSampah = []string{
	"baca juga", "simak juga", "lihat juga", "berita terkait", "artikel terkait",
	"copyright", "hak cipta", "all rights reserved", "editor:", "penyunting:",
	"ikuti kami", "berlangganan", "unduh aplikasi", "advertisement",
	"cookie", "kebijakan privasi", "syarat dan ketentuan",
}

// uraiParagraf memecah HTML jadi paragraf badan berita.
func uraiParagraf(h string) []Paragraf {
	if m := reBody.FindStringSubmatch(h); len(m) == 2 {
		h = m[1]
	}
	for _, re := range reBuang {
		h = re.ReplaceAllString(h, " ")
	}

	kandidat := petikBlok(h, reP)
	if len(kandidat) < 2 {
		// Situs yang memakai <div> per paragraf. Sengaja hanya <div> yang
		// isinya teks polos (tanpa tag anak) agar tidak menangkap pembungkus
		// besar yang memuat seluruh halaman.
		kandidat = append(kandidat, petikBlok(h, reDiv)...)
	}

	var out []Paragraf
	lihat := map[string]bool{}
	for _, t := range kandidat {
		if len(strings.Fields(t)) < minKataParagraf || lihat[t] {
			continue
		}
		if sampah(t) {
			continue
		}
		lihat[t] = true
		out = append(out, Paragraf{Indeks: len(out), Teks: t})
	}
	return out
}

func petikBlok(h string, re *regexp.Regexp) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(h, -1) {
		if len(m) < 2 {
			continue
		}
		t := bersih(buangTag(m[1]))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func sampah(t string) bool {
	low := strings.ToLower(t)
	for _, f := range frasaSampah {
		if strings.Contains(low, f) {
			return true
		}
	}
	return false
}

// Gabung menyatukan paragraf jadi satu teks — dipakai untuk memeriksa apakah
// kata kunci pilihan LLM benar-benar muncul di artikel.
func (i Isi) Gabung() string {
	b := make([]string, 0, len(i.Paragraf))
	for _, p := range i.Paragraf {
		b = append(b, p.Teks)
	}
	return strings.Join(b, "\n")
}

// Bernomor menuliskan paragraf dengan nomornya, siap dikirim ke LLM.
func (i Isi) Bernomor(maksKata int) string {
	var sb strings.Builder
	kata := 0
	for _, p := range i.Paragraf {
		n := len(strings.Fields(p.Teks))
		// Artikel yang sangat panjang dipotong agar muat di konteks model lokal;
		// paragraf awal berita hampir selalu memuat inti (piramida terbalik).
		if maksKata > 0 && kata+n > maksKata {
			break
		}
		kata += n
		fmt.Fprintf(&sb, "[%d] %s\n\n", p.Indeks, p.Teks)
	}
	return strings.TrimSpace(sb.String())
}

// Teks mengembalikan paragraf pada indeks tertentu. ok=false bila di luar
// jangkauan — LLM sesekali menyebut nomor yang tidak ada.
func (i Isi) Teks(indeks int) (string, bool) {
	for _, p := range i.Paragraf {
		if p.Indeks == indeks {
			return p.Teks, true
		}
	}
	return "", false
}

// Tagar mengubah kata kunci jadi tagar.
//
// Hanya kata kunci yang BENAR-BENAR muncul di artikel yang diterima; sisanya
// dibuang. Ini menutup satu-satunya celah mengarang yang tersisa, sebab tagar
// tidak bisa berupa kutipan utuh seperti paragraf.
func (i Isi) Tagar(kunci []string, maks int) []string {
	isi := strings.ToLower(i.Gabung())
	var out []string
	lihat := map[string]bool{}
	for _, k := range kunci {
		k = strings.TrimSpace(k)
		if k == "" || !strings.Contains(isi, strings.ToLower(k)) {
			continue
		}
		t := "#" + rapikanTagar(k)
		if len(t) < 4 || lihat[t] {
			continue
		}
		lihat[t] = true
		out = append(out, t)
		if maks > 0 && len(out) >= maks {
			break
		}
	}
	return out
}

var reBukanHuruf = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// rapikanTagar menyatukan kata jadi satu tagar ber-huruf besar di tiap kata:
// "Jakarta Barat" → "JakartaBarat".
func rapikanTagar(s string) string {
	bagian := reBukanHuruf.Split(s, -1)
	var sb strings.Builder
	for _, b := range bagian {
		if b == "" {
			continue
		}
		r := []rune(b)
		sb.WriteString(strings.ToUpper(string(r[0])))
		sb.WriteString(string(r[1:]))
	}
	return sb.String()
}

// UrutPeringkat mengurutkan hasil penilaian dari skor tertinggi.
func UrutPeringkat(p []Peringkat) {
	sort.SliceStable(p, func(a, b int) bool { return p[a].Skor > p[b].Skor })
}
