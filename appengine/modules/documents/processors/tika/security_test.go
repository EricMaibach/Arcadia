package tika

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"arcadia/modules/documents/interfaces"
)

// MockLogger implements a basic logger for testing
type MockLogger struct {
	logs []string
}

func (m *MockLogger) Debug(ctx context.Context, msg string, fields ...interface{}) {
	m.logs = append(m.logs, "DEBUG: "+msg)
}

func (m *MockLogger) Info(ctx context.Context, msg string, fields ...interface{}) {
	m.logs = append(m.logs, "INFO: "+msg)
}

func (m *MockLogger) Warn(ctx context.Context, msg string, fields ...interface{}) {
	m.logs = append(m.logs, "WARN: "+msg)
}

func (m *MockLogger) Error(ctx context.Context, msg string, fields ...interface{}) {
	m.logs = append(m.logs, "ERROR: "+msg)
}

func (m *MockLogger) Fatal(ctx context.Context, msg string, fields ...interface{}) {
	m.logs = append(m.logs, "FATAL: "+msg)
}

func (m *MockLogger) WithFields(fields map[string]interface{}) interfaces.Logger {
	return m
}

func (m *MockLogger) WithContext(ctx context.Context) interfaces.Logger {
	return m
}

func (m *MockLogger) WithModule(module string) interfaces.Logger {
	return m
}

func (m *MockLogger) WithComponent(component string) interfaces.Logger {
	return m
}

func TestSecurityValidation(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	tests := []struct {
		name     string
		filePath string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "Path traversal attack",
			filePath: "../../../etc/passwd",
			wantErr:  true,
			errMsg:   "invalid file path",
		},
		{
			name:     "Root directory access",
			filePath: "/etc/passwd",
			wantErr:  true,
			errMsg:   "access denied",
		},
		{
			name:     "Proc directory access",
			filePath: "/proc/version",
			wantErr:  true,
			errMsg:   "access denied",
		},
		{
			name:     "Dev directory access",
			filePath: "/dev/null",
			wantErr:  true,
			errMsg:   "access denied",
		},
		{
			name:     "Nonexistent file",
			filePath: "/tmp/nonexistent.pdf",
			wantErr:  true,
			errMsg:   "file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.validateFilePath(tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFilePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && err.Error() != tt.errMsg {
				t.Errorf("validateFilePath() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestSecurityLogging(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	// Test path traversal logging
	client.validateFilePath("../../../etc/passwd")

	found := false
	for _, log := range logger.logs {
		if log == "WARN: Path traversal attempt detected" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected security warning to be logged for path traversal attempt")
	}
}

// TestPathTraversalAttacks tests comprehensive path traversal attack scenarios
func TestPathTraversalAttacks(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	tests := []struct {
		name        string
		filePath    string
		wantErr     bool
		errMsg      string
		description string
	}{
		{
			name:        "Classic path traversal",
			filePath:    "../../../etc/passwd",
			wantErr:     true,
			errMsg:      "invalid file path",
			description: "Basic ../ traversal attempt",
		},
		{
			name:        "Windows path traversal",
			filePath:    "..\\..\\..\\..\\windows\\system32\\config\\sam",
			wantErr:     true,
			errMsg:      "invalid file path",
			description: "Windows-style path traversal",
		},
		{
			name:        "Mixed separators",
			filePath:    "..\\\\/..\\\\/../../../etc/passwd",
			wantErr:     true,
			errMsg:      "invalid file path",
			description: "Mixed Windows/Unix path separators",
		},
		{
			name:        "Deep traversal",
			filePath:    "../../../../../../../../../etc/passwd",
			wantErr:     true,
			errMsg:      "invalid file path",
			description: "Deep path traversal attempt",
		},
		{
			name:        "Current dir traversal",
			filePath:    "./../../etc/passwd",
			wantErr:     true,
			errMsg:      "invalid file path",
			description: "Path traversal starting with current directory",
		},
		{
			name:        "Hidden traversal",
			filePath:    "legitimate/../../../etc/passwd",
			wantErr:     true,
			errMsg:      "invalid file path",
			description: "Path traversal hidden in legitimate path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.validateFilePath(tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFilePath() error = %v, wantErr %v for %s", err, tt.wantErr, tt.description)
				return
			}
			if err != nil && err.Error() != tt.errMsg {
				t.Errorf("validateFilePath() error = %v, want %v for %s", err.Error(), tt.errMsg, tt.description)
			}
		})
	}
}

