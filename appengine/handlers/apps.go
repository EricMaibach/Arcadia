package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bytecodealliance/wasmtime-go"

	"arcadia/services"
)

// Dependencies that will be injected
var (
	registryManager    *services.RegistryManager
	wasmEngineRef      *wasmtime.Engine
	logAppSubmission   func(format string, args ...interface{})
	executeAppTool     func(appID, toolName string, input json.RawMessage) (string, error)
	appCreationService *services.AppCreationService
)

// Types needed for handlers
type App struct {
	AppID          string     `json:"appId"`
	Version        string     `json:"version"`
	Runtime        string     `json:"runtime"`
	Tools          []ToolInfo `json:"tools"`
	ArtifactURI    string     `json:"artifactUri"`
	SourceLanguage string     `json:"sourceLanguage"`
	Files          []File     `json:"files"`
}

type ToolInfo struct {
	Name        string `json:"name"`
	InputFormat string `json:"inputFormat"`
}

type File struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type RunToolRequest struct {
	AppID    string          `json:"appId"`
	ToolName string          `json:"toolName"`
	Input    json.RawMessage `json:"input"`
}

type AppRequest struct {
	AppID   string     `json:"appId"`
	Version string     `json:"version"`
	Runtime string     `json:"runtime"`
	Tools   []ToolInfo `json:"tools"`
	AppSrc  string     `json:"appSrc"`
}

// SetDependencies configures the dependencies for the app handlers
func SetAppDependencies(
	regManager *services.RegistryManager,
	engine *wasmtime.Engine,
	logFunc func(format string, args ...interface{}),
	execFunc func(appID, toolName string, input json.RawMessage) (string, error),
) {
	registryManager = regManager
	wasmEngineRef = engine
	logAppSubmission = logFunc
	executeAppTool = execFunc
	appCreationService = services.NewAppCreationService(regManager, logFunc)
}

// ListAppsHandler handles GET /list_apps
func ListAppsHandler(w http.ResponseWriter, r *http.Request) {
	registry := registryManager.GetRegistry()
	allApps := registry.GetAllApps()

	apps := []*App{}
	for _, app := range allApps {
		apps = append(apps, &App{
			AppID:          app.AppID,
			Version:        app.Version,
			Runtime:        app.Runtime,
			Tools:          convertToolsFromServices(app.Tools),
			ArtifactURI:    app.ArtifactURI,
			SourceLanguage: app.SourceLanguage,
			Files:          convertFilesFromServices(app.Files),
		})
	}
	json.NewEncoder(w).Encode(apps)
}

