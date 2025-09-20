package models

import (
	"fmt"
	"time"
)

// MessageRole defines the role of a message in a conversation
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
)

// Message represents a single message in a conversation
type Message struct {
	Role       MessageRole            `json:"role"`
	Content    any                    `json:"content"` // string or structured content
	ToolCalls  []ToolCall             `json:"tool_calls,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"` // For tool response messages (OpenAI requirement)
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ToolCall represents a tool invocation and its result
type ToolCall struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Input    any     `json:"input"`
	Result   string  `json:"result,omitempty"`
	Error    string  `json:"error,omitempty"`
	Duration float64 `json:"duration_ms,omitempty"`
}

// Tool represents an available tool that the AI can use
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema any                    `json:"input_schema"`
	Category    string                 `json:"category,omitempty"`
	Provider    string                 `json:"provider,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Enabled     bool                   `json:"enabled"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ProviderInfo contains metadata about the AI provider and its capabilities
type ProviderInfo struct {
	Name         string                 `json:"name"`         // Provider name (e.g., "openai")
	Type         string                 `json:"type"`         // Provider type
	Version      string                 `json:"version"`      // Provider version
	Model        string                 `json:"model"`        // Model being used (e.g., "gpt-4")
	Features     []string               `json:"features"`     // Supported features (e.g., "tools", "context", "vision")
	Capabilities []string               `json:"capabilities"` // Provider capabilities
	Status       string                 `json:"status"`       // Provider status
	Config       map[string]interface{} `json:"config,omitempty"`
}

// ConversationContext represents a conversation context with message history
type ConversationContext struct {
	ID           string                 `json:"id"`
	Messages     []Message              `json:"messages"`
	Stats        ContextStats           `json:"stats"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	LastAccessed time.Time              `json:"last_accessed"`
}

// ContextStats represents statistics about a conversation context
type ContextStats struct {
	Messages    int       `json:"messages"`
	Tokens      int       `json:"tokens"`
	LastUpdated time.Time `json:"last_updated"`
	CreatedAt   time.Time `json:"created_at"`
	Duration    float64   `json:"duration_seconds"`
}

// Config represents the AI module configuration
type Config struct {
	Provider              string                 `json:"provider"`
	Providers             map[string]interface{} `json:"providers"`
	DefaultMaxTokens      int                    `json:"max_tokens"`
	DefaultTimeout        time.Duration          `json:"timeout"`
	MaxContextMessages    int                    `json:"max_context_messages"`
	ContextCompaction     bool                   `json:"context_compaction"`
	ContextTTL            time.Duration          `json:"context_ttl"`
	EnableMCP             bool                   `json:"enable_mcp"`
	EnablePersistence     bool                   `json:"enable_persistence"`
	EnableMetrics         bool                   `json:"enable_metrics"`
	EnableCaching         bool                   `json:"enable_caching"`
	EnableEventBus        bool                   `json:"enable_event_bus"`
	MCPServerCmd          string                 `json:"mcp_server_cmd"`
	AllowedToolDomains    []string               `json:"allowed_tool_domains"`
	DisabledTools         []string               `json:"disabled_tools"`
	MaxConcurrentRequests int                    `json:"max_concurrent_requests"`
	RateLimitRequests     int                    `json:"rate_limit_requests"`
	RateLimitWindow       time.Duration          `json:"rate_limit_window"`
	PersistenceBackend    string                 `json:"persistence_backend"`
	StorageConfig         map[string]string      `json:"storage_config"`
	MetricsPrefix         string                 `json:"metrics_prefix"`
	HealthCheckPath       string                 `json:"health_check_path"`
	LogLevel              string                 `json:"log_level"`
	LogFormat             string                 `json:"log_format"`
	TracingEnabled        bool                   `json:"tracing_enabled"`
	MetricsInterval       time.Duration          `json:"metrics_interval"`
	AllowedOrigins        []string               `json:"allowed_origins"`
	RequireAuth           bool                   `json:"require_auth"`
	AuthProvider          string                 `json:"auth_provider"`
	CacheBackend          string                 `json:"cache_backend"`
	CacheTTL              time.Duration          `json:"cache_ttl"`
	CacheConfig           map[string]string      `json:"cache_config"`
}

// MessageRequest represents a request to send a message
type MessageRequest struct {
	Message        string                 `json:"message"`
	ContextID      string                 `json:"context_id,omitempty"`
	MaxTokens      int                    `json:"max_tokens,omitempty"`
	Temperature    float64                `json:"temperature,omitempty"`
	TimeoutSeconds int                    `json:"timeout_seconds,omitempty"`
	ToolChoice     string                 `json:"tool_choice,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	RetryCount     int                    `json:"retry_count,omitempty"`
	StreamResponse bool                   `json:"stream_response,omitempty"`
	IncludeContext bool                   `json:"include_context,omitempty"`
}

