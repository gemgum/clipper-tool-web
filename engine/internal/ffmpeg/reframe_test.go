package ffmpeg

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// --- bentuk rantai filter ---

// Whole Picture pada titik awalnya (zoom 0): seluruh gambar masuk, di atas latar.
func TestWholePictureAtStartShowsEverything(t *testing.T) {
	whole := ReframeFilter(Layout{Mode: "fit", Background: "blur", Zoom: 0}, 1080, 1920)
	for _, part := range []string{
		"split=2[bg][fg]",                      // frame dipakai dua kali
		"gblur=sigma=20",                       // latar di-blur
		"force_original_aspect_ratio=decrease", // depan muat utuh, tidak terpotong
		"overlay=(W-w)/2:(H-h)/2",              // ditaruh persis di tengah
	} {
		if !strings.Contains(whole, part) {
			t.Errorf("Whole Picture zoom 0 kehilangan %q, dapat: %s", part, whole)
		}
	}
}

// "Center of the Picture" pada zoom penuh adalah keluaran yang paling sering
// dipakai, dan pernah sekali berubah tanpa disadari. Rantainya dikunci persis
// di sini: kalau ada yang menyentuhnya, tes ini yang berteriak lebih dulu,
// bukan pengguna yang melihat hasil rendernya berbeda.
func TestCenterFullZoomFilterIsLockedDown(t *testing.T) {
	const want = "scale=1080:1920:force_original_aspect_ratio=increase:flags=lanczos,crop=1080:1920"
	for _, l := range []Layout{
		{Mode: "center", Zoom: 100},                      // tanpa latar
		{Mode: "center", Background: "blur", Zoom: 100},  // latar diabaikan
		{Mode: "center", Background: "black", Zoom: 100}, // latar diabaikan
		{Mode: "center"},                                 // zoom kosong = 100
	} {
		if got := ReframeFilter(l, 1080, 1920); got != want {
			t.Errorf("layout %+v:\n dapat: %s\n ingin: %s", l, got, want)
		}
	}
}

func TestBlackBackgroundDoesNotSplitStream(t *testing.T) {
	got := ReframeFilter(Layout{Mode: "fit", Background: "black", Zoom: 0}, 1080, 1920)
	if !strings.Contains(got, "pad=1080:1920") {
		t.Errorf("latar hitam harus memakai pad, dapat: %s", got)
	}
	// Latar hitam tidak perlu menyalin aliran video — split/overlay hanya
	// dibutuhkan untuk blur, dan memakainya di sini membuang waktu encode.
	for _, forbidden := range []string{"split=2", "gblur", "overlay="} {
		if strings.Contains(got, forbidden) {
			t.Errorf("latar hitam tidak boleh memakai %q: %s", forbidden, got)
		}
	}
}

// Latar adalah ALASAN mode Whole Picture ada, jadi ia wajib hadir selama masih
// ada ruang kosong — yaitu di bawah 100, sebelum gambarnya menutupi bingkai.
func TestWholePictureHasBackgroundBelowFull(t *testing.T) {
	for _, z := range []int{0, 5, 50, 95} {
		blur := ReframeFilter(Layout{Mode: "fit", Background: "blur", Zoom: z}, 1080, 1920)
		if !strings.Contains(blur, "gblur=sigma=20") {
			t.Errorf("fit zoom %d: latar blur hilang: %s", z, blur)
		}
		black := ReframeFilter(Layout{Mode: "fit", Background: "black", Zoom: z}, 1080, 1920)
		if !strings.Contains(black, "pad=1080:1920") {
			t.Errorf("fit zoom %d: latar hitam hilang: %s", z, black)
		}
	}
	// Pada 100 gambar menutupi bingkai, jadi latar tidak dikerjakan lagi.
	got := ReframeFilter(Layout{Mode: "fit", Background: "blur", Zoom: 100}, 1080, 1920)
	if strings.Contains(got, "gblur") {
		t.Errorf("fit zoom 100 masih mengerjakan latar padahal bingkai penuh: %s", got)
	}
}

