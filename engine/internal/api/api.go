// Package api menyediakan HTTP API + SSE untuk GUI.
package api

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/caption"
	"github.com/gemgum/clipper/engine/internal/capture"
	"github.com/gemgum/clipper/engine/internal/card"
	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/correct"
	"github.com/gemgum/clipper/engine/internal/ffmpeg"
	"github.com/gemgum/clipper/engine/internal/job"
	"github.com/gemgum/clipper/engine/internal/news"
	"github.com/gemgum/clipper/engine/internal/pipeline"
	"github.com/gemgum/clipper/engine/internal/score/ollama"
	"github.com/gemgum/clipper/engine/internal/watermark"
	"github.com/gemgum/clipper/engine/internal/writer"
)

// Server membungkus manager job.
type Server struct {
	mgr    *job.Manager
	layout config.Layout
	paths  config.Paths
	ff     *ffmpeg.Client
	cards  *card.Builder
	// installs = pemasangan komponen yang berjalan di latar (lihat installs.go).
	installs installs
	// posts = job pembuat berita yang berjalan di latar (lihat posts.go).
	posts bgStore[writer.Result]
	// captions = job pembuat caption yang berjalan di latar (lihat captions.go).
	captions bgStore[caption.Result]
	// watermarks = job watermark yang berjalan di latar (lihat watermark.go).
	watermarks bgStore[watermark.Result]
	// token = kunci sesi; kosong berarti pemeriksaan dimatikan (lihat token.go).
	token string
	// hosts = nama host tambahan yang boleh dipakai menghubungi engine, di luar
	// keluarga loopback yang selalu diterima (lihat guard.go).
	hosts []string
	// web + password = mode web (branch webview, lihat web.go).
	web      bool
	password string
}

func NewServer(mgr *job.Manager, l config.Layout) *Server {
	paths := config.ResolvePaths(l, config.DefaultOptions())
	s := &Server{
		mgr:    mgr,
		layout: l,
		paths:  paths,
		ff:     ffmpeg.New(paths.FFmpeg, paths.FFprobe),
		cards:  card.New(capture.New(paths.Chrome), paths.FontsDir),
	}
	// Hasil pembuat berita, caption, dan watermark bertahan setelah aplikasi
	// ditutup (riwayat bersama, DESIGN-Clipper-Lanjutan §6).
	runs := filepath.Join(paths.DataDir, "runs")
	s.posts.persistTo(filepath.Join(runs, "post"))
	s.captions.persistTo(filepath.Join(runs, "caption"))
	s.watermarks.persistTo(filepath.Join(runs, "watermark"))
	if mgr != nil {
		mgr.Prepare = func(o *config.Options) { fillEngine(o); fillTranscriber(o) }
	}
	return s
}

// applyPaths membaca ulang letak program setelah pengguna menunjuknya sendiri.
//
// Server menyimpan Paths & pembuat kartu sejak dibuat; tanpa langkah ini,
// browser yang baru saja dipilih baru terpakai setelah aplikasi dijalankan lagi
// — dan pengguna wajar mengira pilihannya tidak tersimpan.
func (s *Server) applyPaths() {
	// Layout ikut dibaca ulang: setelan disimpan sebagai env, dan Locate yang
	// membacanya. Tanpa ini, folder yang baru dipilih hanya berlaku setelah
	// aplikasi dijalankan lagi.
	s.layout = config.Locate(s.layout.Root)
	s.mgr.SetLayout(s.layout)
	s.paths = config.ResolvePaths(s.layout, config.DefaultOptions())
	s.ff = ffmpeg.New(s.paths.FFmpeg, s.paths.FFprobe)
	s.cards = card.New(capture.New(s.paths.Chrome), s.paths.FontsDir)
}

// Handler membangun router.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/config", s.config)
	mux.HandleFunc("GET /api/models", s.listModels)
	// Halaman Requirements: status komponen + pemasangannya.
	mux.HandleFunc("GET /api/requirements", s.requirements)
	mux.HandleFunc("POST /api/requirements/install", s.webOff(s.installComponent))
	mux.HandleFunc("GET /api/requirements/events", s.installEvents)
	mux.HandleFunc("POST /api/requirements/remove", s.webOff(s.removeComponent))
	mux.HandleFunc("POST /api/requirements/path", s.webOff(s.setComponentPath))
	// Mesin LLM: satu daftar untuk seluruh aplikasi (notes/39).
	mux.HandleFunc("GET /api/engines", s.listEngines)
	mux.HandleFunc("POST /api/engines", s.saveEngine)
	mux.HandleFunc("POST /api/engines/test", s.testEngine)
	mux.HandleFunc("POST /api/engines/delete", s.deleteEngine)
	mux.HandleFunc("GET /api/settings/ai", s.getAISettings)
	mux.HandleFunc("POST /api/settings/ai", s.saveAISettings)
	mux.HandleFunc("GET /api/engines/{id}/models", s.engineModels)
	mux.HandleFunc("GET /api/settings", s.getSettings)
	mux.HandleFunc("POST /api/settings", s.postSettings)
	mux.HandleFunc("POST /api/settings/folders", s.webOff(s.postFolders))
	mux.HandleFunc("GET /api/ollama/status", s.ollamaStatus)
	mux.HandleFunc("POST /api/ollama/pull", s.ollamaPull)
	mux.HandleFunc("POST /api/ollama/ping", s.ollamaPing)
	mux.HandleFunc("GET /api/fonts", s.listFonts)
	mux.HandleFunc("GET /api/font-file", s.fontFile)
	mux.HandleFunc("GET /api/font-check", s.fontCheck)
	mux.HandleFunc("GET /api/probe", s.probe)
	mux.HandleFunc("GET /api/frame", s.frame)
	mux.HandleFunc("POST /api/upload", s.upload)
	// Dua ini yang membuat unggahan tidak perlu: berkasnya sudah ada di mesin
	// yang sama, engine tinggal diberi tahu di mana.
	mux.HandleFunc("GET /api/image", s.image)
	mux.HandleFunc("GET /api/browse", s.webOff(s.browse))
	mux.HandleFunc("POST /api/locate", s.webOff(s.locate))
	mux.HandleFunc("POST /api/jobs", s.createJob)
	mux.HandleFunc("GET /api/jobs", s.listJobs)
	mux.HandleFunc("GET /api/jobs/{id}", s.getJob)
	mux.HandleFunc("GET /api/jobs/{id}/events", s.jobEvents)
	mux.HandleFunc("POST /api/jobs/{id}/cancel", s.cancelJob)
	mux.HandleFunc("DELETE /api/jobs/{id}", s.deleteJob)
	mux.HandleFunc("POST /api/jobs/{id}/retry", s.retryJob)
	mux.HandleFunc("POST /api/jobs/{id}/rerender", s.rerenderJob)
	mux.HandleFunc("GET /api/jobs/{id}/log", s.jobLog)
	mux.HandleFunc("GET /api/jobs/{id}/clips", s.jobClips)
	mux.HandleFunc("GET /api/jobs/{id}/clips/{clip}/file", s.clipFile)
	// Menunjukkan berkas di pengelola berkas sistem — menggantikan tautan unduh
	// (lihat reveal.go).
	mux.HandleFunc("POST /api/jobs/{id}/clips/{clip}/reveal", s.webOff(s.revealClip))
	mux.HandleFunc("DELETE /api/jobs/{id}/clips/{clip}", s.deleteClip)
	// Kartu berita (tab kedua di GUI).
	mux.HandleFunc("GET /api/news/feeds", s.newsFeeds)
	mux.HandleFunc("GET /api/news/list", s.newsList)
	mux.HandleFunc("POST /api/news/article", s.newsArticle)
	mux.HandleFunc("POST /api/news/resolve", s.newsResolve)
	mux.HandleFunc("POST /api/news/analyze", s.newsAnalyze)
	// Rancangan ulang News cards (DESIGN-NEWSCARD.md) — lihat newscard.go.
	mux.HandleFunc("POST /api/news/image", s.newsImage)
	mux.HandleFunc("POST /api/news/write", s.newsWrite)
	mux.HandleFunc("POST /api/card", s.makeCard)
	mux.HandleFunc("GET /api/card/{id}/file", s.cardFile)
	mux.HandleFunc("GET /api/card/{id}/zip", s.cardZip)
	mux.HandleFunc("GET /api/cards", s.listCards)
	// Unduh sekaligus apa pun yang dicentang di halaman riwayat — lihat bulk.go.
	mux.HandleFunc("GET /api/download", s.bulkZip)
	mux.HandleFunc("DELETE /api/cards/{id}", s.deleteCard)
	// Pembuat berita (notes/38). Job berjalan di latar; kemajuannya lewat SSE.
	mux.HandleFunc("POST /api/posts", s.createPost)
	mux.HandleFunc("GET /api/posts", s.listPosts)
	mux.HandleFunc("GET /api/posts/events", s.postEvents)
	mux.HandleFunc("GET /api/posts/{id}", s.getPost)
	mux.HandleFunc("GET /api/posts/{id}/file", s.postFile)
	mux.HandleFunc("POST /api/posts/{id}/cancel", s.cancelPost)
	mux.HandleFunc("DELETE /api/posts/{id}", s.posts.deleteHandler)
	mux.HandleFunc("GET /api/posts/limits", s.postLimits)

	mux.HandleFunc("POST /api/captions", s.createCaption)
	mux.HandleFunc("GET /api/captions", s.listCaptions)
	mux.HandleFunc("GET /api/captions/events", s.captionEvents)
	mux.HandleFunc("GET /api/captions/{id}", s.getCaption)
	mux.HandleFunc("GET /api/captions/{id}/file", s.captionFile)
	mux.HandleFunc("POST /api/captions/{id}/cancel", s.cancelCaption)
	mux.HandleFunc("DELETE /api/captions/{id}", s.captions.deleteHandler)

	mux.HandleFunc("POST /api/watermark", s.createWatermark)
	mux.HandleFunc("GET /api/watermark", s.listWatermarks)
	mux.HandleFunc("GET /api/watermark/events", s.watermarkEvents)
	mux.HandleFunc("GET /api/watermark/{id}", s.getWatermark)
	mux.HandleFunc("GET /api/watermark/{id}/file", s.watermarkFile)
	mux.HandleFunc("POST /api/watermark/{id}/cancel", s.cancelWatermark)
	mux.HandleFunc("DELETE /api/watermark/{id}", s.watermarks.deleteHandler)
	if s.web {
		mux.HandleFunc("POST /api/login", s.login)
		mux.HandleFunc("GET /login", loginPage)
	}
	// GUI statis di akar. Didaftarkan terakhir: pola "/" menangkap semua yang
	// tidak cocok dengan rute di atasnya.
	if ui := webUI(s.layout.GUIDir); ui != nil {
		mux.Handle("/", ui)
	} else {
		mux.HandleFunc("/", guiMissingPage)
	}
	// Urutannya dari yang paling murah & paling luas ke yang paling sempit:
	// penjaga menolak bentuk permintaan yang mustahil datang dari GUI ini, CORS
	// menolak halaman dari luar mesin, kunci menolak sisanya.
	return s.withGuard(withCORS(s.withToken(mux), s.hosts...))
}

