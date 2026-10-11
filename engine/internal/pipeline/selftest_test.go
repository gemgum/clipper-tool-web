package pipeline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeOllama = server Ollama palsu: satu model terpasang, dan balasan /api/chat
// ditentukan pemanggil. Yang diuji BUKAN model sungguhan melainkan apakah
// SelfTest MENGENALI kegagalan yang selama ini lolos dari sapaan satu kata.
func fakeOllama(t *testing.T, chat func(system, user string) string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Header().Set("content-type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"name":"test:latest","size":5000000000,` +
				`"details":{"parameter_size":"8.0B","quantization_level":"Q4_K_M","context_length":8192},` +
				`"capabilities":["completion"]}]}`))
		case "/api/chat":
			var req struct {
				Messages []struct{ Role, Content string } `json:"messages"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			system, user := "", ""
			for _, m := range req.Messages {
				if m.Role == "system" {
					system = m.Content
				} else {
					user = m.Content
				}
			}
			w.Header().Set("content-type", "application/json")
			out, _ := json.Marshal(map[string]any{"message": map[string]string{"content": chat(system, user)}})
			_, _ = w.Write(out)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// isPick membedakan permintaan pemilihan momen dari permintaan koreksi tanpa
// menebak urutan pemanggilan.
func isPick(user string) bool { return strings.Contains(user, "Candidates:") }

func allSegments(n int) string {
	var b strings.Builder
	b.WriteString(`{"segments":[`)
	for i := range n {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"index":`)
		b.WriteString(string(rune('0' + i)))
		b.WriteString(`,"text":"Kalimat contoh nomor `)
		b.WriteString(string(rune('0' + i)))
		b.WriteString(` yang panjangnya wajar."}`)
	}
	b.WriteString(`]}`)
	return b.String()
}

func run(t *testing.T, chat func(system, user string) string) []SelfTestStep {
	t.Helper()
	_, steps := SelfTest(context.Background(), fakeOllama(t, chat), "test")
	if len(steps) == 0 || !steps[0].OK {
		t.Fatalf("connect step failed: %+v", steps)
	}
	return steps
}

func TestSelfTestPasses(t *testing.T) {
	steps := run(t, func(system, user string) string {
		if isPick(user) {
			return `{"picks":[{"index":1,"score":82,"title":"Kemasan yang salah","hashtags":["#usaha"]}]}`
		}
		return allSegments(len(selfTestTranscript.Segments))
	})
	for _, st := range steps {
		if !st.OK {
			t.Errorf("step %q failed: %s", st.Name, st.Error)
		}
	}
}

// Model yang membalas SEBAGIAN segmen adalah kegagalan yang paling sering
// terlihat di model kecil, dan di job ia tidak muncul sebagai galat — hanya
// kalimat yang tidak pernah terkoreksi.
func TestSelfTestCatchesMissingSegments(t *testing.T) {
	steps := run(t, func(system, user string) string {
		if isPick(user) {
			return `{"picks":[{"index":0,"score":70,"title":"Judul","hashtags":[]}]}`
		}
		return `{"segments":[{"index":0,"text":"Kalimat contoh nomor 0 yang panjangnya wajar."}]}`
	})
	if steps[1].OK {
		t.Fatalf("transcript correction should fail when segments are unanswered: %+v", steps[1])
	}
}

// Nomor kandidat di luar daftar = model mengarang. Di job pilihan itu dibuang
// diam-diam, jadi gejalanya cuma "klipnya lebih sedikit dari yang diminta".
func TestSelfTestCatchesInventedIndex(t *testing.T) {
	steps := run(t, func(system, user string) string {
		if isPick(user) {
			return `{"picks":[{"index":9,"score":90,"title":"Judul","hashtags":[]}]}`
		}
		return allSegments(len(selfTestTranscript.Segments))
	})
	if !steps[1].OK {
		t.Fatalf("transcript correction should pass here: %s", steps[1].Error)
	}
	if steps[2].OK {
		t.Fatalf("moment selection should fail on an out-of-range index: %+v", steps[2])
	}
}
