package news

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Asal sebuah nilai peringkat.
const (
	SourceLLM       = "llm"
	SourceHeuristic = "heuristic"
)

// Ranking satu paragraf beserta nilai hook-nya.
type Ranking struct {
	Index  int     `json:"index"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
	Text   string  `json:"text"`   // diisi engine dari indeks — TIDAK dari balasan LLM
	Source string  `json:"source"` // llm | heuristic — supaya terlihat di GUI
}

// Selection hasil analisis satu artikel.
type Selection struct {
	Card     int       `json:"card"`    // indeks paragraf usulan untuk kartu
	Caption  int       `json:"caption"` // indeks paragraf usulan untuk caption
	Rankings []Ranking `json:"rankings"`
	Hashtags []string  `json:"hashtags"`
	Engine   string    `json:"engine"`
	Note     string    `json:"note"` // penjelasan bila LLM tidak melengkapi
}

// Completer adalah satu panggilan ke LLM. Dibuat berupa fungsi, bukan
// antarmuka, supaya paket news tidak perlu mengenal paket llm/ollama — lapisan
// api yang merangkainya sesuai mesin yang dipilih pengguna.
type Completer func(ctx context.Context, system, user string) (string, error)

// reply = bentuk JSON yang diminta dari LLM.
//
// Perhatikan: tidak ada satu pun field teks bebas untuk isi kartu maupun
// caption — hanya NOMOR paragraf. LLM memilih, engine yang mengambil kalimat
// aslinya. Reason boleh berupa teks karena itu hanya penjelasan untuk manusia,
// tidak pernah ikut terbit.
type reply struct {
	Card     int `json:"card"`
	Caption  int `json:"caption"`
	Rankings []struct {
		Index  int     `json:"index"`
		Score  float64 `json:"score"`
		Reason string  `json:"reason"`
	} `json:"rankings"`
	Keywords []string `json:"keywords"`
}

// systemSelectPrompt = instruksi pemilihan paragraf.
//
// Instruksinya bahasa Inggris, tetapi "reason" diminta dalam bahasa Inggris juga
// karena teks itu hanya penjelasan untuk pengguna di GUI — bukan isi kartu.
// Isi kartu & caption TIDAK PERNAH ditulis model: ia hanya menunjuk nomor.
const systemSelectPrompt = `You help pick the most interesting parts of a news article for social media content.

THE MOST IMPORTANT RULE: you do NOT write, you do NOT summarise, and you do NOT paraphrase.
You only SELECT THE NUMBER of a paragraph that was given to you. Inventing new sentences is forbidden.

Your tasks:
1. "card"     — the number of the best paragraph to place on the image. Pick one that stands
                on its own (understandable without reading other paragraphs), is compact, and
                carries the core of the news. Avoid paragraphs that open with a back-reference
                such as "He", "That matter", "Meanwhile".
2. "caption"  — the number of the paragraph that most arouses curiosity, for the post caption.
                It may be the same as "card" if that really is the best paragraph.
3. "rankings" — score EVERY paragraph on hook strength, 0-10. Include a short "reason" in
                English, at most 12 words.
4. "keywords" — 5 to 8 IMPORTANT words or phrases that literally appear in the article
                (names of people, institutions, places, events). Copy them exactly as written.
                Do not add words that are absent from the article.

What gives a paragraph a hook: a surprising number, conflict, a blunt direct quote, a
consequence that touches many people, or an unexpected fact.

Reply with JSON ONLY.`

// SelectionSchema = JSON Schema untuk parameter "format" Ollama, supaya model
// lokal tidak perlu diandalkan kepatuhannya pada instruksi bentuk balasan.
func SelectionSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"card":    map[string]any{"type": "integer"},
			"caption": map[string]any{"type": "integer"},
			"rankings": map[string]any{
				"type": "array",
				// minItems memaksa model lokal benar-benar mengisi peringkat.
				// Tanpa ini qwen2.5 kerap membalas "rankings": [] — bentuk JSON
				// sah, isinya kosong. Ini pencegahan di hulu; build() tetap
				// menambal sisanya kalau model masih membandel.
				"minItems": 1,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"index":  map[string]any{"type": "integer"},
						"score":  map[string]any{"type": "number"},
						"reason": map[string]any{"type": "string"},
					},
					"required": []string{"index", "score"},
				},
			},
			"keywords": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
		"required": []string{"card", "caption", "rankings", "keywords"},
	}
}

// maxPromptWords membatasi panjang artikel yang dikirim ke model. 1200 kata
// masih muat di konteks 8k milik model lokal, dan berita Indonesia jarang
// melampauinya.
const maxPromptWords = 1200

// SelectParagraphs meminta LLM menilai paragraf mana yang paling layak jadi
// kartu & caption.
//
// Kegagalan dikembalikan apa adanya — tidak ada perpindahan diam-diam ke mesin
// lain maupun ke tebakan heuristik (lihat notes/12).
func SelectParagraphs(ctx context.Context, content Content, complete Completer, engineName, cacheDir string) (Selection, error) {
	if len(content.Paragraphs) == 0 {
		return Selection{}, fmt.Errorf("the article has no paragraphs to score")
	}
	key := cacheKey(content.Article.URL, engineName)
	if s, ok := loadCache(cacheDir, key); ok {
		return s, nil
	}

	user := fmt.Sprintf("Title: %s\n\nParagraphs:\n\n%s", content.Article.Title, content.Numbered(maxPromptWords))
	raw, err := complete(ctx, systemSelectPrompt, user)
	if err != nil {
		return Selection{}, err
	}

	var r reply
	if err := json.Unmarshal([]byte(extractJSON(raw)), &r); err != nil {
		return Selection{}, fmt.Errorf("%s returned JSON that could not be read: %w; reply: %s",
			engineName, err, truncate(strings.ReplaceAll(raw, "\n", " "), 300))
	}

	result := build(content, r, engineName)
	saveCache(cacheDir, key, result)
	return result, nil
}

// build mengubah balasan LLM jadi hasil yang aman: setiap nomor diperiksa
// jangkauannya, dan teksnya diambil dari artikel — bukan dari balasan.
//
// Tidak pernah mengembalikan galat. Model lokal sering hanya mengisi sebagian
// (mis. memberi "card" & "caption" tapi "rankings": []), dan menolak hasil
// seperti itu berarti membuang jawaban yang sebenarnya sudah bisa dipakai.
// Paragraf yang tidak dinilai model dilengkapi penilaian heuristik, dan
// ditandai supaya pengguna tahu mana yang berasal dari mana.
func build(content Content, r reply, engineName string) Selection {
	if len(content.Paragraphs) == 0 {
		return Selection{Engine: engineName}
	}
	var rankings []Ranking
	scored := map[int]bool{}
	for _, item := range r.Rankings {
		text, ok := content.TextAt(item.Index)
		if !ok || scored[item.Index] {
			continue // nomor tidak ada atau ganda — abaikan, jangan tebak
		}
		scored[item.Index] = true
		rankings = append(rankings, Ranking{
			Index: item.Index, Score: item.Score, Reason: item.Reason,
			Text: text, Source: SourceLLM,
		})
	}

	// Lengkapi paragraf yang terlewat. Selain menambal kemalasan model, ini
	// juga membuat SELURUH paragraf bisa dipilih pengguna di GUI — sebelumnya
	// paragraf yang tak dinilai model tidak muncul sama sekali.
	var missed int
	for _, p := range content.Paragraphs {
		if scored[p.Index] {
			continue
		}
		missed++
		score, reason := hookScore(p, len(content.Paragraphs))
		rankings = append(rankings, Ranking{
			Index: p.Index, Score: score, Reason: reason,
			Text: p.Text, Source: SourceHeuristic,
		})
	}
	SortRankings(rankings)

	note := ""
	switch {
	case len(scored) == 0:
		note = fmt.Sprintf("%s gave no rankings at all: the order below was produced automatically by the engine.", engineName)
	case missed > 0:
		note = fmt.Sprintf("%s scored %d of %d paragraphs; the rest were scored automatically by the engine.",
			engineName, len(scored), len(content.Paragraphs))
	}

	// Nomor kartu/caption yang di luar jangkauan diganti peringkat teratas —
	// ini bukan menebak isi, hanya memilih di antara paragraf artikel sendiri.
	card := r.Card
	if _, ok := content.TextAt(card); !ok {
		card = rankings[0].Index
	}
	caption := r.Caption
	if _, ok := content.TextAt(caption); !ok {
		caption = rankings[0].Index
	}
	// Model lokal cenderung menjawab paragraf 0 untuk dua-duanya, sehingga teks
	// kartu dan captionnya kembar dan postingannya jadi mubazir. Bila kembar,
	// caption digeser ke paragraf berperingkat berikutnya — masih dipilih dari
	// artikel yang sama, dan pengguna tetap bisa menggantinya sekali klik.
	if caption == card && len(rankings) > 1 {
		for _, item := range rankings {
			if item.Index != card {
				caption = item.Index
				break
			}
		}
	}

	return Selection{
		Card:     card,
		Caption:  caption,
		Rankings: rankings,
		Hashtags: content.Hashtags(r.Keywords, 8),
		Engine:   engineName,
		Note:     note,
	}
}

// extractJSON mengambil blok objek JSON dari balasan. Model lokal kadang
// membungkus JSON dengan pagar kode atau kalimat pengantar.
func extractJSON(s string) string {
	i := strings.IndexByte(s, '{')
	j := strings.LastIndexByte(s, '}')
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}

// --- cache ---

// cacheKey = sidik jari URL + mesin. Artikel yang sama dinilai ulang hanya bila
// mesinnya berganti, sehingga bereksperimen dengan gaya kartu tidak memanggil
// LLM berulang kali.
func cacheKey(url, engine string) string {
	h := sha256.Sum256([]byte(url + "\x00" + engine))
	return hex.EncodeToString(h[:16])
}

func cachePath(dir, key string) string {
	return filepath.Join(dir, "cache", "articles", key+".json")
}

func loadCache(dir, key string) (Selection, bool) {
	if dir == "" {
		return Selection{}, false
	}
	raw, err := os.ReadFile(cachePath(dir, key))
	if err != nil {
		return Selection{}, false
	}
	var s Selection
	if err := json.Unmarshal(raw, &s); err != nil || len(s.Rankings) == 0 {
		return Selection{}, false
	}
	return s, true
}

func saveCache(dir, key string, s Selection) {
	if dir == "" {
		return
	}
	path := cachePath(dir, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return // cache itu percepatan, bukan keharusan — gagal simpan tidak fatal
	}
	if raw, err := json.Marshal(s); err == nil {
		_ = os.WriteFile(path, raw, 0o644)
	}
}
