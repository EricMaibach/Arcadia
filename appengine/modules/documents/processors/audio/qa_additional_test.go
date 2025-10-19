package audio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestCircuitBreakerEdgeCases tests edge cases in circuit breaker behavior
func TestCircuitBreakerEdgeCases(t *testing.T) {
	t.Run("ZeroThreshold", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.CircuitBreakerFailureThreshold = 1
		cb := NewCircuitBreaker(&config)
		ctx := context.Background()

		// Single failure should open circuit
		err := cb.Call(ctx, func() error {
			return ErrWhisperUnavailable
		})

		if err == nil {
			t.Error("Expected error, got nil")
		}

		if cb.GetState() != CircuitBreakerOpen {
			t.Errorf("Expected Open state after single failure with threshold 1, got %s", cb.GetState())
		}
	})

	t.Run("MultipleResets", func(t *testing.T) {
		config := DefaultAudioConfig()
		cb := NewCircuitBreaker(&config)

		// Reset multiple times
		for i := 0; i < 5; i++ {
			cb.Reset()
			if cb.GetState() != CircuitBreakerClosed {
				t.Errorf("Reset %d: expected Closed state, got %s", i+1, cb.GetState())
			}
			if cb.GetFailureCount() != 0 {
				t.Errorf("Reset %d: expected 0 failures, got %d", i+1, cb.GetFailureCount())
			}
		}
	})

	t.Run("RapidStateTransitions", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.CircuitBreakerFailureThreshold = 2
		config.CircuitBreakerResetTimeout = 1
		cb := NewCircuitBreaker(&config)
		ctx := context.Background()

		// Open circuit
		for i := 0; i < 2; i++ {
			cb.Call(ctx, func() error { return ErrWhisperUnavailable })
		}

		if cb.GetState() != CircuitBreakerOpen {
			t.Fatal("Expected Open state")
		}

		// Wait and recover
		time.Sleep(1100 * time.Millisecond)
		cb.Call(ctx, func() error { return nil })

		if cb.GetState() != CircuitBreakerClosed {
			t.Errorf("Expected Closed state after recovery, got %s", cb.GetState())
		}

		// Immediately fail again
		for i := 0; i < 2; i++ {
			cb.Call(ctx, func() error { return ErrWhisperUnavailable })
		}

		if cb.GetState() != CircuitBreakerOpen {
			t.Errorf("Expected Open state after new failures, got %s", cb.GetState())
		}
	})
}

// TestCircuitBreakerStressTest performs stress testing on circuit breaker
func TestCircuitBreakerStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	config := DefaultAudioConfig()
	config.CircuitBreakerFailureThreshold = 50
	cb := NewCircuitBreaker(&config)
	ctx := context.Background()

	const numGoroutines = 100
	const callsPerGoroutine = 100

	var wg sync.WaitGroup
	var totalCalls, totalSuccesses, totalFailures int64

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				atomic.AddInt64(&totalCalls, 1)
				err := cb.Call(ctx, func() error {
					// Simulate varying success/failure
					if (id*callsPerGoroutine+j)%3 == 0 {
						return ErrWhisperUnavailable
					}
					return nil
				})

				if err == nil {
					atomic.AddInt64(&totalSuccesses, 1)
				} else {
					atomic.AddInt64(&totalFailures, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Stress test results: total=%d, success=%d, failure=%d, state=%s",
		totalCalls, totalSuccesses, totalFailures, cb.GetState())

	// Verify no panics occurred
	if totalCalls != totalSuccesses+totalFailures {
		t.Errorf("Call count mismatch: %d != %d + %d", totalCalls, totalSuccesses, totalFailures)
	}
}

// TestClientErrorHandling tests various error scenarios
func TestClientErrorHandling(t *testing.T) {
	t.Run("NilConfig", func(t *testing.T) {
		// Should not panic with nil config
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic with nil config")
			}
		}()
		_ = NewWhisperClient(nil)
	})

	t.Run("EmptyWhisperURL", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.WhisperURL = ""
		client := NewWhisperClient(&config)

		ctx := context.Background()
		result, err := client.Transcribe(ctx, "/tmp/test.mp3")

		if err == nil {
			t.Error("Expected error with empty Whisper URL")
		}
		if result != nil {
			t.Error("Expected nil result")
		}
	})

	t.Run("MalformedURL", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.WhisperURL = "://invalid-url"
		config.MaxRetries = 0
		client := NewWhisperClient(&config)

		tempDir := t.TempDir()
		audioPath := filepath.Join(tempDir, "test.mp3")
		os.WriteFile(audioPath, []byte("fake audio"), 0644)

		ctx := context.Background()
		result, err := client.Transcribe(ctx, audioPath)

		if err == nil {
			t.Error("Expected error with malformed URL")
		}
		if result != nil {
			t.Error("Expected nil result")
		}
	})
}

