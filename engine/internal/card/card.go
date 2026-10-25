// Package card menyusun kartu berita siap posting: data artikel dituang ke
// template HTML berukuran pas (mis. 1080x1920), lalu difoto oleh Chrome.
//
// Berbeda dari memotret situs aslinya, di sini kita yang menentukan tata letak
// — jadi tidak ada iklan yang ikut terbawa, judul tidak terpotong, dan ukuran
// hasilnya sudah pas untuk media sosial tanpa perlu dipotong lagi.
package card

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gemgum/clipper/engine/internal/capture"
	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/news"
)

// Gaya tampilan kartu.
const (
	StyleDark  = "dark"
	StyleLight = "light"
	StyleQuote = "quote" // judul besar tanpa foto
)

// Rasio keluaran.
const (
	Ratio916 = "9:16"
	Ratio45  = "4:5"
	Ratio11  = "1:1"
)

// Perataan teks judul & ringkasan di kartu.
const (
	AlignLeft    = "left"
	AlignCenter  = "center"
	AlignRight   = "right"
	AlignJustify = "justify" // rata kiri-kanan
)

// alignCSS menerjemahkan pilihan perataan ke nilai text-align, sekaligus
// menentukan margin garis aksen agar ikut berpindah mengikuti teks —
// kalau tidak, garisnya menggantung sendirian di kiri.
func alignCSS(a string) (align, ruleMargin string) {
	switch a {
	case AlignCenter:
		return "center", "0 auto 32px auto"
	case AlignRight:
		return "right", "0 0 32px auto"
	case AlignJustify:
		return "justify", "0 auto 32px 0"
	default: // left
		return "left", "0 auto 32px 0"
	}
}

// Photo mengatur bingkai foto artikel: digeser dan diperbesar di dalam area
// fotonya. Hanya memengaruhi gambar — tata letak teks di bawahnya tetap.
//
// Satuan geser memakai piksel di ruang koordinat kartu (lebar 1080), sama
// dengan yang dipakai GUI, sehingga menyeret sejauh N piksel di pratinjau
// menggeser foto sejauh N piksel juga di hasil akhir.
type Photo struct {
	OffsetX int     `json:"offset_x"`
	OffsetY int     `json:"offset_y"`
	Zoom    float64 `json:"zoom"` // 1 = titik awal mode Fit, bukan angka mutlak
	Fit     string  `json:"fit"`  // cover | whole
	Fill    string  `json:"fill"` // blur | solid — mengisi ruang yang tak terjangkau
}

// Mode pemasangan foto ke bingkainya. Sumbunya sengaja dibuat sama dengan tab
// klip video (lihat notes/15-sumbu-zoom.md): zoom dibaca RELATIF terhadap titik
// awal modenya, bukan sebagai angka mutlak.
//
//	cover : 1 = foto memenuhi bingkai, sisinya terpotong (titik awal)
//	whole : 1 = SELURUH gambar asli masuk, seberapa pun rasionya
//
// Keduanya naik sampai 4. Yang berbeda hanya dari mana hitungannya dimulai —
// pelajaran yang sudah dibayar mahal di sumbu zoom klip: pindahkan titik
// awalnya, jangan naikkan langit-langitnya.
const (
	FitCover = "cover"
	FitWhole = "whole"

	FillBlur  = "blur"
	FillSolid = "solid"
)

// Fonts menyetel ukuran huruf judul & paragraf, dalam LANGKAH dari ukuran
// standar — bukan angka piksel mutlak.
//
// Disengaja: template ini punya ukuran standar yang sudah diperhitungkan
// (paragraf mengecil sendiri kalau kepanjangan, judul menyesuaikan rasio kartu).
// Angka mutlak akan membuang semua itu dan memaksa pengguna menyetel ulang tiap
// kartu. Langkah relatif menjadikan standar sebagai titik nol yang selalu bisa
// dikembalikan.
//
// Judul dan paragraf dipisah karena keduanya menjawab kebutuhan berbeda: judul
// dipangkas dua baris, paragraf panjangnya mengikuti artikel.
type Fonts struct {
	Title     int `json:"title"`     // -FontSteps..+FontSteps
	Paragraph int `json:"paragraph"` // -FontSteps..+FontSteps
}

const (
	// Banyaknya langkah ke tiap arah, dan besar tiap langkah.
	//
	// 10 langkah x 5% = separuh sampai satu setengah kali ukuran standar. Besar
	// langkahnya dikecilkan dari 10% ke 5% justru saat rentangnya diperlebar:
	// pada 10% per langkah, -10 langkah berarti dikali nol dan hurufnya hilang.
	//
	// Rentang selebar ini membuat judul BISA dibuat lebih besar dari paragraf.
	// Itu memang menyalahi hierarki bawaan kartu ini, dan disengaja: bawaannya
	// tetap paragraf yang jadi bintang, tapi keputusan akhir ada di pengguna.
	FontSteps   = 10
	fontStepPct = 5

	// Batas bawah judul. Di bawah ini judul tidak lagi terbaca di layar ponsel,
	// dan yang tersisa cuma hiasan yang memakan tempat.
	contextMin = 16

	// HeaderMax = sejauh mana isi boleh digeser turun.
	//
	// 400 px kira-kira separuh ruang teks yang tersedia. Lebih dari itu isinya
	// bukan lagi "turun sedikit" melainkan terdesak ke tepi bawah kartu, dan
	// ruang yang tersisa untuk paragraf tinggal sedikit sekali.
	HeaderMax = 400

	// CardTopMax = setinggi apa pita kosong di atas kartu boleh dibuat.
	CardTopMax = 400
)

// scaled menerapkan langkah pengguna pada satu ukuran standar.
func scaled(size, step int) int {
	step = clamp(step, FontSteps)
	return int(math.Round(float64(size) * (100 + float64(step*fontStepPct)) / 100))
}

// Colors menentukan dari mana warna kartu diambil dan bagaimana kotak paragraf
// diperlakukan.
//
// Source "photo" memakai rona foto artikel (lihat palette.go). Source "custom"
// memakai warna pilihan pengguna, lalu MENURUNKAN seluruh palet dari situ lewat
// jalur yang sama persis — jadi satu warna cukup untuk menyetel latar, kertas,
// dan teks pendukung sekaligus.
type Colors struct {
	Source string `json:"source"` // photo | custom
	Custom string `json:"custom"` // #RRGGBB, dipakai saat source=custom
	Box    string `json:"box"`    // auto | none | #RRGGBB
}

const (
	ColorFromPhoto  = "photo"
	ColorFromCustom = "custom"

	BoxAuto = "auto"
	BoxNone = "none"
)

