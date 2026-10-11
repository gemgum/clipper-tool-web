package card

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"

	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// Warna kartu diturunkan dari FOTO artikelnya, bukan dari warna situs asalnya.
//
// Warna situs terdengar lebih masuk akal, tapi tidak ada: dari lima media besar
// Indonesia yang diperiksa (detik, kumparan, CNN Indonesia, Kompas, Tempo) hanya
// kumparan yang menulis <meta name="theme-color">. Empat dari lima kartu akan
// jatuh ke warna bawaan — persis keseragaman yang mau dihilangkan. Fotonya
// selalu ada, dan warnanya memang milik berita itu sendiri.
//
// Yang diambil hanya RONA dan KEPEKATANNYA, tidak pernah terangnya. Terang
// ditetapkan sendiri oleh palet supaya latar selalu cukup gelap dan kertas
// selalu cukup terang — foto malam dan foto siang harus sama terbacanya.

// tone = rona khas sebuah foto. Dipisah dari palette karena satu foto bisa
// dipakai di gaya gelap maupun terang, sedangkan tone-nya sama.
type tone struct {
	hue float64 // 0..360
	sat float64 // 0..1
	ok  bool    // false = foto tidak berwarna (hitam putih / kelabu)
	// exact = rona ini DIPILIH pengguna, bukan ditebak dari foto.
	//
	// Bedanya ada di batas bawah kepekatan. Untuk foto, batas bawah 0,12 mencegah
	// foto yang nyaris kelabu menghasilkan kartu yang terlihat rusak abu-abu —
	// kita tidak tahu apakah itu memang maunya. Untuk warna pilihan, kita tahu:
	// pengguna melihat contekannya lalu menekannya. Memaksakan batas bawah di
	// sana justru membuat pilihan netral & monokromatik tidak berpengaruh sama
	// sekali, dan itulah yang tadi terbaca sebagai bug.
	exact bool
	// pick = warna yang benar-benar ditekan pengguna, apa adanya.
	//
	// Penanda memakai warna ini PERSIS, tidak diturunkan: yang ditekan pengguna
	// adalah warna yang ia mau lihat. Bagian lain (latar, kertas, teks pendukung)
	// tetap diturunkan dari rona & kepekatannya, sebab terangnya harus dikunci
	// demi keterbacaan — dan itu tidak berlaku untuk penanda, yang justru harus
	// menonjol.
	pick string
}

// floor mengembalikan batas bawah kepekatan yang berlaku untuk tone ini.
func (t tone) floor(photo float64) float64 {
	if t.exact {
		return 0
	}
	return photo
}

// ceil mengembalikan batas atas kepekatan.
//
// Untuk foto batasnya rapat: rona diambil dari gambar orang lain, dan satu
// spanduk merah menyala tidak boleh membuat latar berteriak lebih keras dari
// isinya. Untuk warna pilihan batasnya longgar — kalau tidak, "neon" dan "jewel"
// sama-sama mentok di angka yang sama dan menghasilkan kartu yang identik.
// Contekan yang hasilnya kembar sama saja dengan tombol kosong.
func (t tone) ceil(photo, exact float64) float64 {
	if t.exact {
		return exact
	}
	return photo
}

// palette = warna yang benar-benar ditulis ke CSS.
type palette struct {
	Ink        string // latar kartu gaya gelap
	InkRGB     string // "20,23,28" — gradasi butuh komponennya, bukan hex
	Paper      string // guntingan kertas
	LightBg    string // latar kartu gaya terang
	LightBgRGB string
	Muted      string // judul-keterangan di atas latar
	Faint      string // kaki kartu
	Accent     string // garis penanda & stempel sumber
	OnAccent   string // teks di atas penanda — gelap atau terang, mana yang terbaca
}

