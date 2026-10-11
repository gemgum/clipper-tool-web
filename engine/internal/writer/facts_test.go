package writer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/news"
	"github.com/gemgum/clipper/engine/internal/score/ollama"
)

// sample = artikel tiruan bergaya berita Indonesia: ada angka, nama orang,
// nama lembaga, dan satu kutipan langsung. Cukup untuk menguji apakah model
// benar-benar membaca badannya, tanpa bergantung pada jaringan.
var sample = news.Content{
	Article: news.Article{
		Title:  "Judul ini tidak boleh sampai ke model",
		URL:    "https://contoh.id/berita/1",
		Source: "Contoh",
	},
	Paragraphs: []news.Paragraph{
		{Index: 0, Text: "Badan Meteorologi, Klimatologi, dan Geofisika mencatat gempa berkekuatan magnitudo 5,2 mengguncang wilayah Kabupaten Bantul pada Senin pukul 03.14 WIB."},
		{Index: 1, Text: "Pusat gempa berada di laut pada kedalaman 24 kilometer, sekitar 78 kilometer arah barat daya Bantul, dan tidak berpotensi tsunami."},
		{Index: 2, Text: "Kepala Pusat Gempa Bumi dan Tsunami BMKG, Daryono, mengatakan guncangan dirasakan hingga Yogyakarta dan Purworejo dengan skala intensitas III MMI."},
		{Index: 3, Text: "\"Masyarakat diimbau tetap tenang dan tidak terpancing isu yang tidak dapat dipertanggungjawabkan kebenarannya,\" kata Daryono dalam keterangan tertulis."},
		{Index: 4, Text: "Badan Penanggulangan Bencana Daerah Bantul melaporkan tujuh rumah mengalami kerusakan ringan di Kecamatan Kretek dan tidak ada korban jiwa."},
		{Index: 5, Text: "Hingga pukul 06.00 WIB, BMKG mencatat empat kali gempa susulan dengan magnitudo terbesar 3,1."},
		{Index: 6, Text: "Baca juga: Daftar gempa merusak sepanjang tahun ini di Pulau Jawa."},
	},
	WordCount: 120,
}

// fakeCompleter mengembalikan balasan yang sudah disiapkan, tanpa jaringan.
func fakeCompleter(reply string) news.Completer {
	return func(ctx context.Context, system, user string) (string, error) {
		return reply, nil
	}
}

// TestExtractFactsTidakMengirimJudul menjaga janji inti tahap 1: model bekerja
// dari badan artikel saja. Kalau judul bocor ke prompt, model bisa menyalinnya
// dan seolah-olah "membaca" padahal tidak.
func TestExtractFactsTidakMengirimJudul(t *testing.T) {
	var seen string
	complete := func(ctx context.Context, system, user string) (string, error) {
		seen = user
		return `{"facts":[{"text":"Gempa magnitudo 5,2 mengguncang Bantul","paragraph":0}]}`, nil
	}
	if _, err := ExtractFacts(context.Background(), sample, complete, "uji", 0); err != nil {
		t.Fatalf("ExtractFacts: %v", err)
	}
	if strings.Contains(seen, sample.Article.Title) {
		t.Errorf("judul ikut terkirim ke model; prompt:\n%s", seen)
	}
	if !strings.Contains(seen, "[4]") {
		t.Errorf("badan artikel tidak lengkap terkirim; prompt:\n%s", seen)
	}
}

// TestVerifyMembuangYangTidakBisaDipertanggungjawabkan menguji kedua pagar
// tahap 1 sekaligus, dan memastikan yang dibuang TETAP dilaporkan.
func TestVerifyMembuangYangTidakBisaDipertanggungjawabkan(t *testing.T) {
	reply := `{"facts":[
		{"text":"Gempa bermagnitudo 5,2 mengguncang Bantul pada Senin","paragraph":0},
		{"text":"Presiden meninjau lokasi bencana pada sore hari","paragraph":2},
		{"text":"Kerugian ditaksir mencapai Rp2 miliar","paragraph":99}
	]}`
	sheet, err := ExtractFacts(context.Background(), sample, fakeCompleter(reply), "uji", 0)
	if err != nil {
		t.Fatalf("ExtractFacts: %v", err)
	}
	if len(sheet.Facts) != 1 {
		t.Fatalf("Facts = %d, mau 1: %+v", len(sheet.Facts), sheet.Facts)
	}
	if len(sheet.Rejected) != 2 {
		t.Fatalf("Rejected = %d, mau 2: %+v", len(sheet.Rejected), sheet.Rejected)
	}
	// Nomor di luar jangkauan dan fakta yang tidak berbagi kata isi harus
	// disebut alasannya, bukan sekadar hilang.
	for _, r := range sheet.Rejected {
		if r.Reason == "" {
			t.Errorf("fakta dibuang tanpa alasan: %+v", r)
		}
	}
}

