package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"arcadia/config"
	"arcadia/handlers"
	"arcadia/services"
	"arcadia/services/ai"
	_ "arcadia/services/ai/providers/claude" // Import to register Claude provider
	_ "arcadia/services/ai/providers/openai" // Import to register OpenAI provider
)

var (
	// Configuration and registry managers
	configManager   *config.Manager
	registryManager *services.RegistryManager
	wasmRuntime     *services.WasmRuntime
	fileWatcher     services.FileWatcher
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
	queue       *services.QueueService
	embedding   services.EmbeddingServiceInterface
	workers     int
	stopChannel chan bool
	running     bool
}


func executeAppTool(appID, toolName string, input json.RawMessage) (string, error) {
	return wasmRuntime.ExecuteAppTool(appID, toolName, input)
}

// newQueueProcessor creates a new queue processor instance with single worker
func newQueueProcessor(queue *services.QueueService, embedding services.EmbeddingServiceInterface) *QueueProcessor {
	return &QueueProcessor{
		queue:       queue,
		embedding:   embedding,
		workers:     1, // Single worker for Ollama compatibility
		stopChannel: make(chan bool, 1),
		running:     false,
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
	if qp.embedding == nil {
		return fmt.Errorf("embedding service not available")
	}
	
	// Use the embedding service to process the file
	doc, err := qp.embedding.ProcessFile(filePath)
	if err != nil {
		return fmt.Errorf("embedding processing failed: %v", err)
	}
	
	log.Printf("[QueueProcessor] Embedded file %s: %d chunks", 
		filePath, doc.ChunkCount)
		
	return nil
}

// EmbeddingSearchAdapter adapts services.EmbeddingServiceInterface to ai.EmbeddingSearch
type EmbeddingSearchAdapter struct {
	service services.EmbeddingServiceInterface
}

func NewEmbeddingSearchAdapter(service services.EmbeddingServiceInterface) *EmbeddingSearchAdapter {
	return &EmbeddingSearchAdapter{service: service}
}

func (esa *EmbeddingSearchAdapter) SearchDocuments(query string, topK int) ([]*ai.DocumentSearchResult, error) {
	// Check if embedding service is available
	if esa.service == nil {
		return nil, fmt.Errorf("embedding service not available")
	}

	results, err := esa.service.SearchDocuments(query, topK)
	if err != nil {
		return nil, err
	}

	// Convert services.DocumentSearchResult to ai.DocumentSearchResult
	aiResults := make([]*ai.DocumentSearchResult, 0, len(results))
	for i, result := range results {
		// Skip results with nil documents to prevent crashes
		if result.Document == nil {
			log.Printf("[EmbeddingSearchAdapter] Warning: Skipping result %d with nil document", i)
			continue
		}

		aiResults = append(aiResults, &ai.DocumentSearchResult{
			Document: &ai.Document{
				ID:         result.Document.ID,
				FilePath:   result.Document.FilePath,
				FileHash:   result.Document.FileHash,
				Content:    result.Document.Content,
				ChunkCount: result.Document.ChunkCount,
				Metadata:   result.Document.Metadata,
				CreatedAt:  result.Document.CreatedAt,
				UpdatedAt:  result.Document.UpdatedAt,
			},
			Chunks: func() []*ai.ChunkResult {
				chunks := make([]*ai.ChunkResult, len(result.Chunks))
				for j, chunk := range result.Chunks {
					chunks[j] = &ai.ChunkResult{
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

	return aiResults, nil
}

func (esa *EmbeddingSearchAdapter) SearchDocumentsEnhanced(query string, topK int, config ai.SearchConfig) ([]*ai.EnhancedDocumentSearchResult, error) {
	// Check if embedding service is available
	if esa.service == nil {
		return nil, fmt.Errorf("embedding service not available")
	}

	// Convert ai.SearchConfig (empty interface) to services.SearchConfig (struct)
	// Since ai.SearchConfig is an empty interface, we'll use the default services config
	servicesConfig := services.DefaultSearchConfig()
	results, err := esa.service.SearchDocumentsEnhanced(query, topK, servicesConfig)
	if err != nil {
		return nil, err
	}

	// Convert services.EnhancedDocumentSearchResult to ai.EnhancedDocumentSearchResult
	aiResults := make([]*ai.EnhancedDocumentSearchResult, 0, len(results))
	for i, result := range results {
		// Skip results with nil documents to prevent crashes
		if result.Document == nil {
			log.Printf("[EmbeddingSearchAdapter] Warning: Skipping enhanced result %d with nil document", i)
			continue
		}

		aiResults = append(aiResults, &ai.EnhancedDocumentSearchResult{
			Document: &ai.Document{
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

func (esa *EmbeddingSearchAdapter) GetDocument(documentID string) (*ai.Document, error) {
	doc, err := esa.service.GetDocument(documentID)
	if err != nil {
		return nil, err
	}

	if doc == nil {
		return nil, nil
	}

	// Convert services.Document to ai.Document
	return &ai.Document{
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



// setupAIRoutes sets up the new AI API routes
func setupAIRoutes() {
	// Get the global AI service
	service := ai.GetGlobalService()

	if service != nil {
		// Create HTTP handler for the new AI service
		handler := ai.NewHTTPHandler(service)

		// New provider-agnostic endpoints
		http.HandleFunc("/api/ai/v2/chat", handlers.CorsHandler(handler.HandleAIAPI))
		http.HandleFunc("/api/ai/provider/switch", handlers.CorsHandler(handler.HandleProviderSwitch))
		http.HandleFunc("/api/ai/provider/status", handlers.CorsHandler(handler.HandleProviderStatus))

		log.Printf("New AI API endpoints registered:")
		log.Printf("  /api/ai/v2/chat - Provider-agnostic chat endpoint")
		log.Printf("  /api/ai/provider/switch - Switch AI providers")
		log.Printf("  /api/ai/provider/status - Get provider status")
	}

	// Backward compatibility endpoint using the compatibility layer
	http.HandleFunc("/claude", handlers.CorsHandler(ai.HandleClaudeAPI))
	log.Printf("Backward compatibility endpoint:")
	log.Printf("  /claude - Legacy Claude API endpoint (now provider-agnostic)")
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

	// Initialize WASM runtime
	wasmRuntime = services.NewWasmRuntime(configManager, registryManager, dm)

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

	// Initialize embedding service for file processing
	log.Printf("[DEBUG] Initializing embedding service...")
	chunker := services.NewSimpleTextChunker()
	model := services.NewOllamaEmbeddingModel("", "") // Use defaults: localhost:11434, embeddinggemma
	vectorStore := services.GetVectorStore()
	documentStore := services.GetDocumentStore()
	embeddingConfig := services.DefaultChunkingConfig()
	
	if vectorStore != nil && documentStore != nil {
		services.InitDefaultEmbeddingService(chunker, model, vectorStore, documentStore, embeddingConfig)
		
		// Initialize the model
		embeddingService := services.GetDefaultEmbeddingService()
		if embeddingService != nil {
			if err := embeddingService.Initialize(); err != nil {
				log.Printf("WARNING: Failed to initialize embedding service: %v", err)
			} else {
				log.Println("Embedding service initialized successfully")
			}
		}
	} else {
		log.Printf("WARNING: Cannot initialize embedding service - vectorStore: %v, documentStore: %v", 
			vectorStore != nil, documentStore != nil)
	}

	// Initialize and start queue processor for file processing
	queueService := services.GetDefaultQueue()
	embeddingService := services.GetDefaultEmbeddingService()
	
	log.Printf("[DEBUG] Queue service available: %v", queueService != nil)
	log.Printf("[DEBUG] Embedding service available: %v", embeddingService != nil)
	
	if queueService != nil && embeddingService != nil {
		log.Printf("[DEBUG] Initializing queue processor...")
		queueProcessor := newQueueProcessor(queueService, embeddingService)
		queueProcessor.start()
		defer queueProcessor.stop()
		log.Println("Queue processor initialized and started")
	} else {
		log.Printf("WARNING: Queue processor not started - queue: %v, embedding: %v", 
			queueService != nil, embeddingService != nil)
	}

	// Initialize AI service with auto-configuration
	log.Printf("Initializing AI service...")
	if err := ai.InitializeGlobalServiceFromConfigWithFallback(func() *ai.AIConfig {
		// Fallback to legacy Claude configuration from config manager
		legacyConfig := configManager.GetConfig()
		return &ai.AIConfig{
			Provider:           "claude",
			MaxTokens:          legacyConfig.Claude.MaxTokens,
			TimeoutSeconds:     legacyConfig.Claude.TimeoutSeconds,
			MaxContextMessages: legacyConfig.Claude.MaxContextMessages,
			ContextCompaction:  legacyConfig.Claude.ContextCompaction,
			ContextTTLMinutes:  legacyConfig.Claude.ContextTTLMinutes,
			EnableMCP:          legacyConfig.Claude.EnableMCP,
			MCPServerCmd:       legacyConfig.Claude.MCPServerCmd,
			ProviderSettings: map[string]any{
				"api_key":  legacyConfig.Claude.APIKey,
				"base_url": legacyConfig.Claude.BaseURL,
				"model":    legacyConfig.Claude.Model,
			},
		}
	}); err != nil {
		log.Printf("Warning: Failed to initialize AI service: %v", err)
		log.Printf("AI endpoints will not be available")
	} else {
		log.Printf("AI service initialized successfully")
	}

	// Set up dependency injection for legacy Claude service
	configManager.SetupDependencyInjection(
		registryManager.GetRegistryAccess(),
		registryManager.GetAppRunner(),
		registryManager.GetAppCreator(),
	)

	// Add embedding search capability to AI service
	embeddingService = services.GetDefaultEmbeddingService()
	if embeddingService != nil {
		services.SetEmbeddingSearch(embeddingService)
		log.Println("RAG capabilities enabled - AI service can now search and retrieve documents")
	} else {
		log.Printf("WARNING: Embedding service not available - RAG tools will not function")
	}

	// Set up dependency injection for the new AI service now that all components are available
	if ai.IsGlobalServiceInitialized() {
		var embeddingAdapter ai.EmbeddingSearch
		if embeddingService != nil {
			log.Printf("Creating embedding search adapter...")
			embeddingAdapter = NewEmbeddingSearchAdapter(embeddingService)
			log.Printf("Embedding search adapter created successfully")
		} else {
			log.Printf("WARNING: Embedding service is nil, skipping embedding adapter creation")
		}

		if err := ai.SetupGlobalDependencies(
			registryManager.GetRegistryAccess(),
			registryManager.GetAppRunner(),
			registryManager.GetAppCreator(),
			embeddingAdapter,
		); err != nil {
			log.Printf("Warning: Failed to setup AI dependencies: %v", err)
		} else {
			embeddingStatus := "disabled"
			if embeddingAdapter != nil {
				embeddingStatus = "enabled"
			}
			log.Printf("AI service dependencies configured successfully (embedding search: %s)", embeddingStatus)
		}
	}

	// Trigger initial tool refresh now that registry access is set up
	services.TriggerClaudeToolRefresh()

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
	setupAIRoutes()


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
	log.Printf("  /claude - Legacy Claude AI endpoint (POST {\"message\": \"your message\"})")
	log.Printf("  /api/ai/v2/chat - Provider-agnostic AI chat (POST {\"message\": \"...\", \"session_id\": \"...\"})")
	log.Printf("  /api/ai/provider/switch - Switch AI provider (POST {\"provider\": \"...\", \"api_key\": \"...\"})")
	log.Printf("  /api/ai/provider/status - Get current provider status (GET)")
	log.Printf("  /filewatcher/add - Add directory to file watcher (POST {\"path\": \"/path/to/watch\"})")
	log.Printf("  /filewatcher/remove - Remove directory from file watcher (POST {\"path\": \"/path/to/remove\"})")
	log.Printf("  /filewatcher/list - List all watched directories (GET)")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

