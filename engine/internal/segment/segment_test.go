package segment

import (
	"fmt"
	"testing"

	"github.com/gemgum/clipper/engine/internal/types"
)

// sampleTranscript membuat transkrip sintetis: segmen 3 detik, tiap segmen ke-4
// mengakhiri kalimat (ada titik).
func sampleTranscript(n int) types.Transcript {
	var tr types.Transcript
	for i := 0; i < n; i++ {
		text := fmt.Sprintf("kalimat bagian %d", i)
		if i%4 == 3 {
			text += "."
		}
		tr.Segments = append(tr.Segments, types.TranscriptSegment{
			Start: float64(i) * 3,
			End:   float64(i+1) * 3,
			Text:  text,
		})
	}
	return tr
}

func TestCandidatesFollowDurationPreset(t *testing.T) {
	tr := sampleTranscript(120) // 6 menit
	targetMin, targetMax := 48.0, 75.0
	cands := BuildCandidates(tr, targetMin, targetMax)
	if len(cands) < 2 {
		t.Fatalf("hanya %d kandidat", len(cands))
	}
	// Kandidat terakhir boleh lebih pendek (sisa transkrip).
	for i, c := range cands[:len(cands)-1] {
		d := c.Duration()
		if d < targetMin {
			t.Errorf("kandidat %d hanya %.0f detik, di bawah target minimum %.0f", i, d, targetMin)
		}
		if d > targetMax {
			t.Errorf("kandidat %d %.0f detik, melewati target maksimum %.0f", i, d, targetMax)
		}
	}
}

// Dulu semua klip menempel di batas bawah (preset auto → semua ~30 detik).
func TestCandidatesDoNotStickToLowerBound(t *testing.T) {
	tr := sampleTranscript(200)
	targetMin, targetMax := 45.0, 120.0
	cands := BuildCandidates(tr, targetMin, targetMax)
	if len(cands) < 2 {
		t.Fatalf("hanya %d kandidat", len(cands))
	}
	ideal := idealDuration(targetMin, targetMax)
	for i, c := range cands[:len(cands)-1] {
		if d := c.Duration(); d < ideal {
			t.Errorf("kandidat %d hanya %.0f detik, di bawah durasi ideal %.0f", i, d, ideal)
		}
	}
}

// Jeda diam panjang di akhir kalimat boleh memotong lebih awal (pergantian topik).
func TestLongSilenceCutsEarlier(t *testing.T) {
	tr := types.Transcript{Segments: []types.TranscriptSegment{
		{Start: 0, End: 25, Text: "bagian pertama panjang"},
		{Start: 25, End: 50, Text: "dan ini penutupnya."},
		{Start: 55, End: 80, Text: "topik baru dimulai"}, // jeda 5 detik
		{Start: 80, End: 105, Text: "lalu selesai."},
	}}
	cands := BuildCandidates(tr, 45, 120)
	if len(cands) != 2 {
		t.Fatalf("dapat %d kandidat, ingin 2 (dipisah oleh jeda)", len(cands))
	}
	if cands[0].End != 50 {
		t.Errorf("potongan pertama berakhir di %.0f, ingin 50 (sebelum jeda)", cands[0].End)
	}
}
