// Package setup memeriksa dan memasang komponen yang dibutuhkan engine:
// ffmpeg, whisper.cpp, dan model whisper.
//
// Sampai kini pemasangan adalah pekerjaan pengembang — setup.sh, cmake, g++.
// Tidak satu pun boleh dituntut dari pengguna aplikasi desktop, jadi paket ini
// mengerjakannya: unduh rilis biner resmi, bongkar ke ToolsDir/ModelsDir, dan
// laporkan kemajuannya selagi berjalan.
//
// Yang TIDAK dipasang di sini: Ollama dan Chrome. Keduanya aplikasi utuh milik
// orang lain dengan pemasangnya sendiri; yang bisa dilakukan hanyalah mendeteksi
// keberadaannya dan menunjukkan halaman unduhnya.
package setup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/capture"
	"github.com/gemgum/clipper/engine/internal/config"
)

// Kind komponen — menentukan bentuk barisnya di halaman Requirements.
const (
	KindTool  = "tool"  // biner yang dipasang engine
	KindModel = "model" // model whisper
	KindApp   = "app"   // aplikasi terpisah, hanya dideteksi
)

// Component adalah satu baris di halaman Requirements.
type Component struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Required    bool   `json:"required"`
	Installed   bool   `json:"installed"`
	Path        string `json:"path"`   // di mana ia ditemukan
	Detail      string `json:"detail"` // keterangan singkat untuk pengguna
	Size        string `json:"size"`   // perkiraan besar unduhan
	Installable bool   `json:"installable"`
	Hint        string `json:"hint"` // cara memasang manual, bila engine tak bisa
	URL         string `json:"url"`  // halaman unduh untuk pemasangan manual
	// Pointable = boleh ditunjuk ke berkas pilihan pengguna.
	//
	// Ada karena GUI tidak boleh menebaknya sendiri: Ollama TERPASANG sebagai
	// layanan jaringan, bukan berkas yang bisa dipilih, dan tombol "pakai berkas
	// lain" di barisnya membuka pemilih video — dialog yang tidak berarti apa
	// pun di sana (dilaporkan dari lapangan).
	Pointable bool `json:"pointable"`
}

// modelRevision = commit repo HuggingFace tempat model diambil.
//
// Bukan "main": cabang bisa dipindahkan kapan saja oleh pemilik repo, dan berkas
// yang isinya boleh berubah tidak bisa dipaku sidik jarinya. Menaikkan angka ini
// berarti memeriksa ulang sha256 di bawah — keduanya satu paket.
const modelRevision = "5359861c739e955e79d9a303bcbc70fb988958b1"

// Model whisper yang dikenal, beserta perkiraan ukuran & sidik jarinya.
// Satu-satunya daftar model di engine — /api/models ikut membacanya dari sini.
var Models = []struct {
	Name   string
	Size   string
	SHA256 string
}{
	{"tiny", "~75 MB", "be07e048e1e599ad46341c8d2a135645097a538221678b7acdd1b1919c6e1b21"},
	{"base", "~142 MB", "60ed5bc3dd14eea856493d334349b405782ddcaf0028d4b5df4088345fba2efe"},
	{"small", "~466 MB", "1be3a9b2063867b937e64e2ec7483364a79917e157fa98c5d94b5c1fffea987b"},
	{"medium", "~1.5 GB", "6c14d5adee5f86394037b4e4e8b59f1673b6cee10e3cf0b11bbdbee79c156208"},
	{"large-v3", "~2.9 GB", "64d182b440b98d5203c4f9bd541544d84c605196c4f7b845dfa11fb23594d1e2"},
	{"large-v3-turbo", "~1.5 GB", "1fc70f774d38eb169993ac391eea357ef47c88757ef72ee5943879b7e8e2bc69"},
}

// ModelID membentuk id komponen untuk sebuah model.
func ModelID(name string) string { return "model:" + name }

// ModelPath mengembalikan letak berkas model di dalam layout.
func ModelPath(l config.Layout, name string) string {
	return filepath.Join(l.ModelsDir, "ggml-"+name+".bin")
}

// HasModel melaporkan apakah sebuah model sudah terunduh.
func HasModel(l config.Layout, name string) bool {
	_, err := os.Stat(ModelPath(l, name))
	return err == nil
}

