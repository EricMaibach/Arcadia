package audio

import (
	"encoding/json"
	"testing"
)

// TestJSONSerialization tests that AudioConfig can be serialized to JSON
func TestJSONSerialization(t *testing.T) {
	config := DefaultAudioConfig()

	// Serialize to JSON
	jsonData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}

	// Verify JSON is not empty
	if len(jsonData) == 0 {
		t.Error("Expected non-empty JSON data")
	}

	// Verify it contains expected fields
	jsonStr := string(jsonData)
	expectedFields := []string{
		"whisper_url",
		"whisper_timeout",
		"max_retries",
		"retry_delay",
		"circuit_breaker_failure_threshold",
		"circuit_breaker_reset_timeout",
		"max_audio_file_size",
		"max_audio_duration",
		"supported_formats",
		"concurrent_workers",
		"enable_caching",
		"cache_ttl",
		"enable_graceful_degradation",
		"fallback_language",
		"enable_verbose_logging",
	}

	for _, field := range expectedFields {
		found := false
		// Check if field name exists in JSON string
		for i := 0; i <= len(jsonStr)-len(field); i++ {
			if jsonStr[i:i+len(field)] == field {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected JSON to contain field '%s', but it was not found", field)
		}
	}
}

// TestJSONDeserialization tests that AudioConfig can be deserialized from JSON
func TestJSONDeserialization(t *testing.T) {
	jsonData := `{
		"whisper_url": "http://test:8000",
		"whisper_timeout": 600,
		"max_retries": 5,
		"retry_delay": 3,
		"circuit_breaker_failure_threshold": 10,
		"circuit_breaker_reset_timeout": 120,
		"max_audio_file_size": 1048576,
		"max_audio_duration": 1800,
		"supported_formats": [".mp3", ".wav"],
		"concurrent_workers": 4,
		"enable_caching": true,
		"cache_ttl": 7200,
		"enable_graceful_degradation": false,
		"fallback_language": "es",
		"enable_verbose_logging": true
	}`

	var config AudioConfig
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON to config: %v", err)
	}

	// Verify all fields were deserialized correctly
	if config.WhisperURL != "http://test:8000" {
		t.Errorf("Expected WhisperURL 'http://test:8000', got '%s'", config.WhisperURL)
	}
	if config.WhisperTimeout != 600 {
		t.Errorf("Expected WhisperTimeout 600, got %d", config.WhisperTimeout)
	}
	if config.MaxRetries != 5 {
		t.Errorf("Expected MaxRetries 5, got %d", config.MaxRetries)
	}
	if config.RetryDelay != 3 {
		t.Errorf("Expected RetryDelay 3, got %d", config.RetryDelay)
	}
	if config.CircuitBreakerFailureThreshold != 10 {
		t.Errorf("Expected CircuitBreakerFailureThreshold 10, got %d", config.CircuitBreakerFailureThreshold)
	}
	if config.CircuitBreakerResetTimeout != 120 {
		t.Errorf("Expected CircuitBreakerResetTimeout 120, got %d", config.CircuitBreakerResetTimeout)
	}
	if config.MaxAudioFileSize != 1048576 {
		t.Errorf("Expected MaxAudioFileSize 1048576, got %d", config.MaxAudioFileSize)
	}
	if config.MaxAudioDuration != 1800 {
		t.Errorf("Expected MaxAudioDuration 1800, got %d", config.MaxAudioDuration)
	}
	if len(config.SupportedFormats) != 2 || config.SupportedFormats[0] != ".mp3" || config.SupportedFormats[1] != ".wav" {
		t.Errorf("Expected SupportedFormats [.mp3 .wav], got %v", config.SupportedFormats)
	}
	if config.ConcurrentWorkers != 4 {
		t.Errorf("Expected ConcurrentWorkers 4, got %d", config.ConcurrentWorkers)
	}
	if !config.EnableCaching {
		t.Error("Expected EnableCaching true, got false")
	}
	if config.CacheTTL != 7200 {
		t.Errorf("Expected CacheTTL 7200, got %d", config.CacheTTL)
	}
	if config.EnableGracefulDegradation {
		t.Error("Expected EnableGracefulDegradation false, got true")
	}
	if config.FallbackLanguage != "es" {
		t.Errorf("Expected FallbackLanguage 'es', got '%s'", config.FallbackLanguage)
	}
	if !config.EnableVerboseLogging {
		t.Error("Expected EnableVerboseLogging true, got false")
	}

	// Verify the deserialized config is valid
	if err := config.Validate(); err != nil {
		t.Errorf("Deserialized config should be valid, got error: %v", err)
	}
}

// TestJSONRoundTrip tests that config can be serialized and deserialized without data loss
func TestJSONRoundTrip(t *testing.T) {
	original := DefaultAudioConfig()

	// Serialize
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Deserialize
	var restored AudioConfig
	err = json.Unmarshal(jsonData, &restored)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Compare
	if original.WhisperURL != restored.WhisperURL {
		t.Errorf("WhisperURL mismatch: %s != %s", original.WhisperURL, restored.WhisperURL)
	}
	if original.WhisperTimeout != restored.WhisperTimeout {
		t.Errorf("WhisperTimeout mismatch: %d != %d", original.WhisperTimeout, restored.WhisperTimeout)
	}
	if original.MaxRetries != restored.MaxRetries {
		t.Errorf("MaxRetries mismatch: %d != %d", original.MaxRetries, restored.MaxRetries)
	}
	if original.RetryDelay != restored.RetryDelay {
		t.Errorf("RetryDelay mismatch: %d != %d", original.RetryDelay, restored.RetryDelay)
	}
	if original.ConcurrentWorkers != restored.ConcurrentWorkers {
		t.Errorf("ConcurrentWorkers mismatch: %d != %d", original.ConcurrentWorkers, restored.ConcurrentWorkers)
	}
	if original.EnableCaching != restored.EnableCaching {
		t.Errorf("EnableCaching mismatch: %v != %v", original.EnableCaching, restored.EnableCaching)
	}
	if original.CacheTTL != restored.CacheTTL {
		t.Errorf("CacheTTL mismatch: %d != %d", original.CacheTTL, restored.CacheTTL)
	}
	if original.FallbackLanguage != restored.FallbackLanguage {
		t.Errorf("FallbackLanguage mismatch: %s != %s", original.FallbackLanguage, restored.FallbackLanguage)
	}

	// Verify restored config is still valid
	if err := restored.Validate(); err != nil {
		t.Errorf("Restored config should be valid, got error: %v", err)
	}
}

// TestConfigExportedFields verifies all fields are exported (public)
func TestConfigExportedFields(t *testing.T) {
	config := DefaultAudioConfig()

	// All these field accesses should compile - if they don't, fields are not exported
	_ = config.WhisperURL
	_ = config.WhisperTimeout
	_ = config.MaxRetries
	_ = config.RetryDelay
	_ = config.CircuitBreakerFailureThreshold
	_ = config.CircuitBreakerResetTimeout
	_ = config.MaxAudioFileSize
	_ = config.MaxAudioDuration
	_ = config.SupportedFormats
	_ = config.ConcurrentWorkers
	_ = config.EnableCaching
	_ = config.CacheTTL
	_ = config.EnableGracefulDegradation
	_ = config.FallbackLanguage
	_ = config.EnableVerboseLogging

	// Test passes if compilation succeeds
	t.Log("All fields are properly exported")
}