// Palet bawaan = warna kartu sebelum fitur ini ada. Dipakai apa adanya saat
// fotonya tidak berwarna atau tidak bisa diambil, jadi kartu tanpa rona yang
// jelas tetap terlihat seperti kartu yang memang dirancang begitu.
var (
	defaultDark = palette{
		Ink:        "#14171C",
		InkRGB:     "20,23,28",
		Paper:      "#EFEBE1",
		LightBg:    "#DDD8CC",
		LightBgRGB: "221,216,204",
		Muted:      "#9AA3AD",
		Faint:      "#79818B",
		Accent:     baseAccent,
		OnAccent:   inkOnAccent,
	}
	defaultLight = palette{
		Ink:        "#14171C",
		InkRGB:     "20,23,28",
		Paper:      "#FBF9F4",
		LightBg:    "#DDD8CC",
		LightBgRGB: "221,216,204",
		Muted:      "#5C5750",
		Faint:      "#6B665E",
		Accent:     baseAccent,
		OnAccent:   inkOnAccent,
	}
)

// baseAccent = kuning penanda template dasar. Dipakai saat tidak ada rona yang
// bisa diikuti: foto hitam putih, atau warna pilihan yang tak punya rona
// (putih/abu/hitam).
const baseAccent = "#E4B429"

// accentMinLum = kecerahan terkecil yang masih sanggup memikul teks gelap.
//
// Stempel sumber menulis teks #1A1714 di atas penanda, dan atribusi adalah janji
// fitur ini — kalau ia tidak terbaca, fiturnya gagal di titik yang paling
// penting. 0,45 memberi kontras sekitar 8:1 terhadap teks itu.
const accentMinLum = 0.45

// accentFor menurunkan warna penanda dari rona kartu.
//
// Bukan disamakan dengan warna kartunya: penanda yang persis sewarna latar akan
// lenyap. Yang dicari warna yang COCOK — rona sama, ditarik ke rentang pastel.
//
// Terangnya TIDAK bisa ditetapkan sebagai angka HSL. "Lightness" HSL bukan
// kecerahan yang terlihat: kuning di 55% terang benderang, biru di 55% gelap.
// Menetapkan satu angka akan membuat penanda biru gagal memikul teks gelap
// sementara penanda kuning kelewat pucat. Karena itu terangnya dinaikkan
// selangkah demi selangkah sampai LUMINANSI sungguhannya cukup — pagar yang
// berlaku untuk seluruh lingkaran warna, bukan untuk rona yang kebetulan diuji.
func accentFor(t tone) string {
	if !t.ok {
		return baseAccent
	}
	// Warna yang ditekan pengguna dipakai APA ADANYA. Menurunkannya jadi versi
	// lain berarti contekan yang ia lihat bukan warna yang ia dapat — dan itulah
	// yang membuat merah menyala keluar sebagai merah bata.
	if t.pick != "" {
		return t.pick
	}
	s := between(t.sat*0.7, t.floor(0.28), t.ceil(0.50, 0.70))
	l := 0.72
	for ; l < 0.95; l += 0.02 {
		if luminance(toRGB(t.hue, s, l)) >= accentMinLum {
			break
		}
	}
	return hex(t.hue, s, l)
}

