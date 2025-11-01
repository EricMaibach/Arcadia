package audio

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"arcadia/modules/documents/interfaces"
)

// TestCircuitBreakerInitialState verifies the circuit breaker starts in closed state
func TestCircuitBreakerInitialState(t *testing.T) {
	config := DefaultAudioConfig()
	cb := NewCircuitBreaker(&config)

	if cb.GetState() != CircuitBreakerClosed {
		t.Errorf("Expected initial state to be Closed, got %s", cb.GetState())
	}

	if cb.GetFailureCount() != 0 {
		t.Errorf("Expected initial failure count to be 0, got %d", cb.GetFailureCount())
	}
}

// TestCircuitBreakerClosedToOpen tests the transition from Closed to Open
func TestCircuitBreakerClosedToOpen(t *testing.T) {
	config := DefaultAudioConfig()
	config.CircuitBreakerFailureThreshold = 3
	cb := NewCircuitBreaker(&config)
	ctx := context.Background()

	testErr := errors.New("test failure")

	// Execute failures up to threshold - 1
	for i := 0; i < config.CircuitBreakerFailureThreshold-1; i++ {
		err := cb.Call(ctx, func() error {
			return testErr
		})
		if err == nil {
			t.Fatalf("Expected error on attempt %d, got nil", i+1)
		}
		if cb.GetState() != CircuitBreakerClosed {
			t.Errorf("Expected state to remain Closed after %d failures, got %s", i+1, cb.GetState())
		}
	}

	// This failure should open the circuit
	err := cb.Call(ctx, func() error {
		return testErr
	})
	if err == nil {
		t.Fatal("Expected error on threshold attempt, got nil")
	}

	if cb.GetState() != CircuitBreakerOpen {
		t.Errorf("Expected state to be Open after threshold failures, got %s", cb.GetState())
	}

	if cb.GetFailureCount() != config.CircuitBreakerFailureThreshold {
		t.Errorf("Expected failure count to be %d, got %d", config.CircuitBreakerFailureThreshold, cb.GetFailureCount())
	}
}

// TestCircuitBreakerOpenRejectsRequests tests that open circuit breaker rejects requests
func TestCircuitBreakerOpenRejectsRequests(t *testing.T) {
	config := DefaultAudioConfig()
	config.CircuitBreakerFailureThreshold = 2
	config.CircuitBreakerResetTimeout = 10 // 10 seconds
	cb := NewCircuitBreaker(&config)
	ctx := context.Background()

	// Force circuit to open
	testErr := errors.New("test failure")
	for i := 0; i < config.CircuitBreakerFailureThreshold; i++ {
		cb.Call(ctx, func() error { return testErr })
	}

	// Verify circuit is open
	if cb.GetState() != CircuitBreakerOpen {
		t.Fatalf("Expected circuit to be Open, got %s", cb.GetState())
	}

	// Try to call through open circuit
	callAttempted := false
	err := cb.Call(ctx, func() error {
		callAttempted = true
		return nil
	})

	if err != ErrCircuitBreakerOpen {
		t.Errorf("Expected ErrCircuitBreakerOpen, got %v", err)
	}

	if callAttempted {
		t.Error("Function should not have been called when circuit is open")
	}
}

// TestCircuitBreakerOpenToHalfOpen tests automatic transition from Open to Half-Open
func TestCircuitBreakerOpenToHalfOpen(t *testing.T) {
	config := DefaultAudioConfig()
	config.CircuitBreakerFailureThreshold = 2
	config.CircuitBreakerResetTimeout = 1 // 1 second for fast test
	cb := NewCircuitBreaker(&config)
	ctx := context.Background()

	// Force circuit to open
	testErr := errors.New("test failure")
	for i := 0; i < config.CircuitBreakerFailureThreshold; i++ {
		cb.Call(ctx, func() error { return testErr })
	}

	if cb.GetState() != CircuitBreakerOpen {
		t.Fatalf("Expected circuit to be Open, got %s", cb.GetState())
	}

	// Wait for reset timeout to elapse
	time.Sleep(time.Duration(config.CircuitBreakerResetTimeout+1) * time.Second)

	// Next call should transition to half-open and execute
	callExecuted := false
	err := cb.Call(ctx, func() error {
		callExecuted = true
		return nil
	})

	if err != nil {
		t.Errorf("Expected successful call in half-open state, got error: %v", err)
	}

	if !callExecuted {
		t.Error("Function should have been executed in half-open state")
	}

	// After successful call in half-open, should be closed
	if cb.GetState() != CircuitBreakerClosed {
		t.Errorf("Expected state to be Closed after successful half-open call, got %s", cb.GetState())
	}

	if cb.GetFailureCount() != 0 {
		t.Errorf("Expected failure count to be reset to 0, got %d", cb.GetFailureCount())
	}
}