// Center of the Picture di bawah 100 mengecil, jadi butuh latar.
func TestCenterBelowFullNeedsBackground(t *testing.T) {
	for _, z := range []int{5, 50, 95} {
		got := ReframeFilter(Layout{Mode: "center", Background: "black", Zoom: z}, 1080, 1920)
		if !strings.Contains(got, "pad=1080:1920") {
			t.Errorf("center zoom %d harus mengisi sisa bingkai, dapat: %s", z, got)
		}
	}
}

// Mode tak dikenal jatuh ke center; penolakannya ditangani config.Reframe.Check
// supaya pesan errornya muncul sebelum ffmpeg dipanggil.
func TestUnknownModeBehavesLikeCenter(t *testing.T) {
	if ReframeFilter(Layout{Mode: "anything", Zoom: 100}, 720, 1280) !=
		ReframeFilter(Layout{Mode: "center", Zoom: 100}, 720, 1280) {
		t.Error("mode tak dikenal seharusnya sama dengan center")
	}
}

// Koma di dalam min()/max() harus terlindung kutip tunggal. Di dalam kutip
// ffmpeg TIDAK menafsirkan backslash, jadi meng-escape koma justru merusaknya.
func TestExpressionsAreQuotedWithoutBackslash(t *testing.T) {
	got := ReframeFilter(Layout{Mode: "fit", Zoom: 50}, 1080, 1920)
	if strings.Contains(got, `\,`) {
		t.Errorf("koma tidak boleh di-escape di dalam kutip: %s", got)
	}
	for _, expr := range []string{`w='trunc(`, `h='trunc(`, `crop='min(`} {
		if !strings.Contains(got, expr) {
			t.Errorf("ekspresi %q tidak dikutip: %s", expr, got)
		}
	}
}

// --- geometri sungguhan lewat ffmpeg ---

func requireFFmpeg(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skip(bin + " tidak tersedia")
		}
	}
}

// scaledSize menjalankan tahap PENSKALAAN saja pada sumber sintetis, lalu
// melaporkan ukuran gambarnya. Pad sesudahnya selalu mengembalikan hasil ke
// ukuran bingkai, jadi tidak memberi tahu apa pun tentang besar gambarnya
// sendiri — dan mencocokkan string filter tidak membuktikan apa pun soal
// geometri.
func scaledSize(t *testing.T, mode string, zoom, srcW, srcH, targetW, targetH int) (int, int) {
	t.Helper()
	chain := ReframeFilter(Layout{Mode: mode, Background: "black", Zoom: zoom}, targetW, targetH)
	if i := strings.Index(chain, ",pad="); i >= 0 {
		chain = chain[:i]
	}

	out := t.TempDir() + "/frame.png"
	cmd := exec.CommandContext(context.Background(), "ffmpeg", "-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc=size="+itoa(srcW)+"x"+itoa(srcH)+":duration=0.1:rate=1",
		"-frames:v", "1", "-vf", chain, out)
	if err := cmd.Run(); err != nil {
		t.Fatalf("ffmpeg menolak rantai filter %q: %v", chain, err)
	}

	probe, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-of", "json", out).Output()
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Streams []struct{ Width, Height int } `json:"streams"`
	}
	if err := json.Unmarshal(probe, &parsed); err != nil || len(parsed.Streams) == 0 {
		t.Fatalf("tidak bisa membaca dimensi hasil: %v", err)
	}
	return parsed.Streams[0].Width, parsed.Streams[0].Height
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// Inti mode Whole Picture: pada zoom 0 SELURUH video masuk, tanpa ada yang
// terpotong sedikit pun.
func TestWholePictureStartsWithEverythingVisible(t *testing.T) {
	requireFFmpeg(t)
	const tw, th = 1080, 1920
	w, h := scaledSize(t, "fit", 0, 1920, 1080, tw, th)
	if w > tw || h > th {
		t.Errorf("fit zoom 0 = %dx%d, melebihi bingkai %dx%d — ada yang terpotong", w, h, tw, th)
	}
	if w != tw {
		t.Errorf("fit zoom 0 lebar = %d, ingin pas %d (sumber lanskap menyentuh sisi kiri-kanan)", w, tw)
	}
}

