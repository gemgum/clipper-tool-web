package api

import (
	"archive/zip"
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
)

// Unduhan massal harus MEMBAWA yang dicentang, dan menolak id yang salah
// SEBELUM satu byte pun terkirim — setelah itu 404 tidak bisa lagi dikirim.
func TestBulkZipCards(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "data", "cards", "card-123")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"card.png", "caption.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("isi"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := &Server{layout: config.Layout{DataDir: filepath.Join(root, "data")}}

	w := httptest.NewRecorder()
	s.bulkZip(w, httptest.NewRequest("GET", "/api/download?card=card-123", nil))
	if w.Code != 200 {
		t.Fatalf("status = %d, mau 200 (%s)", w.Code, w.Body.String())
	}
	z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range z.File {
		names = append(names, f.Name)
	}
	if len(names) != 2 {
		t.Fatalf("isi zip = %v, mau dua berkas", names)
	}

	// Id yang tidak berbentuk kartu ditolak, bukan diam-diam menghasilkan zip
	// kosong — dan penolakannya harus berupa status, bukan berkas rusak.
	w = httptest.NewRecorder()
	s.bulkZip(w, httptest.NewRequest("GET", "/api/download?card=../../etc", nil))
	if w.Code != 400 {
		t.Fatalf("id salah: status = %d, mau 400", w.Code)
	}
	w = httptest.NewRecorder()
	s.bulkZip(w, httptest.NewRequest("GET", "/api/download", nil))
	if w.Code != 400 {
		t.Fatalf("tanpa pilihan: status = %d, mau 400", w.Code)
	}
}
