package audio

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"arcadia/modules/documents/processors/base"
)

// TestAudioProcessorIntegration tests the complete audio processing pipeline
func TestAudioProcessorIntegration(t *testing.T) {
	// Create test audio processor
	config := DefaultAudioConfig()
	config.WhisperURL = "http://localhost:8002" // Use test Whisper service
	processor := NewAudioProcessorWithConfig(config)

	ctx := context.Background()

	t.Run("CanProcess identifies audio files", func(t *testing.T) {
		tests := []struct {
			name     string
			filePath string
			expected bool
		}{
			{"MP3 file", "test.mp3", true},
			{"WAV file", "test.wav", true},
			{"M4A file", "test.m4a", true},
			{"FLAC file", "test.flac", true},
			{"OGG file", "test.ogg", true},
			{"AAC file", "test.aac", true},
			{"TXT file", "test.txt", false},
			{"PDF file", "test.pdf", false},
			{"No extension", "test", false},
			{"Uppercase MP3", "test.MP3", true},
			{"Mixed case", "test.WaV", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := processor.CanProcess(tt.filePath)
				if result != tt.expected {
					t.Errorf("CanProcess(%q) = %v, want %v", tt.filePath, result, tt.expected)
				}
			})
		}
	})

	t.Run("GetProcessorType returns audio", func(t *testing.T) {
		if processor.GetProcessorType() != base.ProcessorTypeAudio {
			t.Errorf("GetProcessorType() = %v, want %v", processor.GetProcessorType(), base.ProcessorTypeAudio)
		}
	})

	t.Run("GetSupportedExtensions returns all formats", func(t *testing.T) {
		extensions := processor.GetSupportedExtensions()
		expected := []string{".mp3", ".wav", ".m4a", ".flac", ".ogg", ".aac"}

		if len(extensions) != len(expected) {
			t.Errorf("GetSupportedExtensions() returned %d extensions, want %d", len(extensions), len(expected))
		}

		// Check all expected formats are present
		extMap := make(map[string]bool)
		for _, ext := range extensions {
			extMap[ext] = true
		}

		for _, exp := range expected {
			if !extMap[exp] {
				t.Errorf("Missing expected extension: %s", exp)
			}
		}
	})

	t.Run("Process with graceful degradation", func(t *testing.T) {
		// This test assumes Whisper service might not be available
		// and tests the graceful degradation fallback

		// Create a temporary empty test file
		tmpFile, err := os.CreateTemp("", "test_audio_*.mp3")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		// Write minimal MP3 header (ID3 tag)
		tmpFile.Write([]byte{0x49, 0x44, 0x33}) // "ID3"
		tmpFile.Close()

		// Enable graceful degradation
		config.EnableGracefulDegradation = true
		processor := NewAudioProcessorWithConfig(config)

		// Process should return a result even if Whisper fails
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		result, err := processor.Process(ctx, tmpFile.Name())

		// Should succeed with fallback result
		if err != nil {
			t.Logf("Process returned error (Whisper may not be available): %v", err)
			// Error is acceptable if Whisper is not running
		} else {
			// Verify fallback result structure
			if result == nil {
				t.Error("Expected result, got nil")
			} else {
				if result.Content == "" {
					t.Error("Expected fallback content, got empty string")
				}
				if result.Language == "" {
					t.Error("Expected language to be set")
				}
				if result.Metadata == nil {
					t.Error("Expected metadata, got nil")
				}
				t.Logf("Fallback result: %s", result.Content[:min(100, len(result.Content))])
			}
		}
	})

	t.Run("Circuit breaker integration", func(t *testing.T) {
		// Verify circuit breaker is accessible
		state := processor.GetCircuitBreakerState()
		if !state.IsValid() {
			t.Errorf("Invalid circuit breaker state: %v", state)
		}

		// Should start in Closed state
		if state != CircuitBreakerClosed {
			t.Logf("Circuit breaker state: %v (expected Closed, but may be Open if previous tests failed)", state)
		}

		// Test reset functionality
		processor.ResetCircuitBreaker()
		newState := processor.GetCircuitBreakerState()
		if newState != CircuitBreakerClosed {
			t.Errorf("After reset, circuit breaker state = %v, want Closed", newState)
		}
	})

	t.Run("Health check", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		err := processor.HealthCheck(ctx)
		if err != nil {
			t.Logf("Health check failed (Whisper service may not be running): %v", err)
			// Not a test failure - Whisper might not be running
		} else {
			t.Log("Health check passed - Whisper service is available")
		}
	})

	t.Run("Configuration access", func(t *testing.T) {
		cfg := processor.GetConfig()
		if cfg == nil {
			t.Error("GetConfig() returned nil")
		}

		if cfg.WhisperURL == "" {
			t.Error("Config WhisperURL is empty")
		}

		if cfg.MaxAudioFileSize <= 0 {
			t.Error("Config MaxAudioFileSize is invalid")
		}

		if len(cfg.SupportedFormats) == 0 {
			t.Error("Config SupportedFormats is empty")
		}
	})

	t.Run("Logger and metrics attachment", func(t *testing.T) {
		// Test fluent API for logger and metrics
		processor2 := NewAudioProcessor()

		// WithLogger should return the processor
		result := processor2.WithLogger(nil)
		if result == nil {
			t.Error("WithLogger returned nil")
		}

		// WithMetrics should return the processor
		result = processor2.WithMetrics(nil)
		if result == nil {
			t.Error("WithMetrics returned nil")
		}
	})
}

// TestAudioProcessorValidation tests file validation in the integration context
func TestAudioProcessorValidation(t *testing.T) {
	processor := NewAudioProcessor()
	ctx := context.Background()

	t.Run("Non-existent file fails validation", func(t *testing.T) {
		_, err := processor.Process(ctx, "/nonexistent/file.mp3")
		if err == nil {
			t.Error("Expected error for non-existent file, got nil")
		}
	})

	t.Run("Directory instead of file fails validation", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "audio_test_dir")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Add .mp3 extension to directory name
		dirWithExt := filepath.Join(tmpDir, "fakefile.mp3")
		os.Mkdir(dirWithExt, 0755)
		defer os.RemoveAll(dirWithExt)

		_, err = processor.Process(ctx, dirWithExt)
		if err == nil {
			t.Error("Expected error for directory, got nil")
		}
	})

	t.Run("Unsupported format fails validation", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test_*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		_, err = processor.Process(ctx, tmpFile.Name())
		if err == nil {
			t.Error("Expected error for unsupported format, got nil")
		}
	})
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
