package writer

import (
	"context"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/news"
)

// Ringkasan kepanjangan dipotong di batas kata ≤180 dan ditandai "length";
// caption membawa hashtag tanpa "#"; galat mesin diteruskan apa adanya.
func TestWriteCardText(t *testing.T) {
	c := news.Content{Article: news.Article{Title: "Banjir di Bekasi"},
		Paragraphs: []news.Paragraph{{Text: "Banjir merendam Bekasi."}}}
	long := strings.Repeat("banjir merendam ", 20) // ±320 huruf
	fake := func(reply string) Completer {
		return func(context.Context, string, string, any) (string, error) { return reply, nil }
	}

	got, err := WriteCardText(context.Background(), fake(`{"text":"`+long+`"}`), "x", c, KindSummary, "id")
	if err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(got.Text)); n > SummaryMax || strings.HasSuffix(got.Text, " ") {
		t.Errorf("panjang %d (maks %d): %q", n, SummaryMax, got.Text)
	}
	if len(got.Violations) == 0 || got.Violations[0].Kind != "length" {
		t.Errorf("pemotongan tidak ditandai: %v", got.Violations)
	}

	got, err = WriteCardText(context.Background(), fake("Berikut:\n```json\n{\"text\":\"Banjir merendam Bekasi.\",\"hashtags\":[\"#Banjir\",\"Bekasi\"]}\n```"), "x", c, KindCaption, "id")
	if err != nil || strings.Join(got.Hashtags, ",") != "Banjir,Bekasi" || len(got.Violations) != 0 {
		t.Errorf("caption: %+v %v", got, err)
	}

	if _, err := WriteCardText(context.Background(), fake(`{}`), "x", c, "poem", "id"); err == nil {
		t.Error("jenis tak dikenal diterima")
	}
}
