package ai

import (
	"fmt"
	"time"

	"arcadia/modules/ai/core"
	"arcadia/modules/ai/models"
)

// Config holds the complete configuration for the AI module
type Config struct {
	// Core settings
	Provider         string                 `json:"provider" yaml:"provider"`
	Providers        map[string]interface{} `json:"providers" yaml:"providers"`
	DefaultMaxTokens int                    `json:"max_tokens" yaml:"max_tokens"`
	DefaultTimeout   time.Duration          `json:"timeout" yaml:"timeout"`

	// Context management
	MaxContextMessages int           `json:"max_context_messages" yaml:"max_context_messages"`
	ContextCompaction  bool          `json:"context_compaction" yaml:"context_compaction"`
	ContextTTL         time.Duration `json:"context_ttl" yaml:"context_ttl"`

	// Feature flags
	EnableMCP         bool `json:"enable_mcp" yaml:"enable_mcp"`
	EnablePersistence bool `json:"enable_persistence" yaml:"enable_persistence"`
	EnableMetrics     bool `json:"enable_metrics" yaml:"enable_metrics"`
	EnableEventBus    bool `json:"enable_event_bus" yaml:"enable_event_bus"`

	// Tool configuration
	MCPServerCmd       string   `json:"mcp_server_cmd" yaml:"mcp_server_cmd"`
	AllowedToolDomains []string `json:"allowed_tool_domains" yaml:"allowed_tool_domains"`
	DisabledTools      []string `json:"disabled_tools" yaml:"disabled_tools"`

	// Performance settings
	MaxConcurrentRequests int `json:"max_concurrent_requests" yaml:"max_concurrent_requests"`

	// Monitoring settings
	LogLevel string `json:"log_level" yaml:"log_level"`

	// Auto-search settings
	AutoSearchEnabled         bool    `json:"auto_search_enabled" yaml:"auto_search_enabled"`
	AutoSearchMaxResults      int     `json:"auto_search_max_results" yaml:"auto_search_max_results"`
	AutoSearchMinConfidence   float64 `json:"auto_search_min_confidence" yaml:"auto_search_min_confidence"`
	AutoSearchMaxContextSize  int     `json:"auto_search_max_context_size" yaml:"auto_search_max_context_size"`
	AutoSearchIncludeMetadata bool    `json:"auto_search_include_metadata" yaml:"auto_search_include_metadata"`
	AutoSearchTimeout         int     `json:"auto_search_timeout" yaml:"auto_search_timeout"`

	// API logging settings
	EnableAPILogging bool   `json:"enable_api_logging" yaml:"enable_api_logging"`
	APILogPath       string `json:"api_log_path" yaml:"api_log_path"`
	APILogMaxSizeMB  int    `json:"api_log_max_size_mb" yaml:"api_log_max_size_mb"`
}

