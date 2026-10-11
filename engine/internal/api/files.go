package api

import (
	"fmt"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Menjelajah berkas lewat engine, bukan lewat unggahan.
//
// Kotak unggah menyalin video ke data/uploads dulu — 3,84 GB digandakan sebelum
// pekerjaan dimulai. Di aplikasi desktop itu tidak masuk akal: berkasnya sudah
// ada di mesin yang sama, engine tinggal membacanya di tempat. Dua endpoint di
// berkas ini yang membuat penyalinan itu bisa dihindari:
//
//	/api/browse  daftar isi folder, supaya GUI punya pemilih berkas sendiri
//	/api/locate  mencocokkan berkas yang di-drop (nama + ukuran) ke path aslinya
//
// Keduanya hanya boleh dijangkau dari mesin ini sendiri — engine mendengar di
// 127.0.0.1, dan token sesi menjaga sisanya.

// videoExts = akhiran yang dianggap video. Dipakai untuk menyorot berkas yang
// relevan di pemilih; berkas lain tetap ditampilkan supaya pengguna tidak
// bingung mencari sesuatu yang dilihatnya ada di file manager.
var videoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".mov": true, ".avi": true, ".webm": true,
	".m4v": true, ".ts": true, ".flv": true, ".wmv": true, ".mpg": true,
	".mpeg": true, ".m2ts": true, ".mts": true, ".3gp": true, ".ogv": true,
}

// isVideo melaporkan apakah nama berkas berakhiran video yang dikenal.
func isVideo(name string) bool {
	return videoExts[strings.ToLower(filepath.Ext(name))]
}

// entry satu baris di pemilih berkas.
type entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Dir   bool   `json:"dir"`
	Video bool   `json:"video"`
	Size  int64  `json:"size"`
}

// place = pintasan folder yang ditawarkan di sisi pemilih.
type place struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// maxEntries membatasi isi satu folder yang dikirim. Folder berisi puluhan ribu
// berkas ada di mesin siapa pun (cache, dataset); tanpa batas, satu klik salah
// membekukan GUI sampai daftar itu selesai dirender.
const maxEntries = 2000

// browse melaporkan isi satu folder.
func (s *Server) browse(w http.ResponseWriter, r *http.Request) {
	dir := hostPath(r.URL.Query().Get("dir"))
	if dir == "" {
		dir = homeDir()
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		writeErr(w, 400, "invalid folder: "+err.Error())
		return
	}
	abs = openableDir(abs)
	items, err := os.ReadDir(abs)
	if err != nil {
		writeErr(w, 400, "cannot open the folder: "+err.Error())
		return
	}

	entries := make([]entry, 0, len(items))
	for _, it := range items {
		name := it.Name()
		// Berkas tersembunyi disaring: pengguna tidak menyimpan videonya di
		// ~/.cache, dan daftar jadi jauh lebih pendek tanpa itu.
		if strings.HasPrefix(name, ".") {
			continue
		}
		e := entry{Name: name, Path: filepath.Join(abs, name), Dir: it.IsDir()}
		if !e.Dir {
			e.Video = isVideo(name)
			if info, err := it.Info(); err == nil {
				e.Size = info.Size()
			}
		}
		entries = append(entries, e)
		if len(entries) >= maxEntries {
			break
		}
	}
	// Folder dulu, lalu berkas; masing-masing menurut abjad tanpa membedakan
	// besar-kecil huruf — urutan yang sama dengan file manager mana pun.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	parent := filepath.Dir(abs)
	if parent == abs { // sudah di akar
		parent = ""
	}
	writeJSON(w, 200, map[string]any{
		"dir":       abs,
		"parent":    parent,
		"entries":   entries,
		"places":    places(),
		"truncated": len(entries) >= maxEntries,
	})
}

