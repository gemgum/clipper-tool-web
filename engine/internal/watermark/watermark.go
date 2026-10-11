// Package watermark membakar banner watermark + headline ke video yang SUDAH jadi.
//
// Bedanya dengan halaman klip: tidak ada transkripsi, tidak ada LLM, tidak ada
// pemotongan. Masuk satu video 9:16, keluar video yang sama dengan identitas
// akun tertanam di dalamnya. Itu yang dibutuhkan ketika klipnya sudah dipotong
// di tempat lain — atau ketika kontesnya menuntut satu berkas panjang berlogo.
//
// Ia MEMAKAI ULANG dua jalur yang sama dengan halaman klip, bukan membuat jalur
// ketiga: banner lewat ffmpeg.ClipReframe (rantai `movie=` + overlay) dan
// headline lewat subtitle.WriteASS tanpa satu pun segmen ucapan. Kalau perilaku
// watermark berubah, ia berubah di satu tempat untuk kedua halaman.
package watermark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/ffmpeg"
	"github.com/gemgum/clipper/engine/internal/subtitle"
)

// Suffix ditambahkan ke nama berkas hasil.
//
// Berkas sumber TIDAK PERNAH ditimpa, dan itu bukan kehati-hatian berlebihan:
// masukan halaman ini adalah video jadi milik pengguna — seringkali satu-satunya
// salinan — dan watermark yang sudah terbakar tidak bisa dilepas lagi.
const Suffix = "_watermarked"

// aspectTolerance = seberapa jauh rasio video boleh meleset dari 9:16.
//
// Koordinat banner & headline hidup di ruang 1080x1920. Pada video 16:9 angka
// itu menunjuk tempat yang sama sekali lain, dan hasilnya banner melayang di
// luar gambar. Ditolak dengan pesan yang menyebut ukuran aslinya — bukan
// diam-diam di-reframe, sebab memotong video orang tanpa diminta adalah
// perubahan yang jauh lebih besar daripada yang ia minta.
const aspectTolerance = 0.02

// Options satu pekerjaan watermark.
type Options struct {
	Videos    []string
	Watermark config.Watermark
	// Headline mewarisi font dari sini supaya sama dengan halaman klip: satu
	// pemilih font, satu metrik, satu hasil.
	Subtitle config.Subtitle
	// OutDir kosong = hasilnya ditulis DI SEBELAH videonya. Itu bawaannya, sama
	// seperti pembuat caption: di situlah orang mencarinya saat hendak memposting.
	OutDir  string
	Quality string // draft | hd | max
}

// FileResult hasil satu video. Galat per berkas TIDAK menghentikan sisanya:
// satu video 16:9 yang nyasar ke daftar tidak boleh membatalkan dua puluh video
// lain yang sudah benar.
type FileResult struct {
	Video   string  `json:"video"`
	Name    string  `json:"name"`
	Output  string  `json:"output,omitempty"`
	Seconds float64 `json:"seconds,omitempty"`
	Error   string  `json:"error,omitempty"`
}

// Result seluruh job.
type Result struct {
	Files []FileResult `json:"files"`
}

// Progress kabar untuk GUI.
type Progress struct {
	Stage   string  `json:"stage"`
	Value   float64 `json:"value"`
	Message string  `json:"message"`
}

