package api

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// Mesin tambahan pengguna: dibuat dari satu form, siap begitu kunci + alamat
// ada, dan bisa dihapus lagi tanpa sisa. Mode web tidak menampilkan Local AI.
func TestUserEngines(t *testing.T) {
	for _, k := range []string{userEnginesEnv, "ENGINE_U_MY_GATEWAY_NAME", "ENGINE_U_MY_GATEWAY_API_KEY", "ENGINE_U_MY_GATEWAY_BASE_URL", "ENGINE_U_MY_GATEWAY_MODEL"} {
		t.Setenv(k, "")
	}
	s := &Server{}
	s.paths.EnvFile = filepath.Join(t.TempDir(), ".env")
	call := func(fn func(*httptest.ResponseRecorder, string), body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		fn(rec, body)
		return rec
	}
	save := func(rec *httptest.ResponseRecorder, body string) {
		r := httptest.NewRequest("POST", "/api/engines", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		s.saveEngine(rec, r)
	}
	del := func(rec *httptest.ResponseRecorder, body string) {
		r := httptest.NewRequest("POST", "/api/engines/delete", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		s.deleteEngine(rec, r)
	}

	if rec := call(save, `{"id":"","name":"My gateway"}`); rec.Code != 400 {
		t.Fatalf("new engine without an address: %d, want 400", rec.Code)
	}
	rec := call(save, `{"id":"","name":"My gateway!","api_key":"k","base_url":"https://gw.example.com/v1/","model":"glm-5.3"}`)
	if rec.Code != 200 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	d, ok := engineByID("u_my_gateway")
	if !ok || !d.User || d.Name != "My gateway!" {
		t.Fatalf("created engine: %+v ok=%v", d, ok)
	}
	if e := resolve(d); !e.Ready || e.BaseURL != "https://gw.example.com" || e.Model != "glm-5.3" {
		t.Fatalf("resolved: %+v", e)
	}
	if rec := call(del, `{"id":"claude"}`); rec.Code != 400 {
		t.Fatalf("deleting a built-in engine: %d, want 400", rec.Code)
	}
	if rec := call(del, `{"id":"u_my_gateway"}`); rec.Code != 200 {
		t.Fatalf("delete: %d", rec.Code)
	}
	if _, ok := engineByID("u_my_gateway"); ok {
		t.Fatal("engine still listed after delete")
	}

	webRoot = t.TempDir()
	t.Cleanup(func() { webRoot = "" })
	if _, ok := engineByID("ollama"); ok {
		t.Fatal("Local AI must not be offered in web mode")
	}
}
