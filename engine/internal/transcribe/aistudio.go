package transcribe

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gemgum/clipper/engine/internal/httpx"
	"github.com/gemgum/clipper/engine/internal/types"
)

// Transkripsi lewat Google AI Studio (Gemini, API native). Lihat notes/43.
//
// Dipilih per job (Options.Transcriber = "aistudio"), bukan cadangan whisper:
// bila gagal, job berhenti dengan pesan akar masalahnya (notes/12).

// AIStudioDefaultModel dipakai bila AI_STUDIO_MODEL kosong.
// AIStudioDefaultModel: diuji 9 Oktober 2026 dengan audio sungguhan.
// gemini-2.5-flash sudah ditolak (404, "no longer available to new users");
// 3.6/3.7/3.8-flash & gemini-flash-latest membalas 503 "high demand" untuk
// input audio walau teks biasa lolos. Timpa dengan AI_STUDIO_MODEL.
const AIStudioDefaultModel = "gemini-3.5-flash"

// AIStudioChunkSec: panjang satu potongan audio. 10 menit Opus 24 kbps ≈ 1,8 MB
// — jauh di bawah batas 20 MB permintaan inline, dan balasan JSON-nya masih
// muat di jatah keluaran model.
const AIStudioChunkSec = 600

// AIStudio memanggil generateContent dengan audio inline.
type AIStudio struct {
	Key   string
	Model string
	// Base boleh diganti (uji); kosong = generativelanguage.googleapis.com.
	Base string
	HTTP *http.Client
	// Encode menulis potongan audio [start, start+dur) dari sumber ke out
	// sebagai Opus/OGG kecil. Disediakan pemanggil (paket ffmpeg), supaya paket
	// ini tidak perlu tahu letak biner ffmpeg.
	Encode func(ctx context.Context, start, dur float64, out string) error
}

// AIStudioFromEnv membaca kunci & model dari lingkungan (.env sudah dimuat ke
// lingkungan proses saat engine mulai).
func AIStudioFromEnv() *AIStudio {
	model := strings.TrimSpace(os.Getenv("AI_STUDIO_MODEL"))
	if model == "" {
		model = AIStudioDefaultModel
	}
	// Kuncinya SATU: yang diisi tangan di aplikasi (Settings → Video clips, atau
	// mesin "Google AI Studio (Gemini)" di Engines & Keys — keduanya menyimpan
	// GEMINI_API_KEY). Tidak ada lagi AI_STUDIO_KEY yang hanya bisa diisi lewat
	// .env (permintaan pemilik 9 Oktober 2026: semua kunci AI diisi manual).
	return &AIStudio{Key: strings.TrimSpace(os.Getenv("GEMINI_API_KEY")), Model: model}
}

// Available memastikan kuncinya ada SEBELUM audio diekstrak.
func (a *AIStudio) Available() error {
	if a.Key == "" {
		return fmt.Errorf("AI Studio has no API key yet: paste the Google AI Studio API key under Settings → Engines & keys → Google AI Studio (Gemini)")
	}
	return nil
}

