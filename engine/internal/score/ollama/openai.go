package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gemgum/clipper/engine/internal/httpx"
)

// Server LLM lokal SELAIN Ollama.
//
// llama.cpp (server), LocalAI, llamafile, vLLM, Aphrodite, LiteLLM, Exo — semua
// menyediakan API yang sama: `/v1/chat/completions` bergaya OpenAI. Jadi yang
// dibutuhkan bukan tujuh integrasi, melainkan SATU: satu bentuk permintaan yang
// sudah disepakati semuanya, plus satu cara mengenali mana yang sedang jalan.
//
// Ollama tetap dipakai lewat API-nya sendiri (`/api/chat`), bukan lewat lapisan
// /v1-nya: hanya di sana bentuk balasan bisa dipaksa dengan JSON Schema penuh,
// dan justru itu yang membuat model lokal tetap membalas JSON rapi.

// Jenis server yang dikenali.
const (
	KindOllama = "ollama"
	KindOpenAI = "openai" // apa pun yang bicara /v1/chat/completions
)

// Endpoint = satu server LLM lokal yang menjawab.
type Endpoint struct {
	URL  string `json:"url"`
	Kind string `json:"kind"`
	// Name = nama yang dikenali pengguna ("Ollama", "LM Studio"). Ikut di sini
	// supaya daftar pilihan di GUI tidak perlu menerjemahkan port jadi nama
	// sendiri — dua tempat yang menebak hal yang sama pasti akan berbeda.
	Name string `json:"name"`
}

// openAIReq/openAIResp: bagian yang benar-benar dipakai saja. Server-server ini
// berbeda-beda di bagian lain (usage, logprobs, kolom tambahan), dan menuliskan
// semuanya berarti satu server yang menambah kolom bisa mematahkan parsing.
type openAIReq struct {
	Model          string         `json:"model"`
	Messages       []chatMsg      `json:"messages"`
	Temperature    float64        `json:"temperature"`
	MaxTokens      int            `json:"max_tokens,omitempty"`
	ResponseFormat map[string]any `json:"response_format,omitempty"`
	// Stream SELALU true. Bukan demi menampilkan teks sedikit-sedikit: gateway
	// di belakang Cloudflare memutus permintaan yang tidak mengirim satu byte
	// pun dalam 100 detik (status 524), dan model bernalar yang menjawab satu
	// potongan koreksi butuh lebih lama dari itu. Dengan stream, byte mengalir
	// sejak token pertama. Server yang mengabaikannya tetap membalas JSON utuh,
	// dan readReply membaca keduanya.
	Stream bool `json:"stream"`
}

type openAIResp struct {
	Choices []openAIChoice `json:"choices"`
	Error   any            `json:"error"`
}

type openAIChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	// Delta = potongan isi pada balasan stream; dikumpulkan ke Message.
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
	// FinishReason membedakan "modelnya memang tidak menjawab" dari
	// "jatahnya habis" — dan tanpa itu keduanya sampai ke pemanggil sebagai
	// balasan kosong yang sama.
	FinishReason string `json:"finish_reason"`
}

