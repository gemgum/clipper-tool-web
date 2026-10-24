package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Izin per route (INTEGRASI-APLIKASI §4.4 & §6).
//
// SATU sumber kebenaran: `permissions` adalah daftar yang dikirim ke Sarth
// Admin, `routePerms` memetakan setiap route ke salah satunya. TestRoutePerms
// gagal bila ada route tanpa izin atau izin yang tidak ada di daftar.
//
// Route yang belum dipetakan → 403 + log, supaya fitur baru tidak otomatis
// terbuka.

type permission struct {
	Key       string `json:"key"`
	Nama      string `json:"nama"`
	Deskripsi string `json:"deskripsi,omitempty"`
	Risk      int    `json:"risk,omitempty"`
}

var permissions = []permission{
	{Key: "clip.view", Nama: "View clips"},
	{Key: "clip.create", Nama: "Make clips"},
	{Key: "clip.control", Nama: "Cancel clip jobs"},
	{Key: "clip.delete", Nama: "Delete clips", Risk: 1},
	{Key: "card.view", Nama: "View news cards"},
	{Key: "card.create", Nama: "Make news cards"},
	{Key: "card.delete", Nama: "Delete news cards", Risk: 1},
	{Key: "article.view", Nama: "View articles"},
	{Key: "article.create", Nama: "Write articles"},
	{Key: "article.control", Nama: "Cancel article jobs"},
	{Key: "article.delete", Nama: "Delete articles", Risk: 1},
	{Key: "caption.view", Nama: "View captions"},
	{Key: "caption.create", Nama: "Make captions"},
	{Key: "caption.control", Nama: "Cancel caption jobs"},
	{Key: "caption.delete", Nama: "Delete captions", Risk: 1},
	{Key: "watermark.view", Nama: "View watermarked videos"},
	{Key: "watermark.create", Nama: "Watermark videos"},
	{Key: "watermark.control", Nama: "Cancel watermark jobs"},
	{Key: "watermark.delete", Nama: "Delete watermarked videos", Risk: 1},
	{Key: "settings.view", Nama: "View settings"},
	{Key: "settings.manage", Nama: "Change settings & AI keys", Deskripsi: "Shared by the whole team", Risk: 1},
}

const (
	permPublic = "-" // tanpa sesi sama sekali
	permAnyone = "*" // semua yang login
)

