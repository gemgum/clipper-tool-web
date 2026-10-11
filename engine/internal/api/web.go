package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Mode web — HANYA di branch webview (notes/42).
//
// Engine jalan di VPS di belakang reverse proxy HTTPS, dipakai tim kecil dengan
// SATU kata sandi bersama. Bedanya dengan desktop cuma tiga, semuanya dari sini:
//
//	kunci     diturunkan dari kata sandi, ditukar jadi cookie lewat /login —
//	          bukan dibuat acak tiap engine jalan, supaya restart server tidak
//	          mengeluarkan seluruh tim;
//	path      path dari klien hanya boleh menunjuk ke folder unggahan. Di
//	          desktop path itu milik pengguna sendiri; di sini ia path di
//	          SERVER, dan tanpa pagar ini siapa pun yang tahu kata sandinya bisa
//	          menyuruh ffmpeg membaca berkas apa pun di VPS;
//	endpoint  yang menyentuh mesin server (menjelajah folder, memasang komponen,
//	          menunjuk program, membuka file manager) dimatikan — lihat webOff.

// webRoot = satu-satunya folder yang boleh ditunjuk path dari klien. Kosong =
// mode desktop, tanpa pagar.
//
// ponytail: variabel paket, sebab hostPath adalah fungsi bebas yang dipanggil
// di sebelas tempat; jadikan field Server bila suatu saat ada dua Server.
var webRoot string

// SetWeb menyalakan mode web. Sandi kosong hanya boleh untuk engine di loopback.
func (s *Server) SetWeb(password string) {
	s.web = true
	s.password = password
	// Sandi kosong = mode web lokal (engine di loopback, lihat cmdServe):
	// tanpa kunci, jadi withToken tidak memasang gerbang /login.
	if password != "" {
		mac := hmac.New(sha256.New, []byte(password))
		mac.Write([]byte("clipper-session"))
		s.token = base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	}
	webRoot = s.uploadDir()
	if s.mgr != nil {
		s.mgr.OnDone = func(input string) { dropUploads(input) }
	}
}

// dropUploads menghapus video sumber hasil unggahan setelah job-nya selesai.
//
// Di server, video 1–4 jam yang tertinggal menghabiskan disk dalam hitungan
// hari, dan yang dibutuhkan tim adalah HASILNYA — yang sudah diunduh atau masih
// bisa diunduh, sebab hasil tidak ikut terhapus. Hanya dipanggil setelah job
// SUKSES: job yang gagal atau dibatalkan bisa diulang tanpa mengunggah ulang.
// Di luar mode web dan di luar folder unggahan, tidak ada yang disentuh.
func dropUploads(paths ...string) {
	if webRoot == "" {
		return
	}
	for _, p := range paths {
		if p = insideWebRoot(p); p == "" || p == webRoot {
			continue
		}
		_ = os.Remove(p)
		// Folder unggahannya ikut hilang HANYA bila sudah kosong — caption
		// (.txt) dan hasil watermark ditulis di sebelah videonya.
		if d := filepath.Dir(p); d != webRoot {
			_ = os.Remove(d)
		}
	}
}

// uploadDir = folder tempat /api/upload menulis.
func (s *Server) uploadDir() string {
	return filepath.Join(s.paths.DataDir, "uploads")
}

// insideWebRoot mengembalikan p bila ia ada di dalam webRoot, atau "" bila
// tidak. Di mode desktop p dikembalikan apa adanya.
//
// "" sengaja dipilih sebagai penolakan: setiap pemanggil hostPath sudah
// menolak path kosong ("wajib diisi" / "tidak ditemukan"), jadi tidak ada
// pemanggil yang perlu diubah.
func insideWebRoot(p string) string {
	if webRoot == "" || p == "" {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(webRoot, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return abs
}

// webOff mematikan sebuah handler di mode web.
func (s *Server) webOff(h http.HandlerFunc) http.HandlerFunc {
	if !s.web {
		return h
	}
	return func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, 403, "not available in the web version: this works on the server's own files")
	}
}

// loginMu membuat tebakan kata sandi antre satu per satu.
var loginMu sync.Mutex

// login menukar kata sandi tim jadi cookie sesi.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	loginMu.Lock()
	defer loginMu.Unlock()
	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.password)) != 1 {
		// ponytail: satu tebakan per detik untuk SELURUH server; pasang
		// rate_limit/fail2ban di depan Caddy bila alamatnya mulai diserang.
		time.Sleep(time.Second)
		writeErr(w, 401, "wrong password")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    s.token,
		Path:     "/",
		MaxAge:   30 * 24 * 3600,
		HttpOnly: true,
		Secure:   true,
		// Lax, bukan Strict: tautan ke aplikasi yang dibuka dari chat tim harus
		// langsung masuk. Permintaan pengubah tetap terjaga oleh guard.go
		// (wajib JSON + Sec-Fetch-Site) dan CORS.
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// loginPage = satu-satunya halaman yang terbuka tanpa cookie.
func loginPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(loginHTML))
}

const loginHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Clipper — sign in</title>
<style>
body{margin:0;min-height:100vh;display:grid;place-items:center;font:15px system-ui,sans-serif;background:#111;color:#eee}
form{display:grid;gap:10px;width:260px}
input,button{font:inherit;padding:9px 11px;border-radius:6px;border:1px solid #444;background:#1c1c1c;color:inherit}
button{background:#e5484d;border-color:#e5484d;cursor:pointer}
p{margin:0;min-height:1.2em;color:#ff8a8a;font-size:13px}
</style></head><body>
<form id="f"><strong>Clipper</strong>
<input id="p" type="password" placeholder="Team password" autofocus autocomplete="current-password">
<button>Sign in</button><p id="e"></p></form>
<script>
f.onsubmit=async(ev)=>{ev.preventDefault();e.textContent="";
const r=await fetch("/api/login",{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({password:p.value})});
if(r.ok)location.replace("/");else e.textContent=(await r.json().catch(()=>({}))).error||"sign-in failed";};
</script></body></html>`
