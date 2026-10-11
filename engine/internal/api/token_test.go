package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// serveWith menjalankan satu permintaan lewat seluruh lapisan
// (penjaga + CORS + token) — urutan yang sama persis dengan Handler().
//
// Host diisi di sini bila permintaannya belum menyebut: httptest.NewRequest
// memakai "example.com", dan penjaga memang menolak nama itu. Uji yang ingin
// menguji penolakan tersebut menyetel Host-nya sendiri.
func serveWith(t *testing.T, s *Server, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	if r.Host == "" || r.Host == "example.com" {
		r.Host = "127.0.0.1:8787"
	}
	rec := httptest.NewRecorder()
	handler := s.withGuard(withCORS(s.withToken(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]string{"ok": "yes"})
	}))))
	handler.ServeHTTP(rec, r)
	return rec
}

// Tanpa kunci, permintaan ditolak. Ini pokok soalnya: sejak ada /api/browse,
// program lain di mesin yang sama bisa menelusuri berkas pengguna lewat engine.
func TestRequestWithoutTheKeyIsRejected(t *testing.T) {
	s := &Server{token: "rahasia"}

	rec := serveWith(t, s, httptest.NewRequest("GET", "/api/browse", nil))

	if rec.Code != 401 {
		t.Fatalf("kode = %d, mau 401", rec.Code)
	}
}

func TestTheKeyIsAcceptedFromTheCookieAndHeaders(t *testing.T) {
	s := &Server{token: "rahasia"}

	// Cookie: jalur utama. EventSource, <video src>, dan tautan unduh dibuat
	// browser dan tak satu pun bisa menyertakan header — cookie ikut sendiri.
	cookie := httptest.NewRequest("GET", "/api/browse", nil)
	cookie.AddCookie(&http.Cookie{Name: CookieName, Value: "rahasia"})
	if rec := serveWith(t, s, cookie); rec.Code != 200 {
		t.Errorf("cookie: kode = %d, mau 200", rec.Code)
	}

	header := httptest.NewRequest("GET", "/api/browse", nil)
	header.Header.Set("X-Clipper-Token", "rahasia")
	if rec := serveWith(t, s, header); rec.Code != 200 {
		t.Errorf("header: kode = %d, mau 200", rec.Code)
	}

	bearer := httptest.NewRequest("GET", "/api/browse", nil)
	bearer.Header.Set("Authorization", "Bearer rahasia")
	if rec := serveWith(t, s, bearer); rec.Code != 200 {
		t.Errorf("bearer: kode = %d, mau 200", rec.Code)
	}
}

func TestAWrongKeyIsRejected(t *testing.T) {
	s := &Server{token: "rahasia"}
	r := httptest.NewRequest("GET", "/api/browse", nil)
	r.AddCookie(&http.Cookie{Name: CookieName, Value: "salah"})

	if rec := serveWith(t, s, r); rec.Code != 401 {
		t.Fatalf("kode = %d, mau 401", rec.Code)
	}
}

// Inti perpindahan ke cookie: query adalah tempat paling mudah bocor (Referer,
// riwayat, tangkapan layar, salin-tempel), jadi API tidak boleh menerimanya lagi
// — bahkan ketika kuncinya benar.
func TestTheAPIDoesNotAcceptTheKeyFromTheQueryAnymore(t *testing.T) {
	s := &Server{token: "rahasia"}

	rec := serveWith(t, s, httptest.NewRequest("GET", "/api/browse?token=rahasia", nil))

	if rec.Code != 401 {
		t.Fatalf("kode = %d, mau 401 — kunci di query masih diterima", rec.Code)
	}
}

// Satu-satunya tempat query masih dibaca: halaman pertama yang dibuka shell.
// Di situ ia ditukar jadi cookie, dan sesudah itu tidak ada lagi kunci yang
// lewat bilah alamat.
func TestOpeningThePageExchangesTheQueryKeyForACookie(t *testing.T) {
	s := &Server{token: "rahasia"}

	rec := serveWith(t, s, httptest.NewRequest("GET", "/?token=rahasia", nil))

	if rec.Code != 200 {
		t.Fatalf("kode = %d, mau 200", rec.Code)
	}
	var got *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == CookieName {
			got = c
		}
	}
	if got == nil {
		t.Fatal("cookie sesi tidak dipasang")
	}
	if got.Value != "rahasia" {
		t.Errorf("isi cookie = %q, mau %q", got.Value, "rahasia")
	}
	if !got.HttpOnly {
		t.Error("cookie bisa dibaca JavaScript — satu skrip tersusup cukup untuk membawanya keluar")
	}
	if got.SameSite != http.SameSiteStrictMode {
		t.Error("cookie bukan SameSite=Strict — halaman lain bisa memancing browser mengirimkannya")
	}
	if got.Path != "/" {
		t.Errorf("path cookie = %q, mau /", got.Path)
	}
	// Cookie sesi: hilang saat jendela ditutup, sama seperti kuncinya sendiri
	// yang dibuat baru tiap engine dijalankan.
	if got.MaxAge != 0 || !got.Expires.IsZero() {
		t.Error("cookie diberi masa berlaku — kunci basi akan hidup lebih lama dari enginenya")
	}
}

