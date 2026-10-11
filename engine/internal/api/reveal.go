package api

import (
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Menunjukkan berkas hasil DI TEMPATNYA, bukan mengunduhnya.
//
// Tautan <a download> adalah pola web, dan di aplikasi desktop ia mahal tanpa
// membeli apa pun: engine membaca berkas dari disk, mengirimkan isinya lewat
// HTTP, lalu browser menuliskannya LAGI ke folder Downloads. Video 200 MB
// disalin dari satu tempat di disk ke tempat lain di disk yang sama, dan
// hasilnya dua salinan berkas yang sama plus .srt yang berakhir di folder
// berbeda dari videonya.
//
// Ia juga memicu dialog izin Chrome "wants to Download multiple files", sebab
// satu kartu klip menawarkan empat tautan sekaligus. Izin itu disimpan browser
// per origin BESERTA nomor portnya — dan saat terpasang engine memakai port
// acak tiap kali dijalankan, jadi "Allow" tidak pernah menempel.
//
// Semuanya hilang dengan membuka pengelola berkas sistem: tidak ada byte yang
// berpindah, tidak ada salinan, tidak ada yang bisa ditanyakan browser, dan
// keempat berkas terlihat sekaligus sebab memang sudah bersebelahan di sana.
//
// Simetris dengan masukannya (notes/24): berkas sumber tidak pernah diunggah,
// engine cukup diberi tahu letaknya.

// revealClip membuka folder satu klip dengan berkasnya tersorot.
//
// Yang diterima id job + id klip, BUKAN path. Endpoint yang menjalankan program
// sistem dengan path kiriman klien adalah lubang; di sini pathnya disusun
// engine sendiri dari catatan job.
func (s *Server) revealClip(w http.ResponseWriter, r *http.Request) {
	j, ok := s.mgr.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "job not found")
		return
	}
	clipID := r.PathValue("clip")
	for _, cl := range j.Snapshot().Clips {
		if cl.ID != clipID {
			continue
		}
		path := cl.VideoPath
		if path == "" {
			path = cl.VideoPathRaw
		}
		if path == "" {
			break
		}
		if err := reveal(path); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "opened"})
		return
	}
	writeErr(w, 404, "the clip is not available yet")
}

// reveal membuka pengelola berkas sistem pada berkas yang diberikan.
//
// Argumennya diteruskan sebagai argv, tidak pernah lewat shell, jadi spasi &
// tanda kutip di dalam path tidak bisa berubah jadi perintah.
func reveal(path string) error {
	switch {
	case runtime.GOOS == "windows":
		// explorer mengembalikan status keluar BUKAN NOL walau berhasil — ia
		// melapor "tidak ada jendela baru yang dibuat", bukan "gagal". Kalau
		// statusnya diperiksa, tombol yang bekerja dengan benar akan selalu
		// melaporkan galat.
		_ = exec.Command("explorer", "/select,"+path).Run()
		return nil
	case runtime.GOOS == "darwin":
		return exec.Command("open", "-R", path).Run()
	case inWSL():
		// Di WSL tidak ada pengelola berkas; yang ada Explorer di sisi Windows,
		// dan ia tidak paham path Linux.
		win, err := toWindowsPath(path)
		if err != nil {
			return err
		}
		_ = exec.Command("explorer.exe", "/select,"+win).Run()
		return nil
	default:
		// xdg-open tidak punya "sorot berkas ini", jadi yang dibuka foldernya.
		if err := exec.Command("xdg-open", filepath.Dir(path)).Run(); err != nil {
			return fmt.Errorf("could not open the folder: no file manager answered (%w)", err)
		}
		return nil
	}
}

// inWSL: engine berjalan di Linux DAN `wslpath` ada. Keduanya, sebab wslpath
// hanya ada di dalam WSL dan Linux biasa tidak boleh ikut memakai jalur ini.
func inWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := exec.LookPath("wslpath")
	return err == nil
}

func toWindowsPath(lx string) (string, error) {
	out, err := exec.Command("wslpath", "-w", lx).Output()
	if err != nil {
		return "", fmt.Errorf("wslpath could not translate %q: %v", lx, err)
	}
	return strings.TrimSpace(string(out)), nil
}
