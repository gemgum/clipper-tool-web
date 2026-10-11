package news

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// FetchArticle membaca satu artikel dari URL yang ditempel pengguna dan
// mengembalikan bahan untuk kartu.
//
// Sumber datanya adalah tag Open Graph (og:title, og:image, …) — bukan isi
// badan halaman. Alasannya: og: memang dipasang media supaya tautannya tampil
// rapi saat dibagikan ke media sosial, jadi isinya sudah berupa judul, ringkasan
// dan gambar utama yang mereka pilih sendiri. Menebaknya dari badan HTML jauh
// lebih rapuh dan berbeda-beda tiap situs.
func FetchArticle(ctx context.Context, page, lang string) (Article, error) {
	u, err := url.Parse(strings.TrimSpace(page))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Article{}, fmt.Errorf("invalid URL: paste a full link starting with https://")
	}
	raw, err := download(ctx, u.String())
	if err != nil {
		return Article{}, err
	}
	return parseArticle(string(raw), u, lang)
}

// parseArticle membaca metadata dari HTML yang sudah di tangan. Dipisah dari
// FetchArticle supaya bisa dipakai juga untuk DOM hasil browser (tautan Google
// News), yang tidak melewati pengunduhan biasa.
func parseArticle(htmlStr string, u *url.URL, lang string) (Article, error) {
	meta := readMeta(htmlStr)
	title := firstNonEmpty(meta["og:title"], meta["twitter:title"], tagTitle(htmlStr))
	if title == "" {
		return Article{}, fmt.Errorf("no title found at %s: that page may not be an article, or it loads its content via JavaScript", domain(u.String()))
	}
	// og:url menyebut alamat kanonik artikel. Untuk halaman hasil resolusi
	// Google News, inilah satu-satunya tempat alamat aslinya muncul.
	address := u
	// clean() wajib di sini: sebagian situs menulis og:url dengan entitas HTML
	// menempel di ujungnya (mis. "…/artikel&nbsp;&nbsp;"). Diteruskan mentah,
	// alamat yang dihasilkan ikut membawa sampah itu dan berujung 404.
	if canon := clean(firstNonEmpty(meta["og:url"], meta["twitter:url"])); canon != "" {
		if abs, err := u.Parse(canon); err == nil && abs.Host != "" {
			address = abs
		}
	}
	summary := firstNonEmpty(meta["og:description"], meta["twitter:description"], meta["description"])
	// Gambar dicari berlapis, dari yang paling bisa dipercaya.
	//
	// og:image adalah tempat bakunya, tapi tidak semua situs mengisinya —
	// tabloidlugas.com (Blogspot) tidak punya sama sekali, dan kartunya jadi
	// tanpa foto (dilaporkan 7 Agustus 2026). Dua lapis di bawahnya menutup
	// sebagian besar sisanya:
	//
	//  2. JSON-LD schema.org `NewsArticle.image` — dipakai luas oleh media, dan
	//     sering terisi justru ketika og:image kosong.
	//  3. <img> pertama DI DALAM badan artikel. Dibatasi ke badan artikel dengan
	//     sengaja: di tingkat halaman, gambar pertama hampir selalu logo situs
	//     atau iklan. Pada halaman tabloidlugas itu badannya memuat NOL <img>,
	//     jadi lapisan ini benar-benar mengembalikan kosong — dan kosong memang
	//     jawaban yang benar untuk artikel yang tidak berfoto.
	//
	// Images (DESIGN-NEWSCARD §5.2, "Foto"): SEMUA kandidat yang layak, urut
	// dari yang paling bisa dipercaya — pengguna memilih salah satunya di GUI.
	// Image tetap yang pertama, jadi perilaku lama tidak berubah.
	images := articleImages(htmlStr, address,
		clean(firstNonEmpty(meta["og:image"], meta["og:image:url"], meta["twitter:image"])),
		clean(jsonLDImage(htmlStr)))
	image := ""
	if len(images) > 0 {
		image = images[0]
	}
	// Tanggal juga berlapis. Dua sumber terakhir yang baru: sebagian media
	// (termasuk Blogspot) tidak memasang meta tanggal sama sekali, tapi
	// menuliskannya di JSON-LD `datePublished` ATAU di `<time datetime="…">` —
	// keduanya baku, dan tanpa keduanya kartu terbit tanpa tanggal.
	published := firstNonEmpty(
		meta["article:published_time"], meta["og:updated_time"], meta["date"],
		jsonLDDate(htmlStr), timeTagDate(htmlStr),
	)

	return Article{
		Title:     clean(title),
		Summary:   truncate(clean(summary), 300),
		URL:       address.String(),
		Image:     image,
		Images:    images,
		Source:    siteBadge(clean(meta["og:site_name"]), domain(address.String())),
		Domain:    domain(address.String()),
		Date:      formatDate(published, lang),
		Published: rfc3339(published),
	}, nil
}

