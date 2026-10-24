package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/news"
	"github.com/gemgum/clipper/engine/internal/pipeline"
	"github.com/gemgum/clipper/engine/internal/score/llm"
	"github.com/gemgum/clipper/engine/internal/score/ollama"
	"github.com/gemgum/clipper/engine/internal/transcribe"
	"github.com/gemgum/clipper/engine/internal/writer"
)

// Mesin LLM, satu daftar untuk seluruh aplikasi (notes/39).
//
// Sebelumnya tiap tab merakit pilihannya sendiri dan kunci Claude hanya bisa
// diisi dari tab klip. Menambah satu penyedia berarti menyentuh tiga tempat.
// Sekarang: satu tabel di sini, satu halaman setelan, satu pemilih di GUI.
//
// Yang TIDAK ada di tabel ini disengaja — temperature ditentukan tugasnya
// (0,1 untuk mengekstrak fakta, 0,4 untuk memilih momen), max tokens dipatok
// engine sesudah 4096 terbukti memotong JSON di tengah (notes/38), dan nama
// header auth sama untuk semua endpoint OpenAI-compatible. Isian yang tidak
// dibutuhkan hanya menambah cara untuk salah.

// Jenis mesin. Menentukan klien mana yang dipakai, bukan siapa penyedianya.
const (
	kindLocal     = "local"     // server di komputer ini; ditemukan sendiri
	kindAnthropic = "anthropic" // API Claude
	kindOpenAI    = "openai"    // apa pun yang bicara /chat/completions
)

// engineDef = satu mesin yang dikenal aplikasi.
type engineDef struct {
	ID   string
	Name string
	Kind string
	// Base, Model = nilai bawaan pabrik. Base = alamat API UTUH, persis seperti
	// di dokumentasi penyedia (https://api.openai.com/v1): klien hanya menambah
	// nama endpoint-nya (/chat/completions, /messages, /audio/transcriptions). Keduanya boleh ditimpa pengguna lewat
	// env, karena penyedia memindahkan endpoint dan menerbitkan model baru
	// jauh lebih sering daripada aplikasi ini dirilis ulang.
	Base  string
	Model string
	// EnvKey/EnvBase/EnvModel = nama variabel di .env.
	EnvKey   string
	EnvBase  string
	EnvModel string
	// KeysURL = halaman tempat pengguna mengambil kuncinya.
	KeysURL string
	// User = mesin yang ditambahkan pengguna sendiri lewat halaman Pengaturan
	// (bukan dari tabel pabrik di bawah); hanya jenis ini yang boleh dihapus.
	User bool
}

// engineDefs = daftar tertutup. Ditulis di satu tempat supaya menambah penyedia
// berikutnya benar-benar satu baris.
var engineDefs = []engineDef{
	{
		ID: "ollama", Name: "Local AI", Kind: kindLocal,
		EnvBase: "OLLAMA_HOST", EnvKey: "LLM_API_KEY",
	},
	{
		ID: "claude", Name: "Claude", Kind: kindAnthropic,
		Base: "https://api.anthropic.com/v1", Model: "claude-haiku-4-5",
		EnvKey: "ANTHROPIC_API_KEY", EnvBase: "ANTHROPIC_BASE_URL", EnvModel: "ANTHROPIC_MODEL",
		KeysURL: "https://console.anthropic.com/settings/keys",
	},
	{
		ID: "openai", Name: "ChatGPT (OpenAI)", Kind: kindOpenAI,
		Base: "https://api.openai.com/v1", Model: "gpt-5.6",
		EnvKey: "OPENAI_API_KEY", EnvBase: "OPENAI_BASE_URL", EnvModel: "OPENAI_MODEL",
		KeysURL: "https://platform.openai.com/api-keys",
	},
	{
		ID: "gemini", Name: "Google AI Studio (Gemini)", Kind: kindOpenAI,
		Base: "https://generativelanguage.googleapis.com/v1beta/openai", Model: "gemini-3.7-flash",
		EnvKey: "GEMINI_API_KEY", EnvBase: "GEMINI_BASE_URL", EnvModel: "GEMINI_MODEL",
		KeysURL: "https://aistudio.google.com/apikey",
	},
	{
		ID: "deepseek", Name: "DeepSeek", Kind: kindOpenAI,
		Base: "https://api.deepseek.com/v1", Model: "deepseek-v4-pro",
		EnvKey: "DEEPSEEK_API_KEY", EnvBase: "DEEPSEEK_BASE_URL", EnvModel: "DEEPSEEK_MODEL",
		KeysURL: "https://platform.deepseek.com/api_keys",
	},
	{
		// Gateway OpenAI-compatible milik pengguna: satu alamat + satu kunci
		// menyajikan banyak model sekaligus (claude-*, gpt-*, kimi-*, glm-*,
		// minimax-*, deepseek-*). Karena itu Base dan Model dibiarkan kosong —
		// pabrik tidak tahu alamatmu — dan nama modelnya TIDAK dipetakan ulang:
		// dipakai persis seperti yang diterbitkan gateway lewat /v1/models.
		//
		// Env-nya sendiri, jadi memasang ini tidak menyentuh kunci penyedia lain.
		ID: "custom", Name: "Custom (OpenAI-compatible)", Kind: kindOpenAI,
		EnvKey: "CUSTOM_API_KEY", EnvBase: "CUSTOM_BASE_URL", EnvModel: "CUSTOM_MODEL",
	},
}

