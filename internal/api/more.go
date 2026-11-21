package api

import (
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"tonton/internal/catalog"
	"tonton/internal/store"
)

// ---------- Nonton (home) ----------

type dayCount struct {
	Day string `json:"day"`
	N   int    `json:"n"`
}

type decision struct {
	Item
	Reason string `json:"reason"`
}

func (s *Server) home(w http.ResponseWriter, r *http.Request, u store.User) {
	items, counts, l, ok := s.entries(w, u.ID)
	if !ok {
		return
	}
	byUpdated(items)
	now := s.now()
	ml := s.temuanFor(u.ID)
	weekEnd := now.Add(7 * 24 * time.Hour).Unix()

	watching, upcoming, waiting := []Item{}, []Item{}, []Item{}
	var queue []Item
	decisions := []decision{}
	ready := 0
	for _, it := range items {
		e := it.Entry
		days := int(now.Sub(time.UnixMilli(e.Updated)).Hours() / 24)
		switch e.Status {
		case store.Watching:
			if n, ok := s.Cat.Released(it.Anime.ID, now); ok {
				it.Released = &n
				ready += max(0, n-e.Episode)
			}
			watching = append(watching, it)
			if days >= 14 {
				decisions = append(decisions, decision{it, fmt.Sprintf("Tidak ada episode baru sejak %d hari lalu", days)})
			}
		case store.Hold:
			if days >= 30 {
				decisions = append(decisions, decision{it, fmt.Sprintf("Ditunda sejak %d hari lalu", days)})
			}
		case store.Plan:
			if it.Anime.Status == "NOT_YET_RELEASED" {
				waiting = append(waiting, it)
			} else if l.Hidden[it.Anime.ID] == 0 {
				if ml != nil {
					it.Match = ml.Scores[strconv.Itoa(it.Anime.ID)]
				}
				queue = append(queue, it)
			}
		}
		if it.Next != nil && it.Next.At <= weekEnd && (e.Status == store.Watching || e.Status == store.Plan) {
			upcoming = append(upcoming, it)
		}
	}
	sort.SliceStable(queue, func(i, j int) bool { return queueLess(queue[i], queue[j]) })
	sort.Slice(upcoming, func(i, j int) bool { return upcoming[i].Next.At < upcoming[j].Next.At })
	sort.Slice(waiting, func(i, j int) bool { return airKey(waiting[i]) < airKey(waiting[j]) })
	if len(decisions) > 6 {
		decisions = decisions[:6]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"watching": watching, "upcoming": upcoming, "queue": head(queue, 8), "queueTotal": len(queue),
		"waiting": head(waiting, 6), "decisions": decisions, "counts": counts, "ready": ready,
		"activity": s.activity(l, 84), "streak": s.streak(l), "today": l.Log[s.today()],
		"week": s.sumDays(l, 0, 7), "lastWeek": s.sumDays(l, 7, 14),
	})
}

// queueLess orders the plan: priority first, then the recommendation model's
// fit, then how "layak tonton" the title is.
func queueLess(a, b Item) bool {
	if a.Entry.Priority != b.Entry.Priority {
		return a.Entry.Priority
	}
	if a.Match != b.Match {
		return a.Match > b.Match
	}
	if a.Anime.Worth != b.Anime.Worth {
		return a.Anime.Worth > b.Anime.Worth
	}
	return a.Anime.Popularity > b.Anime.Popularity
}

func airKey(it Item) int64 {
	if it.Next != nil {
		return it.Next.At
	}
	return int64(it.Anime.Year)*1e10 + int64(seasonOf(it.Anime))*1e9
}

func seasonOf(a *catalog.Anime) int {
	if a.Season == nil {
		return 4
	}
	return *a.Season
}

func head[T any](list []T, n int) []T {
	if len(list) > n {
		return list[:n]
	}
	if list == nil {
		return []T{}
	}
	return list
}

func (s *Server) dayString(offset int) string {
	return s.now().AddDate(0, 0, -offset).Format("2006-01-02")
}

func (s *Server) sumDays(l store.List, from, to int) int {
	n := 0
	for i := from; i < to; i++ {
		n += l.Log[s.dayString(i)]
	}
	return n
}

