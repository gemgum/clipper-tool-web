package writer

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gemgum/clipper/engine/internal/news"
	"github.com/gemgum/clipper/engine/internal/score/ollama"
)

// sample2 = artikel kedua tentang peristiwa yang sama dari media lain. Ada
// fakta yang bertindih dan ada yang baru — persis bahan tahap 2.
var sample2 = news.Content{
	Article: news.Article{
		Title:  "Gempa Bantul, BPBD DIY catat kerusakan di dua kecamatan",
		URL:    "https://contoh2.id/berita/9",
		Source: "Contoh Dua",
	},
	Paragraphs: []news.Paragraph{
		{Index: 0, Text: "Gempa bermagnitudo 5,2 yang mengguncang Bantul pada Senin dini hari menyebabkan kerusakan di dua kecamatan, menurut Badan Penanggulangan Bencana Daerah Daerah Istimewa Yogyakarta."},
		{Index: 1, Text: "Selain Kecamatan Kretek, kerusakan ringan juga dilaporkan terjadi pada tiga rumah warga di Kecamatan Sanden."},
		{Index: 2, Text: "Bupati Bantul Abdul Halim Muslih meninjau lokasi terdampak pada Senin siang dan menyatakan pendataan masih berlangsung."},
		{Index: 3, Text: "Sejumlah warga sempat keluar rumah saat guncangan terjadi, namun aktivitas kembali normal pada pagi hari."},
		{Index: 4, Text: "BPBD DIY menyatakan belum menerima laporan kerusakan fasilitas umum maupun jalur transportasi."},
	},
	WordCount: 95,
}

func sources() []Source {
	return []Source{
		{Article: sample, Facts: FactSheet{URL: sample.Article.URL, Source: "Contoh", Facts: []Fact{
			{Text: "Gempa magnitudo 5,2 mengguncang Kabupaten Bantul pada Senin pukul 03.14 WIB", Paragraph: 0},
			{Text: "Pusat gempa di laut pada kedalaman 24 kilometer dan tidak berpotensi tsunami", Paragraph: 1},
			{Text: "Tujuh rumah rusak ringan di Kecamatan Kretek dan tidak ada korban jiwa", Paragraph: 4},
		}}},
		{Article: sample2, Facts: FactSheet{URL: sample2.Article.URL, Source: "Contoh Dua", Facts: []Fact{
			{Text: "Tiga rumah rusak ringan di Kecamatan Sanden", Paragraph: 1},
			{Text: "Bupati Bantul Abdul Halim Muslih meninjau lokasi terdampak pada Senin siang", Paragraph: 2},
		}}},
	}
}

// TestInspectMenangkapAngkaKarangan — kesalahan paling mahal di berita.
func TestInspectMenangkapAngkaKarangan(t *testing.T) {
	d := Draft{
		Title: "Gempa Bantul",
		Lead:  "Gempa magnitudo 5,2 mengguncang Bantul.",
		Body:  []string{"Sebanyak 19 rumah dilaporkan rusak di Kecamatan Kretek."},
	}
	vs := inspect(d, sources())
	if !hasViolation(vs, "number", "19") {
		t.Errorf("angka karangan 19 tidak tertangkap: %+v", vs)
	}
	// 5,2 ada di sumber dan tidak boleh dilaporkan.
	if hasViolation(vs, "number", "5,2") {
		t.Errorf("angka yang sah (5,2) ikut dilaporkan: %+v", vs)
	}
}

// TestInspectMenangkapKutipanPalsu — kutipan yang tidak pernah diucapkan
// adalah kesalahan paling merusak kepercayaan.
func TestInspectMenangkapKutipanPalsu(t *testing.T) {
	d := Draft{
		Body: []string{`Bupati menyatakan, "kami akan membangun kembali seluruh rumah warga".`},
	}
	if vs := inspect(d, sources()); !hasKind(vs, "quote") {
		t.Errorf("kutipan palsu tidak tertangkap: %+v", vs)
	}
	// Kutipan yang PERSIS ada di artikel sumber tidak boleh dilaporkan.
	d2 := Draft{
		Body: []string{`Ia berkata, "pendataan masih berlangsung" seusai peninjauan.`},
	}
	if vs := inspect(d2, sources()); hasKind(vs, "quote") {
		t.Errorf("kutipan yang ada di sumber ikut dilaporkan: %+v", vs)
	}
}