// Request pembuatan satu kartu.
type Request struct {
	Article news.Article `json:"article"`
	Style   string       `json:"style"`
	Ratio   string       `json:"ratio"`
	Align   string       `json:"align"` // left | center | right | justify — ringkasan & stempel
	// TitleAlign = perataan judul (dan garis aksen di atasnya); kosong = Align.
	TitleAlign string `json:"title_align"`
	Photo      Photo  `json:"photo"`
	Fonts      Fonts  `json:"fonts"`
	Colors     Colors `json:"colors"`
	// Header = ruang tambahan di bawah foto, dalam piksel ruang kartu 1920.
	//
	// Menggeser SELURUH isi (judul, guntingan, kaki kartu) turun sebagai satu
	// kesatuan. Bukan penggeser per blok — itu sudah dicoba dan dicabut, karena
	// blok yang bisa bergerak sendiri-sendiri akhirnya saling tumpang tindih.
	// Fotonya sendiri tidak ikut membesar; yang bertambah cuma jarak di
	// bawahnya, dan bagian foto di situ memang sudah memudar jadi latar.
	Header int `json:"header"`
	// CardTop = pita kosong di ATAS kartu, dalam piksel ruang kartu 1920.
	//
	// Menurunkan SELURUH kartu: area foto dan blok isi sama-sama turun, jarak
	// antara keduanya tidak berubah. Kanvasnya tetap 1080x1920 — yang muncul di
	// atas adalah latar kartu, bukan kartu yang mengecil.
	//
	// Bedanya dengan Header: Header hanya menurunkan blok isi dan menyisakan
	// ruang di bawah foto; CardTop menurunkan fotonya juga.
	CardTop int `json:"card_top"`
	// CardY = geseran batas foto–isi dalam piksel kanvas (seret di pratinjau).
	CardY int    `json:"card_y"`
	Lang  string `json:"lang"` // bahasa teks tetap di kartu (en | id)
	// Caption & Hashtags ikut disimpan sebagai berkas pendamping agar satu ZIP
	// berisi semua yang dibutuhkan saat memposting.
	Caption  string   `json:"caption"`
	Hashtags []string `json:"hashtags"`
	// Whole = carousel: setelah kartu pertama, badan artikel (Body) dipecah jadi
	// halaman-halaman lanjutan (pages.go). Page = halaman yang diminta pratinjau
	// HTML (0 = kartu pertama).
	Whole bool     `json:"whole"`
	Body  []string `json:"body"`
	Page  int      `json:"page"`
	// Box = gaya box untuk halaman lanjutan: foto artikel memenuhi bingkai dan
	// teks di dalam kotak bergaya kotak judul Mark. nil = panel tema.
	Box *PageBox `json:"box"`
	// Illustrations = foto lanjutan carousel (pages.go).
	Illustrations []Illustration `json:"illustrations"`
	// Theme = salah satu dari empat tampilan jadi (DESIGN-NEWSCARD §5.2):
	// dark | light | photo | paper. Kosong = perilaku lama (style + colors).
	// Bila diisi, ia menang atas Colors.
	Theme string `json:"theme"`
}

// Tampilan jadi kartu berita.
const (
	ThemeDark  = "dark"
	ThemeLight = "light"
	ThemePhoto = "photo"
	ThemePaper = "paper"
	// Tambahan pemilik 9 Oktober 2026 (empat preset §5.2 dirasa kurang).
	ThemeNavy   = "navy"
	ThemeRed    = "red"
	ThemeForest = "forest"
	ThemePlum   = "plum"
	// ThemeAuto = warna yang menyambung dasar foto artikel (AutoLook — rumus
	// yang sama dengan "Auto from image" di halaman Mark); tanpa foto = Dark.
	ThemeAuto = "auto"
)

// themeColors = warna satu tampilan jadi. Panel = latar kartu, Title = judul
// artikel, Body = teks ringkasan, Badge/OnBadge = garis aksen & stempel.
type themeColors struct {
	Panel, PanelRGB, Title, Body, Badge, OnBadge string
	TitlePx, BodyPx                              int
}

// themeTable: nilai persis dari DESIGN-NEWSCARD §5.2. "photo" di sini adalah
// cadangannya saat kartu tidak berfoto (atau ronanya tidak terbaca).
var themeTable = map[string]themeColors{
	ThemeDark:  {Panel: "#15181D", Title: "#FFFFFF", Body: "#C8D1DC", Badge: "#FFD400", OnBadge: "#15181D"},
	ThemeLight: {Panel: "#FFFFFF", Title: "#15181D", Body: "#5A6472", Badge: "#15181D", OnBadge: "#FFFFFF"},
	ThemePhoto: {Panel: "#3C2A1E", Title: "#FFF6EC", Body: "#DEC9B4", Badge: "#E8A33D", OnBadge: "#2A1C12"},
	ThemePaper: {Panel: "#F2EBDD", Title: "#2A2318", Body: "#5E5444", Badge: "#2A2318", OnBadge: "#F2EBDD"},
	// Di luar §5.2. Kontras teks isi di atas panel terhitung ≥ 7:1 (WCAG AAA).
	ThemeNavy:   {Panel: "#0F2747", Title: "#FFFFFF", Body: "#C9D6EA", Badge: "#FFD400", OnBadge: "#0F2747"},
	ThemeRed:    {Panel: "#7F1714", Title: "#FFFFFF", Body: "#F3D6D2", Badge: "#FFFFFF", OnBadge: "#7F1714"},
	ThemeForest: {Panel: "#163A2D", Title: "#F4FAF6", Body: "#C6DCCF", Badge: "#E8C547", OnBadge: "#163A2D"},
	ThemePlum:   {Panel: "#2E1F4A", Title: "#FFFFFF", Body: "#D5CCEA", Badge: "#F5C2E7", OnBadge: "#2E1F4A"},
}

// themePhotoPct: tinggi area foto per rasio saat tampilan jadi dipakai. Angkanya
// milik config.CardPhotoPct — tampilan kartu di halaman Watermark memakai yang sama.
func themePhotoPct(ratio string) int {
	return config.CardPhotoPct(FormatOf(ratio))
}

// FormatOf = format bingkai Watermark yang seukuran rasio kartu ini (Dims sama
// dengan config.FrameSize) — dipakai saat Mark dibakar ke kartu News.
func FormatOf(ratio string) string {
	switch ratio {
	case Ratio45:
		return config.FormatFeed45
	case Ratio11:
		return config.FormatFeed11
	}
	return config.FormatShort
}

// imageURL menandai alamat foto sebagai aman bagi html/template, HANYA bila
// sudah lolos hasImage (http(s) atau data:image/). Tanpa ini data URI
// disensor jadi "#ZgotmplZ" dan foto unggahan tidak pernah tampil.
func imageURL(src string, ok bool) template.URL {
	if !ok {
		return ""
	}
	return template.URL(src)
}

