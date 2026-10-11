// Package correct membenahi transkrip mentah whisper sebelum dipakai pipeline:
// tanda baca, huruf besar, tanda hubung dialog yang bocor, dan kata yang salah
// didengar.
//
// Kenapa perlu: keluaran whisper untuk bahasa Indonesia kerap berbentuk
//
//	"- Dari kemarin harusnya digeledah. - Iya dong."
//	"...berbicara jujur dan terus terang,"   ← kalimat putus di batas segmen
//	"ada beberapa poin yang lagi rime"       ← salah dengar
//
// Bentuk itu ikut terbakar ke subtitle, dan tanda bacanya juga menyesatkan
// segmentasi klip — segment.BuildCandidates memotong di akhir kalimat, jadi
// titik yang salah tempat menghasilkan batas klip yang salah pula.
//
// Batas yang dipegang paket ini: ia MENGOREKSI, bukan menulis ulang. Setiap
// segmen yang kembali dari model diperiksa panjang & kemiripannya terhadap
// aslinya; yang menyimpang terlalu jauh ditolak dan teks aslinya dipertahankan.
// Penolakan itu dilaporkan, tidak disembunyikan (lihat notes/12).
package correct

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gemgum/clipper/engine/internal/types"
)

// PromptVersion ikut jadi bahan kunci cache di pemanggil. Naikkan bila prompt
// atau pagar pengamannya berubah, supaya hasil lama tidak dipakai ulang secara
// keliru.
const PromptVersion = "v4"

// Completer adalah satu panggilan ke LLM. Fungsi, bukan antarmuka, supaya paket
// ini tidak perlu mengenal llm/ollama.
//
// schema diteruskan ke parameter "format" Ollama dan diabaikan oleh Claude.
// Dibuat per panggilan, bukan sekali di awal, karena jumlah entri yang diminta
// berbeda tiap potongan — lihat SegmentsSchema.
type Completer func(ctx context.Context, system, user string, schema any) (string, error)

// Batas pagar pengaman per segmen.
const (
	// maxDrift = selisih jumlah kata yang masih dianggap koreksi. Di atas ini
	// model kemungkinan menyisipkan atau membuang kalimat.
	maxDriftRatio = 0.30
	maxDriftFloor = 3
	// editBudgetDivisor = jatah kata isi yang boleh berubah: satu per sekian
	// kata. Koreksi salah dengar yang sah menyentuh satu-dua kata per segmen;
	// di atas jatah ini model sedang menulis ulang, bukan mengoreksi.
	editBudgetDivisor = 6
)

// Anggaran per permintaan. Koreksi adalah tugas "salin ulang": model harus
// mengeluarkan teks sebanyak yang masuk, jadi potongannya jauh lebih kecil
// daripada potongan pemilihan momen.
//
// Batas jumlah segmen yang menentukan, bukan jumlah kata: diuji dengan qwen2.5,
// satu permintaan berisi 32 segmen hanya dibalas 1 entri, sedangkan potongan
// selusin segmen dibalas lengkap.
const (
	chunkWords    = 400
	chunkSegments = 12
)

// Change mencatat satu segmen yang berubah — dipakai untuk laporan & uji.
type Change struct {
	Index  int     `json:"index"`
	Start  float64 `json:"start"`
	Before string  `json:"before"`
	After  string  `json:"after"`
}

// Report merangkum apa yang dilakukan koreksi pada satu transkrip.
type Report struct {
	Engine   string   `json:"engine"`
	Total    int      `json:"total"`    // segmen yang dikirim ke model
	Changed  int      `json:"changed"`  // segmen yang teksnya berubah
	Rejected int      `json:"rejected"` // koreksi yang ditolak pagar pengaman
	Missing  int      `json:"missing"`  // segmen yang tidak dibalas model
	Failed   int      `json:"failed"`   // potongan yang balasannya tak bisa dibaca
	Split    int      `json:"split"`    // potongan yang dipecah karena balasannya tak terbaca
	Samples  []Change `json:"samples"`  // beberapa contoh untuk log
}

// Berapa kali satu potongan dicoba sebelum menyerah, dan berapa banyak potongan
// yang boleh gagal sebelum seluruh koreksi dinyatakan gagal.
//
// Angkanya ada karena satu laporan nyata: transkrip 85 potongan berhenti di
// potongan ke-25 dengan "unexpected end of JSON input" — balasan model lokal
// yang kosong atau terpotong. Menggagalkan seluruh job berarti membuang 24
// potongan yang sudah benar dan setengah jam kerja, padahal kegagalan model
// lokal semacam ini biasanya sesaat: percobaan kedua berhasil.
//
// Batas 1/4 menjaga agar ini tidak berubah jadi "koreksi dilewati diam-diam"
// (notes/12): kalau modelnya memang tidak sanggup, jobnya tetap berhenti.
const (
	chunkAttempts   = 3
	failedChunkPart = 4 // gagal > total/4 potongan → berhenti
)

