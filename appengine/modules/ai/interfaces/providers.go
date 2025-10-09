package interfaces

import (
	"context"

	"arcadia/modules/ai/models"
)

// AIProvider interface defines the contract that all AI providers must implement
type AIProvider interface {
	// Provider identification
	Name() string
	Type() string
	Version() string

	// Core messaging capabilities
	SendMessage(ctx context.Context, message string) (string, error)
	SendMessageWithContext(ctx context.Context, message string, contextID string) (string, error)
	SendMessageWithOptions(ctx context.Context, request *models.MessageRequest) (*models.MessageResponse, error)

	// Context management
	CreateContext(ctx context.Context, contextID string) error
	GetContext(ctx context.Context, contextID string) (*models.ConversationContext, error)
	UpdateContext(ctx context.Context, contextID string, message models.Message) error
	ClearContext(ctx context.Context, contextID string) error
	GetContextStats(ctx context.Context, contextID string) (*models.ContextStats, error)

	// Tool integration
	SetTools(ctx context.Context, tools []models.Tool) error
	GetTools(ctx context.Context) ([]models.Tool, error)

	// Configuration and validation
	Configure(ctx context.Context, config map[string]interface{}) error
	Validate(ctx context.Context) error
	GetConfiguration(ctx context.Context) (map[string]interface{}, error)

	// Provider information and capabilities
	GetProviderInfo(ctx context.Context) (*models.ProviderInfo, error)
	GetCapabilities(ctx context.Context) (*models.ProviderCapabilities, error)

	// Health and diagnostics
	HealthCheck(ctx context.Context) (*models.HealthStatus, error)
	GetMetrics(ctx context.Context) (*models.ProviderMetrics, error)

	// Lifecycle management
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
}
