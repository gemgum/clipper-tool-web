// Package ollama memakai LLM lokal via Ollama (mis. Qwen) untuk scoring offline.
package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/httpx"
	"github.com/gemgum/clipper/engine/internal/score/llm"
	"github.com/gemgum/clipper/engine/internal/types"
)

const defaultURL = "http://localhost:11434"

// numCtx = jendela konteks yang diminta engine per potongan transkrip. Dipakai
// juga saat menilai model terpasang: model dengan konteks maksimum di bawah ini
// akan memotong prompt diam-diam.
const numCtx = 8192

// minCtx = konteks terkecil yang masih masuk akal untuk satu segmen transkrip
// beserta prompt sistemnya. Di bawah ini modelnya memang tidak bisa dipakai.
const minCtx = 2048

// minParams = batas bawah ukuran model (miliar parameter) yang masih sanggup
// mengerjakan prompt pemilihan momen. Dari uji di notes/12: model 4B membalas
// JSON valid tapi isian kosong, sedangkan 7B mengerjakannya dengan benar.
const minParams = 6.5

// Client memanggil Ollama.
type Client struct {
	URL   string
	Model string
	HTTP  *http.Client
	// Temperature menimpa nilai bawaan bila > 0. Tugas menyalin-ulang seperti
	// koreksi transkrip butuh angka jauh lebih rendah daripada tugas kreatif
	// seperti memilih momen — pada 0.4 keluarannya berbeda-beda tiap kali.
	Temperature float64
	// NumCtx = konteks maksimum model INI, dari metadata Ollama. 0 = tidak tahu.
	NumCtx int
	// Kind = jenis server: KindOllama (bawaan) atau KindOpenAI untuk apa pun
	// yang bicara /v1/chat/completions — llama.cpp, LocalAI, llamafile, vLLM,
	// Aphrodite, LiteLLM, Exo. Lihat openai.go.
	Kind string
	// Path = awalan jalur di bawah URL, tanpa garis miring di ujung. Kosong
	// berarti "/v1", yang benar untuk hampir semua server.
	//
	// Ada karena endpoint OpenAI-compatible milik Gemini bukan "/v1" melainkan
	// "/v1beta/openai" — dan tanpa field ini, satu-satunya jalan memakainya
	// adalah klien kedua yang isinya sama persis.
	Path string
	// APIKey = kunci untuk server INI. Kosong berarti pakai LLM_API_KEY.
	//
	// Dulu semua server berbagi satu variabel global, dan itu cukup selama
	// isinya cuma server lokal di belakang gateway. Dengan beberapa penyedia
	// cloud sekaligus, tiap klien harus membawa kuncinya sendiri.
	APIKey string
}

// path mengembalikan awalan jalur API yang berlaku untuk klien ini.
func (c *Client) path() string {
	if p := strings.TrimRight(strings.TrimSpace(c.Path), "/"); p != "" {
		return p
	}
	return "/v1"
}

// key mengembalikan kunci yang berlaku untuk klien ini.
func (c *Client) key() string {
	if k := strings.TrimSpace(c.APIKey); k != "" {
		return k
	}
	return apiKey()
}

// temperature mengembalikan suhu yang berlaku untuk klien ini.
func (c *Client) temperature() float64 {
	if c.Temperature > 0 {
		return c.Temperature
	}
	return defaultTemperature
}

// defaultTemperature dipakai bila Client.Temperature tidak diisi.
const defaultTemperature = 0.4

// chatTimeout: 12 menit, bukan 5.
//
// Yang dihitung bukan waktu berpikirnya melainkan waktu MEMUAT: model 8B di
// mesin tanpa GPU bisa perlu belasan menit untuk masuk memori pada permintaan
// pertama, dan selama itu Ollama belum mengirim satu header pun. Dilaporkan
// sebagai kegagalan di lapangan; 5 menit terlalu ketat.
const chatTimeout = 12 * time.Minute

