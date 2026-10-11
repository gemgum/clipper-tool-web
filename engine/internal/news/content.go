package news

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Paragraph satu blok teks dari badan artikel.
//
// Index-lah yang dikirim ke dan diterima dari LLM. Dengan begitu LLM tidak
// pernah memegang kesempatan menulis teks sendiri: ia hanya menunjuk nomor,
// dan engine yang mengambil kalimat aslinya. Verbatim terjamin oleh bentuk
// datanya, bukan oleh kepatuhan model pada instruksi.
type Paragraph struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

// Content = artikel beserta badan teksnya yang sudah dipecah jadi paragraf.
type Content struct {
	Article    Article     `json:"article"`
	Paragraphs []Paragraph `json:"paragraphs"`
	WordCount  int         `json:"word_count"`
}

// minParagraphWords = ambang panjang sebuah blok agar dianggap paragraf berita.
// Di bawah ini biasanya menu, label, keterangan foto, atau tombol berbagi.
const minParagraphWords = 12

// Browser membuka satu halaman di browser dan mengembalikan DOM akhirnya —
// setelah skrip halaman selesai berjalan. Dibuat berupa fungsi supaya paket
// news tidak perlu mengenal paket capture; lapisan api yang merangkainya.
type Browser func(ctx context.Context, url string) (string, error)

// FetchContent membaca artikel sekaligus badan teksnya.
//
// Metodenya: ambil isi <p> lebih dulu, sebab hampir semua media Indonesia
// menaruh badan berita di sana. Bila hasilnya terlalu sedikit (ada situs yang
// memakai <div> per paragraf), dicoba lagi dengan <div> berisi teks saja.
//
// Tautan hasil pencarian Google News ditangani berbeda: ia bukan alamat
// artikel melainkan pengalih yang hanya jalan lewat JavaScript, jadi harus
// dibuka pakai browser. Sekali buka, DOM-nya sudah memuat alamat asli, tag og:,
// dan badan tulisannya sekaligus — tidak perlu unduh kedua kali.
func FetchContent(ctx context.Context, page string, browse Browser, cacheDir, lang string) (Content, error) {
	if IsGoogleNewsLink(page) {
		// Tautan yang pernah diresolusi (mis. saat pengguna menyalinnya) cukup
		// diambil dari cache, lalu diproses lewat jalur HTTP biasa — jauh lebih
		// cepat daripada memanggil browser untuk kedua kalinya.
		if original, ok := loadResolved(cacheDir, page); ok {
			page = original
		} else if original, err := decodeGoogleNews(ctx, page); err == nil && !IsGoogleNewsLink(original) {
			// Sama seperti Resolve: alamat aslinya ditanyakan langsung ke
			// Google, dan artikelnya diunduh lewat HTTP biasa di bawah.
			saveResolved(cacheDir, page, original)
			page = original
		} else {
			if browse == nil {
				return Content{}, fmt.Errorf("search-result links must be opened in a browser, but no browser is available: install Chrome/Chromium, or open the link yourself and paste the real address")
			}
			dom, err := browse(ctx, page)
			if err != nil {
				return Content{}, err
			}
			content, err := contentFromHTML(dom, page, lang)
			if err == nil {
				saveResolved(cacheDir, page, content.Article.URL)
			}
			return content, err
		}
	}

	art, err := FetchArticle(ctx, page, lang)
	if err != nil {
		return Content{}, err
	}
	raw, err := download(ctx, art.URL)
	if err != nil {
		return Content{}, err
	}
	return buildContent(art, string(raw))
}

// contentFromHTML membangun Content dari satu HTML utuh (hasil browser).
func contentFromHTML(htmlStr, origin, lang string) (Content, error) {
	u, err := url.Parse(origin)
	if err != nil {
		return Content{}, fmt.Errorf("invalid URL: %v", err)
	}
	art, err := parseArticle(htmlStr, u, lang)
	if err != nil {
		return Content{}, err
	}
	// Kalau alamatnya masih menunjuk Google News, berarti pengalihannya belum
	// terjadi — jangan diteruskan, sebab yang terbaca halaman Google, bukan
	// artikelnya.
	if IsGoogleNewsLink(art.URL) {
		return Content{}, fmt.Errorf("the link has not reached the original article yet: try again, or open it in a browser and paste the address that appears")
	}
	return buildContent(art, htmlStr)
}

func buildContent(art Article, htmlStr string) (Content, error) {
	paragraphs := parseParagraphs(htmlStr)
	if len(paragraphs) == 0 {
		return Content{}, fmt.Errorf(
			"the article body could not be read from %s: the page may be paywalled, or its content is loaded via JavaScript. "+
				"Try another article, or copy the paragraphs yourself", art.Domain)
	}
	words := 0
	for _, p := range paragraphs {
		words += len(strings.Fields(p.Text))
	}
	return Content{Article: art, Paragraphs: paragraphs, WordCount: words}, nil
}

