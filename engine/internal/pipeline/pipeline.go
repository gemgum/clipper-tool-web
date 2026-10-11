// Package pipeline mengorkestrasi proses end-to-end: audio → transkrip →
// segmentasi → scoring → render klip.
package pipeline

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/audio"
	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/correct"
	"github.com/gemgum/clipper/engine/internal/ffmpeg"
	"github.com/gemgum/clipper/engine/internal/score/heuristic"
	"github.com/gemgum/clipper/engine/internal/score/llm"
	"github.com/gemgum/clipper/engine/internal/segment"
	"github.com/gemgum/clipper/engine/internal/subtitle"
	"github.com/gemgum/clipper/engine/internal/transcribe"
	"github.com/gemgum/clipper/engine/internal/types"
)

// clipLeadIn adalah ancang-ancang (detik) sebelum awal ucapan saat memotong
// klip. Whisper menandai awal segmen pas di atau sesudah onset suara, sehingga
// memotong tepat di angka itu memenggal suku kata pertama.
const clipLeadIn = 0.3

// Progress dilaporkan selama proses.
type Progress struct {
	Stage   string      `json:"stage"`
	Value   float64     `json:"progress"`
	Message string      `json:"message,omitempty"`
	Clip    *types.Clip `json:"clip,omitempty"`
	// Summary hanya terisi pada peristiwa terakhir: tabel rincian waktu siap
	// tampil. Dikirim sebagai teks, bukan data mentah, karena kedua penampilnya
	// (terminal & kotak log GUI) sama-sama monospace.
	Summary string `json:"summary,omitempty"`
}

// ProgressFunc callback progres (boleh nil).
type ProgressFunc func(Progress)

// Pipeline menyimpan dependensi & konfigurasi.
type Pipeline struct {
	Paths config.Paths
	Opts  config.Options
	ff    *ffmpeg.Client
	wh    *transcribe.Whisper
}

func New(paths config.Paths, opts config.Options) *Pipeline {
	return &Pipeline{
		Paths: paths,
		Opts:  opts,
		ff:    ffmpeg.New(paths.FFmpeg, paths.FFprobe),
		wh:    transcribe.New(paths.Whisper, paths.Model),
	}
}

func emit(fn ProgressFunc, p Progress) {
	if fn != nil {
		fn(p)
	}
}

