package audio

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// Mock logger for testing
type mockLogger struct {
	warnCalls []mockLogCall
	infoCalls []mockLogCall
}

type mockLogCall struct {
	msg    string
	fields []interface{}
}

func (m *mockLogger) Debug(ctx context.Context, msg string, fields ...interface{}) {
}

func (m *mockLogger) Info(ctx context.Context, msg string, fields ...interface{}) {
	m.infoCalls = append(m.infoCalls, mockLogCall{msg: msg, fields: fields})
}

func (m *mockLogger) Warn(ctx context.Context, msg string, fields ...interface{}) {
	m.warnCalls = append(m.warnCalls, mockLogCall{msg: msg, fields: fields})
}

func (m *mockLogger) Error(ctx context.Context, msg string, fields ...interface{}) {
}

func (m *mockLogger) Fatal(ctx context.Context, msg string, fields ...interface{}) {
}

func (m *mockLogger) WithFields(fields map[string]interface{}) interfaces.Logger {
	return m
}

func (m *mockLogger) WithContext(ctx context.Context) interfaces.Logger {
	return m
}

func (m *mockLogger) WithModule(module string) interfaces.Logger {
	return m
}

func (m *mockLogger) WithComponent(component string) interfaces.Logger {
	return m
}

// TestNewAudioValidator tests validator construction
func TestNewAudioValidator(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)

	if validator == nil {
		t.Fatal("Expected validator to be created, got nil")
	}

	if validator.config == nil {
		t.Error("Expected config to be set")
	}

	if validator.logger != nil {
		t.Error("Expected logger to be nil initially")
	}
}

// TestWithLogger tests the fluent logger API
func TestWithLogger(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)
	logger := &mockLogger{}

	result := validator.WithLogger(logger)

	if result != validator {
		t.Error("Expected WithLogger to return the same validator instance")
	}

	if validator.logger != logger {
		t.Error("Expected logger to be set")
	}
}

// TestValidatePathTraversalAttacks tests path traversal protection (CRITICAL SECURITY)
func TestValidatePathTraversalAttacks(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)

	tests := []struct {
		name        string
		path        string
		description string
	}{
		{
			name:        "Simple parent directory",
			path:        "../etc/passwd",
			description: "Path traversal using single ../",
		},
		{
			name:        "Multiple parent directories",
			path:        "../../sensitive/file.mp3",
			description: "Path traversal using multiple ../",
		},
		{
			name:        "Deep traversal",
			path:        "../../../../../../../etc/passwd",
			description: "Deep path traversal attempt",
		},
		{
			name:        "Parent in middle of path",
			path:        "/tmp/../etc/passwd",
			description: "Path traversal in middle of path",
		},
		{
			name:        "Parent at end",
			path:        "/tmp/audio/..",
			description: "Path traversal at end of path",
		},
		{
			name:        "Multiple dots scattered",
			path:        "/tmp/../audio/../file.mp3",
			description: "Multiple .. sequences in path",
		},
		{
			name:        "Hidden traversal",
			path:        "/tmp/./audio/../../../etc/passwd",
			description: "Hidden traversal with current directory markers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateFile(tt.path)

			if err == nil {
				t.Errorf("Expected path traversal to be blocked for: %s", tt.path)
				return
			}

			docErr, ok := err.(*models.DocumentError)
			if !ok {
				t.Errorf("Expected DocumentError, got: %T", err)
				return
			}

			if docErr.Code != models.ErrInvalidConfig {
				t.Errorf("Expected error code ErrInvalidConfig, got: %s", docErr.Code)
			}

			errMsg := strings.ToLower(err.Error())
			if !strings.Contains(errMsg, "path") && !strings.Contains(errMsg, "traversal") {
				t.Errorf("Expected error message to mention path/traversal, got: %s", err.Error())
			}
		})
	}
}

