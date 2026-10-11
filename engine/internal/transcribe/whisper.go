// Package transcribe membungkus whisper.cpp (whisper-cli) untuk transkripsi.
package transcribe

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gemgum/clipper/engine/internal/types"
)

// Whisper memanggil binary whisper-cli.
type Whisper struct {
	Bin   string
	Model string
}

func New(bin, model string) *Whisper {
	return &Whisper{Bin: bin, Model: model}
}

// Available memastikan binary & model ada.
func (w *Whisper) Available() error {
	if _, err := exec.LookPath(w.Bin); err != nil {
		if _, err2 := os.Stat(w.Bin); err2 != nil {
			// Pesan untuk PENGGUNA aplikasi, bukan pengembang: ./setup.sh
			// adalah skrip repo yang tidak ikut dipasang, jadi menyebutnya sama
			// dengan menyuruh orang berhenti di tengah jalan.
			return fmt.Errorf("the speech recogniser (whisper) is not installed yet: open the Requirements page and press Install")
		}
	}
	if _, err := os.Stat(w.Model); err != nil {
		return fmt.Errorf("the speech model %q has not been downloaded yet: open the Requirements page and press Install next to it",
			modelName(w.Model))
	}
	return nil
}

// modelName mengambil nama model dari path (ggml-small.bin → small).
func modelName(p string) string {
	base := p
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		base = p[i+1:]
	}
	base = strings.TrimPrefix(base, "ggml-")
	base = strings.TrimSuffix(base, ".bin")
	if base == "" {
		return "small"
	}
	return base
}

// offsets adalah rentang waktu (milidetik) di output whisper.cpp.
type offsets struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// tokenOut satu token pada output -ojf.
type tokenOut struct {
	Text    string  `json:"text"`
	Offsets offsets `json:"offsets"`
	ID      int     `json:"id"`
}

// jsonOut adalah struktur output -ojf dari whisper.cpp. Field "tokens" hanya
// terisi bila dipanggil dengan -ojf (output-json-full).
type jsonOut struct {
	Transcription []struct {
		Offsets offsets    `json:"offsets"`
		Text    string     `json:"text"`
		Tokens  []tokenOut `json:"tokens"`
	} `json:"transcription"`
}

// isSpecialToken menyaring token khusus whisper ([_BEG_], [_TT_120], dst).
func isSpecialToken(t string) bool {
	t = strings.TrimSpace(t)
	return strings.HasPrefix(t, "[_") && strings.HasSuffix(t, "]")
}

// wordsFromTokens menggabungkan token sub-kata menjadi kata utuh beserta
// waktunya. Token baru memulai kata bila teksnya diawali spasi (konvensi BPE
// whisper). Token tanpa timestamp (offsets 0) ikut kata berjalan.
func wordsFromTokens(toks []tokenOut, segStart, segEnd float64) []types.Word {
	var words []types.Word
	for _, t := range toks {
		if t.Text == "" || isSpecialToken(t.Text) {
			continue
		}
		start := float64(t.Offsets.From) / 1000.0
		end := float64(t.Offsets.To) / 1000.0
		newWord := strings.HasPrefix(t.Text, " ") || len(words) == 0
		piece := strings.TrimSpace(t.Text)
		if piece == "" {
			continue
		}
		if newWord {
			words = append(words, types.Word{Start: start, End: end, Text: piece})
			continue
		}
		last := &words[len(words)-1]
		last.Text += piece
		if end > last.End {
			last.End = end
		}
	}
	// Rapikan: buang kata tanpa timing yang masuk akal & jaga urutan naik.
	prev := segStart
	for i := range words {
		if words[i].Start < prev || words[i].Start > segEnd {
			words[i].Start = prev
		}
		if words[i].End <= words[i].Start {
			words[i].End = words[i].Start + 0.15
		}
		prev = words[i].End
	}
	if len(words) > 0 && words[len(words)-1].End > segEnd {
		words[len(words)-1].End = segEnd
	}
	return words
}

