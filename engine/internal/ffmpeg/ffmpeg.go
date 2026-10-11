// Package ffmpeg membungkus pemanggilan ffmpeg/ffprobe sebagai subprocess.
package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Client memegang path binary ffmpeg & ffprobe.
type Client struct {
	FFmpeg  string
	FFprobe string
}

func New(ffmpeg, ffprobe string) *Client {
	return &Client{FFmpeg: ffmpeg, FFprobe: ffprobe}
}

// CLIPath menyiapkan sebuah path untuk diserahkan sebagai argumen program lain.
//
// Tidak ada shell di mana pun di engine ini (os/exec tanpa "sh -c"), jadi
// injeksi shell memang tidak berlaku. Yang masih berlaku: ffmpeg membaca
// argumen yang diawali "-" sebagai FLAG-nya sendiri. Sebuah path bernama
// "-report" atau "-loglevel" karenanya berhenti jadi nama berkas dan mulai jadi
// perintah — dan path itu datang dari klien (mis. /api/probe?path=…).
//
// ffmpeg tidak mengenal "--" sebagai penanda akhir flag, jadi yang dipakai
// adalah bentuk yang selalu dipahami: path mutlak. "-report" jadi
// "/folder/kerja/-report", yang tidak lagi diawali tanda hubung namun menunjuk
// berkas yang sama persis. Path yang tidak diawali "-" tidak disentuh sama
// sekali.
func CLIPath(p string) string {
	if !strings.HasPrefix(p, "-") {
		return p
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return "." + string(filepath.Separator) + p
}

// HasAudio melaporkan apakah berkas punya trek suara sama sekali.
//
// Diperiksa SEBELUM ekstraksi, bukan sesudah gagal: video tanpa audio membuat
// ffmpeg berhenti dengan
//
//	Output file does not contain any stream
//	Error opening output file …\tmp\audio.wav.
//	Error opening output files: Invalid argument
//
// — tiga baris yang menunjuk berkas KELUARAN dan tidak menyebut sebab
// sebenarnya sama sekali. Dilaporkan dari lapangan sebagai "error di audio.wav",
// dan tebakan pertamanya (spasi di nama berkas, ukuran video, bahasanya)
// semuanya salah. Reproduksinya: satu mp4 tanpa trek audio, pesan yang sama
// persis.
func (c *Client) HasAudio(ctx context.Context, input string) (bool, error) {
	args := []string{
		"-v", "error",
		"-select_streams", "a",
		"-show_entries", "stream=index",
		"-of", "csv=p=0",
		CLIPath(input),
	}
	out, err := exec.CommandContext(ctx, c.FFprobe, args...).Output()
	if err != nil {
		return false, fmt.Errorf("ffprobe audio streams: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// Duration mengembalikan durasi video (detik) via ffprobe.
func (c *Client) Duration(ctx context.Context, input string) (float64, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		CLIPath(input),
	}
	out, err := exec.CommandContext(ctx, c.FFprobe, args...).Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe duration: %w", err)
	}
	s := strings.TrimSpace(string(out))
	d, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", s, err)
	}
	return d, nil
}

// Dimensions mengembalikan lebar & tinggi video.
func (c *Client) Dimensions(ctx context.Context, input string) (w, h int, err error) {
	args := []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "json",
		CLIPath(input),
	}
	out, err := exec.CommandContext(ctx, c.FFprobe, args...).Output()
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe dimensions: %w", err)
	}
	var parsed struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return 0, 0, fmt.Errorf("parse dimensions: %w", err)
	}
	if len(parsed.Streams) == 0 {
		return 0, 0, fmt.Errorf("no video stream found")
	}
	return parsed.Streams[0].Width, parsed.Streams[0].Height, nil
}

// ExtractAudioWAV mengekstrak audio ke WAV 16kHz mono (format whisper.cpp).
// Streaming langsung ke file — video tidak dimuat ke RAM.
//
// maxSec > 0 hanya mengambil sekian detik pertama. Dipakai pembuat caption,
// yang bekerja dari pembuka video dan tidak perlu membayar transkripsi satu jam
// penuh untuk itu.
func (c *Client) ExtractAudioWAV(ctx context.Context, input, outWAV string, maxSec float64) error {
	args := []string{
		"-y",
		"-i", CLIPath(input),
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "pcm_s16le",
	}
	if maxSec > 0 {
		args = append(args, "-t", fmt.Sprintf("%.3f", maxSec))
	}
	args = append(args, CLIPath(outWAV))
	return c.run(ctx, "extract audio", args)
}

