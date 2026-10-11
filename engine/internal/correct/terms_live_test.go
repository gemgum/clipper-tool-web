package correct

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gemgum/clipper/engine/internal/score/ollama"
	"github.com/gemgum/clipper/engine/internal/types"
)

// termLiveSegs adalah segmen SUNGGUHAN dari transkrip large-v3, apa adanya.
//
// Semuanya memuat "Londo Ireng" yang salah dengar, dan sengaja dibiarkan dalam
// berbagai bentuk salahnya — itulah yang membuat kasus ini berguna: satu istilah
// jarang salah dengan cara yang sama di sepanjang video.
//
//	"Irang"      bentuk paling umum
//	"Iram"       varian yang lebih jauh bunyinya
//	"Londo-Iram" kedua katanya disambung tanda hubung
//	"Irang"      berdiri sendiri, tanpa kata "Londo"
//
// Dua segmen terakhir adalah kontrol: satu bertanda hubung dialog Unicode yang
// harus hilang, satu lagi sudah benar dan harus utuh.
var termLiveSegs = []string{
	"Keluarnya saja, Pak. Misalnya, satu, Londo Irang.",
	"itu diberi cap sebagai Londo Iram.",
	"perdebatan itu dan pernyataan Presiden tentang Londo-Iram,",
	"Jadi kita ini Londo Irang lah kira-kira gitu ya?",
	"Londo Irang, tapi saya coklat.",
	"Krem, baju krem. Anda yang Irang.",
	"Tapi begini, itu agak menyakitkan ya tujuan Londo Irang.",
	"Karena Londo Irang itu sejarah pengkhianatan terhadap perjuangan.",
	"− Kita lanjutkan, Pak Mahfud.",
	"Saya kira kita lihat waktunya dulu ya, Pak.",
}

// TestTermsLive memeriksa daftar istilah terhadap model sungguhan.
//
// Prompt tidak menjamin apa pun, jadi hasilnya harus benar-benar dilihat —
// dan tiap model berbeda tajam di tugas ini. Dilewati bila variabelnya tidak
// diset, supaya `go test ./...` biasa tidak memanggil model.
//
//	CLIPPER_TEST_LIVE=1 CLIPPER_TEST_MODEL=llama3.1 \
//	CLIPPER_TEST_TERMS="Londo Ireng, Mahfud MD, URI" \
//	go test ./internal/correct/ -run TermsLive -v -count=1
//
// Jalankan juga dengan CLIPPER_TEST_TERMS kosong sebagai kontrol: tanpa daftar,
// model memang membiarkan semua ejaan salah itu lewat.
func TestTermsLive(t *testing.T) {
	if os.Getenv("CLIPPER_TEST_LIVE") == "" {
		t.Skip("set CLIPPER_TEST_LIVE=1 untuk menjalankan uji ini terhadap Ollama")
	}
	model := os.Getenv("CLIPPER_TEST_MODEL")
	if model == "" {
		model = "llama3.1"
	}
	terms := ParseTerms(os.Getenv("CLIPPER_TEST_TERMS"))

	tr := types.Transcript{Language: "id"}
	for i, s := range termLiveSegs {
		tr.Segments = append(tr.Segments, types.TranscriptSegment{
			Start: float64(i) * 3, End: float64(i)*3 + 2.5, Text: s,
		})
	}

	c := ollama.New("", model)
	c.Temperature = 0.1
	complete := func(ctx context.Context, system, user string, schema any) (string, error) {
		return c.Complete(ctx, system, user, schema, 4096)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	out, rep, err := Correct(ctx, tr, terms, complete, "Ollama ("+model+")", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("terms = %v", terms)
	t.Logf("%s", rep.Summary())

	var all []string
	for i, s := range out.Segments {
		mark := "  "
		if s.Text != termLiveSegs[i] {
			mark = "->"
		}
		t.Logf("%s %s", mark, s.Text)
		all = append(all, s.Text)
	}
	joined := strings.Join(all, " ")
	wrong := strings.Count(joined, "Irang") + strings.Count(joined, "Iram")
	t.Logf("masih salah: %d | sudah benar (Ireng): %d", wrong, strings.Count(joined, "Ireng"))

	// Sengaja Errorf, bukan Fatalf: keluarannya di atas sudah dicetak, dan yang
	// ingin dilihat justru SEBERAPA banyak yang lolos, bukan sekadar lulus/gagal.
	if len(terms) > 0 && wrong > 2 {
		t.Errorf("daftar istilah seharusnya membereskan sebagian besar; %d masih salah", wrong)
	}
	if strings.Contains(joined, "−") {
		t.Error("tanda hubung dialog Unicode seharusnya dibuang")
	}
}
