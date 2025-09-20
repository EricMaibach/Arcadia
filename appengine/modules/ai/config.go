package ai

import (
	"fmt"
	"time"

	"arcadia/modules/ai/models"
)

// Config holds the complete configuration for the AI module
type Config struct {
	// Core settings
	Provider       string                 `json:"provider" yaml:"provider"`
	Providers      map[string]interface{} `json:"providers" yaml:"providers"`
	DefaultMaxTokens int                  `json:"max_tokens" yaml:"max_tokens"`
	DefaultTimeout   time.Duration        `json:"timeout" yaml:"timeout"`

	// Context management
	MaxContextMessages int           `json:"max_context_messages" yaml:"max_context_messages"`
	ContextCompaction  bool          `json:"context_compaction" yaml:"context_compaction"`
	ContextTTL         time.Duration `json:"context_ttl" yaml:"context_ttl"`

	// Feature flags
	EnableMCP         bool `json:"enable_mcp" yaml:"enable_mcp"`
	EnablePersistence bool `json:"enable_persistence" yaml:"enable_persistence"`
	EnableMetrics     bool `json:"enable_metrics" yaml:"enable_metrics"`
	EnableCaching     bool `json:"enable_caching" yaml:"enable_caching"`
	EnableEventBus    bool `json:"enable_event_bus" yaml:"enable_event_bus"`

	// Tool configuration
	MCPServerCmd       string   `json:"mcp_server_cmd" yaml:"mcp_server_cmd"`
	AllowedToolDomains []string `json:"allowed_tool_domains" yaml:"allowed_tool_domains"`
	DisabledTools      []string `json:"disabled_tools" yaml:"disabled_tools"`

	// Performance settings
	MaxConcurrentRequests int           `json:"max_concurrent_requests" yaml:"max_concurrent_requests"`
	RateLimitRequests     int           `json:"rate_limit_requests" yaml:"rate_limit_requests"`
	RateLimitWindow       time.Duration `json:"rate_limit_window" yaml:"rate_limit_window"`

	// Storage settings
	PersistenceBackend string            `json:"persistence_backend" yaml:"persistence_backend"`
	StorageConfig      map[string]string `json:"storage_config" yaml:"storage_config"`

	// Monitoring settings
	MetricsPrefix    string        `json:"metrics_prefix" yaml:"metrics_prefix"`
	HealthCheckPath  string        `json:"health_check_path" yaml:"health_check_path"`
	LogLevel         string        `json:"log_level" yaml:"log_level"`
	LogFormat        string        `json:"log_format" yaml:"log_format"`
	TracingEnabled   bool          `json:"tracing_enabled" yaml:"tracing_enabled"`
	MetricsInterval  time.Duration `json:"metrics_interval" yaml:"metrics_interval"`

	// Security settings
	AllowedOrigins []string `json:"allowed_origins" yaml:"allowed_origins"`
	RequireAuth    bool     `json:"require_auth" yaml:"require_auth"`
	AuthProvider   string   `json:"auth_provider" yaml:"auth_provider"`

	// Cache settings
	CacheBackend string            `json:"cache_backend" yaml:"cache_backend"`
	CacheTTL     time.Duration     `json:"cache_ttl" yaml:"cache_ttl"`
	CacheConfig  map[string]string `json:"cache_config" yaml:"cache_config"`
}

