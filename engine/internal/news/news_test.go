package news

import (
	"net/url"
	"strings"
	"testing"
)

// contoh feed meniru kebiasaan nyata media Indonesia: gambar diselipkan
// sebagai <img> di dalam description (ANTARA), judul channel menaruh nama
// media di belakang, dan ada entitas HTML di judul.
const contohRSS = `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0"><channel>
  <title>Berita Terkini - ANTARA News</title>
  <item>
    <title>Harga BBM naik &amp; warga mengeluh</title>
    <link>https://www.antaranews.com/berita/123/harga-bbm</link>
    <description><![CDATA[<img src="https://cdn.antaranews.com/foto.jpg" align="left"/>Pemerintah menaikkan harga.]]></description>
    <pubDate>Thu, 30 Jul 2026 19:51:57 +0700</pubDate>
  </item>
  <item>
    <title>Berita kedua</title>
    <link>https://www.antaranews.com/berita/124/kedua</link>
    <description>Tanpa gambar.</description>
    <pubDate>Thu, 30 Jul 2026 18:00:00 +0700</pubDate>
    <enclosure url="https://cdn.antaranews.com/enclosure.jpg"/>
  </item>
</channel></rss>`

func TestUraiFeedAmbilJudulGambarDanTanggal(t *testing.T) {
	art, err := uraiFeed([]byte(contohRSS), "ANTARA", 0)
	if err != nil {
		t.Fatalf("uraiFeed galat: %v", err)
	}
	if len(art) != 2 {
		t.Fatalf("jumlah artikel = %d, mau 2", len(art))
	}

	a := art[0]
	if a.Judul != "Harga BBM naik & warga mengeluh" {
		t.Errorf("judul = %q — entitas HTML seharusnya sudah diterjemahkan", a.Judul)
	}
	// Gambar harus diambil dari <img> di dalam description.
	if a.Gambar != "https://cdn.antaranews.com/foto.jpg" {
		t.Errorf("gambar = %q, mau dari <img> di description", a.Gambar)
	}
	// Tag HTML tidak boleh ikut terbawa ke ringkasan.
	if a.Ringkas != "Pemerintah menaikkan harga." {
		t.Errorf("ringkas = %q — tag <img> seharusnya dibuang", a.Ringkas)
	}
	if a.Tanggal != "Kamis, 30 Juli 2026" {
		t.Errorf("tanggal = %q, mau %q", a.Tanggal, "Kamis, 30 Juli 2026")
	}
	if a.Domain != "antaranews.com" {
		t.Errorf("domain = %q", a.Domain)
	}
	// <enclosure> dipakai bila description tidak memuat gambar.
	if art[1].Gambar != "https://cdn.antaranews.com/enclosure.jpg" {
		t.Errorf("gambar item ke-2 = %q, mau dari enclosure", art[1].Gambar)
	}
}

// Nama kurasi harus menang atas judul channel. Tanpa ini badge kartu ANTARA
// berbunyi "Berita Terkini", sebab nama medianya ada di belakang pemisah.
func TestUraiFeedNamaKurasiMenangAtasJudulChannel(t *testing.T) {
	art, err := uraiFeed([]byte(contohRSS), "ANTARA", 0)
	if err != nil {
		t.Fatal(err)
	}
	if art[0].Sumber != "ANTARA" {
		t.Errorf("sumber = %q, mau %q", art[0].Sumber, "ANTARA")
	}

	// Tanpa nama kurasi, tebakan dari judul channel dipakai apa adanya.
	art, err = uraiFeed([]byte(contohRSS), "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if art[0].Sumber == "" {
		t.Error("sumber kosong — seharusnya jatuh ke tebakan judul channel/domain")
	}
}

