package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bytecodealliance/wasmtime-go"

	"arcadia/services"
)

// Dependencies that will be injected
var (
	registryManager  *services.RegistryManager
	wasmEngineRef    *wasmtime.Engine
	logAppSubmission func(format string, args ...interface{})
	executeAppTool   func(appID, toolName string, input json.RawMessage) (string, error)
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
	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())
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

	// Step 1: Parse and validate request
	logAppSubmission("[%s] STEP 1: Parsing request body", sessionID)
	var req services.AppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logAppSubmission("[%s] ERROR: Failed to decode request body: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Invalid JSON", sessionID)
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

	// Step 2: Validate required fields
	logAppSubmission("[%s] STEP 2: Validating request fields", sessionID)
	if strings.TrimSpace(req.AppSrc) == "" {
		logAppSubmission("[%s] ERROR: AppSrc field is empty or whitespace only", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Missing app source", sessionID)
		http.Error(w, "trait implementation is required", http.StatusBadRequest)
		return
	}

	if req.AppID == "" {
		logAppSubmission("[%s] ERROR: AppID field is empty", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Missing AppID", sessionID)
		http.Error(w, "appId is required", http.StatusBadRequest)
		return
	}

	if req.Version == "" {
		logAppSubmission("[%s] ERROR: Version field is empty", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Missing Version", sessionID)
		http.Error(w, "version is required", http.StatusBadRequest)
		return
	}

	if len(req.Tools) == 0 {
		logAppSubmission("[%s] ERROR: Tools array is empty", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Missing tools", sessionID)
		http.Error(w, "at least one tool is required", http.StatusBadRequest)
		return
	}

	// Validate each tool has both name and input format
	for i, tool := range req.Tools {
		if strings.TrimSpace(tool.Name) == "" {
			logAppSubmission("[%s] ERROR: Tool %d has empty name", sessionID, i)
			logAppSubmission("[%s] RESPONSE: HTTP 400 - Invalid tool name", sessionID)
			http.Error(w, fmt.Sprintf("tool %d: name is required", i), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(tool.InputFormat) == "" {
			logAppSubmission("[%s] ERROR: Tool %d (%s) has empty input format", sessionID, i, tool.Name)
			logAppSubmission("[%s] RESPONSE: HTTP 400 - Invalid tool input format", sessionID)
			http.Error(w, fmt.Sprintf("tool %d (%s): input format is required", i, tool.Name), http.StatusBadRequest)
			return
		}
	}

	logAppSubmission("[%s] All validation passed", sessionID)

	// Step 3: Check for existing compiled WASM
	logAppSubmission("[%s] STEP 3: Checking for existing compiled WASM", sessionID)
	artifactsDir := filepath.Join("artifacts", req.AppID)
	wasmFilename := fmt.Sprintf("%s.wasm", req.Version)
	finalWasmPath := filepath.Join(artifactsDir, wasmFilename)
	logAppSubmission("[%s] Checking path: %s", sessionID, finalWasmPath)

	if _, err := os.Stat(finalWasmPath); err == nil {
		logAppSubmission("[%s] ERROR: WASM file already exists at %s", sessionID, finalWasmPath)
		logAppSubmission("[%s] RESPONSE: HTTP 409 - Conflict", sessionID)
		http.Error(w, fmt.Sprintf("app %s version %s already compiled and exists", req.AppID, req.Version), http.StatusConflict)
		return
	}
	logAppSubmission("[%s] No existing WASM found - proceeding with compilation", sessionID)

	// Step 4: Prepare build directory
	logAppSubmission("[%s] STEP 4: Setting up build directory", sessionID)
	buildDir := filepath.Join("build", req.AppID, req.Version)
	logAppSubmission("[%s] Build directory: %s", sessionID, buildDir)

	// Clear build directory if it exists
	if _, err := os.Stat(buildDir); err == nil {
		logAppSubmission("[%s] Existing build directory found, removing: %s", sessionID, buildDir)
		if err := os.RemoveAll(buildDir); err != nil {
			logAppSubmission("[%s] ERROR: Failed to remove existing build directory: %v", sessionID, err)
			logAppSubmission("[%s] RESPONSE: HTTP 500 - Build directory cleanup failed", sessionID)
			http.Error(w, fmt.Sprintf("failed to clear existing build directory: %v", err), http.StatusInternalServerError)
			return
		}
		logAppSubmission("[%s] Successfully removed existing build directory", sessionID)
	}

	// Create build directory
	logAppSubmission("[%s] Creating build directory: %s", sessionID, buildDir)
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		logAppSubmission("[%s] ERROR: Failed to create build directory: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Build directory creation failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to create build directory: %v", err), http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] Build directory created successfully", sessionID)

	// Step 5: Compile trait to WASM using services
	logAppSubmission("[%s] STEP 5: Compiling Rust trait to WASM", sessionID)
	startTime := time.Now()
	wasmCompiler := services.NewWasmCompiler()
	wasmPath, err := wasmCompiler.CompileTraitToWasm(req, buildDir)
	if err != nil {
		logAppSubmission("[%s] ERROR: Failed to compile trait to WASM: %v", sessionID, err)
		//logAppSubmission("[%s] Cleaning up build directory: %s", sessionID, buildDir)
		//os.RemoveAll(buildDir)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - WASM compilation failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to compile trait to WASM: %v", err), http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] WASM compilation completed successfully in %v", sessionID, time.Since(startTime))
	logAppSubmission("[%s] Compiled WASM path: %s", sessionID, wasmPath)

	// Step 6: Register app in registry and save to file
	logAppSubmission("[%s] STEP 6: Registering app in registry", sessionID)
	registry := registryManager.GetRegistry()
	registry.RegisterApp(&services.App{
		AppID:          req.AppID,
		Version:        req.Version,
		Runtime:        req.Runtime,
		Tools:          req.Tools,
		ArtifactURI:    wasmPath,
		SourceLanguage: "rust",
		Files: []services.File{
			{
				Name:    "src/lib.rs",
				Content: "Generated from trait implementation",
			},
		},
	})

	// Save registry to file for persistence
	if err := registryManager.Save(); err != nil {
		logAppSubmission("[%s] WARNING: Failed to save registry to file: %v", sessionID, err)
		// Don't fail the request, app is already registered in memory
	}

	registrySize := registry.GetAppCount()
	logAppSubmission("[%s] App registered successfully. Registry now contains %d apps", sessionID, registrySize)

	// Step 7: Send success response
	logAppSubmission("[%s] STEP 7: Sending success response", sessionID)
	response := map[string]string{
		"status":   "app trait implementation submitted and compiled",
		"wasmPath": wasmPath,
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
