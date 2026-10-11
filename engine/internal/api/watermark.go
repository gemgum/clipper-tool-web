package api

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/watermark"
)

// WatermarkJob keadaan satu job watermark (bentuknya di bgjob.go).
type WatermarkJob = bgJob[watermark.Result]

// maxWatermarkVideos = batas berkas dalam satu job.
//
// Alasannya sama dengan maxCaptionVideos: bukan batas teknis, melainkan penahan
// salah pilih. Bedanya di sini tiap berkas berarti satu ENCODE penuh — memilih
// seluruh isi folder Videos di halaman ini jauh lebih mahal daripada di halaman
// caption, jadi batasnya lebih rapat.
const maxWatermarkVideos = 20

// createWatermark memulai job watermark lalu LANGSUNG menjawab.
//
// Membakar satu video panjang berarti meng-encode ulang seluruhnya; menunggunya
// di dalam permintaan HTTP berarti job mati saat pengguna pindah halaman
// (notes/25).
func (s *Server) createWatermark(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Videos    []string         `json:"videos"`
		Watermark config.Watermark `json:"watermark"`
		Font      string           `json:"font"`
		Quality   string           `json:"quality"`
		OutDir    string           `json:"out_dir"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if len(req.Videos) == 0 {
		writeErr(w, 400, "the 'videos' field is required: pick at least one video")
		return
	}
	if len(req.Videos) > maxWatermarkVideos {
		writeErr(w, 400, fmt.Sprintf("%d videos in one go is more than the %d this page takes: run it in batches",
			len(req.Videos), maxWatermarkVideos))
		return
	}
	// Path diperiksa di sini, bukan dibiarkan gagal di ffmpeg belasan detik
	// kemudian — sama seperti halaman caption dan halaman klip.
	for i, v := range req.Videos {
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
	if req.Watermark.Image != "" {
		req.Watermark.Image = hostPath(req.Watermark.Image)
		if _, err := checkImage(req.Watermark.Image); err != nil {
			writeErr(w, 400, err.Error())
			return
		}
	}
	// ffmpeg saja yang dibutuhkan halaman ini — tidak ada transkripsi, jadi
	// whisper & modelnya tidak boleh ikut menghalangi.
	if s.paths.FFmpeg == "" {
		writeErr(w, 424, "ffmpeg is not installed yet: open the Requirements page")
		return
	}

	opts := config.DefaultOptions()
	opts.Watermark = req.Watermark
	if req.Font != "" {
		opts.Subtitle.Font = req.Font
	}
	if req.Quality != "" {
		opts.Quality = req.Quality
	}
	if err := opts.Validate(); err != nil {
		writeErr(w, 400, err.Error())
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := s.watermarks.create("watermark", cancel)
	bopts := watermark.Options{
		Videos:    req.Videos,
		Watermark: opts.Watermark,
		Subtitle:  opts.Subtitle,
		OutDir:    hostPath(req.OutDir),
		Quality:   opts.Quality,
	}

	go func() {
		defer cancel()
		res, err := watermark.Run(ctx, bopts, s.ff, s.paths, func(pr watermark.Progress) {
			s.watermarks.update(job.ID, func(j *WatermarkJob) {
				j.Stage, j.Progress = pr.Stage, pr.Value
				if pr.Message != "" {
					j.Log = append(j.Log, pr.Message)
				}
			})
		})
		s.watermarks.finish(job.ID, ctx, res, err)
		if err == nil && ctx.Err() == nil {
			dropUploads(req.Videos...)
		}
	}()

	writeJSON(w, 202, map[string]any{"id": job.ID, "videos": len(req.Videos), "started": true})
}

func (s *Server) listWatermarks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"jobs": s.watermarks.snapshot()})
}

func (s *Server) getWatermark(w http.ResponseWriter, r *http.Request) {
	j, ok := s.watermarks.get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "watermark job not found")
		return
	}
	writeJSON(w, 200, j)
}

func (s *Server) cancelWatermark(w http.ResponseWriter, r *http.Request) {
	if !s.watermarks.stop(r.PathValue("id")) {
		writeErr(w, 404, "watermark job not found")
		return
	}
	writeJSON(w, 200, map[string]any{"canceled": true})
}

func (s *Server) watermarkEvents(w http.ResponseWriter, r *http.Request) {
	s.watermarks.stream(w, r, "watermark")
}

// watermarkFile menyajikan satu video hasil, ?i= menunjuk urutannya di job.
//
// Di desktop hasilnya cukup ditulis di sebelah videonya; di mode web berkas itu
// ada di server, jadi inilah satu-satunya jalan untuk mengambilnya.
func (s *Server) watermarkFile(w http.ResponseWriter, r *http.Request) {
	j, ok := s.watermarks.get(r.PathValue("id"))
	if !ok || j.Result == nil {
		writeErr(w, 404, "no such watermark job, or it has not finished yet")
		return
	}
	i, err := strconv.Atoi(r.URL.Query().Get("i"))
	if err != nil || i < 0 || i >= len(j.Result.Files) || j.Result.Files[i].Output == "" {
		writeErr(w, 404, "no output for that video")
		return
	}
	out := j.Result.Files[i].Output
	w.Header().Set("Content-Disposition",
		mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(out)}))
	http.ServeFile(w, r, out)
}