// --- kartu berita ---

// newsFeeds melaporkan daftar sumber RSS bawaan + apakah browser tersedia.
// GUI memakai flag itu untuk menjelaskan sejak awal bila kartu tak bisa dibuat,
// bukan membiarkan pengguna menemukannya saat menekan tombol.
func (s *Server) newsFeeds(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"feeds":       news.DefaultSources,
		"has_browser": s.paths.Chrome != "",
		"browser":     filepath.Base(s.paths.Chrome),
		"styles":      []string{card.StyleDark, card.StyleLight, card.StyleQuote},
		"ratios":      []string{card.Ratio916, card.Ratio45, card.Ratio11},
		"aligns":      []string{card.AlignLeft, card.AlignCenter, card.AlignRight, card.AlignJustify},
		"photo_fits":  []string{card.FitCover, card.FitWhole},
		"photo_fills": []string{card.FillBlur, card.FillSolid},
		// Banyaknya langkah ukuran huruf ke tiap arah. Dikirim dari sini supaya
		// GUI tidak menyalin angkanya — satu tempat, satu kebenaran.
		"font_steps":   card.FontSteps,
		"header_max":   card.HeaderMax,
		"card_top_max": card.CardTopMax,
		// Warna kartu yang boleh dipilih. Daftar tertutup, dibuat engine — GUI
		// tidak menghitung warnanya sendiri.
		"card_colours": card.Swatches(),
	})
}

// lang membaca bahasa yang diminta klien. Menentukan bahasa teks yang DITULIS
// engine ke dalam kartu (tanggal, kaki kartu, berkas pendamping) — bukan bahasa
// artikelnya, yang selalu apa adanya dari medianya.
func lang(r *http.Request) string {
	if v := strings.TrimSpace(r.URL.Query().Get("lang")); v != "" {
		return v
	}
	return "en"
}

// newsList mengambil isi satu feed. Param 'feed' boleh berupa ID sumber bawaan
// atau URL feed apa pun.
func (s *Server) newsList(w http.ResponseWriter, r *http.Request) {
	max, _ := strconv.Atoi(r.URL.Query().Get("max"))
	// Kata kunci menang atas pilihan feed: kalau pengguna mengetik sesuatu, itu
	// yang dia mau lihat.
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		items, err := news.Search(r.Context(), q, max, lang(r))
		if err != nil {
			writeErr(w, 502, err.Error())
			return
		}
		writeJSON(w, 200, items)
		return
	}

	feed := strings.TrimSpace(r.URL.Query().Get("feed"))
	// "all" merangkak seluruh feed bawaan sekaligus — itu yang dipakai daftar
	// berita di GUI, supaya pengguna tidak perlu memilih sumber satu per satu
	// hanya untuk melihat apa yang baru.
	if feed == "" || feed == "all" {
		items, err := news.ListAll(r.Context(), max, lang(r))
		if err != nil {
			writeErr(w, 502, err.Error())
			return
		}
		writeJSON(w, 200, items)
		return
	}
	// Nama media diambil dari daftar kurasi bila feednya dikenal — judul channel
	// RSS terlalu berbeda-beda antar media untuk dipakai sebagai badge kartu.
	name := ""
	for _, src := range news.DefaultSources {
		if src.ID == feed {
			feed, name = src.URL, src.Name
			break
		}
	}
	if !strings.HasPrefix(feed, "http://") && !strings.HasPrefix(feed, "https://") {
		writeErr(w, 400, "unknown feed: use one of the built-in ids, or paste a full feed URL")
		return
	}
	items, err := news.ListFeed(r.Context(), feed, name, max, lang(r))
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, items)
}

// browser menyediakan pembuka halaman berbasis browser untuk paket news.
// Mengembalikan nil bila browser tidak ada — news yang menerjemahkannya jadi
// pesan yang bisa ditindaklanjuti pengguna.
func (s *Server) browser() news.Browser { return Browser(s.paths.Chrome) }

// Browser merakit pembuka halaman dari letak Chrome. Diekspor karena CLI
// membutuhkan yang persis sama — tautan hasil pencarian Google News hanya bisa
// dibuka lewat browser (notes/13).
func Browser(chrome string) news.Browser {
	if chrome == "" {
		return nil
	}
	cap := capture.New(chrome)
	return func(ctx context.Context, url string) (string, error) {
		return cap.DumpDOM(ctx, url, 15000)
	}
}

