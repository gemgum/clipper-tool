package card

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
)

// Warna kartu diturunkan dari FOTO artikelnya, bukan dari warna situs asalnya.
//
// Warna situs terdengar lebih masuk akal, tapi tidak ada: dari lima media besar
// Indonesia yang diperiksa (detik, kumparan, CNN Indonesia, Kompas, Tempo) hanya
// kumparan yang menulis <meta name="theme-color">. Empat dari lima kartu akan
// jatuh ke warna bawaan — persis keseragaman yang mau dihilangkan. Fotonya
// selalu ada, dan warnanya memang milik berita itu sendiri.
//
// Yang diambil hanya RONA dan KEPEKATANNYA, tidak pernah terangnya. Terang
// ditetapkan sendiri oleh palet supaya latar selalu cukup gelap dan kertas
// selalu cukup terang — foto malam dan foto siang harus sama terbacanya.

// tone = rona khas sebuah foto. Dipisah dari palette karena satu foto bisa
// dipakai di gaya gelap maupun terang, sedangkan tone-nya sama.
type tone struct {
	hue float64 // 0..360
	sat float64 // 0..1
	ok  bool    // false = foto tidak berwarna (hitam putih / kelabu)
}

// palette = warna yang benar-benar ditulis ke CSS.
type palette struct {
	Ink        string // latar kartu gaya gelap
	InkRGB     string // "20,23,28" — gradasi butuh komponennya, bukan hex
	Paper      string // guntingan kertas
	LightBg    string // latar kartu gaya terang
	LightBgRGB string
	Muted      string // judul-keterangan di atas latar
	Faint      string // kaki kartu
}

// Palet bawaan = warna kartu sebelum fitur ini ada. Dipakai apa adanya saat
// fotonya tidak berwarna atau tidak bisa diambil, jadi kartu tanpa rona yang
// jelas tetap terlihat seperti kartu yang memang dirancang begitu.
var (
	defaultDark = palette{
		Ink:        "#14171C",
		InkRGB:     "20,23,28",
		Paper:      "#EFEBE1",
		LightBg:    "#DDD8CC",
		LightBgRGB: "221,216,204",
		Muted:      "#9AA3AD",
		Faint:      "#79818B",
	}
	defaultLight = palette{
		Ink:        "#14171C",
		InkRGB:     "20,23,28",
		Paper:      "#FBF9F4",
		LightBg:    "#DDD8CC",
		LightBgRGB: "221,216,204",
		Muted:      "#5C5750",
		Faint:      "#6B665E",
	}
)

// paletteFor menurunkan seluruh warna kartu dari satu rona.
//
// Kepekatan dijepit di kedua ujung dengan alasan berbeda. Batas bawah supaya
// foto yang nyaris kelabu tidak menghasilkan kartu yang terlihat "rusak abu-abu"
// alih-alih berwarna; batas atas supaya foto dengan spanduk merah menyala tidak
// menghasilkan latar yang berteriak lebih keras dari isinya.
func paletteFor(t tone, dark bool) palette {
	if !t.ok {
		if dark {
			return defaultDark
		}
		return defaultLight
	}
	h := t.hue
	// Terang dikunci di angka palet bawaan (latar 9,4%, kertas 91%), jadi yang
	// berpindah dari kartu ke kartu hanya ronanya.
	p := palette{
		Ink:        hex(h, between(t.sat, 0.12, 0.32), 0.094),
		Paper:      hex(h, between(t.sat*0.35, 0.08, 0.20), 0.910),
		LightBg:    hex(h, between(t.sat*0.30, 0.06, 0.18), 0.830),
		Muted:      hex(h, 0.10, 0.640),
		Faint:      hex(h, 0.08, 0.510),
	}
	if !dark {
		// Di gaya terang kertas duduk di atas latar terang, jadi keduanya harus
		// lebih pucat; teks pendukung justru harus lebih gelap agar terbaca.
		p.Paper = hex(h, between(t.sat*0.25, 0.05, 0.14), 0.965)
		p.Muted = hex(h, 0.07, 0.340)
		p.Faint = hex(h, 0.06, 0.400)
	}
	p.InkRGB = rgbList(h, between(t.sat, 0.12, 0.32), 0.094)
	p.LightBgRGB = rgbList(h, between(t.sat*0.30, 0.06, 0.18), 0.830)
	return p
}

