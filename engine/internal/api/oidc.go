package api

import (
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math/big"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

// Login lewat Sarth Admin (OIDC) — AUTH_MODE=oidc di mode web.
//
// Kontraknya ada di sarth-admin/docs/INTEGRASI-APLIKASI.md (v1.1.0); nomor §
// di komentar berkas ini menunjuk ke sana. Bentuknya BFF: engine adalah
// confidential client, token tidak pernah sampai ke JavaScript browser — yang
// dipegang browser hanya cookie berisi id sesi acak.
//
// Stdlib saja, sesuai aturan engine: JWT di sini cuma RS256 (yang ditandatangani
// Zitadel), dan itu cukup ditulis dengan crypto/rsa.

// OIDCConfig = blok .env dari panel Sarth Admin (§4.8). Jangan ditulis tangan.
type OIDCConfig struct {
	Issuer, ClientID, ClientSecret string
	PublicURL                      string // {APP}, tanpa garis miring di akhir
	AdminURL, Slug, SyncToken      string
	SessionSecret                  string
}

const (
	sidCookie   = "clipper_sid"
	loginCookie = "clipper_login"
	sessionTTL  = 12 * time.Hour
	// clockSkew = toleransi exp/iat (§4.2).
	clockSkew = 60 * time.Second
	// recheckAfter = umur cek keadaan akun sebelum aksi ⚠ (§4.5).
	recheckAfter = 60 * time.Second
	rolesClaim   = "urn:zitadel:iam:org:project:roles"
)

type session struct {
	mu                  sync.Mutex
	sub, name, email    string
	roles               []string
	refresh, idToken    string
	checkedAt, expireAt time.Time
}

type oidcAuth struct {
	cfg    OIDCConfig
	client *http.Client

	discMu sync.Mutex
	disc   *discovery
	keys   map[string]*rsa.PublicKey

	sessMu   sync.Mutex
	sessions map[string]*session

	perms permCache
}

type discovery struct {
	Issuer        string `json:"issuer"`
	Authorization string `json:"authorization_endpoint"`
	Token         string `json:"token_endpoint"`
	EndSession    string `json:"end_session_endpoint"`
	JWKS          string `json:"jwks_uri"`
}

// SetOIDC menyalakan login Sarth Admin. Dipanggil SETELAH SetWeb.
func (s *Server) SetOIDC(cfg OIDCConfig) {
	cfg.PublicURL = strings.TrimRight(cfg.PublicURL, "/")
	cfg.AdminURL = strings.TrimRight(cfg.AdminURL, "/")
	s.auth = &oidcAuth{
		cfg:      cfg,
		client:   &http.Client{Timeout: 15 * time.Second},
		sessions: map[string]*session{},
	}
	if u, err := url.Parse(cfg.PublicURL); err == nil {
		s.AllowHost(u.Hostname())
	}
	// Daftar fitur dikirim tiap engine menyala (§4.4). Gagal = peringatan saja.
	go s.auth.syncPermissions()
}

// --- discovery & JWKS ---

func (a *oidcAuth) discover() (*discovery, error) {
	a.discMu.Lock()
	defer a.discMu.Unlock()
	if a.disc != nil {
		return a.disc, nil
	}
	var d discovery
	if err := a.getJSON(strings.TrimRight(a.cfg.Issuer, "/")+"/.well-known/openid-configuration", &d); err != nil {
		return nil, fmt.Errorf("sign-in server unreachable: %w", err)
	}
	if d.Issuer != a.cfg.Issuer {
		return nil, fmt.Errorf("sign-in server says its issuer is %q, OIDC_ISSUER is %q", d.Issuer, a.cfg.Issuer)
	}
	a.disc = &d
	return a.disc, nil
}

// key mengembalikan kunci publik kid; JWKS dimuat ulang bila kid tak dikenal
// (IdP memutar kuncinya).
func (a *oidcAuth) key(kid string) (*rsa.PublicKey, error) {
	d, err := a.discover()
	if err != nil {
		return nil, err
	}
	a.discMu.Lock()
	defer a.discMu.Unlock()
	if k := a.keys[kid]; k != nil {
		return k, nil
	}
	var set struct {
		Keys []struct{ Kty, Kid, N, E string } `json:"keys"`
	}
	if err := a.getJSON(d.JWKS, &set); err != nil {
		return nil, fmt.Errorf("cannot load signing keys: %w", err)
	}
	a.keys = map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if k.Kty != "RSA" || err1 != nil || err2 != nil {
			continue
		}
		a.keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if k := a.keys[kid]; k != nil {
		return k, nil
	}
	return nil, fmt.Errorf("unknown signing key %q", kid)
}

func (a *oidcAuth) getJSON(u string, v any) error {
	resp, err := a.client.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s: HTTP %d", u, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// --- id_token (§4.2) ---

type idClaims struct {
	Iss   string   `json:"iss"`
	Aud   audience `json:"aud"`
	Azp   string   `json:"azp"`
	Sub   string   `json:"sub"`
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Nonce string   `json:"nonce"`
	Exp   int64    `json:"exp"`
	Iat   int64    `json:"iat"`
	// Bentuknya { "operator": { "<orgId>": "<domain>" } }; yang dipakai key-nya.
	Roles map[string]json.RawMessage `json:"urn:zitadel:iam:org:project:roles"`
}

// audience: klaim aud boleh string atau array.
type audience []string

func (a *audience) UnmarshalJSON(b []byte) error {
	var one string
	if json.Unmarshal(b, &one) == nil {
		*a = audience{one}
		return nil
	}
	return json.Unmarshal(b, (*[]string)(a))
}

// verify memeriksa SEMUA yang diwajibkan §4.2. nonce kosong = tidak dicek
// (id_token dari refresh tidak wajib membawanya).
func (a *oidcAuth) verify(raw, nonce string, now time.Time) (*idClaims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed id_token")
	}
	var hdr struct{ Alg, Kid string }
	if err := decodeSegment(parts[0], &hdr); err != nil {
		return nil, err
	}
	if hdr.Alg != "RS256" {
		return nil, fmt.Errorf("unsupported id_token algorithm %q", hdr.Alg)
	}
	key, err := a.key(hdr.Kid)
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("malformed id_token signature")
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig) != nil {
		return nil, errors.New("id_token signature is invalid")
	}
	var c idClaims
	if err := decodeSegment(parts[1], &c); err != nil {
		return nil, err
	}
	switch {
	case c.Iss != a.cfg.Issuer:
		return nil, errors.New("id_token from another issuer")
	// Token untuk aplikasi lain di IdP yang sama bertanda tangan sah juga.
	case !slices.Contains(c.Aud, a.cfg.ClientID), c.Azp != "" && c.Azp != a.cfg.ClientID:
		return nil, errors.New("id_token was issued to another app")
	case nonce != "" && subtle.ConstantTimeCompare([]byte(c.Nonce), []byte(nonce)) != 1:
		return nil, errors.New("id_token nonce mismatch")
	case now.After(time.Unix(c.Exp, 0).Add(clockSkew)):
		return nil, errors.New("id_token expired")
	case time.Unix(c.Iat, 0).After(now.Add(clockSkew)):
		return nil, errors.New("id_token issued in the future")
	}
	return &c, nil
}