// TestClientMultipartUpload tests multipart form upload edge cases
func TestClientMultipartUpload(t *testing.T) {
	t.Run("LargeFile", func(t *testing.T) {
		fileReceived := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := r.ParseMultipartForm(100 << 20) // 100 MB
			if err != nil {
				t.Errorf("Failed to parse multipart form: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			file, header, err := r.FormFile("file")
			if err != nil {
				t.Errorf("Failed to get form file: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer file.Close()

			// Verify file size
			if header.Size < 1024*1024 {
				t.Error("Expected file size >= 1MB")
			}

			fileReceived = true

			response := map[string]interface{}{
				"text":     "Large file transcription",
				"language": "en",
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		tempDir := t.TempDir()
		audioPath := filepath.Join(tempDir, "large.mp3")
		// Create 1MB file
		data := make([]byte, 1024*1024)
		os.WriteFile(audioPath, data, 0644)

		config := DefaultAudioConfig()
		config.WhisperURL = server.URL
		config.MaxRetries = 0
		client := NewWhisperClient(&config)

		ctx := context.Background()
		result, err := client.Transcribe(ctx, audioPath)

		if err != nil {
			t.Errorf("Transcription failed: %v", err)
		}

		if !fileReceived {
			t.Error("Server did not receive file")
		}

		if result == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("SpecialCharactersInFilename", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.ParseMultipartForm(10 << 20)
			_, header, err := r.FormFile("file")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Verify filename was preserved
			if !strings.Contains(header.Filename, "test") {
				t.Errorf("Expected filename to contain 'test', got: %s", header.Filename)
			}

			response := map[string]interface{}{
				"text":     "Test",
				"language": "en",
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		tempDir := t.TempDir()
		audioPath := filepath.Join(tempDir, "test-file (1) [copy].mp3")
		os.WriteFile(audioPath, []byte("fake audio"), 0644)

		config := DefaultAudioConfig()
		config.WhisperURL = server.URL
		config.MaxRetries = 0
		client := NewWhisperClient(&config)

		ctx := context.Background()
		_, err := client.Transcribe(ctx, audioPath)

		if err != nil {
			t.Errorf("Transcription failed with special filename: %v", err)
		}
	})
}

// TestRetryBehaviorEdgeCases tests retry logic edge cases
func TestRetryBehaviorEdgeCases(t *testing.T) {
	t.Run("NoRetries", func(t *testing.T) {
		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attemptCount++
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		tempDir := t.TempDir()
		audioPath := filepath.Join(tempDir, "test.mp3")
		os.WriteFile(audioPath, []byte("fake audio"), 0644)

		config := DefaultAudioConfig()
		config.WhisperURL = server.URL
		config.MaxRetries = 0 // No retries
		client := NewWhisperClient(&config)

		ctx := context.Background()
		_, err := client.Transcribe(ctx, audioPath)

		if err == nil {
			t.Error("Expected error, got nil")
		}

		if attemptCount != 1 {
			t.Errorf("Expected exactly 1 attempt with MaxRetries=0, got %d", attemptCount)
		}
	})

	t.Run("MaxRetriesReached", func(t *testing.T) {
		attemptCount := int32(0)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attemptCount, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		tempDir := t.TempDir()
		audioPath := filepath.Join(tempDir, "test.mp3")
		os.WriteFile(audioPath, []byte("fake audio"), 0644)

		config := DefaultAudioConfig()
		config.WhisperURL = server.URL
		config.MaxRetries = 5
		config.RetryDelay = 0 // No delay for faster test
		config.CircuitBreakerFailureThreshold = 10 // Higher than max retries
		client := NewWhisperClient(&config)

		ctx := context.Background()
		_, err := client.Transcribe(ctx, audioPath)

		if err == nil {
			t.Error("Expected error after max retries, got nil")
		}

		expectedAttempts := config.MaxRetries + 1 // Initial attempt + retries
		if atomic.LoadInt32(&attemptCount) != int32(expectedAttempts) {
			t.Errorf("Expected %d attempts, got %d", expectedAttempts, attemptCount)
		}

		if !strings.Contains(err.Error(), "after 6 attempts") {
			t.Errorf("Error should mention attempt count, got: %v", err)
		}
	})
}

// TestCircuitBreakerWithRetries tests interaction between circuit breaker and retries
func TestCircuitBreakerWithRetries(t *testing.T) {
	attemptCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	audioPath := filepath.Join(tempDir, "test.mp3")
	os.WriteFile(audioPath, []byte("fake audio"), 0644)

	config := DefaultAudioConfig()
	config.WhisperURL = server.URL
	config.MaxRetries = 2
	config.RetryDelay = 0
	config.CircuitBreakerFailureThreshold = 3
	client := NewWhisperClient(&config)

	ctx := context.Background()

	// First call: will retry 2 times + initial = 3 attempts, opening circuit
	client.Transcribe(ctx, audioPath)

	if client.GetCircuitBreakerState() != CircuitBreakerOpen {
		t.Errorf("Expected circuit to be open after failures, got %s", client.GetCircuitBreakerState())
	}

	// Second call: should be rejected immediately by circuit breaker
	beforeCount := atomic.LoadInt32(&attemptCount)
	_, err := client.Transcribe(ctx, audioPath)

	if err == nil {
		t.Error("Expected error when circuit is open")
	}

	afterCount := atomic.LoadInt32(&attemptCount)
	if afterCount != beforeCount {
		t.Error("Circuit breaker should have prevented request from reaching server")
	}
}

// TestHealthCheckRobustness tests health check under various conditions
func TestHealthCheckRobustness(t *testing.T) {
	t.Run("SlowHealthCheck", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second) // Slower than typical response
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := DefaultAudioConfig()
		config.WhisperURL = server.URL
		client := NewWhisperClient(&config)

		ctx := context.Background()
		start := time.Now()
		err := client.HealthCheck(ctx)
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Health check failed: %v", err)
		}

		if duration < 2*time.Second {
			t.Error("Health check should have waited for slow response")
		}
	})

	t.Run("HealthCheckWithInvalidJSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("not json")) // Health check doesn't parse body
		}))
		defer server.Close()

		config := DefaultAudioConfig()
		config.WhisperURL = server.URL
		client := NewWhisperClient(&config)

		ctx := context.Background()
		err := client.HealthCheck(ctx)

		// Should still pass since only status code matters
		if err != nil {
			t.Errorf("Health check should succeed with any body when status is 200, got: %v", err)
		}
	})
}

