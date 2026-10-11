package setup

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemgum/clipper/engine/internal/config"
)

// sum menghitung sidik jari isi yang akan disajikan server uji.
func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// resumable menulis .part beserta penanda asalnya — bentuk yang sah untuk
// dilanjutkan. Penandanya tidak boleh dilewatkan: tanpa itu, potongannya memang
// SENGAJA dibuang (lihat TestAPartFromAnotherMirror…).
func resumable(t *testing.T, dest, url string, part []byte) {
	t.Helper()
	if err := os.WriteFile(dest+".part", part, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest+".part.from", []byte(url), 0o644); err != nil {
		t.Fatal(err)
	}
}

// makeZip menulis arsip zip berisi nama→isi.
func makeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

// Arsip rilis membungkus binernya dalam satu folder ("Release/", "bin/").
// Struktur itu tidak berarti apa-apa bagi engine — yang penting semua berkas
// mendarat di satu folder, sebab di situlah engine mencari pustaka pendamping.
func TestZipIsFlattenedIntoOneFolder(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.zip")
	makeZip(t, archive, map[string]string{
		"Release/whisper-cli.exe": "bin",
		"Release/whisper.dll":     "lib",
	})
	dest := t.TempDir()

	n, err := (&recipe{kind: "zip"}).extract(archive, dest)
	if err != nil {
		t.Fatal(err)
	}

	if n != 2 {
		t.Errorf("berkas terambil = %d, mau 2", n)
	}
	for _, want := range []string{"whisper-cli.exe", "whisper.dll"} {
		if _, err := os.Stat(filepath.Join(dest, want)); err != nil {
			t.Errorf("%s tidak ada di folder tujuan", want)
		}
	}
}

// Arsip ffmpeg berisi ratusan berkas dokumentasi; hanya dua yang dipakai.
func TestZipTakesOnlyTheWantedFiles(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.zip")
	makeZip(t, archive, map[string]string{
		"build/bin/ffmpeg.exe":   "x",
		"build/bin/ffprobe.exe":  "x",
		"build/bin/ffplay.exe":   "x",
		"build/doc/ffmpeg.html":  "x",
		"build/doc/general.html": "x",
	})
	dest := t.TempDir()

	n, err := (&recipe{kind: "zip", want: []string{"ffmpeg.exe", "ffprobe.exe"}}).extract(archive, dest)
	if err != nil {
		t.Fatal(err)
	}

	if n != 2 {
		t.Fatalf("berkas terambil = %d, mau 2", n)
	}
	if _, err := os.Stat(filepath.Join(dest, "ffplay.exe")); err == nil {
		t.Error("ffplay ikut terambil padahal tidak diminta")
	}
}

// Rilis Linux whisper.cpp memakai rantai symlink
// (libwhisper.so → libwhisper.so.1 → libwhisper.so.1.9.1). Tanpa dukungan ini,
// binernya terpasang tapi tidak bisa dijalankan sama sekali.
func TestTarGzKeepsSymlinks(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := "biner"
	if err := tw.WriteHeader(&tar.Header{
		Name: "whisper-bin/whisper-cli", Mode: 0o755,
		Size: int64(len(body)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{
		Name: "whisper-bin/libwhisper.so", Linkname: "libwhisper.so.1",
		Typeflag: tar.TypeSymlink, Mode: 0o777,
	}); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()
	f.Close()
	dest := t.TempDir()

	if _, err := (&recipe{kind: "tar.gz"}).extract(archive, dest); err != nil {
		t.Fatal(err)
	}

	link, err := os.Readlink(filepath.Join(dest, "libwhisper.so"))
	if err != nil {
		t.Fatalf("symlink tidak dibuat: %v", err)
	}
	if link != "libwhisper.so.1" {
		t.Errorf("tujuan symlink = %q, mau %q", link, "libwhisper.so.1")
	}
	// Biner harus bisa dieksekusi; rilis Windows tidak membawa bit itu, jadi
	// engine yang memasangnya.
	info, err := os.Stat(filepath.Join(dest, "whisper-cli"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o100 == 0 {
		t.Error("whisper-cli tidak bisa dieksekusi")
	}
}

// Unduhan yang DITOLAK server tidak meninggalkan apa pun — termasuk .part,
// sebab tidak ada satu byte pun yang sah untuk dilanjutkan.
//
// Bedakan dengan unduhan yang TERPUTUS di tengah: di situ .part sengaja
// dipertahankan supaya bisa dilanjutkan (lihat TestDownloadResumes…).
func TestARejectedDownloadLeavesNothingBehind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "model.bin")

	err := download(context.Background(), source{srv.URL, sum([]byte("apa pun"))}, dest, nil)

	if err == nil {
		t.Fatal("unduhan gagal tidak dilaporkan")
	}
	if _, err := os.Stat(dest); err == nil {
		t.Error("berkas tujuan tetap dibuat padahal unduhan gagal")
	}
	if _, err := os.Stat(dest + ".part"); err == nil {
		t.Error("berkas .part ditinggalkan")
	}
}

// Progres dilaporkan, dan totalnya ikut supaya GUI bisa menggambar bilah.
func TestDownloadReportsProgress(t *testing.T) {
	payload := strings.Repeat("x", 4096)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "4096")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "model.bin")

	var biggest Progress
	seen := 0
	if err := download(context.Background(), source{srv.URL, sum([]byte(payload))}, dest, func(p Progress) {
		if p.Bytes > biggest.Bytes {
			biggest = p
		}
		seen++
	}); err != nil {
		t.Fatal(err)
	}

	if seen == 0 {
		t.Fatal("tidak ada laporan progres")
	}
	if biggest.Total != 4096 || biggest.Bytes != 4096 {
		t.Errorf("progres terbesar = %d/%d, mau 4096/4096", biggest.Bytes, biggest.Total)
	}
	raw, err := os.ReadFile(dest)
	if err != nil || len(raw) != 4096 {
		t.Errorf("isi berkas = %d byte (err %v), mau 4096", len(raw), err)
	}
}

