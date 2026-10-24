package watermark

import (
	"bytes"
	"context"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/ffmpeg"
	"github.com/gemgum/clipper/engine/internal/subtitle"
)

// Berkas sumber TIDAK PERNAH ditimpa. Masukan halaman ini video jadi milik
// pengguna — seringkali satu-satunya salinan — dan watermark yang sudah terbakar
// tidak bisa dilepas lagi.
func TestOutputPathNeverOverwritesSource(t *testing.T) {
	in := filepath.Join("videos", "klip.mp4")
	got := outputPath(in, "")
	if got == in {
		t.Fatalf("hasil menimpa sumbernya: %s", got)
	}
	if want := filepath.Join("videos", "klip_watermarked.mp4"); got != want {
		t.Fatalf("outputPath = %s, mau %s", got, want)
	}
	// -out mengumpulkan semuanya ke satu folder.
	if got := outputPath(in, "/keluaran"); got != filepath.Join("/keluaran", "klip_watermarked.mp4") {
		t.Fatalf("folder tujuan diabaikan: %s", got)
	}
}

// Tanpa banner DAN tanpa teks, yang dihasilkan cuma salinan yang dikompresi
// ulang. Ditolak sebelum satu detik encode pun dijalankan.
func TestRunRefusesEmptyWatermark(t *testing.T) {
	_, err := Run(context.Background(), Options{Videos: []string{"a.mp4"}}, nil, config.Paths{}, nil)
	if err == nil || !strings.Contains(err.Error(), "nothing to burn") {
		t.Fatalf("watermark kosong seharusnya ditolak, dapat: %v", err)
	}
}

// Sumber "llm" tidak berlaku di sini: tidak ada klip, jadi tidak ada judul yang
// dipilihkan LLM. Dinyatakan, bukan didiamkan lalu menghasilkan teks kosong.
func TestRunRefusesLLMHeadline(t *testing.T) {
	o := Options{Videos: []string{"a.mp4"}}
	o.Watermark.Image = "logo.png"
	o.Watermark.Headline.Source = config.HeadlineLLM
	if _, err := Run(context.Background(), o, nil, config.Paths{}, nil); err == nil ||
		!strings.Contains(err.Error(), "only exists for clips") {
		t.Fatalf("sumber llm seharusnya ditolak, dapat: %v", err)
	}
}

// Rasio lain TIDAK ditolak (notes/44): video 16:9 disesuaikan ke bingkai
// 1080x1920, sedangkan video yang sudah 9:16 tetap seukuran aslinya.
func TestRunFitsOtherRatioIntoFrame(t *testing.T) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg tidak ada di mesin ini")
	}
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe tidak ada di mesin ini")
	}
	dir := t.TempDir()
	mk := func(name, size string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		out, err := exec.Command(bin, "-y", "-f", "lavfi", "-i", "color=c=blue:s="+size+":d=1",
			"-c:v", "libx264", "-pix_fmt", "yuv420p", p).CombinedOutput()
		if err != nil {
			t.Fatalf("menyiapkan %s: %v: %s", name, err, out)
		}
		return p
	}
	portrait := mk("tegak.mp4", "270x480")
	landscape := mk("mendatar.mp4", "480x270")

	o := Options{
		Videos:   []string{landscape, portrait},
		Subtitle: config.DefaultSubtitle(),
		OutDir:   filepath.Join(dir, "out"),
		Quality:  "draft",
	}
	o.Watermark = config.DefaultWatermark()
	o.Watermark.Headline.Text = "HALO"

	ff := ffmpeg.New(bin, probe)
	res, err := Run(context.Background(), o, ff, config.Paths{DataDir: dir}, nil)
	if err != nil {
		t.Fatalf("job berhenti seluruhnya: %v", err)
	}
	if len(res.Files) != 2 {
		t.Fatalf("mau 2 hasil, dapat %d", len(res.Files))
	}
	for i, want := range [][2]int{{1080, 1920}, {270, 480}} {
		if res.Files[i].Error != "" {
			t.Fatalf("%s gagal: %s", res.Files[i].Name, res.Files[i].Error)
		}
		w, h, err := ff.Dimensions(context.Background(), res.Files[i].Output)
		if err != nil || w != want[0] || h != want[1] {
			t.Fatalf("%s: hasil %dx%d (err=%v), mau %dx%d", res.Files[i].Name, w, h, err, want[0], want[1])
		}
	}
}

