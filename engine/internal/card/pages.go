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
	"github.com/gemgum/clipper/engine/internal/config"
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
	// NoPhoto = tanpa foto artikel di belakang: halaman lanjutan memakai
	// tampilan kartu (panel penuh) dengan warna kotak ini.
	NoPhoto bool `json:"no_photo"`
}

// Illustration = foto lanjutan carousel (keputusan pemilik 25 Oktober 2026):
// BUKAN latar, melainkan gambar di zona atas halaman lanjutan, teks mengalir
// di bawahnya. Page = nomor kartu (2 = kartu kedua); DX/DY = geseran dari
// tempat bawaannya (piksel kanvas), diatur dengan seret di pratinjau.
type Illustration struct {
	Image string `json:"image"`
	Page  int    `json:"page"`
	Size  string `json:"size"` // s | m | l
	DX    int    `json:"dx"`
	DY    int    `json:"dy"`
	// W/H = ukuran kotak yang ditarik sendiri di pratinjau (piksel kanvas);
	// 0 = ukuran bawaan dari Size.
	W int `json:"w"`
	H int `json:"h"`
	// Crop = potongan bebas: X/Y/W/H pecahan gambar (0..1), AR = rasio
	// lebar/tinggi potongan dalam piksel (dari GUI, yang tahu ukuran asli).
	Crop *Crop `json:"crop"`
}

// Crop = bagian gambar yang dipakai.
type Crop struct {
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	W  float64 `json:"w"`
	H  float64 `json:"h"`
	AR float64 `json:"ar"`
}

// IllRect = zona ilustrasi halaman yang dipratinjau (sudah digeser DX/DY),
// untuk lapis seret di GUI. Index = urutannya di Request.Illustrations.
type IllRect struct {
	Index int `json:"index"`
	X     int `json:"x"`
	Y     int `json:"y"`
	W     int `json:"w"`
	H     int `json:"h"`
	BaseX int `json:"base_x"` // tempat bawaan (tanpa DX/DY)
	BaseY int `json:"base_y"`
}

// illFor = ilustrasi milik halaman n (yang pertama bila lebih dari satu).
func illFor(req Request, n int) (int, *Illustration) {
	for i := range req.Illustrations {
		if req.Illustrations[i].Page == n && req.Illustrations[i].Image != "" {
			return i, &req.Illustrations[i]
		}
	}
	return -1, nil
}

// illGap = jarak zona ilustrasi ke teks di bawahnya.
const illGap = 28

// illZone = kotak bawaan ilustrasi: selebar 45/65/90% lebar isi (Kecil/
// Sedang/Besar), tinggi = 2/3 lebarnya tapi paling banyak separuh tinggi isi,
// di tengah, menempel ke tepi atas area isi.
func illZone(m pageMetrics, width int, ill *Illustration) (x, y, w, h int) {
	if ill.W > 0 {
		// Ukuran tarikan sendiri: dijepit ke bingkai & dua pertiga tinggi isi.
		w = min(ill.W, width)
		h = ill.H
		if h <= 0 {
			h = w * 2 / 3
		}
		h = min(h, m.avail*2/3)
		return (width - w) / 2, m.top, w, h
	}
	frac := map[string]float64{"s": 0.45, "l": 0.9}[ill.Size]
	if frac == 0 {
		frac = 0.65
	}
	w = int(float64(m.inner) * frac)
	h = min(w*2/3, m.avail/2)
	return (width - w) / 2, m.top, w, h
}

// illCropCSS = potongan dimuat utuh (contain) di kotak w×h: kotak tampil
// dw×dh di tengah, gambar di dalamnya diperbesar & digeser supaya hanya
// potongannya yang terlihat. CSS murni — pratinjau & PNG Chrome sama.
func illCropCSS(c *Crop, w, h int) (dw, dh, dl, dt, iw, ih, il, it int) {
	dw, dh = w, h
	if c.AR > 0 {
		if float64(w)/float64(h) > c.AR {
			dw = int(float64(h) * c.AR)
		} else {
			dh = int(float64(w) / c.AR)
		}
	}
	iw, ih = int(float64(dw)/c.W), int(float64(dh)/c.H)
	return dw, dh, (w - dw) / 2, (h - dh) / 2, iw, ih, -int(c.X * float64(iw)), -int(c.Y * float64(ih))
}

// boxed = halaman lanjutan bergaya kotak di atas foto. Gaya box TANPA foto
// latar memakai tampilan kartu dengan warna kotak (PageBox.NoPhoto).
func boxed(req Request) bool { return req.Box != nil && !req.Box.NoPhoto }

// Ukuran kotak gaya box = kotak judul Mark (subtitle.NewsBox) — angka yang
// sama dari config, ruang PlayRes yang lebarnya sama dengan kanvas.
const (
	boxMargin = config.NewsBoxMargin
	boxPad    = config.NewsBoxPad
	boxRadius = config.NewsBoxRadius
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
	if boxed(req) {
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
	full := float64(m.avail) / (float64(m.bodyPx) * 1.5) * 0.9 // baris
	// Halaman berilustrasi memuat lebih sedikit: zona gambar + jeda diambil
	// dari tinggi isinya.
	capFor := func(n int) float64 {
		if _, ill := illFor(req, n); ill != nil {
			_, _, _, h := illZone(m, width, ill)
			return math.Max(2, full-float64(h+illGap)/(float64(m.bodyPx)*1.5))
		}
		return full
	}
	capacity := capFor(2)
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
				capacity = capFor(len(pages) + 2)
			}
			cur, used, lastShort = append(cur, piece), used+n, short
		}
	}
	// Satu baris daftar yang tersisa sendirian di halaman terakhir ditarik
	// kembali ke halaman sebelumnya — memakai cadangan 10% kapasitas.
	if n := len(pages); n > 0 && len(cur) == 1 && lastShort && !strings.Contains(cur[0], "\n") &&
		prevUsed+1 <= capFor(n+1)/0.9 && strings.Contains(pages[n-1][len(pages[n-1])-1], "\n") {
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
	// Ilustrasi halaman ini (zona atas, sudah digeser) + tinggi ruang yang
	// dicadangkan di atas teks.
	Ill                    template.URL
	IllX, IllY, IllW, IllH int
	IllSpace               int
	// Potongan (Crop): kotak tampil & letak gambar di dalamnya.
	IllCrop                                bool
	CDW, CDH, CDL, CDT, CIW, CIH, CIL, CIT int
}

