package api

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Wajib per INTEGRASI-APLIKASI §6: setiap route punya izin, dan setiap izin
// yang diperiksa ada di daftar yang dikirim ke Sarth Admin.
func TestRoutePerms(t *testing.T) {
	sent := map[string]bool{}
	for _, p := range permissions {
		sent[p.Key] = true
	}
	var routes []string
	for _, oidc := range []bool{false, true} {
		s := &Server{}
		s.paths.DataDir = t.TempDir()
		s.SetWeb("x")
		if oidc {
			s.auth = &oidcAuth{}
		}
		s.Handler()
		routes = append(routes, s.routes...)
	}
	t.Cleanup(func() { webRoot = "" })
	for _, r := range routes {
		perm, ok := routePerms[r]
		if !ok {
			t.Errorf("route %q has no permission in routePerms", r)
		} else if perm != permPublic && perm != permAnyone && !sent[perm] {
			t.Errorf("route %q checks %q, which is not in permissions", r, perm)
		}
	}
}

// Login dari ujung ke ujung melawan IdP + Sarth Admin palsu: callback,
// verifikasi id_token, izin per route, Origin, dan penolakan token aplikasi lain.
func TestOIDCFlow(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	var idp *httptest.Server
	idToken := func(aud, nonce string, roles ...string) string {
		r := map[string]any{}
		for _, x := range roles {
			r[x] = map[string]string{"org": "d"}
		}
		h, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "k1"})
		c, _ := json.Marshal(map[string]any{
			"iss": idp.URL, "aud": []string{aud}, "sub": "u1", "name": "Ana", "email": "a@x",
			"nonce": nonce, "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), rolesClaim: r,
		})
		in := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c)
		sum := sha256.Sum256([]byte(in))
		sig, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		return in + "." + base64.RawURLEncoding.EncodeToString(sig)
	}
	var nextToken string
	idp = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]string{"issuer": idp.URL, "authorization_endpoint": idp.URL + "/authorize",
				"token_endpoint": idp.URL + "/token", "end_session_endpoint": idp.URL + "/end", "jwks_uri": idp.URL + "/keys"})
		case "/keys":
			json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "k1",
				"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
		case "/token":
			if id, sec, _ := r.BasicAuth(); id != "clipper" || sec != "s3cret" || r.FormValue("code_verifier") == "" {
				w.WriteHeader(401)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"id_token": nextToken, "refresh_token": "r"})
		case "/api/v1/apps/clipper/permissions":
			if r.Header.Get("Authorization") != "Bearer sync" {
				w.WriteHeader(401)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"roles": map[string][]string{"viewer": {"card.view"}},
				"risk": map[string]int{"card.delete": 1}})
		}
	}))
	defer idp.Close()

	s := &Server{}
	s.paths.DataDir = t.TempDir()
	s.SetWeb("")
	t.Cleanup(func() { webRoot = "" })
	s.SetOIDC(OIDCConfig{Issuer: idp.URL, ClientID: "clipper", ClientSecret: "s3cret", PublicURL: "https://clip.example.com/",
		AdminURL: idp.URL, Slug: "clipper", SyncToken: "sync", SessionSecret: strings.Repeat("k", 32)})
	h := s.Handler()

	do := func(method, path, origin string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader("{}"))
		r.Host = "clip.example.com"
		r.Header.Set("Content-Type", "application/json")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		for _, c := range cookies {
			r.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}
	login := func(aud string, roles ...string) *httptest.ResponseRecorder {
		rec := do("GET", "/auth/login", "")
		loc, _ := url.Parse(rec.Header().Get("Location"))
		q := loc.Query()
		if rec.Code != 302 || q.Get("prompt") != "select_account" || q.Get("code_challenge_method") != "S256" {
			t.Fatalf("/auth/login: %d %s", rec.Code, loc)
		}
		nextToken = idToken(aud, q.Get("nonce"), roles...)
		return do("GET", "/auth/callback?code=c&state="+q.Get("state"), "", rec.Result().Cookies()[0])
	}

	if rec := do("GET", "/", ""); rec.Code != 302 || rec.Header().Get("Location") != "/auth/login" {
		t.Fatalf("page without session: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := do("GET", "/api/jobs", ""); rec.Code != 401 {
		t.Fatalf("API without session: %d, want 401", rec.Code)
	}
	if rec := login("other-app", "viewer"); rec.Code != 401 {
		t.Fatalf("token for another app: %d, want 401", rec.Code)
	}
	if rec := login("clipper"); rec.Code != 403 {
		t.Fatalf("no roles: %d, want 403", rec.Code)
	}
	rec := login("clipper", "viewer")
	if rec.Code != 302 || len(rec.Result().Cookies()) < 2 {
		t.Fatalf("callback: %d", rec.Code)
	}
	var sid *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sidCookie {
			sid = c
		}
	}
	if sid == nil || !sid.HttpOnly || !sid.Secure {
		t.Fatalf("session cookie: %+v", sid)
	}
	if rec := do("GET", "/api/cards", "", sid); rec.Code != 200 {
		t.Fatalf("viewer GET /api/cards: %d, want 200", rec.Code)
	}
	if rec := do("DELETE", "/api/cards/x", "https://clip.example.com", sid); rec.Code != 403 {
		t.Fatalf("viewer DELETE: %d, want 403", rec.Code)
	}
	if rec := do("POST", "/auth/logout", "https://evil.example", sid); rec.Code != 403 {
		t.Fatalf("cross-site POST: %d, want 403", rec.Code)
	}
	if rec := do("GET", "/auth/me", "", sid); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"permissions":["card.view"]`) {
		t.Fatalf("/auth/me: %d %s", rec.Code, rec.Body)
	}
	rec = do("POST", "/auth/logout", "https://clip.example.com", sid)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "post_logout_redirect_uri=https%3A%2F%2Fclip.example.com") {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body)
	}
	if rec := do("GET", "/api/jobs", "", sid); rec.Code != 401 {
		t.Fatalf("after logout: %d, want 401", rec.Code)
	}
}
