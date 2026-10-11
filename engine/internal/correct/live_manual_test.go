package correct

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gemgum/clipper/engine/internal/score/ollama"
	"github.com/gemgum/clipper/engine/internal/types"
)

// Uji manual terhadap LLM sungguhan. Jalankan dengan:
//
//	CLIPPER_TEST_TRANSCRIPT=data/cache/transcripts/<kunci>.json \
//	  go test ./internal/correct/ -run TestCorrectLive -v
//
// Opsional: CLIPPER_TEST_SEGMENTS=40 (default 30), CLIPPER_TEST_MODEL=qwen2.5.
//
// Dilewati bila variabelnya tidak diset, supaya tidak ikut di CI dan tidak
// memanggil model saat `go test ./...` biasa.
func TestCorrectLive(t *testing.T) {
	path := os.Getenv("CLIPPER_TEST_TRANSCRIPT")
	if path == "" {
		t.Skip("set CLIPPER_TEST_TRANSCRIPT ke berkas cache transkrip untuk menjalankan uji ini")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var tr types.Transcript
	if err := json.Unmarshal(raw, &tr); err != nil {
		t.Fatal(err)
	}

	limit := 30
	if v := os.Getenv("CLIPPER_TEST_SEGMENTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if len(tr.Segments) > limit {
		tr.Segments = tr.Segments[:limit]
	}

	model := os.Getenv("CLIPPER_TEST_MODEL")
	if model == "" {
		model = "qwen2.5"
	}
	client := ollama.New("", model)
	complete := func(ctx context.Context, system, user string, schema any) (string, error) {
		return client.Complete(ctx, system, user, schema, 4096)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	started := time.Now()
	fixed, report, err := Correct(ctx, tr, nil, complete, "Ollama ("+model+")", func(done, total int) {
		t.Logf("potongan %d/%d", done, total)
	})
	if err != nil {
		t.Fatal("GALAT:", err)
	}
	t.Logf("lama: %.1fs | %s", time.Since(started).Seconds(), report.Summary())

	for i := range tr.Segments {
		if tr.Segments[i].Text != fixed.Segments[i].Text {
			t.Logf("\n  [%.2f] SEBELUM: %s\n         SESUDAH: %s",
				tr.Segments[i].Start, tr.Segments[i].Text, fixed.Segments[i].Text)
		}
	}

	// Pemeriksaan yang benar-benar penting: batas segmen tidak boleh bergeser,
	// dan tiap segmen tetap punya kata bertimestamp yang urut naik.
	for i := range fixed.Segments {
		got, want := fixed.Segments[i], tr.Segments[i]
		if got.Start != want.Start || got.End != want.End {
			t.Errorf("batas segmen %d bergeser: %.2f-%.2f, ingin %.2f-%.2f",
				i, got.Start, got.End, want.Start, want.End)
		}
		prev := got.Start - 0.001
		for j, w := range got.Words {
			if w.Start < prev {
				t.Errorf("segmen %d kata %d mundur: mulai %.3f setelah %.3f", i, j, w.Start, prev)
			}
			if w.End <= w.Start {
				t.Errorf("segmen %d kata %d berdurasi nol", i, j)
			}
			prev = w.Start
		}
	}
}
