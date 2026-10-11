package writer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/gemgum/clipper/engine/internal/news"
)

// Source satu artikel sumber beserta fakta yang sudah diekstrak darinya.
//
// Article TIDAK pernah dikirim ke model di tahap 2 — itulah gunanya tahap 1.
// Ia disimpan untuk PAGAR: angka dan kutipan diperiksa terhadap teks aslinya,
// bukan terhadap fakta yang sudah diringkas.
type Source struct {
	Article news.Content
	Facts   FactSheet
}

// Claim satu pernyataan di artikel hasil, beserta asalnya.
type Claim struct {
	Text string `json:"text"`
	// Source = urutan artikel sumber (0-based), Paragraph = nomor paragrafnya.
	Source    int `json:"source"`
	Paragraph int `json:"paragraph"`
}

// Violation pelanggaran pagar. Tidak menggagalkan job — ikut ke keluaran supaya
// terlihat redaktur (notes/38).
type Violation struct {
	Kind   string `json:"kind"` // number | quote | name | length | repetition | coverage | claim
	Text   string `json:"text"`
	Detail string `json:"detail"`
}

// Draft artikel hasil tahap 2.
type Draft struct {
	Title  string   `json:"title"`
	Lead   string   `json:"lead"`
	Body   []string `json:"body"`
	Claims []Claim  `json:"claims,omitempty"`
	Words  int      `json:"words"`
	// Tags = tagar siap tempel, sudah disaring engine (lihat askCompose).
	Tags       []string    `json:"tags,omitempty"`
	Violations []Violation `json:"violations,omitempty"`
	// Repaired menandai draf yang sempat melanggar lalu diperbaiki sekali.
	Repaired bool `json:"repaired,omitempty"`
}

// Panjang badan artikel, dalam kata. Angka dari notes/38.
const (
	MinWords = 400
	MaxWords = 700
)

const systemComposePrompt = `You are a newsroom editor. You write ONE new article from facts that were
extracted from several source articles.

Each fact carries the number of its source article and the paragraph it came from.

Rules:
- Use ONLY the facts you are given. Never add background, context, or anything you happen to know.
- Do NOT use quotation marks. The facts are paraphrases, so putting them in quotes would invent
  a quote nobody actually said.
- Copy names, numbers, dates and places EXACTLY as they appear in the facts.
- Write the whole article in %[1]s — title, lead and body. The facts are in %[1]s.
  NEVER translate. An article in any other language is a failure, however well written.
- Draw on EVERY source. When a source carries a fact the others do not have, that fact is
  exactly why it is there — include it. An article rewritten from only one source is a failure.
- Where sources disagree, say so plainly instead of picking one.
- The [source/paragraph] tags belong in "claims" only. Never let one appear in the article text.
- No conclusion, no opinion, no call to action, no invitation to read more.

Fields:
- "title"  — one line, factual, no clickbait.
- "lead"   — one or two sentences carrying the core of the news.
- "body"   — array of paragraphs, each 3 to 5 full sentences, %[2]d to %[3]d words in total.
             Every paragraph must carry facts. Never pad the array with filler
             to reach a count — an empty-looking paragraph is worse than a short article.
- "claims" — for every factual sentence you write, the source and paragraph numbers it came from.
- "tags"   — 3 to 6 keywords for social media, one or two words each, WITHOUT the # sign.
             Every keyword must be a word that appears in your own article. Names of people,
             places and organisations make the best ones. Not sentences, not topics you assume.

Reply with JSON ONLY, in exactly this shape:

{"title": "...", "lead": "...", "body": ["paragraph one", "paragraph two"],
 "claims": [{"text": "...", "source": 0, "paragraph": 3}], "tags": ["Bantul", "BPBD DIY"]}`

// ComposeSchema = JSON Schema untuk parameter "format" Ollama.
//
// "claims" sengaja TIDAK wajib: model kecil kerap sanggup menulis artikelnya
// tetapi gagal menyusun petanya sekaligus, dan artikel tanpa peta masih berguna
// sementara job yang gagal tidak (notes/38).
func ComposeSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string"},
			"lead":  map[string]any{"type": "string"},
			"body": map[string]any{
				"type":     "array",
				"minItems": 3,
				"items":    map[string]any{"type": "string"},
			},
			"tags": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"claims": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"text":      map[string]any{"type": "string"},
						"source":    map[string]any{"type": "integer"},
						"paragraph": map[string]any{"type": "integer"},
					},
					"required": []string{"text", "source", "paragraph"},
				},
			},
		},
		"required": []string{"title", "lead", "body"},
	}
}

