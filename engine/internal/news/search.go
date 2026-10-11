package news

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Pencarian berita memakai endpoint RSS milik Google News.
//
// Kenapa bukan menyisir halaman pencarian Google: halaman itu menolak akses
// otomatis. Diuji dari mesin biasa dengan Chrome asli yang menjalankan JS,
// balasannya CAPTCHA pada permintaan PERTAMA — nol hasil. Endpoint RSS ini
// sebaliknya memang disediakan untuk dibaca mesin, dan mengembalikan XML yang
// bisa diurai parser feed yang sudah ada.
//
// Operator pencarian tetap berlaku dan sudah diuji: "site:antaranews.com",
// tanda kutip untuk frasa persis, dan "when:7d" untuk membatasi rentang waktu.

// googleNewsHost = awalan tautan yang dikembalikan Google News. Tautan ini BUKAN
// alamat artikelnya, melainkan pengalih yang hanya bisa dibuka browser.
const googleNewsHost = "news.google.com"

// IsGoogleNewsLink melaporkan apakah sebuah URL masih berupa pengalih Google
// News, sehingga perlu diresolusi lebih dulu sebelum isinya bisa dibaca.
func IsGoogleNewsLink(u string) bool {
	return strings.Contains(u, googleNewsHost+"/rss/articles/") ||
		strings.Contains(u, googleNewsHost+"/articles/") ||
		strings.Contains(u, googleNewsHost+"/read/")
}

// Search mengambil berita menurut kata kunci.
//
// Perhatikan: Article.URL yang dikembalikan masih berupa pengalih Google News.
// Pemanggil wajib meresolusinya (lihat FetchContent) sebelum dipakai jadi kartu
// — kalau tidak, yang terbaca adalah halaman Google News, bukan artikelnya.
func Search(ctx context.Context, query string, max int, lang string) ([]Article, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("the search query is empty")
	}
	q := url.Values{}
	q.Set("q", query)
	// hl/gl/ceid menentukan bahasa & negara hasil. Tanpa ini yang keluar berita
	// berbahasa Inggris.
	q.Set("hl", "id")
	q.Set("gl", "ID")
	q.Set("ceid", "ID:id")

	raw, err := download(ctx, "https://"+googleNewsHost+"/rss/search?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("news search failed: %w", err)
	}
	articles, err := parseFeed(raw, "", max, lang)
	// Nol hasil BUKAN feed yang rusak: yang salah kata kuncinya, bukan alamat
	// yang tidak pernah diketik pengguna. Lihat errNoArticles di rss.go.
	if errors.Is(err, errNoArticles) {
		return nil, fmt.Errorf("no news found for %q: try fewer or more common words", query)
	}
	if err != nil {
		return nil, fmt.Errorf("the search results could not be read: %w", err)
	}
	return articles, nil
}
