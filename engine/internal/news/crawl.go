package news

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Crawl berita lewat API pencarian berbayar (notes/45, keputusan pemilik 25
// Oktober 2026). Yang dihemat KREDIT, jadi:
//
//   - crawl hanya dipicu saat website dibuka (MaybeStart dari daftar berita),
//     paling cepat tiap CrawlEvery — server tanpa pengunjung = 0 kredit;
//   - Serper (Google News, lengkap dengan sumber & gambar) = utama, 1 kredit
//     per kueri; SociaVault (Google Search umum) HANYA cadangan bila Serper
//     gagal atau habis;
//   - hasil disimpan lokal dan kedaluwarsa sendiri setelah CrawlKeep;
//   - tidak ada LLM sama sekali di sini.

// CrawlQueries = kategori tiap putaran; CrawlPerQuery = berita terbaru per
// kategori, dari rentang hari ini sampai 7 hari lalu.
var CrawlQueries = []string{"berita terkini", "ekonomi", "olahraga", "teknologi", "polisi"}

const (
	CrawlPerQuery = 10
	CrawlEvery    = 3 * time.Hour
	CrawlKeep     = 7 * 24 * time.Hour
	// crawlMax = batas isi berkas, supaya tidak tumbuh tanpa ujung.
	crawlMax = 300
)

// Nama API & variabel .env kuncinya.
const (
	APISerper     = "serper"
	APISociaVault = "sociavault"
	EnvSerper     = "SERPER_API_KEY"
	EnvSociaVault = "SOCIAVAULT_API_KEY"
)

// CrawlItem = satu berita hasil crawl.
type CrawlItem struct {
	Article
	FoundAt time.Time `json:"found_at"`
	Via     string    `json:"via"`
	Query   string    `json:"query"`
}

// APIStatus = keadaan satu API untuk halaman Requirements.
type APIStatus struct {
	Configured bool      `json:"configured"`
	OK         bool      `json:"ok"`
	Balance    *int      `json:"balance,omitempty"`
	CheckedAt  time.Time `json:"checked_at,omitempty"`
	// Problem = kode masalah (no_credits | key_rejected | rate_limited |
	// unreachable); Error = kalimat aslinya.
	Problem string `json:"problem,omitempty"`
	Error   string `json:"error,omitempty"`
}

type crawlFile struct {
	LastCrawl time.Time            `json:"last_crawl"`
	Items     []CrawlItem          `json:"items"`
	Status    map[string]APIStatus `json:"status"`
}

// Crawler menyimpan hasil di Dir/crawl.json. Key membaca kunci API (os.Getenv
// di engine); Now & dua alamat dasar bisa ditimpa uji.
type Crawler struct {
	Dir           string
	Key           func(env string) string
	Now           func() time.Time
	SerperURL     string
	SociaVaultURL string

	mu      sync.Mutex
	running bool
}

// NewCrawler = Crawler dengan alamat & jam sungguhan.
func NewCrawler(dir string) *Crawler {
	return &Crawler{Dir: dir, Key: os.Getenv, Now: time.Now,
		SerperURL: "https://google.serper.dev", SociaVaultURL: "https://api.sociavault.com"}
}

func (c *Crawler) path() string { return filepath.Join(c.Dir, "crawl.json") }

func (c *Crawler) load() crawlFile {
	var f crawlFile
	if raw, err := os.ReadFile(c.path()); err == nil {
		_ = json.Unmarshal(raw, &f)
	}
	if f.Status == nil {
		f.Status = map[string]APIStatus{}
	}
	return f
}

// save membuang yang kedaluwarsa & yang melebihi crawlMax, lalu menulis.
func (c *Crawler) save(f crawlFile) error {
	cut := c.Now().Add(-CrawlKeep)
	kept := f.Items[:0]
	for _, it := range f.Items {
		if it.FoundAt.After(cut) {
			kept = append(kept, it)
		}
	}
	if len(kept) > crawlMax {
		kept = kept[:crawlMax]
	}
	f.Items = kept
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(f, "", " ")
	tmp := c.path() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path())
}

