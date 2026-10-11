package news

import (
	"regexp"
	"strings"
)

// Penilaian hook tanpa LLM.
//
// Gunanya dua: (1) melengkapi paragraf yang tidak ikut dinilai model — model
// lokal kerap membalas "rankings": [] karena menilai selusin paragraf itu
// tugas panjang yang gampang dilewatkan; (2) menjadi jaring supaya fitur ini
// tidak pernah gagal total hanya karena model malas.
//
// Ini BUKAN pengganti mesin skor yang dipilih pengguna: yang dinilai di sini
// cuma urutan tampilan paragraf, sedangkan isinya tetap verbatim dari artikel.
// Setiap paragraf yang skornya berasal dari sini ditandai Source="heuristic"
// agar terlihat di GUI — bukan penggantian diam-diam (notes/12).

var (
	reDigit     = regexp.MustCompile(`\d`)
	reBigNumber = regexp.MustCompile(`\d[\d.,]{2,}|\d+\s*(persen|%|juta|miliar|triliun|ribu)`)
	reQuote     = regexp.MustCompile(`["“”']`)
)

// backReferencePrefixes = pembuka yang menandakan paragraf ini menyambung
// paragraf sebelumnya, jadi tidak dimengerti bila berdiri sendiri di sebuah
// kartu. Daftarnya bahasa Indonesia karena artikel yang dinilai berbahasa
// Indonesia.
var backReferencePrefixes = []string{
	"ia ", "dia ", "beliau ", "hal itu", "hal tersebut", "sementara itu",
	"selain itu", "menurutnya", "ujarnya", "katanya", "lebih lanjut",
	"adapun ", "sedangkan ", "kemudian ", "selanjutnya", "pihaknya",
}

// strongWords = penanda kabar yang biasanya menarik perhatian pembaca Indonesia.
var strongWords = []string{
	"tewas", "meninggal", "korban", "korupsi", "ditangkap", "tersangka",
	"darurat", "bencana", "gagal", "rugi", "anjlok", "melonjak", "rekor",
	"tertinggi", "terbesar", "pertama kali", "terungkap", "diduga", "protes",
	"tolak", "kecam", "viral", "dilarang", "denda", "sanksi",
}

// hookScore menilai satu paragraf 0..10 beserta alasan singkat.
func hookScore(p Paragraph, total int) (float64, string) {
	text := p.Text
	low := strings.ToLower(text)
	words := len(strings.Fields(text))

	score := 4.0
	var reasons []string

	if reBigNumber.MatchString(low) {
		score += 2
		reasons = append(reasons, "contains a number")
	} else if reDigit.MatchString(text) {
		score += 0.8
	}
	if reQuote.MatchString(text) {
		score += 1.8
		reasons = append(reasons, "direct quote")
	}
	for _, w := range strongWords {
		if strings.Contains(low, w) {
			score += 1.2
			reasons = append(reasons, "strongly charged word")
			break
		}
	}

	// Paragraf pembuka berita Indonesia hampir selalu memuat intinya
	// (piramida terbalik), jadi diberi keunggulan kecil.
	if p.Index == 0 {
		score += 1.5
		reasons = append(reasons, "opening paragraph")
	} else if total > 0 && p.Index < total/3 {
		score += 0.5
	}

	switch {
	case words >= 15 && words <= 45:
		score += 1
	case words > 70:
		score -= 1.5
		reasons = append(reasons, "too long for a card")
	case words < 15:
		score -= 1
		reasons = append(reasons, "too short")
	}

	for _, prefix := range backReferencePrefixes {
		if strings.HasPrefix(low, prefix) {
			score -= 1.8
			reasons = append(reasons, "continues the previous paragraph")
			break
		}
	}

	if score < 0 {
		score = 0
	}
	if score > 10 {
		score = 10
	}
	reason := "scored automatically"
	if len(reasons) > 0 {
		reason = strings.Join(reasons, ", ")
	}
	return score, reason
}
