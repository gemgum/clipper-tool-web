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
		html, err := b.render(context.Background(), Request{Article: a, Theme: theme}, 1080, 1920)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range []string{want.Panel, want.Title, want.Body, want.Badge, want.OnBadge, want.Footer} {
			if !strings.Contains(string(html), c) {
				t.Errorf("%s: warna %s tidak ada di HTML", theme, c)
			}
		}
	}
	html, _ := b.render(context.Background(), Request{Article: a}, 1080, 1920)
	if strings.Contains(string(html), "Tampilan jadi (DESIGN-NEWSCARD") {
		t.Error("tanpa tema, blok penimpa tetap ikut")
	}
	// Foto data URI dihitung sebagai foto; tinggi area foto 9:16 = 46%.
	a.Image = "data:image/png;base64,AAAA"
	html, _ = b.render(context.Background(), Request{Article: a, Theme: ThemeDark}, 1080, 1920)
	if !strings.Contains(string(html), `src="data:image/png;base64,AAAA"`) || !strings.Contains(string(html), "height:883px") {
		t.Error("foto data URI tidak dirender atau tinggi foto bukan 46%")
	}
}
