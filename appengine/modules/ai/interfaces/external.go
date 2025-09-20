package interfaces

import (
	"context"
	"encoding/json"
	"sync"

	"arcadia/modules/ai/models"
)

// External dependencies that the AI module requires from the outside system

// Logger interface for structured logging
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	WithFields(fields map[string]interface{}) Logger
	WithContext(ctx context.Context) Logger
}

// Metrics interface for collecting and reporting metrics
type Metrics interface {
	// Counters
	IncrementCounter(name string, tags map[string]string)
	IncrementCounterWithValue(name string, value float64, tags map[string]string)

	// Gauges
	SetGauge(name string, value float64, tags map[string]string)
	AddToGauge(name string, value float64, tags map[string]string)

	// Histograms
	RecordDuration(name string, duration float64, tags map[string]string)
	RecordValue(name string, value float64, tags map[string]string)

	// Tags and context
	WithTags(tags map[string]string) Metrics
	WithPrefix(prefix string) Metrics
}

// Database interface for data persistence
type Database interface {
	// Basic operations
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// Batch operations
	GetBatch(ctx context.Context, keys []string) (map[string][]byte, error)
	SetBatch(ctx context.Context, items map[string][]byte) error
	DeleteBatch(ctx context.Context, keys []string) error

	// List operations
	List(ctx context.Context, prefix string) ([]string, error)
	ListWithValues(ctx context.Context, prefix string) (map[string][]byte, error)

	// Transactions
	BeginTransaction(ctx context.Context) (Transaction, error)

	// Health and maintenance
	Ping(ctx context.Context) error
	Close() error
}

// Transaction interface for database transactions
type Transaction interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Cache interface for caching frequently accessed data
type Cache interface {
	// Basic operations
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl int) error // ttl in seconds
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// Advanced operations
	GetMulti(ctx context.Context, keys []string) (map[string][]byte, error)
	SetMulti(ctx context.Context, items map[string][]byte, ttl int) error
	DeleteMulti(ctx context.Context, keys []string) error

	// Cache management
	Clear(ctx context.Context) error
	Size(ctx context.Context) (int64, error)
	Stats(ctx context.Context) (map[string]interface{}, error)

	// Health and maintenance
	Ping(ctx context.Context) error
	Close() error
}

// EventBus interface for publishing and subscribing to events
type EventBus interface {
	// Publishing
	Publish(ctx context.Context, event *models.Event) error
	PublishAsync(ctx context.Context, event *models.Event) error

	// Subscribing
	Subscribe(ctx context.Context, eventType string, handler EventHandler) error
	Unsubscribe(ctx context.Context, eventType string, handler EventHandler) error

	// Management
	Close() error
}

// EventHandler function type for handling events
type EventHandler func(ctx context.Context, event *models.Event) error

// RegistryAccess interface for accessing the application registry
type RegistryAccess interface {
	GetRegistry() map[string]any
	GetRegistryMutex() *sync.RWMutex
	GetApp(appID string) (interface{}, bool)
	ListApps() []string
}

// AppRunner interface for executing application tools
type AppRunner interface {
	ExecuteAppTool(appID, toolName string, input json.RawMessage) (string, error)
	IsAppAvailable(appID string) bool
	GetAppInfo(appID string) (interface{}, error)
}

// AppCreator interface for creating new applications
type AppCreator interface {
	CreateApp(appID, version, runtime string, tools []any, appSrc string, dependencies map[string]string) (string, error)
	UpdateApp(appID string, version string, tools []any, appSrc string, dependencies map[string]string) (string, error)
	DeleteApp(appID string) error
	ValidateApp(appID, version, runtime string, tools []any, appSrc string) error
}

// EmbeddingSearch interface for document search capabilities
type EmbeddingSearch interface {
	SearchDocuments(query string, topK int) ([]*models.DocumentSearchResult, error)
	SearchDocumentsEnhanced(query string, topK int, config models.SearchConfig) ([]*models.EnhancedDocumentSearchResult, error)
	GetDocument(documentID string) (*models.Document, error)
	ListDocuments() ([]*models.Document, error)
}