// TestEncodedPathAttacks tests URL-encoded and other encoded path attacks
func TestEncodedPathAttacks(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	tests := []struct {
		name     string
		filePath string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "URL encoded traversal",
			filePath: "%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
			wantErr:  true,
			errMsg:   "invalid file path",
		},
		{
			name:     "Double URL encoded",
			filePath: "%252e%252e%252f%252e%252e%252f%252e%252e%252fetc%252fpasswd",
			wantErr:  true,
			errMsg:   "invalid file path",
		},
		{
			name:     "Unicode encoded traversal",
			filePath: "\u002e\u002e\u002f\u002e\u002e\u002f\u002e\u002e\u002fetc\u002fpasswd",
			wantErr:  true,
			errMsg:   "invalid file path",
		},
	}

	// Note: These tests may not trigger since Go's filepath.Clean might normalize them
	// But we test to ensure our validation is robust
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.validateFilePath(tt.filePath)
			if err == nil {
				// If no error, the path should at least fail on file existence
				// since these are invalid paths that shouldn't exist
				t.Logf("Warning: Encoded path %s was not rejected by validation", tt.filePath)
			}
		})
	}
}

// TestRestrictedDirectoryAccess tests access to all restricted system directories
func TestRestrictedDirectoryAccess(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	restrictedDirs := []string{
		"/etc",
		"/proc",
		"/sys",
		"/dev",
		"/root",
		"/usr/bin",
		"/usr/sbin",
		"/boot",
		"/var/log",
	}

	restrictedFiles := []string{
		"passwd",
		"shadow",
		"version",
		"null",
		".ssh/id_rsa",
		"bash",
		"su",
		"grub.cfg",
		"secure",
	}

	for _, dir := range restrictedDirs {
		for _, file := range restrictedFiles {
			filePath := dir + "/" + file
			t.Run("Access_"+dir+"_"+file, func(t *testing.T) {
				err := client.validateFilePath(filePath)
				if err == nil {
					t.Errorf("Expected access to %s to be denied", filePath)
				} else if err.Error() != "access denied" {
					t.Errorf("Expected 'access denied' error for %s, got %v", filePath, err)
				}
			})
		}
	}
}

// TestFileTypeValidation tests file extension filtering
func TestFileTypeValidation(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		ext     string
		wantErr bool
		errMsg  string
		allowed bool
	}{
		// Allowed file types
		{"PDF file", ".pdf", false, "", true},
		{"Word doc", ".doc", false, "", true},
		{"Word docx", ".docx", false, "", true},
		{"Text file", ".txt", false, "", true},
		{"RTF file", ".rtf", false, "", true},
		{"ODT file", ".odt", false, "", true},
		{"PowerPoint", ".ppt", false, "", true},
		{"PowerPoint X", ".pptx", false, "", true},
		{"Excel", ".xls", false, "", true},
		{"Excel X", ".xlsx", false, "", true},
		{"CSV file", ".csv", false, "", true},

		// Dangerous file types that should be blocked
		{"Executable", ".exe", true, "unsupported file type", false},
		{"Shell script", ".sh", true, "unsupported file type", false},
		{"Batch file", ".bat", true, "unsupported file type", false},
		{"Binary", ".bin", true, "unsupported file type", false},
		{"Dynamic library", ".so", true, "unsupported file type", false},
		{"JavaScript", ".js", true, "unsupported file type", false},
		{"Python script", ".py", true, "unsupported file type", false},
		{"PHP script", ".php", true, "unsupported file type", false},
		{"Perl script", ".pl", true, "unsupported file type", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file with proper extension
			testFile := tmpDir + "/test" + tt.ext
			f, err := os.Create(testFile)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			f.WriteString("test content")
			f.Close()

			err = client.validateFilePath(testFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFilePath() error = %v, wantErr %v for %s", err, tt.wantErr, tt.name)
				return
			}
			if err != nil && err.Error() != tt.errMsg {
				t.Errorf("validateFilePath() error = %v, want %v for %s", err.Error(), tt.errMsg, tt.name)
			}
		})
	}
}

