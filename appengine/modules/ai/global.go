package ai

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"arcadia/modules/ai/interfaces"
	"arcadia/modules/ai/models"
)

// Global service management
var (
	globalAIModule AIModule
	globalMutex    sync.RWMutex
	initialized    bool
)

// GetGlobalService returns the global AI service instance
func GetGlobalService() AIModule {
	globalMutex.RLock()
	defer globalMutex.RUnlock()
	return globalAIModule
}

// IsGlobalServiceInitialized checks if the global AI service is initialized
func IsGlobalServiceInitialized() bool {
	globalMutex.RLock()
	defer globalMutex.RUnlock()
	return initialized && globalAIModule != nil
}

// SimpleAILogger implements the AI module's Logger interface
type SimpleAILogger struct{}

func NewSimpleAILogger() *SimpleAILogger {
	return &SimpleAILogger{}
}

func (sl *SimpleAILogger) Debug(msg string, fields ...interface{}) {
	fmt.Printf("[DEBUG] %s %v\n", msg, fields)
}

func (sl *SimpleAILogger) Info(msg string, fields ...interface{}) {
	fmt.Printf("[INFO] %s %v\n", msg, fields)
}

func (sl *SimpleAILogger) Warn(msg string, fields ...interface{}) {
	fmt.Printf("[WARN] %s %v\n", msg, fields)
}

func (sl *SimpleAILogger) Error(msg string, fields ...interface{}) {
	fmt.Printf("[ERROR] %s %v\n", msg, fields)
}

func (sl *SimpleAILogger) WithFields(fields map[string]interface{}) interfaces.Logger {
	// For simplicity, return self since we're just printing
	return sl
}

func (sl *SimpleAILogger) WithContext(ctx context.Context) interfaces.Logger {
	// For simplicity, return self since we're just printing
	return sl
}

// InitializeGlobalServiceFromConfig initializes the global AI service from configuration
func InitializeGlobalServiceFromConfig() error {
	// Create logger first for logging environment variable issues
	logger := NewSimpleAILogger()

	// Read environment variables
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	aiModel := os.Getenv("AI_MODEL")
	if aiModel == "" {
		aiModel = "gpt-4" // default
	}
	aiProvider := os.Getenv("AI_PROVIDER")
	if aiProvider == "" {
		aiProvider = "openai" // default
	}

	// Log warning if OPENAI_API_KEY is empty (but never log the actual key value)
	if openaiAPIKey == "" {
		logger.Warn("OPENAI_API_KEY environment variable is not set - API calls will fail without a valid API key")
	}

	// Load default configuration
	config := map[string]interface{}{
		"provider":           aiProvider,
		"enable_mcp":         true,
		"enable_persistence": true,
		"enable_metrics":     true,
		"max_tokens":         4096,
		"timeout":            120,
	}

	// Create dependencies struct with required logger
	deps := &interfaces.Dependencies{
		Logger: logger,
		// Other dependencies are optional and can be nil
	}

	// Parse configuration into proper Config struct
	aiConfig := DefaultConfig()

	// Set provider from environment variable
	aiConfig.Provider = aiProvider

	// Create provider configuration map for OpenAI
	aiConfig.Providers["openai"] = map[string]interface{}{
		"api_key": openaiAPIKey,
		"model":   aiModel,
	}

	// Apply configuration overrides
	if provider, ok := config["provider"].(string); ok {
		aiConfig.Provider = provider
	}

	if maxTokens, ok := config["max_tokens"]; ok {
		switch v := maxTokens.(type) {
		case int:
			aiConfig.DefaultMaxTokens = v
		case float64:
			aiConfig.DefaultMaxTokens = int(v)
		}
	}

	if timeout, ok := config["timeout"]; ok {
		switch v := timeout.(type) {
		case int:
			aiConfig.DefaultTimeout = time.Duration(v) * time.Second
		case float64:
			aiConfig.DefaultTimeout = time.Duration(v) * time.Second
		case string:
			if d, err := time.ParseDuration(v); err == nil {
				aiConfig.DefaultTimeout = d
			}
		}
	}

	if enableMCP, ok := config["enable_mcp"].(bool); ok {
		aiConfig.EnableMCP = enableMCP
	}

	if enablePersistence, ok := config["enable_persistence"].(bool); ok {
		aiConfig.EnablePersistence = enablePersistence
	}

	if enableMetrics, ok := config["enable_metrics"].(bool); ok {
		aiConfig.EnableMetrics = enableMetrics
	}

	// Create the AI module with proper dependencies
	ctx := context.Background()
	module, err := NewModule(ctx, aiConfig, deps)
	if err != nil {
		return fmt.Errorf("failed to create AI module: %w", err)
	}

	// Store globally
	globalMutex.Lock()
	globalAIModule = module
	initialized = true
	globalMutex.Unlock()

	// Start the module
	if err := module.Start(ctx); err != nil {
		return fmt.Errorf("failed to start AI module: %w", err)
	}

	return nil
}

