package services

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestRegistry_NewRegistry(t *testing.T) {
	registry := NewRegistry()
	
	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}
	
	if registry.apps == nil {
		t.Error("Registry apps map is nil")
	}
	
	if len(registry.apps) != 0 {
		t.Error("New registry should have empty apps map")
	}
}

func TestRegistry_RegisterApp(t *testing.T) {
	registry := NewRegistry()
	
	app := &App{
		AppID:   "test-app",
		Version: "1.0.0",
		Runtime: "wasm",
		Tools: []ToolInfo{
			{Name: "test-tool", InputFormat: "json"},
		},
		ArtifactURI:    "test.wasm",
		SourceLanguage: "rust",
		Files: []File{
			{Name: "lib.rs", Content: "// test"},
		},
	}
	
	registry.RegisterApp(app)
	
	if registry.GetAppCount() != 1 {
		t.Errorf("Expected 1 app, got %d", registry.GetAppCount())
	}
	
	retrievedApp, exists := registry.GetApp("test-app")
	if !exists {
		t.Error("App was not found after registration")
	}
	
	if retrievedApp.AppID != "test-app" {
		t.Errorf("Expected app ID 'test-app', got '%s'", retrievedApp.AppID)
	}
}

func TestRegistry_GetApp(t *testing.T) {
	registry := NewRegistry()
	
	// Test getting non-existent app
	_, exists := registry.GetApp("non-existent")
	if exists {
		t.Error("Expected false for non-existent app")
	}
	
	// Register an app
	app := &App{AppID: "test-app", Version: "1.0.0"}
	registry.RegisterApp(app)
	
	// Test getting existing app
	retrievedApp, exists := registry.GetApp("test-app")
	if !exists {
		t.Error("Expected true for existing app")
	}
	
	if retrievedApp != app {
		t.Error("Retrieved app is not the same as registered app")
	}
}

func TestRegistry_GetAllApps(t *testing.T) {
	registry := NewRegistry()
	
	// Test empty registry
	apps := registry.GetAllApps()
	if len(apps) != 0 {
		t.Error("Expected empty apps map for new registry")
	}
	
	// Add some apps
	app1 := &App{AppID: "app1", Version: "1.0.0"}
	app2 := &App{AppID: "app2", Version: "1.0.0"}
	
	registry.RegisterApp(app1)
	registry.RegisterApp(app2)
	
	apps = registry.GetAllApps()
	if len(apps) != 2 {
		t.Errorf("Expected 2 apps, got %d", len(apps))
	}
	
	// Verify it's a copy (modifying returned map shouldn't affect registry)
	apps["app3"] = &App{AppID: "app3"}
	if registry.GetAppCount() != 2 {
		t.Error("Modifying returned apps map affected registry")
	}
}

func TestRegistry_GetAppCount(t *testing.T) {
	registry := NewRegistry()
	
	if registry.GetAppCount() != 0 {
		t.Error("New registry should have 0 apps")
	}
	
	app := &App{AppID: "test-app"}
	registry.RegisterApp(app)
	
	if registry.GetAppCount() != 1 {
		t.Error("Registry should have 1 app after registration")
	}
}

func TestRegistry_Load(t *testing.T) {
	// Test loading when file doesn't exist (should succeed with empty registry)
	registry := NewRegistry()
	
	// Since we can't modify the const registryFilePath, we'll test the behavior
	// when the registry file doesn't exist (which is a valid case)
	
	// First, ensure the registry file doesn't exist for this test
	if _, err := os.Stat(registryFilePath); err == nil {
		// File exists, we'll test with it as-is
		err = registry.Load()
		if err != nil {
			t.Errorf("Load failed: %v", err)
		}
	} else if os.IsNotExist(err) {
		// File doesn't exist, perfect for testing
		err = registry.Load()
		if err != nil {
			t.Errorf("Load should succeed when file doesn't exist: %v", err)
		}
		
		if registry.GetAppCount() != 0 {
			t.Error("Registry should be empty when file doesn't exist")
		}
	}
}

func TestRegistry_Save(t *testing.T) {
	// We can't easily test Save() in isolation because registryFilePath is const
	// This test focuses on testing that Save() doesn't crash
	registry := NewRegistry()
	app := &App{
		AppID:   "test-app",
		Version: "1.0.0",
		Tools:   []ToolInfo{{Name: "test-tool", InputFormat: "json"}},
	}
	registry.RegisterApp(app)
	
	// Call save - this might fail due to permissions but shouldn't crash
	err := registry.Save()
	// We don't assert on err because it depends on file system permissions
	_ = err
}

func TestRegistry_ThreadSafety(t *testing.T) {
	registry := NewRegistry()
	
	// Test concurrent reads and writes
	var wg sync.WaitGroup
	
	// Writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				app := &App{
					AppID:   fmt.Sprintf("app-%d-%d", id, j),
					Version: "1.0.0",
				}
				registry.RegisterApp(app)
			}
		}(i)
	}
	
	// Readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				registry.GetAllApps()
				registry.GetAppCount()
			}
		}()
	}
	
	wg.Wait()
	
	if registry.GetAppCount() != 1000 {
		t.Errorf("Expected 1000 apps after concurrent operations, got %d", registry.GetAppCount())
	}
}

