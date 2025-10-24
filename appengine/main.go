package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"arcadia/config"
	"arcadia/handlers"
	"arcadia/modules/documents"
	docInterfaces "arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
	"arcadia/services"
	"arcadia/modules/ai"
	aiInterfaces "arcadia/modules/ai/interfaces"
	aiModels "arcadia/modules/ai/models"
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


// SimpleLogger adapts Go's standard log to the documents module interface
type SimpleLogger struct{}

func NewSimpleLogger() *SimpleLogger {
	return &SimpleLogger{}
}

func (sl *SimpleLogger) Debug(ctx context.Context, msg string, fields ...interface{}) {
	log.Printf("[DEBUG] %s %v", msg, fields)
}

func (sl *SimpleLogger) Info(ctx context.Context, msg string, fields ...interface{}) {
	log.Printf("[INFO] %s %v", msg, fields)
}

func (sl *SimpleLogger) Warn(ctx context.Context, msg string, fields ...interface{}) {
	log.Printf("[WARN] %s %v", msg, fields)
}

func (sl *SimpleLogger) Error(ctx context.Context, msg string, fields ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, fields)
}

func (sl *SimpleLogger) Fatal(ctx context.Context, msg string, fields ...interface{}) {
	log.Fatalf("[FATAL] %s %v", msg, fields)
}

func (sl *SimpleLogger) WithFields(fields map[string]interface{}) docInterfaces.Logger {
	return sl // For simplicity, return self
}

func (sl *SimpleLogger) WithContext(ctx context.Context) docInterfaces.Logger {
	return sl // For simplicity, return self
}


func executeAppTool(appID, toolName string, input json.RawMessage) (string, error) {
	return wasmRuntime.ExecuteAppTool(appID, toolName, input)
}

// newQueueProcessor creates a new queue processor instance with single worker
func newQueueProcessor(queue *services.QueueService, documentsModule documents.DocumentsModule) *QueueProcessor {
	return &QueueProcessor{
		queue:           queue,
		documentsModule: documentsModule,
		workers:         1, // Single worker for Ollama compatibility
		stopChannel:     make(chan bool, 1),
		running:         false,
	}
}

// start begins the queue processing with a single background goroutine
func (qp *QueueProcessor) start() {
	if qp.running {
		log.Printf("[QueueProcessor] Already running")
		return
	}
	
	qp.running = true
	log.Printf("[QueueProcessor] Starting single worker")
	
	// Start single worker goroutine
	go qp.processQueueItems()
}

// stop gracefully stops the queue processor
func (qp *QueueProcessor) stop() {
	if !qp.running {
		return
	}
	
	log.Printf("[QueueProcessor] Stopping...")
	qp.running = false
	
	// Signal worker to stop
	select {
	case qp.stopChannel <- true:
	default:
		// Channel might be full, that's ok
	}
	
	log.Printf("[QueueProcessor] Stopped")
}