// RemoveModel menghapus satu model yang sudah diunduh, BESERTA sisa unduhannya.
//
// Berkas `.part` dan `.part.from` ikut dibuang, dan itu bukan kerapian: unduhan
// yang terputus meninggalkan `.part` sebesar apa yang sempat turun — sampai
// 2,9 GB untuk large-v3 — dan sebelum ini tombol Remove tidak menyentuhnya sama
// sekali. Pengguna menekan Remove, melihat barisnya jadi "missing", dan ruang
// disknya tidak kembali. Dilaporkan dari lapangan sebagai "hapusnya tidak
// bersih".
func RemoveModel(l config.Layout, name string) error {
	for _, m := range Models {
		if m.Name == name {
			p := ModelPath(l, name)
			for _, f := range []string{p, p + ".part", p + ".part.from"} {
				if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
			return nil
		}
	}
	return fmt.Errorf("unknown whisper model %q", name)
}

// LLMServers = server LLM lokal yang boleh dipakai engine ini.
//
// Daftarnya TERTUTUP dan disusun tangan. Yang belum diuji DIKATAKAN belum
// diuji, di barisnya sendiri — daftar yang menyiratkan "semua ini beres"
// padahal separuhnya tebakan lebih buruk daripada daftar pendek yang jujur.
//
// Keadaan per 6 Agustus 2026, diuji dengan Qwen2.5 3B Q4_K_M:
//
//   - Ollama      — dipakai sejak awal; `format` benar-benar memaksa bentuk JSON.
//   - llama.cpp   — DIUJI (llama-server b10295). /api/tags 404 → dikenali
//     sebagai KindOpenAI, `response_format: json_schema` + `strict` dihormati,
//     balasannya persis sesuai skema, dan `meta` memberi n_ctx/n_params.
//   - KoboldCpp   — DIUJI. Ia MENIRU /api/tags Ollama, jadi engine memakainya
//     lewat /api/chat dan itu berhasil — TAPI `format` diabaikan: diminta
//     {"picks":[…]} ia menjawab {"moment2":45}. Bentuk balasannya bergantung
//     pada model yang menuruti prompt, bukan pada pagar server.
//   - LM Studio, Jan, GPT4All — BELUM diuji. Ketiganya butuh jendela untuk
//     menyalakan servernya, jadi tidak bisa diuji dari terminal.
//
// Tidak satu pun dipasang engine: semuanya aplikasi berdiri sendiri, jadi
// barisnya menunjuk ke halaman unduhannya — pola yang sama dengan baris Chrome
// (notes/25: yang dipasang engine punya resep, yang tidak punya tautan).
//
// Nama di sini HARUS sama dengan yang dikembalikan ollama.OS/serverName, sebab
// itulah cara baris yang sedang berjalan dikenali.
var LLMServers = []struct{ ID, Name, Detail, Hint, URL string }{
	{"ollama", "Ollama",
		"Local LLM. Manages its own models: the simplest place to start.",
		"Install it, start it, then pull a model. llama3.1 is the best pick for the term list.",
		"https://ollama.com/download"},
	{"lmstudio", "LM Studio",
		"Local LLM with a window and a built-in model browser. Port 1234. Not tested with Clipper yet.",
		"Install it, download a model inside the app, then Developer → Start Server.",
		"https://lmstudio.ai/download"},
	{"jan", "Jan",
		"Local LLM with a window and its own model hub. Port 1337. Not tested with Clipper yet.",
		"Install it, download a model, then Settings → Local API Server.",
		"https://jan.ai/download"},
	{"llamacpp", "llama.cpp",
		"What the others are built on. Port 8080. Tested: keeps JSON replies in shape.",
		"Get llama-server plus a .gguf model, then run it with --port 8080.",
		"https://github.com/ggml-org/llama.cpp/releases"},
	{"koboldcpp", "KoboldCpp",
		"One file, no install. Port 5001. Tested: works, but does not enforce the JSON shape.",
		"Download it, make it runnable, then run with --model and --port 5001.",
		"https://github.com/LostRuins/koboldcpp/releases"},
	{"gpt4all", "GPT4All",
		"Local LLM with a window and a curated model list. Port 4891. Not tested with Clipper yet.",
		"Install it, download a model, then turn on the Local API Server in Settings.",
		"https://gpt4all.io"},
}

// Status melaporkan keadaan semua komponen.
//
// Urutannya sengaja: yang wajib dulu, lalu model, lalu yang opsional. Halaman
// Requirements menampilkannya apa adanya — pengguna membaca dari atas, dan yang
// paling menghalanginya harus muncul lebih dulu.
func Status(l config.Layout) []Component {
	out := []Component{
		toolStatus(l, "ffmpeg", "ffmpeg", "Cuts and renders video. Required."),
		toolStatus(l, "ffprobe", "ffprobe", "Reads video length and size. Comes with ffmpeg."),
		whisperStatus(l),
	}
	for _, m := range Models {
		p := ModelPath(l, m.Name)
		_, err := os.Stat(p)
		c := Component{
			// Nama berkasnya, sama seperti baris lain — juga sebelum diunduh,
			// supaya satu daftar tidak memakai dua gaya penamaan sekaligus.
			ID: ModelID(m.Name), Name: filepath.Base(p), Kind: KindModel,
			Size: m.Size, Installable: true,
			Detail: "Speech recognition. " + modelNote(m.Name),
		}
		if err == nil {
			c.Installed, c.Path = true, p
		}
		out = append(out, c)
	}

	// Semua server LLM yang bisa dipakai, bukan cuma Ollama. Baris pertama yang
	// TERDETEKSI diberi tanda oleh pemanggil (lihat api/requirements.go).
	for _, srv := range LLMServers {
		c := Component{
			ID: "llm:" + srv.ID, Name: srv.Name, Kind: KindApp,
			Detail: srv.Detail, Hint: srv.Hint, URL: srv.URL,
			Installable: false,
		}
		out = append(out, c)
	}

	chrome := Component{
		ID: "chrome", Name: "Chrome / Edge", Kind: KindApp, Pointable: true,
		Detail:      "Renders the news cards. Not needed for video clips.",
		Hint:        "Install Chrome or Chromium: the Edge that ships with Windows works too.",
		URL:         "https://www.google.com/chrome/",
		Installable: false,
	}
	if p := capture.Find(); p != "" {
		chrome.Installed, chrome.Path = true, p
	}
	out = append(out, chrome)

	// NAMA BARIS = BERKAS YANG BENAR-BENAR DIPAKAI, bukan nama proyeknya.
	//
	// "whisper.cpp" adalah nama proyek; yang dijalankan engine bernama
	// `whisper-cli.exe`, dan itu yang harus dicari pengguna kalau ia mau
	// memeriksanya sendiri atau mengizinkannya di antivirus. Nama proyek yang
	// tidak cocok dengan nama berkas di baris di bawahnya membuat halaman ini
	// terbaca seperti menyebut dua hal berbeda.
	for i := range out {
		if out[i].Installed && out[i].Path != "" {
			out[i].Name = filepath.Base(out[i].Path)
		}
	}

	return out
}

// modelNote menjelaskan untuk apa tiap ukuran model, dalam kalimat pendek.
func modelNote(name string) string {
	switch name {
	case "tiny", "base":
		return "Fast, for quick tests."
	case "small":
		return "The default: the balance this project is tuned for."
	default:
		return "More accurate, much slower on a CPU."
	}
}

// toolStatus memeriksa satu biner: yang dipasang engine lebih dulu, baru PATH.
func toolStatus(l config.Layout, id, bin, detail string) Component {
	c := Component{
		ID: id, Name: bin, Kind: KindTool, Required: true, Detail: detail, Pointable: true,
		Size: ffmpegSize(), Installable: ffmpegRecipe() != nil,
		Hint: ffmpegHint(), URL: "https://ffmpeg.org/download.html",
	}
	local := filepath.Join(l.ToolsDir, exeName(bin))
	if _, err := os.Stat(local); err == nil {
		c.Installed, c.Path = true, local
		return checkRuns(c)
	}
	if p, err := exec.LookPath(bin); err == nil {
		c.Installed, c.Path = true, p
		c.Detail = detail + " Found on this system."
		return checkRuns(c)
	}
	return c
}

// checkRuns BENAR-BENAR MENJALANKAN binernya, bukan sekadar memeriksa berkasnya
// ada. Keduanya dulu dilaporkan sama, dan itu menyesatkan: di Windows berkas
// bisa ada tapi tetap gagal jalan (arsitektur salah, DLL kurang, dibekukan
// Defender). Pengguna melihat "terpasang" berwarna hijau sementara tiap job
// berhenti dengan galat ffmpeg — dan tidak ada satu pun tempat yang memberitahu
// apa galatnya (notes/31).
//
// Yang gagal dilaporkan TIDAK terpasang, dengan pesan sistemnya apa adanya di
// Detail. Itu satu-satunya cara pesan aslinya sampai ke pengguna.
func checkRuns(c Component) Component {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, c.Path, "-version").CombinedOutput()
	if err == nil {
		return c
	}
	msg := strings.TrimSpace(string(out))
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	c.Installed = false
	c.Detail = fmt.Sprintf("%s is at %s but will not run: %v", c.Name, c.Path, err)
	if msg != "" {
		c.Detail += ": " + msg
	}
	return c
}