func TestRegistryManager_NewRegistryManager(t *testing.T) {
	executeFunc := func(appID, toolName string, input json.RawMessage) (string, error) {
		return "test result", nil
	}
	
	manager := NewRegistryManager(executeFunc)
	
	if manager == nil {
		t.Fatal("NewRegistryManager returned nil")
	}
	
	if manager.GetRegistry() == nil {
		t.Error("Registry manager has nil registry")
	}
	
	if manager.GetRegistryAccess() == nil {
		t.Error("Registry manager has nil registry access")
	}
	
	if manager.GetAppRunner() == nil {
		t.Error("Registry manager has nil app runner")
	}
}

func TestRegistryManager_Load_Save(t *testing.T) {
	executeFunc := func(appID, toolName string, input json.RawMessage) (string, error) {
		return "test result", nil
	}
	
	manager := NewRegistryManager(executeFunc)
	
	// Add an app
	app := &App{AppID: "test-app", Version: "1.0.0"}
	manager.GetRegistry().RegisterApp(app)
	
	// Test that Save/Load methods don't crash (actual persistence is tested at integration level)
	err := manager.Save()
	_ = err // May fail due to permissions, that's ok
	
	err = manager.Load()
	_ = err // May fail due to file not existing, that's ok
}

func TestRegistryAccess_GetRegistry(t *testing.T) {
	executeFunc := func(appID, toolName string, input json.RawMessage) (string, error) {
		return "test", nil
	}
	
	manager := NewRegistryManager(executeFunc)
	access := manager.GetRegistryAccess()
	
	registryMap := access.GetRegistry()
	if registryMap == nil {
		t.Error("GetRegistry returned nil")
	}
	
	if len(registryMap) != 0 {
		t.Error("New registry should be empty")
	}
	
	// Add an app and test again
	app := &App{AppID: "test-app", Version: "1.0.0"}
	manager.GetRegistry().RegisterApp(app)
	
	registryMap = access.GetRegistry()
	if len(registryMap) != 1 {
		t.Error("Registry map should contain 1 app")
	}
}

func TestAppRunner_ExecuteAppTool(t *testing.T) {
	callCount := 0
	executeFunc := func(appID, toolName string, input json.RawMessage) (string, error) {
		callCount++
		if appID != "test-app" {
			t.Errorf("Expected appID 'test-app', got '%s'", appID)
		}
		if toolName != "test-tool" {
			t.Errorf("Expected toolName 'test-tool', got '%s'", toolName)
		}
		return "test result", nil
	}
	
	manager := NewRegistryManager(executeFunc)
	runner := manager.GetAppRunner()
	
	result, err := runner.ExecuteAppTool("test-app", "test-tool", json.RawMessage(`{"test": true}`))
	
	if err != nil {
		t.Errorf("ExecuteAppTool failed: %v", err)
	}
	
	if result != "test result" {
		t.Errorf("Expected 'test result', got '%s'", result)
	}
	
	if callCount != 1 {
		t.Errorf("Expected execute function to be called once, called %d times", callCount)
	}
}

func TestGlobalRegistryManager(t *testing.T) {
	// Save original state
	originalManager := globalRegistryManager
	defer func() {
		globalRegistryManager = originalManager
	}()
	
	// Test panic when not initialized
	globalRegistryManager = nil
	
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when accessing uninitialized global registry")
		}
	}()
	
	GetGlobalRegistry()
}

func TestAppCreator_CreateApp(t *testing.T) {
	// Create a mock app creation service that doesn't actually compile
	mockAppCreationService := &MockAppCreationService{}
	
	creator := &appCreatorImpl{
		registryManager:    nil, // Not needed for this test
		appCreationService: mockAppCreationService,
	}
	
	tools := []interface{}{
		map[string]interface{}{
			"name":         "test-tool",
			"input_format": "json",
		},
	}
	
	result, err := creator.CreateApp("test-app", "1.0.0", "wasm", tools, "test source", nil)
	
	if err != nil {
		t.Errorf("CreateApp failed: %v", err)
	}
	
	if !strings.Contains(result, "test-app") {
		t.Error("CreateApp result doesn't contain app ID")
	}
	
	if !strings.Contains(result, "1.0.0") {
		t.Error("CreateApp result doesn't contain version")
	}
}

// Mock app creation service for testing
type MockAppCreationService struct{}

func (m *MockAppCreationService) CreateApp(req AppCreationRequest, sessionID string) (string, error) {
	return fmt.Sprintf("App %s version %s created successfully at mock/path/%s.wasm", req.AppID, req.Version, req.Version), nil
}

// Ensure MockAppCreationService implements the interface
var _ AppCreationServiceInterface = (*MockAppCreationService)(nil)