// MessageResponse represents a response from sending a message
type MessageResponse struct {
	Response     string                 `json:"response"`
	ContextID    string                 `json:"context_id,omitempty"`
	TokensUsed   int                    `json:"tokens_used,omitempty"`
	Duration     float64                `json:"duration_ms"`
	ToolCalls    []ToolCall             `json:"tool_calls,omitempty"`
	ProviderInfo *ProviderInfo          `json:"provider_info,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Cached       bool                   `json:"cached,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
}

// CompletionRequest represents a request for text completion
type CompletionRequest struct {
	Prompt      string                 `json:"prompt"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	TopP        float64                `json:"top_p,omitempty"`
	TopK        int                    `json:"top_k,omitempty"`
	Stop        []string               `json:"stop,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// CompletionResponse represents a response from text completion
type CompletionResponse struct {
	Text         string                 `json:"text"`
	TokensUsed   int                    `json:"tokens_used,omitempty"`
	Duration     float64                `json:"duration_ms"`
	FinishReason string                 `json:"finish_reason,omitempty"`
	ProviderInfo *ProviderInfo          `json:"provider_info,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
}

// CompletionOptions defines options for completion requests
type CompletionOptions struct {
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	TopP        float64                `json:"top_p,omitempty"`
	TopK        int                    `json:"top_k,omitempty"`
	Stop        []string               `json:"stop,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// CompletionResult represents the result of a completion request
type CompletionResult struct {
	Text         string                 `json:"text"`
	TokensUsed   int                    `json:"tokens_used"`
	Duration     float64                `json:"duration_ms"`
	FinishReason string                 `json:"finish_reason"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// CompletionChunk represents a chunk of streaming completion data
type CompletionChunk struct {
	Text         string                 `json:"text"`
	Delta        string                 `json:"delta"`
	Index        int                    `json:"index"`
	FinishReason string                 `json:"finish_reason,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// MessageChunk represents a chunk of streaming message data
type MessageChunk struct {
	Content      string                 `json:"content"`
	Delta        string                 `json:"delta"`
	Role         MessageRole            `json:"role,omitempty"`
	ToolCalls    []ToolCall             `json:"tool_calls,omitempty"`
	FinishReason string                 `json:"finish_reason,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// MessageResult represents the result of sending a message
type MessageResult struct {
	Response   string                 `json:"response"`
	ContextID  string                 `json:"context_id"`
	TokensUsed int                    `json:"tokens_used"`
	Duration   float64                `json:"duration_ms"`
	ToolCalls  []ToolCall             `json:"tool_calls,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// BatchMessage represents a message in a batch request
type BatchMessage struct {
	ID      string         `json:"id"`
	Request MessageRequest `json:"request"`
}

// OperationResult represents the result of an operation
type OperationResult struct {
	ID       string                 `json:"id"`
	Success  bool                   `json:"success"`
	Result   interface{}            `json:"result,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Duration float64                `json:"duration_ms"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SearchConfig interface for search configuration
type SearchConfig interface{}

// DefaultSearchConfig provides a default search configuration
func DefaultSearchConfig() SearchConfig {
	return struct{}{}
}

// FlexTime represents a flexible time that can be unmarshaled from various formats
type FlexTime struct {
	Time time.Time
}

// UnmarshalJSON implements json.Unmarshaler for FlexTime
func (ft *FlexTime) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}

	// Remove quotes from JSON string
	timeStr := string(b)
	if len(timeStr) > 1 && timeStr[0] == '"' && timeStr[len(timeStr)-1] == '"' {
		timeStr = timeStr[1 : len(timeStr)-1]
	}

	// Try multiple common time formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z0700",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			ft.Time = t
			return nil
		}
	}

	return fmt.Errorf("unable to parse time: %s", timeStr)
}

// AddMessage adds a new message to the conversation context
func (c *ConversationContext) AddMessage(message Message) {
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}
	c.Messages = append(c.Messages, message)
	c.Stats.Messages = len(c.Messages)
	c.Stats.LastUpdated = time.Now()
	c.LastAccessed = time.Now()
	c.UpdatedAt = time.Now()
}

