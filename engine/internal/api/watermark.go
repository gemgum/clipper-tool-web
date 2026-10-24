package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gemgum/clipper/engine/internal/card"
	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/ffmpeg"
	"github.com/gemgum/clipper/engine/internal/subtitle"
	"github.com/gemgum/clipper/engine/internal/watermark"
)

// WatermarkJob keadaan satu job watermark (bentuknya di bgjob.go).
type WatermarkJob = bgJob[watermark.Result]

// maxWatermarkVideos = batas berkas dalam satu job.
//
// Alasannya sama dengan maxCaptionVideos: bukan batas teknis, melainkan penahan
// salah pilih. Bedanya di sini tiap berkas berarti satu ENCODE penuh — memilih
// seluruh isi folder Videos di halaman ini jauh lebih mahal daripada di halaman
// caption, jadi batasnya lebih rapat.
const maxWatermarkVideos = 20

// createWatermark memulai job watermark lalu LANGSUNG menjawab.
//
// Membakar satu video panjang berarti meng-encode ulang seluruhnya; menunggunya
// di dalam permintaan HTTP berarti job mati saat pengguna pindah halaman
// (notes/25).
func (s *Server) createWatermark(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Videos    []string         `json:"videos"`
		Watermark config.Watermark `json:"watermark"`
		Font      string           `json:"font"`
		Quality   string           `json:"quality"`
		OutDir    string           `json:"out_dir"`
		Fit       string           `json:"fit"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if len(req.Videos) == 0 {
		writeErr(w, 400, "the 'videos' field is required: pick at least one video")
		return
	}
	if len(req.Videos) > maxWatermarkVideos {
		writeErr(w, 400, fmt.Sprintf("%d videos in one go is more than the %d this page takes: run it in batches",
			len(req.Videos), maxWatermarkVideos))
		return
	}
	// Path diperiksa di sini, bukan dibiarkan gagal di ffmpeg belasan detik
	// kemudian — sama seperti halaman caption dan halaman klip.
	for i, v := range req.Videos {
		v = hostPath(v)
		req.Videos[i] = v
		st, err := os.Stat(v)
		if err != nil {
			writeErr(w, 400, "the video was not found on this computer: "+v)
			return
		}
		if st.IsDir() {
			writeErr(w, 400, "that path is a folder, not a video: "+v)
			return
		}
	}
	opts, code, err := s.watermarkSetup(req.Watermark, req.Font, req.Quality, req.Fit)
	if err != nil {
		writeErr(w, code, err.Error())
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := s.watermarks.create("watermark", cancel)
	bopts := watermark.Options{
		Videos:    req.Videos,
		Watermark: opts.Watermark,
		Subtitle:  opts.Subtitle,
		OutDir:    hostPath(req.OutDir),
		Quality:   opts.Quality,
		Fit:       req.Fit,
	}

	go func() {
		defer cancel()
		res, err := watermark.Run(ctx, bopts, s.ff, s.paths, func(pr watermark.Progress) {
			s.watermarks.update(job.ID, func(j *WatermarkJob) {
				j.Stage, j.Progress = pr.Stage, pr.Value
				if pr.Message != "" {
					j.Log = append(j.Log, pr.Message)
				}
			})
		})
		s.watermarks.finish(job.ID, ctx, res, err)
		if err == nil && ctx.Err() == nil {
			dropUploads(req.Videos...)
		}
	}()

	writeJSON(w, 202, map[string]any{"id": job.ID, "videos": len(req.Videos), "started": true})
}

func (s *Server) listWatermarks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"jobs": s.watermarks.snapshot()})
}

func (s *Server) getWatermark(w http.ResponseWriter, r *http.Request) {
	j, ok := s.watermarks.get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "watermark job not found")
		return
	}
	writeJSON(w, 200, j)
}

func (s *Server) watermarkEvents(w http.ResponseWriter, r *http.Request) {
	s.watermarks.stream(w, r, "watermark")
}

// watermarkFile menyajikan satu video hasil, ?i= menunjuk urutannya di job.
//
// Di desktop hasilnya cukup ditulis di sebelah videonya; di mode web berkas itu
// ada di server, jadi inilah satu-satunya jalan untuk mengambilnya.
func (s *Server) watermarkFile(w http.ResponseWriter, r *http.Request) {
	j, ok := s.watermarks.get(r.PathValue("id"))
	if !ok || j.Result == nil {
		writeErr(w, 404, "no such watermark job, or it has not finished yet")
		return
	}
	i, err := strconv.Atoi(r.URL.Query().Get("i"))
	if err != nil || i < 0 || i >= len(j.Result.Files) || j.Result.Files[i].Output == "" {
		writeErr(w, 404, "no output for that video")
		return
	}
	out := j.Result.Files[i].Output
	w.Header().Set("Content-Disposition",
		mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(out)}))
	http.ServeFile(w, r, out)
}

// watermarkSetup memeriksa setelan watermark yang sama untuk job DAN pratinjau
// hasil: logo ada & berupa gambar, ffmpeg terpasang, nilai dijepit Validate.
// Mengembalikan kode HTTP untuk galatnya.
func (s *Server) watermarkSetup(wm config.Watermark, font, quality, fit string) (config.Options, int, error) {
	if fit != "" && fit != watermark.FitCrop && fit != watermark.FitWhole {
		return config.Options{}, 400, fmt.Errorf("unknown fit %q: choose %q or %q", fit, watermark.FitCrop, watermark.FitWhole)
	}
	if wm.Image != "" {
		wm.Image = hostPath(wm.Image)
		if _, err := checkImage(wm.Image); err != nil {
			return config.Options{}, 400, err
		}
	}
	for i := range wm.Logos {
		l := &wm.Logos[i]
		l.Image = hostPath(l.Image)
		if _, err := checkImage(l.Image); err != nil {
			return config.Options{}, 400, err
		}
	}
	// ffmpeg saja yang dibutuhkan halaman ini — tidak ada transkripsi, jadi
	// whisper & modelnya tidak boleh ikut menghalangi.
	if s.paths.FFmpeg == "" {
		return config.Options{}, 424, fmt.Errorf("ffmpeg is not installed yet: open the Requirements page")
	}
	opts := config.DefaultOptions()
	opts.Watermark = wm
	if font != "" {
		opts.Subtitle.Font = font
	}
	if quality != "" {
		opts.Quality = quality
	}
	if err := opts.Validate(); err != nil {
		return config.Options{}, 400, err
	}
	return opts, 0, nil
}

// watermarkPreview membakar satu frame berkas yang diminta persis seperti hasil
// akhirnya (watermark.Preview) dan menyajikannya sebagai JPEG.
//
// Ditunggu di dalam permintaan, beda dengan job: satu frame cuma sedetik-dua,
// dan pengguna memang sedang menunggu gambarnya.
func (s *Server) watermarkPreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File      string           `json:"file"`
		Watermark config.Watermark `json:"watermark"`
		Font      string           `json:"font"`
		Fit       string           `json:"fit"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	file := hostPath(req.File)
	if st, err := os.Stat(file); file == "" || err != nil || st.IsDir() {
		writeErr(w, 400, "the file was not found: "+req.File)
		return
	}
	opts, code, err := s.watermarkSetup(req.Watermark, req.Font, "", req.Fit)
	if err != nil {
		writeErr(w, code, err.Error())
		return
	}
	img, err := watermark.Preview(r.Context(), watermark.Options{
		Watermark: opts.Watermark, Subtitle: opts.Subtitle, Fit: req.Fit,
	}, s.ff, s.paths, file)
	if err != nil {
		writeErr(w, 422, "preview failed: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(img)
}

// watermarkLook = warna tampilan kartu berita (panel, judul, lencana) untuk satu
// tema kartu berita atau satu "#rrggbb". Tema "photo" diturunkan dari bingkai berkas pertama; tanpa
// berkas (atau bingkainya gagal dibaca) ia memakai warna cadangan temanya.
func (s *Server) watermarkLook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Theme  string `json:"theme"`
		File   string `json:"file"`
		Format string `json:"format"`
		CardY  int    `json:"card_y"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	auto := req.Theme == "auto"
	// Foto artikel News (alamat web / foto unggahan kartu): dibaca lewat card,
	// bukan ffmpeg — tongkat ajaib warna kotak di halaman News.
	if auto {
		if url, err := s.inlineCardImage(req.File); err == nil && url != "" {
			if c, ok := s.cards.AutoLookURL(r.Context(), url); ok {
				writeJSON(w, 200, c)
				return
			}
		}
	}
	var img image.Image
	if file := hostPath(req.File); (auto || req.Theme == card.ThemePhoto) && file != "" {
		fw, fh := config.FrameSize(req.Format)
		// auto membaca AREA GAMBAR kartu (potongan yang sama dengan render);
		// photo membaca seluruh bingkai.
		if auto {
			fh = config.CardPhotoPx(req.Format, req.CardY)
		}
		// Detik 1 untuk video (detik 0 sering hitam); foto hanya punya detik 0.
		for _, t := range []float64{1, 0} {
			raw, err := s.ff.ExtractFrame(r.Context(), file, t, 270, 270*fh/fw/2*2, ffmpeg.Layout{Mode: string(config.ReframeCenter)})
			if err == nil && len(raw) > 0 {
				img, _, _ = image.Decode(bytes.NewReader(raw))
				break
			}
		}
	}
	if auto {
		if img == nil {
			writeErr(w, 400, "auto colour needs a video or photo in the list")
			return
		}
		writeJSON(w, 200, card.AutoLook(img))
		return
	}
	c, ok := card.Look(req.Theme, img)
	if !ok {
		writeErr(w, 400, "unknown look: "+req.Theme)
		return
	}
	writeJSON(w, 200, c)
}

// watermarkLayout = tata letak kotak judul berita untuk pratinjau.
//
// Dihitung engine dengan metrik font asli (subtitle.NewsBox), BUKAN ditiru di
// GUI: pemenggalan baris yang ditebak di dua tempat adalah cara kotak di
// pratinjau berbeda dari hasil render (notes/44).
func (s *Server) watermarkLayout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Watermark config.Watermark `json:"watermark"`
		Font      string           `json:"font"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	opts := config.DefaultOptions()
	opts.Watermark = req.Watermark
	if req.Font != "" {
		opts.Subtitle.Font = req.Font
	}
	if err := opts.Validate(); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	wm := opts.Watermark
	wm.Headline.Font, wm.Headline.FontsDir = opts.Subtitle.Font, s.paths.FontsDir
	if wm.Headline.Card {
		writeJSON(w, 200, subtitle.CardBox(wm, wm.Headline.Text))
		return
	}
	writeJSON(w, 200, subtitle.NewsBox(wm, wm.Headline.Text))
}

// --- template watermark ---
//
// Template = setelan halaman watermark yang disimpan dengan nama (format, logo,
// gaya judul, kredit…), supaya memakainya tinggal memasukkan video dan teks.
// Disimpan di SERVER, satu daftar untuk semua akun: di versi web itu yang
// membuat satu tim memakai identitas yang sama. Isinya milik GUI — engine cuma
// menyimpannya — jadi bentuknya bebas berubah tanpa migrasi di sini.
// ponytail: satu berkas JSON + mutex; pindah ke berkas per template bila
// daftarnya ratusan.

type watermarkTemplate struct {
	Name  string          `json:"name"`
	Data  json.RawMessage `json:"data"`
	Saved time.Time       `json:"saved"`
	// Default = template yang otomatis terpasang di setiap kartu News baru
	// (paling banyak satu). Diatur lewat setDefaultTemplate, bukan saveTemplate.
	Default bool `json:"default,omitempty"`
}

var templateMu sync.Mutex

func (s *Server) templateFile() string {
	return filepath.Join(s.paths.DataDir, "templates", "watermark.json")
}

func (s *Server) readTemplates() []watermarkTemplate {
	out := []watermarkTemplate{}
	if b, err := os.ReadFile(s.templateFile()); err == nil {
		_ = json.Unmarshal(b, &out)
	}
	return out
}

func (s *Server) writeTemplates(list []watermarkTemplate) error {
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.templateFile()), 0o755); err != nil {
		return err
	}
	// Ditulis ke nama sementara lalu diganti nama: aplikasi yang mati di
	// tengah penulisan tidak meninggalkan daftar setengah jadi.
	tmp := s.templateFile() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.templateFile())
}

