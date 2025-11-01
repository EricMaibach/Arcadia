package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Level != LevelInfo {
		t.Errorf("Expected default level to be info, got %s", config.Level)
	}
	if config.Format != FormatJSON {
		t.Errorf("Expected default format to be json, got %s", config.Format)
	}
	if !config.AddSource {
		t.Error("Expected AddSource to be true by default")
	}
	if !config.AddTimestamp {
		t.Error("Expected AddTimestamp to be true by default")
	}
	if len(config.OutputPaths) != 1 || config.OutputPaths[0] != "stdout" {
		t.Errorf("Expected default output to be [stdout], got %v", config.OutputPaths)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "valid config",
			config: Config{
				Level:       LevelInfo,
				OutputPaths: []string{"stdout"},
				Format:      FormatJSON,
			},
			expectError: false,
		},
		{
			name: "invalid level",
			config: Config{
				Level:       "invalid",
				OutputPaths: []string{"stdout"},
				Format:      FormatJSON,
			},
			expectError: true,
		},
		{
			name: "invalid format",
			config: Config{
				Level:       LevelInfo,
				OutputPaths: []string{"stdout"},
				Format:      "xml",
			},
			expectError: true,
		},
		{
			name: "no output paths",
			config: Config{
				Level:       LevelInfo,
				OutputPaths: []string{},
				Format:      FormatJSON,
			},
			expectError: true,
		},
		{
			name: "empty output path",
			config: Config{
				Level:       LevelInfo,
				OutputPaths: []string{"stdout", "  "},
				Format:      FormatJSON,
			},
			expectError: true,
		},
		{
			name: "negative MaxSize",
			config: Config{
				Level:       LevelInfo,
				OutputPaths: []string{"stdout"},
				Format:      FormatJSON,
				MaxSize:     -1,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	config := Config{
		Level:       LevelDebug,
		OutputPaths: []string{"stdout"},
		Format:      FormatJSON,
		AddSource:   true,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	if logger == nil {
		t.Fatal("Logger should not be nil")
	}
}

func TestNewLoggerInvalidConfig(t *testing.T) {
	config := Config{
		Level:       "invalid",
		OutputPaths: []string{"stdout"},
		Format:      FormatJSON,
	}

	_, err := NewLogger(config)
	if err == nil {
		t.Error("Expected error when creating logger with invalid config")
	}
}

func TestLogLevels(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer

	// Create a temporary file for testing
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	// We'll write to the temp file
	config := Config{
		Level:       LevelDebug,
		OutputPaths: []string{tmpFile},
		Format:      FormatJSON,
		AddSource:   false, // Disable source for simpler parsing
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()

	// Test all log levels
	logger.Debug(ctx, "debug message", "key", "value")
	logger.Info(ctx, "info message", "key", "value")
	logger.Warn(ctx, "warn message", "key", "value")
	logger.Error(ctx, "error message", "key", "value")

	// Read the log file
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logOutput := string(content)

	// Verify all levels were logged
	if !strings.Contains(logOutput, "debug message") {
		t.Error("Debug message not found in logs")
	}
	if !strings.Contains(logOutput, "info message") {
		t.Error("Info message not found in logs")
	}
	if !strings.Contains(logOutput, "warn message") {
		t.Error("Warn message not found in logs")
	}
	if !strings.Contains(logOutput, "error message") {
		t.Error("Error message not found in logs")
	}

	// Verify structured fields
	if !strings.Contains(logOutput, "key") {
		t.Error("Structured field 'key' not found in logs")
	}
	if !strings.Contains(logOutput, "value") {
		t.Error("Structured field value not found in logs")
	}

	_ = buf // Silence unused warning
}

func TestStructuredFields(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	config := Config{
		Level:       LevelInfo,
		OutputPaths: []string{tmpFile},
		Format:      FormatJSON,
		AddSource:   false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()

	// Log with multiple fields
	logger.Info(ctx, "test message",
		"string_field", "value",
		"int_field", 42,
		"bool_field", true,
		"float_field", 3.14,
	)

	// Read and parse JSON log
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	// Verify fields
	if logEntry["string_field"] != "value" {
		t.Errorf("Expected string_field to be 'value', got %v", logEntry["string_field"])
	}
	if logEntry["int_field"].(float64) != 42 {
		t.Errorf("Expected int_field to be 42, got %v", logEntry["int_field"])
	}
	if logEntry["bool_field"] != true {
		t.Errorf("Expected bool_field to be true, got %v", logEntry["bool_field"])
	}
	if logEntry["float_field"].(float64) != 3.14 {
		t.Errorf("Expected float_field to be 3.14, got %v", logEntry["float_field"])
	}
}

func TestWithFields(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	config := Config{
		Level:       LevelInfo,
		OutputPaths: []string{tmpFile},
		Format:      FormatJSON,
		AddSource:   false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()

	// Create logger with persistent fields
	loggerWithFields := logger.WithFields(map[string]interface{}{
		"request_id": "123",
		"user_id":    "456",
	})

	loggerWithFields.Info(ctx, "test message", "extra", "field")

	// Read and parse JSON log
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	// Verify persistent fields
	if logEntry["request_id"] != "123" {
		t.Errorf("Expected request_id to be '123', got %v", logEntry["request_id"])
	}
	if logEntry["user_id"] != "456" {
		t.Errorf("Expected user_id to be '456', got %v", logEntry["user_id"])
	}
	if logEntry["extra"] != "field" {
		t.Errorf("Expected extra to be 'field', got %v", logEntry["extra"])
	}
}

func TestModuleComponent(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	config := Config{
		Level:         LevelInfo,
		OutputPaths:   []string{tmpFile},
		Format:        FormatJSON,
		AddSource:     false,
		ModuleName:    "testmodule",
		ComponentName: "testcomponent",
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()
	logger.Info(ctx, "test message")

	// Read and parse JSON log
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	// Verify module and component
	if logEntry["module"] != "testmodule" {
		t.Errorf("Expected module to be 'testmodule', got %v", logEntry["module"])
	}
	if logEntry["component"] != "testcomponent" {
		t.Errorf("Expected component to be 'testcomponent', got %v", logEntry["component"])
	}
}

func TestWithModule(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	config := Config{
		Level:       LevelInfo,
		OutputPaths: []string{tmpFile},
		Format:      FormatJSON,
		AddSource:   false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()
	moduleLogger := logger.WithModule("ai")
	moduleLogger.Info(ctx, "test message")

	// Read and parse JSON log
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if logEntry["module"] != "ai" {
		t.Errorf("Expected module to be 'ai', got %v", logEntry["module"])
	}
}

func TestWithComponent(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	config := Config{
		Level:       LevelInfo,
		OutputPaths: []string{tmpFile},
		Format:      FormatJSON,
		AddSource:   false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()
	componentLogger := logger.WithComponent("database")
	componentLogger.Info(ctx, "test message")

	// Read and parse JSON log
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if logEntry["component"] != "database" {
		t.Errorf("Expected component to be 'database', got %v", logEntry["component"])
	}
}

func TestSourceLocation(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	config := Config{
		Level:       LevelInfo,
		OutputPaths: []string{tmpFile},
		Format:      FormatJSON,
		AddSource:   true,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()
	logger.Info(ctx, "test message")

	// Read and parse JSON log
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	// Verify source field exists
	source, ok := logEntry["source"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected source field in log entry")
	}

	// Verify file is just basename
	file, ok := source["file"].(string)
	if !ok {
		t.Fatal("Expected file field in source")
	}
	if strings.Contains(file, "/") {
		t.Errorf("Expected basename only, got full path: %s", file)
	}
	// Source should point to logger_test.go since that's where the Info() call happens
	if file != "logger_test.go" {
		t.Errorf("Expected file to be logger_test.go, got %s", file)
	}

	// Verify function exists
	if _, ok := source["function"].(string); !ok {
		t.Error("Expected function field in source")
	}

	// Verify line exists
	if _, ok := source["line"].(float64); !ok {
		t.Error("Expected line field in source")
	}
}

func TestMultiOutput(t *testing.T) {
	tmpFile1 := filepath.Join(t.TempDir(), "test1.log")
	tmpFile2 := filepath.Join(t.TempDir(), "test2.log")

	config := Config{
		Level:       LevelInfo,
		OutputPaths: []string{tmpFile1, tmpFile2},
		Format:      FormatJSON,
		AddSource:   false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()
	testMsg := "multi output test"
	logger.Info(ctx, testMsg)

	// Verify both files have the log
	for _, file := range []string{tmpFile1, tmpFile2} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("Failed to read log file %s: %v", file, err)
		}
		if !strings.Contains(string(content), testMsg) {
			t.Errorf("Log file %s does not contain expected message", file)
		}
	}
}

func TestTextFormat(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	config := Config{
		Level:       LevelInfo,
		OutputPaths: []string{tmpFile},
		Format:      FormatText,
		AddSource:   false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()
	logger.Info(ctx, "test message", "key", "value")

	// Read log file
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logOutput := string(content)

	// Text format should contain message and fields
	if !strings.Contains(logOutput, "test message") {
		t.Error("Log output does not contain message")
	}
	if !strings.Contains(logOutput, "key") || !strings.Contains(logOutput, "value") {
		t.Error("Log output does not contain structured fields")
	}
}

func TestErrorFieldHandling(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	config := Config{
		Level:       LevelInfo,
		OutputPaths: []string{tmpFile},
		Format:      FormatJSON,
		AddSource:   false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()
	testErr := os.ErrNotExist
	logger.Error(ctx, "error occurred", "error", testErr)

	// Read and parse JSON log
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	// Verify error is serialized as string
	errorField, ok := logEntry["error"].(string)
	if !ok {
		t.Fatal("Expected error field to be string")
	}
	if !strings.Contains(errorField, "does not exist") {
		t.Errorf("Error message not properly serialized: %s", errorField)
	}
}

func TestOddNumberOfFields(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	config := Config{
		Level:       LevelInfo,
		OutputPaths: []string{tmpFile},
		Format:      FormatJSON,
		AddSource:   false,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.Background()
	// Odd number of fields - should handle gracefully
	logger.Info(ctx, "test message", "key1", "value1", "key2")

	// Should not panic and file should exist
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}
}