// TestIsRetryableLogic tests the retry decision logic
func TestIsRetryableLogic(t *testing.T) {
	testCases := []struct {
		name       string
		err        error
		retryable  bool
	}{
		{"Nil error", nil, false},
		{"Whisper unavailable", ErrWhisperUnavailable, true},
		{"Whisper timeout", ErrWhisperTimeout, true},
		{"Whisper request failed", ErrWhisperRequestFailed, true},
		{"Circuit breaker open", ErrCircuitBreakerOpen, false},
		{"Transcription empty", ErrTranscriptionEmpty, false},
		{"Invalid response", ErrWhisperResponseInvalid, false},
		{"Unsupported format", ErrUnsupportedFormat, false},
		{"Wrapped retryable", &RetryableError{Err: ErrWhisperUnavailable}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsRetryable(tc.err)
			if result != tc.retryable {
				t.Errorf("Expected IsRetryable(%v) = %v, got %v", tc.err, tc.retryable, result)
			}
		})
	}
}

// TestMetricsAccuracy tests that metrics are accurately recorded
func TestMetricsAccuracy(t *testing.T) {
	successCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		successCount++
		response := map[string]interface{}{
			"text":     "Test transcription",
			"language": "en",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	audioPath := filepath.Join(tempDir, "test.mp3")
	os.WriteFile(audioPath, []byte("fake audio"), 0644)

	config := DefaultAudioConfig()
	config.WhisperURL = server.URL
	config.MaxRetries = 0
	mockMetrics := &MockMetrics{}
	client := NewWhisperClient(&config).WithMetrics(mockMetrics)

	ctx := context.Background()

	// Make 5 successful calls
	for i := 0; i < 5; i++ {
		_, err := client.Transcribe(ctx, audioPath)
		if err != nil {
			t.Fatalf("Transcription %d failed: %v", i+1, err)
		}
	}

	if mockMetrics.timerCount != 5 {
		t.Errorf("Expected 5 timer recordings, got %d", mockMetrics.timerCount)
	}

	if mockMetrics.successCount != 5 {
		t.Errorf("Expected 5 success counts, got %d", mockMetrics.successCount)
	}

	if mockMetrics.failedCount != 0 {
		t.Errorf("Expected 0 failed counts, got %d", mockMetrics.failedCount)
	}
}