// hostPath menerjemahkan path gaya Windows jadi path yang benar-benar bisa
// dibuka engine.
//
// Di WSL, video pengguna ada di sisi Windows, dan yang tersalin dari Explorer
// adalah "C:\Users\...\video.mp4". filepath.Abs di Linux mengubahnya jadi
// "<folder kerja>/C:\Users\..." — folder yang tidak pernah ada. Akibatnya
// pemilih berkas diam-diam kembali ke folder kerja dan folder Windows tampak
// tidak bisa dibuka sama sekali, padahal isinya terjangkau lewat /mnt/c.
//
// Tiga bentuk yang benar-benar dipakai orang, dan ketiganya datang dari satu
// perintah yang sama — "Copy as path" di Explorer (Shift + klik kanan):
//
//	C:\Users\Budi\video.mp4                       → /mnt/c/Users/Budi/video.mp4
//	"C:\Users\Budi\video.mp4"                     → idem; Explorer MEMBUNGKUSNYA
//	                                                 dengan kutip, selalu
//	\\wsl.localhost\Ubuntu\home\budi\video.mp4    → /home/budi/video.mp4
//
// Bentuk ketiga muncul saat berkasnya justru ada di sisi Linux dan dibuka lewat
// Explorer. "\\wsl$\..." adalah nama lama untuk hal yang sama.
//
// Share jaringan sungguhan ("\\server\bagi\video.mp4") TIDAK disentuh: ia bukan
// berkas mesin ini, dan menebak terjemahannya hanya menghasilkan path palsu.
//
// Di Windows asli tidak ada yang diterjemahkan — di sana path itu sudah benar
// apa adanya — tetapi kutipnya tetap dibuang, sebab Explorer yang sama juga
// memasangnya di sana.
func hostPath(p string) string {
	// Di mode web hanya folder unggahan yang boleh ditunjuk (web.go).
	return insideWebRoot(localPath(p))
}