// Dan menggeser zoom ke kanan MEMBESARKAN gambarnya, sampai memenuhi bingkai di
// 100 dan terus membesar setelahnya. Terpotongnya sisi itu memang yang diminta.
//
// Yang diukur tingginya: lebar sudah mentok di bingkai sejak zoom 0, jadi ia
// tidak bisa membuktikan apa-apa.
func TestWholePictureZoomEnlarges(t *testing.T) {
	requireFFmpeg(t)
	const tw, th = 1080, 1920

	prev := 0
	for _, z := range []int{0, 25, 50, 75, 100} {
		w, h := scaledSize(t, "fit", z, 1920, 1080, tw, th)
		if h <= prev {
			t.Errorf("fit zoom %d tinggi %d, tidak lebih besar dari langkah sebelumnya %d", z, h, prev)
		}
		if w > tw || h > th {
			t.Errorf("fit zoom %d = %dx%d, melewati bingkai", z, w, h)
		}
		prev = h
	}
	// Pada 100 gambar sudah menutupi bingkai sepenuhnya.
	if w, h := scaledSize(t, "fit", 100, 1920, 1080, tw, th); w != tw || h != th {
		t.Errorf("fit zoom 100 = %dx%d, ingin memenuhi bingkai %dx%d", w, h, tw, th)
	}
}

// Center of the Picture: 100 memenuhi bingkai, di bawahnya mengecil, di atasnya
// punch-in yang tetap memenuhi bingkai.
func TestCenterZoomBehaviour(t *testing.T) {
	requireFFmpeg(t)
	const tw, th = 1080, 1920

	if w, h := scaledSize(t, "center", 100, 1920, 1080, tw, th); w != tw || h != th {
		t.Errorf("center zoom 100 = %dx%d, ingin pas %dx%d", w, h, tw, th)
	}
	if w, h := scaledSize(t, "center", 50, 1920, 1080, tw, th); w >= tw || h >= th {
		t.Errorf("center zoom 50 = %dx%d, seharusnya mengecil di dalam bingkai", w, h)
	}
}

// 100 adalah batas atas KEDUA mode: di situ gambar sudah memenuhi bingkai.
// Nilai di atasnya dijepit, bukan diteruskan jadi pembesaran tambahan.
func TestZoomIsCappedAtFullFrame(t *testing.T) {
	for _, mode := range []string{"center", "fit"} {
		full := ReframeFilter(Layout{Mode: mode, Background: "blur", Zoom: 100}, 1080, 1920)
		for _, over := range []int{150, 200, 999} {
			if got := ReframeFilter(Layout{Mode: mode, Background: "blur", Zoom: over}, 1080, 1920); got != full {
				t.Errorf("%s zoom %d berbeda dari zoom 100:\n dapat: %s\n ingin: %s", mode, over, got, full)
			}
		}
	}
}

// Sumber potret ke bingkai potret: arah tiap mode harus tetap benar walau peran
// lebar & tinggi bertukar.
func TestModesWorkForPortraitSource(t *testing.T) {
	requireFFmpeg(t)
	const tw, th = 1080, 1920
	const sw, sh = 1080, 1350 // 4:5, lebih persegi daripada bingkainya

	if w, h := scaledSize(t, "fit", 0, sw, sh, tw, th); w > tw || h > th {
		t.Errorf("fit zoom 0 = %dx%d, melebihi bingkai %dx%d", w, h, tw, th)
	}
	if w, h := scaledSize(t, "center", 100, sw, sh, tw, th); w < tw || h < th {
		t.Errorf("center = %dx%d, tidak menutupi bingkai %dx%d", w, h, tw, th)
	}
}

