package correct

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/types"
)

// transcript membangun transkrip sintetis dari daftar teks, tiap segmen 3 detik
// dan lengkap dengan timestamp per kata.
func transcript(texts ...string) types.Transcript {
	var tr types.Transcript
	tr.Language = "id"
	for i, text := range texts {
		start := float64(i) * 3
		seg := types.TranscriptSegment{Start: start, End: start + 3, Text: text}
		fields := strings.Fields(text)
		per := 3.0 / float64(max(1, len(fields)))
		t := start
		for _, f := range fields {
			seg.Words = append(seg.Words, types.Word{Start: t, End: t + per, Text: f})
			t += per
		}
		tr.Segments = append(tr.Segments, seg)
	}
	return tr
}

// replyWith membangun Completer palsu yang membalas teks tertentu untuk indeks
// yang diminta. fn menerima indeks & teks asli, mengembalikan teks balasan.
func replyWith(fn func(index int, original string) string) Completer {
	return func(ctx context.Context, system, user string, schema any) (string, error) {
		type item struct {
			Index int    `json:"index"`
			Text  string `json:"text"`
		}
		var items []item
		for _, line := range strings.Split(user, "\n") {
			if !strings.HasPrefix(line, "[") {
				continue
			}
			var idx int
			var rest string
			if _, err := fmt.Sscanf(line, "[%d] ", &idx); err != nil {
				continue
			}
			rest = line[strings.Index(line, "] ")+2:]
			items = append(items, item{Index: idx, Text: fn(idx, rest)})
		}
		out, _ := json.Marshal(map[string]any{"segments": items})
		return string(out), nil
	}
}

