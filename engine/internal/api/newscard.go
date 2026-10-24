package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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

// cardPhotoFile = berkas lokal untuk sebuah gambar kartu: alamat web diunduh
// ke berkas sementara .jpg (dihapus cleanup; ekstensi itulah yang membuat
// watermark.IsImage membacanya sebagai foto — ffmpeg sendiri membaca isinya);
// path lokal dipakai apa adanya bila ia PNG kartu kita atau unggahan kartu.
func (s *Server) cardPhotoFile(ctx context.Context, src string) (string, func(), error) {
	none := func() {}
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		abs, err := filepath.Abs(src)
		if err != nil {
			return "", none, fmt.Errorf("invalid image path")
		}
		for _, root := range []string{s.cardImageDir(), s.layout.CardsRoot()} {
			r, _ := filepath.Abs(root)
			if rel, err := filepath.Rel(r, abs); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
				return abs, none, nil
			}
		}
		return "", none, fmt.Errorf("the card image must be a web address or an image uploaded for cards")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", src, nil)
	if err != nil {
		return "", none, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Clipper)")
	res, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", none, fmt.Errorf("could not download the article photo: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", none, fmt.Errorf("could not download the article photo: HTTP %d", res.StatusCode)
	}
	f, err := os.CreateTemp(filepath.Join(s.paths.DataDir, "cache"), "card-photo-*.jpg")
	if err != nil {
		if err = os.MkdirAll(filepath.Join(s.paths.DataDir, "cache"), 0o755); err == nil {
			f, err = os.CreateTemp(filepath.Join(s.paths.DataDir, "cache"), "card-photo-*.jpg")
		}
		if err != nil {
			return "", none, err
		}
	}
	_, err = io.Copy(f, io.LimitReader(res.Body, 25<<20))
	f.Close()
	if err != nil {
		os.Remove(f.Name())
		return "", none, err
	}
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}

func (s *Server) inlineCardImage(img string) (string, error) {
	if img == "" || strings.HasPrefix(img, "http://") || strings.HasPrefix(img, "https://") {
		return img, nil
	}
	abs, err := filepath.Abs(img)
	if err != nil {
		return "", fmt.Errorf("invalid image path")
	}
	root, _ := filepath.Abs(s.cardImageDir())
	if rel, err := filepath.Rel(root, abs); err != nil || rel == "." || strings.HasPrefix(rel, "..") {
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

// newsWrite: "Ringkas otomatis" / "Tulis otomatis" (DESIGN-NEWSCARD §5.2).
// Mesin dipilih seperti analyze; galatnya dikembalikan apa adanya (502).
func (s *Server) newsWrite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string `json:"url"`
		Engine string `json:"engine"`
		Model  string `json:"model"`
		Lang   string `json:"lang"`
		Kind   string `json:"kind"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.Kind != writer.KindSummary && req.Kind != writer.KindCaption {
		writeErr(w, 400, fmt.Sprintf("kind must be %q or %q", writer.KindSummary, writer.KindCaption))
		return
	}
	ln := firstNonEmpty(req.Lang, lang(r))
	content, err := news.FetchContent(r.Context(), req.URL, s.browser(), s.paths.DataDir, ln)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
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
