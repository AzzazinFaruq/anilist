package catalog

import (
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
		{"rezero", []int{1}},        // compact form of "Re:Zero"
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
