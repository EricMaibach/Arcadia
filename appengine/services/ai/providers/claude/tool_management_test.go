package claude

import (
	"encoding/json"
	"sync"
	"testing"

	"arcadia/services/ai"
)

// Mock implementations for testing tool management

type mockRegistryAccess struct {
	registry map[string]any
	mutex    sync.RWMutex
}

func (m *mockRegistryAccess) GetRegistry() map[string]any {
	return m.registry
}

func (m *mockRegistryAccess) GetRegistryMutex() *sync.RWMutex {
	return &m.mutex
}

type mockAppRunner struct {
	executions []appExecution
}

type appExecution struct {
	appID    string
	toolName string
	input    json.RawMessage
}

func (m *mockAppRunner) ExecuteAppTool(appID, toolName string, input json.RawMessage) (string, error) {
	m.executions = append(m.executions, appExecution{appID, toolName, input})
	return "mock execution result", nil
}

type mockAppCreator struct {
	creations []appCreation
}

type appCreation struct {
	appID        string
	version      string
	runtime      string
	tools        []any
	appSrc       string
	dependencies map[string]string
}

func (m *mockAppCreator) CreateApp(appID, version, runtime string, tools []any, appSrc string, dependencies map[string]string) (string, error) {
	m.creations = append(m.creations, appCreation{appID, version, runtime, tools, appSrc, dependencies})
	return "mock app created successfully", nil
}

type mockEmbeddingSearch struct {
	searches []string
}

func (m *mockEmbeddingSearch) SearchDocuments(query string, topK int) ([]*ai.DocumentSearchResult, error) {
	m.searches = append(m.searches, query)
	return []*ai.DocumentSearchResult{
		{
			Document: &ai.Document{
				ID:       "doc1",
				FilePath: "/test/file.go",
				Content:  "test content",
			},
			BestScore: 0.9,
		},
	}, nil
}

func (m *mockEmbeddingSearch) SearchDocumentsEnhanced(query string, topK int, config ai.SearchConfig) ([]*ai.EnhancedDocumentSearchResult, error) {
	return nil, nil // Not used in current tests
}

func (m *mockEmbeddingSearch) GetDocument(documentID string) (*ai.Document, error) {
	return &ai.Document{
		ID:       documentID,
		FilePath: "/test/file.go",
		Content:  "test document content",
	}, nil
}

// TestToolManagerCreation tests basic tool manager creation and setup
func TestToolManagerCreation(t *testing.T) {
	config := &ai.AIConfig{
		Provider:    "claude",
		EnableMCP:   true,
		MaxTokens:   4096,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	toolManager := ai.NewToolManager(config)
	if toolManager == nil {
		t.Fatal("Tool manager should not be nil")
	}

	// Test initial state
	tools := toolManager.GetTools()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools initially, got %d", len(tools))
	}
}