// TestInspectNamaDiri: nama karangan tertangkap, kata awal kalimat tidak.
func TestInspectNamaDiri(t *testing.T) {
	d := Draft{
		Body: []string{
			"Sementara itu, Gubernur Prabowo Subianto meninjau lokasi.",
			"Menurut BPBD DIY, pendataan masih berlangsung.",
		},
	}
	vs := inspect(d, sources())
	if !hasViolation(vs, "name", "Prabowo") {
		t.Errorf("nama karangan tidak tertangkap: %+v", vs)
	}
	for _, w := range []string{"Sementara", "Menurut"} {
		if hasViolation(vs, "name", w) {
			t.Errorf("kata awal kalimat %q dikira nama: %+v", w, vs)
		}
	}
}

// TestInspectPetaKlaim: nomor yang menunjuk ke sumber/paragraf yang tidak ada
// membuat sumber.json menyesatkan — lebih buruk daripada tidak ada peta.
func TestInspectPetaKlaim(t *testing.T) {
	d := Draft{
		Body: []string{"Tujuh rumah rusak ringan."},
		Claims: []Claim{
			{Text: "Tujuh rumah rusak ringan", Source: 0, Paragraph: 4}, // sah
			{Text: "Entah dari mana", Source: 9, Paragraph: 0},          // sumber tak ada
			{Text: "Entah paragraf mana", Source: 1, Paragraph: 77},     // paragraf tak ada
		},
	}
	vs := inspect(d, sources())
	if n := countKind(vs, "claim"); n != 2 {
		t.Errorf("pelanggaran claim = %d, mau 2: %+v", n, vs)
	}
}

// TestInspectCakupanSumber menjaga premis fiturnya: lima artikel yang menyusut
// jadi satu adalah kegagalan yang tidak kelihatan dari artikelnya.
func TestInspectCakupanSumber(t *testing.T) {
	// Hanya memakai sumber 0; sumber 1 (Sanden, Bupati) diabaikan.
	only0 := Draft{
		Lead: "Gempa magnitudo 5,2 mengguncang Bantul.",
		Body: []string{"Tujuh rumah rusak ringan di Kecamatan Kretek, tidak ada korban jiwa."},
	}
	if vs := inspect(only0, sources()); !hasKind(vs, "coverage") {
		t.Errorf("sumber yang tidak terpakai tidak tertangkap: %+v", vs)
	}
	// Memakai keduanya.
	both := Draft{
		Lead: "Gempa magnitudo 5,2 mengguncang Bantul.",
		Body: []string{
			"Tujuh rumah rusak ringan di Kecamatan Kretek.",
			"Tiga rumah lain rusak di Kecamatan Sanden, kata Bupati Abdul Halim Muslih.",
		},
	}
	if vs := inspect(both, sources()); hasKind(vs, "coverage") {
		t.Errorf("kedua sumber terpakai tetapi tetap dilaporkan: %+v", vs)
	}
}

// TestInspectPengulangan menjaga temuan uji ujung-ke-ujung: diminta 400 kata
// padahal bahannya 120, llama3.1 mengejar targetnya dengan menyalin paragrafnya
// sendiri — panjangnya "tercapai" tanpa satu pun fakta baru.
func TestInspectPengulangan(t *testing.T) {
	sama := "Gempa berkekuatan magnitudo 5,2 mengguncang wilayah Kabupaten Bantul pada Senin pukul 03.14 WIB."
	d := Draft{
		Lead: sama,
		Body: []string{sama, "Tujuh rumah rusak ringan di Kecamatan Kretek."},
	}
	if n := countKind(inspect(d, sources()), "repetition"); n == 0 {
		t.Errorf("paragraf yang mengulang lead tidak tertangkap")
	}
	// Dua paragraf yang membahas hal sama dengan kata berbeda tetap sah.
	beda := Draft{
		Lead: "Gempa magnitudo 5,2 mengguncang Bantul pada Senin dini hari.",
		Body: []string{
			"Tujuh rumah rusak ringan di Kecamatan Kretek dan tidak ada korban jiwa.",
			"Bupati Bantul Abdul Halim Muslih meninjau lokasi terdampak pada Senin siang.",
		},
	}
	if n := countKind(inspect(beda, sources()), "repetition"); n != 0 {
		t.Errorf("paragraf yang berbeda dilaporkan mengulang: %+v", inspect(beda, sources()))
	}
}