// OpenAIConfig holds configuration specific to OpenAI
type OpenAIConfig struct {
	APIKey      string  `json:"api_key" yaml:"api_key"`
	BaseURL     string  `json:"base_url" yaml:"base_url"`
	Model       string  `json:"model" yaml:"model"`
	Temperature float64 `json:"temperature" yaml:"temperature"`
	MaxTokens   int     `json:"max_tokens" yaml:"max_tokens"`
	TopP        float64 `json:"top_p" yaml:"top_p"`
	Timeout     int     `json:"timeout_seconds" yaml:"timeout_seconds"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Provider:              "openai",
		Providers:             make(map[string]interface{}),
		DefaultMaxTokens:      4096,
		DefaultTimeout:        120 * time.Second,
		MaxContextMessages:    20,
		ContextCompaction:     true,
		ContextTTL:            60 * time.Minute,
		EnableMCP:             true,
		EnablePersistence:     false,
		EnableMetrics:         true,
		EnableEventBus:        false,
		MCPServerCmd:          "",
		AllowedToolDomains:    []string{},
		DisabledTools:         []string{},
		MaxConcurrentRequests: 10,
		LogLevel:              "info",
		// Auto-search defaults
		AutoSearchEnabled:         true,
		AutoSearchMaxResults:      3,
		AutoSearchMinConfidence:   0.5, // Lower threshold to be more permissive
		AutoSearchMaxContextSize:  4000,
		AutoSearchIncludeMetadata: true,
		AutoSearchTimeout:         5,
		// API logging defaults
		EnableAPILogging: true,
		APILogPath:       "logs/ai_api_payloads.log",
		APILogMaxSizeMB:  100,
	}
}

// Validate validates the configuration and sets defaults for missing values
func (c *Config) Validate() error {
	if c.Provider == "" {
		return fmt.Errorf("provider is required")
	}

	// Validate provider
	switch c.Provider {
	case "openai", "claude":
		// Valid providers
	default:
		return fmt.Errorf("unsupported provider: %s (supported: openai, claude)", c.Provider)
	}

	// Set defaults if missing
	if c.DefaultMaxTokens <= 0 {
		c.DefaultMaxTokens = 4096
	}

	if c.DefaultTimeout <= 0 {
		c.DefaultTimeout = 120 * time.Second
	}

	if c.MaxContextMessages <= 0 {
		c.MaxContextMessages = 20
	}

	if c.ContextTTL <= 0 {
		c.ContextTTL = 60 * time.Minute
	}

	if c.MaxConcurrentRequests <= 0 {
		c.MaxConcurrentRequests = 10
	}

	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	if c.Providers == nil {
		c.Providers = make(map[string]interface{})
	}

	if c.AllowedToolDomains == nil {
		c.AllowedToolDomains = []string{}
	}

	if c.DisabledTools == nil {
		c.DisabledTools = []string{}
	}

	// Auto-search validation and defaults
	if c.AutoSearchMaxResults <= 0 {
		c.AutoSearchMaxResults = 3
	}
	if c.AutoSearchMinConfidence <= 0 {
		c.AutoSearchMinConfidence = 0.5
	}
	if c.AutoSearchMaxContextSize <= 0 {
		c.AutoSearchMaxContextSize = 4000
	}
	if c.AutoSearchTimeout <= 0 {
		c.AutoSearchTimeout = 5
	}

	// API logging validation and defaults
	if c.APILogPath == "" {
		c.APILogPath = "logs/ai_api_payloads.log"
	}
	if c.APILogMaxSizeMB <= 0 {
		c.APILogMaxSizeMB = 100
	}

	return nil
}

// GetProviderConfig returns the configuration for a specific provider
func (c *Config) GetProviderConfig(providerName string) (map[string]interface{}, bool) {
	if config, exists := c.Providers[providerName]; exists {
		if configMap, ok := config.(map[string]interface{}); ok {
			return configMap, true
		}
	}
	return nil, false
}

// SetProviderConfig sets the configuration for a specific provider
func (c *Config) SetProviderConfig(providerName string, config map[string]interface{}) {
	if c.Providers == nil {
		c.Providers = make(map[string]interface{})
	}
	c.Providers[providerName] = config
}

// GetOpenAIConfig extracts OpenAI configuration from provider settings
func (c *Config) GetOpenAIConfig() (*OpenAIConfig, error) {
	providerConfig, exists := c.GetProviderConfig("openai")
	if !exists {
		return &OpenAIConfig{
			Model:       "gpt-4",
			Temperature: 0.7,
			MaxTokens:   c.DefaultMaxTokens,
			TopP:        1.0,
			Timeout:     int(c.DefaultTimeout.Seconds()),
		}, nil
	}

	config := &OpenAIConfig{
		Model:       "gpt-4",
		Temperature: 0.7,
		MaxTokens:   c.DefaultMaxTokens,
		TopP:        1.0,
		Timeout:     int(c.DefaultTimeout.Seconds()),
	}

	if apiKey, ok := providerConfig["api_key"].(string); ok {
		config.APIKey = apiKey
	}

	if baseURL, ok := providerConfig["base_url"].(string); ok {
		config.BaseURL = baseURL
	}

	if model, ok := providerConfig["model"].(string); ok {
		config.Model = model
	}

	if temp, ok := providerConfig["temperature"].(float64); ok {
		config.Temperature = temp
	}

	if maxTokens, ok := providerConfig["max_tokens"].(int); ok {
		config.MaxTokens = maxTokens
	}

	if topP, ok := providerConfig["top_p"].(float64); ok {
		config.TopP = topP
	}

	if timeout, ok := providerConfig["timeout_seconds"].(int); ok {
		config.Timeout = timeout
	}

	return config, nil
}

// IsToolDisabled checks if a tool is disabled
func (c *Config) IsToolDisabled(toolName string) bool {
	for _, disabled := range c.DisabledTools {
		if disabled == toolName {
			return true
		}
	}
	return false
}

// IsToolDomainAllowed checks if a tool domain is allowed
func (c *Config) IsToolDomainAllowed(domain string) bool {
	if len(c.AllowedToolDomains) == 0 {
		return true // Allow all if no restrictions
	}

	for _, allowed := range c.AllowedToolDomains {
		if allowed == domain || allowed == "*" {
			return true
		}
	}
	return false
}

// Clone creates a deep copy of the configuration
func (c *Config) Clone() *Config {
	clone := *c

	// Deep copy maps
	clone.Providers = make(map[string]interface{})
	for k, v := range c.Providers {
		clone.Providers[k] = v
	}

	// Deep copy slices
	clone.AllowedToolDomains = make([]string, len(c.AllowedToolDomains))
	copy(clone.AllowedToolDomains, c.AllowedToolDomains)

	clone.DisabledTools = make([]string, len(c.DisabledTools))
	copy(clone.DisabledTools, c.DisabledTools)

	return &clone
}

// ToModelsConfig converts this Config to a models.Config
func (c *Config) ToModelsConfig() *models.Config {
	return &models.Config{
		Provider:              c.Provider,
		Providers:             c.Providers,
		DefaultMaxTokens:      c.DefaultMaxTokens,
		DefaultTimeout:        c.DefaultTimeout,
		MaxContextMessages:    c.MaxContextMessages,
		ContextCompaction:     c.ContextCompaction,
		ContextTTL:            c.ContextTTL,
		EnableMCP:             c.EnableMCP,
		EnablePersistence:     c.EnablePersistence,
		EnableMetrics:         c.EnableMetrics,
		EnableEventBus:        c.EnableEventBus,
		MCPServerCmd:          c.MCPServerCmd,
		AllowedToolDomains:    c.AllowedToolDomains,
		DisabledTools:         c.DisabledTools,
		MaxConcurrentRequests: c.MaxConcurrentRequests,
		LogLevel:              c.LogLevel,
	}
}

// GetAutoSearchConfig returns the auto-search configuration
func (c *Config) GetAutoSearchConfig() *core.AutoSearchConfig {
	return &core.AutoSearchConfig{
		Enabled:         c.AutoSearchEnabled,
		MaxResults:      c.AutoSearchMaxResults,
		MinConfidence:   c.AutoSearchMinConfidence,
		MaxContextSize:  c.AutoSearchMaxContextSize,
		IncludeMetadata: c.AutoSearchIncludeMetadata,
		Timeout:         c.AutoSearchTimeout,
	}
}
