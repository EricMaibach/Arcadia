package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"arcadia/modules/documents/interfaces"
)

// TestNewWhisperClient tests client initialization
func TestNewWhisperClient(t *testing.T) {
	config := DefaultAudioConfig()
	client := NewWhisperClient(&config)

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.config != &config {
		t.Error("Config not properly set")
	}

	if client.httpClient == nil {
		t.Error("HTTP client not initialized")
	}

	if client.circuitBreaker == nil {
		t.Error("Circuit breaker not initialized")
	}

	// Test HTTP client timeout
	expectedTimeout := config.GetWhisperTimeoutDuration()
	if client.httpClient.Timeout != expectedTimeout {
		t.Errorf("Expected HTTP timeout %v, got %v", expectedTimeout, client.httpClient.Timeout)
	}
}

// TestWhisperClientWithLogger tests logger attachment
func TestWhisperClientWithLogger(t *testing.T) {
	config := DefaultAudioConfig()
	mockLogger := &MockLogger{logs: make([]LogEntry, 0)}
	client := NewWhisperClient(&config).WithLogger(mockLogger)

	if client.logger == nil {
		t.Error("Logger not set")
	}

	// Circuit breaker should also have logger
	if client.circuitBreaker.logger == nil {
		t.Error("Circuit breaker logger not set")
	}
}

// TestWhisperClientWithMetrics tests metrics attachment
func TestWhisperClientWithMetrics(t *testing.T) {
	config := DefaultAudioConfig()
	mockMetrics := &MockMetrics{}
	client := NewWhisperClient(&config).WithMetrics(mockMetrics)

	if client.metrics == nil {
		t.Error("Metrics not set")
	}
}

