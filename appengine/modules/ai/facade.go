package ai

import (
	"arcadia/modules/ai/models"
	"context"
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
	Version        string                 `json:"version"`
	BuildTime      string                 `json:"build_time"`
	GitCommit      string                 `json:"git_commit"`
	Features       []string               `json:"features"`
	Config         map[string]interface{} `json:"config"`
	Status         string                 `json:"status"`
	StartedAt      string                 `json:"started_at"`
	Uptime         string                 `json:"uptime"`
	Providers      []string               `json:"providers"`
	ActiveProvider string                 `json:"active_provider"`
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

// Constants for the module
const (
	ModuleVersion    = "1.0.0"
	ModuleName       = "ai"
	DefaultMaxTokens = 4096
	DefaultTimeout   = 120 // seconds
)