// Run menjalankan seluruh pipeline untuk satu video. workDir menampung file
// sementara & output klip.
func (p *Pipeline) Run(ctx context.Context, jobID, input, workDir, outDir string, onProgress ProgressFunc) ([]types.Clip, error) {
	if err := p.transcriberReady(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, err
	}
	// Klip final di outDir. Bila pengguna tidak menentukan folder keluaran,
	// outDir = workDir — karena itu berkas kerja (audio, transkrip, subtitle
	// sementara) ditaruh di subfolder tmp/, supaya folder hasil hanya berisi
	// klip dan .srt, tidak tercampur berkas antara.
	if outDir == "" {
		outDir = workDir
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("output folder %q: %w", outDir, err)
	}
	tmpDir := filepath.Join(workDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("work folder %q: %w", tmpDir, err)
	}

	rec := newRecorder()
	// Durasi video dipakai menghitung rasio realtime di ringkasan. Gagal probe
	// tidak boleh menggagalkan job — ringkasannya cukup tampil tanpa rasio.
	videoSec, _ := p.ff.Duration(ctx, input)

	// 1-3. Transkripsi (tahap termahal, lihat transcript.go). Audio tetap
	// diekstrak walau transkripnya dari cache bila mesin skornya heuristik —
	// ia membaca energi audio, bukan teks.
	needAudio := p.Opts.Provider == "heuristic"
	got, err := p.Transcript(ctx, input, tmpDir, 0, needAudio, onProgress)
	if err != nil {
		return nil, err
	}
	tr, cacheKey, cacheHit := got.Transcript, got.CacheKey, got.FromCache
	if got.ExtractDur > 0 {
		rec.add("Extract audio", got.ExtractDur, "ffmpeg")
	}
	if cacheHit {
		rec.add("Transcribe", 0, p.TranscriberName()+" (from cache)")
	} else {
		rec.add("Transcribe", got.WhisperDur, p.TranscriberName())
	}

	var rms audio.FeaturesResult
	if needAudio && got.WAV != "" {
		emit(onProgress, Progress{Stage: "extracting", Value: 0.12, Message: "Analysing audio energy"})
		t0 := time.Now()
		r, err := audio.Features(got.WAV, 100)
		if err != nil {
			return nil, fmt.Errorf("audio features: %w", err)
		}
		rms = r
		rec.since("Audio features", t0, "built-in")
	}

	// 3b. Koreksi transkrip. Dijalankan SEBELUM segmentasi & scoring, bukan
	// hanya sebelum subtitle: tanda baca yang salah tempat membuat
	// segment.BuildCandidates memotong klip di tengah kalimat.
	if p.Opts.TranscriptFix != config.TranscriptFixOff {
		t0 := time.Now()
		fixed, err := p.correctTranscript(ctx, tr, cacheKey, onProgress)
		if err != nil {
			return nil, err
		}
		tr = fixed
		rec.since("Correct transcript", t0, correctionNote(p.Opts))
	}

	// 4-5. Pemilihan momen. Mesin dipilih pengguna dan TIDAK diganti diam-diam:
	// bila mesin yang dipilih gagal, job ikut gagal dengan pesan akar masalah.
	//
	// Tahapnya diumumkan SEBELUM mesinnya disiapkan: menyiapkan mesin (mis.
	// Ollama tanpa model itu) bisa gagal, dan tanpa kabar ini GUI mengira job
	// berhenti di transkripsi — kartu tahap yang salah ikut memerah.
	emit(onProgress, Progress{Stage: "scoring", Value: 0.55})
	var selected []types.Clip
	var sel momentSelector
	engineName := ""
	switch p.Opts.Provider {
	case "claude":
		sel = llm.New(p.Paths.APIKey, p.Opts.LLMModel)
		engineName = "Claude (" + p.Opts.LLMModel + ")"
	case "ollama":
		// Alamat & model dipastikan dulu — lihat ollama_resolve.go.
		c, name, err := resolveOllama(ctx, p.Opts.OllamaURL, p.Opts.OllamaModel)
		if err != nil {
			return nil, err
		}
		sel, engineName = c, name
	case "heuristic":
		engineName = "heuristic"
	default:
		// Penyedia cloud yang bicara /chat/completions (OpenAI, Gemini,
		// DeepSeek, …). Koordinatnya diisi server dari daftar mesin — pipeline
		// tidak menyimpan tabel penyedia sendiri.
		c, name, err := p.cloudClient()
		if err != nil {
			return nil, err
		}
		sel, engineName = c, name
	}

	tSel := time.Now()
	if sel != nil {
		clips, err := p.selectWith(ctx, tr, sel, engineName, onProgress)
		if err != nil {
			return nil, fmt.Errorf("%s failed to select moments: %w", engineName, err)
		}
		selected = clips
	} else {
		emit(onProgress, Progress{Stage: "segmenting", Value: 0.58, Message: "Selecting clips (heuristic)"})
		selected = p.heuristicSelect(tr, rms)
	}
	rec.since("Select moments", tSel, engineName)
	// Buang momen yang jatuh di cuplikan pembuka. Berlaku untuk SEMUA mesin:
	// prompt sudah melarangnya, tapi diuji langsung qwen2.5 tetap memilihnya —
	// model lokal tidak terikat instruksi panjang.
	//
	// Tidak pernah membuang habis: kalau semua momen kena, yang tersisa hanya
	// pilihan buruk, dan pilihan buruk masih lebih baik daripada job gagal
	// dengan "tidak ada klip".
	if kept := dropOpeningPreview(tr, selected); len(kept) > 0 && len(kept) < len(selected) {
		emit(onProgress, Progress{Stage: "segmenting", Value: 0.60, Message: fmt.Sprintf(
			"Skipped %d moment(s) taken from the opening teaser", len(selected)-len(kept))})
		selected = kept
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("%s produced no clips: try loosening the duration preset or lowering the minimum score", engineName)
	}

	// 6. Render klip.
	//
	// Satu klip yang gagal dirender TIDAK menggagalkan job (DESIGN.md §9):
	// klip itu ditandai "failed" beserta sebabnya, sisanya tetap dirender, dan
	// yang gagal bisa dirender ulang belakangan (Rerender). Job baru gagal bila
	// SEMUA klip gagal — saat itu tidak ada hasil apa pun untuk diserahkan.
	tRender := time.Now()
	failed := 0
	for i := range selected {
		cl := &selected[i]
		cl.ID = fmt.Sprintf("clip_%02d", i+1)
		cl.JobID = jobID
		frac := 0.75 + 0.24*float64(i+1)/float64(len(selected))
		emit(onProgress, Progress{Stage: "rendering", Value: frac,
			Message: fmt.Sprintf("Rendering %s (score %d)", cl.ID, cl.Score)})

		segs := segmentsInRange(tr, cl.Start, cl.End)

		// Mundur sedikit dari awal ucapan: timestamp awal whisper jatuh pas atau
		// sesudah onset suara, jadi memotong tepat di angka itu memenggal suku
		// kata pertama. Dilakukan SETELAH segmentsInRange supaya ancang-ancang
		// ini tidak ikut menarik masuk segmen sebelumnya, dan cl.Start memang
		// diubah agar metadata klip cocok dengan berkas yang jadi — termasuk
		// titik acuan subtitle di bawah, supaya tetap sinkron.
		cl.Start -= clipLeadIn
		if cl.Start < 0 {
			cl.Start = 0
		}

		if err := p.renderClip(ctx, input, outDir, tmpDir, segs, cl); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			failed++
			cl.Status, cl.Error = "failed", err.Error()
			emit(onProgress, Progress{Stage: "rendering", Value: frac,
				Message: fmt.Sprintf("%s failed to render: %v", cl.ID, err), Clip: cl})
			continue
		}
		emit(onProgress, Progress{Stage: "rendering", Value: frac, Clip: cl})
	}
	if failed == len(selected) {
		return nil, fmt.Errorf("every clip failed to render. First error: %s", selected[0].Error)
	}

	rec.since(fmt.Sprintf("Render %d %s", len(selected), plural(len(selected), "clip", "clips")), tRender,
		fmt.Sprintf("%s, %s", p.Opts.Resolution, p.Opts.Quality))

	sum := rec.summary(jobID, videoSec, len(selected), cacheHit)
	emit(onProgress, Progress{Stage: "done", Value: 1.0,
		Message: fmt.Sprintf("Done: %d clips", len(selected)),
		Summary: sum.Format()})
	return selected, nil
}