// newsArticle membaca metadata satu artikel dari URL yang ditempel pengguna.
func (s *Server) newsArticle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL  string `json:"url"`
		Lang string `json:"lang"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	// Pengalih hasil pencarian DIBUKA lebih dulu, di sini — bukan diserahkan ke
	// pemanggilnya.
	//
	// Tautan dari daftar berita menunjuk `news.google.com/rss/articles/CBMi…`,
	// dan mengambilnya apa adanya berarti yang terbaca halaman Google sendiri:
	// judul "Google Berita", gambar logo Google, nol paragraf. Dilaporkan
	// 7 Agustus 2026 sebagai "kartunya tidak ada gambar".
	//
	// Langkah ini sudah ada sebelumnya, tapi HANYA di tombol salin-tautan di
	// GUI — jadi menyalin lalu menempel berhasil sementara menekan Fetch pada
	// tautan yang sama gagal. Satu langkah yang hidup di satu tombol saja adalah
	// langkah yang pasti terlewat di tombol berikutnya; tempatnya di engine.
	//
	// Murah untuk tautan biasa: Resolve mengembalikannya apa adanya bila bukan
	// tautan Google, dan hasil yang sudah pernah dibuka diambil dari cache.
	link, err := news.Resolve(r.Context(), req.URL, s.browser(), s.paths.DataDir)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	a, err := news.FetchArticle(r.Context(), link, firstNonEmpty(req.Lang, lang(r)))
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, a)
}

// newsResolve menerjemahkan pengalih hasil pencarian jadi alamat artikel yang
// sebenarnya, supaya pengguna bisa menyalin tautan yang benar-benar menunjuk ke
// medianya — bukan alamat panjang milik Google.
func (s *Server) newsResolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	original, err := news.Resolve(r.Context(), req.URL, s.browser(), s.paths.DataDir)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"url": original})
}

// newsAnalyze mengambil badan artikel lalu meminta LLM menilai paragraf mana
// yang paling layak jadi isi kartu & caption.
//
// LLM di sini hanya MEMILIH NOMOR paragraf — teksnya diambil engine dari
// artikel. Jadi tidak ada peluang mengarang kalimat, dan hasilnya selalu
// verbatim dari sumbernya.
func (s *Server) newsAnalyze(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
		// Engine + Model = bentuk baku pemilih mesin (notes/39). Nama lama
		// masih diterima supaya halaman yang belum diperbarui tetap jalan.
		Engine      string `json:"engine"`
		Model       string `json:"model"`
		Provider    string `json:"provider"`
		LLMModel    string `json:"llm_model"`
		OllamaModel string `json:"ollama_model"`
		Lang        string `json:"lang"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	content, err := news.FetchContent(r.Context(), req.URL, s.browser(), s.paths.DataDir, firstNonEmpty(req.Lang, lang(r)))
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}

	complete, engineName, err := s.llmEngine(
		firstNonEmpty(req.Engine, req.Provider),
		firstNonEmpty(req.Model, req.LLMModel, req.OllamaModel))
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	selection, err := news.SelectParagraphs(r.Context(), content, complete, engineName, s.paths.DataDir)
	if err != nil {
		// Mesin yang dipilih pengguna dipakai apa adanya — bila gagal, job
		// berhenti dengan pesan akar masalah (notes/12). Tidak ada perpindahan
		// diam-diam ke mesin lain.
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"article":    content.Article,
		"paragraphs": content.Paragraphs,
		"selection":  selection,
	})
}

// llmEngine merangkai satu fungsi pemanggil LLM sesuai mesin yang dipilih.
// llmMaxTokens membatasi panjang balasan. Untuk Ollama ini num_predict — batas
// atas, bukan target, jadi menaikkannya tidak membuat balasan jadi bertele-tele:
// skema JSON yang menghentikannya.
//
// 8192, bukan 4096: artikel 700 kata BESERTA peta klaimnya (tiap kalimat
// disalin ulang dengan nomor sumbernya) melewati 4096 token, dan akibatnya
// bukan artikel yang pendek melainkan JSON yang terpotong di tengah — job
// gagal total. Terjadi di uji lapangan dengan tiga artikel Antara.
const llmMaxTokens = 8192

// makeCard merender kartu jadi PNG dan membalas id-nya.
//
// Sengaja sinkron, tidak lewat antrian job: satu kartu selesai dalam hitungan
// detik, sementara mesin job dirancang untuk pekerjaan hitungan menit.
func (s *Server) makeCard(w http.ResponseWriter, r *http.Request) {
	if s.paths.Chrome == "" {
		writeErr(w, 503, "browser not found: install Chrome/Chromium, or set CLIPPER_CHROME to the chrome.exe path")
		return
	}
	var req struct {
		card.Request
		// Pratinjau menimpa satu folder tetap dan melewati berkas pendamping.
		// Menyetel kartu itu pekerjaan puluhan percobaan; tanpa ini tiap
		// percobaan meninggalkan satu folder permanen.
		Preview bool `json:"preview"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	req.Lang = firstNonEmpty(req.Lang, lang(r))
	img, err := s.inlineCardImage(req.Article.Image)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	req.Article.Image = img
	id := previewID
	if !req.Preview {
		id = fmt.Sprintf("card-%d", time.Now().UnixNano())
	}
	if err := s.cards.Build(r.Context(), req.Request, s.cardDir(id), req.Preview); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if !req.Preview {
		s.sweepCards()
	}
	width, height := card.Dims(req.Ratio)
	writeJSON(w, 201, map[string]any{
		"id": id, "width": width, "height": height,
		"file": "/api/card/" + id + "/file",
		"zip":  "/api/card/" + id + "/zip",
	})
}

func (s *Server) cardDir(id string) string {
	return filepath.Join(s.layout.CardsRoot(), id)
}

// cardFile menyajikan PNG kartu. Id dibatasi pola yang kita buat sendiri
// supaya parameter dari luar tidak bisa menunjuk berkas lain.
func (s *Server) cardFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !cardIDPattern.MatchString(id) {
		writeErr(w, 400, "invalid card id")
		return
	}
	p := filepath.Join(s.cardDir(id), card.FilePNG)
	if _, err := os.Stat(p); err != nil {
		writeErr(w, 404, "card not found")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, p)
}

// cardZip membungkus gambar + caption + keterangan sumber jadi satu berkas.
func (s *Server) cardZip(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !cardIDPattern.MatchString(id) {
		writeErr(w, 400, "invalid card id")
		return
	}
	dir := s.cardDir(id)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		writeErr(w, 404, "card not found")
		return
	}
	// Header ditulis lebih dulu karena zip ditulis langsung ke koneksi
	// (tidak disusun di memori); setelah byte pertama terkirim, status HTTP
	// tidak bisa diubah lagi.
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+id+`.zip"`)

	zw := zip.NewWriter(w)
	defer zw.Close()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		f, err := zw.Create(e.Name())
		if err != nil {
			return
		}
		if _, err := f.Write(raw); err != nil {
			return
		}
	}
}

// Pratinjau memakai satu id tetap dan menimpa dirinya sendiri, jadi menyetel
// kartu berkali-kali tidak meninggalkan satu folder per percobaan.
const previewID = "card-preview"

var cardIDPattern = regexp.MustCompile(`^card-(preview|[0-9]{1,25})$`)

// keepCards = berapa kartu tersimpan yang dipertahankan.
//
// Diukur dari pemakaian sehari: 27 folder / 24 MB dalam satu hari mencoba-coba.
// Tanpa batas, folder data tumbuh selamanya untuk kartu yang sudah lama diunduh
// dan tidak akan dibuka lagi. Pratinjau tidak ikut dihitung — ia hanya satu
// folder yang menimpa dirinya sendiri.
const keepCards = 50

