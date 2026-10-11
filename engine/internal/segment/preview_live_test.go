package segment

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/gemgum/clipper/engine/internal/types"
)

// Uji terhadap transkrip sungguhan di cache. Dilewati bila tidak diset, sebab
// data/ tidak ikut di-commit.
//
//	CLIPPER_TEST_TRANSCRIPT=data/cache/transcripts/<kunci>.json \
//	  go test ./internal/segment/ -run Live -v
func TestOpeningPreviewOnRealTranscriptLive(t *testing.T) {
	path := os.Getenv("CLIPPER_TEST_TRANSCRIPT")
	if path == "" {
		t.Skip("set CLIPPER_TEST_TRANSCRIPT untuk menjalankan uji ini")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var tr types.Transcript
	if err := json.Unmarshal(raw, &tr); err != nil {
		t.Fatal(err)
	}
	last := tr.Segments[len(tr.Segments)-1].End
	t.Logf("%d segmen, %.0f detik", len(tr.Segments), last)

	// Bentuk sinyalnya dicetak dulu: inilah yang dipakai menyetel previewWindow.
	// Pada video bercuplikan rasionya bertahan tinggi lalu jatuh tajam; pada
	// video tanpa cuplikan ia tidak pernah tinggi sejak awal.
	for at := 0.0; at < previewOpening; at += previewStep {
		r, ok := echoRatio(tr, at, at+previewWindow)
		if !ok {
			t.Logf("%4.0f-%4.0f  (tak dapat dinilai)", at, at+previewWindow)
			continue
		}
		t.Logf("%4.0f-%4.0f  %.2f", at, at+previewWindow, r)
	}

	end := OpeningPreviewEnd(tr)
	t.Logf("→ ujung cuplikan: %.0f detik", end)

	// Klip yang dulu lolos pada video Mahfud: 78,4 detik. Dicetak, bukan
	// diperiksa — transkrip yang dipakai berbeda-beda antar mesin.
	for _, start := range []float64{0, 45, 78.4, 130, 400} {
		if start > last {
			continue
		}
		t.Logf("   klip mulai %6.1f → %s", start,
			map[bool]string{true: "DIBUANG", false: "dipakai"}[start < end])
	}
}
