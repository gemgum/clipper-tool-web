package caption

import (
	"context"
	"os"

	"github.com/gemgum/clipper/engine/internal/ffmpeg"
	"github.com/gemgum/clipper/engine/internal/pipeline"
)

// Whisper merakit Transcriber dari pipeline yang sudah ada.
//
// Tahap transkripsinya sama persis dengan yang dipakai pipeline klip — termasuk
// cachenya, jadi membuat caption untuk klip yang baru saja dipotong tidak
// mentranskripsi apa pun untuk kedua kalinya.
//
// workDir menampung berkas antara (audio.wav, transkrip mentah); tiap video
// mendapat foldernya sendiri di dalamnya lalu dibersihkan, supaya dua job yang
// kebetulan berjalan bersamaan tidak menimpa audio satu sama lain.
func Whisper(p *pipeline.Pipeline, workDir string, onProgress func(string)) Transcriber {
	ff := ffmpeg.New(p.Paths.FFmpeg, p.Paths.FFprobe)
	return func(ctx context.Context, video string, maxSec float64) (Speech, error) {
		if err := os.MkdirAll(workDir, 0o755); err != nil {
			return Speech{}, err
		}
		tmp, err := os.MkdirTemp(workDir, "caption")
		if err != nil {
			return Speech{}, err
		}
		defer os.RemoveAll(tmp)

		// Gagal probe tidak menggagalkan apa pun: yang hilang cuma keterangan
		// "dari 5:00 pertama dari 47:12" di kepala berkas hasil.
		videoSec, _ := ff.Duration(ctx, video)
		res, err := p.Transcript(ctx, video, tmp, maxSec, false, func(pr pipeline.Progress) {
			if onProgress != nil && pr.Message != "" {
				onProgress(pr.Message)
			}
		})
		if err != nil {
			return Speech{}, err
		}
		used := maxSec
		if videoSec > 0 && (used <= 0 || used > videoSec) {
			used = videoSec
		}
		return Speech{
			Transcript: res.Transcript,
			VideoSec:   videoSec,
			UsedSec:    used,
			Cached:     res.FromCache,
		}, nil
	}
}
