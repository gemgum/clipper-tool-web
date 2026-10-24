package card

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/gemgum/clipper/engine/internal/capture"
)

// Carousel seluruh artikel (keputusan pemilik 24 Oktober 2026): kartu pertama
// seperti biasa, lalu badan artikel — paragraf ASLI dari medianya, tanpa LLM,
// jadi tidak ada yang dikarang — dipecah jadi halaman lanjutan bertema sama:
// garis aksen, teks, lalu sumber & nomor halaman di kaki.

// PageFile = nama PNG halaman n (1 = kartu pertama = FilePNG).
func PageFile(n int) string {
	if n <= 1 {
		return FilePNG
	}
	return fmt.Sprintf("card-%d.png", n)
}

// pageMetrics = ukuran halaman lanjutan, piksel kanvas (skala tinggi kanvas,
// sama dengan kartu pertama).
type pageMetrics struct{ bodyPx, smallPx, pad, top, bottom int }

func metricsFor(req Request, height int) pageMetrics {
	scale := float64(height) / 1920
	px := func(n int) int { return int(float64(n) * scale) }
	return pageMetrics{bodyPx: px(scaled(46, req.Fonts.Paragraph)), smallPx: px(26), pad: px(80), top: px(200), bottom: px(220)}
}

// Pages memecah Body jadi teks tiap halaman lanjutan (kosong bila bukan
// carousel). Kapasitas halaman DIPERKIRAKAN dari ukuran huruf — lebar huruf
// rata-rata 0,5 em, baris 1,5 em, cadangan 15% — karena ukuran sebenarnya baru
// diketahui Chrome; perkiraannya sengaja kurang supaya teks tidak terpotong.
// Paragraf yang lebih panjang dari satu halaman dipecah di batas kalimat,
// tidak pernah di tengah kalimat. Paragraf yang sudah tampil sebagai ringkasan
// di kartu pertama dilewati.
func Pages(req Request, width, height int) [][]string {
	if !req.Whole {
		return nil
	}
	m := metricsFor(req, height)
	cpl := float64(width-2*m.pad) / (float64(m.bodyPx) * 0.5)
	lines := float64(height-m.top-m.bottom) / (float64(m.bodyPx) * 1.5)
	capacity := int(cpl * lines * 0.85)
	gap := int(cpl * 0.6) // jeda antarparagraf ≈ 0,6 baris
	summary := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(req.Article.Summary), ".…"))

	var pages [][]string
	var cur []string
	used := 0
	for _, p := range req.Body {
		p = strings.TrimSpace(p)
		if p == "" || (len(summary) > 20 && strings.HasPrefix(p, summary)) {
			continue
		}
		for _, piece := range splitSentences(p, capacity) {
			n := len([]rune(piece)) + gap
			if used > 0 && used+n > capacity {
				pages, cur, used = append(pages, cur), nil, 0
			}
			cur, used = append(cur, piece), used+n
		}
	}
	if len(cur) > 0 {
		pages = append(pages, cur)
	}
	return pages
}

// splitSentences memecah paragraf panjang jadi potongan <= limit karakter di
// batas kalimat (". ", "! ", "? "). Kalimat tunggal yang lebih panjang dari
// limit dibiarkan utuh.
func splitSentences(p string, limit int) []string {
	if len([]rune(p)) <= limit {
		return []string{p}
	}
	var sentences []string
	start := 0
	for i := 0; i < len(p)-1; i++ {
		if (p[i] == '.' || p[i] == '!' || p[i] == '?') && p[i+1] == ' ' {
			sentences = append(sentences, strings.TrimSpace(p[start:i+1]))
			start = i + 1
		}
	}
	sentences = append(sentences, strings.TrimSpace(p[start:]))
	var out []string
	chunk := ""
	for _, s := range sentences {
		if chunk != "" && len([]rune(chunk))+1+len([]rune(s)) > limit {
			out, chunk = append(out, chunk), ""
		}
		if chunk == "" {
			chunk = s
		} else {
			chunk += " " + s
		}
	}
	if chunk != "" {
		out = append(out, chunk)
	}
	return out
}

