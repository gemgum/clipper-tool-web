package card

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"math"
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

// PageBox = rupa kotak judul Mark (gaya box) yang dipakai halaman lanjutan.
// Warna sudah berupa CSS (#rrggbb) dari GUI, yang menerjemahkan nama warna
// lewat toHex yang sama dengan pratinjau Mark.
type PageBox struct {
	Fill         string `json:"fill"`
	Text         string `json:"text"`
	Outline      int    `json:"outline"`
	OutlineColor string `json:"outline_color"`
	Sharp        bool   `json:"sharp"`
}

// Ukuran kotak gaya box = kotak judul Mark (subtitle.NewsBox): newsMargin 40,
// newsPad 32, sudut 20 — ruang PlayRes yang lebarnya sama dengan kanvas.
const (
	boxMargin = 40
	boxPad    = 32
	boxRadius = 20
)

// maxParagraphs = paragraf terbanyak per halaman lanjutan (keputusan pemilik
// 25 Oktober 2026): halaman yang muat lebih banyak pun tidak dijejali.
const maxParagraphs = 2

// pageMetrics = ukuran halaman lanjutan, piksel kanvas (skala tinggi kanvas,
// sama dengan kartu pertama). inner = lebar teks; avail = tinggi untuk teks.
type pageMetrics struct{ bodyPx, smallPx, pad, top, bottom, inner, avail int }

func metricsFor(req Request, width, height int) pageMetrics {
	scale := float64(height) / 1920
	px := func(n int) int { return int(float64(n) * scale) }
	m := pageMetrics{bodyPx: px(scaled(46, req.Fonts.Paragraph)), smallPx: px(26), pad: px(120), top: px(200), bottom: px(220)}
	m.inner = width - 2*m.pad
	m.avail = height - m.top - m.bottom
	if req.Box != nil {
		// Kotak di tengah bingkai: ruang dalamnya + baris sumber di dasar kotak.
		m.inner = width - 2*(boxMargin+boxPad)
		m.avail = height - m.top - m.bottom - 2*boxPad - 2*m.smallPx
	}
	return m
}