// Status membaca folder dari Layout, bukan dari sekitar biner.
func TestStatusFindsAnInstalledModel(t *testing.T) {
	l := config.Layout{ModelsDir: t.TempDir(), ToolsDir: t.TempDir()}
	if err := os.WriteFile(ModelPath(l, "small"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var small Component
	for _, c := range Status(l) {
		if c.ID == ModelID("small") {
			small = c
		}
	}

	if !small.Installed {
		t.Fatal("model small tidak terdeteksi")
	}
	if small.Path != ModelPath(l, "small") {
		t.Errorf("path = %q, mau %q", small.Path, ModelPath(l, "small"))
	}
}

// Komponen wajib yang belum ada harus bisa disebut namanya — itu yang dipakai
// untuk menolak job dengan pesan yang bisa ditindaklanjuti.
func TestMissingNamesTheRequiredComponents(t *testing.T) {
	l := config.Layout{ModelsDir: t.TempDir(), ToolsDir: t.TempDir()}

	missing := Missing(Status(l))

	found := false
	for _, m := range missing {
		if m == "whisper.cpp" {
			found = true
		}
	}
	if !found {
		t.Errorf("kurang = %v, mau memuat whisper.cpp", missing)
	}
}

func TestInstallRejectsAnUnknownComponent(t *testing.T) {
	l := config.Layout{ModelsDir: t.TempDir(), ToolsDir: t.TempDir()}

	if err := Install(context.Background(), l, "sesuatu", nil); err == nil {
		t.Fatal("komponen tak dikenal tidak ditolak")
	}
	if err := Install(context.Background(), l, "model:tidak-ada", nil); err == nil {
		t.Fatal("model tak dikenal tidak ditolak")
	}
}

// Menghapus model membebaskan ruang; menghapus yang tidak ada bukan error.
func TestRemoveModel(t *testing.T) {
	l := config.Layout{ModelsDir: t.TempDir()}
	if err := os.WriteFile(ModelPath(l, "tiny"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RemoveModel(l, "tiny"); err != nil {
		t.Fatal(err)
	}
	if HasModel(l, "tiny") {
		t.Error("model masih ada setelah dihapus")
	}
	if err := RemoveModel(l, "tiny"); err != nil {
		t.Errorf("menghapus yang sudah tidak ada dilaporkan sebagai galat: %v", err)
	}
	if err := RemoveModel(l, "bukan-model"); err == nil {
		t.Error("nama model asing tidak ditolak")
	}
}

// Unduhan yang terputus dilanjutkan, bukan diulang. Ini yang membedakan gagal
// di 100 MB dari "mengulang 111 MB dari nol" — dan di sambungan yang
// putus-nyambung, itu selisih antara bisa dan tidak bisa memasang sama sekali.
func TestDownloadResumesFromWhatIsAlreadyThere(t *testing.T) {
	full := []byte(strings.Repeat("abcdefghij", 1000)) // 10.000 byte
	var gotRange string
	var sent int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		start := 0
		if gotRange != "" {
			_, _ = fmt.Sscanf(gotRange, "bytes=%d-", &start)
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(full)-1, len(full)))
			w.WriteHeader(http.StatusPartialContent)
		}
		sent = len(full) - start
		_, _ = w.Write(full[start:])
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "berkas.bin")
	// Separuh sudah terunduh dari percobaan sebelumnya, dari alamat yang sama.
	resumable(t, dest, srv.URL, full[:4000])

	if err := download(context.Background(), source{srv.URL, sum(full)}, dest, nil); err != nil {
		t.Fatal(err)
	}

	if gotRange != "bytes=4000-" {
		t.Errorf("header Range = %q, mau %q", gotRange, "bytes=4000-")
	}
	if sent != 6000 {
		t.Errorf("server mengirim %d byte, mau 6000 (sisanya saja)", sent)
	}
	if got, err := os.ReadFile(dest); err != nil || !bytes.Equal(got, full) {
		t.Errorf("isi berkas tidak utuh (%d byte, err %v)", len(got), err)
	}
}

// Server yang MENGABAIKAN Range (menjawab 200) mengirim dari awal. Kalau itu
// ditambahkan ke berkas separuh, hasilnya berkas rusak yang ukurannya justru
// terlihat wajar — kerusakan paling buruk, sebab tidak kelihatan.
func TestDownloadStartsOverWhenTheServerIgnoresRange(t *testing.T) {
	full := []byte(strings.Repeat("x", 5000))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(full)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "berkas.bin")
	resumable(t, dest, srv.URL, []byte(strings.Repeat("y", 2000)))

	if err := download(context.Background(), source{srv.URL, sum(full)}, dest, nil); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, full) {
		t.Errorf("berkas rusak: %d byte, mau %d byte berisi 'x'", len(got), len(full))
	}
}

