package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Kunci sesi & asal yang diizinkan.
//
// Engine mendengar di 127.0.0.1, tapi di mesin pengguna itu tidak cukup: SETIAP
// program di komputer itu — dan setiap halaman web yang sedang dibuka —  bisa
// menghubunginya. Sejak ada /api/browse dan /api/locate, yang bisa dilakukannya
// bukan cuma memerintah engine, tapi juga menelusuri berkas pengguna.
//
// Karena itu dua lapis:
//
//	token   setiap permintaan harus membawa kunci yang dibuat baru tiap engine
//	        dijalankan, dan hanya ditulis ke berkas yang bisa dibaca pemiliknya;
//	origin  halaman web dari internet ditolak di tingkat CORS, jadi browser
//	        tidak pernah mengirimkan permintaannya sejak awal.

// NewToken membuat kunci sesi acak.
func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// rand.Read tidak pernah gagal di OS yang didukung Go; kalau toh gagal,
		// berhenti lebih baik daripada jalan dengan kunci yang bisa ditebak.
		panic("cannot generate a session token: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Handshake adalah isi berkas yang memberi tahu shell desktop cara menghubungi
// engine: port yang dipilih & kunci sesinya.
//
// Berkas, bukan argumen baris perintah: port baru diketahui SETELAH engine
// berhasil mendengar, dan kunci di baris perintah terlihat oleh setiap program
// lain lewat daftar proses.
type Handshake struct {
	URL     string `json:"url"`
	Port    int    `json:"port"`
	Token   string `json:"token"`
	PID     int    `json:"pid"`
	Version string `json:"version"`
}

// HandshakePath mengembalikan letak berkas handshake di dalam folder data.
func HandshakePath(dataDir string) string {
	return filepath.Join(dataDir, "engine.json")
}

// WriteHandshake menulis berkas handshake dengan izin hanya-pemilik.
func WriteHandshake(dataDir string, h Handshake) (string, error) {
	path := HandshakePath(dataDir)
	raw, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return "", err
	}
	// 0600: kuncinya sama saja dengan kata sandi ke seluruh isi berkas
	// pengguna, jadi pengguna lain di mesin yang sama tidak boleh membacanya.
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// SetToken menyalakan pemeriksaan kunci. Token kosong = pemeriksaan mati.
func (s *Server) SetToken(t string) { s.token = t }

// openPaths = jalur yang boleh diakses tanpa kunci.
//
// Hanya /api/health: shell desktop memakainya untuk tahu engine sudah hidup
// sebelum ia sempat membaca berkas handshake, dan jawabannya tidak memuat apa
// pun selain kata "ok".
var openPaths = map[string]bool{"/api/health": true}

// CookieName = nama cookie tempat kunci sesi tinggal.
const CookieName = "clipper_session"

// Kenapa cookie, bukan query.
//
// Dulu kuncinya dititipkan di "?token=" pada setiap alamat, sebab tidak semua
// permintaan dibuat oleh kode kita: EventSource (progres job), <video src>, dan
// tautan unduh dibentuk browser dan tidak bisa membawa header. Satu jalur untuk
// semuanya memang lebih sederhana — tapi query adalah tempat paling mudah bocor:
// ia ikut ke Referer, riwayat browser, tangkapan layar, dan salin-tempel.
//
// Sejak GUI disajikan engine sendiri, ketiganya SATU ASAL dengan engine, dan
// cookie ikut terkirim otomatis oleh ketiga-tiganya. Jadi query tinggal dipakai
// sekali — saat halaman pertama dibuka shell — untuk ditukar jadi cookie:
//
//	HttpOnly    JavaScript di halaman tidak bisa membacanya, jadi satu skrip
//	            yang tersusup tidak bisa membawa kuncinya keluar;
//	SameSite    Strict — halaman lain tidak bisa memancing browser mengirimkannya;
//	tanpa masa  cookie sesi: hilang saat jendela ditutup, sama seperti kuncinya
//	berlaku     sendiri yang dibuat baru tiap engine dijalankan.
//
// Akibatnya /api/ TIDAK lagi menerima kunci dari query sama sekali.

// requestToken mengambil kunci dari permintaan.
//
// Query sengaja tidak ada di sini: itulah inti perpindahan ke cookie.
func requestToken(r *http.Request) string {
	if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
		return c.Value
	}
	if v := r.Header.Get("X-Clipper-Token"); v != "" {
		return v
	}
	if v := r.Header.Get("Authorization"); strings.HasPrefix(v, "Bearer ") {
		return strings.TrimPrefix(v, "Bearer ")
	}
	return ""
}

