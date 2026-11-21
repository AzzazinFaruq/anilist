// Package api is the HTTP layer: JSON endpoints under /api and the embedded
// React app for every other path.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"tonton/internal/auth"
	"tonton/internal/catalog"
	"tonton/internal/store"
)

const cookieName = "tonton"

// Server wires the catalogue, the store and sessions to HTTP.
type Server struct {
	Cat      *catalog.Catalog
	Store    *store.Store
	Sessions auth.Sessions
	Limiter  *auth.Limiter
	Loc      *time.Location
	Static   fs.FS  // built web app; nil serves the API only
	DataDir  string // for per-member recommendation files (temuan/<id>.json)
	Token    string // bearer token for /internal/summary (dashboard); empty disables it
	AnyIP    bool   // accept clients outside LAN/Tailscale (tests, reverse proxies)
	Now      func() time.Time
}

type ctxKey struct{}

// Handler returns the complete HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "titles": s.Cat.Len()})
	})

	// Public: who can log in, first-run setup, login.
	mux.HandleFunc("GET /api/profiles", s.profiles)
	mux.HandleFunc("POST /api/setup", s.setup)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.logout)

	// Signed in.
	mux.Handle("GET /api/me", s.user(s.me))
	mux.Handle("POST /api/me/pin", s.user(s.changePIN))
	mux.Handle("GET /api/home", s.user(s.home))
	mux.Handle("GET /api/list", s.user(s.list))
	mux.Handle("PATCH /api/list/{id}", s.user(s.updateEntry))
	mux.Handle("DELETE /api/list/{id}", s.user(s.removeEntry))
	mux.Handle("GET /api/search", s.user(s.search))
	mux.Handle("GET /api/season", s.user(s.season))
	mux.Handle("GET /api/anime/{id}", s.user(s.detail))
	mux.Handle("GET /api/anime/{id}/similar", s.user(s.similar))
	mux.Handle("GET /api/meta", s.user(s.meta))
	mux.Handle("GET /api/browse", s.user(s.browse))
	mux.Handle("GET /api/schedule", s.user(s.schedule))
	mux.Handle("GET /api/pick", s.user(s.pick))
	mux.Handle("GET /api/temuan", s.user(s.temuan))
	mux.Handle("PUT /api/hidden/{id}", s.user(s.setHidden))
	mux.Handle("PUT /api/taste", s.user(s.setTaste))
	mux.Handle("GET /api/presets", s.user(s.presets))
	mux.Handle("PUT /api/presets", s.user(s.setPresets))
	mux.Handle("GET /api/stats", s.user(s.stats))
	mux.Handle("GET /api/export", s.user(s.export))
	mux.HandleFunc("GET /internal/summary", s.summary)

	// Admin: family members.
	mux.Handle("GET /api/users", s.admin(s.users))
	mux.Handle("POST /api/users", s.admin(s.addUser))
	mux.Handle("PATCH /api/users/{id}", s.admin(s.editUser))
	mux.Handle("DELETE /api/users/{id}", s.admin(s.deleteUser))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotFound, "Alamat API tidak dikenal.")
	})
	if s.Static != nil {
		mux.Handle("/", spa(s.Static))
	}
	return s.guard(mux)
}

// ---------- middleware ----------

var tailnet6 = netip.MustParsePrefix("fd7a:115c:a1e0::/48")
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// PrivateAddr reports whether a client address is loopback, LAN or Tailscale.
func PrivateAddr(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	return ip.IsLoopback() || ip.IsPrivate() || cgnat.Contains(ip) || tailnet6.Contains(ip)
}

func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.AnyIP && !PrivateAddr(r.RemoteAddr) {
			http.Error(w, "Hanya untuk jaringan rumah dan Tailscale.", http.StatusForbidden)
			return
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' https://s4.anilist.co data:; "+
			"style-src 'self'; style-src-attr 'unsafe-inline'; font-src 'self'; connect-src 'self'; "+
			"frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			// Cross-site forms cannot send JSON without a CORS preflight, and we
			// never answer preflights, so this plus the Origin check stops CSRF.
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				writeErr(w, http.StatusUnsupportedMediaType, "Kirim data sebagai JSON.")
				return
			}
			if o := r.Header.Get("Origin"); o != "" && !sameOrigin(o, r.Host) {
				writeErr(w, http.StatusForbidden, "Permintaan dari situs lain ditolak.")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		}
		next.ServeHTTP(w, r)
	})
}

func sameOrigin(origin, host string) bool {
	i := strings.Index(origin, "://")
	return i >= 0 && strings.EqualFold(origin[i+3:], host)
}

