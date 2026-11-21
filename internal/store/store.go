// Package store keeps family accounts and each member's watch list as small
// JSON files under one data directory:
//
//	users.json        accounts (PIN hashes, never the PIN itself)
//	session.key       HMAC key for login cookies
//	lists/<id>.json   one member's list and daily episode log
//	backup/           one copy of every file per day, kept for 14 days
//
// A family has a handful of users and a few hundred entries each, so files
// written atomically (temp file + rename) behind one mutex are simpler and
// easier to back up than a database, and fast enough by orders of magnitude.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Statuses an entry can have.
const (
	Watching = "watching"
	Plan     = "plan"
	Done     = "done"
	Hold     = "hold"
	Drop     = "drop"
)

// ValidStatus reports whether s is a known status.
func ValidStatus(s string) bool {
	switch s {
	case Watching, Plan, Done, Hold, Drop:
		return true
	}
	return false
}

var (
	ErrNotFound  = errors.New("not found")
	ErrNameTaken = errors.New("name already used")
	ErrBadName   = errors.New("name must be 1–24 characters")
	ErrLastAdmin = errors.New("the last admin cannot be removed")
)

// User is a family member.
type User struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Admin   bool      `json:"admin"`
	Hash    string    `json:"hash"`    // see auth.HashPIN
	Version int       `json:"version"` // bumped on PIN change to log out old sessions
	Created time.Time `json:"created"`
}

// Entry is one title on a member's list.
type Entry struct {
	Status   string `json:"status"`
	Episode  int    `json:"episode,omitempty"`
	Rating   int    `json:"rating,omitempty"` // 1–10, 0 = not rated
	Note     string `json:"note,omitempty"`
	Priority bool   `json:"priority,omitempty"` // watch this before the rest of the plan
	Updated  int64  `json:"updated"`            // unix ms
	Started  int64  `json:"started,omitempty"`
	Finished int64  `json:"finished,omitempty"`
}

// List is everything stored for one member.
type List struct {
	Entries map[int]Entry  `json:"entries"`
	Log     map[string]int `json:"log"`               // "2026-11-21" → episodes watched that day
	Hidden  map[int]int64  `json:"hidden,omitempty"`  // "not interested" (from recommendations) → unix ms
	Taste   map[string]int `json:"taste,omitempty"`   // "g:Action" / "t:Isekai" / "x:pure-romance" → 3 like, 2 okay, 1 avoid
	Presets []Preset       `json:"presets,omitempty"` // saved browse filters
}

// Preset is a saved Jelajah filter: a name and the page's query string.
type Preset struct {
	Name  string `json:"name"`
	Query string `json:"query"`
}

// Store is safe for concurrent use.
type Store struct {
	dir      string
	mu       sync.Mutex
	users    []User
	usersMod time.Time
	lists    map[string]*List
	listMod  map[string]time.Time
	now      func() time.Time
}

// Open reads (or initialises) the data directory.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dir, "lists"), 0o750); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, lists: map[string]*List{}, listMod: map[string]time.Time{}, now: time.Now}
	if err := s.refreshUsersLocked(); err != nil {
		return nil, err
	}
	return s, nil
}

func modTime(p string) time.Time {
	if fi, err := os.Stat(p); err == nil {
		return fi.ModTime()
	}
	return time.Time{}
}

// refreshUsersLocked re-reads users.json when it changed on disk, so the
// `tonton user …` commands take effect in a running server.
func (s *Store) refreshUsersLocked() error {
	p := filepath.Join(s.dir, "users.json")
	mod := modTime(p)
	if !mod.IsZero() && mod.Equal(s.usersMod) {
		return nil
	}
	b, err := os.ReadFile(p)
	switch {
	case errors.Is(err, os.ErrNotExist):
		s.users = nil
	case err != nil:
		return err
	default:
		var users []User
		if err := json.Unmarshal(b, &users); err != nil {
			return fmt.Errorf("users.json: %w", err)
		}
		s.users = users
	}
	s.usersMod = mod
	return nil
}

// lockUsers takes the lock with an up-to-date user list. A read error keeps
// the last good copy (and is logged by the caller's next write, if any).
func (s *Store) lockUsers() {
	s.mu.Lock()
	_ = s.refreshUsersLocked()
}

// Key returns the persistent cookie-signing key, creating it on first use.
func (s *Store) Key() ([]byte, error) {
	p := filepath.Join(s.dir, "session.key")
	if b, err := os.ReadFile(p); err == nil && len(b) >= 32 {
		return b, nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, writeFile(p, b, 0o600)
}

// ---------- users ----------

// Users lists members sorted by name.
func (s *Store) Users() []User {
	s.lockUsers()
	defer s.mu.Unlock()
	out := append([]User(nil), s.users...)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// UserByID finds a member.
func (s *Store) UserByID(id string) (User, bool) {
	s.lockUsers()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.ID == id {
			return u, true
		}
	}
	return User{}, false
}

// UserByName finds a member, ignoring case.
func (s *Store) UserByName(name string) (User, bool) {
	s.lockUsers()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if strings.EqualFold(u.Name, strings.TrimSpace(name)) {
			return u, true
		}
	}
	return User{}, false
}