// whisperStatus memeriksa whisper-cli.
func whisperStatus(l config.Layout) Component {
	c := Component{
		ID: "whisper", Name: "whisper.cpp", Kind: KindTool, Required: true, Pointable: true,
		Detail:      "Turns speech into a timed transcript.",
		Size:        whisperSize(),
		Installable: whisperRecipe() != nil,
		Hint:        whisperHint(),
		URL:         "https://github.com/ggml-org/whisper.cpp/releases",
	}
	for _, cand := range []string{exeName("whisper-cli"), exeName("main")} {
		p := filepath.Join(l.ToolsDir, cand)
		if _, err := os.Stat(p); err == nil {
			c.Installed, c.Path = true, p
			return c
		}
	}
	if p, err := exec.LookPath("whisper-cli"); err == nil {
		c.Installed, c.Path = true, p
		c.Detail += " Found on this system."
	}
	return c
}

// Missing melaporkan komponen wajib yang belum ada. Dipakai untuk menjawab
// "kenapa tombol mulai tidak bisa ditekan" sebelum pengguna menekannya.
func Missing(cs []Component) []string {
	// Bukan nil: slice nil menjadi "null" di JSON, dan klien yang menghitung
	// panjangnya berhenti dengan galat alih-alih membaca "tidak ada yang kurang".
	out := []string{}
	for _, c := range cs {
		if c.Required && !c.Installed {
			out = append(out, c.Name)
		}
	}
	return out
}

