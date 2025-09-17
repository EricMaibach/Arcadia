package ai

import (
	"log"
	"sync"
)

// Global AI service instance management
var (
	globalService     AIService
	globalMutex       sync.RWMutex
	globalFactory     *ServiceFactory
	globalInitialized bool
)

func init() {
	globalFactory = NewServiceFactory()
}

// RegisterGlobalProvider registers a provider with the global factory
func RegisterGlobalProvider(name string, provider ServiceProvider) {
	globalFactory.RegisterProvider(name, provider)
}

// InitializeGlobalService initializes the global AI service with the given configuration
func InitializeGlobalService(config *AIConfig) error {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	service, err := globalFactory.CreateService(config)
	if err != nil {
		return err
	}

	globalService = service
	globalInitialized = true

	log.Printf("[AI Global] Initialized global AI service with provider: %s",
		config.Provider)

	return nil
}

// GetGlobalService returns the global AI service instance
func GetGlobalService() AIService {
	globalMutex.RLock()
	defer globalMutex.RUnlock()

	if !globalInitialized {
		log.Printf("[AI Global] Warning: Global AI service not initialized")
		return nil
	}

	return globalService
}

// IsGlobalServiceInitialized returns whether the global service has been initialized
func IsGlobalServiceInitialized() bool {
	globalMutex.RLock()
	defer globalMutex.RUnlock()
	return globalInitialized
}

// ReinitializeGlobalService reinitializes the global service with a new configuration
func ReinitializeGlobalService(config *AIConfig) error {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	service, err := globalFactory.CreateService(config)
	if err != nil {
		return err
	}

	// Clean up old service if needed
	if globalService != nil {
		// Could add cleanup logic here if needed
		log.Printf("[AI Global] Replacing global AI service")
	}

	globalService = service
	globalInitialized = true

	log.Printf("[AI Global] Reinitialized global AI service with provider: %s",
		config.Provider)

	return nil
}

// InitializeGlobalServiceFromConfig initializes the global AI service with auto-configuration
// It tries to load configuration from multiple sources in order of preference:
// 1. Environment variables
// 2. AI configuration file for the detected provider
func InitializeGlobalServiceFromConfig() error {
	return InitializeGlobalServiceFromConfigWithFallback(nil)
}

// InitializeGlobalServiceFromConfigWithFallback initializes the global AI service with auto-configuration
// and an optional legacy configuration fallback.
// It tries to load configuration from multiple sources in order of preference:
// 1. Environment variables
// 2. AI configuration file for the detected provider
// 3. Legacy configuration from the provided function (if provided)
func InitializeGlobalServiceFromConfigWithFallback(legacyConfigFunc func() *AIConfig) error {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	log.Printf("[AI Global] Initializing AI service with auto-configuration...")

	// Check if Claude provider is available (since it's imported)
	if err := globalFactory.ValidateProvider("claude"); err != nil {
		log.Printf("[AI Global] Warning: Claude provider not available: %v", err)
		log.Printf("[AI Global] Available providers: %v", globalFactory.GetSupportedProviders())
		return &AIError{
			Type:     ErrorTypeValidation,
			Message:  "no supported AI providers available",
			Provider: "global",
		}
	}

	// Try to load configuration from multiple sources, in order of preference:
	var config *AIConfig
	var err error

	// Try environment variables first
	config, err = LoadConfigFromEnv()
	if err != nil {
		log.Printf("[AI Global] No environment configuration found: %v", err)

		// Try default Claude configuration file
		config, err = LoadConfigFromDefaultFile("claude")
		if err != nil {
			log.Printf("[AI Global] No AI configuration file found: %v", err)

			// Try legacy configuration fallback if provided
			if legacyConfigFunc != nil {
				config = legacyConfigFunc()
				if config != nil {
					log.Printf("[AI Global] Using legacy configuration fallback")
				} else {
					return &AIError{
						Type:     ErrorTypeValidation,
						Message:  "no valid AI configuration found in any source",
						Provider: "global",
					}
				}
			} else {
				return &AIError{
					Type:     ErrorTypeValidation,
					Message:  "no valid AI configuration found in environment or config files",
					Provider: "global",
				}
			}
		} else {
			log.Printf("[AI Global] Loaded configuration from file: config/ai-claude.json")
		}
	} else {
		log.Printf("[AI Global] Loaded configuration from environment variables")
	}

	// Initialize the global AI service
	service, err := globalFactory.CreateService(config)
	if err != nil {
		return &AIError{
			Type:     ErrorTypeProvider,
			Message:  "failed to create AI service: " + err.Error(),
			Provider: config.Provider,
		}
	}

	globalService = service
	globalInitialized = true

	log.Printf("[AI Global] Successfully initialized global AI service")
	log.Printf("[AI Global] Provider: %s, Model: %s", config.Provider, config.GetProviderString("model"))

	return nil
}