// correctionTemperature sengaja rendah. Koreksi transkrip adalah tugas
// menyalin-ulang, bukan tugas kreatif: pada suhu bawaan (0.4) model lokal
// memberi hasil berbeda tiap kali dijalankan atas transkrip yang sama.
const correctionTemperature = 0.1

// correctionProvider memilih LLM untuk koreksi transkrip.
//
// Mesin yang SAMA dengan pemilihan momen — pengguna menyebutnya sekali, dan
// tidak ada setelan kedua yang bisa tidak sinkron dengan yang pertama. Dulu
// diturunkan dari Mode ("hybrid → Claude"), dan mode itu dibuang bersama
// pemilih mesin bersama (notes/39).
//
// Kekecualiannya cuma satu: koreksi tetap butuh LLM walau skornya heuristik.
// Di situ Ollama yang dipakai — mesin lokal yang tidak menagih apa pun.
func correctionProvider(o config.Options) string {
	if o.Provider == "" || o.Provider == "heuristic" {
		return "ollama"
	}
	return o.Provider
}

// correctionNote menyebut mesin & jumlah istilah yang dipakai tahap koreksi,
// untuk ditampilkan di ringkasan. Daftar istilah ikut disebut karena ia
// mengubah hasil DAN kunci cache — dua percobaan yang waktunya mirip tapi
// daftarnya berbeda bukan percobaan yang sama.
func correctionNote(o config.Options) string {
	name := "Ollama (" + o.OllamaModel + ")"
	if correctionProvider(o) == "claude" {
		name = "Claude (" + o.LLMModel + ")"
	}
	switch n := len(o.Terms); {
	case n == 1:
		name += ", 1 term"
	case n > 1:
		name += fmt.Sprintf(", %d terms", n)
	}
	return name
}