// Swatches = daftar warna kartu yang boleh dipilih pengguna.
//
// Sengaja daftar tertutup, bukan pemilih spektrum penuh. Engine hanya memakai
// RONA dari warna pilihan — terangnya dikunci palet supaya teks selalu terbaca —
// jadi pemilih spektrum menjanjikan sesuatu yang tidak pernah dikerjakan:
// memilih putih atau abu-abu tidak mengubah apa pun, dan itu terbaca sebagai
// bug. Daftar ini hanya berisi warna yang benar-benar berpengaruh.
//
// Yang ditampilkan adalah warna PENANDA yang akan dihasilkan — jadi contekan
// yang dilihat pengguna memang sepotong hasilnya, bukan janji terpisah.
// Tiap baris satu keluarga. Urutannya sengaja tidak diberi nama di GUI: yang
// perlu dilihat pengguna warnanya, bukan istilahnya.
//
// Setiap warna harus BERPENGARUH — kalau dua contekan menghasilkan kartu yang
// sama, salah satunya cuma tombol kosong. Karena engine memakai rona dan
// kepekatan (terang dikunci demi keterbacaan), tiap baris dibedakan lewat salah
// satu dari keduanya:
//
//   - lima baris pertama menyapu RONA, dengan kepekatan khas keluarganya;
//   - monokromatik menyapu KEPEKATAN pada satu rona — itulah arti monokromatik,
//     dan sekaligus satu-satunya cara membuat dua belas kelabu berbeda satu sama
//     lain di mata engine.
func Swatches() [][]string {
	const n = 12
	row := func(f func(i int) string) []string {
		out := make([]string, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, f(i))
		}
		return out
	}
	hue := func(i int) float64 { return float64(i) * 360 / n }

	return [][]string{
		// Pastel — lembut, terang.
		row(func(i int) string { return hex(hue(i), 0.45, 0.80) }),
		// Earth tone — hangat & teredam, jadi ronanya dibatasi ke busur tanah:
		// tanah liat, oker, zaitun. Merah muda atau biru bukan warna tanah.
		row(func(i int) string { return hex(15+float64(i)*9, 0.42, 0.42) }),
		// Neon — kepekatan penuh.
		row(func(i int) string { return hex(hue(i), 1.00, 0.55) }),
		// Jewel tone — pekat tapi gelap: zamrud, safir, delima.
		row(func(i int) string { return hex(hue(i), 0.72, 0.34) }),
		// Netral — kelabu bernuansa: hangat, dingin, kehijauan.
		row(func(i int) string { return hex(hue(i), 0.10, 0.62) }),
		// Monokromatik — satu rona, kepekatan menaik.
		row(func(i int) string { return hex(220, float64(i)*0.04, 0.55) }),
		// Hitam-putih — hitam pekat sampai putih, langkahnya rata seperti baris
		// lain supaya jumlahnya sama.
		//
		// Yang membedakan kedua belasnya di mata engine adalah PENANDA, sebab
		// penanda memakai warna pilihan apa adanya. Latarnya sendiri tidak ikut
		// berubah: terang latar dikunci demi keterbacaan. Kartu yang benar-benar
		// putih didapat dengan memasangkan warna terang ini ke gaya Terang.
		row(func(i int) string { return hex(0, 0, float64(i)/(n-1)) }),
	}
}

// Teks di atas penanda: satu gelap, satu terang. Yang dipakai yang kontrasnya
// lebih besar.
const (
	inkOnAccent   = "#1A1714"
	paperOnAccent = "#FBF9F4"
)

// onAccentFor memilih warna teks stempel sumber.
//
// Sejak penanda memakai warna pilihan APA ADANYA, teks gelap tidak lagi selalu
// menang: merah menyala #FF1A1A menyisakan kontras 4,3 untuk teks gelap dan
// hanya 3,8 untuk teks terang. Membiarkannya tetap gelap membuat atribusi susah
// dibaca di sebagian warna, dan atribusi adalah janji fitur ini.
//
// Titik terburuknya ada di luminansi ±0,20, tempat kedua pilihan sama-sama
// memberi sekitar 4,05 — itu batas matematisnya, bukan kelalaian. Teks stempel
// berukuran besar (26 px di kanvas 1080), jadi angka itu masih di atas ambang
// WCAG untuk teks besar.
func onAccentFor(accent string) string {
	if contrastOf(accent, inkOnAccent) >= contrastOf(accent, paperOnAccent) {
		return inkOnAccent
	}
	return paperOnAccent
}