// TestVerifyMengoreksiNomorSalah menjaga perbaikan yang lahir dari uji nyata:
// llama3.1 menyebut nomor paragraf yang meleset satu, dan seluruh pagar tahap 2
// bekerja terhadap nomor itu. Engine harus mencari sendiri asalnya.
func TestVerifyMengoreksiNomorSalah(t *testing.T) {
	// Kalimat ini milik paragraf 4, tetapi model menyebutnya paragraf 5.
	reply := `{"facts":[{"text":"BPBD Bantul melaporkan tujuh rumah rusak ringan di Kecamatan Kretek","paragraph":5}]}`
	sheet, err := ExtractFacts(context.Background(), sample, fakeCompleter(reply), "uji", 0)
	if err != nil {
		t.Fatalf("ExtractFacts: %v", err)
	}
	if len(sheet.Facts) != 1 {
		t.Fatalf("Facts = %d, mau 1 (fakta sah tidak boleh dibuang cuma karena nomornya salah)", len(sheet.Facts))
	}
	if got := sheet.Facts[0].Paragraph; got != 4 {
		t.Errorf("Paragraph = %d, mau 4", got)
	}
	if !sheet.Facts[0].Recited {
		t.Error("Recited = false, mau true — koreksi nomor harus tercatat")
	}
}

// TestExtractFactsMenerimaLarikTelanjang menjaga temuan lapangan: penyedia yang
// TIDAK memaksakan skema (DeepSeek menolak response_format sama sekali) kerap
// membalas larik telanjang. Menolaknya berarti membuang pekerjaan yang isinya
// sudah benar semata karena bungkusnya beda.
func TestExtractFactsMenerimaLarikTelanjang(t *testing.T) {
	reply := `[{"text":"Gempa magnitudo 5,2 mengguncang Bantul pada Senin","paragraph":0}]`
	sheet, err := ExtractFacts(context.Background(), sample, fakeCompleter(reply), "uji", 0)
	if err != nil {
		t.Fatalf("ExtractFacts: %v", err)
	}
	if len(sheet.Facts) != 1 {
		t.Errorf("Facts = %d, mau 1: %+v", len(sheet.Facts), sheet.Facts)
	}
}

// TestPromptMenyebutBentukBalasan: selama ini bentuk balasan cuma dijaga skema
// di sisi server, dan penyedia yang tidak memaksakannya mengarang bentuknya
// sendiri. Bentuknya harus tertulis di promptnya.
func TestPromptMenyebutBentukBalasan(t *testing.T) {
	for name, prompt := range map[string]string{
		"facts":   systemFactsPrompt,
		"compose": systemComposePrompt,
	} {
		if !strings.Contains(prompt, `"`) || !strings.Contains(prompt, "{") {
			t.Errorf("%s: prompt tidak memperlihatkan bentuk JSON-nya", name)
		}
	}
}

// TestPromptMelarangMenerjemahkan menjaga temuan lapangan: DeepSeek
// menerjemahkan artikel Indonesia jadi fakta berbahasa Inggris, dan pagar
// menolaknya karena tidak lagi berbagi satu kata pun dengan paragrafnya —
// sepertiga fakta hilang tanpa satu pun dari keduanya salah.
func TestPromptMelarangMenerjemahkan(t *testing.T) {
	if !strings.Contains(systemFactsPrompt, "Never translate") {
		t.Error("prompt tahap 1 tidak melarang menerjemahkan")
	}
}

