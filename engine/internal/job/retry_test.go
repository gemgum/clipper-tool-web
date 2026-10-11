package job

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/types"
)

// Coba lagi & render ulang menolak dengan kalimat yang bisa langsung dibaca
// pengguna, dan tidak pernah memasukkan apa pun ke antrian saat ditolak.
func TestRetryAndRerenderRefusals(t *testing.T) {
	m := &Manager{jobs: map[string]*Job{}, queue: make(chan *Job, 4)}
	gone := filepath.Join(t.TempDir(), "hilang.mp4")
	m.jobs["a"] = &Job{ID: "a", Status: StatusDone, Input: gone, Options: config.Options{}, subs: map[chan Event]struct{}{}}
	m.jobs["b"] = &Job{ID: "b", Status: StatusError, Input: gone, subs: map[chan Event]struct{}{}}
	m.jobs["c"] = &Job{ID: "c", Status: StatusDone, Input: gone, subs: map[chan Event]struct{}{},
		Clips: []types.Clip{{ID: "clip_01", Status: "failed"}}}

	if _, err := m.Retry("a"); !errors.Is(err, ErrNotRetryable) {
		t.Errorf("job selesai boleh dicoba lagi? err = %v", err)
	}
	if _, err := m.Retry("b"); !errors.Is(err, ErrSourceGone) {
		t.Errorf("sumber hilang tidak dilaporkan: %v", err)
	}
	if _, err := m.Rerender("a"); !errors.Is(err, ErrNothingFailed) {
		t.Errorf("tanpa klip gagal tetap dirender ulang? err = %v", err)
	}
	if _, err := m.Rerender("c"); !errors.Is(err, ErrSourceGone) {
		t.Errorf("sumber hilang tidak dilaporkan: %v", err)
	}
	if len(m.queue) != 0 {
		t.Errorf("%d job masuk antrian padahal semua ditolak", len(m.queue))
	}
}
