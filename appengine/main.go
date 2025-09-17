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


// --- Main ---

func main() {
	// Initialize logging
	if err := services.InitializeAppLogger(); err != nil {
		log.Fatalf("Failed to initialize app logger: %v", err)
	}
	defer services.CloseAppLogger()

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

	// Set up dependency injection for Claude service
	configManager.SetupDependencyInjection(
		registryManager.GetRegistryAccess(),
		registryManager.GetAppRunner(),
		registryManager.GetAppCreator(),
	)


	// Add embedding search capability to Claude service
	embeddingService = services.GetDefaultEmbeddingService()
	if embeddingService != nil {
		services.SetEmbeddingSearch(embeddingService)
		log.Println("RAG capabilities enabled - Claude can now search and retrieve documents")
	} else {
		log.Printf("WARNING: Embedding service not available - RAG tools will not function")
	}

	// Trigger initial Claude tool refresh now that registry access is set up
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

	// AI Integration endpoints
	claudeService := configManager.GetClaudeService()
	http.HandleFunc("/claude", handlers.CorsHandler(claudeService.HandleClaudeAPI))


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
	log.Printf("  /claude - Send message to Claude AI (POST {\"message\": \"your message\"})")
	log.Printf("  /filewatcher/add - Add directory to file watcher (POST {\"path\": \"/path/to/watch\"})")
	log.Printf("  /filewatcher/remove - Remove directory from file watcher (POST {\"path\": \"/path/to/remove\"})")
	log.Printf("  /filewatcher/list - List all watched directories (GET)")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

