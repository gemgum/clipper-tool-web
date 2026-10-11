package caption

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/correct"
	"github.com/gemgum/clipper/engine/internal/subtitle"
	"github.com/gemgum/clipper/engine/internal/types"
	"github.com/gemgum/clipper/engine/internal/writer"
)

// DefaultMaxSeconds = bagian video yang ditranskripsi bila tidak diatur.
//
// Lima menit. Ini pagar biaya, bukan aturan mutu: caption dibuat untuk klip
// pendek, dan tanpa batas satu berkas salah pilih (podcast 3 jam) menahan
// seluruh antrian bulk di belakangnya. Video yang lebih panjang TIDAK ditolak —
// ia dipotong, dan potongannya disebut di baris pertama berkas hasil.
const DefaultMaxSeconds = 300

// Speech = ucapan satu video, sebatas yang dibutuhkan caption.
//
// Yang dibawa transkrip BERSEGMEN, bukan teks jadi: tahap koreksi bekerja per
// segmen, dan meratakannya lebih dulu berarti membuang satu-satunya pegangan
// yang dipakai pagar-pagarnya.
type Speech struct {
	Transcript types.Transcript
	VideoSec   float64 // durasi video penuh; 0 bila tidak diketahui
	UsedSec    float64 // bagian yang benar-benar ditranskripsi
	Cached     bool    // transkrip datang dari cache
}

// Text meratakan transkrip jadi ucapan tanpa waktu, satu kalimat per baris —
// bentuk yang sama dengan berkas .txt pendamping tiap klip.
func (s Speech) Text() string { return subtitle.Text(s.Transcript.Segments, 0) }

// Transcriber menghasilkan ucapan satu video, dibatasi maxSec detik pertama.
//
// Disuntikkan, bukan diimpor: dengan begitu paket ini tidak perlu mengenal
// whisper maupun ffmpeg, dan bisa diuji tanpa keduanya terpasang.
type Transcriber func(ctx context.Context, video string, maxSec float64) (Speech, error)

// Options setelan satu job caption.
type Options struct {
	// Videos = daftar berkas. Satu berkas untuk single, banyak untuk bulk —
	// tidak ada dua jalan berbeda, cuma panjang daftar yang berbeda.
	Videos []string
	// MaxSeconds membatasi bagian video yang ditranskripsi. 0 = DefaultMaxSeconds.
	MaxSeconds float64
	Lang       string // bahasa caption: id (bawaan) | en
	// Fix menyalakan koreksi transkrip sebelum caption ditulis. Bawaannya
	// menyala, dan itu bukan kemewahan: keluaran mentah whisper untuk percakapan
	// Indonesia penuh salah dengar ("roko", "juster", "Aikos"), dan caption yang
	// ditulis dari teks itu MENYALIN salah dengarnya — termasuk ke tagar.
	// Dimatikan hanya bila mesinnya memang tidak terjangkau.
	Fix      *bool
	Terms    []string // ejaan baku nama & istilah, sama seperti halaman klip
	Variants int      // jumlah caption per video; 0 = DefaultVariants
	MaxWords int      // jatah kata transkrip ke model; 0 = DefaultMaxWords
	// OutDir = folder berkas .txt. KOSONG (bawaan) berarti di sebelah tiap
	// videonya — di situlah orang mencarinya, dan folder aplikasi adalah tempat
	// terakhir yang akan dibuka orang saat hendak memposting.
	OutDir     string
	EngineName string // nama mesin, untuk pesan galat & kaki berkas
}

func (o Options) withDefaults() Options {
	if o.Variants <= 0 {
		o.Variants = DefaultVariants
	}
	if o.MaxWords <= 0 {
		o.MaxWords = DefaultMaxWords
	}
	if o.MaxSeconds <= 0 {
		o.MaxSeconds = DefaultMaxSeconds
	}
	if o.EngineName == "" {
		o.EngineName = "the engine"
	}
	return o
}

// fixOn melaporkan apakah koreksi transkrip menyala (bawaan: ya).
func (o Options) fixOn() bool { return o.Fix == nil || *o.Fix }

// Deps = barang dari luar yang dibutuhkan job ini.
type Deps struct {
	Complete   writer.Completer
	Engine     string
	Transcribe Transcriber
}

// Progress kabar kemajuan. Bentuknya mengikuti pipeline klip & pembuat berita
// supaya lapisan job & SSE memperlakukan ketiganya sama.
type Progress struct {
	Stage   string  `json:"stage"` // transcribing | writing | saving | done
	Value   float64 `json:"value"`
	Message string  `json:"message"`
}

// FileResult hasil untuk satu video.
type FileResult struct {
	Video    string    `json:"video"`
	Name     string    `json:"name"`
	TXT      string    `json:"txt,omitempty"`
	Variants []Variant `json:"variants,omitempty"`
	// Transcript = ucapan yang dipakai menulis caption (sudah dikoreksi bila
	// koreksi menyala), untuk tombol "Lihat transkrip" (DESIGN-Clipper-Lanjutan
	// §5). Berkas .txt di sebelah video berisi CAPTION-nya, bukan ini.
	Transcript string  `json:"transcript,omitempty"`
	VideoSec   float64 `json:"video_seconds,omitempty"`
	UsedSec    float64 `json:"used_seconds,omitempty"`
	// Error = kegagalan video INI saja. Satu berkas rusak di tengah antrian 30
	// berkas tidak boleh membuang 29 yang lain.
	Error string `json:"error,omitempty"`
}

