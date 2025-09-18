package documents

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"arcadia/modules/documents/core"
	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
	"arcadia/modules/documents/stores"
)

// DocumentsConfig contains configuration for the documents module
type DocumentsConfig struct {
	// Core configuration
	ChunkingConfig models.ChunkingConfig `json:"chunking"`
	SearchConfig   models.SearchConfig   `json:"search"`

	// Processing configuration
	MaxWorkers        int     `json:"max_workers"`
	ProcessingTimeout int     `json:"processing_timeout_seconds"`
	BatchSize         int     `json:"batch_size"`
	RetryAttempts     int     `json:"retry_attempts"`
	RetryDelay        int     `json:"retry_delay_seconds"`

	// Storage configuration
	VectorStoreConfig    map[string]interface{} `json:"vector_store"`
	DocumentStoreConfig  map[string]interface{} `json:"document_store"`

	// Cache configuration
	CacheEnabled    bool `json:"cache_enabled"`
	CacheTTL        int  `json:"cache_ttl_seconds"`
	CacheMaxSize    int  `json:"cache_max_size"`

	// File watching configuration
	FileWatcherEnabled bool     `json:"file_watcher_enabled"`
	WatchPaths         []string `json:"watch_paths"`
	IgnorePatterns     []string `json:"ignore_patterns"`

	// Metrics and monitoring
	MetricsEnabled  bool `json:"metrics_enabled"`
	HealthCheckInterval int `json:"health_check_interval_seconds"`

	// Rate limiting
	RateLimitEnabled   bool `json:"rate_limit_enabled"`
	RateLimitRequests  int  `json:"rate_limit_requests"`
	RateLimitDuration  int  `json:"rate_limit_duration_seconds"`

	// Feature flags
	EnableAdvancedSearch   bool `json:"enable_advanced_search"`
	EnableContentAnalysis  bool `json:"enable_content_analysis"`
	EnableEventBus         bool `json:"enable_event_bus"`
}

// Dependencies contains all external dependencies for the documents module
type Dependencies struct {
	// Required dependencies
	DB                interfaces.DatabaseProvider   `json:"-"`
	EmbeddingProvider interfaces.EmbeddingProvider  `json:"-"`
	Logger            interfaces.Logger             `json:"-"`

	// Optional dependencies with defaults
	Queue             interfaces.QueueService       `json:"-"`
	Metrics           interfaces.MetricsCollector   `json:"-"`
	Cache             interfaces.CacheService       `json:"-"`
	RateLimiter       interfaces.RateLimiter        `json:"-"`
	ConfigProvider    interfaces.ConfigProvider     `json:"-"`
	EventBus          interfaces.EventBus           `json:"-"`

	// Configuration
	Config            DocumentsConfig               `json:"config"`
}

// documentsModule implements the DocumentsModule interface
type documentsModule struct {
	version   string
	buildInfo *ModuleInfo
	deps      Dependencies

	// Internal components
	vectorStore     interfaces.VectorStoreInterface
	documentStore   interfaces.DocumentStoreInterface
	processor       interfaces.DocumentProcessor
	searchEngine    interfaces.SearchEngine
	chunker         interfaces.TextChunkerInterface
	contentAnalyzer interfaces.ContentAnalyzer
	workerPool      interfaces.WorkerPool
	fileWatcher     interfaces.FileWatcher
	eventListeners  []EventListener

	// State management
	ctx           context.Context
	cancel        context.CancelFunc
	started       bool
	startedAt     time.Time
	healthStatus  *models.HealthStatus
	metrics       *models.ModuleMetrics

	// Synchronization
	mutex         sync.RWMutex
	stateMutex    sync.Mutex
}

