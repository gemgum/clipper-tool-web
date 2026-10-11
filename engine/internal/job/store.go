package job

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gemgum/clipper/engine/internal/types"
)

// Riwayat job di DISK: satu berkas JSON per job di <DataDir>/jobs/.
//
// Sampai kini job hanya hidup di memori, jadi halaman Output history kosong
// setiap aplikasi dibuka ulang — sementara riwayat KARTU bertahan karena
// /api/cards membaca foldernya apa adanya. Dua daftar bersebelahan dengan
// aturan yang berbeda, dan tidak ada di layar yang menjelaskan kenapa.
//
// Satu berkas per job, bukan satu indeks: menulis satu job tidak pernah bisa
// merusak riwayat job lain, dan menghapus satu job berarti menghapus satu
// berkas. Berkas yang rusak dilewati saat dibaca, tidak menjatuhkan sisanya.
//
// Yang disimpan hanya CATATANNYA. Klip yang berkas videonya sudah tidak ada
// dibuang saat dibaca: pratinjau dan tombol unduhnya pasti gagal. Job yang
// seluruh klipnya hilang tetap muncul — dengan panel kosong.

const jobsFolder = "jobs"

func (m *Manager) jobsDir() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return filepath.Join(m.layout.DataDir, jobsFolder)
}

// Persist menulis satu job ke disk. Kegagalannya TIDAK menjatuhkan job: riwayat
// yang tidak tercatat menyebalkan, klip yang batal dirender jauh lebih buruk.
func (m *Manager) Persist(id string) {
	j, ok := m.Get(id)
	if !ok {
		return
	}
	dir := m.jobsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	b, err := json.MarshalIndent(j.Snapshot(), "", "  ")
	if err != nil {
		return
	}
	// Tulis ke berkas sementara lalu ganti nama: aplikasi yang ditutup di tengah
	// penulisan meninggalkan berkas separuh, dan berkas separuh adalah job yang
	// hilang dari riwayat.
	tmp := filepath.Join(dir, id+".json.tmp")
	if os.WriteFile(tmp, b, 0o644) != nil {
		return
	}
	os.Rename(tmp, filepath.Join(dir, id+".json"))
}

// load membaca seluruh riwayat dari disk ke dalam manager. Dipanggil sekali
// saat manager dibuat, sebelum permintaan pertama dilayani.
func (m *Manager) load() {
	entries, err := os.ReadDir(m.jobsDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(m.jobsDir(), e.Name()))
		if err != nil {
			continue
		}
		var v JobView
		if json.Unmarshal(b, &v) != nil || v.ID == "" {
			continue
		}
		// Job yang tercatat "sedang berjalan" jelas tidak berjalan lagi —
		// prosesnya sudah mati bersama aplikasi. Melaporkannya running berarti
		// halaman klip menyambung ke SSE job yang tidak akan pernah berbunyi.
		if v.Status == StatusRunning || v.Status == StatusQueued {
			v.Status = StatusCanceled
		}
		v.Clips = existingClips(v.Clips)
		j := &Job{
			ID: v.ID, Dir: v.Dir, Status: v.Status, Stage: v.Stage,
			Progress: v.Progress, Error: v.Error, Input: v.Input,
			Options: v.Options, Clips: v.Clips,
			CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
			subs: map[chan Event]struct{}{},
		}
		m.jobs[v.ID] = j
		// Nomor urut harus lanjut dari yang tertinggi di disk, kalau tidak job
		// berikutnya memakai id yang sudah dipakai dan menimpa riwayatnya.
		if n := seqOf(v.ID); n > m.seq {
			m.seq = n
		}
	}
}

// existingClips membuang klip yang berkas videonya sudah tidak ada.
func existingClips(clips []types.Clip) []types.Clip {
	out := clips[:0]
	for _, c := range clips {
		if c.VideoPath == "" {
			continue
		}
		if _, err := os.Stat(c.VideoPath); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}

// seqOf membaca nomor urut dari id job ("job_0007" → 7). 0 bila bentuknya lain.
func seqOf(id string) int {
	var n int
	if _, err := fmt.Sscanf(id, "job_%d", &n); err != nil {
		return 0
	}
	return n
}

// Delete membuang satu job dari riwayat (memori + berkasnya). Job yang masih
// berjalan ditolak: hentikan dulu. Folder klipnya TIDAK disentuh — itu milik
// pengguna; klip satu per satu dihapus lewat DELETE .../clips/{clip}.
func (m *Manager) Delete(id string) error {
	j, ok := m.Get(id)
	if !ok {
		return os.ErrNotExist
	}
	j.mu.Lock()
	busy := j.Status == StatusQueued || j.Status == StatusRunning
	j.mu.Unlock()
	if busy {
		return fmt.Errorf("this job is still running: cancel it first")
	}
	m.mu.Lock()
	delete(m.jobs, id)
	m.mu.Unlock()
	_ = os.Remove(m.logPath(id))
	if err := os.Remove(filepath.Join(m.jobsDir(), id+".json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
