package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gemgum/clipper/engine/internal/score/ollama"
	"github.com/gemgum/clipper/engine/internal/writer"
)

// PostJob keadaan satu job pembuat berita — bentuk job latar yang sama dengan
// pembuat caption, dibedakan hanya oleh tipe hasilnya (bgjob.go).
type PostJob = bgJob[writer.Result]

// postsDir = tempat artikel disimpan. Selalu di bawah DataDir, tidak pernah
// disusun dari akar proyek (CLAUDE.md).
func (s *Server) postsDir() string { return filepath.Join(s.paths.DataDir, "posts") }

// createPost memulai job penulisan lalu LANGSUNG menjawab.
//
// Satu job memanggil LLM (jumlah artikel + 1) kali; pada model lokal itu
// hitungan menit. Menunggunya di dalam permintaan HTTP berarti job mati saat
// pengguna pindah halaman — kesalahan yang pernah dibayar mahal (notes/25).
func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URLs []string `json:"urls"`
		// Engine + Model = bentuk baku pemilih mesin (notes/39). Nama lama
		// masih diterima supaya halaman yang belum diperbarui tetap jalan.
		Engine string `json:"engine"`
		Model  string `json:"model"`
		// Mesin tahap MENULIS, bila berbeda. Kosong = memakai yang di atas.
		WriteEngine string `json:"write_engine"`
		WriteModel  string `json:"write_model"`
		Provider    string `json:"provider"`
		OllamaModel string `json:"ollama_model"`
		LLMModel    string `json:"llm_model"`
		Lang        string `json:"lang"`
		MaxWords    int    `json:"max_words"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if len(req.URLs) == 0 {
		writeErr(w, 400, "the 'urls' field is required: add at least one source article")
		return
	}

	read, readName, err := EngineFor(
		firstNonEmpty(req.Engine, req.Provider),
		firstNonEmpty(req.Model, req.OllamaModel, req.LLMModel))
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	// Mesin tahap menulis dirakit HANYA bila diminta: kosong berarti kedua
	// tahap memakai mesin yang sama, dan itu tetap jalan yang paling umum.
	var write writer.Completer
	writeName := readName
	if req.WriteEngine != "" {
		write, writeName, err = EngineFor(req.WriteEngine, req.WriteModel)
		if err != nil {
			writeErr(w, 400, err.Error())
			return
		}
	}

	// Job berjalan di context.Background() TURUNAN — bukan r.Context(): ia harus
	// hidup setelah permintaan HTTP-nya selesai (notes/25), tapi tetap punya
	// kenop untuk dihentikan.
	ctx, cancel := context.WithCancel(context.Background())
	job := s.posts.create("post", cancel)
	deps := writer.Deps{
		Read:        read,
		ReadEngine:  readName,
		Write:       write,
		WriteEngine: writeName,
		Browse:      s.browser(),
		CacheDir:    s.paths.DataDir,
		OutDir:      s.postsDir(),
	}
	opts := writer.Options{URLs: req.URLs, Lang: req.Lang, MaxWords: req.MaxWords}

	go func() {
		defer cancel()
		res, err := writer.Run(ctx, opts, deps, func(p writer.Progress) {
			s.posts.update(job.ID, func(j *PostJob) {
				j.Stage, j.Progress = p.Stage, p.Value
				if p.Message != "" {
					j.Log = append(j.Log, p.Message)
				}
			})
		})
		s.posts.finish(job.ID, ctx, res, err)
	}()

	writeJSON(w, 202, map[string]any{"id": job.ID, "engine": readName, "write_engine": writeName, "started": true})
}

// postLimits menyebut berapa artikel sumber yang muat untuk mesin PENULIS yang
// dipilih.
//
// Ditanyakan sebelum job dimulai, bukan dilaporkan sebagai galat sesudah lima
// menit menunggu: batasnya bergantung pada jendela konteks model, dan itu hal
// yang engine tahu sementara GUI tidak. Mesin cloud tidak punya angka konteks
// yang bisa ditanyakan — dan memang tidak perlu, semuanya jauh di atas ambang.
func (s *Server) postLimits(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("engine")
	model := r.URL.Query().Get("model")
	d, ok := engineByID(id)
	if !ok {
		writeJSON(w, 200, map[string]any{"max_sources": writer.MaxSources})
		return
	}
	ctxTokens := 0
	if d.Kind == kindLocal {
		e := resolve(d)
		if model == "" {
			model = e.Model
		}
		ctxTokens = ollama.ContextOf(r.Context(), e.BaseURL, model)
	}
	writeJSON(w, 200, map[string]any{
		"max_sources": writer.MaxSourcesFor(ctxTokens),
		"context":     ctxTokens,
	})
}

func (s *Server) cancelPost(w http.ResponseWriter, r *http.Request) {
	if !s.posts.stop(r.PathValue("id")) {
		writeErr(w, 404, "no such post job")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "canceled"})
}

func (s *Server) listPosts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"posts": s.posts.snapshot()})
}

func (s *Server) getPost(w http.ResponseWriter, r *http.Request) {
	j, ok := s.posts.get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "no such post job")
		return
	}
	writeJSON(w, 200, j)
}

// postFile menyajikan satu berkas dari folder artikel.
//
// Nama berkasnya dari daftar tetap, bukan dari permintaan: apa pun yang berasal
// dari luar dan dipakai menyusun path adalah jalan keluar dari folder itu.
func (s *Server) postFile(w http.ResponseWriter, r *http.Request) {
	j, ok := s.posts.get(r.PathValue("id"))
	if !ok || j.Result == nil {
		writeErr(w, 404, "no such post job, or it has not finished yet")
		return
	}
	var path, mime string
	switch r.URL.Query().Get("name") {
	case "docx":
		// Dibuat saat diminta dari draf yang tersimpan (DESIGN-Clipper-Lanjutan §3).
		heading := "Sources"
		if lang(r) == "id" {
			heading = "Sumber"
		}
		raw, err := writer.Docx(j.Result.Draft, j.Result.Sources, heading)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		w.Header().Set("Content-Disposition", `attachment; filename="article.docx"`)
		_, _ = w.Write(raw)
		return
	case "", "article":
		path, mime = j.Result.Post.Markdown, "text/markdown; charset=utf-8"
	case "sources":
		path, mime = j.Result.Post.Sources, "application/json"
	case "image":
		path, mime = j.Result.Post.Image, ""
	default:
		writeErr(w, 400, "unknown file: use name=article, name=docx, name=sources or name=image")
		return
	}
	if path == "" {
		writeErr(w, 404, "that file was not written for this article")
		return
	}
	if _, err := os.Stat(path); err != nil {
		writeErr(w, 404, "file not found")
		return
	}
	if mime != "" {
		w.Header().Set("Content-Type", mime)
	}
	http.ServeFile(w, r, path)
}

// postEvents mengalirkan kemajuan SEMUA job penulisan.
func (s *Server) postEvents(w http.ResponseWriter, r *http.Request) {
	s.posts.stream(w, r, "post")
}
