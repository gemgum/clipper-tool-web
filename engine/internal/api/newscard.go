package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gemgum/clipper/engine/internal/httpx"

	"github.com/gemgum/clipper/engine/internal/card"
	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/news"
	"github.com/gemgum/clipper/engine/internal/watermark"
	"github.com/gemgum/clipper/engine/internal/writer"
)

// Bagian engine dari rancangan ulang News cards (DESIGN-NEWSCARD.md):
// unggah foto sendiri, foto lokal di kartu, dan teks kartu yang ditulis AI.

// cardImageMax = batas ukuran foto unggahan. Foto kartu cuma setinggi 900 px;
// lebih dari ini hampir pasti berkas mentah kamera yang salah pilih.
const cardImageMax = 15 << 20

// cardImageTypes = jenis yang diterima, dibaca dari ISI berkas (bukan nama).
var cardImageTypes = map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}

func (s *Server) cardImageDir() string { return filepath.Join(s.uploadDir(), "images") }

// newsImage menyimpan foto unggahan pengguna untuk kartu. Balasannya path
// absolut, yang dikirim balik sebagai article.image (lihat inlineCardImage).
func (s *Server) newsImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, cardImageMax+1<<20)
	reader, err := r.MultipartReader()
	if err != nil {
		writeErr(w, 400, "not a multipart upload: "+err.Error())
		return
	}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeErr(w, 400, "could not read the upload: "+err.Error())
			return
		}
		if part.FormName() != "file" {
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(part, cardImageMax+1))
		if err != nil {
			writeErr(w, 400, "could not read the upload: "+err.Error())
			return
		}
		if len(raw) > cardImageMax {
			writeErr(w, 413, "the image is larger than 15 MB")
			return
		}
		ext, ok := cardImageTypes[http.DetectContentType(raw)]
		if !ok {
			writeErr(w, 415, "only PNG, JPEG or WebP images can be used on a card")
			return
		}
		if err := os.MkdirAll(s.cardImageDir(), 0o755); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		var rnd [12]byte
		_, _ = rand.Read(rnd[:])
		dst := filepath.Join(s.cardImageDir(), hex.EncodeToString(rnd[:])+ext)
		if err := os.WriteFile(dst, raw, 0o644); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		abs, _ := filepath.Abs(dst)
		writeJSON(w, 200, map[string]string{"path": abs})
		return
	}
	writeErr(w, 400, "no 'file' field found")
}

// inlineCardImage menerima article.image: alamat http(s) dibiarkan, path lokal
// HANYA bila berada di folder unggahan foto, dan ditanam sebagai data URI —
// Chrome headless tidak membaca path, dan path di luar folder itu bukan urusan
// kartu (siapa pun yang memegang halaman bisa saja menyodorkan /etc/…).
// burnCardMark membakar setelan Mark ke satu gambar kartu: src = PNG kartu
// (layout card), atau foto artikel (layout box; alamat web diunduh dulu). Format
// bingkai mengikuti rasio kartu. Mengembalikan kode HTTP bila gagal.
func (s *Server) burnCardMark(ctx context.Context, mark config.Watermark, font, ratio, src, out string) (int, error) {
	mark.Format = card.FormatOf(ratio)
	opts, code, err := s.watermarkSetup(mark, font, "", watermark.FitCrop)
	if err != nil {
		return code, err
	}
	file, cleanup, err := s.cardPhotoFile(ctx, src)
	if err != nil {
		return 400, err
	}
	defer cleanup()
	if err := watermark.BurnTo(ctx, watermark.Options{Watermark: opts.Watermark, Subtitle: opts.Subtitle, Fit: watermark.FitCrop},
		s.ff, s.paths, file, out); err != nil {
		return 422, fmt.Errorf("could not put the mark on the card: %v", err)
	}
	return 0, nil
}