// Tiap langkah 5% harus benar-benar mengubah ukuran — kalau tidak, menggeser
// penggeser terasa seperti tidak berfungsi.
//
// Yang TIDAK diperiksa: kegenapan dimensi antara. ffmpeg menjaga rasio sumber
// sehingga hasil skalanya boleh ganjil; crop/pad sesudahnya yang mengembalikan
// ke ukuran bingkai yang genap, jadi h264 tidak pernah melihatnya. Kegenapan
// hasil AKHIR dijamin TestClipReframeFillsFrameInEveryMode.
func TestEveryFivePercentStepChangesSize(t *testing.T) {
	requireFFmpeg(t)
	// Whole Picture: tingginya yang bergerak, 0 sampai memenuhi bingkai.
	seen := map[int]bool{}
	for z := 0; z <= 100; z += 5 {
		_, h := scaledSize(t, "fit", z, 1920, 1080, 1080, 1920)
		if seen[h] {
			t.Errorf("fit zoom %d menghasilkan tinggi %d yang sama dengan langkah lain", z, h)
		}
		seen[h] = true
	}
	// Center of the Picture: kotaknya yang bergerak, sampai memenuhi bingkai.
	seen = map[int]bool{}
	for z := 5; z <= 100; z += 5 {
		w, _ := scaledSize(t, "center", z, 1920, 1080, 1080, 1920)
		if seen[w] {
			t.Errorf("center zoom %d menghasilkan lebar %d yang sama dengan langkah lain", z, w)
		}
		seen[w] = true
	}
}

// --- render klip utuh ---

// topCentrePixel merender satu frame dari klip lalu membaca piksel di dekat tepi
// ATAS-tengah — tempat yang jadi ruang kosong saat video tidak memenuhi bingkai.
//
// format=rgb24 WAJIB mendahului crop: pada sumber yuv420p, chroma-nya
// disubsampel, dan memotong petak sekecil ini sebelum konversi membuat ffmpeg
// menghitung lebar 0 lalu gagal ("non positive size for width").
func topCentrePixel(t *testing.T, clip string) (r, g, b byte) {
	t.Helper()
	out, err := exec.Command("ffmpeg", "-v", "error", "-i", clip,
		"-frames:v", "1", "-vf", "format=rgb24,crop=2:2:540:10",
		"-f", "rawvideo", "-pix_fmt", "rgb24", "-").Output()
	if err != nil || len(out) < 3 {
		t.Fatalf("tidak bisa membaca piksel dari %s: %v", clip, err)
	}
	return out[0], out[1], out[2]
}

// Uji end-to-end lewat ClipReframe: rantai filter lengkap harus tetap
// menghasilkan bingkai berukuran tepat di SEMUA mode & zoom, dan ruang
// kosongnya benar-benar terisi latar.
func TestClipReframeFillsFrameInEveryMode(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	src := dir + "/src.mp4"
	if err := exec.Command("ffmpeg", "-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc=size=1920x1080:duration=1:rate=10",
		"-pix_fmt", "yuv420p", src).Run(); err != nil {
		t.Fatal(err)
	}

	c := New("ffmpeg", "ffprobe")
	const tw, th = 1080, 1920
	cases := []struct {
		name        string
		mode        string
		zoom        int
		wantDarkTop bool
	}{
		{"whole-awal", "fit", 0, true},         // seluruh video, pita latar
		{"whole-zoom50", "fit", 50, true},      // membesar, latar menyempit
		{"whole-penuh", "fit", 100, false},     // menutupi bingkai
		{"center-penuh", "center", 100, false}, // menutupi bingkai
		{"center-zoom50", "center", 50, true},  // mengecil, ada latar
	}
	for _, tc := range cases {
		out := dir + "/clip_" + tc.name + ".mp4"
		enc := EncodeOpts{CRF: "30", Preset: "ultrafast", Mode: tc.mode, Background: "black", Zoom: tc.zoom}
		if err := c.ClipReframe(context.Background(), src, 0, 0.5, tw, th, enc, out); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		w, h, err := c.Dimensions(context.Background(), out)
		if err != nil {
			t.Fatal(err)
		}
		if w != tw || h != th {
			t.Errorf("%s menghasilkan %dx%d, ingin %dx%d", tc.name, w, h, tw, th)
		}

		r, g, b := topCentrePixel(t, out)
		dark := r < 24 && g < 24 && b < 24
		if dark != tc.wantDarkTop {
			t.Errorf("%s: tepi atas gelap=%v (%d,%d,%d), ingin gelap=%v",
				tc.name, dark, r, g, b, tc.wantDarkTop)
		}
	}
}
