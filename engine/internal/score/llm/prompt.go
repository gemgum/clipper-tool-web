package llm

import "strings"

// languageNames memetakan kode bahasa transkrip ke nama yang dimengerti model.
// Dipakai untuk memberi tahu model dalam bahasa apa judul & tagar harus ditulis:
// judul mengikuti bahasa VIDEO, bukan bahasa antarmuka.
var languageNames = map[string]string{
	"id": "Indonesian",
	"en": "English",
	"ms": "Malay",
	"jv": "Javanese",
	"su": "Sundanese",
}

// LanguageName mengembalikan nama bahasa yang bisa dipakai di dalam prompt.
// Kode yang tidak dikenal dikembalikan apa adanya — model umumnya masih paham.
func LanguageName(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return "Indonesian"
	}
	if name, ok := languageNames[code]; ok {
		return name
	}
	return code
}
