package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/correct"
)

// decodeJobOptions meniru persis apa yang dilakukan createJob pada body:
// disemai default lebih dulu, lalu ditimpa JSON, lalu daftar istilah
// dinormalkan. Ditiru, bukan dipanggil langsung, supaya bisa diuji tanpa
// menjalankan job sungguhan.
func decodeJobOptions(t *testing.T, body string) config.Options {
	t.Helper()
	var req struct {
		Source struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"source"`
		Options config.Options `json:"options"`
	}
	req.Options = config.DefaultOptions()
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("body ditolak decoder: %v", err)
	}
	opts := req.Options
	opts.Terms = correct.ParseTerms(strings.Join(opts.Terms, ","))
	if err := opts.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return opts
}

// Bentuk yang dikirim GUI: daftar yang sudah dipecah di klien.
func TestJobOptionsCarryTerms(t *testing.T) {
	opts := decodeJobOptions(t, `{
		"source": {"type": "path", "value": "/tmp/a.mp4"},
		"options": {"terms": ["Londo Ireng", "Mahfud MD", "URI"]}
	}`)
	want := []string{"Londo Ireng", "Mahfud MD", "URI"}
	if len(opts.Terms) != len(want) {
		t.Fatalf("Terms = %v, want %v", opts.Terms, want)
	}
	for i := range want {
		if opts.Terms[i] != want[i] {
			t.Fatalf("Terms = %v, want %v", opts.Terms, want)
		}
	}
}

// Klien juga boleh mengirim satu string berisi koma; hasilnya harus sama.
// Inilah yang membuat CLI dan GUI berperilaku identik.
func TestJobOptionsAcceptSingleString(t *testing.T) {
	opts := decodeJobOptions(t, `{
		"source": {"type": "path", "value": "/tmp/a.mp4"},
		"options": {"terms": ["Londo Ireng, Mahfud MD"]}
	}`)
	if len(opts.Terms) != 2 || opts.Terms[0] != "Londo Ireng" || opts.Terms[1] != "Mahfud MD" {
		t.Fatalf("Terms = %v", opts.Terms)
	}
}

// Tanpa field terms, Terms harus kosong — DAN kunci cache koreksi harus tetap
// versi polos. Perbedaan inilah yang dipakai untuk membuktikan, dari berkas
// cache saja, apakah suatu job benar-benar membawa daftar istilah.
func TestJobOptionsWithoutTerms(t *testing.T) {
	opts := decodeJobOptions(t, `{
		"source": {"type": "path", "value": "/tmp/a.mp4"},
		"options": {"whisper_model": "large-v3"}
	}`)
	if len(opts.Terms) != 0 {
		t.Fatalf("Terms should be empty, got %v", opts.Terms)
	}
	if v := correct.CacheVersion(opts.Terms); v != correct.PromptVersion {
		t.Errorf("cache version = %q, want the plain %q", v, correct.PromptVersion)
	}
	if v := correct.CacheVersion([]string{"Londo Ireng"}); v == correct.PromptVersion {
		t.Error("terms must change the cache version")
	}
}