func (s *Server) current(r *http.Request) (store.User, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return store.User{}, false
	}
	id, ver, ok := s.Sessions.Parse(c.Value)
	if !ok {
		return store.User{}, false
	}
	u, ok := s.Store.UserByID(id)
	if !ok || u.Version != ver {
		return store.User{}, false
	}
	return u, true
}

func (s *Server) user(fn func(http.ResponseWriter, *http.Request, store.User)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.current(r)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Masuk dulu.")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		fn(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)), u)
	})
}

func (s *Server) admin(fn func(http.ResponseWriter, *http.Request, store.User)) http.Handler {
	return s.user(func(w http.ResponseWriter, r *http.Request, u store.User) {
		if !u.Admin {
			writeErr(w, http.StatusForbidden, "Hanya admin yang bisa mengatur anggota.")
			return
		}
		fn(w, r, u)
	})
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "Data yang dikirim tidak terbaca.")
		return false
	}
	return true
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now().In(s.Loc)
	}
	return time.Now().In(s.Loc)
}

func (s *Server) today() string { return s.now().Format("2006-01-02") }

func pathID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	return id, err == nil && id > 0
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---------- session endpoints ----------

type profile struct {
	Name string `json:"name"`
}

func (s *Server) profiles(w http.ResponseWriter, r *http.Request) {
	us := s.Store.Users()
	out := make([]profile, len(us))
	for i, u := range us {
		out[i] = profile{u.Name}
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": out, "setup": len(us) == 0})
}

type credentials struct {
	Name string `json:"name"`
	PIN  string `json:"pin"`
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !readJSON(w, r, &c) {
		return
	}
	if len(s.Store.Users()) > 0 {
		writeErr(w, http.StatusConflict, "Akun pertama sudah dibuat. Masuk dengan akun itu.")
		return
	}
	hash, err := auth.HashPIN(c.PIN)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "PIN harus 4–32 karakter.")
		return
	}
	u, err := s.Store.AddUser(c.Name, hash, true)
	if err != nil {
		writeErr(w, http.StatusBadRequest, userErr(err))
		return
	}
	s.startSession(w, r, u)
	writeJSON(w, http.StatusCreated, meJSON(u))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !readJSON(w, r, &c) {
		return
	}
	keys := []string{"name:" + strings.ToLower(strings.TrimSpace(c.Name)), "ip:" + clientIP(r)}
	for _, k := range keys {
		if d := s.Limiter.Wait(k); d > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(d.Seconds())+1))
			writeErr(w, http.StatusTooManyRequests, "Terlalu banyak PIN salah. Coba lagi dalam "+humanWait(d)+".")
			return
		}
	}
	u, ok := s.Store.UserByName(c.Name)
	// Hash even for unknown names so timing does not reveal which names exist.
	hash := u.Hash
	if !ok {
		hash = dummyHash
	}
	if !auth.VerifyPIN(hash, c.PIN) || !ok {
		for _, k := range keys {
			s.Limiter.Fail(k)
		}
		writeErr(w, http.StatusUnauthorized, "Nama atau PIN salah.")
		return
	}
	for _, k := range keys {
		s.Limiter.Reset(k)
	}
	s.startSession(w, r, u)
	writeJSON(w, http.StatusOK, meJSON(u))
}

var dummyHash, _ = auth.HashPIN("not-a-real-pin")

func humanWait(d time.Duration) string {
	if d < time.Minute {
		return strconv.Itoa(int(d.Seconds())+1) + " detik"
	}
	return strconv.Itoa(int(d.Minutes())+1) + " menit"
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, u store.User) {
	token, exp := s.Sessions.Issue(u.ID, u.Version)
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/", Expires: exp, MaxAge: int(time.Until(exp).Seconds()),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func meJSON(u store.User) map[string]any {
	return map[string]any{"id": u.ID, "name": u.Name, "admin": u.Admin}
}

func (s *Server) me(w http.ResponseWriter, r *http.Request, u store.User) {
	writeJSON(w, http.StatusOK, meJSON(u))
}

func (s *Server) changePIN(w http.ResponseWriter, r *http.Request, u store.User) {
	var body struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	key := "pin:" + u.ID
	if d := s.Limiter.Wait(key); d > 0 {
		writeErr(w, http.StatusTooManyRequests, "Terlalu banyak PIN salah. Coba lagi dalam "+humanWait(d)+".")
		return
	}
	if !auth.VerifyPIN(u.Hash, body.Old) {
		s.Limiter.Fail(key)
		writeErr(w, http.StatusBadRequest, "PIN lama salah.")
		return
	}
	s.Limiter.Reset(key)
	hash, err := auth.HashPIN(body.New)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "PIN baru harus 4–32 karakter.")
		return
	}
	u, err = s.Store.SetHash(u.ID, hash)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "PIN gagal disimpan.")
		return
	}
	s.startSession(w, r, u) // this device stays signed in, others are signed out
	w.WriteHeader(http.StatusNoContent)
}