var (
	reP    = regexp.MustCompile(`(?is)<p\b[^>]*>(.*?)</\s*p\s*>`)
	reDiv  = regexp.MustCompile(`(?is)<div\b[^>]*>([^<]{60,})</\s*div\s*>`)
	reBody = regexp.MustCompile(`(?is)<body\b[^>]*>(.*)</\s*body\s*>`)
	// Blok paragraf yang isinya teks + tag SEBARIS saja. Karena isinya dibatasi
	// begitu, blok yang memuat <div>/<p> lain tidak akan cocok — hasilnya selalu
	// blok terdalam, bukan pembungkus yang memuat seluruh halaman.
	reInlineBlock = regexp.MustCompile(
		`(?is)<(?:p|div|li)\b[^>]*>((?:[^<]|</?(?:a|b|i|u|em|strong|span|br|small|mark|sup|sub|code|abbr|font)\b[^>]*>)*)</\s*(?:p|div|li)\s*>`)

	// Blok yang isinya tidak pernah jadi badan berita — dibuang lebih dulu
	// beserta isinya, supaya menu & skrip tidak terbaca sebagai paragraf.
	//
	// Satu regex per tag karena RE2 (mesin regexp Go) tidak mendukung
	// backreference, jadi `<(a|b)>...</\1>` tidak bisa dipakai.
	reStrip = blockTagPatterns(
		"script", "style", "noscript", "nav", "header", "footer",
		"aside", "form", "figcaption", "iframe",
	)
)

func blockTagPatterns(tags ...string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(tags))
	for _, t := range tags {
		out = append(out, regexp.MustCompile(`(?is)<`+t+`\b[^>]*>.*?</\s*`+t+`\s*>`))
	}
	return out
}

// junkPhrases = penanda blok yang jelas bukan isi berita meski panjangnya cukup.
// Isinya frasa bahasa Indonesia karena media yang dibaca berbahasa Indonesia.
var junkPhrases = []string{
	"baca juga", "simak juga", "lihat juga", "berita terkait", "artikel terkait",
	"copyright", "hak cipta", "all rights reserved", "editor:", "penyunting:",
	"pewarta:", "ikuti kami", "berlangganan", "unduh aplikasi", "advertisement",
	"cookie", "kebijakan privasi", "syarat dan ketentuan",
	// Pemberitahuan lisensi di kaki artikel. "copyright"/"hak cipta" tidak
	// menangkapnya — Antara menulisnya sebagai kalimat larangan penuh:
	// "Dilarang keras mengambil konten, melakukan crawling atau pengindeksan
	// otomatis untuk AI di situs web ini tanpa izin tertulis dari Kantor Berita
	// ANTARA." Ia lolos ke tahap 1 dan keluar sebagai FAKTA, lalu tertulis di
	// artikel jadi ("Dilarang keras mengambil konten, tetapi informasi tentang
	// Paskibraka dapat diperoleh…", 18 Agustus 2026).
	"dilarang keras mengambil", "dilarang mengambil konten", "dilarang memperbanyak",
	"pengindeksan otomatis", "tanpa izin tertulis dari", "hak kekayaan intelektual",
}

// parseParagraphs memecah HTML jadi paragraf badan berita.
func parseParagraphs(h string) []Paragraph {
	// Dipersempit ke BADAN ARTIKEL lebih dulu bila penandanya dikenali
	// (post-body, entry-content, article-body, itemprop=articleBody). Di dalam
	// sana, blok apa pun boleh dianggap paragraf tanpa takut menangkap menu
	// atau daftar berita lain — dan itu yang membuat lapisan ketiga di bawah
	// aman dipakai.
	if body := articleBodyHTML(h); body != "" {
		h = body
	} else if m := reBody.FindStringSubmatch(h); len(m) == 2 {
		h = m[1]
	}
	for _, re := range reStrip {
		h = re.ReplaceAllString(h, " ")
	}

	candidates := extractBlocks(h, reP)
	if len(candidates) < 2 {
		// Blok yang isinya teks + tag SEBARIS (<a>, <b>, <span>, …).
		//
		// Inilah bentuk yang dipakai Blogspot: tiap paragraf satu <div>, dan
		// hampir semuanya memuat tautan atau penebalan. reDiv di bawah menuntut
		// teks polos, jadi tidak satu pun cocok — halaman dengan 4.759 karakter
		// tulisan terbaca sebagai NOL paragraf (tabloidlugas.com, 7 Agu 2026).
		//
		// Yang membuatnya tidak menangkap pembungkus besar: isinya dibatasi ke
		// tag sebaris saja, jadi blok yang memuat <div> lain tidak cocok — yang
		// tersisa selalu blok terdalam.
		candidates = append(candidates, extractBlocks(h, reInlineBlock)...)
	}
	if len(candidates) < 2 {
		// Situs yang memakai <div> per paragraf. Sengaja hanya <div> yang
		// isinya teks polos (tanpa tag anak) agar tidak menangkap pembungkus
		// besar yang memuat seluruh halaman.
		candidates = append(candidates, extractBlocks(h, reDiv)...)
	}

	var out []Paragraph
	seen := map[string]bool{}
	for _, t := range candidates {
		if len(strings.Fields(t)) < minParagraphWords || seen[t] {
			continue
		}
		if isJunk(t) {
			continue
		}
		seen[t] = true
		out = append(out, Paragraph{Index: len(out), Text: t})
	}
	return out
}