// TestTranscribeSuccess tests successful transcription
func TestTranscribeSuccess(t *testing.T) {
	// Create mock Whisper server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/transcribe") {
			t.Errorf("Expected /transcribe endpoint, got %s", r.URL.Path)
		}

		// Verify Content-Type
		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "multipart/form-data") {
			t.Errorf("Expected multipart/form-data content type, got %s", contentType)
		}

		// Parse multipart form
		err := r.ParseMultipartForm(10 << 20) // 10 MB
		if err != nil {
			t.Errorf("Failed to parse multipart form: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Verify file field exists
		_, _, err = r.FormFile("file")
		if err != nil {
			t.Errorf("File field not found: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Return successful response
		response := map[string]interface{}{
			"text":     "This is a test transcription",
			"language": "en",
			"duration": 10.5,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create test audio file
	tempDir := t.TempDir()
	audioPath := filepath.Join(tempDir, "test.mp3")
	err := os.WriteFile(audioPath, []byte("fake audio data"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create client with mock server URL
	config := DefaultAudioConfig()
	config.WhisperURL = server.URL
	config.MaxRetries = 0 // No retries for this test
	client := NewWhisperClient(&config)

	// Test transcription
	ctx := context.Background()
	result, err := client.Transcribe(ctx, audioPath)

	if err != nil {
		t.Fatalf("Transcription failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Text != "This is a test transcription" {
		t.Errorf("Expected text 'This is a test transcription', got '%s'", result.Text)
	}

	if result.Language != "en" {
		t.Errorf("Expected language 'en', got '%s'", result.Language)
	}

	if result.Duration != 10.5 {
		t.Errorf("Expected duration 10.5, got %f", result.Duration)
	}

	if result.WordCount != 5 {
		t.Errorf("Expected word count 5, got %d", result.WordCount)
	}

	if result.ProcessingTime == 0 {
		t.Error("Expected non-zero processing time")
	}
}

// TestTranscribeWithTrailingSlash tests URL handling with trailing slash
func TestTranscribeWithTrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transcribe" {
			t.Errorf("Expected path '/transcribe', got '%s'", r.URL.Path)
		}
		response := map[string]interface{}{
			"text":     "Test",
			"language": "en",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	audioPath := filepath.Join(tempDir, "test.mp3")
	os.WriteFile(audioPath, []byte("fake audio"), 0644)

	config := DefaultAudioConfig()
	config.WhisperURL = server.URL + "/" // Trailing slash
	config.MaxRetries = 0
	client := NewWhisperClient(&config)

	ctx := context.Background()
	_, err := client.Transcribe(ctx, audioPath)

	if err != nil {
		t.Errorf("Transcription with trailing slash failed: %v", err)
	}
}

// TestTranscribeFileNotFound tests error when file doesn't exist
func TestTranscribeFileNotFound(t *testing.T) {
	config := DefaultAudioConfig()
	config.MaxRetries = 0
	client := NewWhisperClient(&config)

	ctx := context.Background()
	result, err := client.Transcribe(ctx, "/nonexistent/file.mp3")

	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}

	if result != nil {
		t.Error("Expected nil result for failed transcription")
	}

	if !strings.Contains(err.Error(), "failed to open audio file") {
		t.Errorf("Expected 'failed to open audio file' error, got: %v", err)
	}
}

// TestTranscribeEmptyResponse tests handling of empty transcription
func TestTranscribeEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"text":     "",
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
	client := NewWhisperClient(&config)

	ctx := context.Background()
	result, err := client.Transcribe(ctx, audioPath)

	if err == nil {
		t.Error("Expected error for empty transcription, got nil")
	}

	if !strings.Contains(err.Error(), "transcription result is empty") {
		t.Errorf("Expected transcription empty error, got: %v", err)
	}

	if result != nil {
		t.Error("Expected nil result for empty transcription")
	}
}

// TestTranscribeInvalidJSON tests handling of invalid JSON response
func TestTranscribeInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json {{{"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	audioPath := filepath.Join(tempDir, "test.mp3")
	os.WriteFile(audioPath, []byte("fake audio"), 0644)

	config := DefaultAudioConfig()
	config.WhisperURL = server.URL
	config.MaxRetries = 0
	client := NewWhisperClient(&config)

	ctx := context.Background()
	result, err := client.Transcribe(ctx, audioPath)

	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}

	if !strings.Contains(err.Error(), "invalid whisper response") {
		t.Errorf("Expected invalid response error, got: %v", err)
	}

	if result != nil {
		t.Error("Expected nil result for invalid JSON")
	}
}

// TestTranscribeNon200Status tests handling of non-200 status codes
func TestTranscribeNon200Status(t *testing.T) {
	testCases := []struct {
		statusCode int
		name       string
	}{
		{http.StatusBadRequest, "400 Bad Request"},
		{http.StatusInternalServerError, "500 Internal Server Error"},
		{http.StatusServiceUnavailable, "503 Service Unavailable"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				w.Write([]byte("Error message"))
			}))
			defer server.Close()

			tempDir := t.TempDir()
			audioPath := filepath.Join(tempDir, "test.mp3")
			os.WriteFile(audioPath, []byte("fake audio"), 0644)

			config := DefaultAudioConfig()
			config.WhisperURL = server.URL
			config.MaxRetries = 0
			client := NewWhisperClient(&config)

			ctx := context.Background()
			result, err := client.Transcribe(ctx, audioPath)

			if err == nil {
				t.Errorf("Expected error for status %d, got nil", tc.statusCode)
			}

			if result != nil {
				t.Error("Expected nil result for error response")
			}

			// Should be retryable error
			if !IsRetryable(err) {
				t.Error("Expected retryable error for HTTP error status")
			}
		})
	}
}

// TestRetryLogicWithExponentialBackoff tests retry mechanism
func TestRetryLogicWithExponentialBackoff(t *testing.T) {
	attemptCount := 0
	var attemptTimes []time.Time
	mu := sync.Mutex{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attemptCount++
		attemptTimes = append(attemptTimes, time.Now())
		mu.Unlock()

		// Fail first 2 attempts, succeed on 3rd
		if attemptCount < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		response := map[string]interface{}{
			"text":     "Success after retries",
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
	config.MaxRetries = 3
	config.RetryDelay = 1 // 1 second base delay
	client := NewWhisperClient(&config)

	ctx := context.Background()
	startTime := time.Now()
	result, err := client.Transcribe(ctx, audioPath)
	totalDuration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}

	if result == nil || result.Text != "Success after retries" {
		t.Error("Expected successful transcription result")
	}

	mu.Lock()
	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}

	// Verify exponential backoff timing
	if len(attemptTimes) >= 3 {
		// Delay between attempt 1 and 2 should be ~1 second (baseDelay * 1)
		delay1 := attemptTimes[1].Sub(attemptTimes[0])
		if delay1 < 900*time.Millisecond || delay1 > 1200*time.Millisecond {
			t.Errorf("First retry delay should be ~1s, got %v", delay1)
		}

		// Delay between attempt 2 and 3 should be ~2 seconds (baseDelay * 2)
		delay2 := attemptTimes[2].Sub(attemptTimes[1])
		if delay2 < 1900*time.Millisecond || delay2 > 2200*time.Millisecond {
			t.Errorf("Second retry delay should be ~2s, got %v", delay2)
		}
	}
	mu.Unlock()

	t.Logf("Total duration: %v, Attempts: %d", totalDuration, attemptCount)
}

// TestCalculateRetryDelay tests exponential backoff calculation
func TestCalculateRetryDelay(t *testing.T) {
	config := DefaultAudioConfig()
	config.RetryDelay = 2 // 2 seconds
	client := NewWhisperClient(&config)

	testCases := []struct {
		attempt       int
		expectedDelay time.Duration
	}{
		{1, 2 * time.Second},  // 2 * 2^0
		{2, 4 * time.Second},  // 2 * 2^1
		{3, 8 * time.Second},  // 2 * 2^2
		{4, 16 * time.Second}, // 2 * 2^3
		{5, 30 * time.Second}, // Capped at 30
		{6, 30 * time.Second}, // Capped at 30
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Attempt_%d", tc.attempt), func(t *testing.T) {
			delay := client.calculateRetryDelay(tc.attempt)
			if delay != tc.expectedDelay {
				t.Errorf("Expected delay %v for attempt %d, got %v",
					tc.expectedDelay, tc.attempt, delay)
			}
		})
	}
}

