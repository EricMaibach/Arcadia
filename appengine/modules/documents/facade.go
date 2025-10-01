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
	DeleteBatch(ctx context.Context, docIDs []string) error

	// Document management
	ListDocuments(ctx context.Context) ([]*models.Document, error)
	GetDocumentByPath(ctx context.Context, filePath string) (*models.Document, error)

	// Health and diagnostics
	HealthCheck(ctx context.Context) (*models.HealthStatus, error)
	GetMetrics(ctx context.Context) (*models.ModuleMetrics, error)

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
	Version   string                 `json:"version"`
	BuildTime string                 `json:"build_time"`
	GitCommit string                 `json:"git_commit"`
	Features  []string               `json:"features"`
	Config    map[string]interface{} `json:"config"`
	Status    string                 `json:"status"`
	StartedAt string                 `json:"started_at"`
	Uptime    string                 `json:"uptime"`
}

// EventListener defines the interface for listening to module events
type EventListener interface {
	OnDocumentProcessed(ctx context.Context, doc *models.Document) error
	OnDocumentDeleted(ctx context.Context, docID string) error
	OnSearchPerformed(ctx context.Context, query string, resultCount int) error
	OnError(ctx context.Context, err error, context map[string]interface{}) error
}

// Constants for the module
const (
	ModuleVersion     = "1.0.0"
	ModuleName        = "documents"
	DefaultMaxWorkers = 10
	DefaultTimeout    = 30 // seconds
)