// Compose menjalankan tahap 2: beberapa lembar fakta → satu artikel.
//
// Kebijakan saat pagar menemukan pelanggaran (notes/38): perbaiki SEKALI, lalu
// artikelnya tetap dikembalikan apa pun hasilnya. Job tidak pernah digagalkan
// karena pagar — keluarannya draf, dan selalu ada mata redaktur sebelum terbit.
func Compose(ctx context.Context, sources []Source, complete news.Completer, engineName string) (Draft, error) {
	if len(sources) == 0 {
		return Draft{}, fmt.Errorf("no source articles to write from")
	}
	// Bahasanya disebut TEGAS, bukan "sama seperti fakta": llama3.1 dan DeepSeek
	// sama-sama menerjemahkan artikel Antara ke bahasa Inggris walau aturan itu
	// ada, dan pagar nama lalu melaporkan sembilan kata Inggris biasa sebagai
	// nama karangan (18 Agustus 2026).
	lang := languageOf(sources)
	system := fmt.Sprintf(systemComposePrompt, lang, MinWords, MaxWords)
	user := factsPrompt(sources)

	draft, err := askCompose(ctx, system, user, complete, engineName)
	if err != nil {
		return Draft{}, err
	}
	draft.Violations = inspect(draft, sources)
	if len(draft.Violations) == 0 {
		return draft, nil
	}

	// Percobaan kedua, sekali saja. Kalau hasilnya tidak lebih baik, draf
	// pertama yang dipakai — perbaikan tidak boleh membuat lebih buruk.
	// Draf yang KEPENDEKAN tidak bisa diperbaiki dengan menegurnya: model kecil
	// tidak menambah panjang atas perintah, ia menambah panjang kalau diberi
	// bahan. Diuji di lapangan: 38 fakta tersedia, ±10 terpakai, hasilnya 147
	// kata. Jadi pass kedua dibekali daftar fakta yang belum dipakai — dan
	// panjangnya lahir dari bahan, bukan dari tambalan.
	var unused []string
	if draft.Words < MinWords {
		unused = unusedFacts(draft, sources)
	}
	second, err := askCompose(ctx, system, user+"\n\n"+repairNote(draft.Violations, unused), complete, engineName)
	if err == nil {
		second.Violations = inspect(second, sources)
		if better(second, draft) {
			draft = second
		}
	}
	draft.Repaired = true

	// Paragraf yang MENYALIN paragraf lain dibuang, bukan cuma dilaporkan.
	// Membuangnya operasi mekanis tanpa penilaian — dan draf yang tujuh
	// paragraf terakhirnya salinan hampir tidak berguna buat redaktur. Yang
	// dibuang tetap tercatat di Violations, jadi tidak ada yang disembunyikan.
	if repeats := draft.dropRepeats(); len(repeats) > 0 {
		draft.Words = words(draft.Body)
		draft.Violations = append(inspect(draft, sources), repeats...)
	}
	return draft, nil
}

// languageOf menebak bahasa fakta, untuk disebutkan tegas di prompt.
//
// Bukan pendeteksi bahasa serius, cuma hitung kata fungsi — dan memang tidak
// perlu lebih: yang dijawab hanya "Indonesian atau English", dua bahasa yang
// kata fungsinya tidak bertindih sama sekali. Bahasa lain jatuh ke Indonesian,
// dan itu pilihan yang benar untuk alat redaksi berbahasa Indonesia.
func languageOf(sources []Source) string {
	var id, en int
	for _, s := range sources {
		for _, f := range s.Facts.Facts {
			for _, w := range strings.Fields(normalize(f.Text)) {
				switch w {
				case "yang", "dan", "dengan", "untuk", "dari", "pada", "adalah", "tidak", "itu", "akan":
					id++
				case "the", "of", "and", "is", "was", "for", "with", "that", "are", "from":
					en++
				}
			}
		}
	}
	if en > id {
		return "English"
	}
	return "Indonesian"
}

