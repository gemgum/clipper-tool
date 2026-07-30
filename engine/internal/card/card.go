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
	GayaGelap   = "gelap"
	GayaTerang  = "terang"
	GayaKutipan = "kutipan" // judul besar tanpa foto
)

// Rasio keluaran.
const (
	Rasio916 = "9:16"
	Rasio45  = "4:5"
	Rasio11  = "1:1"
)

// Perataan teks judul & ringkasan di kartu.
const (
	RataKiri   = "kiri"
	RataTengah = "tengah"
	RataKanan  = "kanan"
	RataPenuh  = "penuh" // rata kiri-kanan (justify)
)

// rataCSS menerjemahkan pilihan perataan ke nilai text-align, sekaligus
// menentukan margin garis aksen agar ikut berpindah mengikuti teks —
// kalau tidak, garisnya menggantung sendirian di kiri.
func rataCSS(r string) (align, garisMargin string) {
	switch r {
	case RataTengah:
		return "center", "0 auto 32px auto"
	case RataKanan:
		return "right", "0 0 32px auto"
	case RataPenuh:
		return "justify", "0 auto 32px 0"
	default: // kiri
		return "left", "0 auto 32px 0"
	}
}

// Foto mengatur bingkai foto artikel: digeser dan diperbesar di dalam area
// fotonya. Hanya memengaruhi gambar — tata letak teks di bawahnya tetap.
//
// Satuan geser memakai piksel di ruang koordinat kartu (lebar 1080), sama
// dengan yang dipakai GUI, sehingga menyeret sejauh N piksel di pratinjau
// menggeser foto sejauh N piksel juga di hasil akhir.
type Foto struct {
	GeserX int     `json:"geser_x"`
	GeserY int     `json:"geser_y"`
	Zoom   float64 `json:"zoom"` // 1 = ukuran pas bingkai
}

// Permintaan pembuatan satu kartu.
type Permintaan struct {
	Artikel news.Artikel `json:"artikel"`
	Gaya    string       `json:"gaya"`
	Rasio   string       `json:"rasio"`
	Rata    string       `json:"rata"` // kiri | tengah | kanan | penuh
	Foto    Foto         `json:"foto"`
	// Caption & Hashtag ikut disimpan sebagai berkas pendamping agar satu ZIP
	// berisi semua yang dibutuhkan saat memposting.
	Caption string   `json:"caption"`
	Hashtag []string `json:"hashtag"`
}

// Nama berkas di dalam folder kartu.
const (
	BerkasPNG     = "kartu.png"
	BerkasCaption = "caption.txt"
	BerkasSumber  = "sumber.txt"
)

// Dims menerjemahkan rasio ke ukuran piksel (lebar tetap 1080).
func Dims(rasio string) (int, int) {
	switch rasio {
	case Rasio45:
		return 1080, 1350
	case Rasio11:
		return 1080, 1080
	default:
		return 1080, 1920
	}
}

// Builder membuat kartu memakai browser & font bawaan proyek.
type Builder struct {
	cap      *capture.Client
	fontsDir string

	sekali  sync.Once
	fontCSS template.CSS // @font-face dengan font ditanam sebagai data URI
}

func New(cap *capture.Client, fontsDir string) *Builder {
	return &Builder{cap: cap, fontsDir: fontsDir}
}

// Buat merender kartu ke dalam folder dir, bersama berkas pendampingnya
// (caption & sumber) supaya siap dibungkus jadi satu ZIP.
func (b *Builder) Buat(ctx context.Context, p Permintaan, dir string) error {
	if strings.TrimSpace(p.Artikel.Judul) == "" {
		return fmt.Errorf("judul kosong — kartu butuh judul artikel")
	}
	outPNG := filepath.Join(dir, BerkasPNG)
	lebar, tinggi := Dims(p.Rasio)

	htmlBytes, err := b.render(p, lebar, tinggi)
	if err != nil {
		return err
	}

	// Halaman ditulis ke berkas sementara karena Chrome perlu URL untuk dibuka;
	// SiapkanBerkas menaruhnya di lokasi yang terjangkau browser (termasuk saat
	// chrome.exe dipanggil dari WSL).
	url, bersih, err := b.cap.SiapkanBerkas(htmlBytes, ".html")
	if err != nil {
		return err
	}
	defer bersih()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := b.cap.Tangkap(ctx, url, outPNG, capture.Opsi{
		Lebar:  lebar,
		Tinggi: tinggi,
		Skala:  1,
		// Foto artikel diambil dari CDN media, jadi beri waktu memuat.
		Tunggu: 15000,
	}); err != nil {
		return err
	}
	return tulisPendamping(dir, p)
}

