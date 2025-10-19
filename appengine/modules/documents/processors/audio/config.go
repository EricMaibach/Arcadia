package audio

import (
	"fmt"
	"time"
)

// AudioConfig contains configuration for the audio processor
type AudioConfig struct {
	// Whisper Service Configuration
	WhisperURL     string `json:"whisper_url"`     // Default: "http://whisper:8002"
	WhisperTimeout int    `json:"whisper_timeout"` // Timeout in seconds, default: 300 (5 minutes)

	// Retry and Circuit Breaker Configuration
	MaxRetries                     int `json:"max_retries"`                       // Default: 3
	RetryDelay                     int `json:"retry_delay"`                       // Seconds between retries, default: 2
	CircuitBreakerFailureThreshold int `json:"circuit_breaker_failure_threshold"` // Default: 5
	CircuitBreakerResetTimeout     int `json:"circuit_breaker_reset_timeout"`     // Seconds, default: 60

	// File Validation Configuration
	MaxAudioFileSize int64    `json:"max_audio_file_size"` // Bytes, default: 500MB
	MaxAudioDuration int      `json:"max_audio_duration"`  // Seconds, default: 3600 (1 hour)
	SupportedFormats []string `json:"supported_formats"`   // Default: [".mp3", ".wav", ".m4a", ".flac", ".ogg"]

	// Processing Configuration
	ConcurrentWorkers int  `json:"concurrent_workers"` // Default: 2
	EnableCaching     bool `json:"enable_caching"`     // Default: true
	CacheTTL          int  `json:"cache_ttl"`          // Cache TTL in seconds, default: 3600 (1 hour)

	// Fallback Configuration
	EnableGracefulDegradation bool   `json:"enable_graceful_degradation"` // Default: true
	FallbackLanguage          string `json:"fallback_language"`           // Default: "en"

	// Logging and Metrics
	EnableVerboseLogging bool `json:"enable_verbose_logging"` // Default: false
}

// DefaultAudioConfig returns the default audio processor configuration
func DefaultAudioConfig() AudioConfig {
	return AudioConfig{
		// Whisper Service defaults
		WhisperURL:     "http://whisper:8002",
		WhisperTimeout: 300, // 5 minutes for long audio files

		// Retry defaults
		MaxRetries: 3,
		RetryDelay: 2, // 2 seconds

		// Circuit breaker defaults
		CircuitBreakerFailureThreshold: 5,
		CircuitBreakerResetTimeout:     60, // 1 minute

		// File validation defaults
		MaxAudioFileSize: 500 * 1024 * 1024, // 500MB
		MaxAudioDuration: 3600,              // 1 hour
		SupportedFormats: []string{".mp3", ".wav", ".m4a", ".flac", ".ogg", ".aac"},

		// Processing defaults
		ConcurrentWorkers: 2,
		EnableCaching:     true,
		CacheTTL:          3600, // 1 hour

		// Fallback defaults
		EnableGracefulDegradation: true,
		FallbackLanguage:          "en",

		// Logging defaults
		EnableVerboseLogging: false,
	}
}

// Validate checks if the configuration is valid
func (c *AudioConfig) Validate() error {
	// Validate Whisper URL
	if c.WhisperURL == "" {
		return fmt.Errorf("whisper_url cannot be empty")
	}

	// Validate timeouts
	if c.WhisperTimeout <= 0 {
		return fmt.Errorf("whisper_timeout must be positive, got %d", c.WhisperTimeout)
	}
	if c.WhisperTimeout > 3600 {
		return fmt.Errorf("whisper_timeout too large (max 3600 seconds), got %d", c.WhisperTimeout)
	}

	// Validate retry settings
	if c.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative, got %d", c.MaxRetries)
	}
	if c.MaxRetries > 10 {
		return fmt.Errorf("max_retries too large (max 10), got %d", c.MaxRetries)
	}
	if c.RetryDelay < 0 {
		return fmt.Errorf("retry_delay cannot be negative, got %d", c.RetryDelay)
	}

	// Validate circuit breaker settings
	if c.CircuitBreakerFailureThreshold <= 0 {
		return fmt.Errorf("circuit_breaker_failure_threshold must be positive, got %d", c.CircuitBreakerFailureThreshold)
	}
	if c.CircuitBreakerResetTimeout <= 0 {
		return fmt.Errorf("circuit_breaker_reset_timeout must be positive, got %d", c.CircuitBreakerResetTimeout)
	}

	// Validate file size limits
	if c.MaxAudioFileSize <= 0 {
		return fmt.Errorf("max_audio_file_size must be positive, got %d", c.MaxAudioFileSize)
	}
	if c.MaxAudioFileSize > 1*1024*1024*1024 { // 1GB max
		return fmt.Errorf("max_audio_file_size too large (max 1GB), got %d", c.MaxAudioFileSize)
	}

	// Validate duration limits
	if c.MaxAudioDuration <= 0 {
		return fmt.Errorf("max_audio_duration must be positive, got %d", c.MaxAudioDuration)
	}
	if c.MaxAudioDuration > 7200 { // 2 hours max
		return fmt.Errorf("max_audio_duration too large (max 7200 seconds), got %d", c.MaxAudioDuration)
	}

	// Validate supported formats
	if len(c.SupportedFormats) == 0 {
		return fmt.Errorf("supported_formats cannot be empty")
	}
	for i, format := range c.SupportedFormats {
		if format == "" {
			return fmt.Errorf("supported_formats[%d] cannot be empty", i)
		}
		if format[0] != '.' {
			return fmt.Errorf("supported_formats[%d] must start with dot, got %s", i, format)
		}
	}

	// Validate concurrent workers
	if c.ConcurrentWorkers <= 0 {
		return fmt.Errorf("concurrent_workers must be positive, got %d", c.ConcurrentWorkers)
	}
	if c.ConcurrentWorkers > 20 {
		return fmt.Errorf("concurrent_workers too large (max 20), got %d", c.ConcurrentWorkers)
	}

	// Validate cache TTL
	if c.EnableCaching && c.CacheTTL <= 0 {
		return fmt.Errorf("cache_ttl must be positive when caching is enabled, got %d", c.CacheTTL)
	}

	// Validate fallback language
	if c.FallbackLanguage == "" {
		return fmt.Errorf("fallback_language cannot be empty")
	}

	return nil
}

// GetWhisperTimeoutDuration returns the Whisper timeout as a time.Duration
func (c *AudioConfig) GetWhisperTimeoutDuration() time.Duration {
	return time.Duration(c.WhisperTimeout) * time.Second
}

// GetRetryDelayDuration returns the retry delay as a time.Duration
func (c *AudioConfig) GetRetryDelayDuration() time.Duration {
	return time.Duration(c.RetryDelay) * time.Second
}

// GetCircuitBreakerResetTimeoutDuration returns the circuit breaker reset timeout as a time.Duration
func (c *AudioConfig) GetCircuitBreakerResetTimeoutDuration() time.Duration {
	return time.Duration(c.CircuitBreakerResetTimeout) * time.Second
}

// GetCacheTTLDuration returns the cache TTL as a time.Duration
func (c *AudioConfig) GetCacheTTLDuration() time.Duration {
	return time.Duration(c.CacheTTL) * time.Second
}

// IsFormatSupported checks if a file extension is supported
func (c *AudioConfig) IsFormatSupported(extension string) bool {
	for _, format := range c.SupportedFormats {
		if format == extension {
			return true
		}
	}
	return false
}
