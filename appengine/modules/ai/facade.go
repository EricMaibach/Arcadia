package ai

import (
	"context"
	"encoding/json"

	"arcadia/modules/ai/interfaces"
	"arcadia/modules/ai/models"
)

// AIModule defines the public API interface for the AI module
// This is the main interface that external consumers should use
type AIModule interface {
	// Version information
	Version() string
	GetInfo() *ModuleInfo

	// Core AI operations
	SendMessage(ctx context.Context, message string) (string, error)
	SendMessageWithContext(ctx context.Context, message string, contextID string) (string, error)

	// Context management
	ClearContext(ctx context.Context, contextID string) error
	GetContextStats(ctx context.Context, contextID string) (*models.ContextStats, error)
	ListContexts(ctx context.Context) ([]string, error)

	// Tool management
	RefreshTools(ctx context.Context) error
	GetTools(ctx context.Context) ([]models.Tool, error)
	ExecuteTool(ctx context.Context, toolName string, input any) (string, error)

	// Provider management
	GetProviderInfo(ctx context.Context) (*models.ProviderInfo, error)
	SwitchProvider(ctx context.Context, providerName string) error
	ListProviders(ctx context.Context) ([]string, error)

	// Conversation management
	CreateConversation(ctx context.Context, contextID string) error
	GetConversation(ctx context.Context, contextID string) (*models.ConversationContext, error)
	UpdateConversation(ctx context.Context, contextID string, message models.Message) error
	DeleteConversation(ctx context.Context, contextID string) error

	// Health and diagnostics
	HealthCheck(ctx context.Context) (*models.HealthStatus, error)
	GetMetrics(ctx context.Context) (*models.ModuleMetrics, error)
	ValidateConfiguration(ctx context.Context) error

	// Lifecycle management
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error

	// Configuration management
	UpdateConfiguration(ctx context.Context, config map[string]interface{}) error
	GetConfiguration(ctx context.Context) (map[string]interface{}, error)
}

// ModuleInfo contains information about the AI module
type ModuleInfo struct {
	Version     string                 `json:"version"`
	BuildTime   string                 `json:"build_time"`
	GitCommit   string                 `json:"git_commit"`
	Features    []string               `json:"features"`
	Config      map[string]interface{} `json:"config"`
	Status      string                 `json:"status"`
	StartedAt   string                 `json:"started_at"`
	Uptime      string                 `json:"uptime"`
	Providers   []string               `json:"providers"`
	ActiveProvider string              `json:"active_provider"`
}

// AIModuleV1 implements version 1 of the AIModule interface
// This provides a versioned implementation to support API evolution
type AIModuleV1 interface {
	AIModule

	// V1 specific methods
	GenerateCompletion(ctx context.Context, prompt string, options models.CompletionOptions) (*models.CompletionResult, error)
	StreamCompletion(ctx context.Context, prompt string, options models.CompletionOptions) (<-chan models.CompletionChunk, error)
	EstimateTokens(ctx context.Context, text string) (int, error)

	// Advanced context operations
	CompactContext(ctx context.Context, contextID string) error
	ExportContext(ctx context.Context, contextID string) (*models.ContextExport, error)
	ImportContext(ctx context.Context, contextID string, export *models.ContextExport) error

	// Tool registration
	RegisterTool(ctx context.Context, tool models.Tool) error
	UnregisterTool(ctx context.Context, toolName string) error
	GetToolSchema(ctx context.Context, toolName string) (json.RawMessage, error)

	// Experimental features (may change or be removed)
	ExperimentalFeatures() map[string]interface{}
}

// MessageOptions defines options for sending messages
type MessageOptions struct {
	ContextID      string                 `json:"context_id,omitempty"`
	MaxTokens      int                    `json:"max_tokens,omitempty"`
	Temperature    float64                `json:"temperature,omitempty"`
	TimeoutSeconds int                    `json:"timeout_seconds,omitempty"`
	ToolChoice     string                 `json:"tool_choice,omitempty"` // "auto", "none", or specific tool name
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	RetryCount     int                    `json:"retry_count,omitempty"`
}

