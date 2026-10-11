package news

import (
	"net/url"
	"reflect"
	"testing"
)

// Daftar foto: og di depan, lalu foto badan; relatif dijadikan absolut,
// kembar/ikon/piksel kecil dibuang.
func TestArticleImages(t *testing.T) {
	h := `<html><head><meta property="og:image" content="/img/hero.jpg"></head><body>
<img src="/logo.png"><div class="entry-content">
<img src="https://cdn.x.id/a.jpg" width="800">
<img src="/img/hero.jpg">
<img src="https://cdn.x.id/track.gif" width="1" height="1">
<img src="https://cdn.x.id/share-icon.png">
<img src="b.webp"></div><footer></footer></body></html>`
	base, _ := url.Parse("https://x.id/berita/satu")
	got := articleImages(h, base, "/img/hero.jpg", "")
	want := []string{"https://x.id/img/hero.jpg", "https://cdn.x.id/a.jpg", "https://x.id/berita/b.webp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}
