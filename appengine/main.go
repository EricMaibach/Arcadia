package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"arcadia/config"
	"arcadia/handlers"
	"arcadia/modules/documents"
	"arcadia/modules/documents/models"
	"arcadia/services"
	"arcadia/modules/ai"
	aiInterfaces "arcadia/modules/ai/interfaces"
	aiModels "arcadia/modules/ai/models"
	"arcadia/pkg/logging"
)

var (
	// Configuration and registry managers
	configManager   *config.Manager
	registryManager *services.RegistryManager
	wasmRuntime     *services.WasmRuntime
	fileWatcher     services.FileWatcher

	// Module components
	documentsModule documents.DocumentsModule
	aiModule        ai.AIModule
	queueProcessor  *QueueProcessor

	// Logger for global functions
	globalLogger logging.Logger
)

// --- Request Types ---

type AppRequest struct {
	AppID   string              `json:"appId"`
	Version string              `json:"version"`
	Runtime string              `json:"runtime"`
	Tools   []services.ToolInfo `json:"tools"`
	AppSrc  string              `json:"appSrc"`
}

// FileEventData represents file event data for queue processing
type FileEventData struct {
	FilePath  string    `json:"file_path"`
	EventType string    `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`
	IsDir     bool      `json:"is_dir"`
}

// QueueProcessor handles background processing of queued file events
type QueueProcessor struct {
	queue           *services.QueueService
	documentsModule documents.DocumentsModule
	workers         int
	stopChannel     chan bool
	running         bool
	logger          logging.Logger
}

// --- Adapter Implementations for Documents Module ---

// DatabaseAdapter adapts services.Database to the documents module interface
type DatabaseAdapter struct {
	systemDB services.Database
}

func NewDatabaseAdapter(systemDB services.Database) *DatabaseAdapter {
	return &DatabaseAdapter{systemDB: systemDB}
}

func (da *DatabaseAdapter) Execute(ctx context.Context, query string, args ...interface{}) error {
	_, err := da.systemDB.Exec(query, args...)
	return err
}

func (da *DatabaseAdapter) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return da.systemDB.Query(query, args...)
}

func (da *DatabaseAdapter) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	// Check if systemDB is nil first
	if da.systemDB == nil {
		// Create a temporary database to return a proper sql.Row with error
		tempDB, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			// If we can't even create a temp DB, return a nil row
			// This will cause a panic when scanned, which is appropriate for this critical error
			return nil
		}
		defer tempDB.Close()
		return tempDB.QueryRowContext(ctx, "SELECT 1 WHERE 0") // Returns sql.ErrNoRows
	}

	// Use the proper QueryRow method from the Database interface
	return da.systemDB.QueryRow(query, args...)
}

func (da *DatabaseAdapter) Transaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	// This requires access to the underlying *sql.DB which isn't exposed
	// For now, we'll return an error indicating this isn't implemented
	return fmt.Errorf("transactions not implemented in database adapter")
}

func (da *DatabaseAdapter) Begin(ctx context.Context) (*sql.Tx, error) {
	return nil, fmt.Errorf("transactions not implemented in database adapter")
}

func (da *DatabaseAdapter) Ping(ctx context.Context) error {
	// Test with a simple query
	_, err := da.systemDB.Query("SELECT 1")
	return err
}

func (da *DatabaseAdapter) Close() error {
	return da.systemDB.Close()
}

func (da *DatabaseAdapter) Stats() sql.DBStats {
	return sql.DBStats{} // Default stats since not available in current interface
}

func executeAppTool(appID, toolName string, input json.RawMessage) (string, error) {
	return wasmRuntime.ExecuteAppTool(appID, toolName, input)
}

// newQueueProcessor creates a new queue processor instance with single worker
func newQueueProcessor(queue *services.QueueService, documentsModule documents.DocumentsModule, logger logging.Logger) *QueueProcessor {
	return &QueueProcessor{
		queue:           queue,
		documentsModule: documentsModule,
		workers:         1, // Single worker for Ollama compatibility
		stopChannel:     make(chan bool, 1),
		running:         false,
		logger:          logger,
	}
}

