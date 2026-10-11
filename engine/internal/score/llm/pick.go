package llm

import (
	"fmt"
	"strings"

	"github.com/gemgum/clipper/engine/internal/types"
)

// Memilih dari kandidat bernomor, bukan mengarang timestamp.
//
// Sebelumnya model diminta menulis sendiri "start" dan "end". Diukur pada
// qwen2.5, tiga kali jalan pada permintaan yang sama mengembalikan antara lain
// "468-43" dan "367-313" — akhir lebih kecil dari awal — dan momen 8 detik
// padahal yang diminta 30-60 detik. Sebagian besar balasan itu dibuang
// validateMoments, dan yang tersisa itulah yang jadi klip: penyebab utama klip
// terasa berubah-ubah tiap kali dijalankan.
//
// Batas waktu sekarang dibangun engine (segment.BuildCandidates) dari batas
// kalimat, jadi ia sah menurut konstruksinya. Model tinggal MEMILIH NOMOR dan
// menilai — tugas yang jauh lebih ringan dan tidak bisa menghasilkan rentang
// mustahil.
//
// Pola yang sama sudah terbukti di tab kartu berita: LLM hanya memilih nomor
// paragraf, teksnya diambil engine, dan kartunya tidak pernah mengarang
// (notes/13-kartu-berita.md).

// Pick = satu kandidat yang dipilih model, beserta penilaiannya.
//
// Tidak ada Start/End di sini, dan itu disengaja: angka waktunya milik engine.
type Pick struct {
	Index    int           `json:"index"`
	Score    float64       `json:"score"`
	Reasons  MomentReasons `json:"reasons"`
	Title    string        `json:"title"`
	Hashtags []string      `json:"hashtags"`
}

// MaxCandidatesPerRequest membatasi berapa kandidat dikirim sekali jalan.
//
// Bukan soal kemampuan model membaca, melainkan jendela konteksnya: num_ctx
// Ollama 8192 token, sementara satu kandidat 30-60 detik berisi 100-200 kata.
// Di atas belasan kandidat, awal daftarnya terdorong keluar jendela dan model
// menilai sesuatu yang sudah tidak dilihatnya.
const MaxCandidatesPerRequest = 12

// PickSystemPrompt menyusun instruksi pemilihan. Instruksinya bahasa Inggris,
// judul & tagar mengikuti bahasa isi video.
func PickSystemPrompt(maxClips int, contentLang string) string {
	lang := LanguageName(contentLang)
	var b strings.Builder
	b.WriteString(`You curate viral short-form video clips (TikTok, Reels, Shorts).

You are given a numbered list of candidate clips. Each candidate is already a
complete, correctly timed extract — the timing is not your concern and you must
never mention or invent any timestamp.

YOUR ONLY JOB: choose the best candidates by their number, and score them.

HARD RULES:
- 'index' MUST be a number that appears in the list. Never invent one.
- Choose each candidate at most once.
`)
	fmt.Fprintf(&b, "- Choose at most %d candidates. Choose FEWER if fewer are genuinely good — a weak clip costs more than a missing one.\n", maxClips)

	b.WriteString(`
WHAT MAKES A CANDIDATE WORTH CHOOSING:
- It is ONE thought: it opens a subject, develops it, and closes it.
- You can say what it is about in a single sentence, using only what it says.
- It stands on its own without the rest of the video.

WHAT TO REJECT, however lively it sounds:
- Fragments of the teaser most videos open with: short pieces taken from all
  over the video, changing subject every few seconds. It sounds exciting because
  it is a highlight reel, but on its own it says nothing.
- Anything that jumps between unrelated subjects.
- Anything that only makes sense if you already watched what came before it.

SCORING (0-100 per dimension):
- hook: do the first seconds stop someone from scrolling?
- emotion: emotional charge (surprise, humour, anger, tenderness, inspiration)
- clarity: understandable without the rest of the video
- shareability: worth sharing, or likely to draw comments
- standalone: complete as a single story
- score: an overall judgement, not a raw average of the five

`)
	fmt.Fprintf(&b, "- title: a catchy title in %s, at most 60 characters, no quotation marks.\n", lang)
	fmt.Fprintf(&b, "- hashtags: 3-5 relevant hashtags in %s, each starting with #.\n\n", lang)

	b.WriteString(`Reply with VALID JSON ONLY, no explanation, in exactly this shape:
{"picks":[{"index":<number>,"score":<0-100>,"reasons":{"hook":<0-100>,"emotion":<0-100>,"clarity":<0-100>,"shareability":<0-100>,"standalone":<0-100>},"title":"<title>","hashtags":["#..","#.."]}]}`)
	return b.String()
}

// PickUserPrompt merangkai daftar kandidat bernomor.
//
// Durasinya ikut ditulis sebagai keterangan, bukan sebagai angka yang boleh
// dipakai model: ia membantu menilai (klip 55 detik terasa berbeda dari 30
// detik) tanpa memberi peluang mengarang batas waktu.
func PickUserPrompt(cands []types.Candidate, offset int) string {
	var b strings.Builder
	b.WriteString("Candidates:\n")
	for i, c := range cands {
		fmt.Fprintf(&b, "\n[%d] (%.0f seconds)\n%s\n", offset+i, c.Duration(), strings.TrimSpace(c.Text))
	}
	return b.String()
}

// PickSchema mengunci bentuk balasan. Dipakai Ollama lewat parameter "format".
//
// Panjang larik TIDAK dikunci ke satu angka — berbeda dari koreksi transkrip,
// di sini memilih lebih sedikit memang jawaban yang sah. Yang dikunci bentuk
// tiap entrinya, supaya "index" selalu ada dan selalu angka.
func PickSchema(maxClips int) map[string]any {
	if maxClips < 1 {
		maxClips = 1
	}
	num := map[string]any{"type": "number"}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"picks": map[string]any{
				"type":     "array",
				"minItems": 1,
				"maxItems": maxClips,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"index": map[string]any{"type": "integer"},
						"score": num,
						"reasons": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"hook": num, "emotion": num, "clarity": num,
								"shareability": num, "standalone": num,
							},
						},
						"title":    map[string]any{"type": "string"},
						"hashtags": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					},
					"required": []string{"index", "score"},
				},
			},
		},
		"required": []string{"picks"},
	}
}

// PickResponse bentuk balasan yang diurai.
type PickResponse struct {
	Picks []Pick `json:"picks"`
}

// ToReasons membulatkan penilaian model jadi bentuk yang dipakai klip.
//
// Model lokal kerap membiarkan rincian kosong sementara skor keseluruhannya
// terisi. Meratakan skor ke lima dimensi lebih jujur daripada menampilkan nol —
// nol berarti "dinilai buruk", padahal yang terjadi "tidak dinilai".
func ToReasons(r MomentReasons, overall float64) types.Reasons {
	out := types.Reasons{
		Hook:         round(r.Hook),
		Emotion:      round(r.Emotion),
		Clarity:      round(r.Clarity),
		Shareability: round(r.Shareability),
		Standalone:   round(r.Standalone),
	}
	if out == (types.Reasons{}) && overall > 0 {
		s := round(overall)
		out = types.Reasons{Hook: s, Emotion: s, Clarity: s, Shareability: s, Standalone: s}
	}
	return out
}

func round(v float64) int { return int(v + 0.5) }