// TestStaticToolsLoading tests loading of static MCP tools
func TestStaticToolsLoading(t *testing.T) {
	config := &ai.AIConfig{
		Provider:    "claude",
		EnableMCP:   true,
		MaxTokens:   4096,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	toolManager := ai.NewToolManager(config)
	toolManager.LoadMCPTools()

	tools := toolManager.GetTools()

	// Expected static tools
	expectedTools := []string{
		"list_apps",
		"schedule_app_run",
		"list_schedules",
		"search_documents",
	}

	for _, expectedTool := range expectedTools {
		found := false
		for _, tool := range tools {
			if tool.Name == expectedTool {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected static tool '%s' not found", expectedTool)
		}
	}

	t.Logf("Loaded %d static tools", len(tools))
}

// TestDynamicToolsLoading tests loading of dynamic app tools
func TestDynamicToolsLoading(t *testing.T) {
	config := &ai.AIConfig{
		Provider:    "claude",
		EnableMCP:   true,
		MaxTokens:   4096,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	toolManager := ai.NewToolManager(config)

	// Set up mock registry with test apps
	mockRegistry := &mockRegistryAccess{
		registry: map[string]any{
			"test-app": &ai.App{
				AppID:   "test-app",
				Version: "1.0.0",
				Runtime: "wasm",
				Tools: []ai.AppTool{
					{Name: "test_tool", InputFormat: "json"},
					{Name: "another_tool", InputFormat: "string"},
				},
			},
			"empty-app": &ai.App{
				AppID:   "empty-app",
				Version: "1.0.0",
				Runtime: "wasm",
				Tools:   []ai.AppTool{},
			},
		},
	}

	toolManager.SetRegistryAccess(mockRegistry)
	toolManager.LoadMCPTools()

	tools := toolManager.GetTools()

	// Check for dynamic tools (namespaced with app ID)
	expectedDynamicTools := []string{
		"test-app_test_tool",
		"test-app_another_tool",
	}

	for _, expectedTool := range expectedDynamicTools {
		found := false
		for _, tool := range tools {
			if tool.Name == expectedTool {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected dynamic tool '%s' not found", expectedTool)
		}
	}

	t.Logf("Loaded %d total tools (including dynamic)", len(tools))
}

// TestToolExecution tests execution of different types of tools
func TestToolExecution(t *testing.T) {
	config := &ai.AIConfig{
		Provider:    "claude",
		EnableMCP:   true,
		MaxTokens:   4096,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	toolManager := ai.NewToolManager(config)

	// Set up mocks
	mockRegistry := &mockRegistryAccess{
		registry: map[string]any{
			"test-app": &ai.App{
				AppID:   "test-app",
				Version: "1.0.0",
				Runtime: "wasm",
				Tools: []ai.AppTool{
					{Name: "test_tool", InputFormat: "json"},
				},
			},
		},
	}
	mockAppRunner := &mockAppRunner{}
	mockAppCreator := &mockAppCreator{}
	mockEmbedding := &mockEmbeddingSearch{}

	toolManager.SetRegistryAccess(mockRegistry)
	toolManager.SetAppRunner(mockAppRunner)
	toolManager.SetAppCreator(mockAppCreator)
	toolManager.SetEmbeddingSearch(mockEmbedding)

	// Test dynamic app tool execution
	t.Run("Dynamic app tool", func(t *testing.T) {
		input := map[string]any{"test": "data"}
		result, err := toolManager.ExecuteTool("test-app_test_tool", input)
		if err != nil {
			t.Errorf("Tool execution failed: %v", err)
		}
		if result != "mock execution result" {
			t.Errorf("Expected mock result, got: %s", result)
		}

		// Verify the mock was called correctly
		if len(mockAppRunner.executions) != 1 {
			t.Errorf("Expected 1 execution, got %d", len(mockAppRunner.executions))
		} else {
			exec := mockAppRunner.executions[0]
			if exec.appID != "test-app" {
				t.Errorf("Expected appID 'test-app', got '%s'", exec.appID)
			}
			if exec.toolName != "test_tool" {
				t.Errorf("Expected toolName 'test_tool', got '%s'", exec.toolName)
			}
		}
	})

	// Test list_apps system tool
	t.Run("List apps system tool", func(t *testing.T) {
		result, err := toolManager.ExecuteTool("list_apps", nil)
		if err != nil {
			t.Errorf("List apps tool execution failed: %v", err)
		}

		// Result should be JSON representation of registry
		var registryData map[string]any
		if err := json.Unmarshal([]byte(result), &registryData); err != nil {
			t.Errorf("Result is not valid JSON: %v", err)
		}

		if _, exists := registryData["test-app"]; !exists {
			t.Error("Expected test-app in registry results")
		}
	})

	// Test search_documents system tool
	t.Run("Search documents system tool", func(t *testing.T) {
		input := map[string]any{
			"query": "test query",
			"top_k": 5,
		}

		result, err := toolManager.ExecuteTool("search_documents", input)
		if err != nil {
			t.Errorf("Search documents tool execution failed: %v", err)
		}

		// Result should be JSON with search results
		var searchResults map[string]any
		if err := json.Unmarshal([]byte(result), &searchResults); err != nil {
			t.Errorf("Search result is not valid JSON: %v", err)
		}

		if searchResults["query"] != "test query" {
			t.Error("Search query not preserved in results")
		}

		// Verify mock was called
		if len(mockEmbedding.searches) != 1 {
			t.Errorf("Expected 1 search call, got %d", len(mockEmbedding.searches))
		}
	})

	// Test unknown tool
	t.Run("Unknown tool", func(t *testing.T) {
		_, err := toolManager.ExecuteTool("unknown_tool", nil)
		if err == nil {
			t.Error("Expected error for unknown tool")
		}
	})
}

// TestToolRefresh tests tool refresh functionality
func TestToolRefresh(t *testing.T) {
	config := &ai.AIConfig{
		Provider:    "claude",
		EnableMCP:   true,
		MaxTokens:   4096,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	toolManager := ai.NewToolManager(config)

	// Set up initial registry
	mockRegistry := &mockRegistryAccess{
		registry: map[string]any{
			"app1": &ai.App{
				AppID:   "app1",
				Version: "1.0.0",
				Runtime: "wasm",
				Tools: []ai.AppTool{
					{Name: "tool1", InputFormat: "json"},
				},
			},
		},
	}

	toolManager.SetRegistryAccess(mockRegistry)
	toolManager.LoadMCPTools()

	initialToolCount := len(toolManager.GetTools())

	// Add a new app to registry
	mockRegistry.registry["app2"] = &ai.App{
		AppID:   "app2",
		Version: "1.0.0",
		Runtime: "wasm",
		Tools: []ai.AppTool{
			{Name: "tool2", InputFormat: "json"},
		},
	}

	// Refresh tools
	toolManager.RefreshTools()

	newToolCount := len(toolManager.GetTools())
	if newToolCount <= initialToolCount {
		t.Errorf("Expected tool count to increase after refresh, was %d, now %d", initialToolCount, newToolCount)
	}

	// Check that new tool is available
	tools := toolManager.GetTools()
	found := false
	for _, tool := range tools {
		if tool.Name == "app2_tool2" {
			found = true
			break
		}
	}
	if !found {
		t.Error("New tool not found after refresh")
	}
}

// TestCreateAppTool tests the create_app tool functionality
func TestCreateAppTool(t *testing.T) {
	config := &ai.AIConfig{
		Provider:    "claude",
		EnableMCP:   true,
		MaxTokens:   4096,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	toolManager := ai.NewToolManager(config)
	mockAppCreator := &mockAppCreator{}
	toolManager.SetAppCreator(mockAppCreator)

	// Test valid create app request
	input := map[string]any{
		"appId":   "new-app",
		"version": "1.0.0",
		"runtime": "wasm",
		"tools": []map[string]any{
			{"name": "test_tool", "inputFormat": "json"},
		},
		"appSrc": "use arcadia::prelude::*;\n\n// App implementation",
		"dependencies": map[string]string{
			"serde": "1.0",
		},
	}

	result, err := toolManager.ExecuteTool("create_app", input)
	if err != nil {
		t.Errorf("Create app tool execution failed: %v", err)
	}

	if result != "mock app created successfully" {
		t.Errorf("Expected mock creation result, got: %s", result)
	}

	// Verify mock was called correctly
	if len(mockAppCreator.creations) != 1 {
		t.Errorf("Expected 1 app creation, got %d", len(mockAppCreator.creations))
	} else {
		creation := mockAppCreator.creations[0]
		if creation.appID != "new-app" {
			t.Errorf("Expected appID 'new-app', got '%s'", creation.appID)
		}
		if creation.runtime != "wasm" {
			t.Errorf("Expected runtime 'wasm', got '%s'", creation.runtime)
		}
		if len(creation.tools) != 1 {
			t.Errorf("Expected 1 tool, got %d", len(creation.tools))
		}
	}
}

// TestConcurrentToolAccess tests thread safety of tool operations
func TestConcurrentToolAccess(t *testing.T) {
	config := &ai.AIConfig{
		Provider:    "claude",
		EnableMCP:   true,
		MaxTokens:   4096,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	toolManager := ai.NewToolManager(config)

	// Set up mocks
	mockRegistry := &mockRegistryAccess{
		registry: map[string]any{
			"test-app": &ai.App{
				AppID:   "test-app",
				Version: "1.0.0",
				Runtime: "wasm",
				Tools: []ai.AppTool{
					{Name: "test_tool", InputFormat: "json"},
				},
			},
		},
	}
	toolManager.SetRegistryAccess(mockRegistry)

	// Run concurrent operations
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			// Load tools
			toolManager.LoadMCPTools()

			// Get tools
			tools := toolManager.GetTools()
			if len(tools) == 0 {
				t.Error("Expected some tools to be loaded")
			}

			// Refresh tools
			toolManager.RefreshTools()

			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify tools are still accessible
	tools := toolManager.GetTools()
	if len(tools) == 0 {
		t.Error("Expected tools to be available after concurrent operations")
	}

	t.Logf("Final tool count: %d", len(tools))
}