package config

import "testing"

func TestReframeCek(t *testing.T) {
	if err := ReframeCenter.Cek(); err != nil {
		t.Errorf("center seharusnya tersedia: %v", err)
	}
	if err := ReframeFit.Cek(); err != nil {
		t.Errorf("fit seharusnya tersedia: %v", err)
	}
	if err := ReframeFaceFollow.Cek(); err == nil {
		t.Error("face_follow belum dibuat, seharusnya ditolak — bukan diam-diam dirender sebagai center")
	}
	if err := Reframe("ngawur").Cek(); err == nil {
		t.Error("mode tak dikenal seharusnya ditolak")
	}
}

// Validate ikut menolak mode yang belum tersedia, jadi job berhenti di depan
// dengan pesan jelas alih-alih menghasilkan klip yang tidak sesuai pilihan.
func TestValidateTolakFaceFollow(t *testing.T) {
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