// ---------- list endpoints ----------

// Item pairs a title with the member's entry and next airing time.
type Item struct {
	Anime    *catalog.Anime  `json:"anime"`
	Entry    *store.Entry    `json:"entry,omitempty"`
	Next     *catalog.Airing `json:"next,omitempty"`
	Released *int            `json:"released,omitempty"` // episodes aired so far, when known
	Why      []string        `json:"why,omitempty"`      // recommendation reasons
	Match    float64         `json:"match,omitempty"`    // recommendation score 0–1
	Kind     string          `json:"kind,omitempty"`     // recommendation kind: baru, tayang, segera, lama
}

func (s *Server) item(a *catalog.Anime, e *store.Entry) Item {
	it := Item{Anime: a, Entry: e}
	if a.Status == "RELEASING" || a.Status == "NOT_YET_RELEASED" {
		if n, ok := s.Cat.NextAiring(a.ID, s.now()); ok {
			it.Next = &n
		}
	}
	return it
}

func (s *Server) entries(w http.ResponseWriter, uid string) ([]Item, map[string]int, store.List, bool) {
	l, err := s.Store.Snapshot(uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Daftar gagal dibaca.")
		return nil, nil, l, false
	}
	counts := map[string]int{}
	items := make([]Item, 0, len(l.Entries))
	for id, e := range l.Entries {
		a, ok := s.Cat.Get(id)
		if !ok {
			continue // title vanished from the catalogue; keep the entry, hide it
		}
		e := e
		counts[e.Status]++
		items = append(items, s.item(a, &e))
	}
	return items, counts, l, true
}

// byUpdated puts recently changed entries first; ties (e.g. a bulk import)
// fall back to popularity so the order is stable between requests.
func byUpdated(items []Item) {
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Entry.Updated != b.Entry.Updated {
			return a.Entry.Updated > b.Entry.Updated
		}
		if a.Anime.Popularity != b.Anime.Popularity {
			return a.Anime.Popularity > b.Anime.Popularity
		}
		return a.Anime.ID < b.Anime.ID
	})
}

func (s *Server) list(w http.ResponseWriter, r *http.Request, u store.User) {
	items, counts, _, ok := s.entries(w, u.ID)
	if !ok {
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" {
		kept := items[:0]
		for _, it := range items {
			if it.Entry.Status == status {
				kept = append(kept, it)
			}
		}
		items = kept
	}
	byUpdated(items)
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "counts": counts})
}

func (s *Server) updateEntry(w http.ResponseWriter, r *http.Request, u store.User) {
	id, ok := pathID(r)
	a, found := s.Cat.Get(id)
	if !ok || !found {
		writeErr(w, http.StatusNotFound, "Judul tidak ada di katalog.")
		return
	}
	var c store.Change
	if !readJSON(w, r, &c) {
		return
	}
	total := a.Episodes
	if a.Status == "RELEASING" || a.Status == "NOT_YET_RELEASED" {
		total = 0 // count may still grow; do not cap or auto-finish
	}
	e, err := s.Store.Update(u.ID, id, c, total, s.today())
	if err != nil {
		writeErr(w, http.StatusBadRequest, entryErr(err))
		return
	}
	writeJSON(w, http.StatusOK, s.item(a, &e))
}

func entryErr(err error) string {
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "unknown status"):
		return "Status tidak dikenal."
	case strings.HasPrefix(msg, "rating"):
		return "Nilai harus 0–10."
	case strings.HasPrefix(msg, "note"):
		return "Catatan paling panjang 500 karakter."
	}
	log.Printf("update entry: %v", err)
	return "Perubahan gagal disimpan."
}

func (s *Server) removeEntry(w http.ResponseWriter, r *http.Request, u store.User) {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "Judul tidak dikenal.")
		return
	}
	if err := s.Store.Remove(u.ID, id); err != nil && !errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusInternalServerError, "Judul gagal dihapus dari daftar.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// withEntries attaches the member's entries to catalogue results.
func (s *Server) withEntries(w http.ResponseWriter, uid string, as []*catalog.Anime) ([]Item, bool) {
	l, err := s.Store.Snapshot(uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Daftar gagal dibaca.")
		return nil, false
	}
	out := make([]Item, len(as))
	for i, a := range as {
		var ep *store.Entry
		if e, ok := l.Entries[a.ID]; ok {
			ep = &e
		}
		out[i] = Item{Anime: a, Entry: ep}
	}
	return out, true
}

