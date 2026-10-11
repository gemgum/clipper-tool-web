package writer

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// dualCompleter meniru satu mesin LLM yang melayani kedua tahap. Dibedakan dari
// prompt sistemnya, sama seperti di lapangan: satu mesin, dua tugas.
func dualCompleter(t *testing.T, calls *int) Completer {
	t.Helper()
	return func(ctx context.Context, system, user string, schema any) (string, error) {
		*calls++
		if strings.Contains(system, "extract facts") {
			return `{"facts":[
				{"text":"Gempa magnitudo 5,2 mengguncang Kabupaten Bantul pada Senin","paragraph":0},
				{"text":"Pusat gempa di kedalaman 24 kilometer dan tidak berpotensi tsunami","paragraph":1}
			]}`, nil
		}
		return `{"title":"Gempa magnitudo 5,2 guncang Bantul",
			"lead":"Gempa mengguncang Bantul pada Senin dini hari.",
			"body":[
				"Pusat gempa berada di kedalaman 24 kilometer dan tidak berpotensi tsunami.",
				"Badan Penanggulangan Bencana Daerah Bantul melaporkan tujuh rumah rusak ringan.",
				"Tidak ada korban jiwa dalam peristiwa tersebut."
			]}`, nil
	}
}

func twoArticles(t *testing.T) (urls []string, close func()) {
	t.Helper()
	srv := newsServer(t, map[string]func(string) string{
		"/a": func(base string) string {
			return articleHTML(base+"/a", "Gempa magnitudo 5,2 guncang Bantul", "Contoh", paraGempa1, paraGempa2)
		},
		"/b": func(base string) string {
			return articleHTML(base+"/b", "Gempa Bantul, BPBD catat kerusakan", "Contoh Dua", paraGempa2, paraGempa3)
		},
	})
	return []string{srv.URL + "/a", srv.URL + "/b"}, srv.Close
}

// TestRunUjungKeUjung menjalankan ketiga tahap dan memeriksa yang menentukan
// fitur ini berguna: berkasnya ada, dan jumlah panggilan LLM sesuai rancangan.
func TestRunUjungKeUjung(t *testing.T) {
	urls, closeSrv := twoArticles(t)
	defer closeSrv()

	calls := 0
	out := t.TempDir()
	res, err := Run(context.Background(),
		Options{URLs: urls, Lang: "id"},
		Deps{Read: dualCompleter(t, &calls), ReadEngine: "uji", CacheDir: t.TempDir(), OutDir: out},
		nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Satu panggilan per artikel (tahap 1) + satu untuk menulis (tahap 2).
	// Kalau angka ini naik, biayanya naik untuk tiap job — layak dijaga.
	// Draf tiruan selalu melanggar batas panjang, jadi Compose memakai jatah
	// perbaikan satu kalinya — itu memang rancangannya (notes/38).
	want := len(urls) + 1
	if res.Draft.Repaired {
		want++
	}
	if calls != want {
		t.Errorf("panggilan LLM = %d, mau %d", calls, want)
	}
	for _, p := range []string{res.Post.Markdown, res.Post.Sources} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("berkas tidak ada: %v", err)
		}
	}
	md, err := os.ReadFile(res.Post.Markdown)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "Dirangkum dari:") {
		t.Errorf("atribusi tidak ditempel:\n%s", md)
	}
	// Kedua artikel sumber harus terlacak di sumber.json — itu jejak audit
	// yang dipakai redaktur sebelum terbit.
	b, err := os.ReadFile(res.Post.Sources)
	if err != nil {
		t.Fatal(err)
	}
	var got sumberFile
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("sumber.json bukan JSON sah: %v", err)
	}
	if len(got.Sources) != len(urls) {
		t.Errorf("sumber tercatat = %d, mau %d", len(got.Sources), len(urls))
	}
	for _, s := range got.Sources {
		if s.URL == "" || len(s.Facts) == 0 {
			t.Errorf("jejak sumber tidak lengkap: %+v", s)
		}
	}
}