// CleanName trims a display name and checks its length.
func CleanName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if n := utf8.RuneCountInString(name); n < 1 || n > 24 {
		return "", ErrBadName
	}
	return name, nil
}

// AddUser creates a member with an already-hashed PIN.
func (s *Store) AddUser(name, hash string, admin bool) (User, error) {
	name, err := CleanName(name)
	if err != nil {
		return User{}, err
	}
	s.lockUsers()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if strings.EqualFold(u.Name, name) {
			return User{}, ErrNameTaken
		}
	}
	id := make([]byte, 6)
	if _, err := rand.Read(id); err != nil {
		return User{}, err
	}
	u := User{ID: hex.EncodeToString(id), Name: name, Admin: admin, Hash: hash, Version: 1, Created: s.now().UTC()}
	s.users = append(s.users, u)
	if err := s.saveUsers(); err != nil {
		s.users = s.users[:len(s.users)-1]
		return User{}, err
	}
	return u, nil
}

// SetHash replaces a member's PIN hash and logs out their other sessions.
func (s *Store) SetHash(id, hash string) (User, error) {
	return s.updateUser(id, func(u *User) error { u.Hash = hash; u.Version++; return nil })
}

// Rename changes a member's display name.
func (s *Store) Rename(id, name string) (User, error) {
	name, err := CleanName(name)
	if err != nil {
		return User{}, err
	}
	return s.updateUser(id, func(u *User) error {
		for _, o := range s.users {
			if o.ID != id && strings.EqualFold(o.Name, name) {
				return ErrNameTaken
			}
		}
		u.Name = name
		return nil
	})
}

// SetAdmin grants or removes the admin role.
func (s *Store) SetAdmin(id string, admin bool) (User, error) {
	return s.updateUser(id, func(u *User) error {
		if !admin && u.Admin && s.adminsLocked() == 1 {
			return ErrLastAdmin
		}
		u.Admin = admin
		return nil
	})
}

func (s *Store) updateUser(id string, fn func(*User) error) (User, error) {
	s.lockUsers()
	defer s.mu.Unlock()
	for i := range s.users {
		if s.users[i].ID == id {
			old := s.users[i]
			if err := fn(&s.users[i]); err != nil {
				return User{}, err
			}
			if err := s.saveUsers(); err != nil {
				s.users[i] = old
				return User{}, err
			}
			return s.users[i], nil
		}
	}
	return User{}, ErrNotFound
}

// DeleteUser removes a member and their list (a final backup copy is kept).
func (s *Store) DeleteUser(id string) error {
	s.lockUsers()
	defer s.mu.Unlock()
	idx := -1
	for i, u := range s.users {
		if u.ID == id {
			idx = i
		}
	}
	if idx < 0 {
		return ErrNotFound
	}
	if s.users[idx].Admin && s.adminsLocked() == 1 {
		return ErrLastAdmin
	}
	old := s.users
	s.users = append(append([]User(nil), old[:idx]...), old[idx+1:]...)
	if err := s.saveUsers(); err != nil {
		s.users = old
		return err
	}
	p := s.listPath(id)
	if b, err := os.ReadFile(p); err == nil {
		_ = writeFile(filepath.Join(s.dir, "backup", "deleted-"+id+"-"+s.now().Format("20060102")+".json"), b, 0o640)
	}
	delete(s.lists, id)
	delete(s.listMod, id)
	_ = os.Remove(p)
	return nil
}

func (s *Store) adminsLocked() int {
	n := 0
	for _, u := range s.users {
		if u.Admin {
			n++
		}
	}
	return n
}

func (s *Store) saveUsers() error {
	b, err := json.MarshalIndent(s.users, "", " ")
	if err != nil {
		return err
	}
	if err := s.write("users.json", b); err != nil {
		return err
	}
	s.usersMod = modTime(filepath.Join(s.dir, "users.json"))
	return nil
}

// ---------- lists ----------

func newList() *List {
	l := &List{}
	fill(l)
	return l
}

func fill(l *List) {
	if l.Entries == nil {
		l.Entries = map[int]Entry{}
	}
	if l.Log == nil {
		l.Log = map[string]int{}
	}
	if l.Hidden == nil {
		l.Hidden = map[int]int64{}
	}
	if l.Taste == nil {
		l.Taste = map[string]int{}
	}
}

