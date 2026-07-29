package auth

import (
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	s := NewSessions(time.Hour)

	token, err := s.Create("admin")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}

	got, ok := s.Lookup(token)
	if !ok {
		t.Fatal("token not found immediately after creation")
	}
	if got != "admin" {
		t.Errorf("username = %q, want admin", got)
	}
}

func TestSessionTokensAreUnique(t *testing.T) {
	s := NewSessions(time.Hour)

	seen := map[string]bool{}
	for range 100 {
		token, err := s.Create("admin")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if seen[token] {
			t.Fatal("Create returned a duplicate token")
		}
		seen[token] = true
	}
}

func TestSessionLookupRejectsUnknownTokens(t *testing.T) {
	s := NewSessions(time.Hour)
	if _, err := s.Create("admin"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, token := range []string{"", "nonsense", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"} {
		if _, ok := s.Lookup(token); ok {
			t.Errorf("Lookup(%q) succeeded", token)
		}
	}
}

// Logging out must actually invalidate the session server side, not merely ask
// the browser to drop the cookie.
func TestSessionDestroy(t *testing.T) {
	s := NewSessions(time.Hour)
	token, _ := s.Create("admin")

	s.Destroy(token)

	if _, ok := s.Lookup(token); ok {
		t.Error("session still valid after Destroy")
	}
}

// Changing a password must not leave older sessions usable.
func TestSessionDestroyUser(t *testing.T) {
	s := NewSessions(time.Hour)

	a1, _ := s.Create("admin")
	a2, _ := s.Create("admin")
	other, _ := s.Create("operator")

	s.DestroyUser("admin")

	if _, ok := s.Lookup(a1); ok {
		t.Error("first admin session survived DestroyUser")
	}
	if _, ok := s.Lookup(a2); ok {
		t.Error("second admin session survived DestroyUser")
	}
	if _, ok := s.Lookup(other); !ok {
		t.Error("DestroyUser removed another user's session")
	}
}

func TestSessionExpiry(t *testing.T) {
	s := NewSessions(time.Hour)
	now := time.Now()
	s.now = func() time.Time { return now }

	token, _ := s.Create("admin")

	now = now.Add(59 * time.Minute)
	if _, ok := s.Lookup(token); !ok {
		t.Fatal("session expired early")
	}

	// That lookup slid the expiry forward, so an active session keeps working.
	now = now.Add(59 * time.Minute)
	if _, ok := s.Lookup(token); !ok {
		t.Fatal("an active session was expired despite sliding expiry")
	}

	// Idle past the window and it goes.
	now = now.Add(2 * time.Hour)
	if _, ok := s.Lookup(token); ok {
		t.Error("an idle session outlived its ttl")
	}
}

func TestSessionSweepsExpired(t *testing.T) {
	s := NewSessions(time.Hour)
	now := time.Now()
	s.now = func() time.Time { return now }

	for range 5 {
		if _, err := s.Create("admin"); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	if got := s.Len(); got != 5 {
		t.Fatalf("Len = %d, want 5", got)
	}

	now = now.Add(2 * time.Hour)
	if got := s.Len(); got != 0 {
		t.Errorf("Len = %d after expiry, want 0 — expired sessions are not being reaped", got)
	}
}

func TestThrottleBlocksAfterRepeatedFailures(t *testing.T) {
	tr := NewThrottle(3, time.Minute)

	for i := range 3 {
		if !tr.Allowed("admin|10.0.0.1") {
			t.Fatalf("blocked after only %d failures", i)
		}
		tr.Fail("admin|10.0.0.1")
	}

	if tr.Allowed("admin|10.0.0.1") {
		t.Error("still allowed after reaching the failure limit")
	}
}

// One attacker guessing at a username must not lock out the real operator
// working from somewhere else.
func TestThrottleIsPerKey(t *testing.T) {
	tr := NewThrottle(3, time.Minute)

	for range 5 {
		tr.Fail("admin|10.0.0.99")
	}

	if tr.Allowed("admin|10.0.0.99") {
		t.Error("the attacking key was not blocked")
	}
	if !tr.Allowed("admin|10.0.0.1") {
		t.Error("a different source address was blocked by someone else's failures")
	}
}

func TestThrottleClearsOnSuccess(t *testing.T) {
	tr := NewThrottle(3, time.Minute)

	tr.Fail("admin|10.0.0.1")
	tr.Fail("admin|10.0.0.1")
	tr.Succeed("admin|10.0.0.1")

	// The counter restarted, so two more failures must not block.
	tr.Fail("admin|10.0.0.1")
	tr.Fail("admin|10.0.0.1")

	if !tr.Allowed("admin|10.0.0.1") {
		t.Error("failures before a successful login still counted towards the limit")
	}
}

func TestThrottleBlockExpires(t *testing.T) {
	tr := NewThrottle(3, time.Minute)
	now := time.Now()
	tr.now = func() time.Time { return now }

	for range 3 {
		tr.Fail("admin|10.0.0.1")
	}
	if tr.Allowed("admin|10.0.0.1") {
		t.Fatal("not blocked after reaching the limit")
	}

	now = now.Add(2 * time.Minute)
	if !tr.Allowed("admin|10.0.0.1") {
		t.Error("still blocked after the window elapsed")
	}
}

// An operator who mistypes occasionally over a long period must never
// accumulate their way into a block.
func TestThrottleForgetsOldFailures(t *testing.T) {
	tr := NewThrottle(3, time.Minute)
	now := time.Now()
	tr.now = func() time.Time { return now }

	for range 10 {
		tr.Fail("admin|10.0.0.1")
		now = now.Add(2 * time.Minute)
		if !tr.Allowed("admin|10.0.0.1") {
			t.Fatal("blocked by failures spread wider than the window")
		}
	}
}
