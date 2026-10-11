// Package caption membuat caption media sosial dari ucapan sebuah video.
//
// Ini permukaan KEEMPAT setelah klip, kartu berita, dan pembuat berita — dan
// aturan teksnya paling longgar dari semuanya: di sini LLM memang MENGARANG
// kalimat. Yang dijaga bukan kata-katanya, melainkan ISINYA: angka, nama, dan
// klaim yang tidak ada di transkrip dilaporkan sebagai pelanggaran, dan
// transkripnya sendiri selalu ikut ditulis ke berkas hasil supaya ada yang bisa
// dibandingkan sebelum caption itu diposting.
package caption

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/gemgum/clipper/engine/internal/writer"
)

// DefaultVariants = jumlah caption yang diminta sekali jalan.
//
// Tiga, bukan satu: pagar termurah untuk teks karangan adalah manusia yang
// memilih, dan memilih butuh pembanding. Satu caption berarti menerima apa pun
// yang keluar.
const DefaultVariants = 3

// MaxChars = batas panjang satu caption (hook + badan).
//
// Diambil dari batas caption Instagram/TikTok yang masih tampil tanpa "more":
// caption yang terpotong kehilangan justru bagian yang memancing.
const MaxChars = 400

// DefaultMaxWords = jatah kata transkrip yang dikirim ke model.
//
// Video 5 menit ≈ 750 kata, jadi batas ini tidak pernah terpakai pada pemakaian
// biasa. Ia ada untuk saat batas menitnya dinaikkan: konteks 8k model lokal
// habis di sekitar angka ini, dan balasan yang terpotong muncul sebagai JSON
// rusak, bukan sebagai pesan "kepanjangan".
const DefaultMaxWords = 1200

// Variant satu caption siap tempel.
type Variant struct {
	// Style = gaya caption ini (DESIGN-Clipper-Lanjutan §5): "direct",
	// "question", atau "short". Ditampilkan sebagai label di GUI.
	Style string   `json:"style,omitempty"`
	Hook  string   `json:"hook"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags,omitempty"`
	// Violations = pagar yang dilanggar caption ini. Tidak menggagalkan apa pun
	// (kebijakan yang sama dengan pembuat berita, notes/38): keluarannya draf,
	// dan yang dibutuhkan pemakainya adalah tahu MANA yang perlu dicek.
	Violations []string `json:"violations,omitempty"`
}

// Text menggabungkan hook & badan jadi satu caption siap salin.
func (v Variant) Text() string {
	hook := strings.TrimSpace(v.Hook)
	body := strings.TrimSpace(v.Body)
	switch {
	case hook == "":
		return body
	case body == "":
		return hook
	}
	return hook + "\n\n" + body
}

const systemPrompt = `You write captions for short vertical videos on social media.

You are given what is SAID in the video, as plain text.

Write %[1]d captions. Not fewer — %[1]d, each taking a different angle on the video.
Give them these styles in this order, repeating the cycle if more are asked:
1. "direct": the hook states the strongest claim of the video plainly.
2. "question": the hook is a question the video actually answers.
3. "short": a short hook and a body of ONE short sentence.

What a hook is: one line that makes a thumb stop. It is a claim, a question, a number,
or a contradiction that is ACTUALLY IN the video. It is never a summary, and it never
opens with "In this video" or "Video ini".

Rules:
- Use ONLY what the transcript says. Never add a fact, a number, a name, or a claim
  that is not in it, and never promise something the video does not deliver — a
  caption that oversells is a failure however well it reads.
- Write ABOUT what is said. Do not write as if you were the person speaking, and do
  not put your own opinion in their mouth.
- The transcript is speech recognition output, so it holds broken and half-heard
  words. Where a line makes no sense, LEAVE IT OUT — never guess what it meant and
  never repeat it as if it were a phrase.
- Do NOT use quotation marks: no wording here is exact enough to quote.
- Write every caption in %[2]s. NEVER translate the topic into another language.
- Keep each caption under %[3]d characters, hook included. At most two emoji.
- No call to action about liking, following, or watching to the end.

Fields:
- "style" — "direct", "question" or "short", as listed above.
- "hook" — the first line. One sentence, at most 12 words.
- "body" — one to three sentences saying what is actually in the video.
- "tags" — 3 to 6 hashtags WITHOUT the # sign, one or two words each. Only names,
  places and topics the transcript itself mentions, spelled the way it spells them.

Reply with JSON ONLY, in exactly this shape:

{"captions": [{"style": "direct", "hook": "...", "body": "...", "tags": ["Jakarta", "banjir"]}]}`

// Schema = JSON Schema untuk parameter "format" Ollama (notes/35).
//
// minItems mengikuti jumlah yang diminta, bukan 1: model kecil membalas satu
// caption walau promptnya menyebut tiga — terlihat di lapangan, llama3.1
// membalas 1 dari 3. Skema yang menyebut angkanya membuat server modelnya
// sendiri yang menuntut ulang.
func Schema(n int) map[string]any {
	if n < 1 {
		n = 1
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"captions": map[string]any{
				"type":     "array",
				"minItems": n,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"style": map[string]any{"type": "string", "enum": Styles},
						"hook":  map[string]any{"type": "string"},
						"body":  map[string]any{"type": "string"},
						"tags": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
						},
					},
					"required": []string{"hook", "body"},
				},
			},
		},
		"required": []string{"captions"},
	}
}

