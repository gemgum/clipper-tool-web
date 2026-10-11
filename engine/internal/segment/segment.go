// Package segment membangun kandidat klip dari transkrip.
package segment

import (
	"strings"

	"github.com/gemgum/clipper/engine/internal/types"
)

// gapStrong: jeda diam (detik) antar segmen yang dianggap pergantian topik —
// titik potong yang lebih enak daripada sekadar tanda titik.
const gapStrong = 1.5

// idealDuration mengembalikan durasi incaran di dalam rentang. Kandidat tidak
// dipotong sebelum mencapai durasi ini, supaya klip tidak selalu menempel di
// batas bawah (dulu semua klip jadi ~30 detik).
func idealDuration(targetMin, targetMax float64) float64 {
	if targetMax <= targetMin {
		return targetMin
	}
	return targetMin + 0.4*(targetMax-targetMin)
}

// BuildCandidates mengelompokkan segmen transkrip menjadi kandidat klip dengan
// durasi antara targetMin dan targetMax, sedapat mungkin memotong DI BATAS
// KALIMAT dan mengincar durasi ideal (tengah rentang), bukan batas bawah.
//
// Urutan aturan potong:
//  1. sudah >= targetMin DAN diikuti jeda diam panjang DAN akhir kalimat → potong
//     (pergantian topik yang jelas, walau belum mencapai ideal);
//  2. sudah >= ideal DAN akhir kalimat → potong;
//  3. melewati targetMax → mundur ke akhir kalimat terakhir, sisanya dibawa ke
//     kandidat berikutnya agar tidak terpotong di tengah ucapan.
func BuildCandidates(tr types.Transcript, targetMin, targetMax float64) []types.Candidate {
	var cands []types.Candidate
	var cur []types.TranscriptSegment
	ideal := idealDuration(targetMin, targetMax)

	dur := func(segs []types.TranscriptSegment) float64 {
		if len(segs) == 0 {
			return 0
		}
		return segs[len(segs)-1].End - segs[0].Start
	}

	for i, s := range tr.Segments {
		cur = append(cur, s)
		d := dur(cur)

		// Jeda ke segmen berikutnya (0 bila ini segmen terakhir).
		gap := 0.0
		if i+1 < len(tr.Segments) {
			gap = tr.Segments[i+1].Start - s.End
		}

		// (1) Pergantian topik: jeda panjang di akhir kalimat, durasi sudah cukup.
		if d >= targetMin && gap >= gapStrong && endsSentence(s.Text) {
			cands = append(cands, makeCandidate(cur))
			cur = nil
			continue
		}

		// (2) Sudah mencapai durasi ideal & berakhir di kalimat → potong bersih.
		if d >= ideal && endsSentence(s.Text) {
			cands = append(cands, makeCandidate(cur))
			cur = nil
			continue
		}

		// (3) Melewati batas atas → cari akhir kalimat terakhir & bawa sisa.
		if d >= targetMax {
			idx := lastSentenceEnd(cur)
			if idx >= 0 && dur(cur[:idx+1]) >= targetMin*0.6 {
				cands = append(cands, makeCandidate(cur[:idx+1]))
				rest := make([]types.TranscriptSegment, len(cur)-idx-1)
				copy(rest, cur[idx+1:])
				cur = rest
			} else {
				// Tidak ada batas kalimat — terpaksa potong di sini (jarang).
				cands = append(cands, makeCandidate(cur))
				cur = nil
			}
		}
	}
	if dur(cur) >= targetMin*0.5 {
		cands = append(cands, makeCandidate(cur))
	}
	return cands
}

// lastSentenceEnd mengembalikan indeks segmen terakhir yang mengakhiri kalimat.
func lastSentenceEnd(segs []types.TranscriptSegment) int {
	for i := len(segs) - 1; i >= 0; i-- {
		if endsSentence(segs[i].Text) {
			return i
		}
	}
	return -1
}

func makeCandidate(segs []types.TranscriptSegment) types.Candidate {
	parts := make([]string, 0, len(segs))
	for _, s := range segs {
		parts = append(parts, s.Text)
	}
	cp := make([]types.TranscriptSegment, len(segs))
	copy(cp, segs)
	return types.Candidate{
		Start: segs[0].Start,
		End:   segs[len(segs)-1].End,
		Text:  strings.Join(parts, " "),
		Segs:  cp,
	}
}

func endsSentence(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	// Buang tanda kutip/kurung penutup di ujung.
	t = strings.TrimRight(t, `"')`)
	if t == "" {
		return false
	}
	last := t[len(t)-1]
	return last == '.' || last == '?' || last == '!'
}
