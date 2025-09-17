package claude

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"arcadia/services/ai"
)

// TestErrorHandlingScenarios tests various error scenarios and edge cases
func TestErrorHandlingScenarios(t *testing.T) {
	// Test service creation with invalid configurations
	t.Run("Invalid provider configuration", func(t *testing.T) {
		invalidConfig := &ai.AIConfig{
			Provider: "openai", // Wrong provider for Claude service
			ProviderSettings: map[string]any{
				"api_key": "test-key",
			},
		}

		_, err := NewClaudeService(invalidConfig)
		if err == nil {
			t.Error("Expected error for invalid provider")
		}
		if err.Error() != "invalid provider for Claude service: openai" {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	t.Run("Service validation with missing API key", func(t *testing.T) {
		config := &ai.AIConfig{
			Provider: "claude",
			ProviderSettings: map[string]any{
				// No API key
			},
		}

		service, err := NewClaudeService(config)
		if err != nil {
			t.Fatalf("Service creation should not fail: %v", err)
		}

		err = service.Validate()
		if err == nil {
			t.Error("Expected validation error for missing API key")
		}

		if aiErr, ok := err.(*ai.AIError); ok {
			if aiErr.Type != ai.ErrorTypeAuth {
				t.Errorf("Expected auth error, got: %s", aiErr.Type)
			}
			if aiErr.Provider != "claude" {
				t.Errorf("Expected provider 'claude', got: %s", aiErr.Provider)
			}
		} else {
			t.Error("Expected AIError type")
		}
	})
}

// TestNetworkErrorHandling tests handling of various network errors
func TestNetworkErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectedError  string
	}{
		{
			name: "Authentication Error",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": {"type": "authentication_error", "message": "Invalid API key"}}`))
			},
			expectedError: ai.ErrorTypeAuth,
		},
		{
			name: "Rate Limit Error",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error": {"type": "rate_limit_error", "message": "Rate limit exceeded"}}`))
			},
			expectedError: ai.ErrorTypeRateLimit,
		},
		{
			name: "Server Error",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": {"type": "internal_server_error", "message": "Internal server error"}}`))
			},
			expectedError: ai.ErrorTypeProvider,
		},
		{
			name: "Bad Gateway",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				w.Write([]byte(`{"error": {"type": "bad_gateway", "message": "Bad gateway"}}`))
			},
			expectedError: ai.ErrorTypeNetwork,
		},
		{
			name: "Invalid JSON Response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`invalid json response`))
			},
			expectedError: "unknown_error", // Should become unknown error due to JSON parsing failure
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			config := &ai.AIConfig{
				Provider:       "claude",
				MaxTokens:      4096,
				TimeoutSeconds: 30,
				ProviderSettings: map[string]any{
					"api_key":  "test-api-key",
					"base_url": server.URL,
					"model":    "claude-3-5-sonnet-20241022",
				},
			}

			service, err := NewClaudeService(config)
			if err != nil {
				t.Fatalf("Failed to create Claude service: %v", err)
			}

			_, err = service.SendMessage("test message")
			if err == nil {
				t.Error("Expected error but got none")
			}

			// Check if it's an AIError with the expected type
			if aiErr, ok := err.(*ai.AIError); ok {
				if tt.expectedError != "unknown_error" && aiErr.Type != tt.expectedError {
					t.Errorf("Expected error type '%s', got '%s'", tt.expectedError, aiErr.Type)
				}
				if aiErr.Provider != "claude" {
					t.Errorf("Expected provider 'claude', got '%s'", aiErr.Provider)
				}
			} else if tt.expectedError != "unknown_error" {
				t.Errorf("Expected AIError type, got: %T", err)
			}
		})
	}
}