func decodeSegment(seg string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return errors.New("malformed id_token")
	}
	return json.Unmarshal(b, v)
}

func (c *idClaims) roleKeys() []string {
	var out []string
	for k := range c.Roles {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// --- token endpoint ---

type tokenResponse struct {
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	Error        string `json:"error"`
}

// token memanggil token_endpoint dengan client secret via Basic.
// Pesan error hanya membawa kode error IdP — token tidak pernah masuk log.
func (a *oidcAuth) token(form url.Values) (*tokenResponse, error) {
	d, err := a.discover()
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("POST", d.Token, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// RFC 6749 §2.3.1: id & secret di-urlencode dulu sebelum Basic.
	req.SetBasicAuth(url.QueryEscape(a.cfg.ClientID), url.QueryEscape(a.cfg.ClientSecret))
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var t tokenResponse
	_ = json.NewDecoder(resp.Body).Decode(&t)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token endpoint: HTTP %d %s", resp.StatusCode, t.Error)
	}
	return &t, nil
}

// --- cookie login sementara (state, nonce, PKCE) ---

type loginState struct {
	State, Nonce, Verifier string
	Exp                    int64
}

func (a *oidcAuth) sign(b []byte) string {
	mac := hmac.New(sha256.New, []byte(a.cfg.SessionSecret))
	mac.Write(b)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (a *oidcAuth) sealLogin(st loginState) string {
	b, _ := json.Marshal(st)
	return base64.RawURLEncoding.EncodeToString(b) + "." + a.sign(b)
}

func (a *oidcAuth) openLogin(v string, now time.Time) (*loginState, bool) {
	body, mac, ok := strings.Cut(v, ".")
	b, err := base64.RawURLEncoding.DecodeString(body)
	if !ok || err != nil || !hmac.Equal([]byte(mac), []byte(a.sign(b))) {
		return nil, false
	}
	var st loginState
	if json.Unmarshal(b, &st) != nil || now.Unix() > st.Exp {
		return nil, false
	}
	return &st, true
}

func (a *oidcAuth) cookie(name, value, path string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name: name, Value: value, Path: path, MaxAge: maxAge,
		HttpOnly: true,
		Secure:   strings.HasPrefix(a.cfg.PublicURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	}
}

// --- sesi ---

// ponytail: sesi di memori satu proses — restart = semua login ulang (biasanya
// tanpa mengetik sandi, sesi IdP masih hidup). Pindah ke DB bila >1 instance.
func (a *oidcAuth) session(r *http.Request) *session {
	c, err := r.Cookie(sidCookie)
	if err != nil {
		return nil
	}
	a.sessMu.Lock()
	defer a.sessMu.Unlock()
	s := a.sessions[c.Value]
	if s != nil && time.Now().After(s.expireAt) {
		delete(a.sessions, c.Value)
		return nil
	}
	return s
}

func (a *oidcAuth) dropSession(r *http.Request) *session {
	c, err := r.Cookie(sidCookie)
	if err != nil {
		return nil
	}
	a.sessMu.Lock()
	defer a.sessMu.Unlock()
	s := a.sessions[c.Value]
	delete(a.sessions, c.Value)
	return s
}

func (s *session) snapshot() (name, email string, roles []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name, s.email, slices.Clone(s.roles)
}

// recheck = §4.5: sebelum aksi ⚠, cek akun masih aktif lewat refresh_token.
// Dikunci per sesi: refresh token Zitadel berputar, dua refresh serentak
// dengan token yang sama membuat yang kedua gagal.
func (a *oidcAuth) recheck(s *session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.checkedAt) <= recheckAfter {
		return nil
	}
	t, err := a.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {s.refresh}})
	if err != nil {
		return err
	}
	if t.RefreshToken != "" {
		s.refresh = t.RefreshToken
	}
	if t.IDToken != "" {
		c, err := a.verify(t.IDToken, "", time.Now())
		if err != nil {
			return err
		}
		s.idToken, s.roles = t.IDToken, c.roleKeys()
	}
	s.checkedAt = time.Now()
	return nil
}