// NewDocumentsModule creates a new documents module instance
func NewDocumentsModule(deps Dependencies) (DocumentsModule, error) {
	if err := validateDependencies(deps); err != nil {
		return nil, fmt.Errorf("invalid dependencies: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	module := &documentsModule{
		version:   ModuleVersion,
		deps:      deps,
		ctx:       ctx,
		cancel:    cancel,
		started:   false,
		buildInfo: &ModuleInfo{
			Version:   ModuleVersion,
			BuildTime: time.Now().Format(time.RFC3339),
			Status:    "initialized",
		},
		healthStatus: &models.HealthStatus{
			Status:    "initializing",
			Timestamp: time.Now(),
			Components: make(map[string]interface{}),
		},
		metrics: &models.ModuleMetrics{
			ProcessingLatency: make(map[string]float64),
			CustomMetrics:     make(map[string]interface{}),
		},
		eventListeners: make([]EventListener, 0),
	}

	// Initialize internal components
	if err := module.initializeComponents(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}

	return module, nil
}

// validateDependencies validates that all required dependencies are provided
func validateDependencies(deps Dependencies) error {
	if deps.DB == nil {
		return models.NewDocumentError(models.ErrDependencyMissing, "database provider is required")
	}
	if deps.EmbeddingProvider == nil {
		return models.NewDocumentError(models.ErrDependencyMissing, "embedding provider is required")
	}
	if deps.Logger == nil {
		return models.NewDocumentError(models.ErrDependencyMissing, "logger is required")
	}
	return nil
}

// initializeComponents initializes all internal components
func (dm *documentsModule) initializeComponents() error {
	// Initialize stores
	if err := dm.initializeStores(); err != nil {
		return fmt.Errorf("failed to initialize stores: %w", err)
	}

	// Initialize core components
	if err := dm.initializeCoreComponents(); err != nil {
		return fmt.Errorf("failed to initialize core components: %w", err)
	}

	// Initialize optional components
	if err := dm.initializeOptionalComponents(); err != nil {
		return fmt.Errorf("failed to initialize optional components: %w", err)
	}

	return nil
}

// initializeStores initializes vector and document stores
func (dm *documentsModule) initializeStores() error {
	// Initialize vector store
	vectorStoreConfig := stores.VectorStoreConfig{
		Type:      "memory", // Default to memory store for now
		Dimension: 384,      // Default for all-MiniLM-L6-v2
		Metric:    "cosine",
		BatchSize: 100,
		Timeout:   30,
	}

	// Override with custom config if provided
	if dm.deps.Config.VectorStoreConfig != nil {
		if storeType, ok := dm.deps.Config.VectorStoreConfig["type"].(string); ok {
			vectorStoreConfig.Type = storeType
		}
		if dimension, ok := dm.deps.Config.VectorStoreConfig["dimension"].(int); ok {
			vectorStoreConfig.Dimension = dimension
		}
	}

	vectorStoreFactory := stores.NewVectorStoreFactory()
	if dm.deps.Logger != nil {
		vectorStoreFactory.WithLogger(dm.deps.Logger)
	}
	if dm.deps.Metrics != nil {
		vectorStoreFactory.WithMetrics(dm.deps.Metrics)
	}

	var err error
	dm.vectorStore, err = vectorStoreFactory.CreateVectorStore(vectorStoreConfig)
	if err != nil {
		return fmt.Errorf("failed to create vector store: %w", err)
	}

	// Initialize document store
	documentStoreConfig := stores.DocumentStoreConfig{
		Type:      "memory", // Default to memory store for now
		TableName: "documents",
		BatchSize: 100,
		Timeout:   30,
	}

	// Override with custom config if provided
	if dm.deps.Config.DocumentStoreConfig != nil {
		if storeType, ok := dm.deps.Config.DocumentStoreConfig["type"].(string); ok {
			documentStoreConfig.Type = storeType
		}
		if tableName, ok := dm.deps.Config.DocumentStoreConfig["table_name"].(string); ok {
			documentStoreConfig.TableName = tableName
		}
	}

	// Use SQL store if database is available AND type is not explicitly configured
	// Respect explicit configuration over automatic detection
	if dm.deps.DB != nil {
		// Only use SQL store if type wasn't explicitly set or was set to sql
		if dm.deps.Config.DocumentStoreConfig == nil {
			// No explicit config, default to SQL
			documentStoreConfig.Type = "sql"
		} else {
			// Check if type was explicitly configured
			if _, hasType := dm.deps.Config.DocumentStoreConfig["type"]; !hasType {
				// Type not explicitly set, default to SQL when database is available
				documentStoreConfig.Type = "sql"
			}
			// If type was explicitly set, respect that configuration
		}
	}

	documentStoreFactory := stores.NewDocumentStoreFactory(dm.deps.DB)
	if dm.deps.Logger != nil {
		documentStoreFactory.WithLogger(dm.deps.Logger)
	}
	if dm.deps.Metrics != nil {
		documentStoreFactory.WithMetrics(dm.deps.Metrics)
	}

	dm.documentStore, err = documentStoreFactory.CreateDocumentStore(documentStoreConfig)
	if err != nil {
		return fmt.Errorf("failed to create document store: %w", err)
	}

	// Initialize document store if it's SQL-based
	if sqlStore, ok := dm.documentStore.(*stores.SQLDocumentStore); ok {
		if err := sqlStore.Initialize(dm.ctx); err != nil {
			return fmt.Errorf("failed to initialize SQL document store: %w", err)
		}
	}

	return nil
}

// initializeCoreComponents initializes core processing components
func (dm *documentsModule) initializeCoreComponents() error {
	// Initialize text chunker
	dm.chunker = core.NewSimpleTextChunkerWithConfig(dm.deps.Config.ChunkingConfig)

	// Initialize embedding engine
	embeddingConfig := core.DefaultEmbeddingConfig()
	embeddingConfig.ModelName = dm.deps.EmbeddingProvider.GetModelName()
	embeddingConfig.Dimension = dm.deps.EmbeddingProvider.GetDimension()

	embeddingEngine := core.NewEmbeddingEngine(dm.deps.EmbeddingProvider, embeddingConfig)
	if dm.deps.Cache != nil {
		embeddingEngine.WithCache(dm.deps.Cache)
	}
	if dm.deps.Logger != nil {
		embeddingEngine.WithLogger(dm.deps.Logger)
	}
	if dm.deps.Metrics != nil {
		embeddingEngine.WithMetrics(dm.deps.Metrics)
	}

	// Initialize document processor
	processorConfig := core.DefaultProcessorConfig()
	processorConfig.ChunkingConfig = dm.deps.Config.ChunkingConfig

	dm.processor = core.NewDocumentProcessor(
		dm.chunker,
		embeddingEngine,
		dm.vectorStore,
		dm.documentStore,
		processorConfig,
	)

	// Add logger and metrics to processor if available
	if concreteProcessor, ok := dm.processor.(*core.DocumentProcessor); ok {
		if dm.deps.Logger != nil {
			concreteProcessor.WithLogger(dm.deps.Logger)
		}
		if dm.deps.Metrics != nil {
			concreteProcessor.WithMetrics(dm.deps.Metrics)
		}
	}

	// Initialize search engine
	dm.searchEngine = core.NewSearchEngine(
		dm.vectorStore,
		dm.documentStore,
		embeddingEngine,
	)
	// Add logger and metrics to search engine if available
	if concreteEngine, ok := dm.searchEngine.(*core.SearchEngine); ok {
		if dm.deps.Logger != nil {
			concreteEngine.WithLogger(dm.deps.Logger)
		}
		if dm.deps.Metrics != nil {
			concreteEngine.WithMetrics(dm.deps.Metrics)
		}
	}

	return nil
}

// initializeOptionalComponents initializes optional components based on configuration
func (dm *documentsModule) initializeOptionalComponents() error {
	// Initialize worker pool if configured
	if dm.deps.Config.MaxWorkers > 0 {
		queueSize := dm.deps.Config.BatchSize
		if queueSize <= 0 {
			queueSize = 100 // Default queue size
		}

		dm.workerPool = core.NewSimpleWorkerPool(dm.deps.Config.MaxWorkers, queueSize)
		// Add logger and metrics to worker pool if available
		if concretePool, ok := dm.workerPool.(*core.SimpleWorkerPool); ok {
			if dm.deps.Logger != nil {
				concretePool.WithLogger(dm.deps.Logger)
			}
			if dm.deps.Metrics != nil {
				concretePool.WithMetrics(dm.deps.Metrics)
			}
		}
	}

	// Initialize content analyzer if enabled
	if dm.deps.Config.EnableContentAnalysis && dm.contentAnalyzer == nil {
		// Use a simple content analyzer implementation
		dm.contentAnalyzer = core.NewSimpleContentAnalyzer()
		// Add logger to content analyzer if available
		if concreteAnalyzer, ok := dm.contentAnalyzer.(*core.SimpleContentAnalyzer); ok {
			if dm.deps.Logger != nil {
				concreteAnalyzer.WithLogger(dm.deps.Logger)
			}
		}
	}

	// File watcher initialization would go here if implemented
	// For now, we'll leave it as nil since it's an advanced feature

	return nil
}

// Version returns the module version
func (dm *documentsModule) Version() string {
	return dm.version
}

// GetInfo returns module information
func (dm *documentsModule) GetInfo() *ModuleInfo {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	info := *dm.buildInfo
	if dm.started {
		info.Uptime = time.Since(dm.startedAt).String()
	}
	return &info
}

// Start starts the documents module
func (dm *documentsModule) Start(ctx context.Context) error {
	dm.stateMutex.Lock()
	defer dm.stateMutex.Unlock()

	if dm.started {
		return models.NewDocumentError(models.ErrModuleNotInitialized, "module already started")
	}

	// Initialize embedding provider
	if err := dm.deps.EmbeddingProvider.Initialize(ctx); err != nil {
		return models.NewDocumentErrorWithCause(models.ErrDependencyMissing, "failed to initialize embedding provider", err)
	}

	// Start worker pool if configured
	if dm.workerPool != nil {
		if err := dm.workerPool.Start(ctx); err != nil {
			return models.NewDocumentErrorWithCause(models.ErrModuleNotInitialized, "failed to start worker pool", err)
		}
	}

	// Start file watcher if configured
	if dm.fileWatcher != nil && dm.deps.Config.FileWatcherEnabled {
		if err := dm.fileWatcher.Start(ctx); err != nil {
			return models.NewDocumentErrorWithCause(models.ErrModuleNotInitialized, "failed to start file watcher", err)
		}
	}

	// Start event bus if configured
	if dm.deps.EventBus != nil && dm.deps.Config.EnableEventBus {
		if err := dm.deps.EventBus.Start(ctx); err != nil {
			return models.NewDocumentErrorWithCause(models.ErrModuleNotInitialized, "failed to start event bus", err)
		}
	}

	dm.started = true
	dm.startedAt = time.Now()
	dm.buildInfo.Status = "running"
	dm.buildInfo.StartedAt = dm.startedAt.Format(time.RFC3339)

	dm.healthStatus.Status = "healthy"
	dm.healthStatus.Timestamp = time.Now()

	dm.deps.Logger.Info(ctx, "Documents module started successfully")

	return nil
}

// Stop stops the documents module
func (dm *documentsModule) Stop(ctx context.Context) error {
	dm.stateMutex.Lock()
	defer dm.stateMutex.Unlock()

	if !dm.started {
		return nil
	}

	// Stop components in reverse order
	if dm.deps.EventBus != nil {
		dm.deps.EventBus.Stop(ctx)
	}

	if dm.fileWatcher != nil {
		dm.fileWatcher.Stop(ctx)
	}

	if dm.workerPool != nil {
		dm.workerPool.Stop(ctx)
	}

	if dm.deps.EmbeddingProvider != nil {
		dm.deps.EmbeddingProvider.Close()
	}

	dm.cancel()
	dm.started = false
	dm.buildInfo.Status = "stopped"

	dm.healthStatus.Status = "stopped"
	dm.healthStatus.Timestamp = time.Now()

	dm.deps.Logger.Info(ctx, "Documents module stopped successfully")

	return nil
}

// Restart restarts the documents module
func (dm *documentsModule) Restart(ctx context.Context) error {
	if err := dm.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop module: %w", err)
	}

	// Create new context
	dm.ctx, dm.cancel = context.WithCancel(context.Background())

	return dm.Start(ctx)
}