// ProviderConfig holds configuration for a specific AI provider
type ProviderConfig struct {
	Name     string                 `json:"name" yaml:"name"`
	Type     string                 `json:"type" yaml:"type"`
	Enabled  bool                   `json:"enabled" yaml:"enabled"`
	Settings map[string]interface{} `json:"settings" yaml:"settings"`
	Priority int                    `json:"priority" yaml:"priority"`
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

// ClaudeConfig holds configuration specific to Claude/Anthropic
type ClaudeConfig struct {
	APIKey      string  `json:"api_key" yaml:"api_key"`
	BaseURL     string  `json:"base_url" yaml:"base_url"`
	Model       string  `json:"model" yaml:"model"`
	Temperature float64 `json:"temperature" yaml:"temperature"`
	MaxTokens   int     `json:"max_tokens" yaml:"max_tokens"`
	TopP        float64 `json:"top_p" yaml:"top_p"`
	TopK        int     `json:"top_k" yaml:"top_k"`
	Timeout     int     `json:"timeout_seconds" yaml:"timeout_seconds"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Provider:              "openai",
		Providers:            make(map[string]interface{}),
		DefaultMaxTokens:      4096,
		DefaultTimeout:        120 * time.Second,
		MaxContextMessages:    20,
		ContextCompaction:     true,
		ContextTTL:           60 * time.Minute,
		EnableMCP:            true,
		EnablePersistence:    false,
		EnableMetrics:        true,
		EnableCaching:        false,
		EnableEventBus:       false,
		MCPServerCmd:         "",
		AllowedToolDomains:   []string{},
		DisabledTools:        []string{},
		MaxConcurrentRequests: 10,
		RateLimitRequests:    100,
		RateLimitWindow:      1 * time.Minute,
		PersistenceBackend:   "memory",
		StorageConfig:        make(map[string]string),
		MetricsPrefix:        "arcadia_ai",
		HealthCheckPath:      "/health",
		LogLevel:             "info",
		LogFormat:            "json",
		TracingEnabled:       false,
		MetricsInterval:      30 * time.Second,
		AllowedOrigins:       []string{"*"},
		RequireAuth:          false,
		AuthProvider:         "",
		CacheBackend:         "memory",
		CacheTTL:            5 * time.Minute,
		CacheConfig:         make(map[string]string),
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

	if c.RateLimitRequests <= 0 {
		c.RateLimitRequests = 100
	}

	if c.RateLimitWindow <= 0 {
		c.RateLimitWindow = 1 * time.Minute
	}

	if c.MetricsInterval <= 0 {
		c.MetricsInterval = 30 * time.Second
	}

	if c.CacheTTL <= 0 {
		c.CacheTTL = 5 * time.Minute
	}

	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	if c.LogFormat == "" {
		c.LogFormat = "json"
	}

	if c.MetricsPrefix == "" {
		c.MetricsPrefix = "arcadia_ai"
	}

	if c.HealthCheckPath == "" {
		c.HealthCheckPath = "/health"
	}

	if c.PersistenceBackend == "" {
		c.PersistenceBackend = "memory"
	}

	if c.CacheBackend == "" {
		c.CacheBackend = "memory"
	}

	if c.Providers == nil {
		c.Providers = make(map[string]interface{})
	}

	if c.StorageConfig == nil {
		c.StorageConfig = make(map[string]string)
	}

	if c.CacheConfig == nil {
		c.CacheConfig = make(map[string]string)
	}

	if c.AllowedToolDomains == nil {
		c.AllowedToolDomains = []string{}
	}

	if c.DisabledTools == nil {
		c.DisabledTools = []string{}
	}

	if c.AllowedOrigins == nil {
		c.AllowedOrigins = []string{"*"}
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

// GetClaudeConfig extracts Claude configuration from provider settings
func (c *Config) GetClaudeConfig() (*ClaudeConfig, error) {
	providerConfig, exists := c.GetProviderConfig("claude")
	if !exists {
		return &ClaudeConfig{
			Model:       "claude-3-sonnet-20240229",
			Temperature: 0.7,
			MaxTokens:   c.DefaultMaxTokens,
			TopP:        1.0,
			TopK:        0,
			Timeout:     int(c.DefaultTimeout.Seconds()),
		}, nil
	}

	config := &ClaudeConfig{
		Model:       "claude-3-sonnet-20240229",
		Temperature: 0.7,
		MaxTokens:   c.DefaultMaxTokens,
		TopP:        1.0,
		TopK:        0,
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

	if topK, ok := providerConfig["top_k"].(int); ok {
		config.TopK = topK
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

// GetTimeoutDuration returns the timeout as a time.Duration
func (c *Config) GetTimeoutDuration() time.Duration {
	return c.DefaultTimeout
}

// GetContextTTLDuration returns the context TTL as a time.Duration
func (c *Config) GetContextTTLDuration() time.Duration {
	return c.ContextTTL
}

// GetMetricsIntervalDuration returns the metrics interval as a time.Duration
func (c *Config) GetMetricsIntervalDuration() time.Duration {
	return c.MetricsInterval
}

// GetCacheTTLDuration returns the cache TTL as a time.Duration
func (c *Config) GetCacheTTLDuration() time.Duration {
	return c.CacheTTL
}

// GetRateLimitWindowDuration returns the rate limit window as a time.Duration
func (c *Config) GetRateLimitWindowDuration() time.Duration {
	return c.RateLimitWindow
}

// Clone creates a deep copy of the configuration
func (c *Config) Clone() *Config {
	clone := *c

	// Deep copy maps
	clone.Providers = make(map[string]interface{})
	for k, v := range c.Providers {
		clone.Providers[k] = v
	}

	clone.StorageConfig = make(map[string]string)
	for k, v := range c.StorageConfig {
		clone.StorageConfig[k] = v
	}

	clone.CacheConfig = make(map[string]string)
	for k, v := range c.CacheConfig {
		clone.CacheConfig[k] = v
	}

	// Deep copy slices
	clone.AllowedToolDomains = make([]string, len(c.AllowedToolDomains))
	copy(clone.AllowedToolDomains, c.AllowedToolDomains)

	clone.DisabledTools = make([]string, len(c.DisabledTools))
	copy(clone.DisabledTools, c.DisabledTools)

	clone.AllowedOrigins = make([]string, len(c.AllowedOrigins))
	copy(clone.AllowedOrigins, c.AllowedOrigins)

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
		EnableCaching:         c.EnableCaching,
		EnableEventBus:        c.EnableEventBus,
		MCPServerCmd:          c.MCPServerCmd,
		AllowedToolDomains:    c.AllowedToolDomains,
		DisabledTools:         c.DisabledTools,
		MaxConcurrentRequests: c.MaxConcurrentRequests,
		RateLimitRequests:     c.RateLimitRequests,
		RateLimitWindow:       c.RateLimitWindow,
		PersistenceBackend:    c.PersistenceBackend,
		StorageConfig:         c.StorageConfig,
		MetricsPrefix:         c.MetricsPrefix,
		HealthCheckPath:       c.HealthCheckPath,
		LogLevel:              c.LogLevel,
		LogFormat:             c.LogFormat,
		TracingEnabled:        c.TracingEnabled,
		MetricsInterval:       c.MetricsInterval,
		AllowedOrigins:        c.AllowedOrigins,
		RequireAuth:           c.RequireAuth,
		AuthProvider:          c.AuthProvider,
		CacheBackend:          c.CacheBackend,
		CacheTTL:              c.CacheTTL,
		CacheConfig:           c.CacheConfig,
	}
}