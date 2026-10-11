package pipeline

import (
	"context"
	"fmt"
	"os"

	"github.com/gemgum/clipper/engine/internal/score/ollama"
)

// resolveOllama menentukan Ollama MANA dan model MANA yang dipakai job ini,
// lalu mengembalikan klien beserta satu kalimat yang menyebut keduanya.
//
// Ada karena satu laporan yang tidak bisa dijelaskan: GUI menampilkan
// "llama3.1:latest — ready", tapi job berhenti dengan pesan yang menyebut
// qwen2.5. Dari luar itu mustahil dilacak, sebab tidak ada satu pun tempat yang
// menyatakan model apa yang BENAR-BENAR dipakai sebelum pekerjaan dimulai —
// nama model hanya muncul di pesan galat, dan nilai kosong diam-diam berubah
// jadi bawaan di dua tempat berbeda (config.Validate dan ollama.New).
//
// Sekarang: nama model dicocokkan dengan yang benar-benar TERPASANG, alamatnya
// ditemukan sendiri bila tidak disebut, dan keduanya dicatat sebelum job jalan.
// Model yang diminta tapi tidak terpasang TIDAK diganti diam-diam — job berhenti
// dengan pesan yang menyebutkan apa yang ada (notes/12).
func resolveOllama(ctx context.Context, url, model string) (*ollama.Client, string, error) {
	st := ollama.Status(ctx, url)
	if !st.Running {
		where := "http://localhost:11434"
		if url != "" {
			where = url
		}
		return nil, "", fmt.Errorf(
			"no local LLM server answered. Tried %v. Start Ollama (`ollama serve`), or any OpenAI-compatible server (llama.cpp, LocalAI, llamafile, vLLM, LiteLLM), or set OLLAMA_HOST if yours listens elsewhere (looked for %s)",
			ollama.Candidates(), where)
	}
	if len(st.Installed) == 0 {
		return nil, "", fmt.Errorf("Ollama at %s has no models installed: run `ollama pull llama3.1`", st.URL)
	}

	// Nama dicocokkan tanpa tag: yang tersimpan di setelan bisa "llama3.1"
	// sementara yang terpasang bernama "llama3.1:latest".
	name := ""
	for _, m := range st.Installed {
		if m.Name == model || ollama.BaseName(m.Name) == ollama.BaseName(model) {
			name = m.Name
			break
		}
	}
	if name == "" {
		have := make([]string, 0, len(st.Installed))
		for _, m := range st.Installed {
			have = append(have, m.Name)
		}
		return nil, "", fmt.Errorf(
			"model %q is not installed in Ollama at %s: installed: %v. Pick one of those in the GUI, or run `ollama pull %s`",
			model, st.URL, have, model)
	}

	c := ollama.New(st.URL, name)
	c.Kind = st.Kind
	// Konteks model dipakai apa adanya: meminta 8192 pada model yang hanya
	// mendukung 4096 membuat Ollama membalas kosong tanpa pesan galat, dan
	// itulah kegagalan yang paling sering terlihat di model kecil.
	for _, m := range st.Installed {
		if m.Name == name {
			c.NumCtx = m.Context
			break
		}
	}
	label := "Ollama"
	if st.Kind == ollama.KindOpenAI {
		label = "Local LLM server"
	}
	return c, fmt.Sprintf("%s (%s) at %s: %s", label, name, st.URL, ollama.Where(st.URL)), nil
}

// cloudClient merakit klien untuk penyedia yang bicara /chat/completions.
//
// Koordinatnya (alamat, jalur, nama variabel kuncinya) DIISI SERVER dari daftar
// mesin di internal/api — bukan dari permintaan HTTP, dan bukan tabel kedua di
// sini. Dua alasannya berbeda: kuncinya sendiri tidak pernah lewat JSON
// (LLMKeyEnv hanya menyebut NAMA variabelnya), dan tabel penyedia yang ada di
// dua tempat cepat atau lambat berbeda isi.
func (p *Pipeline) cloudClient() (*ollama.Client, string, error) {
	name := p.Opts.EngineName
	if name == "" {
		name = p.Opts.Provider
	}
	if p.Opts.LLMBase == "" {
		return nil, "", fmt.Errorf("unknown score engine %q: pick one on the Requirements page, or choose ollama or heuristic", p.Opts.Provider)
	}
	key := ""
	if p.Opts.LLMKeyEnv != "" {
		key = os.Getenv(p.Opts.LLMKeyEnv)
	}
	if key == "" {
		return nil, "", fmt.Errorf("%s has no API key yet: add it on the Requirements page", name)
	}
	c := ollama.New(p.Opts.LLMBase, p.Opts.LLMModel)
	c.Kind, c.Path, c.APIKey = ollama.KindOpenAI, p.Opts.LLMPath, key
	return c, name, nil
}
