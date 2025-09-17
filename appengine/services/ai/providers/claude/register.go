package claude

import (
	"arcadia/services/ai"
)

// RegisterProvider registers the Claude provider with a factory
func RegisterProvider(factory *ai.ServiceFactory) {
	factory.RegisterProvider("claude", func(config *ai.AIConfig) (ai.AIService, error) {
		return NewClaudeService(config)
	})
}

// init automatically registers the Claude provider when the package is imported
func init() {
	// Register with the global factory
	factory := ai.GetGlobalFactory()
	RegisterProvider(factory)
}