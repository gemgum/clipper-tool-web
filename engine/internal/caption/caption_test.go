package caption

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/types"
)

// spoken membungkus satu kalimat jadi transkrip sesegmen — bentuk yang dipakai
// Transcriber sungguhan, tanpa perlu whisper.
func spoken(text string) types.Transcript {
	return types.Transcript{
		Language: "id",
		Segments: []types.TranscriptSegment{{Start: 0, End: 3, Text: text}},
	}
}

// reply membuat Completer palsu yang selalu membalas teks yang sama.
func reply(s string) func(ctx context.Context, system, user string, schema any) (string, error) {
	return func(context.Context, string, string, any) (string, error) { return s, nil }
}

// Pagar isi: angka yang tidak pernah diucapkan adalah satu-satunya kesalahan
// yang bisa dibuktikan mesin, jadi ia HARUS dilaporkan — caption dengan "3 juta
// penonton" yang tidak ada di videonya itu klaim palsu, bukan gaya bahasa.
func TestGenerateFlagsNumbersNotSpoken(t *testing.T) {
	raw := `{"captions":[{"hook":"Banjir 3 meter","body":"Warga bertahan sejak 2 hari lalu.","tags":["#Jakarta"," banjir ","Jakarta"]}]}`
	got, err := Generate(context.Background(), reply(raw),
		"Air naik sampai 2 meter.\nWarga bertahan di lantai atas.", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("mau 1 caption, dapat %d", len(got))
	}
	v := got[0]
	if len(v.Violations) != 1 || !strings.Contains(v.Violations[0], "3") {
		t.Errorf("angka 3 tidak dilaporkan: %v", v.Violations)
	}
	// "2" diucapkan (2 meter), jadi ia TIDAK boleh ikut dilaporkan — pagar yang
	// berteriak pada angka yang benar akan diabaikan sepenuhnya.
	for _, w := range v.Violations {
		if strings.Contains(w, "number 2 ") {
			t.Errorf("angka 2 diucapkan tapi tetap dilaporkan: %v", v.Violations)
		}
	}
	if want := []string{"Jakarta", "banjir"}; strings.Join(v.Tags, ",") != strings.Join(want, ",") {
		t.Errorf("tagar tidak dibersihkan: %v", v.Tags)
	}
}

// Balasan yang dibungkus pagar kode atau berupa larik telanjang tetap dibaca:
// mesin yang tidak memaksakan skema membalas begitu (notes/35), dan menolaknya
// berarti membuang pekerjaan yang isinya sudah benar.
func TestGenerateReadsFencedAndBareReplies(t *testing.T) {
	for name, raw := range map[string]string{
		"fenced": "```json\n{\"captions\":[{\"hook\":\"Halo\",\"body\":\"Isi.\"}]}\n```",
		"bare":   `[{"hook":"Halo","body":"Isi."}]`,
	} {
		got, err := Generate(context.Background(), reply(raw), "Halo semua.", Options{})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(got) != 1 || got[0].Hook != "Halo" {
			t.Errorf("%s: dapat %+v", name, got)
		}
	}
}

func TestGenerateRejectsEmptyTranscript(t *testing.T) {
	if _, err := Generate(context.Background(), reply(`{"captions":[]}`), "  ", Options{}); err == nil {
		t.Error("transkrip kosong harus jadi galat, bukan caption karangan")
	}
}

