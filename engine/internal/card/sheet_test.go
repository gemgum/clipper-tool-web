package card

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// Lembar 3 halaman bertumpuk dipotong jadi PNG per halaman, tiap halaman
// tepat bagiannya sendiri (warna berbeda per halaman).
func TestSplitSheet(t *testing.T) {
	dir := t.TempDir()
	w, h := 4, 6
	colors := []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}}
	img := image.NewRGBA(image.Rect(0, 0, w, h*len(colors)))
	for i, c := range colors {
		for y := i * h; y < (i+1)*h; y++ {
			for x := 0; x < w; x++ {
				img.Set(x, y, c)
			}
		}
	}
	sheet := filepath.Join(dir, "sheet.png")
	f, _ := os.Create(sheet)
	png.Encode(f, img)
	f.Close()
	if err := splitSheet(sheet, dir, []string{PageFile(2), PageFile(3), PageFile(4)}, w, h); err != nil {
		t.Fatal(err)
	}
	for i, n := range []int{2, 3, 4} {
		f, err := os.Open(filepath.Join(dir, PageFile(n)))
		if err != nil {
			t.Fatal(err)
		}
		p, _ := png.Decode(f)
		f.Close()
		if p.Bounds().Dx() != w || p.Bounds().Dy() != h {
			t.Fatalf("halaman %d berukuran %v", n, p.Bounds())
		}
		r, g, b, _ := p.At(p.Bounds().Min.X+1, p.Bounds().Min.Y+h-1).RGBA()
		want := colors[i]
		if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B {
			t.Fatalf("halaman %d berwarna %d,%d,%d, harus %v", n, r>>8, g>>8, b>>8, want)
		}
	}
	// Lembar yang lebih pendek dari seharusnya = galat, bukan halaman terpotong.
	if err := splitSheet(sheet, dir, []string{PageFile(2), PageFile(3), PageFile(4), PageFile(5)}, w, h); err == nil {
		t.Fatal("lembar yang kurang tinggi harus ditolak")
	}
}
