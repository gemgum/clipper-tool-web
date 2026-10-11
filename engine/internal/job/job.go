// Package job mengelola job clipping: state, antrian, dan langganan progres (SSE).
package job

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/pipeline"
	"github.com/gemgum/clipper/engine/internal/types"
)

// Status job.
const (
	StatusQueued   = "queued"
	StatusRunning  = "running"
	StatusDone     = "done"
	StatusError    = "error"
	StatusCanceled = "canceled"
)

// Event dikirim ke pelanggan SSE.
type Event struct {
	Type string      `json:"type"` // progress | clip | done | error
	Data interface{} `json:"data"`
}

// Job satu pekerjaan clipping.
type Job struct {
	ID        string         `json:"id"`
	Dir       string         `json:"dir"` // nama folder unik: <id>_<tanggal_jam>
	Status    string         `json:"status"`
	Stage     string         `json:"stage"`
	Progress  float64        `json:"progress"`
	Error     string         `json:"error,omitempty"`
	Input     string         `json:"input"`
	Options   config.Options `json:"options"`
	Clips     []types.Clip   `json:"clips"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`

	cancel context.CancelFunc
	// rerender: job ini masuk antrian lagi hanya untuk merender ulang klip yang
	// gagal (Manager.Rerender), bukan menjalankan seluruh pipeline.
	rerender bool
	subs     map[chan Event]struct{}
	mu       sync.Mutex
}

// Manager menyimpan seluruh job & mengatur antrian eksekusi.
type Manager struct {
	paths  config.Paths
	layout config.Layout
	mu     sync.RWMutex
	jobs   map[string]*Job
	seq    int
	queue  chan *Job
	apiKey string // API key Claude yang diset dari GUI (menimpa env bila ada)
	// OnDone dipanggil dengan video sumber setiap job yang SELESAI sukses. Mode
	// web memakainya untuk menghapus unggahan (api/web.go); di desktop nil.
	OnDone func(input string)
	// Prepare mengisi ulang koordinat mesin (alamat, nama variabel kunci) di
	// setelan job yang diulang. Koordinat itu json:"-", jadi job dari riwayat
	// di disk tidak membawanya; tanpa ini "Try again" setelah aplikasi dibuka
	// ulang menjalankan job dengan alamat mesin kosong.
	Prepare func(o *config.Options)
}

// SetLayout memperbarui peta folder setelah pengguna mengubah setelannya.
// Job yang sedang berjalan tidak terpengaruh — pathnya sudah ditentukan saat
// job itu dimulai, dan memindahkannya di tengah jalan justru membingungkan.
func (m *Manager) SetLayout(l config.Layout) {
	m.mu.Lock()
	m.layout = l
	m.mu.Unlock()
}

// SetAPIKey menyimpan API key Claude (dari GUI) di memori.
func (m *Manager) SetAPIKey(key string) {
	m.mu.Lock()
	m.apiKey = key
	m.mu.Unlock()
}

// HasAPIKey melaporkan apakah key tersedia (dari GUI atau env).
func (m *Manager) HasAPIKey() bool {
	m.mu.RLock()
	k := m.apiKey
	m.mu.RUnlock()
	return k != "" || m.paths.APIKey != ""
}

// APIKey mengembalikan key Claude yang berlaku (dari GUI, atau dari .env).
// Dipakai fitur di luar pipeline klip — mis. analisis artikel di tab news.
func (m *Manager) APIKey() string { return m.getAPIKey() }

func (m *Manager) getAPIKey() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.apiKey != "" {
		return m.apiKey
	}
	return m.paths.APIKey
}

// NewManager membuat manager dengan batas konkurensi (default 1 = antrian).
func NewManager(l config.Layout, paths config.Paths, concurrency int) *Manager {
	if concurrency < 1 {
		concurrency = 1
	}
	m := &Manager{
		layout: l,
		paths:  paths,
		jobs:   map[string]*Job{},
		queue:  make(chan *Job, 256),
	}
	// Riwayat yang tersimpan di disk dibaca lebih dulu, sebelum permintaan
	// pertama dilayani — lihat store.go.
	m.load()
	for i := 0; i < concurrency; i++ {
		go m.worker()
	}
	return m
}

