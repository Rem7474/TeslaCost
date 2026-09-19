package auth

import (
	"testing"
	"time"
)

func throttleAt(start time.Time) (*LoginThrottle, *time.Time) {
	now := start
	t := NewLoginThrottle()
	t.now = func() time.Time { return now }
	return t, &now
}

func TestLoginThrottleBlocksAfterRepeatedFailures(t *testing.T) {
	th, _ := throttleAt(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	for i := 0; i < loginFailureLimit-1; i++ {
		th.Fail("Me@Example.org")
	}
	if blocked, _ := th.Blocked("me@example.org"); blocked {
		t.Fatal("one failure short of the limit must still be allowed")
	}
	th.Fail("me@example.org")
	blocked, wait := th.Blocked("ME@example.org ")
	if !blocked || wait <= 0 || wait > loginFailureWindow {
		t.Fatalf("expected a block of at most %v, got %v for %v (the address is case and space insensitive)", loginFailureWindow, blocked, wait)
	}
	if other, _ := th.Blocked("someone-else@example.org"); other {
		t.Error("the limit is per account")
	}
}

func TestLoginThrottleReleasesAfterTheWindow(t *testing.T) {
	th, now := throttleAt(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	for i := 0; i < loginFailureLimit; i++ {
		th.Fail("me@example.org")
	}
	*now = now.Add(loginFailureWindow - time.Second)
	if blocked, wait := th.Blocked("me@example.org"); !blocked || wait > 2*time.Second {
		t.Fatalf("still blocked for about a second, got %v %v", blocked, wait)
	}
	*now = now.Add(2 * time.Second)
	if blocked, _ := th.Blocked("me@example.org"); blocked {
		t.Error("the window has passed")
	}
}

func TestLoginThrottleSlidesInsteadOfResetting(t *testing.T) {
	th, now := throttleAt(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	// Ten failures spread over ten minutes: when the first expires, one attempt is freed, not all of them.
	for i := 0; i < loginFailureLimit; i++ {
		th.Fail("me@example.org")
		*now = now.Add(time.Minute)
	}
	*now = now.Add(loginFailureWindow - loginFailureLimit*time.Minute + 30*time.Second) // first failure just expired
	if blocked, _ := th.Blocked("me@example.org"); blocked {
		t.Fatal("one attempt must be free once the oldest failure leaves the window")
	}
	th.Fail("me@example.org")
	if blocked, _ := th.Blocked("me@example.org"); !blocked {
		t.Error("using it back-to-back blocks again")
	}
}

func TestLoginThrottleResetOnSuccess(t *testing.T) {
	th, _ := throttleAt(time.Now())
	for i := 0; i < loginFailureLimit; i++ {
		th.Fail("me@example.org")
	}
	th.Reset("me@example.org")
	if blocked, _ := th.Blocked("me@example.org"); blocked {
		t.Error("a successful sign-in clears the failures")
	}
}

func TestLoginThrottlePurgesExpiredAccounts(t *testing.T) {
	th, now := throttleAt(time.Now())
	for i := 0; i <= throttlePurgeThreshold; i++ {
		th.Fail("user" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)) + string(rune('a'+(i/676)%26)) + "@example.org")
	}
	*now = now.Add(loginFailureWindow + time.Minute)
	th.Fail("fresh@example.org")
	if len(th.failures) != 1 {
		t.Errorf("expired accounts must not accumulate, %d entries left", len(th.failures))
	}
}

func TestPasswordLengthBounds(t *testing.T) {
	if _, err := HashPassword("1234567"); err != ErrPasswordTooShort {
		t.Errorf("short: %v", err)
	}
	long := make([]byte, MaxPasswordBytes+1)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := HashPassword(string(long)); err != ErrPasswordTooLong {
		t.Errorf("bcrypt would silently ignore what follows byte 72: %v", err)
	}
	if _, err := HashPassword(string(long[:MaxPasswordBytes])); err != nil {
		t.Errorf("exactly 72 bytes is fine: %v", err)
	}
}