// start begins the queue processing with a single background goroutine
func (qp *QueueProcessor) start() {
	ctx := context.Background()
	if qp.running {
		if qp.logger != nil {
			qp.logger.Info(ctx, "Queue processor already running")
		}
		return
	}

	qp.running = true
	if qp.logger != nil {
		qp.logger.Info(ctx, "Starting queue processor worker", "workers", qp.workers)
	}

	// Start single worker goroutine
	go qp.processQueueItems()
}

// stop gracefully stops the queue processor
func (qp *QueueProcessor) stop() {
	ctx := context.Background()
	if !qp.running {
		return
	}

	if qp.logger != nil {
		qp.logger.Info(ctx, "Stopping queue processor")
	}
	qp.running = false

	// Signal worker to stop
	select {
	case qp.stopChannel <- true:
	default:
		// Channel might be full, that's ok
	}

	if qp.logger != nil {
		qp.logger.Info(ctx, "Queue processor stopped successfully")
	}
}

// processQueueItems is the main worker loop for processing queue items
func (qp *QueueProcessor) processQueueItems() {
	ctx := context.Background()
	if qp.logger != nil {
		qp.logger.Info(ctx, "Queue processor worker started")
	}

	for qp.running {
		select {
		case <-qp.stopChannel:
			if qp.logger != nil {
				qp.logger.Info(ctx, "Queue processor worker stopping")
			}
			return
		default:
			// Try to get next item from queue
			item, err := qp.queue.Dequeue()
			if err != nil {
				// Log error and continue
				if qp.logger != nil {
					qp.logger.Error(ctx, "Queue dequeue error", "error", err)
				}
				time.Sleep(5 * time.Second)
				continue
			}

			if item == nil {
				// No items in queue, wait before trying again
				time.Sleep(1 * time.Second)
				continue
			}

			// Process the queue item
			if qp.logger != nil {
				qp.logger.Info(ctx, "Processing queue item", "item_id", item.ID)
			}
			qp.processFileEvent(item)
		}
	}

	if qp.logger != nil {
		qp.logger.Info(ctx, "Queue processor worker finished")
	}
}

// processFileEvent processes a single file event from the queue
func (qp *QueueProcessor) processFileEvent(item *services.QueueItem) {
	ctx := context.Background()
	// Parse the event data
	var eventData FileEventData
	if err := json.Unmarshal([]byte(item.Data), &eventData); err != nil {
		if qp.logger != nil {
			qp.logger.Error(ctx, "Failed to parse event data", "error", err, "item_id", item.ID)
		}
		qp.queue.MarkFailed(item.ID, fmt.Sprintf("Failed to parse event data: %v", err))
		return
	}

	if qp.logger != nil {
		qp.logger.Info(ctx, "Processing file event", "event_type", eventData.EventType, "file_path", eventData.FilePath)
	}

	switch eventData.EventType {
	case "create", "modify":
		// Process file for embedding
		err := qp.processFileForEmbedding(eventData.FilePath)
		if err != nil {
			if qp.logger != nil {
				qp.logger.Error(ctx, "Failed to process file", "file_path", eventData.FilePath, "error", err)
			}
			qp.queue.MarkFailed(item.ID, fmt.Sprintf("Processing failed: %v", err))
			return
		}

		if qp.logger != nil {
			qp.logger.Info(ctx, "Successfully processed file", "file_path", eventData.FilePath)
		}
		qp.queue.MarkCompleted(item.ID)

	case "delete":
		// For now, just mark as completed
		// TODO: Implement vector cleanup for deleted files
		if qp.logger != nil {
			qp.logger.Info(ctx, "Handling delete event (cleanup not implemented)", "file_path", eventData.FilePath)
		}
		qp.queue.MarkCompleted(item.ID)

	default:
		if qp.logger != nil {
			qp.logger.Warn(ctx, "Unknown event type", "event_type", eventData.EventType)
		}
		qp.queue.MarkCompleted(item.ID)
	}
}