type gmPart struct {
	Text       string        `json:"text,omitempty"`
	InlineData *gmInlineData `json:"inlineData,omitempty"`
}
type gmInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}
type gmContent struct {
	Role  string   `json:"role,omitempty"`
	Parts []gmPart `json:"parts"`
}
type gmRequest struct {
	Contents         []gmContent    `json:"contents"`
	GenerationConfig map[string]any `json:"generationConfig"`
}
type gmResponse struct {
	Candidates []struct {
		Content      gmContent `json:"content"`
		FinishReason string    `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// segOut = bentuk segmen yang diminta dari model, detik relatif potongan.
type segOut struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

var segSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"segments": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"start": map[string]any{"type": "number"},
					"end":   map[string]any{"type": "number"},
					"text":  map[string]any{"type": "string"},
				},
				"required": []string{"start", "end", "text"},
			},
		},
	},
	"required": []string{"segments"},
}

// Transcribe mentranskripsi audio sepanjang totalSec detik, potongan demi
// potongan, berurutan. onProgress (boleh nil) menerima fraksi 0..1.
func (a *AIStudio) Transcribe(ctx context.Context, totalSec float64, language, tmpDir string, onProgress func(float64)) (types.Transcript, error) {
	if err := a.Available(); err != nil {
		return types.Transcript{}, err
	}
	if totalSec <= 0 {
		return types.Transcript{}, fmt.Errorf("AI Studio transcription: the audio length is unknown")
	}
	return chunked(ctx, totalSec, tmpDir, a.Encode, onProgress, func(ctx context.Context, audio []byte) ([]segOut, error) {
		segs, err := a.chunk(ctx, audio, language)
		if err != nil {
			return nil, fmt.Errorf("AI Studio (%s): %w", a.Model, err)
		}
		return segs, nil
	}, language)
}

// chunked memotong audio per AIStudioChunkSec, mengodekan tiap potongan lewat
// encode, mengirimnya lewat send, lalu menyusun segmennya ke waktu mutlak.
// Dipakai bersama AI Studio dan mesin OpenAI-compatible (openai.go).
func chunked(ctx context.Context, totalSec float64, tmpDir string,
	encode func(ctx context.Context, start, dur float64, out string) error,
	onProgress func(float64), send func(ctx context.Context, audio []byte) ([]segOut, error), language string) (types.Transcript, error) {
	tr := types.Transcript{Language: language}
	n := int(math.Ceil(totalSec / AIStudioChunkSec))
	for i := 0; i < n; i++ {
		start := float64(i) * AIStudioChunkSec
		dur := math.Min(AIStudioChunkSec, totalSec-start)
		out := filepath.Join(tmpDir, fmt.Sprintf("chunk_%03d.ogg", i))
		if err := encode(ctx, start, dur, out); err != nil {
			return types.Transcript{}, err
		}
		audio, err := os.ReadFile(out)
		_ = os.Remove(out)
		if err != nil {
			return types.Transcript{}, err
		}
		segs, err := send(ctx, audio)
		if err != nil {
			return types.Transcript{}, fmt.Errorf("part %d of %d: %w", i+1, n, err)
		}
		tr.Segments = append(tr.Segments, placeSegments(segs, start, dur)...)
		if onProgress != nil {
			onProgress(float64(i+1) / float64(n))
		}
	}
	return tr, nil
}

// chunk mengirim satu potongan dan mengembalikan segmen relatifnya.
func (a *AIStudio) chunk(ctx context.Context, audio []byte, language string) ([]segOut, error) {
	prompt := fmt.Sprintf("Transcribe this audio verbatim in its spoken language (expected language code: %q). "+
		"Do not translate, summarise or correct grammar. Split it into short segments of one sentence or "+
		"phrase each (at most about 10 seconds). For each segment give start and end in seconds from the "+
		"beginning of THIS audio clip, and the exact words. Skip silence, music and noise.", language)
	body, _ := json.Marshal(gmRequest{
		Contents: []gmContent{{Role: "user", Parts: []gmPart{
			{Text: prompt},
			{InlineData: &gmInlineData{MimeType: "audio/ogg", Data: base64.StdEncoding.EncodeToString(audio)}},
		}}},
		GenerationConfig: map[string]any{
			"temperature":      0,
			"responseMimeType": "application/json",
			"responseSchema":   segSchema,
		},
	})
	base := strings.TrimRight(a.Base, "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}
	url := base + "/v1beta/models/" + a.Model + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-goog-api-key", a.Key)
	hc := a.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Minute, Transport: httpx.Retry(nil)}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AI Studio is unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var gr gmResponse
	if err := json.Unmarshal(raw, &gr); err != nil {
		return nil, fmt.Errorf("the reply could not be read (status %d): %s", resp.StatusCode, clip(string(raw), 200))
	}
	if gr.Error != nil {
		return nil, fmt.Errorf("AI Studio refused the request (status %d): %s", resp.StatusCode, clip(gr.Error.Message, 200))
	}
	if len(gr.Candidates) == 0 {
		return nil, fmt.Errorf("AI Studio returned no answer (status %d)", resp.StatusCode)
	}
	c := gr.Candidates[0]
	var text strings.Builder
	for _, p := range c.Content.Parts {
		text.WriteString(p.Text)
	}
	var parsed struct {
		Segments []segOut `json:"segments"`
	}
	if err := json.Unmarshal([]byte(text.String()), &parsed); err != nil {
		return nil, fmt.Errorf("the transcript JSON could not be read (finish reason %s): %w", c.FinishReason, err)
	}
	return parsed.Segments, nil
}

// placeSegments merapikan segmen satu potongan lalu menggesernya ke waktu
// mutlak: dijepit ke batas potongan, teks kosong dibuang, diurutkan, dan
// tumpang-tindih dipotong (segmen berikutnya mulai paling cepat di akhir yang
// sebelumnya).
func placeSegments(segs []segOut, offset, dur float64) []types.TranscriptSegment {
	sort.SliceStable(segs, func(i, j int) bool { return segs[i].Start < segs[j].Start })
	var out []types.TranscriptSegment
	prevEnd := 0.0
	for _, s := range segs {
		t := strings.TrimSpace(s.Text)
		if t == "" {
			continue
		}
		st := math.Max(math.Min(s.Start, dur), prevEnd)
		en := math.Min(s.End, dur)
		if en <= st {
			continue
		}
		prevEnd = en
		seg := types.TranscriptSegment{Start: offset + st, End: offset + en, Text: t}
		seg.Words = spreadWords(t, seg.Start, seg.End)
		out = append(out, seg)
	}
	return out
}

// spreadWords membagi durasi segmen ke kata-katanya SEBANDING panjang hurufnya.
//
// Ini PERKIRAAN: Gemini tidak memberi waktu per kata yang bisa dipercaya, jadi
// sinkron mode karaoke/word dengan transkrip AI Studio lebih kasar daripada
// whisper (yang memberi waktu per token). Kata panjang mendapat jatah lebih,
// dan jeda di dalam satu segmen tidak terwakili.
func spreadWords(text string, start, end float64) []types.Word {
	fields := strings.Fields(text)
	total := 0
	for _, f := range fields {
		total += utf8.RuneCountInString(f)
	}
	if total == 0 {
		return nil
	}
	words := make([]types.Word, 0, len(fields))
	t, span := start, end-start
	for _, f := range fields {
		d := span * float64(utf8.RuneCountInString(f)) / float64(total)
		words = append(words, types.Word{Start: t, End: t + d, Text: f})
		t += d
	}
	words[len(words)-1].End = end
	return words
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
