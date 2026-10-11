package news

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// Menerjemahkan pengalih Google News TANPA browser.
//
// Kenapa perlu: pengalihnya berpindah lewat JavaScript, dan sebagian tautan
// tidak pernah sampai ke alamat medianya di Chrome headless — dicoba tiga kali
// dengan anggaran 15 detik, hasilnya tetap halaman Google (notes/36 butir 3b).
// Gejalanya di lapangan: berita hasil pencarian jadi kartu tanpa gambar, atau
// gagal dengan "the link has not reached the original article yet".
//
// Yang dipakai di sini adalah RPC yang dipanggil halaman Google itu sendiri
// saat hendak berpindah: `Fbv4je`. Bahannya tiga — id artikel (ada di alamat),
// stempel waktu dan tanda tangan (tertanam di halamannya sebagai atribut
// data-n-a-ts / data-n-a-sg). Ditukar lewat satu POST, jawabannya alamat asli.
//
// Dua permintaan HTTP biasa, di bawah satu detik; browser tetap disimpan
// sebagai jalan cadangan bila bentuk halaman Google berubah.

const batchExecuteURL = "https://news.google.com/_/DotsSplashUi/data/batchexecute"

var (
	reArticleTS = regexp.MustCompile(`data-n-a-ts="(\d+)"`)
	reArticleSG = regexp.MustCompile(`data-n-a-sg="([^"]+)"`)
)

// decodeGoogleNews mengembalikan alamat artikel di balik satu pengalih Google
// News. Galat berarti pemanggil harus mencoba jalan lain (browser).
func decodeGoogleNews(ctx context.Context, link string) (string, error) {
	id := googleArticleID(link)
	if id == "" {
		return "", fmt.Errorf("the link carries no Google News article id")
	}
	page, err := download(ctx, link)
	if err != nil {
		return "", err
	}
	ts := firstGroup(reArticleTS, string(page))
	sg := firstGroup(reArticleSG, string(page))
	if ts == "" || sg == "" {
		return "", fmt.Errorf("the Google News page no longer carries a request signature")
	}

	// Bentuk payload ini milik Google, bukan kita: deretan "X" dan angka di
	// dalamnya adalah nilai isian yang memang diabaikan server. Yang berarti
	// hanya tiga nilai terakhir — id, stempel waktu, tanda tangan.
	inner := `["garturlreq",[["X","X",["X","X"],null,null,1,1,"US:en",null,1,null,null,null,null,null,0,1],"X","X",1,[1,1,1],1,1,null,0,0,null,0],"` +
		id + `",` + ts + `,"` + sg + `"]`
	req, err := json.Marshal([]any{[]any{[]any{"Fbv4je", inner, nil, "generic"}}})
	if err != nil {
		return "", err
	}
	body, err := postForm(ctx, batchExecuteURL, url.Values{"f.req": {string(req)}})
	if err != nil {
		return "", err
	}
	return parseGarturlres(body)
}

// googleArticleID mengambil id artikel dari alamat pengalihnya, apa pun
// bentuknya (/rss/articles/…, /articles/…, /read/…).
func googleArticleID(link string) string {
	u, err := url.Parse(link)
	if err != nil || !strings.Contains(u.Host, googleNewsHost) {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, p := range parts {
		if (p == "articles" || p == "read") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// parseGarturlres memungut alamat artikel dari balasan batchexecute.
//
// Balasannya bukan JSON utuh: baris pertama penjaga anti-hijack `)]}'`, lalu
// baris-baris berisi array yang salah satu elemennya adalah JSON BERBENTUK
// TEKS. Karena itu ada dua tahap penguraian.
func parseGarturlres(body []byte) (string, error) {
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[[") {
			continue
		}
		var rows [][]json.RawMessage
		if json.Unmarshal([]byte(line), &rows) != nil {
			continue
		}
		for _, row := range rows {
			if len(row) < 3 {
				continue
			}
			var name, payload string
			if json.Unmarshal(row[1], &name) != nil || name != "Fbv4je" {
				continue
			}
			if json.Unmarshal(row[2], &payload) != nil {
				continue
			}
			// ["garturlres","https://…",1]
			var parts []any
			if json.Unmarshal([]byte(payload), &parts) != nil || len(parts) < 2 {
				continue
			}
			if got, ok := parts[1].(string); ok && strings.HasPrefix(got, "http") {
				return got, nil
			}
		}
	}
	return "", fmt.Errorf("Google News did not return the original address")
}

// postForm mengirim satu formulir dan mengembalikan isinya, dengan batas ukuran
// yang sama seperti download().
func postForm(ctx context.Context, addr string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", addr, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("User-Agent", "Clipper/0.1 (+content creator; RSS reader)")
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach %s: %v", addr, err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("%s replied HTTP %d", addr, res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, downloadLimit))
}

func firstGroup(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	return ""
}