// better menyebut apakah draf a lebih baik daripada b.
//
// BUKAN sekadar "pelanggarannya lebih sedikit". Aturan itu memberi kemenangan
// pada draf yang paling KOSONG: artikel 35 kata cuma melanggar dua hal (pendek,
// cakupan), sedangkan artikel 500 kata yang menyebut enam nama diri melanggar
// tujuh — jadi perbaikan yang berhasil justru dibuang, dan pemilik proyek
// melihat hasil yang makin pendek tiap kali pagarnya diperketat. Terjadi
// sungguhan, 18 Agustus 2026.
//
// Karena itu KEKURANGAN KATA dinilai lebih dulu: artikel yang panjangnya masuk
// akal selalu menang atas yang kependekan, dan di antara dua yang sama-sama
// kependekan, yang lebih panjang menang — ia memakai lebih banyak bahan. Jumlah
// pelanggaran baru jadi penentu setelah panjangnya setara.
func better(a, b Draft) bool {
	sa, sb := max(0, MinWords-a.Words), max(0, MinWords-b.Words)
	if sa != sb {
		return sa < sb
	}
	return len(a.Violations) < len(b.Violations)
}

// dropRepeats membuang paragraf yang mengulang lead atau paragraf sebelumnya,
// dan mengembalikan catatan tiap pengulangan.
//
// Pembuangannya BERSYARAT: kalau yang tersisa tinggal separuh kata atau kurang,
// pengulangannya cuma dilaporkan dan draf dibiarkan utuh. Pagar tidak boleh
// membuat lebih buruk — pernah terjadi badan 3 paragraf yang paragraf
// pertamanya menyalin lead keluar sebagai artikel 2 kata, dan draf utuh yang
// bertele-tele jauh lebih berguna bagi redaktur daripada itu.
func (d *Draft) dropRepeats() []Violation {
	type dup struct {
		at    int
		label string
	}
	var kept []string
	var dups []dup
	for i, para := range d.Body {
		if label, isDup := repeatOf(d.Lead, kept, para); isDup {
			dups = append(dups, dup{i, label})
			continue
		}
		kept = append(kept, para)
	}
	if len(dups) == 0 {
		return nil
	}

	drop := 2*words(kept) >= words(d.Body)
	var vs []Violation
	for _, du := range dups {
		what := "repeats"
		if drop {
			what = "removed: it repeats"
		}
		vs = append(vs, Violation{"repetition", truncate(d.Body[du.at], 60),
			fmt.Sprintf("paragraph %d %s %s", du.at+1, what, du.label)})
	}
	if drop {
		d.Body = kept
	}
	return vs
}

func words(paras []string) int { return len(strings.Fields(strings.Join(paras, " "))) }

// repeatOf menyebut apa yang diulang sebuah paragraf, bila ia mengulang.
func repeatOf(lead string, kept []string, para string) (string, bool) {
	if similar(lead, para) {
		return "the lead", true
	}
	for j, k := range kept {
		if similar(k, para) {
			return fmt.Sprintf("paragraph %d", j+1), true
		}
	}
	return "", false
}

func askCompose(ctx context.Context, system, user string, complete news.Completer, engineName string) (Draft, error) {
	raw, err := complete(ctx, system, user)
	if err != nil {
		return Draft{}, err
	}
	var d Draft
	if err := json.Unmarshal([]byte(ExtractJSON(raw)), &d); err != nil {
		return Draft{}, JSONError(engineName, raw, err)
	}
	d.strip()
	d.Tags = hashtags(d)
	// Batas panjang berlaku untuk BADAN artikel (notes/38), jadi judul dan lead
	// tidak ikut dihitung — walau keduanya tetap diperiksa pagar.
	d.Words = words(d.Body)
	return d, nil
}

// maxTags = jumlah tagar yang ditulis. Sama dengan tab kartu berita.
const maxTags = 6

// hashtags menyaring kata kunci dari model jadi tagar.
//
// Penyaringnya dipinjam dari tab kartu berita (news.Content.Hashtags): kata
// kunci yang TIDAK muncul di artikelnya sendiri dibuang. Tanpa itu tagar jadi
// satu-satunya celah mengarang yang tersisa — ia terlalu pendek untuk ditangkap
// pagar angka, kutipan, atau nama, tapi cukup untuk menempelkan artikel ke
// peristiwa yang tidak ada di dalamnya.
func hashtags(d Draft) []string {
	if len(d.Tags) == 0 {
		return nil
	}
	keywords := make([]string, 0, len(d.Tags))
	for _, t := range d.Tags {
		// Model kerap menulis "#Paskibraka" walau diminta tanpa tanda pagar, dan
		// tanda itu tidak akan pernah ditemukan di badan artikel.
		if k := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(t), "#")); k != "" {
			keywords = append(keywords, k)
		}
	}
	body := news.Content{Paragraphs: []news.Paragraph{{Text: d.plain()}}}
	return body.Hashtags(keywords, maxTags)
}