// EncodeAudioChunk menulis potongan [start, start+dur) dari audio sumber
// sebagai Opus mono 24 kbps dalam OGG — kecil, supaya bisa dikirim inline ke
// AI Studio (notes/43). 10 menit ≈ 1,8 MB.
func (c *Client) EncodeAudioChunk(ctx context.Context, input string, start, dur float64, out string) error {
	return c.run(ctx, "encode audio chunk", []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", start),
		"-t", fmt.Sprintf("%.3f", dur),
		"-i", CLIPath(input),
		"-vn", "-ac", "1", "-ar", "16000",
		"-c:a", "libopus", "-b:a", "24k",
		CLIPath(out),
	})
}

// EncodeOpts parameter encoding & subtitle.
type EncodeOpts struct {
	CRF        string
	Preset     string
	AssPath    string    // subtitle .ass; kosong = tanpa subtitle
	FontsDir   string    // dir font untuk libass
	Mode       string    // center | fit
	Background string    // blur | black — isi ruang kosong yang tersisa
	Zoom       int       // 5..100 persen ukuran video dalam bingkai
	FPS        int       // 0 = ikut sumber
	Watermark  Watermark // banner watermark; Image kosong = tidak ada
}

// Layout menempatkan video ke bingkai target.
//
// Dipakai bersama oleh render klip DAN preview satu frame — sengaja satu
// sumber, sebab dulu keduanya menyusun filter sendiri-sendiri dan preview
// tertinggal ketika perilaku render berubah.
//
// Mode menentukan CARA video dipasangkan ke bingkai 9:16 — dua pilihan yang
// berdiri sendiri, bukan titik pada satu sumbu:
//
//	center : potong tengah sampai memenuhi bingkai;
//	fit    : ambil SELURUH resolusi video tanpa crop. Sisa ruangnya diisi
//	         Background — inilah alasan blur & hitam ada.
//
// Zoom dibaca RELATIF terhadap titik awal modenya, jadi artinya berbeda di tiap
// mode. Itu disengaja: yang sama di keduanya adalah "0 sampai naik = makin
// diperbesar", bukan angka mutlaknya.
//
//	fit    :   0 = seluruh video masuk (titik awal mode ini) — hanya bisa NAIK
//	         100 = video memenuhi bingkai, sisinya terpotong
//	center : 100 = potongan tengah memenuhi bingkai (titik awal) — hanya bisa TURUN
//	           5 = potongan tengah mengecil di tengah bingkai
//
// Keduanya berhenti di 100: di situ gambar sudah memenuhi bingkai, dan
// memperbesarnya lagi tidak menambah apa pun selain memotong lebih banyak.
type Layout struct {
	Mode       string    // center | fit
	Background string    // blur | black — mengisi ruang kosong yang tersisa
	Zoom       int       // persen; artinya bergantung Mode (lihat di atas)
	Watermark  Watermark // ditumpuk SETELAH video dipasang ke bingkai
}

// Watermark = gambar yang ditumpuk di atas video setelah reframe.
//
// Koordinatnya hidup di ruang 1080x1920 yang sama dengan subtitle, bukan dalam
// piksel bingkai keluaran. libass menskalakan sendiri lewat PlayRes; overlay
// tidak punya PlayRes, jadi penskalaannya dikerjakan watermarkChain di bawah.
type Watermark struct {
	Image string // path gambar; kosong = tidak ada yang ditumpuk
	X, Y  int    // titik TENGAH kotak, ruang wmRefW x wmRefH
	// Width & Height = KOTAK, dalam persen lebar & tinggi bingkai. Gambarnya
	// dimuat utuh ke dalamnya (force_original_aspect_ratio=decrease): tidak
	// digepengkan, tidak dipotong.
	Width, Height int
	At            float64 // detik muncul, relatif AWAL KLIP
	For           float64 // durasi tampil; 0 = sampai klip habis
}

// Ruang koordinat penempatan. Harus sama dengan config.PlayResX/Y — paket ini
// sengaja tidak mengimpor config supaya tetap bisa dipakai sendiri, persis
// alasan maxZoom juga digandakan di sini.
const (
	wmRefW = 1080
	wmRefH = 1920
)

// maxZoom harus sama dengan config.ZoomMax. Paket ini sengaja tidak mengimpor
// config supaya tetap bisa dipakai sendiri.
const maxZoom = 100

