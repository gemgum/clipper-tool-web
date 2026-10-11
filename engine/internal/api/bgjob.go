package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Job latar yang BUKAN job klip: pembuat berita (posts.go) dan pembuat caption
// (captions.go).
//
// Keduanya berbentuk sama persis — mulai, catat kemajuannya, siarkan ke halaman
// yang sedang menonton, simpan hasilnya — jadi bentuk itu hidup di satu tempat
// dan dibedakan hanya oleh tipe hasilnya. Sengaja tidak digabung dengan
// job.Manager: job klip punya antrian, riwayat di disk, dan folder kerja per
// job, sementara ini cuma jendela ke pekerjaan yang sedang berjalan.
//
// Disimpan di memori saja: hasil kerjanya sudah ada di foldernya sendiri.

// bgJob keadaan satu job latar.
type bgJob[T any] struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"` // running | done | error | canceled
	Stage     string    `json:"stage"`
	Progress  float64   `json:"progress"`
	Log       []string  `json:"log"`
	Error     string    `json:"error,omitempty"`
	Result    *T        `json:"result,omitempty"`
	CreatedAt time.Time `json:"created_at"`

	// cancel menghentikan job yang sedang jalan. Tidak ikut ke JSON — ia fungsi,
	// dan lagipula pemanggilnya cukup tahu Status.
	cancel context.CancelFunc `json:"-"`
}

// maxBGLog membatasi baris log yang disimpan per job. Satu job menghasilkan
// puluhan baris, bukan ribuan; batas ini menjaga job yang mengamuk tidak
// menghabiskan memori.
const maxBGLog = 500

// bgStore menyimpan job latar sejenis beserta pelanggan SSE-nya. Nilai nolnya
// langsung bisa dipakai.
type bgStore[T any] struct {
	mu     sync.RWMutex
	seq    int
	prefix string // awalan id, mis. "post" → post_0001
	jobs   map[string]*bgJob[T]
	subs   map[chan bgJob[T]]struct{}
	// dir: folder tempat job yang SELESAI ditulis (<DataDir>/runs/<awalan>).
	// Kosong = tidak disimpan (test). Riwayat bersama (DESIGN-Clipper-Lanjutan
	// §6) butuh hasil pembuat berita, caption, dan watermark bertahan setelah
	// aplikasi ditutup — sebelumnya hanya klip & kartu yang bertahan.
	dir string
}

// persistTo membaca job yang tersimpan di dir lalu menyimpan job berikutnya
// ke sana. Nomor id melanjutkan yang terbesar supaya tidak ada yang tertimpa.
func (p *bgStore[T]) persistTo(dir string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	p.dir = dir
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var j bgJob[T]
		if json.Unmarshal(raw, &j) != nil || j.ID == "" {
			continue
		}
		// Job yang tersimpan saat "running" mati bersama aplikasinya.
		if j.Status == "running" {
			j.Status, j.Stage = "error", "error"
			if j.Error == "" {
				j.Error = "the app was closed while this job was running"
			}
		}
		jj := j
		p.jobs[j.ID] = &jj
		if i := strings.LastIndexByte(j.ID, '_'); i >= 0 {
			if n, err := strconv.Atoi(j.ID[i+1:]); err == nil && n > p.seq {
				p.seq = n
			}
		}
	}
}

// save menulis satu job ke dir (ditulis ke berkas sementara lalu diganti nama,
// supaya aplikasi yang mati di tengah tidak meninggalkan JSON setengah jadi).
func (p *bgStore[T]) save(j bgJob[T]) {
	if p.dir == "" {
		return
	}
	if err := os.MkdirAll(p.dir, 0o755); err != nil {
		return
	}
	raw, err := json.Marshal(j)
	if err != nil {
		return
	}
	tmp := filepath.Join(p.dir, j.ID+".json.tmp")
	if os.WriteFile(tmp, raw, 0o644) == nil {
		_ = os.Rename(tmp, filepath.Join(p.dir, j.ID+".json"))
	}
}

func (p *bgStore[T]) init() {
	if p.jobs == nil {
		p.jobs = map[string]*bgJob[T]{}
		p.subs = map[chan bgJob[T]]struct{}{}
	}
}

func (p *bgStore[T]) create(prefix string, cancel context.CancelFunc) *bgJob[T] {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	p.seq++
	j := &bgJob[T]{
		ID:        fmt.Sprintf("%s_%04d", prefix, p.seq),
		Status:    "running",
		Stage:     "queued",
		CreatedAt: time.Now(),
		cancel:    cancel,
	}
	p.jobs[j.ID] = j
	return j
}

