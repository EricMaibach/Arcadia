package audio

import (
	"testing"
	"time"
)

// TestDefaultAudioConfig tests the default configuration function
func TestDefaultAudioConfig(t *testing.T) {
	config := DefaultAudioConfig()

	// Test Whisper Service defaults
	if config.WhisperURL != "http://whisper:8002" {
		t.Errorf("Expected WhisperURL to be 'http://whisper:8002', got '%s'", config.WhisperURL)
	}
	if config.WhisperTimeout != 300 {
		t.Errorf("Expected WhisperTimeout to be 300, got %d", config.WhisperTimeout)
	}

	// Test Retry defaults
	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", config.MaxRetries)
	}
	if config.RetryDelay != 2 {
		t.Errorf("Expected RetryDelay to be 2, got %d", config.RetryDelay)
	}

	// Test Circuit Breaker defaults
	if config.CircuitBreakerFailureThreshold != 5 {
		t.Errorf("Expected CircuitBreakerFailureThreshold to be 5, got %d", config.CircuitBreakerFailureThreshold)
	}
	if config.CircuitBreakerResetTimeout != 60 {
		t.Errorf("Expected CircuitBreakerResetTimeout to be 60, got %d", config.CircuitBreakerResetTimeout)
	}

	// Test File Validation defaults
	if config.MaxAudioFileSize != 500*1024*1024 {
		t.Errorf("Expected MaxAudioFileSize to be 500MB, got %d", config.MaxAudioFileSize)
	}
	if config.MaxAudioDuration != 3600 {
		t.Errorf("Expected MaxAudioDuration to be 3600, got %d", config.MaxAudioDuration)
	}
	expectedFormats := []string{".mp3", ".wav", ".m4a", ".flac", ".ogg", ".aac"}
	if len(config.SupportedFormats) != len(expectedFormats) {
		t.Errorf("Expected %d supported formats, got %d", len(expectedFormats), len(config.SupportedFormats))
	}

	// Test Processing defaults
	if config.ConcurrentWorkers != 2 {
		t.Errorf("Expected ConcurrentWorkers to be 2, got %d", config.ConcurrentWorkers)
	}
	if !config.EnableCaching {
		t.Error("Expected EnableCaching to be true")
	}
	if config.CacheTTL != 3600 {
		t.Errorf("Expected CacheTTL to be 3600, got %d", config.CacheTTL)
	}

	// Test Fallback defaults
	if !config.EnableGracefulDegradation {
		t.Error("Expected EnableGracefulDegradation to be true")
	}
	if config.FallbackLanguage != "en" {
		t.Errorf("Expected FallbackLanguage to be 'en', got '%s'", config.FallbackLanguage)
	}

	// Test Logging defaults
	if config.EnableVerboseLogging {
		t.Error("Expected EnableVerboseLogging to be false")
	}

	// Most importantly, default config should be valid
	if err := config.Validate(); err != nil {
		t.Errorf("Default configuration should be valid, got error: %v", err)
	}
}

// TestValidateValidConfigurations tests validation with valid configurations
func TestValidateValidConfigurations(t *testing.T) {
	tests := []struct {
		name   string
		config AudioConfig
	}{
		{
			name:   "Default configuration",
			config: DefaultAudioConfig(),
		},
		{
			name: "Minimum valid values",
			config: AudioConfig{
				WhisperURL:                     "http://localhost:8000",
				WhisperTimeout:                 1,
				MaxRetries:                     0,
				RetryDelay:                     0,
				CircuitBreakerFailureThreshold: 1,
				CircuitBreakerResetTimeout:     1,
				MaxAudioFileSize:               1,
				MaxAudioDuration:               1,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              1,
				EnableCaching:                  false,
				CacheTTL:                       0,
				FallbackLanguage:               "en",
			},
		},
		{
			name: "Maximum boundary values",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:9999",
				WhisperTimeout:                 3600, // Max allowed
				MaxRetries:                     10,   // Max allowed
				RetryDelay:                     1000,
				CircuitBreakerFailureThreshold: 100,
				CircuitBreakerResetTimeout:     3600,
				MaxAudioFileSize:               1 * 1024 * 1024 * 1024, // 1GB max
				MaxAudioDuration:               7200,                   // Max allowed
				SupportedFormats:               []string{".mp3", ".wav", ".m4a", ".flac", ".ogg"},
				ConcurrentWorkers:              20, // Max allowed
				EnableCaching:                  true,
				CacheTTL:                       86400, // 24 hours
				FallbackLanguage:               "es",
			},
		},
		{
			name: "Caching disabled with zero TTL",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				EnableCaching:                  false, // Disabled, so TTL can be 0
				CacheTTL:                       0,
				FallbackLanguage:               "en",
			},
		},
		{
			name: "Custom supported formats",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3", ".wav", ".custom"},
				ConcurrentWorkers:              2,
				EnableCaching:                  true,
				CacheTTL:                       3600,
				FallbackLanguage:               "fr",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err != nil {
				t.Errorf("Expected valid configuration, got error: %v", err)
			}
		})
	}
}

