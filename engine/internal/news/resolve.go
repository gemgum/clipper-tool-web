package news

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Resolve menerjemahkan pengalih Google News jadi alamat artikel yang
// sebenarnya. URL yang sudah berupa alamat artikel dikembalikan apa adanya.
//
// Hasilnya disimpan sebagai cache karena resolusi menuntut satu peluncuran
// browser (~2–3 detik). Cache ini juga menghemat di tempat lain: begitu sebuah
// tautan pernah diresolusi, FetchContent tidak perlu memanggil browser lagi —
// ia cukup mengunduh artikelnya lewat HTTP biasa, yang jauh lebih cepat.
func Resolve(ctx context.Context, link string, browse Browser, cacheDir string) (string, error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return "", fmt.Errorf("the link is empty")
	}
	if !IsGoogleNewsLink(link) {
		return link, nil
	}
	if original, ok := loadResolved(cacheDir, link); ok {
		return original, nil
	}
	// Jalur utama: tanya Google lewat RPC yang dipakai halamannya sendiri
	// (google.go). Tidak butuh browser, di bawah satu detik, dan berhasil pada
	// tautan yang tidak pernah berpindah di Chrome headless — itu justru
	// keluhan aslinya.
	if original, err := decodeGoogleNews(ctx, link); err == nil && !IsGoogleNewsLink(original) {
		saveResolved(cacheDir, link, original)
		return original, nil
	}
	if browse == nil {
		return "", fmt.Errorf("search-result links must be opened in a browser, but no browser is available: install Chrome/Chromium, or open the link yourself")
	}
	u, err := url.Parse(link)
	if err != nil {
		return "", err
	}

	// DICOBA BEBERAPA KALI, bukan sekali.
	//
	// Pengalih Google berpindah lewat JavaScript, dan DOM bisa terpotret tepat
	// sebelum perpindahannya selesai — hasilnya masih halaman Google. Sebelum
	// ini kegagalan itu langsung dilempar ke pengguna dengan pesan "try again in
	// a moment": kode menyuruh orang mengerjakan hal yang bisa dikerjakannya
	// sendiri. Akibatnya terlihat di lapangan sebagai "klik tidak memunculkan
	// gambar, tapi copy lalu tempel berhasil" — sebab menekan copy adalah
	// percobaan KEDUA yang kebetulan berhasil (7 Agustus 2026).
	var last error
	for attempt := 0; attempt < resolveTries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(resolveWait):
			}
		}
		dom, err := browse(ctx, link)
		if err != nil {
			last = err
			continue
		}
		art, err := parseArticle(dom, u, "")
		if err != nil {
			last = err
			continue
		}
		if IsGoogleNewsLink(art.URL) {
			last = fmt.Errorf("the redirect has not finished yet")
			continue
		}
		saveResolved(cacheDir, link, art.URL)
		return art.URL, nil
	}
	return "", fmt.Errorf("could not reach the original article behind that search result (%v): open the link in a browser and paste the address that appears", last)
}

const (
	// resolveTries: tiga percobaan sudah menutup hampir semua kegagalan yang
	// terlihat; lebih dari itu hanya membuat pengguna menunggu lebih lama untuk
	// tautan yang memang rusak.
	resolveTries = 3
	// resolveWait: jeda antar percobaan. Yang ditunggu bukan jaringan melainkan
	// JavaScript Google yang belum sempat berpindah halaman.
	resolveWait = 1200 * time.Millisecond
)

// --- cache resolusi ---

func resolvedPath(dir, link string) string {
	h := sha256.Sum256([]byte(link))
	return filepath.Join(dir, "cache", "resolved", hex.EncodeToString(h[:16])+".txt")
}

func loadResolved(dir, link string) (string, bool) {
	if dir == "" {
		return "", false
	}
	raw, err := os.ReadFile(resolvedPath(dir, link))
	if err != nil {
		return "", false
	}
	original := strings.TrimSpace(string(raw))
	if original == "" || IsGoogleNewsLink(original) {
		return "", false
	}
	return original, true
}

func saveResolved(dir, link, original string) {
	if dir == "" {
		return
	}
	path := resolvedPath(dir, link)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return // cache itu percepatan, bukan keharusan
	}
	_ = os.WriteFile(path, []byte(original), 0o644)
}