// localPath = terjemahan hostPath, tanpa pagar mode web.
func localPath(p string) string {
	p = unquote(strings.TrimSpace(p))
	if runtime.GOOS != "linux" || p == "" {
		return p
	}
	if lx, ok := wslUNC(p); ok {
		return lx
	}
	if len(p) < 2 || p[1] != ':' {
		return p
	}
	drive := p[0]
	if !(drive >= 'A' && drive <= 'Z') && !(drive >= 'a' && drive <= 'z') {
		return p
	}
	// Backslash BUKAN pemisah folder di Linux — ia karakter nama berkas yang
	// sah, jadi ia harus ditukar di sini dan bukan diserahkan ke filepath.
	rest := strings.ReplaceAll(p[2:], `\`, "/")
	return filepath.Join("/mnt", strings.ToLower(string(drive)), rest)
}

// unquote membuang kutip pembungkus yang dipasang Explorer.
func unquote(p string) string {
	if len(p) >= 2 && (p[0] == '"' && p[len(p)-1] == '"' || p[0] == '\'' && p[len(p)-1] == '\'') {
		return strings.TrimSpace(p[1 : len(p)-1])
	}
	return p
}

// wslUNC menerjemahkan "\\wsl.localhost\<distro>\..." (dan "\\wsl$\<distro>\...")
// jadi path Linux biasa. ok=false untuk apa pun selain kedua host itu.
func wslUNC(p string) (string, bool) {
	if !strings.HasPrefix(p, `\\`) {
		return "", false
	}
	parts := strings.Split(strings.ReplaceAll(p[2:], `\`, "/"), "/")
	if len(parts) < 3 {
		return "", false
	}
	host := strings.ToLower(parts[0])
	if host != "wsl.localhost" && host != "wsl$" {
		return "", false
	}
	// parts[1] = nama distro. Ia tidak ikut: dari DALAM distro itu, akarnya
	// memang "/". Distro lain tidak terjangkau dari sini, dan menganggapnya
	// terjangkau justru menghasilkan path yang menunjuk berkas yang salah.
	return "/" + strings.Join(parts[2:], "/"), true
}

// openableDir mengubah apa pun yang diberikan klien jadi folder yang benar-benar
// bisa dibuka.
//
// Dua keadaan yang wajar terjadi, dan keduanya dulu berakhir sebagai galat yang
// membingungkan:
//
//   - Yang dikirim adalah BERKAS, bukan folder. Terjadi setiap kali pemilih
//     dibuka dari sebuah komponen yang letaknya sudah diketahui ("ganti
//     berkasnya" untuk ffprobe.exe) — yang dimaksud pengguna jelas: buka folder
//     tempat berkas itu berada.
//   - Path-nya sudah tidak ada lagi (program dipindah, folder dihapus). Naik ke
//     induknya sampai ada yang bisa dibuka, dan kalau tidak ada sama sekali,
//     kembali ke folder rumah.
func openableDir(p string) string {
	for cur := p; ; {
		if st, err := os.Stat(cur); err == nil {
			if st.IsDir() {
				return cur
			}
			cur = filepath.Dir(cur) // berkas → foldernya
			continue
		}
		parent := filepath.Dir(cur)
		if parent == cur { // sudah di akar dan tetap tidak ada
			return homeDir()
		}
		cur = parent
	}
}

// locate mencari path asli sebuah berkas dari nama & ukurannya.
//
// Browser tidak memberi tahu halaman web di mana berkas yang di-drop berada —
// hanya nama, ukuran, dan waktu ubahnya. Padahal berkas itu ada di mesin yang
// sama dengan engine. Jadi engine mencarinya sendiri di tempat-tempat yang
// wajar; kalau ketemu, penyalinan 3,84 GB tidak perlu terjadi sama sekali.
//
// Nama DAN ukuran harus sama persis. Kalau ada lebih dari satu yang cocok,
// jawabannya kosong: menebak berkas mana yang dimaksud lebih buruk daripada
// menyuruh pengguna memilih sendiri.
func (s *Server) locate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	req.Name = filepath.Base(strings.TrimSpace(req.Name))
	if req.Name == "" || req.Name == "." || req.Name == string(filepath.Separator) {
		writeErr(w, 400, "the 'name' field is required")
		return
	}
	if req.Size <= 0 {
		writeErr(w, 400, "the 'size' field is required")
		return
	}

	found := findFile(searchRoots(), req.Name, req.Size, time.Now().Add(locateBudget))
	if len(found) != 1 {
		// 404 juga untuk hasil ganda: GUI memperlakukan keduanya sama —
		// kembali ke unggahan biasa.
		writeJSON(w, 404, map[string]any{
			"error":   "the file was not found on this computer",
			"matches": len(found),
		})
		return
	}
	writeJSON(w, 200, map[string]any{"path": found[0]})
}

// locateBudget membatasi lama pencarian. Ini berjalan sementara pengguna
// menunggu setelah men-drop berkas, jadi ia harus selesai dalam hitungan
// kedipan mata; gagal menemukan bukan bencana, sebab unggahan tetap ada.
const locateBudget = 2500 * time.Millisecond

// locateDepth = kedalaman folder yang ditelusuri dari tiap titik awal.
// Cukup untuk "Videos/2026/Agustus/rekaman.mp4", tidak sampai menyisir seluruh
// disk.
const locateDepth = 4

// findFile menelusuri beberapa folder awal sampai anggaran waktu habis.
func findFile(roots []string, name string, size int64, deadline time.Time) []string {
	var out []string
	seen := map[string]bool{}
	for _, root := range roots {
		if root == "" || seen[root] {
			continue
		}
		seen[root] = true
		walkFor(root, name, size, 0, deadline, &out)
		// Dua hasil sudah cukup untuk tahu jawabannya ambigu.
		if len(out) > 1 || time.Now().After(deadline) {
			break
		}
	}
	return out
}

// skipDirs = folder yang tidak pernah berisi video pengguna tapi isinya puluhan
// ribu berkas. Menyisirnya hanya menghabiskan anggaran waktu.
var skipDirs = map[string]bool{
	"node_modules": true, "AppData": true, "Library": true, "Windows": true,
	"Program Files": true, "Program Files (x86)": true, "$Recycle.Bin": true,
	"snap": true, "go": true, "venv": true, "__pycache__": true,
}

func walkFor(dir, name string, size int64, depth int, deadline time.Time, out *[]string) {
	if depth > locateDepth || time.Now().After(deadline) || len(*out) > 1 {
		return
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var subdirs []string
	for _, it := range items {
		if it.IsDir() {
			n := it.Name()
			if strings.HasPrefix(n, ".") || skipDirs[n] {
				continue
			}
			subdirs = append(subdirs, filepath.Join(dir, n))
			continue
		}
		if it.Name() != name {
			continue
		}
		info, err := it.Info()
		if err != nil || info.Size() != size {
			continue
		}
		*out = append(*out, filepath.Join(dir, name))
		if len(*out) > 1 {
			return
		}
	}
	// Berkas di folder ini diperiksa lebih dulu, baru turun — berkas yang dicari
	// hampir selalu ada di folder dangkal (Downloads, Videos).
	for _, sd := range subdirs {
		walkFor(sd, name, size, depth+1, deadline, out)
		if len(*out) > 1 || time.Now().After(deadline) {
			return
		}
	}
}

// homeDir mengembalikan folder rumah pengguna, atau folder kerja bila tak ada.
func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return string(filepath.Separator)
}

// userFolders = nama folder video yang lazim, dalam bahasa Inggris. Sengaja
// tidak diterjemahkan: yang dibaca adalah nama folder di disk, dan Windows
// maupun Linux menamainya dalam bahasa Inggris walau tampilannya lain.
var userFolders = []string{"Videos", "Movies", "Downloads", "Desktop", "Documents"}

// places mengembalikan pintasan yang ditampilkan di sisi pemilih berkas:
// rumah, folder video yang lazim, dan (di WSL) folder rumah Windows.
func places() []place {
	home := homeDir()
	out := []place{{Name: "Home", Path: home}}
	for _, n := range userFolders {
		p := filepath.Join(home, n)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			out = append(out, place{Name: n, Path: p})
		}
	}
	for _, p := range windowsHomes() {
		base := filepath.Base(p)
		out = append(out, place{Name: "Windows: " + base, Path: p})
		for _, n := range userFolders {
			sub := filepath.Join(p, n)
			if st, err := os.Stat(sub); err == nil && st.IsDir() {
				out = append(out, place{Name: base + " / " + n, Path: sub})
			}
		}
	}
	out = append(out, drives()...)
	return out
}

// drives menambahkan huruf drive di Windows.
//
// Tanpa ini, pengguna Windows yang menyimpan videonya di D: harus mengetik
// path-nya sendiri — folder rumah saja tidak cukup di sistem yang memang
// membagi disknya per huruf.
func drives() []place {
	if runtime.GOOS != "windows" {
		return nil
	}
	var out []place
	for c := 'C'; c <= 'Z'; c++ {
		root := string(c) + `:\`
		if _, err := os.Stat(root); err == nil {
			out = append(out, place{Name: string(c) + ":", Path: root})
		}
	}
	return out
}

// searchRoots = titik awal pencarian /api/locate, dari yang paling mungkin.
func searchRoots() []string {
	var out []string
	home := homeDir()
	for _, n := range userFolders {
		out = append(out, filepath.Join(home, n))
	}
	for _, p := range windowsHomes() {
		for _, n := range userFolders {
			out = append(out, filepath.Join(p, n))
		}
	}
	// Rumah paling akhir: ia mencakup semua di atas, jadi baru dipakai bila
	// yang spesifik tidak membuahkan hasil.
	out = append(out, home)
	return out
}

// windowsHomes mencari folder rumah Windows saat engine jalan di dalam WSL.
//
// Di WSL, video pengguna hampir selalu ada di sisi Windows (/mnt/c/Users/...),
// bukan di folder rumah Linux — tanpa ini pemilih berkas terbuka di tempat yang
// pasti kosong.
func windowsHomes() []string {
	if runtime.GOOS != "linux" {
		return nil
	}
	var out []string
	for _, drive := range []string{"c", "d"} {
		users := filepath.Join("/mnt", drive, "Users")
		items, err := os.ReadDir(users)
		if err != nil {
			continue
		}
		for _, it := range items {
			n := it.Name()
			if !it.IsDir() || n == "Public" || n == "Default" || n == "All Users" ||
				strings.HasPrefix(n, ".") || strings.HasPrefix(n, "Default") {
				continue
			}
			// Hanya folder rumah pengguna yang login — daftar lengkap isi
			// C:\Users membingungkan dan sebagian tidak bisa dibaca.
			if u, err := user.Current(); err == nil && !strings.EqualFold(u.Username, n) {
				if _, err := os.Stat(filepath.Join(users, n, "Desktop")); err != nil {
					continue
				}
			}
			out = append(out, filepath.Join(users, n))
		}
	}
	return out
}

// Gambar watermark: dipilih pengguna, dibaca engine di tempat.
//
// Alurnya sengaja sama persis dengan video (notes/24): tidak ada yang diunggah,
// yang berpindah cuma path-nya. Bedanya satu — banner harus TERLIHAT di
// pratinjau supaya bisa digeser, dan <img> di browser tidak bisa membuka path
// lokal. /api/image itulah jembatannya, dan ia sengaja sesempit mungkin:
// hanya berkas gambar, hanya yang benar-benar ada, dan ada batas ukurannya.
var imageExts = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
}

// maxImageBytes membatasi apa yang boleh dikirim ke browser sebagai banner.
// Banner 1080 px lebar berukuran ratusan kilobyte; 20 MB sudah sangat longgar,
// dan batas ini yang mencegah satu salah pilih (berkas .png hasil ekspor 8K)
// membekukan pratinjau.
const maxImageBytes = 20 << 20

// checkImage memastikan path itu benar-benar berkas gambar yang bisa dipakai,
// dan mengembalikan tipe MIME-nya.
//
// Diperiksa SEBELUM job dibuat, bukan dibiarkan gagal di dalam ffmpeg: rantai
// filter yang gagal membuka `movie=` berhenti dengan pesan yang menyebut sintaks
// filter, bukan nama berkas yang salah — dan itu terjadi setelah transkripsi
// selesai, belasan menit sesudah tombol ditekan.
func checkImage(path string) (string, error) {
	mime, ok := imageExts[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return "", fmt.Errorf("the watermark must be a PNG, JPEG, or WebP image: %s", path)
	}
	st, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("the watermark image was not found on this computer: %s", path)
	}
	if st.IsDir() {
		return "", fmt.Errorf("that path is a folder, not an image: %s", path)
	}
	if st.Size() > maxImageBytes {
		return "", fmt.Errorf("the watermark image is larger than %d MB: %s", maxImageBytes>>20, path)
	}
	return mime, nil
}

// image menyajikan satu berkas gambar lokal supaya pratinjau bisa menampilkannya.
func (s *Server) image(w http.ResponseWriter, r *http.Request) {
	path := hostPath(r.URL.Query().Get("path"))
	if path == "" {
		writeErr(w, 400, "the 'path' parameter is required")
		return
	}
	mime, err := checkImage(path)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeErr(w, 400, "the watermark image could not be read: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", mime)
	// Tanpa cache: berkasnya bisa ditimpa di luar aplikasi, dan pratinjau yang
	// menampilkan versi lama adalah persis jenis kebohongan yang membuat orang
	// menyerahkan render dengan banner yang salah.
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
