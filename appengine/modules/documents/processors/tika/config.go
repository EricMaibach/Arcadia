package tika

import (
	"time"
)

// CircuitBreakerConfig holds configuration for the circuit breaker
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures before opening the circuit
	FailureThreshold int `json:"failure_threshold" yaml:"failure_threshold"`

	// ResetTimeout is how long to wait before attempting to close an open circuit
	ResetTimeout time.Duration `json:"reset_timeout" yaml:"reset_timeout"`

	// HalfOpenMaxRequests is the maximum number of requests allowed in half-open state
	HalfOpenMaxRequests int `json:"half_open_max_requests" yaml:"half_open_max_requests"`
}

// TikaConfig holds configuration for Apache Tika integration
type TikaConfig struct {
	// ServerURL is the base URL of the Tika server
	ServerURL string `json:"server_url" yaml:"server_url"`

	// Timeout is the HTTP request timeout
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// MaxRetries is the maximum number of retry attempts
	MaxRetries int `json:"max_retries" yaml:"max_retries"`

	// EnableFallback determines if Tika should be used as a fallback processor
	EnableFallback bool `json:"enable_fallback" yaml:"enable_fallback"`

	// MaxFileSize is the maximum file size to process (in bytes)
	MaxFileSize int64 `json:"max_file_size" yaml:"max_file_size"`

	// CircuitBreaker holds circuit breaker configuration
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker"`

	// OfficeFormats is a list of Office document formats supported
	OfficeFormats []string `json:"office_formats" yaml:"office_formats"`

	// MaxConnections is the maximum number of HTTP connections
	MaxConnections int `json:"max_connections" yaml:"max_connections"`

	// IdleConnTimeout is the timeout for idle connections
	IdleConnTimeout time.Duration `json:"idle_conn_timeout" yaml:"idle_conn_timeout"`

	// Processor-specific settings

	// AcceptAllFormats enables fallback mode for unknown file types
	AcceptAllFormats bool `json:"accept_all_formats" yaml:"accept_all_formats"`

	// OfficeExtensions defines file extensions for high-confidence processing
	OfficeExtensions []string `json:"office_extensions" yaml:"office_extensions"`

	// FallbackConfidence is the confidence level for fallback processing
	FallbackConfidence float64 `json:"fallback_confidence" yaml:"fallback_confidence"`

	// OfficeConfidence is the confidence level for known Office formats
	OfficeConfidence float64 `json:"office_confidence" yaml:"office_confidence"`
}

// DefaultTikaConfig returns a TikaConfig with sensible defaults
func DefaultTikaConfig() *TikaConfig {
	return &TikaConfig{
		ServerURL:       "http://tika:9998",
		Timeout:         30 * time.Second,
		MaxRetries:      3,
		EnableFallback:  true,
		MaxFileSize:     100 * 1024 * 1024, // 100MB
		MaxConnections:  10,
		IdleConnTimeout: 90 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold:    5,
			ResetTimeout:        60 * time.Second,
			HalfOpenMaxRequests: 3,
		},
		OfficeFormats: []string{
			"doc", "docx", "xls", "xlsx", "ppt", "pptx",
			"odt", "ods", "odp", "rtf", "wpd",
		},
		// Processor-specific defaults
		AcceptAllFormats:   false, // Start with Office-only mode
		OfficeExtensions:   []string{"doc", "docx", "xls", "xlsx", "ppt", "pptx", "odt", "ods", "odp"},
		FallbackConfidence: 0.3,   // Low confidence for unknown formats
		OfficeConfidence:   0.9,   // High confidence for known Office formats
	}
}

// Validate checks if the configuration is valid
func (c *TikaConfig) Validate() error {
	if c.ServerURL == "" {
		return ErrInvalidConfig{"server_url cannot be empty"}
	}
	if c.Timeout <= 0 {
		return ErrInvalidConfig{"timeout must be positive"}
	}
	if c.MaxRetries < 0 {
		return ErrInvalidConfig{"max_retries cannot be negative"}
	}
	if c.MaxFileSize <= 0 {
		return ErrInvalidConfig{"max_file_size must be positive"}
	}
	if c.CircuitBreaker.FailureThreshold <= 0 {
		return ErrInvalidConfig{"circuit_breaker.failure_threshold must be positive"}
	}
	if c.CircuitBreaker.ResetTimeout <= 0 {
		return ErrInvalidConfig{"circuit_breaker.reset_timeout must be positive"}
	}
	if c.CircuitBreaker.HalfOpenMaxRequests <= 0 {
		return ErrInvalidConfig{"circuit_breaker.half_open_max_requests must be positive"}
	}
	if c.FallbackConfidence < 0 || c.FallbackConfidence > 1 {
		return ErrInvalidConfig{"fallback_confidence must be between 0 and 1"}
	}
	if c.OfficeConfidence < 0 || c.OfficeConfidence > 1 {
		return ErrInvalidConfig{"office_confidence must be between 0 and 1"}
	}
	return nil
}

// ErrInvalidConfig represents a configuration validation error
type ErrInvalidConfig struct {
	Message string
}

func (e ErrInvalidConfig) Error() string {
	return "invalid tika config: " + e.Message
}