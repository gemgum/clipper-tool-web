package job

import (
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
)

// Log job harus bisa dibaca ulang persis seperti ditulis — itulah seluruh
// gunanya: kotak log di GUI dipulihkan dari berkas ini setiap halaman dipasang.
func TestLogRoundTrip(t *testing.T) {
	m := NewManager(config.Layout{DataDir: t.TempDir()}, config.Paths{}, 1)

	m.logf("job_0001", "job %s started: %s", "job_0001", "video.mp4")
	m.logf("job_0001", "%s: %s", "transcribing", "whisper is running")
	// Tabel ringkasan waktu: banyak baris, tanpa cap waktu, perataannya wajib utuh.
	m.logRaw("job_0001", "stage      seconds\nextract      12.0\nrender       48.5")
	m.logf("job_0001", "✓ Finished: %d clip(s)", 3)

	lines := m.ReadLog("job_0001")
	if len(lines) != 6 {
		t.Fatalf("baris = %d, mau 6: %q", len(lines), lines)
	}
	if !strings.HasSuffix(lines[0], "job job_0001 started: video.mp4") {
		t.Errorf("baris pertama = %q", lines[0])
	}
	// Baris tabel TIDAK boleh mendapat cap waktu; satu saja merusak kolomnya.
	if lines[3] != "extract      12.0" {
		t.Errorf("baris tabel = %q, mau tanpa cap waktu", lines[3])
	}
	if !strings.HasPrefix(lines[0], "[") || strings.HasPrefix(lines[3], "[") {
		t.Errorf("cap waktu salah tempat: %q / %q", lines[0], lines[3])
	}
}

// Job dari riwayat versi lama tidak punya berkas log. Halaman yang
// menampilkannya tidak boleh gagal karenanya.
func TestReadLogMissingIsEmpty(t *testing.T) {
	m := NewManager(config.Layout{DataDir: t.TempDir()}, config.Paths{}, 1)
	if got := m.ReadLog("job_9999"); len(got) != 0 {
		t.Fatalf("log tak ada = %q, mau kosong", got)
	}
}