func (s *Server) search(w http.ResponseWriter, r *http.Request, u store.User) {
	q := r.URL.Query().Get("q")
	if len(q) > 100 {
		q = q[:100]
	}
	items, ok := s.withEntries(w, u.ID, s.Cat.Search(q, 40))
	if ok {
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func (s *Server) season(w http.ResponseWriter, r *http.Request, u store.User) {
	year, season := catalog.CurrentSeason(s.now())
	if y, err := strconv.Atoi(r.URL.Query().Get("year")); err == nil && y >= 1900 && y <= 2100 {
		year = y
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("season")); err == nil && v >= 0 && v <= 3 {
		season = v
	}
	items, ok := s.withEntries(w, u.ID, s.Cat.Season(year, season))
	if ok {
		writeJSON(w, http.StatusOK, map[string]any{"year": year, "season": season, "items": items})
	}
}

func (s *Server) detail(w http.ResponseWriter, r *http.Request, u store.User) {
	id, ok := pathID(r)
	d, found := s.Cat.Detail(id)
	if !ok || !found {
		writeErr(w, http.StatusNotFound, "Judul tidak ada di katalog.")
		return
	}
	e, has, err := s.Store.Entry(u.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Daftar gagal dibaca.")
		return
	}
	it := s.item(&d.Anime, nil)
	if has {
		it.Entry = &e
	}
	writeJSON(w, http.StatusOK, map[string]any{"anime": d, "entry": it.Entry, "next": it.Next})
}

// ---------- admin endpoints ----------

func userJSON(u store.User) map[string]any {
	return map[string]any{"id": u.ID, "name": u.Name, "admin": u.Admin, "created": u.Created}
}

func userErr(err error) string {
	switch {
	case errors.Is(err, store.ErrNameTaken):
		return "Nama itu sudah dipakai."
	case errors.Is(err, store.ErrBadName):
		return "Nama harus 1–24 karakter."
	case errors.Is(err, store.ErrLastAdmin):
		return "Harus ada setidaknya satu admin."
	case errors.Is(err, store.ErrNotFound):
		return "Anggota tidak ditemukan."
	}
	log.Printf("users: %v", err)
	return "Perubahan gagal disimpan."
}

func (s *Server) users(w http.ResponseWriter, r *http.Request, _ store.User) {
	us := s.Store.Users()
	out := make([]map[string]any, len(us))
	for i, u := range us {
		out[i] = userJSON(u)
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (s *Server) addUser(w http.ResponseWriter, r *http.Request, _ store.User) {
	var body struct {
		Name  string `json:"name"`
		PIN   string `json:"pin"`
		Admin bool   `json:"admin"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	hash, err := auth.HashPIN(body.PIN)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "PIN harus 4–32 karakter.")
		return
	}
	u, err := s.Store.AddUser(body.Name, hash, body.Admin)
	if err != nil {
		writeErr(w, http.StatusBadRequest, userErr(err))
		return
	}
	writeJSON(w, http.StatusCreated, userJSON(u))
}

func (s *Server) editUser(w http.ResponseWriter, r *http.Request, me store.User) {
	id := r.PathValue("id")
	var body struct {
		PIN   *string `json:"pin"`
		Admin *bool   `json:"admin"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	var u store.User
	var err error
	if body.PIN != nil {
		hash, herr := auth.HashPIN(*body.PIN)
		if herr != nil {
			writeErr(w, http.StatusBadRequest, "PIN harus 4–32 karakter.")
			return
		}
		if u, err = s.Store.SetHash(id, hash); err != nil {
			writeErr(w, http.StatusBadRequest, userErr(err))
			return
		}
		if id == me.ID {
			s.startSession(w, r, u)
		}
	}
	if body.Admin != nil {
		if u, err = s.Store.SetAdmin(id, *body.Admin); err != nil {
			writeErr(w, http.StatusBadRequest, userErr(err))
			return
		}
	}
	if u.ID == "" {
		var ok bool
		if u, ok = s.Store.UserByID(id); !ok {
			writeErr(w, http.StatusNotFound, userErr(store.ErrNotFound))
			return
		}
	}
	writeJSON(w, http.StatusOK, userJSON(u))
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request, me store.User) {
	id := r.PathValue("id")
	if id == me.ID {
		writeErr(w, http.StatusBadRequest, "Akunmu sendiri tidak bisa dihapus dari sini.")
		return
	}
	if err := s.Store.DeleteUser(id); err != nil {
		writeErr(w, http.StatusBadRequest, userErr(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- static app ----------

// spa serves the built app; unknown paths get index.html so client routes work.
func spa(files fs.FS) http.Handler {
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" {
			if st, err := fs.Stat(files, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, files, "index.html")
	})
}
