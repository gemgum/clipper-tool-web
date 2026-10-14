package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// ThumbCount = jumlah frame cuplikan per video. GUI menghitung detik tiap frame
// dengan rumus yang sama (thumbAt), jadi angka ini harus sama di keduanya.
const ThumbCount = 8

// thumbAt = detik frame ke-i: di tengah tiap seperdelapan video, supaya frame
// pertama bukan layar hitam pembuka dan yang terakhir bukan layar penutup.
func thumbAt(dur float64, i int) float64 { return dur * (float64(i) + 0.5) / ThumbCount }

// thumb menyajikan frame cuplikan ke-i dari sebuah video.
//
// Dibuat sekali saat pertama diminta, lalu disimpan di data/thumbs/<kunci>/.
// Kuncinya dari path + ukuran + waktu ubah berkas: video yang ditimpa dengan
// nama sama mendapat cuplikan baru, bukan gambar lama. Bukan di sebelah
// videonya: di desktop itu folder milik pengguna, dan di web folder unggahan
// harus bisa kosong supaya ikut terhapus (dropUploads).
func (s *Server) thumb(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	path := hostPath(q.Get("path"))
	i, err := strconv.Atoi(q.Get("i"))
	if path == "" || err != nil || i < 0 || i >= ThumbCount {
		writeErr(w, 400, "'path' and 'i' (0–7) are required")
		return
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		writeErr(w, 404, "video not found")
		return
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d", path, st.Size(), st.ModTime().UnixNano())))
	dir := filepath.Join(s.thumbDir(), hex.EncodeToString(sum[:10]))
	out := filepath.Join(dir, strconv.Itoa(i)+".jpg")
	if _, err := os.Stat(out); err != nil {
		dur, err := s.ff.Duration(r.Context(), path)
		if err != nil || dur <= 0 {
			writeErr(w, 422, "cannot read the video length")
			return
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		// Ditulis ke nama sementara lalu diganti nama: dua permintaan yang
		// sama sekaligus tidak pernah menyajikan JPEG setengah jadi.
		tmp := fmt.Sprintf("%s.%d.tmp.jpg", out, time.Now().UnixNano())
		if err := s.ff.Thumb(r.Context(), path, thumbAt(dur, i), 320, tmp); err != nil {
			os.Remove(tmp)
			writeErr(w, 422, "frame extraction failed")
			return
		}
		if err := os.Rename(tmp, out); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
	}
	w.Header().Set("Cache-Control", "private, no-cache")
	http.ServeFile(w, r, out)
}

func (s *Server) thumbDir() string { return filepath.Join(s.paths.DataDir, "thumbs") }

// --- video aktif per akun ---
//
// Video yang sedang dipilih di halaman Clips disimpan di SERVER per akun, bukan
// di localStorage browser: dengan akun yang sama, komputer mana pun membuka
// video dan pratinjau yang sama. Akun = subjek OIDC; tanpa OIDC (sandi bersama
// atau desktop) semua orang satu akun.

func (s *Server) sourceFile(r *http.Request) string {
	account := "local"
	if s.auth != nil {
		if sess := s.auth.session(r); sess != nil {
			account = "oidc:" + sess.sub
		}
	}
	sum := sha256.Sum256([]byte(account))
	return filepath.Join(s.paths.DataDir, "sources", hex.EncodeToString(sum[:10])+".json")
}

type activeSource struct {
	Path string `json:"path"`
}

// getSource: video aktif akun ini, atau path kosong bila belum ada / berkasnya
// sudah hilang (mis. dihapus setelah job sukses).
func (s *Server) getSource(w http.ResponseWriter, r *http.Request) {
	var cur activeSource
	if b, err := os.ReadFile(s.sourceFile(r)); err == nil {
		_ = json.Unmarshal(b, &cur)
	}
	if p := hostPath(cur.Path); p == "" {
		cur.Path = ""
	} else if st, err := os.Stat(p); err != nil || st.IsDir() {
		cur.Path = ""
	}
	writeJSON(w, 200, cur)
}

// setSource menyimpan video aktif akun ini; path kosong = tidak ada video.
func (s *Server) setSource(w http.ResponseWriter, r *http.Request) {
	var req activeSource
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	f := s.sourceFile(r)
	if req.Path == "" {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, req)
		return
	}
	p := hostPath(req.Path)
	if st, err := os.Stat(p); p == "" || err != nil || st.IsDir() {
		writeErr(w, 400, "that video is not available on the server")
		return
	}
	b, _ := json.Marshal(activeSource{Path: p})
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := os.WriteFile(f, b, 0o644); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, activeSource{Path: p})
}

// media mengalirkan video sumber untuk pemutar di GUI. http.ServeFile melayani
// Range, jadi pemutar bisa melompat ke menit mana pun tanpa mengunduh semuanya.
// Di mode web hostPath hanya mengizinkan folder unggahan.
func (s *Server) media(w http.ResponseWriter, r *http.Request) {
	path := hostPath(r.URL.Query().Get("path"))
	st, err := os.Stat(path)
	if path == "" || err != nil || st.IsDir() {
		writeErr(w, 404, "video not found")
		return
	}
	http.ServeFile(w, r, path)
}
