package card

import (
	"context"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/news"
)

// Tiap tampilan jadi menulis warnanya sendiri ke HTML, dan tinggi area foto
// mengikuti rasio. Tanpa tema, blok penimpanya tidak ada sama sekali.
func TestThemeColoursInHTML(t *testing.T) {
	b := New(nil, t.TempDir())
	a := news.Article{Title: "Judul", Summary: "Ringkasan", Source: "ANTARA"}
	for theme, want := range themeTable {
		html, _, err := b.render(context.Background(), Request{Article: a, Theme: theme}, 1080, 1920)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range []string{want.Panel, want.Title, want.Body, want.Badge, want.OnBadge} {
			if !strings.Contains(string(html), c) {
				t.Errorf("%s: warna %s tidak ada di HTML", theme, c)
			}
		}
	}
	html, _, _ := b.render(context.Background(), Request{Article: a}, 1080, 1920)
	if strings.Contains(string(html), "Tampilan jadi (DESIGN-NEWSCARD") {
		t.Error("tanpa tema, blok penimpa tetap ikut")
	}
	// Foto data URI dihitung sebagai foto; tinggi area foto 9:16 = 46%.
	a.Image = "data:image/png;base64,AAAA"
	html, _, _ = b.render(context.Background(), Request{Article: a, Theme: ThemeDark}, 1080, 1920)
	if !strings.Contains(string(html), `src="data:image/png;base64,AAAA"`) || !strings.Contains(string(html), "height:883px") {
		t.Error("foto data URI tidak dirender atau tinggi foto bukan 46%")
	}
}