// Berkas yang sampai dengan isi berbeda dari yang dipaku TIDAK dipakai — dan
// tidak disimpan untuk dilanjutkan. Ini pagar terakhir sebelum sebuah berkas
// bernama seperti biner yang siap dijalankan: proxy yang membongkar TLS, cermin
// yang diretas, dan rilis yang dibajak semuanya berhenti di sini.
func TestADownloadThatFailsItsChecksumIsThrownAway(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("biner orang lain"))
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "whisper-cli.exe")

	err := download(context.Background(), source{srv.URL, sum([]byte("biner yang benar"))}, dest, nil)

	if err == nil {
		t.Fatal("berkas dengan sidik jari salah diterima")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("pesan galat = %q, mau menyebut checksum", err)
	}
	if _, err := os.Stat(dest); err == nil {
		t.Error("berkas tujuan tetap dibuat padahal sidik jarinya salah")
	}
	if _, err := os.Stat(dest + ".part"); err == nil {
		t.Error(".part ditinggalkan — percobaan berikutnya akan mewarisi isi yang rusak")
	}
}

// Potongan dari cermin LAIN tidak boleh dilanjutkan. Cermin yang berbeda
// menyajikan build yang berbeda; menyambung separuh yang satu dengan separuh
// yang lain menghasilkan berkas yang ukurannya wajar tapi isinya rusak.
//
// Sidik jari memang akan menangkapnya juga, tapi baru setelah seluruh sisanya
// diunduh. Ini yang membuat perbaikannya berhenti di byte pertama.
func TestAPartFromAnotherMirrorIsNotResumed(t *testing.T) {
	full := []byte(strings.Repeat("benar", 1000)) // 5.000 byte
	var gotRange string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		_, _ = w.Write(full)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "berkas.bin")
	// Separuh berkas dari cermin yang berbeda.
	resumable(t, dest, "https://cermin-lain.example/berkas.bin", []byte(strings.Repeat("salah", 400)))

	if err := download(context.Background(), source{srv.URL, sum(full)}, dest, nil); err != nil {
		t.Fatal(err)
	}

	if gotRange != "" {
		t.Errorf("header Range = %q, mau kosong — potongan cermin lain seharusnya dibuang", gotRange)
	}
	if got, _ := os.ReadFile(dest); !bytes.Equal(got, full) {
		t.Errorf("berkas rusak: %d byte, mau %d byte", len(got), len(full))
	}
}