// tulisPendamping menulis caption & keterangan sumber di sebelah gambar.
//
// Sumber selalu ditulis, walau caption kosong: atribusi harus ikut berpindah
// bersama berkasnya, supaya saat kartu dibuka lagi berminggu-minggu kemudian
// asal beritanya tidak hilang.
func tulisPendamping(dir string, p Permintaan) error {
	if c := strings.TrimSpace(p.Caption); c != "" || len(p.Hashtag) > 0 {
		var sb strings.Builder
		sb.WriteString(c)
		if len(p.Hashtag) > 0 {
			sb.WriteString("\n\n")
			sb.WriteString(strings.Join(p.Hashtag, " "))
		}
		sb.WriteString("\n")
		if err := os.WriteFile(filepath.Join(dir, BerkasCaption), []byte(sb.String()), 0o644); err != nil {
			return err
		}
	}
	a := p.Artikel
	var sb strings.Builder
	fmt.Fprintf(&sb, "Judul  : %s\n", a.Judul)
	fmt.Fprintf(&sb, "Media  : %s\n", firstNonEmpty(a.Sumber, a.Domain))
	fmt.Fprintf(&sb, "Tanggal: %s\n", a.Tanggal)
	fmt.Fprintf(&sb, "Tautan : %s\n", a.URL)
	sb.WriteString("\nFoto dan teks tetap milik media di atas. Cantumkan sumber saat memposting.\n")
	return os.WriteFile(filepath.Join(dir, BerkasSumber), []byte(sb.String()), 0o644)
}

// isiTemplate = data siap pakai untuk template.
type isiTemplate struct {
	Lebar, Tinggi int
	Gelap         bool
	Kutipan       bool
	AdaGambar     bool
	Gambar        string
	Sumber        string
	Domain        string
	Tanggal       string
	FontCSS       template.CSS
	TinggiFoto    int
	FotoX, FotoY  int
	FotoZoom      string // ditulis sebagai teks agar tidak jadi notasi ilmiah
	Rata          string // nilai text-align
	GarisM        string // margin blok, ikut perataan

	// Hero = teks yang jadi bintang kartu, ditaruh di guntingan kertas.
	// Konteks = judul artikel sebagai keterangan di atasnya; kosong bila judul
	// itu sendiri yang jadi Hero (artikel tanpa kutipan terpilih).
	Hero          string
	Konteks       string
	UkuranHero    int
	UkuranKonteks int
	UkuranKecil   int
	Padding       int
}