// TestValidateInvalidConfigurations tests validation with invalid configurations
func TestValidateInvalidConfigurations(t *testing.T) {
	tests := []struct {
		name        string
		config      AudioConfig
		expectedErr string
	}{
		{
			name: "Empty Whisper URL",
			config: AudioConfig{
				WhisperURL:                     "",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "whisper_url cannot be empty",
		},
		{
			name: "Negative Whisper timeout",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 -1,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "whisper_timeout must be positive",
		},
		{
			name: "Zero Whisper timeout",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 0,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "whisper_timeout must be positive",
		},
		{
			name: "Whisper timeout exceeds maximum",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 3601, // Over 3600 limit
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "whisper_timeout too large",
		},
		{
			name: "Negative retry count",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     -1,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "max_retries cannot be negative",
		},
		{
			name: "Retry count exceeds maximum",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     11, // Over 10 limit
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "max_retries too large",
		},
		{
			name: "Negative retry delay",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     -1,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "retry_delay cannot be negative",
		},
		{
			name: "Zero circuit breaker failure threshold",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 0,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "circuit_breaker_failure_threshold must be positive",
		},
		{
			name: "Negative circuit breaker failure threshold",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: -1,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "circuit_breaker_failure_threshold must be positive",
		},
		{
			name: "Zero circuit breaker reset timeout",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     0,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "circuit_breaker_reset_timeout must be positive",
		},
		{
			name: "Negative circuit breaker reset timeout",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     -1,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "circuit_breaker_reset_timeout must be positive",
		},
		{
			name: "Zero file size limit",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               0,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "max_audio_file_size must be positive",
		},
		{
			name: "Negative file size limit",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               -1,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "max_audio_file_size must be positive",
		},
		{
			name: "File size exceeds 1GB",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               1*1024*1024*1024 + 1, // 1GB + 1 byte
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "max_audio_file_size too large",
		},
		{
			name: "Zero duration limit",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               0,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "max_audio_duration must be positive",
		},
		{
			name: "Negative duration limit",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               -1,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "max_audio_duration must be positive",
		},
		{
			name: "Duration exceeds maximum",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               7201, // Over 7200 limit
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "max_audio_duration too large",
		},
		{
			name: "Empty supported formats list",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "supported_formats cannot be empty",
		},
		{
			name: "Format without leading dot",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{"mp3"}, // Missing dot
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "supported_formats[0] must start with dot",
		},
		{
			name: "Empty format in list",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3", ""},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "en",
			},
			expectedErr: "supported_formats[1] cannot be empty",
		},
		{
			name: "Zero concurrent workers",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              0,
				FallbackLanguage:               "en",
			},
			expectedErr: "concurrent_workers must be positive",
		},
		{
			name: "Negative concurrent workers",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              -1,
				FallbackLanguage:               "en",
			},
			expectedErr: "concurrent_workers must be positive",
		},
		{
			name: "Concurrent workers exceeds maximum",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              21, // Over 20 limit
				FallbackLanguage:               "en",
			},
			expectedErr: "concurrent_workers too large",
		},
		{
			name: "Negative cache TTL when caching enabled",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				EnableCaching:                  true,
				CacheTTL:                       -1,
				FallbackLanguage:               "en",
			},
			expectedErr: "cache_ttl must be positive when caching is enabled",
		},
		{
			name: "Zero cache TTL when caching enabled",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				EnableCaching:                  true,
				CacheTTL:                       0,
				FallbackLanguage:               "en",
			},
			expectedErr: "cache_ttl must be positive when caching is enabled",
		},
		{
			name: "Empty fallback language",
			config: AudioConfig{
				WhisperURL:                     "http://whisper:8002",
				WhisperTimeout:                 300,
				MaxRetries:                     3,
				RetryDelay:                     2,
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerResetTimeout:     60,
				MaxAudioFileSize:               500 * 1024 * 1024,
				MaxAudioDuration:               3600,
				SupportedFormats:               []string{".mp3"},
				ConcurrentWorkers:              2,
				FallbackLanguage:               "",
			},
			expectedErr: "fallback_language cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err == nil {
				t.Errorf("Expected validation error containing '%s', got nil", tt.expectedErr)
				return
			}
			// Check that error message contains expected text
			errMsg := err.Error()
			if len(tt.expectedErr) > 0 {
				found := false
				// Simple substring check
				if len(errMsg) >= len(tt.expectedErr) {
					for i := 0; i <= len(errMsg)-len(tt.expectedErr); i++ {
						if errMsg[i:i+len(tt.expectedErr)] == tt.expectedErr {
							found = true
							break
						}
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedErr, errMsg)
				}
			}
		})
	}
}