func (s *Server) listTemplates(w http.ResponseWriter, r *http.Request) {
	templateMu.Lock()
	defer templateMu.Unlock()
	writeJSON(w, 200, map[string]any{"templates": s.readTemplates()})
}

// saveTemplate menyimpan, atau MENIMPA template bernama sama.
func (s *Server) saveTemplate(w http.ResponseWriter, r *http.Request) {
	var req watermarkTemplate
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len([]rune(req.Name)) > 60 {
		writeErr(w, 400, "a template needs a name of 1 to 60 characters")
		return
	}
	var obj map[string]any
	if len(req.Data) > 64<<10 || json.Unmarshal(req.Data, &obj) != nil {
		writeErr(w, 400, "the template settings must be a JSON object under 64 KB")
		return
	}
	req.Saved = time.Now()
	templateMu.Lock()
	defer templateMu.Unlock()
	list := s.readTemplates()
	found := false
	req.Default = false
	for i := range list {
		if list[i].Name == req.Name {
			// Menimpa template tidak mencabut tanda bawaannya.
			req.Default = list[i].Default
			list[i], found = req, true
		}
	}
	if !found {
		list = append(list, req)
	}
	if err := s.writeTemplates(list); err != nil {
		writeErr(w, 500, "could not save the template: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"templates": list})
}

// setDefaultTemplate menjadikan satu template bawaan (yang lain dicabut), atau
// mencabutnya bila ia sudah bawaan.
func (s *Server) setDefaultTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	templateMu.Lock()
	defer templateMu.Unlock()
	list := s.readTemplates()
	found := false
	for i := range list {
		on := list[i].Name == name && !list[i].Default
		found = found || list[i].Name == name
		list[i].Default = on
	}
	if !found {
		writeErr(w, 404, "no template with that name")
		return
	}
	if err := s.writeTemplates(list); err != nil {
		writeErr(w, 500, "could not save the templates: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"templates": list})
}

func (s *Server) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	templateMu.Lock()
	defer templateMu.Unlock()
	list := s.readTemplates()
	kept := list[:0]
	for _, t := range list {
		if t.Name != name {
			kept = append(kept, t)
		}
	}
	if len(kept) == len(list) {
		writeErr(w, 404, "no template with that name")
		return
	}
	if err := s.writeTemplates(kept); err != nil {
		writeErr(w, 500, "could not save the templates: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"templates": kept})
}