// TestComposeTidakPernahGagalKarenaPagar menjaga kebijakan notes/38: melanggar
// pagar berarti ditandai, bukan job digagalkan.
func TestComposeTidakPernahGagalKarenaPagar(t *testing.T) {
	// Model membandel: tetap mengarang angka di kedua percobaan.
	reply := `{"title":"Gempa","lead":"Gempa mengguncang Bantul.","body":["Sebanyak 19 rumah rusak."]}`
	d, err := Compose(context.Background(), sources(), fakeCompleter(reply), "uji")
	if err != nil {
		t.Fatalf("Compose tidak boleh gagal karena pagar: %v", err)
	}
	if d.Title == "" || len(d.Body) == 0 {
		t.Fatal("artikel harus tetap keluar walau melanggar")
	}
	if !d.Repaired {
		t.Error("Repaired = false, mau true — percobaan kedua harus tercatat")
	}
	if !hasViolation(d.Violations, "number", "19") {
		t.Errorf("pelanggaran harus ikut ke keluaran: %+v", d.Violations)
	}
}

// TestComposeMemakaiPerbaikanHanyaBilaLebihBaik: percobaan kedua tidak boleh
// membuat hasilnya lebih buruk.
func TestComposeMemakaiPerbaikanHanyaBilaLebihBaik(t *testing.T) {
	n := 0
	complete := func(ctx context.Context, system, user string) (string, error) {
		n++
		if n == 1 {
			// satu pelanggaran: angka 19
			return `{"title":"A","lead":"L","body":["Sebanyak 19 rumah rusak di Kecamatan Kretek."]}`, nil
		}
		// dua pelanggaran: angka 19 dan 42
		return `{"title":"A","lead":"L","body":["Sebanyak 19 rumah rusak, 42 warga mengungsi."]}`, nil
	}
	d, err := Compose(context.Background(), sources(), complete, "uji")
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if hasViolation(d.Violations, "number", "42") {
		t.Errorf("draf kedua yang lebih buruk ikut dipakai: %+v", d.Violations)
	}
}

// TestComposeMembuangParagrafYangMengulang: draf yang tujuh paragraf
// terakhirnya salinan hampir tidak berguna buat redaktur — dan membuang
// salinan adalah operasi mekanis tanpa penilaian.
func TestComposeMembuangParagrafYangMengulang(t *testing.T) {
	sama := "Tujuh rumah rusak ringan di Kecamatan Kretek dan tidak ada korban jiwa sama sekali."
	reply := `{"title":"Gempa Bantul","lead":"Gempa magnitudo 5,2 mengguncang Bantul pada Senin.",
		"body":["` + sama + `","Bupati Bantul Abdul Halim Muslih meninjau lokasi terdampak.","` + sama + `"]}`
	d, err := Compose(context.Background(), sources(), fakeCompleter(reply), "uji")
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if len(d.Body) != 2 {
		t.Errorf("paragraf tersisa = %d, mau 2: %+v", len(d.Body), d.Body)
	}
	// Yang dibuang harus tetap tercatat — tidak ada yang disembunyikan.
	var found bool
	for _, v := range d.Violations {
		if v.Kind == "repetition" && strings.Contains(v.Detail, "removed") {
			found = true
		}
	}
	if !found {
		t.Errorf("pembuangan tidak dicatat: %+v", d.Violations)
	}
	if d.Words != len(strings.Fields(strings.Join(d.Body, " "))) {
		t.Errorf("jumlah kata tidak dihitung ulang setelah pembuangan: %d", d.Words)
	}
}

