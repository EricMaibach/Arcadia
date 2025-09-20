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
	GetLastMessages(ctx context.Context, contextID string, limit int) ([]models.Message, error)

	// Context manipulation
	ClearContext(ctx context.Context, contextID string) error
	CompactContext(ctx context.Context, contextID string) error
	TrimContext(ctx context.Context, contextID string, maxMessages int) error

	// Statistics and metadata
	GetContextStats(ctx context.Context, contextID string) (*models.ContextStats, error)
	UpdateTokenCount(ctx context.Context, contextID string, additionalTokens int) error

	// Persistence
	SaveContext(ctx context.Context, contextID string) error
	LoadContext(ctx context.Context, contextID string) error
	ExportContext(ctx context.Context, contextID string) (*models.ContextExport, error)
	ImportContext(ctx context.Context, contextID string, export *models.ContextExport) error

	// Cleanup and maintenance
	CleanupExpiredContexts(ctx context.Context) (*models.CleanupResult, error)
	GetActiveContextCount(ctx context.Context) (int, error)
	ValidateContextIntegrity(ctx context.Context, contextID string) error
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

// ProviderManager interface for managing AI providers
type ProviderManager interface {
	// Provider lifecycle
	RegisterProvider(ctx context.Context, provider AIProvider) error
	UnregisterProvider(ctx context.Context, providerName string) error
	GetProvider(ctx context.Context, providerName string) (AIProvider, error)
	ListProviders(ctx context.Context) ([]string, error)

	// Provider selection and switching
	SetActiveProvider(ctx context.Context, providerName string) error
	GetActiveProvider(ctx context.Context) (AIProvider, error)
	GetActiveProviderName(ctx context.Context) (string, error)

	// Provider health and status
	ValidateProvider(ctx context.Context, providerName string) error
	GetProviderInfo(ctx context.Context, providerName string) (*models.ProviderInfo, error)
	GetProviderMetrics(ctx context.Context, providerName string) (*models.ProviderMetrics, error)

	// Provider configuration
	UpdateProviderConfig(ctx context.Context, providerName string, config map[string]interface{}) error
	GetProviderConfig(ctx context.Context, providerName string) (map[string]interface{}, error)
}

// ConfigurationManager interface for managing module configuration
type ConfigurationManager interface {
	// Configuration access
	GetConfig(ctx context.Context) (*models.Config, error)
	UpdateConfig(ctx context.Context, updates map[string]interface{}) error
	ReloadConfig(ctx context.Context) error
	ValidateConfig(ctx context.Context) error

	// Configuration sections
	GetProviderConfig(ctx context.Context, providerName string) (map[string]interface{}, error)
	SetProviderConfig(ctx context.Context, providerName string, config map[string]interface{}) error

	// Configuration watching
	WatchConfig(ctx context.Context, handler ConfigChangeHandler) error
	StopWatching(ctx context.Context) error
}

// ConfigChangeHandler function type for configuration change notifications
type ConfigChangeHandler func(ctx context.Context, key string, oldValue, newValue interface{}) error

// MetricsCollector interface for collecting module metrics
type MetricsCollector interface {
	// Core metrics
	RecordMessageSent(ctx context.Context, providerName string, tokenCount int, duration float64)
	RecordMessageReceived(ctx context.Context, providerName string, tokenCount int, duration float64)
	RecordToolExecution(ctx context.Context, toolName string, duration float64, success bool)
	RecordContextOperation(ctx context.Context, operation string, contextID string, duration float64)

	// Error metrics
	RecordError(ctx context.Context, errorType string, providerName string)
	RecordProviderError(ctx context.Context, providerName string, errorCode int)

	// Usage metrics
	RecordProviderUsage(ctx context.Context, providerName string, requestType string)
	RecordFeatureUsage(ctx context.Context, feature string)

	// System metrics
	RecordSystemHealth(ctx context.Context, component string, healthy bool)
	RecordPerformanceMetric(ctx context.Context, metric string, value float64)

	// Metrics retrieval
	GetMetrics(ctx context.Context) (*models.ModuleMetrics, error)
	GetProviderMetrics(ctx context.Context, providerName string) (*models.ProviderMetrics, error)
	GetToolMetrics(ctx context.Context, toolName string) (*models.ToolMetrics, error)
}

