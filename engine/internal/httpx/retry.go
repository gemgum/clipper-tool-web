// Package httpx menampung satu perilaku ulang-coba untuk SEMUA panggilan LLM.
//
// Ditaruh di lapisan http.RoundTripper, bukan di tiap klien, karena di sinilah
// satu-satunya tempat yang masih melihat KODE STATUS. Di atasnya, baik klien
// Claude maupun klien OpenAI-compatible sudah menerjemahkan balasan jadi kalimat
// untuk pengguna, dan mengulang berdasarkan tebakan atas kalimat itu adalah cara
// yang sudah terbukti salah (lihat isSchemaRefusal di paket ollama — sudah dua
// kali ditambal). Sebagai Transport ia juga berlaku untuk gateway pihak ketiga
// tanpa satu baris pun di sisi pemanggil.
package httpx

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

// Angka-angka ini SENGAJA konstanta, bukan setelan.
//
// Satu job klip video dua jam memanggil LLM puluhan kali (transkrip dipotong per
// 12/25 menit, lalu pemilihan momen). Di gateway bersama yang murah, satu 429 di
// panggilan ke sekian membunuh seluruh job — sesudah whisper-nya dibayar. Tiga
// percobaan dengan jeda berlipat adalah jawaban yang benar untuk semua orang;
// angka yang bisa diketik pengguna cuma menambah cara untuk salah (10x5 detik di
// gateway yang tumbang = job menggantung, dan aplikasinya yang disalahkan).
const (
	attempts = 3
	baseWait = time.Second
	maxWait  = 30 * time.Second
)

// retryable = status yang artinya "coba lagi nanti", dan HANYA itu.
//
// 400/401/404 tidak pernah diulang: itu salah konfigurasi (kunci, model, alamat),
// dan mengulangnya cuma menunda pesan galat yang justru dibutuhkan pengguna.
var retryable = map[int]bool{429: true, 500: true, 502: true, 503: true, 504: true}

type transport struct{ next http.RoundTripper }

// Retry membungkus satu Transport. next kosong = http.DefaultTransport.
func Retry(next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &transport{next}
}

func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	for i := 0; ; i++ {
		resp, err := t.next.RoundTrip(r)
		// Galat transport (DNS, sambungan ditolak) TIDAK diulang: pesannya sudah
		// menunjuk sebab yang nyata, dan mengulang cuma memperlambatnya.
		if err != nil || i == attempts-1 || !retryable[resp.StatusCode] {
			return resp, err
		}
		wait := backoff(i, resp.Header.Get("Retry-After"))
		resp.Body.Close()
		// Badan permintaan harus diputar ulang. GetBody diisi sendiri oleh
		// http.NewRequest untuk *bytes.Reader — yang dipakai kedua klien LLM.
		if r.Body != nil {
			if r.GetBody == nil {
				return resp, err
			}
			b, gerr := r.GetBody()
			if gerr != nil {
				return resp, err
			}
			r.Body = b
		}
		select {
		case <-time.After(wait):
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	}
}

// backoff memilih jeda: yang diminta server bila ada, selain itu berlipat dua.
func backoff(attempt int, retryAfter string) time.Duration {
	if n, err := strconv.Atoi(retryAfter); err == nil && n > 0 {
		return min(time.Duration(n)*time.Second, maxWait)
	}
	return min(baseWait*time.Duration(math.Pow(2, float64(attempt))), maxWait)
}

// imageClient = SATU klien unduhan foto (hasil peninjauan redundansi 25
// Oktober 2026): dulu ada empat, dengan batas waktu, User-Agent, dan Accept
// berbeda — foto yang sama bisa terunduh di satu jalur dan gagal di jalur lain.
var imageClient = &http.Client{Timeout: 30 * time.Second, Transport: Retry(nil)}

// GetImage mengunduh gambar (paling banyak limit byte) dan mengembalikan isi
// serta Content-Type-nya. Accept meminta JPEG/PNG terang-terangan: CDN "format
// otomatis" (Cloudinary f_auto) mengirim WebP bila diminta apa saja, dan Go
// tidak bisa membaca WebP tanpa dependency luar.
func GetImage(ctx context.Context, url string, limit int64) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "image/jpeg,image/png;q=0.9,*/*;q=0.5")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Clipper)")
	res, err := imageClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, limit))
	return raw, res.Header.Get("Content-Type"), err
}