// TestRetryExhaustion tests behavior when all retries are exhausted
func TestRetryExhaustion(t *testing.T) {
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
	config.MaxRetries = 2
	config.RetryDelay = 0 // No delay for faster test
	client := NewWhisperClient(&config)

	ctx := context.Background()
	result, err := client.Transcribe(ctx, audioPath)

	if err == nil {
		t.Error("Expected error after retry exhaustion, got nil")
	}

	if result != nil {
		t.Error("Expected nil result after retry exhaustion")
	}

	// Should have made initial attempt + MaxRetries attempts
	expectedAttempts := config.MaxRetries + 1
	if attemptCount != expectedAttempts {
		t.Errorf("Expected %d attempts, got %d", expectedAttempts, attemptCount)
	}

	// Error message should include attempt count
	if !strings.Contains(err.Error(), fmt.Sprintf("after %d attempts", expectedAttempts)) {
		t.Errorf("Error message should include attempt count, got: %v", err)
	}
}

// TestNonRetryableErrorAbortsImmediately tests that non-retryable errors abort
func TestNonRetryableErrorAbortsImmediately(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		// Return empty text (non-retryable error)
		response := map[string]interface{}{
			"text":     "",
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
	config.MaxRetries = 3
	client := NewWhisperClient(&config)

	ctx := context.Background()
	result, err := client.Transcribe(ctx, audioPath)

	if err == nil {
		t.Error("Expected error for empty transcription, got nil")
	}

	if !strings.Contains(err.Error(), "transcription result is empty") {
		t.Errorf("Expected transcription empty error, got: %v", err)
	}

	if result != nil {
		t.Error("Expected nil result")
	}

	// Should only make 1 attempt (no retries for non-retryable error)
	if attemptCount != 1 {
		t.Errorf("Expected 1 attempt for non-retryable error, got %d", attemptCount)
	}
}

// TestContextCancellation tests context cancellation
func TestContextCancellation(t *testing.T) {
	blockChan := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blockChan // Block until test cancels
	}))
	defer server.Close()
	defer close(blockChan)

	tempDir := t.TempDir()
	audioPath := filepath.Join(tempDir, "test.mp3")
	os.WriteFile(audioPath, []byte("fake audio"), 0644)

	config := DefaultAudioConfig()
	config.WhisperURL = server.URL
	config.MaxRetries = 0
	client := NewWhisperClient(&config)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	result, err := client.Transcribe(ctx, audioPath)

	if err == nil {
		t.Error("Expected error from context cancellation, got nil")
	}

	if result != nil {
		t.Error("Expected nil result from context cancellation")
	}

	// Should get timeout error
	if !strings.Contains(err.Error(), "whisper transcription timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

// TestCircuitBreakerIntegration tests circuit breaker integration with client
func TestCircuitBreakerIntegration(t *testing.T) {
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
	config.MaxRetries = 0
	config.CircuitBreakerFailureThreshold = 3
	client := NewWhisperClient(&config)

	ctx := context.Background()

	// Initial state should be closed
	if client.GetCircuitBreakerState() != CircuitBreakerClosed {
		t.Errorf("Expected initial state Closed, got %s", client.GetCircuitBreakerState())
	}

	// Make failures up to threshold
	for i := 0; i < config.CircuitBreakerFailureThreshold; i++ {
		client.Transcribe(ctx, audioPath)
	}

	// Circuit should now be open
	if client.GetCircuitBreakerState() != CircuitBreakerOpen {
		t.Errorf("Expected state Open after threshold failures, got %s",
			client.GetCircuitBreakerState())
	}

	// Next attempt should be rejected by circuit breaker
	beforeCount := atomic.LoadInt32(&attemptCount)
	result, err := client.Transcribe(ctx, audioPath)

	if err == nil {
		t.Error("Expected error when circuit is open, got nil")
	}

	if !strings.Contains(err.Error(), "circuit breaker is open") {
		t.Errorf("Expected circuit breaker open error, got: %v", err)
	}

	if result != nil {
		t.Error("Expected nil result when circuit is open")
	}

	afterCount := atomic.LoadInt32(&attemptCount)
	if afterCount != beforeCount {
		t.Error("Request should not have reached server when circuit is open")
	}
}

// TestClientCircuitBreakerReset tests manual circuit breaker reset via client
func TestClientCircuitBreakerReset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	audioPath := filepath.Join(tempDir, "test.mp3")
	os.WriteFile(audioPath, []byte("fake audio"), 0644)

	config := DefaultAudioConfig()
	config.WhisperURL = server.URL
	config.MaxRetries = 0
	config.CircuitBreakerFailureThreshold = 2
	client := NewWhisperClient(&config)

	ctx := context.Background()

	// Open the circuit
	for i := 0; i < config.CircuitBreakerFailureThreshold; i++ {
		client.Transcribe(ctx, audioPath)
	}

	if client.GetCircuitBreakerState() != CircuitBreakerOpen {
		t.Fatal("Expected circuit to be open")
	}

	// Reset circuit breaker
	client.ResetCircuitBreaker()

	if client.GetCircuitBreakerState() != CircuitBreakerClosed {
		t.Errorf("Expected state Closed after reset, got %s", client.GetCircuitBreakerState())
	}
}