// SetGlobalService sets the global AI service instance
func SetGlobalService(module AIModule) {
	globalMutex.Lock()
	defer globalMutex.Unlock()
	globalAIModule = module
	initialized = module != nil
}

// SetupGlobalDependencies configures the global AI service with external dependencies
func SetupGlobalDependencies(registryAccess interfaces.RegistryAccess, appRunner interfaces.AppRunner, appCreator interfaces.AppCreator, embeddingSearch interfaces.EmbeddingSearch) error {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	if globalAIModule == nil {
		return fmt.Errorf("global AI service not initialized")
	}

	// Get the underlying module to access internals
	if module, ok := globalAIModule.(*Module); ok {
		// Update dependencies
		module.deps.RegistryAccess = registryAccess
		module.deps.AppRunner = appRunner
		module.deps.AppCreator = appCreator
		module.deps.EmbeddingSearch = embeddingSearch

		// Update tool manager dependencies if tool manager is available
		if module.toolManager != nil {
			ctx := context.Background()
			if err := module.toolManager.UpdateDependencies(ctx, module.deps); err != nil {
				// Log warning but don't fail
				if module.deps.Logger != nil {
					module.deps.Logger.Warn("Failed to update tool manager dependencies", "error", err)
				}
			}

			// Refresh tools after updating dependencies
			if err := module.toolManager.RefreshTools(ctx); err != nil {
				// Log warning but don't fail
				if module.deps.Logger != nil {
					module.deps.Logger.Warn("Failed to refresh tools after dependency setup", "error", err)
				}
			}
		}
	}

	return nil
}

// StopGlobalService stops and cleans up the global AI service
func StopGlobalService() error {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	if globalAIModule != nil && initialized {
		ctx := context.Background()
		if err := globalAIModule.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop global AI service: %w", err)
		}
	}

	globalAIModule = nil
	initialized = false
	return nil
}

// Type aliases for backward compatibility
type Document = models.Document
type DocumentSearchResult = models.DocumentSearchResult
type EnhancedDocumentSearchResult = models.EnhancedDocumentSearchResult
type ChunkResult = models.ChunkResult
type SearchConfig = models.SearchConfig
type EmbeddingSearch = interfaces.EmbeddingSearch

// Service interface for backward compatibility
type Service interface {
	SendMessage(message string) (string, error)
	SendMessageWithContext(message string, contextID string) (string, error)
}

// serviceAdapter adapts AIModule to the legacy Service interface
type serviceAdapter struct {
	module AIModule
}

func (s *serviceAdapter) SendMessage(message string) (string, error) {
	ctx := context.Background()
	return s.module.SendMessage(ctx, message)
}

func (s *serviceAdapter) SendMessageWithContext(message string, contextID string) (string, error) {
	ctx := context.Background()
	return s.module.SendMessageWithContext(ctx, message, contextID)
}

// GetService returns a Service interface for backward compatibility
func GetService() Service {
	module := GetGlobalService()
	if module == nil {
		return nil
	}
	return &serviceAdapter{module: module}
}