package api

import (
	"archive/zip"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// bulkZip mengirim SATU zip berisi semua yang dicentang di halaman riwayat.
//
//	GET /api/download?clip=<jobID>/<clipID>&clip=…&card=<cardID>&…
//
// Kenapa satu zip dan bukan sederet unduhan: mengklik sepuluh tautan `download`
// berturut-turut membuat browser bertanya "izinkan mengunduh banyak berkas?"
// (dan WebView2 kadang diam-diam menolak yang kedua dan seterusnya). Satu berkas
// selalu berhasil, dan pengguna yang mencentang sepuluh klip memang menginginkan
// satu paket, bukan sepuluh dialog simpan.
//
// Ditulis LANGSUNG ke koneksi, tidak disusun di memori: satu klip 1080p bisa
// ratusan MB dan riwayat berisi puluhan.
func (s *Server) bulkZip(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clips, cards := q["clip"], q["card"]
	if len(clips)+len(cards) == 0 {
		writeErr(w, 400, "nothing selected")
		return
	}

	// Semua berkas dikumpulkan LEBIH DULU, sebelum satu byte pun dikirim:
	// setelah header terkirim, id yang salah tidak bisa lagi dijawab 404 —
	// yang sampai ke pengguna hanya zip yang isinya kurang tanpa penjelasan.
	type entry struct{ name, path string }
	var files []entry
	for _, key := range clips {
		jobID, clipID, ok := strings.Cut(key, "/")
		if !ok {
			writeErr(w, 400, "invalid clip key "+key)
			return
		}
		j, found := s.mgr.Get(jobID)
		if !found {
			writeErr(w, 404, "job not found: "+jobID)
			return
		}
		for _, c := range j.Snapshot().Clips {
			if c.ID != clipID {
				continue
			}
			// Semua yang dihasilkan klip itu ikut: video (bersubtitle dan/atau
			// polos), .srt, dan .txt bahan caption. Yang mencentang satu klip
			// menginginkan klip itu, bukan sebagian dari klip itu.
			for _, p := range []string{c.VideoPath, c.VideoPathRaw, c.SubtitleSRT, c.TranscriptTXT} {
				if p == "" {
					continue
				}
				files = append(files, entry{jobID + "/" + filepath.Base(p), p})
			}
		}
	}
	for _, id := range cards {
		if !cardIDPattern.MatchString(id) {
			writeErr(w, 400, "invalid card id "+id)
			return
		}
		dir := s.cardDir(id)
		items, err := os.ReadDir(dir)
		if err != nil {
			writeErr(w, 404, "card not found: "+id)
			return
		}
		for _, it := range items {
			if it.IsDir() {
				continue
			}
			files = append(files, entry{id + "/" + it.Name(), filepath.Join(dir, it.Name())})
		}
	}
	if len(files) == 0 {
		writeErr(w, 404, "nothing to download: the files are gone")
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="clipper-selection.zip"`)

	zw := zip.NewWriter(w)
	defer zw.Close()
	// Nama yang sama muncul dua kali kalau dua job menghasilkan berkas berjudul
	// sama; zip mengizinkannya, tapi pembongkarnya menimpa yang pertama.
	seen := map[string]int{}
	for _, f := range files {
		src, err := os.Open(f.path)
		if err != nil {
			continue // berkas dihapus di sela-sela; sisanya tetap dikirim
		}
		name := f.name
		if n := seen[name]; n > 0 {
			ext := filepath.Ext(name)
			name = strings.TrimSuffix(name, ext) + "-" + string(rune('0'+n)) + ext
		}
		seen[f.name]++
		dst, err := zw.Create(name)
		if err == nil {
			_, err = io.Copy(dst, src)
		}
		src.Close()
		if err != nil {
			return // koneksi putus; tidak ada lagi yang bisa dilaporkan
		}
	}
}