// processFileForEmbedding sends a file to the embedding service for processing
func (qp *QueueProcessor) processFileForEmbedding(filePath string) error {
	ctx := context.Background()
	// Use documents module only - no fallback to legacy services
	if qp.documentsModule == nil {
		return fmt.Errorf("documents module not available")
	}

	if qp.logger != nil {
		qp.logger.Debug(ctx, "Processing file with documents module", "file_path", filePath)
	}

	result, err := qp.documentsModule.ProcessFile(ctx, filePath)
	if err != nil {
		return fmt.Errorf("documents module processing failed: %v", err)
	}

	if result != nil && result.Success {
		if qp.logger != nil {
			qp.logger.Info(ctx, "Documents module processed file successfully", "file_path", filePath, "document_id", result.DocumentID)
		}
	} else {
		if qp.logger != nil {
			qp.logger.Warn(ctx, "Documents module processing failed", "file_path", filePath, "error", result.Error)
		}
	}

	return nil
}


// EmbeddingSearchAdapter adapts documents module to ai.EmbeddingSearch
type EmbeddingSearchAdapter struct {
	documentsModule documents.DocumentsModule
	logger          logging.Logger
}

func NewEmbeddingSearchAdapter(documentsModule documents.DocumentsModule, logger logging.Logger) *EmbeddingSearchAdapter {
	return &EmbeddingSearchAdapter{
		documentsModule: documentsModule,
		logger:          logger,
	}
}

func (esa *EmbeddingSearchAdapter) SearchDocuments(query string, topK int) ([]*aiModels.DocumentSearchResult, error) {
	ctx := context.Background()
	// Use documents module only - no fallback to legacy services
	if esa.documentsModule == nil {
		return nil, fmt.Errorf("documents module not available")
	}

	if esa.logger != nil {
		esa.logger.Debug(ctx, "Searching documents with documents module", "query", query, "top_k", topK)
	}

	docResults, err := esa.documentsModule.SearchDocuments(ctx, query, topK)
	if err != nil {
		return nil, fmt.Errorf("documents module search failed: %v", err)
	}

	// Convert documents module results to AI service format
	return esa.convertDocumentSearchResults(docResults), nil
}


// convertDocumentSearchResults converts documents module results to AI service format
func (esa *EmbeddingSearchAdapter) convertDocumentSearchResults(docResults []*models.DocumentSearchResult) []*aiModels.DocumentSearchResult {
	ctx := context.Background()
	aiResults := make([]*aiModels.DocumentSearchResult, 0, len(docResults))
	for i, result := range docResults {
		// Skip results with nil documents to prevent crashes
		if result.Document == nil {
			if esa.logger != nil {
				esa.logger.Warn(ctx, "Skipping search result with nil document", "result_index", i)
			}
			continue
		}

		aiResults = append(aiResults, &aiModels.DocumentSearchResult{
			Document: &aiModels.Document{
				ID:         result.Document.ID,
				FilePath:   result.Document.FilePath,
				FileHash:   result.Document.FileHash,
				Content:    result.Document.Content,
				ChunkCount: result.Document.ChunkCount,
				Metadata:   result.Document.Metadata,
				CreatedAt:  result.Document.CreatedAt,
				UpdatedAt:  result.Document.UpdatedAt,
			},
			Chunks: func() []*aiModels.ChunkResult {
				chunks := make([]*aiModels.ChunkResult, len(result.Chunks))
				for j, chunk := range result.Chunks {
					chunks[j] = &aiModels.ChunkResult{
						Content:    chunk.Content,
						Score:      chunk.Score,
						ChunkIndex: chunk.ChunkIndex,
					}
				}
				return chunks
			}(),
			BestScore:     result.BestScore,
			TotalChunks:   result.TotalChunks,
			RelevanceRank: result.RelevanceRank,
		})
	}

	return aiResults
}