// Feed 4:5: foto DAN video, dua logo, dan kotak judul berita. Dibuktikan dari
// pikselnya, bukan dari string filter: kotak putih memang tergambar di tempat
// yang dihitung NewsBox, dan logo kedua (Logos) ikut ditumpuk.
func TestRunFeedPhotoWithLogosAndNewsBox(t *testing.T) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg tidak ada di mesin ini")
	}
	probe, _ := exec.LookPath("ffprobe")
	dir := t.TempDir()
	gen := func(name string, args ...string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		out, err := exec.Command(bin, append(append([]string{"-y", "-f", "lavfi"}, args...), p)...).CombinedOutput()
		if err != nil {
			t.Fatalf("menyiapkan %s: %v: %s", name, err, out)
		}
		return p
	}
	photo := gen("foto.jpg", "-i", "color=c=blue:s=1080x1350", "-frames:v", "1")
	video := gen("feed.mp4", "-i", "color=c=blue:s=540x676:d=1", "-c:v", "libx264", "-pix_fmt", "yuv420p")
	tall := gen("tegak.mp4", "-i", "color=c=blue:s=270x480:d=1", "-c:v", "libx264", "-pix_fmt", "yuv420p")
	red := gen("merah.png", "-i", "color=c=red:s=200x80", "-frames:v", "1")
	green := gen("hijau.png", "-i", "color=c=lime:s=100x100", "-frames:v", "1")

	o := Options{Videos: []string{photo, video}, Subtitle: config.DefaultSubtitle(),
		OutDir: filepath.Join(dir, "out"), Quality: "draft"}
	o.Watermark = config.DefaultWatermark()
	o.Watermark.Format = config.FormatFeed45
	o.Watermark.Image, o.Watermark.X, o.Watermark.Y = red, 200, 100
	o.Watermark.Width, o.Watermark.Height = 20, 6
	o.Watermark.Logos = []config.Logo{{Image: green, X: 980, Y: 100, Width: 10, Height: 8}}
	o.Watermark.Headline.Text = "Kebakaran Hutan Arjuno-Welirang Meluas, Seluruh Jalur Pendakian Ditutup"
	o.Watermark.Headline.Credit = "Foto: TIMES Indonesia"
	o.Watermark.Headline.Box, o.Watermark.Headline.Color, o.Watermark.Headline.DY = true, "red", 300
	// Logo ketiga sengaja MENINDIH kotak judul: harus tetap terlihat (logo
	// ditumpuk sesudah .ass), sama dengan pratinjau.
	bx := subtitle.NewsBox(o.Watermark, o.Watermark.Headline.Text)
	inX, inY := bx.Left+bx.W/2, bx.Top+bx.H/2
	o.Watermark.Logos = append(o.Watermark.Logos, config.Logo{Image: green, X: inX, Y: inY, Width: 10, Height: 8})

	res, err := Run(context.Background(), o, ffmpeg.New(bin, probe), config.Paths{DataDir: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Files[0].Error != "" || res.Files[1].Error != "" {
		t.Fatalf("feed gagal: %#v", res.Files)
	}
	// Video 9:16 di mode feed disesuaikan ke 1080x1350, dua-duanya cara.
	for _, fit := range []string{FitCrop, FitWhole} {
		o2 := o
		o2.Videos, o2.Fit, o2.OutDir = []string{tall}, fit, filepath.Join(dir, fit)
		r2, err := Run(context.Background(), o2, ffmpeg.New(bin, probe), config.Paths{DataDir: dir}, nil)
		if err != nil || r2.Files[0].Error != "" {
			t.Fatalf("fit %s: %v %#v", fit, err, r2.Files)
		}
		if w, h, _ := ffmpeg.New(bin, probe).Dimensions(context.Background(), r2.Files[0].Output); w != 1080 || h != 1350 {
			t.Fatalf("fit %s: hasil %dx%d, mau 1080x1350", fit, w, h)
		}
	}
	if !strings.HasSuffix(res.Files[0].Output, "foto_watermarked.jpg") {
		t.Fatalf("foto seharusnya keluar sebagai foto: %s", res.Files[0].Output)
	}

	f, err := os.Open(res.Files[0].Output)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 1080 || b.Dy() != 1350 {
		t.Fatalf("ukuran foto berubah: %v", b)
	}
	at := func(x, y int) (int, int, int) {
		r, g, b, _ := img.At(x, y).RGBA()
		return int(r >> 8), int(g >> 8), int(b >> 8)
	}
	left, top, bh := bx.Left, bx.Top, bx.H
	if r, g, b := at(left+8, top+bh-8); r < 230 || g < 230 || b < 230 {
		t.Fatalf("kotak judul tidak putih di (%d,%d): %d,%d,%d", left+8, top+bh-8, r, g, b)
	}
	if r, g, b := at(200, 100); r < 200 || g > 60 || b > 60 {
		t.Fatalf("logo utama tidak ada: %d,%d,%d", r, g, b)
	}
	if r, g, b := at(980, 100); g < 200 || r > 60 || b > 60 {
		t.Fatalf("logo kedua tidak ada: %d,%d,%d", r, g, b)
	}
	if r, g, b := at(inX, inY); g < 200 || r > 60 || b > 60 {
		t.Fatalf("logo di atas kotak judul tertutup kotaknya: %d,%d,%d", r, g, b)
	}
}

// Pratinjau hasil = satu frame lewat jalur yang SAMA dengan Run: ukuran bingkai
// format (video 9:16 di feed 4:5 → 1080x1350) dan kotak judul berwarna pilihan.
func TestPreviewBurnsOneFrameLikeTheResult(t *testing.T) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg tidak ada di mesin ini")
	}
	probe, _ := exec.LookPath("ffprobe")
	dir := t.TempDir()
	tall := filepath.Join(dir, "tegak.mp4")
	if out, err := exec.Command(bin, "-y", "-f", "lavfi", "-i", "color=c=white:s=270x480:d=2",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", tall).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	o := Options{Subtitle: config.DefaultSubtitle()}
	o.Watermark = config.DefaultWatermark()
	o.Watermark.Format = config.FormatFeed45
	o.Watermark.Headline.Text, o.Watermark.Headline.Box = "HALO", true
	o.Watermark.Headline.BoxColor, o.Watermark.Headline.Color = "black", "yellow"
	o.Watermark.At = 1.5 // waktu tampil diabaikan di pratinjau

	b, err := Preview(context.Background(), o, ffmpeg.New(bin, probe), config.Paths{DataDir: dir}, tall)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if bd := img.Bounds(); bd.Dx() != 1080 || bd.Dy() != 1350 {
		t.Fatalf("pratinjau %v, mau 1080x1350", bd)
	}
	bx := subtitle.NewsBox(o.Watermark, "HALO")
	if r, g, bl, _ := img.At(bx.Left+8, bx.Top+bx.H-8).RGBA(); r>>8 > 40 || g>>8 > 40 || bl>>8 > 40 {
		t.Fatalf("kotak judul seharusnya hitam: %d,%d,%d", r>>8, g>>8, bl>>8)
	}
}
