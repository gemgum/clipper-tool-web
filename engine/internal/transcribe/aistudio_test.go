package transcribe

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Permintaan membawa kunci di header, audio inline, dan skema JSON; dua
// potongan digeser ke waktu mutlak dan tiap segmen punya waktu per kata.
func TestAIStudioChunksAndOffsets(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("x-goog-api-key") != "k" {
			t.Errorf("kunci tidak di header: %q", r.Header.Get("x-goog-api-key"))
		}
		if !strings.HasSuffix(r.URL.Path, "/v1beta/models/m:generateContent") {
			t.Errorf("path = %s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		var req gmRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Fatal(err)
		}
		parts := req.Contents[0].Parts
		if len(parts) != 2 || parts[1].InlineData == nil || parts[1].InlineData.MimeType != "audio/ogg" || parts[1].InlineData.Data == "" {
			t.Errorf("audio inline hilang: %+v", parts)
		}
		if req.GenerationConfig["responseMimeType"] != "application/json" || req.GenerationConfig["responseSchema"] == nil {
			t.Errorf("skema tidak diminta: %v", req.GenerationConfig)
		}
		// Segmen sengaja tidak urut, tumpang-tindih, satu kosong, satu lewat batas.
		segs := `{"segments":[{"start":5,"end":9,"text":"dua tiga"},{"start":0,"end":6,"text":"satu"},{"start":7,"end":8,"text":"  "},{"start":590,"end":700,"text":"akhir"}]}`
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{
			"content": map[string]any{"parts": []any{map[string]any{"text": segs}}}, "finishReason": "STOP"}}})
	}))
	defer srv.Close()

	a := &AIStudio{Key: "k", Model: "m", Base: srv.URL, HTTP: srv.Client(),
		Encode: func(_ context.Context, _, _ float64, out string) error {
			return os.WriteFile(out, []byte("ogg"), 0o644)
		}}
	tr, err := a.Transcribe(context.Background(), 900, "id", t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("potongan = %d, mau 2 (900 dtk / 600)", calls)
	}
	// "akhir" (590–700) dijepit ke batas potongan pertama (600); di potongan
	// kedua (panjang 300) ia mulai di luar potongan, jadi dibuang.
	want := [][2]float64{{0, 6}, {6, 9}, {590, 600}, {600, 606}, {606, 609}}
	if len(tr.Segments) != len(want) {
		t.Fatalf("segmen = %+v", tr.Segments)
	}
	for i, w := range want {
		s := tr.Segments[i]
		if math.Abs(s.Start-w[0]) > 1e-9 || math.Abs(s.End-w[1]) > 1e-9 {
			t.Errorf("segmen %d = %.1f–%.1f, mau %.1f–%.1f", i, s.Start, s.End, w[0], w[1])
		}
		if len(s.Words) == 0 || s.Words[0].Start != s.Start || s.Words[len(s.Words)-1].End != s.End {
			t.Errorf("segmen %d: kata tidak menutup segmennya: %+v", i, s.Words)
		}
	}
}

// Waktu per kata dibagi sebanding jumlah huruf.
func TestSpreadWordsProportional(t *testing.T) {
	ws := spreadWords("ab abcd ab", 10, 18) // 2+4+2 = 8 huruf, 8 detik
	want := [][2]float64{{10, 12}, {12, 16}, {16, 18}}
	for i, w := range want {
		if math.Abs(ws[i].Start-w[0]) > 1e-9 || math.Abs(ws[i].End-w[1]) > 1e-9 {
			t.Errorf("kata %d = %.2f–%.2f, mau %.0f–%.0f", i, ws[i].Start, ws[i].End, w[0], w[1])
		}
	}
}

func TestAIStudioMissingKey(t *testing.T) {
	_, err := (&AIStudio{Model: "m"}).Transcribe(context.Background(), 10, "id", t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "Google AI Studio API key") {
		t.Fatalf("galat = %v", err)
	}
}

// Galat dari penyedia sampai ke pemanggil apa adanya — tidak ada cadangan.
func TestAIStudioProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"message":"API key not valid"}}`))
	}))
	defer srv.Close()
	a := &AIStudio{Key: "k", Model: "m", Base: srv.URL, HTTP: srv.Client(),
		Encode: func(_ context.Context, _, _ float64, out string) error { return os.WriteFile(out, []byte("x"), 0o644) }}
	_, err := a.Transcribe(context.Background(), 30, "id", t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "API key not valid") {
		t.Fatalf("galat = %v", err)
	}
}