// Bulk: satu berkas gagal tidak boleh membuang sisa antrian, dan kegagalannya
// harus tercatat pada berkas ITU — bukan jadi satu galat job yang tidak
// menyebut video mana.
func TestRunKeepsGoingAfterOneVideoFails(t *testing.T) {
	dir := t.TempDir()
	deps := Deps{
		Complete: reply(`{"captions":[{"hook":"Halo","body":"Isi.","tags":["tag"]}]}`),
		Engine:   "test",
		Transcribe: func(_ context.Context, video string, maxSec float64) (Speech, error) {
			if strings.Contains(video, "sunyi") {
				return Speech{}, errors.New("this video has no sound track")
			}
			return Speech{Transcript: spoken("Halo semua."), VideoSec: 900, UsedSec: maxSec}, nil
		},
	}
	res, err := Run(context.Background(),
		Options{Videos: []string{"/v/a.mp4", "/v/sunyi.mp4", "/v/b.mp4"}, OutDir: dir},
		deps, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Done() != 2 || len(res.Files) != 3 {
		t.Fatalf("mau 2 dari 3 berhasil, dapat %d dari %d", res.Done(), len(res.Files))
	}
	if res.Files[1].Error == "" {
		t.Error("video gagal tidak membawa alasannya")
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		body := string(raw)
		// Transkrip WAJIB ikut: ia satu-satunya cara memeriksa caption karangan
		// mesin tanpa membuka videonya lagi.
		if !strings.Contains(body, "Halo semua.") {
			t.Errorf("%s tidak memuat transkripnya:\n%s", name, body)
		}
		if !strings.Contains(body, "#tag") {
			t.Errorf("%s tidak memuat tagarnya:\n%s", name, body)
		}
		// Bagian yang dibaca harus disebut — caption dari 5 menit pertama video
		// 15 menit tidak boleh terbaca seolah mewakili seluruhnya.
		if !strings.Contains(body, "of 15:00") {
			t.Errorf("%s tidak menyebut bagian yang dibaca:\n%s", name, body)
		}
	}
}

// Dua video bernama sama dari folder berbeda tidak boleh saling menimpa dalam
// satu job — hasil yang hilang diam-diam lebih buruk daripada nama berkas yang
// jelek.
func TestRunKeepsBothFilesWithTheSameName(t *testing.T) {
	dir := t.TempDir()
	deps := Deps{
		Complete: reply(`{"captions":[{"hook":"Halo","body":"Isi."}]}`),
		Engine:   "test",
		Transcribe: func(context.Context, string, float64) (Speech, error) {
			return Speech{Transcript: spoken("Halo semua."), VideoSec: 60, UsedSec: 60}, nil
		},
	}
	res, err := Run(context.Background(),
		Options{Videos: []string{"/satu/video.mp4", "/dua/video.mp4"}, OutDir: dir}, deps, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Files[0].TXT == res.Files[1].TXT {
		t.Fatalf("kedua video menulis ke berkas yang sama: %s", res.Files[0].TXT)
	}
	for _, f := range res.Files {
		if _, err := os.Stat(f.TXT); err != nil {
			t.Errorf("%s tidak ada: %v", f.TXT, err)
		}
	}
}

// Bawaannya: .txt lahir DI SEBELAH videonya. Folder aplikasi adalah tempat
// terakhir yang akan dibuka orang saat hendak memposting.
func TestRunWritesNextToTheVideo(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "rekaman.mp4")
	if err := os.WriteFile(video, []byte("bukan video sungguhan"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := Deps{
		Complete: reply(`{"captions":[{"hook":"Halo","body":"Isi."}]}`),
		Engine:   "test",
		Transcribe: func(context.Context, string, float64) (Speech, error) {
			return Speech{Transcript: spoken("Halo semua."), VideoSec: 60, UsedSec: 60}, nil
		},
	}
	res, err := Run(context.Background(), Options{Videos: []string{video}}, deps, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "rekaman.txt"); res.Files[0].TXT != want {
		t.Fatalf("ditulis ke %s, mau %s", res.Files[0].TXT, want)
	}

	// Dijalankan ulang: berkas yang SAMA diperbarui, tidak menumpuk salinan
	// bernomor di folder pengguna.
	res2, err := Run(context.Background(), Options{Videos: []string{video}}, deps, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Files[0].TXT != res.Files[0].TXT {
		t.Errorf("jalan kedua menulis ke %s, mau menimpa %s", res2.Files[0].TXT, res.Files[0].TXT)
	}
}

// Berkas ucapan milik klip ("<klip>.txt", ditulis pipeline) memakai nama yang
// sama persis. Ia BUKAN milik kita, jadi ia tidak boleh tertimpa — itu bahan
// caption yang justru dipakai orang.
func TestRunNeverOverwritesAFileThatIsNotOurs(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "clip_01.mp4")
	if err := os.WriteFile(video, []byte("bukan video sungguhan"), 0o644); err != nil {
		t.Fatal(err)
	}
	speech := filepath.Join(dir, "clip_01.txt")
	if err := os.WriteFile(speech, []byte("Ucapan klip ini.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := Deps{
		Complete: reply(`{"captions":[{"hook":"Halo","body":"Isi."}]}`),
		Engine:   "test",
		Transcribe: func(context.Context, string, float64) (Speech, error) {
			return Speech{Transcript: spoken("Halo semua."), VideoSec: 60, UsedSec: 60}, nil
		},
	}
	res, err := Run(context.Background(), Options{Videos: []string{video}}, deps, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "clip_01.caption.txt"); res.Files[0].TXT != want {
		t.Fatalf("ditulis ke %s, mau %s", res.Files[0].TXT, want)
	}
	raw, err := os.ReadFile(speech)
	if err != nil || string(raw) != "Ucapan klip ini.\n" {
		t.Errorf("berkas ucapan klip ikut berubah: %q (%v)", string(raw), err)
	}
}

// Gaya kosong / tak dikenal diisi dari urutan; gaya yang sah dibiarkan.
func TestWithStyles(t *testing.T) {
	vs := withStyles([]Variant{{Style: "question"}, {}, {Style: "aneh"}, {}})
	want := []string{"question", "question", "short", "direct"}
	for i, v := range vs {
		if v.Style != want[i] {
			t.Errorf("varian %d gaya %q, mau %q", i, v.Style, want[i])
		}
	}
}
