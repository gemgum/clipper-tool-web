package pipeline

import (
	"fmt"
	"strings"

	"github.com/gemgum/clipper/engine/internal/types"
)

const (
	// loopShareMax: porsi terbesar segmen berteks sama yang masih dianggap wajar.
	// Percakapan nyata memang mengulang ("Ya.", "Betul.") tapi tidak pernah
	// sampai separuh transkrip.
	loopShareMax = 0.5
	// loopRunMax: panjang deretan segmen identik BERTURUT-TURUT yang masih
	// wajar. Menangkap loop yang cuma mengenai sebagian video, yang porsi
	// keseluruhannya belum melewati loopShareMax.
	loopRunMax = 30
)

// normalizeSeg menyeragamkan teks segmen untuk pembandingan.
func normalizeSeg(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// detectRepetitionLoop mencari tanda whisper terjebak mengulang satu kalimat.
//
// Model besar (large-v3 paling rawan) bisa tergelincir di satu titik lalu
// mengulang kalimat yang sama sampai audio habis. Hasilnya transkrip yang
// bentuknya sah — timestamp rapi, JSON valid — tapi isinya sampah, jadi tidak
// ada satu pun tahap berikutnya yang curiga: koreksi memolesnya, segmentasi
// memotongnya, render membakarnya jadi subtitle.
//
// Karena itu dihentikan di sini, sejalan dengan kebijakan tanpa-fallback:
// lebih baik job gagal dengan sebab yang jelas daripada mengantar klip sampah.
func detectRepetitionLoop(tr types.Transcript) error {
	counts := make(map[string]int, len(tr.Segments))
	total, bestRun, run := 0, 0, 0
	prev, runText := "", ""
	for _, s := range tr.Segments {
		t := normalizeSeg(s.Text)
		if t == "" {
			continue
		}
		total++
		counts[t]++
		if t == prev {
			run++
		} else {
			run, prev = 1, t
		}
		if run > bestRun {
			bestRun, runText = run, t
		}
	}
	if total == 0 {
		return nil
	}

	// Tie-break menurut abjad supaya pesan errornya tidak berubah-ubah antar
	// jalan (urutan iterasi map di Go acak).
	topText, topCount := "", 0
	for t, n := range counts {
		if n > topCount || (n == topCount && t < topText) {
			topText, topCount = t, n
		}
	}

	if share := float64(topCount) / float64(total); share > loopShareMax {
		return loopErr(topText, topCount, total)
	}
	if bestRun >= loopRunMax {
		return loopErr(runText, bestRun, total)
	}
	return nil
}

// loopErr menyusun pesan yang menyebut kalimat pengulangnya, supaya pengguna
// bisa langsung mengenali bagian audio mana yang bermasalah.
func loopErr(text string, n, total int) error {
	if r := []rune(text); len(r) > 60 {
		text = string(r[:60]) + "…"
	}
	return fmt.Errorf(
		"whisper got stuck repeating %q (%d of %d segments): the transcript is "+
			"unusable, so the job was stopped instead of producing clips from it. "+
			"Try a smaller model (-model small) or re-run",
		text, n, total)
}
