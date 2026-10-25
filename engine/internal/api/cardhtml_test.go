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

// Pratinjau kartu: font tertanam diganti penanda dan dikirim sekali saja;
// ?fonts=cached tidak mengirimnya lagi, jadi pindah halaman tinggal HTML-nya.
func TestCardHTMLSendsFontsOnce(t *testing.T) {
	fonts := "../../../assets/fonts"
	if _, err := os.Stat(fonts + "/Montserrat.ttf"); err != nil {
		t.Skip("font bawaan tidak ada (jalankan fetch-fonts.sh)")
	}
	s := &Server{paths: config.Paths{DataDir: t.TempDir()}, cards: card.New(capture.New(""), fonts)}
	body := `{"article":{"title":"Judul uji","summary":"Ringkas"},"ratio":"9:16","preview":true}`
	get := func(q string) map[string]any {
		w := httptest.NewRecorder()
		s.cardHTML(w, httptest.NewRequest("POST", "/api/card/html"+q, bytes.NewBufferString(body)))
		if w.Code != 200 {
			t.Fatalf("status %d: %s", w.Code, w.Body)
		}
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	first := get("")
	html, css := first["html"].(string), first["fonts"].(string)
	if !strings.Contains(html, CardFontMark) || strings.Contains(html, "base64") || !strings.Contains(css, "@font-face") {
		t.Fatalf("HTML harus memakai penanda dan font terpisah (html %d B, fonts %d B)", len(html), len(css))
	}
	if again := get("?fonts=cached"); again["fonts"] != nil || again["html"] != html {
		t.Fatal("?fonts=cached tidak boleh mengirim font lagi, HTML-nya tetap sama")
	}
}
