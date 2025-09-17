package ai_test

import (
	"testing"

	"arcadia/services/ai"
	"arcadia/services/airegistry"
)

func TestRegistryIntegration(t *testing.T) {
	// Setup providers through registry
	airegistry.SetupProviders()

	// Test that providers are registered
	factory := ai.GetGlobalFactory()
	providers := factory.GetSupportedProviders()

	if len(providers) == 0 {
		t.Error("Expected providers to be registered, got none")
	}

	expectedProviders := map[string]bool{
		"claude": false,
		"openai": false,
	}

	for _, provider := range providers {
		if _, exists := expectedProviders[provider]; exists {
			expectedProviders[provider] = true
		}
	}

	for provider, found := range expectedProviders {
		if !found {
			t.Errorf("Expected provider %s to be registered", provider)
		}
	}

	// Test that validation works
	for _, provider := range []string{"claude", "openai"} {
		err := factory.ValidateProvider(provider)
		if err != nil {
			t.Errorf("Expected %s provider to be valid, got error: %v", provider, err)
		}
	}

	// Test invalid provider
	err := factory.ValidateProvider("invalid")
	if err == nil {
		t.Error("Expected error for invalid provider")
	}
}