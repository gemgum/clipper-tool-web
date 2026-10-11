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

// Kata daerah yang lazim diselipkan penutur Indonesia harus lolos utuh.
//
// Uji manual terhadap model sungguhan — prompt tidak menjamin apa pun, jadi
// harus benar-benar dilihat. Dilewati bila variabelnya tidak diset, supaya
// `go test ./...` biasa tidak memanggil model.
//
//	CLIPPER_TEST_LIVE=1 go test ./internal/correct/ -run Javanese -v
func TestJavaneseWordsSurviveLive(t *testing.T) {
	if os.Getenv("CLIPPER_TEST_LIVE") == "" {
		t.Skip("set CLIPPER_TEST_LIVE=1 untuk menjalankan uji ini terhadap Ollama")
	}
	segs := []string{
		"- Motorku warnane ireng, dudu abang.",
		"Jadi dia bilang ojo lali, harus tetap semangat.",
		"Kalau sudah mangan, baru kita berangkat ya.",
		"Mangga atuh, urang balik heula ka bumi.",
		"Gue mah kagak ngerti, emang begitu dari dulu.",
	}
	tr := types.Transcript{Language: "id"}
	for i, s := range segs {
		tr.Segments = append(tr.Segments, types.TranscriptSegment{
			Start: float64(i) * 3, End: float64(i)*3 + 2.5, Text: s,
		})
	}

	c := ollama.New("", "qwen2.5")
	complete := func(ctx context.Context, system, user string, schema any) (string, error) {
		return c.Complete(ctx, system, user, schema, 4096)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	out, rep, err := Correct(ctx, tr, nil, complete, "Ollama (qwen2.5)", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", rep.Summary())

	jaga := []string{"ireng", "abang", "ojo lali", "mangan", "mangga", "urang", "heula", "bumi", "kagak"}
	for i, s := range out.Segments {
		t.Logf("%d\n  sebelum: %s\n  sesudah: %s", i, segs[i], s.Text)
	}
	joined := strings.ToLower(strings.Join(func() []string {
		var v []string
		for _, s := range out.Segments {
			v = append(v, s.Text)
		}
		return v
	}(), " "))
	for _, w := range jaga {
		if !strings.Contains(joined, w) {
			t.Errorf("kata daerah %q hilang dari hasil koreksi", w)
		}
	}
}
