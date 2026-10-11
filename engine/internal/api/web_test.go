package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Mode web dari ujung ke ujung, lewat Handler() yang sama dengan server asli:
// tanpa cookie hanya /login yang terbuka, kata sandi ditukar jadi cookie, path
// di luar folder unggahan ditolak, dan jelajah folder server mati.
func TestWebMode(t *testing.T) {
	s := &Server{}
	s.paths.DataDir = t.TempDir()
	s.AllowHost("clip.example.com")
	s.SetWeb("sandi-tim")
	t.Cleanup(func() { webRoot = "" })
	h := s.Handler()

	do := func(method, path, body string, c *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "clip.example.com"
		r.Header.Set("Origin", "https://clip.example.com")
		r.Header.Set("Content-Type", "application/json")
		if c != nil {
			r.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}

	if rec := do("GET", "/", "", nil); rec.Code != 303 || rec.Header().Get("Location") != "/login" {
		t.Fatalf("halaman tanpa cookie: %d %q, mau 303 ke /login", rec.Code, rec.Header().Get("Location"))
	}
	if rec := do("GET", "/login", "", nil); rec.Code != 200 {
		t.Fatalf("/login: %d, mau 200", rec.Code)
	}
	if rec := do("GET", "/api/jobs", "", nil); rec.Code != 401 {
		t.Fatalf("API tanpa cookie: %d, mau 401", rec.Code)
	}
	if rec := do("POST", "/api/login", `{"password":"salah"}`, nil); rec.Code != 401 {
		t.Fatalf("sandi salah: %d, mau 401", rec.Code)
	}
	rec := do("POST", "/api/login", `{"password":"sandi-tim"}`, nil)
	cookies := rec.Result().Cookies()
	if rec.Code != 200 || len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatalf("sandi benar: %d %v, mau 200 + satu cookie Secure HttpOnly", rec.Code, cookies)
	}
	if rec := do("GET", "/api/browse?dir=/", "", cookies[0]); rec.Code != 403 {
		t.Fatalf("/api/browse di mode web: %d, mau 403", rec.Code)
	}

	inside := filepath.Join(s.uploadDir(), "a", "v.mp4")
	if got := hostPath(inside); got != inside {
		t.Errorf("path di folder unggahan: %q, mau %q", got, inside)
	}
	for _, p := range []string{"/etc/passwd", s.uploadDir() + "/../.env", s.paths.DataDir} {
		if got := hostPath(p); got != "" {
			t.Errorf("hostPath(%q) = %q, mau ditolak", p, got)
		}
	}

	// Setelah job sukses: video unggahan terhapus, foldernya ikut bila kosong,
	// hasil di sebelahnya (caption .txt) tetap, dan apa pun di luar folder
	// unggahan tidak tersentuh.
	write := func(p string) {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("x"), 0o644)
	}
	alone := filepath.Join(s.uploadDir(), "1", "a.mp4")
	withTxt := filepath.Join(s.uploadDir(), "2", "b.mp4")
	txt := filepath.Join(s.uploadDir(), "2", "b.txt")
	outside := filepath.Join(s.paths.DataDir, "keep.mp4")
	for _, p := range []string{alone, withTxt, txt, outside} {
		write(p)
	}
	dropUploads(alone, withTxt, outside)
	for _, p := range []string{alone, filepath.Dir(alone), withTxt} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s masih ada, mau terhapus", p)
		}
	}
	for _, p := range []string{txt, outside, s.uploadDir()} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s ikut terhapus", p)
		}
	}
}