// completeOpenAI mengirim satu pasang prompt ke server bergaya OpenAI.
func (c *Client) completeOpenAI(ctx context.Context, system, user string, schema any, numPredict int) (string, error) {
	// Dua kelonggaran, dan keduanya harus bisa BERLAKU BERSAMA. Ditulis sebagai
	// dua penambalan berurutan, DeepSeek mematahkannya: ia menolak skema dulu,
	// lalu percobaan tanpa skema kehabisan jatah — dan penanganan jatah tidak
	// pernah terpakai karena percobaan itu sudah keburu dikembalikan.
	//
	// Yang dilonggarkan cuma CARA MEMINTA; mesin yang dipilih pengguna tetap
	// dipakai apa adanya (notes/12). Keduanya juga terbaca: tombol Test
	// melaporkan skema yang tidak dipaksakan.
	if schema != nil {
		if _, refuses := noSchema.Load(c.URL); refuses {
			schema = nil
		}
	}
	budget := numPredict
	if v, roomy := needsRoom.Load(c.roomKey()); roomy {
		budget = max(budget, v.(int))
	}

	// Paling banyak empat percobaan: sekali untuk skema, dua kali untuk jatah
	// yang dilipatkan bertahap, plus yang pertama. Batasnya ada supaya server
	// yang selalu menolak tidak dicoba selamanya.
	for i := 0; i < 4; i++ {
		out, err := c.postChat(ctx, system, user, schema, budget)
		if err == nil {
			return out, nil
		}
		switch {
		case schema != nil && isSchemaRefusal(err):
			// DeepSeek membalas "This response_format type is unavailable now"
			// dan MENOLAK seluruh permintaan. Dugaan lama — "server yang tidak
			// sanggup akan mengabaikan field ini" — ternyata salah, dan
			// akibatnya bukan balasan longgar melainkan job yang tidak jalan.
			noSchema.Store(c.URL, true)
			schema = nil
		case isBudgetExhausted(err) && budget < roomCap:
			// Model bernalar memakai jatah yang sama untuk berpikir dan
			// menjawab; pada masukan panjang ia habis sebelum satu huruf
			// jawaban keluar. Dilipatkan BERTAHAP sampai roomCap: sekali lipat
			// ternyata belum cukup untuk deepseek-v4-pro (2048 → 8192 masih
			// habis), dan angka yang dipakai diingat supaya potongan berikutnya
			// tidak mengulang percobaan yang sudah diketahui gagal.
			budget = withRoom(budget)
			needsRoom.Store(c.roomKey(), budget)
		default:
			return "", err
		}
	}
	return "", fmt.Errorf("%s kept refusing the request even after loosening the reply format and the token budget (model %s)", c.URL, c.Model)
}

// noSchema mengingat server yang menolak json_schema, per alamat.
var noSchema sync.Map

// needsRoom mengingat JATAH yang sudah terbukti dibutuhkan satu model, per
// alamat+model.
// Per MODEL, bukan per alamat: satu penyedia bisa menyajikan model bernalar dan
// model biasa sekaligus.
var needsRoom sync.Map

func (c *Client) roomKey() string { return c.URL + "|" + c.Model }

// roomFactor & roomCap: seberapa banyak ruang tambahan diberikan.
//
// Empat kali cukup untuk penalaran atas satu artikel berita; batas atasnya ada
// supaya angka masukan yang besar tidak berlipat jadi tagihan yang mengagetkan.
const (
	roomFactor = 4
	roomCap    = 32768
)

func withRoom(n int) int { return min(n*roomFactor, roomCap) }

// isBudgetExhausted mengenali balasan yang habis jatah SEBELUM menjawab.
func isBudgetExhausted(err error) bool {
	return strings.Contains(err.Error(), "before answering")
}

// SchemaEnforced melaporkan apakah server di alamat itu MENERIMA balasan
// berskema.
//
// Bedanya dengan "balasannya kebetulan berbentuk benar" nyata: tanpa skema,
// bentuk balasan cuma dijaga prompt, dan itu paling sering patah justru pada
// balasan panjang — persis yang diminta tahap menulis pembuat berita.
func SchemaEnforced(url string) bool {
	_, refuses := noSchema.Load(strings.TrimRight(url, "/"))
	return !refuses
}

// isSchemaRefusal membedakan penolakan KARENA response_format dari penolakan
// lain — kunci salah, model tidak ada, kuota habis. Menyamakan semuanya berarti
// mencoba ulang permintaan yang memang mustahil.
func isSchemaRefusal(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "response_format") || strings.Contains(msg, "json_schema")
}