// TestValidateRestrictedDirectories tests restricted directory access (CRITICAL SECURITY)
func TestValidateRestrictedDirectories(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)

	tests := []struct {
		name string
		path string
	}{
		{"etc directory", "/etc/audio.mp3"},
		{"sys directory", "/sys/audio.mp3"},
		{"proc directory", "/proc/audio.mp3"},
		{"dev directory", "/dev/audio.mp3"},
		{"root directory", "/root/audio.mp3"},
		{"etc subdirectory", "/etc/ssl/audio.mp3"},
		{"sys subdirectory", "/sys/class/audio.mp3"},
		{"proc subdirectory", "/proc/self/audio.mp3"},
		{"dev subdirectory", "/dev/null/audio.mp3"},
		{"root subdirectory", "/root/documents/audio.mp3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateFile(tt.path)

			if err == nil {
				t.Errorf("Expected restricted directory access to be blocked for: %s", tt.path)
				return
			}

			docErr, ok := err.(*models.DocumentError)
			if !ok {
				t.Errorf("Expected DocumentError, got: %T", err)
				return
			}

			if docErr.Code != models.ErrInvalidConfig {
				t.Errorf("Expected error code ErrInvalidConfig, got: %s", docErr.Code)
			}

			errMsg := strings.ToLower(err.Error())
			if !strings.Contains(errMsg, "restricted") {
				t.Errorf("Expected error message to mention 'restricted', got: %s", err.Error())
			}
		})
	}
}

// TestValidateFileNotFound tests non-existent file handling
func TestValidateFileNotFound(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)

	err := validator.ValidateFile("/nonexistent/path/audio.mp3")

	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}

	docErr, ok := err.(*models.DocumentError)
	if !ok {
		t.Fatalf("Expected DocumentError, got: %T", err)
	}

	if docErr.Code != models.ErrDocumentNotFound {
		t.Errorf("Expected error code ErrDocumentNotFound, got: %s", docErr.Code)
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected error message to contain 'not found', got: %s", err.Error())
	}
}

// TestValidateDirectory tests that directories are rejected
func TestValidateDirectory(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)

	// Create a temporary directory
	tmpDir := t.TempDir()

	err := validator.ValidateFile(tmpDir)

	if err == nil {
		t.Fatal("Expected error when validating directory")
	}

	docErr, ok := err.(*models.DocumentError)
	if !ok {
		t.Fatalf("Expected DocumentError, got: %T", err)
	}

	if docErr.Code != models.ErrInvalidConfig {
		t.Errorf("Expected error code ErrInvalidConfig, got: %s", docErr.Code)
	}

	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("Expected error message to mention 'directory', got: %s", err.Error())
	}
}

// TestValidateExtensionSupported tests supported extensions
func TestValidateExtensionSupported(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)

	// Create temporary directory
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		filename  string
		shouldErr bool
	}{
		{"MP3 extension", "audio.mp3", false},
		{"WAV extension", "audio.wav", false},
		{"M4A extension", "audio.m4a", false},
		{"FLAC extension", "audio.flac", false},
		{"OGG extension", "audio.ogg", false},
		{"AAC extension", "audio.aac", false},
		{"Unsupported TXT", "audio.txt", true},
		{"Unsupported MP4", "audio.mp4", true},
		{"Unsupported AVI", "audio.avi", true},
		{"Unsupported DOC", "audio.doc", true},
		{"No extension", "audio", true},
		{"Empty filename", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file with minimal content
			var filePath string
			if tt.filename != "" {
				filePath = filepath.Join(tmpDir, tt.filename)
				if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			} else {
				filePath = ""
			}

			err := validator.ValidateFile(filePath)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error for file: %s", tt.filename)
					return
				}

				docErr, ok := err.(*models.DocumentError)
				if !ok {
					t.Errorf("Expected DocumentError, got: %T", err)
					return
				}

				if docErr.Code != models.ErrInvalidConfig && docErr.Code != models.ErrDocumentNotFound {
					t.Errorf("Expected ErrInvalidConfig or ErrDocumentNotFound, got: %s", docErr.Code)
				}
			}
		})
	}
}

// TestValidateExtensionCaseInsensitive tests that extensions are case-insensitive
func TestValidateExtensionCaseInsensitive(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
	}{
		{"Uppercase MP3", "audio.MP3"},
		{"Uppercase WAV", "audio.WAV"},
		{"Mixed case", "audio.Mp3"},
		{"All caps", "AUDIO.MP3"},
		{"Mixed M4A", "audio.M4a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tt.filename)
			// Create file with valid MP3 header
			header := []byte{0xFF, 0xFB, 0x90, 0x44}
			if err := os.WriteFile(filePath, header, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			err := validator.ValidateFile(filePath)

			// Should succeed - extensions are normalized to lowercase
			if err != nil {
				t.Errorf("Expected no error for case-insensitive extension: %s, got: %v", tt.filename, err)
			}
		})
	}
}