// tagRe = penanda [sumber/paragraf] pada bahan tahap 2.
var tagRe = regexp.MustCompile(`\s*\[\d+\s*/\s*\d+\]`)

// bulletRe = penanda daftar di AWAL paragraf: "- ", "* ", "1. ", "2) ".
var bulletRe = regexp.MustCompile(`^\s*(?:[-*•]|\d+[.)])\s+`)

// strip membuang penanda [sumber/paragraf] yang ikut tersalin ke badan artikel.
//
// llama3.1 kerap menyalinnya, dan akibatnya bukan cuma jelek dibaca: nomor
// sumber terbaca pagar sebagai ANGKA KARANGAN, sehingga satu artikel bersih
// dilaporkan melanggar enam kali. Dibersihkan di sini karena bentuk penandanya
// pasti — jauh lebih andal daripada meminta model tidak menyalinnya.
func (d *Draft) strip() {
	clean := func(s string) string {
		// Penanda daftar di awal paragraf ikut dibuang: model kerap menulis
		// "- " atau "1. " walau yang diminta paragraf, dan penanda itu terbawa
		// ke artikel.md sebagai daftar berpoin.
		return bulletRe.ReplaceAllString(strings.TrimSpace(tagRe.ReplaceAllString(s, "")), "")
	}
	d.Title, d.Lead = clean(d.Title), clean(d.Lead)
	for i := range d.Body {
		d.Body[i] = clean(d.Body[i])
	}
	for i := range d.Claims {
		d.Claims[i].Text = clean(d.Claims[i].Text)
	}
}

// plain menggabungkan seluruh teks yang ditulis model — yang inilah yang
// diperiksa pagar.
func (d Draft) plain() string {
	return strings.Join(append([]string{d.Title, d.Lead}, d.Body...), "\n")
}

// factsPrompt menyusun bahan tahap 2. Badan artikel TIDAK ikut — hanya fakta.
//
// Faktanya diselang-seling antar sumber, bukan dikelompokkan per sumber.
// Sebabnya hasil uji: dengan blok per sumber, llama3.1 kerap menyalin ulang
// blok pertama dan mengabaikan sisanya — artikelnya tetap enak dibaca, jadi
// kegagalannya tidak kelihatan tanpa pagar coverage. Diselang-seling, tiap
// sumber muncul sejak baris-baris awal.
//
// Tiap baris tetap membawa [sumber/paragraf] supaya petanya bisa disusun.
func factsPrompt(sources []Source) string {
	var sb strings.Builder
	sb.WriteString("Sources:\n")
	for i, s := range sources {
		fmt.Fprintf(&sb, "  %d = %s\n", i, s.Facts.Source)
	}
	sb.WriteString("\nFacts, one per line, tagged [source/paragraph]:\n")

	most := 0
	for _, s := range sources {
		if n := len(s.Facts.Facts); n > most {
			most = n
		}
	}
	total := 0
	for round := 0; round < most; round++ {
		for i, s := range sources {
			if round < len(s.Facts.Facts) {
				f := s.Facts.Facts[round]
				fmt.Fprintf(&sb, "  [%d/%d] %s\n", i, f.Paragraph, f.Text)
				total++
			}
		}
	}

	// Sasaran panjang disebut ulang sebagai JUMLAH PARAGRAF, bukan cuma jumlah
	// kata. "Antara 400 dan 700 kata" adalah angka yang tidak bisa dilacak model
	// kecil sambil menulis, dan hasilnya artikel satu paragraf; "sekitar 8
	// paragraf, tiap paragraf 60–100 kata" bisa dihitung sambil jalan.
	fmt.Fprintf(&sb, "\nYou have %d facts. Write about %d paragraphs, each 3 to 5 full sentences, "+
		"and use EVERY fact at least once — that is where the length comes from.",
		total, paragraphTarget(total))
	return strings.TrimSpace(sb.String())
}

// paragraphTarget menerjemahkan jumlah fakta jadi jumlah paragraf: sekitar tiga
// fakta per paragraf, dijaga di 4..10 supaya dua fakta tidak jadi satu paragraf
// dan lima puluh fakta tidak jadi tujuh belas.
func paragraphTarget(facts int) int {
	n := (facts + 2) / 3
	return max(4, min(n, 10))
}

