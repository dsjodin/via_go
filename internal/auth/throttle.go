package auth

import (
	"sync"
	"time"
)

// Throttle limits repeated failed authentication attempts.
//
// go-via seeds a well known default account and hands out ESXi root passwords,
// so an unthrottled login endpoint is worth guessing against. Attempts are
// counted per key — username and source address together — so one attacker
// cannot lock out a legitimate operator by guessing at their username from
// somewhere else.
type Throttle struct {
	mu sync.Mutex
	m  map[string]*attempts

	// max failures before the key is blocked.
	max int
	// window is how long failures are remembered, and how long a block lasts.
	window time.Duration

	now func() time.Time
}

type attempts struct {
	count   int
	blocked time.Time
	last    time.Time
}

func NewThrottle(max int, window time.Duration) *Throttle {
	if max <= 0 {
		max = 10
	}
	if window <= 0 {
		window = 15 * time.Minute
	}

	return &Throttle{
		m:      make(map[string]*attempts),
		max:    max,
		window: window,
		now:    time.Now,
	}
}

// Allowed reports whether an attempt for key may proceed.
func (t *Throttle) Allowed(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	a, ok := t.m[key]
	if !ok {
		return true
	}

	now := t.now()

	if !a.blocked.IsZero() {
		if now.Before(a.blocked) {
			return false
		}
		// The block expired; start the key over.
		delete(t.m, key)
		return true
	}

	// Failures older than the window are forgotten, so an operator who
	// mistypes twice a week is never blocked.
	if now.Sub(a.last) > t.window {
		delete(t.m, key)
	}

	return true
}

// Fail records a failed attempt, blocking the key once max is reached.
func (t *Throttle) Fail(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()

	a, ok := t.m[key]
	if !ok || now.Sub(a.last) > t.window {
		a = &attempts{}
		t.m[key] = a
	}

	a.count++
	a.last = now

	if a.count >= t.max {
		a.blocked = now.Add(t.window)
	}
}

// Succeed clears the record for a key after a successful authentication.
func (t *Throttle) Succeed(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.m, key)
}