var routePerms = map[string]string{
	"GET /api/health":    permPublic,
	"GET /auth/login":    permPublic,
	"GET /auth/callback": permPublic,
	"GET /auth/signout":  permPublic,
	"GET /auth/me":       permAnyone,
	"POST /auth/logout":  permAnyone,
	"/":                  permAnyone, // GUI statis

	// Dipakai semua halaman (pemilih mesin, pratinjau, unggahan, berita RSS).
	"GET /api/config":              permAnyone,
	"GET /api/models":              permAnyone,
	"GET /api/engines":             permAnyone,
	"POST /api/engines/test":       permAnyone,
	"GET /api/engines/{id}/models": permAnyone,
	"GET /api/settings/ai":         permAnyone,
	"GET /api/ollama/status":       permAnyone,
	"POST /api/ollama/ping":        permAnyone,
	"GET /api/fonts":               permAnyone,
	"GET /api/font-file":           permAnyone,
	"GET /api/font-check":          permAnyone,
	"GET /api/probe":               permAnyone,
	"GET /api/frame":               permAnyone,
	"GET /api/thumb":               permAnyone,
	"GET /api/media":               permAnyone,
	"GET /api/source":              permAnyone,
	"POST /api/source":             permAnyone,
	"POST /api/upload":             permAnyone,
	"POST /api/upload/remove":      permAnyone,
	"POST /api/fetch":              permAnyone,
	"POST /api/fetch/check":        permAnyone,
	"GET /api/image":               permAnyone,
	"GET /api/news/feeds":          permAnyone,
	"GET /api/news/list":           permAnyone,
	"POST /api/news/article":       permAnyone,
	"POST /api/news/resolve":       permAnyone,

	"GET /api/requirements":          "settings.view",
	"GET /api/requirements/events":   "settings.view",
	"GET /api/settings":              "settings.view",
	"POST /api/requirements/install": "settings.manage",
	"POST /api/requirements/remove":  "settings.manage",
	"POST /api/requirements/path":    "settings.manage",
	"POST /api/engines":              "settings.manage",
	"POST /api/engines/delete":       "settings.manage",
	"POST /api/settings/ai":          "settings.manage",
	"POST /api/settings":             "settings.manage",
	"POST /api/settings/folders":     "settings.manage",
	"POST /api/ollama/pull":          "settings.manage",
	// Dimatikan webOff di mode web; tetap dipetakan supaya tabelnya lengkap.
	"GET /api/browse":                         "settings.manage",
	"POST /api/locate":                        "settings.manage",
	"POST /api/jobs/{id}/clips/{clip}/reveal": "clip.view",

	"POST /api/jobs":                       "clip.create",
	"POST /api/jobs/{id}/retry":            "clip.create",
	"POST /api/jobs/{id}/rerender":         "clip.create",
	"GET /api/jobs":                        "clip.view",
	"GET /api/jobs/{id}":                   "clip.view",
	"GET /api/jobs/{id}/events":            "clip.view",
	"GET /api/jobs/{id}/log":               "clip.view",
	"GET /api/jobs/{id}/clips":             "clip.view",
	"GET /api/jobs/{id}/clips/{clip}/file": "clip.view",
	"POST /api/jobs/{id}/cancel":           "clip.control",
	"DELETE /api/jobs/{id}":                "clip.delete",
	"DELETE /api/jobs/{id}/clips/{clip}":   "clip.delete",
	// ponytail: zip riwayat bisa berisi klip DAN kartu, tapi dijaga clip.view
	// saja; periksa per jenis di bulkZip bila ada role kartu-tanpa-klip.
	"GET /api/download": "clip.view",

	"POST /api/news/analyze":  "card.create",
	"POST /api/news/image":    "card.create",
	"POST /api/news/write":    "card.create",
	"POST /api/card":          "card.create",
	"GET /api/card/{id}/file": "card.view",
	"GET /api/card/{id}/zip":  "card.view",
	"GET /api/cards":          "card.view",
	"DELETE /api/cards/{id}":  "card.delete",

	"POST /api/posts":             "article.create",
	"GET /api/posts":              "article.view",
	"GET /api/posts/events":       "article.view",
	"GET /api/posts/{id}":         "article.view",
	"GET /api/posts/{id}/file":    "article.view",
	"GET /api/posts/limits":       "article.view",
	"POST /api/posts/{id}/cancel": "article.control",
	"DELETE /api/posts/{id}":      "article.delete",

	"POST /api/captions":             "caption.create",
	"GET /api/captions":              "caption.view",
	"GET /api/captions/events":       "caption.view",
	"GET /api/captions/{id}":         "caption.view",
	"GET /api/captions/{id}/file":    "caption.view",
	"POST /api/captions/{id}/cancel": "caption.control",
	"DELETE /api/captions/{id}":      "caption.delete",

	"POST /api/watermark":                    "watermark.create",
	"GET /api/watermark":                     "watermark.view",
	"GET /api/watermark/events":              "watermark.view",
	"POST /api/watermark/layout":             "watermark.view",
	"POST /api/watermark/preview":            "watermark.create",
	"GET /api/watermark/templates":           "watermark.view",
	"POST /api/watermark/templates":          "watermark.create",
	"DELETE /api/watermark/templates/{name}": "watermark.create",
	"GET /api/watermark/{id}":                "watermark.view",
	"GET /api/watermark/{id}/file":           "watermark.view",
	"POST /api/watermark/{id}/cancel":        "watermark.control",
	"DELETE /api/watermark/{id}":             "watermark.delete",

	// Login kata sandi bersama (AUTH_MODE lama); tidak terdaftar saat oidc.
	"POST /api/login": permPublic,
	"GET /login":      permPublic,
}

// gatedMux mencatat setiap pola yang didaftarkan dan membungkusnya dengan
// pemeriksaan izin.
type gatedMux struct {
	*http.ServeMux
	s        *Server
	patterns []string
}

func (m *gatedMux) HandleFunc(p string, h http.HandlerFunc) { m.Handle(p, h) }

func (m *gatedMux) Handle(p string, h http.Handler) {
	m.patterns = append(m.patterns, p)
	m.ServeMux.Handle(p, m.s.gate(p, h))
}