// TestCircuitBreakerHalfOpenSuccess tests successful recovery in half-open state
func TestCircuitBreakerHalfOpenSuccess(t *testing.T) {
	config := DefaultAudioConfig()
	config.CircuitBreakerFailureThreshold = 2
	config.CircuitBreakerResetTimeout = 1
	cb := NewCircuitBreaker(&config)
	ctx := context.Background()

	// Force to open state
	testErr := errors.New("test failure")
	for i := 0; i < config.CircuitBreakerFailureThreshold; i++ {
		cb.Call(ctx, func() error { return testErr })
	}

	// Wait for reset timeout
	time.Sleep(time.Duration(config.CircuitBreakerResetTimeout+1) * time.Second)

	// Successful call should close the circuit
	err := cb.Call(ctx, func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	if cb.GetState() != CircuitBreakerClosed {
		t.Errorf("Expected Closed state after successful half-open recovery, got %s", cb.GetState())
	}

	if cb.GetFailureCount() != 0 {
		t.Errorf("Expected failure count reset to 0, got %d", cb.GetFailureCount())
	}
}

// TestCircuitBreakerHalfOpenFailure tests failure in half-open state
func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	config := DefaultAudioConfig()
	config.CircuitBreakerFailureThreshold = 2
	config.CircuitBreakerResetTimeout = 1
	cb := NewCircuitBreaker(&config)
	ctx := context.Background()

	// Force to open state
	testErr := errors.New("test failure")
	for i := 0; i < config.CircuitBreakerFailureThreshold; i++ {
		cb.Call(ctx, func() error { return testErr })
	}

	// Wait for reset timeout
	time.Sleep(time.Duration(config.CircuitBreakerResetTimeout+1) * time.Second)

	// Failed call in half-open should reopen the circuit
	err := cb.Call(ctx, func() error {
		return testErr
	})

	if err == nil {
		t.Error("Expected error from failed call, got nil")
	}

	if cb.GetState() != CircuitBreakerOpen {
		t.Errorf("Expected Open state after failed half-open attempt, got %s", cb.GetState())
	}

	// Failure count should be incremented
	if cb.GetFailureCount() <= config.CircuitBreakerFailureThreshold {
		t.Errorf("Expected failure count to be incremented, got %d", cb.GetFailureCount())
	}
}

// TestCircuitBreakerReset tests manual reset functionality
func TestCircuitBreakerReset(t *testing.T) {
	config := DefaultAudioConfig()
	config.CircuitBreakerFailureThreshold = 2
	cb := NewCircuitBreaker(&config)
	ctx := context.Background()

	// Force to open state
	testErr := errors.New("test failure")
	for i := 0; i < config.CircuitBreakerFailureThreshold; i++ {
		cb.Call(ctx, func() error { return testErr })
	}

	if cb.GetState() != CircuitBreakerOpen {
		t.Fatalf("Expected Open state, got %s", cb.GetState())
	}

	// Manual reset
	cb.Reset()

	if cb.GetState() != CircuitBreakerClosed {
		t.Errorf("Expected Closed state after reset, got %s", cb.GetState())
	}

	if cb.GetFailureCount() != 0 {
		t.Errorf("Expected failure count to be 0 after reset, got %d", cb.GetFailureCount())
	}

	// Should be able to call through circuit after reset
	callExecuted := false
	err := cb.Call(ctx, func() error {
		callExecuted = true
		return nil
	})

	if err != nil {
		t.Errorf("Expected successful call after reset, got error: %v", err)
	}

	if !callExecuted {
		t.Error("Function should have been executed after reset")
	}
}