// postChat mengirim satu permintaan chat.
func (c *Client) postChat(ctx context.Context, system, user string, schema any, numPredict int) (string, error) {
	req := openAIReq{
		Model: c.Model,
		Messages: []chatMsg{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: c.temperature(),
		MaxTokens:   numPredict,
		Stream:      true,
	}
	if schema != nil {
		// json_object tidak dipakai sebagai cadangan otomatis: sebagian server
		// MENOLAK permintaan yang memuat keduanya, dan promptnya sendiri sudah
		// meminta JSON.
		req.ResponseFormat = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "clipper",
				"schema": schema,
				"strict": true,
			},
		}
	}
	body, _ := json.Marshal(req)
	url := c.URL + c.path() + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("content-type", "application/json")
	// Sebagian server (LiteLLM, vLLM di belakang gateway) mensyaratkan header
	// ini walau kuncinya tidak diperiksa. Isinya dari LLM_API_KEY bila diisi.
	httpReq.Header.Set("authorization", "Bearer "+c.key())

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return "", dialError(c.URL, c.Model, err)
	}
	defer resp.Body.Close()
	parsed, raw, err := readReply(resp)
	if err != nil {
		// Alamat LENGKAP, bukan cuma base-nya: alamat yang salah isi (mis.
		// endpoint gaya Anthropic milik DeepSeek) membalas 404 berbadan kosong,
		// dan tanpa jalur penuh pesan itu tidak menunjukkan apa pun.
		return "", fmt.Errorf("the reply from %s could not be read (status %d): %v; reply: %s", url, resp.StatusCode, err, trunc(string(raw), 200))
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("%s refused the request: %s", url, trunc(errorMessage(parsed.Error), 200))
	}
	if len(parsed.Choices) == 0 {
		return "", nil // balasan kosong: pemanggil yang memutuskan artinya
	}
	out := parsed.Choices[0].Message.Content
	if parsed.Choices[0].FinishReason == "length" {
		// Jatah habis. Dilaporkan sebagai galat WALAU sebagian isinya sudah
		// keluar: yang diminta selalu JSON, dan JSON yang terpotong di tengah
		// sama tidak bergunanya dengan balasan kosong — bedanya cuma ia gagal
		// beberapa lapis kemudian, dengan pesan yang tidak menyebut sebabnya.
		//
		// Model BERNALAR (DeepSeek v4-pro, o-series, R1) memakai jatah yang sama
		// untuk berpikir dan untuk menjawab, jadi pada masukan panjang jatahnya
		// bisa habis sebelum satu huruf jawaban keluar.
		return "", fmt.Errorf("%s spent its whole %d-token budget before answering (model %s). Reasoning models need more room: pick a non-reasoning model, or send less text at a time",
			c.URL, numPredict, c.Model)
	}
	return out, nil
}