// TestHealthCheck tests the health check endpoint
func TestHealthCheck(t *testing.T) {
	t.Run("Healthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				t.Errorf("Expected /health path, got %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := DefaultAudioConfig()
		config.WhisperURL = server.URL
		client := NewWhisperClient(&config)

		ctx := context.Background()
		err := client.HealthCheck(ctx)

		if err != nil {
			t.Errorf("Expected nil error for healthy service, got: %v", err)
		}
	})

	t.Run("Unhealthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		config := DefaultAudioConfig()
		config.WhisperURL = server.URL
		client := NewWhisperClient(&config)

		ctx := context.Background()
		err := client.HealthCheck(ctx)

		if err == nil {
			t.Error("Expected error for unhealthy service, got nil")
		}

		if !strings.Contains(err.Error(), "whisper service unavailable") {
			t.Errorf("Expected unavailable error, got: %v", err)
		}
	})

	t.Run("Unreachable", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.WhisperURL = "http://nonexistent-server-12345.local:9999"
		client := NewWhisperClient(&config)

		ctx := context.Background()
		err := client.HealthCheck(ctx)

		if err != ErrWhisperUnavailable {
			t.Errorf("Expected ErrWhisperUnavailable for unreachable service, got: %v", err)
		}
	})

	t.Run("Timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Second) // Longer than health check timeout
		}))
		defer server.Close()

		config := DefaultAudioConfig()
		config.WhisperURL = server.URL
		client := NewWhisperClient(&config)

		ctx := context.Background()
		err := client.HealthCheck(ctx)

		if err != ErrWhisperUnavailable {
			t.Errorf("Expected ErrWhisperUnavailable for timeout, got: %v", err)
		}
	})
}

