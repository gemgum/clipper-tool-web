package setup

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// source = satu alamat unduhan beserta sidik jari isi yang diharapkan.
//
// sha256 WAJIB terisi, dan itulah seluruh gunanya tipe ini. Engine mengunduh
// program lalu MENJALANKANNYA; kalau yang menjaga cuma HTTPS, maka proxy kantor
// yang membongkar TLS, cermin yang diretas, atau rilis yang dibajak cukup untuk
// membuat engine menjalankan biner orang lain di komputer pengguna.
//
// Karena sidik jari wajib, alamat yang isinya bisa berubah sendiri tidak bisa
// dipakai lagi — "rilis terbaru" tanpa nomor versi berarti sidik jarinya berubah
// tanpa kita tahu. Semua sumber di berkas ini menunjuk satu rilis yang dipaku.
type source struct {
	url    string
	sha256 string
}

// recipe = satu unduhan yang menghasilkan biner siap pakai.
//
// Semua isi arsip diratakan ke satu folder (ToolsDir): whisper.cpp membawa
// pustaka bersama di sebelah binernya, dan engine memang menunjuk
// LD_LIBRARY_PATH ke folder biner itu. Struktur folder di dalam arsip tidak
// membawa arti apa pun bagi kita.
type recipe struct {
	// sources dicoba berurutan. Cermin pertama yang menjawab DAN cocok sidik
	// jarinya dipakai.
	//
	// Bukan kemewahan: gyan.dev — sumber ffmpeg Windows yang lazim — adalah
	// satu server kecil yang dari sebagian jaringan tidak terjangkau sama
	// sekali (terbukti: TLS handshake timeout dari Indonesia). Satu alamat mati
	// berarti komponen wajib tidak bisa dipasang, dan pengguna tidak punya
	// jalan lain di dalam aplikasi.
	sources []source
	// kind: "zip" atau "tar.gz". Keduanya ada di pustaka standar Go — itu
	// sebabnya rilis .tar.xz (mis. ffmpeg statis untuk Linux) tidak dipakai:
	// menambah dependensi eksternal hanya untuk membongkarnya melanggar aturan
	// "standard library saja" proyek ini.
	kind string
	// want: nama berkas (tanpa folder) yang diambil. Kosong = ambil semua.
	// Dipakai untuk ffmpeg, yang arsipnya berisi ratusan berkas dokumentasi.
	want []string
}

func (r *recipe) ext() string {
	if r.kind == "zip" {
		return ".zip"
	}
	return ".tar.gz"
}

