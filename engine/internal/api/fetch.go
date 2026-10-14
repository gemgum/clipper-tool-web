package api

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gemgum/clipper/engine/internal/ytdl"
)

// fetchVideo mengunduh video dari tautan YouTube ke folder unggahan, lalu
// membalas {path, name} — bentuk yang sama dengan /api/upload, jadi GUI
// memperlakukannya persis seperti berkas yang diunggah.
//
// Sinkron, tanpa job: unduhan paling lama hitungan menit, nginx menunggu
// sampai 1 jam, dan request yang dibatalkan (tab ditutup) ikut menghentikan
// unduhannya lewat r.Context().
func (s *Server) fetchVideo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if _, err := ytdl.VideoID(req.URL); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := os.MkdirAll(s.uploadDir(), 0o755); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	dir, err := os.MkdirTemp(s.uploadDir(), "")
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	o := ytdl.Options{Dir: dir, YtDlp: s.paths.YtDlp, FFmpeg: s.paths.FFmpeg,
		Log: func(m string) { log.Print("fetch: ", m) }}
	o.FromEnv()
	p, err := ytdl.Download(r.Context(), req.URL, o)
	if err != nil {
		os.RemoveAll(dir)
		writeErr(w, 502, err.Error())
		return
	}
	abs, _ := filepath.Abs(p)
	writeJSON(w, 200, map[string]string{"path": abs, "name": filepath.Base(p)})
}

// checkVideo menjawab "bisakah video ini diambil?" sebelum mengunduhnya — mesin
// yang sama, urutan yang sama, tanpa video turun. Bisa: {ok, title, duration,
// engine}. Tidak bisa: 422 + alasan dari tiap mesin, untuk peringatan di GUI.
func (s *Server) checkVideo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if _, err := ytdl.VideoID(req.URL); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	o := ytdl.Options{YtDlp: s.paths.YtDlp, Log: func(m string) { log.Print("fetch: ", m) }}
	o.FromEnv()
	info, err := ytdl.Check(r.Context(), req.URL, o)
	if err != nil {
		writeErr(w, 422, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "title": info.Title, "duration": info.Duration, "engine": info.Engine})
}