func extractBlocks(h string, re *regexp.Regexp) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(h, -1) {
		if len(m) < 2 {
			continue
		}
		t := clean(stripTags(m[1]))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func isJunk(t string) bool {
	low := strings.ToLower(t)
	for _, f := range junkPhrases {
		if strings.Contains(low, f) {
			return true
		}
	}
	return false
}

// Join menyatukan paragraf jadi satu teks — dipakai untuk memeriksa apakah
// kata kunci pilihan LLM benar-benar muncul di artikel.
func (c Content) Join() string {
	b := make([]string, 0, len(c.Paragraphs))
	for _, p := range c.Paragraphs {
		b = append(b, p.Text)
	}
	return strings.Join(b, "\n")
}

// Numbered menuliskan paragraf dengan nomornya, siap dikirim ke LLM.
func (c Content) Numbered(maxWords int) string {
	var sb strings.Builder
	words := 0
	for _, p := range c.Paragraphs {
		n := len(strings.Fields(p.Text))
		// Artikel yang sangat panjang dipotong agar muat di konteks model lokal;
		// paragraf awal berita hampir selalu memuat inti (piramida terbalik).
		if maxWords > 0 && words+n > maxWords {
			break
		}
		words += n
		fmt.Fprintf(&sb, "[%d] %s\n\n", p.Index, p.Text)
	}
	return strings.TrimSpace(sb.String())
}

// TextAt mengembalikan paragraf pada indeks tertentu. ok=false bila di luar
// jangkauan — LLM sesekali menyebut nomor yang tidak ada.
func (c Content) TextAt(index int) (string, bool) {
	for _, p := range c.Paragraphs {
		if p.Index == index {
			return p.Text, true
		}
	}
	return "", false
}

// Hashtags mengubah kata kunci jadi tagar.
//
// Hanya kata kunci yang BENAR-BENAR muncul di artikel yang diterima; sisanya
// dibuang. Ini menutup satu-satunya celah mengarang yang tersisa, sebab tagar
// tidak bisa berupa kutipan utuh seperti paragraf.
func (c Content) Hashtags(keywords []string, max int) []string {
	body := strings.ToLower(c.Join())
	var out []string
	seen := map[string]bool{}
	for _, k := range keywords {
		k = strings.TrimSpace(k)
		if k == "" || !strings.Contains(body, strings.ToLower(k)) {
			continue
		}
		t := "#" + toHashtag(k)
		if len(t) < 4 || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

var reNonLetter = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// toHashtag menyatukan kata jadi satu tagar ber-huruf besar di tiap kata:
// "Jakarta Barat" → "JakartaBarat".
func toHashtag(s string) string {
	parts := reNonLetter.Split(s, -1)
	var sb strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		r := []rune(p)
		sb.WriteString(strings.ToUpper(string(r[0])))
		sb.WriteString(string(r[1:]))
	}
	return sb.String()
}

// SortRankings mengurutkan hasil penilaian dari skor tertinggi.
func SortRankings(r []Ranking) {
	sort.SliceStable(r, func(a, b int) bool { return r[a].Score > r[b].Score })
}

// articleBodyHTML memotong HTML jadi badan artikelnya saja, "" bila penandanya
// tidak dikenali.
//
// Penandanya sama dengan yang dipakai bodyOf (article.go) — satu daftar
// untuk dua keperluan, supaya gambar dan teks tidak pernah diambil dari dua
// wilayah yang berbeda.
func articleBodyHTML(h string) string {
	m := reArticleBody.FindStringIndex(h)
	if m == nil {
		return ""
	}
	body := h[m[0]:]
	if end := reBodyEnd.FindStringIndex(body); end != nil {
		body = body[:end[0]]
	}
	return body
}
