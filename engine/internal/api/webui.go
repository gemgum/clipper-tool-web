package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Menyajikan GUI sebagai berkas statis.
//
// Sampai kini GUI dijalankan "npm run dev" di port 3000. Itu mustahil dituntut
// dari pengguna aplikasi: ia berarti Node.js terpasang, terminal terbuka, dan
// dua alamat yang harus cocok. Seluruh GUI ini berjalan di browser — tidak ada
// bagian yang butuh server Next — jadi `next build` menghasilkan HTML+JS biasa
// dan engine menyajikannya sendiri.
//
// Akibat sampingannya yang paling berharga: GUI dan API jadi satu asal (origin)
// yang sama, sehingga tidak ada urusan CORS sama sekali di aplikasi jadi. Yang
// perlu dilakukan shell desktop tinggal membuka satu alamat.

// webUI mengembalikan handler untuk berkas GUI, atau nil bila GUI-nya tidak
// ada di sebelah biner (mis. checkout yang belum pernah menjalankan npm build).
func webUI(dir string) http.Handler {
	if dir == "" {
		return nil
	}
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Halaman GUI tidak boleh disinggahi cache: berkasnya berganti setiap
		// kali GUI dibangun ulang, sedangkan namanya tetap. Berkas di /_next/
		// justru sebaliknya — namanya sudah mengandung sidik jari isi.
		if !strings.HasPrefix(r.URL.Path, "/_next/") {
			w.Header().Set("Cache-Control", "no-cache")
		}
		w.Header().Set("Content-Security-Policy", csp)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		fs.ServeHTTP(w, r)
	})
}

// csp membatasi dari mana halaman boleh memuat apa pun.
//
// Aturan proyek sudah menyebut "nol alamat luar di antarmuka" (notes/29), tapi
// aturan hanya berlaku selama ada yang mengingatnya. Header ini menjadikannya
// hal yang tidak bisa dilanggar tanpa ketahuan: satu tautan CDN yang tak sengaja
// ikut akan langsung gagal memuat, bukan diam-diam bekerja di mesin yang punya
// internet lalu putus di mesin yang tidak.
//
//	img-src   http/https ikut: gambar artikel di tab kartu memang datang dari
//	          medianya. Itu ISI, bukan tampilan — pengecualian yang disebut
//	          notes/29 secara khusus.
//	style-src 'unsafe-inline' terpaksa: React menulis style pada elemen.
//	script-src 'unsafe-inline' juga terpaksa, dan ini diperiksa bukan dikira:
//	          `next build` menaruh tujuh <script> inline di index.html untuk
//	          data hidrasinya. Nonce tidak bisa dipakai (berkasnya statis,
//	          engine tidak menyusun HTML-nya), dan hash berganti tiap build.
//	          Yang tetap didapat: skrip dari HOST LUAR tetap ditolak, dan
//	          'unsafe-eval' tetap tidak ada.
//	frame-ancestors 'none' menutup clickjacking; base-uri 'none' menutup
//	          pengalihan seluruh path relatif lewat satu tag <base>.
const csp = "default-src 'self'; " +
	"img-src 'self' data: blob: https: http:; " +
	"media-src 'self' blob:; " +
	"font-src 'self' data:; " +
	"style-src 'self' 'unsafe-inline'; " +
	"script-src 'self' 'unsafe-inline'; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'none'; " +
	"form-action 'none'; " +
	"frame-ancestors 'none'"

// guiMissingPage menjelaskan cara membangun GUI, alih-alih 404 kosong.
//
// Ini yang dilihat pengembang yang baru meng-clone repo lalu membuka alamat
// engine — pesan yang menyebut perintahnya jauh lebih berguna daripada
// halaman kosong yang menyuruh menebak.
func guiMissingPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprint(w, `<!doctype html>
<meta charset="utf-8">
<title>Clipper — the interface has not been built yet</title>
<body style="font:15px/1.6 system-ui;background:#0f1115;color:#e6e9ef;padding:48px;max-width:640px;margin:auto">
<h1 style="font-size:20px">The interface has not been built yet</h1>
<p>The engine is running, but there are no interface files next to it. Build them once:</p>
<pre style="background:#1e222b;padding:14px;border-radius:8px">cd gui &amp;&amp; npm install &amp;&amp; npm run build</pre>
<p>Then reload this page. The API itself is already up at <code>/api/health</code>.</p>
</body>`)
}

// ShellURLPrefix menandai baris alamat yang dibaca jendela aplikasi dari
// stdout engine. Kontrak antara `clipper serve -shell` dan desktop/.
const ShellURLPrefix = "clipper-url: "

// AppURL menyusun alamat yang dibuka jendela aplikasi: GUI di akar, kunci sesi
// di query. Satu alamat, dan itulah seluruh yang perlu diketahui shell desktop.
func AppURL(base, token string) string {
	if token == "" {
		return base + "/"
	}
	return base + "/?token=" + token
}

// GUIStatus melaporkan letak berkas GUI untuk dicetak di banner.
func GUIStatus(dir string) string {
	if dir == "" {
		return "(not built: run: cd gui && npm run build)"
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	if _, err := os.Stat(filepath.Join(abs, "index.html")); err != nil {
		return abs + " (incomplete)"
	}
	return abs
}
