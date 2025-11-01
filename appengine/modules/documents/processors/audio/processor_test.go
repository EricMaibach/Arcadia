package audio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/processors/base"
)

// Mock logger for processor tests
type mockProcessorLogger struct {
	logs       []string
	debugCalls []processorLogCall
	infoCalls  []processorLogCall
	warnCalls  []processorLogCall
	errorCalls []processorLogCall
}

type processorLogCall struct {
	msg    string
	fields []interface{}
}

func newMockProcessorLogger() *mockProcessorLogger {
	return &mockProcessorLogger{
		logs: make([]string, 0),
	}
}

func (m *mockProcessorLogger) Debug(ctx context.Context, msg string, fields ...interface{}) {
	m.debugCalls = append(m.debugCalls, processorLogCall{msg: msg, fields: fields})
	m.logs = append(m.logs, fmt.Sprintf("DEBUG: %s", msg))
}

func (m *mockProcessorLogger) Info(ctx context.Context, msg string, fields ...interface{}) {
	m.infoCalls = append(m.infoCalls, processorLogCall{msg: msg, fields: fields})
	m.logs = append(m.logs, fmt.Sprintf("INFO: %s", msg))
}

func (m *mockProcessorLogger) Warn(ctx context.Context, msg string, fields ...interface{}) {
	m.warnCalls = append(m.warnCalls, processorLogCall{msg: msg, fields: fields})
	m.logs = append(m.logs, fmt.Sprintf("WARN: %s", msg))
}

func (m *mockProcessorLogger) Error(ctx context.Context, msg string, fields ...interface{}) {
	m.errorCalls = append(m.errorCalls, processorLogCall{msg: msg, fields: fields})
	m.logs = append(m.logs, fmt.Sprintf("ERROR: %s", msg))
}

func (m *mockProcessorLogger) Fatal(ctx context.Context, msg string, fields ...interface{}) {
}

func (m *mockProcessorLogger) WithFields(fields map[string]interface{}) interfaces.Logger {
	return m
}

func (m *mockProcessorLogger) WithContext(ctx context.Context) interfaces.Logger {
	return m
}

func (m *mockProcessorLogger) WithModule(module string) interfaces.Logger {
	return m
}

func (m *mockProcessorLogger) WithComponent(component string) interfaces.Logger {
	return m
}

// Mock metrics collector for testing
type mockProcessorMetrics struct {
	counters   map[string]int
	timers     map[string]float64
	gauges     map[string]float64
	histograms map[string]float64
}

func newMockProcessorMetrics() *mockProcessorMetrics {
	return &mockProcessorMetrics{
		counters:   make(map[string]int),
		timers:     make(map[string]float64),
		gauges:     make(map[string]float64),
		histograms: make(map[string]float64),
	}
}

func (m *mockProcessorMetrics) IncrementCounter(name string, tags map[string]string) {
	m.counters[name]++
}

func (m *mockProcessorMetrics) AddToCounter(name string, value float64, tags map[string]string) {
	if _, ok := m.counters[name]; !ok {
		m.counters[name] = 0
	}
	m.counters[name] += int(value)
}

func (m *mockProcessorMetrics) SetGauge(name string, value float64, tags map[string]string) {
	m.gauges[name] = value
}

func (m *mockProcessorMetrics) RecordHistogram(name string, value float64, tags map[string]string) {
	m.histograms[name] = value
}

func (m *mockProcessorMetrics) StartTimer(name string, tags map[string]string) interfaces.Timer {
	return &mockTimer{}
}

func (m *mockProcessorMetrics) RecordTimer(name string, duration float64, tags map[string]string) {
	m.timers[name] = duration
}

func (m *mockProcessorMetrics) RecordCustomMetric(name string, value interface{}, metricType string, tags map[string]string) {
}

type mockTimer struct {
	startTime time.Time
}

func (mt *mockTimer) Stop() float64 {
	return time.Since(mt.startTime).Seconds() * 1000
}

func (mt *mockTimer) Cancel() {
}

