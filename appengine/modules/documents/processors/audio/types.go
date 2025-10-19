package audio

import (
	"errors"
	"path/filepath"
	"strings"
	"time"
)

// TranscriptionResult represents the result of audio transcription from Whisper
type TranscriptionResult struct {
	// Text is the transcribed text content
	Text string `json:"text"`

	// Language is the detected language code (e.g., "en", "es", "fr")
	Language string `json:"language"`

	// LanguageConfidence is the confidence level of language detection (0.0 to 1.0)
	// Not all Whisper implementations return this, may be 0.0
	LanguageConfidence float64 `json:"language_confidence,omitempty"`

	// Duration is the audio duration in seconds (if available)
	Duration float64 `json:"duration,omitempty"`

	// WordCount is the number of words in the transcription
	WordCount int `json:"word_count"`

	// ProcessingTime is how long transcription took
	ProcessingTime time.Duration `json:"processing_time"`

	// Timestamp when transcription was completed
	TranscribedAt time.Time `json:"transcribed_at"`
}

// Audio processing errors
var (
	// ErrWhisperUnavailable indicates the Whisper service is not available
	ErrWhisperUnavailable = errors.New("whisper service unavailable")

	// ErrWhisperTimeout indicates the transcription request timed out
	ErrWhisperTimeout = errors.New("whisper transcription timeout")

	// ErrWhisperRequestFailed indicates the HTTP request to Whisper failed
	ErrWhisperRequestFailed = errors.New("whisper request failed")

	// ErrWhisperResponseInvalid indicates the Whisper response was invalid or malformed
	ErrWhisperResponseInvalid = errors.New("invalid whisper response")

	// ErrAudioTooLarge indicates the audio file exceeds the size limit
	ErrAudioTooLarge = errors.New("audio file exceeds size limit")

	// ErrAudioDurationExceeded indicates the audio duration exceeds the limit
	ErrAudioDurationExceeded = errors.New("audio duration exceeds limit")

	// ErrUnsupportedFormat indicates the audio format is not supported
	ErrUnsupportedFormat = errors.New("unsupported audio format")

	// ErrTranscriptionEmpty indicates Whisper returned an empty transcription
	ErrTranscriptionEmpty = errors.New("transcription result is empty")

	// ErrCircuitBreakerOpen indicates the circuit breaker is open (too many failures)
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
)

// ProcessingMetadata contains metadata about audio processing
type ProcessingMetadata struct {
	// Original audio file information
	OriginalFormat string `json:"original_format"`
	OriginalSize   int64  `json:"original_size"`
	FileName       string `json:"file_name"`
	FilePath       string `json:"file_path"`

	// Transcription information
	TranscriptionLanguage string  `json:"transcription_language"`
	TranscriptionDuration float64 `json:"transcription_duration_seconds"`
	WordCount             int     `json:"word_count"`

	// Processing information
	ProcessedAt        time.Time     `json:"processed_at"`
	ProcessingDuration time.Duration `json:"processing_duration"`
	ProcessorVersion   string        `json:"processor_version"`

	// Whisper service information
	WhisperURL    string `json:"whisper_url"`
	WhisperModel  string `json:"whisper_model"`
	RetryAttempts int    `json:"retry_attempts"`

	// Quality indicators
	LanguageConfidence float64 `json:"language_confidence,omitempty"`
}

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState string

const (
	// CircuitBreakerClosed means the circuit is functioning normally
	CircuitBreakerClosed CircuitBreakerState = "closed"

	// CircuitBreakerOpen means the circuit is open due to failures
	CircuitBreakerOpen CircuitBreakerState = "open"

	// CircuitBreakerHalfOpen means the circuit is testing if the service recovered
	CircuitBreakerHalfOpen CircuitBreakerState = "half-open"
)

// String returns the string representation of CircuitBreakerState
func (s CircuitBreakerState) String() string {
	return string(s)
}

// IsValid checks if the circuit breaker state is valid
func (s CircuitBreakerState) IsValid() bool {
	switch s {
	case CircuitBreakerClosed, CircuitBreakerOpen, CircuitBreakerHalfOpen:
		return true
	default:
		return false
	}
}

// RetryableError wraps an error to indicate it should be retried
type RetryableError struct {
	Err     error
	Attempt int
}

// Error implements the error interface
func (re *RetryableError) Error() string {
	return re.Err.Error()
}

// Unwrap returns the underlying error
func (re *RetryableError) Unwrap() error {
	return re.Err
}

// IsRetryable checks if an error should be retried
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's already a RetryableError
	var retryErr *RetryableError
	if errors.As(err, &retryErr) {
		return true
	}

	// Check for specific retryable errors
	return errors.Is(err, ErrWhisperUnavailable) ||
		errors.Is(err, ErrWhisperTimeout) ||
		errors.Is(err, ErrWhisperRequestFailed)
}

// AudioProcessingStats contains statistics about audio processing
type AudioProcessingStats struct {
	TotalProcessed        int64               `json:"total_processed"`
	TotalFailed           int64               `json:"total_failed"`
	TotalRetries          int64               `json:"total_retries"`
	AverageProcessingTime time.Duration       `json:"average_processing_time"`
	TotalProcessingTime   time.Duration       `json:"total_processing_time"`
	LastProcessedAt       time.Time           `json:"last_processed_at,omitempty"`
	CircuitBreakerState   CircuitBreakerState `json:"circuit_breaker_state"`
}

// NewTranscriptionResult creates a new TranscriptionResult with defaults
func NewTranscriptionResult(text, language string) *TranscriptionResult {
	wordCount := len(strings.Fields(text))

	return &TranscriptionResult{
		Text:          text,
		Language:      language,
		WordCount:     wordCount,
		TranscribedAt: time.Now(),
	}
}

// IsEmpty checks if the transcription result is empty
func (tr *TranscriptionResult) IsEmpty() bool {
	return tr.Text == "" || len(strings.TrimSpace(tr.Text)) == 0
}

// NewProcessingMetadata creates a new ProcessingMetadata with common fields
func NewProcessingMetadata(filePath, format string, fileSize int64) *ProcessingMetadata {
	return &ProcessingMetadata{
		OriginalFormat:   format,
		OriginalSize:     fileSize,
		FilePath:         filePath,
		FileName:         filepath.Base(filePath),
		ProcessedAt:      time.Now(),
		ProcessorVersion: "1.0.0",
	}
}
