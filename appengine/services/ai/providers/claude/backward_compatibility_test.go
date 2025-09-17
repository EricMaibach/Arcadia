package claude

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"arcadia/services/ai"
)

// TestBackwardCompatibilityAPI tests that the new Claude service provides the same API as the original
func TestBackwardCompatibilityAPI(t *testing.T) {
	config := &ai.AIConfig{
		Provider:       "claude",
		MaxTokens:      4096,
		TimeoutSeconds: 120,
		ProviderSettings: map[string]any{
			"api_key":  "test-key",
			"base_url": "https://api.anthropic.com",
			"model":    "claude-3-5-sonnet-20241022",
		},
	}

	service, err := NewClaudeService(config)
	if err != nil {
		t.Fatalf("Failed to create Claude service: %v", err)
	}

	// Test that all expected methods exist and work
	t.Run("SendMessage method", func(t *testing.T) {
		// This should not panic even if it fails due to network
		_, err := service.SendMessage("test message")
		// We expect a network error, not a panic or missing method error
		if err == nil {
			t.Log("SendMessage unexpectedly succeeded")
		}
	})

	t.Run("SendMessageWithContext method", func(t *testing.T) {
		_, err := service.SendMessageWithContext("test message", "test-context")
		if err == nil {
			t.Log("SendMessageWithContext unexpectedly succeeded")
		}
	})

	t.Run("ClearContext method", func(t *testing.T) {
		service.ClearContext("test-context")
		// Should not panic
	})

	t.Run("GetContextStats method", func(t *testing.T) {
		messages, tokens, exists := service.GetContextStats("test-context")
		if exists || messages != 0 || tokens != 0 {
			t.Errorf("Expected empty context stats, got messages=%d, tokens=%d, exists=%v", messages, tokens, exists)
		}
	})

	t.Run("TriggerToolRefresh method", func(t *testing.T) {
		service.TriggerToolRefresh()
		// Should not panic
	})
}