// contrastOf mengukur kontras dua warna hex menurut WCAG.
//
// Luminansi kedua warna teks DIHITUNG, bukan ditulis sebagai tetapan. Sempat
// ditulis (0,0114 dan 1,0), dan angka 1,0 itu salah: kertas #FBF9F4 luminansinya
// 0,95, bukan putih murni. Selisih kecil itu menggeser titik peralihan dan
// membuat satu warna memilih teks yang justru kurang terbaca.
func contrastOf(a, b string) float64 {
	la, lb := lumOf(a), lumOf(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func lumOf(hexStr string) float64 {
	h, ok := parseHex(hexStr)
	if !ok {
		return 0
	}
	n, _ := strconv.ParseUint(h[1:], 16, 32)
	return luminance(int(n>>16)&0xFF, int(n>>8)&0xFF, int(n)&0xFF)
}

// luminance = luminansi relatif menurut WCAG, dari komponen 0..255.
func luminance(r, g, b int) float64 {
	f := func(v int) float64 {
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*f(r) + 0.7152*f(g) + 0.0722*f(b)
}

// paletteFor menurunkan seluruh warna kartu dari satu rona.
//
// Kepekatan dijepit di kedua ujung dengan alasan berbeda. Batas bawah supaya
// foto yang nyaris kelabu tidak menghasilkan kartu yang terlihat "rusak abu-abu"
// alih-alih berwarna; batas atas supaya foto dengan spanduk merah menyala tidak
// menghasilkan latar yang berteriak lebih keras dari isinya.
func paletteFor(t tone, dark bool) palette {
	if !t.ok {
		if dark {
			return defaultDark
		}
		return defaultLight
	}
	h := t.hue
	// Terang dikunci di angka palet bawaan (latar 9,4%, kertas 91%), jadi yang
	// berpindah dari kartu ke kartu hanya ronanya.
	// Batas ATAS tetap berlaku untuk keduanya: ia menjaga latar tidak berteriak
	// lebih keras dari isinya. Yang berbeda cuma batas bawahnya — untuk warna
	// pilihan, kepekatan rendah memang boleh sampai nol.
	inkSat := between(t.sat, t.floor(0.12), t.ceil(0.32, 0.80))
	lightSat := between(t.sat*0.30, t.floor(0.06), t.ceil(0.18, 0.30))
	p := palette{
		Ink:     hex(h, inkSat, 0.094),
		Paper:   hex(h, between(t.sat*0.35, t.floor(0.08), t.ceil(0.20, 0.35)), 0.910),
		LightBg: hex(h, lightSat, 0.830),
		Muted:   hex(h, between(t.sat*0.30, t.floor(0.10), t.ceil(0.10, 0.22)), 0.640),
		Faint:   hex(h, between(t.sat*0.25, t.floor(0.08), t.ceil(0.08, 0.18)), 0.510),
	}
	if !dark {
		// Di gaya terang kertas duduk di atas latar terang, jadi keduanya harus
		// lebih pucat; teks pendukung justru harus lebih gelap agar terbaca.
		p.Paper = hex(h, between(t.sat*0.25, t.floor(0.05), t.ceil(0.14, 0.26)), 0.965)
		p.Muted = hex(h, between(t.sat*0.25, t.floor(0.07), t.ceil(0.07, 0.16)), 0.340)
		p.Faint = hex(h, between(t.sat*0.20, t.floor(0.06), t.ceil(0.06, 0.14)), 0.400)
	}
	p.Accent = accentFor(t)
	p.OnAccent = onAccentFor(p.Accent)
	p.InkRGB = rgbList(h, inkSat, 0.094)
	p.LightBgRGB = rgbList(h, lightSat, 0.830)
	return p
}

// parseHex membaca "#RRGGBB" (atau "RRGGBB") jadi bentuk baku bertanda pagar.
//
// Hanya enam digit yang diterima. Bentuk singkat tiga digit sengaja ditolak
// daripada ditebak: pengguna yang mengetik separuh warna lebih baik tahu
// sekarang daripada menemukannya di kartu yang sudah terbit.
func parseHex(s string) (string, bool) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "#"))
	if len(s) != 6 {
		return "", false
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("#%06X", n), true
}

// toneOfHex menurunkan rona dari satu warna pilihan pengguna, supaya warna
// manual melewati jalur yang sama persis dengan warna dari foto. Satu warna
// karenanya cukup untuk menyetel latar, kertas, dan teks pendukung sekaligus.
//
// Warna kelabu (kepekatan di bawah ambang yang sama dengan foto) diperlakukan
// sama juga: jatuh ke palet bawaan, bukan dipaksa jadi rona yang tidak ada.
func toneOfHex(s string) tone {
	h, ok := parseHex(s)
	if !ok {
		return tone{}
	}
	n, _ := strconv.ParseUint(h[1:], 16, 32)
	r, g, b := float64((n>>16)&0xFF)/255, float64((n>>8)&0xFF)/255, float64(n&0xFF)/255
	hue, sat, _ := toHSL(r, g, b)
	return tone{hue: hue, sat: sat, ok: true, exact: true, pick: h}
}

// toneOf membaca rona khas sebuah foto.
//
// Piksel yang paling banyak di sebuah foto biasanya justru yang tidak punya
// rona: bayangan, sorot lampu, langit putih. Karena itu piksel yang terlalu
// gelap, terlalu terang, atau terlalu pucat dibuang dulu — kalau tidak, hampir
// semua foto akan "dominan abu-abu" dan hasilnya seragam lagi.
//
// Sisanya dikelompokkan per rona, dan tiap piksel menyumbang sebesar
// kepekatannya sendiri: satu jaket merah menyala lebih menentukan kesan sebuah
// foto daripada seluas apa pun dinding krem di belakangnya.
func toneOf(img image.Image) tone {
	const buckets = 24 // 15 derajat per kelompok
	var (
		sumSin, sumCos [buckets]float64
		sumSat         [buckets]float64 // sekaligus bobot kelompok: piksel pekat menyumbang lebih besar
		count          [buckets]int
		sampled, kept  int
	)

	b := img.Bounds()
	// Foto CDN bisa 2000 px lebih. Dicicip di kisi ±120x120 supaya biayanya tetap
	// sama untuk foto sekecil apa pun besarnya — rona tidak butuh tiap piksel.
	stepX := max(1, b.Dx()/120)
	stepY := max(1, b.Dy()/120)

	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			r16, g16, b16, a16 := img.At(x, y).RGBA()
			sampled++
			if a16 < 0x8000 { // piksel tembus pandang tidak ikut terlihat
				continue
			}
			h, s, l := toHSL(float64(r16)/65535, float64(g16)/65535, float64(b16)/65535)
			if l < 0.12 || l > 0.92 || s < 0.18 {
				continue
			}
			kept++
			i := int(h/360*buckets) % buckets
			rad := h * math.Pi / 180
			sumSin[i] += s * math.Sin(rad)
			sumCos[i] += s * math.Cos(rad)
			sumSat[i] += s
			count[i]++
		}
	}

	// Foto hitam putih, dokumen, atau grafik kelabu: tidak ada rona yang jujur
	// bisa diambil. Ambangnya 2% supaya satu logo kecil berwarna di pojok tidak
	// menentukan warna seluruh kartu.
	if sampled == 0 || kept*50 < sampled {
		return tone{}
	}

	// Satu keluarga warna jarang jatuh rapi dalam satu kelompok: rona hangat
	// sebuah ruangan bisa tersebar di 0-45 derajat, lalu kalah oleh satu bilah
	// sempit yang pekat. Karena itu tiap kelompok dinilai bersama tetangganya —
	// yang dicari adalah keluarga warna, bukan potongan 15 derajat.
	score := func(i int) float64 {
		left, right := (i+buckets-1)%buckets, (i+1)%buckets
		return sumSat[i] + (sumSat[left]+sumSat[right])/2
	}
	best := 0
	for i := 1; i < buckets; i++ {
		if score(i) > score(best) {
			best = i
		}
	}
	if count[best] == 0 {
		return tone{}
	}
	// Rata-rata melingkar: rona itu sudut, jadi 350 dan 10 harus bertemu di 0,
	// bukan di 180.
	h := math.Atan2(sumSin[best], sumCos[best]) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return tone{hue: h, sat: between(sumSat[best]/float64(count[best]), 0, 1), ok: true}
}

