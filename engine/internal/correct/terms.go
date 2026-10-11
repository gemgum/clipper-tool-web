package correct

import "strings"

// Batas daftar istilah. Prompt yang membengkak menggeser perhatian model dari
// transkrip ke daftarnya, dan daftar sepanjang itu hampir pasti salah pakai:
// ini untuk nama & istilah khas satu video, bukan kamus.
const (
	maxTerms   = 40
	maxTermLen = 60
)

// ParseTerms memecah masukan pengguna menjadi daftar istilah.
//
// Koma DAN baris baru sama-sama diterima sebagai pemisah, supaya daftar bisa
// ditempel apa adanya dari catatan mana pun. Ejaan pertama yang muncul menang
// bila ada duplikat, sebab itulah yang diketik pengguna lebih dulu.
func ParseTerms(s string) []string {
	raw := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.Join(strings.Fields(t), " ") // rapikan spasi berlebih
		if t == "" || len([]rune(t)) > maxTermLen {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
		if len(out) == maxTerms {
			break
		}
	}
	return out
}

// termWords menguraikan daftar istilah menjadi himpunan kata ternormalisasi,
// untuk dipakai pagar pengaman di acceptable(). Dipecah per kata karena model
// membenahi satu kata pada satu waktu ("Irang" → "Ireng"), bukan menukar
// seluruh frasa sekaligus.
func termWords(terms []string) map[string]bool {
	if len(terms) == 0 {
		return nil
	}
	set := make(map[string]bool)
	for _, t := range terms {
		for _, w := range strings.Fields(t) {
			if n := normalize(w); n != "" {
				set[n] = true
			}
		}
	}
	return set
}

// CacheVersion menggabungkan versi prompt dengan daftar istilah yang dipakai.
//
// Daftar istilah mengubah keluaran koreksi, jadi ia WAJIB ikut jadi bahan kunci
// cache. Tanpa ini, menambah satu istilah lalu menjalankan ulang video yang sama
// akan memungut hasil koreksi lama dan perbaikannya seolah-olah tidak berefek.
func CacheVersion(terms []string) string {
	if len(terms) == 0 {
		return PromptVersion
	}
	return PromptVersion + "|terms:" + strings.ToLower(strings.Join(terms, ","))
}