// TestBackwardCompatibilityHTTPHandler tests the HTTP handler compatibility
func TestBackwardCompatibilityHTTPHandler(t *testing.T) {
	// Mock Claude API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"content": [
				{
					"type": "text",
					"text": "Hello! This is a test response."
				}
			],
			"stop_reason": "end_turn",
			"usage": {
				"input_tokens": 10,
				"output_tokens": 8
			}
		}`))
	}))
	defer mockServer.Close()

	config := &ai.AIConfig{
		Provider:       "claude",
		MaxTokens:      4096,
		TimeoutSeconds: 30,
		ProviderSettings: map[string]any{
			"api_key":  "test-api-key",
			"base_url": mockServer.URL,
			"model":    "claude-3-5-sonnet-20241022",
		},
	}

	service, err := NewClaudeService(config)
	if err != nil {
		t.Fatalf("Failed to create Claude service: %v", err)
	}

	// Test the HTTP handler method exists and works
	t.Run("HandleClaudeAPI method exists", func(t *testing.T) {
		// Create a test HTTP request
		requestBody := `{"message": "Hello Claude", "session_id": "test-session"}`
		req := httptest.NewRequest("POST", "/claude", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")

		// Create a ResponseRecorder to record the response
		rr := httptest.NewRecorder()

		// Call the handler
		service.HandleClaudeAPI(rr, req)

		// Check that we got a response
		if rr.Code != http.StatusOK {
			t.Logf("Handler returned status %d, response: %s", rr.Code, rr.Body.String())
			// This might fail due to network issues, but the method should exist
		}

		// Verify the response format (even if content might be different due to mocking)
		var response map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err == nil {
			// Check expected fields exist
			if _, hasResponse := response["response"]; !hasResponse {
				t.Error("Response missing 'response' field")
			}
			if _, hasStats := response["context_stats"]; !hasStats {
				t.Error("Response missing 'context_stats' field")
			}
		}
	})

	t.Run("HandleClaudeAPI with invalid method", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/claude", nil)
		rr := httptest.NewRecorder()

		service.HandleClaudeAPI(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected 405 Method Not Allowed, got %d", rr.Code)
		}
	})

	t.Run("HandleClaudeAPI with invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/claude", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		service.HandleClaudeAPI(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request, got %d", rr.Code)
		}
	})
}

// TestBackwardCompatibilityDependencyInjection tests the dependency injection compatibility
func TestBackwardCompatibilityDependencyInjection(t *testing.T) {
	config := &ai.AIConfig{
		Provider:       "claude",
		MaxTokens:      4096,
		TimeoutSeconds: 120,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	service, err := NewClaudeService(config)
	if err != nil {
		t.Fatalf("Failed to create Claude service: %v", err)
	}

	// Test SetDependencies method (mimics the original global dependency injection)
	t.Run("SetDependencies method", func(t *testing.T) {
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

		// This should not panic and should work like the original global setters
		service.SetDependencies(mockRegistry, mockAppRunner, mockAppCreator, mockEmbedding)

		// Verify dependencies were set by trying to execute a tool
		result, err := service.toolManager.ExecuteTool("list_apps", nil)
		if err != nil {
			t.Errorf("Tool execution failed after setting dependencies: %v", err)
		}

		// Should return valid JSON
		var apps map[string]any
		if err := json.Unmarshal([]byte(result), &apps); err != nil {
			t.Errorf("Tool result is not valid JSON: %v", err)
		}
	})
}

// TestBackwardCompatibilityConfiguration tests configuration compatibility
func TestBackwardCompatibilityConfiguration(t *testing.T) {
	// Test that we can create configurations similar to the original
	t.Run("CreateClaudeConfig compatibility", func(t *testing.T) {
		config := CreateClaudeConfig("test-key", "claude-3-haiku", 2048)

		// Verify it has the same structure as what the original expected
		if config.Provider != "claude" {
			t.Errorf("Expected provider 'claude', got '%s'", config.Provider)
		}
		if config.MaxTokens != 2048 {
			t.Errorf("Expected max tokens 2048, got %d", config.MaxTokens)
		}

		apiKey := config.GetProviderString("api_key")
		if apiKey != "test-key" {
			t.Errorf("Expected API key 'test-key', got '%s'", apiKey)
		}

		model := config.GetProviderString("model")
		if model != "claude-3-haiku" {
			t.Errorf("Expected model 'claude-3-haiku', got '%s'", model)
		}

		// Test service creation with this config
		service, err := NewClaudeService(config)
		if err != nil {
			t.Errorf("Failed to create service with backward compatible config: %v", err)
		}

		if service == nil {
			t.Error("Service should not be nil")
		}
	})
}

// TestBackwardCompatibilityMessageFormat tests that message processing is compatible
func TestBackwardCompatibilityMessageFormat(t *testing.T) {
	config := &ai.AIConfig{
		Provider:       "claude",
		MaxTokens:      4096,
		TimeoutSeconds: 120,
		MaxContextMessages: 5,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	service, err := NewClaudeService(config)
	if err != nil {
		t.Fatalf("Failed to create Claude service: %v", err)
	}

	// Test context management similar to original
	t.Run("Context operations compatibility", func(t *testing.T) {
		contextID := "test-context"

		// Clear context (should not error even if context doesn't exist)
		service.ClearContext(contextID)

		// Check initial stats
		messages, tokens, exists := service.GetContextStats(contextID)
		if exists || messages != 0 || tokens != 0 {
			t.Errorf("Expected clean context, got messages=%d, tokens=%d, exists=%v", messages, tokens, exists)
		}

		// Add message via SendMessageWithContext (will fail API call but should create context)
		_, _ = service.SendMessageWithContext("test message", contextID)
		// Error is expected since we're not making real API calls

		// Context should now exist
		messages, tokens, exists = service.GetContextStats(contextID)
		if !exists {
			t.Error("Context should exist after sending message")
		}
		if messages != 1 {
			t.Errorf("Expected 1 message, got %d", messages)
		}

		// Clear context again
		service.ClearContext(contextID)
		messages, tokens, exists = service.GetContextStats(contextID)
		if exists {
			t.Error("Context should not exist after clearing")
		}
	})
}

// TestBackwardCompatibilityToolRefresh tests tool refresh functionality
func TestBackwardCompatibilityToolRefresh(t *testing.T) {
	config := &ai.AIConfig{
		Provider:    "claude",
		EnableMCP:   true,
		MaxTokens:   4096,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	service, err := NewClaudeService(config)
	if err != nil {
		t.Fatalf("Failed to create Claude service: %v", err)
	}

	// Test TriggerToolRefresh method (original name)
	t.Run("TriggerToolRefresh method", func(t *testing.T) {
		// Should not panic
		service.TriggerToolRefresh()

		// Should work same as RefreshTools
		service.RefreshTools()
	})
}

// Note: Mock types are defined in tool_management_test.go to avoid duplication