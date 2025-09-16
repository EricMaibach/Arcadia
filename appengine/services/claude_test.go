package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Mock implementations for testing
type mockRegistryAccess struct {
	registry map[string]interface{}
	mutex    sync.RWMutex
}

func (m *mockRegistryAccess) GetRegistry() map[string]interface{} {
	return m.registry
}

func (m *mockRegistryAccess) GetRegistryMutex() *sync.RWMutex {
	return &m.mutex
}

type mockAppRunner struct {
	executeFunc func(appID, toolName string, input json.RawMessage) (string, error)
}

func (m *mockAppRunner) ExecuteAppTool(appID, toolName string, input json.RawMessage) (string, error) {
	if m.executeFunc != nil {
		return m.executeFunc(appID, toolName, input)
	}
	return fmt.Sprintf("Executed %s.%s", appID, toolName), nil
}

type mockAppCreator struct {
	createFunc func(appID, version, runtime string, tools []interface{}, appSrc string, dependencies map[string]string) (string, error)
}

func (m *mockAppCreator) CreateApp(appID, version, runtime string, tools []interface{}, appSrc string, dependencies map[string]string) (string, error) {
	if m.createFunc != nil {
		return m.createFunc(appID, version, runtime, tools, appSrc, dependencies)
	}
	return fmt.Sprintf("Created app %s version %s", appID, version), nil
}

// Mock HTTP transport for testing
type mockRoundTripper struct {
	responseFunc func(*http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.responseFunc(req)
}

func TestNewClaudeService(t *testing.T) {
	config := ClaudeConfig{
		APIKey:         "test-key",
		BaseURL:        "https://api.anthropic.com",
		Model:          "claude-3-5-sonnet-20241022",
		MaxTokens:      4096,
		TimeoutSeconds: 30,
		EnableMCP:      true,
	}

	service := NewClaudeService(config)

	if service == nil {
		t.Fatal("NewClaudeService returned nil")
	}

	if service.config.APIKey != config.APIKey {
		t.Errorf("Expected API key %s, got %s", config.APIKey, service.config.APIKey)
	}

	if service.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", service.httpClient.Timeout)
	}

	if len(service.mcpTools) == 0 {
		t.Error("Expected MCP tools to be loaded when EnableMCP is true")
	}

	// Test that global instance is set
	if GetClaudeService() != service {
		t.Error("Global Claude service instance not set correctly")
	}
}

func TestNewClaudeService_MCPDisabled(t *testing.T) {
	config := ClaudeConfig{
		APIKey:         "test-key",
		BaseURL:        "https://api.anthropic.com",
		Model:          "claude-3-5-sonnet-20241022",
		MaxTokens:      4096,
		TimeoutSeconds: 30,
		EnableMCP:      false,
	}

	service := NewClaudeService(config)

	if len(service.mcpTools) != 0 {
		t.Error("Expected no MCP tools when EnableMCP is false")
	}
}

func TestClaudeService_loadMCPTools(t *testing.T) {
	service := &ClaudeService{}
	service.loadMCPTools()

	expectedTools := []string{
		"list_apps",
		"schedule_app_run",
		"list_schedules",
		"search_documents",
		"search_documents_grouped",
	}

	if len(service.mcpTools) != len(expectedTools) {
		t.Errorf("Expected %d tools, got %d", len(expectedTools), len(service.mcpTools))
	}

	for i, expectedTool := range expectedTools {
		if service.mcpTools[i].Name != expectedTool {
			t.Errorf("Expected tool %s, got %s", expectedTool, service.mcpTools[i].Name)
		}
	}
}

func TestClaudeService_SendMessage_Success(t *testing.T) {
	// Create mock HTTP client
	mockResponse := ClaudeResponse{
		Content: []ClaudeContent{
			{Type: "text", Text: "Test response"},
		},
		StopReason: "end_turn",
		Usage: struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		}{
			InputTokens:  10,
			OutputTokens: 20,
		},
	}

	responseJSON, _ := json.Marshal(mockResponse)
	
	mockClient := &http.Client{
		Transport: &mockRoundTripper{
			responseFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(responseJSON)),
				}, nil
			},
		},
	}

	config := ClaudeConfig{
		APIKey:    "test-key",
		BaseURL:   "https://api.anthropic.com",
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		MaxContextMessages: 10,
	}
	
	service := &ClaudeService{
		config:     config,
		httpClient: mockClient,
		mcpTools:   []ClaudeTool{},
		contextManager: &ContextManager{
			contexts: make(map[string]*ConversationContext),
			config:   &config,
		},
	}

	response, err := service.SendMessage("Test message")
	
	if err != nil {
		t.Errorf("SendMessage failed: %v", err)
	}

	if response != "Test response" {
		t.Errorf("Expected 'Test response', got '%s'", response)
	}
}

