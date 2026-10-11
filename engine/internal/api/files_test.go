package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func soon() time.Time { return time.Now().Add(2 * time.Second) }

// Inti fiturnya: berkas yang di-drop dikenali dari nama + ukuran, jadi video
// 3,84 GB tidak perlu disalin ke data/uploads sama sekali.
func TestLocateFindsTheFileByNameAndSize(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "Videos", "2026", "podcast.mp4")
	write(t, want, 1234)
	write(t, filepath.Join(root, "Videos", "lain.mp4"), 1234)

	got := findFile([]string{root}, "podcast.mp4", 1234, soon())

	if len(got) != 1 || got[0] != want {
		t.Fatalf("hasil = %v, mau [%s]", got, want)
	}
}

// Nama sama tapi ukuran beda = berkas lain. Kalau ini lolos, engine memproses
// video yang salah tanpa ada yang sadar.
func TestLocateRejectsADifferentSize(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "podcast.mp4"), 999)

	if got := findFile([]string{root}, "podcast.mp4", 1234, soon()); len(got) != 0 {
		t.Fatalf("hasil = %v, mau kosong", got)
	}
}

// Dua kandidat = jawabannya tidak diketahui. Menebak salah satunya lebih buruk
// daripada menyuruh pengguna memilih sendiri.
func TestLocateStaysSilentWhenTwoFilesMatch(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a", "podcast.mp4"), 1234)
	write(t, filepath.Join(root, "b", "podcast.mp4"), 1234)

	if got := findFile([]string{root}, "podcast.mp4", 1234, soon()); len(got) < 2 {
		t.Fatalf("hasil = %v, mau dua kandidat", got)
	}
}

// Pencarian tidak boleh menggantung GUI: begitu anggaran waktu habis, ia
// berhenti walau belum ketemu.
func TestLocateStopsWhenTheBudgetIsSpent(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "deep", "podcast.mp4"), 1234)

	if got := findFile([]string{root}, "podcast.mp4", 1234, time.Now().Add(-time.Second)); len(got) != 0 {
		t.Fatalf("hasil = %v, mau kosong (waktu sudah lewat)", got)
	}
}

// Folder besar yang tak pernah berisi video pengguna dilewati.
func TestLocateSkipsNoiseFolders(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "node_modules", "podcast.mp4"), 1234)

	if got := findFile([]string{root}, "podcast.mp4", 1234, soon()); len(got) != 0 {
		t.Fatalf("hasil = %v, mau kosong", got)
	}
}

// Endpoint locate menjawab 404 saat tidak yakin — itu isyarat bagi GUI untuk
// kembali ke unggahan biasa.
func TestLocateEndpointAnswers404WhenUnsure(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest("POST", "/api/locate", strings.NewReader(`{"name":"tidak-ada-12345.mp4","size":7}`))
	rec := httptest.NewRecorder()

	s.locate(rec, req)

	if rec.Code != 404 {
		t.Fatalf("kode = %d, mau 404", rec.Code)
	}
}

func TestLocateRejectsAnEmptyName(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest("POST", "/api/locate", strings.NewReader(`{"name":"","size":7}`))
	rec := httptest.NewRecorder()

	s.locate(rec, req)

	if rec.Code != 400 {
		t.Fatalf("kode = %d, mau 400", rec.Code)
	}
}

// browse: folder dulu, lalu berkas, masing-masing menurut abjad — urutan yang
// sama dengan file manager mana pun.
func TestBrowseListsFoldersFirst(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "zeta.mp4"), 1)
	write(t, filepath.Join(root, "Alpha.txt"), 1)
	write(t, filepath.Join(root, ".tersembunyi"), 1)
	if err := os.MkdirAll(filepath.Join(root, "Video"), 0o755); err != nil {
		t.Fatal(err)
	}

	body := browseJSON(t, root)

	var names []string
	for _, e := range body.Entries {
		names = append(names, e.Name)
	}
	want := []string{"Video", "Alpha.txt", "zeta.mp4"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("urutan = %v, mau %v", names, want)
	}
	for _, e := range body.Entries {
		if e.Name == "zeta.mp4" && !e.Video {
			t.Error("zeta.mp4 tidak ditandai sebagai video")
		}
	}
}

// Berkas tersembunyi tidak ditampilkan: pengguna tidak menyimpan videonya di
// ~/.cache, dan daftarnya jadi jauh lebih pendek.
func TestBrowseHidesDotFiles(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".rahasia.mp4"), 1)

	if body := browseJSON(t, root); len(body.Entries) != 0 {
		t.Errorf("entri = %v, mau kosong", body.Entries)
	}
}

