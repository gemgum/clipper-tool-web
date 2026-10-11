package card

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"testing"
)

// relLum = luminansi relatif menurut WCAG. Dipakai untuk mengukur kontras
// sungguhan, bukan menebak dari angka "terang" HSL — dua warna dengan terang
// sama bisa jauh berbeda keterbacaannya tergantung ronanya.
func relLum(hexStr string) float64 {
	n, err := strconv.ParseUint(strings.TrimPrefix(hexStr, "#"), 16, 32)
	if err != nil {
		panic(hexStr + ": " + err.Error())
	}
	r, g, b := int(n>>16)&0xFF, int(n>>8)&0xFF, int(n)&0xFF
	f := func(v int) float64 {
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*f(r) + 0.7152*f(g) + 0.0722*f(b)
}

func contrast(a, b string) float64 {
	la, lb := relLum(a), relLum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// Foto tanpa rona harus menghasilkan kartu yang persis seperti sebelum fitur ini
// ada. Kalau angka-angka ini bergeser, kartu lama tidak lagi bisa diproduksi
// ulang — dan foto hitam putih akan terlihat seperti kartu yang rusak, bukan
// kartu yang memang dirancang begitu.
func TestPhotoWithoutHueKeepsTheOriginalPalette(t *testing.T) {
	dark := paletteFor(tone{}, true)
	if dark.Ink != "#14171C" || dark.Paper != "#EFEBE1" || dark.Muted != "#9AA3AD" || dark.Faint != "#79818B" {
		t.Errorf("palet gelap bawaan berubah: %+v", dark)
	}
	light := paletteFor(tone{}, false)
	if light.LightBg != "#DDD8CC" || light.Paper != "#FBF9F4" {
		t.Errorf("palet terang bawaan berubah: %+v", light)
	}
	// Gradasi bawah foto memakai komponen RGB, dan harus menyebut warna yang
	// sama dengan latarnya — kalau meleset, ada garis warna di batas foto.
	if dark.Accent != "#E4B429" || light.Accent != "#E4B429" {
		t.Errorf("penanda tanpa rona berubah: %q / %q", dark.Accent, light.Accent)
	}
	if dark.InkRGB != "20,23,28" || dark.LightBgRGB != "221,216,204" {
		t.Errorf("komponen gradasi tidak cocok dengan latarnya: %+v", dark)
	}
}

// Inti pagar fitur ini: rona boleh apa saja, keterbacaan tidak boleh ikut
// berubah. Disapu seluruh lingkaran warna karena rona datang dari foto orang
// lain — kita tidak bisa memilih yang aman saja.
func TestTextStaysReadableForEveryHue(t *testing.T) {
	const onPaper = "#1A1714"
	for h := 0.0; h < 360; h += 15 {
		for s := 0.0; s <= 1.0; s += 0.1 {
			for _, mode := range []bool{false, true} {
				for _, dark := range []bool{true, false} {
					p := paletteFor(tone{hue: h, sat: s, ok: true, exact: mode}, dark)
					bg := p.Ink
					if !dark {
						bg = p.LightBg
					}
					if c := contrast(p.Paper, onPaper); c < 7 {
						t.Errorf("h=%.0f s=%.1f dark=%v: teks di guntingan kontras %.1f (mau >=7)", h, s, dark, c)
					}
					if c := contrast(p.Muted, bg); c < 3 {
						t.Errorf("h=%.0f s=%.1f dark=%v: judul-keterangan kontras %.1f (mau >=3)", h, s, dark, c)
					}
					if c := contrast(p.Faint, bg); c < 3 {
						t.Errorf("h=%.0f s=%.1f dark=%v: kaki kartu kontras %.1f (mau >=3)", h, s, dark, c)
					}
					// Stempel sumber memikul teks gelap di atas penanda; kalau
					// kontrasnya jatuh, atribusinya tidak terbaca — dan atribusi
					// adalah janji fitur ini.
					if c := contrast(p.Accent, onPaper); c < 7 {
						t.Errorf("h=%.0f s=%.1f dark=%v: teks di stempel kontras %.1f (mau >=7)", h, s, dark, c)
					}
					if dark {
						// Penanda harus terlihat di atas latar gelap. Di gaya terang
						// pagar ini tidak berlaku: penanda yang cukup cerah untuk
						// memikul teks gelap pasti berdekatan dengan kertas terang —
						// itu sudah begitu sejak kartu ini memakai satu kuning tetap.
						if c := contrast(p.Accent, bg); c < 3 {
							t.Errorf("h=%.0f s=%.1f: penanda vs latar kontras %.1f (mau >=3)", h, s, c)
						}
						// Guntingan kertas harus terbaca jelas sebagai kertas di atas
						// latar gelap — itu seluruh gagasan desainnya.
						if c := contrast(p.Paper, p.Ink); c < 10 {
							t.Errorf("h=%.0f s=%.1f: kertas vs latar kontras %.1f (mau >=10)", h, s, c)
						}
					}
				}
			}
		}
	}
}

// fill membuat foto uji: latar kelabu, lalu sebagian kecil diberi warna.
func fill(w, h int, base color.Color, patchW int, patch color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := base
			if x < patchW {
				c = patch
			}
			img.Set(x, y, c)
		}
	}
	return img
}

