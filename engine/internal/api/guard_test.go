package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// DNS rebinding: halaman si penyerang menyuruh browser menghubungi 127.0.0.1,
// tapi alamat yang diketik browser tetap nama miliknya — dan nama itulah yang
// sampai ke Host. Ini satu-satunya pemeriksaan yang menutupnya, sebab semua
// header lain bisa terlihat wajar.
func TestARequestForAnotherHostnameIsRefused(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest("GET", "/api/browse", nil)
	r.Host = "penyerang.example"

	if rec := serveWith(t, s, r); rec.Code != 403 {
		t.Fatalf("kode = %d, mau 403", rec.Code)
	}
}

func TestTheOwnAddressesAreAccepted(t *testing.T) {
	for _, host := range []string{
		"127.0.0.1:8787", "localhost:8787", "localhost", "[::1]:8787", "tauri.localhost",
	} {
		s := &Server{}
		r := httptest.NewRequest("GET", "/api/browse", nil)
		r.Host = host
		if rec := serveWith(t, s, r); rec.Code != 200 {
			t.Errorf("host %q: kode = %d, mau 200", host, rec.Code)
		}
	}
}

// -addr adalah keputusan sadar pengguna. Menolak alamat yang ia sebut sendiri
// berarti flag itu diam-diam tidak berfungsi — kegagalan yang paling
// membingungkan, sebab tidak ada pesannya.
func TestAnExplicitlyChosenAddressIsAccepted(t *testing.T) {
	s := &Server{}
	s.AllowHost("192.168.1.20")
	r := httptest.NewRequest("GET", "/api/browse", nil)
	r.Host = "192.168.1.20:8787"

	if rec := serveWith(t, s, r); rec.Code != 200 {
		t.Fatalf("kode = %d, mau 200", rec.Code)
	}
}

// 0.0.0.0 berarti "dengarkan di semua alamat", bukan nama yang bisa dituju.
// Menerimanya sebagai Host tidak menambah apa pun kecuali satu nama lolos.
func TestAWildcardBindDoesNotBecomeAnAllowedName(t *testing.T) {
	s := &Server{}
	s.AllowHost("0.0.0.0")
	r := httptest.NewRequest("GET", "/api/browse", nil)
	r.Host = "0.0.0.0:8787"

	if rec := serveWith(t, s, r); rec.Code != 403 {
		t.Fatalf("kode = %d, mau 403", rec.Code)
	}
}

// POST dengan Content-Type sederhana adalah bentuk yang bisa dikirim halaman
// asing TANPA izin siapa pun — tidak ada preflight, jadi tidak ada yang menolak.
// Mewajibkan application/json membalik keadaannya: browser jadi wajib meminta
// izin lebih dulu, dan izin itulah yang ditolak CORS.
func TestAPostThatIsNotJSONIsRefused(t *testing.T) {
	for _, ct := range []string{"text/plain", "application/x-www-form-urlencoded", "multipart/form-data", ""} {
		s := &Server{}
		r := httptest.NewRequest("POST", "/api/jobs", strings.NewReader("{}"))
		if ct != "" {
			r.Header.Set("Content-Type", ct)
		}
		if rec := serveWith(t, s, r); rec.Code != 415 {
			t.Errorf("content-type %q: kode = %d, mau 415", ct, rec.Code)
		}
	}
}

func TestAJSONPostIsAccepted(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest("POST", "/api/jobs", strings.NewReader("{}"))
	// Dengan charset — bentuk yang dikirim sebagian klien, dan tetap JSON.
	r.Header.Set("Content-Type", "application/json; charset=utf-8")

	if rec := serveWith(t, s, r); rec.Code != 200 {
		t.Fatalf("kode = %d, mau 200", rec.Code)
	}
}

// Unggahan video memang multipart: berkasnya bisa bergiga-giga dan harus
// dialirkan, bukan dimuat ke memori sebagai JSON.
func TestUploadStaysMultipart(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest("POST", "/api/upload", strings.NewReader("x"))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=abc")

	if rec := serveWith(t, s, r); rec.Code != 200 {
		t.Fatalf("kode = %d, mau 200", rec.Code)
	}
}

// Preflight tidak berbadan. Menolaknya karena "Content-Type bukan JSON" berarti
// menolak permintaan yang justru sedang meminta izin — dan izin itu yang dipakai
// GUI pengembangan di :3000.
func TestPreflightIsNotJudgedByItsContentType(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest("OPTIONS", "/api/jobs", nil)
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Access-Control-Request-Method", "POST")

	if rec := serveWith(t, s, r); rec.Code != 204 {
		t.Fatalf("kode = %d, mau 204", rec.Code)
	}
}