// withToken menolak permintaan tanpa kunci yang benar.
func (s *Server) withToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" {
			next.ServeHTTP(w, r)
			return
		}
		// Halaman & berkas GUI tidak dijaga — isinya tidak rahasia, dan halaman
		// itu harus bisa dimuat LEBIH DULU. Di sinilah satu-satunya tempat
		// "?token=" masih dibaca: alamat yang dibuka shell ditukar jadi cookie,
		// dan sesudah itu tidak ada lagi kunci yang lewat bilah alamat.
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			// Mode web: tanpa cookie yang benar, satu-satunya halaman yang
			// terbuka adalah /login.
			if s.web && r.URL.Path != "/login" && !sameToken(requestToken(r), s.token) {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			if q := r.URL.Query().Get("token"); sameToken(q, s.token) {
				http.SetCookie(w, &http.Cookie{
					Name:     CookieName,
					Value:    s.token,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteStrictMode,
				})
			}
			next.ServeHTTP(w, r)
			return
		}
		if openPaths[r.URL.Path] || (s.web && r.URL.Path == "/api/login") || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if !sameToken(requestToken(r), s.token) {
			if s.web {
				writeErr(w, 401, "you are signed out: reload the page to sign in again")
				return
			}
			writeErr(w, 401, "missing or wrong session key: open the app window again")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// sameToken membandingkan dua kunci dalam waktu tetap.
//
// Token dibandingkan pada setiap permintaan dari sumber mana pun, jadi lama
// perbandingannya tidak boleh membocorkan berapa karakter awal yang sudah benar.
func sameToken(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// localOrigin melaporkan apakah sebuah Origin datang dari mesin ini sendiri.
//
// Halaman web mana pun bisa menyuruh browser pengunjungnya menghubungi
// 127.0.0.1; yang menghentikannya adalah browser menolak MEMBACA jawabannya
// tanpa izin CORS. Jadi izin itu hanya diberikan kepada halaman lokal —
// GUI Next.js di :3000 saat pengembangan, dan jendela aplikasi nanti.
//
// extra = nama host publik mode web (-host); asal dari sana juga diterima.
func localOrigin(origin string, extra ...string) bool {
	if origin == "" || origin == "null" {
		return true // permintaan bukan-browser, atau halaman dari berkas
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "http", "https":
		host := u.Hostname()
		if slices.Contains(extra, strings.ToLower(host)) {
			return true
		}
		// Jendela Tauri di Windows memakai asal http://tauri.localhost, jadi
		// seluruh keluarga *.localhost ikut diterima — nama itu memang dijamin
		// menunjuk ke mesin sendiri.
		return host == "127.0.0.1" || host == "localhost" || host == "::1" ||
			strings.HasSuffix(host, ".localhost")
	}
	// Skema milik shell desktop, disebut satu per satu.
	//
	// Dulu di sini "selain http/https, terima saja". Niatnya tiga skema di
	// bawah, tapi yang tertulis adalah SEMUA — termasuk chrome-extension:// dan
	// skema karangan mana pun. Kuncinya tetap menahan permintaannya, jadi ini
	// bukan lubang; ia hanya longgar tanpa satu pun alasan.
	return shellSchemes[u.Scheme]
}

// shellSchemes = asal non-http yang dipakai shell desktop.
var shellSchemes = map[string]bool{"tauri": true, "app": true, "file": true}

// withCORS memberi izin baca hanya kepada halaman lokal.
func withCORS(next http.Handler, hosts ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if !localOrigin(origin, hosts...) {
			writeErr(w, 403, "this engine only serves apps running on this computer")
			return
		}
		if origin != "" {
			// Origin dipantulkan, bukan "*": dengan "*" browser menolak
			// mengirim header kunci pada permintaan yang butuh preflight.
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Clipper-Token, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
