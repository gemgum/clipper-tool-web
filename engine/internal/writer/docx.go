package writer

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

// Unduhan .docx untuk artikel hasil (DESIGN-Clipper-Lanjutan §3). Ditulis
// dengan stdlib saja (zip + XML WordprocessingML minimum): judul, lead, isi,
// penanda sumber superskrip di akhir kalimat yang punya klaim, lalu daftar
// sumber. Tanpa styles.xml: ukuran & tebal ditulis langsung di run, jadi
// berkasnya terbuka sama di Word, LibreOffice, dan Google Docs.

var sentenceEnd = regexp.MustCompile(`[^.!?]+[.!?]+["”’)]*\s*|[^.!?]+$`)

// SplitSentences memecah paragraf jadi kalimat, mempertahankan tanda bacanya.
func SplitSentences(p string) []string {
	var out []string
	for _, s := range sentenceEnd.FindAllString(p, -1) {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// SourceOf mengembalikan nomor sumber (1-based) untuk satu kalimat, 0 bila
// tidak ada klaim yang cocok. Cocok = teks klaim sama dengan kalimatnya, atau
// salah satunya memuat yang lain (model kadang menyalin kalimat tanpa titik).
func SourceOf(sentence string, claims []Claim) int {
	norm := func(s string) string { return strings.ToLower(strings.Trim(strings.TrimSpace(s), ".!?\"”’ ")) }
	s := norm(sentence)
	if s == "" {
		return 0
	}
	for _, c := range claims {
		ct := norm(c.Text)
		if ct != "" && (ct == s || strings.Contains(s, ct) || strings.Contains(ct, s)) {
			return c.Source + 1
		}
	}
	return 0
}

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func run(text string, size int, bold, sup bool) string {
	var pr strings.Builder
	if bold {
		pr.WriteString("<w:b/>")
	}
	if sup {
		pr.WriteString(`<w:vertAlign w:val="superscript"/>`)
	}
	if size > 0 {
		fmt.Fprintf(&pr, `<w:sz w:val="%d"/>`, size*2)
	}
	return fmt.Sprintf(`<w:r><w:rPr>%s</w:rPr><w:t xml:space="preserve">%s</w:t></w:r>`, pr.String(), esc(text))
}

func para(runs ...string) string {
	return `<w:p><w:pPr><w:spacing w:after="160"/></w:pPr>` + strings.Join(runs, "") + `</w:p>`
}

// sourced menulis satu paragraf dengan penanda sumber di akhir tiap kalimat.
func sourced(p string, claims []Claim, size int, bold bool) string {
	var runs []string
	for i, s := range SplitSentences(p) {
		if i > 0 {
			runs = append(runs, run(" ", size, bold, false))
		}
		runs = append(runs, run(s, size, bold, false))
		if n := SourceOf(s, claims); n > 0 {
			runs = append(runs, run(fmt.Sprint(n), size, false, true))
		}
	}
	return para(runs...)
}

// Docx menyusun artikel jadi berkas .docx.
func Docx(d Draft, sources []SourceRef, sourcesHeading string) ([]byte, error) {
	var body strings.Builder
	body.WriteString(para(run(d.Title, 20, true, false)))
	if strings.TrimSpace(d.Lead) != "" {
		body.WriteString(sourced(d.Lead, d.Claims, 12, true))
	}
	for _, p := range d.Body {
		body.WriteString(sourced(p, d.Claims, 12, false))
	}
	if len(sources) > 0 {
		body.WriteString(para(run(sourcesHeading, 11, true, false)))
		for i, s := range sources {
			line := fmt.Sprintf("%d. %s", i+1, s.Title)
			if s.Media != "" {
				line += ", " + s.Media
			}
			if s.URL != "" {
				line += ": " + s.URL
			}
			body.WriteString(para(run(line, 10, false, false)))
		}
	}
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body.String() + `</w:body></w:document>`,
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml"} {
		f, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write([]byte(files[name])); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
