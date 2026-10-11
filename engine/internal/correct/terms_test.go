package correct

import (
	"strings"
	"testing"
)

func TestParseTerms(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"kosong", "   ", nil},
		{"koma", "Londo Ireng, Mahfud MD", []string{"Londo Ireng", "Mahfud MD"}},
		{"baris baru", "Londo Ireng\nMahfud MD", []string{"Londo Ireng", "Mahfud MD"}},
		{"campur pemisah", "Londo Ireng;Mahfud MD\nURI", []string{"Londo Ireng", "Mahfud MD", "URI"}},
		{"spasi berlebih", "  Londo   Ireng  ,  URI ", []string{"Londo Ireng", "URI"}},
		{"entri kosong dibuang", "Londo Ireng,,, ,URI", []string{"Londo Ireng", "URI"}},
		{"duplikat, ejaan pertama menang", "Londo Ireng, londo ireng", []string{"Londo Ireng"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ParseTerms(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("ParseTerms(%q) = %v, want %v", c.in, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("ParseTerms(%q) = %v, want %v", c.in, got, c.want)
				}
			}
		})
	}
}

// Daftar yang kepanjangan harus dipangkas: prompt yang membengkak menggeser
// perhatian model dari transkripnya sendiri.
func TestParseTermsCaps(t *testing.T) {
	var b strings.Builder
	for i := 0; i < maxTerms+20; i++ {
		b.WriteString("term")
		b.WriteString(strings.Repeat("x", i%5+1))
		b.WriteString(",")
	}
	if got := ParseTerms(b.String()); len(got) > maxTerms {
		t.Errorf("got %d terms, want at most %d", len(got), maxTerms)
	}
	long := strings.Repeat("a", maxTermLen+1)
	if got := ParseTerms(long); len(got) != 0 {
		t.Errorf("an over-long term should be dropped, got %v", got)
	}
}

// Inti fiturnya: koreksi menuju ejaan di daftar istilah tidak boleh memakan
// jatah perubahan, sebab segmen pendek hanya berjatah satu kata dan jatah itu
// kerap sudah habis oleh pembenahan lain.
func TestTermCorrectionSurvivesGuard(t *testing.T) {
	exempt := termWords([]string{"Londo Ireng"})

	before := "Karena Londo Irang itu sejarah pengkhianatan terhadap perjuangan."
	after := "Karena Londo Ireng itu sejarah pengkhianatan terhadap perjuangan."
	if ok, why := acceptable(before, after, exempt); !ok {
		t.Errorf("term correction was rejected: %s", why)
	}

	// Dua kemunculan dalam satu segmen pendek: tanpa pengecualian ini pasti
	// melewati jatah dan koreksinya dibuang diam-diam.
	before2 := "Londo Irang itu, ya Londo Irang."
	after2 := "Londo Ireng itu, ya Londo Ireng."
	if ok, why := acceptable(before2, after2, exempt); !ok {
		t.Errorf("repeated term correction was rejected: %s", why)
	}

	// Pengecualian TIDAK boleh jadi pintu belakang: menulis ulang kalimat tetap
	// harus ditolak walau daftar istilah ada.
	rewrite := "Sebab bangsa ini pernah dikhianati oleh kaki tangan penjajah."
	if ok, _ := acceptable(before, rewrite, exempt); ok {
		t.Error("a full rewrite must still be rejected when terms are set")
	}
}

// Tanpa daftar istilah, perilaku pagar pengaman harus persis seperti sebelumnya.
func TestGuardUnchangedWithoutTerms(t *testing.T) {
	before := "Karena Londo Irang itu sejarah pengkhianatan terhadap perjuangan."
	after := "Karena Londo Ireng itu sejarah pengkhianatan terhadap perjuangan."
	if ok, why := acceptable(before, after, nil); !ok {
		t.Errorf("a single-word fix should pass on its own merits: %s", why)
	}
}

func TestSystemPromptTerms(t *testing.T) {
	plain := systemPrompt("id", nil)
	if strings.Contains(plain, "KNOWN TERMS") {
		t.Error("the terms section must be absent when no terms are given")
	}
	withTerms := systemPrompt("id", []string{"Londo Ireng", "Mahfud MD"})
	for _, want := range []string{"KNOWN TERMS", "Londo Ireng", "Mahfud MD"} {
		if !strings.Contains(withTerms, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}
}

// Daftar istilah mengubah keluaran koreksi, jadi ia wajib mengubah kunci cache —
// kalau tidak, menambah istilah lalu menjalankan ulang akan memungut hasil lama.
func TestCacheVersionTracksTerms(t *testing.T) {
	if CacheVersion(nil) != PromptVersion {
		t.Error("an empty list should leave the version untouched")
	}
	a := CacheVersion([]string{"Londo Ireng"})
	b := CacheVersion([]string{"Londo Ireng", "URI"})
	if a == PromptVersion || a == b {
		t.Errorf("terms must change the cache version: %q vs %q", a, b)
	}
}