// Kunci salah di alamat halaman tidak menghasilkan cookie apa pun. Kalau tidak,
// siapa pun bisa memasang cookie pilihannya sendiri lalu memakainya.
func TestAWrongKeyInThePageAddressSetsNoCookie(t *testing.T) {
	s := &Server{token: "rahasia"}

	rec := serveWith(t, s, httptest.NewRequest("GET", "/?token=salah", nil))

	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("cookie dipasang padahal kuncinya salah")
	}
}

// health tetap terbuka: shell desktop memakainya untuk tahu engine sudah hidup,
// sebelum sempat membaca berkas handshake.
func TestHealthStaysOpen(t *testing.T) {
	s := &Server{token: "rahasia"}

	if rec := serveWith(t, s, httptest.NewRequest("GET", "/api/health", nil)); rec.Code != 200 {
		t.Fatalf("kode = %d, mau 200", rec.Code)
	}
}

// Tanpa token yang diset, engine berperilaku seperti sebelumnya — itu yang
// menjaga alur pengembangan (GUI di :3000) tetap jalan.
func TestNoTokenMeansNoCheck(t *testing.T) {
	s := &Server{}

	if rec := serveWith(t, s, httptest.NewRequest("GET", "/api/browse", nil)); rec.Code != 200 {
		t.Fatalf("kode = %d, mau 200", rec.Code)
	}
}

// Halaman dari internet ditolak sebelum sampai ke handler. Tanpa ini, situs mana
// pun yang sedang dibuka pengguna bisa memerintah engine di komputernya.
func TestARemotePageIsRefused(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest("GET", "/api/browse", nil)
	r.Header.Set("Origin", "https://situs-asing.example")

	rec := serveWith(t, s, r)

	if rec.Code != 403 {
		t.Fatalf("kode = %d, mau 403", rec.Code)
	}
}

// Skema non-http diterima HANYA yang benar-benar dipakai shell desktop.
//
// Dulu di sini "selain http/https, terima saja" — yang berarti
// chrome-extension:// dan skema karangan mana pun ikut lolos. Kuncinya tetap
// menahannya, jadi itu bukan lubang; ia hanya longgar tanpa satu pun alasan.
func TestOnlyTheShellSchemesAreAccepted(t *testing.T) {
	for _, origin := range []string{
		"chrome-extension://abcdef", "evil-scheme://apa-saja", "moz-extension://x",
	} {
		if localOrigin(origin) {
			t.Errorf("origin %q diterima padahal bukan skema shell desktop", origin)
		}
	}
	for _, origin := range []string{"tauri://localhost", "app://index.html", "file://"} {
		if !localOrigin(origin) {
			t.Errorf("origin %q ditolak padahal dipakai shell desktop", origin)
		}
	}
}

func TestLocalPagesAreAllowed(t *testing.T) {
	for _, origin := range []string{
		"http://localhost:3000", "http://127.0.0.1:3000", "tauri://localhost", "null", "",
	} {
		s := &Server{}
		r := httptest.NewRequest("GET", "/api/browse", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		rec := serveWith(t, s, r)
		if rec.Code != 200 {
			t.Errorf("origin %q: kode = %d, mau 200", origin, rec.Code)
		}
		if origin != "" && origin != "null" && rec.Header().Get("Access-Control-Allow-Origin") != origin {
			t.Errorf("origin %q tidak dipantulkan (dapat %q)", origin, rec.Header().Get("Access-Control-Allow-Origin"))
		}
	}
}

// Kunci sesi harus berbeda tiap kali & cukup panjang untuk tidak bisa ditebak.
func TestEveryRunGetsItsOwnKey(t *testing.T) {
	a, b := NewToken(), NewToken()

	if a == b {
		t.Fatal("dua kunci berturut-turut sama")
	}
	if len(a) < 32 {
		t.Fatalf("kunci hanya %d karakter — terlalu pendek", len(a))
	}
}

// Berkas handshake adalah satu-satunya cara shell tahu port & kunci, jadi
// isinya harus lengkap — dan tidak boleh terbaca pengguna lain di mesin ini.
func TestHandshakeFileIsCompleteAndPrivate(t *testing.T) {
	dir := t.TempDir()

	path, err := WriteHandshake(dir, Handshake{
		URL: "http://127.0.0.1:41234", Port: 41234, Token: "kunci", PID: 42, Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("izin berkas = %o, mau 600", perm)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var h Handshake
	if err := json.Unmarshal(raw, &h); err != nil {
		t.Fatal(err)
	}
	if h.Port != 41234 || h.Token != "kunci" || h.URL == "" || h.PID != 42 {
		t.Errorf("isi handshake tidak lengkap: %+v", h)
	}
}