// --- endpoint /auth/* (§4.1) ---

func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	a := s.auth
	d, err := a.discover()
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	st := loginState{State: NewToken(), Nonce: NewToken(), Verifier: NewToken(), Exp: time.Now().Add(10 * time.Minute).Unix()}
	http.SetCookie(w, a.cookie(loginCookie, a.sealLogin(st), "/auth/", 600))
	challenge := sha256.Sum256([]byte(st.Verifier))
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {a.cfg.ClientID},
		"redirect_uri":          {a.cfg.PublicURL + "/auth/callback"},
		"scope":                 {"openid profile email offline_access"},
		"state":                 {st.State},
		"nonce":                 {st.Nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
		"prompt":                {"select_account"},
	}
	http.Redirect(w, r, d.Authorization+"?"+q.Encode(), http.StatusFound)
}

func (s *Server) authCallback(w http.ResponseWriter, r *http.Request) {
	a := s.auth
	http.SetCookie(w, a.cookie(loginCookie, "", "/auth/", -1))
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		deniedPage(w, 403, "The sign-in server refused: "+e)
		return
	}
	c, err := r.Cookie(loginCookie)
	if err != nil {
		deniedPage(w, 400, "Sign-in expired. Start again.")
		return
	}
	st, ok := a.openLogin(c.Value, time.Now())
	if !ok || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(st.State)) != 1 {
		deniedPage(w, 400, "Sign-in expired. Start again.")
		return
	}
	t, err := a.token(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {q.Get("code")},
		"redirect_uri":  {a.cfg.PublicURL + "/auth/callback"},
		"code_verifier": {st.Verifier},
	})
	if err != nil {
		deniedPage(w, 502, "Sign-in failed: "+err.Error())
		return
	}
	claims, err := a.verify(t.IDToken, st.Nonce, time.Now())
	if err != nil {
		deniedPage(w, 401, "Sign-in failed: "+err.Error())
		return
	}
	roles := claims.roleKeys()
	if len(roles) == 0 {
		deniedPage(w, 403, "This account has no access to Clipper.")
		return
	}
	now := time.Now()
	name := claims.Name
	if name == "" {
		name = claims.Email
	}
	sid := NewToken()
	a.sessMu.Lock()
	a.sessions[sid] = &session{
		sub: claims.Sub, name: name, email: claims.Email, roles: roles,
		refresh: t.RefreshToken, idToken: t.IDToken,
		checkedAt: now, expireAt: now.Add(sessionTTL),
	}
	a.sessMu.Unlock()
	http.SetCookie(w, a.cookie(sidCookie, sid, "/", int(sessionTTL.Seconds())))
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) authMe(w http.ResponseWriter, r *http.Request) {
	sess := s.auth.session(r)
	if sess == nil {
		writeErr(w, 401, "signed out")
		return
	}
	name, email, roles := sess.snapshot()
	perms := []string{}
	if set, _, ok := s.auth.perms.allowed(s.auth, roles); ok {
		for p := range set {
			perms = append(perms, p)
		}
		slices.Sort(perms)
	}
	writeJSON(w, 200, map[string]any{"name": name, "email": email, "roles": roles, "permissions": perms})
}

