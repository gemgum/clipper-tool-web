package news

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Serper sehat: 5 kueri × 1 panggilan, hasil disimpan dengan jam terbit dari
// tanggal relatif, dan SociaVault tidak disentuh sama sekali.
func TestCrawlSerperOnly(t *testing.T) {
	var serper, socia atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/news":
			n := serper.Add(1)
			w.Write([]byte(`{"news":[{"title":"Berita ` + string(rune('A'+n)) + `","link":"https://www.contoh.id/b` + string(rune('a'+n)) + `/?utm=x","date":"2 jam yang lalu","source":"https://contoh.id/"}],"credits":1}`))
		case "/account":
			w.Write([]byte(`{"balance":41,"rateLimit":50}`))
		default:
			socia.Add(1)
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	c := &Crawler{Dir: t.TempDir(), Now: func() time.Time { return now }, SerperURL: srv.URL, SociaVaultURL: srv.URL,
		Key: func(env string) string { return map[string]string{EnvSerper: "k"}[env] }}
	c.Run(context.Background())
	if serper.Load() != int32(len(CrawlQueries)) || socia.Load() != 0 {
		t.Fatalf("serper %d kali (harus %d), sociavault %d kali (harus 0)", serper.Load(), len(CrawlQueries), socia.Load())
	}
	items := c.Items()
	if len(items) != len(CrawlQueries) || items[0].Published != "2026-10-25T10:00:00Z" || items[0].Source != "Contoh" {
		t.Fatalf("hasil salah: %+v", items)
	}
	st := c.State()
	if b := st.APIs[APISerper].Balance; b == nil || *b != 41 || !st.APIs[APISerper].OK || st.APIs[APISociaVault].Configured {
		t.Fatalf("status salah: %+v", st.APIs)
	}
	// Belum 3 jam: tidak dipicu lagi.
	if c.MaybeStart(false) {
		t.Fatal("crawl tidak boleh dipicu sebelum 3 jam")
	}
}

// Serper kehabisan kredit: kueri berikutnya TIDAK ke Serper lagi, langsung
// SociaVault; statusnya no_credits.
func TestCrawlFallsBackWhenSerperOutOfCredits(t *testing.T) {
	var serper atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/news":
			serper.Add(1)
			w.WriteHeader(400)
			w.Write([]byte(`{"message":"Not enough credits","statusCode":400}`))
		case r.URL.Path == "/account":
			w.Write([]byte(`{"balance":0}`))
		case strings.HasPrefix(r.URL.Path, "/v1/scrape/google/search"):
			w.Write([]byte(`{"success":true,"data":{"results":{"0":{"title":"Satu","url":"https://a.id/1"},"1":{"title":"Dua","url":"https://b.id/2"}}},"credits_used":1}`))
		case r.URL.Path == "/v1/credits":
			w.Write([]byte(`{"credits":99}`))
		}
	}))
	defer srv.Close()
	now := time.Now()
	c := &Crawler{Dir: t.TempDir(), Now: func() time.Time { return now }, SerperURL: srv.URL, SociaVaultURL: srv.URL,
		Key: func(string) string { return "k" }}
	c.Run(context.Background())
	if serper.Load() != 1 {
		t.Fatalf("Serper yang habis harus dicoba sekali saja, dicoba %d kali", serper.Load())
	}
	st := c.State().APIs
	if st[APISerper].Problem != "no_credits" || !st[APISociaVault].OK || *st[APISociaVault].Balance != 99 {
		t.Fatalf("status salah: %+v", st)
	}
	if n := len(c.Items()); n != 2 { // kembar antar kueri dibuang
		t.Fatalf("harus 2 berita unik dari cadangan, dapat %d", n)
	}
}

// Berita lebih dari 7 hari dibuang saat berkas ditulis.
func TestCrawlExpiresAfterAWeek(t *testing.T) {
	now := time.Now()
	c := &Crawler{Dir: t.TempDir(), Now: func() time.Time { return now }, Key: func(string) string { return "" }}
	_ = c.save(crawlFile{Items: []CrawlItem{
		{Article: Article{URL: "https://a.id/baru"}, FoundAt: now.Add(-time.Hour)},
		{Article: Article{URL: "https://a.id/lama"}, FoundAt: now.Add(-8 * 24 * time.Hour)},
	}})
	if items := c.Items(); len(items) != 1 || items[0].URL != "https://a.id/baru" {
		t.Fatalf("yang lebih dari seminggu harus hilang: %+v", items)
	}
	if c.MaybeStart(true) {
		t.Fatal("tanpa kunci apa pun crawl tidak boleh jalan")
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	for in, want := range map[string]string{
		"13 menit yang lalu": "11:47", "1 jam yang lalu": "11:00", "2 hari yang lalu": "12:00", "3 hours ago": "09:00", "kemarin": "12:00",
	} {
		if got := relativeTime(in, now).Format("15:04"); got != want {
			t.Errorf("%q → %s, harus %s", in, got, want)
		}
	}
}

func TestCleanSource(t *testing.T) {
	for in, want := range map[string]string{
		"https://warta.in/": "Warta", "Rakyatterkini.com": "Rakyatterkini", "SuaraGarut.ID": "SuaraGarut",
		"Demokratis.co.id": "Demokratis", "gadget.qoo10.co.id": "Qoo10", "kabarnusa.com": "Kabarnusa",
		"Investing.com Indonesia": "Investing.com Indonesia", "ANTARA": "ANTARA", "www.topik.id": "Topik",
	} {
		if got := cleanSource(in); got != want {
			t.Errorf("%q → %q, harus %q", in, got, want)
		}
	}
}