// repairNote memberitahu model pelanggaran mana yang harus dibereskan.
func repairNote(vs []Violation, unused []string) string {
	var sb strings.Builder
	sb.WriteString("Your previous draft broke these rules. Write it again without them:\n")
	for _, v := range vs {
		fmt.Fprintf(&sb, "- %s: %s\n", v.Detail, v.Text)
	}
	if len(unused) > 0 {
		sb.WriteString("\nThese facts were left out. Work every one of them into the article — " +
			"it is short because the material is unused. Length must come from facts, never from padding:\n")
		for _, f := range unused {
			fmt.Fprintf(&sb, "- %s\n", f)
		}
	}
	return sb.String()
}

// unusedFacts menyebut fakta yang tidak terpakai di draf.
//
// Kata isinya dibandingkan, bukan kalimatnya: model MEMPARAFRASE fakta, jadi
// pencocokan persis selalu bilang "tidak terpakai". Ambangnya separuh kata isi
// — cukup ketat untuk menangkap fakta yang benar-benar hilang, cukup longgar
// untuk fakta yang ditulis ulang dengan kata lain.
func unusedFacts(d Draft, sources []Source) []string {
	in := map[string]bool{}
	for _, w := range contentWords(d.plain()) {
		in[w] = true
	}
	var out []string
	for _, s := range sources {
		for _, f := range s.Facts.Facts {
			want := contentWords(f.Text)
			if len(want) < 3 {
				continue
			}
			hit := 0
			for _, w := range want {
				if in[w] {
					hit++
				}
			}
			if 2*hit < len(want) {
				out = append(out, f.Text)
			}
		}
	}
	return out
}

// inspect = pagar fakta. Deterministik, tanpa LLM.
//
// Yang diperiksa dan alasannya ada di notes/38. Yang TIDAK diperiksa: apakah
// beritanya benar. Kalau semua sumber sama-sama salah, pagar diam saja.
func inspect(d Draft, sources []Source) []Violation {
	vs := factCheck(d.plain(), corpusOf(sources))
	// 4-7 di bawah khusus artikel utuh; 1-3 ada di factCheck.
	return append(vs, draftChecks(d, sources)...)
}

// CheckText menjalankan pagar fakta 1-3 (angka, kutipan, nama diri) pada teks
// pendek dari SATU artikel: ringkasan & caption kartu berita (DESIGN-NEWSCARD,
// keputusan pemilik 9 Oktober 2026). Panjang, cakupan, dan peta klaim tidak
// berlaku di sana, jadi tidak ikut.
func CheckText(text string, content news.Content) []Violation {
	return factCheck(text, corpusOf([]Source{{Article: content}}))
}

// factCheck = pemeriksaan 1-3 dari inspect.
func factCheck(text string, corpus corpus) []Violation {
	var vs []Violation
	// 1. Angka. Angka karangan adalah kesalahan paling mahal di berita, dan
	//    paling gampang diperiksa.
	for _, n := range numbersIn(text) {
		if !corpus.hasNumber(n) {
			vs = append(vs, Violation{"number", n, "number does not appear in any source article"})
		}
	}
	// 2. Kutipan. Prompt melarang tanda kutip sama sekali; kalau model tetap
	//    memakainya, isinya harus PERSIS ada di artikel sumber.
	for _, q := range quotesIn(text) {
		if !corpus.hasPhrase(q) {
			vs = append(vs, Violation{"quote", q, "quoted words do not appear verbatim in any source article"})
		}
	}
	// 3. Nama diri. Awal kalimat dilewati supaya "Sementara"/"Menurut" tidak
	//    dikira nama.
	for _, name := range namesIn(text) {
		if !corpus.hasWord(name) {
			vs = append(vs, Violation{"name", name, "name does not appear in any source article"})
		}
	}
	return vs
}