type pageData struct {
	Width, Height, BodyPx, SmallPx, Pad, Top, Bottom int
	FontCSS                                          template.CSS
	Lang, Align, RuleMargin, Source                  string
	Panel, Title, Body, Badge, OnBadge               string
	Text                                             []string
	N, Total                                         int
}

// renderPage = halaman lanjutan ke-n (2..total) berisi text.
func (b *Builder) renderPage(ctx context.Context, req Request, width, height, n, total int, text []string) ([]byte, error) {
	a := req.Article
	hasImage := strings.HasPrefix(a.Image, "http") || strings.HasPrefix(a.Image, "data:image/")
	theme := b.themeFor(ctx, req, hasImage)
	if theme == nil {
		tc := themeTable[ThemeDark]
		theme = &tc
	}
	align, ruleMargin := alignCSS(req.Align)
	m := metricsFor(req, height)
	d := pageData{
		Width: width, Height: height, BodyPx: m.bodyPx, SmallPx: m.smallPx, Pad: m.pad, Top: m.top, Bottom: m.bottom,
		FontCSS: b.fonts(), Lang: langAttr(req.Lang), Align: align, RuleMargin: ruleMargin,
		Source: firstNonEmpty(a.Domain, a.Source),
		Panel:  theme.Panel, Title: theme.Title, Body: theme.Body, Badge: theme.Badge, OnBadge: theme.OnBadge,
		Text: text, N: n, Total: total,
	}
	var buf bytes.Buffer
	if err := pageTmpl.Execute(&buf, d); err != nil {
		return nil, fmt.Errorf("could not compose the card page: %v", err)
	}
	return buf.Bytes(), nil
}

// BuildPages menulis halaman lanjutan carousel (card-2.png, …) ke dir dan
// mengembalikan jumlah seluruh kartu (termasuk yang pertama). PNG halaman
// lama di folder yang sama (pratinjau dipakai ulang) dibuang lebih dulu.
func (b *Builder) BuildPages(ctx context.Context, req Request, dir string) (int, error) {
	old, _ := filepath.Glob(filepath.Join(dir, "card-*.png"))
	for _, f := range old {
		os.Remove(f)
	}
	width, height := Dims(req.Ratio)
	pages := Pages(req, width, height)
	total := len(pages) + 1
	for i, text := range pages {
		page, err := b.renderPage(ctx, req, width, height, i+2, total, text)
		if err != nil {
			return 0, err
		}
		url, cleanup, err := b.cap.WriteTempFile(page, ".html")
		if err != nil {
			return 0, err
		}
		err = b.cap.Screenshot(ctx, url, filepath.Join(dir, PageFile(i+2)), capture.Options{Width: width, Height: height, Scale: 1, WaitMS: 5000})
		cleanup()
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}

var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="{{.Lang}}"><head><meta charset="utf-8"><style>
{{.FontCSS}}
*{margin:0;padding:0;box-sizing:border-box}
html,body{width:{{.Width}}px;height:{{.Height}}px;overflow:hidden}
body{position:relative;background:{{.Panel}};color:{{.Body}};font-family:'Clipper Sans',sans-serif;
  padding:{{.Top}}px {{.Pad}}px {{.Bottom}}px;display:flex;flex-direction:column}
.rule{width:64px;height:8px;background:{{.Badge}};margin:{{.RuleMargin}};flex:none}
.text{flex:1;overflow:hidden;font-size:{{.BodyPx}}px;line-height:1.5;text-align:{{.Align}}}
.text p+p{margin-top:.6em}
.foot{position:absolute;left:{{.Pad}}px;right:{{.Pad}}px;bottom:{{.Pad}}px;display:flex;
  justify-content:space-between;align-items:center;
  font-family:'Clipper Condensed','Clipper Sans',sans-serif;font-size:{{.SmallPx}}px;
  letter-spacing:.13em;text-transform:uppercase}
.foot b{background:{{.Badge}};color:{{.OnBadge}};padding:5px 14px;border-radius:2px;font-weight:400}
.foot span{color:{{.Title}}}
</style></head><body>
<div class="rule"></div>
<div class="text">{{range .Text}}<p>{{.}}</p>{{end}}</div>
<div class="foot"><b>{{.Source}}</b><span>{{.N}} / {{.Total}}</span></div>
</body></html>`))
