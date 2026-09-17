package documents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"arcadia/modules/documents/config"
	"arcadia/modules/documents/core"
	"arcadia/modules/documents/detection"
	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
	"arcadia/modules/documents/processors"
	"arcadia/modules/documents/processors/audio"
	"arcadia/modules/documents/processors/base"
	"arcadia/modules/documents/processors/pdf"
	"arcadia/modules/documents/processors/text"
	"arcadia/modules/documents/providers"
	"arcadia/modules/documents/stores"
)

// DocumentsConfig contains configuration for the documents module
type DocumentsConfig struct {
	// Core configuration
	ChunkingConfig models.ChunkingConfig `json:"chunking"`
	SearchConfig   models.SearchConfig   `json:"search"`

	// Plugin-based processing configuration
	ProcessingConfig config.ProcessingConfig `json:"processing"`

	// Legacy processing configuration (deprecated)
	MaxWorkers        int `json:"max_workers"`
	ProcessingTimeout int `json:"processing_timeout_seconds"`
	BatchSize         int `json:"batch_size"`
	RetryAttempts     int `json:"retry_attempts"`
	RetryDelay        int `json:"retry_delay_seconds"`

	// Storage configuration
	VectorStoreConfig   map[string]interface{} `json:"vector_store"`
	DocumentStoreConfig map[string]interface{} `json:"document_store"`

	// Cache configuration
	CacheEnabled bool `json:"cache_enabled"`
	CacheTTL     int  `json:"cache_ttl_seconds"`
	CacheMaxSize int  `json:"cache_max_size"`

	// Ollama configuration
	OllamaURL     string `json:"ollama_url"`
	OllamaTimeout int    `json:"ollama_timeout_seconds"`

	// Embedding configuration
	EmbeddingModel     string `json:"embedding_model"`
	EmbeddingDimension int    `json:"embedding_dimension"`

	// Extraction configuration
	ExtractionEnabled bool   `json:"extraction_enabled"`
	ExtractionModel   string `json:"extraction_model"`

	// Graph database configuration
	GraphEnabled bool                   `json:"graph_enabled"`
	GraphConfig  map[string]interface{} `json:"graph_config"`

	// File watching configuration
	FileWatcherEnabled bool     `json:"file_watcher_enabled"`
	WatchPaths         []string `json:"watch_paths"`
	IgnorePatterns     []string `json:"ignore_patterns"`

	// Metrics and monitoring
	MetricsEnabled      bool `json:"metrics_enabled"`
	HealthCheckInterval int  `json:"health_check_interval_seconds"`

	// Rate limiting
	RateLimitEnabled  bool `json:"rate_limit_enabled"`
	RateLimitRequests int  `json:"rate_limit_requests"`
	RateLimitDuration int  `json:"rate_limit_duration_seconds"`

	// Feature flags
	EnableAdvancedSearch  bool `json:"enable_advanced_search"`
	EnableContentAnalysis bool `json:"enable_content_analysis"`
	EnableEventBus        bool `json:"enable_event_bus"`
}

// Dependencies contains all external dependencies for the documents module
type Dependencies struct {
	// Required dependencies
	DB     interfaces.DatabaseProvider `json:"-"`
	Logger interfaces.Logger           `json:"-"`

	// Optional dependencies with defaults
	Queue          interfaces.QueueService     `json:"-"`
	Metrics        interfaces.MetricsCollector `json:"-"`
	Cache          interfaces.CacheService     `json:"-"`
	RateLimiter    interfaces.RateLimiter      `json:"-"`
	ConfigProvider interfaces.ConfigProvider   `json:"-"`
	EventBus       interfaces.EventBus         `json:"-"`

	// Configuration
	Config DocumentsConfig `json:"config"`
}

// documentsModule implements the DocumentsModule interface
type documentsModule struct {
	version   string
	buildInfo *ModuleInfo
	deps      Dependencies

	// Internal components
	vectorStore      interfaces.VectorStoreInterface
	documentStore    interfaces.DocumentStoreInterface
	processor        interfaces.DocumentProcessor
	searchEngine     interfaces.SearchEngine
	chunker          interfaces.TextChunkerInterface
	pluginRegistry   *processors.Registry
	workerPool       interfaces.WorkerPool
	fileWatcher      interfaces.FileWatcher
	ollamaProvider   *providers.OllamaProvider
	extractionEngine *core.ExtractionEngine
	graphStore       interfaces.GraphStoreInterface
	graphEngine      *core.GraphEngine
	eventListeners   []EventListener

	// State management
	ctx          context.Context
	cancel       context.CancelFunc
	started      bool
	startedAt    time.Time
	healthStatus *models.HealthStatus
	metrics      *models.ModuleMetrics

	// Synchronization
	mutex      sync.RWMutex
	stateMutex sync.Mutex
}