// fetchImage mengunduh foto artikel untuk dibaca ronanya.
//
// Ini pengunduhan KEDUA foto yang sama — Chrome mengunduhnya sendiri saat
// merender. Disengaja: menyuntikkan foto sebagai data URI akan membuat halaman
// kartu membengkak dan menyulitkan penelusuran saat rendernya bermasalah,
// sementara ongkos unduhan kedua ini kecil dan hasilnya di-cache per alamat.
func fetchImage(ctx context.Context, url string) (image.Image, error) {
	// Unggahan pengguna datang sebagai data URI (lihat render): dibaca di tempat.
	if rest, ok := strings.CutPrefix(url, "data:"); ok {
		_, b64, found := strings.Cut(rest, ";base64,")
		if !found {
			return nil, fmt.Errorf("data URI is not base64")
		}
		raw, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, err
		}
		img, _, err := image.Decode(bytes.NewReader(raw))
		return img, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// CDN yang melayani "format otomatis" (mis. Cloudinary f_auto) mengirim WebP
	// bila diminta apa saja. Go tidak bisa membaca WebP tanpa dependency luar,
	// jadi formatnya diminta terang-terangan.
	req.Header.Set("Accept", "image/jpeg,image/png;q=0.9,*/*;q=0.5")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; clipper/1.0)")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("photo returned HTTP %d", res.StatusCode)
	}
	img, _, err := image.Decode(io.LimitReader(res.Body, 12<<20))
	return img, err
}