// sweepCards membuang kartu terlama sampai tersisa keepCards.
//
// Kegagalannya sengaja diabaikan: gagal membersihkan bukan alasan untuk
// menggagalkan kartu yang barusan berhasil dibuat.
func (s *Server) sweepCards() {
	dir := s.layout.CardsRoot()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type aged struct {
		name string
		mod  time.Time
	}
	var cards []aged
	for _, e := range entries {
		if !e.IsDir() || e.Name() == previewID || !cardIDPattern.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		cards = append(cards, aged{e.Name(), info.ModTime()})
	}
	if len(cards) <= keepCards {
		return
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].mod.After(cards[j].mod) })
	for _, c := range cards[keepCards:] {
		os.RemoveAll(filepath.Join(dir, c.name))
	}
}

// jsonHasKey melaporkan apakah body memuat objek "parent" yang berisi "key".
//
// Dipakai untuk membedakan field yang TIDAK dikirim dari field yang dikirim
// bernilai sama dengan defaultnya — pembedaan yang mustahil dilakukan setelah
// JSON masuk ke struct Go, sebab keduanya menghasilkan nilai yang sama.
func jsonHasKey(body []byte, parent, key string) bool {
	var outer map[string]json.RawMessage
	if json.Unmarshal(body, &outer) != nil {
		return false
	}
	raw, ok := outer[parent]
	if !ok {
		return false
	}
	var inner map[string]json.RawMessage
	if json.Unmarshal(raw, &inner) != nil {
		return false
	}
	_, ok = inner[key]
	return ok
}

// firstNonEmpty mengembalikan nilai pertama yang tidak kosong.
func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

// upload menerima berkas video (multipart, streaming) → simpan → balas path.
// Untuk drag/drop di GUI. File di-stream ke disk (tidak dimuat ke RAM).
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	reader, err := r.MultipartReader()
	if err != nil {
		writeErr(w, 400, "bukan multipart: "+err.Error())
		return
	}
	if err := os.MkdirAll(s.uploadDir(), 0o755); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeErr(w, 400, "baca part: "+err.Error())
			return
		}
		if part.FormName() != "file" {
			continue
		}
		name := filepath.Base(part.FileName())
		if name == "" || name == "." {
			name = "upload.bin"
		}
		// Satu folder per unggahan: nama aslinya tetap (nama klip diturunkan
		// darinya), dan dua orang yang mengunggah "rekaman.mp4" tidak saling
		// menimpa.
		dir, err := os.MkdirTemp(s.uploadDir(), "")
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		dst := filepath.Join(dir, name)
		f, err := os.Create(dst)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		if _, err := io.Copy(f, part); err != nil {
			f.Close()
			writeErr(w, 500, "simpan file: "+err.Error())
			return
		}
		f.Close()
		abs, _ := filepath.Abs(dst)
		writeJSON(w, 200, map[string]string{"path": abs, "name": name})
		return
	}
	writeErr(w, 400, "no 'file' field found")
}

// Version diisi main saat program mulai; nomor rilisnya sendiri datang dari
// tag lewat -ldflags (lihat .github/workflows/desktop.yml). Ditaruh di sini
// supaya GUI bisa menyebutkannya di panel setelan — pertanyaan "versi berapa
// yang saya jalankan" tidak boleh dijawab dengan membuka terminal.
var Version = "dev"

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "version": Version, "web": s.web})
}

func (s *Server) config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"modes":   []string{"offline", "hybrid"},
		"reframe": []string{"center", "fit"},
		// Batas bawah & titik awal zoom berbeda per mode, jadi dikirim per mode.
		"zoom": map[string]any{
			"max":  config.ZoomMax,
			"step": config.ZoomStep,
			"fit":  map[string]int{"min": config.ZoomWholeMin, "natural": config.ZoomWholeNatural},
			"center": map[string]int{
				"min": config.ZoomCenterMin, "natural": config.ZoomCenterNatural,
			},
		},
		"subtitle_styles":  []string{"plain", "viral"},
		"resolutions":      []string{"720p", "1080p", "1440p"},
		"qualities":        []string{"draft", "hd", "max"},
		"duration_presets": []string{"auto", "30", "60", "90", "120", "180"},
		"transcript_fix":   []string{config.TranscriptFixOn, config.TranscriptFixOff},
		"defaults":         config.DefaultOptions(),
	})
}

// getSettings melaporkan status setelan (apakah API key sudah ada).
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"has_key": s.mgr.HasAPIKey(),
		// Alamat server LLM yang disetel sendiri; kosong = penemuan otomatis.
		// Kuncinya hanya dilaporkan ADA/TIDAK — isinya tidak pernah dikirim
		// balik, sama seperti kunci Claude.
		"llm_url":     os.Getenv("OLLAMA_HOST"),
		"has_llm_key": os.Getenv("LLM_API_KEY") != "",
		// Kosong = mengikuti folder data. GUI menampilkan letak sebenarnya,
		// bukan kekosongan itu, supaya pengguna tahu di mana berkasnya sekarang.
		"clips_dir":      s.layout.ClipsDir,
		"cards_dir":      s.layout.CardsDir,
		"clips_dir_used": s.layout.ClipsRoot(),
		"cards_dir_used": s.layout.CardsRoot(),
	})
}

// postFolders menyimpan tempat klip & kartu disimpan.
//
// Nilai kosong mengembalikannya ke folder data — itu jalan pulang bila pengguna
// memilih folder yang ternyata tidak cocok.
func (s *Server) postFolders(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Clips *string `json:"clips"`
		Cards *string `json:"cards"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	set := func(key string, val *string) error {
		if val == nil {
			return nil
		}
		dir := strings.TrimSpace(*val)
		if dir != "" {
			// Dibuat sekarang juga, lalu diuji dengan menulis: folder yang
			// hanya "ada" belum tentu bisa ditulisi (Program Files, drive
			// jaringan yang hanya-baca), dan kegagalannya harus muncul di sini
			// — bukan setengah jam kemudian saat klip pertama selesai dirender.
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("cannot use that folder: %w", err)
			}
			probe := filepath.Join(dir, ".clipper-write-test")
			if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
				return fmt.Errorf("that folder cannot be written to: %w", err)
			}
			os.Remove(probe)
		}
		if err := writeEnvKey(s.paths.EnvFile, key, dir); err != nil {
			return err
		}
		return os.Setenv(key, dir)
	}
	if err := set("CLIPPER_CLIPS_DIR", req.Clips); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := set("CLIPPER_CARDS_DIR", req.Cards); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	s.applyPaths()
	writeJSON(w, 200, map[string]any{
		"ok":             true,
		"clips_dir":      s.layout.ClipsDir,
		"cards_dir":      s.layout.CardsDir,
		"clips_dir_used": s.layout.ClipsRoot(),
		"cards_dir_used": s.layout.CardsRoot(),
	})
}

// postSettings menyimpan kunci Claude dan alamat/kunci server LLM lokal — di
// memori (env proses) + .env.
//
// Tiap field POINTER, bukan string: GUI menyimpan setelan ini dari tiga tombol
// berbeda, dan badan permintaan yang hanya memuat satu field TIDAK boleh
// mengosongkan dua lainnya. Dengan string biasa, menyimpan alamat server akan
// menghapus API key Claude tanpa ada yang memintanya.
func (s *Server) postSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AnthropicAPIKey *string `json:"anthropic_api_key"`
		LLMURL          *string `json:"llm_url"`
		LLMAPIKey       *string `json:"llm_api_key"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.AnthropicAPIKey != nil {
		key := strings.TrimSpace(*req.AnthropicAPIKey)
		s.mgr.SetAPIKey(key)
		// os.Setenv, bukan cuma menulis berkas: .env adalah sumber kebenaran
		// pemilih mesin (engines.go), dan tanpa ini kunci yang baru disimpan
		// belum terbaca sampai aplikasinya dijalankan ulang.
		_ = os.Setenv("ANTHROPIC_API_KEY", key)
		if key != "" {
			_ = writeEnvKey(s.paths.EnvFile, "ANTHROPIC_API_KEY", key)
		}
	}
	// Alamat disimpan sebagai OLLAMA_HOST, bukan nama sendiri, dan itu disengaja:
	// Candidates() SUDAH membaca variabel itu lebih dulu dan memenangkannya atas
	// semua tebakan. Menambah variabel baru berarti menulis ulang jalur yang
	// sudah bekerja — termasuk untuk CLI, yang membaca .env yang sama.
	//
	// Namanya menyebut Ollama karena begitulah ia lahir; isinya boleh alamat
	// server apa pun (LM Studio, Jan, llama.cpp), dan kindOf yang menentukan
	// protokolnya.
	if req.LLMURL != nil {
		u := strings.TrimRight(strings.TrimSpace(*req.LLMURL), "/")
		if u != "" && !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			u = "http://" + u
		}
		_ = os.Setenv("OLLAMA_HOST", u)
		_ = writeEnvKey(s.paths.EnvFile, "OLLAMA_HOST", u)
		// Kosong = kembali ke penemuan otomatis, dan itu harus terasa SEKARANG.
		ollama.ResetCache()
	}
	if req.LLMAPIKey != nil {
		k := strings.TrimSpace(*req.LLMAPIKey)
		_ = os.Setenv("LLM_API_KEY", k)
		_ = writeEnvKey(s.paths.EnvFile, "LLM_API_KEY", k)
		// Kunci ikut menentukan apakah server MENJAWAB probe sama sekali, jadi
		// cache alamat ikut dibuang.
		ollama.ResetCache()
	}
	writeJSON(w, 200, map[string]any{
		"ok": true, "has_key": s.mgr.HasAPIKey(),
		"llm_url": os.Getenv("OLLAMA_HOST"), "has_llm_key": os.Getenv("LLM_API_KEY") != "",
	})
}