// TestValidateFileSizeEmpty tests empty file rejection
func TestValidateFileSizeEmpty(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "empty.mp3")
	if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := validator.ValidateFile(filePath)

	if err == nil {
		t.Fatal("Expected error for empty file")
	}

	docErr, ok := err.(*models.DocumentError)
	if !ok {
		t.Fatalf("Expected DocumentError, got: %T", err)
	}

	if docErr.Code != models.ErrInvalidConfig {
		t.Errorf("Expected error code ErrInvalidConfig, got: %s", docErr.Code)
	}

	if !strings.Contains(err.Error(), "empty") || !strings.Contains(err.Error(), "0 bytes") {
		t.Errorf("Expected error to mention 'empty' or '0 bytes', got: %s", err.Error())
	}
}

// TestValidateFileSizeExceeded tests oversized file rejection
func TestValidateFileSizeExceeded(t *testing.T) {
	config := DefaultAudioConfig()
	config.MaxAudioFileSize = 1024 // 1KB limit
	validator := NewAudioValidator(&config)
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "large.mp3")
	// Create file larger than limit
	largeData := make([]byte, 2048) // 2KB
	if err := os.WriteFile(filePath, largeData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := validator.ValidateFile(filePath)

	if err == nil {
		t.Fatal("Expected error for oversized file")
	}

	docErr, ok := err.(*models.DocumentError)
	if !ok {
		t.Fatalf("Expected DocumentError, got: %T", err)
	}

	if docErr.Code != models.ErrInvalidConfig {
		t.Errorf("Expected error code ErrInvalidConfig, got: %s", docErr.Code)
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "exceeds") && !strings.Contains(errMsg, "size") {
		t.Errorf("Expected error to mention size limit, got: %s", errMsg)
	}

	// Verify error message includes actual and allowed sizes
	if !strings.Contains(errMsg, "2048") || !strings.Contains(errMsg, "1024") {
		t.Errorf("Expected error to include actual (2048) and allowed (1024) sizes, got: %s", errMsg)
	}
}

// TestValidateFileSizeExactLimit tests file exactly at size limit
func TestValidateFileSizeExactLimit(t *testing.T) {
	config := DefaultAudioConfig()
	config.MaxAudioFileSize = 1024 // 1KB limit
	validator := NewAudioValidator(&config)
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "exact.mp3")
	// Create MP3 header for magic bytes validation
	mp3Header := []byte{0xFF, 0xFB, 0x90, 0x44} // Valid MP3 frame sync
	exactData := make([]byte, 1024)
	copy(exactData, mp3Header)
	if err := os.WriteFile(filePath, exactData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := validator.ValidateFile(filePath)

	// Should succeed - exactly at limit
	if err != nil {
		t.Errorf("Expected no error for file exactly at size limit, got: %v", err)
	}
}

// TestValidateFileSizeSmall tests very small file (1 byte)
func TestValidateFileSizeSmall(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "tiny.mp3")
	if err := os.WriteFile(filePath, []byte{0xFF}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := validator.ValidateFile(filePath)

	// Should fail validation - but not due to size, passes size check
	// Will fail on magic bytes validation (which is non-blocking)
	// So actually should pass overall validation
	if err != nil {
		// Check it's not a size error
		if strings.Contains(err.Error(), "empty") || strings.Contains(err.Error(), "0 bytes") {
			t.Errorf("1-byte file should not trigger empty file error, got: %v", err)
		}
	}
}