// cardPhotoFile = berkas lokal untuk sebuah gambar kartu: alamat web lewat
// salinan di cache server (newsPhoto, diunduh sekali); path lokal dipakai apa
// adanya bila ia PNG kartu kita atau unggahan kartu. cleanup kini selalu kosong
// — salinannya dibersihkan sweepNewsPhotos, bukan per render.
func (s *Server) cardPhotoFile(ctx context.Context, src string) (string, func(), error) {
	none := func() {}
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		abs, err := filepath.Abs(src)
		if err != nil {
			return "", none, fmt.Errorf("invalid image path")
		}
		for _, root := range []string{s.cardImageDir(), s.layout.CardsRoot()} {
			if within(root, abs) {
				return abs, none, nil
			}
		}
		return "", none, fmt.Errorf("the card image must be a web address or an image uploaded for cards")
	}
	file, err := s.newsPhoto(ctx, src)
	return file, none, err
}

// Foto berita (alamat web) disimpan SEKALI di data/newsimg/<sha url>/:
// photo.jpg = aslinya (ekstensi .jpg supaya watermark.IsImage membacanya
// sebagai foto; ffmpeg sendiri membaca isinya), w240.jpg / w1080.jpg = versi
// kecil untuk GUI. Gambar mini & pratinjau tidak lagi diunduh browser dalam
// ukuran penuh dari situs berita, dan Chrome tidak mengunduh ulang tiap render.
const newsPhotoKeep = 7 * 24 * time.Hour

func (s *Server) newsPhotoDir(url string) string {
	sum := sha256.Sum256([]byte(url))
	return filepath.Join(s.paths.DataDir, "newsimg", hex.EncodeToString(sum[:10]))
}

// newsPhoto = path salinan lokal foto berita url (diunduh bila belum ada).
func (s *Server) newsPhoto(ctx context.Context, url string) (string, error) {
	dir := s.newsPhotoDir(url)
	file := filepath.Join(dir, "photo.jpg")
	if _, err := os.Stat(file); err == nil {
		return file, nil
	}
	raw, _, err := httpx.GetImage(ctx, url, 25<<20)
	if err != nil {
		return "", fmt.Errorf("could not download the article photo: %v", err)
	}
	// Situs yang membalas halaman HTML (404, dinding login) tidak boleh masuk
	// cache sebagai "foto": ia akan disajikan rusak sampai seminggu.
	if mime := http.DetectContentType(raw); strings.HasPrefix(mime, "text/") {
		return "", fmt.Errorf("the article photo address did not return an image (%s)", mime)
	}
	if err := writeFileAtomic(dir, file, raw); err != nil {
		return "", err
	}
	go s.sweepNewsPhotos()
	return file, nil
}

// newsImg menyajikan foto berita yang diperkecil (lebar 240 untuk gambar mini,
// 1080 untuk pratinjau) dari cache server. Satu alamat = isi tetap, jadi
// browser boleh menyimpannya seminggu.
func (s *Server) newsImg(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		writeErr(w, 400, "'url' must be a web address")
		return
	}
	width := 240
	if r.URL.Query().Get("w") == "1080" {
		width = 1080
	}
	photo, err := s.newsPhoto(r.Context(), url)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	out := filepath.Join(filepath.Dir(photo), fmt.Sprintf("w%d.jpg", width))
	if _, err := os.Stat(out); err != nil {
		if err := s.thumbFile(r.Context(), photo, 0, width, out); err != nil {
			writeErr(w, 422, fmt.Sprintf("could not read the article photo: %v", err))
			return
		}
	}
	w.Header().Set("Cache-Control", "private, max-age=604800")
	http.ServeFile(w, r, out)
}

