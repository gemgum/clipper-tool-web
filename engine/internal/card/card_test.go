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

// Inti perbaikan: paragraf sepanjang apa pun harus tetap muat. Dulu ukurannya
// tangga dan anak tangga teratasnya terbuka, jadi paragraf panjang tumpah keluar
// kartu — dan panjang paragraf datang dari artikel orang lain, tidak bisa
// dipesan.
func TestLongParagraphsShrinkInsteadOfOverflowing(t *testing.T) {
	for chars := 40; chars <= 2000; chars += 20 {
		size := heroSizeFor(chars, true, false, 0)
		if size < heroMin || size > heroMax {
			t.Fatalf("%d huruf: ukuran %d di luar batas %d-%d", chars, size, heroMin, heroMax)
		}
		// Di ukuran terkecil pun teks bisa tidak muat; itu batas yang disengaja
		// (lihat heroMin), jadi yang diuji hanya selama belum menyentuhnya.
		if size > heroMin {
			if h := heroHeight(chars, size); h > heroRoomWithPhoto {
				t.Errorf("%d huruf pada %d px: tinggi %.0f px, ruang hanya %d px",
					chars, size, h, heroRoomWithPhoto)
			}
		}
	}
}

// Teks yang panjangnya bertambah tidak boleh membesar hurufnya. Kalau urutannya
// bisa naik-turun, dua kartu berdampingan terlihat tidak sengaja dirancang.
func TestHeroSizeNeverGrowsWithLongerText(t *testing.T) {
	prev := heroSizeFor(1, true, false, 0)
	for chars := 2; chars <= 2000; chars++ {
		size := heroSizeFor(chars, true, false, 0)
		if size > prev {
			t.Fatalf("%d huruf: ukuran naik dari %d ke %d", chars, prev, size)
		}
		prev = size
	}
}

// Kartu kutipan tidak berfoto, jadi ruangnya jauh lebih lega dan hurufnya boleh
// lebih besar — itu memang yang membedakannya.
func TestQuoteCardsUseTheRoomTheyHave(t *testing.T) {
	withPhoto := heroSizeFor(400, true, false, 0)
	quote := heroSizeFor(400, false, true, 0)
	if quote <= withPhoto {
		t.Errorf("kartu kutipan %d px, kartu berfoto %d px — seharusnya lebih besar", quote, withPhoto)
	}
	if quote > heroQuoteMax {
		t.Errorf("kartu kutipan %d px melewati batas %d px", quote, heroQuoteMax)
	}
}

// Zoom foto dibaca relatif terhadap titik awal modenya — sama seperti sumbu zoom
// di tab klip. Batasnya tidak berubah oleh penambahan mode "whole".
func TestPhotoZoomBoundsUnchanged(t *testing.T) {
	for in, want := range map[float64]float64{0: 1, 0.5: 1, 1: 1, 2.5: 2.5, 4: 4, 99: 4} {
		if got := clampZoom(in); got != want {
			t.Errorf("clampZoom(%v) = %v, mau %v", in, got, want)
		}
	}
}

// Ukuran standar paragraf harus SAMA PERSIS dengan tangga yang dipakai sejak
// kartu ini lahir. Bukan "kira-kira sama" — toleransi ±6 px yang sempat dipakai
// di sini justru meloloskan regresi yang membesarkan paragraf 120 huruf dari 56
// ke 62 px, dan barulah pengguna yang menemukannya.
func TestStandardSizeMatchesTheOriginalLadderExactly(t *testing.T) {
	for _, c := range []struct{ chars, want int }{
		{1, 62}, {80, 62}, {110, 62},
		{111, 56}, {120, 56}, {150, 56}, {170, 56},
		{171, 50}, {200, 50}, {240, 50},
		{241, 44}, {280, 44}, {320, 44},
	} {
		if got := heroSizeFor(c.chars, true, false, 0); got != c.want {
			t.Errorf("%d huruf = %d px, mau tepat %d px", c.chars, got, c.want)
		}
	}
}

// Judul tetap jauh lebih kecil dari paragraf: hierarki kartu ini memang dibalik,
// judul turun pangkat jadi keterangan di atas kutipannya.
func TestTitleStaysSmallerThanTheParagraph(t *testing.T) {
	for chars := 1; chars <= heroLadderTop; chars++ {
		if para := heroSizeFor(chars, true, false, 0); para <= contextSize {
			t.Fatalf("%d huruf: paragraf %d px tidak lebih besar dari judul %d px",
				chars, para, contextSize)
		}
	}
}

