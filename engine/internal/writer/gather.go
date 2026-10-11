package writer

import (
	"context"
	"fmt"
	"strings"

	"github.com/gemgum/clipper/engine/internal/news"
)

// MaxSources = batas artikel sumber, dihitung di KERANJANG (notes/38).
//
// Berlaku untuk keseluruhan, bukan per jalan masuk: pengguna boleh mencampur
// hasil pencarian, centangan dari daftar, dan alamat yang ditempel.
const MaxSources = 5

// SafeSources = batas yang berlaku bila jendela konteks model tidak cukup
// besar. Tiga, bukan lima: tahap 2 mengirim SELURUH fakta dari SEMUA sumber
// dalam satu panggilan, jadi tiap sumber tambahan menambah ±400 token prompt
// sekaligus menuntut artikel yang lebih panjang. Pada model berkonteks kecil
// keduanya bertemu di dinding yang sama dan balasannya terpotong di tengah JSON
// (18 Agustus 2026, lihat "Jendela konteks" di notes/38).
const SafeSources = 3

// MaxSourcesFor menerjemahkan jendela konteks model jadi batas jumlah sumber.
//
// ctxTokens = 0 berarti "tidak diketahui", dan itu jawaban yang benar untuk
// mesin cloud: semuanya jauh di atas 32k, dan menghukum mereka dengan batas
// model lokal 8k adalah salah ke arah yang tidak perlu.
func MaxSourcesFor(ctxTokens int) int {
	switch {
	case ctxTokens == 0 || ctxTokens >= 32768:
		return MaxSources
	case ctxTokens >= 16384:
		return 4
	default:
		return SafeSources
	}
}

// Skipped satu alamat yang tidak jadi sumber, beserta alasannya.
//
// Dilaporkan, bukan dibuang diam-diam: artikel yang hilang tanpa kabar terbaca
// sebagai "sumbernya cuma segitu" dan menyesatkan penilaian hasilnya.
type Skipped struct {
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

// Basket = keranjang sumber, hasil dari ketiga jalan masuk.
//
// Ketiganya (cari, jelajah & centang, tempel URL) bermuara ke satu daftar
// alamat, jadi engine tidak perlu tahu asal-usulnya — itu urusan GUI.
type Basket struct {
	Sources  []news.Content `json:"-"`
	Skipped  []Skipped      `json:"skipped,omitempty"`
	OffTopic []string       `json:"off_topic,omitempty"`
}

// Gather mengambil isi tiap alamat dan menyusun keranjangnya.
//
// Dedup terjadi SESUDAH alamatnya diresolusi, bukan sebelum: artikel yang sama
// gampang masuk dua kali — sekali sebagai pengalih news.google.com dari daftar,
// sekali sebagai alamat asli yang ditempel. Dua string yang sangat berbeda,
// satu artikel yang sama (notes/38).
func Gather(ctx context.Context, urls []string, browse news.Browser, cacheDir, lang string) (Basket, error) {
	var b Basket
	seenRaw := map[string]bool{}

	for _, raw := range urls {
		raw = strings.TrimSpace(raw)
		if raw == "" || seenRaw[raw] {
			continue
		}
		seenRaw[raw] = true
		if len(b.Sources) >= MaxSources {
			b.Skipped = append(b.Skipped, Skipped{raw, fmt.Sprintf("over the limit of %d source articles", MaxSources)})
			continue
		}

		content, err := news.FetchContent(ctx, raw, browse, cacheDir, lang)
		if err != nil {
			b.Skipped = append(b.Skipped, Skipped{raw, err.Error()})
			continue
		}
		if len(content.Paragraphs) == 0 {
			b.Skipped = append(b.Skipped, Skipped{raw, "no article body could be read from this page"})
			continue
		}
		if dup, ok := already(b.Sources, content.Article.URL); ok {
			b.Skipped = append(b.Skipped, Skipped{raw, "same article as " + dup})
			continue
		}
		b.Sources = append(b.Sources, content)
	}

	if len(b.Sources) == 0 {
		return b, fmt.Errorf("no usable source article: %s", reasons(b.Skipped))
	}
	b.OffTopic = offTopic(b.Sources)
	return b, nil
}

func already(sources []news.Content, url string) (string, bool) {
	for _, s := range sources {
		if s.Article.URL == url {
			return s.Article.URL, true
		}
	}
	return "", false
}

func reasons(skipped []Skipped) string {
	if len(skipped) == 0 {
		return "nothing was submitted"
	}
	var out []string
	for _, s := range skipped {
		out = append(out, s.URL+": "+s.Reason)
	}
	return strings.Join(out, "; ")
}

// offTopic menandai artikel yang judulnya tidak berbagi kata kunci dengan
// artikel lain mana pun.
//
// DITANDAI, BUKAN DITOLAK (notes/38). Tiga artikel kecelakaan digabung dengan
// dua artikel anggaran memang menghasilkan artikel kacau, tetapi hanya pemilik
// yang tahu apakah kaitannya memang ada.
func offTopic(sources []news.Content) []string {
	if len(sources) < 2 {
		return nil
	}
	// Judul SAJA terlalu sedikit bahannya. Diuji terhadap tiga artikel Antara
	// yang sama-sama tentang Paskibraka: judulnya cuma berbagi satu kata
	// ("paskibraka"), dan ketiganya dilaporkan tidak nyambung. Paragraf pertama
	// ikut supaya ambangnya bisa tetap dua kata.
	words := make([]map[string]bool, len(sources))
	for i, s := range sources {
		words[i] = map[string]bool{}
		text := s.Article.Title
		if len(s.Paragraphs) > 0 {
			text += " " + s.Paragraphs[0].Text
		}
		for _, w := range contentWords(text) {
			words[i][w] = true
		}
	}

	var out []string
	for i := range sources {
		best := 0
		for j := range sources {
			if i == j {
				continue
			}
			n := 0
			for w := range words[i] {
				if words[j][w] {
					n++
				}
			}
			if n > best {
				best = n
			}
		}
		if best < minOverlap {
			out = append(out, sources[i].Article.Title)
		}
	}
	return out
}