func (b *Builder) render(p Permintaan, lebar, tinggi int) ([]byte, error) {
	a := p.Artikel
	kutipan := p.Gaya == GayaKutipan
	adaGambar := strings.HasPrefix(a.Gambar, "http") && !kutipan

	// Skala mengikuti tinggi kanvas supaya kartu 1:1 dan 4:5 tidak terlihat
	// seperti kartu 9:16 yang dipangkas.
	skala := float64(tinggi) / 1920.0
	px := func(n int) int { return int(float64(n) * skala) }

	// Hierarki sengaja dibalik dari kartu berita pada umumnya: yang jadi bintang
	// adalah PARAGRAF TERPILIH, bukan judul. Alasannya bukan gaya — seluruh
	// fitur ini memang bekerja dengan memilih satu paragraf, jadi paragraf itulah
	// muatan kartunya. Judul turun pangkat jadi keterangan di atasnya.
	//
	// Bila tidak ada paragraf terpilih, judul naik jadi Hero supaya kartu tetap
	// punya isi.
	hero := strings.TrimSpace(a.Ringkas)
	konteks := strings.TrimSpace(a.Judul)
	if hero == "" {
		hero, konteks = konteks, ""
	}

	// Teks panjang dikecilkan agar tetap muat tanpa terpotong. Ambangnya dari
	// percobaan pada guntingan selebar 1080 dikurangi padding.
	huruf := len([]rune(hero))
	ukuranHero := 62
	switch {
	case huruf > 320:
		ukuranHero = 38
	case huruf > 240:
		ukuranHero = 44
	case huruf > 170:
		ukuranHero = 50
	case huruf > 110:
		ukuranHero = 56
	}
	if kutipan {
		ukuranHero += 14 // tanpa foto, ruangnya lebih lega
	}

	// Isi kartu dijangkarkan ke bawah, jadi sisa ruang jatuh tepat di bawah foto.
	// Porsi foto dibuat agak besar supaya ruang itu terisi gambar, bukan jadi
	// pita gelap kosong.
	tinggiFoto := 50
	if p.Rasio == Rasio11 {
		// Kanvas persegi jauh lebih pendek, jadi porsi foto yang sama menyisakan
		// pita gelap saat kutipannya singkat.
		tinggiFoto = 48
	}
	// Ukuran bingkai foto dalam piksel — dipakai untuk membatasi geseran.
	tinggiBingkai := tinggi * tinggiFoto / 100
	zoom := zoomSah(p.Foto.Zoom)
	align, garisM := rataCSS(p.Rata)

	d := isiTemplate{
		Lebar: lebar, Tinggi: tinggi,
		Gelap:         p.Gaya != GayaTerang,
		Kutipan:       kutipan,
		AdaGambar:     adaGambar,
		Gambar:        a.Gambar,
		Hero:          hero,
		Konteks:       konteks,
		Sumber:        firstNonEmpty(a.Sumber, a.Domain),
		Domain:        a.Domain,
		Tanggal:       a.Tanggal,
		FontCSS:       b.fonts(),
		TinggiFoto:    tinggiFoto,
		FotoX:         jepit(p.Foto.GeserX, batasGeser(lebar, zoom)),
		FotoY:         jepit(p.Foto.GeserY, batasGeser(tinggiBingkai, zoom)),
		FotoZoom:      strconv.FormatFloat(zoom, 'f', 3, 64),
		Rata:          align,
		GarisM:        garisM,
		UkuranHero:    px(ukuranHero),
		UkuranKonteks: px(38),
		UkuranKecil:   px(26),
		Padding:       px(64),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, d); err != nil {
		return nil, fmt.Errorf("gagal menyusun kartu: %v", err)
	}
	return buf.Bytes(), nil
}

// fonts menanam font bawaan proyek sebagai data URI.
//
// Ditanam, bukan dirujuk lewat path, karena Chrome bisa saja berjalan di sisi
// Windows sementara fontnya ada di sisi Linux — dan supaya kartu tampil sama
// persis di komputer siapa pun, terlepas dari font yang terpasang di sana.
func (b *Builder) fonts() template.CSS {
	b.sekali.Do(func() {
		var sb strings.Builder
		tanam := func(berkas, family string) {
			raw, err := os.ReadFile(filepath.Join(b.fontsDir, berkas))
			if err != nil {
				return // font tidak ada → jatuh ke font sistem lewat daftar cadangan
			}
			fmt.Fprintf(&sb,
				"@font-face{font-family:'%s';src:url(data:font/ttf;base64,%s) format('truetype');font-weight:100 900;font-display:block}\n",
				family, base64.StdEncoding.EncodeToString(raw))
		}
		tanam("Montserrat.ttf", "Clipper Sans")
		tanam("Anton.ttf", "Clipper Display")
		tanam("BebasNeue.ttf", "Clipper Kondensa")
		b.fontCSS = template.CSS(sb.String())
	})
	return b.fontCSS
}

// zoomSah menjaga zoom di rentang masuk akal. Nilai 0 datang dari permintaan
// lama yang belum mengenal field ini, jadi diartikan sebagai "tanpa zoom".
//
// Batas bawahnya 1: di bawah itu foto lebih kecil dari bingkainya dan menyisakan
// celah kosong di tepi kartu.
func zoomSah(z float64) float64 {
	switch {
	case z <= 1:
		return 1
	case z > 4:
		return 4
	}
	return z
}

// batasGeser = sejauh mana foto boleh digeser tanpa memunculkan celah kosong.
// Pada zoom Z, foto jadi Z kali ukuran bingkai, jadi sisa ruangnya (Z-1)/2 di
// tiap sisi. Pada zoom 1 hasilnya 0 — foto pas bingkai, tak ada yang bisa
// digeser. GUI memakai rumus yang sama supaya seretannya berhenti di tempat
// yang sama dengan hasil render.
func batasGeser(ukuran int, zoom float64) int {
	return int(float64(ukuran) * (zoom - 1) / 2)
}

