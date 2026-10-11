// Package news mengambil bahan berita untuk kartu: daftar dari RSS, atau
// metadata satu artikel dari URL yang ditempel pengguna.
//
// Sengaja memakai RSS, bukan menyisir hasil mesin pencari. Feed itu memang
// disediakan untuk dibaca mesin, formatnya stabil, dan tidak akan memblokir
// kita — berbeda dengan scraping halaman pencarian yang cepat kena CAPTCHA.
package news

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Source satu feed berita.
type Source struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Topic string `json:"topic"`
}

// DefaultSources = feed media Indonesia yang dipakai GUI sebagai titik awal.
// Pengguna tetap bisa menempel URL feed lain lewat parameter ?feed=.
var DefaultSources = []Source{
	{"antara", "ANTARA", "https://www.antaranews.com/rss/terkini.xml", "Latest"},
	{"antara-economy", "ANTARA", "https://www.antaranews.com/rss/ekonomi.xml", "Economy"},
	{"antara-sports", "ANTARA", "https://www.antaranews.com/rss/olahraga.xml", "Sports"},
	{"antara-tech", "ANTARA", "https://www.antaranews.com/rss/tekno.xml", "Technology"},
	{"detik", "detikcom", "https://rss.detik.com/index.php/detiknews", "Latest"},
	{"detik-finance", "detikcom", "https://rss.detik.com/index.php/finance", "Economy"},
	{"detik-inet", "detikcom", "https://rss.detik.com/index.php/inet", "Technology"},
	{"cnn", "CNN Indonesia", "https://www.cnnindonesia.com/nasional/rss", "National"},
	{"cnn-economy", "CNN Indonesia", "https://www.cnnindonesia.com/ekonomi/rss", "Economy"},
	{"tempo", "Tempo", "https://rss.tempo.co/nasional", "National"},
	{"liputan6", "Liputan6", "https://feed.liputan6.com/rss", "Latest"},
	{"kumparan", "kumparan", "https://kumparan.com/kumparannews/rss", "Latest"},
}

// Article satu item berita — bentuk yang dipakai kartu.
type Article struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	URL     string `json:"url"`
	Image   string `json:"image"`
	// Images = semua foto layak dari artikel, Image di depan (article.go).
	Images    []string `json:"images"`
	Source    string   `json:"source"`    // nama media, mis. "ANTARA"
	Domain    string   `json:"domain"`    // mis. "antaranews.com"
	Date      string   `json:"date"`      // sudah diformat untuk manusia
	Published string   `json:"published"` // RFC3339 bila terbaca
}

