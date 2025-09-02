package services

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppLogger_Initialize(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(*testing.T) (func(), error)
		wantErr     bool
		wantErrMsg  string
	}{
		{
			name: "successful initialization",
			setupFunc: func(t *testing.T) (func(), error) {
				tmpDir, err := ioutil.TempDir("", "test_logs")
				if err != nil {
					return nil, err
				}
				originalWd, _ := os.Getwd()
				os.Chdir(tmpDir)
				return func() {
					os.Chdir(originalWd)
					os.RemoveAll(tmpDir)
				}, nil
			},
			wantErr: false,
		},
		{
			name: "initialization in readonly directory",
			setupFunc: func(t *testing.T) (func(), error) {
				tmpDir, err := ioutil.TempDir("", "test_logs")
				if err != nil {
					return nil, err
				}
				originalWd, _ := os.Getwd()
				os.Chdir(tmpDir)
				os.Chmod(tmpDir, 0444) // Make read-only
				return func() {
					os.Chmod(tmpDir, 0755) // Restore permissions for cleanup
					os.Chdir(originalWd)
					os.RemoveAll(tmpDir)
				}, nil
			},
			wantErr:    true,
			wantErrMsg: "failed to create logs directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup, err := tt.setupFunc(t)
			if err != nil {
				t.Fatalf("Setup failed: %v", err)
			}
			defer cleanup()

			logger := NewAppLogger()
			err = logger.Initialize()

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("Expected error containing '%s', got '%v'", tt.wantErrMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				logger.Close()
			}
		})
	}
}

func TestAppLogger_LogAppSubmission(t *testing.T) {
	// Create temporary directory
	tmpDir, err := ioutil.TempDir("", "test_logs")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	logger := NewAppLogger()
	err = logger.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

	// Test logging
	testMessage := "Test log message %s %d"
	testArg1 := "arg1"
	testArg2 := 42

	logger.LogAppSubmission(testMessage, testArg1, testArg2)

	// Verify log file was created and contains expected content
	logFiles, err := filepath.Glob(filepath.Join(tmpDir, "logs", "app_submissions_*.log"))
	if err != nil {
		t.Fatalf("Failed to find log files: %v", err)
	}

	if len(logFiles) == 0 {
		t.Fatal("No log files found")
	}

	logContent, err := ioutil.ReadFile(logFiles[0])
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logStr := string(logContent)
	expectedContent := "Test log message arg1 42"

	if !strings.Contains(logStr, expectedContent) {
		t.Errorf("Log file doesn't contain expected content. Got: %s", logStr)
	}

	if !strings.Contains(logStr, "[APP_SUBMISSION]") {
		t.Error("Log file doesn't contain expected prefix")
	}
}

func TestAppLogger_Close(t *testing.T) {
	// Create temporary directory and logger
	tmpDir, err := ioutil.TempDir("", "test_logs")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	logger := NewAppLogger()
	logger.Initialize()

	// Test that we can log before closing
	logger.LogAppSubmission("Before close")

	// Close the logger
	logger.Close()

	// Test that logging after close is safe (no panic)
	logger.LogAppSubmission("After close - should be no-op")

	// Test double close is safe
	logger.Close()
}

func TestAppLogger_GetLogFunc(t *testing.T) {
	logger := NewAppLogger()

	logFunc := logger.GetLogFunc()
	if logFunc == nil {
		t.Fatal("GetLogFunc returned nil")
	}

	// Test that returned function works (should be safe even without initialization)
	logFunc("Test message from function")
}

func TestGlobalLogger(t *testing.T) {
	// Save original state
	originalLogger := globalAppLogger

	// Clean up after test
	defer func() {
		if globalAppLogger != nil {
			CloseAppLogger()
		}
		globalAppLogger = originalLogger
	}()

	// Create temporary directory for this test
	tmpDir, err := ioutil.TempDir("", "test_global_logs")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test InitializeAppLogger
	err = InitializeAppLogger()
	if err != nil {
		t.Fatalf("Failed to initialize global logger: %v", err)
	}

	// Test GetGlobalAppLogger
	logger := GetGlobalAppLogger()
	if logger == nil {
		t.Fatal("GetGlobalAppLogger returned nil")
	}

	// Test LogAppSubmission (global function)
	LogAppSubmission("Global test message")

	// Test GetAppLogFunc
	logFunc := GetAppLogFunc()
	if logFunc == nil {
		t.Fatal("GetAppLogFunc returned nil")
	}
	logFunc("Function test message")

	// Test CloseAppLogger
	CloseAppLogger()

	// Test that functions are safe after close
	LogAppSubmission("After close - should be no-op")
	logFunc = GetAppLogFunc()
	logFunc("Function after close - should be no-op")

	if GetGlobalAppLogger() != nil {
		t.Error("Expected global logger to be nil after close")
	}
}

func TestAppLogger_GetLogFunc_NilSafety(t *testing.T) {
	// Test GetAppLogFunc when global logger is nil
	originalLogger := globalAppLogger
	globalAppLogger = nil
	defer func() {
		globalAppLogger = originalLogger
	}()

	logFunc := GetAppLogFunc()
	if logFunc == nil {
		t.Fatal("GetAppLogFunc returned nil even when logger is nil")
	}

	// Should not panic
	logFunc("Test message when logger is nil")
}

func TestAppLogger_LogFilenameFormat(t *testing.T) {
	// Create temporary directory
	tmpDir, err := ioutil.TempDir("", "test_logs")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	logger := NewAppLogger()
	logger.Initialize()
	defer logger.Close()

	// Log something to ensure file is created
	logger.LogAppSubmission("Test message")

	// Check that log file has correct date format
	today := time.Now().Format("2006-01-02")
	expectedPattern := filepath.Join(tmpDir, "logs", "app_submissions_"+today+".log")

	if _, err := os.Stat(expectedPattern); os.IsNotExist(err) {
		t.Errorf("Expected log file %s does not exist", expectedPattern)
	}
}

func TestAppLogger_ConcurrentLogging(t *testing.T) {
	// Create temporary directory
	tmpDir, err := ioutil.TempDir("", "test_logs")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	logger := NewAppLogger()
	logger.Initialize()
	defer logger.Close()

	// Test concurrent logging doesn't cause race conditions
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				logger.LogAppSubmission("Concurrent log %d-%d", id, j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}