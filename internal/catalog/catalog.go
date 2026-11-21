// Package catalog reads the anime catalogue produced by the Python fetchers
// (~/server/watchlist) and answers search, season and detail queries in memory.
//
// The catalogue is read-only from this service's point of view. A background
// loop reloads it when index.json(.gz) changes on disk, so the nightly fetch
// jobs keep working without restarting the server.
package catalog

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
)

// Season indexes as stored in index.json.
const (
	Winter = iota
	Spring
	Summer
	Fall
)

// Anime is the compact record served to clients.
type Anime struct {
	ID         int      `json:"id"`
	Title      string   `json:"title"`
	English    string   `json:"english,omitempty"`
	Native     string   `json:"native,omitempty"`
	Year       int      `json:"year"`
	Season     *int     `json:"season,omitempty"`
	Format     string   `json:"format,omitempty"`
	Status     string   `json:"status,omitempty"`
	Episodes   int      `json:"episodes,omitempty"`
	Duration   int      `json:"duration,omitempty"`
	Score      int      `json:"score,omitempty"`
	Popularity int      `json:"popularity"`
	Image      string   `json:"image"`
	Genres     []string `json:"genres,omitempty"`
	Studios    []string `json:"studios,omitempty"`

	synonyms []string
	search   string // normalised title + synonyms, for substring search
}

// Detail adds the per-year detail file and the Indonesian synopsis.
type Detail struct {
	Anime
	Synopsis     string `json:"synopsis,omitempty"`
	SynopsisLang string `json:"synopsisLang,omitempty"`
	Links        []Link `json:"links,omitempty"`
}

// Link is a legal streaming page for a title.
type Link struct {
	Site string `json:"site"`
	URL  string `json:"url"`
}

// Airing is the next scheduled episode of a title.
type Airing struct {
	ID      int   `json:"id"`
	Episode int   `json:"episode"`
	At      int64 `json:"at"` // unix seconds
}

type snapshot struct {
	items   []*Anime
	byID    map[int]*Anime
	airing  map[int][]Airing // per title, sorted by time
	modTime time.Time
}

// Catalog is safe for concurrent use.
type Catalog struct {
	dir      string
	synPath  string
	snap     atomic.Pointer[snapshot]
	mu       sync.Mutex
	details  map[int]map[string]rawDetail // year → id → detail, loaded lazily
	synopsID map[string]string
}

// Open loads the catalogue from dir. synopsisPath (optional) points to the
// Indonesian synopsis file keyed by AniList id.
func Open(dir, synopsisPath string) (*Catalog, error) {
	c := &Catalog{dir: dir, synPath: synopsisPath}
	if err := c.Reload(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Catalog) indexPath() (string, os.FileInfo, error) {
	for _, name := range []string{"index.json.gz", "index.json"} {
		p := filepath.Join(c.dir, name)
		if fi, err := os.Stat(p); err == nil {
			return p, fi, nil
		}
	}
	return "", nil, fmt.Errorf("catalog: no index.json(.gz) in %s", c.dir)
}

// Reload re-reads the catalogue if index.json changed since the last load.
func (c *Catalog) Reload() error {
	path, fi, err := c.indexPath()
	if err != nil {
		return err
	}
	old := c.snap.Load()
	if old != nil && fi.ModTime().Equal(old.modTime) {
		// Index unchanged; airing.json is refreshed daily on its own.
		air := c.loadAiring()
		if air != nil {
			c.snap.Store(&snapshot{items: old.items, byID: old.byID, airing: air, modTime: old.modTime})
		}
		return nil
	}
	items, err := loadIndex(path)
	if err != nil {
		return err
	}
	byID := make(map[int]*Anime, len(items))
	for _, a := range items {
		byID[a.ID] = a
	}
	air := c.loadAiring()
	if air == nil {
		air = map[int][]Airing{}
	}
	c.snap.Store(&snapshot{items: items, byID: byID, airing: air, modTime: fi.ModTime()})

	c.mu.Lock()
	c.details = map[int]map[string]rawDetail{} // detail files are regenerated together with the index
	c.mu.Unlock()
	return nil
}

// Watch reloads the catalogue every interval until stop is closed.
func (c *Catalog) Watch(interval time.Duration, stop <-chan struct{}, logf func(string, ...any)) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if err := c.Reload(); err != nil {
				logf("catalog reload: %v", err)
			}
		}
	}
}