// Yang menentukan warna kartu adalah benda berwarna di foto, bukan luasnya.
// Dinding kelabu selalu menang kalau dihitung per luas, dan hasilnya semua kartu
// jadi kelabu — persis keluhan yang memicu fitur ini.
func TestSubjectColourBeatsTheGreyBackground(t *testing.T) {
	// 80% kelabu, 20% merah.
	img := fill(100, 40, color.RGBA{128, 128, 128, 255}, 20, color.RGBA{200, 30, 30, 255})
	got := toneOf(img)
	if !got.ok {
		t.Fatal("rona tidak terbaca padahal ada bidang merah jelas")
	}
	// Merah ada di sekitar 0 derajat; 350-360 sama saja karena lingkaran.
	if got.hue > 15 && got.hue < 345 {
		t.Errorf("rona = %.0f derajat, mau di sekitar merah (0)", got.hue)
	}
	if got.sat < 0.5 {
		t.Errorf("kepekatan = %.2f, mau pekat mengikuti bidang merahnya", got.sat)
	}
}

// Foto hitam putih tidak punya rona yang jujur bisa dipinjam. Menebaknya akan
// memberi warna yang tidak ada di fotonya sama sekali.
func TestGreyPhotoFallsBackInsteadOfGuessing(t *testing.T) {
	img := fill(100, 40, color.RGBA{90, 90, 90, 255}, 30, color.RGBA{210, 210, 210, 255})
	if got := toneOf(img); got.ok {
		t.Errorf("foto kelabu menghasilkan rona %.0f derajat, mau jatuh ke palet bawaan", got.hue)
	}
}

// Satu logo kecil berwarna di pojok tidak boleh menentukan warna seluruh kartu.
func TestATinyColourPatchIsNotTheSubject(t *testing.T) {
	// 1% berwarna, sisanya kelabu — di bawah ambang 2%.
	img := fill(100, 40, color.RGBA{120, 120, 120, 255}, 1, color.RGBA{0, 120, 255, 255})
	if got := toneOf(img); got.ok {
		t.Errorf("bercak 1%% menentukan warna kartu (rona %.0f)", got.hue)
	}
}

// Rona itu sudut. Merah tersebar di 350 dan 10 derajat harus bertemu di 0,
// bukan dirata-ratakan jadi 180 — yang justru warna kebalikannya.
func TestHueAveragesAroundTheCircleNotAcrossIt(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 100; x++ {
			if x%2 == 0 {
				img.Set(x, y, color.RGBA{200, 20, 40, 255}) // sedikit di bawah 0
			} else {
				img.Set(x, y, color.RGBA{200, 40, 20, 255}) // sedikit di atas 0
			}
		}
	}
	got := toneOf(img)
	if !got.ok {
		t.Fatal("rona tidak terbaca")
	}
	if got.hue > 20 && got.hue < 340 {
		t.Errorf("rona = %.0f derajat — dirata-ratakan menyeberangi lingkaran", got.hue)
	}
}

// Piksel tembus pandang tidak terlihat di kartu, jadi tidak boleh ikut
// menentukan warnanya.
func TestTransparentPixelsAreIgnored(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 100; x++ {
			if x < 90 {
				img.Set(x, y, color.RGBA{0, 0, 0, 0}) // hijau tak terlihat
			} else {
				img.Set(x, y, color.RGBA{30, 60, 200, 255})
			}
		}
	}
	got := toneOf(img)
	if !got.ok {
		t.Fatal("rona tidak terbaca padahal ada bidang biru terlihat")
	}
	if got.hue < 200 || got.hue > 260 {
		t.Errorf("rona = %.0f derajat, mau biru (200-260)", got.hue)
	}
}