// RunToolHandler handles POST /run_tool
func RunToolHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := fmt.Sprintf("tool_session_%d", time.Now().UnixNano())
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = forwarded
	}

	logAppSubmission("=== NEW TOOL EXECUTION REQUEST [%s] ===", sessionID)
	logAppSubmission("[%s] Client IP: %s", sessionID, clientIP)
	logAppSubmission("[%s] Request Method: %s", sessionID, r.Method)
	logAppSubmission("[%s] Request URL: %s", sessionID, r.URL.String())
	logAppSubmission("[%s] User-Agent: %s", sessionID, r.Header.Get("User-Agent"))
	logAppSubmission("[%s] Content-Type: %s", sessionID, r.Header.Get("Content-Type"))

	logAppSubmission("[%s] STEP 1: Parsing tool execution request", sessionID)
	var req RunToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logAppSubmission("[%s] ERROR: Failed to decode request body: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Invalid JSON", sessionID)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	logAppSubmission("[%s] Parsed request - AppID: %s, ToolName: %s, InputLength: %d", sessionID, req.AppID, req.ToolName, len(req.Input))

	logAppSubmission("[%s] STEP 2: Looking up app in registry", sessionID)
	registry := registryManager.GetRegistry()
	serviceApp, ok := registry.GetApp(req.AppID)
	if !ok {
		logAppSubmission("[%s] ERROR: App not found in registry: %s", sessionID, req.AppID)
		logAppSubmission("[%s] RESPONSE: HTTP 404 - App not found", sessionID)
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}
	// Convert to handler App type for compatibility
	app := &App{
		AppID:          serviceApp.AppID,
		Version:        serviceApp.Version,
		Runtime:        serviceApp.Runtime,
		Tools:          convertToolsFromServices(serviceApp.Tools),
		ArtifactURI:    serviceApp.ArtifactURI,
		SourceLanguage: serviceApp.SourceLanguage,
		Files:          convertFilesFromServices(serviceApp.Files),
	}
	logAppSubmission("[%s] App found - Version: %s, Runtime: %s, ArtifactURI: %s", sessionID, app.Version, app.Runtime, app.ArtifactURI)

	// Validate that the requested tool exists in the app
	toolFound := false
	for _, tool := range app.Tools {
		if tool.Name == req.ToolName {
			toolFound = true
			logAppSubmission("[%s] Tool '%s' found with input format: %s", sessionID, req.ToolName, tool.InputFormat)
			break
		}
	}
	if !toolFound {
		logAppSubmission("[%s] ERROR: Tool '%s' not found in app. Available tools: %v", sessionID, req.ToolName, app.Tools)
		logAppSubmission("[%s] RESPONSE: HTTP 404 - Tool not found", sessionID)
		http.Error(w, fmt.Sprintf("tool '%s' not found in app", req.ToolName), http.StatusNotFound)
		return
	}

	logAppSubmission("[%s] STEP 3: Loading WASM module from: %s", sessionID, app.ArtifactURI)
	wasmBytes, err := os.ReadFile(app.ArtifactURI)
	if err != nil {
		logAppSubmission("[%s] ERROR: Failed to load WASM artifact: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Artifact load failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to load artifact: %v", err), http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] WASM module loaded successfully (%d bytes)", sessionID, len(wasmBytes))

	logAppSubmission("[%s] STEP 4: Executing tool '%s' (delegating to executeAppTool)", sessionID, req.ToolName)

	logAppSubmission("[%s] STEP 5: Executing tool '%s'", sessionID, req.ToolName)
	output, err := executeAppTool(req.AppID, req.ToolName, req.Input)
	if err != nil {
		logAppSubmission("[%s] ERROR: Tool execution failed: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Tool execution failed", sessionID)
		http.Error(w, fmt.Sprintf("tool execution failed: %v", err), http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] Tool execution completed successfully", sessionID)

	// Step 6: Return response
	logAppSubmission("[%s] STEP 6: Sending response", sessionID)
	response := map[string]string{
		"output": output,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	logAppSubmission("[%s] === TOOL EXECUTION COMPLETED SUCCESSFULLY ===", sessionID)
}

// SubmitAppSrcHandler handles POST /submit_app_src
func SubmitAppSrcHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := fmt.Sprintf("rest_api_%d", time.Now().UnixNano())
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = forwarded
	}

	logAppSubmission("=== NEW APP SUBMISSION REQUEST [%s] ===", sessionID)
	logAppSubmission("[%s] Client IP: %s", sessionID, clientIP)
	logAppSubmission("[%s] Request Method: %s", sessionID, r.Method)
	logAppSubmission("[%s] Request URL: %s", sessionID, r.URL.String())
	logAppSubmission("[%s] User-Agent: %s", sessionID, r.Header.Get("User-Agent"))
	logAppSubmission("[%s] Content-Type: %s", sessionID, r.Header.Get("Content-Type"))

	// Parse request
	logAppSubmission("[%s] Parsing request body", sessionID)
	var req services.AppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logAppSubmission("[%s] ERROR: Failed to decode request body: %v", sessionID, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Log the parsed request (sanitized)
	reqJSON, _ := json.MarshalIndent(map[string]interface{}{
		"appId":        req.AppID,
		"version":      req.Version,
		"runtime":      req.Runtime,
		"tools":        req.Tools,
		"appSrcLength": len(req.AppSrc),
		"appSrcPreview": func() string {
			if len(req.AppSrc) > 100 {
				return req.AppSrc[:100] + "..."
			}
			return req.AppSrc
		}(),
	}, "", "  ")
	logAppSubmission("[%s] Parsed request: %s", sessionID, string(reqJSON))

	// Convert to standardized request
	creationReq := services.AppCreationRequest{
		AppID:        req.AppID,
		Version:      req.Version,
		Runtime:      req.Runtime,
		Tools:        req.Tools,
		AppSrc:       req.AppSrc,
		Dependencies: req.Dependencies,
	}

	// Use centralized app creation service
	logAppSubmission("[%s] Using centralized app creation service", sessionID)
	result, err := appCreationService.CreateApp(creationReq, sessionID)
	if err != nil {
		logAppSubmission("[%s] ERROR: App creation failed: %v", sessionID, err)

		// Map specific errors to HTTP status codes
		var statusCode int
		errorMsg := err.Error()

		if strings.Contains(errorMsg, "is required") {
			statusCode = http.StatusBadRequest
		} else if strings.Contains(errorMsg, "already exists") {
			statusCode = http.StatusConflict
		} else {
			statusCode = http.StatusInternalServerError
		}

		http.Error(w, errorMsg, statusCode)
		return
	}

	// Send success response
	logAppSubmission("[%s] App creation successful: %s", sessionID, result)
	response := map[string]string{
		"status": "app trait implementation submitted and compiled",
		"result": result,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
	logAppSubmission("[%s] === APP SUBMISSION COMPLETED SUCCESSFULLY ===", sessionID)
}

// convertToolsFromServices converts services.ToolInfo to handlers.ToolInfo
func convertToolsFromServices(serviceTools []services.ToolInfo) []ToolInfo {
	result := make([]ToolInfo, len(serviceTools))
	for i, tool := range serviceTools {
		result[i] = ToolInfo{
			Name:        tool.Name,
			InputFormat: tool.InputFormat,
		}
	}
	return result
}

// convertFilesFromServices converts services.File to handlers.File
func convertFilesFromServices(serviceFiles []services.File) []File {
	result := make([]File, len(serviceFiles))
	for i, file := range serviceFiles {
		result[i] = File{
			Name:    file.Name,
			Content: file.Content,
		}
	}
	return result
}