// Folder yang ADA tapi tidak boleh dibaca tetap dijawab 400 dengan pesan,
// bukan 500 atau panik.
//
// Bedakan dengan folder yang TIDAK ADA: sejak pemilih berkas boleh dibuka dari
// letak sebuah program, path yang hilang justru naik ke induknya yang masih ada
// (lihat TestBrowseClimbsToAFolderThatStillExists). Yang tidak bisa diselamatkan
// hanyalah folder yang memang menolak dibuka.
func TestBrowseReportsAFolderItCannotOpen(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root menembus izin berkas, jadi kasus ini tidak bisa diuji")
	}
	terkunci := filepath.Join(t.TempDir(), "terkunci")
	if err := os.Mkdir(terkunci, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(terkunci, 0o755) })

	s := &Server{}
	req := httptest.NewRequest("GET", "/api/browse?dir="+url.QueryEscape(terkunci), nil)
	rec := httptest.NewRecorder()

	s.browse(rec, req)

	if rec.Code != 400 {
		t.Fatalf("kode = %d, mau 400", rec.Code)
	}
}

type browseBody struct {
	Dir     string  `json:"dir"`
	Parent  string  `json:"parent"`
	Entries []entry `json:"entries"`
	Places  []place `json:"places"`
}

func browseJSON(t *testing.T, dir string) browseBody {
	t.Helper()
	s := &Server{}
	req := httptest.NewRequest("GET", "/api/browse?dir="+url.QueryEscape(dir), nil)
	rec := httptest.NewRecorder()
	s.browse(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("kode = %d: %s", rec.Code, rec.Body.String())
	}
	var body browseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

// Pemilih berkas sering dibuka dari letak sebuah PROGRAM, bukan folder —
// "ganti berkasnya" untuk ffprobe.exe mengirim path berkas itu. Dulu itu
// berakhir sebagai "cannot open the folder: readdir …ffprobe.exe".
func TestBrowseOpensTheFolderOfAFile(t *testing.T) {
	dir := t.TempDir()
	berkas := filepath.Join(dir, "ffprobe.exe")
	write(t, berkas, 1)

	body := browseJSON(t, berkas)

	if body.Dir != dir {
		t.Errorf("dir = %q, mau %q", body.Dir, dir)
	}
}

// Path yang sudah tidak ada (program dipindah, folder dihapus) naik ke induknya
// yang masih ada — bukan menolak dengan galat.
func TestBrowseClimbsToAFolderThatStillExists(t *testing.T) {
	dir := t.TempDir()
	hilang := filepath.Join(dir, "sudah", "tidak", "ada.exe")

	body := browseJSON(t, hilang)

	if body.Dir != dir {
		t.Errorf("dir = %q, mau %q", body.Dir, dir)
	}
}

// Path gaya Windows harus bisa ditempel apa adanya saat engine jalan di WSL:
// itu bentuk yang tersalin dari Explorer, dan tanpa terjemahan ia berakhir
// sebagai "<folder kerja>/C:\Users\…" — folder yang tidak pernah ada, jadi
// pemilih berkas diam-diam kembali ke folder kerja.
func TestHostPathTranslatesWindowsPaths(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("terjemahan ini hanya berlaku di Linux/WSL")
	}
	cases := map[string]string{
		`C:\Users\PHP-02\Videos`:    "/mnt/c/Users/PHP-02/Videos",
		`c:/Users/PHP-02/a b.mp4`:   "/mnt/c/Users/PHP-02/a b.mp4",
		`D:\rekaman.mp4`:            "/mnt/d/rekaman.mp4",
		"  C:\\Users\\PHP-02  ":     "/mnt/c/Users/PHP-02",
		"/home/php-02/video.mp4":    "/home/php-02/video.mp4",
		"/mnt/c/Users/PHP-02/a.mp4": "/mnt/c/Users/PHP-02/a.mp4",
		"":                          "",
		// "Copy as path" di Explorer SELALU membungkus dengan kutip, dan itu
		// bentuk yang paling sering ditempel orang. Dilaporkan dari lapangan
		// sebagai "error, videonya tidak ada".
		`"C:\Users\PHP-02\Videos\a.mp4"`: "/mnt/c/Users/PHP-02/Videos/a.mp4",
		// Berkas di sisi Linux yang dibuka lewat Explorer.
		`\\wsl.localhost\Ubuntu\home\php-02\a.mp4`: "/home/php-02/a.mp4",
		`\\wsl$\Ubuntu\home\php-02\a.mp4`:          "/home/php-02/a.mp4",
		// Share jaringan sungguhan BUKAN berkas mesin ini — jangan ditebak.
		`\\server\bagi\a.mp4`: `\\server\bagi\a.mp4`,
	}
	for in, want := range cases {
		if got := hostPath(in); got != want {
			t.Errorf("hostPath(%q) = %q, mau %q", in, got, want)
		}
	}
}