// Bolak-balik RGB→HSL→RGB tidak boleh menggeser warna. Kalau bergeser, palet
// bawaan pun akan meleset dari nilai yang tertulis di desainnya.
func TestColourSurvivesTheRoundTrip(t *testing.T) {
	for _, c := range []struct{ r, g, b int }{
		{20, 23, 28}, {239, 235, 225}, {228, 180, 41}, {0, 0, 0}, {255, 255, 255}, {128, 128, 128},
	} {
		h, s, l := toHSL(float64(c.r)/255, float64(c.g)/255, float64(c.b)/255)
		gr, gg, gb := toRGB(h, s, l)
		if abs(gr-c.r) > 1 || abs(gg-c.g) > 1 || abs(gb-c.b) > 1 {
			t.Errorf("(%d,%d,%d) -> HSL -> (%d,%d,%d)", c.r, c.g, c.b, gr, gg, gb)
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Satu keluarga warna jarang jatuh rapi dalam satu kelompok rona. Rona hangat
// sebuah ruangan tersebar di 0-45 derajat, dan tanpa memperhitungkan tetangga
// ia bisa kalah oleh satu bilah sempit yang pekat — warna kartu jadi diambil
// dari hal terkecil di fotonya.
func TestAColourFamilyBeatsANarrowSpike(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 40))
	warm := []color.RGBA{{204, 76, 51, 255}, {204, 115, 51, 255}, {204, 153, 51, 255}}
	for y := 0; y < 40; y++ {
		for x := 0; x < 100; x++ {
			switch {
			case x < 60: // 60% hangat, tersebar di tiga kelompok rona
				img.Set(x, y, warm[x%3])
			case x < 80: // 20% toska pekat, menumpuk di satu kelompok
				img.Set(x, y, color.RGBA{13, 242, 223, 255})
			default:
				img.Set(x, y, color.RGBA{120, 120, 120, 255})
			}
		}
	}
	got := toneOf(img)
	if !got.ok {
		t.Fatal("rona tidak terbaca")
	}
	if got.hue < 0 || got.hue > 60 {
		t.Errorf("rona = %.0f derajat, mau keluarga hangat (0-60) bukan bilah toska", got.hue)
	}
}

// Warna manual harus melewati jalur yang sama persis dengan warna dari foto.
// Kalau tidak, "sesuaikan seluruh warna dari satu warna" jadi janji kosong:
// yang berubah cuma satu bidang, sisanya tetap warna lama.
func TestCustomColourDrivesTheWholePalette(t *testing.T) {
	fromHex := paletteFor(toneOfHex("#1E6FD9"), true) // biru
	fromDefault := paletteFor(tone{}, true)
	if fromHex.Ink == fromDefault.Ink {
		t.Error("latar tidak ikut warna pilihan")
	}
	if fromHex.Paper == fromDefault.Paper {
		t.Error("kertas tidak ikut warna pilihan")
	}
	if fromHex.Muted == fromDefault.Muted || fromHex.Faint == fromDefault.Faint {
		t.Error("teks pendukung tidak ikut warna pilihan")
	}
	// Dan tetap terbaca — pagar yang sama berlaku untuk warna pilihan sendiri.
	if c := contrast(fromHex.Paper, "#1A1714"); c < 7 {
		t.Errorf("kontras teks di kertas %.1f, mau >=7", c)
	}
}

func TestParseHexAcceptsOnlyFullSixDigits(t *testing.T) {
	for _, in := range []string{"#1E6FD9", "1e6fd9", "  #1E6FD9  "} {
		if got, ok := parseHex(in); !ok || got != "#1E6FD9" {
			t.Errorf("parseHex(%q) = %q,%v — mau #1E6FD9,true", in, got, ok)
		}
	}
	// Tiga digit ditolak, bukan ditebak: pengguna lebih baik tahu sekarang.
	for _, in := range []string{"", "#abc", "#12345", "#1234567", "#GGGGGG", "auto", "none"} {
		if _, ok := parseHex(in); ok {
			t.Errorf("parseHex(%q) seharusnya ditolak", in)
		}
	}
}

// Penanda mengikuti rona kartu, tapi tidak pernah SAMA dengan latar atau
// kertasnya — kalau sama, ia lenyap dan kartu kehilangan penandanya.
func TestAccentFollowsTheHueWithoutMatchingIt(t *testing.T) {
	for h := 0.0; h < 360; h += 30 {
		for _, dark := range []bool{true, false} {
			p := paletteFor(tone{hue: h, sat: 0.6, ok: true}, dark)
			if p.Accent == baseAccent {
				t.Errorf("h=%.0f dark=%v: penanda masih kuning dasar", h, dark)
			}
			if p.Accent == p.Ink || p.Accent == p.Paper || p.Accent == p.LightBg {
				t.Errorf("h=%.0f dark=%v: penanda sewarna dengan latar/kertas (%s)", h, dark, p.Accent)
			}
		}
	}
}

// Daftar warna pilihan harus benar-benar berpengaruh: kalau dua contekan
// menghasilkan kartu yang sama, salah satunya cuma tombol kosong — persis
// keluhan yang membuat pemilih spektrum penuh dibuang.
func TestEverySwatchProducesADistinctCard(t *testing.T) {
	seen := map[palette]string{}
	rows := Swatches()
	if len(rows) < 7 {
		t.Fatalf("baru %d keluarga warna", len(rows))
	}
	// "jumlahnya sama" — tiap baris harus sepanjang baris lain, kalau tidak
	// tabelnya terlihat compang-camping.
	for i, r := range rows {
		if len(r) != len(rows[0]) {
			t.Errorf("baris %d berisi %d warna, baris lain %d", i, len(r), len(rows[0]))
		}
	}
	for r, row := range rows {
		if len(row) < 6 {
			t.Errorf("baris %d cuma %d warna", r, len(row))
		}
		for _, sw := range row {
			got := toneOfHex(sw)
			if !got.ok {
				t.Errorf("baris %d, contekan %s tidak terbaca", r, sw)
				continue
			}
			p := paletteFor(got, true)
			if prev, dup := seen[p]; dup {
				t.Errorf("contekan %s menghasilkan kartu yang sama dengan %s", sw, prev)
			}
			seen[p] = sw
		}
	}
}

// Warna pilihan yang nyaris kelabu HARUS tetap berpengaruh. Untuk foto, kelabu
// jatuh ke palet bawaan karena kita tidak tahu apakah itu maunya; untuk warna
// pilihan kita tahu — pengguna melihat contekannya lalu menekannya.
func TestNeutralChoiceStaysNeutralInsteadOfFallingBack(t *testing.T) {
	base := paletteFor(tone{}, true)
	grey := paletteFor(toneOfHex("#9E9E9E"), true)
	if grey.Ink == base.Ink {
		t.Error("pilihan kelabu jatuh ke palet bawaan — tombolnya jadi tidak berpengaruh")
	}
	// Dan hasilnya memang kelabu, bukan dipaksa berona.
	r, g, b := 0, 0, 0
	if _, err := fmtSscanHex(grey.Ink, &r, &g, &b); err != nil {
		t.Fatal(err)
	}
	if spread := max3(r, g, b) - min3(r, g, b); spread > 4 {
		t.Errorf("latar %s tidak netral (selisih komponen %d)", grey.Ink, spread)
	}
	// Foto kelabu tetap jatuh ke bawaan — perilaku itu tidak boleh ikut berubah.
	if got := paletteFor(tone{hue: 200, sat: 0.05, ok: true}, true); got.Ink == grey.Ink {
		t.Error("foto nyaris kelabu ikut diperlakukan sebagai pilihan manual")
	}
}

func fmtSscanHex(h string, r, g, b *int) (int, error) {
	n, err := strconv.ParseUint(strings.TrimPrefix(h, "#"), 16, 32)
	if err != nil {
		return 0, err
	}
	*r, *g, *b = int(n>>16)&0xFF, int(n>>8)&0xFF, int(n)&0xFF
	return 3, nil
}

func max3(a, b, c int) int { return max(a, max(b, c)) }
func min3(a, b, c int) int { return min(a, min(b, c)) }

// Penanda memakai warna yang ditekan pengguna APA ADANYA. Menurunkannya jadi
// versi lain berarti contekan yang dilihat bukan warna yang didapat — merah
// menyala keluar sebagai merah bata.
func TestAccentIsExactlyTheColourThatWasPicked(t *testing.T) {
	for _, row := range Swatches() {
		for _, sw := range row {
			if got := paletteFor(toneOfHex(sw), true).Accent; got != sw {
				t.Errorf("contekan %s menghasilkan penanda %s", sw, got)
			}
		}
	}
}

// Teks stempel memilih gelap atau terang — mana pun yang lebih terbaca. Sebelum
// ini ia selalu gelap, dan di atas merah menyala kontrasnya cuma 4,3 sementara
// pilihan terang tersedia.
func TestStampTextPicksWhicheverReadsBetter(t *testing.T) {
	both := map[string]bool{}
	for _, row := range Swatches() {
		for _, sw := range row {
			p := paletteFor(toneOfHex(sw), true)
			both[p.OnAccent] = true
			dark := contrast(p.Accent, inkOnAccent)
			light := contrast(p.Accent, paperOnAccent)
			want := inkOnAccent
			if light > dark {
				want = paperOnAccent
			}
			if p.OnAccent != want {
				t.Errorf("penanda %s: teks %s (kontras %.1f), padahal %s memberi %.1f",
					sw, p.OnAccent, contrast(p.Accent, p.OnAccent), want, max(dark, light))
			}
			// 4,05 adalah batas matematis di titik peralihan; di bawah itu berarti
			// ada yang salah, bukan sekadar warna yang sulit.
			if c := contrast(p.Accent, p.OnAccent); c < 4 {
				t.Errorf("penanda %s: kontras teks stempel %.1f (mau >=4)", sw, c)
			}
		}
	}
	if len(both) < 2 {
		t.Error("tidak ada satu pun warna yang memakai teks terang — pemilihannya tidak pernah terpakai")
	}
}
