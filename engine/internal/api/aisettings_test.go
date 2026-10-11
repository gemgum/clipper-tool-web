package api

import "testing"

// Urutan jatuhnya: pengecualian alat → pilihan global → Ollama. Model kosong
// berarti model bawaan mesin itu, bukan string kosong yang dikirim ke penyedia.
func TestEffectiveAI(t *testing.T) {
	t.Setenv("CLIPPER_AI_ENGINE", "")
	t.Setenv("CLIPPER_AI_MODEL", "")
	t.Setenv("CLIPPER_AI_ENGINE_WRITER", "")
	t.Setenv("CLIPPER_AI_MODEL_WRITER", "")

	if got := effectiveAI("writer"); got.Engine != "ollama" {
		t.Fatalf("tanpa setelan = %+v, mau ollama", got)
	}
	t.Setenv("CLIPPER_AI_ENGINE", "custom")
	t.Setenv("CLIPPER_AI_MODEL", "glm-5.3")
	if got := effectiveAI("writer"); got != (aiChoice{"custom", "glm-5.3"}) {
		t.Fatalf("global tidak dipakai: %+v", got)
	}
	t.Setenv("CLIPPER_AI_ENGINE_WRITER", "deepseek")
	got := effectiveAI("writer")
	if got.Engine != "deepseek" || got.Model == "" {
		t.Fatalf("pengecualian alat / model bawaan: %+v", got)
	}
	if other := effectiveAI("captions"); other.Engine != "custom" {
		t.Fatalf("pengecualian bocor ke alat lain: %+v", other)
	}
}
