package api

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/job"
)

// Sapuan: unduhan lama yang tidak dipakai dan sisa unduhan terputus hilang;
// yang dipakai job, yang masih baru, dan unggahan biasa tetap ada.
func TestSweepFetched(t *testing.T) {
	root := t.TempDir()
	l := config.Layout{DataDir: filepath.Join(root, "data")}
	s := &Server{paths: config.Paths{DataDir: l.DataDir}, mgr: job.NewManager(l, config.Paths{}, 1)}
	old := time.Now().Add(-72 * time.Hour)
	mk := func(rel string, age time.Time) string {
		p := filepath.Join(s.uploadDir(), rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, age, age)
		os.Chtimes(filepath.Dir(p), age, age)
		return p
	}
	used := mk("yt-used/a.mp4", old)
	unused := mk("yt-unused/b.mp4", old)
	young := mk("yt-young/c.mp4", time.Now())
	partial := mk("yt-young/.dl-1/video.mp4", old)
	plain := mk("123/upload.mp4", old)
	os.Chtimes(filepath.Dir(filepath.Dir(partial)), time.Now(), time.Now())

	usedAbs, _ := filepath.Abs(used)
	s.mgr.Create(usedAbs, config.DefaultOptions())
	s.sweepFetched(time.Now())

	for p, want := range map[string]bool{used: true, unused: false, young: true, partial: false, plain: true} {
		if _, err := os.Stat(p); (err == nil) != want {
			t.Errorf("%s exists=%v, want %v", p, err == nil, want)
		}
	}
}

// Hapus job dari History → video sumbernya ikut, kecuali masih dipakai job lain.
func TestDeleteJobDropsSource(t *testing.T) {
	root := t.TempDir()
	l := config.Layout{DataDir: filepath.Join(root, "data")}
	s := &Server{paths: config.Paths{DataDir: l.DataDir}, mgr: job.NewManager(l, config.Paths{}, 1)}
	webRoot = s.uploadDir()
	t.Cleanup(func() { webRoot = "" })
	src := filepath.Join(s.uploadDir(), "yt-x", "v.mp4")
	os.MkdirAll(filepath.Dir(src), 0o755)
	os.WriteFile(src, []byte("x"), 0o644)

	a := s.mgr.Create(src, config.DefaultOptions())
	b := s.mgr.Create(src, config.DefaultOptions())
	del := func(id string) {
		for i := 0; i < 100; i++ { // tunggu job (yang pasti gagal: tanpa ffmpeg) berhenti
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("DELETE", "/api/jobs/"+id, nil)
			req.SetPathValue("id", id)
			s.deleteJob(rec, req)
			if rec.Code == 200 {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("job %s never stopped", id)
	}
	del(a.ID)
	if _, err := os.Stat(src); err != nil {
		t.Fatal("source removed while another job still uses it")
	}
	del(b.ID)
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source kept after its last job was deleted")
	}
}

// Remove di kartu Source: unggahan bebas dihapus; yang dipakai job dan berkas di
// luar folder unggahan tidak disentuh.
func TestRemoveUpload(t *testing.T) {
	root := t.TempDir()
	l := config.Layout{DataDir: filepath.Join(root, "data")}
	s := &Server{paths: config.Paths{DataDir: l.DataDir}, mgr: job.NewManager(l, config.Paths{}, 1)}
	webRoot = s.uploadDir()
	t.Cleanup(func() { webRoot = "" })
	mk := func(p string) string {
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
		return p
	}
	free := mk(filepath.Join(s.uploadDir(), "1", "a.mp4"))
	used := mk(filepath.Join(s.uploadDir(), "2", "b.mp4"))
	outside := mk(filepath.Join(root, "mine.mp4"))
	s.mgr.Create(used, config.DefaultOptions())
	for p, gone := range map[string]bool{free: true, used: false, outside: false} {
		rec := httptest.NewRecorder()
		s.removeUpload(rec, httptest.NewRequest("POST", "/api/upload/remove",
			strings.NewReader(`{"path":"`+p+`"}`)))
		if _, err := os.Stat(p); os.IsNotExist(err) != gone {
			t.Errorf("%s gone=%v, want %v (%s)", p, os.IsNotExist(err), gone, rec.Body)
		}
	}
	if _, err := os.Stat(filepath.Dir(free)); !os.IsNotExist(err) {
		t.Error("empty upload folder left behind")
	}
}
