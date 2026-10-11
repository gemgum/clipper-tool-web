package writer

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

// .docx berisi judul, kalimat dengan penanda sumber superskrip (1-based),
// dan daftar sumber; dan merupakan zip yang terbaca.
func TestDocx(t *testing.T) {
	d := Draft{
		Title: "Judul & uji",
		Lead:  "Polisi menangkap empat tersangka. Kalimat tanpa klaim.",
		Body:  []string{"Korban dua bayi."},
		Claims: []Claim{
			{Text: "Polisi menangkap empat tersangka", Source: 0},
			{Text: "Korban dua bayi.", Source: 1},
		},
	}
	raw, err := Docx(d, []SourceRef{{Title: "A", Media: "ANTARA"}, {Title: "B"}}, "Sources")
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var doc string
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			doc = string(b)
		}
	}
	for _, want := range []string{"Judul &amp; uji", `superscript"/><w:sz w:val="24"/></w:rPr><w:t xml:space="preserve">1<`, `>2<`, "1. A, ANTARA"} {
		if !strings.Contains(doc, want) {
			t.Errorf("document.xml tidak memuat %q", want)
		}
	}
	if SourceOf("Kalimat tanpa klaim.", d.Claims) != 0 {
		t.Error("kalimat tanpa klaim diberi sumber")
	}
}