// TestValidateMagicBytesMP3 tests MP3 magic bytes validation
func TestValidateMagicBytesMP3(t *testing.T) {
	config := DefaultAudioConfig()
	logger := &mockLogger{}
	validator := NewAudioValidator(&config).WithLogger(logger)
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		header        []byte
		expectWarning bool
		description   string
	}{
		{
			name:          "Valid ID3 tag",
			header:        []byte{0x49, 0x44, 0x33, 0x04, 0x00, 0x00},
			expectWarning: false,
			description:   "ID3v2 tag (0x49 0x44 0x33)",
		},
		{
			name:          "Valid MPEG frame sync 0xFFFB",
			header:        []byte{0xFF, 0xFB, 0x90, 0x44, 0x00, 0x00},
			expectWarning: false,
			description:   "MPEG frame sync (0xFF 0xFB)",
		},
		{
			name:          "Valid MPEG frame sync 0xFFFA",
			header:        []byte{0xFF, 0xFA, 0x90, 0x44, 0x00, 0x00},
			expectWarning: false,
			description:   "MPEG frame sync (0xFF 0xFA)",
		},
		{
			name:          "Valid MPEG frame sync 0xFFF3",
			header:        []byte{0xFF, 0xF3, 0x44, 0xC4, 0x00, 0x00},
			expectWarning: false,
			description:   "MPEG frame sync (0xFF 0xF3)",
		},
		{
			name:          "Valid MPEG frame sync 0xFFF2",
			header:        []byte{0xFF, 0xF2, 0x40, 0xC4, 0x00, 0x00},
			expectWarning: false,
			description:   "MPEG frame sync (0xFF 0xF2)",
		},
		{
			name:          "Invalid MP3 header",
			header:        []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			expectWarning: true,
			description:   "Invalid magic bytes should log warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger.warnCalls = nil // Reset warnings

			filePath := filepath.Join(tmpDir, "test.mp3")
			if err := os.WriteFile(filePath, tt.header, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			err := validator.ValidateFile(filePath)

			// Magic bytes validation should NOT fail validation, only log warning
			if err != nil {
				t.Errorf("Expected no error (magic bytes validation is non-blocking), got: %v", err)
			}

			if tt.expectWarning {
				if len(logger.warnCalls) == 0 {
					t.Error("Expected warning to be logged for invalid magic bytes")
				} else {
					warnMsg := logger.warnCalls[0].msg
					if !strings.Contains(strings.ToLower(warnMsg), "format") {
						t.Errorf("Expected warning about format validation, got: %s", warnMsg)
					}
				}
			}
		})
	}
}

// TestValidateMagicBytesWAV tests WAV magic bytes validation
func TestValidateMagicBytesWAV(t *testing.T) {
	config := DefaultAudioConfig()
	logger := &mockLogger{}
	validator := NewAudioValidator(&config).WithLogger(logger)
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		header        []byte
		expectWarning bool
	}{
		{
			name:          "Valid WAV header",
			header:        []byte("RIFF----WAVE"),
			expectWarning: false,
		},
		{
			name:          "Invalid WAV header",
			header:        []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			expectWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger.warnCalls = nil

			filePath := filepath.Join(tmpDir, "test.wav")
			if err := os.WriteFile(filePath, tt.header, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			err := validator.ValidateFile(filePath)

			if err != nil {
				t.Errorf("Expected no error (magic bytes validation is non-blocking), got: %v", err)
			}

			if tt.expectWarning && len(logger.warnCalls) == 0 {
				t.Error("Expected warning to be logged for invalid magic bytes")
			}
		})
	}
}

// TestValidateMagicBytesFLAC tests FLAC magic bytes validation
func TestValidateMagicBytesFLAC(t *testing.T) {
	config := DefaultAudioConfig()
	logger := &mockLogger{}
	validator := NewAudioValidator(&config).WithLogger(logger)
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		header        []byte
		expectWarning bool
	}{
		{
			name:          "Valid FLAC header",
			header:        []byte("fLaC"),
			expectWarning: false,
		},
		{
			name:          "Invalid FLAC header",
			header:        []byte{0x00, 0x00, 0x00, 0x00},
			expectWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger.warnCalls = nil

			filePath := filepath.Join(tmpDir, "test.flac")
			if err := os.WriteFile(filePath, tt.header, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			err := validator.ValidateFile(filePath)

			if err != nil {
				t.Errorf("Expected no error (magic bytes validation is non-blocking), got: %v", err)
			}

			if tt.expectWarning && len(logger.warnCalls) == 0 {
				t.Error("Expected warning to be logged for invalid magic bytes")
			}
		})
	}
}