// ConversationOptions defines options for conversation management
type ConversationOptions struct {
	MaxMessages        int                    `json:"max_messages,omitempty"`
	ContextCompaction  bool                   `json:"context_compaction,omitempty"`
	TTLMinutes         int                    `json:"ttl_minutes,omitempty"`
	PersistToDisk      bool                   `json:"persist_to_disk,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

// BulkOperationResult represents the result of a bulk operation
type BulkOperationResult struct {
	TotalRequested int                      `json:"total_requested"`
	Successful     int                      `json:"successful"`
	Failed         int                      `json:"failed"`
	Results        []models.OperationResult `json:"results"`
	Errors         []error                  `json:"errors,omitempty"`
	Duration       float64                  `json:"duration_ms"`
}

// AdvancedAIModule extends the basic interface with advanced features
type AdvancedAIModule interface {
	AIModule

	// Advanced messaging
	SendMessageWithOptions(ctx context.Context, message string, options MessageOptions) (*models.MessageResult, error)
	SendBatchMessages(ctx context.Context, messages []models.BatchMessage) (*BulkOperationResult, error)

	// Advanced conversation management
	CreateConversationWithOptions(ctx context.Context, contextID string, options ConversationOptions) error
	GetConversationHistory(ctx context.Context, contextID string, limit int, offset int) ([]*models.Message, error)
	SearchConversations(ctx context.Context, query string, limit int) ([]*models.ConversationSearchResult, error)

	// Tool analytics
	GetToolUsageStats(ctx context.Context) (*models.ToolUsageStats, error)
	GetToolPerformanceMetrics(ctx context.Context, toolName string) (*models.ToolMetrics, error)

	// Provider management
	RegisterProvider(ctx context.Context, provider interfaces.AIProvider) error
	UnregisterProvider(ctx context.Context, providerName string) error
	GetProviderMetrics(ctx context.Context, providerName string) (*models.ProviderMetrics, error)

	// Context analytics
	GetContextUsageStats(ctx context.Context) (*models.ContextUsageStats, error)
	GetContextMetrics(ctx context.Context, contextID string) (*models.ContextMetrics, error)

	// Data management
	BackupContexts(ctx context.Context, path string) error
	RestoreContexts(ctx context.Context, path string) error
	CleanupExpiredContexts(ctx context.Context) (*models.CleanupResult, error)

	// Data integrity
	ValidateContextIntegrity(ctx context.Context) (*models.IntegrityReport, error)
}

// EventListener defines the interface for listening to module events
type EventListener interface {
	OnMessageSent(ctx context.Context, contextID string, message *models.Message) error
	OnMessageReceived(ctx context.Context, contextID string, response string) error
	OnToolExecuted(ctx context.Context, toolName string, duration float64) error
	OnContextCreated(ctx context.Context, contextID string) error
	OnContextDeleted(ctx context.Context, contextID string) error
	OnProviderSwitched(ctx context.Context, oldProvider, newProvider string) error
	OnError(ctx context.Context, err error, context map[string]interface{}) error
}

// ModuleBuilder provides a fluent interface for constructing the AI module
type ModuleBuilder interface {
	// Dependencies
	WithDatabase(db interface{}) ModuleBuilder
	WithLogger(logger interface{}) ModuleBuilder
	WithMetrics(metrics interface{}) ModuleBuilder
	WithCache(cache interface{}) ModuleBuilder

	// Services
	WithRegistryAccess(registry interface{}) ModuleBuilder
	WithAppRunner(runner interface{}) ModuleBuilder
	WithAppCreator(creator interface{}) ModuleBuilder
	WithEmbeddingSearch(search interface{}) ModuleBuilder

	// Configuration
	WithConfig(config map[string]interface{}) ModuleBuilder
	WithProvider(providerName string) ModuleBuilder
	WithProviderConfig(providerName string, config map[string]interface{}) ModuleBuilder

	// Features
	EnableMCP() ModuleBuilder
	EnableMetrics() ModuleBuilder
	EnableCaching() ModuleBuilder
	EnablePersistence() ModuleBuilder
	EnableEventBus() ModuleBuilder

	// Listeners
	AddEventListener(listener EventListener) ModuleBuilder

	// Build
	Build(ctx context.Context) (AIModule, error)
	BuildAdvanced(ctx context.Context) (AdvancedAIModule, error)
}

// Factory function type for creating the module
type ModuleFactory func(ctx context.Context, config map[string]interface{}) (AIModule, error)

// Constants for the module
const (
	ModuleVersion        = "1.0.0"
	ModuleName           = "ai"
	DefaultMaxTokens     = 4096
	DefaultTimeout       = 120 // seconds
	DefaultMaxMessages   = 20
	DefaultTTLMinutes    = 60
)

// Well-known configuration keys
const (
	ConfigKeyProvider         = "provider"
	ConfigKeyProviders        = "providers"
	ConfigKeyMaxTokens        = "max_tokens"
	ConfigKeyTimeout          = "timeout_seconds"
	ConfigKeyMaxMessages      = "max_context_messages"
	ConfigKeyContextTTL       = "context_ttl_minutes"
	ConfigKeyContextCompaction = "context_compaction"
	ConfigKeyMCP              = "enable_mcp"
	ConfigKeyPersistence      = "enable_persistence"
	ConfigKeyCache            = "cache"
	ConfigKeyMetrics          = "metrics"
	ConfigKeyLogging          = "logging"
	ConfigKeyEventBus         = "event_bus"
)

// Well-known event types
const (
	EventMessageSent       = "message.sent"
	EventMessageReceived   = "message.received"
	EventToolExecuted      = "tool.executed"
	EventContextCreated    = "context.created"
	EventContextDeleted    = "context.deleted"
	EventProviderSwitched  = "provider.switched"
	EventError             = "error.occurred"
	EventModuleStarted     = "module.started"
	EventModuleStopped     = "module.stopped"
)

// Supported AI providers
const (
	ProviderOpenAI = "openai"
	ProviderClaude = "claude"
)