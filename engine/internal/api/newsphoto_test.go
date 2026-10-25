package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
)

// Foto berita diunduh SEKALI ke cache server; render Chrome (renderImage)
// memakai salinan itu sebagai data URI tanpa mengunduh ulang.
func TestNewsPhotoDownloadedOnce(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		w.Write(tinyPNG(4, 3))
	}))
	defer srv.Close()
	s := &Server{paths: config.Paths{DataDir: t.TempDir()}}
	url := srv.URL + "/foto.png"
	a, err := s.newsPhoto(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.newsPhoto(context.Background(), url)
	img, _ := s.renderImage(context.Background(), url)
	if a != b || hits.Load() != 1 || !strings.HasPrefix(img, "data:image/png;base64,") {
		t.Fatalf("harus sekali unduh (dapat %d) dan data URI (dapat %.30s)", hits.Load(), img)
	}
}
