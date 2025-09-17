package ai

import (
	"fmt"
	"log"
)

// ServiceProvider represents a function that can create an AI service
type ServiceProvider func(config *AIConfig) (AIService, error)

// ServiceFactory creates AI services based on configuration
type ServiceFactory struct {
	providers map[string]ServiceProvider
}

// NewServiceFactory creates a new service factory
func NewServiceFactory() *ServiceFactory {
	return &ServiceFactory{
		providers: make(map[string]ServiceProvider),
	}
}

// RegisterProvider registers a service provider for a specific AI provider
func (sf *ServiceFactory) RegisterProvider(name string, provider ServiceProvider) {
	sf.providers[name] = provider
}

// CreateService creates an AI service based on the provider specified in config
func (sf *ServiceFactory) CreateService(config *AIConfig) (AIService, error) {
	if config == nil {
		return nil, &AIError{
			Type:     ErrorTypeValidation,
			Message:  "configuration is required",
			Provider: "factory",
		}
	}

	// Validate configuration first
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Find the registered provider
	provider, exists := sf.providers[config.Provider]
	if !exists {
		supportedProviders := make([]string, 0, len(sf.providers))
		for name := range sf.providers {
			supportedProviders = append(supportedProviders, name)
		}
		return nil, &AIError{
			Type:     ErrorTypeValidation,
			Message:  fmt.Sprintf("unsupported AI provider: %s (supported: %v)", config.Provider, supportedProviders),
			Provider: "factory",
		}
	}

	// Create the service using the registered provider
	service, err := provider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s service: %w", config.Provider, err)
	}

	// Validate the service
	if err := service.Validate(); err != nil {
		return nil, fmt.Errorf("%s service validation failed: %w", config.Provider, err)
	}

	log.Printf("[AI Factory] Created %s service with model: %s",
		config.Provider, service.GetProviderInfo().Model)
	return service, nil
}

// CreateServiceWithDefaults creates a service with default settings for the provider
func (sf *ServiceFactory) CreateServiceWithDefaults(provider, apiKey string) (AIService, error) {
	config := &AIConfig{
		Provider:           provider,
		MaxTokens:          4096,
		TimeoutSeconds:     120,
		MaxContextMessages: 20,
		ContextCompaction:  true,
		ContextTTLMinutes:  60,
		EnableMCP:          true,
		ProviderSettings:   make(map[string]any),
	}

	switch provider {
	case "claude":
		config.ProviderSettings["api_key"] = apiKey
		config.ProviderSettings["base_url"] = "https://api.anthropic.com"
		config.ProviderSettings["model"] = "claude-3-5-sonnet-20241022"

	case "openai":
		config.ProviderSettings["api_key"] = apiKey
		config.ProviderSettings["base_url"] = "https://api.openai.com"
		config.ProviderSettings["model"] = "gpt-4"
		config.ProviderSettings["temperature"] = 1.0

	default:
		return nil, &AIError{
			Type:     ErrorTypeValidation,
			Message:  fmt.Sprintf("unsupported provider for defaults: %s", provider),
			Provider: "factory",
		}
	}

	return sf.CreateService(config)
}

// GetSupportedProviders returns a list of supported AI providers
func (sf *ServiceFactory) GetSupportedProviders() []string {
	providers := make([]string, 0, len(sf.providers))
	for name := range sf.providers {
		providers = append(providers, name)
	}
	return providers
}

// ValidateProvider checks if a provider is supported
func (sf *ServiceFactory) ValidateProvider(provider string) error {
	_, exists := sf.providers[provider]
	if exists {
		return nil
	}

	return &AIError{
		Type:     ErrorTypeValidation,
		Message:  fmt.Sprintf("unsupported provider: %s", provider),
		Provider: "factory",
	}
}