// errorMessage mengambil kalimat yang bisa dibaca dari objek galat penyedia.
//
// Bentuknya `{"error": {"message": "...", "code": ...}}` di hampir semua
// server. Dicetak apa adanya, ia jadi sintaks map Go ("map[code:... message:...]")
// — dan yang membaca pesan itu adalah pengguna, bukan pemrogram.
func errorMessage(v any) string {
	if m, ok := v.(map[string]any); ok {
		if s, ok := m["message"].(string); ok && s != "" {
			return s
		}
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// openAIModels membaca daftar model dari /v1/models.
//
// Bentuk minimum yang dijamin standar cuma `data[].id` — nama, tanpa spesifikasi
// apa pun. Tapi sebagian server memberi lebih: llama.cpp menyertakan `meta`
// berisi jumlah parameter, panjang konteks, ukuran, dan kuantisasi yang
// SEBENARNYA (terukur 6 Agustus 2026 pada llama-server b10295). Yang ada dipakai
// apa adanya, dan nama cuma jadi cadangan bila `meta` tidak dikirim.
//
// `n_ctx` yang paling penting dan bukan sekadar hiasan: resolveOllama
// memakainya sebagai Client.NumCtx, dan nilai 0 membuat koreksi transkrip
// menebak sendiri seberapa besar potongan yang muat.
func openAIModels(ctx context.Context, url string) []ModelInfo {
	return openAIModelsAt(ctx, url, "/v1", "", probeTimeout)
}

// Models membaca daftar model satu server OpenAI-compatible, dengan jalur dan
// kunci yang ditentukan pemanggil. Diekspor karena halaman setelan memakainya
// untuk mengisi sendiri pilihan model tiap penyedia — supaya nama model tidak
// perlu diketik dari ingatan (notes/39).
func Models(ctx context.Context, url, path, key string) []ModelInfo {
	if path == "" {
		path = "/v1"
	}
	// Bukan probeTimeout: 1,2 detik itu jatah untuk MEMINDAI port di komputer
	// sendiri, dan dipakai pada penyedia cloud ia mengembalikan daftar kosong
	// setiap kali jaringannya sedang lambat — terbaca sebagai "gateway tidak
	// punya model", bukan sebagai kehabisan waktu.
	return openAIModelsAt(ctx, url, path, key, 10*time.Second)
}

// openAIModelsAt sama, dengan jalur dan kunci yang ditentukan pemanggil —
// itulah yang dibutuhkan penyedia cloud.
func openAIModelsAt(ctx context.Context, url, path, key string, timeout time.Duration) []ModelInfo {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+path+"/models", nil)
	if err != nil {
		return nil
	}
	if key == "" {
		key = apiKey()
	}
	req.Header.Set("authorization", "Bearer "+key)
	// Retry ikut di sini: daftar model dibaca ulang tiap kali mesin diganti di
	// pemilih, dan satu 429 dari gateway bersama membuat kotaknya kosong tanpa
	// sebab yang terlihat.
	resp, err := (&http.Client{Timeout: timeout, Transport: httpx.Retry(nil)}).Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var parsed struct {
		Data []struct {
			ID   string `json:"id"`
			Meta struct {
				NParams int64  `json:"n_params"`
				NCtx    int    `json:"n_ctx"`
				Size    int64  `json:"size"`
				FType   string `json:"ftype"`
			} `json:"meta"`
		} `json:"data"`
	}
	raw, _ := io.ReadAll(resp.Body)
	if json.Unmarshal(raw, &parsed) != nil {
		return nil
	}
	out := make([]ModelInfo, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if strings.TrimSpace(m.ID) == "" {
			continue
		}
		// Base = nama yang layak dilihat. Perlu karena llama.cpp memakai PATH
		// LENGKAP berkas .gguf sebagai id, dan path 60 karakter di kotak pilihan
		// tidak memberitahu apa pun selain melebarkan kolomnya.
		mi := ModelInfo{Name: m.ID, Base: shortModelName(m.ID), Ready: true,
			Note:    "served by an OpenAI-compatible server",
			Bytes:   m.Meta.Size,
			Context: m.Meta.NCtx,
			Quant:   m.Meta.FType,
		}
		// Ukuran dari metadata selalu menang atas tebakan dari nama.
		params := float64(m.Meta.NParams) / 1e9
		if params == 0 {
			params = paramsFromName(m.ID)
		}
		// Penilaiannya cuma memberi catatan, tidak pernah menolak: salah baca
		// ukuran tidak boleh membuat model yang sebenarnya baik jadi tidak bisa
		// dipilih.
		if params > 0 {
			mi.Params = formatParams(params)
			if params < minParams {
				mi.Note = fmt.Sprintf("%s model: fine for transcript correction, but small models often return empty fields when picking moments", mi.Params)
			}
		}
		out = append(out, mi)
	}
	return out
}

// shortModelName memangkas id menjadi nama yang layak dibaca:
// "/home/x/models/Qwen2.5-3B-Instruct-Q4_K_M.gguf" → "Qwen2.5-3B-Instruct-Q4_K_M".
// Id yang memang sudah pendek dikembalikan apa adanya.
func shortModelName(id string) string {
	s := id
	if i := strings.LastIndexAny(s, `/\`); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(s, ".gguf")
	if s == "" {
		return BaseName(id)
	}
	return s
}

// paramsFromName membaca ukuran model dari namanya: "Qwen2.5-1.5B-Instruct" →
// 1.5, "SmolLM2-360M" → 0.36, "Llama-3.2-3b" → 3. 0 = tidak ketemu.
//
// Hanya untuk server yang TIDAK memberi metadata (semua yang bergaya OpenAI).
// Ollama menyebutkan ukurannya sendiri, dan angka dari sana selalu menang.
func paramsFromName(name string) float64 {
	m := paramInName.FindStringSubmatch(name)
	if m == nil {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	if strings.EqualFold(m[2], "M") {
		return v / 1000
	}
	return v
}

// Angka + B/M yang berdiri sebagai potongan namanya sendiri: "-1.5b-", "_360m".
// Tanpa pembatas itu, "3.2" di "Llama-3.2" ikut terbaca sebagai 3,2 miliar.
var paramInName = regexp.MustCompile(`(?i)[-_.:/ ]([0-9]+(?:\.[0-9]+)?)\s*([bm])(?:[-_.:/ ]|$)`)

// formatParams mengubah 1.5 → "1.5B" dan 0.36 → "360M".
func formatParams(b float64) string {
	if b < 1 {
		return fmt.Sprintf("%.0fM", b*1000)
	}
	return strings.TrimSuffix(fmt.Sprintf("%.1f", b), ".0") + "B"
}

// readReply membaca balasan chat, entah stream (text/event-stream) entah JSON
// utuh, ke bentuk yang sama. raw dikembalikan untuk pesan galat.
//
// Bukan cadangan kebijakan (notes/12): server yang sama, permintaan yang sama —
// hanya bentuk kirimnya yang berbeda, dan galat 4xx/5xx selalu JSON biasa.
func readReply(resp *http.Response) (openAIResp, []byte, error) {
	var parsed openAIResp
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		raw, _ := io.ReadAll(resp.Body)
		// Gemini (jalur OpenAI-nya) membungkus galat dalam ARRAY:
		// [{"error": {...}}]. Tanpa ini kuota habis (429) terbaca sebagai
		// "cannot unmarshal array" dan pesan aslinya terkubur.
		if t := bytes.TrimSpace(raw); len(t) > 0 && t[0] == '[' {
			var list []openAIResp
			if err := json.Unmarshal(t, &list); err != nil || len(list) == 0 {
				return parsed, raw, fmt.Errorf("unexpected reply shape")
			}
			return list[0], raw, nil
		}
		return parsed, raw, json.Unmarshal(raw, &parsed)
	}
	var content strings.Builder
	var finish string
	var last []byte
	done := false
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), "data:")
		data = strings.TrimSpace(data)
		if !ok || data == "" {
			continue // baris kosong, komentar ": keep-alive", event:
		}
		if data == "[DONE]" {
			done = true
			break
		}
		last = []byte(data)
		var ev openAIResp
		if err := json.Unmarshal(last, &ev); err != nil {
			return parsed, last, err
		}
		if ev.Error != nil {
			parsed.Error = ev.Error
			return parsed, last, nil
		}
		for _, ch := range ev.Choices {
			content.WriteString(ch.Delta.Content)
			if ch.FinishReason != "" {
				finish = ch.FinishReason
			}
		}
	}
	if err := sc.Err(); err != nil {
		return parsed, last, fmt.Errorf("the reply stream broke off: %w", err)
	}
	if !done && finish == "" {
		// Sambungan ditutup di tengah jalan TANPA galat baca (proxy/gateway
		// memutus). Dulu potongannya dikembalikan sebagai jawaban sah, dan
		// pemanggil baru gagal kemudian dengan "unexpected end of JSON input".
		return parsed, last, fmt.Errorf("the reply stream ended before the model finished (%d characters received)", content.Len())
	}
	parsed.Choices = []openAIChoice{{FinishReason: finish}}
	parsed.Choices[0].Message.Content = content.String()
	return parsed, last, nil
}