// ProcessFile processes a single file
func (dm *documentsModule) ProcessFile(ctx context.Context, path string) (*models.ProcessResult, error) {
	if !dm.started {
		return nil, models.NewDocumentError(models.ErrModuleNotInitialized, "module not started")
	}

	if dm.processor == nil {
		return nil, models.NewDocumentError(models.ErrModuleNotInitialized, "document processor not initialized")
	}

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Seconds() * 1000
		if dm.deps.Metrics != nil {
			dm.deps.Metrics.RecordTimer("document.process.duration", duration, map[string]string{"operation": "single_file"})
		}
	}()


	// Process the file using the document processor
	doc, err := dm.processor.ProcessFile(ctx, path)
	if err != nil {
		return &models.ProcessResult{
			FilePath:    path,
			Success:     false,
			Error:       err,
			ProcessedAt: time.Now(),
		}, nil
	}

	return &models.ProcessResult{
		DocumentID:  doc.ID,
		FilePath:    path,
		Success:     true,
		Document:    doc,
		ProcessedAt: time.Now(),
	}, nil
}

// ProcessFiles processes multiple files
func (dm *documentsModule) ProcessFiles(ctx context.Context, paths []string) ([]models.ProcessResult, error) {
	if !dm.started {
		return nil, models.NewDocumentError(models.ErrModuleNotInitialized, "module not started")
	}

	results := make([]models.ProcessResult, len(paths))

	// TODO: Implement batch processing
	for i, path := range paths {
		result, err := dm.ProcessFile(ctx, path)
		if result != nil {
			results[i] = *result
		} else {
			results[i] = models.ProcessResult{
				FilePath:    path,
				Success:     false,
				Error:       err,
				ProcessedAt: time.Now(),
			}
		}
	}

	return results, nil
}