// ollamaStatus memeriksa apakah Ollama jalan & model yang terpasang.
func (s *Server) ollamaStatus(w http.ResponseWriter, r *http.Request) {
	info := ollama.Status(r.Context(), r.URL.Query().Get("url"))
	writeJSON(w, 200, info)
}

// ollamaPull mengunduh model via Ollama (bisa lama). Notifikasi bila gagal.
func (s *Server) ollamaPull(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL   string `json:"url"`
		Model string `json:"model"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.Model == "" {
		writeErr(w, 400, "the 'model' field is required")
		return
	}
	// Unduhan model DIPERLAKUKAN SEPERTI PEMASANGAN KOMPONEN: dijalankan di
	// latar, kemajuannya disiarkan lewat SSE yang sudah ada
	// (/api/requirements/events). Sebelum ini ia dijalankan di dalam
	// `r.Context()` — persis pelanggaran yang notes/25 larang — dan akibatnya
	// terlihat 7 Agustus 2026: gemma2 berhenti di 5,44 GB dari 5,4 GB, berkas
	// parsialnya tergeletak 14 menit, dan tidak ada satu pun tempat yang
	// memberitahu bahwa unduhannya sudah mati.
	id := "llm-model:" + req.Model
	if !s.installs.start(id) {
		writeErr(w, 409, "that model is already being downloaded")
		return
	}
	url, model := req.URL, req.Model
	go func() {
		err := ollama.Pull(context.Background(), url, model, func(p ollama.PullProgress) {
			s.installs.update(id, func(st *InstallState) {
				st.Message = p.Status
				st.Bytes, st.Total = p.Completed, p.Total
				// -1 = besarnya belum diketahui; itu yang membuat GUI
				// menampilkan bilah tak tentu alih-alih 0% yang membeku.
				if p.Total > 0 {
					st.Value = float64(p.Completed) / float64(p.Total)
				} else {
					st.Value = -1
				}
			})
		})
		s.installs.update(id, func(st *InstallState) {
			st.Running, st.Done = false, true
			if err != nil {
				st.Error, st.Message = err.Error(), "Failed"
				return
			}
			st.Value, st.Message = 1, "Installed"
		})
	}()
	writeJSON(w, 202, map[string]any{"id": id, "started": true})
}

// writeEnvKey menambah/mengganti satu KEY=value di berkas .env.
func writeEnvKey(path, key, val string) error {
	// Baris baru dibuang: nilai berisi "\n" menyusupkan KEY=value kedua ke .env,
	// mis. CLIPPER_TOOLS_DIR yang menentukan program mana yang dijalankan.
	val = strings.NewReplacer("\r", "", "\n", "").Replace(val)
	var lines []string
	found := false
	if raw, err := os.ReadFile(path); err == nil {
		for _, ln := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(ln), key+"=") {
				lines = append(lines, key+"="+val)
				found = true
			} else {
				lines = append(lines, ln)
			}
		}
	}
	if !found {
		lines = append(lines, key+"="+val)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600)
}

// fontCatalog = font bawaan proyek. Nama harus cocok dengan family internal
// font, sebab nama itulah yang ditulis ke .ass dan dicari libass.
//
// file menunjuk face TEGAK. Untuk Montserrat itu berkas statis, bukan berkas
// variabel yang juga ada di folder yang sama: yang variabel menyebut dirinya
// family "Montserrat Thin", jadi libass tidak pernah mengenalinya sebagai
// "Montserrat" (lihat fetch-fonts.sh). Yang variabel tetap dipakai kartu berita,
// yang dirender Chrome dan memang memahami sumbu variabel.
//
// bold kosong = font itu hanya punya satu bobot. Anton dan Bebas Neue memang
// begitu — keduanya huruf display yang tidak diterbitkan dalam versi tebal.
var fontCatalog = []struct{ file, bold, name string }{
	{"Montserrat-Regular.ttf", "Montserrat-Bold.ttf", "Montserrat"},
	{"Anton.ttf", "", "Anton"},
	{"BebasNeue.ttf", "", "Bebas Neue"},
}

// fontNameOK = format nama font yang diterima: diawali huruf/angka, lalu
// huruf, angka, spasi, titik, kutip satu, "&", atau "-". Maksimal 64 karakter.
// Sengaja ketat — nama ini masuk ke berkas .ass dan ke argumen fc-match.
var fontNameOK = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 .'&-]{0,63}$`)

// fontResult hasil pengecekan satu nama font.
type fontResult struct {
	Valid  bool   `json:"valid"`
	Name   string `json:"name"`   // nama yang diminta
	Family string `json:"family"` // family yang benar-benar akan dipakai libass
	Source string `json:"source"` // "bundled" | "system"
	Error  string `json:"error"`  // alasan bila tidak valid
	// Bold melaporkan apakah font ini punya face tebal SUNGGUHAN. GUI memakainya
	// untuk tahu apakah pratinjau boleh meminta bobot 700 — kalau tidak ada,
	// browser menebalkan sendiri, persis seperti yang dilakukan libass.
	Bold bool `json:"bold"`
	// Scale = piksel CSS per satuan Fontsize .ass. Pratinjau WAJIB memakainya,
	// kalau tidak teksnya tampil jauh lebih besar daripada hasil render — dan
	// selisihnya berbeda tiap font (lihat fontmetrics.go). 0 = tidak diketahui.
	Scale float64 `json:"scale"`

	file     string // lokasi berkas tegak (tidak dikirim ke GUI)
	boldFile string // lokasi face tebal; kosong = font satu bobot
}

