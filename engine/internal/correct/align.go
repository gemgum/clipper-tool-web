package correct

import (
	"strings"
	"unicode"

	"github.com/gemgum/clipper/engine/internal/types"
)

// Penyejajaran ulang timestamp setelah teks segmen dikoreksi.
//
// Ini bagian paling berisiko dari fitur koreksi. Subtitle mode karaoke & word
// menyorot kata per kata memakai timestamp asli dari whisper; begitu teksnya
// berubah, pemetaan kata→waktu harus dibangun ulang atau sorotannya meleset.
//
// Caranya: sejajarkan deret kata LAMA dengan deret kata BARU memakai jarak
// edit (Levenshtein pada level kata), lalu:
//   - kata yang cocok/diganti 1:1  → pakai timestamp kata lama apa adanya;
//   - kata sisipan                 → bagi rata celah waktu di sekitarnya;
//   - kata yang hilang             → waktunya diserap kata sebelumnya, supaya
//     sorotan tidak padam saat audionya masih berbunyi.
//
// Substitusi 1:1 adalah kasus terbanyak koreksi ASR ("rime" → "rame"), dan
// justru kasus itulah yang timing-nya terjaga sempurna.

// normalize menyeragamkan satu kata untuk perbandingan: huruf kecil, tanpa
// tanda baca di tepi. Dipakai HANYA untuk mencocokkan, tidak pernah ditulis
// kembali ke transkrip.
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.TrimFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

// op menjelaskan satu langkah penyejajaran. Indeks -1 berarti tidak ada
// pasangan di sisi itu.
type op struct {
	old int // indeks kata lama, -1 bila kata baru disisipkan
	new int // indeks kata baru, -1 bila kata lama dihapus
}

// alignWords menyejajarkan dua deret kata memakai jarak edit.
//
// Biaya substitusi 1 sama dengan hapus+sisip 2, jadi penyejajaran condong ke
// substitusi 1:1 — persis perilaku yang kita mau untuk koreksi salah dengar.
func alignWords(oldWords, newWords []string) []op {
	n, m := len(oldWords), len(newWords)

	oldNorm := make([]string, n)
	for i, w := range oldWords {
		oldNorm[i] = normalize(w)
	}
	newNorm := make([]string, m)
	for j, w := range newWords {
		newNorm[j] = normalize(w)
	}

	// dist[i][j] = biaya menyejajarkan i kata lama pertama dengan j kata baru.
	dist := make([][]int, n+1)
	for i := range dist {
		dist[i] = make([]int, m+1)
		dist[i][0] = i
	}
	for j := 0; j <= m; j++ {
		dist[0][j] = j
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			cost := 1
			if oldNorm[i-1] == newNorm[j-1] {
				cost = 0
			}
			best := dist[i-1][j-1] + cost // cocok / substitusi
			if d := dist[i-1][j] + 1; d < best {
				best = d // hapus
			}
			if ins := dist[i][j-1] + 1; ins < best {
				best = ins // sisip
			}
			dist[i][j] = best
		}
	}

	// Telusur balik. Urutannya dibalik di akhir supaya maju menurut waktu.
	var ops []op
	i, j := n, m
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0:
			cost := 1
			if oldNorm[i-1] == newNorm[j-1] {
				cost = 0
			}
			if dist[i][j] == dist[i-1][j-1]+cost {
				ops = append(ops, op{old: i - 1, new: j - 1})
				i, j = i-1, j-1
				continue
			}
			if dist[i][j] == dist[i-1][j]+1 {
				ops = append(ops, op{old: i - 1, new: -1})
				i--
				continue
			}
			ops = append(ops, op{old: -1, new: j - 1})
			j--
		case i > 0:
			ops = append(ops, op{old: i - 1, new: -1})
			i--
		default:
			ops = append(ops, op{old: -1, new: j - 1})
			j--
		}
	}
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}
	return ops
}

// contentTokens membuang token yang isinya hanya tanda baca — terutama tanda
// hubung dialog "-", yang justru MEMANG harus dibuang koreksi. Menghitungnya
// sebagai kata membuat pagar pengaman salah menuduh koreksi yang benar.
func contentTokens(tokens []string) []string {
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if normalize(t) != "" {
			out = append(out, t)
		}
	}
	return out
}