// TestAudioProcessorCreation tests processor creation
func TestAudioProcessorCreation(t *testing.T) {
	t.Run("create with default config", func(t *testing.T) {
		processor := NewAudioProcessor()

		if processor == nil {
			t.Fatal("Expected processor to be created")
		}

		if processor.config == nil {
			t.Error("Expected config to be set")
		}

		if processor.validator == nil {
			t.Error("Expected validator to be set")
		}

		if processor.client == nil {
			t.Error("Expected client to be set")
		}
	})

	t.Run("create with custom config", func(t *testing.T) {
		config := DefaultAudioConfig()
		config.WhisperTimeout = 120
		config.MaxRetries = 5

		processor := NewAudioProcessorWithConfig(config)

		if processor == nil {
			t.Fatal("Expected processor to be created")
		}

		if processor.config.WhisperTimeout != 120 {
			t.Errorf("Expected WhisperTimeout=120, got %d", processor.config.WhisperTimeout)
		}

		if processor.config.MaxRetries != 5 {
			t.Errorf("Expected MaxRetries=5, got %d", processor.config.MaxRetries)
		}
	})

	t.Run("create with invalid config falls back to default", func(t *testing.T) {
		config := AudioConfig{
			WhisperURL:     "", // Invalid: empty URL
			WhisperTimeout: -1, // Invalid: negative timeout
		}

		processor := NewAudioProcessorWithConfig(config)

		// Should use default config
		if processor.config.WhisperURL == "" {
			t.Error("Expected default config to be used when validation fails")
		}

		if processor.config.WhisperTimeout <= 0 {
			t.Error("Expected default positive timeout")
		}
	})
}

// TestWithLoggerAndMetrics tests logger and metrics injection
func TestWithLoggerAndMetrics(t *testing.T) {
	processor := NewAudioProcessor()
	logger := newMockProcessorLogger()
	metrics := newMockProcessorMetrics()

	t.Run("with logger", func(t *testing.T) {
		result := processor.WithLogger(logger)

		if result == nil {
			t.Error("Expected processor to be returned")
		}

		if processor.logger == nil {
			t.Error("Expected logger to be set")
		}
	})

	t.Run("with metrics", func(t *testing.T) {
		result := processor.WithMetrics(metrics)

		if result == nil {
			t.Error("Expected processor to be returned")
		}

		if processor.metrics == nil {
			t.Error("Expected metrics to be set")
		}
	})
}

// TestCanProcess tests the CanProcess method
func TestCanProcess(t *testing.T) {
	processor := NewAudioProcessor()

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
		{"Uppercase MP3", "TEST.MP3", true},
		{"Mixed case", "Test.Mp3", true},
		{"Unsupported TXT", "test.txt", false},
		{"Unsupported PDF", "test.pdf", false},
		{"No extension", "testfile", false},
		{"Empty path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.CanProcess(tt.filePath)
			if result != tt.expected {
				t.Errorf("CanProcess(%q) = %v, expected %v", tt.filePath, result, tt.expected)
			}
		})
	}
}

