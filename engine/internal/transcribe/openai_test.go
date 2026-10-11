package transcribe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Permintaan multipart sampai dengan model & format yang benar, segmen
// tergeser ke waktu mutlak, dan model tanpa timestamp ditolak dengan pesan.
func TestOpenAIAudio(t *testing.T) {
	noStamps := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" || r.Header.Get("authorization") != "Bearer k" {
			t.Errorf("alamat/kunci salah: %s %s", r.URL.Path, r.Header.Get("authorization"))
		}
		if r.FormValue("model") != "whisper-1" || r.FormValue("response_format") != "verbose_json" || r.FormValue("language") != "id" {
			t.Errorf("isian salah: %v", r.MultipartForm.Value)
		}
		if noStamps {
			w.Write([]byte(`{"text":"halo semua"}`))
			return
		}
		w.Write([]byte(`{"text":"halo semua","segments":[{"start":1,"end":3,"text":"halo semua"}]}`))
	}))
	defer srv.Close()

	o := &OpenAIAudio{Name: "Custom", Base: srv.URL, Path: "/v1", Key: "k", HTTP: srv.Client(),
		Encode: func(_ context.Context, _, _ float64, out string) error { return os.WriteFile(out, []byte("ogg"), 0o644) }}
	tr, err := o.Transcribe(context.Background(), 700, "id", t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Segments) != 2 || tr.Segments[1].Start != 601 {
		t.Fatalf("segmen = %+v", tr.Segments)
	}
	noStamps = true
	if _, err := o.Transcribe(context.Background(), 10, "id", t.TempDir(), nil); err == nil || !strings.Contains(err.Error(), "without timestamps") {
		t.Fatalf("galat = %v", err)
	}
}
