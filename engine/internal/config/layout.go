package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// AppDirName = nama folder engine di dalam folder data pengguna.
const AppDirName = "Clipper"

// Layout adalah peta folder engine: mana yang ditulis, mana yang cukup dibaca.
//
// Sampai kini semuanya menempel di sebelah biner (root/data, root/models,
// root/bin). Itu jalan selama engine dijalankan dari checkout, tapi mustahil
// untuk aplikasi terpasang: "Program Files" dan "/Applications" hanya-baca.
// Karena itu ada dua bentuk, dipilih Locate tanpa perlu dikonfigurasi:
//
//	checkout  → semua di dalam repo, persis seperti sebelumnya, jadi model dan
//	            cache yang sudah terunduh tetap terpakai;
//	terpasang → yang ditulis pindah ke folder data per pengguna; hanya font
//	            bawaan yang tetap di sebelah biner sebab ia memang tak berubah.
//
// Pemisahan ModelsDir/ToolsDir dari DataDir bukan kerapian belaka: keduanya
// isinya barang unduhan besar yang boleh dibuang tanpa kehilangan hasil kerja,
// dan halaman Requirements nanti perlu tempat pasti untuk menaruhnya.
type Layout struct {
	Root      string // folder aplikasi (akar repo saat checkout)
	DataDir   string // job, cache, kartu, unggahan — WAJIB bisa ditulis
	ModelsDir string // model whisper (diunduh, ratusan MB)
	ToolsDir  string // biner yang disediakan aplikasi: whisper-cli, ffmpeg
	FontsDir  string // font bawaan — cukup dibaca
	GUIDir    string // halaman GUI hasil ekspor statis — cukup dibaca
	EnvFile   string // tempat menyimpan ANTHROPIC_API_KEY

	// Tempat hasil kerja pengguna disimpan. KOSONG = di dalam DataDir.
	//
	// Dipisah dari DataDir karena isinya berbeda sifat: DataDir milik aplikasi
	// (cache, berkas sementara, unggahan) dan boleh dihapus kapan saja,
	// sedangkan klip dan kartu adalah milik pengguna. Ia wajar ingin
	// menaruhnya di Videos, di Drive yang tersinkron, atau di disk lain yang
	// muat — bukan terkubur di dalam AppData.
	ClipsDir string // klip hasil render; kosong = <DataDir>/<job>
	CardsDir string // kartu berita; kosong = <DataDir>/cards
	Dev      bool   // true bila jalan dari checkout sumber
}

// Locate menentukan layout untuk root yang diberikan.
//
// Penanda checkout adalah engine/go.mod: berkas itu hanya ada di pohon sumber,
// tidak pernah ikut terpasang. Sengaja bukan flag atau env — kalau pemilihannya
// harus diminta, mode terpasang akan selalu salah untuk pengguna yang tidak tahu
// harus memintanya.
func Locate(root string) Layout {
	l := Layout{Root: root, Dev: isSourceCheckout(root)}
	base := ""
	if !l.Dev {
		base = userDataDir()
	}
	if base == "" {
		// Checkout, atau OS tanpa folder home yang bisa dibaca. Kembali ke
		// perilaku lama: semuanya relatif root.
		l.DataDir = filepath.Join(root, "data")
		l.ModelsDir = filepath.Join(root, "models")
		l.ToolsDir = filepath.Join(root, "bin")
		l.EnvFile = filepath.Join(root, ".env")
	} else {
		l.DataDir = filepath.Join(base, "data")
		l.ModelsDir = filepath.Join(base, "models")
		l.ToolsDir = filepath.Join(base, "bin")
		l.EnvFile = filepath.Join(base, ".env")
	}
	l.FontsDir = fontsDir(root)
	l.GUIDir = guiDir(root)

	// Env menimpa apa pun: dipakai uji, dan jalan keluar bila pengguna ingin
	// menaruh model di disk lain.
	l.DataDir = env("CLIPPER_DATA_DIR", l.DataDir)
	l.ModelsDir = env("CLIPPER_MODELS_DIR", l.ModelsDir)
	l.ToolsDir = env("CLIPPER_TOOLS_DIR", l.ToolsDir)
	l.FontsDir = env("CLIPPER_FONTS_DIR", l.FontsDir)
	l.GUIDir = env("CLIPPER_GUI_DIR", l.GUIDir)
	l.EnvFile = env("CLIPPER_ENV_FILE", l.EnvFile)
	// Tanpa nilai bawaan: kosong berarti "ikut DataDir", dan itu memang
	// keadaan awal yang benar sampai pengguna memilih sendiri.
	l.ClipsDir = os.Getenv("CLIPPER_CLIPS_DIR")
	l.CardsDir = os.Getenv("CLIPPER_CARDS_DIR")
	return l
}