// HTML = halaman yang sama dengan Build (tanpa Chrome) + letak area foto untuk
// seretan pratinjau: tema memakai tinggi foto config.CardPhotoPct, dan pita
// CardTop menurunkan area foto.
func TestHTMLGeometry(t *testing.T) {
	b := &Builder{}
	a := news.Article{Title: "Judul", Summary: "Isi"}
	_, geo, err := b.HTML(context.Background(), Request{Article: a, Theme: ThemeNavy, Ratio: Ratio45, CardTop: 100, Photo: Photo{Zoom: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if geo.Width != 1080 || geo.Height != 1350 || geo.PhotoH != 1350*42/100 || geo.PhotoTop != int(100*geo.Scale) || geo.OffsetW != 540 {
		t.Fatalf("geometri salah: %+v", geo)
	}
	if _, _, err := b.HTML(context.Background(), Request{}); err == nil {
		t.Fatal("judul kosong harus ditolak")
	}
}

// CardY menggeser batas foto–isi, dijepit 20–85% kanvas.
func TestCardYMovesPhotoEdge(t *testing.T) {
	b := &Builder{}
	a := news.Article{Title: "Judul", Summary: "Isi"}
	_, base, _ := b.HTML(context.Background(), Request{Article: a, Theme: ThemeDark})
	_, geo, _ := b.HTML(context.Background(), Request{Article: a, Theme: ThemeDark, CardY: 300})
	if geo.PhotoH != base.PhotoH+300 || geo.PhotoBase != base.PhotoH {
		t.Fatalf("CardY 300: %+v vs %+v", geo, base)
	}
	_, geo, _ = b.HTML(context.Background(), Request{Article: a, Theme: ThemeDark, CardY: 99999})
	if geo.PhotoH != 1920*85/100 {
		t.Fatalf("jepitan bawah: %d", geo.PhotoH)
	}
}

// Lencana sumber di pojok kiri atas dan "read the full story" sudah dibuang
// (keputusan pemilik 24 Oktober 2026): sumbernya tetap di stempel bawah judul.
func TestNoCornerBadgeNoReadMore(t *testing.T) {
	b := New(nil, t.TempDir())
	a := news.Article{Title: "Judul", Summary: "Isi", Source: "ANTARA", Domain: "antaranews.com", Image: "data:image/png;base64,AAAA"}
	html, _, _ := b.render(context.Background(), Request{Article: a, Theme: ThemeDark}, 1080, 1920)
	s := string(html)
	if strings.Contains(s, `class="badge"`) || strings.Contains(s, "read the full story") || !strings.Contains(s, "antaranews.com") {
		t.Fatal("lencana pojok / read more masih ada, atau stempel sumber hilang")
	}
}

// Ringkasan kosong di tampilan jadi: judul tetap judul (ukuran TitlePx), bukan
// naik jadi hero berukuran teks isi — dulu judulnya mengecil drastis.
func TestEmptySummaryKeepsTitleBig(t *testing.T) {
	b := New(nil, t.TempDir())
	a := news.Article{Title: "Harga emas hari ini", Summary: ""}
	html, _, _ := b.render(context.Background(), Request{Article: a, Theme: ThemeDark}, 1080, 1920)
	s := string(html)
	if !strings.Contains(s, `<div class="context">Harga emas hari ini</div>`) || strings.Contains(s, `<div class="hero">`) {
		t.Fatal("judul harus tetap di .context dan blok hero kosong tidak digambar")
	}
	// Tanpa tema perilaku lama tetap: judul naik jadi hero.
	html, _, _ = b.render(context.Background(), Request{Article: a}, 1080, 1920)
	if !strings.Contains(string(html), `<div class="hero">Harga emas hari ini</div>`) {
		t.Fatal("tanpa tema judul seharusnya naik jadi hero")
	}
}

// Judul & ringkasan bisa diratakan berbeda; tanpa title_align judul ikut align.
func TestTitleAlignSeparate(t *testing.T) {
	b := New(nil, t.TempDir())
	a := news.Article{Title: "Judul", Summary: "Isi"}
	html, _, _ := b.render(context.Background(), Request{Article: a, Theme: ThemeDark, Align: AlignJustify, TitleAlign: AlignCenter}, 1080, 1920)
	s := string(html)
	if !strings.Contains(s, "text-align:center;margin-bottom:30px") || !strings.Contains(s, "letter-spacing:-.005em;text-align:justify") {
		t.Fatal("judul harus center, ringkasan justify")
	}
	html, _, _ = b.render(context.Background(), Request{Article: a, Theme: ThemeDark, Align: AlignRight}, 1080, 1920)
	if !strings.Contains(string(html), "text-align:right;margin-bottom:30px") {
		t.Fatal("tanpa title_align judul harus ikut align")
	}
}

// Tema "auto" tanpa foto jatuh ke Dark (tetap tampilan jadi, bukan gaya lama).
func TestAutoThemeWithoutPhoto(t *testing.T) {
	b := New(nil, t.TempDir())
	html, geo, _ := b.render(context.Background(), Request{Article: news.Article{Title: "Judul"}, Theme: ThemeAuto}, 1080, 1920)
	if !strings.Contains(string(html), themeTable[ThemeDark].Panel) || geo.PhotoBase != 1920*46/100 {
		t.Fatal("auto tanpa foto harus memakai warna Dark dan tinggi foto tampilan jadi")
	}
}

// Carousel: badan artikel dipecah ke beberapa halaman tanpa memotong kalimat,
// paragraf ringkasan dilewati, dan halaman lanjutan berisi teks + nomor.
func TestCarouselPages(t *testing.T) {
	sentence := "Pemerintah menyalurkan bantuan pangan kepada warga di seluruh wilayah terdampak banjir. "
	long := strings.Repeat(sentence, 40)
	req := Request{Whole: true, Theme: ThemeNavy, Ratio: Ratio45,
		Article: news.Article{Title: "Judul", Summary: "Ringkasan di kartu pertama yang cukup panjang.", Domain: "antaranews.com"},
		Body:    []string{"Ringkasan di kartu pertama yang cukup panjang. Lanjutannya.", long, "Paragraf penutup."}}
	pages := Pages(req, 1080, 1350)
	if len(pages) < 2 {
		t.Fatalf("paragraf panjang harus jadi beberapa halaman, dapat %d", len(pages))
	}
	for _, pg := range pages {
		for _, p := range pg {
			if strings.HasPrefix(p, "Ringkasan di kartu pertama") {
				t.Fatal("paragraf ringkasan tidak boleh diulang")
			}
			if !strings.HasSuffix(strings.TrimSpace(p), ".") {
				t.Fatalf("potongan berhenti di tengah kalimat: %q", p[len(p)-30:])
			}
		}
	}
	b := New(nil, t.TempDir())
	req.Page = 1
	html, geo, err := b.HTML(context.Background(), req)
	if err != nil || geo.Pages != len(pages)+1 || !strings.Contains(string(html), "2 / ") || !strings.Contains(string(html), "#0F2747") {
		t.Fatalf("halaman lanjutan salah: pages=%d err=%v", geo.Pages, err)
	}
	if Pages(Request{Body: []string{long}}, 1080, 1350) != nil {
		t.Fatal("tanpa Whole tidak ada halaman lanjutan")
	}
}
