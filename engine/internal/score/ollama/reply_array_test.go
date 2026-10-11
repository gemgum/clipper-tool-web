package ollama

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// Gemini membalas galat sebagai array; pesannya harus sampai utuh.
func TestReadReplyErrorArray(t *testing.T) {
	body := `[{"error":{"code":429,"message":"You exceeded your current quota, please check your plan and billing details.","status":"RESOURCE_EXHAUSTED"}}]`
	resp := &http.Response{StatusCode: 429, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
	parsed, _, err := readReply(resp)
	if err != nil {
		t.Fatal(err)
	}
	if got := errorMessage(parsed.Error); !strings.Contains(got, "exceeded your current quota") {
		t.Fatalf("pesan = %q", got)
	}
}
