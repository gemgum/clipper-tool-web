package pipeline

import (
	"fmt"
	"strings"
	"time"
)

// barWidth = lebar batang proporsi, dalam karakter.
const barWidth = 22

// stage adalah satu tahap yang sudah selesai diukur.
type stage struct {
	Label string        `json:"label"`
	Dur   time.Duration `json:"-"`
	Sec   float64       `json:"seconds"`
	Note  string        `json:"note,omitempty"` // model/mesin/keterangan
}

// Summary adalah rincian waktu satu job.
//
// Dibuat karena menebak "tahap mana yang lama" selalu meleset: pada job 20:24
// dugaannya render, padahal 20 dari 69 menit habis di koreksi transkrip yang
// mengolah teks halusinasi. Angka per tahap membuat perbandingan antar
// percobaan (model, mesin, CPU vs GPU) jadi mungkin.
type Summary struct {
	JobID    string  `json:"job_id"`
	VideoSec float64 `json:"video_seconds"`
	TotalSec float64 `json:"total_seconds"`
	Clips    int     `json:"clips"`
	Stages   []stage `json:"stages"`
	Realtime float64 `json:"realtime_factor"` // durasi video / total waktu
	// Cached menandai transkrip diambil dari cache. Wajib ikut dilaporkan:
	// tanpa itu rasio realtime terbaca seolah mesinnya luar biasa cepat,
	// padahal tahap termahalnya memang tidak dijalankan — dan angka itu jadi
	// tidak sebanding dengan run penuh.
	Cached bool `json:"transcript_cached"`
}

// plural memilih bentuk tunggal/jamak. "Render 1 clips" itu cacat kecil yang
// langsung terbaca dan membuat sisanya ikut terasa asal jadi.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// recorder mengumpulkan durasi tiap tahap selama pipeline berjalan.
type recorder struct {
	stages []stage
	start  time.Time
}

func newRecorder() *recorder {
	return &recorder{start: time.Now()}
}

// add mencatat satu tahap. Tahap berdurasi nol tetap dicatat bila punya
// keterangan — "0:00 (from cache)" itu informasi, bukan kekosongan.
func (r *recorder) add(label string, d time.Duration, note string) {
	if r == nil {
		return
	}
	r.stages = append(r.stages, stage{Label: label, Dur: d, Sec: d.Seconds(), Note: note})
}

// since adalah pembantu agar pemanggil cukup menulis satu baris di akhir tahap.
func (r *recorder) since(label string, t0 time.Time, note string) {
	r.add(label, time.Since(t0), note)
}

func (r *recorder) summary(jobID string, videoSec float64, clips int, cached bool) Summary {
	total := time.Since(r.start)
	s := Summary{
		JobID:    jobID,
		VideoSec: videoSec,
		TotalSec: total.Seconds(),
		Clips:    clips,
		Stages:   r.stages,
		Cached:   cached,
	}
	if total > 0 && videoSec > 0 {
		s.Realtime = videoSec / total.Seconds()
	}
	return s
}

// hhmmss memformat durasi jadi m:ss, atau h:mm:ss bila melewati satu jam.
func hhmmss(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	t := int(sec + 0.5)
	h, m, s := t/3600, (t/60)%60, t%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// bar menggambar batang proporsi. Memakai blok seperdelapan agar tahap yang
// sangat singkat tetap terlihat sebagai goresan, bukan menghilang jadi kosong —
// tahap 3 detik di antara tahap 40 menit tetap harus kelihatan ada.
func bar(frac float64) string {
	if frac <= 0 {
		return ""
	}
	if frac > 1 {
		frac = 1
	}
	eighths := int(frac * float64(barWidth) * 8)
	if eighths < 1 {
		eighths = 1
	}
	full, rem := eighths/8, eighths%8
	b := strings.Repeat("█", full)
	if rem > 0 {
		b += string([]rune("▏▎▍▌▋▊▉")[rem-1])
	}
	return b
}

// padRight menambahkan spasi sampai lebar w DIHITUNG PER KARAKTER, bukan per
// byte. Kata kerja %-*s milik fmt memadding per byte, dan batang proporsi kita
// memakai blok Unicode yang 3 byte per karakter — memakai %-*s membuat kolom
// persen melenceng makin jauh setiap batang bertambah panjang.
func padRight(s string, w int) string {
	if n := len([]rune(s)); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// Format menyusun ringkasan jadi tabel monospace.
//
// Ditulis sebagai teks siap tampil, bukan data mentah, karena tujuannya dibaca
// manusia di dua tempat sekaligus: terminal (CLI) dan kotak log GUI — keduanya
// monospace, jadi satu bentuk cukup untuk dua-duanya.
func (s Summary) Format() string {
	// Lebar kolom label mengikuti isi terpanjang supaya kolom waktu selalu lurus.
	labelW := 0
	for _, st := range s.Stages {
		if n := len([]rune(st.Label)); n > labelW {
			labelW = n
		}
	}
	if labelW < 18 {
		labelW = 18
	}

	rule := strings.Repeat("─", labelW+barWidth+26)
	var b strings.Builder
	head := fmt.Sprintf("Job summary · %s", s.JobID)
	if s.VideoSec > 0 {
		head += fmt.Sprintf(" · video %s", hhmmss(s.VideoSec))
	}
	fmt.Fprintf(&b, "\n%s\n %s\n%s\n", rule, head, rule)

	for _, st := range s.Stages {
		frac := 0.0
		if s.TotalSec > 0 {
			frac = st.Sec / s.TotalSec
		}
		fmt.Fprintf(&b, " %s %8s  %s %5.1f%%",
			padRight(st.Label, labelW), hhmmss(st.Sec), padRight(bar(frac), barWidth), frac*100)
		if st.Note != "" {
			fmt.Fprintf(&b, "  %s", st.Note)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "%s\n", rule)
	fmt.Fprintf(&b, " %s %8s", padRight("Total", labelW), hhmmss(s.TotalSec))
	if s.Realtime > 0 {
		// Rasio realtime adalah angka pembanding antar percobaan: berapa detik
		// video yang selesai per detik jam dinding. Bebas dari panjang video,
		// jadi hasil uji 2 menit bisa dibandingkan dengan job 40 menit.
		fmt.Fprintf(&b, "  %s", realtimeNote(s.Realtime, s.Cached))
	}
	if s.Clips > 0 {
		fmt.Fprintf(&b, " · %d %s", s.Clips, plural(s.Clips, "clip", "clips"))
	}
	fmt.Fprintf(&b, "\n%s", rule)
	return b.String()
}

// realtimeNote menerjemahkan rasio jadi kalimat yang tidak perlu ditafsirkan
// ulang. "0.56×" saja ambigu — lebih cepat atau lebih lambat?
func realtimeNote(rt float64, cached bool) string {
	s := fmt.Sprintf("%.2f× realtime", rt)
	if cached {
		return s + " (transcript reused: not comparable to a full run)"
	}
	if rt >= 1 {
		return s + " (faster than the video)"
	}
	return s + fmt.Sprintf(" (%.1f× longer than the video)", 1/rt)
}
