package api

import (
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
)

// Id mesin dari setelan jadi koordinat pipeline; job yang diulang (Transcriber
// "api" + id tersimpan, koordinat hilang dari disk) terisi ulang; whisper dan
// mesin tanpa endpoint audio (Gemini, Claude) tidak disentuh.
func TestFillTranscriber(t *testing.T) {
	t.Setenv("CUSTOM_BASE_URL", "https://gw.example/v1")
	t.Setenv("CLIPPER_TRANSCRIBE_MODEL", "whisper-large-v3")

	o := config.Options{Transcriber: "custom"}
	fillTranscriber(&o)
	if o.Transcriber != config.TranscriberAPI || o.TranscribeEngine != "custom" || o.TranscribeBase != "https://gw.example" ||
		o.TranscribePath != "/v1" || o.TranscribeKeyEnv != "CUSTOM_API_KEY" || o.TranscribeModel != "whisper-large-v3" {
		t.Fatalf("koordinat salah: %+v", o)
	}

	again := config.Options{Transcriber: config.TranscriberAPI, TranscribeEngine: "custom", TranscribeModel: "m"}
	fillTranscriber(&again)
	if again.TranscribeBase != "https://gw.example" || again.TranscribeModel != "m" {
		t.Fatalf("job ulang tidak terisi: %+v", again)
	}

	for _, id := range []string{"whisper", "aistudio", "gemini", "claude"} {
		o := config.Options{Transcriber: id}
		fillTranscriber(&o)
		if o.Transcriber != id || o.TranscribeBase != "" {
			t.Errorf("%s ikut diubah: %+v", id, o)
		}
	}
}