// renderImage = gambar kartu untuk Chrome (Pratinjau hasil / Simpan): alamat
// web diganti salinan lokalnya sebagai data URI, unggahan lewat inlineCardImage.
// Pratinjau di bingkai TIDAK memakai ini — HTML-nya harus tetap kecil.
func (s *Server) renderImage(ctx context.Context, img string) (string, error) {
	if !strings.HasPrefix(img, "http://") && !strings.HasPrefix(img, "https://") {
		return s.inlineCardImage(img)
	}
	file, err := s.newsPhoto(ctx, img)
	if err != nil {
		return img, nil // tidak terunduh: biarkan Chrome mencobanya sendiri
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return img, nil
	}
	mime := http.DetectContentType(raw)
	if _, ok := cardImageTypes[mime]; !ok {
		return img, nil
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

// sweepNewsPhotos membuang salinan foto berita yang lebih tua dari seminggu
// (paling sering sejam sekali).
func (s *Server) sweepNewsPhotos() {
	now := time.Now().Unix()
	last := newsPhotoSwept.Load()
	if now-last < 3600 || !newsPhotoSwept.CompareAndSwap(last, now) {
		return
	}
	root := filepath.Join(s.paths.DataDir, "newsimg")
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > newsPhotoKeep {
			os.RemoveAll(filepath.Join(root, e.Name()))
		}
	}
}

var newsPhotoSwept atomic.Int64

// writeFileAtomic menulis ke nama sementara lalu mengganti nama: dua permintaan
// yang sama sekaligus tidak pernah membaca berkas setengah jadi.
func writeFileAtomic(dir, file string, raw []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.%d.tmp", file, time.Now().UnixNano())
	err := os.WriteFile(tmp, raw, 0o644)
	if err == nil {
		err = os.Rename(tmp, file)
	}
	if err != nil {
		os.Remove(tmp) // tulisan setengah jadi tidak boleh tertinggal
	}
	return err
}

func (s *Server) inlineCardImage(img string) (string, error) {
	if img == "" || strings.HasPrefix(img, "http://") || strings.HasPrefix(img, "https://") {
		return img, nil
	}
	abs, err := filepath.Abs(img)
	if err != nil {
		return "", fmt.Errorf("invalid image path")
	}
	if !within(s.cardImageDir(), abs) {
		return "", fmt.Errorf("the card image must be a web address or an image uploaded for cards")
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("the uploaded image is gone: upload it again")
	}
	mime := http.DetectContentType(raw)
	if _, ok := cardImageTypes[mime]; !ok {
		return "", fmt.Errorf("only PNG, JPEG or WebP images can be used on a card")
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

// inlineIllustrations = inlineCardImage untuk tiap foto lanjutan carousel.
func (s *Server) inlineIllustrations(req *card.Request) error {
	for i := range req.Illustrations {
		img, err := s.inlineCardImage(req.Illustrations[i].Image)
		if err != nil {
			return err
		}
		req.Illustrations[i].Image = img
	}
	return nil
}

// newsWrite: "Ringkas otomatis" / "Tulis otomatis" (DESIGN-NEWSCARD §5.2).
// Mesin dipilih seperti analyze; galatnya dikembalikan apa adanya (502).
func (s *Server) newsWrite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string `json:"url"`
		Engine string `json:"engine"`
		Model  string `json:"model"`
		Lang   string `json:"lang"`
		Kind   string `json:"kind"`
		// Text = badan berita yang sudah ada di GUI (isi carousel, paragraf
		// dipisah baris baru). Diisi = ditulis ulang dari teks itu, tanpa
		// mengunduh artikelnya lagi; Title/Source melengkapinya.
		Text   string `json:"text"`
		Title  string `json:"title"`
		Source string `json:"source"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.Kind != writer.KindSummary && req.Kind != writer.KindCaption && req.Kind != writer.KindStory {
		writeErr(w, 400, fmt.Sprintf("kind must be %q, %q or %q", writer.KindSummary, writer.KindCaption, writer.KindStory))
		return
	}
	ln := firstNonEmpty(req.Lang, lang(r))
	var content news.Content
	if strings.TrimSpace(req.Text) != "" {
		content.Article = news.Article{Title: req.Title, Source: req.Source, URL: req.URL}
		for _, p := range strings.Split(req.Text, "\n") {
			if p = strings.TrimSpace(p); p != "" {
				content.Paragraphs = append(content.Paragraphs, news.Paragraph{Index: len(content.Paragraphs), Text: p})
			}
		}
	} else {
		var err error
		if content, err = news.FetchContent(r.Context(), req.URL, s.browser(), s.paths.DataDir, ln); err != nil {
			writeErr(w, 502, err.Error())
			return
		}
	}
	complete, engineName, err := EngineFor(req.Engine, req.Model)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	out, err := writer.WriteCardText(r.Context(), complete, engineName, content, req.Kind, ln)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"text": out.Text, "hashtags": nonNil(out.Hashtags), "violations": out.Violations, "engine": engineName,
	})
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