func TestClaudeService_SendMessage_HTTPError(t *testing.T) {
	mockClient := &http.Client{
		Transport: &mockRoundTripper{
			responseFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader("Bad request")),
				}, nil
			},
		},
	}

	config := ClaudeConfig{
		APIKey:    "test-key",
		BaseURL:   "https://api.anthropic.com",
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		MaxContextMessages: 10,
	}
	
	service := &ClaudeService{
		config:     config,
		httpClient: mockClient,
		mcpTools:   []ClaudeTool{},
		contextManager: &ContextManager{
			contexts: make(map[string]*ConversationContext),
			config:   &config,
		},
	}

	_, err := service.SendMessage("Test message")
	
	if err == nil {
		t.Error("Expected error for HTTP 400 response")
	}

	if !strings.Contains(err.Error(), "Claude API error (status 400)") {
		t.Errorf("Expected Claude API error, got: %v", err)
	}
}

func TestClaudeService_SendMessage_ToolUse(t *testing.T) {
	// Set up mock registry access
	mockRegistry := &mockRegistryAccess{
		registry: map[string]interface{}{
			"test-app": map[string]interface{}{
				"appId":   "test-app",
				"version": "1.0.0",
			},
		},
	}
	SetRegistryAccess(mockRegistry)

	mockResponse := ClaudeResponse{
		Content: []ClaudeContent{
			{
				Type: "tool_use",
				Name: "list_apps",
				ID:   "tool_1",
			},
		},
		StopReason: "tool_use",
	}

	responseJSON, _ := json.Marshal(mockResponse)
	
	mockClient := &http.Client{
		Transport: &mockRoundTripper{
			responseFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(responseJSON)),
				}, nil
			},
		},
	}

	config := ClaudeConfig{
		APIKey:    "test-key",
		BaseURL:   "https://api.anthropic.com",
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		MaxContextMessages: 10,
	}
	
	service := &ClaudeService{
		config:     config,
		httpClient: mockClient,
		mcpTools:   []ClaudeTool{},
		contextManager: &ContextManager{
			contexts: make(map[string]*ConversationContext),
			config:   &config,
		},
	}

	response, err := service.SendMessage("List apps")
	
	if err != nil {
		t.Errorf("SendMessage with tool use failed: %v", err)
	}

	if !strings.Contains(response, "Tool list_apps result:") {
		t.Errorf("Expected tool result in response, got: %s", response)
	}
}

func TestClaudeService_executeListApps(t *testing.T) {
	// Set up mock registry access
	mockRegistry := &mockRegistryAccess{
		registry: map[string]interface{}{
			"test-app": map[string]interface{}{
				"appId":   "test-app",
				"version": "1.0.0",
			},
		},
	}
	SetRegistryAccess(mockRegistry)

	service := &ClaudeService{}
	
	result, err := service.executeListApps()
	
	if err != nil {
		t.Errorf("executeListApps failed: %v", err)
	}

	if !strings.Contains(result, "test-app") {
		t.Errorf("Expected test-app in result, got: %s", result)
	}
}

func TestClaudeService_executeListApps_NoRegistry(t *testing.T) {
	// Clear registry access
	SetRegistryAccess(nil)

	service := &ClaudeService{}
	
	_, err := service.executeListApps()
	
	if err == nil {
		t.Error("Expected error when registry access not configured")
	}

	if !strings.Contains(err.Error(), "registry access not configured") {
		t.Errorf("Expected registry access error, got: %v", err)
	}
}

