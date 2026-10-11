package news

import (
	"context"
	"os"
	"testing"
	"time"
)

// Uji manual (butuh jaringan):
//
//	CLIPPER_TEST_NET=1 go test ./internal/news/ -run TestDecodeGoogleNewsManual -v
func TestDecodeGoogleNewsManual(t *testing.T) {
	if os.Getenv("CLIPPER_TEST_NET") == "" {
		t.Skip("set CLIPPER_TEST_NET=1 untuk menjalankan uji jaringan ini")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	arts, err := Search(ctx, "senjata api sekolah", 10, "id")
	if err != nil {
		t.Fatal(err)
	}
	ok := 0
	for _, a := range arts {
		started := time.Now()
		got, err := decodeGoogleNews(ctx, a.URL)
		if err != nil {
			t.Errorf("GAGAL %v", err)
			continue
		}
		ok++
		t.Logf("%4.0fms  %s", float64(time.Since(started).Milliseconds()), got)
	}
	t.Logf("berhasil %d/%d", ok, len(arts))
}