// correctTranscript membenahi transkrip sebelum dipakai tahap berikutnya.
//
// Hasilnya di-cache terpisah dari transkrip mentah: mematikan koreksi tidak
// boleh memaksa transkripsi ulang, dan menyalakannya lagi tidak perlu memanggil
// LLM dua kali untuk video yang sama.
func (p *Pipeline) correctTranscript(ctx context.Context, tr types.Transcript, cacheKey string, onProgress ProgressFunc) (types.Transcript, error) {
	provider := correctionProvider(p.Opts)

	var complete correct.Completer
	var engineName string
	switch provider {
	case "claude":
		c := llm.New(p.Paths.APIKey, p.Opts.LLMModel)
		c.BaseURL = p.Opts.LLMBase
		c.Temperature = correctionTemperature
		engineName = "Claude (" + c.Model + ")"
		// Claude tidak punya parameter skema; bentuk balasannya dijaga prompt.
		complete = func(ctx context.Context, system, user string, _ any) (string, error) {
			return c.Complete(ctx, system, user, 8192)
		}
	case "ollama":
		c, name, err := resolveOllama(ctx, p.Opts.OllamaURL, p.Opts.OllamaModel)
		if err != nil {
			return types.Transcript{}, err
		}
		c.Temperature = correctionTemperature
		engineName = name
		complete = func(ctx context.Context, system, user string, schema any) (string, error) {
			return c.Complete(ctx, system, user, schema, 4096)
		}
	default:
		c, name, err := p.cloudClient()
		if err != nil {
			return types.Transcript{}, err
		}
		c.Temperature = correctionTemperature
		engineName = name
		complete = func(ctx context.Context, system, user string, schema any) (string, error) {
			return c.Complete(ctx, system, user, schema, 4096)
		}
	}

	cachePath := ""
	if cacheKey != "" {
		cachePath = correctedCachePath(p.Paths.DataDir, cacheKey, engineName, correct.CacheVersion(p.Opts.Terms))
		if cached, ok := loadTranscriptCache(cachePath); ok {
			emit(onProgress, Progress{Stage: "correcting", Value: 0.58,
				Message: "Corrected transcript loaded from cache"})
			return cached, nil
		}
	}

	// Jumlah istilah ikut dilaporkan: daftar yang tidak terkirim gagal secara
	// senyap — hasilnya sekadar "tidak ada yang berubah", persis seperti daftar
	// yang terkirim tapi tidak dipakai model. Tanpa angka ini keduanya mustahil
	// dibedakan tanpa membongkar berkas cache.
	terms := ""
	switch n := len(p.Opts.Terms); {
	case n == 1:
		terms = " (1 term)"
	case n > 1:
		terms = fmt.Sprintf(" (%d terms)", n)
	}
	emit(onProgress, Progress{Stage: "correcting", Value: 0.48,
		Message: fmt.Sprintf("%s is correcting the transcript%s", engineName, terms)})

	fixed, report, err := correct.Correct(ctx, tr, p.Opts.Terms, complete, engineName, func(done, total int) {
		emit(onProgress, Progress{
			Stage:   "correcting",
			Value:   0.48 + 0.10*float64(done)/float64(total),
			Message: fmt.Sprintf("%s is correcting the transcript: part %d/%d", engineName, done, total),
		})
	})
	if err != nil {
		// Mesin yang dibutuhkan tidak boleh dilewati diam-diam (notes/12).
		// Pesannya menyebut cara mematikan koreksi, supaya pengguna tanpa LLM
		// tidak kehabisan jalan.
		return types.Transcript{}, fmt.Errorf(
			"transcript correction failed: %w. Fix the engine above, or turn correction off (-transcript-fix off in the CLI, or the checkbox in the GUI)", err)
	}

	emit(onProgress, Progress{Stage: "correcting", Value: 0.58, Message: report.Summary()})
	if cachePath != "" {
		// Gagal menyimpan cache tidak boleh menggagalkan job.
		_ = saveTranscriptCache(cachePath, fixed)
	}
	return fixed, nil
}