// worker mengambil job dari antrian satu per satu.
func (m *Manager) worker() {
	for j := range m.queue {
		j.mu.Lock()
		canceled := j.Status == StatusCanceled
		j.mu.Unlock()
		if canceled {
			// Dibatalkan selagi mengantri.
			j.broadcast(Event{Type: "error", Data: map[string]string{"message": "Canceled by the user"}})
			j.closeSubs()
			continue
		}
		j.mu.Lock()
		again := j.rerender
		j.rerender = false
		j.mu.Unlock()
		if again {
			m.runRerender(j)
			continue
		}
		m.run(j)
	}
}

// Create membuat job baru & memasukkannya ke antrian.
func (m *Manager) Create(input string, opts config.Options) *Job {
	m.mu.Lock()
	m.seq++
	now := time.Now()
	id := fmt.Sprintf("job_%04d", m.seq)
	dir := id + "_" + now.Format("2006-01-02_15-04-05") // unik walau id berulang
	j := &Job{
		ID:        id,
		Dir:       dir,
		Status:    StatusQueued,
		Input:     input,
		Options:   opts,
		CreatedAt: now,
		UpdatedAt: now,
		subs:      map[chan Event]struct{}{},
	}
	m.jobs[id] = j
	m.mu.Unlock()

	m.queue <- j
	return j
}

func (m *Manager) run(j *Job) {
	ctx, cancel := context.WithCancel(context.Background())
	j.mu.Lock()
	j.cancel = cancel
	j.Status = StatusRunning
	j.mu.Unlock()

	// Resolve path per-job agar model whisper sesuai opsi job (mis. base vs small).
	m.mu.RLock()
	layout := m.layout
	m.mu.RUnlock()
	paths := config.ResolvePaths(layout, j.Options)
	if k := m.getAPIKey(); k != "" {
		paths.APIKey = k // key dari GUI menimpa env
	}
	workDir := filepath.Join(paths.DataDir, j.Dir)
	outDir := workDir
	if j.Options.OutputDir != "" {
		// Folder output custom pun dapat subfolder per job agar tak bentrok.
		outDir = filepath.Join(j.Options.OutputDir, j.Dir)
	}
	p := pipeline.New(paths, j.Options)

	// Baris pertama log: apa yang dikerjakan dan dengan apa. Tanpa ini berkas
	// lognya cuma daftar tahap tanpa satu pun keterangan tentang JOB MANA —
	// dan justru itu yang ditanyakan pertama kali saat log dilampirkan.
	m.logf(j.ID, "job %s started: %s", j.ID, filepath.Base(j.Input))
	m.logf(j.ID, "whisper=%s scoring=%s output=%s", j.Options.WhisperModel, j.Options.Provider, outDir)

	clips, err := p.Run(ctx, j.ID, j.Input, workDir, outDir, func(pr pipeline.Progress) {
		j.mu.Lock()
		j.Stage = pr.Stage
		j.Progress = pr.Value
		j.UpdatedAt = time.Now()
		if pr.Clip != nil {
			j.Clips = append(j.Clips, *pr.Clip)
		}
		j.mu.Unlock()
		// Yang ditulis ke log sama persis dengan yang selama ini masuk kotak log
		// GUI — jadi memulihkannya dari berkas menghasilkan isi yang sama, bukan
		// versi ringkasnya.
		if pr.Message != "" {
			m.logf(j.ID, "%s: %s", pr.Stage, pr.Message)
		}
		if pr.Summary != "" {
			m.logRaw(j.ID, pr.Summary)
		}
		if pr.Clip != nil {
			m.logf(j.ID, "clip %s scored %d", pr.Clip.ID, pr.Clip.Score)
			j.broadcast(Event{Type: "clip", Data: pr.Clip})
		} else {
			j.broadcast(Event{Type: "progress", Data: pr})
		}
	})

	canceled := ctx.Err() == context.Canceled
	j.mu.Lock()
	switch {
	case canceled || j.Status == StatusCanceled:
		j.Status = StatusCanceled
		j.Error = ""
	case err != nil:
		j.Status = StatusError
		j.Error = err.Error()
	default:
		j.Status = StatusDone
		j.Clips = clips
		j.Progress = 1.0
	}
	j.UpdatedAt = time.Now()
	status, errMsg := j.Status, j.Error
	j.mu.Unlock()

	switch status {
	case StatusError:
		m.logf(j.ID, "⚠ %s", errMsg)
		j.broadcast(Event{Type: "error", Data: map[string]string{"message": errMsg}})
	case StatusCanceled:
		m.logf(j.ID, "⚠ Canceled by the user")
		j.broadcast(Event{Type: "error", Data: map[string]string{"message": "Canceled by the user"}})
	default:
		m.logf(j.ID, "✓ Finished: %d clip(s)", len(clips))
		j.broadcast(Event{Type: "done", Data: map[string]interface{}{"job_id": j.ID, "clips": len(clips)}})
	}
	j.closeSubs()
	// Dicatat ke disk hanya SETELAH selesai: job yang mati bersama aplikasi
	// tidak akan pernah dilanjutkan, jadi menyimpan keadaan tengahnya cuma
	// menaruh baris "0%" abadi di riwayat.
	m.Persist(j.ID)
	m.maybeDone(j, status)
}

