package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
)

func multipartFile(t *testing.T, data []byte) (*bytes.Buffer, string) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "foto.bin")
	fw.Write(data)
	mw.Close()
	return &body, mw.FormDataContentType()
}

func tinyPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{200, 10, 10, 255})
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

// Unggah foto kartu: jenis dibaca dari isinya (bukan nama), path yang dibalas
// ada di folder unggahan foto, dan hanya path di sana yang boleh masuk kartu.
func TestNewsImageUploadAndInline(t *testing.T) {
	s := &Server{paths: config.Paths{DataDir: t.TempDir()}}

	body, ct := multipartFile(t, []byte("bukan gambar sama sekali"))
	r := httptest.NewRequest("POST", "/api/news/image", body)
	r.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	s.newsImage(w, r)
	if w.Code != 415 {
		t.Fatalf("teks diterima sebagai gambar: %d %s", w.Code, w.Body)
	}

	body, ct = multipartFile(t, tinyPNG(4, 4))
	r = httptest.NewRequest("POST", "/api/news/image", body)
	r.Header.Set("Content-Type", ct)
	w = httptest.NewRecorder()
	s.newsImage(w, r)
	var got struct{ Path string }
	json.Unmarshal(w.Body.Bytes(), &got)
	if w.Code != 200 || !strings.HasPrefix(got.Path, s.cardImageDir()) || !strings.HasSuffix(got.Path, ".png") {
		t.Fatalf("unggah PNG: %d %s", w.Code, w.Body)
	}

	uri, err := s.inlineCardImage(got.Path)
	if err != nil || !strings.HasPrefix(uri, "data:image/png;base64,") {
		t.Fatalf("path unggahan tidak ditanam: %v %.40s", err, uri)
	}
	outside := filepath.Join(s.paths.DataDir, "rahasia.png")
	os.WriteFile(outside, tinyPNG(2, 2), 0o644)
	for _, p := range []string{outside, filepath.Join(s.cardImageDir(), "..", "rahasia.png"), "/etc/passwd"} {
		if _, err := s.inlineCardImage(p); err == nil {
			t.Errorf("path di luar folder unggahan diterima: %s", p)
		}
	}
	if u, _ := s.inlineCardImage("https://x.id/a.jpg"); u != "https://x.id/a.jpg" {
		t.Error("alamat web ikut diubah")
	}
}

// Batas 15 MB ditolak dengan 413.
func TestNewsImageTooLarge(t *testing.T) {
	s := &Server{paths: config.Paths{DataDir: t.TempDir()}}
	big := append(tinyPNG(2, 2), make([]byte, cardImageMax)...)
	body, ct := multipartFile(t, big)
	r := httptest.NewRequest("POST", "/api/news/image", body)
	r.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	s.newsImage(w, r)
	if w.Code != 413 {
		t.Fatalf("berkas >15 MB: %d", w.Code)
	}
}

// Riwayat kartu membaca folder kartu yang DIPILIH pengguna, bukan DataDir/cards,
// dan membawa judul, sumber, serta rasio.
func TestListCardsUsesCardsRoot(t *testing.T) {
	data, picked := t.TempDir(), t.TempDir()
	s := &Server{paths: config.Paths{DataDir: data}, layout: config.Layout{DataDir: data, CardsDir: picked}}
	dir := filepath.Join(picked, "card-1")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "card.png"), tinyPNG(1080, 1350), 0o644)
	os.WriteFile(filepath.Join(dir, "source.txt"), []byte("Judul: Banjir di Bekasi\nMedia: ANTARA\nTanggal: x\n"), 0o644)

	w := httptest.NewRecorder()
	s.listCards(w, httptest.NewRequest("GET", "/api/cards", nil))
	var got []cardEntry
	json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Title != "Banjir di Bekasi" || got[0].Source != "ANTARA" || got[0].Ratio != "4:5" {
		t.Fatalf("riwayat: %s", w.Body)
	}
	r := httptest.NewRequest("DELETE", "/api/cards/card-1", nil)
	r.SetPathValue("id", "card-1")
	s.deleteCard(httptest.NewRecorder(), r)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("hapus kartu tidak menyentuh folder kartu pilihan")
	}
}