// TestGetSupportedExtensions tests the GetSupportedExtensions method
func TestGetSupportedExtensions(t *testing.T) {
	processor := NewAudioProcessor()
	extensions := processor.GetSupportedExtensions()

	if len(extensions) == 0 {
		t.Error("Expected at least one supported extension")
	}

	// Check for common audio formats
	expectedFormats := []string{".mp3", ".wav", ".m4a", ".flac", ".ogg", ".aac"}
	for _, format := range expectedFormats {
		found := false
		for _, ext := range extensions {
			if ext == format {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected format %s to be supported", format)
		}
	}
}

// TestGetProcessorType tests the GetProcessorType method
func TestGetProcessorType(t *testing.T) {
	processor := NewAudioProcessor()

	processorType := processor.GetProcessorType()

	if processorType != base.ProcessorTypeAudio {
		t.Errorf("Expected ProcessorTypeAudio, got %v", processorType)
	}
}

// TestGetConfig tests the GetConfig method
func TestGetConfig(t *testing.T) {
	processor := NewAudioProcessor()

	config := processor.GetConfig()

	if config == nil {
		t.Error("Expected config to be returned")
	}

	if config.WhisperURL == "" {
		t.Error("Expected WhisperURL to be set")
	}
}

// TestProcessorCircuitBreakerIntegration tests circuit breaker integration
func TestProcessorCircuitBreakerIntegration(t *testing.T) {
	processor := NewAudioProcessor()

	t.Run("initial state is closed", func(t *testing.T) {
		state := processor.GetCircuitBreakerState()
		if state != CircuitBreakerClosed {
			t.Errorf("Expected initial state to be closed, got %v", state)
		}
	})

	t.Run("reset circuit breaker", func(t *testing.T) {
		processor.ResetCircuitBreaker()

		state := processor.GetCircuitBreakerState()
		if state != CircuitBreakerClosed {
			t.Errorf("Expected state to be closed after reset, got %v", state)
		}
	})
}

// TestProcessorHealthCheck tests the HealthCheck method
func TestProcessorHealthCheck(t *testing.T) {
	config := DefaultAudioConfig()
	config.WhisperURL = "http://invalid-whisper-service:9999"
	processor := NewAudioProcessorWithConfig(config)

	t.Run("health check fails when service unavailable", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := processor.HealthCheck(ctx)
		if err == nil {
			t.Error("Expected health check to fail when service is unavailable")
		}
	})

	t.Run("health check returns circuit breaker state", func(t *testing.T) {
		// Create a new processor instance for this test
		localConfig := DefaultAudioConfig()
		localConfig.WhisperURL = "http://invalid-whisper-service:9999"
		localProcessor := NewAudioProcessorWithConfig(localConfig)

		// Simulate circuit breaker being open by directly accessing it
		localProcessor.client.circuitBreaker.state = CircuitBreakerOpen
		localProcessor.client.circuitBreaker.lastStateChangeTime = time.Now()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		err := localProcessor.HealthCheck(ctx)

		if err == nil {
			t.Error("Expected health check to fail when circuit breaker is open")
		}

		// The HealthCheck first checks Whisper service, then circuit breaker
		// Since both will fail, we just verify an error was returned
		if err == nil {
			t.Error("Expected health check to fail")
		}

		// Verify circuit breaker state is open
		state := localProcessor.GetCircuitBreakerState()
		if state != CircuitBreakerOpen {
			t.Errorf("Expected circuit breaker to be open, got: %v", state)
		}
	})
}

