// Package card menyusun kartu berita siap posting: data artikel dituang ke
// template HTML berukuran pas (mis. 1080x1920), lalu difoto oleh Chrome.
//
// Berbeda dari memotret situs aslinya, di sini kita yang menentukan tata letak
// — jadi tidak ada iklan yang ikut terbawa, judul tidak terpotong, dan ukuran
// hasilnya sudah pas untuk media sosial tanpa perlu dipotong lagi.
package card

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gemgum/clipper/engine/internal/capture"
	"github.com/gemgum/clipper/engine/internal/news"
)

// Gaya tampilan kartu.
const (
	StyleDark  = "dark"
	StyleLight = "light"
	StyleQuote = "quote" // judul besar tanpa foto
)

// Rasio keluaran.
const (
	Ratio916 = "9:16"
	Ratio45  = "4:5"
	Ratio11  = "1:1"
)

// Perataan teks judul & ringkasan di kartu.
const (
	AlignLeft    = "left"
	AlignCenter  = "center"
	AlignRight   = "right"
	AlignJustify = "justify" // rata kiri-kanan
)

// alignCSS menerjemahkan pilihan perataan ke nilai text-align, sekaligus
// menentukan margin garis aksen agar ikut berpindah mengikuti teks —
// kalau tidak, garisnya menggantung sendirian di kiri.
func alignCSS(a string) (align, ruleMargin string) {
	switch a {
	case AlignCenter:
		return "center", "0 auto 32px auto"
	case AlignRight:
		return "right", "0 0 32px auto"
	case AlignJustify:
		return "justify", "0 auto 32px 0"
	default: // left
		return "left", "0 auto 32px 0"
	}
}

// Photo mengatur bingkai foto artikel: digeser dan diperbesar di dalam area
// fotonya. Hanya memengaruhi gambar — tata letak teks di bawahnya tetap.
//
// Satuan geser memakai piksel di ruang koordinat kartu (lebar 1080), sama
// dengan yang dipakai GUI, sehingga menyeret sejauh N piksel di pratinjau
// menggeser foto sejauh N piksel juga di hasil akhir.
type Photo struct {
	OffsetX int     `json:"offset_x"`
	OffsetY int     `json:"offset_y"`
	Zoom    float64 `json:"zoom"` // 1 = ukuran pas bingkai
}

// Request pembuatan satu kartu.
type Request struct {
	Article news.Article `json:"article"`
	Style   string       `json:"style"`
	Ratio   string       `json:"ratio"`
	Align   string       `json:"align"` // left | center | right | justify
	Photo   Photo        `json:"photo"`
	Lang    string       `json:"lang"` // bahasa teks tetap di kartu (en | id)
	// Caption & Hashtags ikut disimpan sebagai berkas pendamping agar satu ZIP
	// berisi semua yang dibutuhkan saat memposting.
	Caption  string   `json:"caption"`
	Hashtags []string `json:"hashtags"`
}

// Nama berkas di dalam folder kartu.
const (
	FilePNG     = "card.png"
	FileCaption = "caption.txt"
	FileSource  = "source.txt"
)

// Dims menerjemahkan rasio ke ukuran piksel (lebar tetap 1080).
func Dims(ratio string) (int, int) {
	switch ratio {
	case Ratio45:
		return 1080, 1350
	case Ratio11:
		return 1080, 1080
	default:
		return 1080, 1920
	}
}

// Teks tetap yang ikut tercetak di kartu & berkas pendampingnya. Ditaruh di
// satu tempat supaya menambah bahasa baru cukup menambah satu entri.
type phrases struct {
	readMore   string
	labelTitle string
	labelMedia string
	labelDate  string
	labelLink  string
	notice     string
}

var phraseBook = map[string]phrases{
	"en": {
		readMore:   "read the full story",
		labelTitle: "Title ",
		labelMedia: "Media ",
		labelDate:  "Date  ",
		labelLink:  "Link  ",
		notice:     "The photo and text remain the property of the outlet above. Credit the source when posting.",
	},
	"id": {
		readMore:   "baca selengkapnya",
		labelTitle: "Judul  ",
		labelMedia: "Media  ",
		labelDate:  "Tanggal",
		labelLink:  "Tautan ",
		notice:     "Foto dan teks tetap milik media di atas. Cantumkan sumber saat memposting.",
	},
}

func phrasesFor(lang string) phrases {
	if p, ok := phraseBook[strings.ToLower(strings.TrimSpace(lang))]; ok {
		return p
	}
	return phraseBook["en"]
}

// langAttr mengembalikan nilai atribut lang= untuk <html>.
func langAttr(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if _, ok := phraseBook[lang]; ok {
		return lang
	}
	return "en"
}