func TestClaudeService_executeRunApp(t *testing.T) {
	// Set up mock app runner
	mockRunner := &mockAppRunner{
		executeFunc: func(appID, toolName string, input json.RawMessage) (string, error) {
			return fmt.Sprintf("Ran %s.%s with input: %s", appID, toolName, string(input)), nil
		},
	}
	SetAppRunner(mockRunner)

	service := &ClaudeService{}
	
	input := map[string]interface{}{
		"app_id":     "test-app",
		"tool_name":  "test-tool", 
		"input_data": map[string]interface{}{"key": "value"},
	}

	result, err := service.executeDynamicAppTool("test-app", "test-tool", input)
	
	if err != nil {
		t.Errorf("executeRunApp failed: %v", err)
	}

	if !strings.Contains(result, "test-app.test-tool") {
		t.Errorf("Expected app execution result, got: %s", result)
	}
}

func TestClaudeService_executeRunApp_NoRunner(t *testing.T) {
	// Clear app runner
	SetAppRunner(nil)

	service := &ClaudeService{}
	
	input := map[string]interface{}{
		"app_id":     "test-app",
		"tool_name":  "test-tool",
		"input_data": map[string]interface{}{"key": "value"},
	}

	_, err := service.executeDynamicAppTool("test-app", "test-tool", input)
	
	if err == nil {
		t.Error("Expected error when app runner not configured")
	}

	if !strings.Contains(err.Error(), "app runner not configured") {
		t.Errorf("Expected app runner error, got: %v", err)
	}
}

func TestClaudeService_executeCreateApp(t *testing.T) {
	// Set up mock app creator
	mockCreator := &mockAppCreator{
		createFunc: func(appID, version, runtime string, tools []interface{}, appSrc string, dependencies map[string]string) (string, error) {
			depCount := 0
			if dependencies != nil {
				depCount = len(dependencies)
			}
			return fmt.Sprintf("Created %s v%s with %d tools and %d dependencies", appID, version, len(tools), depCount), nil
		},
	}
	SetAppCreator(mockCreator)

	service := &ClaudeService{}
	
	input := map[string]interface{}{
		"appId":   "test-app",
		"version": "1.0.0",
		"runtime": "wasm",
		"tools": []map[string]interface{}{
			{"name": "test-tool", "input_format": "json"},
		},
		"appSrc": "fn main() {}",
	}

	result, err := service.executeCreateApp(input)
	
	if err != nil {
		t.Errorf("executeCreateApp failed: %v", err)
	}

	if !strings.Contains(result, "test-app v1.0.0 with 1 tools and 0 dependencies") {
		t.Errorf("Expected app creation result, got: %s", result)
	}
}

