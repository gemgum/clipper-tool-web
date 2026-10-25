package api

import (
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/ffmpeg"
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
	// Awalan "yt-" menandai folder unduhan: hanya folder inilah yang disapu
	// sweepFetched bila tidak pernah dipakai job.
	dir, err := os.MkdirTemp(s.uploadDir(), "yt-")
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	o := ytdl.Options{Dir: dir, YtDlp: s.paths.YtDlp, FFmpeg: s.paths.FFmpeg,
		Log: func(m string) { log.Print("fetch: ", m) }}
	o.FromEnv()
	ff := ffmpeg.New(s.paths.FFmpeg, s.paths.FFprobe)
	o.Verify = func(p string) error {
		if d, err := ff.Duration(r.Context(), p); err != nil || d <= 0 {
			return errors.New("the downloaded file is not a playable video")
		}
		return nil
	}
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

// inputInUse melaporkan apakah ada job yang masih menunjuk berkas ini.
// Input tidak pernah berubah setelah job dibuat, jadi aman dibaca tanpa kunci.
func (s *Server) inputInUse(p string) bool {
	for _, j := range s.mgr.List() {
		if j.Input == p {
			return true
		}
	}
	return false
}

// videoExt = akhiran berkas yang dianggap video sumber oleh sapuan unggahan.
var videoExt = map[string]bool{".mp4": true, ".mov": true, ".mkv": true, ".webm": true,
	".avi": true, ".m4v": true, ".ts": true, ".flv": true, ".wmv": true}

// Batas umur untuk sweepFetched.
const (
	partialMaxAge = 6 * time.Hour  // sisa unduhan (.dl-*): unduhan hidup tak selama ini (nginx memutus di 1 jam)
	fetchedMaxAge = 48 * time.Hour // video unduhan yang tidak pernah dipakai job
)

// sweepFetched membersihkan sisa YouTube di folder unggahan:
//
//   - .dl-* = unduhan yang terputus (engine restart/mati di tengah jalan);
//
//   - yt-*  = video yang diunduh tapi tidak dipakai job mana pun (pengguna
//     berganti video atau menutup tab). Yang dipakai job tetap ada sampai job
//     sukses (dropUploads) atau dihapus dari History (deleteJob).
//
//   - data/thumbs/* = cuplikan frame yang 7 hari tidak tersentuh.
//
// Di folder unggahan hanya folder buatan /api/fetch yang disentuh.
func (s *Server) sweepFetched(now time.Time) {
	// Cuplikan frame: murah dibuat ulang, jadi yang tak tersentuh 7 hari dibuang.
	thumbs, _ := filepath.Glob(filepath.Join(s.thumbDir(), "*"))
	for _, d := range thumbs {
		if st, err := os.Stat(d); err == nil && now.Sub(st.ModTime()) > 7*24*time.Hour {
			os.RemoveAll(d)
		}
	}
	s.sweepUploads(now)
	s.sweepPreviews(now)
	dirs, _ := filepath.Glob(filepath.Join(s.uploadDir(), "yt-*"))
	for _, d := range dirs {
		parts, _ := filepath.Glob(filepath.Join(d, ".dl-*"))
		for _, p := range parts {
			if st, err := os.Stat(p); err == nil && now.Sub(st.ModTime()) > partialMaxAge {
				os.RemoveAll(p)
			}
		}
		st, err := os.Stat(d)
		if err != nil || now.Sub(st.ModTime()) <= fetchedMaxAge {
			continue
		}
		vids, _ := filepath.Glob(filepath.Join(d, "*.mp4"))
		used := false
		for _, v := range vids {
			abs, _ := filepath.Abs(v)
			used = used || s.inputInUse(abs)
		}
		if !used {
			log.Print("fetch: removing unused download ", d)
			os.RemoveAll(d)
		}
	}
}

// removeUpload: tombol Remove di kartu Source. Di mode web video unggahan/unduhan
// itu dihapus dari server — kecuali sudah dipakai job (job yang memilikinya
// menghapusnya sendiri). Di luar folder unggahan, dan di mode desktop, tidak
// ada yang disentuh: dropUploads menolaknya.
func (s *Server) removeUpload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	removed := false
	if p := insideWebRoot(req.Path); p != "" && !s.inputInUse(p) {
		dropUploads(p)
		_, err := os.Stat(p)
		removed = os.IsNotExist(err)
	}
	writeJSON(w, 200, map[string]bool{"removed": removed})
}

// sweepUploads membuang video unggahan biasa yang tidak pernah dipakai: folder
// berisi TEPAT SATU berkas video, lebih dari 48 jam, dan tidak ditunjuk job mana
// pun. Syarat "satu berkas" menjaga hasil caption & watermark, yang ditulis di
// sebelah videonya; gambar watermark dan gambar kartu (bukan video) tidak
// tersentuh, begitu pula folder images/ dan unduhan yt-* (diurus sweepFetched).
func (s *Server) sweepUploads(now time.Time) {
	entries, _ := os.ReadDir(s.uploadDir())
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "images" || strings.HasPrefix(e.Name(), "yt-") {
			continue
		}
		d := filepath.Join(s.uploadDir(), e.Name())
		files, err := os.ReadDir(d)
		if err != nil || len(files) != 1 || files[0].IsDir() ||
			!videoExt[strings.ToLower(filepath.Ext(files[0].Name()))] {
			continue
		}
		v := filepath.Join(d, files[0].Name())
		st, err := os.Stat(v)
		if err != nil || now.Sub(st.ModTime()) <= fetchedMaxAge {
			continue
		}
		if abs, _ := filepath.Abs(v); s.inputInUse(abs) {
			continue
		}
		log.Print("upload: removing unused upload ", d)
		os.RemoveAll(d)
	}
}
