package transcribe

import (
	"context"
	_ "embed"
	"errors"
	"strings"
)

// testSpeech = 5,5 detik suara sintetis (ffmpeg flite, suara "slt", Opus 24 kbps
// mono, 16 KB) untuk tombol Test mesin transkripsi. Dibuat sendiri, bukan
// potongan video orang, supaya bebas dibawa di dalam biner.
//
//go:embed test-speech.ogg
var testSpeech []byte

// TestSpeechText = kalimat yang diucapkan di test-speech.ogg.
const TestSpeechText = "Hello. This is a short test of speech recognition for Clipper. One, two, three."

// Probe mengirim audio contoh ke endpoint transkripsi dan mengembalikan teks
// yang terdengar. Yang diuji sama persis dengan yang dipakai job: alamat, kunci,
// model, verbose_json, DAN adanya timestamp — model yang hanya membalas teks
// tidak bisa dipakai untuk subtitle, jadi gagal di sini juga.
func (o *OpenAIAudio) Probe(ctx context.Context, language string) (string, error) {
	if err := o.Available(); err != nil {
		return "", err
	}
	segs, err := o.chunk(ctx, testSpeech, language)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(segs))
	for _, s := range segs {
		parts = append(parts, strings.TrimSpace(s.Text))
	}
	text := strings.TrimSpace(strings.Join(parts, " "))
	if text == "" {
		return "", errors.New("the engine answered but heard nothing in the test audio")
	}
	return text, nil
}