// TestValidateMagicBytesOGG tests OGG magic bytes validation
func TestValidateMagicBytesOGG(t *testing.T) {
	config := DefaultAudioConfig()
	logger := &mockLogger{}
	validator := NewAudioValidator(&config).WithLogger(logger)
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		header        []byte
		expectWarning bool
	}{
		{
			name:          "Valid OGG header",
			header:        []byte("OggS"),
			expectWarning: false,
		},
		{
			name:          "Invalid OGG header",
			header:        []byte{0x00, 0x00, 0x00, 0x00},
			expectWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger.warnCalls = nil

			filePath := filepath.Join(tmpDir, "test.ogg")
			if err := os.WriteFile(filePath, tt.header, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			err := validator.ValidateFile(filePath)

			if err != nil {
				t.Errorf("Expected no error (magic bytes validation is non-blocking), got: %v", err)
			}

			if tt.expectWarning && len(logger.warnCalls) == 0 {
				t.Error("Expected warning to be logged for invalid magic bytes")
			}
		})
	}
}

// TestValidateMagicBytesM4A tests M4A/AAC magic bytes validation
func TestValidateMagicBytesM4A(t *testing.T) {
	config := DefaultAudioConfig()
	logger := &mockLogger{}
	validator := NewAudioValidator(&config).WithLogger(logger)
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		filename      string
		header        []byte
		expectWarning bool
		description   string
	}{
		{
			name:          "Valid M4A ftyp header",
			filename:      "test.m4a",
			header:        []byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70},
			expectWarning: false,
			description:   "M4A with ftyp atom",
		},
		{
			name:          "Valid AAC ADTS 0xFFF1",
			filename:      "test.aac",
			header:        []byte{0xFF, 0xF1, 0x50, 0x80, 0x00, 0x00},
			expectWarning: false,
			description:   "AAC ADTS stream (0xFF 0xF1)",
		},
		{
			name:          "Valid AAC ADTS 0xFFF9",
			filename:      "test.aac",
			header:        []byte{0xFF, 0xF9, 0x50, 0x80, 0x00, 0x00},
			expectWarning: false,
			description:   "AAC ADTS stream (0xFF 0xF9)",
		},
		{
			name:          "Invalid M4A header",
			filename:      "test.m4a",
			header:        []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			expectWarning: true,
			description:   "Invalid magic bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger.warnCalls = nil

			filePath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(filePath, tt.header, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			err := validator.ValidateFile(filePath)

			if err != nil {
				t.Errorf("Expected no error (magic bytes validation is non-blocking), got: %v", err)
			}

			if tt.expectWarning && len(logger.warnCalls) == 0 {
				t.Error("Expected warning to be logged for invalid magic bytes")
			}
		})
	}
}

// TestGetFileInfo tests the GetFileInfo method
func TestGetFileInfo(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)
	tmpDir := t.TempDir()

	// Create a test file
	filePath := filepath.Join(tmpDir, "test.mp3")
	testData := []byte("test data")
	if err := os.WriteFile(filePath, testData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	info, err := validator.GetFileInfo(filePath)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if info == nil {
		t.Fatal("Expected file info, got nil")
	}

	if info.Path != filePath {
		t.Errorf("Expected path %s, got %s", filePath, info.Path)
	}

	if info.Size != int64(len(testData)) {
		t.Errorf("Expected size %d, got %d", len(testData), info.Size)
	}

	if info.Extension != ".mp3" {
		t.Errorf("Expected extension .mp3, got %s", info.Extension)
	}

	if info.ModTime.IsZero() {
		t.Error("Expected non-zero modification time")
	}

	// Verify ModTime is recent (within last minute)
	if time.Since(info.ModTime) > time.Minute {
		t.Error("Expected recent modification time")
	}
}

// TestGetFileInfoUppercaseExtension tests extension normalization
func TestGetFileInfoUppercaseExtension(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "test.MP3")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	info, err := validator.GetFileInfo(filePath)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Extension should be lowercase
	if info.Extension != ".mp3" {
		t.Errorf("Expected lowercase extension .mp3, got %s", info.Extension)
	}
}

