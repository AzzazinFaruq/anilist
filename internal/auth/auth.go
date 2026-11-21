// Package auth hashes PINs, signs session cookies and slows down guessing.
//
// The service only listens on the home LAN and Tailscale, and accounts belong
// to one family, so a short PIN is the right amount of friction. What keeps a
// 4–8 digit PIN safe is that every wrong guess costs time: see Limiter.
package auth

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	iterations = 210_000
	keyLen     = 32
)

// ErrBadPIN is returned for PINs that are too short or too long.
var ErrBadPIN = errors.New("PIN must be 4–32 characters")

// CheckPIN validates PIN length.
func CheckPIN(pin string) error {
	if len(pin) < 4 || len(pin) > 32 {
		return ErrBadPIN
	}
	return nil
}

// HashPIN returns "pbkdf2-sha256$<iterations>$<salt>$<hash>".
func HashPIN(pin string) (string, error) {
	if err := CheckPIN(pin); err != nil {
		return "", err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, pin, salt, iterations, keyLen)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", iterations, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// VerifyPIN compares pin with a hash from HashPIN in constant time.
func VerifyPIN(hash, pin string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1000 {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err1 := enc.DecodeString(parts[2])
	want, err2 := enc.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, pin, salt, iter, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

// ---------- session cookies ----------

// Sessions signs and verifies "<userID>.<version>.<expiry>.<mac>" tokens.
// Bumping a user's version (PIN change) invalidates every older token.
type Sessions struct {
	Key []byte
	TTL time.Duration
	Now func() time.Time
}

func (s Sessions) mac(payload string) string {
	m := hmac.New(sha256.New, s.Key)
	m.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// Issue creates a token for a user.
func (s Sessions) Issue(userID string, version int) (token string, expires time.Time) {
	expires = s.Now().Add(s.TTL)
	payload := fmt.Sprintf("%s.%d.%d", userID, version, expires.Unix())
	return payload + "." + s.mac(payload), expires
}

// Parse checks a token and returns its user id and version.
func (s Sessions) Parse(token string) (userID string, version int, ok bool) {
	i := strings.LastIndexByte(token, '.')
	if i < 0 {
		return "", 0, false
	}
	payload, sig := token[:i], token[i+1:]
	if !hmac.Equal([]byte(sig), []byte(s.mac(payload))) {
		return "", 0, false
	}
	parts := strings.Split(payload, ".")
	if len(parts) != 3 {
		return "", 0, false
	}
	v, err1 := strconv.Atoi(parts[1])
	exp, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil || s.Now().Unix() > exp {
		return "", 0, false
	}
	return parts[0], v, true
}

// ---------- guessing limiter ----------

// Limiter allows a few free attempts per key, then makes the caller wait
// 1s, 2s, 4s … (capped) after each further failure. Success clears the key.
type Limiter struct {
	Free int
	Max  time.Duration
	Now  func() time.Time

	mu sync.Mutex
	m  map[string]*strike
}

type strike struct {
	fails int
	until time.Time
}

// Wait returns how long key must wait before its next attempt (0 = go ahead).
func (l *Limiter) Wait(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if st, ok := l.m[key]; ok {
		if d := st.until.Sub(l.Now()); d > 0 {
			return d
		}
	}
	return 0
}

// Fail records a failed attempt.
func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.m == nil {
		l.m = map[string]*strike{}
	}
	st := l.m[key]
	if st == nil {
		st = &strike{}
		l.m[key] = st
	}
	st.fails++
	if over := st.fails - l.Free; over > 0 {
		d := time.Second << min(over-1, 20)
		st.until = l.Now().Add(min(d, l.Max))
	}
	if len(l.m) > 10_000 { // drop stale keys so the map cannot grow without bound
		for k, v := range l.m {
			if l.Now().Sub(v.until) > time.Hour {
				delete(l.m, k)
			}
		}
	}
}

// Reset clears a key after a successful attempt.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.m, key)
}