func (esa *EmbeddingSearchAdapter) SearchDocumentsEnhanced(query string, topK int, config aiModels.SearchConfig) ([]*aiModels.EnhancedDocumentSearchResult, error) {
	ctx := context.Background()
	// Use documents module only - no fallback to legacy services
	if esa.documentsModule == nil {
		return nil, fmt.Errorf("documents module not available")
	}

	// Convert ai.SearchConfig (empty interface) to models.SearchConfig
	// Since ai.SearchConfig is an empty interface, we'll use the default documents config
	documentsConfig := models.DefaultSearchConfig()
	results, err := esa.documentsModule.SearchDocumentsEnhanced(ctx, query, topK, documentsConfig)
	if err != nil {
		return nil, fmt.Errorf("documents module enhanced search failed: %v", err)
	}

	// Convert models.EnhancedDocumentSearchResult to aiModels.EnhancedDocumentSearchResult
	aiResults := make([]*aiModels.EnhancedDocumentSearchResult, 0, len(results))
	for i, result := range results {
		// Skip results with nil documents to prevent crashes
		if result.Document == nil {
			if esa.logger != nil {
				esa.logger.Warn(ctx, "Skipping enhanced search result with nil document", "result_index", i)
			}
			continue
		}

		aiResults = append(aiResults, &aiModels.EnhancedDocumentSearchResult{
			Document: &aiModels.Document{
				ID:         result.Document.ID,
				FilePath:   result.Document.FilePath,
				FileHash:   result.Document.FileHash,
				Content:    result.Document.Content,
				ChunkCount: result.Document.ChunkCount,
				Metadata:   result.Document.Metadata,
				CreatedAt:  result.Document.CreatedAt,
				UpdatedAt:  result.Document.UpdatedAt,
			},
			ContextHighlights: result.ContextHighlights,
			ContentPreview:    result.ContentPreview,
			IsTruncated:       result.IsTruncated,
			BestScore:         result.BestScore,
			RelevanceRank:     result.RelevanceRank,
		})
	}

	return aiResults, nil
}

func (esa *EmbeddingSearchAdapter) GetDocument(documentID string) (*aiModels.Document, error) {
	// Use documents module only - no fallback to legacy services
	if esa.documentsModule == nil {
		return nil, fmt.Errorf("documents module not available")
	}

	ctx := context.Background()
	doc, err := esa.documentsModule.GetDocument(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("documents module get document failed: %v", err)
	}

	if doc == nil {
		return nil, nil
	}

	// Convert models.Document to aiModels.Document
	return &aiModels.Document{
		ID:         doc.ID,
		FilePath:   doc.FilePath,
		FileHash:   doc.FileHash,
		Content:    doc.Content,
		ChunkCount: doc.ChunkCount,
		Metadata:   doc.Metadata,
		CreatedAt:  doc.CreatedAt,
		UpdatedAt:  doc.UpdatedAt,
	}, nil
}

func (esa *EmbeddingSearchAdapter) ListDocuments() ([]*aiModels.Document, error) {
	// Use documents module only - no fallback to legacy services
	if esa.documentsModule == nil {
		return nil, fmt.Errorf("documents module not available")
	}

	ctx := context.Background()
	docs, err := esa.documentsModule.ListDocuments(ctx)
	if err != nil {
		return nil, fmt.Errorf("documents module list failed: %v", err)
	}

	// Convert models.Document to aiModels.Document
	aiDocs := make([]*aiModels.Document, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}

		aiDocs = append(aiDocs, &aiModels.Document{
			ID:         doc.ID,
			FilePath:   doc.FilePath,
			FileHash:   doc.FileHash,
			Content:    doc.Content,
			ChunkCount: doc.ChunkCount,
			Metadata:   doc.Metadata,
			CreatedAt:  doc.CreatedAt,
			UpdatedAt:  doc.UpdatedAt,
		})
	}

	return aiDocs, nil
}