// Sumber tanpa sidik jari ditolak sebelum satu byte pun diunduh. Penjaga ini ada
// supaya menambah cermin baru dan lupa memakukan sidik jarinya gagal seketika,
// bukan diam-diam melewati verifikasi.
func TestASourceWithoutAChecksumIsRefused(t *testing.T) {
	dilarang := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dilarang = false
		_, _ = w.Write([]byte("isi"))
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "berkas.bin")

	err := downloadAny(context.Background(), []source{{srv.URL, ""}}, dest, nil)

	if err == nil {
		t.Fatal("sumber tanpa sidik jari tidak ditolak")
	}
	if !dilarang {
		t.Error("servernya tetap dihubungi padahal sumbernya tidak sah")
	}
}

// Cermin pertama mati, yang kedua dipakai. Tanpa ini, satu server yang tidak
// terjangkau dari jaringan pengguna berarti komponen wajib tidak bisa dipasang
// sama sekali — persis yang terjadi dengan gyan.dev.
func TestDownloadFallsBackToTheNextMirror(t *testing.T) {
	mati := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer mati.Close()
	hidup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("isi"))
	}))
	defer hidup.Close()

	dest := filepath.Join(t.TempDir(), "berkas.bin")

	srcs := []source{{mati.URL, sum([]byte("isi"))}, {hidup.URL, sum([]byte("isi"))}}
	if err := downloadAny(context.Background(), srcs, dest, nil); err != nil {
		t.Fatal(err)
	}

	if got, _ := os.ReadFile(dest); string(got) != "isi" {
		t.Errorf("isi = %q, mau %q", got, "isi")
	}
}

// Berkas yang ADA tapi tidak bisa dijalankan harus dilaporkan TIDAK terpasang,
// bukan hijau — itu seluruh alasan checkRuns ada (notes/31).
func TestCheckRunsRejectsBrokenBinary(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "ffmpeg")
	if err := os.WriteFile(broken, []byte("not a program"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := checkRuns(Component{Name: "ffmpeg", Path: broken, Installed: true})
	if c.Installed {
		t.Fatal("berkas rusak dilaporkan terpasang")
	}
	if !strings.Contains(c.Detail, broken) {
		t.Fatalf("Detail tidak menyebut pathnya: %q", c.Detail)
	}
}

// Tiap server LLM yang ditawarkan harus benar-benar bisa dicari pengguna:
// tanpa tautan, barisnya cuma nama yang tidak bisa ditindaklanjuti.
func TestLLMServersAreActionable(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range LLMServers {
		if s.ID == "" || s.Name == "" || s.URL == "" || s.Hint == "" {
			t.Errorf("baris tidak lengkap: %+v", s)
		}
		if seen[s.ID] {
			t.Errorf("ID ganda: %q", s.ID)
		}
		seen[s.ID] = true
	}
	// Semuanya muncul di halaman Requirements, tidak ada yang tercecer.
	comps := Status(config.Layout{ModelsDir: t.TempDir(), ToolsDir: t.TempDir()})
	for _, s := range LLMServers {
		found := false
		for _, c := range comps {
			if c.ID == "llm:"+s.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("%s tidak ada di Status()", s.Name)
		}
	}
	// Dan tidak satu pun WAJIB: mode heuristik jalan tanpa LLM apa pun, jadi
	// titik merah "ada yang kurang" tidak boleh menyala karenanya.
	for _, c := range comps {
		if strings.HasPrefix(c.ID, "llm:") && c.Required {
			t.Errorf("%s ditandai wajib", c.Name)
		}
	}
}

// Menghapus model harus ikut membuang SISA UNDUHANNYA.
//
// Unduhan yang terputus meninggalkan `.part` sebesar apa yang sempat turun —
// sampai 2,9 GB untuk large-v3. Sebelum ini tombol Remove hanya menyentuh
// berkas .bin, jadi barisnya berubah jadi "missing" sementara ruang disknya
// tidak kembali sama sekali.
func TestRemoveModelAlsoRemovesPartialDownload(t *testing.T) {
	l := config.Layout{ModelsDir: t.TempDir(), ToolsDir: t.TempDir()}
	p := ModelPath(l, "small")
	for _, f := range []string{p, p + ".part", p + ".part.from"} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := RemoveModel(l, "small"); err != nil {
		t.Fatalf("RemoveModel: %v", err)
	}

	for _, f := range []string{p, p + ".part", p + ".part.from"} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("%s masih ada sesudah dihapus", filepath.Base(f))
		}
	}
	// Menghapus dua kali bukan galat: barisnya bisa diklik lagi.
	if err := RemoveModel(l, "small"); err != nil {
		t.Errorf("hapus kedua kali: %v", err)
	}
}