// bentuk XML RSS 2.0. Field yang tidak dipakai sengaja tidak didaftarkan.
type rssFeed struct {
	Channel struct {
		Title string    `xml:"title"`
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
	// Atom (beberapa media memakai ini).
	AtomTitle   string      `xml:"title"`
	AtomEntries []atomEntry `xml:"entry"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	// Gambar bisa datang lewat beberapa cara tergantung media.
	Enclosure struct {
		URL string `xml:"url,attr"`
	} `xml:"enclosure"`
	MediaContent struct {
		URL string `xml:"url,attr"`
	} `xml:"content"`
	MediaThumb struct {
		URL string `xml:"url,attr"`
	} `xml:"thumbnail"`
	// Google News mengisi <source> dengan nama media & domainnya. Lebih akurat
	// daripada daftar kurasi kita, sebab mencakup media mana pun.
	Source struct {
		Name string `xml:",chardata"`
		URL  string `xml:"url,attr"`
	} `xml:"source"`
}

type atomEntry struct {
	Title   string `xml:"title"`
	Summary string `xml:"summary"`
	Updated string `xml:"updated"`
	Link    struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
}

var httpClient = &http.Client{Timeout: 25 * time.Second}

// downloadLimit membatasi besar respons agar satu URL nakal tidak menghabiskan RAM.
const downloadLimit = 4 << 20 // 4 MB

// download mengunduh URL sebagai teks dengan batas ukuran & User-Agent yang jelas.
func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}
	// Jujur menyebut diri; beberapa media memblokir User-Agent kosong.
	req.Header.Set("User-Agent", "Clipper/0.1 (+content creator; RSS reader)")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9")
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach %s: %v", url, err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("%s replied HTTP %d", url, res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, downloadLimit))
}

// ListFeed mengambil isi satu feed dan mengubahnya jadi daftar artikel.
//
// name = nama media untuk badge kartu. Diisi pemanggil bila feednya ada di
// DefaultSources, sebab judul channel RSS tidak bisa dipercaya: ANTARA menulis
// "Berita Terkini - ANTARA News" (nama medianya di belakang) sedangkan CNN
// menulis "CNN Indonesia | ..." (di depan). Bila kosong, judul channel dipakai
// sebagai tebakan, lalu domain sebagai jaring terakhir.
func ListFeed(ctx context.Context, feedURL, name string, max int, lang string) ([]Article, error) {
	raw, err := download(ctx, feedURL)
	if err != nil {
		return nil, err
	}
	return parseFeed(raw, name, max, lang)
}

// errNoArticles = feednya terbaca, isinya nol artikel.
//
// Dipisah jadi galat tersendiri karena artinya BERBEDA menurut siapa yang
// bertanya: untuk alamat feed media, kosong berarti alamatnya keliru; untuk
// pencarian, kosong berarti kata kuncinya tidak menemukan apa pun — dan
// menyuruh orang "check the feed URL" padahal ia baru saja mengetik kata kunci
// adalah petunjuk yang menyesatkan (dilaporkan 7 Agustus 2026).
var errNoArticles = errors.New("the feed parsed but contains no articles: check the feed URL")

// parseFeed mengubah isi XML jadi daftar artikel. Dipisah dari ListFeed supaya
// penguraiannya bisa diuji tanpa menyentuh jaringan.
func parseFeed(raw []byte, name string, max int, lang string) ([]Article, error) {
	if max <= 0 {
		max = 20
	}
	var f rssFeed
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	// Feed Indonesia kadang memakai encoding non-UTF8 atau entitas HTML mentah;
	// tanpa dua setelan ini decoder berhenti di karakter pertama yang aneh.
	dec.Strict = false
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("the feed content is not readable as RSS/Atom: %v", err)
	}

	feedName := clean(f.Channel.Title)
	if feedName == "" {
		feedName = clean(f.AtomTitle)
	}

	// Batas `max` DIPAKAI SESUDAH pengurutan, bukan saat mengumpulkan: memotong
	// lebih dulu berarti artikel terbaru bisa ikut terbuang hanya karena feednya
	// tidak menaruhnya di awal.
	var out []Article
	for _, it := range f.Channel.Items {
		link := clean(it.Link)
		if link == "" {
			continue
		}
		image := firstNonEmpty(it.Enclosure.URL, it.MediaContent.URL, it.MediaThumb.URL, imageInHTML(it.Description))
		// <source> (dipakai Google News) menang atas tebakan apa pun: ia menyebut
		// nama media per artikel, bukan per feed.
		media := clean(it.Source.Name)
		dom := domain(link)
		if it.Source.URL != "" {
			dom = domain(it.Source.URL)
		}
		out = append(out, Article{
			Title:     clean(stripSourceSuffix(it.Title, media)),
			Summary:   truncate(clean(stripTags(it.Description)), 300),
			URL:       link,
			Image:     image,
			Source:    firstNonEmpty(media, name, mediaName(feedName, link)),
			Domain:    dom,
			Date:      formatDate(it.PubDate, lang),
			Published: rfc3339(it.PubDate),
		})
	}
	for _, e := range f.AtomEntries {
		link := clean(e.Link.Href)
		if link == "" {
			continue
		}
		out = append(out, Article{
			Title:     clean(e.Title),
			Summary:   truncate(clean(stripTags(e.Summary)), 300),
			URL:       link,
			Image:     imageInHTML(e.Summary),
			Source:    firstNonEmpty(name, mediaName(feedName, link)),
			Domain:    domain(link),
			Date:      formatDate(e.Updated, lang),
			Published: rfc3339(e.Updated),
		})
	}
	if len(out) == 0 {
		return nil, errNoArticles
	}
	sortNewestFirst(out)
	if len(out) > max {
		out = out[:max]
	}
	return out, nil
}

// sortNewestFirst mengurutkan artikel dari yang paling baru.
//
// Feed TIDAK dijamin urut. Sebagian media menaruh artikel unggulan di atas,
// sebagian lagi mengurut naik, dan Google News mencampur banyak sumber tanpa
// urutan waktu sama sekali — jadi "yang di atas" bukan "yang terbaru".
//
// Yang tanpa tanggal terbaca ditaruh di BELAKANG, bukan di depan: tanggal
// kosong berarti tidak diketahui, dan menaruh yang tidak diketahui di puncak
// daftar "terbaru" adalah kebohongan kecil yang mahal — pengguna memilih
// artikel dari puncak daftar.
func sortNewestFirst(a []Article) {
	sort.SliceStable(a, func(i, j int) bool {
		x, y := a[i].Published, a[j].Published
		if (x == "") != (y == "") {
			return y == ""
		}
		return x > y // RFC3339 tersusun leksikografis = tersusun kronologis
	})
}

// --- util teks ---

var (
	reTag = regexp.MustCompile(`(?s)<[^>]*>`)
	reImg = regexp.MustCompile(`(?i)<img[^>]+src\s*=\s*["']([^"']+)["']`)
	reWS  = regexp.MustCompile(`\s+`)
)

func stripTags(s string) string { return reTag.ReplaceAllString(s, " ") }

// imageInHTML mencari <img src> yang diselipkan di dalam deskripsi RSS —
// kebiasaan ANTARA dan beberapa media lain.
func imageInHTML(s string) string {
	if m := reImg.FindStringSubmatch(s); len(m) == 2 {
		return html.UnescapeString(m[1])
	}
	return ""
}

// stripSourceSuffix memangkas " - Nama Media" di ujung judul. Google News
// menempelkannya ke SETIAP judul (terbukti 100 dari 100 pada uji), padahal nama
// medianya sudah tampil di lencana kartu — kalau dibiarkan, tiap judul berakhir
// dengan pengulangan.
func stripSourceSuffix(title, media string) string {
	title = strings.TrimSpace(title)
	media = strings.TrimSpace(media)
	if media == "" {
		return title
	}
	for _, sep := range []string{" - ", " – ", " — ", " | "} {
		suffix := sep + media
		if strings.HasSuffix(title, suffix) {
			return strings.TrimSpace(strings.TrimSuffix(title, suffix))
		}
	}
	return title
}

func clean(s string) string {
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	return strings.TrimSpace(reWS.ReplaceAllString(s, " "))
}

// truncate memangkas teks di batas kata terdekat agar tidak terputus di tengah.
func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)[:n]
	if i := strings.LastIndex(string(r), " "); i > n/2 {
		return strings.TrimSpace(string(r)[:i]) + "…"
	}
	return strings.TrimSpace(string(r)) + "…"
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

func domain(u string) string {
	s := u
	for _, p := range []string{"https://", "http://"} {
		s = strings.TrimPrefix(s, p)
	}
	s = strings.TrimPrefix(s, "www.")
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return s
}

// mediaName memilih nama yang enak dibaca untuk badge kartu. Judul channel RSS
// sering panjang ("ANTARA News - Berita Terkini..."), jadi dipangkas di
// pemisah umum; bila tetap tidak masuk akal, pakai domainnya.
func mediaName(feedTitle, link string) string {
	t := feedTitle
	for _, sep := range []string{" - ", " | ", " – ", ":"} {
		if i := strings.Index(t, sep); i > 0 {
			t = t[:i]
		}
	}
	t = strings.TrimSpace(t)
	if t == "" || len([]rune(t)) > 24 {
		return domain(link)
	}
	return t
}

// timeFormats = pola tanggal yang muncul di feed Indonesia.
var timeFormats = []string{
	time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822,
	time.RFC3339, "Mon, 2 Jan 2006 15:04:05 -0700", "2006-01-02 15:04:05",
}

func parseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, f := range timeFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// Nama bulan & hari per bahasa. Ini DATA bahasa, bukan bahasa kode: kartu yang
// terbit untuk pembaca Indonesia harus bertanggal Indonesia.
var monthNames = map[string][]string{
	"en": {"", "January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December"},
	"id": {"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember"},
}

var dayNames = map[string]map[time.Weekday]string{
	"en": {
		time.Sunday: "Sunday", time.Monday: "Monday", time.Tuesday: "Tuesday",
		time.Wednesday: "Wednesday", time.Thursday: "Thursday", time.Friday: "Friday",
		time.Saturday: "Saturday",
	},
	"id": {
		time.Sunday: "Minggu", time.Monday: "Senin", time.Tuesday: "Selasa",
		time.Wednesday: "Rabu", time.Thursday: "Kamis", time.Friday: "Jumat",
		time.Saturday: "Sabtu",
	},
}

// normalizeLang menjatuhkan bahasa yang tidak dikenal ke bahasa Inggris.
func normalizeLang(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if _, ok := monthNames[lang]; ok {
		return lang
	}
	return "en"
}

// formatDate menulis tanggal dalam bahasa yang diminta, untuk ditempel di kartu.
func formatDate(s, lang string) string {
	t, ok := parseTime(s)
	if !ok {
		return ""
	}
	l := normalizeLang(lang)
	return fmt.Sprintf("%s, %d %s %d",
		dayNames[l][t.Weekday()], t.Day(), monthNames[l][t.Month()], t.Year())
}

func rfc3339(s string) string {
	if t, ok := parseTime(s); ok {
		return t.Format(time.RFC3339)
	}
	return ""
}

// ListAll merangkak SEMUA feed bawaan sekaligus, menggabungkannya, lalu
// mengurutkan dari yang terbaru.
//
// Serentak, bukan berurutan: satu feed lambat tidak boleh menahan sembilan
// lainnya. Feed yang gagal DILEWATI diam-diam — satu media yang sedang tumbang
// tidak boleh mengosongkan seluruh daftar; yang penting daftarnya tetap terisi
// dari sumber lain. Galat hanya dikembalikan bila TIDAK SATU PUN berhasil.
func ListAll(ctx context.Context, max int, lang string) ([]Article, error) {
	if max <= 0 {
		max = 40
	}
	type result struct {
		items []Article
		err   error
	}
	// Tiap feed diambil secukupnya saja; setelah digabung dan diurutkan, hanya
	// `max` teratas yang dipakai.
	per := max/2 + 5
	res := make([]result, len(DefaultSources))
	var wg sync.WaitGroup
	for i, src := range DefaultSources {
		wg.Add(1)
		go func(i int, src Source) {
			defer wg.Done()
			items, err := ListFeed(ctx, src.URL, src.Name, per, lang)
			res[i] = result{items, err}
		}(i, src)
	}
	wg.Wait()

	var all []Article
	var firstErr error
	seen := map[string]bool{}
	for _, r := range res {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		for _, a := range r.items {
			// Google News dan feed media memuat artikel yang sama; kunci
			// duplikatnya URL, bukan judul — judul sering dipoles per sumber.
			if seen[a.URL] {
				continue
			}
			seen[a.URL] = true
			all = append(all, a)
		}
	}
	if len(all) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("no feed returned any article")
	}
	sortNewestFirst(all)
	if len(all) > max {
		all = all[:max]
	}
	return all, nil
}
