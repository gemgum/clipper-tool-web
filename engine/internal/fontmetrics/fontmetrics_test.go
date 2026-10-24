package fontmetrics

import (
	"os"
	"path/filepath"
	"testing"
)

// Angka pembandingnya dari render libass Montserrat Bold (notes/44):
// "WARGA MEMADAMKAN" 808 px pada Fontsize 88 = 9,18 satuan. Kerning diabaikan,
// jadi hasilnya boleh sedikit di atas, tidak boleh di bawah.
func TestMontserratBoldMatchesLibass(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "assets", "fonts")
	if _, err := os.Stat(filepath.Join(dir, "Montserrat-Bold.ttf")); err != nil {
		t.Skip("font belum diunduh (fetch-fonts.sh)")
	}
	f := ForFamily(dir, "Montserrat", true)
	if f == nil {
		t.Fatal("Montserrat-Bold tidak terbaca")
	}
	if w := f.Width("WARGA MEMADAMKAN"); w < 9.1 || w > 9.6 {
		t.Fatalf("lebar = %.2f, mau ±9,18", w)
	}
	if a, b := f.Width("iii"), f.Width("mmm"); a >= b {
		t.Fatalf("i (%.2f) seharusnya lebih sempit dari m (%.2f)", a, b)
	}
}