// ProcessBatch processes files in a batch operation
func (dm *documentsModule) ProcessBatch(ctx context.Context, paths []string) ([]models.ProcessResult, error) {
	return dm.ProcessFiles(ctx, paths)
}

// SearchDocuments searches for documents
func (dm *documentsModule) SearchDocuments(ctx context.Context, query string, topK int) ([]*models.DocumentSearchResult, error) {
	if !dm.started {
		return nil, models.NewDocumentError(models.ErrModuleNotInitialized, "module not started")
	}

	if dm.searchEngine == nil {
		return nil, models.NewDocumentError(models.ErrSearchFailed, "search engine not initialized")
	}

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Seconds() * 1000
		if dm.deps.Metrics != nil {
			dm.deps.Metrics.RecordTimer("document.search.duration", duration, map[string]string{"operation": "search"})
		}
	}()

	return dm.searchEngine.SearchDocuments(ctx, query, topK)
}

// SearchDocumentsEnhanced performs enhanced document search
func (dm *documentsModule) SearchDocumentsEnhanced(ctx context.Context, query string, topK int, config models.SearchConfig) ([]*models.EnhancedDocumentSearchResult, error) {
	if !dm.started {
		return nil, models.NewDocumentError(models.ErrModuleNotInitialized, "module not started")
	}

	if dm.searchEngine == nil {
		return nil, models.NewDocumentError(models.ErrSearchFailed, "search engine not initialized")
	}

	return dm.searchEngine.SearchDocumentsEnhanced(ctx, query, topK, config)
}