// Progress dilaporkan selama pemasangan.
type Progress struct {
	Value   float64 `json:"value"` // 0..1, -1 bila besarnya tidak diketahui
	Message string  `json:"message"`
	Bytes   int64   `json:"bytes"`
	Total   int64   `json:"total"`
}

// Install memasang satu komponen. Kemajuannya dilaporkan lewat onProgress.
//
// Tidak ada langkah "coba yang lain kalau gagal": bila unduhan gagal, pesannya
// disampaikan apa adanya. Diam-diam memasang versi lain adalah persis jenis
// penggantian senyap yang dilarang di notes/12.
func Install(ctx context.Context, l config.Layout, id string, onProgress func(Progress)) error {
	if onProgress == nil {
		onProgress = func(Progress) {}
	}
	if name, ok := strings.CutPrefix(id, "model:"); ok {
		return installModel(ctx, l, name, onProgress)
	}
	switch id {
	// Satu unduhan menghasilkan keduanya — memasang "ffprobe" berarti memasang
	// paket yang sama.
	case "ffmpeg", "ffprobe":
		return installRecipe(ctx, l, ffmpegRecipe(), "ffmpeg", ffmpegHint(), onProgress)
	case "whisper":
		return installRecipe(ctx, l, whisperRecipe(), "whisper.cpp", whisperHint(), onProgress)
	case "ollama", "chrome":
		return fmt.Errorf("%s is a separate application: install it yourself, then press Refresh", id)
	}
	return fmt.Errorf("unknown component %q", id)
}

// installModel mengunduh satu berkas model dari HuggingFace.
func installModel(ctx context.Context, l config.Layout, name string, onProgress func(Progress)) error {
	sum := ""
	for _, m := range Models {
		if m.Name == name {
			sum = m.SHA256
			break
		}
	}
	if sum == "" {
		return fmt.Errorf("unknown whisper model %q", name)
	}
	if err := os.MkdirAll(l.ModelsDir, 0o755); err != nil {
		return err
	}
	dest := ModelPath(l, name)
	// Dua cermin: HuggingFace resmi, lalu cermin komunitas. Model "small" 466 MB
	// dan "large-v3" 2,9 GB — unduhan sebesar itu di sambungan yang tidak
	// stabil butuh lebih dari satu pintu.
	//
	// Sidik jarinya satu untuk keduanya: cermin yang menyajikan isi berbeda dari
	// aslinya bukan cermin, dan justru itu yang ingin ketahuan di sini.
	tail := "/ggerganov/whisper.cpp/resolve/" + modelRevision + "/ggml-" + name + ".bin"
	srcs := []source{
		{"https://huggingface.co" + tail, sum},
		{"https://hf-mirror.com" + tail, sum},
	}
	onProgress(Progress{Message: "Downloading model " + name + "…"})
	if err := downloadAny(ctx, srcs, dest, onProgress); err != nil {
		return err
	}
	onProgress(Progress{Value: 1, Message: "Model " + name + " is ready."})
	return nil
}

// installRecipe menjalankan satu resep unduh+bongkar.
func installRecipe(ctx context.Context, l config.Layout, r *recipe, what, hint string, onProgress func(Progress)) error {
	if r == nil {
		return fmt.Errorf("this build cannot install %s automatically on %s: %s", what, runtime.GOOS, hint)
	}
	if err := os.MkdirAll(l.ToolsDir, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "clipper-install-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	archive := filepath.Join(tmp, "download"+r.ext())
	onProgress(Progress{Message: "Downloading " + what + "…"})
	if err := downloadAny(ctx, r.sources, archive, onProgress); err != nil {
		return err
	}
	onProgress(Progress{Value: 1, Message: "Unpacking " + what + "…"})
	n, err := r.extract(archive, l.ToolsDir)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("the %s download did not contain the expected files: the release layout may have changed", what)
	}
	onProgress(Progress{Value: 1, Message: fmt.Sprintf("%s is ready (%d files).", what, n)})
	return nil
}

// exeName menambahkan akhiran .exe di Windows.
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