// File event handler - enqueues file events for processing
func handleFileEvent(event services.FileEvent) {
	ctx := context.Background()
	// Skip directory events for now - only process files
	if event.IsDir {
		if globalLogger != nil {
			globalLogger.Debug(ctx, "Skipping directory event", "operation", event.Operation, "path", event.Path)
		}
		return
	}

	// Get the default queue service
	queue := services.GetDefaultQueue()
	if queue == nil {
		if globalLogger != nil {
			globalLogger.Error(ctx, "Queue service not initialized")
		}
		return
	}

	// Create structured event data
	eventData := FileEventData{
		FilePath:  event.Path,
		EventType: event.Operation,
		Timestamp: event.Timestamp,
		IsDir:     event.IsDir,
	}

	// Marshal to JSON for queue storage
	jsonData, err := json.Marshal(eventData)
	if err != nil {
		if globalLogger != nil {
			globalLogger.Error(ctx, "Error marshaling event data", "error", err)
		}
		return
	}

	// Enqueue the file event (priority 1 for normal processing)
	item, err := queue.Enqueue(string(jsonData), 1)
	if err != nil {
		if globalLogger != nil {
			globalLogger.Error(ctx, "Error enqueuing file event", "error", err)
		}
		return
	}

	if globalLogger != nil {
		globalLogger.Info(ctx, "Queued file event", "operation", event.Operation, "path", event.Path, "queue_id", item.ID)
	}
}



// initializeDocumentsModule initializes the documents module with existing services
func initializeDocumentsModule(dm *services.DatabaseManager, logger logging.Logger) error {
	ctx := context.Background()
	logger.Info(ctx, "Initializing documents module")

	// Get existing services
	systemDB := dm.GetSystemDB()
	if systemDB == nil {
		return fmt.Errorf("system database not available")
	}

	// Create database adapter
	dbAdapter := NewDatabaseAdapter(systemDB)

	// Create documents config with defaults
	documentsConfig := documents.DefaultDocumentsConfig()
	documentsConfig.ChunkingConfig.MaxChunkSize = 500
	documentsConfig.ChunkingConfig.ChunkOverlap = 50
	documentsConfig.SearchConfig.MaxDocumentSize = 10000

	// Ollama configuration
	documentsConfig.OllamaURL = "http://ollama:11434"
	documentsConfig.OllamaTimeout = 60

	// Embedding configuration
	documentsConfig.EmbeddingModel = "embeddinggemma" // Use the same model as before
	documentsConfig.EmbeddingDimension = 768

	// Extraction configuration
	documentsConfig.ExtractionEnabled = true
	documentsConfig.ExtractionModel = "llama3:8b"

	// Graph database configuration
	documentsConfig.GraphEnabled = true
	documentsConfig.GraphConfig = map[string]interface{}{
		"type":    "cayley",
		"backend": "bolt",
		"path":    "./data/cayley.db",
	}

	documentsConfig.VectorStoreConfig = map[string]interface{}{
		"type":       "qdrant", // Use QDrant for persistent vector storage
		"host":       "qdrant",
		"port":       6334, // Use gRPC port for QDrant Go client
		"collection": "arcadia_vectors", // Use existing collection
		"dimension":  768, // Use same dimension as embedding model
	}
	documentsConfig.DocumentStoreConfig = map[string]interface{}{
		"type":       "sql",
		"table_name": "documents",
	}

	// Create dependencies for documents module
	deps := documents.Dependencies{
		DB:             dbAdapter,
		Logger:         logger, // Now uses pkg/logging.Logger directly via type alias
		Queue:          nil,    // No queue adapter for now
		Metrics:        nil,    // No metrics adapter for now
		Cache:          nil,    // No cache adapter for now
		RateLimiter:    nil,    // No rate limiter for now
		ConfigProvider: nil,    // No config provider for now
		EventBus:       nil,    // No event bus for now
		Config:         documentsConfig,
	}

	// Create documents module
	var err error
	documentsModule, err = documents.NewDocumentsModule(deps)
	if err != nil {
		return fmt.Errorf("failed to create documents module: %w", err)
	}

	logger.Info(ctx, "Documents module created successfully")
	return nil
}



