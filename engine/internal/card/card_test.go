package card

import "testing"

func TestAlignCSSMapsChoiceToTextAlign(t *testing.T) {
	cases := []struct{ align, css string }{
		{AlignLeft, "left"},
		{AlignCenter, "center"},
		{AlignRight, "right"},
		{AlignJustify, "justify"},
		{"", "left"},         // belum diisi
		{"nonsense", "left"}, // nilai tak dikenal jangan bikin CSS rusak
	}
	for _, c := range cases {
		got, _ := alignCSS(c.align)
		if got != c.css {
			t.Errorf("alignCSS(%q) = %q, mau %q", c.align, got, c.css)
		}
	}
}

// Garis aksen harus ikut berpindah; kalau tidak, ia menggantung di kiri
// sementara teksnya rata kanan atau tengah.
func TestAlignCSSRuleFollowsAlignment(t *testing.T) {
	_, left := alignCSS(AlignLeft)
	_, center := alignCSS(AlignCenter)
	_, right := alignCSS(AlignRight)
	if left == center || center == right || left == right {
		t.Errorf("margin garis harus berbeda tiap perataan: left=%q center=%q right=%q",
			left, center, right)
	}
}

func TestClampZoomKeepsBounds(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0, 1},   // permintaan lama tanpa field foto
		{-2, 1},  // nilai mustahil
		{0.4, 1}, // di bawah 1 akan menyisakan celah di tepi kartu
		{1.6, 1.6},
		{9, 4}, // batas atas
	}
	for _, c := range cases {
		if got := clampZoom(c.in); got != c.want {
			t.Errorf("clampZoom(%v) = %v, mau %v", c.in, got, c.want)
		}
	}
}

// Batas geser harus sama dengan rumus yang dipakai GUI: (zoom-1)/2 × ukuran.
func TestOffsetLimitFollowsZoom(t *testing.T) {
	if got := offsetLimit(1080, 1); got != 0 {
		t.Errorf("offsetLimit pada zoom 1 = %d, mau 0 (foto pas bingkai)", got)
	}
	if got := offsetLimit(1080, 2); got != 540 {
		t.Errorf("offsetLimit(1080, 2) = %d, mau 540", got)
	}
}

func TestClampHoldsValuesInsideLimit(t *testing.T) {
	if got := clamp(900, 540); got != 540 {
		t.Errorf("clamp(900,540) = %d, mau 540", got)
	}
	if got := clamp(-900, 540); got != -540 {
		t.Errorf("clamp(-900,540) = %d, mau -540", got)
	}
	if got := clamp(100, 540); got != 100 {
		t.Errorf("clamp(100,540) = %d, mau 100", got)
	}
}

func TestDimsPerRatio(t *testing.T) {
	cases := []struct {
		ratio string
		w, h  int
	}{
		{Ratio916, 1080, 1920},
		{Ratio45, 1080, 1350},
		{Ratio11, 1080, 1080},
		{"", 1080, 1920}, // default
	}
	for _, c := range cases {
		w, h := Dims(c.ratio)
		if w != c.w || h != c.h {
			t.Errorf("Dims(%q) = %dx%d, mau %dx%d", c.ratio, w, h, c.w, c.h)
		}
	}
}

// Teks tetap pada kartu ikut bahasa yang diminta; bahasa tak dikenal jatuh ke
// bahasa Inggris, bukan string kosong.
func TestPhrasesFollowLanguage(t *testing.T) {
	if got := phrasesFor("id").readMore; got != "baca selengkapnya" {
		t.Errorf("readMore(id) = %q", got)
	}
	if got := phrasesFor("en").readMore; got != "read the full story" {
		t.Errorf("readMore(en) = %q", got)
	}
	if got := phrasesFor("zz").readMore; got != "read the full story" {
		t.Errorf("bahasa tak dikenal seharusnya jatuh ke Inggris, dapat %q", got)
	}
	if got := langAttr("zz"); got != "en" {
		t.Errorf("langAttr(zz) = %q, mau en", got)
	}
}
