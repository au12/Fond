package app

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

type client struct {
	t    *testing.T
	c    *http.Client
	base string
}

func (c *client) do(method, path string, body any, headers ...string) (int, map[string]any) {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, c.base+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", c.base)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func newTestServer(t *testing.T) (*client, *Server) {
	t.Helper()
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	_, lo, _ := net.ParseCIDR("127.0.0.0/8")
	srv, err := New(Config{DataDir: t.TempDir(), SessionTTL: time.Hour, SecureCookies: "auto", TrustedProxies: []*net.IPNet{lo}, Version: "test"},
		store, fstest.MapFS{"index.html": {Data: []byte("<html><!--head--></html>")}})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &client{t: t, c: &http.Client{Jar: jar}, base: ts.URL}, srv
}

func TestAuthAndRecipes(t *testing.T) {
	c, srv := newTestServer(t)
	if !srv.store.fts {
		t.Error("full-text search (FTS5) is not available")
	}
	if code, _ := c.do("GET", "/api/recipes", nil); code != 401 {
		t.Fatalf("anonymous list = %d, want 401", code)
	}
	if code, _ := c.do("POST", "/api/auth/setup", map[string]string{"username": "cook", "password": "short"}); code != 400 {
		t.Fatalf("weak password accepted: %d", code)
	}
	if code, _ := c.do("POST", "/api/auth/setup", map[string]string{"username": "cook", "password": "correct horse"}); code != 200 {
		t.Fatalf("setup = %d", code)
	}
	if code, _ := c.do("POST", "/api/auth/setup", map[string]string{"username": "mallory", "password": "correct horse"}); code != 409 {
		t.Fatalf("second setup = %d, want 409", code)
	}
	body := map[string]any{"title": "Toast", "ingredients": []map[string]any{{"text": "2 slices bread"}}, "steps": []map[string]any{{"text": "Toast for 3 minutes."}}, "tags": []string{"Breakfast", "breakfast"}}
	code, rec := c.do("POST", "/api/recipes", body)
	if code != 201 || rec["slug"] != "toast" {
		t.Fatalf("create = %d %v", code, rec)
	}
	if tags := rec["tags"].([]any); len(tags) != 1 {
		t.Errorf("tags not de-duplicated: %v", tags)
	}
	// A cross-site request carrying the session cookie must be refused.
	if code, _ := c.do("POST", "/api/recipes", body, "Origin", "https://evil.example", "Sec-Fetch-Site", "cross-site"); code != 403 {
		t.Errorf("cross-site write = %d, want 403", code)
	}
	// Sharing exposes exactly one recipe, and only while switched on.
	_, sh := c.do("PUT", "/api/recipes/1/share", map[string]bool{"on": true})
	token := sh["shareToken"].(string)
	anon := &client{t: t, c: &http.Client{}, base: c.base}
	if code, pub := anon.do("GET", "/api/public/recipe/"+token, nil); code != 200 || pub["title"] != "Toast" {
		t.Errorf("public recipe = %d %v", code, pub)
	}
	if code, _ := anon.do("GET", "/api/public/recipe/not-a-token", nil); code != 404 {
		t.Errorf("bad token = %d", code)
	}
	res, _ := http.Get(c.base + "/s/" + token)
	page := new(bytes.Buffer)
	page.ReadFrom(res.Body)
	if !strings.Contains(page.String(), `og:title" content="Toast"`) || !strings.Contains(page.String(), "application/ld+json") {
		t.Errorf("share page lacks preview tags: %s", page)
	}
	if res.Header.Get("Content-Security-Policy") == "" || res.Header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("security headers missing")
	}
	c.do("PUT", "/api/recipes/1/share", map[string]bool{"on": false})
	if code, _ := anon.do("GET", "/api/public/recipe/"+token, nil); code != 404 {
		t.Errorf("unshared recipe still public: %d", code)
	}
	// API tokens work without cookies and are immune to the CSRF check.
	_, tk := c.do("POST", "/api/me/tokens", map[string]string{"name": "shortcut"})
	if code, _ := anon.do("GET", "/api/recipes/1", nil, "Authorization", "Bearer "+tk["token"].(string), "Origin", "https://anywhere.example"); code != 200 {
		t.Errorf("token auth = %d", code)
	}
	if code, _ := anon.do("GET", "/api/recipes/1", nil, "Authorization", "Bearer fond_wrong"); code != 401 {
		t.Errorf("bad token = %d", code)
	}
	// Search reaches into ingredients.
	res2, _ := c.c.Get(c.base + "/api/search?q=bread")
	var ids []int
	json.NewDecoder(res2.Body).Decode(&ids)
	if len(ids) != 1 {
		t.Errorf("search = %v", ids)
	}
}

func TestLoginRateLimit(t *testing.T) {
	c, _ := newTestServer(t)
	c.do("POST", "/api/auth/setup", map[string]string{"username": "cook", "password": "correct horse"})
	anon := &client{t: t, c: &http.Client{}, base: c.base}
	last := 0
	for i := 0; i < 10; i++ {
		last, _ = anon.do("POST", "/api/auth/login", map[string]string{"username": "cook", "password": "nope"})
	}
	if last != 429 {
		t.Fatalf("after 10 bad logins got %d, want 429", last)
	}
	if code, _ := anon.do("POST", "/api/auth/login", map[string]string{"username": "cook", "password": "correct horse"}); code != 429 {
		t.Errorf("locked account accepted a login: %d", code)
	}
}

func TestClientIP(t *testing.T) {
	_, srv := newTestServer(t)
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 127.0.0.1")
	if ip := srv.clientIP(r); ip != "203.0.113.9" {
		t.Errorf("trusted proxy: got %s", ip)
	}
	r.RemoteAddr = "198.51.100.7:1234" // not a trusted proxy: header must be ignored
	if ip := srv.clientIP(r); ip != "198.51.100.7" {
		t.Errorf("spoofed header honoured: got %s", ip)
	}
}

func TestPasswordHash(t *testing.T) {
	h := hashPassword("s3cret-pass")
	if !checkPassword(h, "s3cret-pass") || checkPassword(h, "s3cret-pasS") || checkPassword("garbage", "x") {
		t.Fatal("password verification is wrong")
	}
}
