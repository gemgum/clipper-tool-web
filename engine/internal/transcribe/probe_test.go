package transcribe

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Probe mengirim audio contoh sebagai multipart + verbose_json, dan menolak
// balasan tanpa timestamp (tidak bisa dipakai untuk subtitle).
func TestProbe(t *testing.T) {
	var gotPath, gotFormat string
	var gotBytes int
	reply := `{"text":"hello","segments":[{"start":0,"end":1.2,"text":" Hello."},{"start":1.2,"end":5,"text":" This is a test."}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotFormat = r.FormValue("response_format")
		if f, _, err := r.FormFile("file"); err == nil {
			b, _ := io.ReadAll(f)
			gotBytes = len(b)
		}
		w.Write([]byte(reply))
	}))
	defer srv.Close()
	o := &OpenAIAudio{Name: "t", Base: srv.URL + "/api/v1", Key: "k", Model: "openai/whisper-large-v3-turbo"}

	text, err := o.Probe(context.Background(), "")
	if err != nil || text != "Hello. This is a test." {
		t.Fatalf("text=%q err=%v", text, err)
	}
	if gotPath != "/api/v1/audio/transcriptions" || gotFormat != "verbose_json" || gotBytes != len(testSpeech) {
		t.Fatalf("path=%s format=%s bytes=%d", gotPath, gotFormat, gotBytes)
	}

	reply = `{"text":"hello"}` // tanpa segmen = tanpa timestamp
	if _, err := o.Probe(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "without timestamps") {
		t.Fatalf("untimed reply accepted: %v", err)
	}
}
