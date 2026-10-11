package pipeline

import (
	"testing"

	"github.com/gemgum/clipper/engine/internal/score/llm"
	"github.com/gemgum/clipper/engine/internal/types"
)

func cands(n int) []types.Candidate {
	var out []types.Candidate
	for i := 0; i < n; i++ {
		out = append(out, types.Candidate{
			Start: float64(i) * 40, End: float64(i)*40 + 35,
			Text: "kandidat nomor sekian",
		})
	}
	return out
}

// Batas waktu klip datang dari KANDIDAT, bukan dari model. Ini inti perubahan:
// model memilih nomor, engine yang tahu detiknya.
func TestClipTimesComeFromTheCandidate(t *testing.T) {
	cs := cands(5)
	got, bad := picksToClips([]llm.Pick{{Index: 2, Score: 80, Title: "Judul"}}, cs, 0, 5)
	if bad != 0 || len(got) != 1 {
		t.Fatalf("dapat %d klip, %d dibuang", len(got), bad)
	}
	if got[0].Start != cs[2].Start || got[0].End != cs[2].End {
		t.Errorf("waktu klip %.0f-%.0f, mau %.0f-%.0f",
			got[0].Start, got[0].End, cs[2].Start, cs[2].End)
	}
	if got[0].Transcript != cs[2].Text {
		t.Error("transkrip klip tidak diambil dari kandidatnya")
	}
}

// Nomor di luar daftar dibuang, BUKAN dijepit ke tetangga terdekat: menjepit
// akan mengubah pilihan model jadi klip yang tidak pernah ia lihat.
func TestOutOfRangeIndexIsDroppedNotClamped(t *testing.T) {
	cs := cands(5)
	got, bad := picksToClips([]llm.Pick{
		{Index: -1, Score: 90}, {Index: 99, Score: 90}, {Index: 3, Score: 70},
	}, cs, 0, 5)
	if bad != 2 {
		t.Errorf("%d dibuang, mau 2", bad)
	}
	if len(got) != 1 || got[0].Start != cs[3].Start {
		t.Errorf("hanya nomor sah yang boleh lolos, dapat %+v", got)
	}
}

// Model kadang memilih kandidat yang sama dua kali. Kalau diloloskan, dua klip
// identik ikut dirender — dan pengguna membayar waktu render untuk berkas kembar.
func TestDuplicatePickIsTakenOnce(t *testing.T) {
	cs := cands(5)
	got, bad := picksToClips([]llm.Pick{
		{Index: 1, Score: 80}, {Index: 1, Score: 90},
	}, cs, 0, 5)
	if len(got) != 1 || bad != 1 {
		t.Errorf("dapat %d klip (%d dibuang), mau 1 dan 1", len(got), bad)
	}
}

// Batch kedua dan seterusnya memakai nomor GLOBAL, bukan nomor lokal batch.
// Kalau offsetnya salah, klip yang dirender bukan yang dipilih model — dan
// tidak ada yang menandai kekeliruan itu.
func TestBatchOffsetMapsToTheRightCandidate(t *testing.T) {
	cs := cands(30)
	// Batch kedua: kandidat 12..23, model memilih nomor 15.
	got, bad := picksToClips([]llm.Pick{{Index: 15, Score: 80}}, cs, 12, 24)
	if bad != 0 || len(got) != 1 {
		t.Fatalf("dapat %d klip, %d dibuang", len(got), bad)
	}
	if got[0].Start != cs[15].Start {
		t.Errorf("nomor 15 memetakan ke %.0f, mau %.0f", got[0].Start, cs[15].Start)
	}
	// Dan nomor dari batch lain ditolak walau sah secara global.
	if _, bad := picksToClips([]llm.Pick{{Index: 3, Score: 80}}, cs, 12, 24); bad != 1 {
		t.Error("nomor dari batch lain seharusnya ditolak")
	}
}

// Model lokal kerap mengisi skor tapi membiarkan rinciannya kosong. Nol berarti
// "dinilai buruk", padahal yang terjadi "tidak dinilai".
func TestEmptyReasonsFallBackToTheOverallScore(t *testing.T) {
	got, _ := picksToClips([]llm.Pick{{Index: 0, Score: 75}}, cands(2), 0, 2)
	if got[0].Reasons.Hook != 75 || got[0].Reasons.Standalone != 75 {
		t.Errorf("rincian %+v, mau diratakan dari skor 75", got[0].Reasons)
	}
}