// draftChecks = pemeriksaan 4-7 dari inspect.
func draftChecks(d Draft, sources []Source) []Violation {
	var vs []Violation
	// 4. Panjang.
	if d.Words < MinWords || d.Words > MaxWords {
		vs = append(vs, Violation{"length", fmt.Sprint(d.Words),
			fmt.Sprintf("body is %d words, outside %d-%d", d.Words, MinWords, MaxWords)})
	}
	// 5. Pengulangan. Ditemukan pada uji ujung-ke-ujung: diminta 400 kata
	//    padahal bahannya 120, llama3.1 mengejar targetnya dengan menyalin
	//    paragrafnya sendiri dan mengulang lead sebagai paragraf pertama.
	//    Panjangnya jadi "tercapai" tanpa satu pun fakta baru.
	vs = append(vs, repetitions(d)...)

	// 6. Cakupan sumber. Diuji terhadap llama3.1: draf pertama menyalin ulang
	//    sumber pertama dan mengabaikan sumber kedua sepenuhnya. Kalau lima
	//    artikel menyusut jadi satu, seluruh gunanya fitur ini hilang — dan itu
	//    tidak kelihatan dari artikelnya, sebab hasilnya tetap enak dibaca.
	vs = append(vs, coverage(d, sources)...)

	// 7. Peta klaim. Nomor yang menunjuk ke sumber atau paragraf yang tidak ada
	//    membuat sumber.json menyesatkan — lebih buruk daripada tidak ada peta.
	for _, c := range d.Claims {
		if c.Source < 0 || c.Source >= len(sources) {
			vs = append(vs, Violation{"claim", c.Text, fmt.Sprintf("source %d does not exist", c.Source)})
			continue
		}
		if _, ok := sources[c.Source].Article.TextAt(c.Paragraph); !ok {
			vs = append(vs, Violation{"claim", c.Text,
				fmt.Sprintf("paragraph %d does not exist in source %d", c.Paragraph, c.Source)})
		}
	}
	return vs
}

// repetitions melaporkan paragraf yang mengulang paragraf lain, termasuk lead
// yang disalin jadi paragraf pertama.
//
// Ambangnya 80% kata isi yang sama: menulis ulang sudut yang sama dengan kata
// berbeda itu sah, menyalin kalimatnya tidak.
func repetitions(d Draft) []Violation {
	blocks := append([]string{d.Lead}, d.Body...)
	names := append([]string{"lead"}, make([]string, len(d.Body))...)
	for i := range d.Body {
		names[i+1] = fmt.Sprintf("paragraph %d", i+1)
	}

	var vs []Violation
	for i := 0; i < len(blocks); i++ {
		for j := i + 1; j < len(blocks); j++ {
			if similar(blocks[i], blocks[j]) {
				vs = append(vs, Violation{"repetition", truncate(blocks[j], 60),
					fmt.Sprintf("%s repeats %s", names[j], names[i])})
			}
		}
	}
	return vs
}

// similar benar bila b mengulang setidaknya 80% kata isi a (dan sebaliknya
// tidak jauh lebih panjang) — cukup untuk menangkap salinan dan hampir-salinan
// tanpa menghukum dua paragraf yang kebetulan membahas hal sama.
func similar(a, b string) bool {
	wa, wb := contentWords(a), contentWords(b)
	if len(wa) < 5 || len(wb) < 5 {
		return false
	}
	in := map[string]bool{}
	for _, w := range wa {
		in[w] = true
	}
	same := 0
	for _, w := range wb {
		if in[w] {
			same++
		}
	}
	shorter := min(len(wa), len(wb))
	return same*10 >= shorter*8
}

// coverage melaporkan sumber yang fakta khasnya tidak terpakai sama sekali.
//
// "Khas" = kata isi yang ada di fakta sumber itu dan TIDAK ada di artikel
// sumber mana pun yang lain. Sumber yang isinya bertindih penuh dengan sumber
// lain tidak bisa dinilai dan dilewati — tidak terpakai bukan berarti diabaikan
// kalau semua isinya sudah tercakup sumber lain.
func coverage(d Draft, sources []Source) []Violation {
	if len(sources) < 2 {
		return nil
	}
	inDraft := map[string]bool{}
	for _, w := range contentWords(d.plain()) {
		inDraft[w] = true
	}

	var vs []Violation
	for i, s := range sources {
		own := map[string]bool{}
		for _, f := range s.Facts.Facts {
			for _, w := range contentWords(f.Text) {
				own[w] = true
			}
		}
		for j, other := range sources {
			if j == i {
				continue
			}
			for _, p := range other.Article.Paragraphs {
				for _, w := range contentWords(p.Text) {
					delete(own, w)
				}
			}
		}
		if len(own) == 0 {
			continue
		}
		// Ambang dua kata, alasannya sama dengan minOverlap: satu kata bisa
		// muncul kebetulan. Contoh nyata dari uji — sumber kedua menyumbang
		// "rusak" ke draf yang seluruh isinya dari sumber pertama, semata
		// karena sumber pertama menulis "kerusakan".
		used := 0
		for w := range own {
			if inDraft[w] {
				used++
			}
		}
		if used < minOverlap {
			vs = append(vs, Violation{"coverage", s.Facts.Source,
				fmt.Sprintf("nothing from source %d made it into the article", i)})
		}
	}
	return vs
}