// Generate meminta caption dari satu transkrip.
//
// transcript = ucapan apa adanya, satu kalimat per baris (bentuk yang sama
// dengan berkas .txt pendamping tiap klip). Timestamp sengaja tidak ikut: yang
// dibutuhkan model adalah isinya, dan angka waktu cuma memancingnya menulis
// "di menit 3" yang tidak berlaku di caption.
func Generate(ctx context.Context, complete writer.Completer, transcript string, opts Options) ([]Variant, error) {
	transcript = strings.TrimSpace(transcript)
	if transcript == "" {
		return nil, fmt.Errorf("this video has no speech to write a caption from")
	}
	opts = opts.withDefaults()

	system := fmt.Sprintf(systemPrompt, opts.Variants, langName(opts.Lang), MaxChars)
	if terms := strings.TrimSpace(strings.Join(opts.Terms, ", ")); terms != "" {
		// Daftar istilah dipakai ulang dari halaman klip: whisper menuliskan nama
		// daerah sebagai kata terdekat yang ia kenal, dan nama yang salah di
		// caption jauh lebih terlihat daripada di subtitle.
		system += "\n\nSpell these names and terms exactly like this: " + terms + "."
	}

	user := "Transcript:\n\n" + limitWords(transcript, opts.MaxWords)
	raw, err := complete(ctx, system, user, Schema(opts.Variants))
	if err != nil {
		return nil, err
	}

	variants, err := parse(writer.ExtractJSON(raw))
	if err != nil {
		return nil, writer.JSONError(opts.EngineName, raw, err)
	}
	if len(variants) == 0 {
		return nil, fmt.Errorf("%s returned no captions", opts.EngineName)
	}
	return check(variants, transcript), nil
}

// parse membaca balasan dalam kedua bentuk yang muncul di lapangan: objek
// {"captions": [...]} dari server yang memaksakan skema, dan larik telanjang
// dari yang tidak (DeepSeek menolak response_format sama sekali).
// Styles = urutan gaya yang diminta prompt.
var Styles = []string{"direct", "question", "short"}

func parse(s string) ([]Variant, error) {
	var wrapped struct {
		Captions []Variant `json:"captions"`
	}
	if err := json.Unmarshal([]byte(s), &wrapped); err == nil {
		return withStyles(wrapped.Captions), nil
	}
	var bare []Variant
	if err := json.Unmarshal([]byte(s), &bare); err == nil {
		return withStyles(bare), nil
	}
	return nil, json.Unmarshal([]byte(s), &wrapped)
}

// withStyles mengisi gaya yang kosong atau tak dikenal dari urutannya: model
// yang mengabaikan field itu (server tanpa skema) tetap menghasilkan label
// yang konsisten dengan urutan yang diminta prompt.
func withStyles(vs []Variant) []Variant {
	for i := range vs {
		ok := false
		for _, st := range Styles {
			ok = ok || vs[i].Style == st
		}
		if !ok {
			vs[i].Style = Styles[i%len(Styles)]
		}
	}
	return vs
}

// check menjalankan pagar deterministik atas tiap caption.
//
// Tidak ada yang dibuang. Caption yang melanggar tetap ikut — dengan catatan
// pelanggarannya — sebab yang menilai "terlalu mengada-ada" pada akhirnya
// manusia, sementara "angka ini tidak ada di videonya" bisa dijawab mesin.
func check(variants []Variant, transcript string) []Variant {
	spoken := map[string]bool{}
	for _, n := range numbersIn(transcript) {
		spoken[n] = true
	}
	out := make([]Variant, 0, len(variants))
	for _, v := range variants {
		v.Hook = strings.TrimSpace(v.Hook)
		v.Body = strings.TrimSpace(v.Body)
		if v.Text() == "" {
			continue
		}
		v.Tags = cleanTags(v.Tags)
		text := v.Text()

		for _, n := range numbersIn(text) {
			if !spoken[n] {
				v.Violations = append(v.Violations,
					fmt.Sprintf("the number %s is not said anywhere in the video", n))
			}
		}
		if n := len([]rune(text)); n > MaxChars {
			v.Violations = append(v.Violations,
				fmt.Sprintf("%d characters: %d over the limit, so it will be cut off with a “more”", n, n-MaxChars))
		}
		if strings.ContainsAny(text, `"“”`) {
			v.Violations = append(v.Violations,
				"contains quotation marks: speech recognition output is never exact enough to quote")
		}
		out = append(out, v)
	}
	return out
}

// numbersIn mengumpulkan seluruh deret angka dalam teks.
//
// Angka dipilih sebagai satu-satunya pagar isi yang otomatis karena ia satu-
// satunya yang bisa diperiksa tanpa menebak: "3 juta" yang tidak pernah
// diucapkan adalah kesalahan, titik. Klaim yang mengada-ada tanpa angka tetap
// urusan mata manusia — itulah gunanya transkrip ikut ditulis ke berkasnya.
func numbersIn(s string) []string {
	var out []string
	cur := strings.Builder{}
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		if unicode.IsDigit(r) {
			cur.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

// cleanTags membuang '#', spasi, dan tagar kembar.
func cleanTags(tags []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(t), "#"))
		t = strings.ReplaceAll(t, " ", "")
		if t == "" || seen[strings.ToLower(t)] {
			continue
		}
		seen[strings.ToLower(t)] = true
		out = append(out, t)
	}
	return out
}

// limitWords memotong transkrip ke jatah kata yang dikirim ke model.
func limitWords(s string, max int) string {
	if max <= 0 {
		return s
	}
	fields := strings.Fields(s)
	if len(fields) <= max {
		return s
	}
	return strings.Join(fields[:max], " ")
}

// langName menerjemahkan kode bahasa jadi nama yang dimengerti model.
func langName(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "", "id":
		return "Indonesian"
	case "en":
		return "English"
	}
	return code
}
