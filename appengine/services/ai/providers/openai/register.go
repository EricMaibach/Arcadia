package openai

import (
	"arcadia/services/ai"
)

// RegisterProvider registers the OpenAI provider with a factory
func RegisterProvider(factory *ai.ServiceFactory) {
	factory.RegisterProvider("openai", func(config *ai.AIConfig) (ai.AIService, error) {
		return NewOpenAIService(config)
	})
}

// init automatically registers the OpenAI provider when the package is imported
func init() {
	// Register with the global factory
	factory := ai.GetGlobalFactory()
	RegisterProvider(factory)
}