func jepit(v, batas int) int {
	if v > batas {
		return batas
	}
	if v < -batas {
		return -batas
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
var tmpl = template.Must(template.New("kartu").Parse(`<!doctype html>
<html lang="id"><head><meta charset="utf-8"><style>
{{.FontCSS}}
*{margin:0;padding:0;box-sizing:border-box}
/* Palet sengaja menghindari merah. Hampir semua media Indonesia memakai merah
   di logonya, jadi aksen merah pada kartu akan bertabrakan dengan lencana
   sumbernya sendiri. Kuning penanda dipilih karena berbeda dari warna media
   mana pun, dan karena artinya jelas: bagian ini ditandai. */
:root{
  --tinta:#14171C;        /* biru-kehitaman, seperti tinta cetak */
  --kertas:{{if .Gelap}}#EFEBE1{{else}}#FBF9F4{{end}};
  --tintaKertas:#1A1714;  /* hitam kehangatan, untuk teks di atas kertas */
  --sorot:#E4B429;        /* kuning penanda — satu-satunya aksen */
}
html,body{width:{{.Lebar}}px;height:{{.Tinggi}}px;overflow:hidden}
body{
  font-family:'Clipper Sans',"Segoe UI",Roboto,Arial,sans-serif;
  display:flex;flex-direction:column;
  {{if .Gelap}}background:var(--tinta);color:var(--kertas)
  {{else}}background:#DDD8CC;color:#1A1714{{end}}
}
.foto{position:relative;height:{{.TinggiFoto}}%;flex:none;overflow:hidden}
/* object-fit:cover dipakai—bukan min-width/min-height dengan width:auto—karena
   yang terakhir membuat penskalaan bergantung pada ukuran asli gambar relatif
   terhadap kotaknya. Akibatnya CSS yang sama memberi hasil berbeda di kotak
   1080px (render) dan kotak ±380px (pratinjau GUI). object-fit:cover selalu
   memenuhi bingkai dengan rasio terjaga, apa pun ukuran kotak dan gambarnya.

   Urutan translate lalu scale disengaja: jarak geser tidak ikut terkalikan zoom,
   sehingga menyeret N piksel di pratinjau = N piksel di hasil render. */
.foto img{
  position:absolute;left:50%;top:50%;
  width:100%;height:100%;object-fit:cover;display:block;
  transform:translate(-50%,-50%) translate({{.FotoX}}px,{{.FotoY}}px) scale({{.FotoZoom}});
  transform-origin:center;
}
.foto::after{content:"";position:absolute;inset:0;
  background:linear-gradient(180deg,rgba(0,0,0,.4) 0%,rgba(0,0,0,0) 26%,
    {{if .Gelap}}rgba(20,23,28,0) 58%,rgba(20,23,28,.9) 86%,var(--tinta) 100%
    {{else}}rgba(221,216,204,0) 58%,rgba(221,216,204,.92) 86%,#DDD8CC 100%{{end}})}
.badge{position:absolute;top:{{.Padding}}px;left:{{.Padding}}px;z-index:2;
  display:flex;align-items:center;gap:14px;flex-wrap:wrap}
/* Lencana sumber memakai warna kertas, bukan warna media. Kita tidak tahu
   warna merek mereka yang sebenarnya, dan menebaknya sama saja memalsukan
   identitas orang. Netral lebih jujur. */
.badge b{background:var(--kertas);color:var(--tintaKertas);
  font-family:'Clipper Kondensa','Clipper Sans',sans-serif;font-weight:400;
  font-size:{{.UkuranKecil}}px;letter-spacing:.16em;
  padding:10px 20px;border-radius:3px;text-transform:uppercase}
.badge span{background:rgba(0,0,0,.5);color:var(--kertas);font-size:{{.UkuranKecil}}px;
  font-family:'Clipper Kondensa','Clipper Sans',sans-serif;letter-spacing:.1em;
  padding:10px 18px;border-radius:4px}
/* Dengan foto: isi dijangkarkan ke bawah, sisa ruang jatuh ke bagian foto yang
   sudah memudar — tidak ada rongga menganga di bawah guntingan.
   Tanpa foto: dijangkarkan ke bawah justru menyisakan setengah kartu kosong di
   atas, jadi isinya ditaruh di tengah. */
.isi{flex:1;padding:0 {{.Padding}}px {{.Padding}}px;display:flex;flex-direction:column;
  position:relative;z-index:2;
  {{if .AdaGambar}}justify-content:flex-end;margin-top:-72px
  {{else}}justify-content:center;padding-top:{{.Padding}}px{{end}}}

/* Tanda mulai kutipan. Menandai di mana bahan orang lain mulai dikutip —
   bukan hiasan, dan itu sebabnya ia ikut berpindah mengikuti perataan. */
.tanda{width:64px;height:8px;background:var(--sorot);margin:{{.GarisM}};flex:none}

/* Judul artikel turun pangkat jadi keterangan: huruf kondensa, kapital,
   direnggangkan. Ia menjelaskan kutipan di bawahnya, bukan bersaing dengannya. */
.konteks{font-family:'Clipper Kondensa','Clipper Sans',sans-serif;
  font-size:{{.UkuranKonteks}}px;line-height:1.12;letter-spacing:.045em;
  text-transform:uppercase;text-align:{{.Rata}};margin-bottom:30px;
  /* Judul panjang dipangkas dua baris: perannya menjelaskan, bukan bersaing
     dengan kutipan. Tanpa batas ini judul 15 kata bisa mendorong guntingan
     keluar kartu. */
  display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;
  {{if .Gelap}}color:#9AA3AD{{else}}color:#5C5750{{end}}}

/* GUNTINGAN — inti desain ini. Kutipan disajikan sebagai potongan kertas yang
   ditempel, sebab yang dijual fitur ini adalah bukti: teksnya diambil apa
   adanya, bukan ditulis ulang. Tinta gelap di atas kertas terang juga pilihan
   paling terbaca di layar ponsel, dan keterbacaan itulah alasan mode kartu ada. */
.guntingan{
  background:var(--kertas);color:var(--tintaKertas);
  padding:{{.Padding}}px;border-radius:3px;
  transform:rotate(-.45deg);
  box-shadow:0 20px 44px rgba(0,0,0,{{if .Gelap}}.5{{else}}.22{{end}});
}
.hero{font-size:{{.UkuranHero}}px;line-height:1.34;font-weight:600;
  letter-spacing:-.005em;text-align:{{.Rata}};
  {{if .Kutipan}}font-family:'Clipper Display','Clipper Sans',Impact,sans-serif;
  font-weight:400;line-height:1.1;letter-spacing:0;{{end}}}

/* Stempel asal ditaruh DI ATAS guntingan, bukan di kaki kartu: pada selembar
   kutipan, dari mana ia berasal adalah bagian dari kutipannya sendiri. */
.stempel{margin-top:34px;font-family:'Clipper Kondensa','Clipper Sans',sans-serif;
  font-size:{{.UkuranKecil}}px;letter-spacing:.13em;text-transform:uppercase;
  text-align:{{.Rata}}}
.stempel span{background:var(--sorot);padding:5px 14px;border-radius:2px;
  color:#1A1714;box-decoration-break:clone;-webkit-box-decoration-break:clone}

.kaki{padding-top:28px;font-family:'Clipper Kondensa','Clipper Sans',sans-serif;
  font-size:{{.UkuranKecil}}px;letter-spacing:.12em;text-transform:uppercase;
  {{if .Gelap}}color:#79818B{{else}}color:#6B665E{{end}}}
</style></head><body>
{{if .AdaGambar}}
<div class="foto">
  <div class="badge"><b>{{.Sumber}}</b></div>
  <img src="{{.Gambar}}" alt="">
</div>
{{else}}
<div class="badge" style="position:static;margin:{{.Padding}}px {{.Padding}}px 0">
  <b>{{.Sumber}}</b>
</div>
{{end}}
<div class="isi">
  <div class="tanda"></div>
  {{if .Konteks}}<div class="konteks">{{.Konteks}}</div>{{end}}
  <div class="guntingan">
    <div class="hero">{{.Hero}}</div>
    <div class="stempel"><span>{{.Domain}}{{if .Tanggal}} &middot; {{.Tanggal}}{{end}}</span></div>
  </div>
  <div class="kaki">baca selengkapnya</div>
</div>
</body></html>`))
