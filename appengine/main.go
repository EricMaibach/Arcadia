package main

import (
	"encoding/json"
	"log"
	"net/http"

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
)

// --- Request Types ---

type AppRequest struct {
	AppID   string              `json:"appId"`
	Version string              `json:"version"`
	Runtime string              `json:"runtime"`
	Tools   []services.ToolInfo `json:"tools"`
	AppSrc  string              `json:"appSrc"`
}

func executeAppTool(appID, toolName string, input json.RawMessage) (string, error) {
	return wasmRuntime.ExecuteAppTool(appID, toolName, input)
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

	// Set up dependency injection for Claude service
	configManager.SetupDependencyInjection(
		registryManager.GetRegistryAccess(),
		registryManager.GetAppRunner(),
		registryManager.GetAppCreator(),
	)

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

	// Use CORS middleware from handlers package

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
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
