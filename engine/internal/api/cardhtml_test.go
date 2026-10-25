package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// Folder pratinjau per halaman (render paralel) sah sebagai id kartu, tapi
// tidak dianggap kartu tersimpan.
func TestPreviewPageIDs(t *testing.T) {
	for id, ok := range map[string]bool{"card-preview": true, "card-preview-p3": true, "card-123": true, "card-preview-x": false, "card-preview-p1234": false} {
		if cardIDPattern.MatchString(id) != ok {
			t.Errorf("%s: harus %v", id, ok)
		}
	}
	if !isPreviewID("card-preview-p2") || isPreviewID("card-123") {
		t.Error("isPreviewID salah")
	}
}

// Folder pratinjau yang 24 jam tak tersentuh disapu; kartu tersimpan dan
// pratinjau yang baru dipakai tetap ada.
func TestSweepPreviews(t *testing.T) {
	root := t.TempDir()
	s := &Server{layout: config.Layout{CardsDir: root}}
	old := time.Now().Add(-48 * time.Hour)
	for name, aged := range map[string]bool{"card-preview-p3": true, "card-preview-p4": false, "card-preview": true, "card-123": true} {
		d := filepath.Join(root, name)
		os.MkdirAll(d, 0o755)
		f := filepath.Join(d, "card.png")
		os.WriteFile(f, []byte("x"), 0o644)
		if aged {
			os.Chtimes(f, old, old)
		}
	}
	s.sweepPreviews(time.Now())
	for name, keep := range map[string]bool{"card-preview-p3": false, "card-preview": false, "card-preview-p4": true, "card-123": true} {
		if _, err := os.Stat(filepath.Join(root, name)); (err == nil) != keep {
			t.Errorf("%s: harus tetap ada = %v", name, keep)
		}
	}
}

// Tulisan atomik yang gagal tidak meninggalkan berkas .tmp.
func TestWriteFileAtomicCleansUp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sub") // folder → Rename berkas ke sana gagal
	os.MkdirAll(filepath.Join(target, "isi"), 0o755)
	if err := writeFileAtomic(dir, target, []byte("x")); err == nil {
		t.Fatal("rename ke folder berisi harus gagal")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(left) != 0 {
		t.Fatalf("berkas sementara tertinggal: %v", left)
	}
}