// ctxFor = jendela konteks yang diminta ke Ollama untuk satu panggilan.
//
// Bukan angka tetap, dan dua batasnya berlawanan arah.
//
// Batas ATAS: model kecil sering hanya mendukung 4096 atau 2048, dan meminta
// lebih membuat Ollama memuat model dengan konteks yang tidak bisa dipenuhi —
// balasannya kosong, tanpa satu pun pesan galat. NumCtx diisi pemanggil dari
// metadata model (lihat pipeline/ollama_resolve.go dan api.EngineFor); 0 berarti
// "tidak tahu", dan di situ tidak ada yang boleh dinaikkan.
//
// Batas BAWAH: promptChars = panjang system+user, numPredict = keluaran yang
// diminta, dan keduanya harus MUAT dalam satu jendela. Meminta 8192 token
// keluaran di dalam jendela 8192 berarti promptnya sendiri sudah memakan jatah,
// dan Ollama berhenti diam-diam di dinding konteks — balasannya JSON terpotong
// tanpa satu pun pesan galat. Terlihat pertama kali saat pembuat berita meminta
// artikel utuh (18 Agustus 2026). ~3 karakter per token cukup untuk menaksir.
// Keduanya harus MUAT dalam satu jendela: meminta 8192 token keluaran di dalam
// jendela 8192 berarti promptnya sendiri sudah memakan jatah, dan Ollama
// berhenti diam-diam di dinding konteks — balasannya JSON terpotong tanpa satu
// pun pesan galat. Terlihat pertama kali saat pembuat berita meminta artikel
// utuh (18 Agustus 2026). ~3 karakter per token cukup untuk menaksir prompt.
func (c *Client) ctxFor(promptChars, numPredict int) int {
	want := max(numCtx, numPredict+promptChars/3+256)
	if c.NumCtx >= 512 {
		return min(want, c.NumCtx)
	}
	// Kemampuan model tidak diketahui: JANGAN naikkan. Meminta 16k pada model
	// yang cuma sanggup 4k adalah kegagalan yang lebih buruk — balasan kosong.
	return numCtx
}

func New(url, model string) *Client {
	if url == "" {
		url = defaultURL
	}
	if model == "" {
		model = "qwen2.5"
	}
	return &Client{URL: strings.TrimRight(url, "/"), Model: model, HTTP: &http.Client{Timeout: chatTimeout, Transport: httpx.Retry(nil)}}
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type chatReq struct {
	Model    string         `json:"model"`
	Messages []chatMsg      `json:"messages"`
	Stream   bool           `json:"stream"`
	Format   any            `json:"format,omitempty"` // "json" atau JSON Schema
	Options  map[string]any `json:"options,omitempty"`
}
type chatResp struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Error string `json:"error"`
}