// Result hasil satu job.
type Result struct {
	Files  []FileResult `json:"files"`
	Engine string       `json:"engine"`
}

// Done menghitung berkas yang berhasil.
func (r Result) Done() int {
	n := 0
	for _, f := range r.Files {
		if f.Error == "" {
			n++
		}
	}
	return n
}

// Run mentranskripsi tiap video lalu menuliskan captionnya ke <nama video>.txt.
//
// Satu video memanggil whisper lalu LLM, dan pada model lokal itu hitungan
// menit — jadi pemanggilnya WAJIB menjalankannya di latar dengan
// context.Background(), bukan di dalam context permintaan HTTP (notes/10).
func Run(ctx context.Context, opts Options, deps Deps, onProgress func(Progress)) (Result, error) {
	opts = opts.withDefaults()
	if len(opts.Videos) == 0 {
		return Result{}, fmt.Errorf("no video was given")
	}
	if deps.Transcribe == nil || deps.Complete == nil {
		return Result{}, fmt.Errorf("caption needs both a transcriber and an engine")
	}
	// Folder tujuan dibuat HANYA bila disebut. Kosong berarti di sebelah
	// videonya, dan folder itu sudah pasti ada — videonya baru saja dibaca dari
	// sana.
	if opts.OutDir != "" {
		if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
			return Result{}, fmt.Errorf("output folder %q: %w", opts.OutDir, err)
		}
	}
	opts.EngineName = firstNonEmpty(deps.Engine, opts.EngineName)

	res := Result{Engine: deps.Engine}
	// Berkas yang sudah ditulis DALAM job ini. Dua video bernama sama yang
	// dikumpulkan ke satu folder keluaran kalau tidak akan saling menimpa
	// diam-diam; job yang diulang tetap menimpa hasilnya sendiri, dan itu memang
	// yang diharapkan.
	used := map[string]bool{}

	for i, video := range opts.Videos {
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		default:
		}
		base := float64(i) / float64(len(opts.Videos))
		step := 1 / float64(len(opts.Videos))
		name := filepath.Base(video)

		emit(onProgress, Progress{Stage: "transcribing", Value: base + step*0.05,
			Message: fmt.Sprintf("Transcribing %d/%d: %s", i+1, len(opts.Videos), name)})

		f, err := one(ctx, video, opts, deps, used, func(p Progress) {
			p.Value = base + step*p.Value
			emit(onProgress, p)
		})
		if err != nil {
			// Dibatalkan pengguna menghentikan seluruh job; kegagalan satu
			// berkas tidak.
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			f.Error = err.Error()
			emit(onProgress, Progress{Stage: "writing", Value: base + step,
				Message: "Failed " + name + ": " + err.Error()})
		}
		res.Files = append(res.Files, f)
	}

	if res.Done() == 0 {
		return res, fmt.Errorf("no caption could be written: %s", res.Files[0].Error)
	}
	emit(onProgress, Progress{Stage: "done", Value: 1,
		Message: fmt.Sprintf("%d of %d video(s) captioned", res.Done(), len(res.Files))})
	return res, nil
}

// one mengerjakan satu video: transkrip → caption → berkas.
func one(ctx context.Context, video string, opts Options, deps Deps, used map[string]bool, onProgress func(Progress)) (FileResult, error) {
	f := FileResult{Video: video, Name: filepath.Base(video)}

	speech, err := deps.Transcribe(ctx, video, opts.MaxSeconds)
	if err != nil {
		return f, err
	}
	f.VideoSec, f.UsedSec = speech.VideoSec, speech.UsedSec

	// Koreksi transkrip SEBELUM caption ditulis. Tanpa ini model menulis dari
	// teks yang salah dengar, dan hasilnya menyalin kesalahannya — pernah
	// terjadi: "Aikos" (IQOS) yang salah dengar naik jadi tagar #Aikos, dan
	// kalimat "roko doang aku gak suka kan" tersalin utuh jadi hook.
	//
	// Mesinnya sama dengan yang menulis caption, dan daftar istilah (-terms)
	// baru berlaku di sini — whisper dipanggil dengan -mc 0, jadi kosakata
	// memang tidak bisa dibias saat transkripsi (CLAUDE.md).
	if opts.fixOn() {
		emit(onProgress, Progress{Stage: "correcting", Value: 0.45,
			Message: opts.EngineName + " is correcting the transcript of " + f.Name})
		fixed, report, err := correct.Correct(ctx, speech.Transcript, opts.Terms,
			correct.Completer(deps.Complete), opts.EngineName, nil)
		if err != nil {
			// Mesin yang dibutuhkan tidak dilewati diam-diam (notes/12), tapi
			// kegagalannya tetap kegagalan SATU video — antrian bulk lanjut.
			return f, fmt.Errorf("correcting the transcript: %w", err)
		}
		speech.Transcript = fixed
		emit(onProgress, Progress{Stage: "correcting", Value: 0.55, Message: report.Summary()})
	}

	emit(onProgress, Progress{Stage: "writing", Value: 0.6,
		Message: "Writing captions for " + f.Name + ": " + opts.EngineName})
	f.Transcript = speech.Text()
	variants, err := Generate(ctx, deps.Complete, f.Transcript, opts)
	if err != nil {
		return f, err
	}
	f.Variants = variants

	path := outPath(video, opts.OutDir, used)
	emit(onProgress, Progress{Stage: "saving", Value: 0.9, Message: "Saving " + path})
	if err := os.WriteFile(path, []byte(Format(f, speech, opts)), 0o644); err != nil {
		return f, err
	}
	f.TXT = path
	return f, nil
}

