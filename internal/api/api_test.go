package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"tonton/internal/auth"
	"tonton/internal/catalog"
	"tonton/internal/store"
)

type client struct {
	t   *testing.T
	srv *httptest.Server
	c   *http.Client
}

func newServer(t *testing.T) (*client, *store.Store) {
	t.Helper()
	cat, err := catalog.Open("../catalog/testdata", "")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := func() time.Time { return time.Unix(1795000000, 0) } // 2026-11-18
	s := &Server{
		Cat: cat, Store: st, Loc: loc, Now: now,
		Sessions: auth.Sessions{Key: []byte("0123456789abcdef0123456789abcdef"), TTL: time.Hour, Now: time.Now},
		Limiter:  &auth.Limiter{Free: 3, Max: time.Minute, Now: time.Now},
		Static:   fstest.MapFS{"index.html": {Data: []byte("<!doctype html>app")}, "assets/a.js": {Data: []byte("x")}},
	}
	srv := httptest.NewServer(s.Handler()) // httptest listens on 127.0.0.1, which the guard allows
	t.Cleanup(srv.Close)
	jar, _ := cookiejar.New(nil)
	return &client{t: t, srv: srv, c: &http.Client{Jar: jar}}, st
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.srv.URL+path, r)
	if body != nil || method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func (c *client) fresh() *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: c.t, srv: c.srv, c: &http.Client{Jar: jar}}
}

func TestFlow(t *testing.T) {
	c, _ := newServer(t)

	if code, _ := c.do("GET", "/api/home", nil); code != 401 {
		t.Fatalf("home without login = %d", code)
	}
	_, p := c.do("GET", "/api/profiles", nil)
	if p["setup"] != true {
		t.Fatalf("fresh install should ask for setup: %v", p)
	}
	if code, _ := c.do("POST", "/api/setup", map[string]string{"name": "Rina", "pin": "4821"}); code != 201 {
		t.Fatalf("setup = %d", code)
	}
	if code, _ := c.fresh().do("POST", "/api/setup", map[string]string{"name": "Eve", "pin": "0000"}); code != 409 {
		t.Fatalf("second setup should be refused, got %d", code)
	}

	// Rina adds a family member, who logs in separately.
	if code, _ := c.do("POST", "/api/users", map[string]any{"name": "Bima", "pin": "1111"}); code != 201 {
		t.Fatalf("add user = %d", code)
	}
	b := c.fresh()
	if code, _ := b.do("POST", "/api/login", map[string]string{"name": "bima", "pin": "1111"}); code != 200 {
		t.Fatalf("login = %d", code)
	}
	if code, _ := b.do("GET", "/api/users", nil); code != 403 {
		t.Fatalf("non-admin listing users = %d", code)
	}

	// Progress: +1 on a finished 28-episode show.
	_, it := c.do("PATCH", "/api/list/2", map[string]any{"episode": 1})
	if e := it["entry"].(map[string]any); e["status"] != "watching" || e["episode"] != 1.0 {
		t.Fatalf("entry after +1 = %v", e)
	}
	if code, _ := c.do("PATCH", "/api/list/999", map[string]any{"episode": 1}); code != 404 {
		t.Fatalf("unknown title = %d", code)
	}
	if code, _ := c.do("PATCH", "/api/list/2", map[string]any{"status": "later"}); code != 400 {
		t.Fatalf("bad status = %d", code)
	}
	if code, _ := c.do("PATCH", "/api/list/2", map[string]any{"bogus": 1}); code != 400 {
		t.Fatalf("unknown field = %d", code)
	}
	// Airing show with next episode within a week shows up in upcoming.
	c.do("PATCH", "/api/list/3", map[string]any{"status": "watching"})

	_, home := c.do("GET", "/api/home", nil)
	if n := len(home["watching"].([]any)); n != 2 {
		t.Fatalf("watching = %d, want 2", n)
	}
	up := home["upcoming"].([]any)
	if len(up) != 1 || up[0].(map[string]any)["next"].(map[string]any)["episode"] != 5.0 {
		t.Fatalf("upcoming = %v", up)
	}
	if home["today"] != 1.0 {
		t.Fatalf("today = %v", home["today"])
	}

	// Lists are per member.
	_, bl := b.do("GET", "/api/list", nil)
	if n := len(bl["items"].([]any)); n != 0 {
		t.Fatalf("Bima sees %d items of Rina's list", n)
	}
	_, sr := c.do("GET", "/api/search?q=frieren", nil)
	if items := sr["items"].([]any); len(items) != 1 || items[0].(map[string]any)["entry"] == nil {
		t.Fatalf("search should include Rina's entry: %v", sr)
	}
	_, se := c.do("GET", "/api/season?year=2026&season=3", nil)
	if n := len(se["items"].([]any)); n != 2 {
		t.Fatalf("season items = %d", n)
	}
	_, d := c.do("GET", "/api/anime/2", nil)
	if d["anime"].(map[string]any)["synopsis"] == "" || d["entry"] == nil {
		t.Fatalf("detail = %v", d)
	}
	if code, _ := c.do("DELETE", "/api/list/2", nil); code != 204 {
		t.Fatalf("delete = %d", code)
	}

	// PIN change logs out other sessions.
	if code, _ := b.do("POST", "/api/me/pin", map[string]string{"old": "1111", "new": "2222"}); code != 204 {
		t.Fatalf("change pin = %d", code)
	}
	if code, _ := b.do("GET", "/api/me", nil); code != 200 {
		t.Fatalf("this device should stay signed in, got %d", code)
	}
}

