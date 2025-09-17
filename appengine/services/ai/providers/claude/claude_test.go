package claude

import (
	"strings"
	"testing"

	"arcadia/services/ai"
)

func TestClaudeServiceCreation(t *testing.T) {
	// Test configuration
	config := &ai.AIConfig{
		Provider:       "claude",
		MaxTokens:      4096,
		TimeoutSeconds: 120,
		EnableMCP:      true,
		ProviderSettings: map[string]any{
			"api_key":  "test-key",
			"base_url": "https://api.anthropic.com",
			"model":    "claude-3-5-sonnet-20241022",
		},
	}

	// Create Claude service
	service, err := NewClaudeService(config)
	if err != nil {
		t.Fatalf("Failed to create Claude service: %v", err)
	}

	// Test that it implements AIService interface
	var _ ai.AIService = service

	// Test provider info
	info := service.GetProviderInfo()
	if info.Name != "claude" {
		t.Errorf("Expected provider name 'claude', got '%s'", info.Name)
	}
	if info.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Expected model 'claude-3-5-sonnet-20241022', got '%s'", info.Model)
	}

	// Test validation
	if err := service.Validate(); err != nil {
		t.Errorf("Service validation failed: %v", err)
	}

	// Test context operations
	service.ClearContext("test-context")
	messages, tokens, exists := service.GetContextStats("test-context")
	if exists {
		t.Error("Context should not exist after clearing")
	}
	if messages != 0 || tokens != 0 {
		t.Errorf("Expected 0 messages and tokens, got %d messages, %d tokens", messages, tokens)
	}
}

func TestClaudeConfigParsing(t *testing.T) {
	config := &ai.AIConfig{
		Provider:       "claude",
		MaxTokens:      2048,
		TimeoutSeconds: 60,
		EnableMCP:      false,
		ProviderSettings: map[string]any{
			"api_key":  "sk-test123",
			"base_url": "https://custom.anthropic.com",
			"model":    "claude-3-haiku",
		},
	}

	claudeConfig, err := parseClaudeConfig(config)
	if err != nil {
		t.Fatalf("Failed to parse Claude config: %v", err)
	}

	if claudeConfig.APIKey != "sk-test123" {
		t.Errorf("Expected API key 'sk-test123', got '%s'", claudeConfig.APIKey)
	}
	if claudeConfig.BaseURL != "https://custom.anthropic.com" {
		t.Errorf("Expected base URL 'https://custom.anthropic.com', got '%s'", claudeConfig.BaseURL)
	}
	if claudeConfig.Model != "claude-3-haiku" {
		t.Errorf("Expected model 'claude-3-haiku', got '%s'", claudeConfig.Model)
	}
	if claudeConfig.MaxTokens != 2048 {
		t.Errorf("Expected max tokens 2048, got %d", claudeConfig.MaxTokens)
	}
}

func TestClaudeConfigDefaults(t *testing.T) {
	config := &ai.AIConfig{
		Provider:       "claude",
		MaxTokens:      4096,
		TimeoutSeconds: 120,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
			// No base_url or model specified
		},
	}

	claudeConfig, err := parseClaudeConfig(config)
	if err != nil {
		t.Fatalf("Failed to parse Claude config: %v", err)
	}

	// Test defaults
	if claudeConfig.BaseURL != "https://api.anthropic.com" {
		t.Errorf("Expected default base URL 'https://api.anthropic.com', got '%s'", claudeConfig.BaseURL)
	}
	if claudeConfig.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Expected default model 'claude-3-5-sonnet-20241022', got '%s'", claudeConfig.Model)
	}
}

func TestInvalidProvider(t *testing.T) {
	config := &ai.AIConfig{
		Provider: "openai", // Wrong provider
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	_, err := NewClaudeService(config)
	if err == nil {
		t.Error("Expected error for invalid provider")
	}
	if !strings.Contains(err.Error(), "invalid provider") {
		t.Errorf("Expected 'invalid provider' error, got: %v", err)
	}
}

func TestValidationErrors(t *testing.T) {
	// Test missing API key
	config := &ai.AIConfig{
		Provider:         "claude",
		ProviderSettings: map[string]any{
			// No API key
		},
	}

	service, err := NewClaudeService(config)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	err = service.Validate()
	if err == nil {
		t.Error("Expected validation error for missing API key")
	}

	// Check error type
	if aiErr, ok := err.(*ai.AIError); ok {
		if aiErr.Type != ai.ErrorTypeAuth {
			t.Errorf("Expected auth error type, got: %s", aiErr.Type)
		}
		if aiErr.Provider != "claude" {
			t.Errorf("Expected provider 'claude', got: %s", aiErr.Provider)
		}
	} else {
		t.Error("Expected AIError type")
	}
}

func TestCreateClaudeConfig(t *testing.T) {
	config := CreateClaudeConfig("sk-test123", "claude-3-haiku", 2048)

	if config.Provider != "claude" {
		t.Errorf("Expected provider 'claude', got '%s'", config.Provider)
	}
	if config.MaxTokens != 2048 {
		t.Errorf("Expected max tokens 2048, got %d", config.MaxTokens)
	}

	apiKey := config.GetProviderString("api_key")
	if apiKey != "sk-test123" {
		t.Errorf("Expected API key 'sk-test123', got '%s'", apiKey)
	}

	model := config.GetProviderString("model")
	if model != "claude-3-haiku" {
		t.Errorf("Expected model 'claude-3-haiku', got '%s'", model)
	}

	// Test service creation with this config
	service, err := NewClaudeService(config)
	if err != nil {
		t.Fatalf("Failed to create service with CreateClaudeConfig: %v", err)
	}

	info := service.GetProviderInfo()
	if info.Model != "claude-3-haiku" {
		t.Errorf("Expected service model 'claude-3-haiku', got '%s'", info.Model)
	}
}

func TestCreateClaudeConfigWithDefaults(t *testing.T) {
	config := CreateClaudeConfig("test-key", "", 4096)

	model := config.GetProviderString("model")
	if model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Expected default model 'claude-3-5-sonnet-20241022', got '%s'", model)
	}

	baseURL := config.GetProviderString("base_url")
	if baseURL != "https://api.anthropic.com" {
		t.Errorf("Expected default base URL 'https://api.anthropic.com', got '%s'", baseURL)
	}

	// Test default values
	if !config.EnableMCP {
		t.Error("Expected MCP to be enabled by default")
	}
	if !config.ContextCompaction {
		t.Error("Expected context compaction to be enabled by default")
	}
	if config.ContextTTLMinutes != 60 {
		t.Errorf("Expected default TTL 60 minutes, got %d", config.ContextTTLMinutes)
	}
}
