package news

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
)

// sampleRSS meniru kebiasaan nyata media Indonesia: gambar diselipkan
// sebagai <img> di dalam description (ANTARA), judul channel menaruh nama
// media di belakang, dan ada entitas HTML di judul.
const sampleRSS = `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0"><channel>
  <title>Berita Terkini - ANTARA News</title>
  <item>
    <title>Harga BBM naik &amp; warga mengeluh</title>
    <link>https://www.antaranews.com/berita/123/harga-bbm</link>
    <description><![CDATA[<img src="https://cdn.antaranews.com/foto.jpg" align="left"/>Pemerintah menaikkan harga.]]></description>
    <pubDate>Thu, 30 Jul 2026 19:51:57 +0700</pubDate>
  </item>
  <item>
    <title>Berita kedua</title>
    <link>https://www.antaranews.com/berita/124/kedua</link>
    <description>Tanpa gambar.</description>
    <pubDate>Thu, 30 Jul 2026 18:00:00 +0700</pubDate>
    <enclosure url="https://cdn.antaranews.com/enclosure.jpg"/>
  </item>
</channel></rss>`

func TestParseFeedReadsTitleImageAndDate(t *testing.T) {
	articles, err := parseFeed([]byte(sampleRSS), "ANTARA", 0, "en")
	if err != nil {
		t.Fatalf("parseFeed galat: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("jumlah artikel = %d, mau 2", len(articles))
	}

	a := articles[0]
	if a.Title != "Harga BBM naik & warga mengeluh" {
		t.Errorf("title = %q — entitas HTML seharusnya sudah diterjemahkan", a.Title)
	}
	// Gambar harus diambil dari <img> di dalam description.
	if a.Image != "https://cdn.antaranews.com/foto.jpg" {
		t.Errorf("image = %q, mau dari <img> di description", a.Image)
	}
	// Tag HTML tidak boleh ikut terbawa ke ringkasan.
	if a.Summary != "Pemerintah menaikkan harga." {
		t.Errorf("summary = %q — tag <img> seharusnya dibuang", a.Summary)
	}
	if a.Date != "Thursday, 30 July 2026" {
		t.Errorf("date = %q, mau %q", a.Date, "Thursday, 30 July 2026")
	}
	if a.Domain != "antaranews.com" {
		t.Errorf("domain = %q", a.Domain)
	}
	// <enclosure> dipakai bila description tidak memuat gambar.
	if articles[1].Image != "https://cdn.antaranews.com/enclosure.jpg" {
		t.Errorf("image item ke-2 = %q, mau dari enclosure", articles[1].Image)
	}
}

// Tanggal kartu mengikuti bahasa yang diminta: kartu untuk pembaca Indonesia
// harus bertanggal Indonesia walau antarmukanya berbahasa Inggris.
func TestFormatDateFollowsLanguage(t *testing.T) {
	const pub = "Thu, 30 Jul 2026 19:51:57 +0700"
	if got := formatDate(pub, "id"); got != "Kamis, 30 Juli 2026" {
		t.Errorf("formatDate(id) = %q, mau %q", got, "Kamis, 30 Juli 2026")
	}
	if got := formatDate(pub, "en"); got != "Thursday, 30 July 2026" {
		t.Errorf("formatDate(en) = %q, mau %q", got, "Thursday, 30 July 2026")
	}
	// Bahasa tak dikenal jatuh ke Inggris, bukan panik atau string kosong.
	if got := formatDate(pub, "zz"); got != "Thursday, 30 July 2026" {
		t.Errorf("formatDate(zz) = %q, mau jatuh ke Inggris", got)
	}
}

// Nama kurasi harus menang atas judul channel. Tanpa ini badge kartu ANTARA
// berbunyi "Berita Terkini", sebab nama medianya ada di belakang pemisah.
func TestParseFeedCuratedNameBeatsChannelTitle(t *testing.T) {
	articles, err := parseFeed([]byte(sampleRSS), "ANTARA", 0, "en")
	if err != nil {
		t.Fatal(err)
	}
	if articles[0].Source != "ANTARA" {
		t.Errorf("source = %q, mau %q", articles[0].Source, "ANTARA")
	}

	// Tanpa nama kurasi, tebakan dari judul channel dipakai apa adanya.
	articles, err = parseFeed([]byte(sampleRSS), "", 0, "en")
	if err != nil {
		t.Fatal(err)
	}
	if articles[0].Source == "" {
		t.Error("source kosong — seharusnya jatuh ke tebakan judul channel/domain")
	}
}

func TestParseFeedRespectsMaxLimit(t *testing.T) {
	articles, err := parseFeed([]byte(sampleRSS), "ANTARA", 1, "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Errorf("jumlah artikel = %d, mau dibatasi 1", len(articles))
	}
}

func TestParseFeedRejectsNonFeedContent(t *testing.T) {
	if _, err := parseFeed([]byte(`<html><body>not a feed</body></html>`), "", 0, "en"); err == nil {
		t.Error("mau galat untuk isi yang bukan feed, dapat nil")
	}
}

func TestTruncateStopsAtWordBoundary(t *testing.T) {
	got := truncate("satu dua tiga empat lima enam", 12)
	if got != "satu dua…" {
		t.Errorf("truncate = %q, mau %q", got, "satu dua…")
	}
	// Teks pendek tidak boleh diubah.
	if got := truncate("pendek", 50); got != "pendek" {
		t.Errorf("truncate teks pendek = %q", got)
	}
}

func TestMediaNameFallsBackToDomain(t *testing.T) {
	// Judul channel yang kepanjangan bukan nama media — pakai domain saja.
	long := "Portal Berita Terlengkap Dan Terpercaya Sepanjang Masa Sekali"
	if got := mediaName(long, "https://www.contoh.co.id/a"); got != "contoh.co.id" {
		t.Errorf("mediaName = %q, mau %q", got, "contoh.co.id")
	}
	// Pemisah "|" di depan: bagian pertama memang nama medianya.
	if got := mediaName("CNN Indonesia | Berita Terkini", "https://cnnindonesia.com/a"); got != "CNN Indonesia" {
		t.Errorf("mediaName = %q, mau %q", got, "CNN Indonesia")
	}
}

// readMeta adalah jalan masuk fitur "tempel link", jadi pola penulisan meta
// yang berbeda-beda antar situs harus tetap terbaca.
func TestReadMetaHandlesAttributeOrder(t *testing.T) {
	h := `<html><head>
	<meta property="og:title" content="Judul OG">
	<meta content="Ringkasan OG" name="og:description">
	<meta name='og:image' content='https://contoh.id/g.jpg'>
	<title>Judul Tag</title>
	</head><body><meta property="og:title" content="JANGAN DIPAKAI"></body></html>`

	m := readMeta(h)
	if m["og:title"] != "Judul OG" {
		t.Errorf("og:title = %q", m["og:title"])
	}
	// content sebelum name juga harus tertangkap.
	if m["og:description"] != "Ringkasan OG" {
		t.Errorf("og:description = %q", m["og:description"])
	}
	// Kutip satu sama sahnya dengan kutip dua.
	if m["og:image"] != "https://contoh.id/g.jpg" {
		t.Errorf("og:image = %q", m["og:image"])
	}
	if got := tagTitle(h); got != "Judul Tag" {
		t.Errorf("tagTitle = %q", got)
	}
}

// --- ekstraksi paragraf ---

const sampleArticle = `<html><body>
<nav><p>Beranda Ekonomi Olahraga Teknologi Nasional Internasional</p></nav>
<script>var iklan = "Pasang iklan di sini sekarang juga hubungi kami";</script>
<div class="isi">
  <p>Menteri Keuangan menyatakan penerimaan pajak tahun ini tumbuh 12 persen dibandingkan tahun lalu.</p>
  <p>Baca juga: Sepuluh cara menghemat uang belanja bulanan agar dompet tetap aman</p>
  <p>Singkat</p>
  <p>Angka itu disebut sebagai capaian tertinggi dalam lima tahun terakhir menurut laporan resmi kementerian.</p>
  <p>Menteri Keuangan menyatakan penerimaan pajak tahun ini tumbuh 12 persen dibandingkan tahun lalu.</p>
</div>
<footer><p>Copyright 2026 Redaksi Pedoman Media Siber Karier Kontak Kami Semua</p></footer>
</body></html>`

func TestParseParagraphsDropsMenusScriptsAndDuplicates(t *testing.T) {
	paragraphs := parseParagraphs(sampleArticle)
	if len(paragraphs) != 2 {
		for _, p := range paragraphs {
			t.Logf("[%d] %s", p.Index, p.Text)
		}
		t.Fatalf("jumlah paragraf = %d, mau 2", len(paragraphs))
	}
	if !strings.HasPrefix(paragraphs[0].Text, "Menteri Keuangan menyatakan") {
		t.Errorf("paragraf pertama = %q", paragraphs[0].Text)
	}
	// Nomor harus berurutan mulai 0 — inilah yang dipakai LLM untuk menunjuk.
	for i, p := range paragraphs {
		if p.Index != i {
			t.Errorf("index paragraf ke-%d = %d", i, p.Index)
		}
	}
	for _, p := range paragraphs {
		low := strings.ToLower(p.Text)
		if strings.Contains(low, "baca juga") || strings.Contains(low, "beranda") ||
			strings.Contains(low, "iklan") || strings.Contains(low, "copyright") {
			t.Errorf("blok sampah ikut terbawa: %q", p.Text)
		}
	}
}

func TestParseParagraphsRejectsEmptyPage(t *testing.T) {
	if paragraphs := parseParagraphs(`<html><body><p>Terlalu pendek</p></body></html>`); len(paragraphs) != 0 {
		t.Errorf("mau 0 paragraf, dapat %d", len(paragraphs))
	}
}

// --- tagar ---

func TestHashtagsOnlyAcceptWordsPresentInArticle(t *testing.T) {
	c := Content{Paragraphs: []Paragraph{{0, "Bank Indonesia menahan suku bunga acuan di Jakarta pekan ini."}}}
	got := c.Hashtags([]string{"Bank Indonesia", "Jakarta", "Bitcoin melambung"}, 8)
	want := []string{"#BankIndonesia", "#Jakarta"}
	if len(got) != len(want) {
		t.Fatalf("hashtags = %v, mau %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("hashtags[%d] = %q, mau %q", i, got[i], want[i])
		}
	}
}

func TestHashtagsRespectMaxCount(t *testing.T) {
	c := Content{Paragraphs: []Paragraph{{0, "satu dua tiga empat lima"}}}
	if got := c.Hashtags([]string{"satu", "dua", "tiga", "empat"}, 2); len(got) != 2 {
		t.Errorf("hashtags = %v, mau dibatasi 2", got)
	}
}

// --- build: pagar pengaman terhadap balasan LLM ---

func sampleContent() Content {
	return Content{
		Article: Article{Title: "Uji", URL: "https://contoh.id/a"},
		Paragraphs: []Paragraph{
			{0, "Paragraf nol berisi kalimat pembuka berita yang cukup panjang."},
			{1, "Paragraf satu berisi angka mengejutkan sebesar dua belas persen."},
		},
	}
}

// rankingReply merangkai satu entri "rankings" pada balasan LLM.
func rankingReply(index int, score float64, reason string) struct {
	Index  int     `json:"index"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
} {
	return struct {
		Index  int     `json:"index"`
		Score  float64 `json:"score"`
		Reason string  `json:"reason"`
	}{Index: index, Score: score, Reason: reason}
}

func TestBuildTakesTextFromArticleNotFromLLM(t *testing.T) {
	r := reply{Card: 1, Caption: 0}
	r.Rankings = append(r.Rankings, rankingReply(1, 9, "has a number"))

	s := build(sampleContent(), r, "test")
	if s.Rankings[0].Text != "Paragraf satu berisi angka mengejutkan sebesar dua belas persen." {
		t.Errorf("text = %q — harus diambil dari artikel", s.Rankings[0].Text)
	}
	if s.Card != 1 || s.Caption != 0 {
		t.Errorf("card=%d caption=%d", s.Card, s.Caption)
	}
}

func TestBuildIgnoresOutOfRangeIndexes(t *testing.T) {
	r := reply{Card: 99, Caption: 99}
	for _, ix := range []int{0, 42} { // 42 tidak ada
		r.Rankings = append(r.Rankings, rankingReply(ix, 5, ""))
	}
	s := build(sampleContent(), r, "test")
	// Nomor 42 dibuang, tapi kedua paragraf artikel tetap muncul: yang tak
	// dinilai LLM dilengkapi heuristik supaya semuanya bisa dipilih pengguna.
	if len(s.Rankings) != 2 {
		t.Fatalf("rankings = %d, mau 2 (semua paragraf artikel)", len(s.Rankings))
	}
	for _, item := range s.Rankings {
		if item.Index == 42 {
			t.Error("nomor 42 tidak ada di artikel — seharusnya dibuang")
		}
	}
	// Card/caption di luar jangkauan jatuh ke peringkat teratas, bukan menebak.
	// Caption lalu digeser agar tidak kembar dengan card.
	if s.Card != 0 {
		t.Errorf("card = %d, mau peringkat teratas (0)", s.Card)
	}
	if _, ok := sampleContent().TextAt(s.Caption); !ok || s.Caption == s.Card {
		t.Errorf("caption = %d — harus paragraf sah yang berbeda dari card", s.Caption)
	}
}

// Model lokal kerap membalas "rankings": [] padahal card/caption-nya sah.
// Dulu ini ditolak mentah-mentah; sekarang harus tetap menghasilkan pilihan.
func TestBuildStillWorksWhenRankingsEmpty(t *testing.T) {
	s := build(sampleContent(), reply{Card: 1, Caption: 0}, "test")
	if len(s.Rankings) != 2 {
		t.Fatalf("rankings = %d, mau 2 dari heuristik", len(s.Rankings))
	}
	for _, item := range s.Rankings {
		if item.Source != SourceHeuristic {
			t.Errorf("source = %q, mau %q", item.Source, SourceHeuristic)
		}
		if item.Text == "" {
			t.Error("text kosong — harus diambil dari artikel")
		}
	}
	if s.Card != 1 || s.Caption != 0 {
		t.Errorf("card=%d caption=%d — nomor sah dari LLM harus dihormati", s.Card, s.Caption)
	}
	if s.Note == "" {
		t.Error("note kosong — penggantian tidak boleh senyap")
	}
}

// Nomor card/caption ngawur + rankings kosong: tetap harus memberi hasil,
// jatuh ke paragraf berperingkat teratas.
func TestBuildNeverFailsOnNonsenseReply(t *testing.T) {
	s := build(sampleContent(), reply{Card: 99, Caption: -3}, "test")
	if len(s.Rankings) == 0 {
		t.Fatal("rankings kosong — fitur tidak boleh gagal total")
	}
	if _, ok := sampleContent().TextAt(s.Card); !ok {
		t.Errorf("card = %d, bukan nomor paragraf yang ada", s.Card)
	}
	if _, ok := sampleContent().TextAt(s.Caption); !ok {
		t.Errorf("caption = %d, bukan nomor paragraf yang ada", s.Caption)
	}
}

// Paragraf berkutipan langsung & berangka harus menang atas paragraf
// penyambung yang diawali kata rujukan.
func TestHookScorePrefersQuotesAndNumbers(t *testing.T) {
	strong := Paragraph{0, `"Kerugian negara mencapai 1,2 triliun rupiah," kata Ketua KPK dalam konferensi pers di Jakarta hari ini.`}
	weak := Paragraph{5, "Sementara itu, ia menambahkan bahwa proses masih terus berjalan sebagaimana mestinya di lapangan."}
	strongScore, _ := hookScore(strong, 10)
	weakScore, _ := hookScore(weak, 10)
	if strongScore <= weakScore {
		t.Errorf("skor kutipan+angka (%.1f) harus di atas paragraf penyambung (%.1f)", strongScore, weakScore)
	}
}

// Model lokal cenderung menjawab paragraf 0 untuk card sekaligus caption.
// Kalau dibiarkan, teks kartu dan captionnya kembar.
func TestBuildKeepsCaptionDifferentFromCard(t *testing.T) {
	s := build(sampleContent(), reply{Card: 0, Caption: 0}, "test")
	if s.Card != 0 {
		t.Errorf("card = %d, mau 0 (pilihan LLM dihormati)", s.Card)
	}
	if s.Caption == s.Card {
		t.Error("caption kembar dengan card — seharusnya digeser ke peringkat berikutnya")
	}
	if _, ok := sampleContent().TextAt(s.Caption); !ok {
		t.Errorf("caption = %d, bukan paragraf yang ada di artikel", s.Caption)
	}
}

// Artikel satu paragraf tidak punya pilihan lain — kembar diterima apa adanya.
func TestBuildAllowsDuplicateWhenOnlyOneParagraph(t *testing.T) {
	c := Content{Paragraphs: []Paragraph{{0, "Satu-satunya paragraf yang ada di artikel pendek ini."}}}
	s := build(c, reply{Card: 0, Caption: 0}, "test")
	if s.Card != 0 || s.Caption != 0 {
		t.Errorf("card=%d caption=%d, mau 0 keduanya", s.Card, s.Caption)
	}
}

// Sebagian situs menulis og:url dengan entitas HTML menempel di ujungnya.
// Diteruskan mentah, alamat hasilnya membawa sampah itu dan berujung 404.
func TestParseArticleCleansEntitiesInOgURL(t *testing.T) {
	h := `<html><head>
	<meta property="og:title" content="Cegah Narkoba dari Keluarga">
	<meta property="og:url" content="https://contoh.go.id/Cegah_Narkoba&nbsp;&nbsp;">
	<meta property="og:image" content="https://contoh.go.id/foto.jpg&nbsp;">
	</head><body></body></html>`
	u, _ := url.Parse("https://news.google.com/rss/articles/CBMiabc")
	a, err := parseArticle(h, u, "en")
	if err != nil {
		t.Fatal(err)
	}
	if a.URL != "https://contoh.go.id/Cegah_Narkoba" {
		t.Errorf("url = %q — entitas di ujung harus dibuang", a.URL)
	}
	if a.Image != "https://contoh.go.id/foto.jpg" {
		t.Errorf("image = %q — entitas di ujung harus dibuang", a.Image)
	}
	// og:url menang atas alamat pengalih yang dipakai untuk membuka halaman.
	if a.Domain != "contoh.go.id" {
		t.Errorf("domain = %q, mau contoh.go.id", a.Domain)
	}
}

// Resolve harus MURAH untuk tautan biasa: dikembalikan apa adanya, tanpa
// browser, tanpa cache.
//
// Itu yang membuatnya aman dipanggil di depan SETIAP pengambilan artikel
// (api.newsArticle). Kalau suatu saat ia mulai menuntut browser untuk tautan
// biasa, jalur tempel-link akan mati di komputer tanpa Chrome — dan matinya
// hanya terlihat saat dijalankan, bukan saat dikompilasi.
func TestResolvePassesOrdinaryLinksThrough(t *testing.T) {
	for _, link := range []string{
		"https://bacaini.id/misteri-995-senjata-api.html",
		"http://contoh.go.id/berita/1",
	} {
		got, err := Resolve(context.Background(), link, nil, t.TempDir())
		if err != nil {
			t.Fatalf("Resolve(%s): %v", link, err)
		}
		if got != link {
			t.Errorf("Resolve(%s) = %q — tautan biasa tidak boleh diubah", link, got)
		}
	}
	// Tautan Google TANPA browser harus berkata jelas apa yang kurang, bukan
	// mengembalikan alamat Google yang akan terbaca sebagai halaman Google.
	_, err := Resolve(context.Background(), "https://news.google.com/rss/articles/CBMiabc", nil, t.TempDir())
	if err == nil {
		t.Error("tautan Google tanpa browser seharusnya galat, bukan diteruskan mentah")
	}
}

// Gambar dicari berlapis: og:image → JSON-LD → <img> di badan artikel.
func TestArticleImageFallbacks(t *testing.T) {
	u, _ := url.Parse("https://contoh.id/berita/1")

	// 1. Tanpa og:image, JSON-LD yang dipakai.
	h := `<html><head><meta property="og:title" content="Judul">
	<script type="application/ld+json">{"@type":"NewsArticle","image":["https://contoh.id/foto.jpg"]}</script>
	</head><body></body></html>`
	a, err := parseArticle(h, u, "en")
	if err != nil {
		t.Fatal(err)
	}
	if a.Image != "https://contoh.id/foto.jpg" {
		t.Errorf("JSON-LD: image = %q", a.Image)
	}

	// 2. JSON-LD rusak (template Blogger yang gagal) TIDAK boleh dipakai, dan
	//    <img> di badan artikel yang menggantikannya.
	h = `<html><head><meta property="og:title" content="Judul">
	<script type="application/ld+json">{"@type":"NewsArticle",
	"image":["<!--Can't find substitution for tag [post.featuredImage.jsonEscaped]-->"]}</script>
	</head><body>
	<img src='https://contoh.id/logo-situs.png' alt='logo'>
	<div class='post-body entry-content'><p>isi</p><img src='https://contoh.id/isi.jpg'></div>
	</body></html>`
	a, err = parseArticle(h, u, "en")
	if err != nil {
		t.Fatal(err)
	}
	// Logo di luar badan artikel TIDAK boleh menang.
	if a.Image != "https://contoh.id/isi.jpg" {
		t.Errorf("badan artikel: image = %q, mau isi.jpg (bukan logo halaman)", a.Image)
	}

	// 3. Artikel yang memang tidak berfoto tetap kosong — kartu tanpa foto lebih
	//    baik daripada kartu berlogo situs.
	h = `<html><head><meta property="og:title" content="Judul"></head><body>
	<img src='https://contoh.id/logo-situs.png' alt='logo'>
	<div class='post-body entry-content'><p>isi tanpa gambar</p></div>
	</body></html>`
	a, err = parseArticle(h, u, "en")
	if err != nil {
		t.Fatal(err)
	}
	if a.Image != "" {
		t.Errorf("tanpa foto: image = %q, harus kosong", a.Image)
	}
}

// Paragraf yang dibungkus <div> DENGAN tag sebaris di dalamnya harus terbaca.
//
// Bentuk ini dipakai Blogspot: satu <div> per paragraf, dan hampir semuanya
// memuat tautan atau penebalan. Pola lama menuntut teks polos (`[^<]{60,}`),
// jadi halaman dengan ribuan karakter tulisan terbaca sebagai NOL paragraf —
// dan tombol Analyse tidak punya bahan apa pun (tabloidlugas.com, 7 Agu 2026).
func TestParagraphsInsideDivsWithInlineTags(t *testing.T) {
	h := `<html><body>
	<nav><div>Menu Beranda Berita Olahraga Ekonomi Politik Nasional Internasional Hiburan</div></nav>
	<div class='post-body entry-content'>
	  <div><br /></div>
	  <div><b>Oleh: Mahar Prastowo</b></div>
	  <div>Angka itu terlalu besar untuk diabaikan dan angka itu terus disebut berulang kali oleh banyak pihak.</div>
	  <div>Begitu jumlah yang disebut ditemukan di lingkungan sebuah yayasan sekolah swasta di <a href="#">Kebayoran Lama</a> menurut keterangan resmi kepolisian.</div>
	</div>
	<div class='post-footer'><div>Baca juga berita lain yang sedang ramai dibicarakan pembaca hari ini juga</div></div>
	</body></html>`

	ps := parseParagraphs(h)
	if len(ps) != 2 {
		t.Fatalf("paragraf = %d, mau 2: %+v", len(ps), ps)
	}
	// Paragraf dengan <a> di dalamnya HARUS ikut, dan tautannya jadi teks biasa.
	if !strings.Contains(ps[1].Text, "Kebayoran Lama") {
		t.Errorf("teks tautan hilang: %q", ps[1].Text)
	}
	// Menu di luar badan artikel TIDAK boleh ikut walau cukup panjang.
	for _, p := range ps {
		if strings.Contains(p.Text, "Menu Beranda") {
			t.Errorf("menu ikut terbaca sebagai paragraf: %q", p.Text)
		}
		if strings.Contains(p.Text, "Baca juga") {
			t.Errorf("kaki artikel ikut terbaca: %q", p.Text)
		}
	}
}

// Lencana sumber harus memberitahu pembaca DARI MANA beritanya.
func TestSiteBadge(t *testing.T) {
	for _, c := range []struct{ name, host, want string }{
		// Nama yang masih mengenali situsnya sendiri: dipakai apa adanya.
		{"Kompas.com", "kompas.com", "Kompas.com"},
		{"CNN Indonesia", "cnnindonesia.com", "CNN Indonesia"},
		{"Bacaini.id", "bacaini.id", "Bacaini.id"},
		{"Republika Online", "republika.co.id", "Republika Online"},
		// Branding musiman yang tidak menyebut situsnya: pakai domainnya.
		{"LUGAS 28th", "tabloidlugas.com", "tabloidlugas.com"},
		{"", "contoh.id", "contoh.id"},
		// Label pendek tidak boleh cocok secara kebetulan.
		{"Something Idea", "id.com", "id.com"},
	} {
		if got := siteBadge(c.name, c.host); got != c.want {
			t.Errorf("siteBadge(%q, %q) = %q, mau %q", c.name, c.host, got, c.want)
		}
	}
}

// Tanggal dicari sampai ke JSON-LD dan <time> — sebagian media tidak memasang
// meta tanggal sama sekali, dan kartu tanpa tanggal terlihat seperti bug.
func TestPublishedDateFallbacks(t *testing.T) {
	u, _ := url.Parse("https://contoh.id/berita/1")

	h := `<html><head><meta property="og:title" content="Judul">
	<script type="application/ld+json">{"@type":"NewsArticle",
	"datePublished":"2026-08-07T00:58:00+07:00","dateModified":"2026-08-09T10:00:00+07:00"}</script>
	</head><body></body></html>`
	a, err := parseArticle(h, u, "en")
	if err != nil {
		t.Fatal(err)
	}
	// datePublished yang dipakai, BUKAN dateModified.
	if !strings.HasPrefix(a.Published, "2026-08-07") {
		t.Errorf("JSON-LD: published = %q, mau 2026-08-07…", a.Published)
	}
	if a.Date == "" {
		t.Error("Date kosong padahal tanggalnya terbaca")
	}

	h = `<html><head><meta property="og:title" content="Judul"></head><body>
	<time class='published' datetime='2026-08-07T00:58:00+07:00'>7 Agustus</time>
	</body></html>`
	a, err = parseArticle(h, u, "en")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a.Published, "2026-08-07") {
		t.Errorf("<time>: published = %q", a.Published)
	}
}

// Balasan batchexecute bukan JSON utuh: ada penjaga anti-hijack di baris
// pertama, dan alamatnya bersembunyi di dalam JSON yang berbentuk TEKS.
// Contoh di bawah disalin apa adanya dari balasan sungguhan.
func TestParseGarturlres(t *testing.T) {
	body := ")]}'\n\n" +
		`[["wrb.fr","Fbv4je","[\"garturlres\",\"https://megapolitan.kompas.com/read/2026/08/06/18140221/kronologi\",1]",null,null,null,"generic"],["di",11],["af.httprm",10,"-397",14]]` + "\n"
	got, err := parseGarturlres([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://megapolitan.kompas.com/read/2026/08/06/18140221/kronologi"; got != want {
		t.Errorf("url = %q, want %q", got, want)
	}
	if _, err := parseGarturlres([]byte(")]}'\n\n[[\"di\",11]]\n")); err == nil {
		t.Error("balasan tanpa alamat harus jadi galat, bukan string kosong")
	}
}

func TestGoogleArticleID(t *testing.T) {
	cases := map[string]string{
		"https://news.google.com/rss/articles/CBMiabc?oc=5": "CBMiabc",
		"https://news.google.com/articles/CBMiabc":          "CBMiabc",
		"https://news.google.com/read/CBMiabc?hl=id":        "CBMiabc",
		"https://kompas.com/read/2026/08/06/judul":          "",
	}
	for link, want := range cases {
		if got := googleArticleID(link); got != want {
			t.Errorf("googleArticleID(%q) = %q, want %q", link, got, want)
		}
	}
}

// Nol hasil pencarian tidak boleh dilaporkan sebagai feed yang rusak: pengguna
// mengetik kata kunci, bukan alamat feed (dilaporkan 7 Agustus 2026).
func TestEmptyFeedIsItsOwnError(t *testing.T) {
	empty := []byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>x</title></channel></rss>`)
	_, err := parseFeed(empty, "", 5, "id")
	if !errors.Is(err, errNoArticles) {
		t.Fatalf("galat = %v, mau errNoArticles", err)
	}
}