// Items = berita tersimpan yang belum kedaluwarsa, terbaru dulu.
func (c *Crawler) Items() []Article {
	c.mu.Lock()
	f := c.load()
	c.mu.Unlock()
	cut := c.Now().Add(-CrawlKeep)
	var out []Article
	for _, it := range f.Items {
		if it.FoundAt.After(cut) {
			it.Source = cleanSource(it.Source) // juga untuk simpanan sebelum aturan nama ini
			out = append(out, it.Article)
		}
	}
	sortNewestFirst(out)
	return out
}

// CrawlState = jawaban /api/news/crawl.
type CrawlState struct {
	LastCrawl time.Time            `json:"last_crawl"`
	NextAfter time.Time            `json:"next_after"`
	Running   bool                 `json:"running"`
	Items     int                  `json:"items"`
	Queries   []string             `json:"queries"`
	APIs      map[string]APIStatus `json:"apis"`
}

// State = keadaan untuk halaman Requirements. "configured" dibaca ulang tiap
// kali, sebab kunci bisa baru saja disimpan.
func (c *Crawler) State() CrawlState {
	c.mu.Lock()
	defer c.mu.Unlock()
	f := c.load()
	apis := map[string]APIStatus{}
	for name, env := range map[string]string{APISerper: EnvSerper, APISociaVault: EnvSociaVault} {
		st := f.Status[name]
		st.Configured = c.Key(env) != ""
		apis[name] = st
	}
	next := f.LastCrawl.Add(CrawlEvery)
	return CrawlState{LastCrawl: f.LastCrawl, NextAfter: next, Running: c.running,
		Items: len(f.Items), Queries: CrawlQueries, APIs: apis}
}

// MaybeStart memulai crawl di latar bila yang terakhir sudah lewat CrawlEvery
// (atau force) dan tidak ada yang sedang jalan. Tanpa kunci apa pun, tidak
// ada yang dikerjakan. true = crawl dimulai.
func (c *Crawler) MaybeStart(force bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running || (c.Key(EnvSerper) == "" && c.Key(EnvSociaVault) == "") {
		return false
	}
	if !force && c.Now().Sub(c.load().LastCrawl) < CrawlEvery {
		return false
	}
	c.running = true
	// context.Background(): crawl tidak boleh mati karena halaman ditutup
	// (aturan CLAUDE.md "GUI tidak mengendalikan apa pun yang memakan waktu").
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		c.Run(ctx)
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
	}()
	return true
}

// Run = satu putaran crawl: tiap kueri ke Serper; bila Serper gagal, sisa
// kueri ke SociaVault. Saldo kedua API diperbarui (gratis) di akhir.
func (c *Crawler) Run(ctx context.Context) {
	now := c.Now()
	status := map[string]APIStatus{}
	var found []CrawlItem
	useSerper := c.Key(EnvSerper) != ""
	for _, q := range CrawlQueries {
		var items []CrawlItem
		var err error
		if useSerper {
			if items, err = c.serper(ctx, q, now); err != nil {
				status[APISerper] = failed(err, now)
				useSerper = false // jangan habiskan kueri berikutnya ke API yang sama
			} else {
				status[APISerper] = APIStatus{OK: true, CheckedAt: now}
			}
		}
		if !useSerper && c.Key(EnvSociaVault) != "" {
			if items, err = c.sociaVault(ctx, q, now); err != nil {
				status[APISociaVault] = failed(err, now)
				break
			}
			status[APISociaVault] = APIStatus{OK: true, CheckedAt: now}
		}
		found = append(found, items...)
	}
	c.balances(ctx, status, now)

	c.mu.Lock()
	defer c.mu.Unlock()
	f := c.load()
	f.LastCrawl = now
	for name, st := range status {
		f.Status[name] = st
	}
	seen := map[string]bool{}
	var merged []CrawlItem
	for _, it := range append(found, f.Items...) {
		k := normalizeURL(it.URL)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		merged = append(merged, it)
	}
	f.Items = merged
	_ = c.save(f)
}

// apiError membawa kode status supaya bisa dipetakan jadi masalah.
type apiError struct {
	status int
	body   string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.status, strings.TrimSpace(e.body))
}

