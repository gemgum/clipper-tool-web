package api

import (
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/ffmpeg"
)

// Video aktif disimpan per akun di server: yang disimpan bisa dibaca kembali,
// berkas yang hilang dilaporkan kosong, dan path di luar folder unggahan ditolak.
func TestActiveSource(t *testing.T) {
	root := t.TempDir()
	s := &Server{paths: config.Paths{DataDir: filepath.Join(root, "data")}}
	webRoot = s.uploadDir()
	t.Cleanup(func() { webRoot = "" })
	vid := filepath.Join(s.uploadDir(), "1", "v.mp4")
	os.MkdirAll(filepath.Dir(vid), 0o755)
	os.WriteFile(vid, []byte("x"), 0o644)

	call := func(method, body string) string {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/source", strings.NewReader(body))
		if method == "GET" {
			s.getSource(rec, req)
		} else {
			s.setSource(rec, req)
		}
		return strings.TrimSpace(rec.Body.String())
	}
	if got := call("POST", `{"path":"`+filepath.Join(root, "elsewhere.mp4")+`"}`); !strings.Contains(got, "error") {
		t.Fatalf("outside path accepted: %s", got)
	}
	call("POST", `{"path":"`+vid+`"}`)
	if got := call("GET", ""); !strings.Contains(got, vid) {
		t.Fatalf("saved source not returned: %s", got)
	}
	os.Remove(vid)
	if got := call("GET", ""); got != `{"path":""}` {
		t.Fatalf("missing file still reported: %s", got)
	}
}

// Cuplikan frame dibuat dari video sungguhan, lalu dipakai ulang dari cache.
func TestThumb(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	root := t.TempDir()
	vid := filepath.Join(root, "v.mp4")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x180:rate=10",
		"-t", "4", "-pix_fmt", "yuv420p", vid).CombinedOutput(); err != nil {
		t.Fatalf("make test video: %v %s", err, out)
	}
	s := &Server{paths: config.Paths{DataDir: filepath.Join(root, "data")}, ff: ffmpeg.New("ffmpeg", "ffprobe")}
	get := func(q string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.thumb(rec, httptest.NewRequest("GET", "/api/thumb?"+q, nil))
		return rec
	}
	rec := get("path=" + vid + "&i=7")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/jpeg" || rec.Body.Len() < 500 {
		t.Fatalf("thumb: %d %s %d bytes", rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
	}
	if n, _ := filepath.Glob(filepath.Join(s.thumbDir(), "*", "7.jpg")); len(n) != 1 {
		t.Fatalf("thumb not cached: %v", n)
	}
	if rec := get("path=" + vid + "&i=8"); rec.Code != 400 {
		t.Fatalf("i out of range accepted: %d", rec.Code)
	}
}