// contentEdits menghitung berapa kata isi yang tidak sama persis antara versi
// lama dan baru — jumlah substitusi, penghapusan, dan penyisipan.
//
// Ini pagar pengaman utama. Koreksi yang sah menyentuh satu-dua kata; model
// yang memparafrase, memangkas kalimat, atau memecah nama ("Londo-Irang" jadi
// "Londo-I rang") menyentuh jauh lebih banyak. Mengembalikan juga jumlah kata
// isi versi lama, sebagai dasar menghitung jatah perubahan.
//
// exempt berisi kata-kata penyusun daftar istilah pengguna (boleh nil).
// Perubahan YANG MENUJU kata di daftar itu tidak dihitung: ejaannya sudah
// disetujui pengguna, jadi itu koreksi terarah, bukan karangan. Tanpa
// pengecualian ini jatah per segmen (satu kata per enam) sering habis lebih
// dulu oleh pembenahan tanda baca, dan justru perbaikan istilah yang ditolak.
func contentEdits(oldWords, newWords []string, exempt map[string]bool) (changed, total int) {
	oldContent := contentTokens(oldWords)
	newContent := contentTokens(newWords)
	total = len(oldContent)

	matched, forgiven := 0, 0
	for _, o := range alignWords(oldContent, newContent) {
		switch {
		case o.old >= 0 && o.new >= 0:
			if normalize(oldContent[o.old]) == normalize(newContent[o.new]) {
				matched++
			} else if exempt[normalize(newContent[o.new])] {
				matched++ // substitusi menuju ejaan baku
			}
		case o.old < 0 && o.new >= 0 && exempt[normalize(newContent[o.new])]:
			forgiven++ // kata yang dulu hilang, kini melengkapi istilah
		}
	}
	// Kata lama yang tak tercocokkan + kata baru yang tak tercocokkan. Sepasang
	// substitusi dihitung sekali, sebab keduanya menempati posisi yang sama.
	dropped := len(oldContent) - matched
	added := len(newContent) - matched - forgiven
	changed = dropped
	if added > changed {
		changed = added
	}
	return changed, total
}

// minInsertedDur = durasi minimum yang diberikan ke kata sisipan bila di
// sekitarnya benar-benar tidak ada celah waktu tersisa.
const minInsertedDur = 0.06

// retime membangun ulang daftar kata bertimestamp untuk teks yang sudah
// dikoreksi, memakai timing kata lama sebagai jangkar.
//
// segStart/segEnd adalah batas segmen; dipakai sebagai jangkar tepi bila
// sisipan terjadi di awal atau akhir.
func retime(oldWords []types.Word, newText string, segStart, segEnd float64) []types.Word {
	newTokens := strings.Fields(newText)
	if len(newTokens) == 0 {
		return nil
	}
	if len(oldWords) == 0 {
		return spread(newTokens, segStart, segEnd)
	}

	oldTokens := make([]string, len(oldWords))
	for i, w := range oldWords {
		oldTokens[i] = w.Text
	}

	out := make([]types.Word, len(newTokens))
	assigned := make([]bool, len(newTokens))

	// 1. Kata yang punya pasangan mewarisi timing kata lamanya. Kata lama yang
	//    dihapus tidak diberikan ke siapa pun dulu — waktunya diserap di
	//    langkah 3.
	for _, o := range alignWords(oldTokens, newTokens) {
		if o.old >= 0 && o.new >= 0 {
			out[o.new] = types.Word{
				Start: oldWords[o.old].Start,
				End:   oldWords[o.old].End,
				Text:  newTokens[o.new],
			}
			assigned[o.new] = true
		}
	}

	// 2. Kata sisipan: cari jangkar kiri & kanan, lalu bagi rata celahnya.
	for j := 0; j < len(newTokens); j++ {
		if assigned[j] {
			continue
		}
		runEnd := j
		for runEnd < len(newTokens) && !assigned[runEnd] {
			runEnd++
		}
		// Batas kiri: akhir kata beranjak terakhir, atau awal segmen.
		left := segStart
		if j > 0 {
			left = out[j-1].End
		}
		// Batas kanan: awal kata beranjak berikutnya, atau akhir segmen.
		right := segEnd
		if runEnd < len(newTokens) {
			right = out[runEnd].Start
		}
		count := runEnd - j
		if right <= left {
			// Tidak ada celah: pinjam sedikit waktu ke depan supaya urutannya
			// tetap naik dan tiap kata tetap punya durasi.
			right = left + float64(count)*minInsertedDur
		}
		step := (right - left) / float64(count)
		for k := 0; k < count; k++ {
			out[j+k] = types.Word{
				Start: left + step*float64(k),
				End:   left + step*float64(k+1),
				Text:  newTokens[j+k],
			}
			assigned[j+k] = true
		}
		j = runEnd - 1
	}

	// 3. Kata yang hilang meninggalkan celah. Celah itu diserap kata sebelumnya
	//    supaya sorotan karaoke tidak padam selagi audionya masih berbunyi.
	for j := 0; j+1 < len(out); j++ {
		if out[j].End < out[j+1].Start {
			out[j].End = out[j+1].Start
		}
	}

	// 4. Jaga urutan tetap naik & tiap kata punya durasi positif.
	prev := segStart
	for j := range out {
		if out[j].Start < prev {
			out[j].Start = prev
		}
		if out[j].End <= out[j].Start {
			out[j].End = out[j].Start + minInsertedDur
		}
		prev = out[j].End
	}
	if last := len(out) - 1; last >= 0 && segEnd > out[last].Start && out[last].End > segEnd {
		out[last].End = segEnd
	}
	return out
}

// spread membagi rentang waktu rata untuk semua kata. Dipakai bila segmen
// aslinya memang tidak punya timestamp per kata.
func spread(tokens []string, start, end float64) []types.Word {
	dur := end - start
	if dur <= 0 {
		dur = float64(len(tokens)) * 0.35
	}
	per := dur / float64(len(tokens))
	out := make([]types.Word, len(tokens))
	for i, tok := range tokens {
		out[i] = types.Word{
			Start: start + per*float64(i),
			End:   start + per*float64(i+1),
			Text:  tok,
		}
	}
	return out
}