// rgbOf "#RRGGBB" → "r,g,b" untuk gradasi rgba().
func rgbOf(hex string) string {
	var r, g, b int
	fmt.Sscanf(strings.TrimPrefix(hex, "#"), "%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("%d,%d,%d", r, g, b)
}

// Nama berkas di dalam folder kartu.
const (
	FilePNG     = "card.png"
	FileCaption = "caption.txt"
	FileSource  = "source.txt"
)

// Dims menerjemahkan rasio ke ukuran piksel (lebar tetap 1080).
func Dims(ratio string) (int, int) {
	switch ratio {
	case Ratio45:
		return 1080, 1350
	case Ratio11:
		return 1080, 1080
	default:
		return 1080, 1920
	}
}

// Teks tetap yang ikut tercetak di kartu & berkas pendampingnya. Ditaruh di
// satu tempat supaya menambah bahasa baru cukup menambah satu entri.
type phrases struct {
	labelTitle string
	labelMedia string
	labelDate  string
	labelLink  string
	notice     string
}

var phraseBook = map[string]phrases{
	"en": {
		labelTitle: "Title ",
		labelMedia: "Media ",
		labelDate:  "Date  ",
		labelLink:  "Link  ",
		notice:     "The photo and text remain the property of the outlet above. Credit the source when posting.",
	},
	"id": {
		labelTitle: "Judul  ",
		labelMedia: "Media  ",
		labelDate:  "Tanggal",
		labelLink:  "Tautan ",
		notice:     "Foto dan teks tetap milik media di atas. Cantumkan sumber saat memposting.",
	},
}

func phrasesFor(lang string) phrases {
	if p, ok := phraseBook[strings.ToLower(strings.TrimSpace(lang))]; ok {
		return p
	}
	return phraseBook["en"]
}

// langAttr mengembalikan nilai atribut lang= untuk <html>.
func langAttr(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if _, ok := phraseBook[lang]; ok {
		return lang
	}
	return "en"
}

// Builder membuat kartu memakai browser & font bawaan proyek.
type Builder struct {
	cap      *capture.Client
	fontsDir string

	once    sync.Once
	fontCSS template.CSS // @font-face dengan font ditanam sebagai data URI
	fontKey string       // sidik jari fontCSS (cache browser, lihat FontKey)

	// Rona foto per alamat. Satu artikel biasanya dirender berkali-kali sambil
	// pengguna menggeser fotonya, dan foto yang sama tidak perlu diunduh ulang
	// tiap kali hanya untuk mendapat warna yang sama.
	mu    sync.Mutex
	tones map[string]tone
	// Warna tema "auto" per alamat foto, dengan alasan yang sama.
	autos map[string]LookColors
}

func New(cap *capture.Client, fontsDir string) *Builder {
	return &Builder{cap: cap, fontsDir: fontsDir}
}

// Build merender kartu ke dalam folder dir, bersama berkas pendampingnya
// (caption & sumber) supaya siap dibungkus jadi satu ZIP.
//
// preview melewati berkas pendamping: pratinjau dibuat berkali-kali sambil
// menyetel, dan caption/keterangan sumber baru berguna saat kartunya benar-benar
// disimpan.
func (b *Builder) Build(ctx context.Context, req Request, dir string, preview bool) error {
	if strings.TrimSpace(req.Article.Title) == "" {
		return fmt.Errorf("the title is empty: a card needs the article title")
	}
	outPNG := filepath.Join(dir, FilePNG)
	width, height := Dims(req.Ratio)

	htmlBytes, _, err := b.render(ctx, req, width, height)
	if err != nil {
		return err
	}

	// Halaman ditulis ke berkas sementara karena Chrome perlu URL untuk dibuka;
	// WriteTempFile menaruhnya di lokasi yang terjangkau browser (termasuk saat
	// chrome.exe dipanggil dari WSL).
	url, cleanup, err := b.cap.WriteTempFile(htmlBytes, ".html")
	if err != nil {
		return err
	}
	defer cleanup()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := b.cap.Screenshot(ctx, url, outPNG, capture.Options{
		Width:  width,
		Height: height,
		Scale:  1,
		// Foto artikel diambil dari CDN media, jadi beri waktu memuat.
		WaitMS: 15000,
	}); err != nil {
		return err
	}
	if preview {
		return nil
	}
	return WriteSidecars(dir, req)
}

// Geometry = letak bagian kartu yang bisa diseret di pratinjau, piksel kanvas.
type Geometry struct {
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	PhotoTop  int      `json:"photo_top"`     // tepi atas area foto (= pita CardTop)
	PhotoH    int      `json:"photo_h"`       // tinggi area foto (sudah digeser CardY)
	PhotoBase int      `json:"photo_base"`    // tinggi area foto tanpa CardY
	Scale     float64  `json:"scale"`         // piksel kanvas per piksel ruang 1920 (Header, CardTop)
	OffsetW   int      `json:"offset_w"`      // batas geser foto mendatar (offsetLimit)
	OffsetH   int      `json:"offset_h"`      // batas geser foto tegak
	Pages     int      `json:"pages"`         // jumlah kartu carousel (1 = tanpa halaman lanjutan)
	Ill       *IllRect `json:"ill,omitempty"` // zona ilustrasi halaman lanjutan yang dipratinjau
}

// HTML = halaman kartu yang SAMA dengan yang difoto Chrome di Build, tanpa
// Chrome: font & foto unggahan sudah tertanam, jadi GUI bisa menampilkannya
// langsung sebagai pratinjau seketika (seperti pratinjau Watermark).
// Page > 0 = halaman lanjutan carousel ke-(Page+1); Geometry.Pages = jumlah kartu.
func (b *Builder) HTML(ctx context.Context, req Request) ([]byte, Geometry, error) {
	if strings.TrimSpace(req.Article.Title) == "" {
		return nil, Geometry{}, fmt.Errorf("the title is empty: a card needs the article title")
	}
	width, height := Dims(req.Ratio)
	pages := Pages(req, width, height)
	if req.Page > 0 && req.Page <= len(pages) {
		page, ill, err := b.renderPage(ctx, req, width, height, req.Page+1, len(pages)+1, pages[req.Page-1])
		return page, Geometry{Width: width, Height: height, Scale: float64(height) / 1920, Pages: len(pages) + 1, Ill: ill}, err
	}
	page, geo, err := b.render(ctx, req, width, height)
	geo.Pages = len(pages) + 1
	return page, geo, err
}

// WriteSidecars menulis caption & keterangan sumber di sebelah gambar.
//
// Sumber selalu ditulis, walau caption kosong: atribusi harus ikut berpindah
// bersama berkasnya, supaya saat kartu dibuka lagi berminggu-minggu kemudian
// asal beritanya tidak hilang.
func WriteSidecars(dir string, req Request) error {
	if c := strings.TrimSpace(req.Caption); c != "" || len(req.Hashtags) > 0 {
		var sb strings.Builder
		sb.WriteString(c)
		if len(req.Hashtags) > 0 {
			sb.WriteString("\n\n")
			sb.WriteString(strings.Join(req.Hashtags, " "))
		}
		sb.WriteString("\n")
		if err := os.WriteFile(filepath.Join(dir, FileCaption), []byte(sb.String()), 0o644); err != nil {
			return err
		}
	}
	a := req.Article
	p := phrasesFor(req.Lang)
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s: %s\n", p.labelTitle, a.Title)
	fmt.Fprintf(&sb, "%s: %s\n", p.labelMedia, firstNonEmpty(a.Source, a.Domain))
	fmt.Fprintf(&sb, "%s: %s\n", p.labelDate, a.Date)
	fmt.Fprintf(&sb, "%s: %s\n", p.labelLink, a.URL)
	fmt.Fprintf(&sb, "\n%s\n", p.notice)
	return os.WriteFile(filepath.Join(dir, FileSource), []byte(sb.String()), 0o644)
}

// Ukuran huruf guntingan: tangga berdasarkan jumlah huruf, dengan anak tangga
// TERATASNYA ditutup.
//
// Tangganya sendiri hasil percobaan dan terbukti enak dilihat, jadi angkanya
// dipertahankan apa adadanya. Yang dulu salah cuma satu: anak tangga teratas
// terbuka (>320 huruf → 38 px, tanpa batas bawah), sehingga paragraf jauh lebih
// panjang tetap tumpah keluar kartu.
//
// Karena itu penghitungan hanya bekerja DI ATAS anak tangga teratas. Di bawahnya
// tangga menang. Sempat saya ganti seluruhnya dengan rumus, dan akibatnya
// paragraf 120 huruf membesar dari 56 ke 62 px — lebih besar dari yang pernah
// diminta siapa pun, dan merusak jarak terhadap judul yang tetap 38 px.
//
// Angka penghitungnya diukur dari render sungguhan, bukan ditebak: paragraf 520
// huruf pada ukuran 38 px jatuh tepat 15 baris di guntingan selebar 824 px.
const (
	// Tinggi yang tersisa untuk teks guntingan di ruang kartu 1920 px, saat isi
	// belum digeser (Header = 0).
	//
	// DIUKUR, bukan dijumlahkan dari tata letaknya — penjumlahan sempat dipakai
	// dan meleset jauh. Cara mengukurnya: render dengan pengecilan dimatikan,
	// lalu periksa apakah kertas atau stempel menyentuh tepi bawah kanvas.
	//   440 huruf @ 38 px → taksiran tinggi 662 px → masih muat
	//   480 huruf @ 38 px → taksiran tinggi 713 px → terpotong
	// 680 diambil di antara keduanya.
	//
	// ANGKA INI TERIKAT PADA CARA ISI DIJANGKARKAN. Ia pernah 720 waktu isi
	// dijangkarkan ke bawah; begitu jangkarnya pindah ke atas, angkanya berubah
	// dan sempat tidak diukur ulang — akibatnya menggeser isi ke bawah membuat
	// kaki kartu terpotong. Setiap kali tata letaknya diubah, ukur lagi.
	heroRoomWithPhoto = 680
	// Kartu kutipan tidak berfoto, jadi seluruh porsi foto (960 px) jadi ruang
	// tambahan. Diturunkan dari angka di atas, bukan diukur tersendiri.
	heroRoomNoPhoto = 1640

	heroCharWidth = 0.62  // lebar rata-rata satu huruf, relatif ke ukuran font
	heroLineWidth = 824.0 // lebar satu baris di dalam guntingan
	heroLineGap   = 1.34  // line-height .hero

	heroMax      = 62 // ukuran paling besar; teks pendek berhenti di sini
	heroQuoteMax = 76 // kartu kutipan tidak berfoto, jadi boleh lebih besar
	// Di bawah ini teks tidak lagi terbaca di layar ponsel. Paragraf sepanjang
	// itu memang lebih baik terpotong daripada terbit tak terbaca — dan itu
	// pertanda paragrafnya yang perlu dipilih ulang, bukan kartunya.
	heroMin = 22
)

// headerFor menjepit geseran ke sejauh isi masih muat DI UKURAN YANG SUDAH
// DIPILIH.
//
// Menggeser isi ke bawah TIDAK boleh mengecilkan hurufnya. Sempat begitu, dan
// hasilnya persis kebalikan dari yang diminta: pengguna menggeser sedikit, lalu
// paragrafnya menyusut drastis sementara ruang di bawahnya masih kosong. Yang
// diminta adalah isi yang sama, turun.
//
// Karena ukurannya tidak boleh mengalah, yang mengalah adalah geserannya:
// penggeser berhenti di titik isi menyentuh tepi bawah kartu. Berapa jauh ia
// bisa turun karena itu berbeda tiap artikel — paragraf pendek bisa jauh,
// paragraf panjang hampir tidak bisa.
func headerFor(want, chars int, hasImage bool, size int) int {
	want = min(HeaderMax, max(0, want))
	if chars <= 0 {
		return want
	}
	room := float64(heroRoomWithPhoto)
	if !hasImage {
		room = heroRoomNoPhoto
	}
	// Satu baris kelonggaran, alasannya sama dengan di heroSizeFor: taksiran
	// tinggi meleset sampai satu baris karena teks membungkus di batas kata.
	used := heroHeight(chars, size) + float64(size)*heroLineGap
	return min(want, max(0, int(room-used)))
}

// heroHeight menaksir tinggi teks guntingan pada satu ukuran huruf.
//
// Baris dibulatkan KE ATAS karena baris tidak bisa setengah: sisa satu kata pun
// memakan satu baris penuh. Pembulatan inilah yang membuat rumus tertutup saja
// tidak cukup — lihat heroSizeFor.
func heroHeight(chars, size int) float64 {
	lines := math.Ceil(float64(chars) * heroCharWidth * float64(size) / heroLineWidth)
	return lines * float64(size) * heroLineGap
}

// heroLadder = ukuran standar paragraf menurut panjangnya. Nilai-nilai ini hasil
// percobaan sejak kartu ini lahir; jangan diubah tanpa melihat hasil rendernya.
func heroLadder(chars int) int {
	switch {
	case chars > heroLadderTop:
		return 38
	case chars > 240:
		return 44
	case chars > 170:
		return 50
	case chars > 110:
		return 56
	}
	return 62
}

const (
	// heroLadderTop = anak tangga teratas. Di atas sinilah dulu ukurannya berhenti
	// mengecil dan teks mulai tumpah.
	heroLadderTop = 320

	// contextSize = ukuran standar judul. Ia sengaja jauh di bawah paragraf:
	// hierarki kartu ini dibalik, judul turun pangkat jadi keterangan di atas
	// kutipannya. Langkah pengguna menggeser dari sini, bukan menggantikannya.
	contextSize = 38
)

// heroSizeFor menentukan ukuran huruf paragraf.
//
// Urutannya: tangga dulu (agar kartu yang selama ini enak dilihat tidak
// berubah), lalu langkah pilihan pengguna, baru pengecilan agar muat.
//
// Pengecilan itu berjalan SELALU, bukan hanya untuk paragraf panjang. Ia aman
// bagi nilai tangga karena tidak satu pun dari mereka melewati ruang yang ada
// (yang terbesar 240 huruf @ 50 px = 670 px, ruangnya 720 px) — dijaga oleh
// TestStandardSizeMatchesTheOriginalLadderExactly. Gunanya sekarang untuk
// pengguna: memperbesar +10 langkah tidak bisa mendorong teks keluar kartu.
func heroSizeFor(chars int, hasImage, quote bool, step int) int {
	if chars <= 0 {
		chars = 1
	}
	size := heroLadder(chars)
	if quote {
		size += 14 // tanpa foto, ruangnya lebih lega
	}
	size = scaled(size, step)

	room := float64(heroRoomWithPhoto)
	if !hasImage {
		room = heroRoomNoPhoto
	}
	// Taksiran tinggi meleset sampai SATU BARIS, karena teks membungkus di batas
	// kata sementara taksirannya menghitung huruf. Di ukuran besar satu baris itu
	// 80 px — cukup untuk memotong stempel sumber.
	//
	// Kelonggaran ini TIDAK dipakai pada setelan bawaan, sebab nilai tangga sudah
	// diuji satu per satu di render sungguhan dan memang muat. Menerapkannya di
	// sana justru mengecilkan kartu yang sudah benar.
	if step != 0 {
		room -= float64(size) * heroLineGap
	}
	for size > heroMin && heroHeight(chars, size) > room {
		size--
	}
	if size < heroMin {
		size = heroMin
	}
	return size
}

// templateData = data siap pakai untuk template.
type templateData struct {
	Width, Height int
	Dark          bool
	Quote         bool
	HasImage      bool
	Image         template.URL // sudah diperiksa: http(s) atau data:image/
	Domain        string
	Date          string
	FontCSS       template.CSS
	PhotoX        int
	PhotoY        int
	PhotoZoom     string  // ditulis sebagai teks agar tidak jadi notasi ilmiah
	Align         string  // nilai text-align
	RuleMargin    string  // margin garis aksen, ikut perataan judul
	TitleAlign    string  // nilai text-align judul
	Lang          string  // atribut lang= pada <html>
	Palette       palette // warna kartu, diturunkan dari foto atau warna pilihan
	PhotoFit      string  // cover | contain
	PhotoFill     bool    // isi ruang sisa dengan salinan buram fotonya
	BoxOff        bool    // kotak paragraf dihilangkan
	ContentTop    int     // jarak isi dari bawah foto, sudah termasuk geseran
	CardTop       int     // pita kosong di atas kartu
	PhotoPx       int     // tinggi area foto dalam piksel

	// Hero = teks yang jadi bintang kartu, ditaruh di guntingan kertas.
	// Context = judul artikel sebagai keterangan di atasnya; kosong bila judul
	// itu sendiri yang jadi Hero (artikel tanpa kutipan terpilih).
	Hero        string
	Context     string
	HeroSize    int
	ContextSize int
	SmallSize   int
	Padding     int
	Theme       *themeColors // nil = tanpa tampilan jadi
}

func (b *Builder) render(ctx context.Context, req Request, width, height int) ([]byte, Geometry, error) {
	a := req.Article
	quote := req.Style == StyleQuote
	// Foto boleh alamat http(s) ATAU data URI — unggahan pengguna ditanam
	// sebagai data URI oleh pemanggil (api), sebab Chrome tidak membaca path.
	hasImage := isImageURL(a.Image) && !quote

	// Warna kartu: dari warna pilihan pengguna bila ada, selain itu dari rona
	// fotonya. Keduanya lewat paletteFor yang sama, jadi satu warna cukup untuk
	// menyetel latar, kertas, dan teks pendukung sekaligus.
	dark := req.Style != StyleLight
	var pal palette
	switch {
	case req.Colors.Source == ColorFromCustom:
		pal = paletteFor(toneOfHex(req.Colors.Custom), dark)
	case hasImage:
		pal = paletteFor(b.tone(ctx, a.Image), dark)
	default:
		// Kartu kutipan tidak punya foto — tidak ada apa pun untuk ditiru.
		pal = paletteFor(tone{}, dark)
	}
	// Kotak paragraf boleh ditimpa: dihilangkan sama sekali, atau diberi warna
	// sendiri. Keduanya tidak mengubah warna kartu lainnya.
	box := strings.TrimSpace(req.Colors.Box)
	boxTransparent := box == BoxNone
	if hex, ok := parseHex(box); ok {
		pal.Paper = hex
	}

	// Tampilan jadi: menimpa warna lewat satu blok CSS di akhir template, jadi
	// jalur lama (style + colors) tidak tersentuh sama sekali.
	theme := b.themeFor(ctx, req, hasImage)

	// Skala mengikuti tinggi kanvas supaya kartu 1:1 dan 4:5 tidak terlihat
	// seperti kartu 9:16 yang dipangkas.
	scale := float64(height) / 1920.0
	px := func(n int) int { return int(float64(n) * scale) }

	// Hierarki sengaja dibalik dari kartu berita pada umumnya: yang jadi bintang
	// adalah PARAGRAF TERPILIH, bukan judul. Alasannya bukan gaya — seluruh
	// fitur ini memang bekerja dengan memilih satu paragraf, jadi paragraf itulah
	// muatan kartunya. Judul turun pangkat jadi keterangan di atasnya.
	//
	// Bila tidak ada paragraf terpilih, judul naik jadi Hero supaya kartu tetap
	// punya isi.
	hero := strings.TrimSpace(a.Summary)
	context := strings.TrimSpace(a.Title)
	// Tampilan jadi (theme) TIDAK menukar: di sana judul memang yang terbesar
	// (TitlePx) dan ringkasan berukuran teks isi (BodyPx) — judul yang dinaikkan
	// jadi hero malah mengecil. Ringkasan kosong = blok ringkasannya tidak ada.
	if hero == "" && theme == nil {
		hero, context = context, ""
	}

	// Ukuran huruf ditentukan lebih dulu dan TIDAK dipengaruhi geseran; geseran
	// yang menyesuaikan diri, bukan sebaliknya.
	chars := len([]rune(hero))
	heroSize := heroSizeFor(chars, hasImage, quote, req.Fonts.Paragraph)
	header := headerFor(req.Header, chars, hasImage, heroSize)
	// Menurunkan kartu memakan sisa ruang yang SAMA dengan menurunkan blok isi:
	// keduanya mendorong isi ke arah tepi bawah. Karena itu jatah dihitung
	// berurutan — geseran blok isi dilayani dulu, sisanya untuk pita atas.
	cardTop := min(CardTopMax, max(0, req.CardTop))
	if room := headerFor(CardTopMax, chars, hasImage, heroSize) - header; cardTop > room {
		cardTop = max(0, room)
	}

	// Isi kartu dijangkarkan ke bawah, jadi sisa ruang jatuh tepat di bawah foto.
	// Porsi foto dibuat agak besar supaya ruang itu terisi gambar, bukan jadi
	// pita gelap kosong.
	photoHeight := 50
	if req.Ratio == Ratio11 {
		// Kanvas persegi jauh lebih pendek, jadi porsi foto yang sama menyisakan
		// pita gelap saat kutipannya singkat.
		photoHeight = 48
	}
	if theme != nil {
		photoHeight = themePhotoPct(req.Ratio)
	}
	// Ukuran bingkai foto dalam piksel — dipakai untuk membatasi geseran.
	// CardY menggeser BATAS foto–isi (seretan pratinjau, sama dengan
	// config.Headline.CardY di Mark): foto memanjang/memendek, isi ikut, dijepit
	// 20–85% kanvas supaya foto dan isi sama-sama tetap ada.
	photoBase := height * photoHeight / 100
	frameHeight := min(max(photoBase+req.CardY, height/5), height*85/100)
	zoom := clampZoom(req.Photo.Zoom)
	whole := req.Photo.Fit == FitWhole
	// Mode "whole" memakai object-fit:contain — seluruh gambar masuk apa pun
	// rasionya. Ruang yang tak terjangkau gambar diisi salinan buram fotonya
	// (bawaan) atau warna latar kartu.
	fitCSS, fillBlur := "cover", false
	if whole {
		fitCSS = "contain"
		fillBlur = req.Photo.Fill != FillSolid
	}
	// Pada mode whole, zoom 1 berarti gambar utuh — ia belum tentu memenuhi
	// bingkai, jadi geseran belum punya ruang sampai zoom melewati 1. Batasnya
	// dihitung dengan rumus yang sama supaya GUI cukup mencerminkan satu rumus.
	limitW, limitH := offsetLimit(width, zoom), offsetLimit(frameHeight, zoom)
	align, _ := alignCSS(req.Align)
	titleAlign, ruleMargin := alignCSS(firstNonEmpty(req.TitleAlign, req.Align))

	d := templateData{
		Width: width, Height: height,
		Dark:      req.Style != StyleLight,
		Quote:     quote,
		HasImage:  hasImage,
		Image:     imageURL(a.Image, hasImage),
		Hero:      hero,
		Context:   context,
		Domain:    a.Domain,
		Date:      a.Date,
		FontCSS:   b.FontCSS(),
		PhotoX:    clamp(req.Photo.OffsetX, limitW),
		PhotoY:    clamp(req.Photo.OffsetY, limitH),
		PhotoFit:  fitCSS,
		PhotoFill: fillBlur,
		BoxOff:    boxTransparent,
		// -72 = jarak bawaan: isi sedikit menaiki bagian foto yang sudah memudar,
		// supaya tidak ada garis pemisah yang kentara. Geseran pengguna menambah
		// dari sana.
		ContentTop:  px(-72 + header),
		CardTop:     px(cardTop),
		PhotoPx:     frameHeight,
		PhotoZoom:   strconv.FormatFloat(zoom, 'f', 3, 64),
		Align:       align,
		RuleMargin:  ruleMargin,
		TitleAlign:  titleAlign,
		Lang:        langAttr(req.Lang),
		Palette:     pal,
		HeroSize:    px(heroSize),
		ContextSize: px(max(contextMin, scaled(contextSize, req.Fonts.Title))),
		SmallSize:   px(26),
		Padding:     px(64),
		Theme:       theme,
	}
	if theme != nil {
		// Skala tipografi DESIGN-NEWSCARD §7.2: judul 72/700, isi 40/400.
		theme.TitlePx = px(scaled(72, req.Fonts.Title))
		theme.BodyPx = px(scaled(40, req.Fonts.Paragraph))
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, d); err != nil {
		return nil, Geometry{}, fmt.Errorf("could not compose the card: %v", err)
	}
	return buf.Bytes(), Geometry{Width: width, Height: height, PhotoTop: d.CardTop, PhotoH: frameHeight, PhotoBase: photoBase,
		Scale: scale, OffsetW: limitW, OffsetH: limitH}, nil
}

// themeFor = warna tampilan jadi untuk req (nil = tanpa tema, gaya lama).
// Dipakai kartu pertama DAN halaman lanjutan carousel (pages.go), jadi
// warnanya pasti sama.
func (b *Builder) themeFor(ctx context.Context, req Request, hasImage bool) *themeColors {
	a := req.Article
	if tc, ok := themeTable[req.Theme]; ok {
		if req.Theme == ThemePhoto && hasImage {
			if t := b.tone(ctx, a.Image); t.ok {
				p := paletteFor(t, true)
				tc = themeColors{Panel: p.Ink, Title: p.Paper, Body: p.Muted, Badge: p.Accent, OnBadge: p.OnAccent}
			}
		}
		tc.PanelRGB = rgbOf(tc.Panel)
		return &tc
	}
	// Warna bebas dari pemilih warna (kotak terakhir ThemePicker): panel =
	// warna itu, judul/lencana dihitung Look supaya tetap terbaca.
	if hex, ok := parseHex(req.Theme); ok {
		c, _ := Look(hex, nil)
		tc := themeColors{Panel: c.Panel, Title: c.Title, Body: c.Body, Badge: c.Badge, OnBadge: c.OnBadge}
		tc.PanelRGB = rgbOf(tc.Panel)
		return &tc
	}
	if req.Theme == ThemeAuto {
		tc := themeTable[ThemeDark]
		if hasImage {
			if c, ok := b.autoLook(ctx, a.Image); ok {
				tc = themeColors{Panel: c.Panel, Title: c.Title, Body: c.Body, Badge: c.Badge, OnBadge: c.OnBadge}
			}
		}
		tc.PanelRGB = rgbOf(tc.Panel)
		return &tc
	}
	return nil
}

// FontCSS menanam font bawaan proyek sebagai data URI.
//
// Ditanam, bukan dirujuk lewat path, karena Chrome bisa saja berjalan di sisi
// Windows sementara fontnya ada di sisi Linux — dan supaya kartu tampil sama
// persis di komputer siapa pun, terlepas dari font yang terpasang di sana.
func (b *Builder) FontCSS() template.CSS {
	b.once.Do(func() {
		css, _ := b.fontFaces(func(raw []byte) (string, error) {
			return "data:font/ttf;base64," + base64.StdEncoding.EncodeToString(raw), nil
		})
		b.fontCSS = template.CSS(css)
		sum := sha256.Sum256([]byte(b.fontCSS))
		b.fontKey = fmt.Sprintf("%x", sum[:8])
	})
	return b.fontCSS
}

// cardFonts = font kartu: berkas di fontsDir → nama family di CSS.
var cardFonts = [][2]string{{"Montserrat.ttf", "Clipper Sans"}, {"Anton.ttf", "Clipper Display"}, {"BebasNeue.ttf", "Clipper Condensed"}}

// fontFaces menyusun @font-face tiap font kartu; src mengubah isi berkas font
// jadi alamat url(): data URI (FontCSS, satu halaman mandiri) atau berkas
// sementara (lembar shootSheet — satu salinan untuk semua halaman). Font yang
// tidak ada dilewati: jatuh ke font sistem lewat daftar cadangan.
func (b *Builder) fontFaces(src func(raw []byte) (string, error)) (string, error) {
	var sb strings.Builder
	for _, f := range cardFonts {
		raw, err := os.ReadFile(filepath.Join(b.fontsDir, f[0]))
		if err != nil {
			continue
		}
		u, err := src(raw)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&sb, "@font-face{font-family:'%s';src:url(%s) format('truetype');font-weight:100 900;font-display:block}\n", f[1], u)
	}
	return sb.String(), nil
}