// momentSelector adalah mesin yang MEMILIH dari kandidat bernomor (Claude/Ollama).
//
// Dulu antarmuka ini bernama SelectMoments dan menyerahkan penentuan batas waktu
// ke model. Diukur pada qwen2.5, tiga kali jalan pada permintaan yang sama
// mengembalikan rentang terbalik ("468-43") dan momen 8 detik padahal diminta
// 30-60 — sebagian besar dibuang validateMoments, dan sisa yang lolos itulah
// yang membuat klip terasa berubah-ubah tiap kali dijalankan.
//
// Sekarang batas waktunya dibangun engine dari batas kalimat, dan model hanya
// memilih nomor. Kelas kegagalan itu hilang karena angkanya tidak pernah ada di
// tangan model.
type momentSelector interface {
	PickMoments(ctx context.Context, cands []types.Candidate, offset, maxClips int, contentLang string) ([]llm.Pick, error)
}

// selectWith menjalankan selektor per potongan transkrip lalu menggabungkan
// hasilnya. Kegagalan potongan mana pun menggagalkan seluruh job (tanpa
// fallback), agar pengguna tahu persis mesin mana yang bermasalah.
func (p *Pipeline) selectWith(ctx context.Context, tr types.Transcript, sel momentSelector, engineName string, onProgress ProgressFunc) ([]types.Clip, error) {
	// Kandidat dibangun dari SELURUH transkrip sekaligus, bukan per potongan.
	// Batas kandidat sudah ditentukan batas kalimat, jadi tidak ada momen yang
	// terbelah di sambungan potongan — penggabungan lintas potongan yang dulu
	// diperlukan (Moment.Continues) jadi tidak relevan.
	cands := segment.BuildCandidates(tr, p.Opts.TargetMin, p.Opts.TargetMax)
	if len(cands) == 0 {
		return nil, fmt.Errorf("no candidate clips could be built from the transcript")
	}

	// Dikirim berbatch: yang membatasi bukan kemampuan model membaca melainkan
	// jendela konteksnya. Model menilai per batch, penyaringan akhir tetap di
	// topN — jadi batch tidak mengurangi mutu pilihan, hanya membaginya.
	batch := llm.MaxCandidatesPerRequest
	batches := (len(cands) + batch - 1) / batch
	perBatch := p.Opts.MaxClips/batches + 1
	if perBatch < 2 {
		perBatch = 2
	}

	var clips []types.Clip
	for b := 0; b < batches; b++ {
		lo := b * batch
		hi := lo + batch
		if hi > len(cands) {
			hi = len(cands)
		}
		msg := fmt.Sprintf("%s is choosing from %d candidate clips", engineName, len(cands))
		if batches > 1 {
			msg = fmt.Sprintf("%s is choosing clips: part %d/%d", engineName, b+1, batches)
		}
		emit(onProgress, Progress{Stage: "scoring", Value: 0.58 + 0.15*float64(b)/float64(batches), Message: msg})

		picks, err := sel.PickMoments(ctx, cands[lo:hi], lo, perBatch, tr.Language)
		if err != nil {
			if batches > 1 {
				return nil, fmt.Errorf("part %d of %d: %w", b+1, batches, err)
			}
			return nil, err
		}
		got, bad := picksToClips(picks, cands, lo, hi)
		if bad > 0 {
			emit(onProgress, Progress{Stage: "scoring", Value: 0.73, Message: fmt.Sprintf(
				"%d pick(s) dropped: the number was not in the list", bad)})
		}
		clips = append(clips, got...)
	}
	if len(clips) == 0 {
		return nil, fmt.Errorf("%s chose none of the %d candidate clips", engineName, len(cands))
	}
	return topN(clips, p.Opts.MaxClips, p.Opts.MinScore), nil
}