// TestGetFileInfoNonExistent tests error handling for non-existent file
func TestGetFileInfoNonExistent(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)

	info, err := validator.GetFileInfo("/nonexistent/file.mp3")

	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	if info != nil {
		t.Error("Expected nil info for error case")
	}
}

// TestValidateWithCustomConfig tests validator with custom configuration
func TestValidateWithCustomConfig(t *testing.T) {
	// Custom config with only .mp3 and .wav supported
	config := AudioConfig{
		MaxAudioFileSize: 1024,
		SupportedFormats: []string{".mp3", ".wav"},
	}
	validator := NewAudioValidator(&config)
	tmpDir := t.TempDir()

	tests := []struct {
		filename  string
		shouldErr bool
	}{
		{"audio.mp3", false}, // supported
		{"audio.wav", false}, // supported
		{"audio.flac", true}, // not in custom config
		{"audio.ogg", true},  // not in custom config
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tt.filename)
			// Create valid MP3 header
			header := []byte{0xFF, 0xFB, 0x90, 0x44}
			if err := os.WriteFile(filePath, header, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			err := validator.ValidateFile(filePath)

			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for %s with custom config", tt.filename)
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("Expected no error for %s, got: %v", tt.filename, err)
			}
		})
	}
}

// TestValidateErrorMessages tests that error messages are clear and actionable
func TestValidateErrorMessages(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)
	tmpDir := t.TempDir()

	t.Run("Unsupported format includes supported list", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		err := validator.ValidateFile(filePath)
		if err == nil {
			t.Fatal("Expected error")
		}

		errMsg := err.Error()
		// Should mention supported formats
		if !strings.Contains(errMsg, ".mp3") || !strings.Contains(errMsg, ".wav") {
			t.Errorf("Expected error to list supported formats, got: %s", errMsg)
		}
	})

	t.Run("File size error includes actual and allowed sizes", func(t *testing.T) {
		config := AudioConfig{
			MaxAudioFileSize: 100,
			SupportedFormats: []string{".mp3"},
		}
		validator := NewAudioValidator(&config)

		filePath := filepath.Join(tmpDir, "large.mp3")
		largeData := make([]byte, 200)
		if err := os.WriteFile(filePath, largeData, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		err := validator.ValidateFile(filePath)
		if err == nil {
			t.Fatal("Expected error")
		}

		errMsg := err.Error()
		if !strings.Contains(errMsg, "200") || !strings.Contains(errMsg, "100") {
			t.Errorf("Expected error to include actual (200) and max (100) sizes, got: %s", errMsg)
		}
	})
}