// Pages memecah Body jadi teks tiap halaman lanjutan (kosong bila bukan
// carousel). Kapasitas halaman DIPERKIRAKAN dalam BARIS — lebar huruf 0,55 em,
// baris 1,5 em, jeda antarparagraf 0,4 baris, cadangan 10% — karena ukuran
// sebenarnya baru diketahui Chrome; perkiraannya sengaja kurang supaya teks
// tidak terpotong. Dihitung per baris, bukan per huruf: baris pendek (daftar
// harga "1 gram: Rp2.467.000") tetap memakan satu baris penuh.
// Paragraf yang lebih panjang dari satu halaman dipecah di batas kalimat,
// tidak pernah di tengah kalimat. Paragraf yang sudah tampil sebagai ringkasan
// di kartu pertama dilewati.
func Pages(req Request, width, height int) [][]string {
	if !req.Whole {
		return nil
	}
	m := metricsFor(req, width, height)
	cpl := float64(m.inner) / (float64(m.bodyPx) * 0.55)
	capacity := float64(m.avail) / (float64(m.bodyPx) * 1.5) * 0.9 // baris
	cost := func(t string) float64 { return math.Ceil(float64(len([]rune(t)))/cpl) + 0.4 }
	summary := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(req.Article.Summary), ".…"))

	var pages [][]string
	var cur []string
	used := 0.0
	lastShort := false
	prevUsed := 0.0
	for _, p := range req.Body {
		p = strings.Trim(p, " \t\n\u200e\u200f\ufeff") // tanda arah teks tak terlihat (Antara)
		if p == "" || (len(summary) > 20 && strings.HasPrefix(p, summary)) {
			continue
		}
		for _, piece := range splitSentences(p, int((capacity-1)*cpl)) {
			// Baris pendek berturut-turut (daftar harga, subjudul) = SATU
			// paragraf berbaris-baris, supaya batas maxParagraphs tidak
			// membuat satu daftar terserak ke belasan halaman.
			short := len([]rune(piece)) <= int(cpl)
			if short && lastShort && used+1 <= capacity {
				cur[len(cur)-1] += "\n" + piece
				used++
				continue
			}
			n := cost(piece)
			if used > 0 && (used+n > capacity || len(cur) >= maxParagraphs) {
				pages, cur, prevUsed, used = append(pages, cur), nil, used, 0
			}
			cur, used, lastShort = append(cur, piece), used+n, short
		}
	}
	// Satu baris daftar yang tersisa sendirian di halaman terakhir ditarik
	// kembali ke halaman sebelumnya — memakai cadangan 10% kapasitas.
	if n := len(pages); n > 0 && len(cur) == 1 && lastShort && !strings.Contains(cur[0], "\n") &&
		prevUsed+1 <= capacity/0.9 && strings.Contains(pages[n-1][len(pages[n-1])-1], "\n") {
		last := pages[n-1]
		last[len(last)-1] += "\n" + cur[0]
		cur = nil
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
	// Gaya box: foto (data:/http) di belakang + kotak bergaya Mark.
	Photo                     template.URL
	Box                       *PageBox
	BoxMargin, BoxPad, Radius int
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
	// Halaman lanjutan SELALU rata kiri-kanan: teks panjang paling nyaman
	// dibaca begitu (keputusan pemilik 25 Oktober 2026). Alignment di tab Mark
	// tetap berlaku untuk judul & ringkasan kartu pertama.
	align, ruleMargin := alignCSS(AlignJustify)
	m := metricsFor(req, width, height)
	d := pageData{
		Width: width, Height: height, BodyPx: m.bodyPx, SmallPx: m.smallPx, Pad: m.pad, Top: m.top, Bottom: m.bottom,
		FontCSS: b.fonts(), Lang: langAttr(req.Lang), Align: align, RuleMargin: ruleMargin,
		Source: firstNonEmpty(a.Domain, a.Source),
		Panel:  theme.Panel, Title: theme.Title, Body: theme.Body, Badge: theme.Badge, OnBadge: theme.OnBadge,
		Text: text, N: n, Total: total,
	}
	if req.Box != nil {
		d.Box, d.BoxMargin, d.BoxPad, d.Radius = req.Box, boxMargin, boxPad, boxRadius
		if req.Box.Sharp {
			d.Radius = 0
		}
		if hasImage {
			d.Photo = template.URL(a.Image)
		}
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

// Nomor halaman duduk di SEBELAH sumber (kiri bawah), bukan di pojok kanan:
// logo Mark biasanya di pojok kanan bawah dan menutupinya.
var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="{{.Lang}}"><head><meta charset="utf-8"><style>
{{.FontCSS}}
*{margin:0;padding:0;box-sizing:border-box}
html,body{width:{{.Width}}px;height:{{.Height}}px;overflow:hidden}
body{position:relative;background:{{.Panel}};color:{{.Body}};font-family:'Clipper Sans',sans-serif;
  padding:{{.Top}}px {{.Pad}}px {{.Bottom}}px;display:flex;flex-direction:column;justify-content:center}
.rule{width:64px;height:8px;background:{{.Badge}};margin:{{.RuleMargin}};flex:none}
.text{flex:none;max-height:100%;overflow:hidden;font-size:{{.BodyPx}}px;line-height:1.5;text-align:{{.Align}}}
.text p{white-space:pre-line}
.text p+p{margin-top:.6em}
.foot{position:absolute;left:{{.Pad}}px;bottom:{{.Pad}}px;display:flex;gap:16px;align-items:center;
  font-family:'Clipper Condensed','Clipper Sans',sans-serif;font-size:{{.SmallPx}}px;
  letter-spacing:.13em;text-transform:uppercase}
.foot b{background:{{.Badge}};color:{{.OnBadge}};padding:5px 14px;border-radius:2px;font-weight:400}
.foot span{color:{{.Title}}}
{{with .Box}}
body{background:#111;padding:{{$.Top}}px {{$.BoxMargin}}px {{$.Bottom}}px;justify-content:center}
.photo{position:absolute;inset:0;width:100%;height:100%;object-fit:cover}
.box{position:relative;background:{{.Fill}};color:{{.Text}};padding:{{$.BoxPad}}px;border-radius:{{$.Radius}}px;
  {{if .Outline}}box-shadow:0 0 0 {{.Outline}}px {{.OutlineColor}};{{end}}max-height:100%;display:flex;flex-direction:column}
.box .text{flex:none}
.box .foot{position:static;margin-top:{{$.SmallPx}}px;letter-spacing:.1em;opacity:.75}
.box .foot b,.box .foot span{background:none;color:inherit;padding:0}
{{end}}
</style></head><body>
{{if .Box}}{{if .Photo}}<img class="photo" src="{{.Photo}}" alt="">{{end}}
<div class="box"><div class="text">{{range .Text}}<p>{{.}}</p>{{end}}</div>
<div class="foot"><b>{{.Source}}</b><span>{{.N}} / {{.Total}}</span></div></div>
{{else}}
<div class="rule"></div>
<div class="text">{{range .Text}}<p>{{.}}</p>{{end}}</div>
<div class="foot"><b>{{.Source}}</b><span>{{.N}} / {{.Total}}</span></div>
{{end}}
</body></html>`))