// renderClip menulis berkas satu klip (video polos dan/atau bersubtitle, .srt,
// .txt). Dipakai Run dan Rerender — satu jalan render, jadi klip yang dirender
// ulang identik dengan yang dirender pertama kali. cl.Start sudah termasuk
// ancang-ancang clipLeadIn.
func (p *Pipeline) renderClip(ctx context.Context, input, outDir, tmpDir string, segs []types.TranscriptSegment, cl *types.Clip) error {
	tw, th := p.Opts.Dims()
	crf, preset := p.Opts.Encode()
	// Teks ucapan klip tanpa timestamp — bahan untuk dibuatkan caption oleh
	// LLM mana pun. Ditulis untuk SETIAP klip, tidak seperti .srt yang hanya
	// ada di mode clean/both: caption dibutuhkan saat memposting, dan orang
	// memposting klip bersubtitle juga.
	//
	// Kegagalannya tidak menggagalkan job: klipnya sudah jadi, dan berkas
	// pendamping yang hilang bukan alasan membuang pekerjaan berat itu.
	txt := filepath.Join(outDir, cl.ID+".txt")
	if err := subtitle.WriteText(txt, segs, cl.Start); err == nil {
		cl.TranscriptTXT = txt
	}

	enc := ffmpeg.EncodeOpts{
		CRF: crf, Preset: preset, FontsDir: p.Paths.FontsDir,
		Mode: string(p.Opts.Reframe), Background: p.Opts.Background, Zoom: p.Opts.Zoom,
		FPS: p.Opts.FPS,
	}
	// Varian polos (tanpa subtitle) — untuk mode clean & both.
	if p.Opts.SubtitleOutput == config.OutputClean || p.Opts.SubtitleOutput == config.OutputBoth {
		name := cl.ID + ".mp4"
		if p.Opts.SubtitleOutput == config.OutputBoth {
			name = cl.ID + "_clean.mp4"
		}
		raw := filepath.Join(outDir, name)
		if err := p.ff.ClipReframe(ctx, input, cl.Start, cl.End, tw, th, enc, raw); err != nil {
			return err
		}
		cl.VideoPathRaw = raw
		cl.VideoPath = raw // dipakai GUI bila tidak ada varian bersubtitle
		// Sertakan .srt agar klip polos bisa disubtitle di editor lain.
		srt := filepath.Join(outDir, cl.ID+".srt")
		if err := subtitle.WriteSRT(srt, segs, cl.Start, p.Opts.Subtitle); err == nil {
			cl.SubtitleSRT = srt
		}
	}
	// Varian bersubtitle (dibakar) — untuk mode burn & both.
	//
	// .ass hanya dibuat bila memang akan dibakar, ditulis ke tmp/, lalu
	// dihapus setelah pembakaran berhasil: isinya sudah menyatu di video
	// dan tidak dipakai siapa pun sesudah itu. Bila render gagal, berkasnya
	// sengaja ditinggal supaya penyebabnya masih bisa ditelusuri.
	if p.Opts.SubtitleOutput != config.OutputClean {
		// Watermark hanya di sini, dan itu disengaja: berkas "clean" dipakai
		// untuk disunting ulang di editor lain, dan identitas yang sudah
		// terbakar di dalamnya tidak bisa dilepas lagi.
		//
		// Sumber "llm" memakai judul yang dipilihkan LLM untuk KLIP INI,
		// jadi teksnya beda tiap klip — itu sebabnya ia dihitung di dalam
		// perulangan, bukan sekali di luar.
		headline := p.Opts.Watermark.Headline.Text
		if p.Opts.Watermark.Headline.Source == config.HeadlineLLM {
			headline = cl.Title
		}
		assPath := filepath.Join(tmpDir, cl.ID+".ass")
		if err := subtitle.WriteASS(assPath, segs, cl.Start, p.Opts.Subtitle,
			p.Opts.Watermark, headline, cl.End-cl.Start); err != nil {
			return err
		}
		enc.AssPath = assPath
		enc.Watermark = ffmpeg.Watermark{
			Image: p.Opts.Watermark.Image, X: p.Opts.Watermark.X, Y: p.Opts.Watermark.Y,
			Width: p.Opts.Watermark.Width, Height: p.Opts.Watermark.Height,
			At: p.Opts.Watermark.At, For: p.Opts.Watermark.For,
		}
		outMP4 := filepath.Join(outDir, cl.ID+".mp4")
		if err := p.ff.ClipReframe(ctx, input, cl.Start, cl.End, tw, th, enc, outMP4); err != nil {
			return err
		}
		_ = os.Remove(assPath)
		cl.VideoPath = outMP4
	}
	cl.Status, cl.Error = "rendered", ""
	return nil
}

