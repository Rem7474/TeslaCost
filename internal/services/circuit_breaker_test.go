package services

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreakerNominalClosed(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		Cooldown:         50 * time.Millisecond,
	})

	if cb.State() != CircuitClosed {
		t.Fatalf("expected state CLOSED, got %s", cb.State())
	}
	if err := cb.CanExecute(); err != nil {
		t.Fatalf("expected CanExecute to succeed, got %v", err)
	}

	cb.RecordSuccess()
	if cb.ConsecutiveFailures() != 0 {
		t.Fatalf("expected 0 failures, got %d", cb.ConsecutiveFailures())
	}
}

func TestCircuitBreakerTripsToOpenAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		Cooldown:         100 * time.Millisecond,
	})

	testErr := errors.New("connection refused")

	// 1st failure
	cb.RecordFailure(testErr)
	if cb.State() != CircuitClosed {
		t.Fatalf("expected CLOSED after 1 failure, got %s", cb.State())
	}
	if cb.ConsecutiveFailures() != 1 {
		t.Fatalf("expected 1 failure count, got %d", cb.ConsecutiveFailures())
	}
	if err := cb.CanExecute(); err != nil {
		t.Fatalf("should still allow execution: %v", err)
	}

	// 2nd failure
	cb.RecordFailure(testErr)
	if cb.State() != CircuitClosed {
		t.Fatalf("expected CLOSED after 2 failures, got %s", cb.State())
	}

	// 3rd failure -> Trips to OPEN
	cb.RecordFailure(testErr)
	if cb.State() != CircuitOpen {
		t.Fatalf("expected OPEN after 3 failures, got %s", cb.State())
	}
	if cb.ConsecutiveFailures() != 3 {
		t.Fatalf("expected 3 failures, got %d", cb.ConsecutiveFailures())
	}

	// Execution should now be rejected
	err := cb.CanExecute()
	if err == nil || !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if cb.NextRetry().IsZero() {
		t.Fatal("expected NextRetry timestamp to be set")
	}
}

func TestCircuitBreakerHalfOpenAndRecovery(t *testing.T) {
	cooldown := 50 * time.Millisecond
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 2,
		Cooldown:         cooldown,
	})

	cb.RecordFailure(errors.New("fail 1"))
	cb.RecordFailure(errors.New("fail 2"))
	if cb.State() != CircuitOpen {
		t.Fatalf("expected OPEN, got %s", cb.State())
	}

	// Wait for cooldown to expire
	time.Sleep(cooldown + 10*time.Millisecond)

	// CanExecute should transition to HALF_OPEN and permit trial probe
	if err := cb.CanExecute(); err != nil {
		t.Fatalf("expected probe to be allowed in HALF_OPEN, got %v", err)
	}

	// Successful probe resets to CLOSED
	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Fatalf("expected CLOSED after probe success, got %s", cb.State())
	}
	if cb.ConsecutiveFailures() != 0 {
		t.Fatalf("expected 0 failures after recovery, got %d", cb.ConsecutiveFailures())
	}
}

func TestCircuitBreakerHalfOpenFailureReopens(t *testing.T) {
	cooldown := 40 * time.Millisecond
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 2,
		Cooldown:         cooldown,
	})

	cb.RecordFailure(errors.New("fail 1"))
	cb.RecordFailure(errors.New("fail 2"))
	if cb.State() != CircuitOpen {
		t.Fatalf("expected OPEN, got %s", cb.State())
	}

	time.Sleep(cooldown + 10*time.Millisecond)

	if err := cb.CanExecute(); err != nil {
		t.Fatalf("expected probe to be allowed, got %v", err)
	}

	// Probe fails -> immediate transition back to OPEN
	cb.RecordFailure(errors.New("probe failed"))
	if cb.State() != CircuitOpen {
		t.Fatalf("expected immediate transition back to OPEN, got %s", cb.State())
	}

	// Should be rejected again
	if err := cb.CanExecute(); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected rejection after failed probe, got %v", err)
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{FailureThreshold: 1, Cooldown: time.Hour})
	cb.RecordFailure(errors.New("err"))
	if cb.State() != CircuitOpen {
		t.Fatalf("expected OPEN, got %s", cb.State())
	}

	cb.Reset()
	if cb.State() != CircuitClosed {
		t.Fatalf("expected CLOSED after reset, got %s", cb.State())
	}
	if cb.ConsecutiveFailures() != 0 {
		t.Fatalf("expected 0 failures, got %d", cb.ConsecutiveFailures())
	}
	if err := cb.CanExecute(); err != nil {
		t.Fatalf("expected CanExecute to succeed after reset, got %v", err)
	}
}
