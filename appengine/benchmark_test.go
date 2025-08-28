package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
)

// Benchmark tests for performance-critical operations

func BenchmarkListAppsHandler(b *testing.B) {
	// Setup registry with multiple apps
	setupTestRegistry()
	registryMutex.Lock()
	for i := 0; i < 100; i++ {
		appID := fmt.Sprintf("bench-app-%d", i)
		registry[appID] = &App{
			AppID:          appID,
			Version:        "1.0.0",
			Runtime:        "wasm",
			Tools:          []ToolInfo{{"tool1", "json"}, {"tool2", "xml"}},
			ArtifactURI:    "/tmp/test.wasm",
			SourceLanguage: "rust",
		}
	}
	registryMutex.Unlock()
	defer cleanupTestRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/list_apps", nil)
		w := httptest.NewRecorder()
		listAppsHandler(w, req)
	}
}

func BenchmarkJSONMarshalApp(b *testing.B) {
	app := &App{
		AppID:          "benchmark-app",
		Version:        "1.0.0",
		Runtime:        "wasm",
		Tools:          []ToolInfo{{"tool1", "json"}, {"tool2", "xml"}, {"tool3", "yaml"}},
		ArtifactURI:    "/tmp/benchmark.wasm",
		SourceLanguage: "rust",
		Files: []File{
			{Name: "main.rs", Content: "fn main() { println!(\"Hello, world!\"); }"},
			{Name: "lib.rs", Content: "pub fn test() -> i32 { 42 }"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(app)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONUnmarshalAppRequest(b *testing.B) {
	request := AppRequest{
		AppID:   "benchmark-app",
		Version: "1.0.0",
		Runtime: "wasm",
		Tools:   []ToolInfo{{"tool1", "json"}, {"tool2", "xml"}, {"tool3", "yaml"}},
		AppSrc:  "struct BenchmarkApp; impl ArcadiaApp for BenchmarkApp { fn handle_tool(&mut self, tool: &str, data: Option<serde_json::Value>, db: &DatabaseConnection) -> Result<serde_json::Value, String> { Ok(serde_json::json!({\"result\": \"success\"})) } fn get_available_tools(&self) -> Vec<&'static str> { vec![\"tool1\", \"tool2\", \"tool3\"] } }",
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var req AppRequest
		err := json.Unmarshal(jsonData, &req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRunToolRequestHandling(b *testing.B) {
	setupTestRegistry()
	defer cleanupTestRegistry()

	request := RunToolRequest{
		AppID:    "test-app",
		ToolName: "test-tool",
		Input:    json.RawMessage(`{"benchmark": true, "iterations": 1000}`),
	}

	body, _ := json.Marshal(request)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/run_tool", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		
		// This will fail at WASM loading, but we're benchmarking the request parsing
		runToolHandler(w, req)
	}
}

func BenchmarkGetProjectNameFromCargo(b *testing.B) {
	cargoContent := `[package]
name = "benchmark-project"
version = "1.0.0"
edition = "2021"

[dependencies]
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"

[lib]
crate-type = ["cdylib"]`

	tmpDir := b.TempDir()
	cargoPath := fmt.Sprintf("%s/Cargo.toml", tmpDir)
	if err := writeFile(cargoPath, cargoContent); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := getProjectNameFromCargo(cargoPath)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper function for benchmarks
func writeFile(path, content string) error {
	return writeFileBytes(path, []byte(content))
}

func writeFileBytes(path string, content []byte) error {
	// Simple file write implementation for benchmarks
	file, err := createFile(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	_, err = file.Write(content)
	return err
}

func createFile(path string) (*os.File, error) {
	return os.Create(path)
}