// Builder membuat kartu memakai browser & font bawaan proyek.
type Builder struct {
	cap      *capture.Client
	fontsDir string

	once    sync.Once
	fontCSS template.CSS // @font-face dengan font ditanam sebagai data URI
}

func New(cap *capture.Client, fontsDir string) *Builder {
	return &Builder{cap: cap, fontsDir: fontsDir}
}

// Build merender kartu ke dalam folder dir, bersama berkas pendampingnya
// (caption & sumber) supaya siap dibungkus jadi satu ZIP.
func (b *Builder) Build(ctx context.Context, req Request, dir string) error {
	if strings.TrimSpace(req.Article.Title) == "" {
		return fmt.Errorf("the title is empty — a card needs the article title")
	}
	outPNG := filepath.Join(dir, FilePNG)
	width, height := Dims(req.Ratio)

	htmlBytes, err := b.render(req, width, height)
	if err != nil {
		return err
	}

	// Halaman ditulis ke berkas sementara karena Chrome perlu URL untuk dibuka;
	// WriteTempFile menaruhnya di lokasi yang terjangkau browser (termasuk saat
	// chrome.exe dipanggil dari WSL).
	url, cleanup, err := b.cap.WriteTempFile(htmlBytes, ".html")
	if err != nil {
		return err
	}
	defer cleanup()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := b.cap.Screenshot(ctx, url, outPNG, capture.Options{
		Width:  width,
		Height: height,
		Scale:  1,
		// Foto artikel diambil dari CDN media, jadi beri waktu memuat.
		WaitMS: 15000,
	}); err != nil {
		return err
	}
	return writeSidecars(dir, req)
}

// writeSidecars menulis caption & keterangan sumber di sebelah gambar.
//
// Sumber selalu ditulis, walau caption kosong: atribusi harus ikut berpindah
// bersama berkasnya, supaya saat kartu dibuka lagi berminggu-minggu kemudian
// asal beritanya tidak hilang.
func writeSidecars(dir string, req Request) error {
	if c := strings.TrimSpace(req.Caption); c != "" || len(req.Hashtags) > 0 {
		var sb strings.Builder
		sb.WriteString(c)
		if len(req.Hashtags) > 0 {
			sb.WriteString("\n\n")
			sb.WriteString(strings.Join(req.Hashtags, " "))
		}
		sb.WriteString("\n")
		if err := os.WriteFile(filepath.Join(dir, FileCaption), []byte(sb.String()), 0o644); err != nil {
			return err
		}
	}
	a := req.Article
	p := phrasesFor(req.Lang)
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s: %s\n", p.labelTitle, a.Title)
	fmt.Fprintf(&sb, "%s: %s\n", p.labelMedia, firstNonEmpty(a.Source, a.Domain))
	fmt.Fprintf(&sb, "%s: %s\n", p.labelDate, a.Date)
	fmt.Fprintf(&sb, "%s: %s\n", p.labelLink, a.URL)
	fmt.Fprintf(&sb, "\n%s\n", p.notice)
	return os.WriteFile(filepath.Join(dir, FileSource), []byte(sb.String()), 0o644)
}

// templateData = data siap pakai untuk template.
type templateData struct {
	Width, Height int
	Dark          bool
	Quote         bool
	HasImage      bool
	Image         string
	Source        string
	Domain        string
	Date          string
	FontCSS       template.CSS
	PhotoHeight   int
	PhotoX        int
	PhotoY        int
	PhotoZoom     string // ditulis sebagai teks agar tidak jadi notasi ilmiah
	Align         string // nilai text-align
	RuleMargin    string // margin blok, ikut perataan
	Lang          string // atribut lang= pada <html>
	ReadMore      string // teks kaki kartu

	// Hero = teks yang jadi bintang kartu, ditaruh di guntingan kertas.
	// Context = judul artikel sebagai keterangan di atasnya; kosong bila judul
	// itu sendiri yang jadi Hero (artikel tanpa kutipan terpilih).
	Hero        string
	Context     string
	HeroSize    int
	ContextSize int
	SmallSize   int
	Padding     int
}

