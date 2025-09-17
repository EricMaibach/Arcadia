package ai

import (
	"testing"
)

func TestServiceFactory(t *testing.T) {
	factory := NewServiceFactory()

	// Test with no providers registered
	providers := factory.GetSupportedProviders()
	if len(providers) != 0 {
		t.Errorf("Expected 0 providers, got %d", len(providers))
	}

	// Test registering a mock provider
	mockProvider := func(config *AIConfig) (AIService, error) {
		return &mockAIService{}, nil
	}

	factory.RegisterProvider("mock", mockProvider)

	// Test provider is now available
	providers = factory.GetSupportedProviders()
	if len(providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(providers))
	}

	if providers[0] != "mock" {
		t.Errorf("Expected 'mock' provider, got %s", providers[0])
	}

	// Test validation
	err := factory.ValidateProvider("mock")
	if err != nil {
		t.Errorf("Expected valid provider, got error: %v", err)
	}

	err = factory.ValidateProvider("nonexistent")
	if err == nil {
		t.Errorf("Expected error for nonexistent provider")
	}
}

// Mock AI service for testing
type mockAIService struct{}

func (m *mockAIService) SendMessage(message string) (string, error) {
	return "mock response", nil
}

func (m *mockAIService) SendMessageWithContext(message string, contextID string) (string, error) {
	return "mock response with context", nil
}

func (m *mockAIService) ClearContext(contextID string) {}

func (m *mockAIService) GetContextStats(contextID string) (messages int, tokens int, exists bool) {
	return 0, 0, false
}

func (m *mockAIService) RefreshTools() {}

func (m *mockAIService) TriggerToolRefresh() {}

func (m *mockAIService) GetProviderInfo() ProviderInfo {
	return ProviderInfo{
		Name:     "mock",
		Model:    "mock-model",
		Features: []string{"test"},
	}
}

func (m *mockAIService) Validate() error {
	return nil
}