// TestMetricsCollection tests that metrics are properly recorded
func TestMetricsCollection(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		client.Transcribe(ctx, audioPath)

		// Verify timer was recorded
		if mockMetrics.timerCount == 0 {
			t.Error("Expected timer to be recorded")
		}

		// Verify success counter was incremented
		if mockMetrics.successCount == 0 {
			t.Error("Expected success counter to be incremented")
		}

		// Verify correct tags
		if mockMetrics.lastTimerTags["status"] != "success" {
			t.Errorf("Expected status tag 'success', got %s", mockMetrics.lastTimerTags["status"])
		}
	})

	t.Run("Failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
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
		client.Transcribe(ctx, audioPath)

		// Verify timer was recorded
		if mockMetrics.timerCount == 0 {
			t.Error("Expected timer to be recorded")
		}

		// Verify failed counter was incremented
		if mockMetrics.failedCount == 0 {
			t.Error("Expected failed counter to be incremented")
		}

		// Verify correct tags
		if mockMetrics.lastTimerTags["status"] != "failed" {
			t.Errorf("Expected status tag 'failed', got %s", mockMetrics.lastTimerTags["status"])
		}
	})
}

// TestLoggingBehavior tests logging at various stages
func TestLoggingBehavior(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"text":     "Test",
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
	mockLogger := &MockLogger{logs: make([]LogEntry, 0)}
	client := NewWhisperClient(&config).WithLogger(mockLogger)

	ctx := context.Background()
	client.Transcribe(ctx, audioPath)

	// Verify start log
	hasStartLog := false
	for _, log := range mockLogger.logs {
		if log.Level == "info" && strings.Contains(log.Message, "Starting transcription") {
			hasStartLog = true
			break
		}
	}
	if !hasStartLog {
		t.Error("Expected start log message")
	}

	// Verify success log
	hasSuccessLog := false
	for _, log := range mockLogger.logs {
		if log.Level == "info" && strings.Contains(log.Message, "Transcription successful") {
			hasSuccessLog = true
			break
		}
	}
	if !hasSuccessLog {
		t.Error("Expected success log message")
	}
}

// TestConcurrentTranscriptions tests thread safety under concurrent load
func TestConcurrentTranscriptions(t *testing.T) {
	requestCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		// Small delay to simulate processing
		time.Sleep(10 * time.Millisecond)
		response := map[string]interface{}{
			"text":     "Concurrent transcription",
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
	client := NewWhisperClient(&config)

	const numGoroutines = 20
	var wg sync.WaitGroup
	successCount := int32(0)
	errorCount := int32(0)

	ctx := context.Background()
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			result, err := client.Transcribe(ctx, audioPath)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
			} else if result != nil {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	if successCount != numGoroutines {
		t.Errorf("Expected %d successful transcriptions, got %d", numGoroutines, successCount)
	}

	if errorCount != 0 {
		t.Errorf("Expected 0 errors, got %d", errorCount)
	}

	if atomic.LoadInt32(&requestCount) != numGoroutines {
		t.Errorf("Expected %d requests, got %d", numGoroutines, requestCount)
	}
}

// MockMetrics for testing
type MockMetrics struct {
	timerCount    int
	successCount  int
	failedCount   int
	lastTimerTags map[string]string
	mu            sync.Mutex
}

func (mm *MockMetrics) IncrementCounter(name string, tags map[string]string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if strings.Contains(name, "success") {
		mm.successCount++
	} else if strings.Contains(name, "failed") {
		mm.failedCount++
	}
}

func (mm *MockMetrics) AddToCounter(name string, value float64, tags map[string]string) {}

func (mm *MockMetrics) SetGauge(name string, value float64, tags map[string]string) {}

func (mm *MockMetrics) RecordHistogram(name string, value float64, tags map[string]string) {}

func (mm *MockMetrics) StartTimer(name string, tags map[string]string) interfaces.Timer {
	return &MockTimer{}
}

// MockTimer for testing
type MockTimer struct{}

func (mt *MockTimer) Stop() float64 {
	return 0
}

func (mt *MockTimer) Cancel() {}

func (mm *MockMetrics) RecordTimer(name string, duration float64, tags map[string]string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.timerCount++
	mm.lastTimerTags = tags
}

func (mm *MockMetrics) RecordCustomMetric(name string, value interface{}, metricType string, tags map[string]string) {
}

// BenchmarkTranscribeSuccess benchmarks successful transcription
func BenchmarkTranscribeSuccess(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"text":     "Benchmark transcription",
			"language": "en",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tempDir := b.TempDir()
	audioPath := filepath.Join(tempDir, "test.mp3")
	os.WriteFile(audioPath, []byte("fake audio data for benchmarking"), 0644)

	config := DefaultAudioConfig()
	config.WhisperURL = server.URL
	config.MaxRetries = 0
	client := NewWhisperClient(&config)

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		client.Transcribe(ctx, audioPath)
	}
}