// DeleteDocument deletes a document
func (dm *documentsModule) DeleteDocument(ctx context.Context, docID string) error {
	if !dm.started {
		return models.NewDocumentError(models.ErrModuleNotInitialized, "module not started")
	}

	if docID == "" {
		return models.NewDocumentError(models.ErrInvalidInput, "document ID cannot be empty")
	}

	if dm.documentStore == nil {
		return models.NewDocumentError(models.ErrStorageFailed, "document store not initialized")
	}

	if dm.vectorStore == nil {
		return models.NewDocumentError(models.ErrStorageFailed, "vector store not initialized")
	}

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Seconds() * 1000
		if dm.deps.Metrics != nil {
			dm.deps.Metrics.RecordTimer("document.delete.duration", duration, map[string]string{"operation": "delete"})
		}
	}()

	// Get document to check if it exists and get metadata
	doc, err := dm.documentStore.GetDocument(ctx, docID)
	if err != nil {
		if dm.deps.Logger != nil {
			dm.deps.Logger.Warn(ctx, "Failed to get document for deletion", "docID", docID, "error", err)
		}
		return err
	}

	// Delete from vector store first
	if err := dm.vectorStore.DeleteDocumentVectors(ctx, docID); err != nil {
		if dm.deps.Logger != nil {
			dm.deps.Logger.Warn(ctx, "Failed to delete document vectors from vector store", "docID", docID, "error", err)
		}
		// Continue with document store deletion even if vector store fails
	}

	// Delete from document store
	if err := dm.documentStore.DeleteDocument(ctx, docID); err != nil {
		if dm.deps.Logger != nil {
			dm.deps.Logger.Error(ctx, "Failed to delete document from document store", "docID", docID, "error", err)
		}
		return models.NewDocumentErrorWithCause(models.ErrDocumentDeleteFailed, "failed to delete document", err)
	}

	if dm.deps.Logger != nil {
		dm.deps.Logger.Info(ctx, "Document deleted successfully", "docID", docID, "filePath", doc.FilePath)
	}

	if dm.deps.Metrics != nil {
		dm.deps.Metrics.IncrementCounter("document.delete.success", map[string]string{"operation": "delete"})
	}

	return nil
}

// GetDocument retrieves a document by ID
func (dm *documentsModule) GetDocument(ctx context.Context, docID string) (*models.Document, error) {
	if !dm.started {
		return nil, models.NewDocumentError(models.ErrModuleNotInitialized, "module not started")
	}

	if dm.documentStore == nil {
		return nil, models.NewDocumentError(models.ErrStorageFailed, "document store not initialized")
	}

	return dm.documentStore.GetDocument(ctx, docID)
}

// DeleteBatch deletes multiple documents
func (dm *documentsModule) DeleteBatch(ctx context.Context, docIDs []string) error {
	for _, docID := range docIDs {
		if err := dm.DeleteDocument(ctx, docID); err != nil {
			return err
		}
	}
	return nil
}

// ListDocuments lists all documents
func (dm *documentsModule) ListDocuments(ctx context.Context) ([]*models.Document, error) {
	if !dm.started {
		return nil, models.NewDocumentError(models.ErrModuleNotInitialized, "module not started")
	}

	if dm.documentStore == nil {
		return nil, models.NewDocumentError(models.ErrStorageFailed, "document store not initialized")
	}

	return dm.documentStore.ListDocuments(ctx)
}

// GetDocumentByPath retrieves a document by file path
func (dm *documentsModule) GetDocumentByPath(ctx context.Context, filePath string) (*models.Document, error) {
	if !dm.started {
		return nil, models.NewDocumentError(models.ErrModuleNotInitialized, "module not started")
	}

	if dm.documentStore == nil {
		return nil, models.NewDocumentError(models.ErrStorageFailed, "document store not initialized")
	}

	return dm.documentStore.GetDocumentByPath(ctx, filePath)
}