// toneOf membaca rona khas sebuah foto.
//
// Piksel yang paling banyak di sebuah foto biasanya justru yang tidak punya
// rona: bayangan, sorot lampu, langit putih. Karena itu piksel yang terlalu
// gelap, terlalu terang, atau terlalu pucat dibuang dulu — kalau tidak, hampir
// semua foto akan "dominan abu-abu" dan hasilnya seragam lagi.
//
// Sisanya dikelompokkan per rona, dan tiap piksel menyumbang sebesar
// kepekatannya sendiri: satu jaket merah menyala lebih menentukan kesan sebuah
// foto daripada seluas apa pun dinding krem di belakangnya.
func toneOf(img image.Image) tone {
	const buckets = 24 // 15 derajat per kelompok
	var (
		sumSin, sumCos [buckets]float64
		sumSat         [buckets]float64 // sekaligus bobot kelompok: piksel pekat menyumbang lebih besar
		count          [buckets]int
		sampled, kept  int
	)

	b := img.Bounds()
	// Foto CDN bisa 2000 px lebih. Dicicip di kisi ±120x120 supaya biayanya tetap
	// sama untuk foto sekecil apa pun besarnya — rona tidak butuh tiap piksel.
	stepX := max(1, b.Dx()/120)
	stepY := max(1, b.Dy()/120)

	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			r16, g16, b16, a16 := img.At(x, y).RGBA()
			sampled++
			if a16 < 0x8000 { // piksel tembus pandang tidak ikut terlihat
				continue
			}
			h, s, l := toHSL(float64(r16)/65535, float64(g16)/65535, float64(b16)/65535)
			if l < 0.12 || l > 0.92 || s < 0.18 {
				continue
			}
			kept++
			i := int(h/360*buckets) % buckets
			rad := h * math.Pi / 180
			sumSin[i] += s * math.Sin(rad)
			sumCos[i] += s * math.Cos(rad)
			sumSat[i] += s
			count[i]++
		}
	}

	// Foto hitam putih, dokumen, atau grafik kelabu: tidak ada rona yang jujur
	// bisa diambil. Ambangnya 2% supaya satu logo kecil berwarna di pojok tidak
	// menentukan warna seluruh kartu.
	if sampled == 0 || kept*50 < sampled {
		return tone{}
	}

	// Satu keluarga warna jarang jatuh rapi dalam satu kelompok: rona hangat
	// sebuah ruangan bisa tersebar di 0-45 derajat, lalu kalah oleh satu bilah
	// sempit yang pekat. Karena itu tiap kelompok dinilai bersama tetangganya —
	// yang dicari adalah keluarga warna, bukan potongan 15 derajat.
	score := func(i int) float64 {
		left, right := (i+buckets-1)%buckets, (i+1)%buckets
		return sumSat[i] + (sumSat[left]+sumSat[right])/2
	}
	best := 0
	for i := 1; i < buckets; i++ {
		if score(i) > score(best) {
			best = i
		}
	}
	if count[best] == 0 {
		return tone{}
	}
	// Rata-rata melingkar: rona itu sudut, jadi 350 dan 10 harus bertemu di 0,
	// bukan di 180.
	h := math.Atan2(sumSin[best], sumCos[best]) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return tone{hue: h, sat: between(sumSat[best]/float64(count[best]), 0, 1), ok: true}
}

// fetchImage mengunduh foto artikel untuk dibaca ronanya.
//
// Ini pengunduhan KEDUA foto yang sama — Chrome mengunduhnya sendiri saat
// merender. Disengaja: menyuntikkan foto sebagai data URI akan membuat halaman
// kartu membengkak dan menyulitkan penelusuran saat rendernya bermasalah,
// sementara ongkos unduhan kedua ini kecil dan hasilnya di-cache per alamat.
func fetchImage(ctx context.Context, url string) (image.Image, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// CDN yang melayani "format otomatis" (mis. Cloudinary f_auto) mengirim WebP
	// bila diminta apa saja. Go tidak bisa membaca WebP tanpa dependency luar,
	// jadi formatnya diminta terang-terangan.
	req.Header.Set("Accept", "image/jpeg,image/png;q=0.9,*/*;q=0.5")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; clipper/1.0)")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("photo returned HTTP %d", res.StatusCode)
	}
	img, _, err := image.Decode(io.LimitReader(res.Body, 12<<20))
	return img, err
}

// tone mengambil rona foto artikel, dengan hasil disimpan per alamat.
//
// Kegagalan TIDAK dianggap error: kartu tetap terbit dengan palet bawaan. Foto
// yang tidak terbaca ronanya bukan alasan untuk menggagalkan kartu — Chrome
// masih bisa menampilkan fotonya, dan hanya warna latarnya yang jadi bawaan.
func (b *Builder) tone(ctx context.Context, url string) tone {
	if url == "" {
		return tone{}
	}
	b.mu.Lock()
	if t, seen := b.tones[url]; seen {
		b.mu.Unlock()
		return t
	}
	b.mu.Unlock()

	var t tone
	if img, err := fetchImage(ctx, url); err == nil {
		t = toneOf(img)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	// Batas kasar supaya server yang hidup berhari-hari tidak menumpuk alamat
	// tanpa henti. Dibuang seluruhnya, bukan satu per satu: daftar ini murah
	// diisi ulang, dan LRU di sini menambah rumit tanpa manfaat.
	if len(b.tones) > 64 {
		b.tones = nil
	}
	if b.tones == nil {
		b.tones = make(map[string]tone)
	}
	b.tones[url] = t
	return t
}

// --- konversi warna ---

// toHSL memisahkan rona, kepekatan, dan terang dari sebuah warna RGB (0..1).
func toHSL(r, g, bl float64) (h, s, l float64) {
	maxc := math.Max(r, math.Max(g, bl))
	minc := math.Min(r, math.Min(g, bl))
	l = (maxc + minc) / 2
	d := maxc - minc
	if d == 0 {
		return 0, 0, l // kelabu murni: ronanya tidak ada, bukan nol
	}
	if l > 0.5 {
		s = d / (2 - maxc - minc)
	} else {
		s = d / (maxc + minc)
	}
	switch maxc {
	case r:
		h = math.Mod((g-bl)/d, 6)
	case g:
		h = (bl-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, s, l
}

// toRGB mengembalikan komponen 0..255 dari sebuah warna HSL.
func toRGB(h, s, l float64) (int, int, int) {
	if s <= 0 {
		v := round(l * 255)
		return v, v, v
	}
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return round((r + m) * 255), round((g + m) * 255), round((b + m) * 255)
}

func hex(h, s, l float64) string {
	r, g, b := toRGB(h, s, l)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func rgbList(h, s, l float64) string {
	r, g, b := toRGB(h, s, l)
	return fmt.Sprintf("%d,%d,%d", r, g, b)
}

func between(v, lo, hi float64) float64 {
	return math.Min(hi, math.Max(lo, v))
}

func round(v float64) int {
	return int(math.Round(math.Min(255, math.Max(0, v))))
}