// maxSamples membatasi contoh yang disimpan; laporan ini masuk log, bukan
// tempat menyimpan seluruh transkrip untuk kedua kalinya.
const maxSamples = 5

// SegmentsSchema = JSON Schema bentuk balasan untuk potongan berisi n segmen.
//
// minItems DAN maxItems dikunci ke n — inilah yang memaksa model membalas satu
// entri untuk SETIAP segmen. Tanpa itu qwen2.5 membalas satu entri lalu
// berhenti, dan 31 dari 32 segmen tidak pernah terkoreksi. Prompt saja tidak
// cukup; batasannya harus dijamin di sisi decoder.
func SegmentsSchema(n int) map[string]any {
	if n < 1 {
		n = 1
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"segments": map[string]any{
				"type":     "array",
				"minItems": n,
				"maxItems": n,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"index": map[string]any{"type": "integer"},
						"text":  map[string]any{"type": "string"},
					},
					"required": []string{"index", "text"},
				},
			},
		},
		"required": []string{"segments"},
	}
}

// languageNames memetakan kode bahasa ke nama yang dimengerti model.
var languageNames = map[string]string{
	"id": "Indonesian",
	"en": "English",
	"ms": "Malay",
	"jv": "Javanese",
	"su": "Sundanese",
}

func languageName(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return "Indonesian"
	}
	if name, ok := languageNames[code]; ok {
		return name
	}
	return code
}

// systemPrompt menyusun instruksi koreksi. Instruksinya bahasa Inggris, tetapi
// transkripnya WAJIB tetap dalam bahasanya sendiri.
func systemPrompt(contentLang string, terms []string) string {
	lang := languageName(contentLang)
	var b strings.Builder
	b.WriteString(`You clean up an automatic speech-recognition transcript. You are CORRECTING a transcript, not rewriting it.

You are given numbered transcript segments. Return a corrected version of every segment.

HARD RULES:
- Return EXACTLY the same segment indexes you were given, one entry per segment. Never add, drop, merge, or split segments, and never move words from one segment to another.
- Never translate. `)
	fmt.Fprintf(&b, "The transcript is in %s and must stay in %s.\n", lang, lang)
	b.WriteString(`- Never summarise, never paraphrase, never shorten, never add commentary. The speaker's words and meaning must survive intact.
- If a segment is already correct, return it unchanged.

WHAT TO FIX:
1. Punctuation and capitalisation, so each sentence reads properly.
2. Dialogue dashes the recogniser inserted to mark a change of speaker. They are
   not spoken, so remove EVERY one of them — at the start of the segment AND in
   the middle of it. This covers every dash character the recogniser produces:
   the plain hyphen "-", the en dash "–", the em dash "—" and the minus sign "−".
   Keep the words around the dash. For example
   "- Oh ya? - Iya dulu."  becomes  "Oh ya? Iya dulu."
   "− Kita lanjutkan, Pak."  becomes  "Kita lanjutkan, Pak."
3. Sentences that run across two segments: end the earlier segment with a comma or with no punctuation at all (NOT a full stop), and begin the next segment in lower case. A full stop in the wrong place makes the clip get cut in the wrong place.
4. Obvious speech-recognition errors, but ONLY where the intended word is unmistakable from the surrounding context. Prefer the word that was actually spoken. If you are unsure, leave it exactly as it is.

WHAT NOT TO TOUCH:
- Quotation marks that are already in the text: keep every one of them, and keep the exact same character. Do not delete them, do not add new ones, and never swap " for ' or for a typographic quote.
- Names of people, places and organisations you do not recognise. Leave them spelled exactly as transcribed.
- Filler words the speaker actually said ("ya", "kan", "nah"). They are part of how the person talks.
- Words from a REGIONAL LANGUAGE mixed into the speech. Indonesian speakers
  constantly drop in Javanese, Sundanese, Betawi or other local words, and those
  words are what the person actually said. Do not turn them into their national
  equivalent, and do not "fix" their spelling towards a word you recognise.
  For example, the Javanese "ireng" must stay "ireng" — it is not a misheard
  "irang" and it is not to be replaced with "hitam"
- Words with no close neighbour in standard Indonesian. Before you leave an
  unfamiliar word alone, run one check: is there a common Indonesian word that
  differs from it by only one or two letters, AND does the sentence read
  correctly with that word in its place? BOTH must be true. If they are not,
  the word is not a mistake — it is simply a word you do not know, so write it
  exactly as it is and move on. Failing to recognise a word is never, on its
  own, evidence that it was misheard. This check does not apply to the regional
  words covered by the rule above: those stay as they are even when a national
  word sits close to them.
  Leave alone: "ireng" — no Indonesian word sits that close to it, and "irang"
  is not a word either.
  Fix: "numenkelatur" → "nomenklatur" — the letters are transposed, and the
  sentence is about a budget heading, so the intended word is unmistakable..`)

	if len(terms) > 0 {
		// Ditaruh SESUDAH "WHAT NOT TO TOUCH" dan disebut tegas sebagai
		// pengecualiannya: tanpa itu aturan "biarkan kata asing apa adanya" di
		// atas justru melindungi salah dengar yang mau kita perbaiki.
		b.WriteString("\n\nKNOWN TERMS — THE ONE EXCEPTION TO THE RULES ABOVE:\n")
		b.WriteString("These are the correct spellings of names and terms this speaker uses:\n")
		for _, t := range terms {
			fmt.Fprintf(&b, "  - %s\n", t)
		}
		b.WriteString(`If a segment contains something that clearly SOUNDS like one of these but is written differently, replace it with the spelling above. The recogniser does not know these terms, so it writes down the nearest word it does know, and that is exactly the mistake you are fixing here.

The same term is usually misheard in SEVERAL different ways in one transcript, so fix every variant you meet, not just the most common one:
- the words may be joined by a hyphen ("Londo-Irang");
- only ONE word of the term may be wrong ("Londo Iram", "Londo Hirang");
- the term may appear WITHOUT its other words, on its own ("Anda yang Irang" → "Anda yang Ireng"), including when the segment starts or ends mid-sentence.

This overrides the two rules above about regional words and unfamiliar words — but ONLY for close acoustic matches to this list. Never force a listed term onto a word that merely looks similar in writing, never insert a term the speaker did not say, and leave every other unfamiliar word alone.`)
	}

	b.WriteString(`

Reply with VALID JSON ONLY, in exactly this shape:
{"segments":[{"index":<number>,"text":"<corrected text>"}]}`)
	return b.String()
}