// TestGetWhisperTimeoutDuration tests the Whisper timeout duration conversion
func TestGetWhisperTimeoutDuration(t *testing.T) {
	tests := []struct {
		name            string
		timeout         int
		expectedSeconds int
	}{
		{"Default timeout", 300, 300},
		{"Small timeout", 1, 1},
		{"Large timeout", 3600, 3600},
		{"Zero timeout", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AudioConfig{WhisperTimeout: tt.timeout}
			duration := config.GetWhisperTimeoutDuration()
			expected := time.Duration(tt.expectedSeconds) * time.Second
			if duration != expected {
				t.Errorf("Expected %v, got %v", expected, duration)
			}
		})
	}
}

// TestGetRetryDelayDuration tests the retry delay duration conversion
func TestGetRetryDelayDuration(t *testing.T) {
	tests := []struct {
		name            string
		delay           int
		expectedSeconds int
	}{
		{"Default delay", 2, 2},
		{"No delay", 0, 0},
		{"Long delay", 60, 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AudioConfig{RetryDelay: tt.delay}
			duration := config.GetRetryDelayDuration()
			expected := time.Duration(tt.expectedSeconds) * time.Second
			if duration != expected {
				t.Errorf("Expected %v, got %v", expected, duration)
			}
		})
	}
}

// TestGetCircuitBreakerResetTimeoutDuration tests the circuit breaker reset timeout conversion
func TestGetCircuitBreakerResetTimeoutDuration(t *testing.T) {
	tests := []struct {
		name            string
		timeout         int
		expectedSeconds int
	}{
		{"Default timeout", 60, 60},
		{"Short timeout", 10, 10},
		{"Long timeout", 300, 300},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AudioConfig{CircuitBreakerResetTimeout: tt.timeout}
			duration := config.GetCircuitBreakerResetTimeoutDuration()
			expected := time.Duration(tt.expectedSeconds) * time.Second
			if duration != expected {
				t.Errorf("Expected %v, got %v", expected, duration)
			}
		})
	}
}

// TestGetCacheTTLDuration tests the cache TTL duration conversion
func TestGetCacheTTLDuration(t *testing.T) {
	tests := []struct {
		name            string
		ttl             int
		expectedSeconds int
	}{
		{"Default TTL", 3600, 3600},
		{"Short TTL", 60, 60},
		{"Long TTL", 86400, 86400},
		{"Zero TTL", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AudioConfig{CacheTTL: tt.ttl}
			duration := config.GetCacheTTLDuration()
			expected := time.Duration(tt.expectedSeconds) * time.Second
			if duration != expected {
				t.Errorf("Expected %v, got %v", expected, duration)
			}
		})
	}
}