// Rerender merender ulang klip berstatus "failed" dari job yang sudah selesai.
//
// Transkrip (dan koreksinya) diambil dari cache dengan jalan yang sama seperti
// Run, jadi tidak ada transkripsi ulang selama video sumbernya masih ada.
// Klip lain tidak disentuh. Hasilnya daftar klip lengkap dengan status baru.
func (p *Pipeline) Rerender(ctx context.Context, input, workDir, outDir string, clips []types.Clip, onProgress ProgressFunc) ([]types.Clip, error) {
	if outDir == "" {
		outDir = workDir
	}
	tmpDir := filepath.Join(workDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("work folder %q: %w", tmpDir, err)
	}
	got, err := p.Transcript(ctx, input, tmpDir, 0, false, onProgress)
	if err != nil {
		return nil, err
	}
	tr := got.Transcript
	if p.Opts.TranscriptFix != config.TranscriptFixOff {
		if tr, err = p.correctTranscript(ctx, tr, got.CacheKey, onProgress); err != nil {
			return nil, err
		}
	}
	out := append([]types.Clip(nil), clips...)
	var todo []int
	for i := range out {
		if out[i].Status == "failed" {
			todo = append(todo, i)
		}
	}
	for n, i := range todo {
		cl := &out[i]
		frac := float64(n+1) / float64(len(todo))
		emit(onProgress, Progress{Stage: "rendering", Value: frac,
			Message: fmt.Sprintf("Rendering %s again", cl.ID)})
		// Segmen dicari dari awal ucapan ASLI (sebelum ancang-ancang), sama
		// seperti di Run.
		segs := segmentsInRange(tr, cl.Start+clipLeadIn, cl.End)
		if err := p.renderClip(ctx, input, outDir, tmpDir, segs, cl); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			cl.Status, cl.Error = "failed", err.Error()
			emit(onProgress, Progress{Stage: "rendering", Value: frac,
				Message: fmt.Sprintf("%s failed to render again: %v", cl.ID, err), Clip: cl})
			continue
		}
		emit(onProgress, Progress{Stage: "rendering", Value: frac, Clip: cl})
	}
	return out, nil
}

// picksToClips memetakan nomor pilihan model kembali ke kandidatnya.
//
// Nomor di luar daftar dibuang, bukan dijepit ke tetangga terdekat: menjepit
// akan mengubah pilihan model jadi klip yang tidak pernah ia lihat. Nomor
// ganda juga dibuang — model kadang memilih kandidat yang sama dua kali.
func picksToClips(picks []llm.Pick, cands []types.Candidate, lo, hi int) ([]types.Clip, int) {
	seen := map[int]bool{}
	var out []types.Clip
	bad := 0
	for _, pk := range picks {
		if pk.Index < lo || pk.Index >= hi || seen[pk.Index] {
			bad++
			continue
		}
		seen[pk.Index] = true
		c := cands[pk.Index]
		out = append(out, types.Clip{
			Start:      c.Start,
			End:        c.End,
			Duration:   c.Duration(),
			Score:      int(math.Round(pk.Score)),
			Reasons:    llm.ToReasons(pk.Reasons, pk.Score),
			Title:      strings.TrimSpace(pk.Title),
			Hashtags:   pk.Hashtags,
			Transcript: c.Text,
			Status:     "scored",
		})
	}
	return out, bad
}