// TestComposeTidakMenghabiskanArtikel menjaga temuan lapangan 17 Agustus 2026:
// badan 3 paragraf yang paragraf pertamanya menyalin lead keluar sebagai artikel
// DUA KATA, sebab pembuangan diterapkan tanpa melihat apa yang tersisa. Pagar
// boleh melaporkan, tidak boleh membuat lebih buruk.
func TestComposeTidakMenghabiskanArtikel(t *testing.T) {
	lead := "Gempa magnitudo 5,2 mengguncang Bantul pada Senin pagi dan tujuh rumah rusak ringan."
	reply := `{"title":"Gempa Bantul","lead":"` + lead + `",
		"body":["` + lead + `","claims","dua kata"]}`
	d, err := Compose(context.Background(), sources(), fakeCompleter(reply), "uji")
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if len(d.Body) != 3 {
		t.Errorf("badan artikel dipangkas jadi %d paragraf: %+v", len(d.Body), d.Body)
	}
	// Dilaporkan, ya. Disembunyikan dengan cara dibuang, tidak.
	var reported bool
	for _, v := range d.Violations {
		if v.Kind == "repetition" {
			reported = true
			if strings.Contains(v.Detail, "removed") {
				t.Errorf("mengaku membuang padahal tidak: %+v", v)
			}
		}
	}
	if !reported {
		t.Errorf("pengulangan tidak dilaporkan sama sekali: %+v", d.Violations)
	}
}

// TestComposeMembuangPenandaSumber menjaga temuan uji: llama3.1 menyalin
// penanda [sumber/paragraf] ke badan artikel, dan nomornya lalu terbaca pagar
// sebagai angka karangan — satu artikel bersih dilaporkan melanggar enam kali.
func TestComposeMembuangPenandaSumber(t *testing.T) {
	reply := `{"title":"Gempa Bantul [0/0]","lead":"Gempa mengguncang Bantul. [0/0]",
		"body":["[0/4] Tujuh rumah rusak ringan di Kecamatan Kretek."]}`
	d, err := Compose(context.Background(), sources(), fakeCompleter(reply), "uji")
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	for _, s := range append([]string{d.Title, d.Lead}, d.Body...) {
		if strings.Contains(s, "[0/") {
			t.Errorf("penanda sumber masih ada di teks: %q", s)
		}
	}
	for _, v := range d.Violations {
		if v.Kind == "number" {
			t.Errorf("nomor penanda terbaca sebagai angka karangan: %+v", v)
		}
	}
}

// TestComposeTidakMengirimBadanArtikel: seluruh gunanya tahap 1 adalah supaya
// tahap 2 tidak perlu memuat artikel penuh.
func TestComposeTidakMengirimBadanArtikel(t *testing.T) {
	var seen string
	complete := func(ctx context.Context, system, user string) (string, error) {
		seen = user
		return `{"title":"A","lead":"L","body":["B"]}`, nil
	}
	if _, err := Compose(context.Background(), sources(), complete, "uji"); err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if strings.Contains(seen, "Sejumlah warga sempat keluar rumah") {
		t.Errorf("badan artikel sumber ikut terkirim ke tahap 2:\n%s", seen)
	}
}

// TestComposeLive menjalankan tahap 1 + tahap 2 berurutan terhadap Ollama.
//
//	CLIPPER_TEST_LIVE=1 go test ./internal/writer/ -run ComposeLive -v
func TestComposeLive(t *testing.T) {
	if os.Getenv("CLIPPER_TEST_LIVE") == "" {
		t.Skip("set CLIPPER_TEST_LIVE=1 untuk menjalankan uji ini terhadap Ollama")
	}
	model := os.Getenv("CLIPPER_TEST_MODEL")
	if model == "" {
		model = "llama3.1"
	}
	c := ollama.New(ollama.Discover(context.Background()), model)
	c.Temperature = 0.1
	ctx := context.Background()

	var srcs []Source
	for _, content := range []news.Content{sample, sample2} {
		facts := func(ctx context.Context, system, user string) (string, error) {
			return c.Complete(ctx, system, user, FactsSchema(), 2048)
		}
		sheet, err := ExtractFacts(ctx, content, facts, model, 0)
		if err != nil {
			t.Fatalf("ExtractFacts(%s): %v", content.Article.Source, err)
		}
		srcs = append(srcs, Source{Article: content, Facts: sheet})
	}

	compose := func(ctx context.Context, system, user string) (string, error) {
		return c.Complete(ctx, system, user, ComposeSchema(), 3072)
	}
	draft, err := Compose(ctx, srcs, compose, model)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	b, _ := json.MarshalIndent(draft, "", "  ")
	t.Logf("model: %s\n%s", model, b)

	if draft.Title == "" || len(draft.Body) == 0 {
		t.Fatal("artikel kosong")
	}

	// Tahap 3 ikut dijalankan supaya rantainya teruji utuh: yang menentukan
	// fitur ini berguna bukan draf di memori, melainkan berkas yang bisa
	// disalin redaktur.
	post, err := Save(ctx, t.TempDir(), draft, srcs, "id", time.Now())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	md, err := os.ReadFile(post.Markdown)
	if err != nil {
		t.Fatalf("baca artikel.md: %v", err)
	}
	t.Logf("artikel.md:\n%s", md)
	if post.ImageNote != "" {
		t.Logf("gambar: %s", post.ImageNote)
	}
	// Pelanggaran yang berarti fakta karangan. Panjang sengaja tidak diuji:
	// dua artikel tiruan yang pendek memang tidak menyediakan 400 kata bahan.
	for _, v := range draft.Violations {
		if v.Kind == "number" || v.Kind == "quote" || v.Kind == "name" {
			t.Errorf("fakta karangan lolos ke draf: %s %q — %s", v.Kind, v.Text, v.Detail)
		}
	}
}