// setupAIRoutes sets up the new AI API routes
func setupAIRoutes(aiMod ai.AIModule, logger logging.Logger) {
	ctx := context.Background()
	if aiMod != nil {
		// Set up dependency injection for AI handlers
		handlers.SetAIDependencies(aiMod)

		// New provider-agnostic endpoints
		http.HandleFunc("/api/ai/v2/chat", handlers.CorsHandler(handlers.HandleAIAPI))
		http.HandleFunc("/api/ai/provider/switch", handlers.CorsHandler(handlers.HandleProviderSwitch))
		http.HandleFunc("/api/ai/provider/status", handlers.CorsHandler(handlers.HandleProviderStatus))

		if logger != nil {
			logger.Info(ctx, "AI API endpoints registered",
				"endpoints", []string{
					"/api/ai/v2/chat - Provider-agnostic chat endpoint",
					"/api/ai/provider/switch - Switch AI providers",
					"/api/ai/provider/status - Get provider status",
				})
		}
	}

}

// --- Main ---

func main() {
	ctx := context.Background()

	// Initialize unified logging system
	logConfig := logging.Config{
		Level:       logging.LevelInfo,
		OutputPaths: []string{"stdout", "/workspace/appengine/logs/arcadia.log"},
		Format:      logging.FormatJSON,
		AddSource:   true,
		ModuleName:  "main",
	}

	mainLogger, err := logging.NewLogger(logConfig)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	mainLogger.Info(ctx, "Starting Arcadia App Engine")

	// Set global logger for use in global functions
	globalLogger = mainLogger.WithModule("filewatch")

	// Create module-specific loggers
	documentsLogger := mainLogger.WithModule("documents")
	aiLogger := mainLogger.WithModule("ai")
	servicesLogger := mainLogger.WithModule("services")
	handlersLogger := mainLogger.WithModule("handlers")
	configLogger := mainLogger.WithModule("config")
	queueLogger := mainLogger.WithModule("queue")

	// Initialize and load configuration
	configManager = config.NewManager(configLogger)
	if err := configManager.Load(); err != nil {
		mainLogger.Fatal(ctx, "Failed to load configuration", "error", err)
	}

	// Initialize databases with the new manager approach
	// This also initializes the default scheduler
	dm, err := services.InitDatabasesWithManager(servicesLogger.WithComponent("database"))
	if err != nil {
		mainLogger.Fatal(ctx, "Failed to initialize databases", "error", err)
	}
	defer dm.Close()

	// Initialize registry manager
	registryManager = services.NewRegistryManager(executeAppTool, servicesLogger.WithComponent("registry"))
	if err := registryManager.Load(); err != nil {
		mainLogger.Fatal(ctx, "Failed to load registry", "error", err)
	}

	// Initialize WASM runtime (will be initialized after AI module)

	// Initialize file watcher service
	fileWatcherConfig := services.DefaultFileWatcherConfig()
	var err2 error
	fileWatcher, err2 = services.NewFileWatcherService(dm.GetSystemDB(), fileWatcherConfig, servicesLogger.WithComponent("filewatcher"))
	if err2 != nil {
		mainLogger.Fatal(ctx, "Failed to initialize file watcher", "error", err2)
	}
	defer func() {
		if fws, ok := fileWatcher.(*services.FileWatcherService); ok {
			fws.Close()
		}
	}()

	// Register file event handler
	fileWatcher.RegisterEventHandler(handleFileEvent)

	// Start the file watcher service
	if err := fileWatcher.Start(); err != nil {
		mainLogger.Fatal(ctx, "Failed to start file watcher", "error", err)
	}
	defer fileWatcher.Stop()

	mainLogger.Info(ctx, "File watcher service initialized and started")

	// Legacy embedding service initialization removed - using documents module only

	// Initialize documents module
	if err := initializeDocumentsModule(dm, documentsLogger); err != nil {
		mainLogger.Warn(ctx, "Failed to initialize documents module", "error", err)
		mainLogger.Warn(ctx, "Documents module will not be available")
	} else {
		mainLogger.Info(ctx, "Documents module initialized successfully")
		// Start the documents module
		if err := documentsModule.Start(context.Background()); err != nil {
			mainLogger.Warn(ctx, "Failed to start documents module", "error", err)
		} else {
			mainLogger.Info(ctx, "Documents module started successfully")
		}
	}

	// Initialize and start queue processor for file processing
	queueService := services.GetDefaultQueue()

	mainLogger.Debug(ctx, "Queue service available", "available", queueService != nil)
	mainLogger.Debug(ctx, "Documents module available", "available", documentsModule != nil)

	if queueService != nil && documentsModule != nil {
		mainLogger.Debug(ctx, "Initializing queue processor")
		queueProcessor = newQueueProcessor(queueService, documentsModule, queueLogger)
		queueProcessor.start()
		defer queueProcessor.stop()
		mainLogger.Info(ctx, "Queue processor initialized and started")
	} else {
		mainLogger.Warn(ctx, "Queue processor not started", "queue_available", queueService != nil, "documents_available", documentsModule != nil)
	}

	// Initialize AI module with auto-configuration
	mainLogger.Info(ctx, "Initializing AI module")
	aiModule, err = ai.NewAIModuleFromEnv(ctx, aiLogger)
	if err != nil {
		mainLogger.Warn(ctx, "Failed to initialize AI module", "error", err)
		mainLogger.Warn(ctx, "AI endpoints will not be available")
	} else {
		mainLogger.Info(ctx, "AI module initialized successfully")
	}

	// Set up dependency injection for the AI module now that all components are available
	if aiModule != nil {
		var embeddingAdapter aiInterfaces.EmbeddingSearch
		if documentsModule != nil {
			mainLogger.Info(ctx, "Creating embedding search adapter with documents module")
			embeddingAdapter = NewEmbeddingSearchAdapter(documentsModule, aiLogger.WithComponent("embedding"))
			mainLogger.Info(ctx, "Embedding search adapter created successfully")
		} else {
			mainLogger.Warn(ctx, "Documents module is nil, skipping embedding adapter creation")
		}

		if err := ai.SetupModuleDependencies(
			aiModule,
			registryManager.GetRegistryAccess(),
			registryManager.GetAppRunner(),
			registryManager.GetAppCreator(),
			embeddingAdapter,
		); err != nil {
			mainLogger.Warn(ctx, "Failed to setup AI module dependencies", "error", err)
		} else {
			embeddingStatus := "disabled"
			if embeddingAdapter != nil {
				embeddingStatus = "enabled"
			}
			mainLogger.Info(ctx, "AI module dependencies configured successfully", "embedding_search", embeddingStatus)
		}
	}

	// Initialize WASM runtime with AI module
	mainLogger.Info(ctx, "Initializing WASM runtime")
	wasmRuntime = services.NewWasmRuntime(registryManager, dm, aiModule, servicesLogger.WithComponent("wasm"))
	mainLogger.Info(ctx, "WASM runtime initialized successfully")

	// Start the scheduler (now properly initialized)
	if err := services.StartScheduler(); err != nil {
		mainLogger.Fatal(ctx, "Failed to start scheduler", "error", err)
	}
	defer services.StopScheduler()

	// Set up dependency injection for app tool execution
	services.SetExecuteAppTool(executeAppTool)

	// Create app log function for backward compatibility with handlers
	// This wraps the structured logger to provide Printf-style logging
	appLogFunc := func(format string, args ...interface{}) {
		mainLogger.Info(ctx, fmt.Sprintf(format, args...))
	}

	// Set up handlers with dependency injection
	handlers.SetLogger(handlersLogger)
	handlers.SetAppDependencies(registryManager, wasmRuntime.GetEngine(), appLogFunc, executeAppTool)
	handlers.SetScheduleDependencies(appLogFunc, registryManager)
	handlers.SetFileWatcherDependencies(fileWatcher)

	// Use CORS middleware from handlers package

	// File watcher endpoints
	http.HandleFunc("/filewatcher/add", handlers.CorsHandler(handlers.AddWatchDirHandler))
	http.HandleFunc("/filewatcher/remove", handlers.CorsHandler(handlers.RemoveWatchDirHandler))
	http.HandleFunc("/filewatcher/list", handlers.CorsHandler(handlers.ListWatchedDirsHandler))

	// App management endpoints
	http.HandleFunc("/list_apps", handlers.CorsHandler(handlers.ListAppsHandler))
	http.HandleFunc("/run_tool", handlers.CorsHandler(handlers.RunToolHandler))
	http.HandleFunc("/submit_app_src", handlers.CorsHandler(handlers.SubmitAppSrcHandler))

	// Schedule management endpoints
	http.HandleFunc("/schedule_app_run", handlers.CorsHandler(handlers.ScheduleAppRunHandler))
	http.HandleFunc("/list_schedules", handlers.CorsHandler(handlers.ListSchedulesHandler))
	http.HandleFunc("/get_schedule", handlers.CorsHandler(handlers.GetScheduleHandler))
	http.HandleFunc("/delete_schedule", handlers.CorsHandler(handlers.DeleteScheduleHandler))
	http.HandleFunc("/update_schedule", handlers.CorsHandler(handlers.UpdateScheduleHandler))
	http.HandleFunc("/list_scheduled_runs", handlers.CorsHandler(handlers.ListScheduledRunsHandler))

	// AI Integration endpoints - use new system
	setupAIRoutes(aiModule, mainLogger)


	// Set up graceful shutdown
	server := &http.Server{
		Addr: ":" + configManager.GetServerPort(),
	}

	// Channel to listen for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		port := configManager.GetServerPort()
		mainLogger.Info(ctx, "Starting Arcadia App Engine server", "port", port)
		mainLogger.Info(ctx, "Available endpoints",
			"endpoints", []string{
				"/list_apps - List all registered apps",
				"/run_tool - Execute a tool from an app",
				"/submit_app_src - Submit new app source code",
				"/schedule_app_run - Schedule app runs (one-time or recurring)",
				"/list_schedules - List all schedules",
				"/get_schedule?id=<id> - Get specific schedule",
				"/delete_schedule?id=<id> - Delete a schedule",
				"/update_schedule?id=<id> - Update a schedule",
				"/list_scheduled_runs[?schedule_id=<id>] - List scheduled runs",
				"/api/ai/v2/chat - Provider-agnostic AI chat",
				"/api/ai/provider/switch - Switch AI provider",
				"/api/ai/provider/status - Get current provider status",
				"/filewatcher/add - Add directory to file watcher",
				"/filewatcher/remove - Remove directory from file watcher",
				"/filewatcher/list - List all watched directories",
			})

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			mainLogger.Fatal(ctx, "Server failed to start", "error", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	mainLogger.Info(ctx, "Shutting down server")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()


	// Stop documents module
	if documentsModule != nil {
		mainLogger.Info(ctx, "Stopping documents module")
		if err := documentsModule.Stop(shutdownCtx); err != nil {
			mainLogger.Error(ctx, "Error stopping documents module", "error", err)
		} else {
			mainLogger.Info(ctx, "Documents module stopped successfully")
		}
	}

	// Stop AI module
	if aiModule != nil {
		mainLogger.Info(ctx, "Stopping AI module")
		if err := aiModule.Stop(shutdownCtx); err != nil {
			mainLogger.Error(ctx, "Error stopping AI module", "error", err)
		} else {
			mainLogger.Info(ctx, "AI module stopped successfully")
		}
	}

	// Stop queue processor
	if queueProcessor != nil {
		mainLogger.Info(ctx, "Stopping queue processor")
		queueProcessor.stop()
		mainLogger.Info(ctx, "Queue processor stopped successfully")
	}

	// Stop scheduler
	mainLogger.Info(ctx, "Stopping scheduler")
	services.StopScheduler()

	// Stop file watcher
	if fileWatcher != nil {
		mainLogger.Info(ctx, "Stopping file watcher")
		fileWatcher.Stop()
	}

	// Close database manager
	if dm != nil {
		mainLogger.Info(ctx, "Closing database connections")
		dm.Close()
	}

	// Shutdown HTTP server
	mainLogger.Info(ctx, "Shutting down HTTP server")
	if err := server.Shutdown(shutdownCtx); err != nil {
		mainLogger.Error(ctx, "Error during server shutdown", "error", err)
	}

	mainLogger.Info(ctx, "Server gracefully stopped")
}