// TestExtractFactsBalasanBukanJSON memastikan kegagalan model dilaporkan apa
// adanya, bukan diam-diam jadi nol fakta (notes/12).
func TestExtractFactsBalasanBukanJSON(t *testing.T) {
	_, err := ExtractFacts(context.Background(), sample, fakeCompleter("maaf, saya tidak bisa membantu"), "uji", 0)
	if err == nil {
		t.Fatal("mau galat karena balasan bukan JSON, dapat nil")
	}
}

// TestExtractFactsLive menembak Ollama sungguhan.
//
//	CLIPPER_TEST_LIVE=1 go test ./internal/writer/ -run Live -v
//
// Model bisa ditimpa dengan CLIPPER_TEST_MODEL (bawaan: llama3.1), dan
// CLIPPER_TEST_URL menukar artikel tiruan dengan artikel sungguhan — yang
// panjang, berantakan, dan bercampur teks navigasi seperti di lapangan.
func TestExtractFactsLive(t *testing.T) {
	if os.Getenv("CLIPPER_TEST_LIVE") == "" {
		t.Skip("set CLIPPER_TEST_LIVE=1 untuk menjalankan uji ini terhadap Ollama")
	}
	content := sample
	if u := os.Getenv("CLIPPER_TEST_URL"); u != "" {
		var err error
		content, err = news.FetchContent(context.Background(), u, nil, t.TempDir(), "id")
		if err != nil {
			t.Fatalf("FetchContent: %v", err)
		}
		t.Logf("artikel: %q — %d paragraf, %d kata", content.Article.Title, len(content.Paragraphs), content.WordCount)
	}
	model := os.Getenv("CLIPPER_TEST_MODEL")
	if model == "" {
		model = "llama3.1"
	}
	url := ollama.Discover(context.Background())
	c := ollama.New(url, model)
	// Ekstraksi fakta itu tugas menyalin-ulang, bukan tugas kreatif — suhu
	// tinggi di sini melahirkan "fakta" yang tidak ada di paragrafnya.
	c.Temperature = 0.1

	complete := func(ctx context.Context, system, user string) (string, error) {
		return c.Complete(ctx, system, user, FactsSchema(), 2048)
	}

	sheet, err := ExtractFacts(context.Background(), content, complete, "Ollama ("+model+")", 0)
	if err != nil {
		t.Fatalf("ExtractFacts: %v", err)
	}

	b, _ := json.MarshalIndent(sheet, "", "  ")
	t.Logf("model: %s @ %s\n%s", model, url, b)

	if len(sheet.Facts) == 0 {
		t.Fatal("nol fakta lolos — model tidak mengerjakan tugasnya")
	}

	// Ini inti ujinya: fakta harus tersebar, bukan menumpuk di paragraf awal.
	// Model yang cuma membaca beberapa baris pertama menghasilkan semua fakta
	// dari paragraf 0-1.
	seen := map[int]bool{}
	last := 0
	for _, f := range sheet.Facts {
		seen[f.Paragraph] = true
		if f.Paragraph > last {
			last = f.Paragraph
		}
	}
	if len(seen) < 3 {
		t.Errorf("fakta hanya berasal dari %d paragraf (%v) — badan artikel tampaknya tidak dibaca utuh", len(seen), keys(seen))
	}
	// Separuh akhir artikel harus ikut terwakili. Batasnya longgar: paragraf
	// penutup berita sering memang tidak berisi fakta baru.
	if half := len(content.Paragraphs) / 2; last < half {
		t.Errorf("fakta terakhir dari paragraf %d padahal artikel punya %d paragraf — indikasi model berhenti di awal", last, len(content.Paragraphs))
	}

	if os.Getenv("CLIPPER_TEST_URL") == "" {
		// Paragraf 6 artikel tiruan adalah teaser "Baca juga"; kalau ikut,
		// promptnya kurang tegas soal teks navigasi.
		for _, f := range sheet.Facts {
			if f.Paragraph == 6 {
				t.Errorf("teaser artikel lain ikut jadi fakta: %+v", f)
			}
		}
	}
}

func keys(m map[int]bool) string {
	var s []string
	for k := range m {
		s = append(s, fmt.Sprint(k))
	}
	return strings.Join(s, ",")
}