// Len is the number of titles.
func (c *Catalog) Len() int { return len(c.snap.Load().items) }

// Get returns a title by id.
func (c *Catalog) Get(id int) (*Anime, bool) {
	a, ok := c.snap.Load().byID[id]
	return a, ok
}

// NextAiring returns the next scheduled episode for id. An episode stays
// "next" for three hours after it starts so a show airing now still shows up.
func (c *Catalog) NextAiring(id int, now time.Time) (Airing, bool) {
	for _, a := range c.snap.Load().airing[id] {
		if a.At >= now.Unix()-3*3600 {
			return a, true
		}
	}
	return Airing{}, false
}

// Season lists the titles that started in the given season, most popular first.
func (c *Catalog) Season(year, season int) []*Anime {
	var out []*Anime
	for _, a := range c.snap.Load().items {
		if a.Year == year && a.Season != nil && *a.Season == season {
			out = append(out, a)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Popularity > out[j].Popularity })
	return out
}

// Search finds titles whose romaji, English, native title or synonyms contain
// every word of q. Titles starting with the query rank first, then popularity.
func (c *Catalog) Search(q string, limit int) []*Anime {
	words := strings.Fields(normalize(q))
	if len(words) == 0 {
		return nil
	}
	whole := strings.Join(words, " ")
	type hit struct {
		a    *Anime
		rank int
	}
	var hits []hit
	for _, a := range c.snap.Load().items {
		ok := true
		for _, w := range words {
			if !strings.Contains(a.search, w) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		rank := 2
		if strings.HasPrefix(a.search, whole) || strings.Contains(a.search, "|"+whole) {
			rank = 0
		} else if strings.Contains(a.search, " "+whole) {
			rank = 1
		}
		hits = append(hits, hit{a, rank})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].rank != hits[j].rank {
			return hits[i].rank < hits[j].rank
		}
		return hits[i].a.Popularity > hits[j].a.Popularity
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]*Anime, len(hits))
	for i, h := range hits {
		out[i] = h.a
	}
	return out
}

// Detail returns the title with synopsis and streaming links.
func (c *Catalog) Detail(id int) (*Detail, bool) {
	a, ok := c.Get(id)
	if !ok {
		return nil, false
	}
	d := &Detail{Anime: *a}
	if raw, ok := c.rawDetail(a.Year, id); ok {
		d.Synopsis, d.SynopsisLang = raw.Desc, "en"
		for _, l := range raw.Links {
			if len(l) >= 2 {
				site, _ := l[0].(string)
				url, _ := l[1].(string)
				if site != "" && strings.HasPrefix(url, "https://") {
					d.Links = append(d.Links, Link{Site: site, URL: url})
				}
			}
		}
	}
	if s := c.synopsisID(id); s != "" {
		d.Synopsis, d.SynopsisLang = s, "id"
	}
	return d, true
}

// ---------- loading ----------

type rawIndex struct {
	Genres  []string   `json:"genres"`
	Studios []string   `json:"studios"`
	Items   []rawAnime `json:"items"`
}

type rawAnime struct {
	ID       int      `json:"i"`
	Title    string   `json:"t"`
	English  string   `json:"e"`
	Native   string   `json:"n"`
	Year     int      `json:"y"`
	Season   *int     `json:"s"`
	Format   string   `json:"f"`
	Status   string   `json:"st"`
	Episodes int      `json:"ep"`
	Duration int      `json:"du"`
	Score    int      `json:"sc"`
	Pop      int      `json:"p"`
	Image    string   `json:"img"`
	Genres   []int    `json:"g"`
	Studios  []int    `json:"stu"`
	Synonyms []string `json:"syn"`
}