// TestValidateEdgeCases tests edge cases and boundary conditions
func TestValidateEdgeCases(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)
	tmpDir := t.TempDir()

	t.Run("Very long file path", func(t *testing.T) {
		// Create nested directories for long path
		longPath := tmpDir
		for i := 0; i < 10; i++ {
			longPath = filepath.Join(longPath, "very_long_directory_name_to_test_path_limits")
		}
		if err := os.MkdirAll(longPath, 0755); err != nil {
			t.Fatalf("Failed to create long path: %v", err)
		}

		filePath := filepath.Join(longPath, "test.mp3")
		header := []byte{0xFF, 0xFB, 0x90, 0x44}
		if err := os.WriteFile(filePath, header, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		err := validator.ValidateFile(filePath)
		if err != nil {
			t.Errorf("Expected no error for long valid path, got: %v", err)
		}
	})

	t.Run("Path with spaces", func(t *testing.T) {
		dirPath := filepath.Join(tmpDir, "dir with spaces")
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		filePath := filepath.Join(dirPath, "file with spaces.mp3")
		header := []byte{0xFF, 0xFB, 0x90, 0x44}
		if err := os.WriteFile(filePath, header, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		err := validator.ValidateFile(filePath)
		if err != nil {
			t.Errorf("Expected no error for path with spaces, got: %v", err)
		}
	})

	t.Run("Path with special characters", func(t *testing.T) {
		dirPath := filepath.Join(tmpDir, "dir-with_special.chars")
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		filePath := filepath.Join(dirPath, "file-name_test.mp3")
		header := []byte{0xFF, 0xFB, 0x90, 0x44}
		if err := os.WriteFile(filePath, header, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		err := validator.ValidateFile(filePath)
		if err != nil {
			t.Errorf("Expected no error for path with special chars, got: %v", err)
		}
	})

	t.Run("Multiple extensions", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "archive.tar.mp3")
		header := []byte{0xFF, 0xFB, 0x90, 0x44}
		if err := os.WriteFile(filePath, header, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Should use last extension (.mp3)
		err := validator.ValidateFile(filePath)
		if err != nil {
			t.Errorf("Expected no error for multiple extensions, got: %v", err)
		}
	})
}

// TestValidateCompleteFlow tests complete validation flow with valid file
func TestValidateCompleteFlow(t *testing.T) {
	config := DefaultAudioConfig()
	logger := &mockLogger{}
	validator := NewAudioValidator(&config).WithLogger(logger)
	tmpDir := t.TempDir()

	// Create valid MP3 file
	filePath := filepath.Join(tmpDir, "valid.mp3")
	mp3Data := []byte{0xFF, 0xFB, 0x90, 0x44}        // Valid MP3 header
	mp3Data = append(mp3Data, make([]byte, 1000)...) // Add some data
	if err := os.WriteFile(filePath, mp3Data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := validator.ValidateFile(filePath)

	if err != nil {
		t.Fatalf("Expected valid file to pass validation, got: %v", err)
	}

	// Should not have any warnings for valid file
	if len(logger.warnCalls) > 0 {
		t.Errorf("Expected no warnings for valid file, got %d warnings", len(logger.warnCalls))
	}
}

// TestValidateFileInfoStruct tests AudioFileInfo struct
func TestValidateFileInfoStruct(t *testing.T) {
	now := time.Now()
	info := &AudioFileInfo{
		Path:      "/tmp/test.mp3",
		Size:      1024,
		Extension: ".mp3",
		ModTime:   now,
	}

	if info.Path != "/tmp/test.mp3" {
		t.Errorf("Expected path /tmp/test.mp3, got %s", info.Path)
	}

	if info.Size != 1024 {
		t.Errorf("Expected size 1024, got %d", info.Size)
	}

	if info.Extension != ".mp3" {
		t.Errorf("Expected extension .mp3, got %s", info.Extension)
	}

	if !info.ModTime.Equal(now) {
		t.Errorf("Expected ModTime to equal %v, got %v", now, info.ModTime)
	}
}

// TestValidateSecuritySummary provides a summary test for all security features
func TestValidateSecuritySummary(t *testing.T) {
	config := DefaultAudioConfig()
	validator := NewAudioValidator(&config)

	securityTests := []struct {
		category string
		path     string
		blocked  bool
	}{
		// Path traversal attacks
		{"Path Traversal", "../etc/passwd", true},
		{"Path Traversal", "../../root/.ssh/id_rsa", true},
		{"Path Traversal", "/tmp/../etc/shadow", true},

		// Restricted directories
		{"Restricted Dir", "/etc/passwd", true},
		{"Restricted Dir", "/sys/kernel/config", true},
		{"Restricted Dir", "/proc/self/environ", true},
		{"Restricted Dir", "/dev/sda", true},
		{"Restricted Dir", "/root/.bashrc", true},

		// Safe paths (should not be blocked by security checks alone)
		{"Safe Path", "/tmp/audio.mp3", false},
		{"Safe Path", "/home/user/music/song.mp3", false},
		{"Safe Path", "/var/www/uploads/audio.mp3", false},
	}

	for _, tt := range securityTests {
		t.Run(tt.category+": "+tt.path, func(t *testing.T) {
			err := validator.ValidateFile(tt.path)

			if tt.blocked {
				if err == nil {
					t.Errorf("SECURITY FAILURE: Path should be blocked: %s", tt.path)
				} else {
					docErr, ok := err.(*models.DocumentError)
					if !ok {
						t.Errorf("Expected DocumentError, got: %T", err)
					} else if docErr.Code != models.ErrInvalidConfig && docErr.Code != models.ErrDocumentNotFound {
						t.Errorf("Expected ErrInvalidConfig or ErrDocumentNotFound, got: %s", docErr.Code)
					}
				}
			}
			// Note: Safe paths will still fail if file doesn't exist, but should not
			// fail due to security checks (would need to create actual files to fully test)
		})
	}
}