// TestIsJunkPemberitahuanLisensi menjaga temuan 18 Agustus 2026: kaki artikel
// Antara lolos penyaring, keluar sebagai FAKTA dari tahap 1, dan tertulis di
// artikel jadi — "Dilarang keras mengambil konten, tetapi informasi tentang
// Paskibraka dapat diperoleh dari sumber-sumber resmi."
func TestIsJunkPemberitahuanLisensi(t *testing.T) {
	junk := []string{
		"Dilarang keras mengambil konten, melakukan crawling atau pengindeksan otomatis untuk AI di situs web ini tanpa izin tertulis dari Kantor Berita ANTARA.",
		"Pewarta: M. Hilal Eka Saputra Harahap Editor: Suryanto Copyright © ANTARA 2026",
		"Baca juga: Sejarah singkat dan perbedaan paskibra serta paskibraka",
	}
	for _, s := range junk {
		if !isJunk(s) {
			t.Errorf("lolos padahal bukan isi berita: %q", s)
		}
	}
	// Kalimat berita biasa TIDAK boleh ikut terbuang.
	ok := []string{
		"Paskibraka bertugas mengibarkan bendera pusaka pada upacara kenegaraan.",
		"Polisi menyatakan pelaku ditahan karena membangun rumah tanpa izin dari pemerintah daerah.",
	}
	for _, s := range ok {
		if isJunk(s) {
			t.Errorf("kalimat berita ikut dibuang: %q", s)
		}
	}
}
