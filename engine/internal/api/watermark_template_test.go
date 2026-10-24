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