func TestLoginLimiter(t *testing.T) {
	c, st := newServer(t)
	h, _ := auth.HashPIN("4821")
	st.AddUser("Rina", h, true)
	for i := 0; i < 3; i++ {
		if code, _ := c.do("POST", "/api/login", map[string]string{"name": "Rina", "pin": "0000"}); code != 401 {
			t.Fatalf("wrong PIN = %d", code)
		}
	}
	c.do("POST", "/api/login", map[string]string{"name": "Rina", "pin": "0000"})
	if code, body := c.do("POST", "/api/login", map[string]string{"name": "Rina", "pin": "4821"}); code != 429 {
		t.Fatalf("after too many failures even the right PIN must wait, got %d %v", code, body)
	}
}

func TestCSRFAndStatic(t *testing.T) {
	c, _ := newServer(t)
	req, _ := http.NewRequest("POST", c.srv.URL+"/api/login", strings.NewReader("name=a&pin=b"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if res, _ := c.c.Do(req); res.StatusCode != 415 {
		t.Fatalf("form post = %d, want 415", res.StatusCode)
	}
	req, _ = http.NewRequest("POST", c.srv.URL+"/api/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example")
	if res, _ := c.c.Do(req); res.StatusCode != 403 {
		t.Fatalf("cross-origin post = %d, want 403", res.StatusCode)
	}
	res, _ := c.c.Get(c.srv.URL + "/daftar")
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "app") {
		t.Fatalf("client routes should get index.html, got %q", body)
	}
	res, _ = c.c.Get(c.srv.URL + "/assets/a.js")
	if !strings.Contains(res.Header.Get("Cache-Control"), "immutable") {
		t.Fatal("hashed assets should be cached forever")
	}
	if res.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("missing CSP")
	}
}

func TestPrivateAddr(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:1": true, "192.168.100.20:5": true, "10.0.0.2:1": true, "100.100.20.30:1": true,
		"[fd7a:115c:a1e0::1]:1": true, "8.8.8.8:1": false, "[2001:db8::1]:1": false, "[::ffff:192.168.1.2]:1": true,
	} {
		if PrivateAddr(addr) != want {
			t.Errorf("PrivateAddr(%s) = %v", addr, !want)
		}
	}
}
