// Package heuristic memberi skor kandidat klip berbasis aturan (bahasa Indonesia).
package heuristic

import (
	"math"
	"strings"

	"github.com/gemgum/clipper/engine/internal/types"
)

// Kata bermuatan emosi (Indonesia). Daftar awal, bisa diperluas.
var emotionWords = []string{
	"cinta", "benci", "marah", "takut", "kaget", "sedih", "senang", "bahagia",
	"gila", "parah", "banget", "luar biasa", "menakjubkan", "menyebalkan",
	"bangga", "kecewa", "nyesel", "menyesal", "heran", "syok", "ngeri",
	"keren", "hebat", "dahsyat", "mengerikan", "menangis", "tertawa",
}

// Kata pemicu rasa penasaran / hook.
var triggerWords = []string{
	"rahasia", "ternyata", "jangan", "bahaya", "penting", "wajib", "trik",
	"cara", "kenapa", "mengapa", "sebenarnya", "faktanya", "percaya",
	"jarang", "tidak akan", "pertama kali", "terbukti", "hasilnya",
}

// Score menilai satu kandidat. rmsClip adalah deret RMS audio khusus jendela
// klip (boleh nil bila worker tidak tersedia).
func Score(c types.Candidate, targetMin, targetMax float64, rmsClip []float64) (int, types.Reasons) {
	textLower := strings.ToLower(c.Text)
	words := strings.Fields(textLower)
	wordCount := len(words)

	// --- Hook: sinyal di ~kalimat pertama (30 kata awal) ---
	head := textLower
	if len(words) > 30 {
		head = strings.Join(words[:30], " ")
	}
	hook := 30
	if strings.Contains(head, "?") || containsAny(head, []string{"kenapa", "mengapa", "gimana", "bagaimana"}) {
		hook += 30
	}
	if containsAny(head, triggerWords) {
		hook += 30
	}
	if hasNumber(head) {
		hook += 10
	}
	hook = clamp(hook)

	// --- Emosi: kepadatan kata emosi ---
	emoHits := countHits(textLower, emotionWords)
	emotion := 20
	if wordCount > 0 {
		density := float64(emoHits) / float64(wordCount)
		emotion = 20 + int(density*400) // ~5% emosi → +20
	}
	if emoHits >= 3 {
		emotion += 15
	}
	emotion = clamp(emotion)

	// --- Clarity: kepadatan bicara wajar (bukan terlalu sepi/padat) ---
	dur := c.Duration()
	clarity := 50
	if dur > 0 {
		wpm := float64(wordCount) / dur * 60.0
		// 90-190 wpm dianggap enak didengar.
		switch {
		case wpm >= 90 && wpm <= 190:
			clarity = 85
		case wpm >= 60 && wpm < 90, wpm > 190 && wpm <= 230:
			clarity = 65
		default:
			clarity = 40
		}
	}

	// --- Shareability: kombinasi trigger + panjang ideal ---
	shareability := 40
	if containsAny(textLower, triggerWords) {
		shareability += 25
	}
	if dur >= targetMin && dur <= targetMax {
		shareability += 25
	}
	shareability = clamp(shareability)

	// --- Standalone: berakhir seperti pikiran utuh ---
	standalone := 50
	trimmed := strings.TrimSpace(c.Text)
	if trimmed != "" {
		last := trimmed[len(trimmed)-1]
		if last == '.' || last == '?' || last == '!' {
			standalone += 30
		}
	}
	if wordCount >= 15 {
		standalone += 10
	}
	standalone = clamp(standalone)

	// --- Energi audio (opsional dari worker C++) → dorong emosi/hook ---
	energyBonus := 0
	if len(rmsClip) > 3 {
		energyBonus = energyVariationBonus(rmsClip) // 0..15
		emotion = clamp(emotion + energyBonus/2)
		hook = clamp(hook + energyBonus/2)
	}

	// Bobot akhir.
	total := 0.30*float64(hook) +
		0.25*float64(emotion) +
		0.15*float64(clarity) +
		0.15*float64(shareability) +
		0.15*float64(standalone)

	return clamp(int(math.Round(total))), types.Reasons{
		Hook:         hook,
		Emotion:      emotion,
		Clarity:      clarity,
		Shareability: shareability,
		Standalone:   standalone,
	}
}

// energyVariationBonus memberi poin bila ada variasi energi (momen "rame").
func energyVariationBonus(rms []float64) int {
	var mean, max float64
	for _, v := range rms {
		mean += v
		if v > max {
			max = v
		}
	}
	mean /= float64(len(rms))
	if mean <= 0 {
		return 0
	}
	ratio := max / mean // makin tinggi = makin dinamis
	b := int((ratio - 1.0) * 10)
	if b < 0 {
		b = 0
	}
	if b > 15 {
		b = 15
	}
	return b
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func countHits(s string, subs []string) int {
	n := 0
	for _, sub := range subs {
		n += strings.Count(s, sub)
	}
	return n
}

func hasNumber(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func clamp(v int) int { return min(100, max(0, v)) }
