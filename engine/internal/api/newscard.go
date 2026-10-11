package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gemgum/clipper/engine/internal/news"
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
