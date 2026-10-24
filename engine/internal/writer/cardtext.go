package writer

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gemgum/clipper/engine/internal/news"
)

// Teks kartu berita yang DITULIS AI: ringkasan satu kalimat atau caption
// postingan (DESIGN-NEWSCARD §5.2, keputusan pemilik 9 Oktober 2026 yang
// menggantikan aturan "hanya memilih paragraf" untuk dua aksi ini, notes/13).
//
// Penjaganya sama dengan pembuat berita: pagar fakta CheckText (angka,
// kutipan, nama diri harus ada di artikel). Pelanggaran TIDAK dibuang diam-
// diam, ia dikembalikan supaya GUI menandainya untuk diperiksa manusia.

// Jenis teks.
const (
	KindSummary = "summary"
	KindCaption = "caption"
	// KindStory = isi halaman lanjutan carousel: berita diceritakan ulang
	// dengan rinci, paragraf dipisah baris kosong (keputusan pemilik 25
	// Oktober 2026 — menggantikan paragraf asli artikel bila diisi).
	KindStory = "story"
)

// SummaryMax = batas keras ringkasan kartu. Di atas ini huruf kartu mengecil
// sampai sulit dibaca (DESIGN-NEWSCARD §5.2, penghitung karakter).
const SummaryMax = 180

// CardText = balasan untuk GUI.
type CardText struct {
	Text       string      `json:"text"`
	Hashtags   []string    `json:"hashtags"`
	Violations []Violation `json:"violations"`
}

// WriteCardText meminta satu ringkasan atau caption dari mesin yang dipilih.
// Galat mesin dikembalikan apa adanya: tidak ada teks cadangan (notes/12).
func WriteCardText(ctx context.Context, complete Completer, engineName string, content news.Content, kind, lang string) (CardText, error) {
	source := cmp.Or(content.Article.Source, content.Article.Domain)
	system, schema := cardPrompt(kind, source)
	if system == "" {
		return CardText{}, fmt.Errorf("unknown kind %q: use %q, %q or %q", kind, KindSummary, KindCaption, KindStory)
	}
	user := fmt.Sprintf("Title: %s\n\nArticle:\n\n%s", content.Article.Title, content.Numbered(1500))
	raw, err := complete(ctx, system, user, schema)
	if err != nil {
		return CardText{}, err
	}
	var r struct {
		Text       string   `json:"text"`
		Hashtags   []string `json:"hashtags"`
		Paragraphs []string `json:"paragraphs"`
	}
	if err := json.Unmarshal([]byte(ExtractJSON(raw)), &r); err != nil {
		return CardText{}, JSONError(engineName, raw, err)
	}
	if kind == KindStory {
		var ps []string
		for _, p := range r.Paragraphs {
			if p = strings.TrimSpace(p); p != "" {
				ps = append(ps, p)
			}
		}
		r.Text = strings.Join(ps, "\n\n")
	}
	out := CardText{Text: strings.TrimSpace(r.Text), Violations: []Violation{}}
	if out.Text == "" {
		return CardText{}, fmt.Errorf("%s returned an empty %s", engineName, kind)
	}
	for _, h := range r.Hashtags {
		if h = strings.TrimSpace(strings.TrimLeft(h, "#")); h != "" {
			out.Hashtags = append(out.Hashtags, strings.ReplaceAll(h, " ", ""))
		}
	}
	if kind == KindSummary {
		if cut, ok := cutAtWord(out.Text, SummaryMax); ok {
			out.Text = cut
			out.Violations = append(out.Violations, Violation{"length", fmt.Sprint(SummaryMax),
				fmt.Sprintf("the summary was longer than %d characters and was shortened", SummaryMax)})
		}
	}
	// Nama sumber yang DIMINTA disebut ("Dikutip dari ANTARA") sah walau tidak
	// tertulis di badan artikel.
	checked := content
	checked.Article.Title += " " + source
	out.Violations = append(out.Violations, CheckText(out.Text, checked)...)
	return out, nil
}

// cutAtWord memotong s di batas kata terakhir yang muat dalam max karakter.
// ok=false bila s sudah muat.
func cutAtWord(s string, max int) (string, bool) {
	r := []rune(s)
	if len(r) <= max {
		return s, false
	}
	cut := string(r[:max])
	if i := strings.LastIndexAny(cut, " \t\n"); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:-"), true
}

// cardPrompt: tulisan SELALU memakai bahasa artikelnya, bukan bahasa
// antarmuka (`lang` hanya untuk teks tetap kartu). Diuji 9 Oktober 2026:
// artikel Indonesia dengan antarmuka EN menghasilkan caption Inggris, padahal
// postingannya untuk pembaca artikel itu — dan pagar fakta mencocokkan nama
// serta angka paling tepat dalam bahasa sumbernya.
//
// Gaya caption & isi carousel = kreator konten yang menyampaikan ulang berita
// ke pengikutnya dan menyebut sumbernya (keputusan pemilik 25 Oktober 2026);
// ringkasan kartu pertama tetap satu kalimat netral.
func cardPrompt(kind, source string) (string, map[string]any) {
	language := "the same language as the article"
	str := map[string]any{"type": "string"}
	switch kind {
	case KindSummary:
		return fmt.Sprintf(`You write the text for a news card shown on social media.
Write ONE sentence in %s that tells the core of the article.
- Ideally 140 characters or fewer, never more than %d.
- Use only facts stated in the article: no new numbers, names, or quotes.
- Factual and calm: no clickbait, no questions, no exclamation marks, no emoji.
Reply with JSON only: {"text": "<the sentence>"}`, language, SummaryMax),
			map[string]any{"type": "object", "properties": map[string]any{"text": str}, "required": []string{"text"}}
	case KindStory:
		return fmt.Sprintf(`You are a news content creator on social media. Your post is a carousel: the first slide shows the headline, the next slides explain the story in detail.
Write the text for those next slides in %s: 4 to 8 short paragraphs, each 1 to 3 sentences.
- The first paragraph names the source the way creators do, e.g. "Dikutip dari %[2]s, ..." in Indonesian or "According to %[2]s, ..." in English. Write the source name exactly as %[2]q.
- Explain what happened, who is involved, the key numbers, and what happens next, in a casual but informative voice, as if talking to your followers.
- Use only facts stated in the article: no new numbers, names, or quotes.
- No clickbait, no questions, no emoji.
Reply with JSON only: {"paragraphs": ["...", "..."]}`, language, source),
			map[string]any{"type": "object", "properties": map[string]any{
				"paragraphs": map[string]any{"type": "array", "items": str},
			}, "required": []string{"paragraphs"}}
	case KindCaption:
		return fmt.Sprintf(`You are a news content creator on social media, writing the caption of a post that retells a news story to your followers.
Write 2 to 4 short sentences in %s, plus 3 to 6 hashtags.
- Mention the source naturally, e.g. "Dikutip dari %[2]s, ..." in Indonesian or "According to %[2]s, ..." in English. Write the source name exactly as %[2]q.
- Tell what is new, in a casual but informative voice, as if talking to your followers.
- Use only facts stated in the article: no new numbers, names, or quotes.
- No clickbait, no emoji.
- Hashtags: one or two words each, WITHOUT the # sign, taken from the article's topic, places, and people.
Reply with JSON only: {"text": "<caption>", "hashtags": ["...", "..."]}`, language, source),
			map[string]any{"type": "object", "properties": map[string]any{
				"text": str, "hashtags": map[string]any{"type": "array", "items": str},
			}, "required": []string{"text", "hashtags"}}
	}
	return "", nil
}