// FontKey = sidik jari CSS font: berubah hanya bila berkas fontnya berubah,
// jadi GUI boleh menyimpan /api/card/fonts?v=<kunci> di cache browser selamanya.
func (b *Builder) FontKey() string {
	b.FontCSS()
	return b.fontKey
}

// clampZoom menjaga zoom di rentang masuk akal. Nilai 0 datang dari permintaan
// lama yang belum mengenal field ini, jadi diartikan sebagai "tanpa zoom".
//
// Batas bawahnya 1: di bawah itu foto lebih kecil dari bingkainya dan menyisakan
// celah kosong di tepi kartu.
func clampZoom(z float64) float64 { return min(4, max(1, z)) }

// offsetLimit = sejauh mana foto boleh digeser tanpa memunculkan celah kosong.
// Pada zoom Z, foto jadi Z kali ukuran bingkai, jadi sisa ruangnya (Z-1)/2 di
// tiap sisi. Pada zoom 1 hasilnya 0 — foto pas bingkai, tak ada yang bisa
// digeser. GUI memakai rumus yang sama supaya seretannya berhenti di tempat
// yang sama dengan hasil render.
func offsetLimit(size int, zoom float64) int {
	return int(float64(size) * (zoom - 1) / 2)
}

func clamp(v, limit int) int { return min(limit, max(-limit, v)) }

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