// failed memetakan galat jadi status. Serper menjawab kredit habis dengan 400
// "Not enough credits"; 402 dipakai layanan lain untuk hal yang sama.
func failed(err error, now time.Time) APIStatus {
	st := APIStatus{CheckedAt: now, Error: err.Error(), Problem: "unreachable"}
	if e, ok := err.(*apiError); ok {
		low := strings.ToLower(e.body)
		switch {
		case e.status == 402 || strings.Contains(low, "credit"):
			st.Problem = "no_credits"
		case e.status == 401 || e.status == 403:
			st.Problem = "key_rejected"
		case e.status == 429:
			st.Problem = "rate_limited"
		}
	}
	return st
}

func (c *Crawler) call(ctx context.Context, method, u, header, key string, body any) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set(header, key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Klien biasa, BUKAN httpClient paket ini: httpx.Retry di sana mengulang
	// 429/5xx — untuk API berbayar setiap ulangan bisa berarti kredit.
	res, err := apiClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, downloadLimit))
	if res.StatusCode != 200 {
		return nil, &apiError{res.StatusCode, string(raw)}
	}
	return raw, nil
}

var apiClient = &http.Client{Timeout: 30 * time.Second}

// serper = Google News, terbaru dulu (sbd:1), hari ini sampai 7 hari lalu
// (qdr:w, keputusan pemilik 25 Oktober 2026), 1 kredit.
func (c *Crawler) serper(ctx context.Context, q string, now time.Time) ([]CrawlItem, error) {
	raw, err := c.call(ctx, "POST", c.SerperURL+"/news", "X-API-KEY", c.Key(EnvSerper),
		map[string]any{"q": q, "gl": "id", "hl": "id", "num": CrawlPerQuery, "tbs": "qdr:w,sbd:1"})
	if err != nil {
		return nil, err
	}
	var r struct {
		News []struct {
			Title, Link, Snippet, Date, Source, ImageURL string
		} `json:"news"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("unreadable Serper reply: %v", err)
	}
	var out []CrawlItem
	for _, n := range r.News {
		if n.Link == "" || n.Title == "" {
			continue
		}
		t := relativeTime(n.Date, now)
		out = append(out, CrawlItem{Article: Article{
			Title: clean(n.Title), Summary: clean(n.Snippet), URL: n.Link, Image: n.ImageURL,
			Source: firstNonEmpty(cleanSource(n.Source), cleanSource(domain(n.Link))), Domain: domain(n.Link),
			Date: formatDate(t.Format(time.RFC3339), "id"), Published: t.Format(time.RFC3339),
		}, FoundAt: now, Via: APISerper, Query: q})
		if len(out) >= CrawlPerQuery {
			break
		}
	}
	return out, nil
}

// sociaVault = Google Search umum 7 hari terakhir: tanpa gambar, sumber, dan
// jam terbit — waktunya diisi saat ditemukan.
func (c *Crawler) sociaVault(ctx context.Context, q string, now time.Time) ([]CrawlItem, error) {
	v := url.Values{"query": {q + " berita"}, "region": {"ID"}, "date_posted": {"last-week"}}
	raw, err := c.call(ctx, "GET", c.SociaVaultURL+"/v1/scrape/google/search?"+v.Encode(), "X-API-Key", c.Key(EnvSociaVault), nil)
	if err != nil {
		return nil, err
	}
	var r struct {
		Data struct {
			Results map[string]struct{ Title, URL, Description string } `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("unreadable SociaVault reply: %v", err)
	}
	var out []CrawlItem
	for i := 0; i < len(r.Data.Results) && len(out) < CrawlPerQuery; i++ {
		n, ok := r.Data.Results[strconv.Itoa(i)]
		if !ok || n.URL == "" || n.Title == "" {
			continue
		}
		out = append(out, CrawlItem{Article: Article{
			Title: clean(n.Title), Summary: clean(n.Description), URL: n.URL,
			Source: cleanSource(domain(n.URL)), Domain: domain(n.URL),
			Date: formatDate(now.Format(time.RFC3339), "id"), Published: now.Format(time.RFC3339),
		}, FoundAt: now, Via: APISociaVault, Query: q})
	}
	return out, nil
}