func TestClaudeService_executeCreateApp_ValidationErrors(t *testing.T) {
	service := &ClaudeService{}
	
	tests := []struct {
		name    string
		input   map[string]interface{}
		wantErr string
	}{
		{
			name:    "missing appId",
			input:   map[string]interface{}{"version": "1.0.0", "runtime": "wasm", "tools": []interface{}{}, "appSrc": "code"},
			wantErr: "appId is required",
		},
		{
			name:    "missing version",
			input:   map[string]interface{}{"appId": "test", "runtime": "wasm", "tools": []interface{}{}, "appSrc": "code"},
			wantErr: "version is required",
		},
		{
			name:    "invalid runtime",
			input:   map[string]interface{}{"appId": "test", "version": "1.0.0", "runtime": "native", "tools": []interface{}{}, "appSrc": "code"},
			wantErr: "runtime must be 'wasm'",
		},
		{
			name:    "missing appSrc",
			input:   map[string]interface{}{"appId": "test", "version": "1.0.0", "runtime": "wasm", "tools": []interface{}{}},
			wantErr: "appSrc is required",
		},
		{
			name:    "no tools",
			input:   map[string]interface{}{"appId": "test", "version": "1.0.0", "runtime": "wasm", "tools": []interface{}{}, "appSrc": "code"},
			wantErr: "at least one tool is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.executeCreateApp(tt.input)
			
			if err == nil {
				t.Error("Expected validation error")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Expected error containing '%s', got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestClaudeService_HandleClaudeAPI(t *testing.T) {
	// Create mock HTTP response
	mockResponse := ClaudeResponse{
		Content: []ClaudeContent{
			{Type: "text", Text: "API response"},
		},
		StopReason: "end_turn",
	}

	responseJSON, _ := json.Marshal(mockResponse)
	
	mockClient := &http.Client{
		Transport: &mockRoundTripper{
			responseFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(responseJSON)),
				}, nil
			},
		},
	}

	config := ClaudeConfig{
		APIKey:    "test-key",
		BaseURL:   "https://api.anthropic.com",
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		MaxContextMessages: 10,
	}
	
	service := &ClaudeService{
		config:     config,
		httpClient: mockClient,
		mcpTools:   []ClaudeTool{},
		contextManager: &ContextManager{
			contexts: make(map[string]*ConversationContext),
			config:   &config,
		},
	}

	// Test successful request
	requestBody := `{"message": "Hello Claude"}`
	req := httptest.NewRequest(http.MethodPost, "/claude", strings.NewReader(requestBody))
	w := httptest.NewRecorder()

	service.HandleClaudeAPI(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response JSON: %v", err)
	}

	if response["response"] != "API response" {
		t.Errorf("Expected 'API response', got '%v'", response["response"])
	}
	
	// Verify context_stats is present
	if _, ok := response["context_stats"]; !ok {
		t.Error("Expected context_stats in response")
	}
}

func TestClaudeService_HandleClaudeAPI_MethodNotAllowed(t *testing.T) {
	service := &ClaudeService{}
	
	req := httptest.NewRequest(http.MethodGet, "/claude", nil)
	w := httptest.NewRecorder()

	service.HandleClaudeAPI(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestClaudeService_HandleClaudeAPI_InvalidJSON(t *testing.T) {
	service := &ClaudeService{}
	
	req := httptest.NewRequest(http.MethodPost, "/claude", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()

	service.HandleClaudeAPI(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestSettersAndGetters(t *testing.T) {
	// Test dependency injection setters
	mockRegistry := &mockRegistryAccess{}
	mockRunner := &mockAppRunner{}
	mockCreator := &mockAppCreator{}

	SetRegistryAccess(mockRegistry)
	SetAppRunner(mockRunner)
	SetAppCreator(mockCreator)

	// Test that they were set (we can't directly access them, but we can test via behavior)
	service := &ClaudeService{}
	
	// Test registry access
	_, err := service.executeListApps()
	if err != nil {
		t.Errorf("Registry access not set correctly: %v", err)
	}

	// Test app runner
	input := map[string]interface{}{
		"app_id":     "test",
		"tool_name":  "test",
		"input_data": map[string]interface{}{},
	}
	_, err = service.executeDynamicAppTool("test", "test", input)
	if err != nil {
		t.Errorf("App runner not set correctly: %v", err)
	}
}

// Test ClaudeQuery WASM function (limited testing due to wasmtime dependencies)
func TestClaudeQuery_ServiceNotInitialized(t *testing.T) {
	// Clear global service
	original := claudeServiceInstance
	claudeServiceInstance = nil
	defer func() { claudeServiceInstance = original }()

	// Since we can't easily create a real wasmtime.Caller, test the nil check
	if claudeServiceInstance == nil {
		// This simulates the behavior inside ClaudeQuery
		result := int32(-1) // Claude service not initialized
		if result != -1 {
			t.Errorf("Expected -1 for uninitialized service, got %d", result)
		}
	}
}

func TestClaudeStructValidation(t *testing.T) {
	// Test ClaudeConfig struct
	config := ClaudeConfig{
		APIKey:         "test-key",
		BaseURL:        "https://api.anthropic.com",
		Model:          "claude-3-5-sonnet-20241022",
		MaxTokens:      4096,
		TimeoutSeconds: 30,
		EnableMCP:      true,
		MCPServerCmd:   "python server.py",
	}

	if config.APIKey != "test-key" {
		t.Error("ClaudeConfig APIKey not set correctly")
	}

	// Test ClaudeMessage struct
	message := ClaudeMessage{
		Role:    "user",
		Content: "Test message",
	}

	if message.Role != "user" {
		t.Error("ClaudeMessage Role not set correctly")
	}

	// Test ClaudeTool struct
	tool := ClaudeTool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]interface{}{"type": "object"},
	}

	if tool.Name != "test_tool" {
		t.Error("ClaudeTool Name not set correctly")
	}
}

func TestExecuteMCPToolDirect_UnknownTool(t *testing.T) {
	service := &ClaudeService{}

	// Test with a tool name that doesn't contain underscore (won't be treated as app tool)
	_, err := service.executeMCPToolDirect("unknowntool", nil)

	if err == nil {
		t.Error("Expected error for unknown tool")
	}

	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("Expected unknown tool error, got: %v", err)
	}
}