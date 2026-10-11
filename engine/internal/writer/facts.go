// Package writer menulis satu artikel baru dari beberapa artikel sumber.
//
// Bedanya dengan paket news: di sana LLM hanya MEMILIH nomor paragraf dan tidak
// pernah menulis (notes/13). Di sini LLM memang menulis — dan karena itu tiap
// klaimnya wajib menunjuk paragraf asalnya, lalu diperiksa kode. Lihat notes/38.
//
// Berkas ini tahap 1 saja: satu artikel → daftar fakta bernomor.
package writer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/gemgum/clipper/engine/internal/news"
)

// Fact satu fakta yang diambil dari satu paragraf artikel sumber.
type Fact struct {
	Text string `json:"text"`
	// Paragraph = news.Paragraph.Index tempat fakta ini berasal. Inilah yang
	// membuat tahap berikutnya bisa diperiksa: tanpa nomor, tidak ada yang bisa
	// dicocokkan dengan sumbernya.
	//
	// Nilainya ditentukan ENGINE, bukan model — lihat resolve.
	Paragraph int `json:"paragraph"`
	// Recited menandai nomor yang dikoreksi engine karena model salah sebut.
	// Dipakai untuk mengukur mutu model, bukan untuk logika.
	Recited bool `json:"recited,omitempty"`
}

// Rejected fakta yang dibuang beserta alasannya. Dilaporkan, tidak dibuang
// diam-diam — pola yang sama dengan correct.Report.Rejected (notes/14).
type Rejected struct {
	Fact
	Reason string `json:"reason"`
}

// FactSheet hasil tahap 1 untuk SATU artikel sumber.
type FactSheet struct {
	URL    string `json:"url"`
	Source string `json:"source"`
	// Title disimpan untuk manusia saja. Ia TIDAK pernah dikirim ke model di
	// tahap ini — lihat ExtractFacts.
	Title    string     `json:"title"`
	Facts    []Fact     `json:"facts"`
	Rejected []Rejected `json:"rejected,omitempty"`
}

// systemFactsPrompt = instruksi tahap 1.
//
// Berbahasa Inggris mengikuti konvensi proyek; yang diekstrak tetap teks
// berbahasa apa pun artikelnya.
const systemFactsPrompt = `You extract facts from a news article for a newsroom.

You are given the BODY of one article, split into numbered paragraphs. The headline is
deliberately NOT given to you: work from the paragraphs, nothing else.

For every fact you list, you MUST give the number of the paragraph it came from.

Rules:
- List only facts that are actually stated in the paragraphs. Never add background knowledge,
  context you happen to know, or anything you infer.
- One fact per item, at most 25 words each.
- Copy names, numbers, dates, places and job titles EXACTLY as they are written.
- Write each fact in the SAME LANGUAGE as the paragraphs. Never translate. A translated fact
  no longer shares any wording with its paragraph, and the engine drops it as unsourced.
- Include NEGATIVE facts as well: what did not happen, denials, and the absence of something
  ("no casualties", "no tsunami potential", "declined to comment"). In news these often matter
  most, and they are easy to skip.
- Do not summarise the article as a whole. List its facts, in the order they appear.
- Skip navigation text, related-article teasers, and advertising.

Reply with JSON ONLY, in exactly this shape — an object with a "facts" array, and
"text"/"paragraph" on every item:

{"facts": [{"text": "...", "paragraph": 0}, {"text": "...", "paragraph": 2}]}`

// FactsSchema = JSON Schema untuk parameter "format" Ollama.
//
// minItems memaksa model lokal benar-benar mengisi. Tanpa itu model kecil kerap
// membalas "facts": [] — bentuk sah, isinya kosong (pelajaran dari
// news.SelectionSchema).
func FactsSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"facts": map[string]any{
				"type":     "array",
				"minItems": 1,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"text":      map[string]any{"type": "string"},
						"paragraph": map[string]any{"type": "integer"},
					},
					"required": []string{"text", "paragraph"},
				},
			},
		},
		"required": []string{"facts"},
	}
}

// DefaultMaxWords = jatah kata badan artikel yang dikirim ke model.
//
// Tahap 1 memuat SATU artikel, jadi jatahnya boleh lebih longgar daripada
// news.maxPromptWords yang harus menyisakan ruang untuk daftar peringkat.
const DefaultMaxWords = 1500

// ExtractFacts menjalankan tahap 1 atas satu artikel.
//
// Judul sengaja tidak ikut dikirim. Dengan begitu model tidak punya jalan
// pintas: satu-satunya bahan yang ada adalah badan artikelnya, dan nomor
// paragraf yang ia sebut membuktikan ia benar-benar membacanya (notes/38).
//
// maxWords <= 0 memakai DefaultMaxWords.
func ExtractFacts(ctx context.Context, content news.Content, complete news.Completer, engineName string, maxWords int) (FactSheet, error) {
	sheet := FactSheet{
		URL:    content.Article.URL,
		Source: content.Article.Source,
		Title:  content.Article.Title,
	}
	if len(content.Paragraphs) == 0 {
		return sheet, fmt.Errorf("the article has no paragraphs to read")
	}
	if maxWords <= 0 {
		maxWords = DefaultMaxWords
	}

	user := "Paragraphs:\n\n" + content.Numbered(maxWords)
	raw, err := complete(ctx, systemFactsPrompt, user)
	if err != nil {
		return sheet, err
	}

	facts, err := parseFacts(ExtractJSON(raw))
	if err != nil {
		return sheet, JSONError(engineName, raw, err)
	}

	sheet.Facts, sheet.Rejected = verify(content, facts)
	return sheet, nil
}

