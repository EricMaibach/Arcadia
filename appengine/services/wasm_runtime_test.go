package services

import (
	"encoding/json"
	"testing"
)

// Mock implementations for testing
type mockWasmConfigManager struct {
	claudeService *ClaudeService
}

func (m *mockWasmConfigManager) GetClaudeService() *ClaudeService {
	return m.claudeService
}

// Since we need to use the actual RegistryManager type, create a simple helper
func createMockRegistryManager() *RegistryManager {
	executeFunc := func(appID, toolName string, input json.RawMessage) (string, error) {
		return "mock result", nil
	}
	return NewRegistryManager(executeFunc)
}

func TestWasmConfigManager_Interface(t *testing.T) {
	// Test that mockWasmConfigManager implements WasmConfigManager interface
	var _ WasmConfigManager = (*mockWasmConfigManager)(nil)
}

func TestNewWasmRuntime(t *testing.T) {
	configMgr := &mockWasmConfigManager{}
	registryMgr := createMockRegistryManager()
	dbMgr := NewDatabaseManager()

	runtime := NewWasmRuntime(configMgr, registryMgr, dbMgr)

	if runtime == nil {
		t.Fatal("NewWasmRuntime returned nil")
	}

	if runtime.engine == nil {
		t.Error("WASM engine not initialized")
	}

	if runtime.configManager != configMgr {
		t.Error("Config manager not set correctly")
	}

	if runtime.registryManager == nil {
		t.Error("Registry manager not set correctly")
	}

	if runtime.databaseManager == nil {
		t.Error("Database manager not set correctly")
	}
}

func TestWasmRuntime_GetEngine(t *testing.T) {
	configMgr := &mockWasmConfigManager{}
	registryMgr := createMockRegistryManager()
	dbMgr := NewDatabaseManager()
	runtime := NewWasmRuntime(configMgr, registryMgr, dbMgr)

	engine := runtime.GetEngine()
	if engine == nil {
		t.Error("GetEngine returned nil")
	}

	if engine != runtime.engine {
		t.Error("GetEngine returned wrong engine instance")
	}
}

func TestWasmRuntime_ExecuteAppTool_AppNotFound(t *testing.T) {
	configMgr := &mockWasmConfigManager{}
	
	// Create a mock registry manager without any registered apps
	registryMgr := createMockRegistryManager()

	dbMgr := NewDatabaseManager()
	runtime := NewWasmRuntime(configMgr, registryMgr, dbMgr)

	_, err := runtime.ExecuteAppTool("nonexistent-app", "test-tool", json.RawMessage(`{}`))
	
	if err == nil {
		t.Error("Expected error for non-existent app")
	}

	// Note: We can't easily test the actual error message due to the type assertion
	// in the actual code that would fail, but we can test that an error occurs
}

func TestWasmRuntime_ExecuteAppTool_ToolNotFound(t *testing.T) {
	configMgr := &mockWasmConfigManager{}
	
	// Create an app with tools but not the one we're looking for
	app := &App{
		AppID: "test-app",
		Tools: []ToolInfo{
			{Name: "other-tool", InputFormat: "json"},
		},
		ArtifactURI: "test.wasm",
	}

	registryMgr := createMockRegistryManager()
	registryMgr.GetRegistry().RegisterApp(app)

	dbMgr := NewDatabaseManager()
	runtime := NewWasmRuntime(configMgr, registryMgr, dbMgr)

	_, err := runtime.ExecuteAppTool("test-app", "nonexistent-tool", json.RawMessage(`{}`))
	
	if err == nil {
		t.Error("Expected error for non-existent tool")
	}

	// The error message should mention the tool not being found
	// But we can't easily test the exact message due to the implementation
}

func TestWasmRuntime_ExecuteAppTool_WasmFileNotFound(t *testing.T) {
	configMgr := &mockWasmConfigManager{}
	
	// Create an app with a non-existent WASM file
	app := &App{
		AppID: "test-app",
		Tools: []ToolInfo{
			{Name: "test-tool", InputFormat: "json"},
		},
		ArtifactURI: "/nonexistent/path/test.wasm",
	}

	registryMgr := createMockRegistryManager()
	registryMgr.GetRegistry().RegisterApp(app)

	dbMgr := NewDatabaseManager()
	runtime := NewWasmRuntime(configMgr, registryMgr, dbMgr)

	_, err := runtime.ExecuteAppTool("test-app", "test-tool", json.RawMessage(`{}`))
	
	if err == nil {
		t.Error("Expected error for non-existent WASM file")
	}
	
	// The error should be about loading the WASM artifact
	// But we can't test the exact message without a valid file path
}

