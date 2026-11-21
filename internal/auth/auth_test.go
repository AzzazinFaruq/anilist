package auth

import (
	"testing"
	"time"
)

func TestPIN(t *testing.T) {
	h, err := HashPIN("4821")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPIN(h, "4821") || VerifyPIN(h, "4822") || VerifyPIN("garbage", "4821") {
		t.Fatal("PIN verification is wrong")
	}
	h2, _ := HashPIN("4821")
	if h == h2 {
		t.Fatal("hashes must be salted")
	}
	if _, err := HashPIN("12"); err != ErrBadPIN {
		t.Fatalf("short PIN should fail, got %v", err)
	}
}

func TestSessions(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	s := Sessions{Key: []byte("0123456789abcdef0123456789abcdef"), TTL: time.Hour, Now: func() time.Time { return now }}
	tok, _ := s.Issue("abc", 3)
	if id, v, ok := s.Parse(tok); !ok || id != "abc" || v != 3 {
		t.Fatalf("Parse = %q %d %v", id, v, ok)
	}
	if _, _, ok := s.Parse(tok[:len(tok)-2] + "xx"); ok {
		t.Fatal("tampered token accepted")
	}
	if _, _, ok := s.Parse("abd" + tok[3:]); ok {
		t.Fatal("token with a changed user id accepted")
	}
	other := s
	other.Key = []byte("ffffffffffffffffffffffffffffffff")
	if _, _, ok := other.Parse(tok); ok {
		t.Fatal("token accepted under another key")
	}
	now = now.Add(2 * time.Hour)
	if _, _, ok := s.Parse(tok); ok {
		t.Fatal("expired token accepted")
	}
}

func TestLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := &Limiter{Free: 2, Max: 5 * time.Second, Now: func() time.Time { return now }}
	l.Fail("k")
	l.Fail("k")
	if l.Wait("k") != 0 {
		t.Fatal("free attempts should not wait")
	}
	l.Fail("k")
	if l.Wait("k") != time.Second {
		t.Fatalf("third failure should wait 1s, got %v", l.Wait("k"))
	}
	for i := 0; i < 10; i++ {
		l.Fail("k")
	}
	if l.Wait("k") != 5*time.Second {
		t.Fatalf("wait should cap at 5s, got %v", l.Wait("k"))
	}
	l.Reset("k")
	if l.Wait("k") != 0 {
		t.Fatal("reset should clear")
	}
}
