package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/capture"
	"github.com/gemgum/clipper/engine/internal/card"
	"github.com/gemgum/clipper/engine/internal/config"
)

// Pratinjau kartu: font tertanam diganti penanda + kunci; CSS fontnya dari
// cardFonts, yang boleh disimpan browser selamanya untuk kunci yang cocok.
func TestCardHTMLFontsSeparate(t *testing.T) {
	fonts := "../../../assets/fonts"
	if _, err := os.Stat(fonts + "/Montserrat.ttf"); err != nil {
		t.Skip("font bawaan tidak ada (jalankan fetch-fonts.sh)")
	}
	s := &Server{paths: config.Paths{DataDir: t.TempDir()}, cards: card.New(capture.New(""), fonts)}
	w := httptest.NewRecorder()
	body := `{"article":{"title":"Judul uji","summary":"Ringkas"},"ratio":"9:16","preview":true}`
	s.cardHTML(w, httptest.NewRequest("POST", "/api/card/html", bytes.NewBufferString(body)))
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	html, key := out["html"].(string), out["fonts_key"].(string)
	if !strings.Contains(html, CardFontMark) || strings.Contains(html, "base64") || key == "" || out["fonts"] != nil {
		t.Fatalf("HTML harus memakai penanda dan kunci font, tanpa font (html %d B, key %q)", len(html), key)
	}
	// Font: kunci yang cocok = boleh di-cache selamanya; kunci lain = tidak.
	for _, c := range []struct{ v, cache string }{{key, "immutable"}, {"lama", "no-cache"}} {
		w := httptest.NewRecorder()
		s.cardFonts(w, httptest.NewRequest("GET", "/api/card/fonts?v="+c.v, nil))
		if !strings.Contains(w.Body.String(), "@font-face") || !strings.Contains(w.Header().Get("Cache-Control"), c.cache) {
			t.Fatalf("v=%s: cache %q, isi %d B", c.v, w.Header().Get("Cache-Control"), w.Body.Len())
		}
	}
}
