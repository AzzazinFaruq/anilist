package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func TestUsers(t *testing.T) {
	s, dir := newStore(t)
	a, err := s.AddUser("  Rina  ", "h", true)
	if err != nil || a.Name != "Rina" {
		t.Fatalf("AddUser = %+v, %v", a, err)
	}
	if _, err := s.AddUser("rina", "h", false); err != ErrNameTaken {
		t.Fatalf("duplicate name (other case) should fail, got %v", err)
	}
	if _, err := s.AddUser("", "h", false); err != ErrBadName {
		t.Fatalf("empty name should fail, got %v", err)
	}
	b, _ := s.AddUser("Bima", "h", false)
	if _, err := s.SetAdmin(a.ID, false); err != ErrLastAdmin {
		t.Fatalf("removing the last admin should fail, got %v", err)
	}
	if err := s.DeleteUser(a.ID); err != ErrLastAdmin {
		t.Fatalf("deleting the last admin should fail, got %v", err)
	}
	if _, err := s.Rename(b.ID, "rina"); err != ErrNameTaken {
		t.Fatalf("rename onto an existing name should fail, got %v", err)
	}
	if u, err := s.Rename(b.ID, "Bimo"); err != nil || u.Name != "Bimo" {
		t.Fatalf("Rename = %+v, %v", u, err)
	}
	u, _ := s.SetHash(b.ID, "h2")
	if u.Version != 2 {
		t.Fatalf("PIN change should bump version, got %d", u.Version)
	}
	// Survives a reopen.
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := s2.UserByName("BIMO"); !ok || got.Hash != "h2" {
		t.Fatalf("reopened store lost data: %+v", got)
	}
}

func TestUpdateRules(t *testing.T) {
	s, _ := newStore(t)
	u, _ := s.AddUser("Rina", "h", true)
	day := "2026-11-21"

	e, err := s.Update(u.ID, 10, Change{}, 12, day)
	if err != nil || e.Status != Plan {
		t.Fatalf("new entry should default to plan: %+v %v", e, err)
	}
	e, _ = s.Update(u.ID, 10, Change{Episode: ptr(3)}, 12, day)
	if e.Status != Watching || e.Started == 0 {
		t.Fatalf("progress should start watching: %+v", e)
	}
	e, _ = s.Update(u.ID, 10, Change{Episode: ptr(99)}, 12, day)
	if e.Episode != 12 || e.Status != Done || e.Finished == 0 {
		t.Fatalf("last episode should cap and finish: %+v", e)
	}
	l, _ := s.Snapshot(u.ID)
	if l.Log[day] != 12 {
		t.Fatalf("log should count 12 episodes, got %d", l.Log[day])
	}
	// A mis-tapped episode comes off today's count.
	s.Update(u.ID, 13, Change{Episode: ptr(1)}, 0, day)
	s.Update(u.ID, 13, Change{Episode: ptr(0)}, 0, day)
	if l, _ = s.Snapshot(u.ID); l.Log[day] != 12 {
		t.Fatalf("+1 then -1 should leave the log at 12, got %d", l.Log[day])
	}
	s.Remove(u.ID, 13)
	// Marking done fills the episode count but is not logged as viewing.
	s.Update(u.ID, 11, Change{Status: ptr(Done)}, 24, day)
	l, _ = s.Snapshot(u.ID)
	if l.Entries[11].Episode != 24 || l.Log[day] != 12 {
		t.Fatalf("done should fill episodes without logging: %+v log=%d", l.Entries[11], l.Log[day])
	}
	// Unknown totals (airing shows) are not capped.
	e, _ = s.Update(u.ID, 12, Change{Episode: ptr(40)}, 0, day)
	if e.Episode != 40 || e.Status != Watching {
		t.Fatalf("airing show: %+v", e)
	}
	if _, err := s.Update(u.ID, 12, Change{Status: ptr("maybe")}, 0, day); err == nil {
		t.Fatal("unknown status should fail")
	}
	if _, err := s.Update(u.ID, 12, Change{Rating: ptr(11)}, 0, day); err == nil {
		t.Fatal("rating 11 should fail")
	}
	if err := s.Remove(u.ID, 12); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(u.ID, 12); err != ErrNotFound {
		t.Fatalf("second remove should be not found, got %v", err)
	}
}

func TestListsAreSeparate(t *testing.T) {
	s, _ := newStore(t)
	a, _ := s.AddUser("A", "h", true)
	b, _ := s.AddUser("B", "h", false)
	s.Update(a.ID, 1, Change{Status: ptr(Watching)}, 0, "")
	l, _ := s.Snapshot(b.ID)
	if len(l.Entries) != 0 {
		t.Fatalf("B sees A's list: %+v", l.Entries)
	}
}

func TestImportKeepsExisting(t *testing.T) {
	s, _ := newStore(t)
	u, _ := s.AddUser("A", "h", true)
	s.Update(u.ID, 1, Change{Status: ptr(Done)}, 0, "")
	n, err := s.Import(u.ID, map[int]Entry{1: {Status: Plan}, 2: {Status: Plan}, 3: {Status: "nope"}})
	if err != nil || n != 1 {
		t.Fatalf("Import = %d, %v", n, err)
	}
	if e, _, _ := s.Entry(u.ID, 1); e.Status != Done {
		t.Fatal("import overwrote an existing entry")
	}
}

func TestDailyBackup(t *testing.T) {
	s, dir := newStore(t)
	s.now = func() time.Time { return time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC) }
	s.AddUser("A", "h", true)
	s.AddUser("B", "h", false) // second write of the day backs up the first version
	b, err := os.ReadFile(filepath.Join(dir, "backup", "20261121", "users.json"))
	if err != nil || len(b) == 0 {
		t.Fatalf("expected a backup: %v", err)
	}
	for d := 1; d <= 20; d++ {
		os.MkdirAll(filepath.Join(dir, "backup", "202610"+twoDigits(d)), 0o750)
	}
	s.pruneBackups(14)
	entries, _ := os.ReadDir(filepath.Join(dir, "backup"))
	if len(entries) != 14 {
		t.Fatalf("kept %d backup days, want 14", len(entries))
	}
}

func twoDigits(n int) string { return string(rune('0'+n/10)) + string(rune('0'+n%10)) }

func TestSeesChangesFromAnotherProcess(t *testing.T) {
	server, dir := newStore(t)
	server.Users() // warm the cache
	cli, _ := Open(dir)
	u, _ := cli.AddUser("Rina", "h", true)
	cli.Import(u.ID, map[int]Entry{7: {Status: Plan}})
	if _, ok := server.UserByName("rina"); !ok {
		t.Fatal("server did not pick up a user added by the CLI")
	}
	if l, _ := server.Snapshot(u.ID); len(l.Entries) != 1 {
		t.Fatal("server did not pick up an imported list")
	}
}
