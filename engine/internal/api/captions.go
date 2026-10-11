package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gemgum/clipper/engine/internal/caption"
	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/correct"
	"github.com/gemgum/clipper/engine/internal/pipeline"
)

// CaptionJob keadaan satu job pembuat caption (bentuknya di bgjob.go).
type CaptionJob = bgJob[caption.Result]

// maxCaptionVideos = batas berkas dalam satu job bulk.
//
// Bukan batas teknis: antriannya berurutan, jadi 500 berkas cuma berarti
// menunggu lebih lama. Batas ini menahan salah pilih — memilih seluruh isi
// folder Videos lalu menunggu berjam-jam adalah kekeliruan yang tidak terlihat
// sampai sudah terlambat.
const maxCaptionVideos = 50

// createCaption memulai job caption lalu LANGSUNG menjawab.
//
// Satu video memanggil whisper lalu LLM; pada model lokal itu hitungan menit,
// dan bulk mengalikannya. Menunggunya di dalam permintaan HTTP berarti job mati
// saat pengguna pindah halaman (notes/25).
func (s *Server) createCaption(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Videos []string `json:"videos"`
		Engine string   `json:"engine"`
		Model  string   `json:"model"`
		// Minutes = bagian tiap video yang didengarkan. 0 memakai bawaan.
		Minutes  float64 `json:"minutes"`
		Variants int     `json:"variants"`
		Lang     string  `json:"lang"`
		Terms    string  `json:"terms"`
		// WhisperModel = model transkripsi, sama seperti di halaman klip.
		WhisperModel string `json:"whisper_model"`
		// OutDir = folder berkas .txt. Kosong berarti di sebelah tiap videonya.
		OutDir string `json:"out_dir"`
		// TranscriptFix: "off" mematikan koreksi transkrip. Kosong = menyala.
		TranscriptFix string `json:"transcript_fix"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if len(req.Videos) == 0 {
		writeErr(w, 400, "the 'videos' field is required: pick at least one video")
		return
	}
	if len(req.Videos) > maxCaptionVideos {
		writeErr(w, 400, fmt.Sprintf("%d videos in one go is more than the %d this page takes: run it in batches",
			len(req.Videos), maxCaptionVideos))
		return
	}
	// Path diperiksa di sini, bukan dibiarkan gagal di ffmpeg belasan detik
	// kemudian: berkas lokal TIDAK diunggah (notes/24), jadi yang sampai ke
	// engine cuma alamatnya — dan alamat yang salah adalah kekeliruan yang
	// paling mungkin terjadi.
	for i, v := range req.Videos {
		// Path gaya Windows ("C:\Users\...") diterjemahkan lebih dulu: di WSL
		// itu bentuk yang tersalin dari Explorer, dan tanpa ini ia gagal sebagai
		// "berkas tidak ditemukan" padahal berkasnya jelas ada.
		v = hostPath(v)
		req.Videos[i] = v
		st, err := os.Stat(v)
		if err != nil {
			writeErr(w, 400, "the video was not found on this computer: "+v)
			return
		}
		if st.IsDir() {
			writeErr(w, 400, "that path is a folder, not a video: "+v)
			return
		}
	}
	if err := s.missingRequirement(); err != nil {
		writeErr(w, 424, err.Error())
		return
	}

	complete, engineName, err := EngineFor(req.Engine, req.Model)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}

	// Transkripsi memakai tahap yang sama dengan pipeline klip, jadi opsinya pun
	// opsi pipeline — hanya model whisper & bahasanya yang berlaku di sini.
	opts := config.DefaultOptions()
	if req.WhisperModel != "" {
		opts.WhisperModel = req.WhisperModel
	}
	if req.Lang != "" {
		opts.Language = req.Lang
	}
	paths := config.ResolvePaths(s.layout, opts)

	ctx, cancel := context.WithCancel(context.Background())
	job := s.captions.create("caption", cancel)
	p := pipeline.New(paths, opts)
	work := filepath.Join(paths.DataDir, "cache", "caption")

	log := func(msg string) {
		s.captions.update(job.ID, func(j *CaptionJob) { j.Log = append(j.Log, msg) })
	}
	deps := caption.Deps{
		Complete:   complete,
		Engine:     engineName,
		Transcribe: caption.Whisper(p, work, log),
	}
	fix := req.TranscriptFix != "off"
	copts := caption.Options{
		Videos:     req.Videos,
		MaxSeconds: req.Minutes * 60,
		Lang:       opts.Language,
		Terms:      correct.ParseTerms(req.Terms),
		Variants:   req.Variants,
		Fix:        &fix,
		OutDir:     hostPath(req.OutDir),
		EngineName: engineName,
	}

	go func() {
		defer cancel()
		res, err := caption.Run(ctx, copts, deps, func(pr caption.Progress) {
			s.captions.update(job.ID, func(j *CaptionJob) {
				j.Stage, j.Progress = pr.Stage, pr.Value
				if pr.Message != "" {
					j.Log = append(j.Log, pr.Message)
				}
			})
		})
		s.captions.finish(job.ID, ctx, res, err)
		if err == nil && ctx.Err() == nil {
			dropUploads(req.Videos...)
		}
	}()

	writeJSON(w, 202, map[string]any{"id": job.ID, "engine": engineName, "videos": len(req.Videos), "started": true})
}

func (s *Server) listCaptions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"captions": s.captions.snapshot()})
}

func (s *Server) getCaption(w http.ResponseWriter, r *http.Request) {
	j, ok := s.captions.get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "no such caption job")
		return
	}
	writeJSON(w, 200, j)
}

func (s *Server) cancelCaption(w http.ResponseWriter, r *http.Request) {
	if !s.captions.stop(r.PathValue("id")) {
		writeErr(w, 404, "no such caption job")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "canceled"})
}

// captionFile menyajikan satu berkas .txt hasil job.
//
// Berkasnya ditunjuk lewat NOMOR URUT, bukan nama: path yang datang dari luar
// dan dipakai menyusun alamat berkas adalah jalan keluar dari folder itu, dan
// nomor tidak bisa memuat "..".
func (s *Server) captionFile(w http.ResponseWriter, r *http.Request) {
	j, ok := s.captions.get(r.PathValue("id"))
	if !ok || j.Result == nil {
		writeErr(w, 404, "no such caption job, or it has not finished yet")
		return
	}
	i, err := strconv.Atoi(r.URL.Query().Get("i"))
	if err != nil || i < 0 || i >= len(j.Result.Files) {
		writeErr(w, 400, "which file? pass i=0 for the first video in this job")
		return
	}
	path := j.Result.Files[i].TXT
	if path == "" {
		writeErr(w, 404, "no caption was written for that video")
		return
	}
	if _, err := os.Stat(path); err != nil {
		writeErr(w, 404, "file not found")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.ServeFile(w, r, path)
}

// captionEvents mengalirkan kemajuan SEMUA job caption.
func (s *Server) captionEvents(w http.ResponseWriter, r *http.Request) {
	s.captions.stream(w, r, "caption")
}
