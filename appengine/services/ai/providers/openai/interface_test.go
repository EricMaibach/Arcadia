package openai

import (
	"testing"

	"arcadia/services/ai"
)

// TestOpenAIServiceImplementsInterface verifies that OpenAIService implements AIService interface
func TestOpenAIServiceImplementsInterface(t *testing.T) {
	// Create a basic configuration for testing
	config := &ai.AIConfig{
		Provider:           "openai",
		ProviderSettings:   map[string]any{
			"api_key":   "test-key",
			"base_url":  "https://api.openai.com",
			"model":     "gpt-4",
		},
		MaxTokens:          4096,
		TimeoutSeconds:     120,
		MaxContextMessages: 20,
		ContextTTLMinutes:  60,
		EnableMCP:          false,
	}

	// Create OpenAI service
	service, err := NewOpenAIService(config)
	if err != nil {
		t.Fatalf("Failed to create OpenAI service: %v", err)
	}

	// Verify it implements AIService interface by assigning to interface type
	var aiService ai.AIService = service
	if aiService == nil {
		t.Fatal("OpenAI service does not implement AIService interface")
	}

	// Test interface methods exist (this will fail compilation if methods are missing)
	_ = aiService.SendMessage
	_ = aiService.SendMessageWithContext
	_ = aiService.ClearContext
	_ = aiService.GetContextStats
	_ = aiService.RefreshTools
	_ = aiService.TriggerToolRefresh
	_ = aiService.GetProviderInfo
	_ = aiService.Validate

	t.Log("OpenAI service successfully implements AIService interface")
}

// TestOpenAIServiceProviderInfo verifies the provider info is correctly set
func TestOpenAIServiceProviderInfo(t *testing.T) {
	config := &ai.AIConfig{
		Provider:           "openai",
		ProviderSettings:   map[string]any{
			"api_key":   "test-key",
			"base_url":  "https://api.openai.com",
			"model":     "gpt-3.5-turbo",
		},
		MaxTokens:          2048,
		TimeoutSeconds:     60,
		MaxContextMessages: 10,
		ContextTTLMinutes:  30,
		EnableMCP:          false,
	}

	service, err := NewOpenAIService(config)
	if err != nil {
		t.Fatalf("Failed to create OpenAI service: %v", err)
	}

	providerInfo := service.GetProviderInfo()

	if providerInfo.Name != "openai" {
		t.Errorf("Expected provider name 'openai', got '%s'", providerInfo.Name)
	}

	if providerInfo.Model != "gpt-3.5-turbo" {
		t.Errorf("Expected model 'gpt-3.5-turbo', got '%s'", providerInfo.Model)
	}

	expectedFeatures := []string{"tools", "context", "streaming"}
	if len(providerInfo.Features) != len(expectedFeatures) {
		t.Errorf("Expected %d features, got %d", len(expectedFeatures), len(providerInfo.Features))
	}

	for i, feature := range expectedFeatures {
		if i >= len(providerInfo.Features) || providerInfo.Features[i] != feature {
			t.Errorf("Expected feature '%s' at index %d", feature, i)
		}
	}
}

// TestOpenAIServiceValidation verifies the validation logic
func TestOpenAIServiceValidation(t *testing.T) {
	// Test missing API key
	config := &ai.AIConfig{
		Provider:           "openai",
		ProviderSettings:   map[string]any{
			"base_url": "https://api.openai.com",
			"model":    "gpt-4",
		},
		MaxTokens:      4096,
		TimeoutSeconds: 120,
		EnableMCP:      false,
	}

	service, err := NewOpenAIService(config)
	if err != nil {
		t.Fatalf("Failed to create OpenAI service: %v", err)
	}

	err = service.Validate()
	if err == nil {
		t.Error("Expected validation error for missing API key")
	}

	aiErr, ok := err.(*ai.AIError)
	if !ok {
		t.Error("Expected AIError type")
	} else {
		if aiErr.Type != ai.ErrorTypeAuth {
			t.Errorf("Expected auth error, got %s", aiErr.Type)
		}
		if aiErr.Provider != "openai" {
			t.Errorf("Expected provider 'openai', got %s", aiErr.Provider)
		}
	}

	// Test with valid configuration
	config.ProviderSettings["api_key"] = "test-key"
	service, err = NewOpenAIService(config)
	if err != nil {
		t.Fatalf("Failed to create OpenAI service: %v", err)
	}

	err = service.Validate()
	if err != nil {
		t.Errorf("Unexpected validation error: %v", err)
	}
}

// TestMessageSequenceValidation verifies that our message sequence validation works correctly
func TestMessageSequenceValidation(t *testing.T) {
	config := &ai.AIConfig{
		Provider:           "openai",
		ProviderSettings:   map[string]any{
			"api_key":   "test-key",
			"base_url":  "https://api.openai.com",
			"model":     "gpt-4",
		},
		MaxTokens:      4096,
		TimeoutSeconds: 120,
		EnableMCP:      false,
	}

	service, err := NewOpenAIService(config)
	if err != nil {
		t.Fatalf("Failed to create OpenAI service: %v", err)
	}

	// Test valid sequence: user -> assistant with tool calls -> tool responses
	validMessages := []OpenAIMessage{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "What's the weather?"},
		{
			Role: "assistant",
			Content: "I'll check the weather for you.",
			ToolCalls: []OpenAIToolCall{
				{ID: "call_123", Type: "function", Function: OpenAIFunctionCall{Name: "get_weather", Arguments: "{}"}},
			},
		},
		{Role: "tool", Content: "Sunny, 25°C", ToolCallID: "call_123"},
		{Role: "assistant", Content: "The weather is sunny and 25°C."},
	}

	err = service.validateMessageSequence(validMessages)
	if err != nil {
		t.Errorf("Valid message sequence should pass validation, got error: %v", err)
	}

	// Test invalid sequence: tool message without tool_call_id
	invalidMessages1 := []OpenAIMessage{
		{Role: "user", Content: "Hello"},
		{Role: "tool", Content: "Response"},  // Missing tool_call_id
	}

	err = service.validateMessageSequence(invalidMessages1)
	if err == nil {
		t.Error("Invalid sequence (tool without tool_call_id) should fail validation")
	}

	// Test invalid sequence: tool message with unknown tool_call_id
	invalidMessages2 := []OpenAIMessage{
		{Role: "user", Content: "Hello"},
		{
			Role: "assistant",
			ToolCalls: []OpenAIToolCall{
				{ID: "call_123", Type: "function", Function: OpenAIFunctionCall{Name: "test", Arguments: "{}"}},
			},
		},
		{Role: "tool", Content: "Response", ToolCallID: "call_456"}, // Wrong ID
	}

	err = service.validateMessageSequence(invalidMessages2)
	if err == nil {
		t.Error("Invalid sequence (tool with wrong tool_call_id) should fail validation")
	}

	// Test invalid sequence: unfulfilled tool calls
	invalidMessages3 := []OpenAIMessage{
		{Role: "user", Content: "Hello"},
		{
			Role: "assistant",
			ToolCalls: []OpenAIToolCall{
				{ID: "call_123", Type: "function", Function: OpenAIFunctionCall{Name: "test", Arguments: "{}"}},
				{ID: "call_456", Type: "function", Function: OpenAIFunctionCall{Name: "test2", Arguments: "{}"}},
			},
		},
		{Role: "tool", Content: "Response", ToolCallID: "call_123"}, // Only one response
	}

	err = service.validateMessageSequence(invalidMessages3)
	if err == nil {
		t.Error("Invalid sequence (unfulfilled tool calls) should fail validation")
	}
}