package documents

import (
	"context"

	"arcadia/modules/documents/models"
)

// DocumentsModule defines the public API interface for the documents module
// This is the main interface that external consumers should use
type DocumentsModule interface {
	// Version information
	Version() string
	GetInfo() *ModuleInfo

	// Core document operations
	ProcessFile(ctx context.Context, path string) (*models.ProcessResult, error)
	ProcessFiles(ctx context.Context, paths []string) ([]models.ProcessResult, error)
	DeleteDocument(ctx context.Context, docID string) error
	GetDocument(ctx context.Context, docID string) (*models.Document, error)

	// Search operations
	SearchDocuments(ctx context.Context, query string, topK int) ([]*models.DocumentSearchResult, error)
	SearchDocumentsEnhanced(ctx context.Context, query string, topK int, config models.SearchConfig) ([]*models.EnhancedDocumentSearchResult, error)

	// Batch operations
	ProcessBatch(ctx context.Context, paths []string) ([]models.ProcessResult, error)
	DeleteBatch(ctx context.Context, docIDs []string) error

	// Document management
	ListDocuments(ctx context.Context) ([]*models.Document, error)
	GetDocumentByPath(ctx context.Context, filePath string) (*models.Document, error)
	UpdateDocumentMetadata(ctx context.Context, docID string, metadata map[string]interface{}) error

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

// ModuleInfo contains information about the documents module
type ModuleInfo struct {
	Version     string                 `json:"version"`
	BuildTime   string                 `json:"build_time"`
	GitCommit   string                 `json:"git_commit"`
	Features    []string               `json:"features"`
	Config      map[string]interface{} `json:"config"`
	Status      string                 `json:"status"`
	StartedAt   string                 `json:"started_at"`
	Uptime      string                 `json:"uptime"`
}

// DocumentsModuleV1 implements version 1 of the DocumentsModule interface
// This provides a versioned implementation to support API evolution
type DocumentsModuleV1 interface {
	DocumentsModule

	// V1 specific methods
	SearchWithVector(ctx context.Context, vector []float32, topK int) ([]*models.SearchResult, error)
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	ChunkText(ctx context.Context, content string, config models.ChunkingConfig) ([]models.TextChunk, error)

	// Experimental features (may change or be removed)
	ExperimentalFeatures() map[string]interface{}
}

// ProcessingOptions defines options for document processing
type ProcessingOptions struct {
	ChunkingConfig  *models.ChunkingConfig `json:"chunking_config,omitempty"`
	SkipIfExists    bool                   `json:"skip_if_exists"`
	ForceReprocess  bool                   `json:"force_reprocess"`
	ExtractMetadata bool                   `json:"extract_metadata"`
	ValidateContent bool                   `json:"validate_content"`
	Async           bool                   `json:"async"`
	Priority        int                    `json:"priority"`
}

// SearchOptions defines advanced search options
type SearchOptions struct {
	Config          models.SearchConfig        `json:"config"`
	Filters         map[string]interface{}     `json:"filters,omitempty"`
	SortBy          string                     `json:"sort_by,omitempty"`
	SortOrder       string                     `json:"sort_order,omitempty"` // "asc" or "desc"
	IncludeContent  bool                       `json:"include_content"`
	IncludeMetadata bool                       `json:"include_metadata"`
	Timeout         int                        `json:"timeout_seconds,omitempty"`
}

// BulkOperationResult represents the result of a bulk operation
type BulkOperationResult struct {
	TotalRequested int                    `json:"total_requested"`
	Successful     int                    `json:"successful"`
	Failed         int                    `json:"failed"`
	Results        []models.ProcessResult `json:"results"`
	Errors         []error                `json:"errors,omitempty"`
	Duration       float64                `json:"duration_ms"`
}

// AdvancedDocumentsModule extends the basic interface with advanced features
type AdvancedDocumentsModule interface {
	DocumentsModule

	// Advanced processing
	ProcessFileWithOptions(ctx context.Context, path string, options ProcessingOptions) (*models.ProcessResult, error)
	ProcessFilesWithOptions(ctx context.Context, paths []string, options ProcessingOptions) (*BulkOperationResult, error)

	// Advanced search
	SearchWithOptions(ctx context.Context, query string, topK int, options SearchOptions) ([]*models.DocumentSearchResult, error)
	SearchByFilters(ctx context.Context, filters map[string]interface{}, limit int) ([]*models.Document, error)

	// Content analysis
	AnalyzeContent(ctx context.Context, content string) (*models.ContentAnalysis, error)
	ExtractKeywords(ctx context.Context, docID string) ([]string, error)
	GetSimilarDocuments(ctx context.Context, docID string, topK int) ([]*models.DocumentSearchResult, error)

	// Statistics and reporting
	GetDocumentStats(ctx context.Context) (*models.DocumentStats, error)
	GetProcessingHistory(ctx context.Context, limit int) ([]*models.ProcessingRecord, error)
	GetSearchHistory(ctx context.Context, limit int) ([]*models.SearchRecord, error)

	// File watching
	StartFileWatcher(ctx context.Context, paths []string) error
	StopFileWatcher(ctx context.Context) error
	GetWatchedPaths(ctx context.Context) ([]string, error)

	// Data management
	CompactVectorStore(ctx context.Context) error
	BackupData(ctx context.Context, path string) error
	RestoreData(ctx context.Context, path string) error

	// Data integrity
	ValidateDataIntegrity(ctx context.Context) (*models.IntegrityReport, error)
}

// EventListener defines the interface for listening to module events
type EventListener interface {
	OnDocumentProcessed(ctx context.Context, doc *models.Document) error
	OnDocumentDeleted(ctx context.Context, docID string) error
	OnSearchPerformed(ctx context.Context, query string, resultCount int) error
	OnError(ctx context.Context, err error, context map[string]interface{}) error
}

// ModuleBuilder provides a fluent interface for constructing the documents module
type ModuleBuilder interface {
	// Dependencies
	WithEmbeddingProvider(provider interface{}) ModuleBuilder
	WithDatabase(db interface{}) ModuleBuilder
	WithCache(cache interface{}) ModuleBuilder
	WithLogger(logger interface{}) ModuleBuilder
	WithMetrics(metrics interface{}) ModuleBuilder

	// Configuration
	WithConfig(config map[string]interface{}) ModuleBuilder
	WithChunkingConfig(config models.ChunkingConfig) ModuleBuilder
	WithSearchConfig(config models.SearchConfig) ModuleBuilder

	// Features
	EnableFileWatching() ModuleBuilder
	EnableMetrics() ModuleBuilder
	EnableCaching() ModuleBuilder
	EnableEventBus() ModuleBuilder

	// Listeners
	AddEventListener(listener EventListener) ModuleBuilder

	// Build
	Build(ctx context.Context) (DocumentsModule, error)
	BuildAdvanced(ctx context.Context) (AdvancedDocumentsModule, error)
}

// Factory function type for creating the module
type ModuleFactory func(ctx context.Context, config map[string]interface{}) (DocumentsModule, error)

// Constants for the module
const (
	ModuleVersion     = "1.0.0"
	ModuleName        = "documents"
	DefaultMaxWorkers = 10
	DefaultTimeout    = 30 // seconds
)

// Well-known configuration keys
const (
	ConfigKeyChunking        = "chunking"
	ConfigKeySearch          = "search"
	ConfigKeyEmbedding       = "embedding"
	ConfigKeyVectorStore     = "vector_store"
	ConfigKeyDocumentStore   = "document_store"
	ConfigKeyFileWatcher     = "file_watcher"
	ConfigKeyCache           = "cache"
	ConfigKeyMetrics         = "metrics"
	ConfigKeyLogging         = "logging"
	ConfigKeyWorkerPool      = "worker_pool"
	ConfigKeyRateLimit       = "rate_limit"
	ConfigKeyTimeouts        = "timeouts"
)

// Well-known event types
const (
	EventDocumentProcessed = "document.processed"
	EventDocumentDeleted   = "document.deleted"
	EventSearchPerformed   = "search.performed"
	EventError             = "error.occurred"
	EventModuleStarted     = "module.started"
	EventModuleStopped     = "module.stopped"
)