// reMeta menangkap satu tag <meta>. Atribut property/name bisa muncul sebelum
// atau sesudah content, jadi tagnya ditangkap utuh lalu dibedah terpisah.
var (
	reMeta     = regexp.MustCompile(`(?is)<meta\s+[^>]*>`)
	reProperty = regexp.MustCompile(`(?is)\b(?:property|name|itemprop)\s*=\s*["']([^"']+)["']`)
	reContent  = regexp.MustCompile(`(?is)\bcontent\s*=\s*["']([^"']*)["']`)
	reTitle    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reHead     = regexp.MustCompile(`(?is)</head>`)
)

// readMeta mengumpulkan seluruh tag meta jadi peta kunci→isi.
func readMeta(h string) map[string]string {
	// Cukup pindai bagian <head>; badan artikel bisa sangat panjang dan tidak
	// pernah berisi tag og:.
	if loc := reHead.FindStringIndex(h); loc != nil {
		h = h[:loc[1]]
	}
	out := map[string]string{}
	for _, tag := range reMeta.FindAllString(h, -1) {
		k := reProperty.FindStringSubmatch(tag)
		v := reContent.FindStringSubmatch(tag)
		if len(k) != 2 || len(v) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(k[1]))
		if _, exists := out[key]; exists {
			continue // pakai yang pertama; duplikat biasanya kurang spesifik
		}
		out[key] = v[1]
	}
	return out
}

func tagTitle(h string) string {
	if m := reTitle.FindStringSubmatch(h); len(m) == 2 {
		return m[1]
	}
	return ""
}

// jsonLDImage membaca `image` dari blok schema.org di halaman.
//
// Bentuknya beragam — string, daftar string, atau objek ImageObject dengan
// field `url` — jadi yang diambil adalah alamat http(s) PERTAMA yang muncul di
// dalam nilai `image`. Itu juga yang menyaring isian rusak: template Blogger
// yang gagal menghasilkan gambar menulis
// `"<!--Can't find substitution for tag [post.featuredImage.jsonEscaped]-->"`
// di sana, dan itu bukan alamat, jadi ditolak dengan sendirinya.
func jsonLDImage(h string) string {
	for _, m := range reLDBlock.FindAllStringSubmatch(h, -1) {
		if !strings.Contains(m[1], "\"image\"") {
			continue
		}
		// Dipotong mulai dari kunci "image" supaya alamat lain di blok yang sama
		// (logo penerbit, foto penulis) tidak ikut terambil.
		seg := m[1][strings.Index(m[1], "\"image\""):]
		if u := reLDURL.FindStringSubmatch(seg); len(u) == 2 {
			return strings.ReplaceAll(u[1], "\\/", "/")
		}
	}
	return ""
}

// maxImages: batas kandidat foto per artikel. Lebih dari ini cuma memenuhi
// baris pilihan foto dengan gambar sisipan yang tidak pernah dipilih.
const maxImages = 8

// articleImages menyusun daftar foto: og/twitter, JSON-LD, lalu <img> di
// badan artikel. Alamat dijadikan absolut, kembar dibuang, ikon/logo/piksel
// pelacak dan gambar ber-width/height < 100 dilewati.
func articleImages(h string, base *url.URL, meta, ld string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(src string) {
		src = clean(src)
		if src == "" || strings.HasPrefix(src, "data:") || strings.HasPrefix(src, "#") || len(out) >= maxImages {
			return
		}
		abs, err := base.Parse(src)
		if err != nil || (abs.Scheme != "http" && abs.Scheme != "https") || seen[abs.String()] {
			return
		}
		seen[abs.String()] = true
		out = append(out, abs.String())
	}
	add(meta)
	add(ld)
	body := bodyOf(h)
	for _, tag := range reImgFull.FindAllString(body, -1) {
		m := reImgTag.FindStringSubmatch(tag)
		if m == nil || tinyOrIcon(tag, m[1]) {
			continue
		}
		add(m[1])
	}
	return out
}

var (
	reImgFull = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	reImgDim  = regexp.MustCompile(`(?is)\b(?:width|height)\s*=\s*["']?(\d+)`)
	reIconish = regexp.MustCompile(`(?i)(logo|icon|sprite|avatar|pixel|spacer|blank|emoji|badge|button|ads?[/_-])`)
)

