package subtitle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/fontmetrics"
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

// Kotak judul: Enter pengguna = baris baru, dan TIDAK ADA baris yang lebih
// lebar dari kotaknya — diukur dengan metrik font asli, termasuk kata tunggal
// yang lebih panjang dari kotak (dipenggal per huruf). notes/44.
func TestNewsBoxKeepsEnterAndStaysInside(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "assets", "fonts")
	if _, err := os.Stat(filepath.Join(dir, "Montserrat-Bold.ttf")); err != nil {
		t.Skip("font belum diunduh (fetch-fonts.sh)")
	}
	w := config.DefaultWatermark()
	w.Format = config.FormatFeed45
	w.Headline.Box, w.Headline.Size, w.Headline.FontsDir = true, 88, dir
	text := "Gambar orang sedang memiliki ide tapi tidak melakukan apapun\nINFORMASI " +
		strings.Repeat("m", 40)
	bx := NewsBox(w, text)

	f := fontmetrics.ForFamily(dir, "Montserrat", true)
	limit := float64(bx.W - 2*bx.Pad)
	enter := -1
	for i, ln := range bx.Lines {
		if px := f.Width(ln) * 88; px > limit {
			t.Fatalf("baris %q selebar %.0f px, kotaknya %.0f px", ln, px, limit)
		}
		if strings.HasPrefix(ln, "INFORMASI") {
			enter = i
		}
	}
	if enter <= 0 || !strings.HasSuffix(bx.Lines[enter-1], "apapun") {
		t.Fatalf("Enter tidak dihormati: %q", bx.Lines)
	}
}

// Kredit selalu terbaca: gelap di kotak terang, terang di kotak gelap.
func TestCreditColorFollowsBox(t *testing.T) {
	for box, want := range map[string]string{"": "&H222222&", "white": "&H222222&", "yellow": "&H222222&",
		"black": "&HEEEEEE&", "red": "&HEEEEEE&", "#1d4ed8": "&HEEEEEE&"} {
		if got := creditColor(box); got != want {
			t.Errorf("creditColor(%q) = %s, mau %s", box, got, want)
		}
	}
}

// Perataan kotak judul: jangkar \an7/8/9 di tepi kiri isi, tengah kotak, atau
// tepi kanan isi — judul DAN kreditnya.
func TestNewsBoxAlign(t *testing.T) {
	w := config.DefaultWatermark()
	w.Headline.Box, w.Headline.Credit = true, "Foto: X"
	for align, want := range map[string]string{
		config.AlignLeft: `\an7\pos(72,`, config.AlignCenter: `\an8\pos(540,`, config.AlignRight: `\an9\pos(1008,`,
	} {
		w.Headline.Align = align
		var b strings.Builder
		writeNewsBox(&b, w, "HALO", "0:00:00.00", "0:00:05.00")
		if strings.Count(b.String(), want) != 2 {
			t.Errorf("%s: mau 2x %q (judul + kredit) di\n%s", align, want, b.String())
		}
	}
}

// Justify: baris yang bukan terakhir di paragrafnya ditulis per kata, dan kata
// terakhirnya berakhir tepat di tepi kanan isi kotak. Baris terakhir tetap utuh.
func TestNewsBoxJustify(t *testing.T) {
	w := config.DefaultWatermark()
	w.Headline.Box = true
	w.Headline.Align = config.AlignJustify
	text := "BEGINI KATA MIDLANER ANDALAN RRQ HOSHI SOAL KEKALAHAN KEMARIN MALAM"
	bx := NewsBox(w, text)
	if len(bx.Lines) < 2 || !bx.Stretch[0] || bx.Stretch[len(bx.Lines)-1] {
		t.Fatalf("stretch salah: lines=%q stretch=%v", bx.Lines, bx.Stretch)
	}
	var b strings.Builder
	writeNewsBox(&b, w, text, "0:00:00.00", "0:00:05.00")
	out := b.String()
	first := strings.Fields(bx.Lines[0])
	if n := strings.Count(out, "}"+first[0]+"\n"); n != 1 {
		t.Fatalf("baris pertama harus ditulis per kata:\n%s", out)
	}
	if !strings.Contains(out, "}"+bx.Lines[len(bx.Lines)-1]+"\n") {
		t.Fatalf("baris terakhir harus utuh:\n%s", out)
	}
}

// Kartu berita: panel menutup bawah bingkai, judul di bawah batas gambar, dan
// kredit jadi lencana berwarna Badge dengan teks OnBadge.
func TestCardLook(t *testing.T) {
	w := config.DefaultWatermark()
	w.Headline.Box, w.Headline.Card = true, true
	w.Headline.BoxColor, w.Headline.Badge, w.Headline.OnBadge = "#0F2747", "#FFD400", "#15181D"
	w.Headline.Credit = "ANTARANEWS.COM"
	bx := CardBox(w, "Karantina Kepri musnahkan 29 ton bawang putih")
	if bx.Photo != 1920*46/100 || bx.Top <= bx.Photo || bx.Credit == 0 {
		t.Fatalf("tata letak salah: %+v", bx)
	}
	var b strings.Builder
	writeCard(&b, w, "Karantina Kepri musnahkan 29 ton bawang putih", "0:00:00.00", "0:00:05.00")
	out := b.String()
	for _, want := range []string{
		"\\1c&H0047270F&\\1a&H00&", // panel navy, pekat
		"\\1c&H0000D4FF&",          // garis aksen & lencana kuning
		"\\1c&H001D1815&}ANTARANEWS.COM",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("tidak ada %q di:\n%s", want, out)
		}
	}
}

// CardY menggeser BATAS gambar–panel; judul ikut menempel di bawahnya. Batas
// dijepit 20–85% bingkai, dan judul tidak pernah keluar dari dasar.
func TestCardYMovesPanel(t *testing.T) {
	w := config.DefaultWatermark()
	w.Headline.Box, w.Headline.Card = true, true
	base := CardBox(w, "Judul uji")
	w.Headline.CardY = 300
	if got := CardBox(w, "Judul uji"); got.Photo != base.Photo+300 || got.Top != base.Top+300 {
		t.Fatalf("CardY 300: photo %d top %d, harus %d %d", got.Photo, got.Top, base.Photo+300, base.Top+300)
	}
	w.Headline.CardY = 99999
	if got := CardBox(w, "Judul uji"); got.Photo != 1920*85/100 || got.Top+got.H > 1920-cardPad {
		t.Fatalf("jepitan bawah salah: %+v", got)
	}
	w.Headline.CardY = -99999
	if got := CardBox(w, "Judul uji"); got.Photo != 1920/5 {
		t.Fatalf("jepitan atas salah: %+v", got)
	}
}
