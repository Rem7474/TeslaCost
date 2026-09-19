package auth

import (
	"strings"
	"sync"
	"time"
)

// Failed sign-ins tolerated per account inside the window before further attempts are refused.
const (
	loginFailureLimit  = 10
	loginFailureWindow = 15 * time.Minute
	// Above this many tracked accounts, expired entries are purged on the next write.
	throttlePurgeThreshold = 1024
)

// LoginThrottle limits password guessing against one account, whatever the address it comes from: the per-address
// limit on the login route does not stop a botnet, and a guessed password is per account. It lives in memory, which
// is enough for a single instance; a restart clears it. Locking an account after repeated failures lets someone who
// knows the address hold that account's sign-in for the window: the trade-off favours the account owner's password.
type LoginThrottle struct {
	mu       sync.Mutex
	failures map[string][]time.Time
	now      func() time.Time
}

func NewLoginThrottle() *LoginThrottle {
	return &LoginThrottle{failures: map[string][]time.Time{}, now: time.Now}
}

func throttleKey(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// recent drops the failures older than the window and returns what is left. Callers hold the lock.
func (t *LoginThrottle) recent(key string) []time.Time {
	cutoff := t.now().Add(-loginFailureWindow)
	list := t.failures[key]
	kept := list[:0]
	for _, ts := range list {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) == 0 {
		delete(t.failures, key)
		return nil
	}
	t.failures[key] = kept
	return kept
}

// Blocked reports whether the account has used up its failures, and for how long attempts are refused.
func (t *LoginThrottle) Blocked(email string) (bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	failures := t.recent(throttleKey(email))
	if len(failures) < loginFailureLimit {
		return false, 0
	}
	// The oldest failure leaving the window frees one attempt
	return true, failures[len(failures)-loginFailureLimit].Add(loginFailureWindow).Sub(t.now())
}

// Fail records a failed sign-in.
func (t *LoginThrottle) Fail(email string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.failures) > throttlePurgeThreshold {
		for key := range t.failures {
			t.recent(key)
		}
	}
	key := throttleKey(email)
	t.failures[key] = append(t.recent(key), t.now())
}

// Reset forgets the failures of an account after a successful sign-in.
func (t *LoginThrottle) Reset(email string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.failures, throttleKey(email))
}