// Transcribe mentranskripsi WAV 16kHz mono → Transcript. onProgress (boleh nil)
// dipanggil dengan fraksi 0..1 selama proses (diparse dari output whisper).
func (w *Whisper) Transcribe(ctx context.Context, wavPath, outBase, language string, threads int, onProgress func(float64)) (types.Transcript, error) {
	if threads <= 0 {
		threads = 4
	}
	args := []string{
		"-m", w.Model,
		"-f", wavPath,
		"-l", language,
		"-t", fmt.Sprintf("%d", threads),
		// Jangan suapkan teks segmen sebelumnya sebagai konteks decoder
		// (bawaan -1 = tanpa batas). Konteks itu yang membuat model besar
		// terjebak: sekali tergelincir mengulang satu kalimat, kalimat itu
		// ikut jadi konteks segmen berikutnya dan loop-nya menguatkan diri
		// sendiri sampai audio habis. Kerugiannya kecil untuk kita — subtitle
		// dinilai per kalimat, bukan sebagai prosa panjang.
		"-mc", "0",
		// Tekan token non-ucapan ([MUSIC], tepuk tangan, dsb) yang jadi pemicu
		// halusinasi di bagian hening.
		"-sns",
		"-ojf",         // output JSON lengkap (termasuk timestamp per token)
		"-of", outBase, // output file prefix
		"-pp", // print progress (ke stderr)
	}
	cmd := exec.CommandContext(ctx, w.Bin, args...)
	// Agar library whisper (.so) ditemukan walau folder dipindah: cari .so di
	// folder yang sama dengan binary (kita salin ke bin/ saat setup).
	cmd.Env = append(os.Environ(),
		"LD_LIBRARY_PATH="+filepath.Dir(w.Bin)+string(os.PathListSeparator)+os.Getenv("LD_LIBRARY_PATH"))
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return types.Transcript{}, err
	}
	if err := cmd.Start(); err != nil {
		return types.Transcript{}, fmt.Errorf("start whisper: %w", err)
	}

	// Baca stderr: parse progress + simpan beberapa baris terakhir untuk pesan error.
	var tail []string
	last := -1
	sc := bufio.NewScanner(stderr)
	for sc.Scan() {
		line := sc.Text()
		tail = append(tail, line)
		if len(tail) > 8 {
			tail = tail[1:]
		}
		if p, ok := parseProgress(line); ok && p != last {
			last = p
			if onProgress != nil {
				onProgress(float64(p) / 100.0)
			}
		}
	}
	if err := cmd.Wait(); err != nil {
		return types.Transcript{}, fmt.Errorf("whisper failed: %w: %s", err, strings.Join(tail, " | "))
	}

	raw, err := os.ReadFile(outBase + ".json")
	if err != nil {
		return types.Transcript{}, fmt.Errorf("read whisper output: %w", err)
	}
	var parsed jsonOut
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return types.Transcript{}, fmt.Errorf("parse whisper output: %w", err)
	}

	tr := types.Transcript{Language: language}
	for _, s := range parsed.Transcription {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			continue
		}
		start := float64(s.Offsets.From) / 1000.0
		end := float64(s.Offsets.To) / 1000.0
		tr.Segments = append(tr.Segments, types.TranscriptSegment{
			Start: start,
			End:   end,
			Text:  text,
			Words: wordsFromTokens(s.Tokens, start, end),
		})
	}
	return tr, nil
}

// parseProgress mengambil persen dari baris "...progress =  20%".
func parseProgress(line string) (int, bool) {
	i := strings.Index(line, "progress =")
	if i < 0 {
		return 0, false
	}
	rest := strings.TrimSpace(line[i+len("progress ="):])
	rest = strings.TrimSuffix(strings.TrimSpace(rest), "%")
	rest = strings.TrimSpace(rest)
	n, err := strconv.Atoi(rest)
	if err != nil {
		return 0, false
	}
	return n, true
}