// maybeDone memanggil OnDone (mode web: hapus unggahan) hanya bila job selesai
// TANPA klip gagal. Klip gagal masih butuh video sumbernya untuk dirender ulang.
func (m *Manager) maybeDone(j *Job, status string) {
	if status != StatusDone || m.OnDone == nil || failedClips(j) > 0 {
		return
	}
	m.OnDone(j.Input)
	m.logf(j.ID, "uploaded source removed from the server")
}

func failedClips(j *Job) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	n := 0
	for _, c := range j.Clips {
		if c.Status == "failed" {
			n++
		}
	}
	return n
}

// ErrNotRetryable dan kawan-kawan: alasan permintaan ulang ditolak, dalam
// kalimat yang langsung bisa ditampilkan.
var (
	ErrNotRetryable  = errors.New("only a job that failed or was canceled can be tried again")
	ErrNothingFailed = errors.New("this job has no failed clips to render again")
	ErrSourceGone    = errors.New("the source video is no longer on this machine: upload it again and start a new job")
)

// Retry menjalankan ulang job yang gagal/dibatalkan sebagai job BARU dengan
// setelan yang sama ("Coba lagi dari tahap ini", DESIGN.md §9). Tahap yang
// sudah selesai tidak diulang karena hasilnya ada di cache: transkrip
// (data/cache/transcripts) dan koreksinya (data/cache/corrected).
func (m *Manager) Retry(id string) (*Job, error) {
	j, ok := m.Get(id)
	if !ok {
		return nil, os.ErrNotExist
	}
	j.mu.Lock()
	status, input, opts := j.Status, j.Input, j.Options
	j.mu.Unlock()
	if status != StatusError && status != StatusCanceled {
		return nil, ErrNotRetryable
	}
	if _, err := os.Stat(input); err != nil {
		return nil, ErrSourceGone
	}
	if m.Prepare != nil {
		m.Prepare(&opts)
	}
	return m.Create(input, opts), nil
}

// Rerender memasukkan job yang sudah selesai ke antrian lagi untuk merender
// ulang klip berstatus "failed" saja ("Render ulang yang gagal").
func (m *Manager) Rerender(id string) (*Job, error) {
	j, ok := m.Get(id)
	if !ok {
		return nil, os.ErrNotExist
	}
	if failedClips(j) == 0 {
		return nil, ErrNothingFailed
	}
	j.mu.Lock()
	if j.Status == StatusQueued || j.Status == StatusRunning {
		j.mu.Unlock()
		return nil, errors.New("this job is still running")
	}
	input := j.Input
	j.mu.Unlock()
	if _, err := os.Stat(input); err != nil {
		return nil, ErrSourceGone
	}
	j.mu.Lock()
	j.Status, j.Error, j.rerender = StatusQueued, "", true
	j.UpdatedAt = time.Now()
	j.mu.Unlock()
	m.queue <- j
	return j, nil
}

