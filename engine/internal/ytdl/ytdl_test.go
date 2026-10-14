package ytdl

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVideoID(t *testing.T) {
	ok := map[string]string{
		"https://www.youtube.com/watch?v=jNQXAC9IVRw":        "jNQXAC9IVRw",
		"https://youtube.com/watch?v=jNQXAC9IVRw&t=10s":      "jNQXAC9IVRw",
		"https://m.youtube.com/watch?v=jNQXAC9IVRw":          "jNQXAC9IVRw",
		"https://youtu.be/jNQXAC9IVRw?si=abc":                "jNQXAC9IVRw",
		"https://www.youtube.com/shorts/jNQXAC9IVRw":         "jNQXAC9IVRw",
		"https://www.youtube.com/live/jNQXAC9IVRw?feature=x": "jNQXAC9IVRw",
		"  https://music.youtube.com/watch?v=jNQXAC9IVRw ":   "jNQXAC9IVRw",
	}
	for in, want := range ok {
		if got, err := VideoID(in); err != nil || got != want {
			t.Errorf("VideoID(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{
		"--exec=rm -rf ~",
		"https://evil.com/watch?v=jNQXAC9IVRw",
		"https://youtube.com.evil.com/watch?v=jNQXAC9IVRw",
		"http://127.0.0.1/watch?v=jNQXAC9IVRw",
		"file:///etc/passwd",
		"https://www.youtube.com/watch?v=--exec=xyz",
		"https://www.youtube.com/watch?v=short",
		"https://www.youtube.com/playlist?list=PL123",
	} {
		if id, err := VideoID(bad); err == nil {
			t.Errorf("VideoID(%q) accepted as %q", bad, id)
		}
	}
}

// Mesin pertama gagal → mesin kedua dipakai; tiap kegagalan tercatat.
func TestFallbackToNextEngine(t *testing.T) {
	dir := t.TempDir()
	// yt-dlp palsu: gagal untuk client bawaan, berhasil untuk android_vr.
	fake := filepath.Join(dir, "fake-yt-dlp")
	script := `#!/bin/sh
out=""; client=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift ;;
    --extractor-args) client="$2"; shift ;;
  esac
  shift
done
[ -z "$client" ] && { echo "ERROR: Sign in to confirm you're not a bot" >&2; exit 1; }
echo video > "$out"
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var log []string
	p, err := Download(context.Background(), "https://youtu.be/jNQXAC9IVRw",
		Options{Dir: filepath.Join(dir, "out"), YtDlp: fake, Log: func(s string) { log = append(log, s) }})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "jNQXAC9IVRw.mp4" {
		t.Fatalf("path %q", p)
	}
	all := strings.Join(log, "\n")
	if !strings.Contains(all, "yt-dlp default failed: ") || !strings.Contains(all, "Downloaded with yt-dlp android_vr") {
		t.Fatalf("log:\n%s", all)
	}
}

// Cek: mesin pertama gagal → mesin kedua menjawab; siaran langsung ditolak.
func TestCheck(t *testing.T) {
	dir := t.TempDir()
	fake := func(live string) string {
		p := filepath.Join(dir, "yt-"+live)
		script := `#!/bin/sh
case "$*" in *player_client*) ;; *) echo "ERROR: Sign in to confirm" >&2; exit 1 ;; esac
printf 'Me at the zoo\n19\n` + live + `\n'
`
		if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	info, err := Check(context.Background(), "https://youtu.be/jNQXAC9IVRw", Options{YtDlp: fake("False")})
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Me at the zoo" || info.Duration != 19 || info.Engine != "yt-dlp android_vr" {
		t.Fatalf("info %+v", info)
	}
	if _, err := Check(context.Background(), "https://youtu.be/jNQXAC9IVRw", Options{YtDlp: fake("True")}); err == nil || !strings.Contains(err.Error(), "live stream") {
		t.Fatalf("live stream accepted: %v", err)
	}
	if _, err := Check(context.Background(), "https://youtu.be/jNQXAC9IVRw", Options{}); err != errNoEngine {
		t.Fatalf("no engine: %v", err)
	}
}

// Berkas yang lolos dari mesin tapi rusak (Verify gagal) dibuang, mesin
// berikutnya dicoba, dan tidak ada sisa di folder hasil.
func TestVerifyRejectsBrokenFile(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake")
	script := `#!/bin/sh
out=""; client="default"
while [ $# -gt 0 ]; do
  case "$1" in -o) out="$2"; shift ;; --extractor-args) client="$2"; shift ;; esac
  shift
done
echo "$client" > "$out"
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	p, err := Download(context.Background(), "https://youtu.be/jNQXAC9IVRw", Options{
		Dir: out, YtDlp: fake,
		Verify: func(p string) error {
			b, _ := os.ReadFile(p)
			if strings.TrimSpace(string(b)) == "default" {
				return os.ErrInvalid // "rusak"
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); !strings.Contains(string(b), "android_vr") {
		t.Fatalf("kept the broken file: %q", b)
	}
	if left, _ := filepath.Glob(filepath.Join(out, ".dl-*")); len(left) != 0 {
		t.Fatalf("leftovers: %v", left)
	}
}
