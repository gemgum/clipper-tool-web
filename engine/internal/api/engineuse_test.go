package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Kegunaan mesin menentukan daftar: "chat" tidak muncul di transkripsi,
// "transcribe" ditolak untuk chat, kosong (nilai lama) boleh keduanya.
func TestEngineUse(t *testing.T) {
	clearEngineEnv(t)
	t.Setenv("CUSTOM_BASE_URL", "https://gw.example")
	t.Setenv("CUSTOM_API_KEY", "k")

	t.Setenv("CUSTOM_USE", "")
	if !audioEngine("custom") {
		t.Fatal("legacy engine (no use set) dropped from transcription")
	}
	t.Setenv("CUSTOM_USE", "chat")
	if audioEngine("custom") {
		t.Fatal("chat-only engine offered for transcription")
	}
	t.Setenv("CUSTOM_USE", "transcribe")
	if !audioEngine("custom") {
		t.Fatal("transcription engine missing from transcription")
	}
	if _, _, err := EngineFor("custom", ""); err == nil || !strings.Contains(err.Error(), "transcription only") {
		t.Fatalf("transcription engine accepted for chat: %v", err)
	}
	if useEnv(engineDef{ID: "claude", Kind: kindAnthropic, EnvKey: "ANTHROPIC_API_KEY"}) != "" {
		t.Fatal("Claude offered a speech-to-text use")
	}
}

// Test transkripsi memakai yang TERSIMPAN lewat jalur job (fillTranscriber →
// pipeline): alamat, kunci, model, DAN bahasa job ikut terkirim.
func TestTestEngineTranscribesSample(t *testing.T) {
	clearEngineEnv(t)
	t.Setenv("CUSTOM_USE", "transcribe")
	t.Setenv("CUSTOM_API_KEY", "saved-key")
	t.Setenv("CLIPPER_TRANSCRIBE_MODEL", "")
	var auth, path, lang string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path, lang = r.Header.Get("authorization"), r.URL.Path, r.FormValue("language")
		w.Write([]byte(`{"segments":[{"start":0,"end":5,"text":" Hello. This is a short test."}]}`))
	}))
	defer srv.Close()

	s := &Server{}
	rec := httptest.NewRecorder()
	t.Setenv("CUSTOM_BASE_URL", srv.URL+"/api/v1")
	body := `{"id":"custom","model":"openai/whisper-large-v3-turbo"}`
	s.testEngine(rec, httptest.NewRequest("POST", "/api/engines/test", strings.NewReader(body)))
	var out EngineTest
	json.Unmarshal(rec.Body.Bytes(), &out)
	if !out.OK || out.Mode != "transcribe" || out.Heard != "Hello. This is a short test." {
		t.Fatalf("result %+v", out)
	}
	if auth != "Bearer saved-key" || path != "/api/v1/audio/transcriptions" || lang != "id" {
		t.Fatalf("auth=%q path=%q language=%q", auth, path, lang)
	}
}