// Langkah pengguna menggeser dari ukuran standar, dan nol berarti standar apa
// adanya — kalau tidak, membuka GUI saja sudah mengubah kartu.
func TestFontStepsMoveFromTheStandard(t *testing.T) {
	std := heroSizeFor(200, true, false, 0)
	if up := heroSizeFor(200, true, false, 1); up <= std {
		t.Errorf("satu langkah naik = %d px, standar %d px", up, std)
	}
	if down := heroSizeFor(200, true, false, -1); down >= std {
		t.Errorf("satu langkah turun = %d px, standar %d px", down, std)
	}
	// Di luar batas dijepit, tidak diteruskan.
	if a, b := heroSizeFor(200, true, false, 9), heroSizeFor(200, true, false, FontSteps); a != b {
		t.Errorf("langkah 9 = %d px, mau sama dengan batas %d px", a, b)
	}
	if got, want := scaled(38, 0), 38; got != want {
		t.Errorf("scaled(38,0) = %d, mau %d — nol harus berarti standar", got, want)
	}
}

// Paragraf panjang tetap dijaga muat, bahkan saat pengguna memperbesarnya.
// Kendali manual boleh menang atas selera, tidak boleh menang atas tepi kartu.
func TestUserStepCannotPushLongTextOutOfTheCard(t *testing.T) {
	for _, chars := range []int{400, 520, 700, 1200} {
		size := heroSizeFor(chars, true, false, FontSteps)
		if size > heroMin && heroHeight(chars, size) > heroRoomWithPhoto {
			t.Errorf("%d huruf pada langkah maksimum: tinggi %.0f px, ruang %d px",
				chars, heroHeight(chars, size), heroRoomWithPhoto)
		}
	}
}

// Rentang ±10 langkah membuat pengguna bisa memperbesar jauh. Itu tidak boleh
// berarti teks bisa didorong keluar kartu — pengecilan agar muat tetap kata
// terakhir, di panjang paragraf mana pun.
func TestMaxEnlargementNeverOverflows(t *testing.T) {
	for chars := 20; chars <= 2000; chars += 10 {
		size := heroSizeFor(chars, true, false, FontSteps)
		if size > heroMin && heroHeight(chars, size) > heroRoomWithPhoto {
			t.Errorf("%d huruf pada +%d langkah: tinggi %.0f px, ruang %d px",
				chars, FontSteps, heroHeight(chars, size), heroRoomWithPhoto)
		}
	}
}

// Langkah paling kecil tidak boleh menghasilkan huruf yang hilang atau tak
// terbaca. Pada 10% per langkah, -10 langkah berarti dikali nol — itulah sebabnya
// besar langkahnya dikecilkan saat rentangnya diperlebar.
func TestSmallestStepStaysReadable(t *testing.T) {
	if got := heroSizeFor(200, true, false, -FontSteps); got < heroMin {
		t.Errorf("paragraf pada -%d langkah = %d px, batas bawah %d px", FontSteps, got, heroMin)
	}
	if got := scaled(contextSize, -FontSteps); got <= 0 {
		t.Errorf("judul pada -%d langkah = %d px", FontSteps, got)
	}
	if got := max(contextMin, scaled(contextSize, -FontSteps)); got < contextMin {
		t.Errorf("judul menembus batas bawah: %d px", got)
	}
}

// Rentang selebar ini memang memungkinkan judul dibuat lebih besar dari
// paragraf. Itu keputusan pengguna, bukan kecelakaan — yang dijaga hanya
// bawaannya (langkah 0), diuji terpisah.
func TestUserCanOutgrowTheDefaultHierarchy(t *testing.T) {
	title := scaled(contextSize, FontSteps)
	para := heroSizeFor(200, true, false, -FontSteps)
	if title <= para {
		t.Errorf("judul maksimum %d px tidak bisa melampaui paragraf minimum %d px", title, para)
	}
}