// gate = sesi + izin + Origin untuk satu route. Tanpa OIDC: lewat apa adanya.
func (s *Server) gate(pattern string, h http.Handler) http.Handler {
	a := s.auth
	if a == nil {
		return h
	}
	perm, mapped := routePerms[pattern]
	if perm == permPublic {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// §4.6: SameSite=Lax tidak menahan form multipart lintas situs.
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" &&
			r.Header.Get("Origin") != a.origin() {
			writeErr(w, 403, "request from another site")
			return
		}
		sess := a.session(r)
		if sess == nil {
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/auth/") {
				writeErr(w, 401, "you are signed out: reload the page to sign in again")
			} else {
				http.Redirect(w, r, "/auth/login", http.StatusFound)
			}
			return
		}
		if !mapped {
			log.Printf("access: route %q has no permission; denied", pattern)
			writeErr(w, 403, "this action has no permission yet")
			return
		}
		if perm != permAnyone {
			_, _, roles := sess.snapshot()
			set, risk, ok := a.perms.allowed(a, roles)
			if !ok {
				writeErr(w, 503, "cannot reach Sarth Admin to check your permissions")
				return
			}
			if !set[perm] {
				writeErr(w, 403, "your role does not allow "+perm)
				return
			}
			if risk[perm] >= 1 {
				if err := a.recheck(sess); err != nil {
					a.dropSession(r)
					writeErr(w, 401, "your access was revoked or expired: sign in again")
					return
				}
				// Role bisa berubah setelah recheck.
				_, _, roles = sess.snapshot()
				if set, _, _ = a.perms.allowed(a, roles); !set[perm] {
					writeErr(w, 403, "your role does not allow "+perm)
					return
				}
			}
		}
		h.ServeHTTP(w, r)
	})
}

func (a *oidcAuth) origin() string {
	scheme, rest, _ := strings.Cut(a.cfg.PublicURL, "://")
	host, _, _ := strings.Cut(rest, "/")
	return scheme + "://" + host
}

// --- peta izin dari Sarth Admin ---

type permCache struct {
	mu    sync.Mutex
	roles map[string][]string
	risk  map[string]int
	ok    bool // pernah berhasil
	tried time.Time
}

// allowed = gabungan peta[role] untuk semua role orang itu. Cache 60 detik;
// gagal tarik → cache terakhir; belum pernah berhasil → ok=false (fail closed).
func (c *permCache) allowed(a *oidcAuth, roles []string) (map[string]bool, map[string]int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.tried) > 60*time.Second {
		c.tried = time.Now()
		var m struct {
			Roles map[string][]string `json:"roles"`
			Risk  map[string]int      `json:"risk"`
		}
		if err := a.sarth("GET", nil, &m); err != nil {
			log.Printf("access: cannot fetch permissions from Sarth Admin: %v", err)
		} else {
			c.roles, c.risk, c.ok = m.Roles, m.Risk, true
		}
	}
	if !c.ok {
		return nil, nil, false
	}
	set := map[string]bool{}
	for _, r := range roles {
		for _, p := range c.roles[r] {
			set[p] = true
		}
	}
	return set, c.risk, true
}

// SyncPermissions mengirim daftar fitur tanpa menyalakan login OIDC — untuk
// masa transisi: §5 mewajibkan daftar fitur sudah sampai & matriks diisi
// SEBELUM AUTH_MODE=oidc, kalau tidak semua orang terkunci.
func SyncPermissions(cfg OIDCConfig) {
	cfg.AdminURL = strings.TrimRight(cfg.AdminURL, "/")
	a := &oidcAuth{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}}
	go a.syncPermissions()
}

// syncPermissions mengirim SELURUH daftar fitur (§4.4). Aman diulang.
func (a *oidcAuth) syncPermissions() {
	var res struct {
		Added   []string `json:"added"`
		Removed []string `json:"removed"`
		Total   int      `json:"total"`
	}
	if err := a.sarth("PUT", map[string]any{"permissions": permissions}, &res); err != nil {
		log.Printf("access: cannot send permissions to Sarth Admin (will retry on next start): %v", err)
		return
	}
	log.Printf("access: sent %d permissions to Sarth Admin (added %v, removed %v)", res.Total, res.Added, res.Removed)
}

// sarth memanggil /api/v1/apps/{slug}/permissions dengan token sinkron.
func (a *oidcAuth) sarth(method string, body, out any) error {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, a.cfg.AdminURL+"/api/v1/apps/"+a.cfg.Slug+"/permissions", rd)
	req.Header.Set("Authorization", "Bearer "+a.cfg.SyncToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		var e struct{ Error string }
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("HTTP %d %s", resp.StatusCode, e.Error)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