// Run membakar watermark ke tiap video, berurutan.
func Run(ctx context.Context, o Options, ff *ffmpeg.Client, paths config.Paths, onProgress func(Progress)) (Result, error) {
	if len(o.Videos) == 0 {
		return Result{}, fmt.Errorf("pick at least one video")
	}
	o.Watermark.Headline.Font = o.Subtitle.Font
	headline := strings.TrimSpace(o.Watermark.Headline.Text)
	if o.Watermark.Image == "" && headline == "" {
		// Halaman ini TIDAK punya pekerjaan lain: tanpa banner dan tanpa teks,
		// yang dihasilkannya cuma salinan yang dikompresi ulang.
		return Result{}, fmt.Errorf("nothing to burn: choose a watermark image, type a headline, or both")
	}
	// Sumber "llm" tidak berlaku di sini: tidak ada klip, jadi tidak ada judul
	// yang dipilihkan LLM. Dinyatakan, bukan didiamkan.
	if o.Watermark.Headline.Source == config.HeadlineLLM {
		return Result{}, fmt.Errorf("the LLM title only exists for clips: type a headline for this page")
	}

	work := filepath.Join(paths.DataDir, "cache", "watermark")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return Result{}, fmt.Errorf("work folder: %w", err)
	}
	crf, preset := config.Options{Quality: o.Quality}.Encode()

	res := Result{}
	for i, video := range o.Videos {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		name := filepath.Base(video)
		emit(onProgress, Progress{
			Stage: "burning", Value: float64(i) / float64(len(o.Videos)),
			Message: fmt.Sprintf("Burning watermark into %s (%d/%d)", name, i+1, len(o.Videos)),
		})

		out, sec, err := one(ctx, o, ff, paths, work, video, crf, preset, i)
		fr := FileResult{Video: video, Name: name, Output: out, Seconds: sec}
		if err != nil {
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			fr.Error = err.Error()
			emit(onProgress, Progress{Stage: "burning", Value: float64(i+1) / float64(len(o.Videos)),
				Message: fmt.Sprintf("%s: %s", name, err)})
		}
		res.Files = append(res.Files, fr)
	}
	emit(onProgress, Progress{Stage: "done", Value: 1, Message: fmt.Sprintf("Done: %d video(s)", len(res.Files))})
	return res, nil
}

// one mengerjakan satu video.
func one(ctx context.Context, o Options, ff *ffmpeg.Client, paths config.Paths,
	work, video, crf, preset string, idx int) (string, float64, error) {

	w, h, err := ff.Dimensions(ctx, video)
	if err != nil {
		return "", 0, err
	}
	// 9:16, atau ditolak. Lihat aspectTolerance.
	if want := 9.0 / 16.0; abs(float64(w)/float64(h)-want) > aspectTolerance {
		return "", 0, fmt.Errorf("this video is %dx%d, not 9:16: watermark is placed in a 1080x1920 space, so cut it to 9:16 first", w, h)
	}
	dur, err := ff.Duration(ctx, video)
	if err != nil {
		return "", 0, err
	}

	// Headline ditulis sebagai .ass TANPA satu pun segmen ucapan: berkas ini
	// hanya membawa satu baris statis. Jalur penulisannya sengaja sama dengan
	// halaman klip supaya gaya, pemenggalan, dan waktunya tidak bisa berbeda.
	assPath := ""
	if strings.TrimSpace(o.Watermark.Headline.Text) != "" {
		assPath = filepath.Join(work, fmt.Sprintf("headline_%d_%d.ass", time.Now().UnixNano(), idx))
		if err := subtitle.WriteASS(assPath, nil, 0, o.Subtitle, o.Watermark, o.Watermark.Headline.Text, dur); err != nil {
			return "", 0, err
		}
		defer os.Remove(assPath)
	}

	out := outputPath(video, o.OutDir)
	enc := ffmpeg.EncodeOpts{
		CRF: crf, Preset: preset, FontsDir: paths.FontsDir,
		AssPath: assPath,
		// Mode center + zoom 100 pada bingkai seukuran videonya sendiri berarti
		// "biarkan gambarnya apa adanya" — tidak ada pemotongan, tidak ada
		// bantalan. Dipakai supaya jalur render tetap SATU, bukan supaya video
		// diubah.
		Mode: string(config.ReframeCenter), Background: config.BackgroundBlack, Zoom: config.ZoomMax,
		Watermark: ffmpeg.Watermark{
			Image: o.Watermark.Image, X: o.Watermark.X, Y: o.Watermark.Y,
			Width: o.Watermark.Width, Height: o.Watermark.Height,
			At: o.Watermark.At, For: o.Watermark.For,
		},
	}
	if err := ff.ClipReframe(ctx, video, 0, dur, w, h, enc, out); err != nil {
		return "", 0, err
	}
	return out, dur, nil
}

// outputPath menentukan nama berkas hasil. Sumbernya tidak pernah ditimpa —
// lihat Suffix.
func outputPath(video, outDir string) string {
	dir := filepath.Dir(video)
	if outDir != "" {
		dir = outDir
	}
	base := strings.TrimSuffix(filepath.Base(video), filepath.Ext(video))
	return filepath.Join(dir, base+Suffix+".mp4")
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func emit(fn func(Progress), p Progress) {
	if fn != nil {
		fn(p)
	}
}
