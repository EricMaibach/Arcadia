package interfaces

import (
	"context"
	"database/sql"

	"arcadia/pkg/logging"
)

// DatabaseProvider defines the interface for database operations
type DatabaseProvider interface {
	// Basic operations
	Execute(ctx context.Context, query string, args ...interface{}) error
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row

	// Transaction support
	Transaction(ctx context.Context, fn func(tx *sql.Tx) error) error
	Begin(ctx context.Context) (*sql.Tx, error)

	// Connection management
	Ping(ctx context.Context) error
	Close() error
	Stats() sql.DBStats
}

// QueueService defines the interface for background job queuing
type QueueService interface {
	// Job management
	Enqueue(ctx context.Context, job QueueJob) error
	EnqueueBatch(ctx context.Context, jobs []QueueJob) error
	Dequeue(ctx context.Context, queueName string) (*QueueJob, error)

	// Queue management
	CreateQueue(ctx context.Context, queueName string, config QueueConfig) error
	DeleteQueue(ctx context.Context, queueName string) error
	PurgeQueue(ctx context.Context, queueName string) error
	GetQueueStats(ctx context.Context, queueName string) (*QueueStats, error)

	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// QueueJob represents a job that can be queued
type QueueJob struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Payload  map[string]interface{} `json:"payload"`
	Priority int                    `json:"priority"`
	Retries  int                    `json:"retries"`
	MaxRetries int                  `json:"max_retries"`
}

// QueueConfig defines configuration for a queue
type QueueConfig struct {
	MaxRetries      int   `json:"max_retries"`
	RetryDelay      int   `json:"retry_delay_seconds"`
	VisibilityTimeout int `json:"visibility_timeout_seconds"`
	MaxConcurrency  int   `json:"max_concurrency"`
}

// QueueStats contains statistics about a queue
type QueueStats struct {
	QueueName        string `json:"queue_name"`
	PendingJobs      int64  `json:"pending_jobs"`
	ActiveJobs       int64  `json:"active_jobs"`
	CompletedJobs    int64  `json:"completed_jobs"`
	FailedJobs       int64  `json:"failed_jobs"`
	TotalJobs        int64  `json:"total_jobs"`
	AverageLatency   float64 `json:"average_latency_ms"`
}

// Logger type alias for backward compatibility - use pkg/logging.Logger
type Logger = logging.Logger

// MetricsCollector defines the interface for collecting metrics
type MetricsCollector interface {
	// Counters
	IncrementCounter(name string, tags map[string]string)
	AddToCounter(name string, value float64, tags map[string]string)

	// Gauges
	SetGauge(name string, value float64, tags map[string]string)

	// Histograms
	RecordHistogram(name string, value float64, tags map[string]string)

	// Timers
	StartTimer(name string, tags map[string]string) Timer
	RecordTimer(name string, duration float64, tags map[string]string)

	// Custom metrics
	RecordCustomMetric(name string, value interface{}, metricType string, tags map[string]string)
}

// Timer represents a timing measurement
type Timer interface {
	Stop() float64  // Returns duration in milliseconds
	Cancel()
}

// RateLimiter defines the interface for rate limiting
type RateLimiter interface {
	// Rate limiting
	Allow(ctx context.Context, key string) (bool, error)
	AllowN(ctx context.Context, key string, n int) (bool, error)

	// Configuration
	SetLimit(key string, limit RateLimit) error
	GetLimit(key string) (*RateLimit, error)
	RemoveLimit(key string) error

	// Statistics
	GetUsage(ctx context.Context, key string) (*RateLimitUsage, error)
}

// RateLimit defines rate limiting configuration
type RateLimit struct {
	Requests  int   `json:"requests"`   // Number of requests
	Duration  int   `json:"duration"`   // Time window in seconds
	BurstSize int   `json:"burst_size"` // Burst allowance
}

// RateLimitUsage contains rate limit usage statistics
type RateLimitUsage struct {
	Key           string  `json:"key"`
	Current       int     `json:"current"`       // Current usage
	Limit         int     `json:"limit"`         // Rate limit
	Remaining     int     `json:"remaining"`     // Remaining allowance
	ResetTime     int64   `json:"reset_time"`    // When the limit resets
	BurstUsed     int     `json:"burst_used"`    // Burst allowance used
	BurstRemaining int    `json:"burst_remaining"` // Burst allowance remaining
}

// CacheService defines the interface for caching
type CacheService interface {
	// Basic operations
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}, ttl int) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// Batch operations
	GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error)
	SetMulti(ctx context.Context, items map[string]interface{}, ttl int) error
	DeleteMulti(ctx context.Context, keys []string) error

	// Cache management
	Clear(ctx context.Context) error
	GetStats(ctx context.Context) (*CacheStats, error)

	// Advanced operations
	Increment(ctx context.Context, key string, delta int64) (int64, error)
	Decrement(ctx context.Context, key string, delta int64) (int64, error)
	SetIfNotExists(ctx context.Context, key string, value interface{}, ttl int) (bool, error)
}

// CacheStats contains cache statistics
type CacheStats struct {
	Hits        int64   `json:"hits"`
	Misses      int64   `json:"misses"`
	HitRate     float64 `json:"hit_rate"`
	Keys        int64   `json:"keys"`
	Memory      int64   `json:"memory_bytes"`
	Expired     int64   `json:"expired"`
	Evicted     int64   `json:"evicted"`
}

// ConfigProvider defines the interface for configuration management
type ConfigProvider interface {
	// Basic config access
	GetString(key string) string
	GetInt(key string) int
	GetBool(key string) bool
	GetFloat(key string) float64
	GetStringSlice(key string) []string

	// Complex config access
	GetConfig(key string) map[string]interface{}
	GetTypedConfig(key string, target interface{}) error

	// Dynamic config
	Watch(key string, callback func(interface{})) error
	Reload() error
}

// EventBus defines the interface for event publishing and subscription
type EventBus interface {
	// Publishing
	Publish(ctx context.Context, topic string, event interface{}) error
	PublishAsync(ctx context.Context, topic string, event interface{}) error

	// Subscription
	Subscribe(topic string, handler EventHandler) error
	Unsubscribe(topic string, handler EventHandler) error

	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// EventHandler defines a function type for handling events
type EventHandler func(ctx context.Context, event interface{}) error