// TestRunMelaporkanTiapTahap: ringkasan waktu per tahap adalah angka pembanding
// antar mesin LLM, jadi tiap tahap harus benar-benar tercatat.
func TestRunMelaporkanTiapTahap(t *testing.T) {
	urls, closeSrv := twoArticles(t)
	defer closeSrv()

	var stages []string
	var last string
	calls := 0
	res, err := Run(context.Background(),
		Options{URLs: urls, Lang: "id"},
		Deps{Read: dualCompleter(t, &calls), ReadEngine: "uji", CacheDir: t.TempDir(), OutDir: t.TempDir()},
		func(p Progress) {
			if len(stages) == 0 || stages[len(stages)-1] != p.Stage {
				stages = append(stages, p.Stage)
			}
			last = p.Message
		})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{"gathering", "reading", "writing", "saving", "done"}
	if strings.Join(stages, ",") != strings.Join(want, ",") {
		t.Errorf("urutan tahap = %v, mau %v", stages, want)
	}
	// Kumpulkan + baca 2 artikel + tulis + simpan = 5 tahap terukur.
	if len(res.Summary.Stages) != 5 {
		t.Errorf("tahap terukur = %d, mau 5: %+v", len(res.Summary.Stages), res.Summary.Stages)
	}
	if res.Summary.TotalSec <= 0 {
		t.Error("total waktu tidak terukur")
	}
	// Pesan terakhir adalah tabel ringkasan yang tampil di terminal & kotak log.
	for _, want := range []string{"Time per stage", "TOTAL", "Write article"} {
		if !strings.Contains(last, want) {
			t.Errorf("ringkasan tidak memuat %q:\n%s", want, last)
		}
	}
}

// TestRunMesinTerpisahPerTahap menjaga alasan pemisahannya: tahap 1 dipanggil
// sekali per artikel, tahap 2 sekali saja. Menyatukan keduanya berarti membayar
// mesin mahal lima kali untuk pekerjaan menyalin-ulang (notes/39).
func TestRunMesinTerpisahPerTahap(t *testing.T) {
	urls, closeSrv := twoArticles(t)
	defer closeSrv()

	var reads, writes int
	readLLM := func(ctx context.Context, system, user string, schema any) (string, error) {
		reads++
		return `{"facts":[{"text":"Gempa magnitudo 5,2 mengguncang Kabupaten Bantul pada Senin","paragraph":0}]}`, nil
	}
	writeLLM := func(ctx context.Context, system, user string, schema any) (string, error) {
		writes++
		return `{"title":"Gempa Bantul","lead":"Gempa mengguncang Bantul.",
			"body":["Pusat gempa di kedalaman 24 kilometer.","Tujuh rumah rusak ringan.","Tidak ada korban jiwa."]}`, nil
	}

	res, err := Run(context.Background(),
		Options{URLs: urls, Lang: "id"},
		Deps{
			Read: readLLM, ReadEngine: "murah",
			Write: writeLLM, WriteEngine: "pandai",
			CacheDir: t.TempDir(), OutDir: t.TempDir(),
		}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if reads != len(urls) {
		t.Errorf("panggilan baca = %d, mau %d (satu per artikel)", reads, len(urls))
	}
	if writes < 1 {
		t.Errorf("mesin tulis tidak dipakai sama sekali")
	}
	// Ringkasan harus MENYEBUT keduanya: tanpa itu, angka waktunya tidak bisa
	// dibandingkan dengan percobaan lain.
	if res.Summary.ReadEngine != "murah" || res.Summary.WriteEngine != "pandai" {
		t.Errorf("ringkasan tidak mencatat kedua mesin: %+v", res.Summary)
	}
	if out := res.Summary.Format(); !strings.Contains(out, "read murah") {
		t.Errorf("tabel ringkasan tidak menyebut kedua mesin:\n%s", out)
	}
}

// TestRunMesinTulisKosongPakaiMesinBaca: kosong berarti satu mesin untuk
// keduanya — jalan yang paling umum, dan tidak boleh jadi galat.
func TestRunMesinTulisKosongPakaiMesinBaca(t *testing.T) {
	urls, closeSrv := twoArticles(t)
	defer closeSrv()

	calls := 0
	res, err := Run(context.Background(),
		Options{URLs: urls, Lang: "id"},
		Deps{Read: dualCompleter(t, &calls), ReadEngine: "satu-satunya", CacheDir: t.TempDir(), OutDir: t.TempDir()},
		nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Summary.WriteEngine != "satu-satunya" {
		t.Errorf("WriteEngine = %q, mau jatuh ke mesin baca", res.Summary.WriteEngine)
	}
}

// TestRunGagalBilaTakAdaSumber: tanpa satu pun artikel, tidak ada yang bisa
// ditulis — dan pesannya harus menyebutkan sebabnya.
func TestRunGagalBilaTakAdaSumber(t *testing.T) {
	srv := newsServer(t, nil)
	defer srv.Close()
	calls := 0
	_, err := Run(context.Background(),
		Options{URLs: []string{srv.URL + "/kosong"}},
		Deps{Read: dualCompleter(t, &calls), ReadEngine: "uji", CacheDir: t.TempDir(), OutDir: t.TempDir()},
		nil)
	if err == nil {
		t.Fatal("mau galat")
	}
	if calls != 0 {
		t.Errorf("LLM dipanggil %d kali padahal tidak ada sumber", calls)
	}
}
