package audio

import (
	"context"
	"sync"
	"time"

	"arcadia/modules/documents/interfaces"
)

// CircuitBreaker implements the circuit breaker pattern for the Whisper service
type CircuitBreaker struct {
	config              *AudioConfig
	logger              interfaces.Logger
	state               CircuitBreakerState
	failureCount        int
	lastFailureTime     time.Time
	lastStateChangeTime time.Time
	mutex               sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *AudioConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config:              config,
		state:               CircuitBreakerClosed,
		lastStateChangeTime: time.Now(),
	}
}

// WithLogger adds logging to the circuit breaker
func (cb *CircuitBreaker) WithLogger(logger interfaces.Logger) *CircuitBreaker {
	cb.logger = logger
	return cb
}

// Call executes a function through the circuit breaker
func (cb *CircuitBreaker) Call(ctx context.Context, fn func() error) error {
	cb.mutex.Lock()

	// Check if circuit breaker should transition from open to half-open
	if cb.state == CircuitBreakerOpen {
		if time.Since(cb.lastStateChangeTime) >= cb.config.GetCircuitBreakerResetTimeoutDuration() {
			cb.state = CircuitBreakerHalfOpen
			cb.lastStateChangeTime = time.Now()
			if cb.logger != nil {
				cb.logger.Info(ctx, "Circuit breaker transitioning to half-open", "previous_failures", cb.failureCount)
			}
		} else {
			cb.mutex.Unlock()
			return ErrCircuitBreakerOpen
		}
	}

	currentState := cb.state
	cb.mutex.Unlock()

	// Execute the function
	err := fn()

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err != nil {
		cb.recordFailure(ctx)
		return err
	}

	cb.recordSuccess(ctx, currentState)
	return nil
}

// recordFailure records a failure and potentially opens the circuit
func (cb *CircuitBreaker) recordFailure(ctx context.Context) {
	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.state == CircuitBreakerHalfOpen {
		// Failed in half-open state, go back to open
		cb.state = CircuitBreakerOpen
		cb.lastStateChangeTime = time.Now()
		if cb.logger != nil {
			cb.logger.Warn(ctx, "Circuit breaker reopened after failure in half-open state")
		}
	} else if cb.failureCount >= cb.config.CircuitBreakerFailureThreshold {
		// Exceeded threshold, open the circuit
		cb.state = CircuitBreakerOpen
		cb.lastStateChangeTime = time.Now()
		if cb.logger != nil {
			cb.logger.Error(ctx, "Circuit breaker opened due to failures",
				"failure_count", cb.failureCount,
				"threshold", cb.config.CircuitBreakerFailureThreshold)
		}
	}
}

// recordSuccess records a success and potentially closes the circuit
func (cb *CircuitBreaker) recordSuccess(ctx context.Context, previousState CircuitBreakerState) {
	if previousState == CircuitBreakerHalfOpen {
		// Success in half-open state, close the circuit
		cb.state = CircuitBreakerClosed
		cb.failureCount = 0
		cb.lastStateChangeTime = time.Now()
		if cb.logger != nil {
			cb.logger.Info(ctx, "Circuit breaker closed after successful recovery")
		}
	} else if cb.state == CircuitBreakerClosed {
		// Reset failure count on success
		cb.failureCount = 0
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetFailureCount returns the current failure count
func (cb *CircuitBreaker) GetFailureCount() int {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.failureCount
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.state = CircuitBreakerClosed
	cb.failureCount = 0
	cb.lastStateChangeTime = time.Now()
}