func TestInitializeWasmRuntime(t *testing.T) {
	// Save the original global runtime
	originalRuntime := globalRuntime
	defer func() { globalRuntime = originalRuntime }()

	configMgr := &mockWasmConfigManager{}
	registryMgr := createMockRegistryManager()

	dbMgr := NewDatabaseManager()
	InitializeWasmRuntime(configMgr, registryMgr, dbMgr)

	if globalRuntime == nil {
		t.Error("Global runtime not initialized")
	}

	if globalRuntime.configManager != configMgr {
		t.Error("Global runtime config manager not set correctly")
	}
}

func TestGetGlobalWasmRuntime(t *testing.T) {
	// Save the original global runtime
	originalRuntime := globalRuntime
	defer func() { globalRuntime = originalRuntime }()

	// Test panic when not initialized
	globalRuntime = nil
	
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when global runtime not initialized")
		}
	}()
	
	GetGlobalWasmRuntime()
}

func TestGetGlobalWasmRuntime_Initialized(t *testing.T) {
	// Save the original global runtime
	originalRuntime := globalRuntime
	defer func() { globalRuntime = originalRuntime }()

	configMgr := &mockWasmConfigManager{}
	registryMgr := createMockRegistryManager()

	dbMgr := NewDatabaseManager()
	InitializeWasmRuntime(configMgr, registryMgr, dbMgr)

	runtime := GetGlobalWasmRuntime()
	
	if runtime == nil {
		t.Error("GetGlobalWasmRuntime returned nil")
	}

	if runtime != globalRuntime {
		t.Error("GetGlobalWasmRuntime returned wrong instance")
	}
}

func TestWasmExecuteAppTool(t *testing.T) {
	// Save the original global runtime
	originalRuntime := globalRuntime
	defer func() { globalRuntime = originalRuntime }()

	// Test panic when not initialized
	globalRuntime = nil
	
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when calling WasmExecuteAppTool with uninitialized runtime")
		}
	}()
	
	WasmExecuteAppTool("test-app", "test-tool", json.RawMessage(`{}`))
}

func TestWasmExecuteAppTool_Initialized(t *testing.T) {
	// Save the original global runtime
	originalRuntime := globalRuntime
	defer func() { globalRuntime = originalRuntime }()

	configMgr := &mockWasmConfigManager{}
	registryMgr := createMockRegistryManager()

	dbMgr := NewDatabaseManager()
	InitializeWasmRuntime(configMgr, registryMgr, dbMgr)

	// This should call the global runtime's ExecuteAppTool method
	_, err := WasmExecuteAppTool("nonexistent-app", "test-tool", json.RawMessage(`{}`))
	
	// Should get an error because the app doesn't exist
	if err == nil {
		t.Error("Expected error for non-existent app")
	}
}

func TestWasmRuntime_Structs(t *testing.T) {
	// Test WasmRuntime struct construction
	configMgr := &mockWasmConfigManager{}
	registryMgr := createMockRegistryManager()

	runtime := WasmRuntime{
		configManager:   configMgr,
		registryManager: registryMgr,
	}

	if runtime.configManager != configMgr {
		t.Error("WasmRuntime configManager not set correctly")
	}

	if runtime.registryManager == nil {
		t.Error("WasmRuntime registryManager not set correctly")
	}

	// Test that engine can be set
	if runtime.engine != nil {
		t.Error("WasmRuntime engine should be nil initially")
	}
}

// Note: Testing the actual WASM execution and host functions would require:
// 1. Valid WASM bytecode
// 2. Proper wasmtime setup
// 3. Mock database connections
// 4. Mock Claude service
// This is complex integration testing that would be better suited for integration tests
// rather than unit tests. The tests above cover the main logic paths and error conditions.

func TestWasmRuntime_ClaudeQuery_NoService(t *testing.T) {
	// Test the claudeQuery method when Claude service is not available
	configMgr := &mockWasmConfigManager{
		claudeService: nil, // No Claude service
	}
	registryMgr := createMockRegistryManager()
	
	dbMgr := NewDatabaseManager()
	runtime := NewWasmRuntime(configMgr, registryMgr, dbMgr)

	// We can't easily test the claudeQuery method directly since it requires
	// a wasmtime.Caller, but we can verify the service availability check
	service := runtime.configManager.GetClaudeService()
	if service != nil {
		t.Error("Expected nil Claude service")
	}
}

func TestWasmRuntime_ClaudeQuery_WithService(t *testing.T) {
	// Create a mock Claude service
	mockService := &ClaudeService{
		config: ClaudeConfig{
			APIKey:  "test-key",
			BaseURL: "https://api.anthropic.com",
		},
	}

	configMgr := &mockWasmConfigManager{
		claudeService: mockService,
	}
	registryMgr := createMockRegistryManager()
	
	dbMgr := NewDatabaseManager()
	runtime := NewWasmRuntime(configMgr, registryMgr, dbMgr)

	// Verify the service is available
	service := runtime.configManager.GetClaudeService()
	if service != mockService {
		t.Error("Claude service not set correctly")
	}
}