// tmpl memakai html/template sehingga judul & ringkasan dari situs luar
// otomatis di-escape — teks berisi < atau & tidak bisa merusak tata letak,
// apalagi menyisipkan skrip.
var tmpl = template.Must(template.New("card").Parse(`<!doctype html>
<html lang="{{.Lang}}"><head><meta charset="utf-8"><style>
{{.FontCSS}}
*{margin:0;padding:0;box-sizing:border-box}
/* Latar & kertas mengambil rona dari FOTO artikelnya (lihat palette.go), jadi
   tiap berita membawa warnanya sendiri. Yang dipinjam hanya ronanya; terangnya
   dikunci palet supaya foto malam dan foto siang sama terbacanya.

   Penanda (garis & stempel sumber) ikut rona itu juga, tapi ditarik ke rentang
   pastel — bukan disamakan. Penanda yang persis sewarna latar akan lenyap.
   Kuning #E4B429 tetap jadi warna dasarnya: dipakai saat tidak ada rona yang
   bisa diikuti (foto hitam putih, atau warna pilihan tanpa rona seperti putih). */
:root{
  --ink:{{.Palette.Ink}};           /* latar, segelap tinta cetak */
  --paper:{{.Palette.Paper}};       /* kertas guntingan, ikut menghangat */
  --ink-on-paper:#1A1714;           /* hitam kehangatan, untuk teks di atas kertas */
  --highlight:{{.Palette.Accent}};  /* penanda: rona kartu, ditarik ke pastel */
}
html,body{width:{{.Width}}px;height:{{.Height}}px;overflow:hidden}
/* padding-top = pita kosong saat SELURUH kartu diturunkan. Kanvasnya tetap
   1080x1920 (box-sizing:border-box), jadi yang muncul di atas adalah latar
   kartu — bukan kartu yang mengecil. */
body{
  font-family:'Clipper Sans',"Segoe UI",Roboto,Arial,sans-serif;
  display:flex;flex-direction:column;padding-top:{{.CardTop}}px;
  {{if .Dark}}background:var(--ink);color:var(--paper)
  {{else}}background:{{.Palette.LightBg}};color:#1A1714{{end}}
}
/* Tinggi dalam PIKSEL, bukan persen: persen dihitung terhadap kotak yang sudah
   dikurangi pita atas, jadi menurunkan kartu malah memendekkan fotonya —
   padahal yang diminta foto yang sama, turun. */
/* flex-shrink 1: foto yang diseret memanjang (CardY) MENGALAH bila isi kartu
   tidak muat lagi — isi tidak pernah terpotong di dasar, sama dengan Mark. */
.photo{position:relative;height:{{.PhotoPx}}px;flex:0 1 auto;overflow:hidden;
  background:var(--ink)}
/* object-fit:cover dipakai—bukan min-width/min-height dengan width:auto—karena
   yang terakhir membuat penskalaan bergantung pada ukuran asli gambar relatif
   terhadap kotaknya. Akibatnya CSS yang sama memberi hasil berbeda di kotak
   1080px (render) dan kotak ±380px (pratinjau GUI). object-fit:cover selalu
   memenuhi bingkai dengan rasio terjaga, apa pun ukuran kotak dan gambarnya.

   Urutan translate lalu scale disengaja: jarak geser tidak ikut terkalikan zoom,
   sehingga menyeret N piksel di pratinjau = N piksel di hasil render. */
.photo img{
  position:absolute;left:50%;top:50%;
  width:100%;height:100%;object-fit:{{.PhotoFit}};display:block;
  transform:translate(-50%,-50%) translate({{.PhotoX}}px,{{.PhotoY}}px) scale({{.PhotoZoom}});
  transform-origin:center;
}
/* Mode "whole" memasukkan seluruh gambar apa pun rasionya, jadi hampir selalu
   ada ruang yang tidak terjangkau gambar. Isiannya salinan buram foto itu
   sendiri — bukan warna polos — supaya bingkai tetap terasa satu gambar utuh.
   Urutannya di HTML yang menentukan: isian ditulis SEBELUM gambar utamanya, jadi
   gambar utama menimpanya. Bukan z-index negatif — itu akan menjatuhkannya ke
   belakang latar .photo sendiri dan isiannya hilang sama sekali.
   Sengaja dibesarkan 1,1x: blur di tepi meninggalkan pinggiran pucat kalau
   gambarnya pas-pasan. */
.photo .fill{
  position:absolute;inset:0;width:100%;height:100%;object-fit:cover;
  transform:scale(1.1);filter:blur(48px) saturate(1.25) brightness(.8);
}
.photo::after{content:"";position:absolute;inset:0;
  background:linear-gradient(180deg,rgba(0,0,0,.4) 0%,rgba(0,0,0,0) 26%,
    {{if .Dark}}rgba({{.Palette.InkRGB}},0) 58%,rgba({{.Palette.InkRGB}},.9) 86%,var(--ink) 100%
    {{else}}rgba({{.Palette.LightBgRGB}},0) 58%,rgba({{.Palette.LightBgRGB}},.92) 86%,{{.Palette.LightBg}} 100%{{end}})}
/* Dengan foto: isi dijangkarkan ke bawah, sisa ruang jatuh ke bagian foto yang
   sudah memudar — tidak ada rongga menganga di bawah guntingan.
   Tanpa foto: dijangkarkan ke bawah justru menyisakan setengah kartu kosong di
   atas, jadi isinya ditaruh di tengah. */
/* Isi dijangkarkan ke ATAS, tepat di bawah foto. Jadi mengecilkan huruf membuat
   guntingan dan kaki kartu NAIK, bukan tenggelam ke bawah — arah yang sama
   dengan yang dilakukan pengguna pada penggesernya.
   Dulu dijangkarkan ke bawah supaya tidak ada rongga di bawah guntingan;
   ongkosnya rongga itu pindah ke bawah kartu, dan di sanalah UI aplikasi sosmed
   menutupinya — jadi ruang kosong justru lebih baik ada di situ.
   Tanpa foto: ditaruh di tengah, karena menjangkarkan ke atas menyisakan
   setengah kartu kosong di bawah. */
.content{flex:1;padding:0 {{.Padding}}px {{.Padding}}px;display:flex;flex-direction:column;
  position:relative;z-index:2;
  {{if .HasImage}}justify-content:flex-start;margin-top:{{.ContentTop}}px
  {{else}}justify-content:center;padding-top:{{.Padding}}px{{end}}}

/* Tanda mulai kutipan. Menandai di mana bahan orang lain mulai dikutip —
   bukan hiasan, dan itu sebabnya ia ikut berpindah mengikuti perataan. */
.rule{width:64px;height:8px;background:var(--highlight);margin:{{.RuleMargin}};flex:none}

/* Judul artikel turun pangkat jadi keterangan: huruf kondensa, kapital,
   direnggangkan. Ia menjelaskan kutipan di bawahnya, bukan bersaing dengannya. */
.context{font-family:'Clipper Condensed','Clipper Sans',sans-serif;
  font-size:{{.ContextSize}}px;line-height:1.12;letter-spacing:.045em;
  text-transform:uppercase;text-align:{{.TitleAlign}};margin-bottom:30px;
  /* Judul panjang dipangkas dua baris: perannya menjelaskan, bukan bersaing
     dengan kutipan. Tanpa batas ini judul 15 kata bisa mendorong guntingan
     keluar kartu. */
  display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;
  color:{{.Palette.Muted}}}

/* GUNTINGAN — inti desain ini. Kutipan disajikan sebagai potongan kertas yang
   ditempel, sebab yang dijual fitur ini adalah bukti: teksnya diambil apa
   adanya, bukan ditulis ulang. Tinta gelap di atas kertas terang juga pilihan
   paling terbaca di layar ponsel, dan keterbacaan itulah alasan mode kartu ada. */
.clipping{
  padding:{{.Padding}}px;border-radius:3px;
  transform:rotate(-.45deg);
{{if .BoxOff}}
  /* Tanpa kertas, teks duduk langsung di atas foto atau latar. Bayangan gelap
     ini yang menggantikan tugas kertas: menjaga teks tetap terbaca di atas foto
     ramai. Tanpa itu, menghilangkan kotak = kartu yang tidak bisa dibaca. */
  color:var(--paper);
  text-shadow:0 2px 18px rgba(0,0,0,.85),0 0 46px rgba(0,0,0,.7);
  padding:0;
{{else}}
  background:var(--paper);color:var(--ink-on-paper);
  box-shadow:0 20px 44px rgba(0,0,0,{{if .Dark}}.5{{else}}.22{{end}});
{{end}}
}
.hero{font-size:{{.HeroSize}}px;line-height:1.34;font-weight:600;
  letter-spacing:-.005em;text-align:{{.Align}};
  {{if .Quote}}font-family:'Clipper Display','Clipper Sans',Impact,sans-serif;
  font-weight:400;line-height:1.1;letter-spacing:0;{{end}}}

/* Stempel asal ditaruh DI ATAS guntingan, bukan di kaki kartu: pada selembar
   kutipan, dari mana ia berasal adalah bagian dari kutipannya sendiri. */
.stamp{margin-top:34px;font-family:'Clipper Condensed','Clipper Sans',sans-serif;
  font-size:{{.SmallSize}}px;letter-spacing:.13em;text-transform:uppercase;
  text-align:{{.Align}}}
/* Warna teks stempel mengikuti penandanya: sejak penanda memakai warna pilihan
   apa adanya, teks gelap tidak lagi selalu yang paling terbaca. */
.stamp span{background:var(--highlight);padding:5px 14px;border-radius:2px;
  color:{{.Palette.OnAccent}};box-decoration-break:clone;-webkit-box-decoration-break:clone}

{{with .Theme}}
/* Tampilan jadi (DESIGN-NEWSCARD §5.2): panel polos, judul besar di atas,
   ringkasan di bawahnya, lencana berwarna. Ditaruh terakhir supaya menimpa. */
body{background:{{.Panel}};color:{{.Body}}}
.photo{background:{{.Panel}}}
.photo::after{background:linear-gradient(180deg,rgba(0,0,0,.35) 0%,rgba(0,0,0,0) 26%,
  rgba({{.PanelRGB}},0) 60%,rgba({{.PanelRGB}},.92) 88%,{{.Panel}} 100%)}
.rule{background:{{.Badge}}}
.context{color:{{.Title}};font-family:'Clipper Sans',sans-serif;font-weight:700;
  font-size:{{.TitlePx}}px;line-height:1.22;letter-spacing:0;text-transform:none;
  -webkit-line-clamp:4}
.clipping{background:none;box-shadow:none;text-shadow:none;transform:none;padding:0;color:{{.Body}}}
.hero{font-size:{{.BodyPx}}px;font-weight:400;line-height:1.5;color:{{.Body}}}
.stamp span{background:{{.Badge}};color:{{.OnBadge}}}
{{end}}
</style></head><body>
{{if .HasImage}}
<div class="photo">
  {{if .PhotoFill}}<img class="fill" src="{{.Image}}" alt="">{{end}}
  <img src="{{.Image}}" alt="">
</div>
{{end}}
<div class="content">
  <div class="rule"></div>
  {{if .Context}}<div class="context">{{.Context}}</div>{{end}}
  <div class="clipping">
    {{if .Hero}}<div class="hero">{{.Hero}}</div>{{end}}
    <div class="stamp"><span>{{.Domain}}{{if .Date}} &middot; {{.Date}}{{end}}</span></div>
  </div>
</div>
</body></html>`))