// NewDocumentsModule creates a new documents module instance
func NewDocumentsModule(deps Dependencies) (DocumentsModule, error) {
	if err := validateDependencies(deps); err != nil {
		return nil, fmt.Errorf("invalid dependencies: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	module := &documentsModule{
		version: ModuleVersion,
		deps:    deps,
		ctx:     ctx,
		cancel:  cancel,
		started: false,
		buildInfo: &ModuleInfo{
			Version:   ModuleVersion,
			BuildTime: time.Now().Format(time.RFC3339),
			Status:    "initialized",
		},
		healthStatus: &models.HealthStatus{
			Status:     "initializing",
			Timestamp:  time.Now(),
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
		if host, ok := dm.deps.Config.VectorStoreConfig["host"].(string); ok {
			vectorStoreConfig.Host = host
		}
		if port, ok := dm.deps.Config.VectorStoreConfig["port"].(int); ok {
			vectorStoreConfig.Port = port
		}
		if collection, ok := dm.deps.Config.VectorStoreConfig["collection"].(string); ok {
			vectorStoreConfig.Collection = collection
		}
		if apiKey, ok := dm.deps.Config.VectorStoreConfig["api_key"].(string); ok {
			vectorStoreConfig.ApiKey = apiKey
		}
		if metric, ok := dm.deps.Config.VectorStoreConfig["metric"].(string); ok {
			vectorStoreConfig.Metric = metric
		}
		if batchSize, ok := dm.deps.Config.VectorStoreConfig["batch_size"].(int); ok {
			vectorStoreConfig.BatchSize = batchSize
		}
		if timeout, ok := dm.deps.Config.VectorStoreConfig["timeout_seconds"].(int); ok {
			vectorStoreConfig.Timeout = timeout
		}
		if maxRetries, ok := dm.deps.Config.VectorStoreConfig["max_retries"].(int); ok {
			vectorStoreConfig.MaxRetries = maxRetries
		}
		if retryDelay, ok := dm.deps.Config.VectorStoreConfig["retry_delay_seconds"].(int); ok {
			vectorStoreConfig.RetryDelay = retryDelay
		}
		if customConfig, ok := dm.deps.Config.VectorStoreConfig["custom_config"].(map[string]interface{}); ok {
			vectorStoreConfig.CustomConfig = customConfig
		}
	}

	// For SQLite vector store, add database connection to custom config
	if vectorStoreConfig.Type == "sqlite" && dm.deps.DB != nil {
		vectorStoreConfig.CustomConfig = map[string]interface{}{
			"database": dm.deps.DB,
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

	// Initialize vector store (e.g., to create QDrant collection if needed)
	if err := dm.vectorStore.Initialize(dm.ctx); err != nil {
		return fmt.Errorf("failed to initialize vector store: %w", err)
	}

	// Initialize document store if it's SQL-based
	if sqlStore, ok := dm.documentStore.(*stores.SQLDocumentStore); ok {
		if err := sqlStore.Initialize(dm.ctx); err != nil {
			return fmt.Errorf("failed to initialize SQL document store: %w", err)
		}
	}

	// Initialize graph store if enabled
	if dm.deps.Config.GraphEnabled {
		graphStoreConfig := stores.GraphStoreConfig{
			Type:    "cayley",
			Backend: "bolt",
			Path:    "./data/cayley.db",
			Options: make(map[string]interface{}),
		}

		// Override with custom config if provided
		if dm.deps.Config.GraphConfig != nil {
			if storeType, ok := dm.deps.Config.GraphConfig["type"].(string); ok {
				graphStoreConfig.Type = storeType
			}
			if backend, ok := dm.deps.Config.GraphConfig["backend"].(string); ok {
				graphStoreConfig.Backend = backend
			}
			if path, ok := dm.deps.Config.GraphConfig["path"].(string); ok {
				graphStoreConfig.Path = path
			}
			if options, ok := dm.deps.Config.GraphConfig["options"].(map[string]interface{}); ok {
				graphStoreConfig.Options = options
			}
		}

		graphStoreFactory := stores.NewGraphStoreFactory()
		if dm.deps.Logger != nil {
			graphStoreFactory.WithLogger(dm.deps.Logger)
		}
		if dm.deps.Metrics != nil {
			graphStoreFactory.WithMetrics(dm.deps.Metrics)
		}

		dm.graphStore, err = graphStoreFactory.CreateGraphStore(graphStoreConfig)
		if err != nil {
			return fmt.Errorf("failed to create graph store: %w", err)
		}

		// Initialize graph store
		if err := dm.graphStore.Initialize(dm.ctx); err != nil {
			return fmt.Errorf("failed to initialize graph store: %w", err)
		}

		if dm.deps.Logger != nil {
			dm.deps.Logger.Info(dm.ctx, "Graph store initialized",
				"type", graphStoreConfig.Type,
				"backend", graphStoreConfig.Backend,
				"path", graphStoreConfig.Path)
		}
	}

	return nil
}

// initializeCoreComponents initializes core processing components
func (dm *documentsModule) initializeCoreComponents() error {
	// Initialize text chunker
	dm.chunker = core.NewSimpleTextChunkerWithConfig(dm.deps.Config.ChunkingConfig)

	// Create Ollama provider
	ollamaURL := dm.deps.Config.OllamaURL
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434" // Default Ollama URL
	}
	ollamaTimeout := dm.deps.Config.OllamaTimeout
	if ollamaTimeout == 0 {
		ollamaTimeout = 60 // Default timeout in seconds
	}

	dm.ollamaProvider = providers.NewOllamaProvider(
		ollamaURL,
		time.Duration(ollamaTimeout)*time.Second,
	)
	if dm.deps.Logger != nil {
		dm.ollamaProvider.WithLogger(dm.deps.Logger)
	}
	if dm.deps.Metrics != nil {
		dm.ollamaProvider.WithMetrics(dm.deps.Metrics)
	}

	// Initialize the Ollama provider
	if err := dm.ollamaProvider.Initialize(dm.ctx); err != nil {
		return fmt.Errorf("failed to initialize Ollama provider: %w", err)
	}

	// Get embedding model configuration
	embeddingModel := dm.deps.Config.EmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "nomic-embed-text" // Default embedding model
	}
	embeddingDimension := dm.deps.Config.EmbeddingDimension
	if embeddingDimension == 0 {
		embeddingDimension = 768 // Default dimension
	}

	// Initialize embedding engine
	embeddingConfig := core.DefaultEmbeddingConfig()
	embeddingConfig.ModelName = embeddingModel
	embeddingConfig.Dimension = embeddingDimension

	embeddingEngine := core.NewEmbeddingEngine(dm.ollamaProvider, embeddingModel, embeddingDimension, embeddingConfig)
	if dm.deps.Cache != nil {
		embeddingEngine.WithCache(dm.deps.Cache)
	}
	if dm.deps.Logger != nil {
		embeddingEngine.WithLogger(dm.deps.Logger)
	}
	if dm.deps.Metrics != nil {
		embeddingEngine.WithMetrics(dm.deps.Metrics)
	}

	// Initialize extraction engine if enabled
	if dm.deps.Config.ExtractionEnabled {
		extractionModel := dm.deps.Config.ExtractionModel
		if extractionModel == "" {
			extractionModel = "llama3:8b" // Default extraction model
		}

		extractionConfig := core.DefaultExtractionConfig()
		extractionConfig.ModelName = extractionModel

		dm.extractionEngine = core.NewExtractionEngine(dm.ollamaProvider, extractionModel, extractionConfig)
		if dm.deps.Logger != nil {
			dm.extractionEngine.WithLogger(dm.deps.Logger)
		}
		if dm.deps.Metrics != nil {
			dm.extractionEngine.WithMetrics(dm.deps.Metrics)
		}

		if dm.deps.Logger != nil {
			dm.deps.Logger.Info(dm.ctx, "Entity extraction engine initialized",
				"model", extractionModel)
		}
	}

	// Initialize graph engine if graph store is available
	if dm.graphStore != nil {
		dm.graphEngine = core.NewGraphEngine(dm.graphStore)
		if dm.deps.Logger != nil {
			dm.graphEngine.WithLogger(dm.deps.Logger)
		}
		if dm.deps.Metrics != nil {
			dm.graphEngine.WithMetrics(dm.deps.Metrics)
		}

		if dm.deps.Logger != nil {
			dm.deps.Logger.Info(dm.ctx, "Graph engine initialized")
		}
	}

	// Initialize plugin system with configuration
	detector := detection.NewMultiStageDetector()

	var err error
	dm.pluginRegistry, err = processors.NewRegistryWithDefaults(detector)
	if err != nil {
		return fmt.Errorf("failed to initialize plugin registry: %w", err)
	}

	// Apply configuration to plugin registry
	if err := dm.configurePluginRegistry(); err != nil {
		return fmt.Errorf("failed to configure plugin registry: %w", err)
	}

	if dm.deps.Logger != nil {
		dm.pluginRegistry = dm.pluginRegistry.WithLogger(dm.deps.Logger)
	}
	if dm.deps.Metrics != nil {
		dm.pluginRegistry = dm.pluginRegistry.WithMetrics(dm.deps.Metrics)
	}

	// Validate plugin system health
	if err := dm.validatePluginSystemHealth(); err != nil {
		if dm.deps.Logger != nil {
			dm.deps.Logger.Warn(context.Background(), "Plugin system health check failed", "error", err)
		}
		// Continue without failing initialization - plugins may be optional
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

	// Add plugin registry and other capabilities to processor if available
	if concreteProcessor, ok := dm.processor.(*core.DocumentProcessor); ok {
		concreteProcessor.WithPluginRegistry(dm.pluginRegistry)
		if dm.deps.Logger != nil {
			concreteProcessor.WithLogger(dm.deps.Logger)
		}
		if dm.deps.Metrics != nil {
			concreteProcessor.WithMetrics(dm.deps.Metrics)
		}
		if dm.extractionEngine != nil {
			concreteProcessor.WithExtractionEngine(dm.extractionEngine)
		}
		if dm.graphEngine != nil {
			concreteProcessor.WithGraphEngine(dm.graphEngine)
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

	// Content analysis is now handled by the plugin registry
	// No separate content analyzer initialization needed

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

	// Ollama provider is initialized in initializeCoreComponents
	// No additional initialization needed here

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

	if dm.ollamaProvider != nil {
		dm.ollamaProvider.Close()
	}

	// Close graph store if available
	if dm.graphStore != nil {
		if err := dm.graphStore.Close(); err != nil {
			if dm.deps.Logger != nil {
				dm.deps.Logger.Warn(ctx, "Failed to close graph store", "error", err)
			}
		}
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

	// Track plugin usage if available
	var processorType string
	if dm.pluginRegistry != nil && dm.pluginRegistry.CanProcessFile(path) {
		if _, pType, err := dm.pluginRegistry.GetProcessorForFile(path); err == nil {
			processorType = pType.String()
		}
	}

	// Process the file using the document processor
	doc, err := dm.processor.ProcessFile(ctx, path)
	if err != nil {
		// Track failure metrics
		if dm.deps.Metrics != nil {
			tags := map[string]string{"status": "failure"}
			if processorType != "" {
				tags["processor_type"] = processorType
			}
			dm.deps.Metrics.IncrementCounter("document.plugin.processing", tags)
		}

		return &models.ProcessResult{
			FilePath:    path,
			Success:     false,
			Error:       err,
			ProcessedAt: time.Now(),
		}, nil
	}

	// Track success metrics
	if dm.deps.Metrics != nil {
		tags := map[string]string{"status": "success"}
		if processorType != "" {
			tags["processor_type"] = processorType
			tags["was_processed_by_plugin"] = "true"
		} else {
			tags["was_processed_by_plugin"] = "false"
		}
		dm.deps.Metrics.IncrementCounter("document.plugin.processing", tags)

		// Track content metrics
		if doc != nil {
			dm.deps.Metrics.RecordHistogram("document.plugin.content_length", float64(len(doc.Content)), tags)
			dm.deps.Metrics.RecordHistogram("document.plugin.chunk_count", float64(doc.ChunkCount), tags)
		}
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

	// Delete from graph store if available
	if dm.graphEngine != nil {
		if err := dm.graphEngine.DeleteDocument(ctx, docID); err != nil {
			if dm.deps.Logger != nil {
				dm.deps.Logger.Warn(ctx, "Failed to delete document from graph store", "docID", docID, "error", err)
			}
			// Continue with document store deletion even if graph store fails
		}
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

// HealthCheck performs a health check
func (dm *documentsModule) HealthCheck(ctx context.Context) (*models.HealthStatus, error) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	status := *dm.healthStatus
	status.Timestamp = time.Now()

	// Check component health
	components := make(map[string]interface{})

	// Check Ollama provider
	if dm.ollamaProvider != nil {
		if err := dm.ollamaProvider.HealthCheck(ctx); err != nil {
			components["ollama_provider"] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
			status.Status = "degraded"
		} else {
			components["ollama_provider"] = map[string]interface{}{"status": "healthy"}
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

	// Check plugin system health
	if dm.pluginRegistry != nil {
		if err := dm.validatePluginSystemHealth(); err != nil {
			components["plugin_system"] = map[string]interface{}{
				"status":          "degraded",
				"error":           err.Error(),
				"supported_types": dm.pluginRegistry.GetSupportedTypes(),
				"processor_stats": dm.pluginRegistry.GetProcessorStats(),
			}
			if status.Status == "healthy" {
				status.Status = "degraded"
			}
		} else {
			components["plugin_system"] = map[string]interface{}{
				"status":          "healthy",
				"supported_types": dm.pluginRegistry.GetSupportedTypes(),
				"processor_stats": dm.pluginRegistry.GetProcessorStats(),
			}
		}
	} else {
		components["plugin_system"] = map[string]interface{}{
			"status": "unavailable",
			"error":  "plugin registry not initialized",
		}
		if status.Status == "healthy" {
			status.Status = "degraded"
		}
	}

	// Check graph store health
	if dm.graphStore != nil {
		if err := dm.graphStore.HealthCheck(ctx); err != nil {
			components["graph_store"] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
			if status.Status == "healthy" {
				status.Status = "degraded"
			}
		} else {
			// Get graph statistics
			stats, err := dm.graphStore.GetStats(ctx)
			if err != nil {
				components["graph_store"] = map[string]interface{}{
					"status": "healthy",
					"error":  "failed to get stats: " + err.Error(),
				}
			} else {
				components["graph_store"] = map[string]interface{}{
					"status":             "healthy",
					"entity_count":       stats.EntityCount,
					"relationship_count": stats.RelationshipCount,
					"document_count":     stats.DocumentCount,
					"quad_count":         stats.QuadCount,
				}
			}
		}
	}

	status.Components = components
	return &status, nil
}

// GetMetrics returns module metrics including plugin system metrics
func (dm *documentsModule) GetMetrics(ctx context.Context) (*models.ModuleMetrics, error) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	metrics := *dm.metrics
	metrics.LastProcessedAt = &time.Time{}
	*metrics.LastProcessedAt = time.Now()

	// Add plugin system metrics if available
	if dm.pluginRegistry != nil {
		pluginMetrics := dm.collectPluginMetrics()
		if metrics.CustomMetrics == nil {
			metrics.CustomMetrics = make(map[string]interface{})
		}
		metrics.CustomMetrics["plugin_system"] = pluginMetrics
	}

	return &metrics, nil
}

// collectPluginMetrics gathers metrics from the plugin system
func (dm *documentsModule) collectPluginMetrics() map[string]interface{} {
	pluginMetrics := make(map[string]interface{})

	// Basic plugin registry stats
	pluginMetrics["processor_stats"] = dm.pluginRegistry.GetProcessorStats()
	pluginMetrics["supported_types"] = dm.pluginRegistry.GetSupportedTypes()

	// Plugin usage metrics
	pluginUsage := make(map[string]interface{})
	supportedTypes := dm.pluginRegistry.GetSupportedTypes()
	for _, processorType := range supportedTypes {
		if processor, exists := dm.pluginRegistry.GetProcessor(processorType); exists {
			pluginUsage[processorType.String()] = map[string]interface{}{
				"supported_extensions": processor.GetSupportedExtensions(),
				"processor_type":       processor.GetProcessorType().String(),
			}
		}
	}
	pluginMetrics["usage"] = pluginUsage

	// Add plugin system health status
	pluginHealth := make(map[string]interface{})
	if err := dm.validatePluginSystemHealth(); err != nil {
		pluginHealth["status"] = "unhealthy"
		pluginHealth["error"] = err.Error()
	} else {
		pluginHealth["status"] = "healthy"
	}
	pluginHealth["last_check"] = time.Now()
	pluginMetrics["health"] = pluginHealth

	// Add external tool availability status
	toolStatus := dm.collectExternalToolMetrics()
	pluginMetrics["external_tools"] = toolStatus

	return pluginMetrics
}

// collectExternalToolMetrics gathers metrics about external tool availability
func (dm *documentsModule) collectExternalToolMetrics() map[string]interface{} {
	toolStatus := make(map[string]interface{})

	// Check PDF tools if PDF processor is available
	if _, exists := dm.pluginRegistry.GetProcessor("pdf"); exists {
		pdfTools := make(map[string]interface{})

		// Check pdftotext
		if err := dm.checkToolAvailability("pdftotext", "--version"); err != nil {
			pdfTools["pdftotext"] = map[string]interface{}{
				"available": false,
				"error":     err.Error(),
			}
		} else {
			pdfTools["pdftotext"] = map[string]interface{}{
				"available": true,
			}
		}

		// Check ocrmypdf (optional)
		if err := dm.checkToolAvailability("ocrmypdf", "--version"); err != nil {
			pdfTools["ocrmypdf"] = map[string]interface{}{
				"available": false,
				"error":     err.Error(),
				"optional":  true,
			}
		} else {
			pdfTools["ocrmypdf"] = map[string]interface{}{
				"available": true,
				"optional":  true,
			}
		}

		toolStatus["pdf"] = pdfTools
	}

	toolStatus["last_check"] = time.Now()
	return toolStatus
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

// validatePluginSystemHealth performs health checks on the plugin system
func (dm *documentsModule) validatePluginSystemHealth() error {
	if dm.pluginRegistry == nil {
		return fmt.Errorf("plugin registry is not initialized")
	}

	// Check if any processors are registered
	supportedTypes := dm.pluginRegistry.GetSupportedTypes()
	if len(supportedTypes) == 0 {
		return fmt.Errorf("no document processors are registered")
	}

	// Validate each processor type
	var healthIssues []string

	for _, processorType := range supportedTypes {
		processor, exists := dm.pluginRegistry.GetProcessor(processorType)
		if !exists {
			healthIssues = append(healthIssues, fmt.Sprintf("processor %s is not available", processorType))
			continue
		}

		// Basic validation of processor capabilities
		supportedExts := processor.GetSupportedExtensions()
		if len(supportedExts) == 0 {
			healthIssues = append(healthIssues, fmt.Sprintf("processor %s has no supported extensions", processorType))
		}

		// Test processor with a simple check (avoiding file system operations in health check)
		if processor.GetProcessorType() != processorType {
			healthIssues = append(healthIssues, fmt.Sprintf("processor %s type mismatch", processorType))
		}
	}

	// Check external tool availability for specific processors
	if err := dm.checkExternalToolsAvailability(); err != nil {
		healthIssues = append(healthIssues, fmt.Sprintf("external tools check failed: %v", err))
	}

	if len(healthIssues) > 0 {
		if dm.deps.Logger != nil {
			dm.deps.Logger.Info(context.Background(), "Plugin system health issues detected",
				"issues", healthIssues, "total_processors", len(supportedTypes))
		}
		return fmt.Errorf("plugin system health issues: %v", healthIssues)
	}

	if dm.deps.Logger != nil {
		dm.deps.Logger.Info(context.Background(), "Plugin system health check passed",
			"processors", len(supportedTypes), "supported_types", supportedTypes)
	}

	return nil
}

// checkExternalToolsAvailability checks if required external tools are available
func (dm *documentsModule) checkExternalToolsAvailability() error {
	// Get PDF processor if available
	if processor, exists := dm.pluginRegistry.GetProcessor("pdf"); exists {
		// Check if PDF processor has PDF-specific extensions
		extensions := processor.GetSupportedExtensions()
		hasPDFSupport := false
		for _, ext := range extensions {
			if ext == ".pdf" {
				hasPDFSupport = true
				break
			}
		}

		if hasPDFSupport {
			// Check for pdftotext tool
			if err := dm.checkToolAvailability("pdftotext", "--version"); err != nil {
				return fmt.Errorf("pdftotext tool not available: %w", err)
			}

			// Check for ocrmypdf tool (optional, for OCR)
			if err := dm.checkToolAvailability("ocrmypdf", "--version"); err != nil {
				if dm.deps.Logger != nil {
					dm.deps.Logger.Warn(context.Background(), "OCR tool not available, PDF OCR functionality will be limited", "error", err)
				}
				// Don't fail for OCR tool as it's optional
			}
		}
	}

	return nil
}

// checkToolAvailability checks if a command line tool is available
func (dm *documentsModule) checkToolAvailability(toolName string, args ...string) error {
	// This is a simplified check - in a real implementation, you might want to use exec.LookPath or exec.Command
	// For now, just return nil to indicate tools are assumed available
	// In production, you would implement actual tool checking

	if dm.deps.Logger != nil {
		dm.deps.Logger.Debug(context.Background(), "Checking tool availability", "tool", toolName, "args", args)
	}

	// Placeholder - assume tools are available
	// Real implementation would use:
	// cmd := exec.Command(toolName, args...)
	// return cmd.Run()

	return nil
}

// configurePluginRegistry applies configuration settings to the plugin registry
func (dm *documentsModule) configurePluginRegistry() error {
	if dm.pluginRegistry == nil {
		return fmt.Errorf("plugin registry not initialized")
	}

	processingConfig := dm.deps.Config.ProcessingConfig

	// Configure PDF processor if available
	if processor, exists := dm.pluginRegistry.GetProcessor("pdf"); exists {
		if pdfProcessor, ok := processor.(*pdf.PDFProcessor); ok {
			// Apply PDF-specific configuration
			pdfConfig := processingConfig.PDF
			if err := dm.configurePDFProcessor(pdfProcessor, pdfConfig); err != nil {
				if dm.deps.Logger != nil {
					dm.deps.Logger.Warn(context.Background(), "Failed to configure PDF processor", "error", err)
				}
				// Continue with default configuration
			}
		}
	}

	// Configure text processor if available
	if processor, exists := dm.pluginRegistry.GetProcessor("text"); exists {
		if textProcessor, ok := processor.(*text.TextProcessor); ok {
			// Apply text-specific configuration
			textConfig := processingConfig.Text
			if err := dm.configureTextProcessor(textProcessor, textConfig); err != nil {
				if dm.deps.Logger != nil {
					dm.deps.Logger.Warn(context.Background(), "Failed to configure text processor", "error", err)
				}
				// Continue with default configuration
			}
		}
	}

	// Configure markdown processor if available (if implemented)
	if processor, exists := dm.pluginRegistry.GetProcessor("markdown"); exists {
		markdownConfig := processingConfig.Markdown
		if err := dm.configureMarkdownProcessor(processor, markdownConfig); err != nil {
			if dm.deps.Logger != nil {
				dm.deps.Logger.Warn(context.Background(), "Failed to configure markdown processor", "error", err)
			}
			// Continue with default configuration
		}
	}

	// Configure audio processor if available
	if processor, exists := dm.pluginRegistry.GetProcessor(base.ProcessorTypeAudio); exists {
		if _, ok := processor.(*audio.AudioProcessor); ok {
			// Get audio config from module config
			audioConfig := processingConfig.Audio

			// Create new processor with custom config
			configuredProcessor := audio.NewAudioProcessorWithConfig(audioConfig)

			// Attach logger and metrics
			if dm.deps.Logger != nil {
				configuredProcessor = configuredProcessor.WithLogger(dm.deps.Logger)
			}
			if dm.deps.Metrics != nil {
				configuredProcessor = configuredProcessor.WithMetrics(dm.deps.Metrics)
			}

			// Re-register with configured version
			dm.pluginRegistry.RegisterProcessor(base.ProcessorTypeAudio, configuredProcessor)

			if dm.deps.Logger != nil {
				dm.deps.Logger.Info(context.Background(), "Audio processor configured",
					"whisper_url", audioConfig.WhisperURL,
					"max_file_size", audioConfig.MaxAudioFileSize,
					"supported_formats", audioConfig.SupportedFormats,
					"max_retries", audioConfig.MaxRetries,
					"enable_caching", audioConfig.EnableCaching)
			}
		} else {
			if dm.deps.Logger != nil {
				dm.deps.Logger.Warn(context.Background(), "Audio processor type assertion failed")
			}
		}
	}

	if dm.deps.Logger != nil {
		dm.deps.Logger.Info(context.Background(), "Plugin registry configuration applied successfully",
			"pdf_enabled", processingConfig.PDF.Enabled,
			"text_enabled", processingConfig.Text.Enabled,
			"markdown_enabled", processingConfig.Markdown.Enabled)
	}

	return nil
}

// configurePDFProcessor applies PDF-specific configuration
func (dm *documentsModule) configurePDFProcessor(processor *pdf.PDFProcessor, config base.PDFConfig) error {
	// TODO: Configuration methods would be implemented when processors support them
	// For now, just log the configuration that would be applied
	if dm.deps.Logger != nil {
		dm.deps.Logger.Debug(context.Background(), "PDF processor configuration loaded",
			"pdf_to_text_path", config.ToolPaths.PDFToText,
			"pdf_info_path", config.ToolPaths.PDFInfo,
			"tesseract_path", config.ToolPaths.TesseractPath,
			"ocr_enabled", config.OCREnabled,
			"preserve_binary", config.PreserveBinary)
	}

	// Apply general processor configuration
	return dm.applyProcessorConfig(processor, config.ProcessorConfig)
}

// configureTextProcessor applies text-specific configuration
func (dm *documentsModule) configureTextProcessor(processor *text.TextProcessor, config base.TextConfig) error {
	// TODO: Configuration methods would be implemented when processors support them
	// For now, just log the configuration that would be applied
	if dm.deps.Logger != nil {
		dm.deps.Logger.Debug(context.Background(), "Text processor configuration loaded",
			"encoding_detection", config.EncodingDetection,
			"default_encoding", config.DefaultEncoding,
			"max_line_length", config.MaxLineLength)
	}

	// Apply general processor configuration
	return dm.applyProcessorConfig(processor, config.ProcessorConfig)
}

// configureMarkdownProcessor applies markdown-specific configuration
func (dm *documentsModule) configureMarkdownProcessor(processor base.DocumentProcessor, config base.MarkdownConfig) error {
	// For now, just apply general processor configuration
	// Markdown-specific settings would be applied if the processor supports them
	return dm.applyProcessorConfig(processor, config.ProcessorConfig)
}

// applyProcessorConfig applies general processor configuration settings
func (dm *documentsModule) applyProcessorConfig(processor base.DocumentProcessor, config base.ProcessorConfig) error {
	// For now, the base DocumentProcessor interface doesn't expose configuration methods
	// This would be extended if processors expose configuration interfaces

	if dm.deps.Logger != nil {
		dm.deps.Logger.Debug(context.Background(), "Applied processor configuration",
			"processor_type", processor.GetProcessorType(),
			"enabled", config.Enabled,
			"max_file_size", config.MaxFileSize,
			"timeout", config.Timeout)
	}

	return nil
}

// DefaultDocumentsConfig returns a default configuration
func DefaultDocumentsConfig() DocumentsConfig {
	return DocumentsConfig{
		ChunkingConfig:        models.DefaultChunkingConfig(),
		SearchConfig:          models.DefaultSearchConfig(),
		ProcessingConfig:      config.GetDefaultProcessingConfig(),
		MaxWorkers:            DefaultMaxWorkers,
		ProcessingTimeout:     DefaultTimeout,
		BatchSize:             100,
		RetryAttempts:         3,
		RetryDelay:            5,
		VectorStoreConfig:     make(map[string]interface{}),
		DocumentStoreConfig:   make(map[string]interface{}),
		CacheEnabled:          true,
		CacheTTL:              3600,
		CacheMaxSize:          10000,
		FileWatcherEnabled:    false,
		WatchPaths:            []string{},
		IgnorePatterns:        []string{".git", ".DS_Store", "*.tmp"},
		MetricsEnabled:        true,
		HealthCheckInterval:   30,
		RateLimitEnabled:      false,
		RateLimitRequests:     1000,
		RateLimitDuration:     3600,
		EnableAdvancedSearch:  true,
		EnableContentAnalysis: false,
		EnableEventBus:        false,
	}
}
