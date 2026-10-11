package capture

import (
	"context"
	"os"
	"testing"
	"time"
)

// Uji manual: jalankan dengan
//
//	CLIPPER_TEST_URL="https://..." go test ./internal/capture/ -run TestDumpDOMManual -v
//
// Dilewati bila variabel itu tidak diset, supaya tidak ikut di CI.
func TestDumpDOMManual(t *testing.T) {
	url := os.Getenv("CLIPPER_TEST_URL")
	if url == "" {
		t.Skip("set CLIPPER_TEST_URL untuk menjalankan uji manual ini")
	}
	bin := Find()
	t.Log("browser:", bin)
	c := New(bin)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	started := time.Now()
	dom, err := c.DumpDOM(ctx, url, 15000)
	t.Logf("elapsed: %.1fs", time.Since(started).Seconds())
	if err != nil {
		t.Fatal("ERROR:", err)
	}
	t.Log("DOM length:", len(dom))
}