// chunk satu potongan segmen yang dikirim dalam satu permintaan.
type chunk struct {
	segments []int  // indeks segmen di dalam transkrip
	context  string // teks segmen sebelum potongan ini, hanya untuk konteks
}

// buildChunks memecah transkrip menurut anggaran kata. Potongannya berurutan
// dan tidak tumpang tindih: indeks segmen harus tetap unik agar balasan model
// bisa dipetakan kembali tanpa ambiguitas.
func buildChunks(tr types.Transcript, budget int) []chunk {
	if budget <= 0 {
		budget = chunkWords
	}
	var chunks []chunk
	var current []int
	words := 0
	for i, s := range tr.Segments {
		n := len(strings.Fields(s.Text))
		if len(current) > 0 && (words+n > budget || len(current) >= chunkSegments) {
			chunks = append(chunks, chunk{segments: current})
			current, words = nil, 0
		}
		current = append(current, i)
		words += n
	}
	if len(current) > 0 {
		chunks = append(chunks, chunk{segments: current})
	}
	// Segmen terakhir sebelum tiap potongan dibawa sebagai konteks baca-saja,
	// supaya model tahu apakah kalimatnya menyambung dari potongan sebelumnya.
	for c := 1; c < len(chunks); c++ {
		prev := chunks[c-1].segments
		chunks[c].context = tr.Segments[prev[len(prev)-1]].Text
	}
	return chunks
}

// userPrompt merangkai satu potongan jadi permintaan.
func userPrompt(tr types.Transcript, c chunk) string {
	var b strings.Builder
	if c.context != "" {
		b.WriteString("PREVIOUS SEGMENT (context only — do NOT return it):\n")
		b.WriteString(c.context)
		b.WriteString("\n\n")
	}
	fmt.Fprintf(&b, "SEGMENTS TO CORRECT — return EXACTLY %d entries, one per segment, keeping these index numbers:\n", len(c.segments))
	for _, i := range c.segments {
		fmt.Fprintf(&b, "[%d] %s\n", i, tr.Segments[i].Text)
	}
	return b.String()
}