// Ensure membuat folder yang akan ditulis engine.
//
// Dipanggil di awal, bukan saat berkas pertama ditulis: kegagalan izin di
// folder data harus muncul sebagai pesan saat start, bukan sebagai job yang
// gagal di tengah setelah pengguna menunggu setengah jam.
func (l Layout) Ensure() error {
	for _, d := range []string{l.DataDir, l.ModelsDir, l.ToolsDir, l.ClipsDir, l.CardsDir} {
		if d == "" {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("cannot create the folder %s: %w", d, err)
		}
	}
	return nil
}

// ClipsRoot mengembalikan tempat klip disimpan.
func (l Layout) ClipsRoot() string {
	if l.ClipsDir != "" {
		return l.ClipsDir
	}
	return l.DataDir
}

// CardsRoot mengembalikan tempat kartu disimpan.
func (l Layout) CardsRoot() string {
	if l.CardsDir != "" {
		return l.CardsDir
	}
	return filepath.Join(l.DataDir, "cards")
}

// isSourceCheckout melaporkan apakah root adalah pohon sumber clipper.
func isSourceCheckout(root string) bool {
	if root == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(root, "engine", "go.mod"))
	return err == nil
}

// userDataDir mengembalikan folder data per pengguna menurut kebiasaan OS,
// atau "" bila folder home tidak terbaca.
func userDataDir() string {
	switch runtime.GOOS {
	case "windows":
		// LOCALAPPDATA, bukan APPDATA: isinya model ratusan MB sampai beberapa
		// GB, dan folder roaming ikut disalin antar mesin di jaringan kantor.
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, AppDirName)
		}
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, "AppData", "Local", AppDirName)
		}
	case "darwin":
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, "Library", "Application Support", AppDirName)
		}
	default:
		// XDG: data (bukan cache/config) — model dan klip bukan berkas sekali
		// pakai, dan pengguna tidak menyuntingnya dengan tangan.
		if v := os.Getenv("XDG_DATA_HOME"); v != "" {
			return filepath.Join(v, "clipper")
		}
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, ".local", "share", "clipper")
		}
	}
	return ""
}

// fontsDir mencari font bawaan. Ia dibundel bersama biner, jadi dicari dari
// letak biner lebih dulu; root hanya jadi cadangan terakhir (kasus checkout,
// dan saat engine dijalankan lewat "go run" yang binernya ada di folder sesaat).
func fontsDir(root string) string {
	var cands []string
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		dir := filepath.Dir(exe)
		cands = append(cands,
			filepath.Join(dir, "assets", "fonts"),
			filepath.Join(dir, "..", "assets", "fonts"),
			// Bundel .app macOS: Contents/MacOS/clipper → Contents/Resources.
			filepath.Join(dir, "..", "Resources", "assets", "fonts"),
		)
	}
	cands = append(cands, filepath.Join(root, "assets", "fonts"))
	return firstExisting(cands...)
}

// guiDir mencari halaman GUI hasil ekspor statis (gui/out).
//
// Aplikasi desktop menyajikan GUI-nya sendiri: menuntut Node.js dan
// "npm run dev" di komputer pengguna jelas tidak masuk akal, dan seluruh GUI
// ini memang berjalan di browser. Dicari di sebelah biner lebih dulu (di situ
// letaknya setelah dikemas), lalu di gui/out untuk checkout sumber.
func guiDir(root string) string {
	var cands []string
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		dir := filepath.Dir(exe)
		cands = append(cands,
			filepath.Join(dir, "gui"),
			filepath.Join(dir, "..", "gui"),
			filepath.Join(dir, "..", "Resources", "gui"),
		)
	}
	cands = append(cands, filepath.Join(root, "gui", "out"))
	for _, c := range cands {
		// index.html, bukan sekadar foldernya: di checkout, "gui" adalah kode
		// sumber GUI, dan itu bukan yang boleh disajikan.
		if _, err := os.Stat(filepath.Join(c, "index.html")); err == nil {
			return c
		}
	}
	return ""
}

// exeName menambahkan akhiran .exe di Windows.
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