func (s *Server) activity(l store.List, days int) []dayCount {
	out := make([]dayCount, days)
	for i := 0; i < days; i++ {
		d := s.dayString(days - 1 - i)
		out[i] = dayCount{d, l.Log[d]}
	}
	return out
}

// streak counts consecutive days with at least one episode, ending today
// (or yesterday, so the streak does not look broken first thing in the morning).
func (s *Server) streak(l store.List) int {
	start := 0
	if l.Log[s.dayString(0)] == 0 {
		start = 1
	}
	n := 0
	for i := start; i < 3650 && l.Log[s.dayString(i)] > 0; i++ {
		n++
	}
	return n
}

// ---------- catalogue browsing ----------

var sortLabels = map[string]string{
	"pop": "Paling populer", "worth": "Paling layak tonton", "score": "Skor tertinggi", "fav": "Paling difavoritkan",
	"gem": "Hidden gem", "new": "Terbaru", "old": "Terlama", "az": "Judul A–Z",
}

func (s *Server) meta(w http.ResponseWriter, r *http.Request, _ store.User) {
	m := s.Cat.Meta()
	tags := []catalog.TagInfo{}
	for _, t := range m.Tags {
		if t.Count >= 15 && !strings.HasPrefix(t.Category, "Sexual") {
			tags = append(tags, t)
		}
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Count > tags[j].Count })
	sorts := make([][2]string, len(catalog.SortKeys))
	for i, k := range catalog.SortKeys {
		sorts[i] = [2]string{k, sortLabels[k]}
	}
	y, season := catalog.CurrentSeason(s.now())
	writeJSON(w, http.StatusOK, map[string]any{
		"genres": m.Genres, "tags": tags, "studios": m.Studios, "tiers": m.Tiers, "years": m.Years,
		"sorts": sorts, "current": map[string]int{"year": y, "season": season},
	})
}

func splitList(v string) []string {
	if v == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(v, "|") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *Server) browse(w http.ResponseWriter, r *http.Request, u store.User) {
	q := r.URL.Query()
	num := func(k string, def int) int {
		if n, err := strconv.Atoi(q.Get(k)); err == nil {
			return n
		}
		return def
	}
	f := catalog.Filter{
		Year: num("year", 0), Season: num("season", -1), From: num("from", 0), To: num("to", 0),
		Query: q.Get("q"), Genres: splitList(q.Get("g")), NoGenres: splitList(q.Get("gx")),
		Tags: splitList(q.Get("t")), NoTags: splitList(q.Get("tx")), Formats: splitList(q.Get("fmt")),
		Statuses: splitList(q.Get("st")), Length: q.Get("len"), Countries: splitList(q.Get("co")),
		MinScore: num("score", 0), MinPop: num("pop", 0), Studio: q.Get("studio"), Tier: q.Get("tier"),
		NoSequels: q.Get("nosq") == "1", Sort: q.Get("sort"),
	}
	if len(f.Query) > 100 {
		f.Query = f.Query[:100]
	}
	l, err := s.Store.Snapshot(u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Daftar gagal dibaca.")
		return
	}
	if q.Get("hide") == "1" {
		f.Exclude = map[int]bool{}
		for id := range l.Entries {
			f.Exclude[id] = true
		}
		for id := range l.Hidden {
			f.Exclude[id] = true
		}
	}
	all := s.Cat.Browse(f)
	if q.Get("random") == "1" {
		a := catalog.Pick(all)
		if a == nil {
			writeErr(w, http.StatusNotFound, "Tidak ada judul yang cocok dengan filter ini.")
			return
		}
		items := s.attach(l, []*catalog.Anime{a})
		writeJSON(w, http.StatusOK, map[string]any{"item": items[0]})
		return
	}
	size := min(max(num("size", 60), 1), 120)
	page := max(num("page", 1), 1)
	start := min((page-1)*size, len(all))
	end := min(start+size, len(all))
	writeJSON(w, http.StatusOK, map[string]any{
		"total": len(all), "page": page, "size": size, "items": s.attach(l, all[start:end]),
	})
}

// attach pairs catalogue titles with the member's entries.
func (s *Server) attach(l store.List, as []*catalog.Anime) []Item {
	out := make([]Item, len(as))
	for i, a := range as {
		var ep *store.Entry
		if e, ok := l.Entries[a.ID]; ok {
			ep = &e
		}
		out[i] = Item{Anime: a, Entry: ep}
	}
	return out
}

