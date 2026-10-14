package pipeline

import (
	"context"
	"strings"
	"testing"
)

// Model kosong (setelan "default") memakai model yang terpasang; model yang
// DIPILIH tapi tidak terpasang tetap ditolak, tidak diganti diam-diam.
func TestResolveOllamaModel(t *testing.T) {
	url := fakeOllama(t, func(string, string) string { return "" })

	c, label, err := resolveOllama(context.Background(), url, "")
	if err != nil {
		t.Fatalf("empty model: %v", err)
	}
	if c.Model != "test:latest" || !strings.Contains(label, "test:latest") {
		t.Fatalf("empty model resolved to %q (%s)", c.Model, label)
	}
	if c, _, err := resolveOllama(context.Background(), url, "test"); err != nil || c.Model != "test:latest" {
		t.Fatalf("tagless name: %v", err)
	}
	if _, _, err := resolveOllama(context.Background(), url, "qwen2.5"); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("missing model accepted: %v", err)
	}
}