func (s *Store) listPath(uid string) string { return filepath.Join(s.dir, "lists", uid+".json") }

func (s *Store) listLocked(uid string) (*List, error) {
	mod := modTime(s.listPath(uid))
	if l, ok := s.lists[uid]; ok && mod.Equal(s.listMod[uid]) {
		return l, nil
	}
	l := newList()
	b, err := os.ReadFile(s.listPath(uid))
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(b, l); err != nil {
			return nil, fmt.Errorf("list %s: %w", uid, err)
		}
		fill(l)
	}
	s.lists[uid] = l
	s.listMod[uid] = mod
	return l, nil
}

func (s *Store) saveList(uid string, l *List) error {
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	if err := s.write(filepath.Join("lists", uid+".json"), b); err != nil {
		return err
	}
	s.listMod[uid] = modTime(s.listPath(uid))
	return nil
}

// Snapshot returns a copy of a member's list.
func (s *Store) Snapshot(uid string) (List, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, err := s.listLocked(uid)
	if err != nil {
		return List{}, err
	}
	out := *newList()
	for k, v := range l.Entries {
		out.Entries[k] = v
	}
	for k, v := range l.Log {
		out.Log[k] = v
	}
	for k, v := range l.Hidden {
		out.Hidden[k] = v
	}
	for k, v := range l.Taste {
		out.Taste[k] = v
	}
	out.Presets = append([]Preset(nil), l.Presets...)
	return out, nil
}

// Entry returns one entry.
func (s *Store) Entry(uid string, id int) (Entry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, err := s.listLocked(uid)
	if err != nil {
		return Entry{}, false, err
	}
	e, ok := l.Entries[id]
	return e, ok, nil
}

// Change describes a partial update; nil fields are left alone.
type Change struct {
	Status   *string `json:"status"`
	Episode  *int    `json:"episode"`
	Rating   *int    `json:"rating"`
	Note     *string `json:"note"`
	Priority *bool   `json:"priority"`
}