// listFonts melaporkan font yang tersedia di folder assets/fonts.
func (s *Server) listFonts(w http.ResponseWriter, r *http.Request) {
	type fontInfo struct {
		Name string `json:"name"`
		// Scale dikirim bersama namanya supaya GUI tidak perlu bertanya sekali
		// lagi per font hanya untuk bisa menggambar pratinjau dengan benar.
		Scale float64 `json:"scale"`
	}
	out := []fontInfo{}
	for _, f := range fontCatalog {
		p := filepath.Join(s.paths.FontsDir, f.file)
		if _, err := os.Stat(p); err == nil {
			out = append(out, fontInfo{Name: f.name, Scale: fontScale(p)})
		}
	}
	if len(out) == 0 {
		out = append(out, fontInfo{Name: "Montserrat"}) // fallback
	}
	writeJSON(w, 200, out)
}

// fontCheck memvalidasi nama font yang diketik manual di GUI: formatnya benar
// dan fontnya betul-betul ada (bawaan proyek atau terpasang di sistem).
func (s *Server) fontCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.resolveFont(r.Context(), r.URL.Query().Get("name")))
}

// resolveFont mencari berkas font untuk sebuah nama family.
//
// Urutannya: font bawaan proyek dulu, baru font sistem lewat fontconfig —
// sama dengan urutan yang dipakai libass saat merender (fontsdir lebih dulu).
// fc-match SELALU mengembalikan sesuatu (jatuh ke font pengganti bila tak
// ketemu), jadi nama family hasilnya wajib dibandingkan dengan yang diminta;
// kalau berbeda berarti font itu tidak terpasang.
func (s *Server) resolveFont(ctx context.Context, name string) fontResult {
	name = strings.TrimSpace(name)
	res := fontResult{Name: name}
	if name == "" {
		res.Error = "the font name is empty"
		return res
	}
	if !fontNameOK.MatchString(name) {
		res.Error = "invalid name format: use letters/digits, spaces, dots, ' & or -, at most 64 characters (e.g. \"Poppins\", \"Bebas Neue\")"
		return res
	}
	for _, f := range fontCatalog {
		if strings.EqualFold(f.name, name) {
			p := filepath.Join(s.paths.FontsDir, f.file)
			if _, err := os.Stat(p); err == nil {
				r := fontResult{Valid: true, Name: name, Family: f.name, Source: "bundled", file: p, Scale: fontScale(p)}
				if f.bold != "" {
					b := filepath.Join(s.paths.FontsDir, f.bold)
					if _, err := os.Stat(b); err == nil {
						r.boldFile, r.Bold = b, true
					}
				}
				return r
			}
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "fc-match", "-f", "%{family}\t%{file}", name).Output()
	if err != nil {
		res.Error = "cannot check system fonts (fontconfig/fc-match unavailable): use a bundled font"
		return res
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
	if len(parts) < 2 {
		res.Error = "font not found on this system"
		return res
	}
	// %{family} bisa berisi beberapa alias yang dipisah koma.
	for _, fam := range strings.Split(parts[0], ",") {
		if strings.EqualFold(strings.TrimSpace(fam), name) {
			return fontResult{Valid: true, Name: name, Family: strings.TrimSpace(fam), Source: "system", file: parts[1], Scale: fontScale(parts[1])}
		}
	}
	res.Family = strings.TrimSpace(strings.Split(parts[0], ",")[0])
	res.Error = fmt.Sprintf("font %q is not installed: subtitles will be rendered with %q instead", name, res.Family)
	return res
}

// fontFile menyajikan berkas font agar GUI bisa memuat font asli untuk preview.
// Melayani font bawaan maupun font sistem yang lolos pengecekan.
//
// ?weight=700 meminta face TEBAL. Tanpa itu pratinjau memuat face tegak lalu
// menyuruh browser menebalkannya sendiri — dan penebalan buatan browser tidak
// sama dengan face tebal sungguhan yang dipakai libass saat merender. Selisih
// itu persis yang membuat subtitle hasil render tidak sama dengan pratinjau.
func (s *Server) fontFile(w http.ResponseWriter, r *http.Request) {
	res := s.resolveFont(r.Context(), r.URL.Query().Get("name"))
	if !res.Valid {
		writeErr(w, 404, res.Error)
		return
	}
	if wantsBold(r.URL.Query().Get("weight")) && res.boldFile != "" {
		res.file = res.boldFile
	}
	if ext := strings.ToLower(filepath.Ext(res.file)); ext == ".otf" {
		w.Header().Set("Content-Type", "font/otf")
	} else {
		w.Header().Set("Content-Type", "font/ttf")
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, res.file)
}

// wantsBold membaca parameter ?weight= sebagai "minta face tebal".
//
// Ambangnya 600 mengikuti CSS: di sana 600 (semibold) ke atas sudah dianggap
// tebal, dan GUI memang mengirim angka CSS.
func wantsBold(weight string) bool {
	n, err := strconv.Atoi(strings.TrimSpace(weight))
	return err == nil && n >= 600
}

// probe mengembalikan durasi & dimensi video.
func (s *Server) probe(w http.ResponseWriter, r *http.Request) {
	path := hostPath(r.URL.Query().Get("path"))
	if path == "" {
		writeErr(w, 400, "the 'path' parameter is required")
		return
	}
	ctx := r.Context()
	dur, err := s.ff.Duration(ctx, path)
	if err != nil {
		writeErr(w, 400, "could not read the video: "+err.Error())
		return
	}
	vw, vh, _ := s.ff.Dimensions(ctx, path)
	// has_audio ikut dilaporkan supaya GUI bisa memperingatkan SEBELUM tombol
	// Mulai ditekan: video tanpa suara tidak bisa dikerjakan sama sekali, dan
	// mengetahuinya setelah menunggu ekstraksi gagal adalah waktu yang terbuang.
	hasAudio, _ := s.ff.HasAudio(ctx, path)
	writeJSON(w, 200, map[string]any{"duration": dur, "width": vw, "height": vh, "has_audio": hasAudio})
}

// frame mengembalikan 1 frame (JPEG) 9:16 untuk preview subtitle, disesuaikan
// memakai mode reframe yang sama dengan render — param 'reframe' (default
// center). Mode yang belum tersedia ditolak, bukan diganti diam-diam.
func (s *Server) frame(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	path := hostPath(q.Get("path"))
	if path == "" {
		writeErr(w, 400, "the 'path' parameter is required")
		return
	}
	t, _ := strconv.ParseFloat(q.Get("t"), 64)
	// Preview ringan: paksa 720p 9:16.
	opts := config.DefaultOptions()
	opts.Resolution = "720p"
	if rf := q.Get("reframe"); rf != "" {
		opts.Reframe = config.Reframe(rf)
	}
	// Latar & zoom ikut dikirim supaya preview memakai penempatan yang sama
	// persis dengan render — Validate yang menjepit nilainya.
	opts.Background = q.Get("background")
	opts.Zoom, _ = strconv.Atoi(q.Get("zoom"))
	if err := opts.Validate(); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	tw, th := opts.Dims()
	img, err := s.ff.ExtractFrame(r.Context(), path, t, tw, th, ffmpeg.Layout{
		Mode: string(opts.Reframe), Background: opts.Background, Zoom: opts.Zoom,
	})
	if err != nil || len(img) == 0 {
		writeErr(w, 400, "frame extraction failed")
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(200)
	_, _ = w.Write(img)
}

func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Source struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"source"`
		Options config.Options `json:"options"`
	}

	// Body dibaca ke memori supaya bisa diperiksa dua kali: sekali untuk tahu
	// FIELD MANA yang benar-benar dikirim, sekali untuk mengisi struct.
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, 400, "could not read the request body: "+err.Error())
		return
	}

	// Disemai dengan default LEBIH DULU: decoder JSON hanya menimpa field yang
	// benar-benar ada di body, jadi field yang tidak dikirim mempertahankan
	// defaultnya alih-alih jatuh ke nilai nol Go.
	req.Options = config.DefaultOptions()
	req.Options.Transcriber = savedTranscriber()
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, 400, "invalid JSON body: "+err.Error())
		return
	}

	// Nilai bawaan zoom hanya cocok untuk mode center. Bila klien memilih mode
	// lain TANPA menyebut zoom, dipakai titik awal mode itu — kalau tidak,
	// reframe:"fit" justru menghasilkan gambar terpotong penuh, kebalikan dari
	// arti modenya. Karena itu keberadaan kuncinya diperiksa, bukan nilainya:
	// "tidak dikirim" dan "dikirim 100" harus bisa dibedakan.
	if !jsonHasKey(body, "options", "zoom") {
		req.Options.Zoom = config.NaturalZoom(req.Options.Reframe)
	}

	req.Source.Value = hostPath(req.Source.Value)
	if req.Source.Value == "" {
		writeErr(w, 400, "source.value (video path) is required")
		return
	}
	// Path diperiksa di sini, bukan dibiarkan gagal di ffmpeg belasan detik
	// kemudian: sejak GUI mengirim path alih-alih mengunggah salinan, salah
	// ketik satu huruf adalah kekeliruan yang paling mungkin terjadi.
	if req.Source.Type == "" || req.Source.Type == "path" {
		st, err := os.Stat(req.Source.Value)
		if err != nil {
			writeErr(w, 400, "the video was not found on this computer: "+req.Source.Value)
			return
		}
		if st.IsDir() {
			writeErr(w, 400, "that path is a folder, not a video: "+req.Source.Value)
			return
		}
	}
	// Komponen wajib diperiksa sebelum job dibuat: kalau ffmpeg atau whisper
	// belum ada, pesannya harus menyebut namanya dan ke mana pengguna pergi —
	// bukan galat exec dari dalam pipeline belasan detik kemudian.
	if err := s.missingRequirement(); err != nil {
		writeErr(w, 424, err.Error())
		return
	}
	opts := req.Options
	// Folder klip pilihan pengguna dipakai bila job ini tidak menyebut folder
	// sendiri. Pilihan per-job tetap menang: setelan hanyalah nilai bawaan,
	// bukan pagar.
	// Di mode web folder keluaran bukan pilihan klien: path itu di SERVER.
	if opts.OutputDir == "" || s.web {
		opts.OutputDir = s.layout.ClipsDir
	}
	// Daftar istilah dibersihkan di sini, bukan di klien, supaya GUI dan CLI
	// memperlakukan masukan yang sama persis: klien boleh mengirim satu string
	// berisi koma maupun daftar yang sudah dipecah.
	opts.Terms = correct.ParseTerms(strings.Join(opts.Terms, ","))
	// Koordinat mesin diisi DI SINI, dari daftar mesin yang sama dengan tab
	// kartu berita dan pembuat berita (notes/39). Pipeline menerima alamat,
	// jalur, dan NAMA variabel kuncinya — bukan kuncinya, dan bukan tabel
	// penyedia kedua.
	fillEngine(&opts)
	fillTranscriber(&opts)
	// Path banner menempuh pemeriksaan yang sama dengan path video: ia juga
	// datang dari klien dan juga berakhir sebagai argumen ffmpeg.
	if opts.Watermark.Image != "" {
		opts.Watermark.Image = hostPath(opts.Watermark.Image)
		if _, err := checkImage(opts.Watermark.Image); err != nil {
			writeErr(w, 400, err.Error())
			return
		}
	}
	if err := opts.Validate(); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	j := s.mgr.Create(req.Source.Value, opts)
	snap := j.Snapshot()
	writeJSON(w, 201, snap)
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	jobs := s.mgr.List()
	out := make([]job.JobView, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, j.Snapshot())
	}
	writeJSON(w, 200, out)
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	j, ok := s.mgr.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "job not found")
		return
	}
	snap := j.Snapshot()
	writeJSON(w, 200, snap)
}

// jobLog mengembalikan log yang ditulis engine untuk satu job.
//
// Inilah yang membuat kotak log di GUI selamat dari berpindah tab: isinya tidak
// lagi hidup HANYA sebagai state React, melainkan dibaca ulang dari berkas.
// Job tanpa berkas log (riwayat dari versi sebelum ini) menjawab daftar kosong,
// bukan 404 — halaman yang menampilkannya tidak boleh gagal karenanya.
func (s *Server) jobLog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"lines": s.mgr.ReadLog(r.PathValue("id"))})
}

func (s *Server) jobClips(w http.ResponseWriter, r *http.Request) {
	j, ok := s.mgr.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "job not found")
		return
	}
	snap := j.Snapshot()
	writeJSON(w, 200, snap.Clips)
}

func (s *Server) clipFile(w http.ResponseWriter, r *http.Request) {
	j, ok := s.mgr.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "job not found")
		return
	}
	clipID := r.PathValue("clip")
	// ?variant=clean → berkas tanpa subtitle (bila dibuat); ?variant=srt → subtitle;
	// ?variant=txt → ucapan klip tanpa timestamp, bahan caption untuk LLM lain.
	variant := r.URL.Query().Get("variant")
	snap := j.Snapshot()
	for _, cl := range snap.Clips {
		if cl.ID != clipID {
			continue
		}
		path := cl.VideoPath
		switch variant {
		case "clean":
			path = cl.VideoPathRaw
		case "srt":
			path = cl.SubtitleSRT
		case "txt":
			path = cl.TranscriptTXT
		}
		if path != "" {
			http.ServeFile(w, r, path)
			return
		}
	}
	writeErr(w, 404, "the clip is not available yet")
}

func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	if !s.mgr.Cancel(r.PathValue("id")) {
		writeErr(w, 404, "job not found")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "canceled"})
}

// deleteJob membuang satu job dari riwayat (lihat job.Manager.Delete).
func (s *Server) deleteJob(w http.ResponseWriter, r *http.Request) {
	if err := s.mgr.Delete(r.PathValue("id")); err != nil {
		if os.IsNotExist(err) {
			writeErr(w, 404, "job not found")
			return
		}
		writeErr(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"deleted": true})
}

// retryJob: "Coba lagi dari tahap ini" — job baru, setelan sama; tahap yang
// sudah selesai diambil dari cache (job.Manager.Retry).
func (s *Server) retryJob(w http.ResponseWriter, r *http.Request) {
	j, err := s.mgr.Retry(r.PathValue("id"))
	if err != nil {
		writeJobErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"id": j.ID, "status": "queued"})
}

// rerenderJob: "Render ulang yang gagal" — job yang sama, hanya klip gagal.
func (s *Server) rerenderJob(w http.ResponseWriter, r *http.Request) {
	j, err := s.mgr.Rerender(r.PathValue("id"))
	if err != nil {
		writeJobErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"id": j.ID, "status": "queued"})
}

func writeJobErr(w http.ResponseWriter, err error) {
	if errors.Is(err, os.ErrNotExist) {
		writeErr(w, 404, "job not found")
		return
	}
	writeErr(w, 409, err.Error())
}

// jobEvents mengalirkan progres via Server-Sent Events.
func (s *Server) jobEvents(w http.ResponseWriter, r *http.Request) {
	j, ok := s.mgr.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "job not found")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, unsub := j.Subscribe()
	defer unsub()

	// Kirim status awal.
	snap := j.Snapshot()
	writeSSE(w, "progress", snap)
	flusher.Flush()

	// Jika job sudah selesai/gagal/dibatalkan sebelum kita berlangganan,
	// sampaikan event terminalnya sekarang (agar GUI tidak menunggu selamanya).
	switch snap.Status {
	case job.StatusDone:
		for _, cl := range snap.Clips {
			c := cl
			writeSSE(w, "clip", c)
		}
		writeSSE(w, "done", map[string]any{"job_id": snap.ID, "clips": len(snap.Clips)})
		flusher.Flush()
		return
	case job.StatusError:
		writeSSE(w, "error", map[string]string{"message": snap.Error})
		flusher.Flush()
		return
	case job.StatusCanceled:
		writeSSE(w, "error", map[string]string{"message": "Canceled by the user"})
		flusher.Flush()
		return
	}

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			writeSSE(w, ev.Type, ev.Data)
			flusher.Flush()
			if ev.Type == "done" || ev.Type == "error" {
				return
			}
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// --- util ---

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// readJSON mendekode body permintaan. Batas ukurannya sengaja kecil: badan
// permintaan API ini selalu berupa beberapa field pendek, jadi apa pun yang
// lebih besar adalah kesalahan atau penyalahgunaan.
func readJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

func writeSSE(w http.ResponseWriter, event string, data any) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
}

// ollamaPing MENJALANKAN pekerjaan sungguhan pada model, bukan menyapanya.
//
// Bukan sekadar "terpasang atau tidak" — itu sudah dijawab /api/ollama/status
// dari `ollama list`. Sapaan satu kata pun tidak cukup: yang dilaporkan
// berulang kali adalah komputer baru dengan Ollama jalan, model terpasang,
// tombol uji hijau, dan job klip yang tetap berhenti. Karena itu yang dikerjakan
// di sini dua tahap LLM yang sama persis dengan yang dipakai job (lihat
// pipeline.SelfTest) — sekaligus memuat model ke memori, seperti sebelumnya.
func (s *Server) ollamaPing(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Model string `json:"model"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	model := strings.TrimSpace(body.Model)
	if model == "" {
		writeErr(w, 400, "model is required")
		return
	}
	engine, steps := pipeline.SelfTest(r.Context(), "", model)
	ok, total, firstErr := true, int64(0), ""
	for _, st := range steps {
		total += st.MS
		if !st.OK {
			ok = false
			if firstErr == "" {
				firstErr = st.Error
			}
		}
	}
	writeJSON(w, 200, map[string]any{
		"ok": ok, "error": firstErr, "ms": total, "engine": engine, "steps": steps,
	})
}

// deleteClip membuang SEMUA berkas milik satu klip: video berjenis apa pun,
// .srt, dan .txt pendampingnya.
//
// Berkas pendampingnya ikut dibuang, dan itu disengaja: menyisakan .srt tanpa
// videonya berarti folder keluaran penuh yatim yang tidak bisa dipakai siapa
// pun — dan pengguna yang menekan "hapus" bermaksud membuang klipnya, bukan
// sebagian dari klipnya.
func (s *Server) deleteClip(w http.ResponseWriter, r *http.Request) {
	j, ok := s.mgr.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "job not found")
		return
	}
	clipID := r.PathValue("clip")
	snap := j.Snapshot()
	for _, cl := range snap.Clips {
		if cl.ID != clipID {
			continue
		}
		removed := 0
		for _, p := range []string{cl.VideoPath, cl.VideoPathRaw, cl.SubtitleSRT, cl.TranscriptTXT} {
			if p == "" {
				continue
			}
			if err := os.Remove(p); err == nil {
				removed++
			} else if !os.IsNotExist(err) {
				writeErr(w, 500, "could not delete "+filepath.Base(p)+": "+err.Error())
				return
			}
		}
		j.ForgetClip(clipID)
		// Riwayat di disk ikut diperbarui, kalau tidak klip yang barusan
		// dihapus muncul lagi begitu aplikasi dibuka ulang.
		s.mgr.Persist(j.ID)
		writeJSON(w, 200, map[string]any{"ok": true, "removed": removed})
		return
	}
	writeErr(w, 404, "clip not found")
}