// Complete mengirim satu pasang prompt ke Ollama dan mengembalikan isi balasan.
//
// schema (boleh nil) diteruskan ke parameter "format" Ollama sehingga bentuk
// balasan dijamin di sisi server — inilah yang membuat model lokal tetap
// membalas JSON rapi walau promptnya panjang.
func (c *Client) Complete(ctx context.Context, system, user string, schema any, numPredict int) (string, error) {
	if numPredict <= 0 {
		numPredict = 2048
	}
	if c.Kind == KindOpenAI {
		return c.completeOpenAI(ctx, system, user, schema, numPredict)
	}
	temperature := c.temperature()
	reqBody := chatReq{
		Model: c.Model,
		Messages: []chatMsg{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Stream: false,
		Format: schema,
		Options: map[string]any{"temperature": temperature,
			"num_ctx": c.ctxFor(len(system)+len(user), numPredict), "num_predict": numPredict},
	}
	buf, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/api/chat", bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", dialError(c.URL, c.Model, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var cr chatResp
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("the Ollama response could not be read (status %d): %s", resp.StatusCode, trunc(string(raw), 200))
	}
	if cr.Error != "" {
		return "", ollamaError(c.Model, cr.Error)
	}
	return cr.Message.Content, nil
}

// dialError membedakan "tidak bisa dihubungi" dari "menjawab terlalu lambat".
//
// Keduanya dulu memakai kalimat yang sama — "Ollama is unreachable, run `ollama
// serve`" — dan itu menyesatkan: pada laporan yang memicu perbaikan ini, Ollama
// JALAN, ia cuma butuh lebih lama daripada batas waktunya untuk memuat model ke
// memori. Pengguna diarahkan memeriksa hal yang sudah benar.
func dialError(url, model string, err error) error {
	if isTimeout(err) {
		return fmt.Errorf("Ollama at %s did not answer within %s. It is running, but the model %q is probably still being loaded into memory: that is normal on the first request after starting. Wait and try again, or pick a smaller model", url, chatTimeout, model)
	}
	return fmt.Errorf("Ollama is unreachable at %s: make sure it is installed and run `ollama serve`: %w", url, err)
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// Ping menyapa model dengan satu kata dan mengembalikan balasannya.
//
// Gunanya dua, dan yang kedua yang menentukan: ia MEMBUKTIKAN model benar-benar
// bisa menjawab (bukan sekadar terdaftar di `ollama list`), DAN ia memuat model
// itu ke memori. Sesudah Ping berhasil, pekerjaan sungguhan tidak lagi
// menanggung waktu muat yang panjang itu diam-diam di dalam batas waktunya.
func (c *Client) Ping(ctx context.Context) (string, time.Duration, error) {
	start := time.Now()
	out, err := c.Complete(ctx, "Reply with one short word.", "halo", nil, 16)
	return strings.TrimSpace(out), time.Since(start), err
}

// ollamaError menerjemahkan pesan Ollama jadi petunjuk yang bisa ditindaklanjuti.
func ollamaError(model, msg string) error {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "not found") || strings.Contains(low, "no such model"):
		return fmt.Errorf("model %q is not installed in Ollama: click \"download model\" in the GUI or run `ollama pull %s`", model, model)
	case strings.Contains(low, "format"):
		return fmt.Errorf("Ollama rejected the JSON schema: update Ollama to a version that supports structured output (%s)", msg)
	case strings.Contains(low, "memory") || strings.Contains(low, "out of"):
		return fmt.Errorf("not enough RAM/VRAM for model %q: use a smaller model (%s)", model, msg)
	}
	return fmt.Errorf("Ollama failed to process the request: %s", msg)
}

