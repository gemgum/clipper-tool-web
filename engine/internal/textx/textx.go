// Package textx = pembantu teks yang dipakai banyak paket (hasil peninjauan
// redundansi 25 Oktober 2026): dulu pengambil JSON dari balasan LLM ada empat
// salinan dan pemotong teks lima, sebagian memotong di tengah huruf UTF-8.
package textx

import "strings"

// ExtractJSON mengambil nilai JSON dari balasan yang kadang dibungkus prosa
// atau pagar kode.
//
// Larik ikut dikenali, bukan cuma objek: model yang bentuk balasannya tidak
// dipaksakan server kerap membalas "[...]" — dan mengambil dari "{" pertama
// sampai "}" terakhir pada larik justru MEMOTONG isinya jadi JSON rusak.
func ExtractJSON(s string) string {
	s = strings.TrimSpace(s)
	obj := span(s, "{", "}")
	arr := span(s, "[", "]")
	// Yang dipakai adalah yang MULAI lebih dulu: objek berisi larik akan cocok
	// keduanya, dan pembungkus terluar yang benar.
	if obj != "" && (arr == "" || strings.Index(s, "{") < strings.Index(s, "[")) {
		return obj
	}
	if arr != "" {
		return arr
	}
	return s
}

func span(s, open, close string) string {
	i := strings.Index(s, open)
	j := strings.LastIndex(s, close)
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return ""
}

// Snip = potongan pendek untuk pesan galat: baris baru jadi spasi, paling
// banyak n HURUF (bukan byte — tidak pernah memotong di tengah huruf UTF-8),
// "…" bila dipotong.
func Snip(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
