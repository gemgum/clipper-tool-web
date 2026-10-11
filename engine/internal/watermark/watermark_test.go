package watermark

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/ffmpeg"
)

// Berkas sumber TIDAK PERNAH ditimpa. Masukan halaman ini video jadi milik
// pengguna — seringkali satu-satunya salinan — dan watermark yang sudah terbakar
// tidak bisa dilepas lagi.
func TestOutputPathNeverOverwritesSource(t *testing.T) {
	in := filepath.Join("videos", "klip.mp4")
	got := outputPath(in, "")
	if got == in {
		t.Fatalf("hasil menimpa sumbernya: %s", got)
	}
	if want := filepath.Join("videos", "klip_watermarked.mp4"); got != want {
		t.Fatalf("outputPath = %s, mau %s", got, want)
	}
	// -out mengumpulkan semuanya ke satu folder.
	if got := outputPath(in, "/keluaran"); got != filepath.Join("/keluaran", "klip_watermarked.mp4") {
		t.Fatalf("folder tujuan diabaikan: %s", got)
	}
}

// Tanpa banner DAN tanpa teks, yang dihasilkan cuma salinan yang dikompresi
// ulang. Ditolak sebelum satu detik encode pun dijalankan.
func TestRunRefusesEmptyWatermark(t *testing.T) {
	_, err := Run(context.Background(), Options{Videos: []string{"a.mp4"}}, nil, config.Paths{}, nil)
	if err == nil || !strings.Contains(err.Error(), "nothing to burn") {
		t.Fatalf("watermark kosong seharusnya ditolak, dapat: %v", err)
	}
}

// Sumber "llm" tidak berlaku di sini: tidak ada klip, jadi tidak ada judul yang
// dipilihkan LLM. Dinyatakan, bukan didiamkan lalu menghasilkan teks kosong.
func TestRunRefusesLLMHeadline(t *testing.T) {
	o := Options{Videos: []string{"a.mp4"}}
	o.Watermark.Image = "logo.png"
	o.Watermark.Headline.Source = config.HeadlineLLM
	if _, err := Run(context.Background(), o, nil, config.Paths{}, nil); err == nil ||
		!strings.Contains(err.Error(), "only exists for clips") {
		t.Fatalf("sumber llm seharusnya ditolak, dapat: %v", err)
	}
}

// Video 16:9 ditolak PER BERKAS, dan sisanya tetap dikerjakan: satu berkas yang
// nyasar ke daftar tidak boleh membuang sembilan belas yang sudah benar.
func TestRunRejectsWrongAspectButKeepsGoing(t *testing.T) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg tidak ada di mesin ini")
	}
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe tidak ada di mesin ini")
	}
	dir := t.TempDir()
	mk := func(name, size string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		out, err := exec.Command(bin, "-y", "-f", "lavfi", "-i", "color=c=blue:s="+size+":d=1",
			"-c:v", "libx264", "-pix_fmt", "yuv420p", p).CombinedOutput()
		if err != nil {
			t.Fatalf("menyiapkan %s: %v: %s", name, err, out)
		}
		return p
	}
	portrait := mk("tegak.mp4", "270x480")
	landscape := mk("mendatar.mp4", "480x270")

	o := Options{
		Videos:   []string{landscape, portrait},
		Subtitle: config.DefaultSubtitle(),
		OutDir:   filepath.Join(dir, "out"),
		Quality:  "draft",
	}
	o.Watermark = config.DefaultWatermark()
	o.Watermark.Headline.Text = "HALO"

	res, err := Run(context.Background(), o, ffmpeg.New(bin, probe),
		config.Paths{DataDir: dir}, nil)
	if err != nil {
		t.Fatalf("job berhenti seluruhnya: %v", err)
	}
	if len(res.Files) != 2 {
		t.Fatalf("mau 2 hasil, dapat %d", len(res.Files))
	}
	if !strings.Contains(res.Files[0].Error, "not 9:16") {
		t.Fatalf("video mendatar seharusnya ditolak: %#v", res.Files[0])
	}
	if res.Files[1].Error != "" {
		t.Fatalf("video tegak gagal: %s", res.Files[1].Error)
	}
	if st, err := os.Stat(res.Files[1].Output); err != nil || st.Size() == 0 {
		t.Fatalf("berkas hasil kosong atau tidak ada (err=%v)", err)
	}
}