// Update applies c to an entry, creating it (as "plan") if needed. total is
// the title's episode count (0 if unknown) and caps the episode number.
// Reaching the last episode marks the entry done. Episode increases are
// added to today's log.
func (s *Store) Update(uid string, id int, c Change, total int, day string) (Entry, error) {
	if c.Status != nil && !ValidStatus(*c.Status) {
		return Entry{}, fmt.Errorf("unknown status %q", *c.Status)
	}
	if c.Rating != nil && (*c.Rating < 0 || *c.Rating > 10) {
		return Entry{}, errors.New("rating must be 0–10")
	}
	if c.Note != nil && utf8.RuneCountInString(*c.Note) > 500 {
		return Entry{}, errors.New("note is longer than 500 characters")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l, err := s.listLocked(uid)
	if err != nil {
		return Entry{}, err
	}
	old, existed := l.Entries[id]
	e := old
	if !existed {
		e.Status = Plan
	}
	now := s.now().UnixMilli()
	if c.Episode != nil {
		ep := max(0, *c.Episode)
		if total > 0 {
			ep = min(ep, total)
		}
		e.Episode = ep
		if ep > old.Episode && c.Status == nil && (e.Status == Plan || e.Status == Hold || e.Status == Drop) {
			e.Status = Watching
		}
		if total > 0 && ep == total && ep > old.Episode && c.Status == nil {
			e.Status = Done
		}
	}
	if c.Status != nil {
		e.Status = *c.Status
		if e.Status == Done && total > 0 && c.Episode == nil {
			e.Episode = total
		}
	}
	if c.Rating != nil {
		e.Rating = *c.Rating
	}
	if c.Note != nil {
		e.Note = strings.TrimSpace(*c.Note)
	}
	if c.Priority != nil {
		e.Priority = *c.Priority
	}
	if e.Status == Watching && e.Started == 0 {
		e.Started = now
	}
	if e.Status == Done && e.Finished == 0 {
		e.Finished = now
	}
	e.Updated = now
	l.Entries[id] = e
	// Only explicit episode changes count as watching; marking a title done
	// (which fills in the episode count) is not logged as today's viewing.
	// Going back an episode (a mis-tap) takes it off today's count again.
	delta := 0
	if c.Episode != nil {
		delta = e.Episode - old.Episode
	}
	oldLog, hadLog := l.Log[day]
	if delta != 0 && day != "" {
		if n := max(0, oldLog+delta); n > 0 {
			l.Log[day] = n
		} else {
			delete(l.Log, day)
		}
	}
	if err := s.saveList(uid, l); err != nil {
		if existed {
			l.Entries[id] = old
		} else {
			delete(l.Entries, id)
		}
		if hadLog {
			l.Log[day] = oldLog
		} else {
			delete(l.Log, day)
		}
		return Entry{}, err
	}
	return e, nil
}

// Remove deletes an entry.
func (s *Store) Remove(uid string, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, err := s.listLocked(uid)
	if err != nil {
		return err
	}
	old, ok := l.Entries[id]
	if !ok {
		return ErrNotFound
	}
	delete(l.Entries, id)
	if err := s.saveList(uid, l); err != nil {
		l.Entries[id] = old
		return err
	}
	return nil
}

// SetHidden marks a title "not interested" (or clears it).
func (s *Store) SetHidden(uid string, id int, hidden bool) error {
	return s.mutate(uid, func(l *List) error {
		if hidden {
			l.Hidden[id] = s.now().UnixMilli()
		} else {
			delete(l.Hidden, id)
		}
		return nil
	})
}

// SetTaste replaces the member's own taste settings.
func (s *Store) SetTaste(uid string, taste map[string]int) error {
	clean := map[string]int{}
	for k, v := range taste {
		if len(k) < 3 || len(k) > 64 || k[1] != ':' || !strings.ContainsRune("gtx", rune(k[0])) {
			return fmt.Errorf("bad taste key %q", k)
		}
		if v < 1 || v > 3 {
			continue // 0 clears
		}
		clean[k] = v
	}
	if len(clean) > 200 {
		return errors.New("too many taste settings")
	}
	return s.mutate(uid, func(l *List) error { l.Taste = clean; return nil })
}

// SetPresets replaces the saved browse filters.
func (s *Store) SetPresets(uid string, p []Preset) error {
	if len(p) > 30 {
		return errors.New("at most 30 saved filters")
	}
	for i := range p {
		p[i].Name = strings.TrimSpace(p[i].Name)
		if p[i].Name == "" || utf8.RuneCountInString(p[i].Name) > 40 || len(p[i].Query) > 1000 {
			return errors.New("preset name must be 1–40 characters")
		}
	}
	return s.mutate(uid, func(l *List) error { l.Presets = p; return nil })
}

func (s *Store) mutate(uid string, fn func(*List) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, err := s.listLocked(uid)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(l) // cheap deep copy for rollback
	if err := fn(l); err != nil {
		return err
	}
	if err := s.saveList(uid, l); err != nil {
		restored := newList()
		_ = json.Unmarshal(b, restored)
		fill(restored)
		s.lists[uid] = restored
		return err
	}
	return nil
}

// ApplyLegacy copies what the old Arsip kept beside the entries: priority
// marks (only onto entries that exist) and taste settings (only when the
// member has none yet). It returns how many priorities were set.
func (s *Store) ApplyLegacy(uid string, priority []int, taste map[string]int) (int, error) {
	n := 0
	err := s.mutate(uid, func(l *List) error {
		for _, id := range priority {
			if e, ok := l.Entries[id]; ok && !e.Priority {
				e.Priority = true
				l.Entries[id] = e
				n++
			}
		}
		if len(l.Taste) == 0 {
			for k, v := range taste {
				if v >= 1 && v <= 3 && len(k) > 2 && k[1] == ':' {
					l.Taste[k] = v
				}
			}
		}
		return nil
	})
	return n, err
}

// Import adds entries that are not on the list yet and returns how many were added.
func (s *Store) Import(uid string, entries map[int]Entry) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, err := s.listLocked(uid)
	if err != nil {
		return 0, err
	}
	n := 0
	for id, e := range entries {
		if _, ok := l.Entries[id]; ok || !ValidStatus(e.Status) {
			continue
		}
		l.Entries[id] = e
		n++
	}
	if n == 0 {
		return 0, nil
	}
	return n, s.saveList(uid, l)
}

// ---------- files ----------

// write stores rel atomically and keeps one backup of the previous version per day.
func (s *Store) write(rel string, b []byte) error {
	p := filepath.Join(s.dir, rel)
	if old, err := os.ReadFile(p); err == nil {
		day := s.now().Format("20060102")
		bp := filepath.Join(s.dir, "backup", day, rel)
		if _, err := os.Stat(bp); errors.Is(err, os.ErrNotExist) {
			_ = writeFile(bp, old, 0o640)
			s.pruneBackups(14)
		}
	}
	return writeFile(p, b, 0o640)
}

func (s *Store) pruneBackups(keep int) {
	dirs, err := os.ReadDir(filepath.Join(s.dir, "backup"))
	if err != nil {
		return
	}
	var days []string
	for _, d := range dirs {
		if d.IsDir() && len(d.Name()) == 8 {
			days = append(days, d.Name())
		}
	}
	sort.Strings(days)
	for len(days) > keep {
		_ = os.RemoveAll(filepath.Join(s.dir, "backup", days[0]))
		days = days[1:]
	}
}

func writeFile(path string, b []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
