package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Server bergaya OpenAI (llama.cpp, LocalAI, llamafile, vLLM, Aphrodite,
// LiteLLM, Exo) harus dikenali DAN dipakai tanpa setelan apa pun.
func TestOpenAIServerDiscoveredAndUsed(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags": // BUKAN Ollama
			w.WriteHeader(404)
		case "/v1/models":
			w.Write([]byte(`{"object":"list","data":[{"id":"Qwen2.5-1.5B-Instruct"},{"id":"SmolLM2-360M"}]}`))
		case "/v1/chat/completions":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.Write([]byte(`{"choices":[{"message":{"content":"{\"segments\":[]}"}}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	t.Setenv("OLLAMA_HOST", srv.URL)
	resetDiscoverCache()

	ep := DiscoverEndpoint(context.Background())
	if ep.Kind != KindOpenAI || ep.URL != srv.URL {
		t.Fatalf("endpoint = %+v, mau kind=openai url=%s", ep, srv.URL)
	}

	st := Status(context.Background(), "")
	if !st.Running || len(st.Installed) != 2 || st.Installed[0].Name != "Qwen2.5-1.5B-Instruct" {
		t.Fatalf("status = %+v", st)
	}

	c := New(srv.URL, "Qwen2.5-1.5B-Instruct")
	c.Kind = KindOpenAI
	out, err := c.Complete(context.Background(), "sys", "user", map[string]any{"type": "object"}, 512)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if out != `{"segments":[]}` {
		t.Fatalf("isi balasan = %q", out)
	}
	// Bentuk permintaannya harus yang disepakati bersama, bukan bentuk Ollama.
	if gotBody["model"] != "Qwen2.5-1.5B-Instruct" {
		t.Fatalf("model tidak ikut terkirim: %v", gotBody["model"])
	}
	if _, ok := gotBody["response_format"]; !ok {
		t.Fatal("response_format tidak dikirim — bentuk balasan jadi tidak dijamin")
	}
	if _, ok := gotBody["messages"]; !ok {
		t.Fatal("messages tidak dikirim")
	}
}

// Port yang kebetulan terbuka TAPI bukan server LLM tidak boleh dianggap LLM.
func TestNonLLMServerIsIgnored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>halaman biasa</html>`))
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_HOST", srv.URL)
	resetDiscoverCache()
	if ep := DiscoverEndpoint(context.Background()); ep.URL == srv.URL {
		t.Fatalf("server biasa dikira LLM: %+v", ep)
	}
}

// Ukuran model dari NAMANYA — satu-satunya petunjuk yang ada pada server yang
// tidak memberi metadata.
func TestParamsFromName(t *testing.T) {
	cases := map[string]float64{
		"Qwen2.5-1.5B-Instruct": 1.5,
		"SmolLM2-360M":          0.36,
		"Llama-3.2-3b":          3,
		"llama-3.1-8B-instruct": 8,
		"DeepSeek-R1-1.5B":      1.5,
		"Phi-4-Mini-3.8B":       3.8,
		"gpt-oss:120b-cloud":    120,
		"models/gemma-3-4b-it":  4,
		"Llama-3.2":             0, // versi, bukan ukuran
		"tanpa-angka":           0,
	}
	for name, want := range cases {
		if got := paramsFromName(name); got != want {
			t.Errorf("paramsFromName(%q) = %v, mau %v", name, got, want)
		}
	}
	if got := formatParams(0.36); got != "360M" {
		t.Errorf("formatParams(0.36) = %q", got)
	}
	if got := formatParams(1.5); got != "1.5B" {
		t.Errorf("formatParams(1.5) = %q", got)
	}
}

// Metadata yang DIKIRIM server harus dipakai, bukan ditebak dari nama.
//
// Bentuk balasan di bawah disalin dari llama-server b10295 yang benar-benar
// dijalankan (6 Agustus 2026), bukan dikarang: nama modelnya path lengkap
// berkas .gguf, dan spesifikasinya ada di `meta`. Yang paling menentukan
// `n_ctx` — resolveOllama memakainya sebagai Client.NumCtx, dan 0 membuat
// koreksi transkrip menebak sendiri besar potongan yang muat.
func TestOpenAIModelMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.WriteHeader(404)
		case "/v1/models":
			w.Write([]byte(`{"object":"list","data":[{"id":"/home/x/models/Qwen2.5-3B-Instruct-Q4_K_M.gguf",` +
				`"meta":{"n_ctx":4096,"n_params":3085938688,"size":1923946496,"ftype":"Q4_K - Medium"}}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	t.Setenv("OLLAMA_HOST", srv.URL)
	resetDiscoverCache()

	st := Status(context.Background(), "")
	if len(st.Installed) != 1 {
		t.Fatalf("model = %+v", st.Installed)
	}
	m := st.Installed[0]
	// Nama TETAP utuh: itu yang dikirim balik ke servernya saat job jalan.
	if m.Name != "/home/x/models/Qwen2.5-3B-Instruct-Q4_K_M.gguf" {
		t.Errorf("Name = %q, harus id apa adanya", m.Name)
	}
	// Base yang dipendekkan untuk dilihat.
	if m.Base != "Qwen2.5-3B-Instruct-Q4_K_M" {
		t.Errorf("Base = %q", m.Base)
	}
	if m.Context != 4096 {
		t.Errorf("Context = %d, mau 4096 — inilah yang jadi NumCtx", m.Context)
	}
	if m.Bytes != 1923946496 {
		t.Errorf("Bytes = %d", m.Bytes)
	}
	if m.Quant != "Q4_K - Medium" {
		t.Errorf("Quant = %q", m.Quant)
	}
	// 3.085.938.688 parameter → "3.1B", dari metadata; bukan "3B" dari namanya.
	if m.Params != "3.1B" {
		t.Errorf("Params = %q, mau 3.1B (dari n_params, bukan dari nama)", m.Params)
	}
}

// Server dikenali dengan NAMA yang dipakai pengguna, bukan nomor portnya.
func TestServerName(t *testing.T) {
	for _, c := range []struct{ url, kind, want string }{
		{"http://127.0.0.1:11434", KindOllama, "Ollama"},
		{"http://127.0.0.1:1234", KindOpenAI, "LM Studio"},
		{"http://127.0.0.1:1337", KindOpenAI, "Jan"},
		{"http://127.0.0.1:5001", KindOpenAI, "KoboldCpp"},
		{"http://127.0.0.1:4891", KindOpenAI, "GPT4All"},
		{"http://127.0.0.1:9999", KindOpenAI, "Local LLM server"},
	} {
		if got := serverName(c.url, c.kind); got != c.want {
			t.Errorf("serverName(%s) = %q, mau %q", c.url, got, c.want)
		}
	}
}

// Kunci server ikut terkirim: server dengan auth menyala membalas 401, dan
// probeJSON menganggap apa pun selain 200 sebagai "bukan server LLM" — jadi
// tanpa kunci yang benar server itu TIDAK TERLIHAT, bukan terlihat salah kunci.
func TestAPIKeySent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("authorization") != "Bearer rahasia" {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/api/tags":
			w.WriteHeader(404)
		case "/v1/models":
			w.Write([]byte(`{"object":"list","data":[{"id":"m"}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	// Diuji lewat kindOf, BUKAN lewat Status(ctx, ""): penemuan otomatis
	// mengetuk localhost, jadi server LLM sungguhan di mesin yang menjalankan
	// uji ini akan menjawab menggantikan server uji yang sengaja menolak — dan
	// ujinya lulus/gagal tergantung apa yang kebetulan hidup di komputer itu.
	t.Setenv("LLM_API_KEY", "rahasia")
	if k := kindOf(context.Background(), srv.URL); k != KindOpenAI {
		t.Fatalf("kunci benar: kind = %q, mau openai", k)
	}
	if ms := openAIModels(context.Background(), srv.URL); len(ms) != 1 {
		t.Fatalf("kunci benar: model = %+v", ms)
	}

	t.Setenv("LLM_API_KEY", "salah")
	if k := kindOf(context.Background(), srv.URL); k != "" {
		t.Fatalf("kunci salah: kind = %q, seharusnya tidak dikenali sama sekali", k)
	}
}

// KoboldCpp MENIRU /api/tags milik Ollama (terukur 6 Agustus 2026), jadi
// kindOf melaporkannya KindOllama. Namanya tetap harus KoboldCpp: kalau ia
// disebut "Ollama", baris yang menyala di halaman Requirements adalah baris
// aplikasi yang bahkan tidak terpasang.
func TestKoboldCppIsNotCalledOllama(t *testing.T) {
	if got := serverName("http://127.0.0.1:5001", KindOllama); got != "KoboldCpp" {
		t.Errorf("serverName = %q, mau KoboldCpp", got)
	}
	// Ollama asli tetap Ollama.
	if got := serverName("http://127.0.0.1:11434", KindOllama); got != "Ollama" {
		t.Errorf("serverName = %q, mau Ollama", got)
	}
}

// Model bernalar bisa menghabiskan jatah SEBELUM menjawab lebih dari sekali:
// deepseek-v4-pro masih habis di 8192 (2048 dilipatkan sekali). Jatahnya harus
// dilipatkan bertahap sampai roomCap, dan angka yang akhirnya berhasil diingat
// supaya potongan berikutnya tidak mengulang percobaan yang sudah gagal.
func TestBudgetGrowsUntilTheModelAnswers(t *testing.T) {
	var asked []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIReq
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)
		asked = append(asked, req.MaxTokens)
		if req.MaxTokens < roomCap {
			io.WriteString(w, `{"choices":[{"message":{"content":""},"finish_reason":"length"}]}`)
			return
		}
		io.WriteString(w, `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "reasoner-test")
	c.Kind = KindOpenAI
	out, err := c.Complete(context.Background(), "s", "u", nil, 2048)
	if err != nil || out != "ok" {
		t.Fatalf("balasan = %q, err = %v", out, err)
	}
	if len(asked) != 3 || asked[0] != 2048 || asked[1] != 8192 || asked[2] != roomCap {
		t.Fatalf("jatah yang diminta = %v, mau [2048 8192 %d]", asked, roomCap)
	}

	// Permintaan kedua langsung memakai jatah yang sudah terbukti.
	asked = nil
	if _, err := c.Complete(context.Background(), "s", "u", nil, 2048); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || asked[0] != roomCap {
		t.Fatalf("jatah pada permintaan kedua = %v, mau [%d]", asked, roomCap)
	}

	// Yang diingat itu SATU model, bukan servernya: satu penyedia menyajikan
	// model bernalar dan model biasa sekaligus, dan model biasa tidak boleh ikut
	// diminta 32768 token gara-gara tetangganya bernalar.
	asked = nil
	lain := New(srv.URL, "biasa-test")
	lain.Kind = KindOpenAI
	lain.Complete(context.Background(), "s", "u", nil, 2048)
	if len(asked) == 0 || asked[0] != 2048 {
		t.Fatalf("model lain mulai dari %v, mau 2048", asked)
	}
}

// Balasan stream (text/event-stream) dirakit jadi satu isi utuh — inilah yang
// membuat gateway di belakang Cloudflare tidak memutus model lambat dengan 524.
// Galat yang datang DI TENGAH stream tetap sampai ke pemanggil.
func TestStreamedReplyIsAssembled(t *testing.T) {
	var gotBody map[string]any
	reply := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(reply))
	}))
	defer srv.Close()
	c := New(srv.URL, "glm")
	c.Kind = KindOpenAI

	reply = ": keep-alive\n\n" +
		`data: {"choices":[{"delta":{"role":"assistant"}}]}` + "\n\n" +
		`data: {"choices":[{"delta":{"content":"{\"segments\""}}]}` + "\n\n" +
		`data: {"choices":[{"delta":{"content":":[]}"},"finish_reason":"stop"}]}` + "\n\n" +
		"data: [DONE]\n\n"
	out, err := c.Complete(context.Background(), "sys", "user", nil, 512)
	if err != nil || out != `{"segments":[]}` {
		t.Fatalf("isi = %q, galat = %v", out, err)
	}
	if gotBody["stream"] != true {
		t.Fatalf("stream tidak diminta: %v", gotBody["stream"])
	}

	reply = `data: {"choices":[{"delta":{"content":"{"}}]}` + "\n\n" +
		`data: {"error":{"message":"upstream overloaded"}}` + "\n\n"
	if _, err := c.Complete(context.Background(), "sys", "user", nil, 512); err == nil ||
		!strings.Contains(err.Error(), "upstream overloaded") {
		t.Fatalf("galat di tengah stream hilang: %v", err)
	}

	// Stream yang putus tanpa [DONE] dan tanpa finish_reason = galat, bukan
	// JSON terpotong yang dikembalikan sebagai jawaban.
	reply = `data: {"choices":[{"delta":{"content":"{\"picks\":[{\"title\":\"#politik"},"finish_reason":""}]}` + "\n\n"
	if _, err := c.Complete(context.Background(), "sys", "user", nil, 512); err == nil ||
		!strings.Contains(err.Error(), "ended before") {
		t.Fatalf("stream putus dikira selesai: %v", err)
	}
}

// Balasan pilihan yang terpotong (model mengulang frasa di "title") diminta
// ulang ke mesin yang sama, bukan langsung menggagalkan job.
func TestPickRetriesUnreadableReply(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		content := `{"picks":[{"index":0,"score":80,"title":"Judul (80 seconds) (80 seconds)`
		if calls > 1 {
			content = `{"picks":[{"index":0,"score":80,"title":"Judul"}]}`
		}
		b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
			"message": map[string]any{"content": content}, "finish_reason": "stop"}}})
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}))
	defer srv.Close()
	c := New(srv.URL, "glm")
	c.Kind = KindOpenAI

	picks, err := c.PickMoments(context.Background(), nil, 0, 1, "id")
	if err != nil || len(picks) != 1 || picks[0].Title != "Judul" || calls != 2 {
		t.Fatalf("picks=%v err=%v calls=%d", picks, err, calls)
	}
}
