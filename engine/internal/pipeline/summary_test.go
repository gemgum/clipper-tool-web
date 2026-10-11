package pipeline

import (
	"strings"
	"testing"
	"time"
)

// Angka di sini diambil dari job 20:24 yang sungguhan — job yang membuat fitur
// ini dibutuhkan: dugaan awalnya render yang lama, padahal koreksi transkrip
// memakan 20 menit untuk mengolah teks halusinasi.
func realisticSummary() Summary {
	r := newRecorder()
	r.add("Extract audio", 12*time.Second, "ffmpeg")
	r.add("Audio features", 3*time.Second, "C++ worker")
	r.add("Transcribe", 42*time.Minute+31*time.Second, "whisper large-v3")
	r.add("Correct transcript", 19*time.Minute+48*time.Second, "Ollama (llama3.1), 3 terms")
	r.add("Select moments", 90*time.Second, "Ollama (llama3.1)")
	r.add("Render 6 clips", 4*time.Minute+52*time.Second, "1080p, hd")

	total := 0.0
	for _, st := range r.stages {
		total += st.Sec
	}
	return Summary{
		JobID: "job_0001", VideoSec: 2304, TotalSec: total, Clips: 6,
		Stages: r.stages, Realtime: 2304 / total,
	}
}

func TestSummaryFormat(t *testing.T) {
	out := realisticSummary().Format()
	t.Logf("%s", out)

	for _, want := range []string{
		"job_0001",
		"video 38:24",           // durasi video terbaca
		"Transcribe",            //
		"42:31",                 // tahap terlama
		"whisper large-v3",      // keterangan mesin ikut tampil
		"3 terms",               // daftar istilah ikut tercatat
		"Total",                 //
		"longer than the video", // rasio realtime diterjemahkan
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ringkasan tidak memuat %q", want)
		}
	}
}

// Kolom harus lurus: seluruh baris tabel wajib sama lebarnya, kalau tidak
// tampilannya berantakan di terminal maupun kotak log GUI.
func TestSummaryColumnsLineUp(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(realisticSummary().Format()), "\n")
	width := 0
	for _, l := range lines {
		if n := len([]rune(l)); strings.HasPrefix(l, "─") {
			if width == 0 {
				width = n
			} else if n != width {
				t.Errorf("lebar garis pemisah tidak seragam: %d vs %d", n, width)
			}
		}
	}
	if width == 0 {
		t.Fatal("tidak ada garis pemisah sama sekali")
	}
	// Tiap baris tahap harus punya persentase yang sejajar — dicek lewat posisi
	// karakter '%' yang harus sama di semua baris tahap.
	//
	// Posisinya dihitung PER KARAKTER: strings.Index mengembalikan indeks byte,
	// dan baris yang batangnya lebih panjang punya lebih banyak byte meski
	// kolomnya lurus. Persis jebakan yang sama yang bikin padRight harus ada.
	runeIndex := func(s string, target rune) int {
		for i, r := range []rune(s) {
			if r == target {
				return i
			}
		}
		return -1
	}
	pos := -1
	for _, l := range lines {
		i := runeIndex(l, '%')
		if i < 0 {
			continue
		}
		if pos == -1 {
			pos = i
		} else if i != pos {
			t.Errorf("kolom persen tidak lurus: %d vs %d pada %q", i, pos, l)
		}
	}
}

// Tahap yang sangat singkat tidak boleh hilang jadi batang kosong — justru
// tahap kecil di antara tahap raksasa yang perlu terlihat keberadaannya.
func TestTinyStageStillDrawsABar(t *testing.T) {
	if got := bar(3.0 / 4000.0); got == "" {
		t.Error("tahap 3 detik dari total ~1 jam seharusnya tetap tergambar")
	}
	if got := bar(0); got != "" {
		t.Errorf("durasi nol seharusnya tanpa batang, dapat %q", got)
	}
}

func TestHHMMSS(t *testing.T) {
	cases := map[float64]string{
		0: "0:00", 9: "0:09", 90: "1:30", 599: "9:59",
		3600: "1:00:00", 4136: "1:08:56",
	}
	for in, want := range cases {
		if got := hhmmss(in); got != want {
			t.Errorf("hhmmss(%v) = %q, want %q", in, got, want)
		}
	}
}