// TestCircuitBreakerSuccessResetsFailureCount tests that success in closed state resets failures
func TestCircuitBreakerSuccessResetsFailureCount(t *testing.T) {
	config := DefaultAudioConfig()
	config.CircuitBreakerFailureThreshold = 5
	cb := NewCircuitBreaker(&config)
	ctx := context.Background()

	// Add some failures (but not enough to open)
	testErr := errors.New("test failure")
	for i := 0; i < 3; i++ {
		cb.Call(ctx, func() error { return testErr })
	}

	if cb.GetFailureCount() != 3 {
		t.Fatalf("Expected failure count 3, got %d", cb.GetFailureCount())
	}

	// A success should reset the failure count
	err := cb.Call(ctx, func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected successful call, got error: %v", err)
	}

	if cb.GetFailureCount() != 0 {
		t.Errorf("Expected failure count to be reset to 0 after success, got %d", cb.GetFailureCount())
	}

	if cb.GetState() != CircuitBreakerClosed {
		t.Errorf("Expected state to remain Closed, got %s", cb.GetState())
	}
}

// TestCircuitBreakerConcurrency tests thread safety under concurrent load
func TestCircuitBreakerConcurrency(t *testing.T) {
	config := DefaultAudioConfig()
	config.CircuitBreakerFailureThreshold = 10
	cb := NewCircuitBreaker(&config)
	ctx := context.Background()

	const numGoroutines = 50
	const callsPerGoroutine = 20

	var wg sync.WaitGroup
	var successCount, failureCount int32

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				err := cb.Call(ctx, func() error {
					// Alternate between success and failure
					if (id+j)%2 == 0 {
						return nil
					}
					return errors.New("test error")
				})

				if err == nil {
					atomic.AddInt32(&successCount, 1)
				} else {
					atomic.AddInt32(&failureCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	totalCalls := successCount + failureCount
	expectedCalls := int32(numGoroutines * callsPerGoroutine)

	// Note: Some calls might be rejected if circuit opens, so totalCalls might be less
	t.Logf("Total calls: %d (success: %d, failure: %d)", totalCalls, successCount, failureCount)
	t.Logf("Circuit breaker final state: %s, failures: %d", cb.GetState(), cb.GetFailureCount())

	// Basic sanity checks
	if totalCalls > expectedCalls {
		t.Errorf("Total calls (%d) exceeded expected (%d)", totalCalls, expectedCalls)
	}
}

// TestCircuitBreakerConcurrentStateReads tests concurrent state reads
func TestCircuitBreakerConcurrentStateReads(t *testing.T) {
	config := DefaultAudioConfig()
	cb := NewCircuitBreaker(&config)

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrently read state and failure count
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				state := cb.GetState()
				if !state.IsValid() {
					t.Errorf("Got invalid state: %s", state)
				}
				_ = cb.GetFailureCount()
			}
		}()
	}

	wg.Wait()
}

// TestCircuitBreakerTimingPrecision tests timing precision of state transitions
func TestCircuitBreakerTimingPrecision(t *testing.T) {
	config := DefaultAudioConfig()
	config.CircuitBreakerFailureThreshold = 1
	config.CircuitBreakerResetTimeout = 2 // 2 seconds
	cb := NewCircuitBreaker(&config)
	ctx := context.Background()

	// Open the circuit
	cb.Call(ctx, func() error { return errors.New("test error") })

	if cb.GetState() != CircuitBreakerOpen {
		t.Fatalf("Expected Open state, got %s", cb.GetState())
	}

	// Try immediately (should fail)
	err := cb.Call(ctx, func() error { return nil })
	if err != ErrCircuitBreakerOpen {
		t.Errorf("Expected ErrCircuitBreakerOpen immediately after opening, got %v", err)
	}

	// Wait 1.5 seconds (not enough)
	time.Sleep(1500 * time.Millisecond)
	err = cb.Call(ctx, func() error { return nil })
	if err != ErrCircuitBreakerOpen {
		t.Errorf("Expected ErrCircuitBreakerOpen before timeout, got %v", err)
	}

	// Wait another 1 second (total 2.5 seconds, should transition)
	time.Sleep(1000 * time.Millisecond)
	callExecuted := false
	err = cb.Call(ctx, func() error {
		callExecuted = true
		return nil
	})

	if err != nil {
		t.Errorf("Expected successful call after timeout, got error: %v", err)
	}

	if !callExecuted {
		t.Error("Function should have been executed after timeout")
	}
}

