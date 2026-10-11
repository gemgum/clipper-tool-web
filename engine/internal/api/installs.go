package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gemgum/clipper/engine/internal/setup"
)

// Pemasangan komponen berjalan di LATAR, bukan di dalam permintaan HTTP.
//
// Sebelumnya unduhan hidup di dalam permintaan halaman: `setup.Install` dipanggil
// dengan `r.Context()`, jadi begitu koneksi halaman putus — pengguna pindah
// jendela dan Windows menidurkan WebView2 — konteksnya dibatalkan dan unduhan
// 111 MB berhenti di tengah. Dari sisi pengguna terlihat "mengulang dari nol".
//
// Pola yang benar sudah ada di paket `job` sejak awal (notes/23: "pakai ulang,
// jangan bikin mekanisme kedua"): pekerjaan berjalan sendiri, GUI hanya
// berlangganan kabarnya. Berkas ini menerapkannya untuk pemasangan komponen:
//
//	POST /api/requirements/install   memulai, langsung menjawab
//	GET  /api/requirements/events    menonton, boleh tersambung & putus kapan pun
//
// Akibatnya halaman Requirements tidak lagi mengendalikan apa pun. Ia bisa
// ditutup, dimuat ulang, atau ditinggal — pemasangannya jalan terus.

// InstallState adalah keadaan satu pemasangan yang bisa dilihat GUI.
type InstallState struct {
	ID      string  `json:"id"`
	Running bool    `json:"running"`
	Value   float64 `json:"value"` // 0..1, -1 bila besarnya tidak diketahui
	Message string  `json:"message"`
	Bytes   int64   `json:"bytes"`
	Total   int64   `json:"total"`
	Error   string  `json:"error,omitempty"`
	Done    bool    `json:"done"`
}

// installs menyimpan pemasangan yang sedang & baru saja berjalan, dan
// menyiarkan perubahannya ke pelanggan SSE.
type installs struct {
	mu    sync.RWMutex
	state map[string]*InstallState
	subs  map[chan InstallState]struct{}
}

func (in *installs) init() {
	if in.state == nil {
		in.state = map[string]*InstallState{}
		in.subs = map[chan InstallState]struct{}{}
	}
}

// start menandai satu komponen mulai dipasang. false = sedang berjalan.
func (in *installs) start(id string) bool {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.init()
	if st, ok := in.state[id]; ok && st.Running {
		return false
	}
	in.state[id] = &InstallState{ID: id, Running: true, Message: "Starting…"}
	return true
}

// update menyimpan kemajuan & menyiarkannya.
func (in *installs) update(id string, fn func(*InstallState)) {
	in.mu.Lock()
	in.init()
	st, ok := in.state[id]
	if !ok {
		st = &InstallState{ID: id}
		in.state[id] = st
	}
	fn(st)
	snapshot := *st
	subs := make([]chan InstallState, 0, len(in.subs))
	for c := range in.subs {
		subs = append(subs, c)
	}
	in.mu.Unlock()

	for _, c := range subs {
		// Pelanggan yang lambat dilewati, bukan ditunggu: satu halaman yang
		// membeku tidak boleh menghentikan unduhan yang sedang berjalan.
		select {
		case c <- snapshot:
		default:
		}
	}
}

// snapshot mengembalikan seluruh keadaan yang diketahui — dikirim ke pelanggan
// baru supaya halaman yang baru dibuka langsung melihat yang sedang berjalan.
func (in *installs) snapshot() []InstallState {
	in.mu.RLock()
	defer in.mu.RUnlock()
	out := make([]InstallState, 0, len(in.state))
	for _, st := range in.state {
		out = append(out, *st)
	}
	return out
}

func (in *installs) subscribe() chan InstallState {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.init()
	c := make(chan InstallState, 64)
	in.subs[c] = struct{}{}
	return c
}

func (in *installs) unsubscribe(c chan InstallState) {
	in.mu.Lock()
	delete(in.subs, c)
	in.mu.Unlock()
	close(c)
}

// installComponent memulai pemasangan lalu LANGSUNG menjawab.
//
// Tidak menunggu selesai: itulah seluruh maksud perubahan ini. Kemajuannya
// diikuti lewat /api/requirements/events.
func (s *Server) installComponent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.ID == "" {
		writeErr(w, 400, "the 'id' field is required")
		return
	}
	if !s.installs.start(req.ID) {
		writeErr(w, 409, "that component is already being installed")
		return
	}

	id := req.ID
	// context.Background(), BUKAN r.Context(): pemasangan tidak boleh mati
	// bersama halaman yang memulainya.
	go func() {
		err := setup.Install(context.Background(), s.layout, id, func(p setup.Progress) {
			s.installs.update(id, func(st *InstallState) {
				st.Value, st.Message, st.Bytes, st.Total = p.Value, p.Message, p.Bytes, p.Total
			})
		})
		s.installs.update(id, func(st *InstallState) {
			st.Running, st.Done = false, true
			if err != nil {
				st.Error = err.Error()
				st.Message = "Failed"
				return
			}
			st.Value, st.Message = 1, "Installed"
		})
		if err == nil {
			// Letak program dibaca ulang SEKARANG, bukan saat aplikasi
			// dijalankan lagi. Server menyimpan Paths sejak dibuat, jadi ffmpeg
			// yang baru saja diunduh tidak akan terpakai sampai proses ini
			// dimulai ulang — dilaporkan dari lapangan sebagai "sudah install
			// tapi harus restart dulu".
			s.applyPaths()
		}
	}()

	writeJSON(w, 202, map[string]any{"id": id, "started": true})
}

// installEvents mengalirkan kemajuan semua pemasangan.
//
// Boleh disambung ulang kapan saja: pesan pertama selalu berisi keadaan
// terkini, jadi halaman yang baru dibuka langsung tahu apa yang sedang
// berjalan tanpa perlu menebak.
func (s *Server) installEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming is not supported by this connection")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)

	for _, st := range s.installs.snapshot() {
		writeSSE(w, "install", st)
	}
	writeSSE(w, "ready", map[string]any{"ok": true})
	flusher.Flush()

	ch := s.installs.subscribe()
	defer s.installs.unsubscribe(ch)

	// Denyut berkala menjaga sambungan hidup lewat perantara yang memutus
	// koneksi diam, dan membuat halaman tahu engine masih ada.
	tick := time.NewTicker(20 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case st := <-ch:
			writeSSE(w, "install", st)
			flusher.Flush()
		case <-tick.C:
			writeSSE(w, "ping", map[string]any{})
			flusher.Flush()
		}
	}
}