// userEnginesEnv = daftar id mesin tambahan pengguna, dipisah koma. Isian tiap
// mesinnya di .env juga (userEnv), sama seperti kunci pabrik: satu sumber
// kebenaran, ikut terbaca CLI.
const userEnginesEnv = "CLIPPER_ENGINES"

func userEnv(id, field string) string { return "ENGINE_" + strings.ToUpper(id) + "_" + field }

func userEngineIDs() []string {
	var out []string
	for _, id := range strings.Split(os.Getenv(userEnginesEnv), ",") {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// allEngineDefs = tabel pabrik + mesin tambahan pengguna.
//
// Mode web membuang mesin lokal: server tim tidak menjalankan Ollama, dan
// pilihan yang tidak mungkin siap cuma membingungkan.
func allEngineDefs() []engineDef {
	out := make([]engineDef, 0, len(engineDefs)+4)
	for _, d := range engineDefs {
		if d.Kind == kindLocal && webRoot != "" {
			continue
		}
		out = append(out, d)
	}
	for _, id := range userEngineIDs() {
		// Semua mesin tambahan bicara /chat/completions: itu satu-satunya
		// bentuk yang bisa diisi cukup dengan alamat + kunci + model.
		out = append(out, engineDef{
			ID: id, Name: firstNonEmpty(os.Getenv(userEnv(id, "NAME")), id), Kind: kindOpenAI,
			EnvKey: userEnv(id, "API_KEY"), EnvBase: userEnv(id, "BASE_URL"), EnvModel: userEnv(id, "MODEL"),
			User: true,
		})
	}
	return out
}

// defaultEngineID = mesin bila belum ada yang dipilih: Ollama di desktop,
// mesin siap pertama di mode web.
func defaultEngineID() string {
	if webRoot == "" {
		return "ollama"
	}
	for _, e := range Engines() {
		if e.Ready {
			return e.ID
		}
	}
	return "claude"
}

func engineByID(id string) (engineDef, bool) {
	for _, d := range allEngineDefs() {
		if d.ID == id {
			return d, true
		}
	}
	return engineDef{}, false
}

// Engine = satu mesin sebagaimana BERLAKU sekarang: bawaan pabrik yang sudah
// ditimpa isi .env. Inilah yang dikirim ke GUI.
type Engine struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	HasKey  bool   `json:"has_key"`
	// Ready = boleh muncul di pemilih mesin. Mesin cloud tanpa kunci TIDAK
	// muncul — keputusan pemilik proyek; pintunya baris "Add an engine…" yang
	// mengarah ke setelan (notes/39).
	Ready   bool   `json:"ready"`
	KeysURL string `json:"keys_url,omitempty"`
	User    bool   `json:"user,omitempty"`
	// Use = untuk apa mesin ini dipakai: "chat", "transcribe", atau "" (keduanya).
	// Menentukan di daftar mana ia muncul dan bagaimana tombol Test mengujinya.
	Use string `json:"use"`
}

// Kegunaan mesin. "" = keduanya: nilai lama yang belum pernah dipilih
// (mis. Groq yang sudah dipakai untuk transkripsi) tetap muncul di dua daftar.
const (
	useChat       = "chat"
	useTranscribe = "transcribe"
)

// useEnv = nama variabel .env kegunaan mesin, atau "" bila kegunaannya tetap.
// Hanya mesin OpenAI-compatible yang punya endpoint audio; Gemini transkripsinya
// lewat AI Studio, Claude dan mesin lokal hanya chat.
func useEnv(d engineDef) string {
	if d.Kind != kindOpenAI || d.ID == "gemini" || !strings.HasSuffix(d.EnvKey, "API_KEY") {
		return ""
	}
	return strings.TrimSuffix(d.EnvKey, "API_KEY") + "USE"
}

func validUse(v string) bool { return v == "" || v == useChat || v == useTranscribe }

// resolve membaca satu mesin beserta timpaan .env-nya.
func resolve(d engineDef) Engine {
	e := Engine{ID: d.ID, Name: d.Name, Kind: d.Kind, BaseURL: d.Base, Model: d.Model, KeysURL: d.KeysURL, User: d.User}
	if v := envOr(d.EnvBase, ""); v != "" {
		e.BaseURL = cleanBase(v)
	}
	if v := envOr(d.EnvModel, ""); v != "" {
		e.Model = v
	}
	e.HasKey = engineKey(d) != ""
	e.Use = useChat
	if name := useEnv(d); name != "" {
		e.Use = strings.TrimSpace(os.Getenv(name))
		if !validUse(e.Use) {
			e.Use = ""
		}
	}

	if d.Kind == kindLocal {
		// Mesin lokal tidak butuh kunci; yang menentukan siap atau tidak adalah
		// ADA SERVERNYA. Alamat kosong berarti "cari sendiri" — dan penemuan
		// itu mahal, jadi hasilnya dibaca dari cache Discover.
		if e.BaseURL == "" {
			e.BaseURL = ollama.Discover(context.Background())
		}
		e.Ready = e.BaseURL != ""
		return e
	}
	// Alamat ikut menentukan siap atau tidak: mesin "custom" tidak punya alamat
	// pabrik, dan tanpa syarat ini kunci tanpa alamat lolos ke klien OpenAI yang
	// lalu jatuh ke localhost:11434 — galat yang tidak menyebut sebab aslinya.
	e.Ready = e.HasKey && e.BaseURL != ""
	return e
}

// engineKey mengambil kunci satu mesin dari lingkungan.
//
// .env adalah SATU-SATUNYA sumber kebenaran, dan itu disengaja: CLI membaca
// berkas yang sama, jadi kunci yang diisi lewat GUI langsung berlaku di sana
// juga. Karena itu setiap penyimpanan kunci wajib os.Setenv, bukan cuma menulis
// berkasnya (lihat setEnv).
func engineKey(d engineDef) string { return envOr(d.EnvKey, "") }

func envOr(name, fallback string) string {
	if name == "" {
		return fallback
	}
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

// Engines melaporkan seluruh mesin beserta keadaannya.
func Engines() []Engine {
	defs := allEngineDefs()
	out := make([]Engine, 0, len(defs))
	for _, d := range defs {
		out = append(out, resolve(d))
	}
	return out
}

// EngineFor merakit pemanggil LLM untuk satu mesin.
//
// model kosong = model yang berlaku untuk mesin itu (bawaan pabrik atau
// timpaan pengguna). Tanpa fallback: mesin yang dipilih dipakai apa adanya,
// gagal ya gagal dengan pesan akar masalahnya (notes/12).
func EngineFor(id, model string) (writer.Completer, string, error) {
	if id == "" {
		id = defaultEngineID()
	}
	d, ok := engineByID(id)
	if !ok {
		return nil, "", fmt.Errorf("unknown engine %q: choose one of: %s", id, engineIDs())
	}
	return engineFor(d, resolve(d), engineKey(d), model)
}

// engineFor = EngineFor dengan alamat & kunci yang sudah ditentukan pemanggil —
// tombol Test memakainya untuk isian yang belum disimpan.
func engineFor(d engineDef, e Engine, key, model string) (writer.Completer, string, error) {
	if e.Use == useTranscribe {
		return nil, "", fmt.Errorf("%s is set up for transcription only: pick a chat engine, or change its use on the Requirements page", d.Name)
	}
	if model = strings.TrimSpace(model); model == "" {
		model = e.Model
	}

	switch d.Kind {
	case kindAnthropic:
		if key == "" {
			return nil, "", fmt.Errorf("%s has no API key yet: add it on the Requirements page", d.Name)
		}
		c := llm.New(key, model)
		c.BaseURL = e.BaseURL
		return func(ctx context.Context, system, user string, _ any) (string, error) {
			// Claude tidak menerima skema: bentuk balasannya diminta lewat prompt.
			return c.Complete(ctx, system, user, llmMaxTokens)
		}, d.Name + " (" + c.Model + ")", nil

	case kindOpenAI:
		if e.BaseURL == "" {
			return nil, "", fmt.Errorf("%s has no address yet: add it on the Requirements page", d.Name)
		}
		if key == "" {
			return nil, "", fmt.Errorf("%s has no API key yet: add it on the Requirements page", d.Name)
		}
		c := ollama.New(e.BaseURL, model)
		c.Kind, c.APIKey = ollama.KindOpenAI, key
		return func(ctx context.Context, system, user string, schema any) (string, error) {
			return c.Complete(ctx, system, user, schema, llmMaxTokens)
		}, d.Name + " (" + c.Model + ")", nil

	default: // kindLocal
		c := ollama.New(e.BaseURL, model)
		// Konteks maksimum model ditanyakan sekali di sini, bukan per panggilan:
		// tanpa itu jendelanya terkunci di 8192 dan permintaan keluaran besar —
		// artikel utuh — terpotong di tengah JSON tanpa pesan galat apa pun.
		c.NumCtx = ollama.ContextOf(context.Background(), c.URL, c.Model)
		return func(ctx context.Context, system, user string, schema any) (string, error) {
			return c.Complete(ctx, system, user, schema, llmMaxTokens)
		}, "Ollama (" + c.Model + ")", nil
	}
}

func engineIDs() string {
	defs := allEngineDefs()
	ids := make([]string, 0, len(defs))
	for _, d := range defs {
		ids = append(ids, d.ID)
	}
	sort.Strings(ids)
	return strings.Join(ids, ", ")
}

// --- HTTP ---

func (s *Server) listEngines(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"engines": Engines()})
}

