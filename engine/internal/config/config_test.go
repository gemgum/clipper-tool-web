package config

import "testing"

func TestTranscriberValidation(t *testing.T) {
	o := DefaultOptions()
	if err := o.Validate(); err != nil || o.Transcriber != TranscriberWhisper {
		t.Fatalf("bawaan = %q, %v", o.Transcriber, err)
	}
	o = DefaultOptions()
	o.Transcriber = "deepgram"
	if err := o.Validate(); err == nil {
		t.Fatal("mesin transkripsi tak dikenal diterima")
	}
}

func TestReframeCheck(t *testing.T) {
	if err := ReframeCenter.Check(); err != nil {
		t.Errorf("center seharusnya tersedia: %v", err)
	}
	if err := ReframeFit.Check(); err != nil {
		t.Errorf("fit seharusnya tersedia: %v", err)
	}
	if err := Reframe("nonsense").Check(); err == nil {
		t.Error("mode tak dikenal seharusnya ditolak")
	}
}

// Validate ikut menolak mode yang tidak dikenal, jadi job berhenti di depan
// dengan pesan jelas alih-alih menghasilkan klip yang tidak sesuai pilihan.
func TestValidateRejectsAnUnknownReframe(t *testing.T) {
	o := DefaultOptions()
	o.Reframe = Reframe("nonsense")
	if err := o.Validate(); err == nil {
		t.Fatal("Validate seharusnya menolak mode reframe yang tidak dikenal")
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

// Ketiga mode adalah pilihan yang berdiri sendiri. "fit" BUKAN alias: ia punya
// perilakunya sendiri — seluruh gambar masuk tanpa crop — dan harus bertahan
// apa adanya lewat Validate, bersama nilai zoom yang menyertainya.
func TestFitIsAFirstClassMode(t *testing.T) {
	o := DefaultOptions()
	o.Reframe = ReframeFit
	o.Zoom = 20
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	if o.Reframe != ReframeFit {
		t.Errorf("reframe = %q, mau tetap %q — bukan diterjemahkan jadi mode lain", o.Reframe, ReframeFit)
	}
	if o.Zoom != 20 {
		t.Errorf("zoom = %d, mau tetap 20 — mode tidak boleh menimpa zoom", o.Zoom)
	}
}

// Default: potong tengah, memenuhi bingkai.
func TestDefaultIsCenterAtNaturalZoom(t *testing.T) {
	o := DefaultOptions()
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	if o.Reframe != ReframeCenter {
		t.Errorf("reframe default = %q, mau %q", o.Reframe, ReframeCenter)
	}
	if o.Zoom != ZoomCenterNatural {
		t.Errorf("zoom default = %d, mau %d", o.Zoom, ZoomCenterNatural)
	}
}

// Titik awal tiap mode berbeda, dan itulah yang membuat zoom 0 sah di Whole
// Picture tapi mustahil di Center of the Picture (kotak potongan tak boleh nol).
func TestNaturalZoomPerMode(t *testing.T) {
	if got := NaturalZoom(ReframeFit); got != 0 {
		t.Errorf("NaturalZoom(fit) = %d, mau 0 (seluruh video masuk)", got)
	}
	if got := NaturalZoom(ReframeCenter); got != 100 {
		t.Errorf("NaturalZoom(center) = %d, mau 100 (memenuhi bingkai)", got)
	}
}

// Whole Picture: 0 SAH dan bermakna — itu titik awalnya.
func TestWholePictureZoomBounds(t *testing.T) {
	cases := map[int]int{
		0:   0,   // titik awal: seluruh video masuk
		-40: 0,   // di bawah batas
		3:   0,   // dibulatkan ke bawah ke kelipatan 5
		52:  50,  //
		100: 100, // memenuhi bingkai — batas atasnya
		250: 100, // di atas batas
	}
	for in, want := range cases {
		o := DefaultOptions()
		o.Reframe = ReframeFit
		o.Zoom = in
		if err := o.Validate(); err != nil {
			t.Fatal(err)
		}
		if o.Zoom != want {
			t.Errorf("fit zoom %d → %d, mau %d", in, o.Zoom, want)
		}
	}
}

// Center of the Picture: 0 berarti "belum diisi" dan jatuh ke titik awalnya,
// sebab kotak potongan berukuran nol tidak punya arti.
func TestCenterZoomBounds(t *testing.T) {
	cases := map[int]int{
		0:   100, // belum diisi → titik awal
		-40: 100, // mustahil
		3:   5,   // di bawah batas bawah
		52:  50,  //
		100: 100, // titik awal, sekaligus batas atas
		250: 100, // di atas batas
	}
	for in, want := range cases {
		o := DefaultOptions()
		o.Reframe = ReframeCenter
		o.Zoom = in
		if err := o.Validate(); err != nil {
			t.Fatal(err)
		}
		if o.Zoom != want {
			t.Errorf("center zoom %d → %d, mau %d", in, o.Zoom, want)
		}
	}
}

func TestHeadlineBoxFree(t *testing.T) {
	b := Watermark{Image: "logo.png", X: 900, Y: 200, Width: 20, Height: 10}
	if cx, _, w, _ := b.HeadlineBox(); cx != 900 || w != 216 {
		t.Fatalf("boxed headline: cx=%d w=%d", cx, w)
	}
	b.Headline.Free = true
	if cx, cy, w, h := b.HeadlineBox(); cx != 540 || cy != 960 || w != 1080 || h != 1920 {
		t.Fatalf("free headline should use the whole frame, got %d,%d %dx%d", cx, cy, w, h)
	}
}