// TestIsFormatSupported tests the format checking functionality
func TestIsFormatSupported(t *testing.T) {
	config := DefaultAudioConfig()

	tests := []struct {
		name      string
		extension string
		supported bool
	}{
		{"MP3 format", ".mp3", true},
		{"WAV format", ".wav", true},
		{"M4A format", ".m4a", true},
		{"FLAC format", ".flac", true},
		{"OGG format", ".ogg", true},
		{"AAC format", ".aac", true},
		{"Unsupported AVI", ".avi", false},
		{"Unsupported MP4", ".mp4", false},
		{"Empty extension", "", false},
		{"Without dot", "mp3", false},
		{"Uppercase MP3", ".MP3", false}, // Case-sensitive
		{"Uppercase WAV", ".WAV", false}, // Case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.IsFormatSupported(tt.extension)
			if result != tt.supported {
				t.Errorf("IsFormatSupported(%s): expected %v, got %v", tt.extension, tt.supported, result)
			}
		})
	}
}

// TestIsFormatSupportedCustomFormats tests format checking with custom format list
func TestIsFormatSupportedCustomFormats(t *testing.T) {
	config := AudioConfig{
		SupportedFormats: []string{".custom", ".test"},
	}

	tests := []struct {
		extension string
		supported bool
	}{
		{".custom", true},
		{".test", true},
		{".mp3", false},
		{".wav", false},
	}

	for _, tt := range tests {
		t.Run(tt.extension, func(t *testing.T) {
			result := config.IsFormatSupported(tt.extension)
			if result != tt.supported {
				t.Errorf("Expected %v for %s, got %v", tt.supported, tt.extension, result)
			}
		})
	}
}

// TestEdgeCases tests boundary and edge cases
func TestEdgeCases(t *testing.T) {
	t.Run("Boundary: Exactly 1GB file size", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.MaxAudioFileSize = 1 * 1024 * 1024 * 1024 // Exactly 1GB
		if err := config.Validate(); err != nil {
			t.Errorf("1GB file size should be valid, got error: %v", err)
		}
	})

	t.Run("Boundary: Exactly 3600 second timeout", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.WhisperTimeout = 3600 // Exactly at max
		if err := config.Validate(); err != nil {
			t.Errorf("3600 second timeout should be valid, got error: %v", err)
		}
	})

	t.Run("Boundary: Exactly 7200 second duration", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.MaxAudioDuration = 7200 // Exactly at max
		if err := config.Validate(); err != nil {
			t.Errorf("7200 second duration should be valid, got error: %v", err)
		}
	})

	t.Run("Boundary: Exactly 10 retries", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.MaxRetries = 10 // Exactly at max
		if err := config.Validate(); err != nil {
			t.Errorf("10 retries should be valid, got error: %v", err)
		}
	})

	t.Run("Boundary: Exactly 20 workers", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.ConcurrentWorkers = 20 // Exactly at max
		if err := config.Validate(); err != nil {
			t.Errorf("20 workers should be valid, got error: %v", err)
		}
	})

	t.Run("Boundary: 1 worker (minimum)", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.ConcurrentWorkers = 1 // Minimum valid
		if err := config.Validate(); err != nil {
			t.Errorf("1 worker should be valid, got error: %v", err)
		}
	})

	t.Run("Edge: Very large but valid timeout", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.WhisperTimeout = 3599 // Just under max
		if err := config.Validate(); err != nil {
			t.Errorf("3599 second timeout should be valid, got error: %v", err)
		}
	})

	t.Run("Edge: Single format in list", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.SupportedFormats = []string{".mp3"}
		if err := config.Validate(); err != nil {
			t.Errorf("Single format should be valid, got error: %v", err)
		}
	})

	t.Run("Edge: Format with special characters", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.SupportedFormats = []string{".mp3-special", ".wav_test"}
		if err := config.Validate(); err != nil {
			t.Errorf("Formats with special characters should be valid, got error: %v", err)
		}
	})

	t.Run("Edge: Long format extension", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.SupportedFormats = []string{".verylongextension"}
		if err := config.Validate(); err != nil {
			t.Errorf("Long extension should be valid, got error: %v", err)
		}
	})

	t.Run("Edge: Caching disabled with negative TTL", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.EnableCaching = false
		config.CacheTTL = -100 // Should be ignored when caching disabled
		if err := config.Validate(); err != nil {
			t.Errorf("Negative TTL should be ignored when caching disabled, got error: %v", err)
		}
	})

	t.Run("Edge: Zero retries with zero delay", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.MaxRetries = 0
		config.RetryDelay = 0
		if err := config.Validate(); err != nil {
			t.Errorf("Zero retries with zero delay should be valid, got error: %v", err)
		}
	})
}