func TestUraiFeedHormatiBatasMaks(t *testing.T) {
	art, err := uraiFeed([]byte(contohRSS), "ANTARA", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(art) != 1 {
		t.Errorf("jumlah artikel = %d, mau dibatasi 1", len(art))
	}
}

func TestUraiFeedTolakIsiBukanFeed(t *testing.T) {
	if _, err := uraiFeed([]byte(`<html><body>bukan feed</body></html>`), "", 0); err == nil {
		t.Error("mau galat untuk isi yang bukan feed, dapat nil")
	}
}

func TestPotongBerhentiDiBatasKata(t *testing.T) {
	got := potong("satu dua tiga empat lima enam", 12)
	if got != "satu dua…" {
		t.Errorf("potong = %q, mau %q", got, "satu dua…")
	}
	// Teks pendek tidak boleh diubah.
	if got := potong("pendek", 50); got != "pendek" {
		t.Errorf("potong teks pendek = %q", got)
	}
}

func TestNamaMediaPakaiDomainBilaJudulTakMasukAkal(t *testing.T) {
	// Judul channel yang kepanjangan bukan nama media — pakai domain saja.
	panjang := "Portal Berita Terlengkap Dan Terpercaya Sepanjang Masa Sekali"
	if got := namaMedia(panjang, "https://www.contoh.co.id/a"); got != "contoh.co.id" {
		t.Errorf("namaMedia = %q, mau %q", got, "contoh.co.id")
	}
	// Pemisah "|" di depan: bagian pertama memang nama medianya.
	if got := namaMedia("CNN Indonesia | Berita Terkini", "https://cnnindonesia.com/a"); got != "CNN Indonesia" {
		t.Errorf("namaMedia = %q, mau %q", got, "CNN Indonesia")
	}
}

// bacaMeta adalah jalan masuk fitur "tempel link", jadi pola penulisan meta
// yang berbeda-beda antar situs harus tetap terbaca.
func TestBacaMetaBerbagaiUrutanAtribut(t *testing.T) {
	h := `<html><head>
	<meta property="og:title" content="Judul OG">
	<meta content="Ringkasan OG" name="og:description">
	<meta name='og:image' content='https://contoh.id/g.jpg'>
	<title>Judul Tag</title>
	</head><body><meta property="og:title" content="JANGAN DIPAKAI"></body></html>`

	m := bacaMeta(h)
	if m["og:title"] != "Judul OG" {
		t.Errorf("og:title = %q", m["og:title"])
	}
	// content sebelum name juga harus tertangkap.
	if m["og:description"] != "Ringkasan OG" {
		t.Errorf("og:description = %q", m["og:description"])
	}
	// Kutip satu sama sahnya dengan kutip dua.
	if m["og:image"] != "https://contoh.id/g.jpg" {
		t.Errorf("og:image = %q", m["og:image"])
	}
	if got := tagTitle(h); got != "Judul Tag" {
		t.Errorf("tagTitle = %q", got)
	}
}

// --- ekstraksi paragraf ---

const contohArtikel = `<html><body>
<nav><p>Beranda Ekonomi Olahraga Teknologi Nasional Internasional</p></nav>
<script>var iklan = "Pasang iklan di sini sekarang juga hubungi kami";</script>
<div class="isi">
  <p>Menteri Keuangan menyatakan penerimaan pajak tahun ini tumbuh 12 persen dibandingkan tahun lalu.</p>
  <p>Baca juga: Sepuluh cara menghemat uang belanja bulanan agar dompet tetap aman</p>
  <p>Singkat</p>
  <p>Angka itu disebut sebagai capaian tertinggi dalam lima tahun terakhir menurut laporan resmi kementerian.</p>
  <p>Menteri Keuangan menyatakan penerimaan pajak tahun ini tumbuh 12 persen dibandingkan tahun lalu.</p>
</div>
<footer><p>Copyright 2026 Redaksi Pedoman Media Siber Karier Kontak Kami Semua</p></footer>
</body></html>`

func TestUraiParagrafBuangMenuSkripDanDuplikat(t *testing.T) {
	par := uraiParagraf(contohArtikel)
	if len(par) != 2 {
		for _, p := range par {
			t.Logf("[%d] %s", p.Indeks, p.Teks)
		}
		t.Fatalf("jumlah paragraf = %d, mau 2", len(par))
	}
	if !strings.HasPrefix(par[0].Teks, "Menteri Keuangan menyatakan") {
		t.Errorf("paragraf pertama = %q", par[0].Teks)
	}
	// Nomor harus berurutan mulai 0 — inilah yang dipakai LLM untuk menunjuk.
	for i, p := range par {
		if p.Indeks != i {
			t.Errorf("indeks paragraf ke-%d = %d", i, p.Indeks)
		}
	}
	for _, p := range par {
		low := strings.ToLower(p.Teks)
		if strings.Contains(low, "baca juga") || strings.Contains(low, "beranda") ||
			strings.Contains(low, "iklan") || strings.Contains(low, "copyright") {
			t.Errorf("blok sampah ikut terbawa: %q", p.Teks)
		}
	}
}

func TestUraiParagrafTolakHalamanTanpaIsi(t *testing.T) {
	if par := uraiParagraf(`<html><body><p>Terlalu pendek</p></body></html>`); len(par) != 0 {
		t.Errorf("mau 0 paragraf, dapat %d", len(par))
	}
}

// --- tagar ---

func TestTagarHanyaTerimaKataYangAdaDiArtikel(t *testing.T) {
	isi := Isi{Paragraf: []Paragraf{{0, "Bank Indonesia menahan suku bunga acuan di Jakarta pekan ini."}}}
	got := isi.Tagar([]string{"Bank Indonesia", "Jakarta", "Bitcoin melambung"}, 8)
	mau := []string{"#BankIndonesia", "#Jakarta"}
	if len(got) != len(mau) {
		t.Fatalf("tagar = %v, mau %v", got, mau)
	}
	for i := range mau {
		if got[i] != mau[i] {
			t.Errorf("tagar[%d] = %q, mau %q", i, got[i], mau[i])
		}
	}
}

func TestTagarHormatiBatasJumlah(t *testing.T) {
	isi := Isi{Paragraf: []Paragraf{{0, "satu dua tiga empat lima"}}}
	if got := isi.Tagar([]string{"satu", "dua", "tiga", "empat"}, 2); len(got) != 2 {
		t.Errorf("tagar = %v, mau dibatasi 2", got)
	}
}

// --- susun: pagar pengaman terhadap balasan LLM ---

func isiUji() Isi {
	return Isi{
		Artikel: Artikel{Judul: "Uji", URL: "https://contoh.id/a"},
		Paragraf: []Paragraf{
			{0, "Paragraf nol berisi kalimat pembuka berita yang cukup panjang."},
			{1, "Paragraf satu berisi angka mengejutkan sebesar dua belas persen."},
		},
	}
}

func TestSusunAmbilTeksDariArtikelBukanDariLLM(t *testing.T) {
	b := balasan{Kartu: 1, Caption: 0}
	b.Peringkat = append(b.Peringkat, struct {
		Indeks int     `json:"indeks"`
		Skor   float64 `json:"skor"`
		Alasan string  `json:"alasan"`
	}{Indeks: 1, Skor: 9, Alasan: "ada angka"})

	p := susun(isiUji(), b, "uji")
	if p.Peringkat[0].Teks != "Paragraf satu berisi angka mengejutkan sebesar dua belas persen." {
		t.Errorf("teks = %q — harus diambil dari artikel", p.Peringkat[0].Teks)
	}
	if p.Kartu != 1 || p.Caption != 0 {
		t.Errorf("kartu=%d caption=%d", p.Kartu, p.Caption)
	}
}

func TestSusunAbaikanNomorDiLuarJangkauan(t *testing.T) {
	b := balasan{Kartu: 99, Caption: 99}
	for _, ix := range []int{0, 42} { // 42 tidak ada
		b.Peringkat = append(b.Peringkat, struct {
			Indeks int     `json:"indeks"`
			Skor   float64 `json:"skor"`
			Alasan string  `json:"alasan"`
		}{Indeks: ix, Skor: 5})
	}
	p := susun(isiUji(), b, "uji")
	// Nomor 42 dibuang, tapi kedua paragraf artikel tetap muncul: yang tak
	// dinilai LLM dilengkapi heuristik supaya semuanya bisa dipilih pengguna.
	if len(p.Peringkat) != 2 {
		t.Fatalf("peringkat = %d, mau 2 (semua paragraf artikel)", len(p.Peringkat))
	}
	for _, r := range p.Peringkat {
		if r.Indeks == 42 {
			t.Error("nomor 42 tidak ada di artikel — seharusnya dibuang")
		}
	}
	// Kartu/caption di luar jangkauan jatuh ke peringkat teratas, bukan menebak.
	// Caption lalu digeser agar tidak kembar dengan kartu.
	if p.Kartu != 0 {
		t.Errorf("kartu = %d, mau peringkat teratas (0)", p.Kartu)
	}
	if _, ok := isiUji().Teks(p.Caption); !ok || p.Caption == p.Kartu {
		t.Errorf("caption = %d — harus paragraf sah yang berbeda dari kartu", p.Caption)
	}
}

// Model lokal kerap membalas "peringkat": [] padahal kartu/caption-nya sah.
// Dulu ini ditolak mentah-mentah; sekarang harus tetap menghasilkan pilihan.
func TestSusunTetapJalanSaatPeringkatKosong(t *testing.T) {
	p := susun(isiUji(), balasan{Kartu: 1, Caption: 0}, "uji")
	if len(p.Peringkat) != 2 {
		t.Fatalf("peringkat = %d, mau 2 dari heuristik", len(p.Peringkat))
	}
	for _, r := range p.Peringkat {
		if r.Sumber != SumberHeuristik {
			t.Errorf("sumber = %q, mau %q", r.Sumber, SumberHeuristik)
		}
		if r.Teks == "" {
			t.Error("teks kosong — harus diambil dari artikel")
		}
	}
	if p.Kartu != 1 || p.Caption != 0 {
		t.Errorf("kartu=%d caption=%d — nomor sah dari LLM harus dihormati", p.Kartu, p.Caption)
	}
	if p.Catatan == "" {
		t.Error("catatan kosong — penggantian tidak boleh senyap")
	}
}

// Nomor kartu/caption ngawur + peringkat kosong: tetap harus memberi hasil,
// jatuh ke paragraf berperingkat teratas.
func TestSusunTidakPernahGagalWalauBalasanNgawur(t *testing.T) {
	p := susun(isiUji(), balasan{Kartu: 99, Caption: -3}, "uji")
	if len(p.Peringkat) == 0 {
		t.Fatal("peringkat kosong — fitur tidak boleh gagal total")
	}
	if _, ok := isiUji().Teks(p.Kartu); !ok {
		t.Errorf("kartu = %d, bukan nomor paragraf yang ada", p.Kartu)
	}
	if _, ok := isiUji().Teks(p.Caption); !ok {
		t.Errorf("caption = %d, bukan nomor paragraf yang ada", p.Caption)
	}
}

// Paragraf berkutipan langsung & berangka harus menang atas paragraf
// penyambung yang diawali kata rujukan.
func TestSkorHookUtamakanKutipanDanAngka(t *testing.T) {
	kuat := Paragraf{0, `"Kerugian negara mencapai 1,2 triliun rupiah," kata Ketua KPK dalam konferensi pers di Jakarta hari ini.`}
	lemah := Paragraf{5, "Sementara itu, ia menambahkan bahwa proses masih terus berjalan sebagaimana mestinya di lapangan."}
	sKuat, _ := skorHook(kuat, 10)
	sLemah, _ := skorHook(lemah, 10)
	if sKuat <= sLemah {
		t.Errorf("skor kutipan+angka (%.1f) harus di atas paragraf penyambung (%.1f)", sKuat, sLemah)
	}
}

// Model lokal cenderung menjawab paragraf 0 untuk kartu sekaligus caption.
// Kalau dibiarkan, teks kartu dan captionnya kembar.
func TestSusunBedakanCaptionDariKartu(t *testing.T) {
	p := susun(isiUji(), balasan{Kartu: 0, Caption: 0}, "uji")
	if p.Kartu != 0 {
		t.Errorf("kartu = %d, mau 0 (pilihan LLM dihormati)", p.Kartu)
	}
	if p.Caption == p.Kartu {
		t.Error("caption kembar dengan kartu — seharusnya digeser ke peringkat berikutnya")
	}
	if _, ok := isiUji().Teks(p.Caption); !ok {
		t.Errorf("caption = %d, bukan paragraf yang ada di artikel", p.Caption)
	}
}

// Artikel satu paragraf tidak punya pilihan lain — kembar diterima apa adanya.
func TestSusunSatuParagrafBolehKembar(t *testing.T) {
	isi := Isi{Paragraf: []Paragraf{{0, "Satu-satunya paragraf yang ada di artikel pendek ini."}}}
	p := susun(isi, balasan{Kartu: 0, Caption: 0}, "uji")
	if p.Kartu != 0 || p.Caption != 0 {
		t.Errorf("kartu=%d caption=%d, mau 0 keduanya", p.Kartu, p.Caption)
	}
}

// Sebagian situs menulis og:url dengan entitas HTML menempel di ujungnya.
// Diteruskan mentah, alamat hasilnya membawa sampah itu dan berujung 404.
func TestUraiArtikelBersihkanEntitasDiOgUrl(t *testing.T) {
	h := `<html><head>
	<meta property="og:title" content="Cegah Narkoba dari Keluarga">
	<meta property="og:url" content="https://contoh.go.id/Cegah_Narkoba&nbsp;&nbsp;">
	<meta property="og:image" content="https://contoh.go.id/foto.jpg&nbsp;">
	</head><body></body></html>`
	u, _ := url.Parse("https://news.google.com/rss/articles/CBMiabc")
	a, err := uraiArtikel(h, u)
	if err != nil {
		t.Fatal(err)
	}
	if a.URL != "https://contoh.go.id/Cegah_Narkoba" {
		t.Errorf("url = %q — entitas di ujung harus dibuang", a.URL)
	}
	if a.Gambar != "https://contoh.go.id/foto.jpg" {
		t.Errorf("gambar = %q — entitas di ujung harus dibuang", a.Gambar)
	}
	// og:url menang atas alamat pengalih yang dipakai untuk membuka halaman.
	if a.Domain != "contoh.go.id" {
		t.Errorf("domain = %q, mau contoh.go.id", a.Domain)
	}
}
