package ai

import (
	"encoding/json"
	"fmt"
	"os"
)

// ConfigBuilder helps build AI configurations fluently
type ConfigBuilder struct {
	config *AIConfig
}

// NewConfigBuilder creates a new configuration builder
func NewConfigBuilder(provider string) *ConfigBuilder {
	return &ConfigBuilder{
		config: &AIConfig{
			Provider:           provider,
			MaxTokens:          4096,
			TimeoutSeconds:     120,
			MaxContextMessages: 20,
			ContextCompaction:  true,
			ContextTTLMinutes:  60,
			EnableMCP:          true,
			ProviderSettings:   make(map[string]any),
		},
	}
}

// WithAPIKey sets the API key for the provider
func (cb *ConfigBuilder) WithAPIKey(apiKey string) *ConfigBuilder {
	cb.config.ProviderSettings["api_key"] = apiKey
	return cb
}

// WithModel sets the model for the provider
func (cb *ConfigBuilder) WithModel(model string) *ConfigBuilder {
	cb.config.ProviderSettings["model"] = model
	return cb
}

// WithMaxTokens sets the maximum tokens
func (cb *ConfigBuilder) WithMaxTokens(maxTokens int) *ConfigBuilder {
	cb.config.MaxTokens = maxTokens
	return cb
}

// WithTimeout sets the request timeout in seconds
func (cb *ConfigBuilder) WithTimeout(seconds int) *ConfigBuilder {
	cb.config.TimeoutSeconds = seconds
	return cb
}

// WithContextSettings configures context management
func (cb *ConfigBuilder) WithContextSettings(maxMessages int, compaction bool, ttlMinutes int) *ConfigBuilder {
	cb.config.MaxContextMessages = maxMessages
	cb.config.ContextCompaction = compaction
	cb.config.ContextTTLMinutes = ttlMinutes
	return cb
}

// WithMCP enables or disables MCP tools
func (cb *ConfigBuilder) WithMCP(enabled bool, serverCmd string) *ConfigBuilder {
	cb.config.EnableMCP = enabled
	cb.config.MCPServerCmd = serverCmd
	return cb
}

// WithProviderSetting sets a custom provider-specific setting
func (cb *ConfigBuilder) WithProviderSetting(key string, value any) *ConfigBuilder {
	cb.config.ProviderSettings[key] = value
	return cb
}

// Build returns the final configuration
func (cb *ConfigBuilder) Build() *AIConfig {
	return cb.config
}

// Configuration loading functions

// LoadConfigFromFile loads AI configuration from a JSON file
func LoadConfigFromFile(filename string) (*AIConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config AIConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Expand environment variables in provider settings
	expandEnvironmentVariables(config.ProviderSettings)

	return &config, nil
}

// LoadConfigFromDefaultFile loads AI configuration from a default file based on provider
func LoadConfigFromDefaultFile(provider string) (*AIConfig, error) {
	filename := fmt.Sprintf("config/ai-%s.json", provider)
	return LoadConfigFromFile(filename)
}

// expandEnvironmentVariables expands environment variables in provider settings
func expandEnvironmentVariables(settings map[string]any) {
	for key, value := range settings {
		if str, ok := value.(string); ok {
			if len(str) > 2 && str[0] == '$' && str[1] == '{' && str[len(str)-1] == '}' {
				envVar := str[2 : len(str)-1]
				if envValue := os.Getenv(envVar); envValue != "" {
					settings[key] = envValue
				}
			}
		}
	}
}

// LoadConfigFromEnv loads AI configuration from environment variables
func LoadConfigFromEnv() (*AIConfig, error) {
	provider := os.Getenv("AI_PROVIDER")
	if provider == "" {
		return nil, fmt.Errorf("AI_PROVIDER environment variable is required")
	}

	// Try provider-specific API key first, then fall back to generic AI_API_KEY
	var apiKey string
	switch provider {
	case "openai":
		apiKey = os.Getenv("OPENAI_API_KEY")
	case "claude":
		apiKey = os.Getenv("CLAUDE_API_KEY")
	case "anthropic":
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	// Fall back to generic AI_API_KEY if provider-specific key not found
	if apiKey == "" {
		apiKey = os.Getenv("AI_API_KEY")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API key environment variable is required (OPENAI_API_KEY for OpenAI, CLAUDE_API_KEY/ANTHROPIC_API_KEY for Claude/Anthropic, or AI_API_KEY for generic)")
	}

	config := NewConfigBuilder(provider).
		WithAPIKey(apiKey).
		Build()

	// Load optional environment variables
	if model := os.Getenv("AI_MODEL"); model != "" {
		config.ProviderSettings["model"] = model
	}

	if baseURL := os.Getenv("AI_BASE_URL"); baseURL != "" {
		config.ProviderSettings["base_url"] = baseURL
	}

	return config, nil
}

// SaveConfigToFile saves AI configuration to a JSON file
func SaveConfigToFile(config *AIConfig, filename string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}