// Sec-Fetch-Site dipasang browser sendiri dan tidak bisa disetel halaman. Ia
// menangkap bentuk yang tidak membawa Origin sama sekali — <img src>,
// <script src>, navigasi dari halaman asing — yang lolos dari pemeriksaan CORS.
func TestACrossSiteRequestWithoutAnOriginIsRefused(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest("GET", "/api/browse", nil)
	r.Header.Set("Sec-Fetch-Site", "cross-site")

	if rec := serveWith(t, s, r); rec.Code != 403 {
		t.Fatalf("kode = %d, mau 403", rec.Code)
	}
}

// GUI pengembangan di localhost:3000 menghubungi engine di 127.0.0.1:8787.
// "localhost" dan "127.0.0.1" adalah host BERBEDA bagi browser, jadi labelnya
// "cross-site" — sama persis dengan halaman penyerang. Yang membedakan keduanya
// hanya Origin, dan itulah kenapa label ini tidak boleh dipakai sendirian DI
// MODE PENGEMBANGAN.
//
// Pernah dilanggar sekali: menolak semua "cross-site" membuat `npm run dev`
// menjawab "Cannot reach the engine" di ketiga halaman.
func TestTheDevelopmentGUIIsNotMistakenForAnAttacker(t *testing.T) {
	for _, origin := range []string{
		"http://localhost:3000", "http://127.0.0.1:3000", "http://tauri.localhost",
	} {
		s := &Server{} // kunci mati = checkout sumber
		r := httptest.NewRequest("GET", "/api/browse", nil)
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		r.Header.Set("Origin", origin)
		if rec := serveWith(t, s, r); rec.Code != 200 {
			t.Errorf("origin %q: kode = %d, mau 200", origin, rec.Code)
		}
	}
}

// Begitu kunci menyala, aplikasinya terpasang: GUI disajikan engine sendiri,
// jadi setiap permintaan yang sah adalah same-origin. Tidak ada cross-site yang
// wajar, jadi tidak ada yang perlu dimaafkan — termasuk yang Origin-nya lokal.
//
// Inilah yang membuat build yang dikirim ke pengguna seketat mungkin tanpa
// mematikan alur pengembangan.
func TestAnInstalledEngineRefusesEveryCrossSiteRequest(t *testing.T) {
	for _, origin := range []string{
		"http://localhost:3000", "http://tauri.localhost", "",
	} {
		s := &Server{token: "rahasia"}
		r := httptest.NewRequest("GET", "/api/browse", nil)
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		r.AddCookie(&http.Cookie{Name: CookieName, Value: "rahasia"})
		if rec := serveWith(t, s, r); rec.Code != 403 {
			t.Errorf("origin %q: kode = %d, mau 403", origin, rec.Code)
		}
	}
}

// Yang same-origin tetap jalan saat kunci menyala — itu seluruh alur aplikasi
// terpasang, jadi kalau ini ikut tertolak aplikasinya mati.
func TestAnInstalledEngineStillServesItsOwnPages(t *testing.T) {
	s := &Server{token: "rahasia"}
	r := httptest.NewRequest("GET", "/api/browse", nil)
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set("Origin", "http://127.0.0.1:8787")
	r.AddCookie(&http.Cookie{Name: CookieName, Value: "rahasia"})

	if rec := serveWith(t, s, r); rec.Code != 200 {
		t.Fatalf("kode = %d, mau 200", rec.Code)
	}
}

// Halaman dari internet tetap ditolak walau membawa Origin — di situ lapisan
// CORS yang menjawab, dan jawabannya 403.
func TestACrossSiteRequestFromTheInternetIsStillRefused(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest("GET", "/api/browse", nil)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	r.Header.Set("Origin", "https://penyerang.example")

	if rec := serveWith(t, s, r); rec.Code != 403 {
		t.Fatalf("kode = %d, mau 403", rec.Code)
	}
}

// Permintaan tanpa Host sama sekali bukan browser, dan bukan pula sesuatu yang
// perlu dilayani — HTTP/1.1 mewajibkannya.
func TestARequestWithoutAHostIsRefused(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest("GET", "/api/browse", nil)
	r.Host = ""

	rec := httptest.NewRecorder()
	s.withGuard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]string{"ok": "yes"})
	})).ServeHTTP(rec, r)

	if rec.Code != 403 {
		t.Fatalf("kode = %d, mau 403", rec.Code)
	}
}
