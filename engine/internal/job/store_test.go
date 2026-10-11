package job

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/types"
)

// Riwayat harus selamat dari aplikasi yang ditutup — DAN tidak boleh menawarkan
// klip yang berkasnya sudah tidak ada lagi.
func TestPersistThenLoad(t *testing.T) {
	dir := t.TempDir()
	kept := filepath.Join(dir, "kept.mp4")
	if err := os.WriteFile(kept, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := config.Layout{DataDir: dir}

	m := NewManager(l, config.Paths{}, 1)
	m.seq = 7
	m.jobs["job_0007"] = &Job{
		ID: "job_0007", Dir: "job_0007_x", Status: StatusDone, Input: "a.mp4",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Clips: []types.Clip{
			{ID: "c1", VideoPath: kept},
			{ID: "c2", VideoPath: filepath.Join(dir, "gone.mp4")},
		},
		subs: map[chan Event]struct{}{},
	}
	m.Persist("job_0007")

	next := NewManager(l, config.Paths{}, 1)
	jobs := next.List()
	if len(jobs) != 1 {
		t.Fatalf("job termuat = %d, mau 1", len(jobs))
	}
	got := jobs[0].Snapshot()
	if len(got.Clips) != 1 || got.Clips[0].ID != "c1" {
		t.Fatalf("klip termuat = %+v, mau hanya c1 (berkasnya masih ada)", got.Clips)
	}
	// Nomor urut harus lanjut dari disk, kalau tidak job baru menimpa riwayat.
	if id := next.Create("b.mp4", config.Options{}).ID; id != "job_0008" {
		t.Fatalf("id job baru = %q, mau job_0008", id)
	}
}