// ReframeFilter menyusun rantai filter -vf untuk menempatkan video ke bingkai
// target.
func ReframeFilter(l Layout, targetW, targetH int) string {
	chain := centerCropChain(l, targetW, targetH)
	if l.Mode == "fit" {
		chain = wholePictureChain(l, targetW, targetH)
	}
	return watermarkChain(chain, l.Watermark, targetW, targetH)
}

// watermarkChain menumpuk banner watermark di atas rantai yang sudah jadi.
//
// Dipasang lewat sumber `movie=` di dalam -vf, BUKAN sebagai masukan kedua
// dengan -filter_complex. Alasannya bukan selera: ReframeFilter dipakai bersama
// oleh render klip DAN pratinjau satu frame, dan keduanya menyerahkan satu
// string filter. Beralih ke filter_complex berarti memecah kembali jalur yang
// sengaja disatukan (lihat komentar Layout).
//
// Waktu tampil dijaga `enable`. `t` di situ relatif terhadap AWAL KELUARAN, dan
// karena -ss diletakkan sebelum -i di ClipReframe, awal keluaran = awal klip —
// jadi "muncul detik 1" berarti detik 1 klipnya, bukan detik 1 video sumbernya.
func watermarkChain(chain string, b Watermark, targetW, targetH int) string {
	if b.Image == "" || b.Width <= 0 || b.Height <= 0 {
		return chain
	}
	w := evenBox(targetW * b.Width / 100)
	h := evenBox(targetH * b.Height / 100)
	x := b.X * targetW / wmRefW
	y := b.Y * targetH / wmRefH

	enable := ""
	switch {
	case b.For > 0:
		enable = fmt.Sprintf(":enable='between(t,%.3f,%.3f)'", b.At, b.At+b.For)
	case b.At > 0:
		enable = fmt.Sprintf(":enable='gte(t,%.3f)'", b.At)
	}

	// force_original_aspect_ratio=decrease: gambar dimuat UTUH ke dalam kotak
	// w x h. Bukan `scale=w:h` mentah (menggepengkan logo orang) dan bukan crop
	// (memotongnya) — dua-duanya merusak lambang yang justru harus dikenali.
	//
	// overlay memakai w/h HASIL penskalaan, bukan kotaknya, jadi yang duduk di
	// titik (x,y) adalah tengah GAMBARNYA — sama persis dengan object-fit:
	// contain di pratinjau.
	//
	// format=rgba dipertahankan supaya PNG beralfa tetap tembus pandang; tanpa
	// itu gambar berlatar transparan bisa mendarat sebagai kotak hitam.
	return fmt.Sprintf(
		"%s[bvid];movie='%s',scale=%d:%d:force_original_aspect_ratio=decrease,format=rgba[blogo];"+
			"[bvid][blogo]overlay=x='%d-w/2':y='%d-h/2'%s",
		chain, escapeFilterPath(b.Image), w, h, x, y, enable)
}

// wholePictureChain: zoom 0 memasukkan SELURUH video, lalu naik = membesar
// sampai memenuhi bingkai di 100.
//
// Kedua ujungnya memakai bentuk yang dihitung ffmpeg sendiri supaya tepat —
// pembulatan ekspresi bisa meleset satu piksel, dan pada 100 satu piksel meleset
// berarti segaris latar terlihat di tepi.
func wholePictureChain(l Layout, targetW, targetH int) string {
	zoom := clampZoom(l.Zoom, 0)

	var foreground string
	switch {
	case zoom <= 0:
		foreground = fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos",
			targetW, targetH)
	case zoom == 100:
		foreground = fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d",
			targetW, targetH, targetW, targetH)
	default:
		//	utuh  = min(TW/iw, TH/ih)      penuh = max(TW/iw, TH/ih)
		//	skala = utuh + (penuh-utuh) * zoom/100
		//
		// Dihitung ffmpeg lewat ekspresi supaya dimensi sumber tidak perlu
		// diprobe lebih dulu. Ekspresinya dikutip tunggal agar koma di dalam
		// min()/max() tidak dibaca sebagai pemisah filter — di dalam kutip,
		// koma TIDAK boleh di-escape dengan backslash.
		whole := fmt.Sprintf("min(%d/iw,%d/ih)", targetW, targetH)
		full := fmt.Sprintf("max(%d/iw,%d/ih)", targetW, targetH)
		scale := fmt.Sprintf("(%s+(%s-%s)*%.4f)", whole, full, whole, float64(zoom)/100)
		// Dibulatkan genap: encoder h264 menolak dimensi ganjil.
		foreground = fmt.Sprintf(
			"scale=w='trunc(iw*%s/2)*2':h='trunc(ih*%s/2)*2':flags=lanczos"+
				",crop='min(iw,%d)':'min(ih,%d)'",
			scale, scale, targetW, targetH)
	}

	// Mulai 100 gambar sudah menutupi bingkai, jadi latar tidak terlihat lagi.
	if zoom >= 100 {
		return foreground
	}
	return withBackground(l.Background, foreground, targetW, targetH)
}