// HealthMonitor interface for monitoring module health
type HealthMonitor interface {
	// Health checks
	CheckHealth(ctx context.Context) (*models.HealthStatus, error)
	CheckProviderHealth(ctx context.Context, providerName string) (*models.HealthStatus, error)
	CheckDependencyHealth(ctx context.Context) (map[string]*models.HealthStatus, error)

	// Health monitoring
	StartHealthChecks(ctx context.Context) error
	StopHealthChecks(ctx context.Context) error
	GetHealthHistory(ctx context.Context, duration int) ([]*models.HealthStatus, error)

	// Alerting
	RegisterHealthAlert(ctx context.Context, threshold float64, handler HealthAlertHandler) error
	UnregisterHealthAlert(ctx context.Context, handler HealthAlertHandler) error
}

// HealthAlertHandler function type for health alert notifications
type HealthAlertHandler func(ctx context.Context, alert *models.HealthAlert) error

// EventPublisher interface for publishing module events
type EventPublisher interface {
	// Event publishing
	PublishEvent(ctx context.Context, event *models.Event) error
	PublishEventAsync(ctx context.Context, event *models.Event) error

	// Event types
	PublishMessageEvent(ctx context.Context, contextID string, message *models.Message) error
	PublishToolEvent(ctx context.Context, toolName string, execution *models.ToolExecution) error
	PublishProviderEvent(ctx context.Context, providerName string, event *models.ProviderEvent) error
	PublishErrorEvent(ctx context.Context, err error, context map[string]interface{}) error

	// Event management
	StartEventPublisher(ctx context.Context) error
	StopEventPublisher(ctx context.Context) error
}

// StateManager interface for managing module state
type StateManager interface {
	// State management
	GetState(ctx context.Context) (*models.ModuleState, error)
	SetState(ctx context.Context, state *models.ModuleState) error
	UpdateState(ctx context.Context, updates map[string]interface{}) error

	// State transitions
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	Pause(ctx context.Context) error
	Resume(ctx context.Context) error

	// State monitoring
	IsRunning(ctx context.Context) bool
	GetUptime(ctx context.Context) (float64, error)
	GetStartTime(ctx context.Context) (int64, error)
}

// PersistenceManager interface for data persistence
type PersistenceManager interface {
	// Context persistence
	SaveContext(ctx context.Context, contextID string, context *models.ConversationContext) error
	LoadContext(ctx context.Context, contextID string) (*models.ConversationContext, error)
	DeletePersistedContext(ctx context.Context, contextID string) error
	ListPersistedContexts(ctx context.Context) ([]string, error)

	// Configuration persistence
	SaveConfig(ctx context.Context, config *models.Config) error
	LoadConfig(ctx context.Context) (*models.Config, error)

	// Metrics persistence
	SaveMetrics(ctx context.Context, metrics *models.ModuleMetrics) error
	LoadMetrics(ctx context.Context) (*models.ModuleMetrics, error)

	// Backup and restore
	CreateBackup(ctx context.Context, path string) error
	RestoreBackup(ctx context.Context, path string) error
	ListBackups(ctx context.Context) ([]string, error)

	// Data integrity
	ValidateData(ctx context.Context) (*models.IntegrityReport, error)
	RepairData(ctx context.Context) (*models.RepairResult, error)
}

// CacheManager interface for managing cached data
type CacheManager interface {
	// Basic cache operations
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}, ttl int) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// Cache categories
	CacheContext(ctx context.Context, contextID string, context *models.ConversationContext) error
	GetCachedContext(ctx context.Context, contextID string) (*models.ConversationContext, error)
	CacheToolResult(ctx context.Context, toolName string, input any, result string) error
	GetCachedToolResult(ctx context.Context, toolName string, input any) (string, bool, error)

	// Cache management
	Clear(ctx context.Context) error
	ClearCategory(ctx context.Context, category string) error
	GetStats(ctx context.Context) (*models.CacheStats, error)
	SetTTL(ctx context.Context, key string, ttl int) error
}

// RateLimitManager interface for rate limiting
type RateLimitManager interface {
	// Rate limiting
	CheckLimit(ctx context.Context, key string) (bool, error)
	CheckLimitWithCount(ctx context.Context, key string, count int) (bool, error)
	GetRemainingLimit(ctx context.Context, key string) (int, error)
	ResetLimit(ctx context.Context, key string) error

	// Rate limit configuration
	SetLimit(ctx context.Context, key string, limit int, window int) error
	GetLimit(ctx context.Context, key string) (int, int, error)

	// Rate limit categories
	CheckProviderLimit(ctx context.Context, providerName string) (bool, error)
	CheckToolLimit(ctx context.Context, toolName string) (bool, error)
	CheckUserLimit(ctx context.Context, userID string) (bool, error)
}