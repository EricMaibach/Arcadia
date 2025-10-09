package interfaces

import (
	"context"
	"encoding/json"

	"arcadia/modules/ai/models"
)

// Core interfaces that define the internal architecture of the AI module

// ContextManager interface for managing conversation contexts
type ContextManager interface {
	// Context lifecycle
	CreateContext(ctx context.Context, contextID string) error
	GetContext(ctx context.Context, contextID string) (*models.ConversationContext, error)
	DeleteContext(ctx context.Context, contextID string) error
	ListContexts(ctx context.Context) ([]string, error)

	// Message management
	AddMessage(ctx context.Context, contextID string, message models.Message) error
	GetMessages(ctx context.Context, contextID string) ([]models.Message, error)

	// Context manipulation
	ClearContext(ctx context.Context, contextID string) error

	// Statistics and metadata
	GetContextStats(ctx context.Context, contextID string) (*models.ContextStats, error)

	// Cleanup and maintenance
	CleanupExpiredContexts(ctx context.Context) (*models.CleanupResult, error)
	GetActiveContextCount(ctx context.Context) (int, error)
}

// ToolManager interface for managing AI tools
type ToolManager interface {
	// Tool lifecycle
	RegisterTool(ctx context.Context, tool models.Tool) error
	UnregisterTool(ctx context.Context, toolName string) error
	GetTool(ctx context.Context, toolName string) (*models.Tool, error)
	ListTools(ctx context.Context) ([]models.Tool, error)

	// Tool execution
	ExecuteTool(ctx context.Context, toolName string, input any) (string, error)
	ValidateToolInput(ctx context.Context, toolName string, input any) error
	GetToolSchema(ctx context.Context, toolName string) (json.RawMessage, error)

	// Tool discovery and management
	RefreshTools(ctx context.Context) error
	LoadMCPTools(ctx context.Context) error
	LoadDynamicTools(ctx context.Context) error

	// Tool categories and filtering
	GetToolsByCategory(ctx context.Context, category string) ([]models.Tool, error)
	GetToolsByProvider(ctx context.Context, provider string) ([]models.Tool, error)
	IsToolEnabled(ctx context.Context, toolName string) (bool, error)

	// Tool metrics and analytics
	GetToolUsageStats(ctx context.Context) (*models.ToolUsageStats, error)
	GetToolMetrics(ctx context.Context, toolName string) (*models.ToolMetrics, error)
	RecordToolExecution(ctx context.Context, toolName string, duration float64, success bool) error

	// Dependency management
	UpdateDependencies(ctx context.Context, deps *Dependencies) error
}
