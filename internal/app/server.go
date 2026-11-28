// Package app is the Fond HTTP server: authentication, the JSON API and the
// embedded web client.
package app

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/au12/fond/internal/images"
	"github.com/au12/fond/internal/scrape"
	"golang.org/x/crypto/argon2"
)

// Config is read from the environment in cmd/fond.
type Config struct {
	DataDir            string
	Addr               string
	BaseURL            string
	TrustedProxies     []*net.IPNet
	AllowPrivateImport bool
	SecureCookies      string // auto | always | never
	SessionTTL         time.Duration
	HSTS               bool
	AIBase, AIModel    string
	AIKey              string
	Version            string
}

type asset struct {
	data, gz []byte
	ctype    string
	etag     string
}

// Server holds everything a request needs.
type Server struct {
	cfg     Config
	store   *Store
	images  *images.Store
	public  *scrape.Fetcher // SSRF-guarded, for user-supplied links
	private *scrape.Fetcher // unguarded, for admin-configured endpoints
	assets  map[string]*asset
	index   []byte
	limit   *limiter
	cop     *http.CrossOriginProtection
	jobs    sync.Map
	tickets tickets
	clips   sync.Map
	hides   sync.Map      // one lock per cover being drawn
	easel   chan struct{} // room to draw two covers at a time
	mux     *http.ServeMux
}

// New builds the server. web is the built client (web/dist).
func New(cfg Config, store *Store, web fs.FS) (*Server, error) {
	s := &Server{
		cfg: cfg, store: store,
		images:  &images.Store{Dir: filepath.Join(cfg.DataDir, "images")},
		public:  scrape.NewFetcher(cfg.AllowPrivateImport),
		private: scrape.NewFetcher(true),
		assets:  map[string]*asset{},
		limit:   newLimiter(),
		cop:     http.NewCrossOriginProtection(),
		easel:   make(chan struct{}, 2),
		mux:     http.NewServeMux(),
	}
	for _, d := range []string{"images", "import", "tmp", "leather"} {
		if err := os.MkdirAll(filepath.Join(cfg.DataDir, d), 0o755); err != nil {
			return nil, fmt.Errorf("data directory is not writable: %w", err)
		}
	}
	err := fs.WalkDir(web, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(web, p)
		if err != nil {
			return err
		}
		a := &asset{data: b, ctype: mime.TypeByExtension(path.Ext(p))}
		switch path.Ext(p) {
		case ".webmanifest":
			a.ctype = "application/manifest+json"
		case ".woff2":
			a.ctype = "font/woff2"
		case "":
			a.ctype = "application/octet-stream"
		}
		sum := sha256.Sum256(b)
		a.etag = `"` + hex.EncodeToString(sum[:8]) + `"`
		if len(b) > 512 && (strings.HasPrefix(a.ctype, "text/") || strings.Contains(a.ctype, "javascript") || strings.Contains(a.ctype, "json") || strings.Contains(a.ctype, "svg")) {
			var buf bytes.Buffer
			zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
			zw.Write(b)
			zw.Close()
			a.gz = buf.Bytes()
		}
		s.assets["/"+p] = a
		return nil
	})
	if err != nil {
		return nil, err
	}
	if a, ok := s.assets["/index.html"]; ok {
		s.index = a.data
	} else {
		s.index = []byte("<!doctype html><title>Fond</title><p>The web client was not built. Run <code>npm run build</code> in <code>web/</code>.")
	}
	s.routes()
	go s.janitor()
	go s.drawMissingLeather()
	return s, nil
}

func (s *Server) janitor() {
	for range time.Tick(time.Hour) {
		s.store.Exec(`DELETE FROM sessions WHERE expires_at<?`, now())
		s.limit.prune()
		s.pruneLeather()
		entries, _ := os.ReadDir(filepath.Join(s.cfg.DataDir, "tmp"))
		for _, e := range entries {
			if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > 6*time.Hour {
				os.Remove(filepath.Join(s.cfg.DataDir, "tmp", e.Name()))
			}
		}
	}
}

// ------------------------------------------------------------------ plumbing

type apiError struct {
	Status int    `json:"-"`
	Msg    string `json:"error"`
}

func (e *apiError) Error() string { return e.Msg }

func fail(status int, msg string) error { return &apiError{status, msg} }

