package main

import (
	"encoding/json"
	"testing"
)

// Test JSON marshaling/unmarshaling of our data structures

func TestAppRequestJSONHandling(t *testing.T) {
	original := AppRequest{
		AppID:   "test-app",
		Version: "1.0.0",
		Runtime: "wasm",
		Tools:   []ToolInfo{{"tool1", "json"}, {"tool2", "xml"}},
		AppSrc:  "struct TestApp; impl ArcadiaApp for TestApp {}",
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal AppRequest: %v", err)
	}

	// Unmarshal back
	var unmarshaled AppRequest
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal AppRequest: %v", err)
	}

	// Verify fields
	if unmarshaled.AppID != original.AppID {
		t.Errorf("AppID mismatch: expected %s, got %s", original.AppID, unmarshaled.AppID)
	}
	if unmarshaled.AppSrc != original.AppSrc {
		t.Errorf("AppSrc mismatch: expected %s, got %s", original.AppSrc, unmarshaled.AppSrc)
	}
	if len(unmarshaled.Tools) != len(original.Tools) {
		t.Errorf("Tools length mismatch: expected %d, got %d", len(original.Tools), len(unmarshaled.Tools))
	}
}

func TestRunToolRequestJSONHandling(t *testing.T) {
	original := RunToolRequest{
		AppID:    "test-app",
		ToolName: "test-tool",
		Input:    json.RawMessage(`{"key": "value", "number": 42}`),
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal RunToolRequest: %v", err)
	}

	// Unmarshal back
	var unmarshaled RunToolRequest
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal RunToolRequest: %v", err)
	}

	// Verify fields
	if unmarshaled.AppID != original.AppID {
		t.Errorf("AppID mismatch: expected %s, got %s", original.AppID, unmarshaled.AppID)
	}
	if unmarshaled.ToolName != original.ToolName {
		t.Errorf("ToolName mismatch: expected %s, got %s", original.ToolName, unmarshaled.ToolName)
	}
	// Compare the actual JSON content, not the string representation
	var originalData, unmarshaledData interface{}
	json.Unmarshal(original.Input, &originalData)
	json.Unmarshal(unmarshaled.Input, &unmarshaledData)
	
	originalJSON, _ := json.Marshal(originalData)
	unmarshaledJSON, _ := json.Marshal(unmarshaledData)
	
	if string(originalJSON) != string(unmarshaledJSON) {
		t.Errorf("Input mismatch: expected %s, got %s", string(originalJSON), string(unmarshaledJSON))
	}
}

func TestAppJSONHandling(t *testing.T) {
	original := App{
		AppID:          "test-app",
		Version:        "1.0.0",
		Runtime:        "wasm",
		Tools:          []ToolInfo{{"tool1", "json"}, {"tool2", "xml"}},
		ArtifactURI:    "/tmp/test.wasm",
		SourceLanguage: "rust",
		Files: []File{
			{Name: "main.rs", Content: "fn main() {}"},
			{Name: "lib.rs", Content: "pub fn test() {}"},
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal App: %v", err)
	}

	// Unmarshal back
	var unmarshaled App
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal App: %v", err)
	}

	// Verify fields
	if unmarshaled.AppID != original.AppID {
		t.Errorf("AppID mismatch: expected %s, got %s", original.AppID, unmarshaled.AppID)
	}
	if len(unmarshaled.Files) != len(original.Files) {
		t.Errorf("Files length mismatch: expected %d, got %d", len(original.Files), len(unmarshaled.Files))
	}
	if len(unmarshaled.Files) > 0 && unmarshaled.Files[0].Name != original.Files[0].Name {
		t.Errorf("File name mismatch: expected %s, got %s", original.Files[0].Name, unmarshaled.Files[0].Name)
	}
}

func TestJSONTagsMatchExpectedFormat(t *testing.T) {
	// Test that our JSON tags produce the expected field names
	app := App{
		AppID:          "test",
		Version:        "1.0",
		Runtime:        "wasm",
		Tools:          []ToolInfo{{"tool", "json"}},
		ArtifactURI:    "/tmp/test",
		SourceLanguage: "rust",
	}

	jsonData, err := json.Marshal(app)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	jsonStr := string(jsonData)
	
	// Check for expected JSON field names (camelCase)
	expectedFields := []string{
		`"appId"`,
		`"version"`,
		`"runtime"`,
		`"tools"`,
		`"artifactUri"`,
		`"sourceLanguage"`,
	}

	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("Expected JSON field %s not found in: %s", field, jsonStr)
		}
	}
}

// Helper function for string containment check
func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || 
		(len(str) > len(substr) && (str[:len(substr)] == substr || 
		str[len(str)-len(substr):] == substr || 
		containsSubstring(str, substr))))
}

func containsSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}