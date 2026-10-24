package api

import (
	"net/http"
	"strings"

	"github.com/gemgum/clipper/engine/internal/news"
)

// Crawl berita (notes/45): status & kunci untuk halaman Requirements.

func (s *Server) crawlState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.crawl.State())
}

// crawlNow = "Crawl now" di Requirements: memakai kredit, jadi hanya untuk
// yang boleh mengubah setelan, dan tetap satu putaran sekali jalan.
func (s *Server) crawlNow(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 202, map[string]any{"started": s.crawl.MaybeStart(true)})
}

// crawlKeys menyimpan kunci API ke .env. Field yang tidak dikirim tidak
// disentuh; string kosong = hapus kunci.
func (s *Server) crawlKeys(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serper     *string `json:"serper"`
		SociaVault *string `json:"sociavault"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.Serper != nil {
		s.setEnv(news.EnvSerper, strings.TrimSpace(*req.Serper))
	}
	if req.SociaVault != nil {
		s.setEnv(news.EnvSociaVault, strings.TrimSpace(*req.SociaVault))
	}
	writeJSON(w, 200, s.crawl.State())
}