// tinyOrIcon: gambar yang hampir pasti bukan foto berita.
func tinyOrIcon(tag, src string) bool {
	for _, d := range reImgDim.FindAllStringSubmatch(tag, -1) {
		if n, err := strconv.Atoi(d[1]); err == nil && n < 100 {
			return true
		}
	}
	return reIconish.MatchString(src)
}

// bodyOf memotong HTML ke badan artikel yang dikenali; "" bila tidak dikenali.
func bodyOf(h string) string {
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

var (
	reLDBlock = regexp.MustCompile(`(?is)<script[^>]+application/ld\+json[^>]*>(.*?)</script>`)
	reLDURL   = regexp.MustCompile(`(?is)(https?:[^"']+\.(?:jpg|jpeg|png|webp|gif)[^"']*)`)
	// Penanda badan artikel yang dipakai luas: WordPress (entry-content),
	// Blogger (post-body), schema.org (itemprop=articleBody), dan <article>.
	reArticleBody = regexp.MustCompile(`(?is)<(?:div|section|article)\b[^>]*(?:class|id|itemprop)\s*=\s*["'][^"']*(?:post-body|entry-content|article-body|articleBody|post-content)[^"']*["']`)
	reBodyEnd     = regexp.MustCompile(`(?is)(?:post-footer|entry-footer|comments|<footer\b)`)
	reImgTag      = regexp.MustCompile(`(?is)<img[^>]+src\s*=\s*["']([^"']+)["']`)
)

// siteBadge memilih nama sumber untuk lencana kartu.
//
// og:site_name biasanya nama yang enak dibaca ("Kompas.com", "CNN Indonesia"),
// jadi ia yang dipakai — TAPI hanya bila ia masih mengenali dirinya sebagai
// situs itu. Sebagian situs mengisinya dengan slogan atau branding musiman:
// tabloidlugas.com menulis "LUGAS 28th", dan lencana berbunyi "LUGAS 28th"
// tidak memberi tahu pembaca dari mana beritanya (dilaporkan 7 Agustus 2026).
//
// Ujinya sederhana dan tidak menghakimi isi: salah satu harus memuat yang lain
// setelah dibersihkan jadi huruf & angka. "kompascom" memuat "kompas" → dipakai;
// "lugas28th" dan "tabloidlugas" tidak saling memuat → pakai domainnya.
func siteBadge(siteName, host string) string {
	name := strings.TrimSpace(siteName)
	if name == "" {
		return host
	}
	a := alnum(name)
	b := alnum(mainLabel(host))
	// Label pendek ("id", "co") terlalu gampang cocok secara kebetulan.
	if a == "" || len(b) < 4 {
		return host
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return name
	}
	return host
}

// mainLabel mengambil bagian pertama sebuah host: "tabloidlugas.com" →
// "tabloidlugas".
func mainLabel(host string) string {
	if i := strings.IndexByte(host, '.'); i > 0 {
		return host[:i]
	}
	return host
}

// alnum menyisakan huruf & angka, dalam huruf kecil.
func alnum(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// jsonLDDate membaca `datePublished` dari blok schema.org.
//
// dateModified sengaja TIDAK dipakai sebagai cadangan: kartu menyebut kapan
// beritanya terbit, dan tanggal penyuntingan terakhir bisa jauh berbeda.
func jsonLDDate(h string) string {
	for _, m := range reLDBlock.FindAllStringSubmatch(h, -1) {
		if d := reLDPublished.FindStringSubmatch(m[1]); len(d) == 2 {
			return d[1]
		}
	}
	return ""
}

// timeTagDate membaca <time datetime="…"> pertama di halaman.
//
// ponytail: yang PERTAMA, bukan yang paling tepat — halaman bisa memuat
// beberapa <time> (mis. daftar "artikel terkait" di bawahnya). Pada susunan yang
// dijumpai, yang pertama selalu milik artikelnya sendiri. Kalau suatu saat ada
// situs yang menaruh tanggal lain lebih dulu, saring dengan class/itemprop
// "published" — jangan menambah tebakan lain.
func timeTagDate(h string) string {
	if m := reTimeTag.FindStringSubmatch(h); len(m) == 2 {
		return m[1]
	}
	return ""
}

var (
	reLDPublished = regexp.MustCompile(`(?is)"datePublished"\s*:\s*"([^"]+)"`)
	reTimeTag     = regexp.MustCompile(`(?is)<time\b[^>]*\bdatetime\s*=\s*["']([^"']+)["']`)
)