func (b *Builder) render(req Request, width, height int) ([]byte, error) {
	a := req.Article
	quote := req.Style == StyleQuote
	hasImage := strings.HasPrefix(a.Image, "http") && !quote

	// Skala mengikuti tinggi kanvas supaya kartu 1:1 dan 4:5 tidak terlihat
	// seperti kartu 9:16 yang dipangkas.
	scale := float64(height) / 1920.0
	px := func(n int) int { return int(float64(n) * scale) }

	// Hierarki sengaja dibalik dari kartu berita pada umumnya: yang jadi bintang
	// adalah PARAGRAF TERPILIH, bukan judul. Alasannya bukan gaya — seluruh
	// fitur ini memang bekerja dengan memilih satu paragraf, jadi paragraf itulah
	// muatan kartunya. Judul turun pangkat jadi keterangan di atasnya.
	//
	// Bila tidak ada paragraf terpilih, judul naik jadi Hero supaya kartu tetap
	// punya isi.
	hero := strings.TrimSpace(a.Summary)
	context := strings.TrimSpace(a.Title)
	if hero == "" {
		hero, context = context, ""
	}

	// Teks panjang dikecilkan agar tetap muat tanpa terpotong. Ambangnya dari
	// percobaan pada guntingan selebar 1080 dikurangi padding.
	chars := len([]rune(hero))
	heroSize := 62
	switch {
	case chars > 320:
		heroSize = 38
	case chars > 240:
		heroSize = 44
	case chars > 170:
		heroSize = 50
	case chars > 110:
		heroSize = 56
	}
	if quote {
		heroSize += 14 // tanpa foto, ruangnya lebih lega
	}

	// Isi kartu dijangkarkan ke bawah, jadi sisa ruang jatuh tepat di bawah foto.
	// Porsi foto dibuat agak besar supaya ruang itu terisi gambar, bukan jadi
	// pita gelap kosong.
	photoHeight := 50
	if req.Ratio == Ratio11 {
		// Kanvas persegi jauh lebih pendek, jadi porsi foto yang sama menyisakan
		// pita gelap saat kutipannya singkat.
		photoHeight = 48
	}
	// Ukuran bingkai foto dalam piksel — dipakai untuk membatasi geseran.
	frameHeight := height * photoHeight / 100
	zoom := clampZoom(req.Photo.Zoom)
	align, ruleMargin := alignCSS(req.Align)
	p := phrasesFor(req.Lang)

	d := templateData{
		Width: width, Height: height,
		Dark:        req.Style != StyleLight,
		Quote:       quote,
		HasImage:    hasImage,
		Image:       a.Image,
		Hero:        hero,
		Context:     context,
		Source:      firstNonEmpty(a.Source, a.Domain),
		Domain:      a.Domain,
		Date:        a.Date,
		FontCSS:     b.fonts(),
		PhotoHeight: photoHeight,
		PhotoX:      clamp(req.Photo.OffsetX, offsetLimit(width, zoom)),
		PhotoY:      clamp(req.Photo.OffsetY, offsetLimit(frameHeight, zoom)),
		PhotoZoom:   strconv.FormatFloat(zoom, 'f', 3, 64),
		Align:       align,
		RuleMargin:  ruleMargin,
		Lang:        langAttr(req.Lang),
		ReadMore:    p.readMore,
		HeroSize:    px(heroSize),
		ContextSize: px(38),
		SmallSize:   px(26),
		Padding:     px(64),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, d); err != nil {
		return nil, fmt.Errorf("could not compose the card: %v", err)
	}
	return buf.Bytes(), nil
}

// fonts menanam font bawaan proyek sebagai data URI.
//
// Ditanam, bukan dirujuk lewat path, karena Chrome bisa saja berjalan di sisi
// Windows sementara fontnya ada di sisi Linux — dan supaya kartu tampil sama
// persis di komputer siapa pun, terlepas dari font yang terpasang di sana.
func (b *Builder) fonts() template.CSS {
	b.once.Do(func() {
		var sb strings.Builder
		embed := func(file, family string) {
			raw, err := os.ReadFile(filepath.Join(b.fontsDir, file))
			if err != nil {
				return // font tidak ada → jatuh ke font sistem lewat daftar cadangan
			}
			fmt.Fprintf(&sb,
				"@font-face{font-family:'%s';src:url(data:font/ttf;base64,%s) format('truetype');font-weight:100 900;font-display:block}\n",
				family, base64.StdEncoding.EncodeToString(raw))
		}
		embed("Montserrat.ttf", "Clipper Sans")
		embed("Anton.ttf", "Clipper Display")
		embed("BebasNeue.ttf", "Clipper Condensed")
		b.fontCSS = template.CSS(sb.String())
	})
	return b.fontCSS
}

// clampZoom menjaga zoom di rentang masuk akal. Nilai 0 datang dari permintaan
// lama yang belum mengenal field ini, jadi diartikan sebagai "tanpa zoom".
//
// Batas bawahnya 1: di bawah itu foto lebih kecil dari bingkainya dan menyisakan
// celah kosong di tepi kartu.
func clampZoom(z float64) float64 {
	switch {
	case z <= 1:
		return 1
	case z > 4:
		return 4
	}
	return z
}

