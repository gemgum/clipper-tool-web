package httpx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Yang diuji: 429/5xx diulang DENGAN badan permintaan yang utuh, dan 400 tidak.
// Badan yang utuh itu intinya — percobaan kedua yang mengirim badan kosong akan
// "berhasil" di tingkat HTTP lalu gagal beberapa lapis kemudian.
func TestRetry(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		want   int
	}{
		{"rate limit", 429, 3},
		{"bad gateway", 502, 3},
		{"bad request", 400, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hits := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits++
				if b, _ := io.ReadAll(r.Body); string(b) != "ping" {
					t.Errorf("attempt %d got body %q, want %q", hits, b, "ping")
				}
				w.Header().Set("Retry-After", "1") // jeda uji, bukan 1s/2s berlipat
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader("ping"))
			resp, err := (&http.Client{Transport: Retry(nil)}).Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if hits != tc.want {
				t.Errorf("server saw %d requests, want %d", hits, tc.want)
			}
			if resp.StatusCode != tc.status {
				t.Errorf("status %d, want %d", resp.StatusCode, tc.status)
			}
		})
	}
}