func TestCorrectAppliesPunctuationAndKeepsTiming(t *testing.T) {
	tr := transcript("- dari kemarin harusnya digeledah", "iya dong")
	fixed, report, err := Correct(context.Background(), tr, nil,
		replyWith(func(i int, original string) string {
			if i == 0 {
				return "Dari kemarin harusnya digeledah."
			}
			return "Iya dong."
		}), "uji", nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Changed != 2 {
		t.Errorf("changed = %d, ingin 2", report.Changed)
	}
	if fixed.Segments[0].Text != "Dari kemarin harusnya digeledah." {
		t.Errorf("teks = %q", fixed.Segments[0].Text)
	}
	// Batas segmen tidak boleh ikut bergeser — pipeline memakainya untuk
	// memotong klip.
	for i := range fixed.Segments {
		if fixed.Segments[i].Start != tr.Segments[i].Start || fixed.Segments[i].End != tr.Segments[i].End {
			t.Errorf("batas segmen %d bergeser", i)
		}
	}
	// Kata harus tetap punya timestamp, kalau tidak karaoke rusak.
	if len(fixed.Segments[0].Words) == 0 {
		t.Error("kata bertimestamp hilang setelah koreksi")
	}
}

// Transkrip asli tidak boleh ikut berubah: pemanggil masih memakainya sebagai
// kunci cache dan sebagai pembanding.
func TestCorrectDoesNotMutateInput(t *testing.T) {
	tr := transcript("halo dunia")
	before := tr.Segments[0].Text
	_, _, err := Correct(context.Background(), tr, nil,
		replyWith(func(int, string) string { return "Halo dunia." }), "uji", nil)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Segments[0].Text != before {
		t.Errorf("transkrip masukan ikut berubah jadi %q", tr.Segments[0].Text)
	}
}

// Pagar pengaman: model yang memparafrase harus ditolak, teks asli dipertahankan.
func TestCorrectRejectsParaphrase(t *testing.T) {
	tr := transcript("ada beberapa poin yang lagi rime sekarang")
	fixed, report, err := Correct(context.Background(), tr, nil,
		replyWith(func(int, string) string {
			return "Terdapat sejumlah hal hangat belakangan ini."
		}), "uji", nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Rejected != 1 {
		t.Errorf("rejected = %d, ingin 1", report.Rejected)
	}
	if report.Changed != 0 {
		t.Errorf("changed = %d, ingin 0", report.Changed)
	}
	if fixed.Segments[0].Text != tr.Segments[0].Text {
		t.Errorf("teks asli tidak dipertahankan: %q", fixed.Segments[0].Text)
	}
}

// Pagar pengaman: model yang membuang separuh kalimat juga ditolak.
func TestCorrectRejectsTruncation(t *testing.T) {
	tr := transcript("satu dua tiga empat lima enam tujuh delapan sembilan sepuluh")
	_, report, err := Correct(context.Background(), tr, nil,
		replyWith(func(int, string) string { return "Satu dua." }), "uji", nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Rejected != 1 {
		t.Errorf("rejected = %d, ingin 1 (kalimat terpangkas)", report.Rejected)
	}
}

// Koreksi satu kata di kalimat panjang harus LOLOS — pagar pengaman tidak boleh
// terlalu ketat sampai membunuh koreksi yang sah.
func TestCorrectAcceptsSingleWordFix(t *testing.T) {
	tr := transcript("ada beberapa poin yang lagi rime sekarang dan itu ditanyakan")
	_, report, err := Correct(context.Background(), tr, nil,
		replyWith(func(_ int, original string) string {
			return strings.Replace(original, "rime", "rame", 1)
		}), "uji", nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Changed != 1 || report.Rejected != 0 {
		t.Errorf("changed=%d rejected=%d, ingin 1 dan 0", report.Changed, report.Rejected)
	}
}

// Segmen yang tidak dibalas model dihitung, bukan diam-diam dianggap kosong.
func TestCorrectCountsMissingSegments(t *testing.T) {
	tr := transcript("satu", "dua", "tiga")
	fixed, report, err := Correct(context.Background(), tr, nil,
		func(ctx context.Context, system, user string, schema any) (string, error) {
			// Hanya membalas segmen 0.
			return `{"segments":[{"index":0,"text":"Satu."}]}`, nil
		}, "uji", nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Missing != 2 {
		t.Errorf("missing = %d, ingin 2", report.Missing)
	}
	// Segmen yang terlewat tetap memakai teks aslinya.
	if fixed.Segments[1].Text != "dua" {
		t.Errorf("segmen terlewat berubah jadi %q", fixed.Segments[1].Text)
	}
}

// Balasan model yang tidak bisa dibaca harus menggagalkan job, bukan
// menghasilkan transkrip kosong (notes/12: tanpa fallback senyap).
func TestCorrectFailsOnUnreadableReply(t *testing.T) {
	tr := transcript("satu")
	_, _, err := Correct(context.Background(), tr, nil,
		func(ctx context.Context, system, user string, schema any) (string, error) {
			return "maaf saya tidak mengerti", nil
		}, "Ollama (qwen2.5)", nil)
	if err == nil {
		t.Fatal("balasan rusak seharusnya menggagalkan koreksi")
	}
	if !strings.Contains(err.Error(), "Ollama (qwen2.5)") {
		t.Errorf("pesan tidak menyebut mesinnya: %v", err)
	}
}

// Indeks segmen harus unik & lengkap di seluruh potongan, kalau tidak balasan
// model tidak bisa dipetakan kembali.
func TestBuildChunksCoversEverySegmentExactlyOnce(t *testing.T) {
	texts := make([]string, 200)
	for i := range texts {
		texts[i] = "kalimat panjang berisi delapan kata untuk menguji pemotongan ini"
	}
	tr := transcript(texts...)
	chunks := buildChunks(tr, chunkWords)
	if len(chunks) < 2 {
		t.Fatalf("hanya %d potongan untuk 200 segmen", len(chunks))
	}
	seen := map[int]int{}
	for _, c := range chunks {
		for _, i := range c.segments {
			seen[i]++
		}
	}
	if len(seen) != len(tr.Segments) {
		t.Errorf("tercakup %d segmen, ingin %d", len(seen), len(tr.Segments))
	}
	for i, n := range seen {
		if n != 1 {
			t.Errorf("segmen %d muncul %d kali — indeks harus unik", i, n)
		}
	}
	// Potongan kedua dan seterusnya membawa konteks dari potongan sebelumnya.
	if chunks[1].context == "" {
		t.Error("potongan lanjutan tidak membawa konteks segmen sebelumnya")
	}
}

// Progres dilaporkan per potongan supaya GUI tidak diam selama koreksi panjang.
func TestCorrectReportsProgress(t *testing.T) {
	texts := make([]string, 200)
	for i := range texts {
		texts[i] = "kalimat panjang berisi delapan kata untuk menguji pemotongan ini"
	}
	tr := transcript(texts...)
	calls := 0
	_, _, err := Correct(context.Background(), tr, nil,
		replyWith(func(_ int, original string) string { return original }),
		"uji", func(done, total int) { calls++ })
	if err != nil {
		t.Fatal(err)
	}
	if calls < 2 {
		t.Errorf("progres dilaporkan %d kali, ingin sekali per potongan", calls)
	}
}

// Tanda kutip menandai kutipan langsung. Model sesekali membuangnya atau
// menukarnya jadi kutip tunggal; keduanya harus ditolak, sebab itu bukan
// koreksi dan lolos dari pemeriksaan kemiripan (normalize membuang tanda baca).
func TestCorrectRejectsDroppedQuotationMarks(t *testing.T) {
	const quoted = `"Pak maaf saya bukan Londo, tapi Bapak salah mendirikan URI itu"`

	// Kutip dihapus seluruhnya.
	tr := transcript(quoted)
	_, report, err := Correct(context.Background(), tr, nil,
		replyWith(func(_ int, original string) string {
			return strings.ReplaceAll(original, `"`, "")
		}), "uji", nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Rejected != 1 {
		t.Errorf("kutip dihapus: rejected = %d, ingin 1", report.Rejected)
	}

	// Kutip ditukar jadi kutip tunggal.
	_, report, err = Correct(context.Background(), transcript(quoted), nil,
		replyWith(func(_ int, original string) string {
			return strings.ReplaceAll(original, `"`, "'")
		}), "uji", nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Rejected != 1 {
		t.Errorf("kutip ditukar: rejected = %d, ingin 1", report.Rejected)
	}

	// Menambahkan tanda baca lain di dalam kutipan tetap boleh.
	_, report, err = Correct(context.Background(), transcript(quoted), nil,
		replyWith(func(_ int, original string) string { return original + "." }), "uji", nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Rejected != 0 || report.Changed != 1 {
		t.Errorf("koreksi sah tertolak: changed=%d rejected=%d", report.Changed, report.Rejected)
	}
}

// Model yang tidak sanggup membalas satu potongan penuh harus DILAYANI, bukan
// menggagalkan job: potongannya dipecah sampai ia sanggup.
func TestCorrectSplitsChunkWhenReplyIsEmpty(t *testing.T) {
	tr := types.Transcript{Language: "id", Segments: []types.TranscriptSegment{
		{Start: 0, End: 2, Text: "halo dunia"},
		{Start: 2, End: 4, Text: "apa kabar"},
	}}
	// Kosong untuk permintaan berisi DUA segmen; benar untuk permintaan berisi
	// satu segmen — persis kelakuan model yang kehabisan ruang.
	complete := func(ctx context.Context, system, user string, schema any) (string, error) {
		if strings.Count(user, "[") >= 2 {
			return "", nil
		}
		if strings.Contains(user, "[0]") {
			return `{"segments":[{"index":0,"text":"Halo dunia."}]}`, nil
		}
		return `{"segments":[{"index":1,"text":"Apa kabar?"}]}`, nil
	}
	out, rep, err := Correct(context.Background(), tr, nil, complete, "uji", nil)
	if err != nil {
		t.Fatalf("job gagal padahal pemecahan berhasil: %v", err)
	}
	if rep.Split == 0 {
		t.Fatal("potongan tidak pernah dipecah")
	}
	if rep.Failed != 0 {
		t.Fatalf("ada potongan yang dinyatakan gagal: %+v", rep)
	}
	if out.Segments[0].Text != "Halo dunia." || out.Segments[1].Text != "Apa kabar?" {
		t.Fatalf("hasil koreksi = %q / %q", out.Segments[0].Text, out.Segments[1].Text)
	}
}

// Model yang SELALU membalas kosong, bahkan untuk satu segmen, memang tidak
// sanggup — jobnya harus berhenti dengan pesan yang menyebut balasannya.
func TestCorrectStopsWhenEvenOneSegmentFails(t *testing.T) {
	tr := types.Transcript{Language: "id", Segments: []types.TranscriptSegment{
		{Start: 0, End: 2, Text: "halo dunia"},
		{Start: 2, End: 4, Text: "apa kabar"},
	}}
	always := func(ctx context.Context, system, user string, schema any) (string, error) { return "", nil }
	_, _, err := Correct(context.Background(), tr, nil, always, "uji", nil)
	if err == nil {
		t.Fatal("balasan kosong terus-menerus harus menggagalkan koreksi")
	}
	if !strings.Contains(err.Error(), "(empty)") {
		t.Fatalf("pesan tidak menyebut balasan kosongnya: %v", err)
	}
}
