package transcribe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/httpx"
	"github.com/gemgum/clipper/engine/internal/types"
)

// Transkripsi lewat mesin OpenAI-compatible dari halaman Engines & Keys:
// POST {Base}{Path}/audio/transcriptions (OpenAI, gateway pengguna, Groq, …).
//
// Seperti AI Studio: dipilih, bukan cadangan whisper — bila gagal, job berhenti
// dengan pesan sebabnya (notes/12). Model WAJIB mengembalikan segmen
// bertimestamp (response_format=verbose_json); model yang hanya membalas teks
// ditolak dengan pesan, bukan dijadikan satu subtitle sepanjang 10 menit.

// OpenAIDefaultModel dipakai bila model transkripsi belum dipilih: satu-satunya
// nama yang dikenal hampir semua penyedia yang meniru API audio OpenAI.
const OpenAIDefaultModel = "whisper-1"

// OpenAIAudio memanggil endpoint transkripsi OpenAI-compatible.
type OpenAIAudio struct {
	Name  string // untuk pesan galat, mis. "Custom (OpenAI-compatible)"
	Base  string // tanpa /v1 (lihat normalizeBase di paket api)
	Path  string // "/v1"
	Key   string
	Model string
	HTTP  *http.Client
	// Encode: sama dengan AIStudio.Encode.
	Encode func(ctx context.Context, start, dur float64, out string) error
}

// Available memastikan alamat & kunci ada SEBELUM audio diekstrak.
func (o *OpenAIAudio) Available() error {
	if o.Base == "" || o.Key == "" {
		return fmt.Errorf("%s has no address or API key yet: fill it in on the Engines & Keys page", o.Name)
	}
	return nil
}

// Transcribe: lihat AIStudio.Transcribe.
func (o *OpenAIAudio) Transcribe(ctx context.Context, totalSec float64, language, tmpDir string, onProgress func(float64)) (types.Transcript, error) {
	if err := o.Available(); err != nil {
		return types.Transcript{}, err
	}
	if totalSec <= 0 {
		return types.Transcript{}, fmt.Errorf("%s transcription: the audio length is unknown", o.Name)
	}
	return chunked(ctx, totalSec, tmpDir, o.Encode, onProgress, func(ctx context.Context, audio []byte) ([]segOut, error) {
		segs, err := o.chunk(ctx, audio, language)
		if err != nil {
			return nil, fmt.Errorf("%s (%s): %w", o.Name, o.model(), err)
		}
		return segs, nil
	}, language)
}

func (o *OpenAIAudio) model() string {
	if o.Model == "" {
		return OpenAIDefaultModel
	}
	return o.Model
}

func (o *OpenAIAudio) chunk(ctx context.Context, audio []byte, language string) ([]segOut, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "audio.ogg")
	if err != nil {
		return nil, err
	}
	fw.Write(audio)
	mw.WriteField("model", o.model())
	mw.WriteField("response_format", "verbose_json")
	mw.WriteField("timestamp_granularities[]", "segment")
	if language != "" {
		mw.WriteField("language", language)
	}
	mw.Close()

	url := strings.TrimRight(o.Base, "/") + o.Path + "/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", mw.FormDataContentType())
	req.Header.Set("authorization", "Bearer "+o.Key)
	hc := o.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Minute, Transport: httpx.Retry(nil)}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s is unreachable: %w", url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Text     string   `json:"text"`
		Segments []segOut `json:"segments"`
		Error    *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("the reply could not be read (status %d): %s", resp.StatusCode, clip(string(raw), 200))
	}
	if out.Error != nil || resp.StatusCode >= 300 {
		msg := clip(string(raw), 200)
		if out.Error != nil {
			msg = clip(out.Error.Message, 200)
		}
		return nil, fmt.Errorf("%s refused the request (status %d): %s", url, resp.StatusCode, msg)
	}
	if len(out.Segments) == 0 && strings.TrimSpace(out.Text) != "" {
		return nil, fmt.Errorf("this model returned text without timestamps, so subtitles cannot be timed: pick a model that supports verbose_json, such as %s", OpenAIDefaultModel)
	}
	return out.Segments, nil
}
