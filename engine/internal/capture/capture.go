// Package capture menangkap foto layar halaman web memakai Chrome headless.
//
// Mengikuti pola paket ffmpeg/transcribe: engine tidak memasang pustaka browser,
// melainkan memanggil biner yang sudah ada di sistem. Chrome/Edge praktis selalu
// tersedia (Edge bawaan Windows), jadi tidak menambah beban pemasangan.
package capture

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client menjalankan Chrome headless.
type Client struct {
	Bin string // path ke chrome/chromium/msedge
}

func New(bin string) *Client { return &Client{Bin: bin} }

// Browser yang bisa dipakai HARUS keluarga Chromium.
//
// Bukan pilihan selera: kartu dirender dengan flag khas Chrome
// (--headless=new, --screenshot, --force-device-scale-factor, --dump-dom).
// Firefox tidak mengenal satu pun dari itu, jadi ia tidak bisa dipakai walau
// terpasang. Yang bisa: Edge, Chrome, Brave, Vivaldi, Opera, Chromium.
var linuxCandidates = []string{
	"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
	"microsoft-edge", "microsoft-edge-stable", "brave-browser",
	"vivaldi", "vivaldi-stable", "opera",
}

// Letak lazim di Windows, relatif terhadap folder program.
//
// Edge didahulukan: ia ada di setiap Windows 10/11 tanpa perlu dipasang, jadi
// pencarian hampir selalu berhenti di baris pertama.
var windowsRelative = []string{
	`Microsoft\Edge\Application\msedge.exe`,
	`Google\Chrome\Application\chrome.exe`,
	`BraveSoftware\Brave-Browser\Application\brave.exe`,
	`Vivaldi\Application\vivaldi.exe`,
	`Chromium\Application\chrome.exe`,
	`Opera\opera.exe`,
	`Microsoft\Edge Beta\Application\msedge.exe`,
	`Google\Chrome Beta\Application\chrome.exe`,
}

// windowsRoots = folder tempat program Windows dipasang, termasuk pemasangan
// per-pengguna yang tidak butuh hak admin (LOCALAPPDATA) — Chrome dan Brave
// sering mendarat di sana pada komputer kantor.
func windowsRoots() []string {
	var roots []string
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "ProgramW6432", "LOCALAPPDATA"} {
		if v := os.Getenv(env); v != "" {
			roots = append(roots, v)
		}
	}
	if len(roots) == 0 {
		roots = []string{`C:\Program Files`, `C:\Program Files (x86)`}
	}
	return roots
}