// Format menyusun isi berkas .txt.
//
// Transkripnya ikut ditulis, di bawah caption. Itu bukan pelengkap: caption di
// atasnya karangan mesin, dan satu-satunya cara memeriksanya adalah membaca apa
// yang sebenarnya diucapkan — di berkas yang sama, bukan di tempat lain yang
// harus dicari dulu.
func Format(f FileResult, speech Speech, opts Options) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", f.Name)
	fmt.Fprintf(&b, "%s · %s\n", opts.EngineName, sourceNote(speech))
	for i, v := range f.Variants {
		fmt.Fprintf(&b, "\n--- caption %d ---\n%s\n", i+1, v.Text())
		if len(v.Tags) > 0 {
			fmt.Fprintf(&b, "#%s\n", strings.Join(v.Tags, " #"))
		}
		for _, w := range v.Violations {
			fmt.Fprintf(&b, "! check: %s\n", w)
		}
	}
	fmt.Fprintf(&b, "\n--- what is said in the video ---\n%s\n", strings.TrimSpace(speech.Text()))
	return b.String()
}

// sourceNote menyebut bagian video yang benar-benar dibaca.
//
// Selalu tertulis, bahkan saat seluruh video terbaca: pembacanya tidak bisa
// menebak dari captionnya sendiri apakah bagian belakang video ikut dihitung.
func sourceNote(s Speech) string {
	switch {
	case s.VideoSec <= 0:
		return "from the first " + fmtDur(s.UsedSec)
	case s.UsedSec > 0 && s.UsedSec < s.VideoSec-1:
		return fmt.Sprintf("from the first %s of %s", fmtDur(s.UsedSec), fmtDur(s.VideoSec))
	}
	return "from all " + fmtDur(s.VideoSec)
}

func fmtDur(sec float64) string {
	d := time.Duration(sec * float64(time.Second)).Round(time.Second)
	return fmt.Sprintf("%d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

// txtName mengganti akhiran berkas video jadi .txt.
func txtName(video string) string {
	return strings.TrimSuffix(video, filepath.Ext(video)) + ".txt"
}

// marker = penanda bahwa sebuah .txt adalah keluaran pembuat caption.
//
// Dibutuhkan karena nama bawaannya bertabrakan dengan berkas lain yang sudah
// ada: tiap klip yang dipotong Clipper punya "<klip>.txt" berisi ucapannya —
// bahan caption yang justru dipakai orang. Menimpanya berarti menghapus bahan
// itu dengan hasilnya sendiri. Berkas ucapan tidak pernah memuat baris ini,
// jadi ia mudah dibedakan dari keluaran kita sendiri, yang MEMANG boleh ditimpa
// (menjalankan ulang job yang sama harus memperbarui berkasnya, bukan menumpuk
// salinan bernomor).
const marker = "--- caption 1 ---"

// outPath menentukan ke mana .txt satu video ditulis.
//
// Bawaannya "<video>.txt" di sebelah videonya. Bila nama itu sudah dipakai
// berkas ORANG LAIN, dipakai "<video>.caption.txt", lalu bernomor — apa pun
// lebih baik daripada menghapus berkas yang bukan milik kita.
func outPath(video, outDir string, used map[string]bool) string {
	dir := outDir
	if dir == "" {
		dir = filepath.Dir(video)
	}
	base := strings.TrimSuffix(filepath.Base(video), filepath.Ext(video))

	for n := 0; ; n++ {
		name := base + ".txt"
		switch {
		case n == 1:
			name = base + ".caption.txt"
		case n > 1:
			name = fmt.Sprintf("%s.caption (%d).txt", base, n)
		}
		path := filepath.Join(dir, name)
		if !taken(path, used) {
			used[strings.ToLower(path)] = true
			return path
		}
	}
}

// taken melaporkan apakah sebuah path sudah dipakai job ini, atau ditempati
// berkas yang bukan keluaran pembuat caption.
func taken(path string, used map[string]bool) bool {
	if used[strings.ToLower(path)] {
		return true
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false // belum ada (atau tak terbaca — biarkan penulisan yang melapor)
	}
	return !strings.Contains(string(raw), marker)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func emit(fn func(Progress), p Progress) {
	if fn != nil {
		fn(p)
	}
}