func hasViolation(vs []Violation, kind, text string) bool {
	for _, v := range vs {
		if v.Kind == kind && v.Text == text {
			return true
		}
	}
	return false
}

func hasKind(vs []Violation, kind string) bool { return countKind(vs, kind) > 0 }

func countKind(vs []Violation, kind string) int {
	n := 0
	for _, v := range vs {
		if v.Kind == kind {
			n++
		}
	}
	return n
}

// TestBetterTidakMemenangkanDrafKosong menjaga temuan 18 Agustus 2026: memilih
// draf dengan pelanggaran TERSEDIKIT memberi kemenangan pada draf yang paling
// kosong, sebab artikel 35 kata nyaris tidak punya apa-apa untuk dilanggar.
// Akibatnya perbaikan yang berhasil dibuang dan hasilnya makin pendek.
func TestBetterTidakMemenangkanDrafKosong(t *testing.T) {
	pendek := Draft{Words: 35, Violations: make([]Violation, 2)}
	panjang := Draft{Words: 520, Violations: make([]Violation, 7)}
	if better(pendek, panjang) {
		t.Error("draf 35 kata menang atas 520 kata — pelanggaran dihitung, panjang tidak")
	}
	if !better(panjang, pendek) {
		t.Error("draf yang panjangnya masuk akal harus menang")
	}

	// Sama-sama kependekan: yang lebih panjang memakai lebih banyak bahan.
	if !better(Draft{Words: 300, Violations: make([]Violation, 5)},
		Draft{Words: 100, Violations: make([]Violation, 3)}) {
		t.Error("di antara dua draf kependekan, yang lebih panjang harus menang")
	}

	// Panjang setara → barulah jumlah pelanggaran yang menentukan.
	if !better(Draft{Words: 500, Violations: make([]Violation, 1)},
		Draft{Words: 600, Violations: make([]Violation, 4)}) {
		t.Error("panjang setara: pelanggaran tersedikit harus menang")
	}
}

// TestLanguageOfDanPenandaDaftar menjaga dua temuan 18 Agustus 2026: llama3.1
// menerjemahkan artikel Antara ke bahasa Inggris (dan pagar nama lalu
// melaporkan "Difference", "August", "President" sebagai nama karangan), serta
// menulis tiap paragraf berawalan "- " walau yang diminta paragraf.
func TestLanguageOfDanPenandaDaftar(t *testing.T) {
	if got := languageOf(sources()); got != "Indonesian" {
		t.Errorf("languageOf = %q, mau Indonesian", got)
	}
	en := []Source{{Facts: FactSheet{Facts: []Fact{
		{Text: "The earthquake was recorded at a depth of 24 kilometres and is not expected to trigger a tsunami"},
	}}}}
	if got := languageOf(en); got != "English" {
		t.Errorf("languageOf = %q, mau English", got)
	}

	reply := `{"title":"Gempa","lead":"Gempa mengguncang Bantul.",
		"body":["- Tujuh rumah rusak ringan di Kecamatan Kretek.","2) Bupati meninjau lokasi."]}`
	d, err := Compose(context.Background(), sources(), fakeCompleter(reply), "uji")
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	for _, p := range d.Body {
		if bulletRe.MatchString(p) {
			t.Errorf("penanda daftar masih ada: %q", p)
		}
	}
}
