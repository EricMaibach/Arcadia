package ai

import (
	"context"
	"fmt"
	"os"
	"time"

	"arcadia/modules/ai/core"
	"arcadia/modules/ai/interfaces"
	"arcadia/modules/ai/models"
	"arcadia/pkg/logging"
)

// NewAIModuleFromEnv creates a new AI module instance from environment variables
// This replaces the global service pattern with explicit dependency injection
func NewAIModuleFromEnv(ctx context.Context, logger logging.Logger) (AIModule, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	// Read environment variables
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	aiModel := os.Getenv("AI_MODEL")
	if aiModel == "" {
		aiModel = "gpt-4"
	}
	aiProvider := os.Getenv("AI_PROVIDER")
	if aiProvider == "" {
		aiProvider = "openai"
	}

	// Log warning if OPENAI_API_KEY is empty
	if openaiAPIKey == "" {
		logger.Warn(ctx, "OPENAI_API_KEY environment variable is not set - API calls will fail without a valid API key")
	}

	// Create dependencies
	deps := &interfaces.Dependencies{
		Logger: logger,
	}

	// Create configuration
	aiConfig := DefaultConfig()
	aiConfig.Provider = aiProvider
	aiConfig.Providers["openai"] = map[string]interface{}{
		"api_key": openaiAPIKey,
		"model":   aiModel,
	}

	// Create and start the module
	module, err := NewModule(ctx, aiConfig, deps)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI module: %w", err)
	}

	if err := module.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start AI module: %w", err)
	}

	return module, nil
}

// NewAIModuleWithConfig creates a new AI module instance with custom configuration
func NewAIModuleWithConfig(ctx context.Context, config map[string]interface{}, deps *interfaces.Dependencies) (AIModule, error) {
	// Ensure logger is available
	if deps == nil || deps.Logger == nil {
		return nil, fmt.Errorf("logger is required in dependencies")
	}

	// Parse configuration
	aiConfig := DefaultConfig()

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

	if providers, ok := config["providers"].(map[string]interface{}); ok {
		aiConfig.Providers = providers
	}

	// Create and start the module
	module, err := NewModule(ctx, aiConfig, deps)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI module: %w", err)
	}

	if err := module.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start AI module: %w", err)
	}

	return module, nil
}

// SetupModuleDependencies configures an AI module with external dependencies
// This should be called after creating the module if you need to inject additional dependencies
func SetupModuleDependencies(module AIModule, registryAccess interfaces.RegistryAccess, appRunner interfaces.AppRunner, appCreator interfaces.AppCreator, embeddingSearch interfaces.EmbeddingSearch) error {
	// Get the underlying module to access internals
	if m, ok := module.(*Module); ok {
		// Update dependencies
		m.deps.RegistryAccess = registryAccess
		m.deps.AppRunner = appRunner
		m.deps.AppCreator = appCreator
		m.deps.EmbeddingSearch = embeddingSearch

		// Initialize auto-search components if enabled and embedding search is now available
		if m.config.AutoSearchEnabled && embeddingSearch != nil {
			m.queryAnalyzer = core.NewQueryAnalyzer(m.deps.Logger, m.deps.Metrics)

			autoSearchConfig := m.config.GetAutoSearchConfig()
			m.autoSearchExecutor = core.NewAutoSearchExecutor(
				embeddingSearch,
				autoSearchConfig,
				m.deps.Logger,
				m.deps.Metrics,
			)

			m.contextBuilder = core.NewContextBuilder(m.deps.Logger)

			// Create context for logging
			ctx := context.Background()
			if m.deps.Logger != nil {
				m.deps.Logger.Info(ctx, "Auto-search components initialized after dependency setup",
					"max_results", autoSearchConfig.MaxResults,
					"min_confidence", autoSearchConfig.MinConfidence,
					"max_context_size", autoSearchConfig.MaxContextSize)
			}
		}

		// Update tool manager dependencies if tool manager is available
		if m.toolManager != nil {
			ctx := context.Background()
			if err := m.toolManager.UpdateDependencies(ctx, m.deps); err != nil {
				if m.deps.Logger != nil {
					m.deps.Logger.Warn(ctx, "Failed to update tool manager dependencies", "error", err)
				}
			}

			// Refresh tools after updating dependencies
			if err := m.toolManager.RefreshTools(ctx); err != nil {
				if m.deps.Logger != nil {
					m.deps.Logger.Warn(ctx, "Failed to refresh tools after dependency setup", "error", err)
				}
			}
		}
	}

	return nil
}

// Type aliases for backward compatibility
type Document = models.Document
type DocumentSearchResult = models.DocumentSearchResult
type EnhancedDocumentSearchResult = models.EnhancedDocumentSearchResult
type ChunkResult = models.ChunkResult
type SearchConfig = models.SearchConfig
type EmbeddingSearch = interfaces.EmbeddingSearch
