package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
)

// Template watermark: simpan, simpan lagi dengan nama sama = menimpa (bukan
// dobel), lalu hapus. Isi yang bukan objek JSON ditolak.
func TestWatermarkTemplates(t *testing.T) {
	s := &Server{paths: config.Paths{DataDir: t.TempDir()}}
	save := func(body string) string {
		rec := httptest.NewRecorder()
		s.saveTemplate(rec, httptest.NewRequest("POST", "/api/watermark/templates", strings.NewReader(body)))
		return rec.Body.String()
	}
	if got := save(`{"name":"TIMES","data":"teks"}`); !strings.Contains(got, "error") {
		t.Fatalf("data bukan objek diterima: %s", got)
	}
	save(`{"name":"TIMES","data":{"format":"feed45"}}`)
	got := save(`{"name":"TIMES","data":{"format":"feed11"}}`)
	if strings.Count(got, `"name":"TIMES"`) != 1 || !strings.Contains(got, "feed11") {
		t.Fatalf("nama sama seharusnya menimpa: %s", got)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/watermark/templates/TIMES", nil)
	req.SetPathValue("name", "TIMES")
	s.deleteTemplate(rec, req)
	if len(s.readTemplates()) != 0 {
		t.Fatalf("template tidak terhapus: %s", rec.Body.String())
	}
}

// Template bawaan: paling banyak satu; memilih yang lain memindahkannya, memilih
// yang sama mencabutnya, dan menimpa template tidak menghapus tandanya.
func TestDefaultTemplate(t *testing.T) {
	s := &Server{paths: config.Paths{DataDir: t.TempDir()}}
	for _, n := range []string{"A", "B"} {
		s.saveTemplate(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"`+n+`","data":{}}`)))
	}
	def := func(name string) {
		req := httptest.NewRequest("POST", "/", strings.NewReader("{}"))
		req.SetPathValue("name", name)
		s.setDefaultTemplate(httptest.NewRecorder(), req)
	}
	which := func() (out string) {
		for _, t := range s.readTemplates() {
			if t.Default {
				out += t.Name
			}
		}
		return
	}
	def("A")
	def("B")
	if got := which(); got != "B" {
		t.Fatalf("bawaan harus B saja, dapat %q", got)
	}
	s.saveTemplate(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"B","data":{"x":1}}`)))
	if got := which(); got != "B" {
		t.Fatalf("menimpa B mencabut tandanya: %q", got)
	}
	def("B")
	if got := which(); got != "" {
		t.Fatalf("memilih bawaan yang sama harus mencabutnya: %q", got)
	}
}
