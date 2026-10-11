package subtitle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/types"
)

func headlineWatermark() config.Watermark {
	b := config.DefaultWatermark()
	b.Image = "/tmp/banner.png"
	b.Width = 92
	b.Headline.Text = "RINZ KENA MENTAL?"
	return b
}

// Headline dipenggal memakai lebar KOTAKNYA — kotak gambar watermark, bukan
// lebar bingkai. Itu janji yang diminta: teks tidak boleh melewati gambar yang
// memuatnya.
func TestHeadlineLinesFollowBoxWidth(t *testing.T) {
	text := "BEGINI KATA MIDLANER ANDALAN RRQ HOSHI SOAL KEKALAHAN KEMARIN"
	wide := headlineLines(text, 40, 1080)
	narrow := headlineLines(text, 40, 400)
	if len(narrow) <= len(wide) {
		t.Fatalf("kotak sempit harus menghasilkan lebih banyak baris: %d vs %d", len(narrow), len(wide))
	}
	// (400 - 2*16) / (40 * 0,6) = 15 karakter.
	for _, ln := range narrow {
		if len(ln) > 15 {
			t.Fatalf("baris melewati lebar kotak: %q", ln)
		}
	}
}

// Blok teks DIJEPIT ke dalam kotaknya: geseran sebesar apa pun tidak bisa
// mengeluarkannya. Inilah "batas teks" yang dulu tidak ada.
func TestHeadlineAnchorStaysInsideBox(t *testing.T) {
	h := config.DefaultWatermark().Headline
	h.Size = 40
	lines := []string{"HALO"} // ~96 unit lebar, 40 tinggi
	// Kotak 400x300 berpusat di (540, 700); geseran 9999 harus terjepit.
	x, y := headlineAnchor(lines, config.Headline{Size: h.Size, DX: 9999, DY: 9999}, 540, 700, 400, 300)
	if x > 540+200 || y > 700+150 {
		t.Fatalf("jangkar keluar dari kotak: %d,%d", x, y)
	}
	// Tanpa geseran ia TEPAT di tengah kotak — itu bawaannya.
	if x0, y0 := headlineAnchor(lines, config.Headline{Size: h.Size}, 540, 700, 400, 300); x0 != 540 || y0 != 700 {
		t.Fatalf("tanpa geseran harus di tengah kotak, dapat %d,%d", x0, y0)
	}
}

// Baris yang diketik pengguna sendiri dihormati: kalau ia menekan enter, di
// situlah barisnya patah.
func TestHeadlineLinesKeepManualBreaks(t *testing.T) {
	got := headlineLines("RINZ\nKENA MENTAL", 64, 1080)
	if len(got) != 2 || got[0] != "RINZ" {
		t.Fatalf("pemenggalan manual hilang: %#v", got)
	}
}

// Headline ditulis sebagai style KEDUA di .ass yang sama, pada Layer 1 supaya ia
// berada di atas subtitle bila keduanya bertumpang tindih.
func TestWriteASSAddsHeadline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.ass")
	segs := []types.TranscriptSegment{{Start: 0, End: 3, Text: "halo semuanya"}}
	if err := WriteASS(path, segs, 0, config.DefaultSubtitle(), headlineWatermark(), "RINZ KENA MENTAL?", 30); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	s := string(out)
	if !strings.Contains(s, "Style: Headline,") {
		t.Fatalf("style headline tidak ditulis:\n%s", s)
	}
	if !strings.Contains(s, "Dialogue: 1,") || !strings.Contains(s, ",Headline,,") {
		t.Fatalf("baris headline tidak ditulis di Layer 1:\n%s", s)
	}
	// For = 0 → sampai klip habis (30 detik), bukan sampai segmen terakhir.
	if !strings.Contains(s, "0:00:30.00") {
		t.Fatalf("headline tidak bertahan sampai akhir klip:\n%s", s)
	}
}

// Tanpa teks, .ass harus sama seperti sebelum fitur ini ada — tidak ada style
// menganggur, tidak ada baris kosong.
func TestWriteASSWithoutHeadlineStaysPlain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.ass")
	segs := []types.TranscriptSegment{{Start: 0, End: 3, Text: "halo semuanya"}}
	if err := WriteASS(path, segs, 0, config.DefaultSubtitle(), config.DefaultWatermark(), "", 30); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if strings.Contains(string(out), "Headline") {
		t.Fatalf("headline muncul padahal teksnya kosong:\n%s", out)
	}
}

// Rentang waktu: At + For jadi ujung akhirnya.
func TestWriteASSHeadlineWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.ass")
	b := headlineWatermark()
	b.At, b.For = 2, 5
	if err := WriteASS(path, nil, 0, config.DefaultSubtitle(), b, "HALO", 30); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "Dialogue: 1,0:00:02.00,0:00:07.00,Headline") {
		t.Fatalf("jendela waktu headline salah:\n%s", out)
	}
}