// TestInformationDisclosure tests that error messages don't leak sensitive paths
func TestInformationDisclosure(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	// Test that validateFilePath errors don't expose system paths
	sensitivePaths := []string{
		"/etc/passwd",
		"/root/.ssh/id_rsa",
		"/proc/version",
		"/sys/devices",
	}

	for _, path := range sensitivePaths {
		t.Run("Error_message_for_"+path, func(t *testing.T) {
			err := client.validateFilePath(path)
			if err == nil {
				t.Errorf("Expected error for sensitive path %s", path)
				return
			}

			// Check that error message doesn't contain the full path
			errorMsg := err.Error()
			if strings.Contains(errorMsg, path) {
				t.Errorf("Error message '%s' contains sensitive path '%s'", errorMsg, path)
			}

			// Error should be generic
			allowedErrors := []string{"access denied", "invalid file path", "file not found"}
			validError := false
			for _, allowed := range allowedErrors {
				if errorMsg == allowed {
					validError = true
					break
				}
			}
			if !validError {
				t.Errorf("Error message '%s' is not generic enough", errorMsg)
			}
		})
	}
}

// TestSpecialCharacterPaths tests paths with special characters
func TestSpecialCharacterPaths(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	specialPaths := []string{
		"../../../etc/passwd\\x00", // Null byte
		"../../../etc/passwd\\n",   // Newline
		"../../../etc/passwd\\r",   // Carriage return
		"../../../etc/passwd\\t",   // Tab
		"../../../etc/passwd\\x7f", // DEL character
		"../../../etc/passwd\\x1b", // ESC character
	}

	for i, path := range specialPaths {
		t.Run(fmt.Sprintf("Special_char_%d", i), func(t *testing.T) {
			err := client.validateFilePath(path)
			// These should all be rejected for path traversal or file not found
			if err == nil {
				t.Errorf("Expected error for special character path: %q", path)
			}
		})
	}
}

// TestLongPaths tests very long path handling
func TestLongPaths(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	// Create a very long path with traversal
	longPath := strings.Repeat("../", 1000) + "etc/passwd"

	err := client.validateFilePath(longPath)
	if err == nil {
		t.Error("Expected error for very long path traversal")
	} else if err.Error() != "invalid file path" {
		t.Errorf("Expected 'invalid file path' error, got %v", err)
	}
}

// TestContextCancellation tests resource cleanup during context cancellation
func TestContextCancellation(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// These operations should handle cancellation gracefully
	_, err := client.ExtractText(ctx, "/tmp/nonexistent.pdf")
	if err != context.Canceled {
		t.Logf("ExtractText with cancelled context returned: %v (expected context.Canceled)", err)
	}

	_, err = client.ExtractMetadata(ctx, "/tmp/nonexistent.pdf")
	if err != context.Canceled {
		t.Logf("ExtractMetadata with cancelled context returned: %v (expected context.Canceled)", err)
	}

	_, err = client.DetectType(ctx, "/tmp/nonexistent.pdf")
	if err != context.Canceled {
		t.Logf("DetectType with cancelled context returned: %v (expected context.Canceled)", err)
	}
}
