package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"arcadia/modules/documents/interfaces"
)

// WhisperClient handles communication with the Whisper transcription service
type WhisperClient struct {
	config         *AudioConfig
	httpClient     *http.Client
	circuitBreaker *CircuitBreaker
	logger         interfaces.Logger
	metrics        interfaces.MetricsCollector
}

// NewWhisperClient creates a new Whisper HTTP client
func NewWhisperClient(config *AudioConfig) *WhisperClient {
	return &WhisperClient{
		config: config,
		httpClient: &http.Client{
			Timeout: config.GetWhisperTimeoutDuration(),
		},
		circuitBreaker: NewCircuitBreaker(config),
	}
}

// WithLogger adds logging to the client
func (wc *WhisperClient) WithLogger(logger interfaces.Logger) *WhisperClient {
	wc.logger = logger
	wc.circuitBreaker.WithLogger(logger)
	return wc
}

// WithMetrics adds metrics collection to the client
func (wc *WhisperClient) WithMetrics(metrics interfaces.MetricsCollector) *WhisperClient {
	wc.metrics = metrics
	return wc
}

// Transcribe sends an audio file to Whisper and returns the transcription
func (wc *WhisperClient) Transcribe(ctx context.Context, audioPath string) (*TranscriptionResult, error) {
	startTime := time.Now()

	if wc.logger != nil {
		wc.logger.Info(ctx, "Starting transcription request",
			"audio_path", audioPath,
			"whisper_url", wc.config.WhisperURL)
	}

	// Attempt transcription with retry logic
	var lastErr error
	for attempt := 0; attempt <= wc.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			delay := wc.calculateRetryDelay(attempt)
			if wc.logger != nil {
				wc.logger.Info(ctx, "Retrying transcription request",
					"attempt", attempt,
					"delay", delay)
			}
			time.Sleep(delay)
		}

		result, err := wc.transcribeWithCircuitBreaker(ctx, audioPath)
		if err == nil {
			// Success
			duration := time.Since(startTime)
			result.ProcessingTime = duration

			if wc.metrics != nil {
				wc.metrics.RecordTimer("audio.transcription.duration", duration.Seconds()*1000, map[string]string{
					"status": "success",
				})
				wc.metrics.IncrementCounter("audio.transcription.success", nil)
			}

			if wc.logger != nil {
				wc.logger.Info(ctx, "Transcription successful",
					"duration", duration,
					"language", result.Language,
					"word_count", result.WordCount,
					"attempts", attempt+1)
			}

			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			if wc.logger != nil {
				wc.logger.Error(ctx, "Non-retryable error, aborting",
					"error", err,
					"attempt", attempt+1)
			}
			break
		}

		if wc.logger != nil {
			wc.logger.Warn(ctx, "Transcription attempt failed",
				"error", err,
				"attempt", attempt+1,
				"max_retries", wc.config.MaxRetries)
		}
	}

	// All retries exhausted
	duration := time.Since(startTime)
	if wc.metrics != nil {
		wc.metrics.RecordTimer("audio.transcription.duration", duration.Seconds()*1000, map[string]string{
			"status": "failed",
		})
		wc.metrics.IncrementCounter("audio.transcription.failed", nil)
	}

	return nil, fmt.Errorf("transcription failed after %d attempts: %w", wc.config.MaxRetries+1, lastErr)
}

// transcribeWithCircuitBreaker performs transcription through the circuit breaker
func (wc *WhisperClient) transcribeWithCircuitBreaker(ctx context.Context, audioPath string) (*TranscriptionResult, error) {
	var result *TranscriptionResult
	var err error

	cbErr := wc.circuitBreaker.Call(ctx, func() error {
		result, err = wc.doTranscribe(ctx, audioPath)
		return err
	})

	if cbErr != nil {
		return nil, cbErr
	}

	return result, err
}

// doTranscribe performs the actual HTTP request to Whisper
func (wc *WhisperClient) doTranscribe(ctx context.Context, audioPath string) (*TranscriptionResult, error) {
	// Open the audio file
	file, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	// Create multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file to form
	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file to form: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create HTTP request
	url := strings.TrimRight(wc.config.WhisperURL, "/") + "/transcribe"
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := wc.httpClient.Do(req)
	if err != nil {
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, ErrWhisperTimeout
		}
		return nil, &RetryableError{Err: ErrWhisperRequestFailed, Attempt: 0}
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, &RetryableError{
			Err:     fmt.Errorf("%w: status %d: %s", ErrWhisperRequestFailed, resp.StatusCode, string(bodyBytes)),
			Attempt: 0,
		}
	}

	// Parse response
	var whisperResponse struct {
		Text     string  `json:"text"`
		Language string  `json:"language"`
		Duration float64 `json:"duration,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&whisperResponse); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrWhisperResponseInvalid, err)
	}

	// Validate response
	if whisperResponse.Text == "" {
		return nil, ErrTranscriptionEmpty
	}

	// Create result
	result := NewTranscriptionResult(whisperResponse.Text, whisperResponse.Language)
	result.Duration = whisperResponse.Duration

	return result, nil
}

// calculateRetryDelay calculates exponential backoff delay
func (wc *WhisperClient) calculateRetryDelay(attempt int) time.Duration {
	baseDelay := wc.config.GetRetryDelayDuration()
	// Exponential backoff: baseDelay * 2^(attempt-1)
	multiplier := 1 << (attempt - 1) // 2^(attempt-1)
	delay := baseDelay * time.Duration(multiplier)

	// Cap at 30 seconds
	maxDelay := 30 * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// HealthCheck checks if the Whisper service is available
func (wc *WhisperClient) HealthCheck(ctx context.Context) error {
	url := strings.TrimRight(wc.config.WhisperURL, "/") + "/health"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrWhisperUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrWhisperUnavailable, resp.StatusCode)
	}

	return nil
}

// GetCircuitBreakerState returns the current circuit breaker state
func (wc *WhisperClient) GetCircuitBreakerState() CircuitBreakerState {
	return wc.circuitBreaker.GetState()
}

// ResetCircuitBreaker resets the circuit breaker to closed state
func (wc *WhisperClient) ResetCircuitBreaker() {
	wc.circuitBreaker.Reset()
	if wc.logger != nil {
		wc.logger.Info(context.Background(), "Circuit breaker manually reset")
	}
}
