package catalog

import (
	"math"
	"math/rand/v2"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Filter is everything the Jelajah page can narrow the catalogue by.
// Zero values mean "no constraint".
type Filter struct {
	Year, Season int // Season -1 = whole year; Year 0 = all years
	From, To     int // year range when Year == 0
	Query        string
	Genres       []string // must have all
	NoGenres     []string // must have none
	Tags         []string
	NoTags       []string
	Formats      []string
	Statuses     []string // FINISHED, RELEASING, NOT_YET_RELEASED …
	Length       string   // short ≤13, mid 14–26, long 27–60, xl >60 episodes
	Countries    []string
	MinScore     int
	MinPop       int // AniList members who listed the title
	Studio       string
	Tier         string
	NoSequels    bool
	Exclude      map[int]bool // e.g. titles already on the member's list
	Sort         string       // pop, worth, score, fav, gem, new, old, az
}

// SortKeys are the accepted Filter.Sort values, in menu order.
var SortKeys = []string{"pop", "worth", "score", "fav", "gem", "new", "old", "az"}

func (c *Catalog) names(list []string, vocab []string) map[int]bool {
	if len(list) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, n := range list {
		want[strings.ToLower(n)] = true
	}
	out := map[int]bool{}
	for i, n := range vocab {
		if want[strings.ToLower(n)] {
			out[i] = true
		}
	}
	if len(out) < len(want) {
		out[-1] = true // an unknown name can never match
	}
	return out
}

func tagNames(m *Meta) []string {
	out := make([]string, len(m.Tags))
	for i, t := range m.Tags {
		out[i] = t.Name
	}
	return out
}

func hasAll(have []int, want map[int]bool) bool {
	if want == nil {
		return true
	}
	if want[-1] {
		return false
	}
	n := 0
	for _, h := range have {
		if want[h] {
			n++
		}
	}
	return n == len(want)
}

func hasAny(have []int, bad map[int]bool) bool {
	for _, h := range have {
		if bad[h] {
			return true
		}
	}
	return false
}

func inSet(v string, set []string) bool {
	if len(set) == 0 {
		return true
	}
	for _, s := range set {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}

func lengthOK(a *Anime, bucket string) bool {
	ep := a.Episodes
	switch bucket {
	case "short":
		return ep > 0 && ep <= 13
	case "mid":
		return ep >= 14 && ep <= 26
	case "long":
		return ep >= 27 && ep <= 60
	case "xl":
		return ep > 60
	}
	return true
}

func gemScore(a *Anime) float64 {
	return float64(a.Score)*3 - math.Log10(float64(a.Popularity)+1)*10
}

// isGem: well rated, not widely seen, not a sequel, not overhyped.
func isGem(a *Anime) bool {
	return !a.Sequel && (a.Format == "TV" || a.Format == "MOVIE") && a.Score >= 72 &&
		a.Popularity >= 1000 && a.Popularity <= 60000 && (a.hypeGap == nil || *a.hypeGap <= 0)
}

// Browse returns every title matching f, sorted.
func (c *Catalog) Browse(f Filter) []*Anime {
	snap := c.snap.Load()
	tags := tagNames(snap.meta)
	gIn, gOut := c.names(f.Genres, snap.meta.Genres), c.names(f.NoGenres, snap.meta.Genres)
	tIn, tOut := c.names(f.Tags, tags), c.names(f.NoTags, tags)
	words := strings.Fields(normalize(f.Query))
	studio := strings.ToLower(strings.TrimSpace(f.Studio))
	var out []*Anime
	for _, a := range snap.items {
		switch {
		case f.Year != 0 && a.Year != f.Year:
			continue
		case f.Year != 0 && f.Season >= 0 && (a.Season == nil || *a.Season != f.Season):
			continue
		case f.Year == 0 && f.From != 0 && a.Year < f.From:
			continue
		case f.Year == 0 && f.To != 0 && a.Year > f.To:
			continue
		case !inSet(a.Format, f.Formats), !inSet(a.Status, f.Statuses), !inSet(a.Country, f.Countries):
			continue
		case f.MinScore > 0 && a.Score < f.MinScore, f.MinPop > 0 && a.Popularity < f.MinPop:
			continue
		case f.NoSequels && a.Sequel, f.Exclude[a.ID]:
			continue
		case f.Tier != "" && a.Tier != f.Tier:
			continue
		case f.Sort == "gem" && !isGem(a):
			continue
		case !lengthOK(a, f.Length):
			continue
		case !hasAll(a.genreIdx, gIn) || hasAny(a.genreIdx, gOut) || !hasAll(a.tagIdx, tIn) || hasAny(a.tagIdx, tOut):
			continue
		}
		if studio != "" {
			found := false
			for _, s := range a.Studios {
				if strings.Contains(strings.ToLower(s), studio) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if len(words) > 0 {
			found := true
			for _, w := range words {
				if !strings.Contains(a.search, w) {
					found = false
					break
				}
			}
			if !found {
				continue
			}
		}
		out = append(out, a)
	}
	less := map[string]func(x, y *Anime) bool{
		"pop":   func(x, y *Anime) bool { return x.Popularity > y.Popularity },
		"worth": func(x, y *Anime) bool { return x.Worth > y.Worth || x.Worth == y.Worth && x.Popularity > y.Popularity },
		"score": func(x, y *Anime) bool { return x.Score > y.Score || x.Score == y.Score && x.Popularity > y.Popularity },
		"fav":   func(x, y *Anime) bool { return x.favorites > y.favorites },
		"gem":   func(x, y *Anime) bool { return gemScore(x) > gemScore(y) },
		"new":   func(x, y *Anime) bool { return x.date > y.date },
		"old":   func(x, y *Anime) bool { return x.date < y.date },
		"az":    func(x, y *Anime) bool { return strings.ToLower(x.Title) < strings.ToLower(y.Title) },
	}[f.Sort]
	if less == nil {
		less = func(x, y *Anime) bool { return x.Popularity > y.Popularity }
	}
	sort.SliceStable(out, func(i, j int) bool { return less(out[i], out[j]) })
	return out
}

// Similar ranks titles by shared themes (tags weighted by rarity, genres) plus
// AniList's own recommendation links. Other entries of the same franchise and
// sequels are skipped: you will find those anyway.
func (c *Catalog) Similar(id, n int) []*Anime {
	snap := c.snap.Load()
	a, ok := snap.byID[id]
	if !ok {
		return nil
	}
	vec := func(x *Anime) map[int]float64 {
		v := map[int]float64{}
		for i, t := range x.tagIdx {
			if t >= 0 && t < len(snap.idf) {
				v[t] = snap.idf[t] / (1 + 0.12*float64(i))
			}
		}
		for _, g := range x.genreIdx {
			v[-1-g] = 0.8
		}
		return v
	}
	norm := func(v map[int]float64) float64 {
		s := 0.0
		for _, x := range v {
			s += x * x
		}
		return math.Sqrt(s)
	}
	me := vec(a)
	myNorm := norm(me)
	if myNorm == 0 {
		myNorm = 1
	}
	recs := map[int]int{}
	for _, r := range c.graphFor(id) {
		recs[r[0]] = r[1]
	}
	type hit struct {
		a     *Anime
		score float64
	}
	var hits []hit
	for _, b := range snap.items {
		if b.ID == a.ID || b.Sequel || b.Popularity < 300 || (a.franchise != 0 && b.franchise == a.franchise) {
			continue
		}
		v := vec(b)
		dot := 0.0
		for k, x := range v {
			dot += x * me[k]
		}
		score := 0.0
		if bn := norm(v); bn > 0 {
			score = dot / (myNorm * bn)
		}
		if r, ok := recs[b.ID]; ok {
			score += 0.25 + math.Min(0.5, math.Log10(float64(r)+1)/4)
		}
		if score > 0.05 {
			hits = append(hits, hit{b, score + float64(max(b.Score, 60))/1000})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) > n {
		hits = hits[:n]
	}
	out := make([]*Anime, len(hits))
	for i, h := range hits {
		out[i] = h.a
	}
	return out
}

func (c *Catalog) graphFor(id int) [][2]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.graph == nil {
		c.graph = map[string][][2]int{}
		_ = readJSON(filepath.Join(c.dir, "graph.json.gz"), &c.graph)
	}
	return c.graph[strconv.Itoa(id)]
}

// Schedule lists every known episode airing between from and to, by time.
func (c *Catalog) Schedule(from, to time.Time) []Airing {
	var out []Airing
	for _, list := range c.snap.Load().airing {
		for _, a := range list {
			if a.At >= from.Unix() && a.At < to.Unix() {
				out = append(out, a)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At < out[j].At || out[i].At == out[j].At && out[i].ID < out[j].ID })
	return out
}

// Released returns how many episodes of id have aired by now, when the
// schedule tells us (the next episode number minus one).
func (c *Catalog) Released(id int, now time.Time) (int, bool) {
	a, ok := c.snap.Load().byID[id]
	if !ok {
		return 0, false
	}
	if a.Status == "FINISHED" {
		return a.Episodes, a.Episodes > 0
	}
	if next, ok := c.NextAiring(id, now); ok {
		if next.At <= now.Unix() {
			return next.Episode, true // airing right now
		}
		return next.Episode - 1, true
	}
	return 0, false
}

// Pick returns a random title from list (nil when empty).
func Pick(list []*Anime) *Anime {
	if len(list) == 0 {
		return nil
	}
	return list[rand.IntN(len(list))]
}