func (s *Server) similar(w http.ResponseWriter, r *http.Request, u store.User) {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "Judul tidak dikenal.")
		return
	}
	l, err := s.Store.Snapshot(u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Daftar gagal dibaca.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.attach(l, s.Cat.Similar(id, 12))})
}

func (s *Server) schedule(w http.ResponseWriter, r *http.Request, u store.User) {
	l, err := s.Store.Snapshot(u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Daftar gagal dibaca.")
		return
	}
	mine := r.URL.Query().Get("scope") != "all"
	now := s.now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.Loc)
	out := []Item{}
	for _, air := range s.Cat.Schedule(start, start.AddDate(0, 0, 7)) {
		a, ok := s.Cat.Get(air.ID)
		if !ok {
			continue
		}
		var ep *store.Entry
		if e, ok := l.Entries[a.ID]; ok {
			ep = &e
		}
		if mine && (ep == nil || (ep.Status != store.Watching && ep.Status != store.Plan)) {
			continue
		}
		air := air
		out = append(out, Item{Anime: a, Entry: ep, Next: &air})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// pick draws a random released title from the plan ("gacha").
func (s *Server) pick(w http.ResponseWriter, r *http.Request, u store.User) {
	l, err := s.Store.Snapshot(u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Daftar gagal dibaca.")
		return
	}
	genre := r.URL.Query().Get("genre")
	var pool []*catalog.Anime
	for id, e := range l.Entries {
		a, ok := s.Cat.Get(id)
		if !ok || e.Status != store.Plan || a.Status == "NOT_YET_RELEASED" {
			continue
		}
		if genre != "" && !inStrings(genre, a.Genres) {
			continue
		}
		pool = append(pool, a)
	}
	a := catalog.Pick(pool)
	if a == nil {
		writeErr(w, http.StatusNotFound, "Tidak ada judul rencana yang cocok.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": s.attach(l, []*catalog.Anime{a})[0]})
}

func inStrings(v string, list []string) bool {
	for _, x := range list {
		if strings.EqualFold(x, v) {
			return true
		}
	}
	return false
}

// ---------- Temuan (recommendations) ----------

type temuanFile struct {
	Generated string `json:"generated"`
	Model     struct {
		Trained   string   `json:"trained"`
		Positives int      `json:"positives"`
		Negatives int      `json:"negatives"`
		AUC       *float64 `json:"auc"`
		Likes     []string `json:"likes"`
		Dislikes  []string `json:"dislikes"`
	} `json:"model"`
	NextBatch string `json:"next_batch"`
	Batches   []struct {
		Date  string `json:"date"`
		Items []struct {
			I   int      `json:"i"`
			P   float64  `json:"p"`
			K   string   `json:"k"`
			Why []string `json:"why"`
		} `json:"items"`
	} `json:"batches"`
	Scores map[string]float64 `json:"scores"`
}

var temuanCache = struct {
	sync.Mutex
	m map[string]cachedTemuan
}{m: map[string]cachedTemuan{}}

type cachedTemuan struct {
	mod time.Time
	t   *temuanFile
}

func (s *Server) temuanPath(uid string) string {
	return filepath.Join(s.DataDir, "temuan", uid+".json")
}

// temuanFor reads a member's recommendation file written by discover.py.
func (s *Server) temuanFor(uid string) *temuanFile {
	if s.DataDir == "" {
		return nil
	}
	p := s.temuanPath(uid)
	fi, err := os.Stat(p)
	if err != nil {
		return nil
	}
	temuanCache.Lock()
	defer temuanCache.Unlock()
	if c, ok := temuanCache.m[uid]; ok && c.mod.Equal(fi.ModTime()) {
		return c.t
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	t := &temuanFile{}
	if json.Unmarshal(b, t) != nil {
		return nil
	}
	temuanCache.m[uid] = cachedTemuan{fi.ModTime(), t}
	return t
}

type batchOut struct {
	Date  string `json:"date"`
	Items []Item `json:"items"`
}

func (s *Server) temuan(w http.ResponseWriter, r *http.Request, u store.User) {
	l, err := s.Store.Snapshot(u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Daftar gagal dibaca.")
		return
	}
	t := s.temuanFor(u.ID)
	filter := r.URL.Query().Get("filter")
	counts := map[string]int{"open": 0, "all": 0, "yes": 0, "no": 0}
	batches := []batchOut{}
	if t != nil {
		for _, b := range t.Batches {
			out := batchOut{Date: b.Date, Items: []Item{}}
			for _, x := range b.Items {
				a, ok := s.Cat.Get(x.I)
				if !ok {
					continue
				}
				e, onList := l.Entries[x.I]
				_, hidden := l.Hidden[x.I]
				state := "open"
				if onList {
					state = "yes"
				} else if hidden {
					state = "no"
				}
				counts["all"]++
				counts[state]++
				if filter != "all" && state != filter && !(filter == "" && state == "open") {
					continue
				}
				it := s.item(a, nil)
				if onList {
					it.Entry = &e
				}
				it.Why, it.Match, it.Kind = x.Why, x.P, x.K
				out.Items = append(out.Items, it)
			}
			if len(out.Items) > 0 {
				batches = append(batches, out)
			}
		}
	}
	resp := map[string]any{"batches": batches, "counts": counts, "taste": l.Taste, "hidden": len(l.Hidden)}
	if t != nil {
		resp["model"] = t.Model
		resp["next"] = t.NextBatch
		resp["generated"] = t.Generated
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) setHidden(w http.ResponseWriter, r *http.Request, u store.User) {
	id, ok := pathID(r)
	if _, found := s.Cat.Get(id); !ok || !found {
		writeErr(w, http.StatusNotFound, "Judul tidak ada di katalog.")
		return
	}
	var body struct {
		Hidden bool `json:"hidden"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if err := s.Store.SetHidden(u.ID, id, body.Hidden); err != nil {
		writeErr(w, http.StatusInternalServerError, "Perubahan gagal disimpan.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setTaste(w http.ResponseWriter, r *http.Request, u store.User) {
	var body struct {
		Taste map[string]int `json:"taste"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if err := s.Store.SetTaste(u.ID, body.Taste); err != nil {
		writeErr(w, http.StatusBadRequest, "Selera tidak valid.")
		return
	}
	// Ask the nightly job to re-pick this member's latest batch soon.
	if s.DataDir != "" {
		dir := filepath.Join(s.DataDir, "temuan")
		if os.MkdirAll(dir, 0o750) == nil {
			_ = os.WriteFile(filepath.Join(dir, u.ID+".rebatch"), []byte(s.now().Format(time.RFC3339)), 0o640)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- saved filters ----------

func (s *Server) presets(w http.ResponseWriter, r *http.Request, u store.User) {
	l, err := s.Store.Snapshot(u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Daftar gagal dibaca.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"presets": head(l.Presets, 30)})
}

func (s *Server) setPresets(w http.ResponseWriter, r *http.Request, u store.User) {
	var body struct {
		Presets []store.Preset `json:"presets"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if err := s.Store.SetPresets(u.ID, body.Presets); err != nil {
		writeErr(w, http.StatusBadRequest, "Nama filter harus 1–40 karakter, paling banyak 30 filter.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- statistics & export ----------

type named struct {
	Name string `json:"name"`
	N    int    `json:"n"`
}

func top(m map[string]int, n int) []named {
	out := make([]named, 0, len(m))
	for k, v := range m {
		out = append(out, named{k, v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].N > out[j].N || out[i].N == out[j].N && out[i].Name < out[j].Name })
	return head(out, n)
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request, u store.User) {
	items, counts, l, ok := s.entries(w, u.ID)
	if !ok {
		return
	}
	genres, studios, years := map[string]int{}, map[string]int{}, map[string]int{}
	episodes, minutes, rated, ratingSum := 0, 0, 0, 0
	for _, it := range items {
		e, a := it.Entry, it.Anime
		episodes += e.Episode
		minutes += e.Episode * max(a.Duration, 0)
		if e.Rating > 0 {
			rated++
			ratingSum += e.Rating
		}
		if e.Status != store.Done && e.Status != store.Watching {
			continue
		}
		for _, g := range a.Genres {
			genres[g]++
		}
		for _, st := range head(a.Studios, 1) {
			studios[st]++
		}
		years[strconv.Itoa(a.Year)]++
	}
	byYear := make([]named, 0, len(years))
	for y, n := range years {
		byYear = append(byYear, named{y, n})
	}
	sort.Slice(byYear, func(i, j int) bool { return byYear[i].Name < byYear[j].Name })
	avg := 0.0
	if rated > 0 {
		avg = float64(ratingSum) / float64(rated)
	}
	best := 0
	for _, n := range l.Log {
		best = max(best, n)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"counts": counts, "episodes": episodes, "hours": minutes / 60, "rated": rated, "avgRating": avg,
		"genres": top(genres, 8), "studios": top(studios, 6), "years": byYear,
		"streak": s.streak(l), "bestDay": best, "activity": s.activity(l, 84),
	})
}

func (s *Server) export(w http.ResponseWriter, r *http.Request, u store.User) {
	items, _, _, ok := s.entries(w, u.ID)
	if !ok {
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Anime.Title < items[j].Anime.Title })
	day := s.now().Format("2006-01-02")
	if r.URL.Query().Get("format") == "json" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="tonton-%s.json"`, day))
		writeJSON(w, http.StatusOK, map[string]any{"exported": day, "member": u.Name, "items": items})
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="tonton-%s.csv"`, day))
	_, _ = w.Write([]byte("\xef\xbb\xbf")) // so Excel reads UTF-8
	c := csv.NewWriter(w)
	_ = c.Write([]string{"id", "judul", "judul_inggris", "status", "episode", "total_episode", "nilai", "prioritas", "catatan", "tahun", "format", "anilist"})
	for _, it := range items {
		a, e := it.Anime, it.Entry
		pr := ""
		if e.Priority {
			pr = "ya"
		}
		_ = c.Write([]string{strconv.Itoa(a.ID), csvSafe(a.Title), csvSafe(a.English), e.Status, strconv.Itoa(e.Episode),
			strconv.Itoa(a.Episodes), strconv.Itoa(e.Rating), pr, csvSafe(e.Note), strconv.Itoa(a.Year), a.Format,
			"https://anilist.co/anime/" + strconv.Itoa(a.ID)})
	}
	c.Flush()
}

// csvSafe stops spreadsheet apps from treating a note as a formula.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}

// ---------- dashboard summary ----------

// summary feeds the home-server dashboard tile. It needs the shared token.
func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if s.Token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.Token)) != 1 {
		writeErr(w, http.StatusUnauthorized, "Token salah.")
		return
	}
	type watch struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
		Image string `json:"image"`
		Ep    int    `json:"ep"`
		Total int    `json:"total,omitempty"`
		At    int64  `json:"at"`
	}
	type member struct {
		Name     string         `json:"name"`
		Admin    bool           `json:"admin"`
		Counts   map[string]int `json:"counts"`
		Watching []watch        `json:"watching"`
		Week     int            `json:"week"`
		Streak   int            `json:"streak"`
		Temuan   int            `json:"temuanOpen"`
	}
	out := []member{}
	for _, u := range s.Store.Users() {
		l, err := s.Store.Snapshot(u.ID)
		if err != nil {
			continue
		}
		m := member{Name: u.Name, Admin: u.Admin, Counts: map[string]int{}, Watching: []watch{},
			Week: s.sumDays(l, 0, 7), Streak: s.streak(l)}
		for id, e := range l.Entries {
			m.Counts[e.Status]++
			if a, ok := s.Cat.Get(id); ok && e.Status == store.Watching {
				m.Watching = append(m.Watching, watch{id, catalogTitle(a), a.Image, e.Episode, a.Episodes, e.Updated})
			}
		}
		sort.Slice(m.Watching, func(i, j int) bool { return m.Watching[i].At > m.Watching[j].At })
		m.Watching = head(m.Watching, 6)
		if t := s.temuanFor(u.ID); t != nil {
			for _, b := range t.Batches {
				for _, x := range b.Items {
					_, on := l.Entries[x.I]
					_, hid := l.Hidden[x.I]
					if !on && !hid {
						m.Temuan++
					}
				}
			}
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": out})
}

func catalogTitle(a *catalog.Anime) string {
	if a.English != "" && len(a.English) <= len(a.Title) {
		return a.English
	}
	return a.Title
}