// centerCropChain: potongan tengah, zoom mengatur besarnya kotak di dalam
// bingkai. 100 = memenuhi bingkai.
func centerCropChain(l Layout, targetW, targetH int) string {
	zoom := clampZoom(l.Zoom, 100)

	fw := evenBox(targetW * zoom / 100)
	fh := evenBox(targetH * zoom / 100)
	foreground := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d",
		fw, fh, fw, fh)

	if zoom >= 100 {
		return foreground
	}
	return withBackground(l.Background, foreground, targetW, targetH)
}

// withBackground mengisi ruang yang tidak terjangkau video.
func withBackground(background, foreground string, targetW, targetH int) string {
	if background == "black" {
		// Tidak perlu menggandakan aliran: cukup beri bantalan hitam di sekeliling.
		return foreground + fmt.Sprintf(",pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black", targetW, targetH)
	}
	// Blur: video dipakai dua kali — satu jadi latar yang dibesarkan &
	// diburamkan, satu lagi jadi gambar depan yang ditumpuk di tengahnya.
	return fmt.Sprintf(
		"split=2[bg][fg];"+
			"[bg]scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d,gblur=sigma=20[bgb];"+
			"[fg]%s[fgb];"+
			"[bgb][fgb]overlay=(W-w)/2:(H-h)/2",
		targetW, targetH, targetW, targetH, foreground)
}

// clampZoom menjepit zoom ke rentang yang sah. whenUnset dipakai saat nilainya
// nol DAN nol bukan nilai yang bermakna di mode itu.
func clampZoom(zoom, whenUnset int) int {
	if zoom <= 0 && whenUnset > 0 {
		return whenUnset
	}
	return min(maxZoom, max(0, zoom))
}

func evenBox(n int) int {
	if n < 2 {
		return 2
	}
	if n%2 != 0 {
		n--
	}
	return n
}

// ClipReframe memotong [start,end], menyesuaikan ke rasio target, dan membakar
// subtitle .ass bila diisi. Mode "fit" menampilkan seluruh gambar di atas latar;
// selain itu potong tengah. Zoom mengatur besarnya di dalam bingkai.
func (c *Client) ClipReframe(ctx context.Context, input string, start, end float64, targetW, targetH int, enc EncodeOpts, out string) error {
	dur := end - start
	if dur <= 0 {
		return fmt.Errorf("invalid clip duration: %.2f", dur)
	}
	// Pastikan folder tujuan ada. Render satu job bisa berjalan puluhan menit;
	// bila foldernya sempat terhapus atau dipindah di tengah jalan, ffmpeg baru
	// gagal setelah selesai meng-encode (ENOENT, exit 254) dan seluruh kerjanya
	// terbuang. Ini folder kerja job sendiri, jadi membuatnya ulang aman.
	if dir := filepath.Dir(out); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("output folder %q: %w", dir, err)
		}
	}
	// Filter subtitle (ditempel di akhir rantai).
	sub := ""
	if enc.AssPath != "" {
		sub = fmt.Sprintf("subtitles='%s'", escapeFilterPath(enc.AssPath))
		if enc.FontsDir != "" {
			sub += fmt.Sprintf(":fontsdir='%s'", escapeFilterPath(enc.FontsDir))
		}
	}

	vf := ReframeFilter(Layout{
		Mode: enc.Mode, Background: enc.Background, Zoom: enc.Zoom, Watermark: enc.Watermark,
	}, targetW, targetH)
	if enc.FPS > 0 {
		vf += fmt.Sprintf(",fps=%d", enc.FPS)
	}
	if sub != "" {
		vf += "," + sub
	}

	crf := enc.CRF
	if crf == "" {
		crf = "20"
	}
	preset := enc.Preset
	if preset == "" {
		preset = "medium"
	}
	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", start),
		"-i", CLIPath(input),
		"-t", fmt.Sprintf("%.3f", dur),
		"-map", "0:v:0", // ambil video pertama
		"-map", "0:a:0?", // ambil audio pertama bila ada (? = opsional)
		"-vf", vf,
		"-c:v", "libx264",
		"-preset", preset,
		"-crf", crf,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "160k",
		"-ac", "2", // paksa stereo (kompatibel semua pemutar)
		"-movflags", "+faststart",
		CLIPath(out),
	}
	return c.run(ctx, "clip+reframe", args)
}

