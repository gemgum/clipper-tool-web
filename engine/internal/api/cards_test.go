package api

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gemgum/clipper/engine/internal/config"
)

// server membuat Server yang hanya tahu folder datanya — cukup untuk sweep.
//
// Lewat layout, bukan paths: sejak kartu bisa diarahkan ke folder pilihan
// pengguna, letaknya ditentukan Layout.CardsRoot().
func server(dataRoot string) *Server {
	return &Server{layout: config.Layout{DataDir: filepath.Join(dataRoot, "data")}}
}

// makeCards menyiapkan folder kartu palsu dengan waktu ubah berurutan: indeks
// kecil = paling lama.
func makeCards(t *testing.T, root string, names []string) {
	t.Helper()
	dir := filepath.Join(root, "data", "cards")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	for i, name := range names {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		when := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
	}
}

func exists(t *testing.T, root, name string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(root, "data", "cards", name))
	return err == nil
}

// Kartu terlama dibuang, yang terbaru dipertahankan. Tanpa ini folder data
// tumbuh selamanya — 27 folder / 24 MB hanya dari sehari mencoba-coba.
func TestSweepKeepsTheNewestCards(t *testing.T) {
	root := t.TempDir()
	var names []string
	for i := 0; i < keepCards+12; i++ {
		names = append(names, fmt.Sprintf("card-%d", 1000+i))
	}
	makeCards(t, root, names)

	s := server(root)
	s.sweepCards()

	// 12 terlama harus hilang, 50 terbaru harus tetap ada.
	for i, name := range names {
		got := exists(t, root, name)
		want := i >= 12
		if got != want {
			t.Errorf("%s: ada=%v, mau ada=%v", name, got, want)
		}
	}
}

// Pratinjau menimpa dirinya sendiri dan tidak pernah menua, jadi ia tidak boleh
// ikut terhitung apalagi terhapus — kalau hilang, pratinjau berikutnya gagal
// ditampilkan tanpa alasan yang jelas bagi pengguna.
func TestSweepNeverTouchesThePreview(t *testing.T) {
	root := t.TempDir()
	names := []string{previewID}
	for i := 0; i < keepCards+5; i++ {
		names = append(names, fmt.Sprintf("card-%d", 2000+i))
	}
	makeCards(t, root, names)

	s := server(root)
	s.sweepCards()

	if !exists(t, root, previewID) {
		t.Error("folder pratinjau ikut terhapus")
	}
}

// Apa pun yang bukan folder kartu tidak boleh disentuh. Fungsi ini menghapus
// folder secara rekursif; salah sasaran di sini berarti data pengguna hilang.
func TestSweepIgnoresAnythingThatIsNotACard(t *testing.T) {
	root := t.TempDir()
	var names []string
	for i := 0; i < keepCards+8; i++ {
		names = append(names, fmt.Sprintf("card-%d", 3000+i))
	}
	strangers := []string{"notes", "card-", "kartu-1", "card-abc", ".keep"}
	makeCards(t, root, append(strangers, names...))

	s := server(root)
	s.sweepCards()

	for _, name := range strangers {
		if !exists(t, root, name) {
			t.Errorf("%q terhapus padahal bukan folder kartu", name)
		}
	}
}

// Di bawah ambang, tidak ada yang boleh hilang.
func TestSweepDoesNothingBelowTheLimit(t *testing.T) {
	root := t.TempDir()
	var names []string
	for i := 0; i < keepCards; i++ {
		names = append(names, fmt.Sprintf("card-%d", 4000+i))
	}
	makeCards(t, root, names)

	s := server(root)
	s.sweepCards()

	for _, name := range names {
		if !exists(t, root, name) {
			t.Errorf("%s terhapus padahal jumlahnya masih di batas", name)
		}
	}
}

// Folder yang belum ada bukan error: engine yang baru pertama kali dijalankan
// belum punya data/cards sama sekali.
func TestSweepSurvivesAMissingFolder(t *testing.T) {
	s := server(t.TempDir())
	s.sweepCards() // tidak boleh panik
}

// Folder kartu pilihan pengguna dipakai apa adanya — tanpa menyelipkan
// subfolder "cards" di dalamnya. Pengguna yang memilih D:\Kartu berharap
// kartunya ada di D:\Kartu, bukan di D:\Kartu\cards.
func TestChosenCardFolderIsUsedAsIs(t *testing.T) {
	pilihan := t.TempDir()
	s := &Server{layout: config.Layout{DataDir: t.TempDir(), CardsDir: pilihan}}

	got := s.cardDir("card-123")

	if want := filepath.Join(pilihan, "card-123"); got != want {
		t.Errorf("cardDir = %q, mau %q", got, want)
	}
}

// Tanpa pilihan, kartu tetap di dalam folder data seperti sebelumnya.
func TestCardFolderFallsBackToDataDir(t *testing.T) {
	data := t.TempDir()
	s := &Server{layout: config.Layout{DataDir: data}}

	got := s.cardDir("card-9")

	if want := filepath.Join(data, "cards", "card-9"); got != want {
		t.Errorf("cardDir = %q, mau %q", got, want)
	}
}
