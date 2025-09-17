package airegistry

import (
	"arcadia/services/ai"
	"arcadia/services/ai/providers/claude"
	"arcadia/services/ai/providers/openai"
)

// SetupProviders registers all available AI providers with the global factory
// This function should be called once during application initialization
func SetupProviders() {
	factory := ai.GetGlobalFactory()
	claude.RegisterProvider(factory)
	openai.RegisterProvider(factory)
}