// UpdateDocumentMetadata updates document metadata
func (dm *documentsModule) UpdateDocumentMetadata(ctx context.Context, docID string, metadata map[string]interface{}) error {
	if !dm.started {
		return models.NewDocumentError(models.ErrModuleNotInitialized, "module not started")
	}

	if docID == "" {
		return models.NewDocumentError(models.ErrInvalidInput, "document ID cannot be empty")
	}

	if metadata == nil {
		return models.NewDocumentError(models.ErrInvalidInput, "metadata cannot be nil")
	}

	if dm.documentStore == nil {
		return models.NewDocumentError(models.ErrStorageFailed, "document store not initialized")
	}

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Seconds() * 1000
		if dm.deps.Metrics != nil {
			dm.deps.Metrics.RecordTimer("document.update_metadata.duration", duration, map[string]string{"operation": "update_metadata"})
		}
	}()

	// Get existing document
	doc, err := dm.documentStore.GetDocument(ctx, docID)
	if err != nil {
		if dm.deps.Logger != nil {
			dm.deps.Logger.Warn(ctx, "Failed to get document for metadata update", "docID", docID, "error", err)
		}
		return err
	}

	// Merge new metadata with existing metadata
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]interface{})
	}

	// Track what changes were made for logging
	var changedKeys []string
	for key, value := range metadata {
		// Validate metadata key and value
		if key == "" {
			if dm.deps.Logger != nil {
				dm.deps.Logger.Warn(ctx, "Skipping empty metadata key", "docID", docID)
			}
			continue
		}

		// Prevent updating system-controlled metadata
		if dm.isSystemMetadataKey(key) {
			if dm.deps.Logger != nil {
				dm.deps.Logger.Warn(ctx, "Skipping system metadata key", "docID", docID, "key", key)
			}
			continue
		}

		// Update the metadata
		oldValue := doc.Metadata[key]
		doc.Metadata[key] = value

		if oldValue != value {
			changedKeys = append(changedKeys, key)
		}
	}

	if len(changedKeys) == 0 {
		if dm.deps.Logger != nil {
			dm.deps.Logger.Info(ctx, "No metadata changes detected", "docID", docID)
		}
		return nil
	}

	// Update the document with new metadata
	doc.UpdatedAt = time.Now()
	if err := dm.documentStore.UpdateDocument(ctx, doc); err != nil {
		if dm.deps.Logger != nil {
			dm.deps.Logger.Error(ctx, "Failed to update document metadata", "docID", docID, "error", err)
		}
		return models.NewDocumentErrorWithCause(models.ErrStorageFailed, "failed to update document metadata", err)
	}

	if dm.deps.Logger != nil {
		dm.deps.Logger.Info(ctx, "Document metadata updated successfully",
			"docID", docID,
			"changedKeys", changedKeys,
			"totalKeys", len(doc.Metadata))
	}

	if dm.deps.Metrics != nil {
		dm.deps.Metrics.IncrementCounter("document.update_metadata.success", map[string]string{"operation": "update_metadata"})
	}

	return nil
}

// HealthCheck performs a health check
func (dm *documentsModule) HealthCheck(ctx context.Context) (*models.HealthStatus, error) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	status := *dm.healthStatus
	status.Timestamp = time.Now()

	// Check component health
	components := make(map[string]interface{})

	// Check embedding provider
	if dm.deps.EmbeddingProvider != nil {
		if err := dm.deps.EmbeddingProvider.HealthCheck(ctx); err != nil {
			components["embedding_provider"] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
			status.Status = "degraded"
		} else {
			components["embedding_provider"] = map[string]interface{}{"status": "healthy"}
		}
	}

	// Check database
	if dm.deps.DB != nil {
		if err := dm.deps.DB.Ping(ctx); err != nil {
			components["database"] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
			status.Status = "unhealthy"
		} else {
			components["database"] = map[string]interface{}{"status": "healthy"}
		}
	}

	status.Components = components
	return &status, nil
}

// GetMetrics returns module metrics
func (dm *documentsModule) GetMetrics(ctx context.Context) (*models.ModuleMetrics, error) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	metrics := *dm.metrics
	metrics.LastProcessedAt = &time.Time{}
	*metrics.LastProcessedAt = time.Now()

	return &metrics, nil
}

