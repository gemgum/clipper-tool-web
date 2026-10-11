package correct

import (
	"math"
	"testing"

	"github.com/gemgum/clipper/engine/internal/types"
)

// words membuat deret kata bertimestamp berdurasi tetap, meniru keluaran
// whisper dengan -ojf.
func words(start, per float64, texts ...string) []types.Word {
	out := make([]types.Word, len(texts))
	t := start
	for i, text := range texts {
		out[i] = types.Word{Start: t, End: t + per, Text: text}
		t += per
	}
	return out
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// Kasus terpenting: koreksi salah dengar mengganti satu kata dengan satu kata.
// Seluruh timestamp harus tetap persis seperti aslinya — inilah yang menjaga
// mode karaoke & word tetap tepat.
func TestRetimeSubstitutionKeepsEveryTimestamp(t *testing.T) {
	original := words(10, 0.5, "poin", "yang", "lagi", "rime", "sekarang")
	got := retime(original, "poin yang lagi rame sekarang", 10, 12.5)

	if len(got) != 5 {
		t.Fatalf("dapat %d kata, ingin 5", len(got))
	}
	for i := range got {
		if !near(got[i].Start, original[i].Start) || !near(got[i].End, original[i].End) {
			t.Errorf("kata %d bergeser: %.2f-%.2f, ingin %.2f-%.2f",
				i, got[i].Start, got[i].End, original[i].Start, original[i].End)
		}
	}
	if got[3].Text != "rame" {
		t.Errorf("kata terkoreksi = %q, ingin %q", got[3].Text, "rame")
	}
}

// Tanda baca & huruf besar menempel di kata, bukan mengganti kata. Timing tetap.
func TestRetimePunctuationKeepsTimestamps(t *testing.T) {
	original := words(0, 0.4, "dari", "kemarin", "harusnya", "digeledah")
	got := retime(original, "Dari kemarin harusnya digeledah.", 0, 1.6)

	if len(got) != 4 {
		t.Fatalf("dapat %d kata, ingin 4", len(got))
	}
	for i := range got {
		if !near(got[i].Start, original[i].Start) {
			t.Errorf("kata %d bergeser dari %.2f ke %.2f", i, original[i].Start, got[i].Start)
		}
	}
	if got[0].Text != "Dari" || got[3].Text != "digeledah." {
		t.Errorf("teks tidak ikut terkoreksi: %q … %q", got[0].Text, got[3].Text)
	}
}

// Tanda hubung dialog dibuang → satu token hilang. Kata yang tersisa harus
// tetap memakai timing kata aslinya, bukan bergeser satu langkah.
func TestRetimeDeletionKeepsRemainingWordsAligned(t *testing.T) {
	original := words(5, 0.5, "-", "Iya", "dong")
	got := retime(original, "Iya dong.", 5, 6.5)

	if len(got) != 2 {
		t.Fatalf("dapat %d kata, ingin 2", len(got))
	}
	// "Iya" tetap mulai di 5.5, bukan mundur ke 5.0.
	if !near(got[0].Start, 5.5) {
		t.Errorf("kata pertama mulai %.2f, ingin 5.50", got[0].Start)
	}
	// Waktu token yang dibuang diserap kata sebelumnya — tidak ada lubang.
	if !near(got[0].End, got[1].Start) {
		t.Errorf("ada celah antara kata: %.2f → %.2f", got[0].End, got[1].Start)
	}
}

// Kata sisipan tidak punya timing sendiri; ia harus membagi celah di sekitarnya
// tanpa menabrak kata bertimestamp asli.
func TestRetimeInsertionStaysWithinNeighbours(t *testing.T) {
	original := words(0, 1.0, "saya", "makan")
	got := retime(original, "saya sedang makan", 0, 2)

	if len(got) != 3 {
		t.Fatalf("dapat %d kata, ingin 3", len(got))
	}
	if got[1].Text != "sedang" {
		t.Errorf("kata sisipan = %q", got[1].Text)
	}
	// Urutan harus tetap naik dan tiap kata punya durasi positif.
	for i := range got {
		if got[i].End <= got[i].Start {
			t.Errorf("kata %d berdurasi nol: %.3f-%.3f", i, got[i].Start, got[i].End)
		}
		if i > 0 && got[i].Start < got[i-1].Start {
			t.Errorf("urutan mundur di kata %d", i)
		}
	}
	// Kata terakhir tetap berakhir di akhir segmen.
	if got[2].End > 2.0001 {
		t.Errorf("kata terakhir melewati akhir segmen: %.3f", got[2].End)
	}
}

// Segmen tanpa timestamp per kata (whisper tanpa -ojf) tetap harus menghasilkan
// kata bertimestamp, dibagi rata.
func TestRetimeWithoutWordTimestampsSpreadsEvenly(t *testing.T) {
	got := retime(nil, "satu dua tiga empat", 0, 4)
	if len(got) != 4 {
		t.Fatalf("dapat %d kata, ingin 4", len(got))
	}
	for i := range got {
		if !near(got[i].Start, float64(i)) {
			t.Errorf("kata %d mulai %.2f, ingin %.2f", i, got[i].Start, float64(i))
		}
	}
}

func TestContentEditsIgnoresDialogueDashes(t *testing.T) {
	// Membuang tanda hubung dialog bukan perubahan kata — kalau dihitung,
	// pagar pengaman akan menolak koreksi yang justru paling kita inginkan.
	oldW := []string{"-", "Oh", "ya?", "-", "Iya", "dulu."}
	newW := []string{"Oh", "ya?", "Iya", "dulu."}
	changed, total := contentEdits(oldW, newW, nil)
	if changed != 0 {
		t.Errorf("changed = %d, ingin 0 (hanya tanda hubung yang hilang)", changed)
	}
	if total != 4 {
		t.Errorf("total kata isi = %d, ingin 4", total)
	}
}

func TestContentEditsCountsSubstitutionOnce(t *testing.T) {
	oldW := []string{"poin", "yang", "lagi", "rime", "sekarang"}
	newW := []string{"poin", "yang", "lagi", "rame", "sekarang"}
	if changed, _ := contentEdits(oldW, newW, nil); changed != 1 {
		t.Errorf("changed = %d, ingin 1", changed)
	}
}

// Kasus nyata dari uji dengan qwen2.5: nama tak dikenal dipecah jadi dua kata
// DAN satu kata hilang. Dulu ini lolos karena penyejajaran membacanya sebagai
// rangkaian substitusi.
func TestContentEditsCatchesSplitNameAndDroppedWord(t *testing.T) {
	oldW := []string{"-", "Agak", "menyakitkan", "ya", "tujuan", "Londo-Irang.", "-", "Menyakitkan."}
	newW := []string{"Agak", "menyakitkan", "ya", "tujuan", "Londo-I", "rang."}
	changed, total := contentEdits(oldW, newW, nil)
	if total != 6 {
		t.Fatalf("total kata isi = %d, ingin 6", total)
	}
	if changed < 2 {
		t.Errorf("changed = %d, ingin >= 2 supaya melebihi jatah", changed)
	}
}

func TestNormalizeStripsEdgePunctuationOnly(t *testing.T) {
	cases := map[string]string{
		"Dari":        "dari",
		"digeledah.":  "digeledah",
		`"Kerugian`:   "kerugian",
		"aling-aling": "aling-aling", // tanda hubung di tengah dipertahankan
		"-":           "",
	}
	for in, want := range cases {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, ingin %q", in, got, want)
		}
	}
}