func (s *Server) writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"encoding failed"}`, 500)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	if len(b) > 1400 && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		var buf bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
		zw.Write(b)
		zw.Close()
		h.Set("Content-Encoding", "gzip")
		h.Add("Vary", "Accept-Encoding")
		b = buf.Bytes()
	}
	w.WriteHeader(status)
	w.Write(b)
}

func (s *Server) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var ae *apiError
	if errors.As(err, &ae) {
		s.writeJSON(w, r, ae.Status, ae)
		return
	}
	if errors.Is(err, errNotFound) || strings.Contains(err.Error(), "no rows") {
		s.writeJSON(w, r, 404, &apiError{Msg: "Not found"})
		return
	}
	log.Printf("error: %s %s: %v", r.Method, r.URL.Path, err)
	s.writeJSON(w, r, 500, &apiError{Msg: "Something went wrong on the server"})
}

var errNotFound = errors.New("not found")

func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fail(400, "That request couldn't be read")
	}
	return nil
}

func pathID(r *http.Request, name string) int64 {
	v := r.PathValue(name)
	if i := strings.IndexByte(v, '-'); i > 0 {
		v = v[:i]
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func randToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

// ------------------------------------------------------------------ client address

func (s *Server) trusted(ip net.IP) bool {
	for _, n := range s.cfg.TrustedProxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIP returns the real client address, honouring X-Forwarded-For only
// when the request came through a trusted proxy.
func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	remote := net.ParseIP(host)
	if remote == nil || !s.trusted(remote) {
		return host
	}
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(parts[i]))
		if ip == nil {
			continue
		}
		if !s.trusted(ip) || i == 0 {
			return ip.String()
		}
	}
	if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
		return ip.String()
	}
	return host
}

func (s *Server) isHTTPS(r *http.Request) bool {
	if r.TLS != nil || s.cfg.SecureCookies == "always" {
		return true
	}
	if s.cfg.SecureCookies == "never" {
		return false
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip := net.ParseIP(host); ip != nil && s.trusted(ip) {
		return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	}
	return false
}

func (s *Server) origin(r *http.Request) string {
	if s.cfg.BaseURL != "" {
		return strings.TrimRight(s.cfg.BaseURL, "/")
	}
	scheme := "http"
	if s.isHTTPS(r) {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// ------------------------------------------------------------------ rate limiting

type bucket struct {
	n     int
	reset time.Time
}

type limiter struct {
	mu sync.Mutex
	m  map[string]*bucket
}

func newLimiter() *limiter { return &limiter{m: map[string]*bucket{}} }

// hit records one event and reports whether the caller is still within max
// events per window.
func (l *limiter) hit(key string, max int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.m[key]
	if b == nil || time.Now().After(b.reset) {
		b = &bucket{reset: time.Now().Add(window)}
		l.m[key] = b
	}
	b.n++
	return b.n <= max
}

func (l *limiter) over(key string, max int) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.m[key]
	if b == nil || time.Now().After(b.reset) {
		return false, 0
	}
	return b.n >= max, time.Until(b.reset)
}

func (l *limiter) clear(key string) {
	l.mu.Lock()
	delete(l.m, key)
	l.mu.Unlock()
}

func (l *limiter) prune() {
	l.mu.Lock()
	for k, b := range l.m {
		if time.Now().After(b.reset) {
			delete(l.m, k)
		}
	}
	l.mu.Unlock()
}

// ------------------------------------------------------------------ passwords

const (
	argonTime    = 2
	argonMemory  = 19 * 1024 // KiB — OWASP baseline for argon2id
	argonThreads = 1
)

func hashPassword(pw string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

func checkPassword(encoded, pw string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[4])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash keeps login timing the same for unknown usernames.
var dummyHash = hashPassword("fond-dummy-password")

func validPassword(pw string) error {
	if len(pw) < 8 {
		return fail(400, "Use a password with at least 8 characters")
	}
	if len(pw) > 256 {
		return fail(400, "That password is too long")
	}
	return nil
}

// ------------------------------------------------------------------ sessions

const cookieName = "fond_session"

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, u *User) error {
	token := randToken(32)
	if err := s.store.CreateSession(hashToken(token), u.ID, s.cfg.SessionTTL, r.UserAgent(), s.clientIP(r)); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/", HttpOnly: true, Secure: s.isHTTPS(r),
		SameSite: http.SameSiteLaxMode, MaxAge: int(s.cfg.SessionTTL.Seconds()),
	})
	return nil
}

func (s *Server) clearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, Secure: s.isHTTPS(r), SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

// identify resolves the caller from a bearer token or the session cookie.
// Cookie-authenticated writes must also pass the cross-origin (CSRF) check.
func (s *Server) identify(r *http.Request) (*User, error) {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		u, err := s.store.TokenUser(hashToken(strings.TrimSpace(h[7:])))
		if err != nil {
			return nil, fail(401, "That API token isn't valid")
		}
		return u, nil
	}
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return nil, fail(401, "Sign in to continue")
	}
	if err := s.cop.Check(r); err != nil {
		return nil, fail(403, "Cross-site request blocked")
	}
	u, err := s.store.SessionUser(hashToken(c.Value), s.cfg.SessionTTL, s.clientIP(r))
	if err != nil {
		return nil, fail(401, "Sign in to continue")
	}
	return u, nil
}

type userHandler func(w http.ResponseWriter, r *http.Request, u *User) error

func (s *Server) user(h userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := s.identify(r)
		if err == nil {
			err = h(w, r, u)
		}
		if err != nil {
			s.writeErr(w, r, err)
		}
	}
}

func (s *Server) admin(h userHandler) http.HandlerFunc {
	return s.user(func(w http.ResponseWriter, r *http.Request, u *User) error {
		if u.Role != "admin" {
			return fail(403, "Only an admin can do that")
		}
		return h(w, r, u)
	})
}

func (s *Server) open(h func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			s.writeErr(w, r, err)
		}
	}
}

// ------------------------------------------------------------------ http

const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; " +
	"connect-src 'self'; manifest-src 'self'; worker-src 'self'; media-src 'self' data:; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

// ServeHTTP applies the security headers and panic recovery to every request.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil && rec != http.ErrAbortHandler {
			log.Printf("panic: %s %s: %v", r.Method, r.URL.Path, rec)
			http.Error(w, "internal error", 500)
		}
	}()
	h := w.Header()
	h.Set("Content-Security-Policy", csp)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=(), interest-cohort=()")
	h.Set("X-Robots-Tag", "noindex, nofollow")
	if s.cfg.HSTS && s.isHTTPS(r) {
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
	s.mux.ServeHTTP(w, r)
}

// HTTPServer returns a net/http server with conservative timeouts.
func (s *Server) HTTPServer() *http.Server {
	return &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
}

func (s *Server) serveAsset(w http.ResponseWriter, r *http.Request, a *asset, cache string) {
	h := w.Header()
	h.Set("Content-Type", a.ctype)
	h.Set("Cache-Control", cache)
	h.Set("ETag", a.etag)
	if r.Header.Get("If-None-Match") == a.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	body := a.data
	if a.gz != nil && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		h.Set("Content-Encoding", "gzip")
		h.Add("Vary", "Accept-Encoding")
		body = a.gz
	}
	h.Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method != http.MethodHead {
		w.Write(body)
	}
}

// serveIndex sends the app shell, optionally with extra <head> tags (used to
// give shared recipes a proper link preview).
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request, head string) {
	body := s.index
	if head != "" {
		body = bytes.Replace(body, []byte("<!--head-->"), []byte(head), 1)
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		h.Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		zw.Write(body)
		zw.Close()
		return
	}
	w.Write(body)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p := path.Clean(r.URL.Path)
	if strings.HasPrefix(p, "/api/") {
		s.writeJSON(w, r, 404, &apiError{Msg: "Not found"})
		return
	}
	if a, ok := s.assets[p]; ok && p != "/index.html" {
		cache := "no-cache"
		if strings.HasPrefix(p, "/assets/") {
			cache = "public, max-age=31536000, immutable"
		} else if strings.HasPrefix(p, "/icons/") || strings.HasPrefix(p, "/fonts/") {
			cache = "public, max-age=604800"
		}
		s.serveAsset(w, r, a, cache)
		return
	}
	if ext := path.Ext(p); ext != "" && len(ext) <= 6 && !strings.HasPrefix(p, "/r/") {
		http.NotFound(w, r)
		return
	}
	s.serveIndex(w, r, "")
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !images.ValidName(name) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(s.images.Dir, name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	http.ServeContent(w, r, "", st.ModTime(), f)
}

// Healthcheck is used by `fond healthcheck` (the container HEALTHCHECK).
func Healthcheck(addr string) error {
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	c := &http.Client{Timeout: 4 * time.Second}
	res, err := c.Get("http://" + addr + "/healthz")
	if err != nil {
		return err
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)
	if res.StatusCode != 200 {
		return fmt.Errorf("status %d", res.StatusCode)
	}
	return nil
}
