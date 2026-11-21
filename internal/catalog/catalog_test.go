package catalog

import (
	"fmt"
	"testing"
	"time"
)

func open(t *testing.T) *Catalog {
	t.Helper()
	c, err := Open("testdata", "testdata/id-synopsis.json")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func ids(as []*Anime) []int {
	out := make([]int, len(as))
	for i, a := range as {
		out[i] = a.ID
	}
	return out
}

func TestSearch(t *testing.T) {
	c := open(t)
	cases := []struct {
		q    string
		want []int
	}{
		{"frieren", []int{2}},
		{"FRIEREN beyond", []int{2}},
		{"rezero", []int{1, 5}},     // compact form of "Re:Zero"
		{"zero kara", []int{3, 1}},  // title prefix beats a match deeper in the title
		{"葬送", []int{2}},            // native title
		{"starting life", []int{1}}, // English title
		{"   ", nil},
		{"nothing like this", nil},
	}
	for _, tc := range cases {
		got := ids(c.Search(tc.q, 10))
		if len(got) != len(tc.want) {
			t.Errorf("Search(%q) = %v, want %v", tc.q, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("Search(%q) = %v, want %v", tc.q, got, tc.want)
				break
			}
		}
	}
}

func TestSeasonSortsByPopularity(t *testing.T) {
	got := ids(open(t).Season(2026, Fall))
	if len(got) != 2 || got[0] != 4 || got[1] != 3 {
		t.Fatalf("Season = %v, want [4 3]", got)
	}
}

func TestDetail(t *testing.T) {
	c := open(t)
	d, ok := c.Detail(2)
	if !ok || d.Synopsis == "" || d.SynopsisLang != "en" {
		t.Fatalf("detail 2 = %+v", d)
	}
	if len(d.Links) != 1 || d.Links[0].Site != "Crunchyroll" {
		t.Fatalf("links should keep only https URLs, got %+v", d.Links)
	}
	if len(d.Genres) != 1 || d.Genres[0] != "Drama" || d.Studios[0] != "Bones" {
		t.Fatalf("genres/studios not resolved: %+v", d.Anime)
	}
	d, _ = c.Detail(1)
	if d.SynopsisLang != "id" {
		t.Fatalf("Indonesian synopsis should win, got %q", d.SynopsisLang)
	}
}

func TestNextAiring(t *testing.T) {
	a, ok := open(t).NextAiring(3, time.Unix(1795000000, 0))
	if !ok || a.Episode != 5 {
		t.Fatalf("NextAiring = %+v %v", a, ok)
	}
	if a, _ := open(t).NextAiring(3, time.Unix(1795400000, 0)); a.Episode != 6 {
		t.Fatalf("after ep 5 aired, next should be 6, got %+v", a)
	}
	if _, ok := open(t).NextAiring(3, time.Unix(1796000000, 0)); ok {
		t.Fatal("past airing should be ignored")
	}
}

func TestCurrentSeason(t *testing.T) {
	for m, want := range map[time.Month]int{time.January: Winter, time.May: Spring, time.September: Summer, time.November: Fall} {
		if _, s := CurrentSeason(time.Date(2026, m, 1, 0, 0, 0, 0, time.UTC)); s != want {
			t.Errorf("month %v → %d, want %d", m, s, want)
		}
	}
}

func TestBrowse(t *testing.T) {
	c := open(t)
	cases := []struct {
		name string
		f    Filter
		want []int
	}{
		{"one season", Filter{Year: 2026, Season: Fall}, []int{4, 3}},
		{"whole year", Filter{Year: 2026, Season: -1, Formats: []string{"TV"}}, []int{3}},
		{"year range by worth", Filter{From: 2016, To: 2023, Season: -1, Sort: "worth"}, []int{2, 1, 5}},
		{"genre + tag", Filter{Season: -1, Genres: []string{"action"}, Tags: []string{"Isekai"}}, []int{1, 5, 3}},
		{"exclude genre", Filter{Season: -1, NoGenres: []string{"Drama"}}, []int{4, 3}},
		{"unknown genre matches nothing", Filter{Season: -1, Genres: []string{"Nope"}}, nil},
		{"no sequels", Filter{Season: -1, Tags: []string{"Time Loop"}, NoSequels: true}, []int{1}},
		{"country", Filter{Season: -1, Countries: []string{"KR"}}, []int{3}},
		{"length", Filter{Season: -1, Length: "mid"}, []int{1}},
		{"min score + newest", Filter{Season: -1, MinScore: 85, Sort: "new"}, []int{2, 5}},
		{"query", Filter{Season: -1, Query: "rezero"}, []int{1, 5}},
		{"exclude ids", Filter{Season: -1, Exclude: map[int]bool{1: true, 2: true, 3: true, 4: true}}, []int{5}},
	}
	for _, tc := range cases {
		got := ids(c.Browse(tc.f))
		if fmt.Sprint(got) != fmt.Sprint(tc.want) && !(len(got) == 0 && len(tc.want) == 0) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestSimilarSkipsFranchiseAndUsesGraph(t *testing.T) {
	got := ids(open(t).Similar(1, 5))
	if len(got) == 0 || got[0] != 2 {
		t.Fatalf("Similar(1) = %v, want Frieren (AniList recommendation) first", got)
	}
	for _, id := range got {
		if id == 5 {
			t.Fatal("a sequel of the same franchise should not be suggested")
		}
	}
}

func TestDetailExtras(t *testing.T) {
	c := open(t)
	d, _ := c.Detail(2)
	if d.Trailer != "https://www.youtube.com/watch?v=abcDEF_123" {
		t.Fatalf("trailer = %q", d.Trailer)
	}
	if a, _ := c.Get(3); a.Color != "" {
		t.Fatalf("unsafe colour should be dropped, got %q", a.Color)
	}
	if a, _ := c.Get(1); a.Color != "#e4a15d" || a.Tier != "S" {
		t.Fatalf("colour/tier = %q %q", a.Color, a.Tier)
	}
	if len(c.Meta().Tags) != 4 || c.Meta().Tags[0].Count != 3 {
		t.Fatalf("meta tags = %+v", c.Meta().Tags)
	}
}
