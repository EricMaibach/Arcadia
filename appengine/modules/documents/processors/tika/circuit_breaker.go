package tika

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int32

const (
	// StateClosed allows requests to pass through
	StateClosed CircuitBreakerState = iota
	// StateOpen blocks all requests
	StateOpen
	// StateHalfOpen allows limited requests to test if service is recovered
	StateHalfOpen
)

// String returns the string representation of the circuit breaker state
func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker implements the circuit breaker pattern for fault tolerance
type CircuitBreaker struct {
	config           CircuitBreakerConfig
	state            int32 // atomic access to CircuitBreakerState
	failures         int32 // atomic counter for failures
	lastFailureTime  int64 // atomic timestamp of last failure
	halfOpenRequests int32 // atomic counter for half-open requests
	mutex            sync.RWMutex
	onStateChange    func(from, to CircuitBreakerState)
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  int32(StateClosed),
	}
}

// Execute runs the given function through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	// Check if we should allow the request
	if !cb.allowRequest() {
		return ErrCircuitBreakerOpen{}
	}

	// Execute the function
	err := fn(ctx)

	// Handle the result
	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

// allowRequest determines if a request should be allowed based on the current state
func (cb *CircuitBreaker) allowRequest() bool {
	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))

	switch state {
	case StateClosed:
		return true
	case StateOpen:
		return cb.shouldAttemptReset()
	case StateHalfOpen:
		return cb.allowHalfOpenRequest()
	default:
		return false
	}
}

// shouldAttemptReset checks if enough time has passed to attempt closing the circuit
func (cb *CircuitBreaker) shouldAttemptReset() bool {
	lastFailureTime := atomic.LoadInt64(&cb.lastFailureTime)
	if time.Since(time.Unix(0, lastFailureTime)) >= cb.config.ResetTimeout {
		if cb.setState(StateOpen, StateHalfOpen) {
			atomic.StoreInt32(&cb.halfOpenRequests, 0)
			return true
		}
	}
	return false
}

// allowHalfOpenRequest checks if we can allow another request in half-open state
func (cb *CircuitBreaker) allowHalfOpenRequest() bool {
	halfOpenRequests := atomic.LoadInt32(&cb.halfOpenRequests)
	if halfOpenRequests < int32(cb.config.HalfOpenMaxRequests) {
		atomic.AddInt32(&cb.halfOpenRequests, 1)
		return true
	}
	return false
}

// onSuccess handles a successful request
func (cb *CircuitBreaker) onSuccess() {
	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))

	switch state {
	case StateHalfOpen:
		// Reset failure counter and move to closed state
		atomic.StoreInt32(&cb.failures, 0)
		cb.setState(StateHalfOpen, StateClosed)
	case StateClosed:
		// Reset failure counter on success
		atomic.StoreInt32(&cb.failures, 0)
	}
}

// onFailure handles a failed request
func (cb *CircuitBreaker) onFailure() {
	failures := atomic.AddInt32(&cb.failures, 1)
	atomic.StoreInt64(&cb.lastFailureTime, time.Now().UnixNano())

	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))

	switch state {
	case StateClosed:
		if failures >= int32(cb.config.FailureThreshold) {
			cb.setState(StateClosed, StateOpen)
		}
	case StateHalfOpen:
		// Any failure in half-open state should open the circuit
		cb.setState(StateHalfOpen, StateOpen)
	}
}

// setState atomically changes the circuit breaker state
func (cb *CircuitBreaker) setState(from, to CircuitBreakerState) bool {
	if atomic.CompareAndSwapInt32(&cb.state, int32(from), int32(to)) {
		if cb.onStateChange != nil {
			cb.onStateChange(from, to)
		}
		return true
	}
	return false
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	return CircuitBreakerState(atomic.LoadInt32(&cb.state))
}

// GetFailures returns the current failure count
func (cb *CircuitBreaker) GetFailures() int32 {
	return atomic.LoadInt32(&cb.failures)
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	oldState := CircuitBreakerState(atomic.LoadInt32(&cb.state))
	atomic.StoreInt32(&cb.state, int32(StateClosed))
	atomic.StoreInt32(&cb.failures, 0)
	atomic.StoreInt32(&cb.halfOpenRequests, 0)
	atomic.StoreInt64(&cb.lastFailureTime, 0)

	if cb.onStateChange != nil && oldState != StateClosed {
		cb.onStateChange(oldState, StateClosed)
	}
}

// SetStateChangeHandler sets a callback function for state changes
func (cb *CircuitBreaker) SetStateChangeHandler(handler func(from, to CircuitBreakerState)) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.onStateChange = handler
}

// ErrCircuitBreakerOpen is returned when the circuit breaker is open
type ErrCircuitBreakerOpen struct{}

func (e ErrCircuitBreakerOpen) Error() string {
	return "circuit breaker is open"
}

// IsCircuitBreakerOpen checks if an error is due to circuit breaker being open
func IsCircuitBreakerOpen(err error) bool {
	_, ok := err.(ErrCircuitBreakerOpen)
	return ok
}

// CircuitBreakerStats provides statistics about the circuit breaker
type CircuitBreakerStats struct {
	State            CircuitBreakerState `json:"state"`
	Failures         int32               `json:"failures"`
	LastFailureTime  time.Time           `json:"last_failure_time"`
	HalfOpenRequests int32               `json:"half_open_requests"`
}

// GetStats returns current statistics of the circuit breaker
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	lastFailureTime := atomic.LoadInt64(&cb.lastFailureTime)
	var lastFailure time.Time
	if lastFailureTime > 0 {
		lastFailure = time.Unix(0, lastFailureTime)
	}

	return CircuitBreakerStats{
		State:            CircuitBreakerState(atomic.LoadInt32(&cb.state)),
		Failures:         atomic.LoadInt32(&cb.failures),
		LastFailureTime:  lastFailure,
		HalfOpenRequests: atomic.LoadInt32(&cb.halfOpenRequests),
	}
}