// SchedulerService interface for scheduling tasks
type SchedulerService interface {
	CreateSchedule(req models.ScheduleRequest) (*models.Schedule, error)
	GetSchedule(scheduleID string) (*models.Schedule, error)
	UpdateSchedule(scheduleID string, req models.ScheduleRequest) (*models.Schedule, error)
	DeleteSchedule(scheduleID string) error
	ListSchedules(appID string) ([]models.Schedule, error)
	GetAllSchedules(appID string) []models.Schedule
}

// FileWatcher interface for watching file system changes
type FileWatcher interface {
	Watch(ctx context.Context, paths []string, handler FileChangeHandler) error
	Stop(ctx context.Context) error
	IsWatching() bool
	GetWatchedPaths() []string
}

// FileChangeHandler function type for handling file changes
type FileChangeHandler func(ctx context.Context, change *models.FileChange) error

// ConfigManager interface for managing configuration
type ConfigManager interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}) error
	GetAll(ctx context.Context) (map[string]interface{}, error)
	Watch(ctx context.Context, key string, handler ConfigChangeHandler) error
	Validate(ctx context.Context) error
}

// Note: ConfigChangeHandler is defined in core.go to avoid duplicate declarations

// HealthChecker interface for health checking external dependencies
type HealthChecker interface {
	CheckHealth(ctx context.Context) (*models.HealthStatus, error)
	GetHealthStatus(ctx context.Context) map[string]*models.HealthStatus
}

// RateLimiter interface for rate limiting requests
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
	AllowN(ctx context.Context, key string, n int) (bool, error)
	Reset(ctx context.Context, key string) error
	GetLimit(ctx context.Context, key string) (int, error)
	GetRemaining(ctx context.Context, key string) (int, error)
}

// Tracer interface for distributed tracing
type Tracer interface {
	StartSpan(ctx context.Context, operationName string) (context.Context, Span)
	Extract(ctx context.Context, headers map[string]string) (context.Context, error)
	Inject(ctx context.Context, headers map[string]string) error
}

// Span interface for tracing spans
type Span interface {
	SetTag(key string, value interface{})
	SetError(err error)
	LogFields(fields map[string]interface{})
	Finish()
}

// SecurityManager interface for security operations
type SecurityManager interface {
	ValidateAPIKey(ctx context.Context, apiKey string) (bool, error)
	ValidateJWT(ctx context.Context, token string) (*models.Claims, error)
	EncryptData(ctx context.Context, data []byte) ([]byte, error)
	DecryptData(ctx context.Context, encryptedData []byte) ([]byte, error)
	HashPassword(password string) (string, error)
	VerifyPassword(password, hash string) bool
}

// NotificationService interface for sending notifications
type NotificationService interface {
	SendNotification(ctx context.Context, notification *models.Notification) error
	SendBulkNotifications(ctx context.Context, notifications []*models.Notification) error
	GetNotificationStatus(ctx context.Context, notificationID string) (*models.NotificationStatus, error)
}

// Dependencies holds all external dependencies for the AI module
type Dependencies struct {
	Logger              Logger
	Metrics             Metrics
	Database            Database
	Cache               Cache
	EventBus            EventBus
	RegistryAccess      RegistryAccess
	AppRunner           AppRunner
	AppCreator          AppCreator
	EmbeddingSearch     EmbeddingSearch
	SchedulerService    SchedulerService
	FileWatcher         FileWatcher
	ConfigManager       ConfigManager
	HealthChecker       HealthChecker
	RateLimiter         RateLimiter
	Tracer              Tracer
	SecurityManager     SecurityManager
	NotificationService NotificationService
}

// Validate checks that required dependencies are provided
func (d *Dependencies) Validate() error {
	// Required dependencies
	if d.Logger == nil {
		return ErrMissingLogger
	}

	// Optional dependencies (can be nil)
	// Database, Cache, EventBus, etc. are optional

	return nil
}

// Common dependency errors
var (
	ErrMissingLogger    = &models.AIError{Type: "dependency_error", Message: "logger is required", Provider: "module"}
	ErrMissingMetrics   = &models.AIError{Type: "dependency_error", Message: "metrics is required", Provider: "module"}
	ErrMissingDatabase  = &models.AIError{Type: "dependency_error", Message: "database is required", Provider: "module"}
	ErrInvalidDependency = &models.AIError{Type: "dependency_error", Message: "invalid dependency configuration", Provider: "module"}
)