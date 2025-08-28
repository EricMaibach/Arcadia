package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// Test setup and teardown helpers
func setupTestDB(t *testing.T) *sql.DB {
	testDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Set global db for testing
	db = testDB

	// Create default tables
	if err := createDefaultTables(); err != nil {
		t.Fatalf("Failed to create test tables: %v", err)
	}

	return testDB
}

func cleanupTestDB(testDB *sql.DB) {
	if testDB != nil {
		testDB.Close()
	}
	db = nil
}

func setupTestRegistry() {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	registry = map[string]*App{
		"test-app": {
			AppID:          "test-app",
			Version:        "1.0.0",
			Runtime:        "wasm",
			Tools:          []ToolInfo{{"test-tool", "json"}},
			ArtifactURI:    "/tmp/test.wasm",
			SourceLanguage: "rust",
		},
	}
}

func cleanupTestRegistry() {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	registry = make(map[string]*App)
}

// HTTP Handler Tests

func TestListAppsHandler(t *testing.T) {
	tests := []struct {
		name           string
		setupRegistry  func()
		expectedStatus int
		expectedApps   int
	}{
		{
			name: "empty registry",
			setupRegistry: func() {
				cleanupTestRegistry()
			},
			expectedStatus: http.StatusOK,
			expectedApps:   0,
		},
		{
			name: "registry with apps",
			setupRegistry: func() {
				setupTestRegistry()
			},
			expectedStatus: http.StatusOK,
			expectedApps:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupRegistry()
			defer cleanupTestRegistry()

			req := httptest.NewRequest("GET", "/list_apps", nil)
			w := httptest.NewRecorder()

			listAppsHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var apps []*App
			if err := json.NewDecoder(w.Body).Decode(&apps); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if len(apps) != tt.expectedApps {
				t.Errorf("Expected %d apps, got %d", tt.expectedApps, len(apps))
			}
		})
	}
}