// wanted melaporkan apakah satu berkas di dalam arsip perlu diambil.
func (r *recipe) wanted(name string) bool {
	base := path.Base(strings.ReplaceAll(name, `\`, "/"))
	if base == "" || strings.HasPrefix(base, ".") {
		return false
	}
	if len(r.want) == 0 {
		return true
	}
	for _, w := range r.want {
		if strings.EqualFold(base, w) {
			return true
		}
	}
	return false
}

// --- resep per komponen & per OS ---
//
// Versinya dipaku, tidak mengikuti "latest": rilis baru bisa mengubah nama
// berkas di dalam arsip, dan yang gagal karenanya adalah pemasangan di mesin
// pengguna — tempat paling buruk untuk menemukan kejutan. Menaikkan versi
// adalah keputusan sadar, bukan efek samping waktu.
const whisperVersion = "v1.9.1"

// Versi ffmpeg yang dipaku di cermin GitHub. gyan.dev sendiri hanya menyediakan
// "release terbaru" tanpa versi di alamatnya, jadi cermin inilah yang membuat
// hasil unduhan bisa diulang persis.
const ffmpegVersion = "9.0"

// Cermin kedua ffmpeg Windows. Bukan build yang sama dengan yang pertama —
// pengunggah yang berbeda memang tidak menerbitkan berkas yang identik — dan
// itu justru maksudnya: cermin yang bisa hilang bersama pengunggah pertama
// tidak menolong siapa pun.
//
// Ditunjuk ke satu tag autobuild, bukan ke tag "latest" yang isinya berganti
// tiap hari: yang berganti isinya tidak bisa dipaku sidik jarinya.
const btbnBuild = "autobuild-2026-08-04-21-26"
const btbnFile = "ffmpeg-n8.1.2-34-g9b6c8969e0-win64-gpl-8.1.zip"

func whisperRecipe() *recipe {
	base := "https://github.com/ggml-org/whisper.cpp/releases/download/" + whisperVersion + "/"
	switch runtime.GOOS {
	case "windows":
		// Berisi whisper-cli.exe + DLL pendampingnya.
		return &recipe{kind: "zip", sources: []source{{
			base + "whisper-bin-x64.zip",
			"7d8be46ecd31828e1eb7a2ecdd0d6b314feafd82163038ab6092594b0a063539",
		}}}
	case "linux":
		if runtime.GOARCH == "arm64" {
			return &recipe{kind: "tar.gz", sources: []source{{
				base + "whisper-bin-ubuntu-arm64.tar.gz",
				"e0b66cd551ff6f2a28fabe3c6e89691eea037bb76833493abb9a71ca788994b3",
			}}}
		}
		return &recipe{kind: "tar.gz", sources: []source{{
			base + "whisper-bin-ubuntu-x64.tar.gz",
			"f3bf3b4369a99b54665b0f19b88483b30de27f25963b0414235dea03198515c5",
		}}}
	}
	// macOS: rilis resminya hanya xcframework, bukan biner baris perintah.
	return nil
}

func whisperSize() string {
	switch runtime.GOOS {
	case "windows":
		return "~8 MB"
	case "linux":
		return "~9 MB"
	}
	return ""
}

func whisperHint() string {
	if runtime.GOOS == "darwin" {
		return "On macOS, install it with: brew install whisper-cpp"
	}
	return "No ready-made build for this system: it has to be compiled from source"
}

func ffmpegRecipe() *recipe {
	switch runtime.GOOS {
	case "windows":
		return &recipe{
			sources: []source{
				// Cermin GitHub dari build gyan.dev — CDN GitHub terjangkau dari
				// mana pun aplikasi ini bisa diunduh, jadi ia didahulukan.
				{
					"https://github.com/GyanD/codexffmpeg/releases/download/" + ffmpegVersion +
						"/ffmpeg-" + ffmpegVersion + "-essentials_build.zip",
					"e6b54767a6065919048f1a098eb27211ca4e12b4348a05d88777a5855d0b6e71",
				},
				// Build pihak lain, kalau yang pertama hilang.
				{
					"https://github.com/BtbN/FFmpeg-Builds/releases/download/" + btbnBuild + "/" + btbnFile,
					"b7f08f5b4975e6ceecb9785584e559cfed0968fae701db92abd968e7a2ae0402",
				},
				// Sumber asli gyan.dev sengaja TIDAK dipakai lagi: alamatnya
				// selalu menunjuk "rilis terbaru" tanpa nomor versi, jadi isinya
				// berganti sendiri dan sidik jarinya mustahil dipaku. Cermin
				// pertama di atas adalah build yang sama, dari pengunggah yang
				// sama, hanya saja bernomor.
			},
			kind: "zip",
			want: []string{"ffmpeg.exe", "ffprobe.exe"},
		}
	}
	// macOS: sumber yang lazim (evermeet.cx) mengemas SATU biner per zip,
	// sedangkan daftar urls di sini berarti "cermin" — dicoba sampai satu
	// berhasil, lalu berhenti. Memakainya untuk dua berkas berbeda akan
	// memasang ffmpeg saja dan meninggalkan ffprobe hilang tanpa pesan apa pun.
	// Lebih jujur menyerahkannya ke Homebrew, yang memasang keduanya sekaligus.
	//
	// Linux: rilis statis resminya .tar.xz, dan xz tidak ada di pustaka standar.
	return nil
}

func ffmpegSize() string {
	switch runtime.GOOS {
	case "windows":
		return "~110 MB"
	}
	return ""
}

func ffmpegHint() string {
	switch runtime.GOOS {
	case "linux":
		return "Install it with your package manager, e.g. sudo apt install ffmpeg"
	case "darwin":
		return "Or install it with: brew install ffmpeg"
	}
	return "Download the release build and put ffmpeg.exe next to the app"
}

// --- unduh ---

// downloadAny mencoba tiap cermin sampai ada yang berhasil.
//
// Galat terakhir yang dilaporkan, bukan yang pertama: yang pertama biasanya
// cermin yang memang sudah lama mati, sedangkan yang terakhir lebih mungkin
// menggambarkan keadaan jaringan pengguna.
func downloadAny(ctx context.Context, srcs []source, dest string, onProgress func(Progress)) error {
	onProgress = orNoop(onProgress)
	var last error
	for i, src := range srcs {
		// Sumber tanpa sidik jari ditolak di sini, bukan diunduh lalu dipercaya.
		// Penjaganya di jalur pemakaian — bukan hanya di uji — supaya sumber baru
		// yang lupa dipaku gagal seketika alih-alih diam-diam melewati verifikasi.
		if src.sha256 == "" {
			last = fmt.Errorf("refusing to download %s: no checksum is pinned for it", src.url)
			continue
		}
		if i > 0 {
			onProgress(Progress{Message: fmt.Sprintf("Trying another source (%d of %d)…", i+1, len(srcs))})
		}
		err := download(ctx, src, dest, onProgress)
		if err == nil {
			return nil
		}
		// Pembatalan oleh pengguna bukan kegagalan cermin — jangan pindah.
		if ctx.Err() != nil {
			return err
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("no download source is configured")
	}
	return last
}

// orNoop membuat pelapor progres selalu aman dipanggil. Penjaganya ada di sini,
// bukan hanya di Install: fungsi unduh dipanggil juga dari tempat lain (dan
// dari uji), dan "tanpa pelapor" adalah pemakaian yang sah — bukan alasan untuk
// panik di tengah unduhan 111 MB.
func orNoop(fn func(Progress)) func(Progress) {
	if fn == nil {
		return func(Progress) {}
	}
	return fn
}

// download menyimpan url ke dest sambil melaporkan kemajuannya.
//
// Ditulis ke berkas .part lalu diganti nama di akhir: berkas setengah jadi yang
// bernama seperti berkas jadi akan dianggap "sudah terpasang" pada percobaan
// berikutnya.
//
// Berkas .part TIDAK dihapus saat gagal — justru itu yang membuat percobaan
// berikutnya bisa meminta sisanya saja lewat header Range. Di sambungan yang
// putus-nyambung, gagal di 100 MB tidak lagi berarti mengulang 111 MB.
//
// Tapi .part hanya boleh dilanjutkan oleh alamat yang MEMBUATNYA. Cermin yang
// berbeda menyajikan build yang berbeda, jadi melanjutkan potongan cermin
// pertama dengan potongan cermin kedua menghasilkan dua berkas berbeda yang
// disambung jadi satu: ukurannya wajar, isinya rusak. Karena itu alamat asalnya
// dicatat di berkas pendamping, dan berpindah cermin berarti mulai dari nol.
func download(ctx context.Context, src source, dest string, onProgress func(Progress)) error {
	onProgress = orNoop(onProgress)
	tmp := dest + ".part"
	from := tmp + ".from"

	// Berapa byte yang sudah ada dari percobaan sebelumnya — dan apakah byte itu
	// memang datang dari alamat yang sedang dicoba sekarang.
	var have int64
	if st, err := os.Stat(tmp); err == nil {
		if prev, err := os.ReadFile(from); err == nil && string(prev) == src.url {
			have = st.Size()
		} else {
			os.Remove(tmp)
			os.Remove(from)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.url, nil)
	if err != nil {
		return err
	}
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
	}
	client := &http.Client{Timeout: 6 * time.Hour} // model besar di sambungan lambat
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer res.Body.Close()

	// 206 = server menerima permintaan lanjutan. 200 = ia mengabaikannya dan
	// mengirim dari awal, jadi berkas lama harus dibuang supaya tidak tersambung
	// jadi berkas rusak yang ukurannya kelihatan benar.
	resume := res.StatusCode == http.StatusPartialContent
	switch {
	case resume:
	case res.StatusCode == http.StatusOK:
		have = 0
	default:
		return fmt.Errorf("download failed: %s said %s", src.url, res.Status)
	}

	flag := os.O_CREATE | os.O_WRONLY
	if resume {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	f, err := os.OpenFile(tmp, flag, 0o644)
	if err != nil {
		return err
	}
	// Ditulis setelah berkasnya benar-benar dibuka: penanda tanpa .part hanya
	// akan membuat percobaan berikutnya mengira ada yang bisa dilanjutkan.
	if err := os.WriteFile(from, []byte(src.url), 0o644); err != nil {
		f.Close()
		return err
	}

	total := res.ContentLength
	if total > 0 {
		total += have // ContentLength hanya menghitung sisanya saat melanjutkan
	}
	pr := &progressReader{r: res.Body, total: total, read: have, onProgress: onProgress}
	_, err = io.Copy(f, pr)
	closeErr := f.Close()
	if err != nil {
		return fmt.Errorf("download interrupted: %w", err)
	}
	if closeErr != nil {
		return closeErr
	}

	// Diperiksa SEBELUM berkasnya bernama seperti berkas jadi: begitu namanya
	// benar, langkah berikutnya menganggapnya terpasang dan menjalankannya.
	onProgress(Progress{Value: -1, Message: "Checking the download…"})
	sum, err := fileSum(tmp)
	if err != nil {
		return err
	}
	if !strings.EqualFold(sum, src.sha256) {
		// Dibuang, bukan disimpan untuk dilanjutkan: isi yang salah tidak akan
		// menjadi benar dengan ditambahi byte berikutnya, dan .part yang tertinggal
		// justru membuat percobaan berikutnya mewarisi kerusakan yang sama.
		os.Remove(tmp)
		os.Remove(from)
		return fmt.Errorf("%s does not match its known checksum: the file was rejected (expected %s, got %s)",
			src.url, src.sha256, sum)
	}

	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	os.Remove(from)
	return nil
}

// fileSum menghitung sha256 sebuah berkas.
//
// Dibaca ulang dari disk, bukan dihitung sambil mengunduh: unduhan yang
// dilanjutkan hanya melewatkan sisanya, jadi hitungan yang berjalan tidak pernah
// melihat bagian yang diunduh percobaan sebelumnya.
func fileSum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// progressReader melaporkan kemajuan salinan, dibatasi sekali per 200 ms supaya
// aliran SSE tidak dibanjiri ribuan pesan yang isinya nyaris sama.
type progressReader struct {
	r          io.Reader
	total      int64
	read       int64
	last       time.Time
	onProgress func(Progress)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if time.Since(p.last) > 200*time.Millisecond || err == io.EOF {
		p.last = time.Now()
		value := -1.0
		if p.total > 0 {
			value = float64(p.read) / float64(p.total)
		}
		p.onProgress(Progress{
			Value:   value,
			Message: fmt.Sprintf("%s of %s", human(p.read), human(p.total)),
			Bytes:   p.read,
			Total:   p.total,
		})
	}
	return n, err
}

func human(b int64) string {
	switch {
	case b <= 0:
		return "?"
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(b)/(1<<20))
	default:
		return fmt.Sprintf("%.0f KB", float64(b)/(1<<10))
	}
}

// --- bongkar ---

// extract membongkar arsip ke dir dan mengembalikan jumlah berkas yang diambil.
func (r *recipe) extract(archivePath, dir string) (int, error) {
	if r.kind == "zip" {
		return r.extractZip(archivePath, dir)
	}
	return r.extractTarGz(archivePath, dir)
}

func (r *recipe) extractZip(archivePath, dir string) (int, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0, err
	}
	defer zr.Close()
	n := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !r.wanted(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return n, err
		}
		err = writeFile(filepath.Join(dir, path.Base(f.Name)), rc, f.FileInfo().Mode())
		rc.Close()
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (r *recipe) extractTarGz(archivePath, dir string) (int, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	n := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return n, err
		}
		if !r.wanted(h.Name) {
			continue
		}
		target := filepath.Join(dir, path.Base(h.Name))
		switch h.Typeflag {
		case tar.TypeReg:
			if err := writeFile(target, tr, os.FileMode(h.Mode)); err != nil {
				return n, err
			}
			n++
		case tar.TypeSymlink:
			// Pustaka whisper datang sebagai rantai symlink
			// (libwhisper.so → libwhisper.so.1 → libwhisper.so.1.9.1). Karena
			// semuanya diratakan ke satu folder, tujuannya cukup nama berkasnya.
			os.Remove(target)
			if err := os.Symlink(path.Base(h.Linkname), target); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

// writeFile menulis satu berkas hasil bongkaran, menimpa yang lama.
//
// Berkas lama dihapus lebih dulu, tidak sekadar ditimpa: menimpa biner yang
// sedang berjalan gagal di Windows, dan menghapus-lalu-membuat memberi pesan
// yang lebih jelas ketimbang "access denied" di tengah penyalinan.
func writeFile(dest string, src io.Reader, mode os.FileMode) error {
	os.Remove(dest)
	if mode == 0 {
		mode = 0o644
	}
	// Biner harus bisa dieksekusi. Rilis Windows tidak membawa bit itu, dan
	// tanpa ini whisper-cli hasil unduhan tidak bisa dijalankan sama sekali.
	if isExecutableName(dest) {
		mode |= 0o755
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, src); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// isExecutableName menebak apakah sebuah berkas hasil bongkaran adalah program
// atau pustaka — keduanya perlu bit x di Unix.
func isExecutableName(p string) bool {
	base := strings.ToLower(filepath.Base(p))
	if strings.HasSuffix(base, ".exe") || strings.HasSuffix(base, ".dll") {
		return true
	}
	if strings.Contains(base, ".so") || strings.HasSuffix(base, ".dylib") {
		return true
	}
	return !strings.Contains(base, ".") // biner Unix umumnya tanpa akhiran
}