// parseFacts membaca balasan tahap 1, dalam kedua bentuk yang muncul di
// lapangan.
//
// Bentuk bakunya objek — {"facts": [...]} — dan itulah yang dipaksakan server
// yang mendukung JSON Schema. Penyedia yang TIDAK memaksakannya (DeepSeek
// menolak response_format sama sekali) kerap membalas larik telanjang. Menolak
// balasan itu berarti membuang pekerjaan yang isinya sudah benar semata karena
// bungkusnya beda.
func parseFacts(s string) ([]Fact, error) {
	var wrapped struct {
		Facts []Fact `json:"facts"`
	}
	if err := json.Unmarshal([]byte(s), &wrapped); err == nil {
		return wrapped.Facts, nil
	}
	var bare []Fact
	if err := json.Unmarshal([]byte(s), &bare); err == nil {
		return bare, nil
	}
	// Galat dari bentuk BAKU yang dilaporkan — itu yang menjelaskan apa yang
	// sebenarnya diminta.
	return nil, json.Unmarshal([]byte(s), &wrapped)
}

// minOverlap = jumlah kata isi yang harus sama sebelum sebuah fakta dianggap
// berasal dari suatu paragraf.
//
// Satu kata terlalu murah: kalimat karangan seperti "Presiden meninjau lokasi
// bencana" menempel ke paragraf mana pun yang kebetulan memuat "bencana". Dua
// kata sudah menutup itu, dan fakta sungguhan hampir selalu membawa lebih —
// nama, angka, atau istilah teknis sekaligus.
const minOverlap = 2

// verify menentukan asal tiap fakta dan membuang yang tidak ada asalnya.
//
// Nomor dari model TIDAK dipercaya, cuma dipakai sebagai pemecah seri. Diuji
// terhadap artikel Antara sungguhan: llama3.1 salah menyebut nomor pada 2 dari
// 12 fakta, dan salah satunya lolos pemeriksaan "berbagi kata" karena paragraf
// sebelahnya memuat istilah yang sama. Karena SELURUH pagar tahap 2 bekerja
// terhadap nomor ini, salah nomor berarti memverifikasi ke sumber yang keliru —
// jadi engine mencari sendiri paragrafnya.
//
// Tidak pernah mengembalikan galat: artikel yang menyumbang nol fakta tetap
// artikel yang boleh dilanjutkan dengan sumber lain (notes/38).
func verify(content news.Content, facts []Fact) (kept []Fact, rejected []Rejected) {
	for _, f := range facts {
		f.Text = strings.TrimSpace(f.Text)
		if f.Text == "" {
			continue
		}
		cited := f.Paragraph
		best, score := resolve(content, f.Text, cited)
		if score < minOverlap {
			rejected = append(rejected, Rejected{f, "no paragraph in the article shares enough wording with this"})
			continue
		}
		f.Paragraph, f.Recited = best, best != cited
		kept = append(kept, f)
	}
	return kept, rejected
}

// resolve mencari paragraf yang paling mungkin jadi asal sebuah fakta, diukur
// dari jumlah kata isi yang sama.
//
// Sengaja longgar: fakta memang boleh diringkas dan disusun ulang, jadi yang
// dicari bukan kemiripan kalimat melainkan jejak — nama, angka, atau istilah
// yang tidak mungkin muncul kalau paragrafnya tidak dibaca. Pemeriksaan yang
// ketat (angka & kutipan harus persis) tempatnya di tahap 2.
//
// Seri dimenangkan nomor yang disebut model: kalau dua paragraf sama-sama cocok,
// tebakannya sudah sebaik tebakan kita.
func resolve(content news.Content, fact string, cited int) (index, score int) {
	want := contentWords(fact)
	for _, p := range content.Paragraphs {
		inPara := map[string]bool{}
		for _, w := range contentWords(p.Text) {
			inPara[w] = true
		}
		n := 0
		for _, w := range want {
			if inPara[w] {
				n++
			}
		}
		if n > score || (n == score && n > 0 && p.Index == cited) {
			index, score = p.Index, n
		}
	}
	return index, score
}

// contentWords memecah teks jadi kata isi: minimal 4 huruf, atau apa pun yang
// memuat angka. Kata pendek dibuang karena "di", "dan", "yang" ada di hampir
// semua paragraf dan tidak membuktikan apa pun.
func contentWords(s string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(w) >= 4 || strings.ContainsFunc(w, unicode.IsDigit) {
			out = append(out, w)
		}
	}
	return out
}

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

// JSONError menerangkan balasan yang tidak bisa dibaca.
//
// Balasan yang TERPOTONG dibedakan dari yang bentuknya salah: keduanya muncul
// sebagai "unexpected end of JSON input", padahal penyelesaiannya jauh berbeda
// — yang satu butuh jatah token lebih besar, yang satu butuh model lain.
func JSONError(engineName, raw string, err error) error {
	hint := ""
	if s := strings.TrimSpace(raw); !strings.HasSuffix(s, "}") {
		hint = ": the reply was cut off before it ended, so the model ran out of its output budget"
	}
	return fmt.Errorf("%s returned JSON that could not be read: %w%s; reply: %s",
		engineName, err, hint, truncate(strings.ReplaceAll(raw, "\n", " "), 300))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
