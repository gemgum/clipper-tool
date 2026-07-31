package config

import "testing"

func TestReframeCheck(t *testing.T) {
	if err := ReframeCenter.Check(); err != nil {
		t.Errorf("center seharusnya tersedia: %v", err)
	}
	if err := ReframeFit.Check(); err != nil {
		t.Errorf("fit seharusnya tersedia: %v", err)
	}
	if err := ReframeFaceFollow.Check(); err == nil {
		t.Error("face_follow belum dibuat, seharusnya ditolak — bukan diam-diam dirender sebagai center")
	}
	if err := Reframe("nonsense").Check(); err == nil {
		t.Error("mode tak dikenal seharusnya ditolak")
	}
}

// Validate ikut menolak mode yang belum tersedia, jadi job berhenti di depan
// dengan pesan jelas alih-alih menghasilkan klip yang tidak sesuai pilihan.
func TestValidateRejectsFaceFollow(t *testing.T) {
	o := DefaultOptions()
	o.Reframe = ReframeFaceFollow
	if err := o.Validate(); err == nil {
		t.Fatal("Validate seharusnya menolak face_follow")
	}

	o = DefaultOptions()
	o.Reframe = ReframeFit
	if err := o.Validate(); err != nil {
		t.Fatalf("Validate seharusnya menerima fit: %v", err)
	}

	// Kosong = pakai default (center), tetap lolos.
	o = DefaultOptions()
	o.Reframe = ""
	if err := o.Validate(); err != nil {
		t.Fatalf("reframe kosong seharusnya jatuh ke default: %v", err)
	}
	if o.Reframe != ReframeCenter {
		t.Errorf("default reframe = %q, mau %q", o.Reframe, ReframeCenter)
	}
}

// "fit" dulu kendali terpisah yang menyatakan hal sama dengan zoom; sekarang ia
// hanyalah ujung atas FrameVisible. Alias lama harus tetap menghasilkan gambar
// yang sama, bukan ditolak.
func TestFitIsAliasForWholeFrame(t *testing.T) {
	o := DefaultOptions()
	o.Reframe = ReframeFit
	o.FrameVisible = 20 // sengaja bertabrakan: "fit" yang menang, sebab itu definisinya
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	if o.Reframe != ReframeCenter {
		t.Errorf("reframe = %q, mau diterjemahkan ke %q", o.Reframe, ReframeCenter)
	}
	if o.FrameVisible != FrameVisibleMax {
		t.Errorf("frame_visible = %d, mau %d (frame asli utuh)", o.FrameVisible, FrameVisibleMax)
	}
}

// Default harus tetap "isi penuh" — keluaran andalan yang sudah dipakai sejak
// awal. Kebetulan itu juga nilai nol Go, jadi permintaan yang tidak mengirim
// field ini tetap mendapat hasil yang sama.
func TestDefaultIsFullFrame(t *testing.T) {
	o := DefaultOptions()
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	if o.FrameVisible != 0 {
		t.Errorf("frame_visible default = %d, mau 0 (isi penuh)", o.FrameVisible)
	}
	if o.PictureSize != 100 {
		t.Errorf("picture_size default = %d, mau 100", o.PictureSize)
	}

	// Permintaan kosong (semua field nol) harus sampai ke tempat yang sama.
	var empty Options
	if err := empty.Validate(); err != nil {
		t.Fatal(err)
	}
	if empty.FrameVisible != 0 || empty.PictureSize != 100 {
		t.Errorf("permintaan kosong → frame_visible=%d picture_size=%d, mau 0 dan 100",
			empty.FrameVisible, empty.PictureSize)
	}
}

func TestFrameVisibleIsClampedAndSnapped(t *testing.T) {
	cases := map[int]int{-40: 0, 3: 0, 52: 50, 100: 100, 250: 100}
	for in, want := range cases {
		o := DefaultOptions()
		o.FrameVisible = in
		if err := o.Validate(); err != nil {
			t.Fatal(err)
		}
		if o.FrameVisible != want {
			t.Errorf("frame_visible %d → %d, mau %d", in, o.FrameVisible, want)
		}
	}
}

// PictureSize 0 tidak sah (gambar tanpa ukuran), jadi dipakai sebagai penanda
// "belum diisi" dan jatuh ke memenuhi bingkai — beda perlakuan dari FrameVisible.
func TestPictureSizeTreatsZeroAsUnset(t *testing.T) {
	cases := map[int]int{0: 100, -5: 100, 3: 5, 52: 50, 100: 100, 250: 100}
	for in, want := range cases {
		o := DefaultOptions()
		o.PictureSize = in
		if err := o.Validate(); err != nil {
			t.Fatal(err)
		}
		if o.PictureSize != want {
			t.Errorf("picture_size %d → %d, mau %d", in, o.PictureSize, want)
		}
	}
}