// heuristicSelect memilih klip via segmentasi window + skor heuristik (fallback).
func (p *Pipeline) heuristicSelect(tr types.Transcript, rms audio.FeaturesResult) []types.Clip {
	cands := segment.BuildCandidates(tr, p.Opts.TargetMin, p.Opts.TargetMax)
	var clips []types.Clip
	for _, c := range cands {
		total, reasons := heuristic.Score(c, p.Opts.TargetMin, p.Opts.TargetMax, rmsSlice(rms, c))
		clips = append(clips, types.Clip{
			Start: c.Start, End: c.End, Duration: c.Duration(),
			Score: total, Reasons: reasons, Transcript: c.Text, Status: "scored",
		})
	}
	return topN(clips, p.Opts.MaxClips, p.Opts.MinScore)
}

// joinRange menggabungkan teks segmen dalam rentang [start,end].
func joinRange(tr types.Transcript, start, end float64) string {
	var parts []string
	for _, s := range tr.Segments {
		if s.End > start && s.Start < end {
			parts = append(parts, s.Text)
		}
	}
	return strings.Join(parts, " ")
}

// topN mengurutkan menurun, menyaring skor minimum, mengambil n teratas.
func topN(clips []types.Clip, n, minScore int) []types.Clip {
	sort.Slice(clips, func(i, j int) bool { return clips[i].Score > clips[j].Score })
	var out []types.Clip
	for _, c := range clips {
		if c.Score < minScore {
			continue
		}
		out = append(out, c)
		if len(out) >= n {
			break
		}
	}
	if len(out) == 0 && len(clips) > 0 {
		out = clips[:min(n, len(clips))]
	}
	return out
}

// rmsSlice mengambil potongan RMS untuk jendela kandidat.
func rmsSlice(r audio.FeaturesResult, c types.Candidate) []float64 {
	if len(r.RMS) == 0 || r.HopMS <= 0 {
		return nil
	}
	hop := float64(r.HopMS) / 1000.0
	i := int(c.Start / hop)
	j := int(c.End / hop)
	if i < 0 {
		i = 0
	}
	if j > len(r.RMS) {
		j = len(r.RMS)
	}
	if i >= j {
		return nil
	}
	return r.RMS[i:j]
}

func segmentsInRange(tr types.Transcript, start, end float64) []types.TranscriptSegment {
	var out []types.TranscriptSegment
	for _, s := range tr.Segments {
		if s.End > start && s.Start < end {
			out = append(out, s)
		}
	}
	return out
}

// dropOpeningPreview menyaring momen yang berasal dari cuplikan pembuka.
//
// Dipisah dari pemanggilnya supaya bisa diuji tanpa menjalankan seluruh
// pipeline — dan supaya jelas bahwa ia hanya MENYARING, tidak mengubah momen.
func dropOpeningPreview(tr types.Transcript, clips []types.Clip) []types.Clip {
	// Ujung cuplikan dihitung sekali, bukan per klip: satu video punya satu
	// ujung cuplikan, dan menghitungnya ulang tiap klip hanya memindai seluruh
	// transkrip berkali-kali untuk jawaban yang sama.
	previewEnd := segment.OpeningPreviewEnd(tr)
	if previewEnd <= 0 {
		return clips
	}
	kept := make([]types.Clip, 0, len(clips))
	for _, c := range clips {
		// Cukup awalnya yang diperiksa. Klip yang MULAI di dalam cuplikan sudah
		// cacat betapa pun panjang ekornya — justru klip berkaki dua seperti
		// itulah yang dulu lolos.
		if c.Start < previewEnd {
			continue
		}
		kept = append(kept, c)
	}
	return kept
}