// tone mengambil rona foto artikel, dengan hasil disimpan per alamat.
//
// Kegagalan TIDAK dianggap error: kartu tetap terbit dengan palet bawaan. Foto
// yang tidak terbaca ronanya bukan alasan untuk menggagalkan kartu — Chrome
// masih bisa menampilkan fotonya, dan hanya warna latarnya yang jadi bawaan.
func (b *Builder) tone(ctx context.Context, url string) tone {
	if url == "" {
		return tone{}
	}
	// Data URI bisa berukuran megabyte: kunci cache-nya sidik jarinya saja.
	key := url
	if strings.HasPrefix(url, "data:") {
		sum := sha256.Sum256([]byte(url))
		key = fmt.Sprintf("data:%x", sum)
	}
	b.mu.Lock()
	if t, seen := b.tones[key]; seen {
		b.mu.Unlock()
		return t
	}
	b.mu.Unlock()

	var t tone
	if img, err := fetchImage(ctx, url); err == nil {
		t = toneOf(img)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	// Batas kasar supaya server yang hidup berhari-hari tidak menumpuk alamat
	// tanpa henti. Dibuang seluruhnya, bukan satu per satu: daftar ini murah
	// diisi ulang, dan LRU di sini menambah rumit tanpa manfaat.
	if len(b.tones) > 64 {
		b.tones = nil
	}
	if b.tones == nil {
		b.tones = make(map[string]tone)
	}
	b.tones[key] = t
	return t
}

// --- konversi warna ---

// toHSL memisahkan rona, kepekatan, dan terang dari sebuah warna RGB (0..1).
func toHSL(r, g, bl float64) (h, s, l float64) {
	maxc := math.Max(r, math.Max(g, bl))
	minc := math.Min(r, math.Min(g, bl))
	l = (maxc + minc) / 2
	d := maxc - minc
	if d == 0 {
		return 0, 0, l // kelabu murni: ronanya tidak ada, bukan nol
	}
	if l > 0.5 {
		s = d / (2 - maxc - minc)
	} else {
		s = d / (maxc + minc)
	}
	switch maxc {
	case r:
		h = math.Mod((g-bl)/d, 6)
	case g:
		h = (bl-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, s, l
}

// toRGB mengembalikan komponen 0..255 dari sebuah warna HSL.
func toRGB(h, s, l float64) (int, int, int) {
	if s <= 0 {
		v := round(l * 255)
		return v, v, v
	}
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return round((r + m) * 255), round((g + m) * 255), round((b + m) * 255)
}

func hex(h, s, l float64) string {
	r, g, b := toRGB(h, s, l)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func rgbList(h, s, l float64) string {
	r, g, b := toRGB(h, s, l)
	return fmt.Sprintf("%d,%d,%d", r, g, b)
}

func between(v, lo, hi float64) float64 {
	return math.Min(hi, math.Max(lo, v))
}

func round(v float64) int {
	return int(math.Round(math.Min(255, math.Max(0, v))))
}