// Clear removes all messages from the conversation context
func (c *ConversationContext) Clear() {
	c.Messages = []Message{}
	c.Stats.Messages = 0
	c.Stats.Tokens = 0
	c.Stats.LastUpdated = time.Now()
	c.LastAccessed = time.Now()
	c.UpdatedAt = time.Now()
}

// GetMessages returns the messages in the conversation context
func (c *ConversationContext) GetMessages() []Message {
	c.LastAccessed = time.Now()
	return c.Messages
}

// GetStats returns the statistics for the conversation context
func (c *ConversationContext) GetStats() ContextStats {
	c.LastAccessed = time.Now()
	return c.Stats
}

// UpdateTokenCount updates the token count for the context
func (c *ConversationContext) UpdateTokenCount(additionalTokens int) {
	c.Stats.Tokens += additionalTokens
	c.Stats.LastUpdated = time.Now()
	c.LastAccessed = time.Now()
	c.UpdatedAt = time.Now()
}

// CircuitState represents the state of a circuit breaker
type CircuitState struct {
	State                string    `json:"state"`                  // "closed", "open", "half-open"
	FailureCount         int       `json:"failure_count"`
	SuccessCount         int       `json:"success_count"`
	ConsecutiveFailures  int       `json:"consecutive_failures"`
	LastFailure          time.Time `json:"last_failure,omitempty"`
	LastSuccess          time.Time `json:"last_success,omitempty"`
	NextRetryTime        time.Time `json:"next_retry_time,omitempty"`
	FailureThreshold     int       `json:"failure_threshold"`
	RecoveryTimeout      int       `json:"recovery_timeout_seconds"`
	StateChangeTime      time.Time `json:"state_change_time"`
}

// CircuitStats represents statistics for a circuit breaker
type CircuitStats struct {
	TotalRequests      int64     `json:"total_requests"`
	SuccessfulRequests int64     `json:"successful_requests"`
	FailedRequests     int64     `json:"failed_requests"`
	AverageResponseTime float64  `json:"average_response_time_ms"`
	LastRequestTime    time.Time `json:"last_request_time"`
	Uptime             float64   `json:"uptime_percentage"`
	State              string    `json:"current_state"`
}

// FallbackStats represents statistics for fallback provider usage
type FallbackStats struct {
	TotalFallbacks     int64     `json:"total_fallbacks"`
	SuccessfulFallbacks int64    `json:"successful_fallbacks"`
	FailedFallbacks    int64     `json:"failed_fallbacks"`
	LastFallbackTime   time.Time `json:"last_fallback_time"`
	AverageFallbackDuration float64 `json:"average_fallback_duration_ms"`
	FallbackRate       float64   `json:"fallback_rate_percentage"`
}

// PoolStats represents statistics for a provider pool
type PoolStats struct {
	TotalProviders   int       `json:"total_providers"`
	ActiveProviders  int       `json:"active_providers"`
	IdleProviders    int       `json:"idle_providers"`
	FailedProviders  int       `json:"failed_providers"`
	LastPoolUpdate   time.Time `json:"last_pool_update"`
	AverageLoadTime  float64   `json:"average_load_time_ms"`
	TotalRequests    int64     `json:"total_requests"`
	QueuedRequests   int       `json:"queued_requests"`
}

// PoolHealth represents the health status of a provider pool
type PoolHealth struct {
	OverallHealth   string                    `json:"overall_health"`
	ProviderHealth  map[string]string         `json:"provider_health"`
	LastHealthCheck time.Time                 `json:"last_health_check"`
	HealthScore     float64                   `json:"health_score"`
	Issues          []string                  `json:"issues,omitempty"`
	Metadata        map[string]interface{}    `json:"metadata,omitempty"`
}