// authLogout menjawab URL logout IdP; GUI yang membukanya.
func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	a := s.auth
	sess := a.dropSession(r)
	http.SetCookie(w, a.cookie(sidCookie, "", "/", -1))
	d, err := a.discover()
	if err != nil {
		writeJSON(w, 200, map[string]string{"url": "/"})
		return
	}
	q := url.Values{"post_logout_redirect_uri": {a.cfg.PublicURL}}
	if sess != nil {
		sess.mu.Lock()
		// id_token_hint di URL = RP-Initiated Logout, pengecualian standar (§4.1).
		q.Set("id_token_hint", sess.idToken)
		sess.mu.Unlock()
	} else {
		q.Set("client_id", a.cfg.ClientID)
	}
	writeJSON(w, 200, map[string]string{"url": d.EndSession + "?" + q.Encode()})
}

// authSignout = jalan keluar dari halaman "akses ditolak": orang yang salah
// akun harus bisa mengakhiri sesi IdP-nya juga.
func (s *Server) authSignout(w http.ResponseWriter, r *http.Request) {
	a := s.auth
	a.dropSession(r)
	http.SetCookie(w, a.cookie(sidCookie, "", "/", -1))
	d, err := a.discover()
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	q := url.Values{"client_id": {a.cfg.ClientID}, "post_logout_redirect_uri": {a.cfg.PublicURL}}
	http.Redirect(w, r, d.EndSession+"?"+q.Encode(), http.StatusFound)
}

func deniedPage(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>Clipper — sign in</title>
<style>body{margin:0;min-height:100vh;display:grid;place-items:center;font:15px system-ui,sans-serif;background:#111;color:#eee}
div{display:grid;gap:12px;max-width:320px;text-align:center}a{color:#ff8a8a}</style></head>
<body><div><strong>Clipper</strong><p>%s</p><a href="/auth/login">Try again</a><a href="/auth/signout">Use another account</a></div></body></html>`,
		html.EscapeString(msg))
}
