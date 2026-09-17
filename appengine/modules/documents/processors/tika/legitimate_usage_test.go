package tika

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLegitimateFileAccess tests that legitimate file operations still work correctly
func TestLegitimateFileAccess(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Test cases for legitimate file access
	tests := []struct {
		name        string
		ext         string
		content     string
		shouldPass  bool
		description string
	}{
		{
			name:        "PDF document",
			ext:         ".pdf",
			content:     "PDF content simulation",
			shouldPass:  true,
			description: "Legitimate PDF file should pass validation",
		},
		{
			name:        "Word document",
			ext:         ".docx",
			content:     "Word document content",
			shouldPass:  true,
			description: "Legitimate Word document should pass validation",
		},
		{
			name:        "Text file",
			ext:         ".txt",
			content:     "Plain text content",
			shouldPass:  true,
			description: "Legitimate text file should pass validation",
		},
		{
			name:        "Excel spreadsheet",
			ext:         ".xlsx",
			content:     "Excel spreadsheet content",
			shouldPass:  true,
			description: "Legitimate Excel file should pass validation",
		},
		{
			name:        "PowerPoint presentation",
			ext:         ".pptx",
			content:     "PowerPoint presentation content",
			shouldPass:  true,
			description: "Legitimate PowerPoint file should pass validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			filePath := filepath.Join(tmpDir, "test"+tt.ext)
			f, err := os.Create(filePath)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			_, err = f.WriteString(tt.content)
			if err != nil {
				t.Fatalf("Failed to write test content: %v", err)
			}
			f.Close()

			// Test file path validation
			err = client.validateFilePath(filePath)
			if tt.shouldPass && err != nil {
				t.Errorf("validateFilePath() failed for legitimate file %s: %v", tt.description, err)
			} else if !tt.shouldPass && err == nil {
				t.Errorf("validateFilePath() should have failed for %s", tt.description)
			}
		})
	}
}

// TestSecurePathNormalization tests that path normalization works correctly for legitimate paths
func TestSecurePathNormalization(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.pdf")
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	f.WriteString("test content")
	f.Close()

	// Test various legitimate path formats
	legitimatePaths := []string{
		testFile,                       // Direct path
		tmpDir + "/./test.pdf",         // With current directory reference
		tmpDir + "/subdir/../test.pdf", // With parent directory reference that resolves correctly
	}

	for _, path := range legitimatePaths {
		t.Run("Path_"+path, func(t *testing.T) {
			err := client.validateFilePath(path)
			if err != nil {
				t.Errorf("Legitimate path %s should pass validation, got error: %v", path, err)
			}
		})
	}
}

// TestPerformanceImpact tests that security validation doesn't significantly impact performance
func TestPerformanceImpact(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "performance_test.pdf")
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	f.WriteString("test content for performance testing")
	f.Close()

	// Measure validation performance
	iterations := 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		err := client.validateFilePath(testFile)
		if err != nil {
			t.Fatalf("Validation failed: %v", err)
		}
	}

	duration := time.Since(start)
	avgTime := duration / time.Duration(iterations)

	t.Logf("Average validation time per file: %v", avgTime)

	// Validation should be very fast (less than 1ms per file)
	if avgTime > time.Millisecond {
		t.Errorf("Security validation is too slow: %v per file (expected < 1ms)", avgTime)
	}
}

// TestConcurrentSecurityValidation tests thread safety of security validation
func TestConcurrentSecurityValidation(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	tmpDir := t.TempDir()

	// Create multiple test files
	var testFiles []string
	for i := 0; i < 10; i++ {
		testFile := filepath.Join(tmpDir, fmt.Sprintf("concurrent_test_%d.pdf", i))
		f, err := os.Create(testFile)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		f.WriteString(fmt.Sprintf("test content %d", i))
		f.Close()
		testFiles = append(testFiles, testFile)
	}

	// Test concurrent access
	done := make(chan bool, len(testFiles))
	errors := make(chan error, len(testFiles))

	for _, file := range testFiles {
		go func(filePath string) {
			err := client.validateFilePath(filePath)
			if err != nil {
				errors <- err
			}
			done <- true
		}(file)
	}

	// Wait for all goroutines to complete
	for i := 0; i < len(testFiles); i++ {
		select {
		case <-done:
			// Success
		case err := <-errors:
			t.Errorf("Concurrent validation failed: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent validation timed out")
		}
	}
}

// TestFileSizeLimits tests that file size limits are properly enforced
func TestFileSizeLimits(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	config.MaxFileSize = 1024 // Set small limit for testing
	client := NewTikaClient(config, logger)

	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		size    int64
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Small file within limit",
			size:    512,
			wantErr: false,
		},
		{
			name:    "File at exact limit",
			size:    1024,
			wantErr: false,
		},
		{
			name:    "File exceeding limit",
			size:    2048,
			wantErr: true,
			errMsg:  "file too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, fmt.Sprintf("size_test_%d.pdf", tt.size))
			f, err := os.Create(testFile)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Write specified amount of data
			data := make([]byte, tt.size)
			for i := range data {
				data[i] = 'A'
			}
			_, err = f.Write(data)
			if err != nil {
				t.Fatalf("Failed to write test data: %v", err)
			}
			f.Close()

			err = client.validateFilePath(testFile)
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

// TestDirectoryTraversalLogging tests that security violations are properly logged
func TestDirectoryTraversalLogging(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultTikaConfig()
	client := NewTikaClient(config, logger)

	// Test various security violations
	violations := []struct {
		path        string
		expectedLog string
	}{
		{
			path:        "../../../etc/passwd",
			expectedLog: "WARN: Path traversal attempt detected",
		},
		{
			path:        "/etc/passwd",
			expectedLog: "WARN: Attempted access to restricted directory",
		},
		{
			path:        "/root/.ssh/id_rsa",
			expectedLog: "WARN: Attempted access to restricted directory",
		},
	}

	for _, violation := range violations {
		t.Run("Logging_"+violation.path, func(t *testing.T) {
			// Clear previous logs
			logger.logs = []string{}

			// Trigger security violation
			client.validateFilePath(violation.path)

			// Check if appropriate warning was logged
			found := false
			for _, log := range logger.logs {
				if strings.Contains(log, "WARN:") {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected security warning to be logged for path: %s", violation.path)
			}
		})
	}
}