// TestCircuitBreakerWithLogger tests logging functionality
func TestCircuitBreakerWithLogger(t *testing.T) {
	config := DefaultAudioConfig()
	config.CircuitBreakerFailureThreshold = 2
	config.CircuitBreakerResetTimeout = 1

	// Create a mock logger
	mockLogger := &MockLogger{logs: make([]LogEntry, 0)}
	cb := NewCircuitBreaker(&config).WithLogger(mockLogger)
	ctx := context.Background()

	// Force to open state (should trigger error log)
	testErr := errors.New("test failure")
	for i := 0; i < config.CircuitBreakerFailureThreshold; i++ {
		cb.Call(ctx, func() error { return testErr })
	}

	// Check for error log when circuit opens
	hasOpenLog := false
	for _, log := range mockLogger.logs {
		if log.Level == "error" && log.Message == "Circuit breaker opened due to failures" {
			hasOpenLog = true
			break
		}
	}
	if !hasOpenLog {
		t.Error("Expected error log when circuit breaker opened")
	}

	// Wait and recover (should trigger info log)
	time.Sleep(time.Duration(config.CircuitBreakerResetTimeout+1) * time.Second)
	cb.Call(ctx, func() error { return nil })

	// Check for info log when circuit closes
	hasClosedLog := false
	for _, log := range mockLogger.logs {
		if log.Level == "info" && log.Message == "Circuit breaker closed after successful recovery" {
			hasClosedLog = true
			break
		}
	}
	if !hasClosedLog {
		t.Error("Expected info log when circuit breaker closed")
	}
}

// MockLogger for testing
type MockLogger struct {
	logs []LogEntry
	mu   sync.Mutex
}

type LogEntry struct {
	Level   string
	Message string
	Fields  []interface{}
}

func (ml *MockLogger) Debug(ctx context.Context, msg string, fields ...interface{}) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	ml.logs = append(ml.logs, LogEntry{Level: "debug", Message: msg, Fields: fields})
}

func (ml *MockLogger) Info(ctx context.Context, msg string, fields ...interface{}) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	ml.logs = append(ml.logs, LogEntry{Level: "info", Message: msg, Fields: fields})
}

func (ml *MockLogger) Warn(ctx context.Context, msg string, fields ...interface{}) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	ml.logs = append(ml.logs, LogEntry{Level: "warn", Message: msg, Fields: fields})
}

func (ml *MockLogger) Error(ctx context.Context, msg string, fields ...interface{}) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	ml.logs = append(ml.logs, LogEntry{Level: "error", Message: msg, Fields: fields})
}

func (ml *MockLogger) Fatal(ctx context.Context, msg string, fields ...interface{}) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	ml.logs = append(ml.logs, LogEntry{Level: "fatal", Message: msg, Fields: fields})
}

func (ml *MockLogger) WithFields(fields map[string]interface{}) interfaces.Logger {
	return ml
}

func (ml *MockLogger) WithContext(ctx context.Context) interfaces.Logger {
	return ml
}

func (ml *MockLogger) WithModule(module string) interfaces.Logger {
	return ml
}

func (ml *MockLogger) WithComponent(component string) interfaces.Logger {
	return ml
}

// BenchmarkCircuitBreakerClosed benchmarks performance in closed state
func BenchmarkCircuitBreakerClosed(b *testing.B) {
	config := DefaultAudioConfig()
	cb := NewCircuitBreaker(&config)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Call(ctx, func() error { return nil })
	}
}

// BenchmarkCircuitBreakerOpen benchmarks performance in open state
func BenchmarkCircuitBreakerOpen(b *testing.B) {
	config := DefaultAudioConfig()
	config.CircuitBreakerFailureThreshold = 1
	cb := NewCircuitBreaker(&config)
	ctx := context.Background()

	// Open the circuit
	cb.Call(ctx, func() error { return errors.New("test error") })

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Call(ctx, func() error { return nil })
	}
}