func TestRunToolHandler(t *testing.T) {
	setupTestRegistry()
	defer cleanupTestRegistry()

	// Create a temporary WASM file for testing
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "test.wasm")
	if err := os.WriteFile(wasmPath, []byte("fake wasm content"), 0644); err != nil {
		t.Fatalf("Failed to create test WASM file: %v", err)
	}

	// Update registry with test WASM path
	registryMutex.Lock()
	registry["test-app"].ArtifactURI = wasmPath
	registryMutex.Unlock()

	tests := []struct {
		name           string
		request        RunToolRequest
		expectedStatus int
	}{
		{
			name: "missing app",
			request: RunToolRequest{
				AppID:    "nonexistent-app",
				ToolName: "test-tool",
				Input:    json.RawMessage(`{"test": "data"}`),
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "valid app but WASM loading will fail",
			request: RunToolRequest{
				AppID:    "test-app",
				ToolName: "test-tool",
				Input:    json.RawMessage(`{"test": "data"}`),
			},
			expectedStatus: http.StatusInternalServerError, // Will fail on WASM compilation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/run_tool", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			runToolHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestSubmitAppSrcHandler(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	tests := []struct {
		name           string
		request        AppRequest
		expectedStatus int
	}{
		{
			name: "missing app source",
			request: AppRequest{
				AppID:   "test-app",
				Version: "1.0.0",
				Runtime: "wasm",
				Tools:   []ToolInfo{{"test", "json"}},
				AppSrc:  "",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "valid request",
			request: AppRequest{
				AppID:   "test-app",
				Version: "1.0.0",
				Runtime: "wasm",
				Tools:   []ToolInfo{{"test", "json"}},
				AppSrc:  "struct TestApp; impl ArcadiaApp for TestApp { /* implementation */ }",
			},
			expectedStatus: http.StatusInternalServerError, // Will fail on Rust compilation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/submit_app_src", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			submitAppSrcHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// Utility Function Tests

func TestGenerateRustProjectFromTrait(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a dummy template file
	templatePath := filepath.Join(tmpDir, "wasm_wrapper_template.rs")
	templateContent := `// Template content
// USER_APP_IMPL_PLACEHOLDER - This will be replaced with user's trait implementation
// End of template`
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to create template file: %v", err)
	}

	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	req := AppRequest{
		AppID:   "test-app",
		Version: "1.0.0",
		Runtime: "wasm",
		Tools:   []ToolInfo{{"test", "json"}},
		AppSrc:  "struct TestApp; impl ArcadiaApp for TestApp {}",
	}

	buildDir := filepath.Join(tmpDir, "build")
	err := generateRustProjectFromTrait(req, buildDir)

	if err != nil {
		t.Errorf("generateRustProjectFromTrait failed: %v", err)
	}

	// Check if files were created
	libPath := filepath.Join(buildDir, "src", "lib.rs")
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		t.Error("lib.rs was not created")
	}

	cargoPath := filepath.Join(buildDir, "Cargo.toml")
	if _, err := os.Stat(cargoPath); os.IsNotExist(err) {
		t.Error("Cargo.toml was not created")
	}

	// Check if app source was injected
	libContent, err := os.ReadFile(libPath)
	if err != nil {
		t.Fatalf("Failed to read lib.rs: %v", err)
	}

	if !strings.Contains(string(libContent), req.AppSrc) {
		t.Error("App source was not injected into lib.rs")
	}
}

func TestGetProjectNameFromCargo(t *testing.T) {
	tests := []struct {
		name         string
		cargoContent string
		expected     string
		expectError  bool
	}{
		{
			name: "valid cargo.toml",
			cargoContent: `[package]
name = "test-project"
version = "1.0.0"`,
			expected:    "test-project",
			expectError: false,
		},
		{
			name: "cargo.toml with quotes",
			cargoContent: `[package]
name = "quoted-project"
version = "1.0.0"`,
			expected:    "quoted-project",
			expectError: false,
		},
		{
			name: "missing name field",
			cargoContent: `[package]
version = "1.0.0"`,
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "Cargo.toml")
			if err := os.WriteFile(tmpFile, []byte(tt.cargoContent), 0644); err != nil {
				t.Fatalf("Failed to create test Cargo.toml: %v", err)
			}

			result, err := getProjectNameFromCargo(tmpFile)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")
	testContent := "test file content"

	// Create source file
	if err := os.WriteFile(srcFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Test copy
	err := copyFile(srcFile, dstFile)
	if err != nil {
		t.Errorf("copyFile failed: %v", err)
	}

	// Verify destination file
	content, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("File content mismatch. Expected %q, got %q", testContent, string(content))
	}
}

// Database Tests

func TestCreateDefaultTables(t *testing.T) {
	testDB := setupTestDB(t)
	defer cleanupTestDB(testDB)

	// Tables should already be created by setupTestDB, so let's verify they exist
	tables := []string{"app_data", "app_logs", "sessions"}
	
	for _, table := range tables {
		query := fmt.Sprintf("SELECT name FROM sqlite_master WHERE type='table' AND name='%s'", table)
		var name string
		err := testDB.QueryRow(query).Scan(&name)
		if err != nil {
			t.Errorf("Table %s was not created: %v", table, err)
		}
	}
}

func TestDbExec(t *testing.T) {
	testDB := setupTestDB(t)
	defer cleanupTestDB(testDB)

	tests := []struct {
		name     string
		stmt     string
		expected int32
	}{
		{
			name:     "insert statement",
			stmt:     "INSERT INTO app_data (app_id, key, value) VALUES ('test-app', 'test-key', 'test-value')",
			expected: 1,
		},
		{
			name:     "update statement",
			stmt:     "UPDATE app_data SET value = 'updated-value' WHERE app_id = 'test-app'",
			expected: 1,
		},
		{
			name:     "delete statement", 
			stmt:     "DELETE FROM app_data WHERE app_id = 'test-app'",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := testDB.Exec(tt.stmt)
			if err != nil {
				t.Errorf("Exec failed: %v", err)
				return
			}

			rowsAffected, err := result.RowsAffected()
			if err != nil {
				t.Errorf("Failed to get rows affected: %v", err)
				return
			}

			if int32(rowsAffected) != tt.expected {
				t.Errorf("Expected %d rows affected, got %d", tt.expected, rowsAffected)
			}
		})
	}
}

// Registry Tests

func TestLoadRegistryFileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	err := loadRegistry()
	if err != nil {
		t.Errorf("loadRegistry should not fail when file doesn't exist: %v", err)
	}

	registryMutex.RLock()
	registryLen := len(registry)
	registryMutex.RUnlock()

	if registryLen != 0 {
		t.Errorf("Expected empty registry, got %d apps", registryLen)
	}
}

func TestSaveAndLoadRegistry(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	// Setup test registry
	testApp := &App{
		AppID:          "save-test-app",
		Version:        "1.0.0",
		Runtime:        "wasm",
		Tools:          []ToolInfo{{"tool1", "json"}, {"tool2", "xml"}},
		ArtifactURI:    "/tmp/test.wasm",
		SourceLanguage: "rust",
	}

	registryMutex.Lock()
	registry = map[string]*App{
		"save-test-app": testApp,
	}
	registryMutex.Unlock()

	// Save registry
	err := saveRegistry()
	if err != nil {
		t.Errorf("saveRegistry failed: %v", err)
	}

	// Clear registry and reload
	registryMutex.Lock()
	registry = make(map[string]*App)
	registryMutex.Unlock()

	err = loadRegistry()
	if err != nil {
		t.Errorf("loadRegistry failed: %v", err)
	}

	// Verify loaded data
	registryMutex.RLock()
	loadedApp, exists := registry["save-test-app"]
	registryMutex.RUnlock()

	if !exists {
		t.Error("App was not loaded from registry")
		return
	}

	if loadedApp.AppID != testApp.AppID {
		t.Errorf("AppID mismatch. Expected %s, got %s", testApp.AppID, loadedApp.AppID)
	}

	if len(loadedApp.Tools) != len(testApp.Tools) {
		t.Errorf("Tools length mismatch. Expected %d, got %d", len(testApp.Tools), len(loadedApp.Tools))
	}
}