// engineModels menjawab daftar model satu mesin.
//
// Dipisah dari /test karena harganya beda jauh: ini satu GET, sedangkan /test
// memanggil LLM sungguhan. Pemilih model di GUI memanggil yang ini tiap kali
// mesinnya diganti — itu tidak boleh berbiaya token.
//
// Balasan kosong BUKAN galat: Claude tidak menyediakan daftar lewat jalur ini,
// dan server lokal yang sedang mati juga tidak. Kotak modelnya tetap bisa
// diketik (notes/39).
func (s *Server) engineModels(w http.ResponseWriter, r *http.Request) {
	d, ok := engineByID(r.PathValue("id"))
	if !ok {
		writeErr(w, 400, "unknown engine "+r.PathValue("id"))
		return
	}
	e := resolve(d)
	var names []string
	if d.Kind != kindAnthropic && e.BaseURL != "" {
		for _, m := range ollama.Models(r.Context(), e.BaseURL, engineKey(d)) {
			names = append(names, m.Name)
		}
	}
	writeJSON(w, 200, map[string]any{"models": names})
}

// saveEngine menyimpan kunci/alamat/model satu mesin ke .env.
//
// Field yang TIDAK dikirim tidak disentuh, dan string kosong berarti "kembali
// ke bawaan" — bukan "simpan kosong". Tanpa pembedaan itu, menyimpan model saja
// akan menghapus kunci yang sudah ada.
func (s *Server) saveEngine(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID      string  `json:"id"`
		Name    *string `json:"name"`
		APIKey  *string `json:"api_key"`
		BaseURL *string `json:"base_url"`
		Model   *string `json:"model"`
		Use     *string `json:"use"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.Use != nil && !validUse(*req.Use) {
		writeErr(w, 400, "use must be chat, transcribe, or empty for both")
		return
	}
	if req.BaseURL != nil {
		if n := endpointInBase(*req.BaseURL); n != "" {
			writeErr(w, 400, "the address ends with "+n+", which Clipper adds itself: enter it up to the part before "+n+", e.g. https://openrouter.ai/api/v1")
			return
		}
	}
	if req.ID == "" {
		// Mesin baru: id diturunkan dari namanya, dan alamatnya wajib — tanpa
		// alamat ia tidak pernah bisa siap.
		name := ""
		if req.Name != nil {
			name = strings.TrimSpace(*req.Name)
		}
		if name == "" || req.BaseURL == nil || strings.TrimSpace(*req.BaseURL) == "" {
			writeErr(w, 400, "a new engine needs a name and an address")
			return
		}
		req.ID = newEngineID(name)
		s.setEnv(userEnginesEnv, strings.Join(append(userEngineIDs(), req.ID), ","))
	}
	d, ok := engineByID(req.ID)
	if !ok {
		writeErr(w, 400, "unknown engine "+req.ID)
		return
	}
	if req.Name != nil && d.User {
		if n := strings.TrimSpace(*req.Name); n != "" {
			s.setEnv(userEnv(d.ID, "NAME"), n)
			d.Name = n
		}
	}
	if req.APIKey != nil {
		key := strings.TrimSpace(*req.APIKey)
		s.setEnv(d.EnvKey, key)
		if d.Kind == kindAnthropic {
			// Manager menyimpan kunci untuk job klip yang sudah berjalan.
			s.mgr.SetAPIKey(key)
		}
	}
	if req.BaseURL != nil {
		s.setEnv(d.EnvBase, cleanBase(*req.BaseURL))
		if d.Kind == kindLocal {
			// Alamat kosong berarti kembali ke penemuan otomatis, dan itu harus
			// terasa SEKARANG, bukan setelah aplikasi dijalankan lagi.
			ollama.ResetCache()
		}
	}
	if req.Model != nil {
		s.setEnv(d.EnvModel, strings.TrimSpace(*req.Model))
	}
	if name := useEnv(d); req.Use != nil && name != "" {
		s.setEnv(name, *req.Use)
	}
	writeJSON(w, 200, resolve(d))
}

// newEngineID membentuk id dari nama: huruf kecil, angka, dan garis bawah saja
// (ia jadi bagian nama variabel .env), berawalan "u_" supaya tidak pernah
// bertabrakan dengan mesin pabrik, dan diberi nomor bila sudah terpakai.
func newEngineID(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "_"):
			b.WriteByte('_')
		}
	}
	base := "u_" + strings.Trim(b.String(), "_")
	if base == "u_" {
		base = "u_engine"
	}
	id := base
	for n := 2; ; n++ {
		if _, taken := engineByID(id); !taken {
			return id
		}
		id = fmt.Sprintf("%s_%d", base, n)
	}
}

// deleteEngine membuang satu mesin tambahan pengguna beserta isiannya di .env.
// Mesin pabrik tidak bisa dihapus; kosongkan kuncinya bila tidak dipakai.
func (s *Server) deleteEngine(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	d, ok := engineByID(req.ID)
	if !ok || !d.User {
		writeErr(w, 400, "only engines you added can be removed")
		return
	}
	var keep []string
	for _, id := range userEngineIDs() {
		if id != d.ID {
			keep = append(keep, id)
		}
	}
	s.setEnv(userEnginesEnv, strings.Join(keep, ","))
	for _, f := range []string{"NAME", "API_KEY", "BASE_URL", "MODEL"} {
		s.setEnv(userEnv(d.ID, f), "")
	}
	writeJSON(w, 200, map[string]any{"engines": Engines()})
}

// setEnv menulis ke proses ini DAN ke .env. Keduanya perlu: yang pertama supaya
// perubahannya berlaku tanpa dijalankan ulang, yang kedua supaya ia bertahan —
// dan supaya CLI, yang membaca .env yang sama, ikut kebagian.
func (s *Server) setEnv(name, value string) {
	if name == "" {
		return
	}
	_ = os.Setenv(name, value)
	_ = writeEnvKey(s.paths.EnvFile, name, value)
}

// cleanBase merapikan alamat tanpa mengubah jalurnya: spasi & garis miring di
// ujung dibuang, skema ditambah bila tidak ada (OLLAMA_HOST lazim ditulis
// "127.0.0.1:11434"). Jalur TIDAK disentuh — alamat dipakai apa adanya, sebab
// dulu /v1 dipangkas lalu ditambah lagi oleh klien, dan penyedia yang jalurnya
// bukan /v1 (atau pindah ke /v2) jadi tidak bisa dipakai sama sekali.
func cleanBase(v string) string {
	v = strings.TrimRight(strings.TrimSpace(v), "/")
	if v != "" && !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
		v = "http://" + v
	}
	return v
}

// endpointNames = akhiran yang ditambahkan klien sendiri. Alamat yang sudah
// memuatnya tersusun dobel (.../audio/transcriptions/audio/transcriptions) dan
// gejalanya cuma 404 — pernah terjadi 15 Oktober 2026 dengan OpenRouter.
var endpointNames = []string{"/chat/completions", "/audio/transcriptions", "/messages", "/models"}

func endpointInBase(v string) string {
	for _, n := range endpointNames {
		if strings.HasSuffix(cleanBase(v), n) {
			return n
		}
	}
	return ""
}

// baseURLsFullEnv menandai .env yang alamatnya sudah UTUH (dengan /v1). Sebelum
// 15 Oktober 2026 alamat disimpan tanpa jalurnya; migrateBaseURLs menambahkan
// jalur lama itu sekali, lalu memasang penanda ini.
const baseURLsFullEnv = "CLIPPER_BASE_URLS"

// oldPath = jalur yang dulu ditambahkan klien untuk mesin ini.
func oldPath(d engineDef) string {
	if d.ID == "gemini" {
		return "/v1beta/openai"
	}
	return "/v1"
}

func (s *Server) migrateBaseURLs() {
	if os.Getenv(baseURLsFullEnv) == "full" {
		return
	}
	for _, d := range allEngineDefs() {
		if d.Kind == kindLocal || d.EnvBase == "" {
			continue
		}
		if v := cleanBase(os.Getenv(d.EnvBase)); v != "" && !strings.HasSuffix(v, oldPath(d)) {
			s.setEnv(d.EnvBase, v+oldPath(d))
		}
	}
	s.setEnv(baseURLsFullEnv, "full")
}

// EngineTest = hasil uji satu mesin.
//
// Bukan sekadar lampu hijau. Tiap hal yang bisa DITANYAKAN ke servernya
// menghapus satu isian yang harus diketik pengguna (notes/39):
//
//   - Models  → pengguna memilih dari daftar, bukan mengetik dari ingatan;
//   - Schema  → apakah penyedia menghormati balasan berskema. Ini yang menopang
//     seluruh pagar fakta pembuat berita, dan penyedia yang membandel gejalanya
//     BUKAN galat melainkan job gagal parsing di tengah jalan.
type EngineTest struct {
	OK     bool     `json:"ok"`
	Engine string   `json:"engine,omitempty"`
	Models []string `json:"models,omitempty"`
	// Schema = balasannya BERBENTUK seperti yang diminta.
	Schema bool `json:"schema"`
	// Strict = servernya benar-benar MEMAKSAKAN bentuk itu. Keduanya dipisah
	// karena model bisa saja menuruti prompt pada balasan pendek lalu melenceng
	// pada yang panjang — dan yang panjang itulah tahap menulis.
	Strict bool `json:"strict"`
	// Mode = jenis uji yang dijalankan: "chat" atau "transcribe".
	Mode string `json:"mode"`
	// Heard = teks yang didengar mesin transkripsi dari audio contoh;
	// Expected = kalimat aslinya, untuk dibandingkan di GUI.
	Heard    string `json:"heard,omitempty"`
	Expected string `json:"expected,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (s *Server) testEngine(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		// Mode = "chat" atau "transcribe"; kosong = sesuai kegunaan mesin.
		Mode string `json:"mode"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	d, ok := engineByID(req.ID)
	if !ok {
		writeErr(w, 400, "unknown engine "+req.ID)
		return
	}
	e := resolve(d)
	mode := req.Mode
	if mode == "" && e.Use == useTranscribe {
		mode = useTranscribe
	}

	// Yang diuji adalah yang TERSIMPAN, dirakit lewat jalur job: Options bawaan
	// → fillEngine/fillTranscriber (Manager.Prepare) → pipeline. Dulu tombol ini
	// memakai isian form yang belum disimpan dan kliennya sendiri, sehingga Test
	// bisa hijau sementara job 404 (15 Oktober 2026).
	o := config.DefaultOptions()
	paths := s.paths
	if d.Kind == kindAnthropic {
		paths.APIKey = engineKey(d)
	}
	out := EngineTest{Mode: firstNonEmpty(mode, useChat)}
	if mode == useTranscribe {
		if !audioEngine(d.ID) {
			out.Error = d.Name + " is not set up for transcription: set its use to Transcription or Both and save"
			writeJSON(w, 200, out)
			return
		}
		o.Transcriber, o.TranscribeModel = d.ID, strings.TrimSpace(req.Model)
		fillTranscriber(&o)
		p := pipeline.New(paths, o)
		out.Engine = p.TranscriberName()
		heard, err := p.ProbeTranscriber(r.Context())
		if err != nil {
			out.Error = err.Error()
		} else {
			out.OK, out.Schema, out.Strict = true, true, true
			out.Heard, out.Expected = heard, transcribe.TestSpeechText
		}
		writeJSON(w, 200, out)
		return
	}
	if e.Use == useTranscribe {
		out.Error = d.Name + " is set up for transcription only: change its use to Chat or Both and save"
		writeJSON(w, 200, out)
		return
	}
	if d.Kind != kindAnthropic && e.BaseURL != "" {
		for _, m := range ollama.Models(r.Context(), e.BaseURL, engineKey(d)) {
			out.Models = append(out.Models, m.Name)
		}
	}

	// Uji DUA TAHAP yang sama persis dengan job (koreksi transkrip + pemilihan
	// momen), untuk SEMUA mesin — bukan sapaan "ok". Sapaan menguji bahwa
	// servernya menjawab, bukan bahwa modelnya sanggup (temuan lama di Ollama).
	o.Provider, o.LLMModel = d.ID, strings.TrimSpace(req.Model)
	if d.Kind == kindLocal {
		o.OllamaURL, o.OllamaModel = e.BaseURL, firstNonEmpty(o.LLMModel, e.Model)
	} else {
		fillEngine(&o)
	}
	name, steps := pipeline.New(paths, o).SelfTest(r.Context())
	out.Engine = name
	out.OK, out.Schema = true, true
	for _, st := range steps {
		if !st.OK {
			out.OK, out.Schema = false, false
			out.Error = st.Name + ": " + st.Error
			break
		}
	}
	out.Strict = out.OK && (d.Kind != kindOpenAI || ollama.SchemaEnforced(e.BaseURL))
	writeJSON(w, 200, out)
}

// llmEngine merakit pemanggil untuk tab kartu berita, dengan skema pemilihan
// paragraf yang sudah dipaku.
func (s *Server) llmEngine(id, model string) (news.Completer, string, error) {
	llmFn, name, err := EngineFor(id, model)
	if err != nil {
		return nil, "", err
	}
	return func(ctx context.Context, system, user string) (string, error) {
		return llmFn(ctx, system, user, news.SelectionSchema())
	}, name, nil
}

// fillEngine menerjemahkan mesin yang dipilih pengguna jadi koordinat yang
// dipahami pipeline klip.
//
// "ollama" dan "heuristic" dibiarkan apa adanya: keduanya sudah punya jalannya
// sendiri di pipeline (penemuan server lokal, dan tanpa LLM sama sekali).
func fillEngine(o *config.Options) {
	d, ok := engineByID(o.Provider)
	if !ok || d.Kind == kindLocal {
		return
	}
	e := resolve(d)
	o.EngineName = d.Name + " (" + firstNonEmpty(o.LLMModel, e.Model) + ")"
	o.LLMBase, o.LLMKeyEnv = e.BaseURL, d.EnvKey
	if o.LLMModel == "" {
		o.LLMModel = e.Model
	}
	// Claude punya klien sendiri di pipeline; yang perlu diteruskan hanya
	// alamatnya, sebab kuncinya sudah dibaca lewat Paths.APIKey.
	if d.Kind == kindAnthropic {
		o.Provider = "claude"
	}
}