// wslCandidates = jalur ke browser Windows saat engine jalan DI DALAM WSL.
//
// Terpisah dari daftar Windows asli, dan itu bukan pengulangan: path /mnt/c
// hanya ada di WSL, sedangkan aplikasi yang dipasang di Windows tidak pernah
// melihatnya. Menggabungkan keduanya dulu membuat pengguna Windows mendapat
// "No browser found" padahal Chrome-nya ada.
func wslCandidates() []string {
	if runtime.GOOS != "linux" {
		return nil
	}
	var out []string
	for _, base := range []string{`/mnt/c/Program Files`, `/mnt/c/Program Files (x86)`} {
		for _, rel := range windowsRelative {
			out = append(out, filepath.Join(base, strings.ReplaceAll(rel, `\`, "/")))
		}
	}
	return out
}

// Candidates mengembalikan seluruh tempat yang dicari, apa pun sistemnya.
// Dipakai Find dan juga untuk melaporkan "sudah dicari di mana saja".
func Candidates() []string {
	var out []string
	if runtime.GOOS == "windows" {
		for _, root := range windowsRoots() {
			for _, rel := range windowsRelative {
				out = append(out, filepath.Join(root, strings.ReplaceAll(rel, `\`, string(filepath.Separator))))
			}
		}
		return out
	}
	for _, name := range linuxCandidates {
		if p, err := exec.LookPath(name); err == nil {
			out = append(out, p)
		}
	}
	return append(out, wslCandidates()...)
}

// Find menemukan biner browser yang bisa dipakai. Mengembalikan "" bila tidak
// ada — pemanggil yang memutuskan cara melaporkannya.
func Find() string {
	if v := os.Getenv("CLIPPER_CHROME"); v != "" {
		return v
	}
	for _, p := range Candidates() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// isWindowsBin melaporkan apakah biner ini program Windows yang dijalankan lewat
// interop WSL. Program Windows tidak paham path Linux, jadi berkas masukan &
// keluaran harus lewat folder yang terlihat oleh kedua sisi.
func (c *Client) isWindowsBin() bool {
	// HANYA berlaku saat engine sendiri berjalan di Linux. Di Windows asli,
	// browsernya memang .exe dan tidak ada yang perlu diterjemahkan — engine
	// dan browser sama-sama memakai path Windows.
	//
	// Tanpa syarat ini, aplikasi Windows mencoba memanggil `wslpath` (perkakas
	// yang hanya ada di dalam WSL) dan kartu berita gagal dengan pesan yang
	// menyebut perkakas yang seharusnya tidak pernah ikut terlibat.
	if runtime.GOOS != "linux" {
		return false
	}
	return strings.HasSuffix(strings.ToLower(c.Bin), ".exe")
}

// Options satu penangkapan.
type Options struct {
	Width  int     // lebar viewport (px CSS)
	Height int     // tinggi viewport (px CSS)
	Scale  float64 // faktor skala perangkat; 2 = hasil 2x lebih tajam
	WaitMS int     // anggaran waktu virtual (ms) agar gambar & JS sempat termuat
}

func (o *Options) applyDefaults() {
	if o.Width <= 0 {
		o.Width = 1080
	}
	if o.Height <= 0 {
		o.Height = 1920
	}
	if o.Scale <= 0 {
		o.Scale = 1
	}
	if o.WaitMS <= 0 {
		o.WaitMS = 12000
	}
}

// Screenshot merender url (boleh http/https atau file://) menjadi PNG di outPNG.
//
// Bila Chrome yang dipakai adalah program Windows, penangkapan dilakukan di
// folder temp Windows lalu hasilnya disalin ke outPNG — sebab chrome.exe tidak
// bisa menulis ke path Linux.
func (c *Client) Screenshot(ctx context.Context, url, outPNG string, o Options) error {
	if c.Bin == "" {
		return fmt.Errorf("browser not found: install Chrome/Chromium, or set CLIPPER_CHROME to the chrome.exe path")
	}
	o.applyDefaults()
	if err := os.MkdirAll(filepath.Dir(outPNG), 0o755); err != nil {
		return err
	}

	target := outPNG // path yang ditulis Chrome
	var cleanup func()
	if c.isWindowsBin() {
		winOut, lxOut, err := tempWindowsFile(".png")
		if err != nil {
			return err
		}
		target = winOut
		cleanup = func() { os.Remove(lxOut) }
		defer func() {
			// Salin hasil dari temp Windows ke tujuan sebenarnya.
			if data, err := os.ReadFile(lxOut); err == nil {
				_ = os.WriteFile(outPNG, data, 0o644)
			}
			cleanup()
		}()
	}

	// Profil sementara: mencegah bentrok dengan Chrome yang sedang dipakai
	// pengguna (Chrome menolak dua proses berbagi satu user-data-dir).
	//
	// Untuk chrome.exe profilnya WAJIB berada di disk Windows. Folder /tmp Linux
	// memang terlihat dari Windows lewat \\wsl.localhost, tapi jalur itu tidak
	// mendukung penguncian berkas — Chrome langsung mati dengan LockFileEx gagal.
	profileArg, removeProfile, err := c.profileDir()
	if err != nil {
		return err
	}
	defer removeProfile()

	args := []string{
		"--headless=new",
		"--disable-gpu",
		"--hide-scrollbars",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--mute-audio",
		fmt.Sprintf("--window-size=%d,%d", o.Width, o.Height),
		"--force-device-scale-factor=" + strconv.FormatFloat(o.Scale, 'f', -1, 64),
		fmt.Sprintf("--virtual-time-budget=%d", o.WaitMS),
		"--screenshot=" + target,
	}
	if profileArg != "" {
		args = append(args, "--user-data-dir="+profileArg)
	}
	args = append(args, url)

	// Batas waktu keras: Chrome sesekali menggantung pada halaman bermasalah.
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("browser did not respond within 90 seconds while opening %s", url)
		}
		return fmt.Errorf("browser failed: %v (%s)", err, summarizeStderr(stderr.String()))
	}

	// Chrome bisa keluar dengan status 0 tanpa menulis berkas (mis. URL ditolak).
	check := target
	if c.isWindowsBin() {
		if lx, err := toLinuxPath(target); err == nil {
			check = lx
		}
	}
	if fi, err := os.Stat(check); err != nil || fi.Size() == 0 {
		return fmt.Errorf("browser produced no image for %s: %s", url, summarizeStderr(stderr.String()))
	}
	return nil
}

// profileDir membuat folder profil sementara di sisi yang benar: disk Windows
// bila browsernya chrome.exe, /tmp biasa bila browsernya asli Linux.
func (c *Client) profileDir() (arg string, remove func(), err error) {
	if !c.isWindowsBin() {
		d, err := os.MkdirTemp("", "clipper-chrome-")
		if err != nil {
			return "", nil, err
		}
		return d, func() { os.RemoveAll(d) }, nil
	}
	base, err := windowsTempDir()
	if err != nil {
		return "", nil, err
	}
	win := base + `\` + fmt.Sprintf("clipper-profile-%d", time.Now().UnixNano())
	lx, err := toLinuxPath(win)
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(lx, 0o755); err != nil {
		return "", nil, err
	}
	return win, func() { os.RemoveAll(lx) }, nil
}

// windowsTempDir membaca %TEMP% Windows. Hasilnya di-cache: memanggil cmd.exe
// memakan ratusan milidetik, sedangkan nilainya tidak berubah selama proses.
var (
	tempOnce sync.Once
	tempWin  string
	tempErr  error
)

func windowsTempDir() (string, error) {
	tempOnce.Do(func() {
		out, err := exec.Command("cmd.exe", "/c", "echo %TEMP%").Output()
		if err != nil {
			tempErr = fmt.Errorf("cannot read the Windows temp folder (WSL interop disabled?): %v", err)
			return
		}
		dir := strings.TrimSpace(strings.ReplaceAll(string(out), "\r", ""))
		if dir == "" || strings.Contains(dir, "%TEMP%") {
			tempErr = fmt.Errorf("the Windows temp folder could not be read")
			return
		}
		tempWin = dir
	})
	return tempWin, tempErr
}

// DumpDOM membuka url, menunggu skripnya selesai, lalu mengembalikan DOM akhir.
//
// Dipakai untuk halaman yang pindah alamat lewat JavaScript — Google News,
// misalnya, yang tautannya tidak bisa diikuti dengan redirect HTTP biasa dan
// kodenya tidak lagi memuat URL aslinya. Setelah skripnya jalan, yang terbaca
// di sini sudah berupa halaman artikel yang sebenarnya, lengkap dengan tag
// og: dan badan tulisannya — jadi satu panggilan cukup.
func (c *Client) DumpDOM(ctx context.Context, url string, waitMS int) (string, error) {
	if c.Bin == "" {
		return "", fmt.Errorf("browser not found: install Chrome/Chromium, or set CLIPPER_CHROME to the chrome.exe path")
	}
	if waitMS <= 0 {
		waitMS = 15000
	}
	// Dicoba dua kali. Halaman yang pindah alamat lewat JavaScript kadang belum
	// selesai saat anggaran waktu virtual habis — terutama pada peluncuran
	// Chrome pertama yang masih dingin. Gejalanya khas: keluar sukses tapi DOM
	// nyaris kosong. Percobaan kedua dengan anggaran dua kali lipat hampir
	// selalu berhasil, jadi lebih baik menunggu sebentar daripada melempar
	// galat yang sebenarnya cuma soal waktu.
	dom, err := c.dumpOnce(ctx, url, waitMS)
	if err == nil {
		return dom, nil
	}
	return c.dumpOnce(ctx, url, waitMS*2)
}

func (c *Client) dumpOnce(ctx context.Context, url string, waitMS int) (string, error) {
	profile, remove, err := c.profileDir()
	if err != nil {
		return "", err
	}
	defer remove()

	args := []string{
		"--headless=new",
		"--disable-gpu",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--mute-audio",
		"--user-data-dir=" + profile,
		fmt.Sprintf("--virtual-time-budget=%d", waitMS),
		"--dump-dom",
		url,
	}

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("browser did not respond within 90 seconds while opening %s", url)
		}
		return "", fmt.Errorf("browser could not open the page: %v (%s)", err, summarizeStderr(stderr.String()))
	}
	dom := out.String()
	if len(dom) < 200 {
		return "", fmt.Errorf("page %s returned no content: %s", url, summarizeStderr(stderr.String()))
	}
	return dom, nil
}

// tempWindowsFile menyiapkan nama berkas di folder temp Windows dan
// mengembalikan pasangan (path Windows, path Linux) untuk berkas yang sama.
func tempWindowsFile(ext string) (win, lx string, err error) {
	dir, err := windowsTempDir()
	if err != nil {
		return "", "", err
	}
	win = dir + `\` + fmt.Sprintf("clipper-%d%s", time.Now().UnixNano(), ext)
	lx, err = toLinuxPath(win)
	if err != nil {
		return "", "", err
	}
	return win, lx, nil
}

func toLinuxPath(win string) (string, error) {
	out, err := exec.Command("wslpath", "-u", win).Output()
	if err != nil {
		return "", fmt.Errorf("wslpath could not translate %q: %v", win, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func toWindowsPath(lx string) (string, error) {
	out, err := exec.Command("wslpath", "-w", lx).Output()
	if err != nil {
		return "", fmt.Errorf("wslpath could not translate %q: %v", lx, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// fileURL menyusun URL file:// dari sebuah path absolut, di sistem mana pun.
//
// Menempelkan "file://" di depan path Windows menghasilkan alamat yang rusak:
// "file://C:\Users\..." membuat "C:" terbaca sebagai NAMA HOST, dan backslash
// bukan karakter yang sah di URL. Chrome menolaknya tanpa pesan yang berguna —
// kartu berita tinggal keluar kosong.
//
// Bentuk yang benar untuk Windows adalah "file:///C:/Users/...": tiga garis
// miring (host kosong) dan pemisah folder berupa garis miring biasa.
func fileURL(abs string) string {
	p := abs
	// Sengaja BUKAN filepath.ToSlash: fungsi itu ikut sistem tempat kode
	// berjalan, sedangkan di sini path Windows juga harus benar saat
	// diterjemahkan dari WSL. Pemeriksaan huruf drive membuat backslash di
	// dalam nama berkas Linux (sah, walau langka) tidak ikut diubah.
	if isWindowsPath(p) {
		p = strings.ReplaceAll(p, `\`, "/")
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // "C:/Users/…" → "/C:/Users/…"
	}
	// Spasi lazim di path Windows ("C:/Users/Budi Santoso/…") dan memutus URL
	// saat diteruskan sebagai argumen baris perintah.
	p = strings.ReplaceAll(p, " ", "%20")
	p = strings.ReplaceAll(p, "#", "%23")
	p = strings.ReplaceAll(p, "?", "%3F")
	return "file://" + p
}

// isWindowsPath melaporkan apakah path diawali huruf drive, mis. "C:\\…".
func isWindowsPath(p string) bool {
	if len(p) < 2 || p[1] != ':' {
		return false
	}
	c := p[0]
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// FileURL mengubah path berkas lokal jadi URL file:// yang bisa dibuka Chrome.
// Saat engine di WSL memanggil chrome.exe, path-nya diterjemahkan dulu ke
// bentuk Windows — chrome.exe tidak mengenal path Linux.
func (c *Client) FileURL(path string) (string, error) {
	if c.isWindowsBin() {
		w, err := toWindowsPath(path)
		if err != nil {
			return "", err
		}
		return fileURL(w), nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return fileURL(abs), nil
}

// WriteTempFile menulis isi ke lokasi yang bisa dibaca browser, lalu
// mengembalikan URL-nya beserta fungsi pembersih.
//
// Untuk chrome.exe berkas ditaruh di temp Windows: berkas di /tmp Linux memang
// bisa dijangkau lewat \\wsl.localhost, tapi jalur itu lambat dan kadang
// diblokir kebijakan keamanan Windows.
func (c *Client) WriteTempFile(content []byte, ext string) (url string, cleanup func(), err error) {
	if c.isWindowsBin() {
		win, lx, err := tempWindowsFile(ext)
		if err != nil {
			return "", nil, err
		}
		if err := os.WriteFile(lx, content, 0o644); err != nil {
			return "", nil, err
		}
		return fileURL(win), func() { os.Remove(lx) }, nil
	}
	f, err := os.CreateTemp("", "clipper-*"+ext)
	if err != nil {
		return "", nil, err
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	f.Close()
	return fileURL(f.Name()), func() { os.Remove(f.Name()) }, nil
}

// summarizeStderr mengambil baris stderr yang berguna saja. Chrome membanjiri
// stderr dengan peringatan GPU/USB/ekstensi yang tidak ada kaitannya dengan
// kegagalan.
func summarizeStderr(s string) string {
	var kept []string
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		l := strings.ToLower(ln)
		if strings.Contains(l, "usb") || strings.Contains(l, "gpu") ||
			strings.Contains(l, "extension") || strings.Contains(l, "gcm") ||
			strings.Contains(l, "registration") || strings.Contains(l, "dbus") ||
			strings.Contains(l, "bluetooth") || strings.Contains(l, "voice") {
			continue
		}
		kept = append(kept, ln)
		if len(kept) >= 3 {
			break
		}
	}
	if len(kept) == 0 {
		return "(no clear error message)"
	}
	return strings.Join(kept, "; ")
}
