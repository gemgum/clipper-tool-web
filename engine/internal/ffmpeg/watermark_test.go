package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Tanpa gambar, rantainya harus PERSIS seperti sebelum ada watermark — fitur yang
// dimatikan tidak boleh meninggalkan jejak di perintah ffmpeg.
func TestWatermarkChainOffLeavesChainAlone(t *testing.T) {
	base := ReframeFilter(Layout{Mode: "center", Zoom: 100}, 1080, 1920)
	with := ReframeFilter(Layout{Mode: "center", Zoom: 100,
		Watermark: Watermark{X: 540, Y: 700, Width: 25, Height: 25}}, 1080, 1920)
	if base != with {
		t.Fatalf("watermark tanpa Image mengubah rantai:\n%s\n%s", base, with)
	}
}

// Koordinat pengguna hidup di 1080x1920; bingkai 720p separuhnya, jadi titik
// tengah banner harus ikut menyusut. Ini yang membuat pratinjau dan hasil render
// menunjuk tempat yang sama.
func TestWatermarkChainScalesToTargetFrame(t *testing.T) {
	vf := ReframeFilter(Layout{
		Mode: "center", Zoom: 100,
		Watermark: Watermark{Image: "/tmp/logo.png", X: 540, Y: 960, Width: 50, Height: 25},
	}, 720, 1280)

	if !strings.Contains(vf, "movie='/tmp/logo.png'") {
		t.Fatalf("sumber gambar hilang: %s", vf)
	}
	// Kotaknya 50% x 25% dari 720x1280 = 360x320, dan gambarnya DIMUAT ke
	// dalamnya — bukan digepengkan ke ukuran itu, bukan dipotong.
	if !strings.Contains(vf, "scale=360:320:force_original_aspect_ratio=decrease") {
		t.Fatalf("kotak watermark tidak diskalakan ke bingkai: %s", vf)
	}
	// 540/1080*720 = 360 ; 960/1920*1280 = 640.
	if !strings.Contains(vf, "overlay=x='360-w/2':y='640-h/2'") {
		t.Fatalf("titik tengah tidak diskalakan: %s", vf)
	}
	if strings.Contains(vf, "enable=") {
		t.Fatalf("tanpa At/For tidak boleh ada enable: %s", vf)
	}
}

// Waktu tampil: rentang bila For diisi, "mulai detik sekian lalu tetap" bila
// tidak. Keduanya harus muncul sebagai enable, sebab tanpa itu banner tampil
// sepanjang klip apa pun angkanya.
func TestWatermarkChainTiming(t *testing.T) {
	l := Layout{Mode: "center", Zoom: 100, Watermark: Watermark{Image: "logo.png", X: 540, Y: 700, Width: 25, Height: 25, At: 1.5, For: 6}}
	if vf := ReframeFilter(l, 1080, 1920); !strings.Contains(vf, "enable='between(t,1.500,7.500)'") {
		t.Fatalf("rentang waktu salah: %s", vf)
	}
	l.Watermark.For = 0
	if vf := ReframeFilter(l, 1080, 1920); !strings.Contains(vf, "enable='gte(t,1.500)'") {
		t.Fatalf("tanpa durasi harusnya gte: %s", vf)
	}
}

// Path bertanda titik dua (drive Windows) harus di-escape seperti path subtitle,
// kalau tidak ffmpeg membaca "C" sebagai nama berkas dan sisanya sebagai opsi.
func TestWatermarkChainEscapesPath(t *testing.T) {
	vf := ReframeFilter(Layout{
		Mode: "center", Zoom: 100,
		Watermark: Watermark{Image: `C:\watermark\logo.png`, X: 540, Y: 700, Width: 25, Height: 25},
	}, 1080, 1920)
	if !strings.Contains(vf, `movie='C\:\\watermark\\logo.png'`) {
		t.Fatalf("path tidak di-escape: %s", vf)
	}
}

// Rantai filternya benar-benar dijalankan ffmpeg.
//
// Tes di atas cuma membaca stringnya, dan string yang terbaca benar masih bisa
// ditolak ffmpeg: `movie=` adalah filter SUMBER, jadi ia harus berdiri sebagai
// rantai sendiri di dalam -vf, dan label yang salah satu huruf pun membuat
// seluruh grafiknya gagal dibangun. Yang paling rawan justru sambungannya
// dengan burn subtitle, sebab `subtitles=` ditempel SESUDAH overlay ini.
func TestClipReframeWithWatermarkAndSubtitles(t *testing.T) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg tidak ada di mesin ini")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp4")
	logo := filepath.Join(dir, "logo.png")
	ass := filepath.Join(dir, "sub.ass")
	out := filepath.Join(dir, "out.mp4")

	mk := func(args ...string) {
		t.Helper()
		if o, err := exec.Command(bin, append([]string{"-y"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("menyiapkan bahan: %v: %s", err, o)
		}
	}
	mk("-f", "lavfi", "-i", "testsrc=size=640x360:duration=2", "-c:v", "libx264", "-pix_fmt", "yuv420p", src)
	mk("-f", "lavfi", "-i", "color=c=white:s=200x60", "-frames:v", "1", logo)

	if err := os.WriteFile(ass, []byte(`[Script Info]
ScriptType: v4.00+
PlayResX: 1080
PlayResY: 1920
WrapStyle: 2

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Headline,Sans,64,&H00FFFFFF,&H00FFFFFF,&H00000000,&H90000000,1,0,0,0,100,100,0,0,1,3,0,5,60,60,40,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 1,0:00:00.00,0:00:02.00,Headline,,0,0,0,,{\an8\pos(540,640)}HALO
`), 0o644); err != nil {
		t.Fatal(err)
	}

	c := New(bin, "ffprobe")
	enc := EncodeOpts{
		CRF: "30", Preset: "ultrafast", Mode: "center", Zoom: 100, AssPath: ass,
		Watermark: Watermark{Image: logo, X: 540, Y: 700, Width: 25, Height: 25, At: 0.2, For: 1},
	}
	if err := c.ClipReframe(context.Background(), src, 0, 1.5, 1080, 1920, enc, out); err != nil {
		t.Fatalf("render dengan banner + subtitle gagal: %v", err)
	}
	st, err := os.Stat(out)
	if err != nil || st.Size() == 0 {
		t.Fatalf("berkas keluaran kosong atau tidak ada (err=%v)", err)
	}
}
