package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Alamat yang disetel pengguna (OLLAMA_HOST) harus menang atas tebakan apa pun,
// dan alamat tanpa skema tetap dipakai.
func TestCandidatesHonoursOllamaHost(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "192.168.1.9:11434")
	c := Candidates()
	if len(c) == 0 || c[0] != "http://192.168.1.9:11434" {
		t.Fatalf("kandidat pertama = %v, mau http://192.168.1.9:11434", c)
	}
	// "0.0.0.0" adalah alamat MENDENGARKAN, bukan tujuan.
	t.Setenv("OLLAMA_HOST", "http://0.0.0.0:11434")
	if c := Candidates(); c[0] != "http://127.0.0.1:11434" {
		t.Fatalf("0.0.0.0 tidak diterjemahkan: %v", c)
	}
}

// Discover memakai alamat yang benar-benar menjawab, dan diam saja bila tidak
// ada — bukan mengembalikan localhost yang belum tentu ada isinya.
func TestDiscoverPicksLiveAddress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			w.WriteHeader(404)
			return
		}
		w.Write([]byte(`{"models":[]}`))
	}))
	defer srv.Close()

	t.Setenv("OLLAMA_HOST", srv.URL)
	resetDiscoverCache()
	if got := Discover(context.Background()); got != srv.URL {
		t.Fatalf("Discover = %q, mau %q", got, srv.URL)
	}

	// Alamat mati: tidak ada yang menjawab → kosong, dan pemanggil yang
	// memutuskan apa artinya (pesan "Ollama tidak ditemukan", bukan galat
	// koneksi yang membingungkan di tengah job).
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")
	resetDiscoverCache()
	if got := Discover(context.Background()); got != "" && got != defaultURL {
		t.Fatalf("Discover pada alamat mati = %q", got)
	}
}

// Alamat PUBLIK tidak boleh ikut dipindai: kandidat diambil dari resolv.conf,
// dan di sebagian WSL isinya DNS publik (1.1.1.1).
func TestHostURLRejectsPublicAddress(t *testing.T) {
	if got := hostURL("1.1.1.1"); got != "" {
		t.Fatalf("alamat publik ikut jadi kandidat: %q", got)
	}
	if got := hostURL("172.23.80.1"); got != "http://172.23.80.1:11434" {
		t.Fatalf("gerbang WSL ditolak: %q", got)
	}
}