// TestToolExecutionErrors tests error handling in tool execution
func TestToolExecutionErrors(t *testing.T) {
	config := &ai.AIConfig{
		Provider:    "claude",
		EnableMCP:   true,
		MaxTokens:   4096,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	toolManager := ai.NewToolManager(config)

	// Test unknown tool execution
	t.Run("Unknown tool execution", func(t *testing.T) {
		_, err := toolManager.ExecuteTool("unknown_tool", nil)
		if err == nil {
			t.Error("Expected error for unknown tool")
		}
		if err.Error() != "unknown tool: unknown_tool" {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	// Test tool execution without required dependencies
	t.Run("Tool execution without dependencies", func(t *testing.T) {
		_, err := toolManager.ExecuteTool("list_apps", nil)
		if err == nil {
			t.Error("Expected error for missing registry access")
		}
		if err.Error() != "registry access not configured" {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	// Test search_documents without embedding search
	t.Run("Search documents without embedding search", func(t *testing.T) {
		input := map[string]any{
			"query": "test query",
			"top_k": 5,
		}

		_, err := toolManager.ExecuteTool("search_documents", input)
		if err == nil {
			t.Error("Expected error for missing embedding search")
		}
		if err.Error() != "embedding search service not configured" {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	// Test create_app with invalid input
	t.Run("Create app with invalid input", func(t *testing.T) {
		invalidInputs := []map[string]any{
			{}, // Empty input
			{"appId": ""}, // Empty app ID
			{"appId": "test", "version": ""}, // Empty version
			{"appId": "test", "version": "1.0", "runtime": "invalid"}, // Invalid runtime
			{"appId": "test", "version": "1.0", "runtime": "wasm"}, // Missing appSrc
			{"appId": "test", "version": "1.0", "runtime": "wasm", "appSrc": "code"}, // Missing tools
		}

		for i, input := range invalidInputs {
			_, err := toolManager.ExecuteTool("create_app", input)
			if err == nil {
				t.Errorf("Test case %d: Expected error for invalid input: %v", i, input)
			}
		}
	})
}

// TestContextErrorHandling tests error handling in context management
func TestContextErrorHandling(t *testing.T) {
	config := &ai.AIConfig{
		Provider:           "claude",
		MaxTokens:          4096,
		MaxContextMessages: 3,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	contextManager := ai.NewContextManager(config)

	// Test context operations on non-existent context
	t.Run("Operations on non-existent context", func(t *testing.T) {
		// GetContext should return false for non-existent context
		_, exists := contextManager.GetContext("non-existent")
		if exists {
			t.Error("Expected false for non-existent context")
		}

		// GetContextStats should return false for non-existent context
		messages, tokens, exists := contextManager.GetContextStats("non-existent")
		if exists || messages != 0 || tokens != 0 {
			t.Errorf("Expected empty stats for non-existent context, got messages=%d, tokens=%d, exists=%v", messages, tokens, exists)
		}

		// UpdateTokenCount on non-existent context should not crash
		contextManager.UpdateTokenCount("non-existent", 100)

		// ClearContext on non-existent context should not crash
		contextManager.ClearContext("non-existent")
	})

	// Test context with malformed messages
	t.Run("Context with various message types", func(t *testing.T) {
		contextID := "test-context"

		// Add various types of messages
		messages := []ai.Message{
			{Role: ai.RoleUser, Content: "string content"},
			{Role: ai.RoleAssistant, Content: map[string]any{"type": "text", "text": "structured content"}},
			{Role: ai.RoleSystem, Content: "system message"},
		}

		for _, msg := range messages {
			contextManager.AddMessage(contextID, msg)
		}

		// Verify context exists and has messages
		retrievedContext, exists := contextManager.GetContext(contextID)
		if !exists {
			t.Error("Context should exist after adding messages")
		}

		if len(retrievedContext.Messages) != len(messages) {
			t.Errorf("Expected %d messages, got %d", len(messages), len(retrievedContext.Messages))
		}
	})
}

// TestHTTPClientErrorHandling tests HTTP client error scenarios
func TestHTTPClientErrorHandling(t *testing.T) {
	t.Run("HTTP client timeout", func(t *testing.T) {
		client := ai.NewHTTPClient(1) // 1 second timeout
		client.SetBaseURL("http://192.0.2.1:81") // Non-routable address

		_, err := client.Get("/test")
		if err == nil {
			t.Error("Expected timeout error")
		}
	})

	t.Run("Invalid JSON marshaling", func(t *testing.T) {
		client := ai.NewHTTPClient(30)
		client.SetBaseURL("http://example.com")

		// Create a payload that can't be marshaled to JSON
		invalidPayload := map[string]any{
			"invalid": make(chan int), // Channels can't be marshaled to JSON
		}

		_, err := client.PostJSON("/test", invalidPayload)
		if err == nil {
			t.Error("Expected JSON marshaling error")
		}
	})

	t.Run("Invalid URL", func(t *testing.T) {
		client := ai.NewHTTPClient(30)
		client.SetBaseURL("://invalid-url")

		_, err := client.Get("/test")
		if err == nil {
			t.Error("Expected URL parsing error")
		}
	})
}

// TestEdgeCases tests various edge cases and boundary conditions
func TestEdgeCases(t *testing.T) {
	t.Run("Zero and negative configuration values", func(t *testing.T) {
		config := &ai.AIConfig{
			Provider:           "claude",
			MaxTokens:          0,  // Should be set to default
			TimeoutSeconds:     -1, // Should be set to default
			MaxContextMessages: -5, // Should be set to default
			ContextTTLMinutes:  0,  // Should be set to default
			ProviderSettings: map[string]any{
				"api_key": "test-key",
			},
		}

		err := config.Validate()
		if err != nil {
			t.Errorf("Config validation failed: %v", err)
		}

		// Check that defaults were set
		if config.MaxTokens <= 0 {
			t.Error("MaxTokens should be set to positive default")
		}
		if config.TimeoutSeconds <= 0 {
			t.Error("TimeoutSeconds should be set to positive default")
		}
		if config.MaxContextMessages <= 0 {
			t.Error("MaxContextMessages should be set to positive default")
		}
		if config.ContextTTLMinutes <= 0 {
			t.Error("ContextTTLMinutes should be set to positive default")
		}
	})

	t.Run("Empty strings in configuration", func(t *testing.T) {
		config := &ai.AIConfig{
			Provider: "", // Empty provider
			ProviderSettings: map[string]any{
				"api_key":  "",
				"base_url": "",
				"model":    "",
			},
		}

		err := config.Validate()
		if err == nil {
			t.Error("Expected validation error for empty provider")
		}
	})

	t.Run("Very large context messages", func(t *testing.T) {
		config := &ai.AIConfig{
			Provider:           "claude",
			MaxContextMessages: 1000000, // Very large number
			ProviderSettings: map[string]any{
				"api_key": "test-key",
			},
		}

		contextManager := ai.NewContextManager(config)

		// This should not crash or cause memory issues
		for i := 0; i < 100; i++ {
			message := ai.Message{
				Role:    ai.RoleUser,
				Content: "Test message number " + string(rune(i)),
			}
			contextManager.AddMessage("test-context", message)
		}

		messages, _, exists := contextManager.GetContextStats("test-context")
		if !exists {
			t.Error("Context should exist")
		}
		if messages != 100 {
			t.Errorf("Expected 100 messages, got %d", messages)
		}
	})

	t.Run("Concurrent operations stress test", func(t *testing.T) {
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

		// Run many concurrent operations to test for race conditions
		done := make(chan bool, 100)

		for i := 0; i < 100; i++ {
			go func(id int) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic in goroutine %d: %v", id, r)
					}
					done <- true
				}()

				contextID := "stress-test-context"

				// Mix of operations
				service.ClearContext(contextID)
				_, _, _ = service.GetContextStats(contextID)
				service.TriggerToolRefresh()

				// Try to send message (will fail but shouldn't crash)
				_, _ = service.SendMessageWithContext("test", contextID)
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 100; i++ {
			<-done
		}
	})
}