// offsetLimit = sejauh mana foto boleh digeser tanpa memunculkan celah kosong.
// Pada zoom Z, foto jadi Z kali ukuran bingkai, jadi sisa ruangnya (Z-1)/2 di
// tiap sisi. Pada zoom 1 hasilnya 0 — foto pas bingkai, tak ada yang bisa
// digeser. GUI memakai rumus yang sama supaya seretannya berhenti di tempat
// yang sama dengan hasil render.
func offsetLimit(size int, zoom float64) int {
	return int(float64(size) * (zoom - 1) / 2)
}

func clamp(v, limit int) int {
	if v > limit {
		return limit
	}
	if v < -limit {
		return -limit
	}
	return v
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

// tmpl memakai html/template sehingga judul & ringkasan dari situs luar
// otomatis di-escape — teks berisi < atau & tidak bisa merusak tata letak,
// apalagi menyisipkan skrip.
var tmpl = template.Must(template.New("card").Parse(`<!doctype html>
<html lang="{{.Lang}}"><head><meta charset="utf-8"><style>
{{.FontCSS}}
*{margin:0;padding:0;box-sizing:border-box}
/* Palet sengaja menghindari merah. Hampir semua media Indonesia memakai merah
   di logonya, jadi aksen merah pada kartu akan bertabrakan dengan lencana
   sumbernya sendiri. Kuning penanda dipilih karena berbeda dari warna media
   mana pun, dan karena artinya jelas: bagian ini ditandai. */
:root{
  --ink:#14171C;          /* biru-kehitaman, seperti tinta cetak */
  --paper:{{if .Dark}}#EFEBE1{{else}}#FBF9F4{{end}};
  --ink-on-paper:#1A1714; /* hitam kehangatan, untuk teks di atas kertas */
  --highlight:#E4B429;    /* kuning penanda — satu-satunya aksen */
}
html,body{width:{{.Width}}px;height:{{.Height}}px;overflow:hidden}
body{
  font-family:'Clipper Sans',"Segoe UI",Roboto,Arial,sans-serif;
  display:flex;flex-direction:column;
  {{if .Dark}}background:var(--ink);color:var(--paper)
  {{else}}background:#DDD8CC;color:#1A1714{{end}}
}
.photo{position:relative;height:{{.PhotoHeight}}%;flex:none;overflow:hidden}
/* object-fit:cover dipakai—bukan min-width/min-height dengan width:auto—karena
   yang terakhir membuat penskalaan bergantung pada ukuran asli gambar relatif
   terhadap kotaknya. Akibatnya CSS yang sama memberi hasil berbeda di kotak
   1080px (render) dan kotak ±380px (pratinjau GUI). object-fit:cover selalu
   memenuhi bingkai dengan rasio terjaga, apa pun ukuran kotak dan gambarnya.

   Urutan translate lalu scale disengaja: jarak geser tidak ikut terkalikan zoom,
   sehingga menyeret N piksel di pratinjau = N piksel di hasil render. */
.photo img{
  position:absolute;left:50%;top:50%;
  width:100%;height:100%;object-fit:cover;display:block;
  transform:translate(-50%,-50%) translate({{.PhotoX}}px,{{.PhotoY}}px) scale({{.PhotoZoom}});
  transform-origin:center;
}
.photo::after{content:"";position:absolute;inset:0;
  background:linear-gradient(180deg,rgba(0,0,0,.4) 0%,rgba(0,0,0,0) 26%,
    {{if .Dark}}rgba(20,23,28,0) 58%,rgba(20,23,28,.9) 86%,var(--ink) 100%
    {{else}}rgba(221,216,204,0) 58%,rgba(221,216,204,.92) 86%,#DDD8CC 100%{{end}})}
.badge{position:absolute;top:{{.Padding}}px;left:{{.Padding}}px;z-index:2;
  display:flex;align-items:center;gap:14px;flex-wrap:wrap}
/* Lencana sumber memakai warna kertas, bukan warna media. Kita tidak tahu
   warna merek mereka yang sebenarnya, dan menebaknya sama saja memalsukan
   identitas orang. Netral lebih jujur. */
.badge b{background:var(--paper);color:var(--ink-on-paper);
  font-family:'Clipper Condensed','Clipper Sans',sans-serif;font-weight:400;
  font-size:{{.SmallSize}}px;letter-spacing:.16em;
  padding:10px 20px;border-radius:3px;text-transform:uppercase}
.badge span{background:rgba(0,0,0,.5);color:var(--paper);font-size:{{.SmallSize}}px;
  font-family:'Clipper Condensed','Clipper Sans',sans-serif;letter-spacing:.1em;
  padding:10px 18px;border-radius:4px}
/* Dengan foto: isi dijangkarkan ke bawah, sisa ruang jatuh ke bagian foto yang
   sudah memudar — tidak ada rongga menganga di bawah guntingan.
   Tanpa foto: dijangkarkan ke bawah justru menyisakan setengah kartu kosong di
   atas, jadi isinya ditaruh di tengah. */
.content{flex:1;padding:0 {{.Padding}}px {{.Padding}}px;display:flex;flex-direction:column;
  position:relative;z-index:2;
  {{if .HasImage}}justify-content:flex-end;margin-top:-72px
  {{else}}justify-content:center;padding-top:{{.Padding}}px{{end}}}

/* Tanda mulai kutipan. Menandai di mana bahan orang lain mulai dikutip —
   bukan hiasan, dan itu sebabnya ia ikut berpindah mengikuti perataan. */
.rule{width:64px;height:8px;background:var(--highlight);margin:{{.RuleMargin}};flex:none}

/* Judul artikel turun pangkat jadi keterangan: huruf kondensa, kapital,
   direnggangkan. Ia menjelaskan kutipan di bawahnya, bukan bersaing dengannya. */
.context{font-family:'Clipper Condensed','Clipper Sans',sans-serif;
  font-size:{{.ContextSize}}px;line-height:1.12;letter-spacing:.045em;
  text-transform:uppercase;text-align:{{.Align}};margin-bottom:30px;
  /* Judul panjang dipangkas dua baris: perannya menjelaskan, bukan bersaing
     dengan kutipan. Tanpa batas ini judul 15 kata bisa mendorong guntingan
     keluar kartu. */
  display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;
  {{if .Dark}}color:#9AA3AD{{else}}color:#5C5750{{end}}}

/* GUNTINGAN — inti desain ini. Kutipan disajikan sebagai potongan kertas yang
   ditempel, sebab yang dijual fitur ini adalah bukti: teksnya diambil apa
   adanya, bukan ditulis ulang. Tinta gelap di atas kertas terang juga pilihan
   paling terbaca di layar ponsel, dan keterbacaan itulah alasan mode kartu ada. */
.clipping{
  background:var(--paper);color:var(--ink-on-paper);
  padding:{{.Padding}}px;border-radius:3px;
  transform:rotate(-.45deg);
  box-shadow:0 20px 44px rgba(0,0,0,{{if .Dark}}.5{{else}}.22{{end}});
}
.hero{font-size:{{.HeroSize}}px;line-height:1.34;font-weight:600;
  letter-spacing:-.005em;text-align:{{.Align}};
  {{if .Quote}}font-family:'Clipper Display','Clipper Sans',Impact,sans-serif;
  font-weight:400;line-height:1.1;letter-spacing:0;{{end}}}

/* Stempel asal ditaruh DI ATAS guntingan, bukan di kaki kartu: pada selembar
   kutipan, dari mana ia berasal adalah bagian dari kutipannya sendiri. */
.stamp{margin-top:34px;font-family:'Clipper Condensed','Clipper Sans',sans-serif;
  font-size:{{.SmallSize}}px;letter-spacing:.13em;text-transform:uppercase;
  text-align:{{.Align}}}
.stamp span{background:var(--highlight);padding:5px 14px;border-radius:2px;
  color:#1A1714;box-decoration-break:clone;-webkit-box-decoration-break:clone}

.footer{padding-top:28px;font-family:'Clipper Condensed','Clipper Sans',sans-serif;
  font-size:{{.SmallSize}}px;letter-spacing:.12em;text-transform:uppercase;
  {{if .Dark}}color:#79818B{{else}}color:#6B665E{{end}}}
</style></head><body>
{{if .HasImage}}
<div class="photo">
  <div class="badge"><b>{{.Source}}</b></div>
  <img src="{{.Image}}" alt="">
</div>
{{else}}
<div class="badge" style="position:static;margin:{{.Padding}}px {{.Padding}}px 0">
  <b>{{.Source}}</b>
</div>
{{end}}
<div class="content">
  <div class="rule"></div>
  {{if .Context}}<div class="context">{{.Context}}</div>{{end}}
  <div class="clipping">
    <div class="hero">{{.Hero}}</div>
    <div class="stamp"><span>{{.Domain}}{{if .Date}} &middot; {{.Date}}{{end}}</span></div>
  </div>
  <div class="footer">{{.ReadMore}}</div>
</div>
</body></html>`))
