package job

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
)

// Saat engine mulai, folder kerja tmp/ semua job dibuang (sisa job terputus /
// versi lama); klipnya tetap.
func TestLoadRemovesJobTmp(t *testing.T) {
	root := t.TempDir()
	l := config.Layout{DataDir: root}
	jobDir := "job_0001_2026-10-10_01-36-41"
	wav := filepath.Join(root, jobDir, "tmp", "audio.wav")
	clip := filepath.Join(root, jobDir, "clip_01.mp4")
	os.MkdirAll(filepath.Dir(wav), 0o755)
	os.WriteFile(wav, []byte("x"), 0o644)
	os.WriteFile(clip, []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(root, "jobs"), 0o755)
	os.WriteFile(filepath.Join(root, "jobs", "job_0001.json"),
		[]byte(`{"id":"job_0001","dir":"`+jobDir+`","status":"done","input":"/x.mp4"}`), 0o644)

	NewManager(l, config.Paths{}, 1)

	if _, err := os.Stat(filepath.Dir(wav)); !os.IsNotExist(err) {
		t.Fatal("job tmp/ kept")
	}
	if _, err := os.Stat(clip); err != nil {
		t.Fatal("clip removed")
	}
}
