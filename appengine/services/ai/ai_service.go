package ai

import (
	"fmt"
	"time"
)

// AIService defines the core interface for AI service implementations
// This interface provides a provider-agnostic way to interact with AI services
// supporting both Claude and OpenAI implementations
type AIService interface {
	// Core messaging capabilities
	SendMessage(message string) (string, error)
	SendMessageWithContext(message string, contextID string) (string, error)

	// Context management
	ClearContext(contextID string)
	GetContextStats(contextID string) (messages int, tokens int, exists bool)

	// Tool management
	RefreshTools()
	TriggerToolRefresh() // For compatibility with existing code

	// Health and introspection
	GetProviderInfo() ProviderInfo
	Validate() error
}

// ProviderInfo contains metadata about the AI provider and its capabilities
type ProviderInfo struct {
	Name     string   `json:"name"`     // Provider name (e.g., "openai")
	Model    string   `json:"model"`    // Model being used (e.g., "gpt-4")
	Features []string `json:"features"` // Supported features (e.g., "tools", "context", "vision")
}

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
	Role       MessageRole `json:"role"`
	Content    any         `json:"content"` // string or structured content
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"` // For tool response messages (OpenAI requirement)
}

// ToolCall represents a tool invocation and its result
type ToolCall struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Input  any    `json:"input"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Tool represents an available tool that the AI can use
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"`
}

// AIConfig holds configuration for AI service providers
type AIConfig struct {
	Provider         string         `json:"provider"` // "openai"
	ProviderSettings map[string]any `json:"provider_settings"`

	// Common settings
	MaxTokens          int  `json:"max_tokens"`
	TimeoutSeconds     int  `json:"timeout_seconds"`
	MaxContextMessages int  `json:"max_context_messages"`
	ContextCompaction  bool `json:"context_compaction"`
	ContextTTLMinutes  int  `json:"context_ttl_minutes"`

	// Tool configuration
	EnableMCP    bool   `json:"enable_mcp"`
	MCPServerCmd string `json:"mcp_server_cmd"`
}

// Validate validates the AI configuration and sets defaults for missing values
func (c *AIConfig) Validate() error {
	if c.Provider == "" {
		return fmt.Errorf("provider is required")
	}

	if c.Provider != "claude" && c.Provider != "openai" {
		return fmt.Errorf("unsupported provider: %s (supported: claude, openai)", c.Provider)
	}

	if c.MaxTokens <= 0 {
		c.MaxTokens = 4096 // default
	}

	if c.TimeoutSeconds <= 0 {
		c.TimeoutSeconds = 120 // default 2 minutes
	}

	if c.MaxContextMessages <= 0 {
		c.MaxContextMessages = 20 // default
	}

	if c.ContextTTLMinutes <= 0 {
		c.ContextTTLMinutes = 60 // default 1 hour
	}

	return nil
}

// GetProviderString retrieves a string value from provider-specific settings
func (c *AIConfig) GetProviderString(key string) string {
	if val, exists := c.ProviderSettings[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// GetProviderBool retrieves a boolean value from provider-specific settings
func (c *AIConfig) GetProviderBool(key string) bool {
	if val, exists := c.ProviderSettings[key]; exists {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// GetProviderInt retrieves an integer value from provider-specific settings
func (c *AIConfig) GetProviderInt(key string) int {
	if val, exists := c.ProviderSettings[key]; exists {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	return 0
}

// AIError represents an error from an AI service provider
type AIError struct {
	Type     string `json:"type"`
	Message  string `json:"message"`
	Provider string `json:"provider"`
	Code     int    `json:"code,omitempty"`
}

// Error implements the error interface
func (e *AIError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Provider, e.Type, e.Message)
}

// Common error types for AI service operations
const (
	ErrorTypeValidation = "validation_error"
	ErrorTypeAuth       = "authentication_error"
	ErrorTypeRateLimit  = "rate_limit_error"
	ErrorTypeQuota      = "quota_exceeded"
	ErrorTypeTimeout    = "timeout_error"
	ErrorTypeNetwork    = "network_error"
	ErrorTypeProvider   = "provider_error"
	ErrorTypeUnknown    = "unknown_error"
)

// NewAIError creates a new AIError with the specified parameters
func NewAIError(errorType, message, provider string, code int) *AIError {
	return &AIError{
		Type:     errorType,
		Message:  message,
		Provider: provider,
		Code:     code,
	}
}

// ContextStats represents statistics about a conversation context
type ContextStats struct {
	Messages    int       `json:"messages"`
	Tokens      int       `json:"tokens"`
	LastUpdated time.Time `json:"last_updated"`
	CreatedAt   time.Time `json:"created_at"`
}

// ConversationContext represents a conversation context with message history
type ConversationContext struct {
	ID       string       `json:"id"`
	Messages []Message    `json:"messages"`
	Stats    ContextStats `json:"stats"`
}

// AddMessage adds a new message to the conversation context
func (c *ConversationContext) AddMessage(message Message) {
	c.Messages = append(c.Messages, message)
	c.Stats.Messages = len(c.Messages)
	c.Stats.LastUpdated = time.Now()
}

// Clear removes all messages from the conversation context
func (c *ConversationContext) Clear() {
	c.Messages = []Message{}
	c.Stats.Messages = 0
	c.Stats.Tokens = 0
	c.Stats.LastUpdated = time.Now()
}

// GetMessages returns the messages in the conversation context
func (c *ConversationContext) GetMessages() []Message {
	return c.Messages
}

// GetStats returns the statistics for the conversation context
func (c *ConversationContext) GetStats() ContextStats {
	return c.Stats
}
