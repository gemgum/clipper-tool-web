package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/transcribe"
)

// Mesin AI GLOBAL (DESIGN-Clipper-Lanjutan.md §1, keputusan pemilik 9 Oktober
// 2026): satu mesin & model untuk semua alat, plus pengecualian per alat.
// Sebelumnya tiap halaman punya pemilih mesinnya sendiri (notes/39) — empat
// tempat untuk satu pertanyaan, dan keempatnya bisa tidak sinkron.
//
// Disimpan di .env lewat setEnv, sama seperti kunci API: berlaku seketika,
// bertahan setelah aplikasi ditutup, dan di mode web dipakai bersama satu tim.
// Permintaan kerja (job, tulis, caption) tetap membawa engine/model sendiri;
// GUI mengisinya dari sini, jadi tidak ada jalur lama yang patah.

// aiTools = alat yang boleh punya pengecualian.
var aiTools = []string{"clips", "news", "writer", "captions"}

type aiChoice struct {
	Engine string `json:"engine"`
	Model  string `json:"model"`
}

func aiEnvNames(tool string) (engine, model string) {
	if tool == "" {
		return "CLIPPER_AI_ENGINE", "CLIPPER_AI_MODEL"
	}
	t := strings.ToUpper(tool)
	return "CLIPPER_AI_ENGINE_" + t, "CLIPPER_AI_MODEL_" + t
}

func readAIChoice(tool string) *aiChoice {
	en, mn := aiEnvNames(tool)
	e := strings.TrimSpace(os.Getenv(en))
	if e == "" {
		return nil
	}
	return &aiChoice{Engine: e, Model: strings.TrimSpace(os.Getenv(mn))}
}

// effectiveAI: pengecualian alat bila ada, kalau tidak pilihan global, kalau
// belum pernah diisi juga: defaultEngineID. Model kosong =
// model bawaan mesin itu (yang disimpan di halaman Pengaturan).
func effectiveAI(tool string) aiChoice {
	c := readAIChoice(tool)
	if c == nil {
		c = readAIChoice("")
	}
	if c == nil {
		c = &aiChoice{Engine: defaultEngineID()}
	}
	out := *c
	if out.Model == "" && out.Engine != "heuristic" {
		if d, ok := engineByID(out.Engine); ok {
			out.Model = resolve(d).Model
		}
	}
	return out
}

func aiSettingsView() map[string]any {
	overrides := map[string]*aiChoice{}
	effective := map[string]aiChoice{}
	for _, t := range aiTools {
		overrides[t] = readAIChoice(t)
		effective[t] = effectiveAI(t)
	}
	global := readAIChoice("")
	if global == nil {
		g := effectiveAI("")
		global = &g
	}
	tr := map[string]any{"engine": savedTranscriber(), "model": "", "key_set": true}
	switch v := savedTranscriber(); v {
	case config.TranscriberWhisper:
	case config.TranscriberAIStudio:
		as := transcribe.AIStudioFromEnv()
		tr["model"], tr["key_set"] = as.Model, as.Key != ""
	default:
		d, _ := engineByID(v)
		tr["model"] = strings.TrimSpace(os.Getenv(transcribeModelEnv))
		tr["key_set"] = resolve(d).Ready
	}
	return map[string]any{"global": global, "overrides": overrides, "effective": effective, "tools": aiTools, "transcriber": tr}
}

// Mesin transkripsi bawaan job klip (notes/43): whisper lokal, AI Studio, atau
// id mesin OpenAI-compatible dari Engines & Keys (endpoint /audio/transcriptions).
// Job yang menyebut "transcriber" sendiri tetap menang.
const (
	transcriberEnv     = "CLIPPER_TRANSCRIBER"
	transcribeModelEnv = "CLIPPER_TRANSCRIBE_MODEL"
)

func savedTranscriber() string {
	v := strings.TrimSpace(os.Getenv(transcriberEnv))
	if v == config.TranscriberAIStudio || audioEngine(v) {
		return v
	}
	return config.TranscriberWhisper
}

// audioEngine: id mesin yang bisa dipakai untuk transkripsi — mesin
// OpenAI-compatible. Gemini dilewati: transkripsinya lewat AI Studio (API
// native), sebab jalur OpenAI-nya tidak punya endpoint audio.
func audioEngine(id string) bool {
	d, ok := engineByID(id)
	return ok && d.Kind == kindOpenAI && d.ID != "gemini"
}

// fillTranscriber menerjemahkan pilihan transkripsi job (id mesin) menjadi
// koordinat yang dibaca pipeline. whisper/aistudio dibiarkan apa adanya.
func fillTranscriber(o *config.Options) {
	id := o.Transcriber
	if id == config.TranscriberAPI {
		id = o.TranscribeEngine // job yang diulang: id-nya sudah tersimpan
	}
	if !audioEngine(id) {
		return
	}
	d, _ := engineByID(id)
	e := resolve(d)
	o.Transcriber, o.TranscribeEngine = config.TranscriberAPI, id
	o.TranscribeName, o.TranscribeBase, o.TranscribePath, o.TranscribeKeyEnv = d.Name, e.BaseURL, d.Path, d.EnvKey
	if o.TranscribeModel == "" {
		o.TranscribeModel = strings.TrimSpace(os.Getenv(transcribeModelEnv))
	}
}

func (s *Server) getAISettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, aiSettingsView())
}

// saveAISettings: {tool?: "", engine, model}. tool kosong = pilihan global;
// tool + engine kosong = hapus pengecualian alat itu (kembali ikut global).
func (s *Server) saveAISettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tool   string `json:"tool"`
		Engine string `json:"engine"`
		Model  string `json:"model"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.Tool == "transcribe" {
		if req.Engine != config.TranscriberWhisper && req.Engine != config.TranscriberAIStudio && !audioEngine(req.Engine) {
			writeErr(w, 400, "unknown transcriber "+req.Engine+": choose whisper, aistudio, or an OpenAI-compatible engine")
			return
		}
		s.setEnv(transcriberEnv, req.Engine)
		switch {
		case req.Engine == config.TranscriberAIStudio:
			s.setEnv("AI_STUDIO_MODEL", strings.TrimSpace(req.Model))
		case req.Engine != config.TranscriberWhisper:
			s.setEnv(transcribeModelEnv, strings.TrimSpace(req.Model))
		}
		writeJSON(w, 200, aiSettingsView())
		return
	}
	if req.Tool != "" && !contains(aiTools, req.Tool) {
		writeErr(w, 400, "unknown tool "+req.Tool+": choose one of "+strings.Join(aiTools, ", "))
		return
	}
	engine := strings.TrimSpace(req.Engine)
	if engine != "" && engine != "heuristic" {
		if _, ok := engineByID(engine); !ok {
			writeErr(w, 400, "unknown engine "+engine+": choose one of: "+engineIDs())
			return
		}
	}
	if engine == "" && req.Tool == "" {
		writeErr(w, 400, "the shared engine cannot be empty")
		return
	}
	en, mn := aiEnvNames(req.Tool)
	s.setEnv(en, engine)
	s.setEnv(mn, strings.TrimSpace(req.Model))
	writeJSON(w, 200, aiSettingsView())
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
