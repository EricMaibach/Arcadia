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
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Test setup and teardown helpers
func setupTestDB(t *testing.T) *sql.DB {
	testAppDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test app database: %v", err)
	}

	testSystemDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test system database: %v", err)
	}

	// Set global databases for testing
	appDB = testAppDB
	systemDB = testSystemDB

	// Create app tables
	if err := createAppTables(); err != nil {
		t.Fatalf("Failed to create test app tables: %v", err)
	}

	// Create system tables for schedules
	if err := createSystemTables(); err != nil {
		t.Fatalf("Failed to create test system tables: %v", err)
	}

	return testAppDB
}

func cleanupTestDB(_ *sql.DB) {
	if appDB != nil {
		appDB.Close()
		appDB = nil
	}
	if systemDB != nil {
		systemDB.Close()
		systemDB = nil
	}
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

func setupTestSchedules() {
	schedulesMutex.Lock()
	defer schedulesMutex.Unlock()
	schedules = make(map[string]*AppSchedule)
}

func cleanupTestSchedules() {
	schedulesMutex.Lock()
	defer schedulesMutex.Unlock()
	schedules = make(map[string]*AppSchedule)
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
			expected:    "test_project",
			expectError: false,
		},
		{
			name: "cargo.toml with quotes",
			cargoContent: `[package]
name = "quoted-project"
version = "1.0.0"`,
			expected:    "quoted_project",
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

func TestCreateTables(t *testing.T) {
	testDB := setupTestDB(t)
	defer cleanupTestDB(testDB)

	// Test app tables
	appTables := []string{"app_data", "app_logs", "sessions"}
	
	for _, table := range appTables {
		query := fmt.Sprintf("SELECT name FROM sqlite_master WHERE type='table' AND name='%s'", table)
		var name string
		err := appDB.QueryRow(query).Scan(&name)
		if err != nil {
			t.Errorf("App table %s was not created: %v", table, err)
		}
	}

	// Test system tables
	systemTables := []string{"app_schedules", "scheduled_runs"}
	
	for _, table := range systemTables {
		query := fmt.Sprintf("SELECT name FROM sqlite_master WHERE type='table' AND name='%s'", table)
		var name string
		err := systemDB.QueryRow(query).Scan(&name)
		if err != nil {
			t.Errorf("System table %s was not created: %v", table, err)
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
			result, err := appDB.Exec(tt.stmt)
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

// Scheduled Run Tests

func TestScheduleAppRunHandler(t *testing.T) {
	testDB := setupTestDB(t)
	defer cleanupTestDB(testDB)
	setupTestRegistry()
	defer cleanupTestRegistry()

	tests := []struct {
		name           string
		request        ScheduleRequest
		expectedStatus int
	}{
		{
			name: "valid one-time schedule",
			request: ScheduleRequest{
				AppID:         "test-app",
				ToolName:      "test-tool",
				Input:         json.RawMessage(`{"test": "data"}`),
				ScheduleType:  ScheduleTypeOneTime,
				ScheduledTime: time.Now().Add(1 * time.Hour),
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "valid recurring schedule",
			request: ScheduleRequest{
				AppID:         "test-app",
				ToolName:      "test-tool",
				Input:         json.RawMessage(`{"test": "data"}`),
				ScheduleType:  ScheduleTypeRecurring,
				ScheduledTime: time.Now().Add(1 * time.Hour),
				Recurrence: &RecurrenceRule{
					Interval: 1,
					Unit:     "hours",
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "missing app id",
			request: ScheduleRequest{
				AppID:         "",
				ToolName:      "test-tool",
				Input:         json.RawMessage(`{"test": "data"}`),
				ScheduleType:  ScheduleTypeOneTime,
				ScheduledTime: time.Now().Add(1 * time.Hour),
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing tool name",
			request: ScheduleRequest{
				AppID:         "test-app",
				ToolName:      "",
				Input:         json.RawMessage(`{"test": "data"}`),
				ScheduleType:  ScheduleTypeOneTime,
				ScheduledTime: time.Now().Add(1 * time.Hour),
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "past scheduled time",
			request: ScheduleRequest{
				AppID:         "test-app",
				ToolName:      "test-tool",
				Input:         json.RawMessage(`{"test": "data"}`),
				ScheduleType:  ScheduleTypeOneTime,
				ScheduledTime: time.Now().Add(-1 * time.Hour),
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "nonexistent app",
			request: ScheduleRequest{
				AppID:         "nonexistent-app",
				ToolName:      "test-tool",
				Input:         json.RawMessage(`{"test": "data"}`),
				ScheduleType:  ScheduleTypeOneTime,
				ScheduledTime: time.Now().Add(1 * time.Hour),
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "recurring without recurrence",
			request: ScheduleRequest{
				AppID:         "test-app",
				ToolName:      "test-tool",
				Input:         json.RawMessage(`{"test": "data"}`),
				ScheduleType:  ScheduleTypeRecurring,
				ScheduledTime: time.Now().Add(1 * time.Hour),
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/schedule_app_run", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			scheduleAppRunHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
				t.Logf("Response: %s", w.Body.String())
			}
		})
	}
}

func TestListSchedulesHandler(t *testing.T) {
	testDB := setupTestDB(t)
	defer cleanupTestDB(testDB)
	setupTestRegistry()
	defer cleanupTestRegistry()
	setupTestSchedules()
	defer cleanupTestSchedules()

	// Create a test schedule first
	schedule := &AppSchedule{
		ID:            "test-schedule-1",
		AppID:         "test-app",
		ToolName:      "test-tool",
		Input:         json.RawMessage(`{"test": "data"}`),
		ScheduleType:  ScheduleTypeOneTime,
		ScheduledTime: time.Now().Add(1 * time.Hour),
		IsActive:      true,
		CreatedAt:     time.Now(),
		RunCount:      0,
	}
	
	// Add to both database and memory
	if err := saveScheduleToDatabase(schedule); err != nil {
		t.Fatalf("Failed to save test schedule: %v", err)
	}
	schedulesMutex.Lock()
	schedules[schedule.ID] = schedule
	schedulesMutex.Unlock()

	tests := []struct {
		name           string
		appIdFilter    string
		expectedStatus int
	}{
		{
			name:           "list all schedules",
			appIdFilter:    "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by app id",
			appIdFilter:    "test-app",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by non-existent app",
			appIdFilter:    "non-existent",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/list_schedules"
			if tt.appIdFilter != "" {
				url += "?appId=" + tt.appIdFilter
			}
			
			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			listSchedulesHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var schedules []*AppSchedule
			if err := json.NewDecoder(w.Body).Decode(&schedules); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if tt.appIdFilter == "" && len(schedules) == 0 {
				t.Error("Expected at least one schedule")
			}
			if tt.appIdFilter == "test-app" && len(schedules) == 0 {
				t.Error("Expected to find schedule for test-app")
			}
			if tt.appIdFilter == "non-existent" && len(schedules) != 0 {
				t.Errorf("Expected no schedules for non-existent app, got %d schedules", len(schedules))
				for _, sched := range schedules {
					t.Logf("Found schedule: ID=%s, AppID=%s", sched.ID, sched.AppID)
				}
			}
		})
	}
}

func TestGetScheduleHandler(t *testing.T) {
	testDB := setupTestDB(t)
	defer cleanupTestDB(testDB)
	setupTestSchedules()
	defer cleanupTestSchedules()

	// Create a test schedule first
	schedule := &AppSchedule{
		ID:            "test-schedule-get",
		AppID:         "test-app",
		ToolName:      "test-tool",
		Input:         json.RawMessage(`{"test": "data"}`),
		ScheduleType:  ScheduleTypeOneTime,
		ScheduledTime: time.Now().Add(1 * time.Hour),
		IsActive:      true,
		CreatedAt:     time.Now(),
		RunCount:      0,
	}
	
	// Add to both database and memory
	if err := saveScheduleToDatabase(schedule); err != nil {
		t.Fatalf("Failed to save test schedule: %v", err)
	}
	schedulesMutex.Lock()
	schedules[schedule.ID] = schedule
	schedulesMutex.Unlock()

	tests := []struct {
		name           string
		scheduleId     string
		expectedStatus int
	}{
		{
			name:           "get existing schedule",
			scheduleId:     "test-schedule-get",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "get non-existent schedule",
			scheduleId:     "non-existent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/get_schedule?id=" + tt.scheduleId
			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			getScheduleHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var returnedSchedule AppSchedule
				if err := json.NewDecoder(w.Body).Decode(&returnedSchedule); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if returnedSchedule.ID != tt.scheduleId {
					t.Errorf("Expected schedule ID %s, got %s", tt.scheduleId, returnedSchedule.ID)
				}
			}
		})
	}
}

func TestDeleteScheduleHandler(t *testing.T) {
	testDB := setupTestDB(t)
	defer cleanupTestDB(testDB)
	setupTestSchedules()
	defer cleanupTestSchedules()

	// Create a test schedule first
	schedule := &AppSchedule{
		ID:            "test-schedule-delete",
		AppID:         "test-app",
		ToolName:      "test-tool",
		Input:         json.RawMessage(`{"test": "data"}`),
		ScheduleType:  ScheduleTypeOneTime,
		ScheduledTime: time.Now().Add(1 * time.Hour),
		IsActive:      true,
		CreatedAt:     time.Now(),
		RunCount:      0,
	}
	
	// Add to both database and memory
	if err := saveScheduleToDatabase(schedule); err != nil {
		t.Fatalf("Failed to save test schedule: %v", err)
	}
	schedulesMutex.Lock()
	schedules[schedule.ID] = schedule
	schedulesMutex.Unlock()

	tests := []struct {
		name           string
		scheduleId     string
		expectedStatus int
	}{
		{
			name:           "delete existing schedule",
			scheduleId:     "test-schedule-delete",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "delete non-existent schedule",
			scheduleId:     "non-existent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/delete_schedule?id=" + tt.scheduleId
			req := httptest.NewRequest("DELETE", url, nil)
			w := httptest.NewRecorder()

			deleteScheduleHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestUpdateScheduleHandler(t *testing.T) {
	testDB := setupTestDB(t)
	defer cleanupTestDB(testDB)
	setupTestSchedules()
	defer cleanupTestSchedules()

	// Create a test schedule first
	schedule := &AppSchedule{
		ID:            "test-schedule-update",
		AppID:         "test-app",
		ToolName:      "test-tool",
		Input:         json.RawMessage(`{"test": "data"}`),
		ScheduleType:  ScheduleTypeOneTime,
		ScheduledTime: time.Now().Add(1 * time.Hour),
		IsActive:      true,
		CreatedAt:     time.Now(),
		RunCount:      0,
	}
	
	// Add to both database and memory
	if err := saveScheduleToDatabase(schedule); err != nil {
		t.Fatalf("Failed to save test schedule: %v", err)
	}
	schedulesMutex.Lock()
	schedules[schedule.ID] = schedule
	schedulesMutex.Unlock()

	tests := []struct {
		name           string
		scheduleId     string
		updateData     map[string]any
		expectedStatus int
	}{
		{
			name:       "update existing schedule",
			scheduleId: "test-schedule-update",
			updateData: map[string]any{
				"isActive": false,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:       "update non-existent schedule",
			scheduleId: "non-existent",
			updateData: map[string]any{
				"isActive": false,
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.updateData)
			url := "/update_schedule?id=" + tt.scheduleId
			req := httptest.NewRequest("PUT", url, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			updateScheduleHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestListScheduledRunsHandler(t *testing.T) {
	testDB := setupTestDB(t)
	defer cleanupTestDB(testDB)

	// Create a test scheduled run first
	run := &ScheduledRun{
		ID:         "test-run-1",
		ScheduleID: "test-schedule-1",
		AppID:      "test-app",
		ToolName:   "test-tool",
		Input:      json.RawMessage(`{"test": "data"}`),
		StartedAt:  time.Now(),
		Status:     "completed",
		Output:     "test output",
	}

	// Save to database (assuming we have a helper function)
	systemDBMutex.Lock()
	_, err := systemDB.Exec(`INSERT INTO scheduled_runs 
		(id, schedule_id, app_id, tool_name, input_data, started_at, status, output) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.ScheduleID, run.AppID, run.ToolName, string(run.Input), 
		run.StartedAt.Format(time.RFC3339), run.Status, run.Output)
	systemDBMutex.Unlock()
	
	if err != nil {
		t.Fatalf("Failed to save test scheduled run: %v", err)
	}

	tests := []struct {
		name           string
		params         map[string]string
		expectedStatus int
	}{
		{
			name:           "list all runs",
			params:         map[string]string{},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by app id",
			params:         map[string]string{"appId": "test-app"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by status",
			params:         map[string]string{"status": "completed"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by schedule id",
			params:         map[string]string{"scheduleId": "test-schedule-1"},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/list_scheduled_runs"
			if len(tt.params) > 0 {
				url += "?"
				for key, value := range tt.params {
					url += key + "=" + value + "&"
				}
				url = strings.TrimSuffix(url, "&")
			}

			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			listScheduledRunsHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var runs []*ScheduledRun
			if err := json.NewDecoder(w.Body).Decode(&runs); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Basic validation - should have at least one run for most cases
			if len(tt.params) == 0 && len(runs) == 0 {
				t.Error("Expected at least one scheduled run")
			}
		})
	}
}

// Validation Function Tests

func TestValidateRecurrence(t *testing.T) {
	tests := []struct {
		name        string
		recurrence  *RecurrenceRule
		expectError bool
	}{
		{
			name: "valid hourly recurrence",
			recurrence: &RecurrenceRule{
				Interval: 1,
				Unit:     "hours",
			},
			expectError: false,
		},
		{
			name: "valid weekly recurrence with days",
			recurrence: &RecurrenceRule{
				Interval:   1,
				Unit:       "weeks",
				DaysOfWeek: []int{1, 3, 5}, // Mon, Wed, Fri
			},
			expectError: false,
		},
		{
			name: "invalid unit",
			recurrence: &RecurrenceRule{
				Interval: 1,
				Unit:     "invalid",
			},
			expectError: true,
		},
		{
			name: "invalid interval",
			recurrence: &RecurrenceRule{
				Interval: 0,
				Unit:     "hours",
			},
			expectError: true,
		},
		{
			name: "invalid days of week",
			recurrence: &RecurrenceRule{
				Interval:   1,
				Unit:       "weeks",
				DaysOfWeek: []int{7, 8}, // Invalid days
			},
			expectError: true,
		},
		{
			name: "end date in past",
			recurrence: &RecurrenceRule{
				Interval: 1,
				Unit:     "hours",
				EndDate:  &[]time.Time{time.Now().Add(-1 * time.Hour)}[0],
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRecurrence(tt.recurrence)
			
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}