// GetGlobalFactory returns the global service factory
func GetGlobalFactory() *ServiceFactory {
	return globalFactory
}

// Backward compatibility functions for existing code

// TriggerGlobalToolRefresh triggers tool refresh on the global service
func TriggerGlobalToolRefresh() {
	service := GetGlobalService()
	if service != nil {
		service.TriggerToolRefresh()
	} else {
		log.Printf("[AI Global] Warning: Cannot refresh tools, global service not initialized")
	}
}

// SendGlobalMessage sends a message using the global service
func SendGlobalMessage(message string) (string, error) {
	service := GetGlobalService()
	if service == nil {
		return "", &AIError{
			Type:     ErrorTypeValidation,
			Message:  "global AI service not initialized",
			Provider: "global",
		}
	}

	return service.SendMessage(message)
}

// SendGlobalMessageWithContext sends a message with context using the global service
func SendGlobalMessageWithContext(message string, contextID string) (string, error) {
	service := GetGlobalService()
	if service == nil {
		return "", &AIError{
			Type:     ErrorTypeValidation,
			Message:  "global AI service not initialized",
			Provider: "global",
		}
	}

	return service.SendMessageWithContext(message, contextID)
}

// Note: Dependency injection interfaces are defined in tool_manager.go to avoid duplication

// SetupGlobalDependencies configures dependency injection for the global AI service
func SetupGlobalDependencies(
	registryAccess RegistryAccess,
	appRunner AppRunner,
	appCreator AppCreator,
	embeddingSearch EmbeddingSearch,
) error {
	service := GetGlobalService()
	if service == nil {
		return &AIError{
			Type:     ErrorTypeValidation,
			Message:  "global AI service must be initialized before setting up dependencies",
			Provider: "global",
		}
	}

	log.Printf("[AI DI] Setting up dependency injection for global AI service")

	// Use type assertion to access the underlying tool manager
	// This works for both Claude and OpenAI services since they both have toolManager fields
	switch svc := service.(type) {
	case interface{ GetToolManager() *ToolManager }:
		toolManager := svc.GetToolManager()
		if toolManager != nil {
			log.Printf("[AI DI] Configuring tool manager dependencies")
			toolManager.SetRegistryAccess(registryAccess)
			toolManager.SetAppRunner(appRunner)
			toolManager.SetAppCreator(appCreator)
			toolManager.SetEmbeddingSearch(embeddingSearch)

			// Refresh tools to load newly available tools with dependencies
			toolManager.RefreshTools()
			log.Printf("[AI DI] Tool manager dependencies configured and tools refreshed")
		} else {
			log.Printf("[AI DI] Warning: Tool manager not available in service")
		}
	default:
		// Fallback: try to access tool manager via reflection-like approach
		log.Printf("[AI DI] Service type: %T - attempting direct field access", service)
		// For now, we'll attempt a more direct approach by calling the refresh method
		// which should trigger tool loading with whatever dependencies are available
		service.RefreshTools()
		log.Printf("[AI DI] Called RefreshTools() as fallback dependency setup")
	}

	return nil
}

// SetupDependencyInjection is an alias for SetupGlobalDependencies for backward compatibility
func SetupDependencyInjection(
	registryAccess RegistryAccess,
	appRunner AppRunner,
	appCreator AppCreator,
	embeddingSearch EmbeddingSearch,
) error {
	return SetupGlobalDependencies(registryAccess, appRunner, appCreator, embeddingSearch)
}