// balances memperbarui saldo kedua API. Cek saldo GRATIS di keduanya
// (diperiksa 25 Oktober 2026), jadi aman dipanggil tiap putaran.
func (c *Crawler) balances(ctx context.Context, status map[string]APIStatus, now time.Time) {
	check := func(name, env, method, u, header, field string) {
		key := c.Key(env)
		if key == "" {
			return
		}
		st, seen := status[name]
		raw, err := c.call(ctx, method, u, header, key, nil)
		if err != nil {
			if !seen {
				status[name] = failed(err, now)
			}
			return
		}
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		if n, ok := m[field].(float64); ok {
			b := int(n)
			st.Balance = &b
			if !seen {
				st.OK, st.CheckedAt = b > 0, now
			}
			if b <= 0 {
				st.OK, st.Problem, st.Error = false, "no_credits", "balance is 0"
			}
		}
		status[name] = st
	}
	check(APISerper, EnvSerper, "GET", c.SerperURL+"/account", "X-API-KEY", "balance")
	check(APISociaVault, EnvSociaVault, "GET", c.SociaVaultURL+"/v1/credits", "X-API-Key", "credits")
}

var reRelative = regexp.MustCompile(`(?i)(\d+)\s*(detik|menit|jam|hari|minggu|second|minute|min|hour|day|week)`)

// relativeTime membaca tanggal Serper ("13 menit yang lalu", "2 hours ago").
// Yang tidak terbaca = saat ditemukan.
func relativeTime(s string, now time.Time) time.Time {
	m := reRelative.FindStringSubmatch(s)
	if m == nil {
		if t, ok := parseTime(s); ok {
			return t
		}
		return now
	}
	n, _ := strconv.Atoi(m[1])
	unit := map[string]time.Duration{
		"detik": time.Second, "second": time.Second,
		"menit": time.Minute, "minute": time.Minute, "min": time.Minute,
		"jam": time.Hour, "hour": time.Hour,
		"hari": 24 * time.Hour, "day": 24 * time.Hour,
		"minggu": 7 * 24 * time.Hour, "week": 7 * 24 * time.Hour,
	}[strings.ToLower(m[2])]
	return now.Add(-time.Duration(n) * unit).Truncate(time.Minute)
}

// cleanSource = nama media untuk lencana & chip sumber, BUKAN domain
// (keputusan pemilik 25 Oktober 2026). Serper sering memberi domain
// ("Rakyatterkini.com", "https://warta.in/", "gadget.qoo10.co.id"); yang
// berbentuk alamat diganti label utamanya: "Rakyatterkini", "Warta", "Qoo10".
// Nama yang memuat spasi ("Investing.com Indonesia") sudah nama — dibiarkan.
func cleanSource(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, " ") || !strings.Contains(s, ".") {
		return s
	}
	host := strings.TrimPrefix(domain(s), "www.")
	labels := strings.Split(host, ".")
	for len(labels) > 1 && suffixLabel[strings.ToLower(labels[len(labels)-1])] {
		labels = labels[:len(labels)-1]
	}
	name := labels[len(labels)-1]
	// Pertahankan huruf besar-kecil asli bila ada ("SuaraGarut.ID" → "SuaraGarut").
	if i := strings.Index(strings.ToLower(s), strings.ToLower(name)); i >= 0 && strings.ToLower(s[i:i+len(name)]) != s[i:i+len(name)] {
		return s[i : i+len(name)]
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// suffixLabel = akhiran domain (TLD & tingkat kedua Indonesia) yang bukan nama.
var suffixLabel = map[string]bool{
	"com": true, "id": true, "net": true, "org": true, "co": true, "go": true, "ac": true, "or": true,
	"web": true, "my": true, "sch": true, "info": true, "news": true, "tv": true, "io": true, "biz": true, "in": true,
}

// Merge menggabungkan daftar berita (RSS + crawl): kembar dibuang lewat URL
// yang dinormalkan, terbaru dulu, paling banyak max (0 = semua).
func Merge(max int, lists ...[]Article) []Article {
	seen := map[string]bool{}
	var out []Article
	for _, l := range lists {
		for _, a := range l {
			k := normalizeURL(a.URL)
			if k == "" || seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, a)
		}
	}
	sortNewestFirst(out)
	if max > 0 && len(out) > max {
		out = out[:max]
	}
	return out
}

// normalizeURL = kunci kembar: tanpa skema, www, query, fragmen, /amp, garis
// miring akhir.
func normalizeURL(s string) string {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || u.Host == "" {
		return ""
	}
	p := strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), "/amp")
	return strings.TrimPrefix(strings.ToLower(u.Host), "www.") + p
}