// renderPage = halaman lanjutan ke-n (2..total) berisi text.
func (b *Builder) renderPage(ctx context.Context, req Request, width, height, n, total int, text []string) ([]byte, *IllRect, error) {
	a := req.Article
	hasImage := isImageURL(a.Image)
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
		FontCSS: b.FontCSS(), Lang: langAttr(req.Lang), Align: align, RuleMargin: ruleMargin,
		Source: firstNonEmpty(a.Domain, a.Source),
		Panel:  theme.Panel, Title: theme.Title, Body: theme.Body, Badge: theme.Badge, OnBadge: theme.OnBadge,
		Text: text, N: n, Total: total,
	}
	switch {
	case boxed(req):
		d.Box, d.BoxMargin, d.BoxPad, d.Radius = req.Box, boxMargin, boxPad, boxRadius
		if req.Box.Sharp {
			d.Radius = 0
		}
		if hasImage {
			d.Photo = template.URL(a.Image)
		}
	case req.Box != nil:
		// Box tanpa foto latar: tampilan kartu, warna mengikuti kotak.
		d.Panel, d.Title, d.Body, d.Badge, d.OnBadge = req.Box.Fill, req.Box.Text, req.Box.Text, req.Box.Text, req.Box.Fill
	}
	var rect *IllRect
	if i, ill := illFor(req, n); ill != nil && isImageURL(ill.Image) {
		x, y, w, h := illZone(m, width, ill)
		rect = &IllRect{Index: i, X: x + ill.DX, Y: y + ill.DY, W: w, H: h, BaseX: x, BaseY: y}
		d.Ill, d.IllX, d.IllY, d.IllW, d.IllH, d.IllSpace = template.URL(ill.Image), rect.X, rect.Y, w, h, h+illGap
		if c := ill.Crop; c != nil && c.W > 0 && c.H > 0 {
			d.IllCrop = true
			d.CDW, d.CDH, d.CDL, d.CDT, d.CIW, d.CIH, d.CIL, d.CIT = illCropCSS(c, w, h)
		}
	}
	var buf bytes.Buffer
	if err := pageTmpl.Execute(&buf, d); err != nil {
		return nil, nil, fmt.Errorf("could not compose the card page: %v", err)
	}
	return buf.Bytes(), rect, nil
}

func isImageURL(s string) bool {
	return strings.HasPrefix(s, "http") || strings.HasPrefix(s, "data:image/")
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
		page, _, err := b.renderPage(ctx, req, width, height, i+2, total, text)
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
{{if .Ill}}body{justify-content:flex-start}
.ill-space{flex:none;height:{{.IllSpace}}px}
.ill{position:absolute;left:{{.IllX}}px;top:{{.IllY}}px;width:{{.IllW}}px;height:{{.IllH}}px;object-fit:contain;z-index:1}
.ill-c{position:absolute;left:{{.CDL}}px;top:{{.CDT}}px;width:{{.CDW}}px;height:{{.CDH}}px;overflow:hidden}
.ill-c img{position:absolute;left:{{.CIL}}px;top:{{.CIT}}px;width:{{.CIW}}px;height:{{.CIH}}px;max-width:none}{{end}}
{{with .Box}}
body{background:#111;padding:{{$.Top}}px {{$.BoxMargin}}px {{$.Bottom}}px;justify-content:{{if $.Ill}}flex-start{{else}}center{{end}}}
.photo{position:absolute;inset:0;width:100%;height:100%;object-fit:cover}
.box{position:relative;background:{{.Fill}};color:{{.Text}};padding:{{$.BoxPad}}px;border-radius:{{$.Radius}}px;
  {{if .Outline}}box-shadow:0 0 0 {{.Outline}}px {{.OutlineColor}};{{end}}max-height:100%;display:flex;flex-direction:column}
.box .text{flex:none}
.box .foot{position:static;margin-top:{{$.SmallPx}}px;letter-spacing:.1em;opacity:.75}
.box .foot b,.box .foot span{background:none;color:inherit;padding:0}
{{end}}
</style></head><body>
{{if .Ill}}{{if .IllCrop}}<div class="ill"><div class="ill-c"><img src="{{.Ill}}" alt=""></div></div>{{else}}<img class="ill" src="{{.Ill}}" alt="">{{end}}<div class="ill-space"></div>{{end}}
{{if .Box}}{{if .Photo}}<img class="photo" src="{{.Photo}}" alt="">{{end}}
<div class="box"><div class="text">{{range .Text}}<p>{{.}}</p>{{end}}</div>
<div class="foot"><b>{{.Source}}</b><span>{{.N}} / {{.Total}}</span></div></div>
{{else}}
<div class="rule"></div>
<div class="text">{{range .Text}}<p>{{.}}</p>{{end}}</div>
<div class="foot"><b>{{.Source}}</b><span>{{.N}} / {{.Total}}</span></div>
{{end}}
</body></html>`))