func trunc(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ModelInfo satu model terpasang di Ollama, sudah dinilai kelayakannya untuk
// tugas pemilihan momen — supaya GUI tinggal menampilkan, bukan menebak.
type ModelInfo struct {
	Name    string `json:"name"`    // nama lengkap dengan tag, mis. "qwen2.5:latest"
	Base    string `json:"base"`    // nama tanpa tag, mis. "qwen2.5"
	Params  string `json:"params"`  // mis. "7.6B" (apa adanya dari Ollama)
	Quant   string `json:"quant"`   // mis. "Q4_K_M"
	Bytes   int64  `json:"bytes"`   // ukuran berkas model
	Context int    `json:"context"` // konteks maksimum yang didukung model
	Ready   bool   `json:"ready"`   // sanggup mengerjakan prompt pemilihan momen
	Note    string `json:"note"`    // alasan bila tidak siap
}

// StatusInfo hasil pengecekan Ollama.
type StatusInfo struct {
	Running bool `json:"running"`
	// URL yang benar-benar menjawab, dan penjelasannya dalam satu kalimat.
	// Keduanya ditampilkan GUI: "Ollama jalan" tanpa menyebut DI MANA membuat
	// susunan Windows+WSL mustahil didiagnosis.
	URL   string `json:"url"`
	Where string `json:"where"`
	// OS = satu kata: "Windows", "WSL", "Linux", "macOS", "remote".
	OS string `json:"os"`
	// Kind = "ollama" atau "openai" (llama.cpp, LocalAI, vLLM, …).
	Kind string `json:"kind"`
	// Server = nama yang dikenali pengguna ("LM Studio", "Jan", "Ollama"),
	// bukan jenis protokolnya. Dipakai GUI sebagai label; "openai" tidak
	// berarti apa-apa bagi orang yang memasang LM Studio.
	Server string `json:"server"`
	// Models = daftar nama lengkap; dipertahankan agar pemakai lama tetap jalan.
	Models    []string    `json:"models"`
	Installed []ModelInfo `json:"installed"`
	// Servers = SEMUA server LLM yang menjawab di komputer ini, bukan cuma yang
	// dipakai. Itu yang membuat GUI bisa menawarkan pilihan alih-alih menuntut
	// pengguna mengetik alamat server keduanya dari ingatan.
	Servers []Endpoint `json:"servers"`
}

// Status memeriksa apakah Ollama jalan & daftar model terpasang, lengkap dengan
// penilaian siap/tidaknya tiap model.
func Status(ctx context.Context, url string) StatusInfo {
	// Alamat kosong berarti "cari sendiri" — bukan "pakai localhost". Localhost
	// hanya benar bila servernya ada di sistem yang sama, dan itu justru susunan
	// yang paling sering TIDAK berlaku di Windows (lihat discover.go).
	ep := Endpoint{URL: strings.TrimRight(url, "/"), Kind: KindOllama}
	if url == "" {
		ep = DiscoverEndpoint(ctx)
	} else if k := kindOf(ctx, ep.URL); k != "" {
		ep.Kind = k
	}
	if ep.URL == "" {
		return StatusInfo{Running: false, Where: Where("")}
	}
	// Server bergaya OpenAI: metadatanya cuma daftar nama (lihat openai.go).
	if ep.Kind == KindOpenAI {
		models := openAIModels(ctx, ep.URL)
		info := StatusInfo{Running: true, URL: ep.URL, Where: Where(ep.URL), OS: OS(ep.URL),
			Kind: ep.Kind, Server: serverName(ep.URL, ep.Kind), Servers: DiscoverAll(ctx)}
		for _, m := range models {
			info.Models = append(info.Models, m.Name)
		}
		info.Installed = models
		return info
	}
	url = ep.URL
	cl := &http.Client{Timeout: 3 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url+"/api/tags", nil)
	resp, err := cl.Do(req)
	if err != nil {
		return StatusInfo{Running: false}
	}
	defer resp.Body.Close()
	var parsed struct {
		Models []struct {
			Name    string `json:"name"`
			Size    int64  `json:"size"`
			Details struct {
				ParameterSize     string `json:"parameter_size"`
				QuantizationLevel string `json:"quantization_level"`
				ContextLength     int    `json:"context_length"`
			} `json:"details"`
			Capabilities []string `json:"capabilities"`
		} `json:"models"`
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(raw, &parsed)
	info := StatusInfo{Running: true, URL: url, Where: Where(url), OS: OS(url),
		Kind: KindOllama, Server: serverName(url, KindOllama), Servers: DiscoverAll(ctx)}
	for _, m := range parsed.Models {
		mi := ModelInfo{
			Name:    m.Name,
			Base:    BaseName(m.Name),
			Params:  m.Details.ParameterSize,
			Quant:   m.Details.QuantizationLevel,
			Bytes:   m.Size,
			Context: m.Details.ContextLength,
		}
		mi.Ready, mi.Note = judge(m.Name, m.Size, m.Details.ParameterSize, m.Details.ContextLength, m.Capabilities)
		info.Models = append(info.Models, m.Name)
		info.Installed = append(info.Installed, mi)
	}
	return info
}

// ContextOf menyebut konteks maksimum satu model menurut servernya, 0 bila
// tidak diketahui. Pemanggil mengisikannya ke Client.NumCtx — tanpa itu jendela
// terkunci di angka bawaan, dan permintaan keluaran besar terpotong diam-diam.
//
// Nama dicocokkan tanpa tag: yang tersimpan di setelan bisa "llama3.1"
// sementara yang terpasang bernama "llama3.1:latest".
func ContextOf(ctx context.Context, url, model string) int {
	for _, m := range Status(ctx, url).Installed {
		if m.Name == model || BaseName(m.Name) == BaseName(model) {
			return m.Context
		}
	}
	return 0
}

// BaseName membuang tag Ollama: "qwen2.5:latest" → "qwen2.5".
func BaseName(name string) string {
	if i := strings.IndexByte(name, ':'); i > 0 {
		return name[:i]
	}
	return name
}

// judge menilai apakah sebuah model terpasang layak dipakai memilih momen.
func judge(name string, bytes int64, paramSize string, ctxLen int, caps []string) (bool, string) {
	// Model CLOUD Ollama ("gpt-oss:120b-cloud") terdaftar seperti model biasa
	// tapi berukuran 0 byte: ia berjalan di server Ollama, butuh akun, dan
	// bukan model lokal. Dilaporkan dari lapangan: ia tampil "ready" lalu
	// jobnya gagal di tengah jalan.
	if strings.HasSuffix(name, "-cloud") || bytes == 0 {
		return false, "runs on Ollama's servers, not on this computer: it needs an Ollama account, and Clipper is built around a local model"
	}
	if len(caps) > 0 && !slices.Contains(caps, "completion") {
		return false, "this model does not generate text (e.g. an embedding model)"
	}
	// Konteks kecil TIDAK lagi menolak modelnya: potongan transkrip yang
	// balasannya gagal dibaca kini dipecah sampai model sanggup (lihat
	// correct.Correct), jadi model berkonteks 4096 tetap bisa dipakai — hanya
	// lebih lambat. Yang benar-benar tidak masuk akal adalah di bawah minCtx.
	if ctxLen > 0 && ctxLen < minCtx {
		return false, fmt.Sprintf("maximum context is only %d tokens: too small even for one transcript segment", ctxLen)
	}
	if ctxLen > 0 && ctxLen < numCtx {
		return true, fmt.Sprintf("small context (%d tokens): Clipper will send smaller pieces, which is slower", ctxLen)
	}
	// Model KECIL (1B–4B) tidak lagi ditolak, hanya ditandai.
	//
	// Dulu ditolak karena diuji membalas isian kosong saat memilih momen
	// (notes/12). Itu masih benar — tapi menolaknya berarti pengguna yang hanya
	// punya mesin kecil tidak bisa memakai apa pun, padahal koreksi transkrip
	// kini menyesuaikan diri dengan memecah potongan, dan mesin skor heuristik
	// selalu tersedia sebagai gantinya.
	if b := parseParams(paramSize); b > 0 && b < minParams {
		return true, fmt.Sprintf("small model (%s): fine for transcript correction, but it often returns empty fields when picking moments; the built-in heuristic is the safer choice there", paramSize)
	}
	return true, ""
}

// parseParams mengubah "7.6B" → 7.6 dan "270M" → 0.27. Bentuk MoE seperti
// "8x7B" diambil angka setelah "x" (yang aktif per token). 0 = tak diketahui.
func parseParams(s string) float64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	if i := strings.LastIndexByte(s, 'X'); i >= 0 {
		s = s[i+1:]
	}
	mult := 1.0
	switch {
	case strings.HasSuffix(s, "B"):
		s = strings.TrimSuffix(s, "B")
	case strings.HasSuffix(s, "M"):
		s, mult = strings.TrimSuffix(s, "M"), 0.001
	default:
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v * mult
}

// Pull mengunduh model lewat Ollama (bisa lama). Mengembalikan error bila gagal.
func Pull(ctx context.Context, url, model string, onProgress func(PullProgress)) error {
	if url == "" {
		url = defaultURL
	}
	if onProgress == nil {
		onProgress = func(PullProgress) {}
	}
	url = strings.TrimRight(url, "/")
	// stream:true — DIALIRKAN, bukan satu balasan di akhir.
	//
	// Dengan stream:false unduhan 5 GB adalah satu permintaan HTTP yang diam
	// belasan menit: tidak ada persentase, tidak ada cara membedakan "sedang
	// jalan" dari "sudah mati". Terjadi sungguhan 7 Agustus 2026 — gemma2
	// berhenti di 5,44 GB dan berkas parsialnya tergeletak 14 menit tanpa ada
	// satu pun tempat yang memberitahu.
	body, _ := json.Marshal(map[string]any{"name": model, "stream": true})
	cl := &http.Client{Timeout: 60 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url+"/api/pull", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/json")
	resp, err := cl.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama is unreachable (download failed): %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("model download failed (status %d): %s", resp.StatusCode, trunc(string(raw), 200))
	}

	// Tiap baris satu objek JSON. Baris terakhir yang berisi "error" menang atas
	// apa pun sebelumnya: Ollama melaporkan kemajuan lebih dulu, baru gagal.
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	seen := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var p PullProgress
		if json.Unmarshal([]byte(line), &p) != nil {
			continue
		}
		if p.Error != "" {
			return fmt.Errorf("model download failed: %s", p.Error)
		}
		seen = true
		onProgress(p)
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("the download stopped early: %w", err)
	}
	if !seen {
		return fmt.Errorf("Ollama at %s said nothing about the download of %q", url, model)
	}
	return nil
}

// PullProgress = satu baris kemajuan dari /api/pull.
//
// Completed/Total hanya terisi selama lapisan berkas diunduh; baris lain
// ("verifying sha256 digest", "success") cuma membawa Status. Karena itu GUI
// menampilkan Status apa adanya dan persentasenya hanya bila Total > 0 —
// bilangan 0% yang menggantung di tahap verifikasi terbaca seperti macet.
type PullProgress struct {
	Status    string `json:"status"`
	Digest    string `json:"digest"`
	Total     int64  `json:"total"`
	Completed int64  `json:"completed"`
	Error     string `json:"error"`
}

// PickMoments meminta model lokal MEMILIH dari kandidat bernomor.
//
// Inilah perubahan yang menghapus seluruh kelas kegagalan "angka waktu ngawur":
// model tidak lagi memegang timestamp, jadi rentang terbalik atau durasi 8 detik
// tidak mungkin lagi terjadi. Bentuk balasannya tetap dijamin JSON Schema.
func (c *Client) PickMoments(ctx context.Context, cands []types.Candidate, offset, maxClips int, contentLang string) ([]llm.Pick, error) {
	var content string
	var readErr error
	for attempt := 1; attempt <= pickAttempts; attempt++ {
		var err error
		content, err = c.Complete(ctx, llm.PickSystemPrompt(maxClips, contentLang),
			llm.PickUserPrompt(cands, offset), llm.PickSchema(maxClips), 2048)
		if err != nil {
			return nil, err
		}
		var wrap llm.PickResponse
		if readErr = json.Unmarshal([]byte(content), &wrap); readErr == nil {
			return wrap.Picks, nil
		}
	}
	return nil, fmt.Errorf("local model %s returned JSON that could not be read (%d attempts): %w; reply: %s",
		c.Model, pickAttempts, readErr, trunc(content, 300))
}

// pickAttempts: berapa kali satu bagian diminta ulang bila balasannya bukan
// JSON utuh. Model bisa terjebak mengulang frasa di dalam "title" ("(80
// seconds) (80 seconds) …") sampai balasannya terpotong, dan gateway yang
// mengabaikan maxLength skema tidak menghentikannya. Pada temperatur 0.4
// percobaan berikutnya hampir selalu lain. Mesin yang sama, permintaan yang
// sama — bukan cadangan ke mesin lain (notes/12).
const pickAttempts = 3