// corpus = seluruh teks artikel sumber, disiapkan untuk pencocokan.
type corpus struct {
	words   map[string]bool
	numbers map[string]bool
	flat    string
}

func corpusOf(sources []Source) corpus {
	c := corpus{words: map[string]bool{}, numbers: map[string]bool{}}
	var sb strings.Builder
	for _, s := range sources {
		for _, p := range s.Article.Paragraphs {
			sb.WriteString(p.Text)
			sb.WriteString(" ")
		}
		// Judul artikel sumber ikut: nama yang hanya muncul di judul tetap
		// nama yang sah.
		sb.WriteString(s.Article.Article.Title)
		sb.WriteString(" ")
	}
	c.flat = normalize(sb.String())
	for _, w := range strings.Fields(c.flat) {
		c.words[w] = true
	}
	for _, n := range numbersIn(sb.String()) {
		c.numbers[normalizeNumber(n)] = true
	}
	return c
}

// hasWord: normalize() mengapit hasilnya dengan spasi supaya hasPhrase bisa
// mencari frasa utuh, jadi untuk lookup per kata spasinya harus dilucuti dulu.
// Tanpa itu SETIAP nama diri dilaporkan sebagai karangan.
func (c corpus) hasWord(w string) bool   { return c.words[strings.TrimSpace(normalize(w))] }
func (c corpus) hasPhrase(p string) bool { return strings.Contains(c.flat, normalize(p)) }
func (c corpus) hasNumber(n string) bool { return c.numbers[normalizeNumber(n)] }

// normalize menyeragamkan teks untuk dicocokkan: huruf kecil, tanda baca jadi
// spasi, spasi ganda dirapikan. Tanpa ini "Bantul," dan "Bantul" jadi dua kata
// berbeda.
func normalize(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		} else {
			sb.WriteRune(' ')
		}
	}
	return " " + strings.Join(strings.Fields(sb.String()), " ") + " "
}

// normalizeNumber menyamakan penulisan desimal Indonesia dan Inggris: "5,2"
// dan "5.2" adalah angka yang sama, dan pemisah ribuan dibuang.
func normalizeNumber(n string) string {
	n = strings.ReplaceAll(n, ".", ",")
	n = strings.TrimRight(n, ",")
	// 1,000,000 (ribuan) -> 1000000; 5,2 (desimal) dibiarkan.
	if parts := strings.Split(n, ","); len(parts) > 2 {
		n = strings.Join(parts, "")
	}
	return n
}

var (
	numberRe = regexp.MustCompile(`\d+(?:[.,]\d+)*`)
	quoteRe  = regexp.MustCompile(`["“”]([^"“”]{8,})["“”]`)
	nameRe   = regexp.MustCompile(`\p{Lu}[\p{L}]{3,}`)
	sentRe   = regexp.MustCompile(`(?:^|[.!?:]\s+|\n)\s*(\p{Lu}[\p{L}]*)`)
)

func numbersIn(s string) []string {
	return dedup(numberRe.FindAllString(s, -1))
}

func quotesIn(s string) []string {
	var out []string
	for _, m := range quoteRe.FindAllStringSubmatch(s, -1) {
		out = append(out, strings.TrimSpace(m[1]))
	}
	return dedup(out)
}

// namesIn mengambil kata berhuruf kapital yang BUKAN kata pertama kalimat.
//
// Kata pertama kalimat dilewati karena bahasa Indonesia mengapitalkannya juga —
// tanpa ini "Sementara", "Menurut", dan "Namun" akan terus dilaporkan sebagai
// nama karangan.
func namesIn(s string) []string {
	skip := map[string]bool{}
	for _, m := range sentRe.FindAllStringSubmatch(s, -1) {
		skip[m[1]] = true
	}
	var out []string
	for _, w := range nameRe.FindAllString(s, -1) {
		if !skip[w] {
			out = append(out, w)
		}
	}
	return dedup(out)
}

func dedup(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