// update menyimpan perubahan lalu menyiarkannya.
func (p *bgStore[T]) update(id string, fn func(*bgJob[T])) {
	p.mu.Lock()
	p.init()
	j, ok := p.jobs[id]
	if !ok {
		p.mu.Unlock()
		return
	}
	fn(j)
	if len(j.Log) > maxBGLog {
		j.Log = j.Log[len(j.Log)-maxBGLog:]
	}
	snapshot := *j
	subs := make([]chan bgJob[T], 0, len(p.subs))
	for c := range p.subs {
		subs = append(subs, c)
	}
	p.mu.Unlock()

	for _, c := range subs {
		// Pelanggan yang lambat dilewati, bukan ditunggu — satu halaman yang
		// membeku tidak boleh menghentikan job yang sedang berjalan.
		select {
		case c <- snapshot:
		default:
		}
	}
}

// stop menghentikan job yang sedang berjalan. Mengembalikan false bila job-nya
// tidak ada; job yang sudah selesai dianggap berhasil dibatalkan supaya tombol
// di GUI tidak melaporkan galat untuk keadaan yang tidak salah.
func (p *bgStore[T]) stop(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	j, ok := p.jobs[id]
	if !ok {
		return false
	}
	if j.Status == "running" && j.cancel != nil {
		j.cancel()
	}
	return true
}

// remove membuang job yang sudah berhenti dari riwayat (memori + berkasnya).
// Hasil kerjanya (berkas .txt, video, artikel) tidak disentuh.
func (p *bgStore[T]) remove(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	j, ok := p.jobs[id]
	if !ok {
		return os.ErrNotExist
	}
	if j.Status == "running" {
		return fmt.Errorf("this job is still running: cancel it first")
	}
	delete(p.jobs, id)
	if p.dir != "" {
		if err := os.Remove(filepath.Join(p.dir, id+".json")); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// deleteHandler = DELETE /api/<jenis>/{id} untuk store ini.
func (p *bgStore[T]) deleteHandler(w http.ResponseWriter, r *http.Request) {
	if err := p.remove(r.PathValue("id")); err != nil {
		if os.IsNotExist(err) {
			writeErr(w, 404, "job not found")
			return
		}
		writeErr(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"deleted": true})
}

func (p *bgStore[T]) get(id string) (bgJob[T], bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	j, ok := p.jobs[id]
	if !ok {
		var zero bgJob[T]
		return zero, false
	}
	return *j, true
}

// snapshot mengembalikan seluruh job, terbaru dulu. Diurutkan menurut waktu
// dibuat: urutan map Go acak, jadi membalik hasil iterasinya (cara lama) tidak
// pernah menjamin "terbaru dulu".
func (p *bgStore[T]) snapshot() []bgJob[T] {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]bgJob[T], 0, len(p.jobs))
	for _, j := range p.jobs {
		out = append(out, *j)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].CreatedAt.After(out[b].CreatedAt) })
	return out
}

func (p *bgStore[T]) subscribe() chan bgJob[T] {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	c := make(chan bgJob[T], 64)
	p.subs[c] = struct{}{}
	return c
}

func (p *bgStore[T]) unsubscribe(c chan bgJob[T]) {
	p.mu.Lock()
	delete(p.subs, c)
	p.mu.Unlock()
	close(c)
}

// finish menutup satu job: galat, dibatalkan, atau selesai dengan hasilnya.
//
// Dipakai bersama kedua jenis job supaya keduanya memperlakukan pembatalan
// dengan cara yang sama — dibatalkan pengguna BUKAN galat, dan menampilkannya
// merah membuat tombol yang baru saja ditekan terlihat rusak.
func (p *bgStore[T]) finish(id string, ctx context.Context, res T, err error) {
	p.update(id, func(j *bgJob[T]) {
		switch {
		case err != nil && ctx.Err() != nil:
			j.Status, j.Stage, j.Error = "canceled", "canceled", ""
		case err != nil:
			j.Status, j.Stage, j.Error = "error", "error", err.Error()
		default:
			j.Status, j.Stage, j.Progress = "done", "done", 1
			j.Result = &res
		}
	})
	if j, ok := p.get(id); ok {
		p.save(j)
	}
}

// stream mengalirkan kemajuan SELURUH job dalam satu store lewat SSE.
//
// Boleh disambung ulang kapan saja: pesan pertama berisi keadaan terkini, jadi
// halaman yang baru dibuka langsung tahu apa yang sedang berjalan.
func (p *bgStore[T]) stream(w http.ResponseWriter, r *http.Request, event string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming is not supported by this connection")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)

	for _, j := range p.snapshot() {
		writeSSE(w, event, j)
	}
	writeSSE(w, "ready", map[string]any{"ok": true})
	flusher.Flush()

	ch := p.subscribe()
	defer p.unsubscribe(ch)

	tick := time.NewTicker(20 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case j := <-ch:
			writeSSE(w, event, j)
			flusher.Flush()
		case <-tick.C:
			writeSSE(w, "ping", map[string]any{})
			flusher.Flush()
		}
	}
}