// cardEntry adalah satu kartu yang pernah disimpan.
type cardEntry struct {
	ID      string `json:"id"`
	Made    string `json:"made"` // RFC3339
	Bytes   int64  `json:"bytes"`
	File    string `json:"file"`
	Zip     string `json:"zip"`
	Caption string `json:"caption,omitempty"`
	Title   string `json:"title"`
	Source  string `json:"source"`
	Ratio   string `json:"ratio,omitempty"` // 9:16 | 4:5 | 1:1, dari ukuran PNG
}

// sidecarField membaca nilai baris ke-n source.txt ("Label: nilai"). Barisnya
// tetap urutannya (card.writeSidecars): judul, media, tanggal, tautan.
func sidecarField(s string, n int) string {
	lines := strings.Split(s, "\n")
	if n >= len(lines) {
		return ""
	}
	_, v, ok := strings.Cut(lines[n], ": ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

// pngRatio membaca ukuran PNG dari kepalanya saja (IHDR), tanpa memuat gambar.
func pngRatio(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil || cfg.Width == 0 {
		return ""
	}
	switch r := float64(cfg.Height) / float64(cfg.Width); {
	case r > 1.6:
		return "9:16"
	case r > 1.1:
		return "4:5"
	default:
		return "1:1"
	}
}

// listCards membaca folder kartu apa adanya, terbaru dulu.
//
// Tidak ada basis data: kartu memang HANYA berkas di disk, dan menyimpan indeks
// terpisah berarti dua kebenaran yang bisa berbeda begitu pengguna menghapus
// sesuatu lewat file manager.
func (s *Server) listCards(w http.ResponseWriter, r *http.Request) {
	// Akar yang SAMA dengan makeCard: folder kartu bisa dipindah pengguna, dan
	// dulu riwayat tetap membaca DataDir/cards sehingga kartunya tak terlihat.
	root := s.layout.CardsRoot()
	dirs, err := os.ReadDir(root)
	if err != nil {
		writeJSON(w, 200, []cardEntry{}) // belum pernah menyimpan satu pun
		return
	}
	out := make([]cardEntry, 0, len(dirs))
	for _, d := range dirs {
		if !d.IsDir() || d.Name() == "card-preview" {
			continue
		}
		png := filepath.Join(root, d.Name(), "card.png")
		st, err := os.Stat(png)
		if err != nil {
			continue
		}
		e := cardEntry{
			ID:    d.Name(),
			Made:  st.ModTime().UTC().Format(time.RFC3339),
			Bytes: st.Size(),
			File:  "/api/card/" + d.Name() + "/file",
			Zip:   "/api/card/" + d.Name() + "/zip",
		}
		if b, err := os.ReadFile(filepath.Join(root, d.Name(), "caption.txt")); err == nil {
			e.Caption = trimLine(string(b))
		}
		if b, err := os.ReadFile(filepath.Join(root, d.Name(), "source.txt")); err == nil {
			e.Title, e.Source = sidecarField(string(b), 0), sidecarField(string(b), 1)
		}
		e.Ratio = pngRatio(png)
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Made > out[j].Made })
	writeJSON(w, 200, out)
}

func (s *Server) deleteCard(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Nama folder kartu dibuat engine sendiri, tapi ia tetap datang dari alamat
	// permintaan — jadi diperiksa, bukan dipercaya.
	if id == "" || strings.ContainsAny(id, `/\.`) {
		writeErr(w, 400, "invalid card id")
		return
	}
	dir := filepath.Join(s.layout.CardsRoot(), id)
	if err := os.RemoveAll(dir); err != nil {
		writeErr(w, 500, "could not delete the card: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// trimLine mengambil baris pertama yang tidak kosong, dipendekkan.
func trimLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			if len(t) > 160 {
				return t[:160] + "…"
			}
			return t
		}
	}
	return ""
}
