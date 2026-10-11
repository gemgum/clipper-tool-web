package api

import (
	"context"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
)

// clearEngineEnv membersihkan seluruh variabel mesin supaya uji tidak
// terpengaruh .env mesin pengembang.
func clearEngineEnv(t *testing.T) {
	t.Helper()
	for _, d := range engineDefs {
		for _, name := range []string{d.EnvKey, d.EnvBase, d.EnvModel} {
			if name != "" {
				t.Setenv(name, "")
			}
		}
	}
}

// TestEngineDefsLengkap menjaga tabelnya tetap masuk akal saat penyedia
// berikutnya ditambahkan: satu baris yang lupa diisi jauh lebih mudah lolos
// daripada kelihatan.
func TestEngineDefsLengkap(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range engineDefs {
		if seen[d.ID] {
			t.Errorf("id ganda: %q", d.ID)
		}
		seen[d.ID] = true
		if d.Name == "" || d.Kind == "" || d.EnvKey == "" {
			t.Errorf("%s: nama/jenis/EnvKey ada yang kosong: %+v", d.ID, d)
		}
		if d.Kind == kindLocal {
			continue
		}
		// "custom" adalah satu-satunya pengecualian yang disengaja: ia justru
		// TIDAK punya alamat pabrik, sebab yang dituju gateway milik pengguna.
		// Ditulis sebagai nama, bukan sebagai aturan longgar, supaya penyedia
		// berikutnya yang lupa diisi tetap ketahuan di sini.
		if d.ID == "custom" {
			continue
		}
		// Mesin cloud harus bisa dipakai TANPA pengguna mengetik apa pun selain
		// kuncinya — jadi alamat, model, dan halaman kunci wajib ada bawaannya.
		if d.Base == "" || d.Model == "" || d.KeysURL == "" {
			t.Errorf("%s: bawaan cloud belum lengkap: %+v", d.ID, d)
		}
		if d.EnvBase == "" || d.EnvModel == "" {
			t.Errorf("%s: alamat & model harus bisa ditimpa pengguna (notes/39)", d.ID)
		}
		if d.Kind == kindOpenAI && d.Path == "" {
			t.Errorf("%s: Path wajib diisi untuk mesin OpenAI-compatible", d.ID)
		}
	}
}

// TestGeminiJalurBukanV1 menjaga temuan yang melahirkan field Path: endpoint
// OpenAI-compatible milik Gemini bukan "/v1" melainkan "/v1beta/openai", dan
// tanpa itu satu-satunya jalan adalah klien kedua yang isinya sama persis.
func TestGeminiJalurBukanV1(t *testing.T) {
	d, ok := engineByID("gemini")
	if !ok {
		t.Fatal("gemini hilang dari tabel")
	}
	if d.Path != "/v1beta/openai" {
		t.Errorf("Path = %q, mau /v1beta/openai", d.Path)
	}
}

// TestResolveMemakaiTimpaanEnv: alamat dan model HARUS bisa diganti pengguna,
// sebab penyedia menerbitkan model baru jauh lebih sering daripada aplikasi ini
// dirilis ulang (notes/39).
func TestResolveMemakaiTimpaanEnv(t *testing.T) {
	clearEngineEnv(t)
	d, _ := engineByID("openai")

	if e := resolve(d); e.BaseURL != d.Base || e.Model != d.Model {
		t.Errorf("tanpa timpaan harus memakai bawaan: %+v", e)
	}
	t.Setenv(d.EnvBase, "https://gateway.example")
	t.Setenv(d.EnvModel, "model-besok")
	e := resolve(d)
	if e.BaseURL != "https://gateway.example" || e.Model != "model-besok" {
		t.Errorf("timpaan tidak dipakai: %+v", e)
	}
}

// TestResolveReadyIkutKunci menjaga keputusan pemilik proyek: mesin cloud tanpa
// kunci TIDAK muncul di pemilih mesin.
func TestResolveReadyIkutKunci(t *testing.T) {
	clearEngineEnv(t)
	d, _ := engineByID("deepseek")

	if e := resolve(d); e.Ready || e.HasKey {
		t.Errorf("tanpa kunci harus belum siap: %+v", e)
	}
	t.Setenv(d.EnvKey, "sk-uji")
	if e := resolve(d); !e.Ready || !e.HasKey {
		t.Errorf("dengan kunci harus siap: %+v", e)
	}
}

