package card

import "testing"

func TestRataCSSMemetakanPilihanKeTextAlign(t *testing.T) {
	kasus := []struct{ rata, align string }{
		{RataKiri, "left"},
		{RataTengah, "center"},
		{RataKanan, "right"},
		{RataPenuh, "justify"},
		{"", "left"},       // belum diisi
		{"ngawur", "left"}, // nilai tak dikenal jangan bikin CSS rusak
	}
	for _, k := range kasus {
		got, _ := rataCSS(k.rata)
		if got != k.align {
			t.Errorf("rataCSS(%q) = %q, mau %q", k.rata, got, k.align)
		}
	}
}

// Garis aksen harus ikut berpindah; kalau tidak, ia menggantung di kiri
// sementara teksnya rata kanan atau tengah.
func TestRataCSSGarisIkutBerpindah(t *testing.T) {
	_, kiri := rataCSS(RataKiri)
	_, tengah := rataCSS(RataTengah)
	_, kanan := rataCSS(RataKanan)
	if kiri == tengah || tengah == kanan || kiri == kanan {
		t.Errorf("margin garis harus berbeda tiap perataan: kiri=%q tengah=%q kanan=%q",
			kiri, tengah, kanan)
	}
}

func TestZoomSahJagaBatas(t *testing.T) {
	kasus := []struct {
		in, mau float64
	}{
		{0, 1},   // permintaan lama tanpa field foto
		{-2, 1},  // nilai mustahil
		{0.4, 1}, // di bawah 1 akan menyisakan celah di tepi kartu
		{1.6, 1.6},
		{9, 4}, // batas atas
	}
	for _, k := range kasus {
		if got := zoomSah(k.in); got != k.mau {
			t.Errorf("zoomSah(%v) = %v, mau %v", k.in, got, k.mau)
		}
	}
}

// Batas geser harus sama dengan rumus yang dipakai GUI: (zoom-1)/2 × ukuran.
func TestBatasGeserIkutZoom(t *testing.T) {
	if got := batasGeser(1080, 1); got != 0 {
		t.Errorf("batasGeser pada zoom 1 = %d, mau 0 (foto pas bingkai)", got)
	}
	if got := batasGeser(1080, 2); got != 540 {
		t.Errorf("batasGeser(1080, 2) = %d, mau 540", got)
	}
}

func TestJepitTahanNilaiDiLuarBatas(t *testing.T) {
	if got := jepit(900, 540); got != 540 {
		t.Errorf("jepit(900,540) = %d, mau 540", got)
	}
	if got := jepit(-900, 540); got != -540 {
		t.Errorf("jepit(-900,540) = %d, mau -540", got)
	}
	if got := jepit(100, 540); got != 100 {
		t.Errorf("jepit(100,540) = %d, mau 100", got)
	}
}

func TestDimsPerRasio(t *testing.T) {
	kasus := []struct {
		rasio string
		w, h  int
	}{
		{Rasio916, 1080, 1920},
		{Rasio45, 1080, 1350},
		{Rasio11, 1080, 1080},
		{"", 1080, 1920}, // default
	}
	for _, k := range kasus {
		w, h := Dims(k.rasio)
		if w != k.w || h != k.h {
			t.Errorf("Dims(%q) = %dx%d, mau %dx%d", k.rasio, w, h, k.w, k.h)
		}
	}
}
