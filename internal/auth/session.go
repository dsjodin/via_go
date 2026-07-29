// Package auth holds session handling and login throttling.
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// CookieName is the session cookie go-via sets for browser clients.
const CookieName = "go-via-session"

// DefaultTTL is how long a session lasts without being used.
const DefaultTTL = 12 * time.Hour

type session struct {
	username string
	expires  time.Time
}

// Sessions is an in-memory session store.
//
// Sessions are deliberately server-side rather than signed cookies carrying
// their own claims. It means logout actually invalidates a session instead of
// merely asking the client to forget it, and there is no cookie signing key to
// manage or rotate. The cost is that sessions do not survive a restart, which
// for an appliance means logging in again.
type Sessions struct {
	mu  sync.Mutex
	m   map[string]session
	ttl time.Duration

	// now is overridable so expiry can be tested without sleeping.
	now func() time.Time
}

func NewSessions(ttl time.Duration) *Sessions {
	if ttl <= 0 {
		ttl = DefaultTTL
	}

	return &Sessions{
		m:   make(map[string]session),
		ttl: ttl,
		now: time.Now,
	}
}

// Create issues a session token for username.
func (s *Sessions) Create(username string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.m[token] = session{username: username, expires: s.now().Add(s.ttl)}
	s.sweepLocked()

	return token, nil
}

// Lookup returns the username a token belongs to, extending its lifetime.
func (s *Sessions) Lookup(token string) (string, bool) {
	if token == "" {
		return "", false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.m[token]
	if !ok {
		return "", false
	}
	if s.now().After(v.expires) {
		delete(s.m, token)
		return "", false
	}

	// Sliding expiry: an active session should not be logged out mid-use.
	v.expires = s.now().Add(s.ttl)
	s.m[token] = v

	return v.username, true
}

// Destroy invalidates a single session.
func (s *Sessions) Destroy(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, token)
}

// DestroyUser invalidates every session belonging to a user. Changing a
// password must not leave older sessions usable.
func (s *Sessions) DestroyUser(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for token, v := range s.m {
		if v.username == username {
			delete(s.m, token)
		}
	}
}

// Len reports the number of live sessions.
func (s *Sessions) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sweepLocked()
	return len(s.m)
}

func (s *Sessions) sweepLocked() {
	now := s.now()
	for token, v := range s.m {
		if now.After(v.expires) {
			delete(s.m, token)
		}
	}
}
