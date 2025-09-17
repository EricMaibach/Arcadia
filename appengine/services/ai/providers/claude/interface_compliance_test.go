package claude

import (
	"reflect"
	"testing"

	"arcadia/services/ai"
)

// TestAIServiceInterfaceCompliance validates that ClaudeService implements all AIService interface methods
func TestAIServiceInterfaceCompliance(t *testing.T) {
	// Create a test service
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

	// Verify it implements AIService interface
	var _ ai.AIService = service

	// Test that the service type implements all interface methods
	serviceType := reflect.TypeOf(service)
	interfaceType := reflect.TypeOf((*ai.AIService)(nil)).Elem()

	for i := 0; i < interfaceType.NumMethod(); i++ {
		method := interfaceType.Method(i)

		// Check if the service has this method
		serviceMethod, found := serviceType.MethodByName(method.Name)
		if !found {
			t.Errorf("ClaudeService missing method: %s", method.Name)
			continue
		}

		// For struct methods, the receiver counts as the first parameter,
		// so we need to adjust the comparison
		expectedIn := method.Type.NumIn()
		actualIn := serviceMethod.Type.NumIn() - 1 // Subtract receiver

		if actualIn != expectedIn {
			t.Errorf("Method %s has wrong number of input parameters: expected %d, got %d",
				method.Name, expectedIn, actualIn)
		}

		if serviceMethod.Type.NumOut() != method.Type.NumOut() {
			t.Errorf("Method %s has wrong number of output parameters: expected %d, got %d",
				method.Name, method.Type.NumOut(), serviceMethod.Type.NumOut())
		}
	}
}

// TestAllInterfaceMethods validates that all interface methods can be called without panicking
func TestAllInterfaceMethods(t *testing.T) {
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

	// Test SendMessage (should not panic, even without real API call)
	t.Run("SendMessage", func(t *testing.T) {
		// This will fail with network error but should not panic
		_, err := service.SendMessage("test message")
		// We expect an error since we're not making real API calls
		if err == nil {
			t.Log("SendMessage unexpectedly succeeded (might have made real API call)")
		}
	})

	// Test SendMessageWithContext
	t.Run("SendMessageWithContext", func(t *testing.T) {
		// This will fail with network error but should not panic
		_, err := service.SendMessageWithContext("test message", "test-context")
		// We expect an error since we're not making real API calls
		if err == nil {
			t.Log("SendMessageWithContext unexpectedly succeeded (might have made real API call)")
		}
	})

	// Test ClearContext (should not error)
	t.Run("ClearContext", func(t *testing.T) {
		service.ClearContext("test-context")
		// Should not panic or error
	})

	// Test GetContextStats
	t.Run("GetContextStats", func(t *testing.T) {
		messages, tokens, exists := service.GetContextStats("test-context")
		if exists {
			t.Error("Context should not exist after clearing")
		}
		if messages != 0 || tokens != 0 {
			t.Errorf("Expected 0 messages and tokens, got %d messages, %d tokens", messages, tokens)
		}
	})

	// Test RefreshTools (should not error)
	t.Run("RefreshTools", func(t *testing.T) {
		service.RefreshTools()
		// Should not panic or error
	})

	// Test TriggerToolRefresh (should not error)
	t.Run("TriggerToolRefresh", func(t *testing.T) {
		service.TriggerToolRefresh()
		// Should not panic or error
	})

	// Test GetProviderInfo
	t.Run("GetProviderInfo", func(t *testing.T) {
		info := service.GetProviderInfo()
		if info.Name != "claude" {
			t.Errorf("Expected provider name 'claude', got '%s'", info.Name)
		}
		if info.Model == "" {
			t.Error("Expected non-empty model")
		}
		if len(info.Features) == 0 {
			t.Error("Expected non-empty features list")
		}
	})

	// Test Validate
	t.Run("Validate", func(t *testing.T) {
		err := service.Validate()
		if err != nil {
			t.Errorf("Service validation failed: %v", err)
		}
	})
}

// TestProviderInfoStructure validates the ProviderInfo structure
func TestProviderInfoStructure(t *testing.T) {
	config := &ai.AIConfig{
		Provider:       "claude",
		MaxTokens:      4096,
		TimeoutSeconds: 120,
		ProviderSettings: map[string]any{
			"api_key":  "test-key",
			"base_url": "https://api.anthropic.com",
			"model":    "claude-3-haiku",
		},
	}

	service, err := NewClaudeService(config)
	if err != nil {
		t.Fatalf("Failed to create Claude service: %v", err)
	}

	info := service.GetProviderInfo()

	// Validate required fields
	if info.Name == "" {
		t.Error("ProviderInfo.Name should not be empty")
	}
	if info.Model == "" {
		t.Error("ProviderInfo.Model should not be empty")
	}
	if info.Features == nil {
		t.Error("ProviderInfo.Features should not be nil")
	}

	// Validate specific values
	if info.Name != "claude" {
		t.Errorf("Expected provider name 'claude', got '%s'", info.Name)
	}
	if info.Model != "claude-3-haiku" {
		t.Errorf("Expected model 'claude-3-haiku', got '%s'", info.Model)
	}

	// Validate features contains expected capabilities
	expectedFeatures := []string{"tools", "context", "system_prompts"}
	for _, expected := range expectedFeatures {
		found := false
		for _, feature := range info.Features {
			if feature == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected feature '%s' not found in features list: %v", expected, info.Features)
		}
	}
}