// ExtractFrame mengambil 1 frame pada detik t, disesuaikan ke rasio target
// memakai mode reframe yang sama dengan render, dikembalikan sebagai JPEG
// (untuk preview subtitle di GUI). Hasilnya = satu frame dari klip jadi,
// jadi koordinat subtitle yang diatur di preview berlaku apa adanya.
func (c *Client) ExtractFrame(ctx context.Context, input string, t float64, targetW, targetH int, layout Layout) ([]byte, error) {
	vf := ReframeFilter(layout, targetW, targetH)
	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", t),
		"-i", CLIPath(input),
		"-frames:v", "1",
		"-vf", vf,
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"pipe:1",
	}
	cmd := exec.CommandContext(ctx, c.FFmpeg, args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("frame extraction failed: %w", err)
	}
	return out.Bytes(), nil
}

func (c *Client) run(ctx context.Context, label string, args []string) error {
	cmd := exec.CommandContext(ctx, c.FFmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w: %s", label, err, summarizeError(stderr.String()))
	}
	return nil
}

// errorKeywords = penanda baris stderr yang benar-benar menjelaskan kegagalan.
var errorKeywords = []string{
	"error", "invalid", "no such", "permission", "unable",
	"not found", "denied", "no space", "cannot", "failed",
}

// summarizeError menyaring baris stderr ffmpeg yang menjelaskan sebab kegagalan.
//
// Dulu bagian ini hanya memotong 500 karakter TERAKHIR — padahal ekor keluaran
// ffmpeg selalu berisi blok statistik x264, bagian yang paling tidak berguna,
// sementara baris sebabnya tercetak jauh lebih awal dan ikut terbuang. Kasus
// nyatanya: "exit status 254" yang ternyata "No such file or directory" karena
// folder keluaran hilang saat render, tapi pesan itu tidak pernah terlihat.
func summarizeError(stderr string) string {
	var important []string
	seen := map[string]bool{}
	for _, ln := range strings.Split(stderr, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || seen[ln] {
			continue
		}
		low := strings.ToLower(ln)
		// "Conversion failed!" selalu muncul dan tidak menjelaskan apa pun —
		// dipakai hanya kalau tak ada baris lain yang lebih berguna.
		if low == "conversion failed!" {
			continue
		}
		for _, k := range errorKeywords {
			if strings.Contains(low, k) {
				important = append(important, ln)
				seen[ln] = true
				break
			}
		}
	}
	if len(important) == 0 {
		// Tak ada baris yang cocok: kembali ke ekor keluaran seadanya.
		tail := strings.TrimSpace(stderr)
		if len(tail) > 300 {
			tail = "…" + tail[len(tail)-300:]
		}
		return tail
	}
	// Baris terakhir yang cocok biasanya sebab paling dekat dengan kegagalan;
	// dua baris sebelumnya ikut dibawa sebagai konteks.
	if len(important) > 3 {
		important = important[len(important)-3:]
	}
	message := strings.Join(important, " | ")
	if hint := errorHint(message); hint != "" {
		message += ": " + hint
	}
	return message
}

// errorHint menerjemahkan galat ffmpeg yang sering muncul jadi langkah yang
// bisa ditindaklanjuti pengguna.
func errorHint(msg string) string {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "no such file or directory"):
		return "the output folder or file is missing (deleted or moved during the render?)"
	case strings.Contains(low, "permission denied"):
		return "write permission denied for the output folder"
	case strings.Contains(low, "no space left"):
		return "the disk is full"
	case strings.Contains(low, "invalid data found"):
		return "the source file is corrupt or its format is unsupported"
	}
	return ""
}

// escapeFilterPath meng-escape karakter khusus dalam path untuk filter ffmpeg.
func escapeFilterPath(p string) string {
	p = strings.ReplaceAll(p, `\`, `\\`)
	p = strings.ReplaceAll(p, `:`, `\:`)
	p = strings.ReplaceAll(p, `'`, `\'`)
	return p
}
