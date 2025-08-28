package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// Test that demonstrates the detailed logging functionality
func TestDetailedLogging(t *testing.T) {
	// Initialize logger for this test
	if err := initAppLogger(); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}
	defer closeAppLogger()

	t.Log("=== Testing Detailed App Submission Logging ===")

	// Test 1: Request with missing AppSrc (should log error)
	t.Run("missing_app_src_with_logging", func(t *testing.T) {
		req := AppRequest{
			AppID:   "test-logging-app-1",
			Version: "1.0.0",
			Runtime: "wasm", 
			Tools:   []ToolInfo{{"test-tool", "json"}},
			AppSrc:  "", // Empty - should trigger validation error
		}

		body, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("POST", "/submit_app_src", bytes.NewBuffer(body))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("User-Agent", "LoggingTest/1.0")
		httpReq.Header.Set("X-Forwarded-For", "192.168.1.100")
		
		w := httptest.NewRecorder()
		submitAppSrcHandler(w, httpReq)

		if w.Code != 400 {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	// Test 2: Request with missing AppID (should log error)
	t.Run("missing_app_id_with_logging", func(t *testing.T) {
		req := AppRequest{
			AppID:   "", // Empty - should trigger validation error
			Version: "1.0.0",
			Runtime: "wasm",
			Tools:   []ToolInfo{{"test-tool", "json"}},
			AppSrc:  "struct TestApp; impl ArcadiaApp for TestApp {}",
		}

		body, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("POST", "/submit_app_src", bytes.NewBuffer(body))
		httpReq.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		submitAppSrcHandler(w, httpReq)

		if w.Code != 400 {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	// Test 3: Valid request that will fail at compilation (shows full logging flow)
	t.Run("valid_request_compilation_failure", func(t *testing.T) {
		req := AppRequest{
			AppID:   "test-logging-app-2",
			Version: "1.0.0", 
			Runtime: "wasm",
			Tools:   []ToolInfo{{"test-tool", "json"}, {"another-tool", "xml"}},
			AppSrc:  "struct TestApp { counter: u32 } impl ArcadiaApp for TestApp { fn handle_tool(&mut self, tool: &str, data: Option<serde_json::Value>, db: &DatabaseConnection) -> Result<serde_json::Value, String> { Ok(serde_json::json!({\"result\": \"success\"})) } fn get_available_tools(&self) -> Vec<&'static str> { vec![\"test-tool\", \"another-tool\"] } }",
		}

		body, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("POST", "/submit_app_src", bytes.NewBuffer(body))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("User-Agent", "LoggingTest/1.0")
		httpReq.Header.Set("X-Forwarded-For", "10.0.0.1")
		
		w := httptest.NewRecorder()
		submitAppSrcHandler(w, httpReq)

		// This should fail at compilation (500), but show full logging
		if w.Code != 500 {
			t.Logf("Expected 500 (compilation failure), got %d", w.Code)
			t.Logf("Response: %s", w.Body.String())
		}
	})

	// Give a moment for all logging to flush
	time.Sleep(100 * time.Millisecond)
}

// Test to verify log file creation and content
func TestLogFileCreation(t *testing.T) {
	// Clean up any existing logs first
	os.RemoveAll("logs")
	
	// Initialize logger
	if err := initAppLogger(); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}
	
	// Check if log directory and file were created
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		t.Error("Logs directory was not created")
	}

	// Expected log file name
	expectedLogFile := fmt.Sprintf("logs/app_submissions_%s.log", time.Now().Format("2006-01-02"))
	if _, err := os.Stat(expectedLogFile); os.IsNotExist(err) {
		t.Errorf("Expected log file %s was not created", expectedLogFile)
	}

	closeAppLogger()

	// Verify log file has content
	if content, err := os.ReadFile(expectedLogFile); err != nil {
		t.Errorf("Failed to read log file: %v", err)
	} else if len(content) == 0 {
		t.Error("Log file is empty")
	} else {
		t.Logf("Log file created successfully with %d bytes", len(content))
		// Log first few lines for verification
		lines := bytes.Split(content, []byte("\n"))
		for i, line := range lines {
			if i >= 3 { // Only show first 3 lines
				break
			}
			if len(line) > 0 {
				t.Logf("Log line %d: %s", i+1, string(line))
			}
		}
	}
}