// Menggeser isi ke bawah TIDAK boleh mengubah ukuran hurufnya. Sempat begitu,
// dan hasilnya kebalikan dari yang diminta: menggeser sedikit membuat paragraf
// menyusut drastis sementara ruang di bawahnya masih kosong.
func TestPushingDownNeverChangesTheFontSize(t *testing.T) {
	for _, chars := range []int{120, 240, 400, 700, 1200} {
		for _, step := range []int{-FontSteps, 0, FontSteps} {
			want := heroSizeFor(chars, true, false, step)
			// Ukuran dihitung tanpa tahu geserannya sama sekali — tanda tangannya
			// memang tidak lagi menerima parameter itu. Yang diuji di sini:
			// geseran yang dipilih tidak pernah memaksa ukuran lain.
			for _, hdr := range []int{0, 100, 200, HeaderMax} {
				if h := headerFor(hdr, chars, true, want); h < 0 || h > hdr {
					t.Errorf("%d huruf: geseran %d jadi %d", chars, hdr, h)
				}
			}
		}
	}
}

// Geseran berhenti di titik isi menyentuh tepi bawah — itu yang mengalah, bukan
// ukuran hurufnya.
//
// Yang dijaga BUKAN "makin panjang makin sedikit": paragraf sangat panjang
// hurufnya sudah dikecilkan lebih dulu, jadi ia justru bisa digeser lagi. Yang
// dijaga adalah invarian sebenarnya — pada geseran yang dipilih, isinya muat.
func TestHeaderStopsWhenTheContentWouldRunOut(t *testing.T) {
	if got := headerFor(HeaderMax, 60, true, heroSizeFor(60, true, false, 0)); got == 0 {
		t.Error("paragraf sangat pendek seharusnya masih bisa digeser")
	}
	if got := headerFor(HeaderMax, 240, true, heroSizeFor(240, true, false, 0)); got != 0 {
		t.Errorf("paragraf 240 huruf sudah nyaris memenuhi kartu, geseran = %d px, mau 0", got)
	}
	for chars := 40; chars <= 2000; chars += 20 {
		size := heroSizeFor(chars, true, false, 0)
		h := headerFor(HeaderMax, chars, true, size)
		if h == 0 {
			continue // tidak bisa digeser; tidak ada yang perlu dijaga
		}
		if heroHeight(chars, size) > float64(heroRoomWithPhoto-h) {
			t.Errorf("%d huruf pada geseran %d px: tinggi %.0f px, ruang %d px",
				chars, h, heroHeight(chars, size), heroRoomWithPhoto-h)
		}
	}
}

// Meminta lebih sedikit tidak pernah dinaikkan diam-diam.
func TestHeaderNeverExceedsWhatWasAsked(t *testing.T) {
	size := heroSizeFor(120, true, false, 0)
	if got := headerFor(50, 120, true, size); got != 50 {
		t.Errorf("permintaan 50 px jadi %d px", got)
	}
	if got := headerFor(-500, 120, true, size); got != 0 {
		t.Errorf("permintaan negatif jadi %d px, mau 0", got)
	}
}

// Menurunkan kartu dan menggeser blok isi memakan jatah yang SAMA: keduanya
// mendorong isi ke tepi bawah. Kalau dijepit sendiri-sendiri, memakai keduanya
// sekaligus tetap bisa memotong kaki kartu.
func TestCardTopAndHeaderShareOneBudget(t *testing.T) {
	const chars = 111
	size := heroSizeFor(chars, true, false, 0)
	total := headerFor(HeaderMax, chars, true, size)
	if total == 0 {
		t.Fatal("paragraf pendek seharusnya punya sisa ruang")
	}
	// Seluruh jatah dipakai geseran blok isi -> tidak ada sisa untuk pita atas.
	if got := budgetLeft(total, chars, true, size); got != 0 {
		t.Errorf("jatah tersisa %d px, mau 0", got)
	}
	// Separuh dipakai -> sisanya kira-kira separuh.
	if got := budgetLeft(total/2, chars, true, size); got != total-total/2 {
		t.Errorf("jatah tersisa %d px, mau %d", got, total-total/2)
	}
}

// budgetLeft meniru perhitungan di render: jatah untuk pita atas = sisa setelah
// geseran blok isi dilayani.
func budgetLeft(header, chars int, hasImage bool, size int) int {
	left := headerFor(CardTopMax, chars, hasImage, size) - header
	if left < 0 {
		return 0
	}
	return left
}