// processQueueItems is the main worker loop for processing queue items
func (qp *QueueProcessor) processQueueItems() {
	log.Printf("[QueueProcessor] Worker started")
	
	for qp.running {
		select {
		case <-qp.stopChannel:
			log.Printf("[QueueProcessor] Worker stopping")
			return
		default:
			// Try to get next item from queue
			item, err := qp.queue.Dequeue()
			if err != nil {
				// Log error and continue
				log.Printf("[QueueProcessor] Queue error: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			
			if item == nil {
				// No items in queue, wait before trying again
				time.Sleep(1 * time.Second)
				continue
			}
			
			// Process the queue item
			log.Printf("[QueueProcessor] Processing item %s", item.ID)
			qp.processFileEvent(item)
		}
	}
	
	log.Printf("[QueueProcessor] Worker finished")
}

// processFileEvent processes a single file event from the queue
func (qp *QueueProcessor) processFileEvent(item *services.QueueItem) {
	// Parse the event data
	var eventData FileEventData
	if err := json.Unmarshal([]byte(item.Data), &eventData); err != nil {
		log.Printf("[QueueProcessor] Failed to parse event data: %v", err)
		qp.queue.MarkFailed(item.ID, fmt.Sprintf("Failed to parse event data: %v", err))
		return
	}
	
	log.Printf("[QueueProcessor] Processing %s event for file: %s", 
		eventData.EventType, eventData.FilePath)
	
	switch eventData.EventType {
	case "create", "modify":
		// Process file for embedding
		err := qp.processFileForEmbedding(eventData.FilePath)
		if err != nil {
			log.Printf("[QueueProcessor] Failed to process file %s: %v", 
				eventData.FilePath, err)
			qp.queue.MarkFailed(item.ID, fmt.Sprintf("Processing failed: %v", err))
			return
		}
		
		log.Printf("[QueueProcessor] Successfully processed file: %s", eventData.FilePath)
		qp.queue.MarkCompleted(item.ID)
		
	case "delete":
		// For now, just mark as completed
		// TODO: Implement vector cleanup for deleted files
		log.Printf("[QueueProcessor] Handling delete event for: %s (cleanup not implemented)", 
			eventData.FilePath)
		qp.queue.MarkCompleted(item.ID)
		
	default:
		log.Printf("[QueueProcessor] Unknown event type: %s", eventData.EventType)
		qp.queue.MarkCompleted(item.ID)
	}
}

// processFileForEmbedding sends a file to the embedding service for processing
func (qp *QueueProcessor) processFileForEmbedding(filePath string) error {
	// Use documents module only - no fallback to legacy services
	if qp.documentsModule == nil {
		return fmt.Errorf("documents module not available")
	}

	log.Printf("[QueueProcessor] Processing file with documents module: %s", filePath)

	ctx := context.Background()
	result, err := qp.documentsModule.ProcessFile(ctx, filePath)
	if err != nil {
		return fmt.Errorf("documents module processing failed: %v", err)
	}

	if result != nil && result.Success {
		log.Printf("[QueueProcessor] Documents module processed file %s: document ID %s",
			filePath, result.DocumentID)
	} else {
		log.Printf("[QueueProcessor] Documents module processing failed for %s: %v",
			filePath, result.Error)
	}

	return nil
}


// EmbeddingSearchAdapter adapts documents module to ai.EmbeddingSearch
type EmbeddingSearchAdapter struct {
	documentsModule documents.DocumentsModule
}

func NewEmbeddingSearchAdapter(documentsModule documents.DocumentsModule) *EmbeddingSearchAdapter {
	return &EmbeddingSearchAdapter{
		documentsModule: documentsModule,
	}
}

func (esa *EmbeddingSearchAdapter) SearchDocuments(query string, topK int) ([]*aiModels.DocumentSearchResult, error) {
	// Use documents module only - no fallback to legacy services
	if esa.documentsModule == nil {
		return nil, fmt.Errorf("documents module not available")
	}

	log.Printf("[EmbeddingSearchAdapter] Searching with documents module: %s", query)

	ctx := context.Background()
	docResults, err := esa.documentsModule.SearchDocuments(ctx, query, topK)
	if err != nil {
		return nil, fmt.Errorf("documents module search failed: %v", err)
	}

	// Convert documents module results to AI service format
	return esa.convertDocumentSearchResults(docResults), nil
}


// convertDocumentSearchResults converts documents module results to AI service format
func (esa *EmbeddingSearchAdapter) convertDocumentSearchResults(docResults []*models.DocumentSearchResult) []*aiModels.DocumentSearchResult {
	aiResults := make([]*aiModels.DocumentSearchResult, 0, len(docResults))
	for i, result := range docResults {
		// Skip results with nil documents to prevent crashes
		if result.Document == nil {
			log.Printf("[EmbeddingSearchAdapter] Warning: Skipping documents module result %d with nil document", i)
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
	// Use documents module only - no fallback to legacy services
	if esa.documentsModule == nil {
		return nil, fmt.Errorf("documents module not available")
	}

	ctx := context.Background()

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
			log.Printf("[EmbeddingSearchAdapter] Warning: Skipping enhanced result %d with nil document", i)
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
	// Skip directory events for now - only process files
	if event.IsDir {
		log.Printf("[FileWatcher] Skipping directory event: %s %s", event.Operation, event.Path)
		return
	}

	// Get the default queue service
	queue := services.GetDefaultQueue()
	if queue == nil {
		log.Printf("[FileWatcher] Error: Queue service not initialized")
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
		log.Printf("[FileWatcher] Error marshaling event data: %v", err)
		return
	}

	// Enqueue the file event (priority 1 for normal processing)
	item, err := queue.Enqueue(string(jsonData), 1)
	if err != nil {
		log.Printf("[FileWatcher] Error enqueuing file event: %v", err)
		return
	}

	log.Printf("[FileWatcher] Queued file event: %s %s (queue ID: %s)", 
		event.Operation, event.Path, item.ID)
}



// initializeDocumentsModule initializes the documents module with existing services
func initializeDocumentsModule(dm *services.DatabaseManager) error {
	log.Printf("Initializing documents module...")

	// Get existing services
	systemDB := dm.GetSystemDB()
	if systemDB == nil {
		return fmt.Errorf("system database not available")
	}

	// Create database adapter
	dbAdapter := NewDatabaseAdapter(systemDB)

	// Create logger adapter
	logger := NewSimpleLogger()

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
		Logger:         logger,
		Queue:          nil, // No queue adapter for now
		Metrics:        nil, // No metrics adapter for now
		Cache:          nil, // No cache adapter for now
		RateLimiter:    nil, // No rate limiter for now
		ConfigProvider: nil, // No config provider for now
		EventBus:       nil, // No event bus for now
		Config:         documentsConfig,
	}

	// Create documents module
	var err error
	documentsModule, err = documents.NewDocumentsModule(deps)
	if err != nil {
		return fmt.Errorf("failed to create documents module: %w", err)
	}

	log.Printf("Documents module created successfully")
	return nil
}



// setupAIRoutes sets up the new AI API routes
func setupAIRoutes(aiMod ai.AIModule) {
	if aiMod != nil {
		// Set up dependency injection for AI handlers
		handlers.SetAIDependencies(aiMod)

		// New provider-agnostic endpoints
		http.HandleFunc("/api/ai/v2/chat", handlers.CorsHandler(handlers.HandleAIAPI))
		http.HandleFunc("/api/ai/provider/switch", handlers.CorsHandler(handlers.HandleProviderSwitch))
		http.HandleFunc("/api/ai/provider/status", handlers.CorsHandler(handlers.HandleProviderStatus))

		log.Printf("New AI API endpoints registered:")
		log.Printf("  /api/ai/v2/chat - Provider-agnostic chat endpoint")
		log.Printf("  /api/ai/provider/switch - Switch AI providers")
		log.Printf("  /api/ai/provider/status - Get provider status")
	}

}

// --- Main ---

func main() {
	// Initialize comprehensive logging (both general and app submission logging)
	if err := services.InitializeAllLoggers(); err != nil {
		log.Fatalf("Failed to initialize loggers: %v", err)
	}
	defer services.CloseAllLoggers()

	// Initialize and load configuration
	configManager = config.NewManager()
	if err := configManager.Load(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize databases with the new manager approach
	// This also initializes the default scheduler
	dm, err := services.InitDatabasesWithManager()
	if err != nil {
		log.Fatalf("Failed to initialize databases: %v", err)
	}
	defer dm.Close()

	// Initialize registry manager
	registryManager = services.NewRegistryManager(executeAppTool)
	if err := registryManager.Load(); err != nil {
		log.Fatalf("Failed to load registry: %v", err)
	}

	// Initialize WASM runtime (will be initialized after AI module)

	// Initialize file watcher service
	fileWatcherConfig := services.DefaultFileWatcherConfig()
	var err2 error
	fileWatcher, err2 = services.NewFileWatcherService(dm.GetSystemDB(), fileWatcherConfig)
	if err2 != nil {
		log.Fatalf("Failed to initialize file watcher: %v", err2)
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
		log.Fatalf("Failed to start file watcher: %v", err)
	}
	defer fileWatcher.Stop()
	
	log.Println("File watcher service initialized and started")

	// Legacy embedding service initialization removed - using documents module only

	// Initialize documents module
	if err := initializeDocumentsModule(dm); err != nil {
		log.Printf("WARNING: Failed to initialize documents module: %v", err)
		log.Printf("Documents module will not be available")
	} else {
		log.Printf("Documents module initialized successfully")
		// Start the documents module
		if err := documentsModule.Start(context.Background()); err != nil {
			log.Printf("WARNING: Failed to start documents module: %v", err)
		} else {
			log.Printf("Documents module started successfully")
		}
	}

	// Initialize and start queue processor for file processing
	queueService := services.GetDefaultQueue()

	log.Printf("[DEBUG] Queue service available: %v", queueService != nil)
	log.Printf("[DEBUG] Documents module available: %v", documentsModule != nil)

	if queueService != nil && documentsModule != nil {
		log.Printf("[DEBUG] Initializing queue processor...")
		queueProcessor = newQueueProcessor(queueService, documentsModule)
		queueProcessor.start()
		defer queueProcessor.stop()
		log.Println("Queue processor initialized and started")
	} else {
		log.Printf("WARNING: Queue processor not started - queue: %v, documents: %v",
			queueService != nil, documentsModule != nil)
	}

	// Initialize AI module with auto-configuration
	log.Printf("Initializing AI module...")
	ctx := context.Background()
	aiModule, err = ai.NewAIModuleFromEnv(ctx)
	if err != nil {
		log.Printf("Warning: Failed to initialize AI module: %v", err)
		log.Printf("AI endpoints will not be available")
	} else {
		log.Printf("AI module initialized successfully")
	}

	// Set up dependency injection for the AI module now that all components are available
	if aiModule != nil {
		var embeddingAdapter aiInterfaces.EmbeddingSearch
		if documentsModule != nil {
			log.Printf("Creating embedding search adapter with documents module...")
			embeddingAdapter = NewEmbeddingSearchAdapter(documentsModule)
			log.Printf("Embedding search adapter created successfully")
		} else {
			log.Printf("WARNING: Documents module is nil, skipping embedding adapter creation")
		}

		if err := ai.SetupModuleDependencies(
			aiModule,
			registryManager.GetRegistryAccess(),
			registryManager.GetAppRunner(),
			registryManager.GetAppCreator(),
			embeddingAdapter,
		); err != nil {
			log.Printf("Warning: Failed to setup AI module dependencies: %v", err)
		} else {
			embeddingStatus := "disabled"
			if embeddingAdapter != nil {
				embeddingStatus = "enabled"
			}
			log.Printf("AI module dependencies configured successfully (embedding search: %s)", embeddingStatus)
		}
	}

	// Initialize WASM runtime with AI module
	log.Printf("Initializing WASM runtime...")
	wasmRuntime = services.NewWasmRuntime(registryManager, dm, aiModule)
	log.Printf("WASM runtime initialized successfully")

	// Start the scheduler (now properly initialized)
	if err := services.StartScheduler(); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}
	defer services.StopScheduler()

	// Set up dependency injection for app tool execution
	services.SetExecuteAppTool(executeAppTool)

	// Set up handlers with dependency injection
	handlers.SetAppDependencies(registryManager, wasmRuntime.GetEngine(), services.GetAppLogFunc(), executeAppTool)
	handlers.SetScheduleDependencies(services.GetAppLogFunc(), registryManager)
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
	setupAIRoutes(aiModule)


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
		log.Printf("Starting Arcadia App Engine server on port %s...", port)
		log.Printf("Available endpoints:")
		log.Printf("  /list_apps - List all registered apps")
		log.Printf("  /run_tool - Execute a tool from an app")
		log.Printf("  /submit_app_src - Submit new app source code")
		log.Printf("  /schedule_app_run - Schedule app runs (one-time or recurring)")
		log.Printf("  /list_schedules - List all schedules")
		log.Printf("  /get_schedule?id=<id> - Get specific schedule")
		log.Printf("  /delete_schedule?id=<id> - Delete a schedule")
		log.Printf("  /update_schedule?id=<id> - Update a schedule")
		log.Printf("  /list_scheduled_runs[?schedule_id=<id>] - List scheduled runs")
		log.Printf("  /api/ai/v2/chat - Provider-agnostic AI chat (POST {\"message\": \"...\", \"session_id\": \"...\"})")
		log.Printf("  /api/ai/provider/switch - Switch AI provider (POST {\"provider\": \"...\", \"api_key\": \"...\"})")
		log.Printf("  /api/ai/provider/status - Get current provider status (GET)")
		log.Printf("  /filewatcher/add - Add directory to file watcher (POST {\"path\": \"/path/to/watch\"})")
		log.Printf("  /filewatcher/remove - Remove directory from file watcher (POST {\"path\": \"/path/to/remove\"})")
		log.Printf("  /filewatcher/list - List all watched directories (GET)")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()


	// Stop documents module
	if documentsModule != nil {
		log.Printf("Stopping documents module...")
		if err := documentsModule.Stop(ctx); err != nil {
			log.Printf("Error stopping documents module: %v", err)
		} else {
			log.Printf("Documents module stopped successfully")
		}
	}

	// Stop AI module
	if aiModule != nil {
		log.Printf("Stopping AI module...")
		if err := aiModule.Stop(ctx); err != nil {
			log.Printf("Error stopping AI module: %v", err)
		} else {
			log.Printf("AI module stopped successfully")
		}
	}

	// Stop queue processor
	if queueProcessor != nil {
		log.Printf("Stopping queue processor...")
		queueProcessor.stop()
		log.Printf("Queue processor stopped successfully")
	}

	// Stop scheduler
	log.Printf("Stopping scheduler...")
	services.StopScheduler()

	// Stop file watcher
	if fileWatcher != nil {
		log.Printf("Stopping file watcher...")
		fileWatcher.Stop()
	}

	// Close database manager
	if dm != nil {
		log.Printf("Closing database connections...")
		dm.Close()
	}

	// Close loggers
	log.Printf("Closing loggers...")
	services.CloseAllLoggers()

	// Shutdown HTTP server
	log.Printf("Shutting down HTTP server...")
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}

	log.Println("Server gracefully stopped")
}