// ValidateConfiguration validates the module configuration
func (dm *documentsModule) ValidateConfiguration(ctx context.Context) error {
	config := dm.deps.Config

	// Validate core configuration
	if config.MaxWorkers < 0 {
		return models.NewDocumentError(models.ErrInvalidConfig, "max_workers must be non-negative")
	}

	if config.ProcessingTimeout <= 0 {
		return models.NewDocumentError(models.ErrInvalidConfig, "processing_timeout_seconds must be positive")
	}

	if config.BatchSize <= 0 {
		return models.NewDocumentError(models.ErrInvalidConfig, "batch_size must be positive")
	}

	if config.RetryAttempts < 0 {
		return models.NewDocumentError(models.ErrInvalidConfig, "retry_attempts must be non-negative")
	}

	if config.RetryDelay < 0 {
		return models.NewDocumentError(models.ErrInvalidConfig, "retry_delay_seconds must be non-negative")
	}

	// Validate chunking configuration
	if err := dm.validateChunkingConfig(config.ChunkingConfig); err != nil {
		return err
	}

	// Validate search configuration
	if err := dm.validateSearchConfig(config.SearchConfig); err != nil {
		return err
	}

	// Validate cache configuration
	if config.CacheEnabled {
		if config.CacheTTL <= 0 {
			return models.NewDocumentError(models.ErrInvalidConfig, "cache_ttl_seconds must be positive when cache is enabled")
		}
		if config.CacheMaxSize <= 0 {
			return models.NewDocumentError(models.ErrInvalidConfig, "cache_max_size must be positive when cache is enabled")
		}
	}

	// Validate file watcher configuration
	if config.FileWatcherEnabled {
		if len(config.WatchPaths) == 0 {
			return models.NewDocumentError(models.ErrInvalidConfig, "watch_paths must not be empty when file_watcher_enabled is true")
		}
		// Validate watch paths exist and are accessible
		for _, path := range config.WatchPaths {
			if path == "" {
				return models.NewDocumentError(models.ErrInvalidConfig, "watch paths cannot be empty")
			}
			// Check path security
			cleanPath := filepath.Clean(path)
			absPath, err := filepath.Abs(cleanPath)
			if err != nil {
				return models.NewDocumentErrorWithCause(models.ErrInvalidConfig, fmt.Sprintf("invalid watch path: %s", path), err)
			}
			// Basic path validation (not using isRestrictedPath as watch paths might be more permissive)
			if strings.Contains(cleanPath, "..") {
				return models.NewDocumentError(models.ErrInvalidConfig, fmt.Sprintf("invalid watch path contains directory traversal: %s", path))
			}
			_ = absPath // Use absPath to avoid unused variable warning
		}
	}

	// Validate health check configuration
	if config.HealthCheckInterval <= 0 {
		return models.NewDocumentError(models.ErrInvalidConfig, "health_check_interval_seconds must be positive")
	}

	// Validate rate limiting configuration
	if config.RateLimitEnabled {
		if config.RateLimitRequests <= 0 {
			return models.NewDocumentError(models.ErrInvalidConfig, "rate_limit_requests must be positive when rate limiting is enabled")
		}
		if config.RateLimitDuration <= 0 {
			return models.NewDocumentError(models.ErrInvalidConfig, "rate_limit_duration_seconds must be positive when rate limiting is enabled")
		}
	}

	// Validate vector store configuration
	if config.VectorStoreConfig != nil {
		if err := dm.validateVectorStoreConfig(config.VectorStoreConfig); err != nil {
			return err
		}
	}

	// Validate document store configuration
	if config.DocumentStoreConfig != nil {
		if err := dm.validateDocumentStoreConfig(config.DocumentStoreConfig); err != nil {
			return err
		}
	}

	if dm.deps.Logger != nil {
		dm.deps.Logger.Info(ctx, "Configuration validation completed successfully")
	}

	return nil
}

// UpdateConfiguration updates the module configuration
func (dm *documentsModule) UpdateConfiguration(ctx context.Context, config map[string]interface{}) error {
	// TODO: Implement configuration updates
	return models.NewDocumentError(models.ErrInvalidConfig, "not implemented")
}

// GetConfiguration returns the current configuration
func (dm *documentsModule) GetConfiguration(ctx context.Context) (map[string]interface{}, error) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	// Convert config to map
	configMap := make(map[string]interface{})
	// TODO: Implement configuration serialization

	return configMap, nil
}

// isSystemMetadataKey checks if a metadata key is system-controlled and should not be updated
func (dm *documentsModule) isSystemMetadataKey(key string) bool {
	systemKeys := []string{
		"file_ext",
		"size",
		"modified",
		"created_at",
		"updated_at",
		"hash",
		"chunk_count",
		"processing_version",
		"embedding_model",
		"content_type",
		"file_name",
		"file_dir",
	}

	for _, systemKey := range systemKeys {
		if key == systemKey {
			return true
		}
	}

	return false
}