type rawDetail struct {
	Desc  string  `json:"d"`
	Links [][]any `json:"li"`
}

func openMaybeGz(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(path, ".gz") {
		return f, nil
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return struct {
		io.Reader
		io.Closer
	}{gz, f}, nil
}

func readJSON(path string, v any) error {
	r, err := openMaybeGz(path)
	if err != nil {
		return err
	}
	defer r.Close()
	return json.NewDecoder(r).Decode(v)
}

func loadIndex(path string) ([]*Anime, error) {
	var raw rawIndex
	if err := readJSON(path, &raw); err != nil {
		return nil, fmt.Errorf("catalog: read %s: %w", path, err)
	}
	if len(raw.Items) == 0 {
		return nil, errors.New("catalog: index has no items")
	}
	pick := func(names []string, idx []int) []string {
		var out []string
		for _, i := range idx {
			if i >= 0 && i < len(names) {
				out = append(out, names[i])
			}
		}
		return out
	}
	items := make([]*Anime, 0, len(raw.Items))
	for _, r := range raw.Items {
		if r.ID == 0 || r.Title == "" {
			continue
		}
		a := &Anime{
			ID: r.ID, Title: r.Title, English: r.English, Native: r.Native, Year: r.Year, Season: r.Season,
			Format: r.Format, Status: r.Status, Episodes: r.Episodes, Duration: r.Duration, Score: r.Score,
			Popularity: r.Pop, Image: r.Image, Genres: pick(raw.Genres, r.Genres), Studios: pick(raw.Studios, r.Studios),
			synonyms: r.Synonyms,
		}
		parts := append([]string{r.Title, r.English, r.Native}, r.Synonyms...)
		var keys []string
		for _, p := range parts {
			if n := normalize(p); n != "" {
				keys = append(keys, n)
				if compact := strings.ReplaceAll(n, " ", ""); compact != n {
					keys = append(keys, compact)
				}
			}
		}
		a.search = strings.Join(keys, "|")
		items = append(items, a)
	}
	return items, nil
}

func (c *Catalog) loadAiring() map[int][]Airing {
	var raw struct {
		Items [][3]int64 `json:"items"`
	}
	if err := readJSON(filepath.Join(c.dir, "airing.json"), &raw); err != nil {
		return nil
	}
	out := make(map[int][]Airing, len(raw.Items))
	for _, it := range raw.Items {
		id := int(it[0])
		out[id] = append(out[id], Airing{ID: id, Episode: int(it[1]), At: it[2]})
	}
	for _, list := range out {
		sort.Slice(list, func(i, j int) bool { return list[i].At < list[j].At })
	}
	return out
}

func (c *Catalog) rawDetail(year, id int) (rawDetail, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.details == nil {
		c.details = map[int]map[string]rawDetail{}
	}
	m, ok := c.details[year]
	if !ok {
		m = map[string]rawDetail{}
		_ = readJSON(filepath.Join(c.dir, fmt.Sprintf("details-%d.json.gz", year)), &m)
		c.details[year] = m
	}
	d, ok := m[strconv.Itoa(id)]
	return d, ok
}

func (c *Catalog) synopsisID(id int) string {
	if c.synPath == "" {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.synopsID == nil {
		c.synopsID = map[string]string{}
		_ = readJSON(c.synPath, &c.synopsID)
	}
	return c.synopsID[strconv.Itoa(id)]
}

// normalize lowercases and keeps letters/digits, collapsing everything else to
// single spaces, so "Re:Zero" matches "re zero". A space-free copy of every
// title is indexed too, so "rezero" matches as well.
func normalize(s string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// CurrentSeason returns the anime season containing t.
func CurrentSeason(t time.Time) (year, season int) {
	return t.Year(), (int(t.Month()) - 1) / 3
}
