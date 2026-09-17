package services

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the current operating state of the circuit breaker.
type CircuitState string

const (
	CircuitClosed   CircuitState = "CLOSED"
	CircuitOpen     CircuitState = "OPEN"
	CircuitHalfOpen CircuitState = "HALF_OPEN"
)

const (
	defaultFailureThreshold = 3
	defaultCooldown         = 10 * time.Minute
)

// ErrCircuitOpen is returned when execution is rejected because the circuit is open.
var ErrCircuitOpen = errors.New("circuit breaker is OPEN: upstream service temporarily marked unreachable")

// CircuitBreakerConfig configures the failure threshold and recovery cooldown.
type CircuitBreakerConfig struct {
	FailureThreshold int
	Cooldown         time.Duration
}

// CircuitBreaker prevents hammering an unreachable service (e.g. TeslaMate container down).
// It transitions between CLOSED (nominal), OPEN (short-circuited after N failures),
// and HALF_OPEN (probing recovery after cooldown).
type CircuitBreaker struct {
	mu               sync.RWMutex
	state            CircuitState
	consecutiveFails int
	failureThreshold int
	cooldown         time.Duration
	lastFailureTime  time.Time
	lastStateChange  time.Time
}

// NewCircuitBreaker creates a circuit breaker with optional custom configuration.
func NewCircuitBreaker(cfg ...CircuitBreakerConfig) *CircuitBreaker {
	threshold := defaultFailureThreshold
	cooldown := defaultCooldown

	if len(cfg) > 0 {
		if cfg[0].FailureThreshold > 0 {
			threshold = cfg[0].FailureThreshold
		}
		if cfg[0].Cooldown > 0 {
			cooldown = cfg[0].Cooldown
		}
	}

	return &CircuitBreaker{
		state:            CircuitClosed,
		failureThreshold: threshold,
		cooldown:         cooldown,
		lastStateChange:  time.Now().UTC(),
	}
}

// CanExecute checks if an operation is allowed to proceed.
// Returns nil if allowed, or ErrCircuitOpen if rejected.
func (cb *CircuitBreaker) CanExecute() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now().UTC()

	switch cb.state {
	case CircuitClosed:
		return nil

	case CircuitOpen:
		if now.Sub(cb.lastFailureTime) >= cb.cooldown {
			cb.state = CircuitHalfOpen
			cb.lastStateChange = now
			return nil
		}
		retryAt := cb.lastFailureTime.Add(cb.cooldown)
		return fmt.Errorf("%w (retry allowed after %s)", ErrCircuitOpen, retryAt.Format(time.RFC3339))

	case CircuitHalfOpen:
		// Allow probe execution
		return nil

	default:
		return nil
	}
}

// RecordSuccess registers a successful operation, resetting failure count and closing the circuit.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFails = 0
	cb.state = CircuitClosed
	cb.lastStateChange = time.Now().UTC()
}

// RecordFailure registers a failed operation.
// If the failure threshold is reached (or if already in HALF_OPEN), the circuit trips to OPEN.
func (cb *CircuitBreaker) RecordFailure(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now().UTC()
	cb.lastFailureTime = now
	cb.consecutiveFails++

	if cb.state == CircuitHalfOpen || cb.consecutiveFails >= cb.failureThreshold {
		cb.state = CircuitOpen
		cb.lastStateChange = now
	}
}

// State returns the current circuit state, checking if cooldown has elapsed.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == CircuitOpen && time.Since(cb.lastFailureTime) >= cb.cooldown {
		return CircuitHalfOpen
	}
	return cb.state
}

// ConsecutiveFailures returns the current number of consecutive failures.
func (cb *CircuitBreaker) ConsecutiveFailures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.consecutiveFails
}

// NextRetry returns the timestamp when a retry attempt will be permitted if OPEN.
func (cb *CircuitBreaker) NextRetry() time.Time {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == CircuitOpen {
		return cb.lastFailureTime.Add(cb.cooldown)
	}
	return time.Time{}
}

// Reset forcibly resets the breaker to CLOSED and clears failure counters.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.consecutiveFails = 0
	cb.lastStateChange = time.Now().UTC()
}