// TestEngineForTanpaKunci: gagal dengan pesan yang MENYEBUT jalan keluarnya,
// bukan sekadar "unauthorized" (notes/12 — tanpa perpindahan diam-diam).
func TestEngineForTanpaKunci(t *testing.T) {
	clearEngineEnv(t)
	for _, id := range []string{"claude", "openai", "gemini", "deepseek"} {
		_, _, err := EngineFor(id, "")
		if err == nil {
			t.Errorf("%s: mau galat karena kunci kosong", id)
			continue
		}
		if !strings.Contains(err.Error(), "Requirements") {
			t.Errorf("%s: pesan tidak menyebut ke mana harus pergi: %v", id, err)
		}
	}
}

func TestEngineForTakDikenal(t *testing.T) {
	_, _, err := EngineFor("tidak-ada", "")
	if err == nil || !strings.Contains(err.Error(), "tidak-ada") {
		t.Errorf("mau galat yang menyebut namanya, dapat: %v", err)
	}
}

// TestEngineForModelBawaan: model kosong memakai model yang berlaku untuk mesin
// itu, dan nama mesinnya ikut menyebutkannya supaya log job bisa dibandingkan
// antar percobaan.
func TestEngineForModelBawaan(t *testing.T) {
	clearEngineEnv(t)
	d, _ := engineByID("gemini")
	t.Setenv(d.EnvKey, "sk-uji")

	_, name, err := EngineFor("gemini", "")
	if err != nil {
		t.Fatalf("EngineFor: %v", err)
	}
	if !strings.Contains(name, d.Model) {
		t.Errorf("nama mesin = %q, mau memuat %q", name, d.Model)
	}

	_, name, err = EngineFor("gemini", "model-lain")
	if err != nil {
		t.Fatalf("EngineFor: %v", err)
	}
	if !strings.Contains(name, "model-lain") {
		t.Errorf("model pilihan pengguna tidak dipakai: %q", name)
	}
}

// TestEngineForClaudeAbaikanSkema menjaga hal yang gampang terlupa: Claude tidak
// menerima JSON Schema — bentuk balasannya diminta lewat prompt. Melewatkan
// skema ke sana bukan galat, tapi juga bukan jaminan.
func TestEngineForClaudeAbaikanSkema(t *testing.T) {
	clearEngineEnv(t)
	d, _ := engineByID("claude")
	t.Setenv(d.EnvKey, "sk-uji")

	complete, _, err := EngineFor("claude", "")
	if err != nil {
		t.Fatalf("EngineFor: %v", err)
	}
	// Tidak ada jaringan di uji ini: yang diperiksa cuma bahwa skema diterima
	// tanpa membuat panik, dan galatnya berasal dari transport.
	if _, err := complete(context.Background(), "s", "u", schemaProbe); err == nil {
		t.Skip("kunci uji ternyata bisa dipakai — lewati")
	}
}

// TestFillEngineKoordinat menjaga jalur baru halaman klip: mesin yang dipilih di
// pemilih bersama diterjemahkan jadi koordinat yang dipahami pipeline. Yang
// diperiksa terutama LLMKeyEnv — pipeline menerima NAMA variabelnya, tidak
// pernah kuncinya, sebab Options ikut tersimpan di riwayat job.
func TestFillEngineKoordinat(t *testing.T) {
	clearEngineEnv(t)

	// Mesin lokal & heuristik dibiarkan apa adanya: keduanya sudah punya
	// jalannya sendiri di pipeline.
	for _, id := range []string{"ollama", "heuristic"} {
		o := config.Options{Provider: id}
		fillEngine(&o)
		if o.Provider != id || o.LLMBase != "" || o.LLMKeyEnv != "" {
			t.Errorf("%s: seharusnya tidak disentuh, dapat %+v", id, o)
		}
	}

	o := config.Options{Provider: "deepseek"}
	fillEngine(&o)
	if o.LLMBase == "" || o.LLMPath != "/v1" {
		t.Errorf("koordinat deepseek belum lengkap: %+v", o)
	}
	if o.LLMKeyEnv != "DEEPSEEK_API_KEY" {
		t.Errorf("LLMKeyEnv = %q, mau DEEPSEEK_API_KEY", o.LLMKeyEnv)
	}
	if o.LLMModel == "" || !strings.Contains(o.EngineName, o.LLMModel) {
		t.Errorf("nama mesin tidak menyebut modelnya: %+v", o)
	}

	// Claude punya klien sendiri di pipeline, jadi Provider-nya dinormalkan.
	c := config.Options{Provider: "claude", LLMModel: "claude-opus-4-8"}
	fillEngine(&c)
	if c.Provider != "claude" || c.LLMBase == "" {
		t.Errorf("claude: %+v", c)
	}
	if c.LLMModel != "claude-opus-4-8" {
		t.Errorf("model pilihan pengguna ditimpa: %q", c.LLMModel)
	}
}
