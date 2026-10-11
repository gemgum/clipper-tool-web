package pipeline

import (
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/types"
)

// trFrom membangun transkrip dari daftar teks; timestamp tidak dipakai detektor.
func trFrom(texts []string) types.Transcript {
	tr := types.Transcript{Language: "id"}
	for i, t := range texts {
		tr.Segments = append(tr.Segments, types.TranscriptSegment{
			Start: float64(i), End: float64(i) + 1, Text: t,
		})
	}
	return tr
}

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

func TestDetectRepetitionLoop(t *testing.T) {
	// Percakapan wajar: banyak sisipan pendek berulang, tapi tidak mendominasi
	// dan tidak berderet panjang.
	varied := []string{}
	for i := 0; i < 40; i++ {
		varied = append(varied, "Iya.", "Menurut saya begitu, Pak.", "Iya.", "Lalu bagaimana kelanjutannya?")
	}

	cases := []struct {
		name    string
		texts   []string
		wantErr bool
	}{
		{"kosong", nil, false},
		{"pendek", []string{"Halo.", "Apa kabar?"}, false},
		{"percakapan wajar", varied, false},
		{
			// Deretan identik tepat di bawah ambang, sisanya beragam.
			name:    "deretan di bawah ambang",
			texts:   append(repeat("Terima kasih.", loopRunMax-1), varied...),
			wantErr: false,
		},
		{
			// Kasus nyata: loop mulai di tengah lalu tidak pernah pulih.
			name:    "loop menguasai transkrip",
			texts:   append([]string{"Seharusnya sudah digeledah.", "Iya dong."}, repeat("Terima kasih Pak Febri.", 200)...),
			wantErr: true,
		},
		{
			// Loop hanya sebagian: porsi keseluruhan masih di bawah loopShareMax,
			// jadi hanya deretan berturut-turut yang bisa menangkapnya.
			name:    "loop sebagian",
			texts:   append(varied, repeat("Terima kasih Pak Febri.", loopRunMax)...),
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := detectRepetitionLoop(trFrom(c.texts))
			if (err != nil) != c.wantErr {
				t.Fatalf("detectRepetitionLoop() error = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

// Pesan error harus menyebut kalimat pengulangnya supaya pengguna bisa mengenali
// bagian audio yang bermasalah.
func TestDetectRepetitionLoopMessage(t *testing.T) {
	err := detectRepetitionLoop(trFrom(repeat("Terima kasih Pak Febri.", 100)))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "terima kasih pak febri.") {
		t.Errorf("message should quote the repeated line, got: %v", err)
	}
	if !strings.Contains(err.Error(), "100 of 100 segments") {
		t.Errorf("message should report the counts, got: %v", err)
	}
}

// Perbedaan huruf besar/kecil dan spasi tidak boleh menyembunyikan loop —
// keluaran whisper yang sudah dikoreksi LLM kerap berganti kapitalisasi.
func TestDetectRepetitionLoopNormalizes(t *testing.T) {
	var texts []string
	for i := 0; i < 50; i++ {
		texts = append(texts, "terima kasih pak febri.", "Terima Kasih   Pak Febri.")
	}
	if err := detectRepetitionLoop(trFrom(texts)); err == nil {
		t.Error("expected case/space differences to still count as one repeated line")
	}
}