// runRerender: pasangan run() untuk Rerender. Status akhir "done" walau ada
// klip yang gagal lagi — klipnya sendiri yang membawa sebabnya.
func (m *Manager) runRerender(j *Job) {
	ctx, cancel := context.WithCancel(context.Background())
	j.mu.Lock()
	j.cancel = cancel
	j.Status = StatusRunning
	j.Progress = 0
	clips := append([]types.Clip(nil), j.Clips...)
	j.mu.Unlock()

	m.mu.RLock()
	layout := m.layout
	m.mu.RUnlock()
	paths := config.ResolvePaths(layout, j.Options)
	if k := m.getAPIKey(); k != "" {
		paths.APIKey = k
	}
	workDir := filepath.Join(paths.DataDir, j.Dir)
	outDir := workDir
	if j.Options.OutputDir != "" {
		outDir = filepath.Join(j.Options.OutputDir, j.Dir)
	}
	m.logf(j.ID, "rendering the failed clips again")
	out, err := pipeline.New(paths, j.Options).Rerender(ctx, j.Input, workDir, outDir, clips, func(pr pipeline.Progress) {
		j.mu.Lock()
		j.Stage, j.Progress, j.UpdatedAt = pr.Stage, pr.Value, time.Now()
		j.mu.Unlock()
		if pr.Message != "" {
			m.logf(j.ID, "%s: %s", pr.Stage, pr.Message)
		}
		if pr.Clip != nil {
			j.broadcast(Event{Type: "clip", Data: pr.Clip})
		} else {
			j.broadcast(Event{Type: "progress", Data: pr})
		}
	})

	j.mu.Lock()
	switch {
	case ctx.Err() == context.Canceled || j.Status == StatusCanceled:
		// Dibatalkan: klip lama tetap seperti semula, job kembali "done".
		j.Status, j.Error = StatusDone, ""
	case err != nil:
		j.Status, j.Error = StatusDone, ""
		m.logf(j.ID, "⚠ render again failed: %v", err)
	default:
		j.Status, j.Clips, j.Progress = StatusDone, out, 1.0
	}
	j.UpdatedAt = time.Now()
	n := len(j.Clips)
	j.mu.Unlock()
	if err != nil {
		j.broadcast(Event{Type: "error", Data: map[string]string{"message": err.Error()}})
	} else {
		m.logf(j.ID, "✓ Finished: %d clip(s), %d still failed", n, failedClips(j))
		j.broadcast(Event{Type: "done", Data: map[string]interface{}{"job_id": j.ID, "clips": n}})
	}
	j.closeSubs()
	m.Persist(j.ID)
	m.maybeDone(j, StatusDone)
}

// Get mengembalikan job.
func (m *Manager) Get(id string) (*Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.jobs[id]
	return j, ok
}

// List mengembalikan semua job.
func (m *Manager) List() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, j)
	}
	return out
}

// Cancel membatalkan job.
func (m *Manager) Cancel(id string) bool {
	j, ok := m.Get(id)
	if !ok {
		return false
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	// Bisa membatalkan job yang sedang jalan maupun yang masih mengantri.
	if j.Status == StatusQueued || j.Status == StatusRunning {
		if j.cancel != nil {
			j.cancel()
		}
		j.Status = StatusCanceled
	}
	return true
}

// Subscribe mendaftarkan channel untuk menerima event; kembalikan fungsi unsubscribe.
func (j *Job) Subscribe() (chan Event, func()) {
	ch := make(chan Event, 32)
	j.mu.Lock()
	j.subs[ch] = struct{}{}
	j.mu.Unlock()
	return ch, func() {
		j.mu.Lock()
		if _, ok := j.subs[ch]; ok {
			delete(j.subs, ch)
			close(ch)
		}
		j.mu.Unlock()
	}
}

func (j *Job) broadcast(e Event) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for ch := range j.subs {
		select {
		case ch <- e:
		default: // pelanggan lambat — lewati
		}
	}
}

func (j *Job) closeSubs() {
	j.mu.Lock()
	defer j.mu.Unlock()
	for ch := range j.subs {
		delete(j.subs, ch)
		close(ch)
	}
}

// JobView adalah salinan job untuk serialisasi JSON (tanpa mutex/subs/cancel).
type JobView struct {
	ID        string         `json:"id"`
	Dir       string         `json:"dir"`
	Status    string         `json:"status"`
	Stage     string         `json:"stage"`
	Progress  float64        `json:"progress"`
	Error     string         `json:"error,omitempty"`
	Input     string         `json:"input"`
	Options   config.Options `json:"options"`
	Clips     []types.Clip   `json:"clips"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// Snapshot mengembalikan salinan aman untuk serialisasi JSON.
// ForgetClip membuang satu klip dari daftar job.
//
// Dipanggil sesudah berkasnya dihapus: daftar yang masih menyebut klip yang
// tidak ada lagi akan menawarkan tombol unduh yang pasti gagal.
func (j *Job) ForgetClip(id string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := j.Clips[:0]
	for _, c := range j.Clips {
		if c.ID != id {
			out = append(out, c)
		}
	}
	j.Clips = out
}

func (j *Job) Snapshot() JobView {
	j.mu.Lock()
	defer j.mu.Unlock()
	return JobView{
		ID:        j.ID,
		Dir:       j.Dir,
		Status:    j.Status,
		Stage:     j.Stage,
		Progress:  j.Progress,
		Error:     j.Error,
		Input:     j.Input,
		Options:   j.Options,
		Clips:     append([]types.Clip(nil), j.Clips...),
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
	}
}