// reply = bentuk balasan model.
type reply struct {
	Segments []struct {
		Index int    `json:"index"`
		Text  string `json:"text"`
	} `json:"segments"`
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

// acceptable memutuskan apakah satu koreksi layak dipakai.
//
// Inilah pagar yang membedakan "koreksi" dari "karangan": model boleh membenahi
// kata, tapi tidak boleh mengganti isinya. Alasan penolakan dikembalikan supaya
// bisa dicatat, bukan dibuang diam-diam.
func acceptable(before, after string, exempt map[string]bool) (bool, string) {
	after = strings.TrimSpace(after)
	if after == "" {
		return false, "empty"
	}
	oldWords := strings.Fields(before)
	newWords := strings.Fields(after)
	if len(oldWords) == 0 {
		return true, ""
	}
	drift := len(newWords) - len(oldWords)
	if drift < 0 {
		drift = -drift
	}
	limit := int(float64(len(oldWords)) * maxDriftRatio)
	if limit < maxDriftFloor {
		limit = maxDriftFloor
	}
	if drift > limit {
		return false, fmt.Sprintf("word count moved by %d (limit %d)", drift, limit)
	}
	// Pagar utama: berapa kata ISI yang berubah. Tanda hubung dialog tidak
	// dihitung, sebab membuangnya justru tugas koreksi.
	changed, total := contentEdits(oldWords, newWords, exempt)
	budget := total / editBudgetDivisor
	if budget < 1 {
		budget = 1
	}
	if changed > budget {
		return false, fmt.Sprintf("%d of %d content words changed (budget %d)", changed, total, budget)
	}
	// Tanda kutip membawa makna: ia menandai bahwa kalimat itu kutipan langsung.
	// Membuangnya bukan koreksi. Diuji dengan qwen2.5, model memang sesekali
	// menghapus sepasang kutip atau menukarnya jadi kutip tunggal — dan itu
	// lolos dari pemeriksaan kemiripan, sebab normalize() membuang tanda baca.
	if before, after := countQuotes(before), countQuotes(after); after < before {
		return false, fmt.Sprintf("dropped %d quotation mark(s)", before-after)
	}
	return true, ""
}

// countQuotes menghitung tanda kutip ganda, termasuk bentuk tipografisnya.
// Kutip tunggal sengaja tidak dihitung: di teks Indonesia ia lebih sering jadi
// apostrof di dalam kata daripada penanda kutipan.
func countQuotes(s string) int {
	n := 0
	for _, r := range s {
		if r == '"' || r == '“' || r == '”' {
			n++
		}
	}
	return n
}

// Progress dilaporkan per potongan supaya GUI tidak diam selama koreksi.
type Progress func(done, total int)

// Correct menjalankan koreksi atas seluruh transkrip.
//
// Kegagalan mesin dikembalikan apa adanya — TIDAK ada perpindahan diam-diam ke
// mesin lain, dan koreksi tidak pernah dilewati diam-diam (notes/12). Yang
// boleh gagal senyap hanyalah koreksi PER SEGMEN yang ditolak pagar pengaman,
// dan itu pun dihitung di Report.
func Correct(ctx context.Context, tr types.Transcript, terms []string, complete Completer, engineName string, onProgress Progress) (types.Transcript, Report, error) {
	report := Report{Engine: engineName, Total: len(tr.Segments)}
	if len(tr.Segments) == 0 {
		return tr, report, nil
	}

	// Salinan dalam: transkrip masukan tidak boleh ikut berubah, sebab yang
	// asli masih dipakai sebagai kunci cache oleh pemanggil.
	out := types.Transcript{Language: tr.Language}
	out.Segments = make([]types.TranscriptSegment, len(tr.Segments))
	copy(out.Segments, tr.Segments)

	system := systemPrompt(tr.Language, terms)
	exempt := termWords(terms)
	chunks := buildChunks(tr, chunkWords)

	// Antrian, bukan perulangan biasa: potongan yang balasannya tak terbaca
	// DIPECAH DUA dan dicoba lagi, jadi daftar kerjanya bisa bertambah di
	// tengah jalan. Itu cara menyesuaikan diri dengan kapasitas model yang
	// sebenarnya tanpa menebak-nebak jumlah tokennya: model yang tidak sanggup
	// membalas 12 segmen sekaligus hampir selalu sanggup membalas 6, dan
	// pemecahan berhenti sendiri di 1 segmen.
	queue := append([]chunk(nil), chunks...)
	total := len(chunks)
	for ci := 0; ci < len(queue); ci++ {
		c := queue[ci]
		var r reply
		var readErr error
		var lastRaw string
		for attempt := 1; attempt <= chunkAttempts; attempt++ {
			raw, err := complete(ctx, system, userPrompt(tr, c), SegmentsSchema(len(c.segments)))
			if err != nil {
				return types.Transcript{}, report, fmt.Errorf("part %d of %d: %w", ci+1, total, err)
			}
			lastRaw = raw
			r = reply{}
			if readErr = json.Unmarshal([]byte(extractJSON(raw)), &r); readErr == nil {
				break
			}
			// Balasan KOSONG tidak akan berubah kalau permintaannya sama
			// persis: percobaan kedua hanya menghabiskan waktu. Yang menolong
			// adalah permintaan yang lebih kecil, jadi langsung ke sana.
			if strings.TrimSpace(lastRaw) == "" && len(c.segments) > 1 {
				break
			}
		}
		if readErr != nil {
			if len(c.segments) > 1 {
				// Dipecah dua dan dikerjakan ulang. Potongan pecahan mewarisi
				// konteksnya supaya kalimat yang menyambung tetap terbaca.
				half := len(c.segments) / 2
				left := chunk{segments: c.segments[:half], context: c.context}
				right := chunk{segments: c.segments[half:], context: tr.Segments[c.segments[half-1]].Text}
				queue = append(queue, left, right)
				total += 2
				report.Split++
				continue
			}
			// Satu segmen pun tidak terbaca: dilewati, teks aslinya
			// dipertahankan, dan itu DIHITUNG — tidak ada yang hilang diam-diam.
			report.Failed++
			report.Missing += len(c.segments)
			if report.Failed*failedChunkPart > total {
				return types.Transcript{}, report, fmt.Errorf(
					"%s could not return readable JSON for %d of %d parts, even after splitting them down to a single segment (last reply: %s). "+
						"The model is most likely too small for this task: llama3.1 (8B) is the smallest that handles it reliably here",
					engineName, report.Failed, total, describeReply(lastRaw))
			}
			if onProgress != nil {
				onProgress(ci+1, total)
			}
			continue
		}

		// Balasan dipetakan lewat indeks, bukan urutan: model kadang mengubah
		// urutan atau melewatkan satu entri.
		got := map[int]string{}
		for _, item := range r.Segments {
			got[item.Index] = item.Text
		}
		for _, i := range c.segments {
			text, ok := got[i]
			if !ok {
				report.Missing++
				continue
			}
			applyCorrection(&out.Segments[i], tr.Segments[i], text, exempt, &report)
		}

		if onProgress != nil {
			onProgress(ci+1, total)
		}
	}
	return out, report, nil
}

// applyCorrection memasang satu koreksi ke segmen, lengkap dengan penyejajaran
// ulang timestamp per kata.
func applyCorrection(dst *types.TranscriptSegment, src types.TranscriptSegment, text string, exempt map[string]bool, report *Report) {
	text = strings.TrimSpace(text)
	if text == src.Text {
		return
	}
	if ok, _ := acceptable(src.Text, text, exempt); !ok {
		report.Rejected++
		return // teks asli dipertahankan
	}
	dst.Text = text
	dst.Words = retime(src.WordList(), text, src.Start, src.End)
	report.Changed++
	if len(report.Samples) < maxSamples {
		report.Samples = append(report.Samples, Change{
			Index: len(report.Samples), Start: src.Start, Before: src.Text, After: text,
		})
	}
}

// Summary merangkum laporan jadi satu baris untuk log & progress.
func (r Report) Summary() string {
	s := fmt.Sprintf("%s corrected %d of %d segments", r.Engine, r.Changed, r.Total)
	if r.Rejected > 0 {
		s += fmt.Sprintf("; %d correction(s) rejected as rewrites", r.Rejected)
	}
	if r.Missing > 0 {
		s += fmt.Sprintf("; %d segment(s) not returned by the model", r.Missing)
	}
	if r.Split > 0 {
		s += fmt.Sprintf("; %d part(s) split into smaller pieces for this model", r.Split)
	}
	if r.Failed > 0 {
		s += fmt.Sprintf("; %d part(s) skipped after unreadable replies", r.Failed)
	}
	return s
}

// describeReply membuat balasan model bisa dibaca di pesan galat.
//
// Balasan KOSONG adalah kasus yang paling sering dan paling membingungkan:
// tanpa keterangan ini pesannya cuma "unexpected end of JSON input", yang
// terbaca seperti bug engine padahal modelnya memang tidak membalas apa pun.
func describeReply(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "(empty)"
	}
	if len(trimmed) > 200 {
		return trimmed[:200] + "…"
	}
	return trimmed
}
