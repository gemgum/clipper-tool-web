package api

import (
	"mime"
	"net"
	"net/http"
	"slices"
	"strings"
)

// Penolakan permintaan yang tidak mungkin datang dari GUI ini.
//
// Kunci sesi sudah menjaga /api/, tapi ia MATI di mode pengembangan — dari
// checkout, engine mendengar di 8787 tanpa kunci supaya `npm run dev` bisa
// bicara ke sana. Selama itu berjalan, satu-satunya penjaga yang tersisa adalah
// pemeriksaan Origin, dan Origin tidak dikirim pada setiap bentuk permintaan.
//
// Tiga pemeriksaan di berkas ini berlaku di SEMUA mode, jadi mode pengembangan
// ikut terlindung tanpa harus menyalakan kunci di sana:
//
//	Host             menutup DNS rebinding — pada serangan itu alamat yang
//	                 diketik browser adalah nama si penyerang, dan nama itulah
//	                 yang muncul di sini, berapa pun IP yang akhirnya dituju;
//	Content-Type      permintaan berbadan wajib JSON. Halaman asing bisa mengirim
//	                 POST text/plain tanpa izin siapa pun; begitu ia harus
//	                 menyebut application/json, browser mewajibkan preflight —
//	                 dan preflight itulah yang ditolak CORS;
//	Sec-Fetch-Site   penanda yang dipasang browser sendiri dan tidak bisa
//	                 dipalsukan halaman. Menangkap bentuk yang tidak membawa
//	                 Origin sama sekali, mis. <img src> dan <script src> —
//	                 satu-satunya celah yang lolos dari pemeriksaan Origin.
//
// Seberapa ketat "cross-site" ditolak mengikuti KEADAAN KUNCI, dan itu bukan
// kompromi melainkan pengamatan:
//
//	kunci menyala  aplikasi terpasang. GUI disajikan engine sendiri, jadi setiap
//	(terpasang)    permintaan yang sah adalah same-origin — tidak ada satu pun
//	               cross-site yang wajar. Semuanya ditolak, titik.
//	kunci mati     checkout sumber. Di sini `npm run dev` di localhost:3000
//	(pengembangan) menghubungi engine di 127.0.0.1:8787, dan bagi browser itu
//	               "cross-site" sebab localhost dan 127.0.0.1 adalah host
//	               BERBEDA. Jendela Tauri dengan asal http://tauri.localhost
//	               jatuh di keranjang yang sama. Jadi di sini yang ditolak hanya
//	               yang TIDAK membawa Origin; sisanya diserahkan ke CORS, yang
//	               memang membedakan halaman lokal dari halaman internet.
//
// Hasilnya: build yang dikirim ke pengguna seketat mungkin, alur pengembangan
// tetap hidup, dan tidak ada mode yang kehilangan penjaga.

// jsonFreePaths = jalur yang badannya memang bukan JSON.
//
// Hanya unggahan berkas: bentuknya multipart karena videonya bisa berukuran
// gigabyte dan harus dialirkan, bukan dimuat ke memori.
// /api/news/image: foto kartu unggahan pengguna (newscard.go).
var jsonFreePaths = map[string]bool{"/api/upload": true, "/api/news/image": true}

// localHost melaporkan apakah sebuah nilai header Host menunjuk mesin ini.
func localHost(host string, extra []string) bool {
	if host == "" {
		// HTTP/1.1 mewajibkan Host; yang tidak membawanya bukan browser, dan
		// bukan pula sesuatu yang perlu dilayani.
		return false
	}
	h := host
	if only, _, err := net.SplitHostPort(host); err == nil {
		h = only
	}
	h = strings.ToLower(strings.Trim(h, "[]"))
	if slices.Contains(extra, h) {
		return true
	}
	// *.localhost ikut: jendela Tauri di Windows memakai nama itu, dan seluruh
	// keluarganya memang dijamin menunjuk ke mesin sendiri.
	return h == "127.0.0.1" || h == "localhost" || h == "::1" ||
		strings.HasSuffix(h, ".localhost")
}

// AllowHost menambah satu nama host yang boleh dipakai untuk menghubungi engine.
//
// Dipakai saat pengguna menyebut -addr sendiri: mengikat ke alamat non-loopback
// adalah keputusan sadar, dan menolaknya di sini berarti flag itu diam-diam
// tidak berfungsi.
func (s *Server) AllowHost(h string) {
	h = strings.ToLower(strings.Trim(h, "[]"))
	if h == "" || h == "0.0.0.0" || h == "::" {
		return
	}
	s.hosts = append(s.hosts, h)
}

// withGuard menolak permintaan sebelum sampai ke lapisan mana pun di bawahnya.
func (s *Server) withGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !localHost(r.Host, s.hosts) {
			writeErr(w, 403, "this engine only answers to its own address on this computer")
			return
		}
		if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") &&
			(s.token != "" || r.Header.Get("Origin") == "") {
			writeErr(w, 403, "this engine only answers pages it serves itself")
			return
		}
		if hasBody(r.Method) && !jsonFreePaths[r.URL.Path] {
			ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if !strings.EqualFold(ct, "application/json") {
				writeErr(w, 415, "this endpoint only accepts application/json")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// hasBody melaporkan apakah sebuah metode membawa badan permintaan yang perlu
// diperiksa jenisnya. OPTIONS tidak ikut: preflight memang tanpa badan, dan
// menolaknya berarti menolak permintaan yang justru sedang meminta izin.
func hasBody(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	}
	return false
}
