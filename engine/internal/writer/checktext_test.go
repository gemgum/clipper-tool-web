package writer

import (
	"testing"

	"github.com/gemgum/clipper/engine/internal/news"
)

// Ringkasan kartu: angka, kutipan, dan nama yang tidak ada di artikel
// ditandai; yang ada di artikel lolos. Panjang tidak dinilai.
func TestCheckText(t *testing.T) {
	c := news.Content{
		Article:    news.Article{Title: "Polda Jateng dampingi kasus di Pemalang"},
		Paragraphs: []news.Paragraph{{Text: "Polres Pemalang memeriksa dua anak, usia 14 tahun, kata Kapolres Budi."}},
	}
	if vs := CheckText("Polres Pemalang memeriksa dua anak berusia 14 tahun.", c); len(vs) != 0 {
		t.Fatalf("teks setia ditandai: %v", vs)
	}
	vs := CheckText(`Polisi menahan 3 anak, "kami tidak akan ragu", kata Kapolres Joko.`, c)
	kinds := map[string]bool{}
	for _, v := range vs {
		kinds[v.Kind] = true
	}
	for _, k := range []string{"number", "quote", "name"} {
		if !kinds[k] {
			t.Errorf("pelanggaran %q tidak terdeteksi: %v", k, vs)
		}
	}
}
