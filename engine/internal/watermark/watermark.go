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

// aspectTolerance = seberapa jauh rasio berkas boleh meleset dari rasio format
// supaya dianggap SAMA dan dibiarkan seukuran aslinya.
//
// Yang meleset lebih jauh DISESUAIKAN ke bingkai format (Fit), bukan ditolak
// seperti dulu — keputusan pemilik 18 Oktober 2026 (notes/44): "tidak perlu ada
// salah rasio, tinggal masukkan gambarnya, disesuaikan". Pratinjau memakai
// penyesuaian yang sama (/api/frame), jadi potongannya terlihat sebelum jalan.
const aspectTolerance = 0.02

// Cara menyesuaikan berkas yang rasionya beda dengan format.
const (
	FitCrop  = "crop"  // penuhi bingkai, sisi yang lebih potong di tengah (bawaan)
	FitWhole = "whole" // seluruh gambar masuk, sisa bingkai diisi versi blur-nya
)

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
	Fit     string // crop (bawaan) | whole — lihat FitCrop
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
	o.Watermark.Headline.FontsDir = paths.FontsDir
	headline := strings.TrimSpace(o.Watermark.Headline.Text)
	if o.Watermark.Image == "" && len(o.Watermark.Logos) == 0 && headline == "" {
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

		out, sec, err := one(ctx, o, ff, paths, work, video, crf, preset, i, "")
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

// Preview membakar SATU frame berkas pertama persis seperti hasil akhirnya —
// jalur, ukuran, logo, dan .ass yang sama dengan Run — dan mengembalikan
// JPEG-nya. Pratinjau di GUI adalah tiruan HTML; ini jawaban "seperti apa
// jadinya" yang sebenarnya. Logo & judul selalu tampil di sini (waktu tampil
// diabaikan): yang dinilai rupanya, bukan jadwalnya.
func Preview(ctx context.Context, o Options, ff *ffmpeg.Client, paths config.Paths, file string) ([]byte, error) {
	out := filepath.Join(paths.DataDir, "cache", "watermark", fmt.Sprintf("preview_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(out)
	if err := BurnTo(ctx, o, ff, paths, file, out); err != nil {
		return nil, err
	}
	return os.ReadFile(out)
}

// BurnTo membakar watermark ke SATU bingkai berkas file (foto, atau detik ke-1
// video) dan menulisnya ke out; ekstensi out menentukan formatnya (.jpg/.png).
// Dipakai Preview dan kartu News (logo + judul Mark di atas kartu).
// ApplyLayers = lapis Mark di ffmpeg: logo pertama + logo tambahan (ruang
// bingkai formatnya), logo di atas kotak judul, dan area gambar kartu berita
// (Look) bila judulnya ikut dibakar. SATU susunan untuk halaman Mark (BurnTo)
// dan klip (pipeline) — dulu pipeline hanya mengirim logo pertama.
func ApplyLayers(enc *ffmpeg.EncodeOpts, w config.Watermark, withHeadline bool) {
	_, fh := w.Frame()
	logo := func(img string, x, y, lw, lh int) ffmpeg.Watermark {
		return ffmpeg.Watermark{Image: img, X: x, Y: y, Width: lw, Height: lh, At: w.At, For: w.For, RefH: fh}
	}
	enc.Watermark = logo(w.Image, w.X, w.Y, w.Width, w.Height)
	enc.Logos = nil
	for _, l := range w.Logos {
		enc.Logos = append(enc.Logos, logo(l.Image, l.X, l.Y, l.Width, l.Height))
	}
	// Kotak judul buram: logo harus di atasnya, sama dengan pratinjau.
	enc.LogosOverSubs = w.Headline.Box
	// Kartu berita: gambar di bagian atas, panel & teksnya dari .ass.
	if hl := w.Headline; hl.Box && hl.Card && withHeadline {
		enc.Photo = float64(config.CardPhotoPx(w.Format, hl.CardY)) / float64(fh)
	}
}

func BurnTo(ctx context.Context, o Options, ff *ffmpeg.Client, paths config.Paths, file, out string) error {
	o.Watermark.Headline.Font = o.Subtitle.Font
	o.Watermark.Headline.FontsDir = paths.FontsDir
	o.Watermark.At, o.Watermark.For = 0, 0
	work := filepath.Join(paths.DataDir, "cache", "watermark")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	_, _, err := one(ctx, o, ff, paths, work, file, "", "", 0, out)
	return err
}

// one mengerjakan satu video. preview != "" = cuma satu frame ke path itu
// (lihat Preview).
func one(ctx context.Context, o Options, ff *ffmpeg.Client, paths config.Paths,
	work, video, crf, preset string, idx int, preview string) (string, float64, error) {

	w, h, err := ff.Dimensions(ctx, video)
	if err != nil {
		return "", 0, err
	}
	// Rasio sama = ukuran asli dibiarkan. Beda = keluarannya bingkai format
	// (1080 x 1920/1350/1080) dan berkasnya disesuaikan ke sana. Lihat
	// aspectTolerance.
	fw, fh := o.Watermark.Frame()
	if abs(float64(w)/float64(h)-float64(fw)/float64(fh)) > aspectTolerance {
		w, h = fw, fh
	}
	photo := IsImage(video)
	dur := 0.0
	if photo {
		// Foto tidak punya waktu: logo & judul selalu tampil. o salinan, jadi
		// berkas berikutnya tidak ikut berubah.
		o.Watermark.At, o.Watermark.For = 0, 0
	} else {
		if dur, err = ff.Duration(ctx, video); err != nil {
			return "", 0, err
		}
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
		// center + zoom 100 = penuhi bingkai, potong di tengah; fit + zoom 0 =
		// seluruh gambar masuk di atas latar blur. Pada bingkai seukuran
		// berkasnya sendiri keduanya berarti "biarkan apa adanya".
		Mode: string(config.ReframeCenter), Background: config.BackgroundBlack, Zoom: config.ZoomMax,
	}
	ApplyLayers(&enc, o.Watermark, assPath != "")
	if o.Fit == FitWhole {
		enc.Mode, enc.Background, enc.Zoom = string(config.ReframeFit), config.BackgroundBlur, 0
	}
	if preview != "" {
		// Detik ke-1 (atau tengah video yang lebih pendek): frame pembuka
		// sering hitam.
		return preview, 0, ff.BurnImage(ctx, video, min(1, dur/2), w, h, enc, preview)
	}
	if photo {
		return out, 0, ff.BurnImage(ctx, video, 0, w, h, enc, out)
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
	ext := ".mp4"
	if IsImage(video) {
		// PNG tetap PNG (tanpa kompresi rugi); foto lain jadi JPEG.
		ext = ".jpg"
		if strings.EqualFold(filepath.Ext(video), ".png") {
			ext = ".png"
		}
	}
	return filepath.Join(dir, base+Suffix+ext)
}

// IsImage = berkas ini foto (untuk feed), bukan video. Dari akhirannya: ffprobe
// melaporkan JPEG sebagai "video" berdurasi 0,04 detik.
func IsImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	}
	return false
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
