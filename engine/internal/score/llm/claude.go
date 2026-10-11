// Package llm memanggil Claude API untuk memilih & menilai momen klip.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/httpx"
	"github.com/gemgum/clipper/engine/internal/types"
)

// defaultBase = alamat resmi Claude. Bisa ditimpa lewat Client.BaseURL supaya
// pengguna bisa menunjuk ke gateway/proxy sendiri tanpa kode baru (notes/39).
const defaultBase = "https://api.anthropic.com"

// Client Claude API.
type Client struct {
	APIKey string
	Model  string
	HTTP   *http.Client
	// Temperature dikirim hanya bila > 0; selain itu dipakai bawaan API.
	// Tugas menyalin-ulang seperti koreksi transkrip butuh angka rendah.
	Temperature float64
	// BaseURL menimpa alamat resmi. Kosong = defaultBase.
	BaseURL string
}

// endpoint menyusun alamat penuh /v1/messages untuk klien ini.
func (c *Client) endpoint() string {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		base = defaultBase
	}
	return base + "/v1/messages"
}

func New(apiKey, model string) *Client {
	if model == "" {
		model = "claude-haiku-4-5"
	}
	return &Client{
		APIKey: apiKey,
		Model:  model,
		HTTP:   &http.Client{Timeout: 120 * time.Second, Transport: httpx.Retry(nil)},
	}
}

// anthropic request/response shapes.
type msgReq struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	System      string    `json:"system"`
	Messages    []anthMsg `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
}
type anthMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type msgResp struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// MomentReasons rincian skor (float agar toleran output model lokal).
type MomentReasons struct {
	Hook         float64 `json:"hook"`
	Emotion      float64 `json:"emotion"`
	Clarity      float64 `json:"clarity"`
	Shareability float64 `json:"shareability"`
	Standalone   float64 `json:"standalone"`
}

// Complete mengirim satu pasang prompt dan mengembalikan teks balasan apa
// adanya. Ini lapisan HTTP telanjang yang dipakai bersama oleh pemilihan momen
// klip dan pemilihan paragraf berita — keduanya cuma berbeda prompt.
//
// Seperti SelectMoments: setiap kegagalan dikembalikan apa adanya, TIDAK ada
// perpindahan diam-diam ke mesin lain (lihat notes/12).
func (c *Client) Complete(ctx context.Context, system, user string, maxTokens int) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("the Claude API key is empty: paste it under Settings → Engines & keys")
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	reqBody := msgReq{
		Model:     c.Model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  []anthMsg{{Role: "user", Content: user}},
	}
	if c.Temperature > 0 {
		reqBody.Temperature = &c.Temperature
	}
	buf, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("Claude is unreachable (check your internet connection): %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var parsed msgResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("the Claude response could not be read (status %d): %s", resp.StatusCode, truncate(string(raw), 200))
	}
	if parsed.Error != nil {
		return "", claudeError(resp.StatusCode, parsed.Error.Message)
	}
	if len(parsed.Content) == 0 {
		return "", fmt.Errorf("Claude returned an empty reply (status %d)", resp.StatusCode)
	}
	return parsed.Content[0].Text, nil
}

// claudeError menerjemahkan pesan API jadi petunjuk yang bisa ditindaklanjuti.
func claudeError(status int, msg string) error {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "x-api-key") || status == 401:
		return fmt.Errorf("the Claude API key was rejected: update it in the AI engine panel (%s)", msg)
	case status == 429:
		return fmt.Errorf("Claude quota/rate limit exceeded: wait a moment and try again (%s)", msg)
	case strings.Contains(low, "model"):
		return fmt.Errorf("unknown Claude model: pick a different model in the GUI (%s)", msg)
	case status == 400 && strings.Contains(low, "credit"):
		return fmt.Errorf("the Claude API balance is exhausted (%s)", msg)
	}
	return fmt.Errorf("Claude rejected the request (status %d): %s", status, msg)
}

// extractBlock mengambil blok pertama..terakhir di antara pembuka & penutup.
func extractBlock(s string, open, close byte) string {
	i := strings.IndexByte(s, open)
	j := strings.LastIndexByte(s, close)
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return ""
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// PickMoments meminta Claude MEMILIH dari kandidat bernomor, bukan mengarang
// batas waktu. Lihat pick.go untuk alasannya.
func (c *Client) PickMoments(ctx context.Context, cands []types.Candidate, offset, maxClips int, contentLang string) ([]Pick, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("the Claude API key is empty: paste it under Settings → Engines & keys")
	}
	text, err := c.Complete(ctx, PickSystemPrompt(maxClips, contentLang), PickUserPrompt(cands, offset), 4096)
	if err != nil {
		return nil, err
	}
	var wrap PickResponse
	if err := json.Unmarshal([]byte(extractBlock(text, '{', '}')), &wrap); err != nil {
		return nil, fmt.Errorf("Claude (%s) returned JSON that could not be read: %w; reply: %s",
			c.Model, err, truncate(text, 300))
	}
	return wrap.Picks, nil
}