// TestProcessValidationFailure tests processing with validation errors
func TestProcessValidationFailure(t *testing.T) {
	processor := NewAudioProcessor()
	logger := newMockProcessorLogger()
	metrics := newMockProcessorMetrics()
	processor.WithLogger(logger).WithMetrics(metrics)

	t.Run("non-existent file", func(t *testing.T) {
		ctx := context.Background()
		result, err := processor.Process(ctx, "/nonexistent/file.mp3")

		if err == nil {
			t.Error("Expected error for non-existent file")
		}

		if result != nil {
			t.Error("Expected nil result for validation failure")
		}

		// Check metrics
		if metrics.counters["audio.processing.failed"] != 1 {
			t.Errorf("Expected failed counter to be incremented")
		}
	})

	t.Run("unsupported format", func(t *testing.T) {
		// Create a temporary text file
		tmpDir := t.TempDir()
		txtFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(txtFile, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		ctx := context.Background()
		result, err := processor.Process(ctx, txtFile)

		if err == nil {
			t.Error("Expected error for unsupported format")
		}

		if result != nil {
			t.Error("Expected nil result for validation failure")
		}
	})

	t.Run("directory instead of file", func(t *testing.T) {
		tmpDir := t.TempDir()

		ctx := context.Background()
		result, err := processor.Process(ctx, tmpDir)

		if err == nil {
			t.Error("Expected error for directory")
		}

		if result != nil {
			t.Error("Expected nil result for validation failure")
		}
	})
}

// TestProcessContextTimeout tests context timeout handling
func TestProcessContextTimeout(t *testing.T) {
	config := DefaultAudioConfig()
	config.WhisperTimeout = 1 // 1 second timeout
	config.MaxRetries = 0     // No retries
	config.WhisperURL = "http://slow-service:8002"

	processor := NewAudioProcessorWithConfig(config)

	// Create a valid MP3 file
	tmpDir := t.TempDir()
	mp3File := filepath.Join(tmpDir, "test.mp3")

	// Create minimal valid MP3 header
	mp3Header := []byte{0xFF, 0xFB, 0x90, 0x00}
	if err := os.WriteFile(mp3File, mp3Header, 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("context timeout is respected", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		result, err := processor.Process(ctx, mp3File)
		duration := time.Since(start)

		// Since graceful degradation is enabled by default, we might get a result
		// The key is that it should respect the timeout and not hang
		if err == nil && result == nil {
			t.Error("Expected either error or result (with graceful degradation)")
		}

		// Should fail/complete relatively quickly (within timeout + small buffer)
		if duration > 5*time.Second {
			t.Errorf("Process took too long: %v", duration)
		}

		// If graceful degradation is enabled and we got a result, it should be fallback
		if result != nil && processor.config.EnableGracefulDegradation {
			if result.Confidence >= 0.5 {
				t.Error("Expected low confidence for fallback result")
			}
		}
	})
}

// TestFallbackResult tests fallback result creation
func TestFallbackResult(t *testing.T) {
	config := DefaultAudioConfig()
	config.EnableGracefulDegradation = true
	config.FallbackLanguage = "en"

	processor := NewAudioProcessorWithConfig(config)

	fileInfo := &AudioFileInfo{
		Path:      "/test/audio.mp3",
		Size:      1024,
		Extension: ".mp3",
		ModTime:   time.Now(),
	}

	testErr := fmt.Errorf("transcription service unavailable")

	t.Run("creates valid fallback result", func(t *testing.T) {
		result, err := processor.createFallbackResult("/test/audio.mp3", fileInfo, testErr)

		if err != nil {
			t.Errorf("Expected no error creating fallback, got: %v", err)
		}

		if result == nil {
			t.Fatal("Expected non-nil fallback result")
		}

		if result.Content == "" {
			t.Error("Expected fallback content to be set")
		}

		if result.Language != "en" {
			t.Errorf("Expected fallback language 'en', got %s", result.Language)
		}

		if result.Confidence >= 0.5 {
			t.Errorf("Expected low confidence for fallback, got %f", result.Confidence)
		}

		if result.ContentType != "text/plain" {
			t.Errorf("Expected content type 'text/plain', got %s", result.ContentType)
		}
	})

	t.Run("fallback metadata includes error", func(t *testing.T) {
		result, err := processor.createFallbackResult("/test/audio.mp3", fileInfo, testErr)

		if err != nil {
			t.Fatal(err)
		}

		metadata := result.Metadata

		if metadata["fallback_mode"] != true {
			t.Error("Expected fallback_mode to be true")
		}

		if metadata["transcription_error"] == nil {
			t.Error("Expected transcription_error in metadata")
		}

		if metadata["original_format"] != ".mp3" {
			t.Errorf("Expected original_format '.mp3', got %v", metadata["original_format"])
		}

		if metadata["processor_type"] != "audio" {
			t.Error("Expected processor_type to be 'audio'")
		}
	})

	t.Run("fallback content is searchable", func(t *testing.T) {
		result, err := processor.createFallbackResult("/test/audio.mp3", fileInfo, testErr)

		if err != nil {
			t.Fatal(err)
		}

		// Content should contain useful information
		content := result.Content
		if content == "" {
			t.Error("Expected non-empty content")
		}

		// Should mention it's an audio file
		if !contains(content, "Audio") && !contains(content, "audio") {
			t.Error("Expected content to mention 'audio'")
		}

		// Should include file name
		if !contains(content, "audio.mp3") {
			t.Error("Expected content to include file name")
		}
	})
}

// TestSuccessResult tests success result creation
func TestSuccessResult(t *testing.T) {
	processor := NewAudioProcessor()

	fileInfo := &AudioFileInfo{
		Path:      "/test/audio.mp3",
		Size:      2048,
		Extension: ".mp3",
		ModTime:   time.Now(),
	}

	transcription := &TranscriptionResult{
		Text:               "This is a test transcription with multiple words.",
		Language:           "en",
		LanguageConfidence: 0.95,
		Duration:           45.5,
		WordCount:          9,
		ProcessingTime:     2 * time.Second,
		TranscribedAt:      time.Now(),
	}

	t.Run("creates valid success result", func(t *testing.T) {
		result := processor.createSuccessResult("/test/audio.mp3", fileInfo, transcription)

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if result.Content != transcription.Text {
			t.Errorf("Expected content to be transcription text")
		}

		if result.Language != "en" {
			t.Errorf("Expected language 'en', got %s", result.Language)
		}

		if result.Confidence < 0.8 {
			t.Errorf("Expected high confidence, got %f", result.Confidence)
		}

		if result.ContentType != "text/plain" {
			t.Errorf("Expected content type 'text/plain', got %s", result.ContentType)
		}
	})

	t.Run("metadata includes all required fields", func(t *testing.T) {
		result := processor.createSuccessResult("/test/audio.mp3", fileInfo, transcription)

		metadata := result.Metadata

		requiredFields := []string{
			"original_format",
			"original_size",
			"file_name",
			"file_path",
			"transcription_language",
			"transcription_duration",
			"word_count",
			"processed_at",
			"processor_version",
			"whisper_url",
			"whisper_model",
			"language_confidence",
			"audio_duration",
			"transcribed_at",
			"processor_type",
		}

		for _, field := range requiredFields {
			if metadata[field] == nil {
				t.Errorf("Expected metadata field '%s' to be set", field)
			}
		}

		// Verify specific values
		if metadata["processor_type"] != "audio" {
			t.Error("Expected processor_type to be 'audio'")
		}

		if metadata["word_count"] != 9 {
			t.Errorf("Expected word_count 9, got %v", metadata["word_count"])
		}

		if metadata["transcription_language"] != "en" {
			t.Errorf("Expected transcription_language 'en', got %v", metadata["transcription_language"])
		}
	})

	t.Run("metadata count is sufficient", func(t *testing.T) {
		result := processor.createSuccessResult("/test/audio.mp3", fileInfo, transcription)

		// Should have at least 15 metadata fields
		if len(result.Metadata) < 15 {
			t.Errorf("Expected at least 15 metadata fields, got %d", len(result.Metadata))
		}
	})
}

// TestInterfaceCompliance tests that AudioProcessor implements DocumentProcessor
func TestInterfaceCompliance(t *testing.T) {
	var _ base.DocumentProcessor = (*AudioProcessor)(nil)

	processor := NewAudioProcessor()

	// Test all interface methods are callable
	t.Run("CanProcess is callable", func(t *testing.T) {
		_ = processor.CanProcess("test.mp3")
	})

	t.Run("GetSupportedExtensions is callable", func(t *testing.T) {
		_ = processor.GetSupportedExtensions()
	})

	t.Run("GetProcessorType is callable", func(t *testing.T) {
		_ = processor.GetProcessorType()
	})

	t.Run("Process is callable", func(t *testing.T) {
		ctx := context.Background()
		_, _ = processor.Process(ctx, "/nonexistent/test.mp3")
	})
}

// TestProcessorWithLogger tests processor behavior with logger
func TestProcessorWithLogger(t *testing.T) {
	processor := NewAudioProcessor()
	logger := newMockProcessorLogger()
	processor.WithLogger(logger)

	// Create a test file
	tmpDir := t.TempDir()
	mp3File := filepath.Join(tmpDir, "test.mp3")
	mp3Header := []byte{0xFF, 0xFB, 0x90, 0x00}
	if err := os.WriteFile(mp3File, mp3Header, 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("logs are created during processing", func(t *testing.T) {
		ctx := context.Background()
		_, _ = processor.Process(ctx, mp3File)

		if len(logger.logs) == 0 {
			t.Error("Expected logs to be created")
		}

		// Should have start log
		hasStartLog := false
		for _, log := range logger.logs {
			if contains(log, "Starting audio processing") {
				hasStartLog = true
				break
			}
		}

		if !hasStartLog {
			t.Error("Expected start log message")
		}
	})
}

// TestProcessorWithMetrics tests processor behavior with metrics
func TestProcessorWithMetrics(t *testing.T) {
	processor := NewAudioProcessor()
	metrics := newMockProcessorMetrics()
	processor.WithMetrics(metrics)

	// Create a test file
	tmpDir := t.TempDir()
	mp3File := filepath.Join(tmpDir, "test.mp3")
	mp3Header := []byte{0xFF, 0xFB, 0x90, 0x00}
	if err := os.WriteFile(mp3File, mp3Header, 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("metrics are recorded during processing", func(t *testing.T) {
		ctx := context.Background()
		_, _ = processor.Process(ctx, mp3File)

		// Should record processing duration
		if _, ok := metrics.timers["audio.processing.duration"]; !ok {
			t.Error("Expected processing duration timer to be recorded")
		}

		// Should increment some counter (either success, failed, or fallback)
		totalCounters := metrics.counters["audio.processing.failed"] +
			metrics.counters["audio.processing.success"] +
			metrics.counters["audio.processing.fallback"]

		if totalCounters < 1 {
			t.Errorf("Expected at least one counter to be incremented, got: %+v", metrics.counters)
		}
	})
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