// validateChunkingConfig validates chunking configuration
func (dm *documentsModule) validateChunkingConfig(config models.ChunkingConfig) error {
	if config.MaxChunkSize <= 0 {
		return models.NewDocumentError(models.ErrInvalidConfig, "max_chunk_size must be positive")
	}
	if config.ChunkOverlap < 0 {
		return models.NewDocumentError(models.ErrInvalidConfig, "chunk_overlap must be non-negative")
	}
	if config.ChunkOverlap >= config.MaxChunkSize {
		return models.NewDocumentError(models.ErrInvalidConfig, "chunk_overlap must be less than max_chunk_size")
	}
	return nil
}

// validateSearchConfig validates search configuration
func (dm *documentsModule) validateSearchConfig(config models.SearchConfig) error {
	if config.MaxDocumentSize <= 0 {
		return models.NewDocumentError(models.ErrInvalidConfig, "max_document_size must be positive")
	}
	if config.MaxHighlights < 0 {
		return models.NewDocumentError(models.ErrInvalidConfig, "max_highlights must be non-negative")
	}
	return nil
}

// validateVectorStoreConfig validates vector store configuration
func (dm *documentsModule) validateVectorStoreConfig(config map[string]interface{}) error {
	if storeType, ok := config["type"].(string); ok {
		validTypes := []string{"memory", "qdrant"}
		isValid := false
		for _, validType := range validTypes {
			if storeType == validType {
				isValid = true
				break
			}
		}
		if !isValid {
			return models.NewDocumentError(models.ErrInvalidConfig, fmt.Sprintf("unsupported vector store type: %s", storeType))
		}
	}

	if dimension, ok := config["dimension"].(int); ok {
		if dimension <= 0 {
			return models.NewDocumentError(models.ErrInvalidConfig, "vector dimension must be positive")
		}
	}

	if batchSize, ok := config["batch_size"].(int); ok {
		if batchSize <= 0 {
			return models.NewDocumentError(models.ErrInvalidConfig, "vector store batch_size must be positive")
		}
	}

	if timeout, ok := config["timeout"].(int); ok {
		if timeout <= 0 {
			return models.NewDocumentError(models.ErrInvalidConfig, "vector store timeout must be positive")
		}
	}

	return nil
}

// validateDocumentStoreConfig validates document store configuration
func (dm *documentsModule) validateDocumentStoreConfig(config map[string]interface{}) error {
	if storeType, ok := config["type"].(string); ok {
		validTypes := []string{"memory", "sql"}
		isValid := false
		for _, validType := range validTypes {
			if storeType == validType {
				isValid = true
				break
			}
		}
		if !isValid {
			return models.NewDocumentError(models.ErrInvalidConfig, fmt.Sprintf("unsupported document store type: %s", storeType))
		}
	}

	if tableName, ok := config["table_name"].(string); ok {
		if tableName == "" {
			return models.NewDocumentError(models.ErrInvalidConfig, "document store table_name cannot be empty")
		}
	}

	if batchSize, ok := config["batch_size"].(int); ok {
		if batchSize <= 0 {
			return models.NewDocumentError(models.ErrInvalidConfig, "document store batch_size must be positive")
		}
	}

	if timeout, ok := config["timeout"].(int); ok {
		if timeout <= 0 {
			return models.NewDocumentError(models.ErrInvalidConfig, "document store timeout must be positive")
		}
	}

	return nil
}

// DefaultDocumentsConfig returns a default configuration
func DefaultDocumentsConfig() DocumentsConfig {
	return DocumentsConfig{
		ChunkingConfig:         models.DefaultChunkingConfig(),
		SearchConfig:          models.DefaultSearchConfig(),
		MaxWorkers:            DefaultMaxWorkers,
		ProcessingTimeout:     DefaultTimeout,
		BatchSize:             100,
		RetryAttempts:        3,
		RetryDelay:           5,
		VectorStoreConfig:    make(map[string]interface{}),
		DocumentStoreConfig:  make(map[string]interface{}),
		CacheEnabled:         true,
		CacheTTL:             3600,
		CacheMaxSize:         10000,
		FileWatcherEnabled:   false,
		WatchPaths:           []string{},
		IgnorePatterns:       []string{".git", ".DS_Store", "*.tmp"},
		MetricsEnabled:       true,
		HealthCheckInterval:  30,
		RateLimitEnabled:     false,
		RateLimitRequests:    1000,
		RateLimitDuration:    3600,
		EnableAdvancedSearch: true,
		EnableContentAnalysis: false,
		EnableEventBus:       false,
	}
}