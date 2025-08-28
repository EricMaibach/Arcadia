package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bytecodealliance/wasmtime-go"
	_ "github.com/mattn/go-sqlite3"
)

// --- Data structures ---

type File struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type ToolInfo struct {
	Name        string `json:"name"`
	InputFormat string `json:"inputFormat"`
}

type App struct {
	AppID          string     `json:"appId"`
	Version        string     `json:"version"`
	Runtime        string     `json:"runtime"`
	Tools          []ToolInfo `json:"tools"`
	ArtifactURI    string     `json:"artifactUri"`
	SourceLanguage string     `json:"sourceLanguage,omitempty"`
	Files          []File     `json:"files,omitempty"`
}

const registryFilePath = "app_registry.json"

var (
	registry      = make(map[string]*App)
	registryMutex sync.RWMutex
	wasmEngine    = wasmtime.NewEngine()
	db            *sql.DB
	dbMutex       sync.RWMutex
	appLogger     *log.Logger
	logFile       *os.File
)

// --- Logging Setup ---

func initAppLogger() error {
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Create log file with timestamp
	logFileName := fmt.Sprintf("logs/app_submissions_%s.log", time.Now().Format("2006-01-02"))
	var err error
	logFile, err = os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to create log file: %v", err)
	}

	// Create logger that writes to both file and stdout
	appLogger = log.New(io.MultiWriter(os.Stdout, logFile), "[APP_SUBMISSION] ", log.LstdFlags|log.Lmicroseconds)

	appLogger.Println("=== App Submission Logger Initialized ===")
	return nil
}

func closeAppLogger() {
	if logFile != nil {
		appLogger.Println("=== App Submission Logger Closing ===")
		logFile.Close()
	}
}

// Safe logging helper
func logAppSubmission(format string, args ...interface{}) {
	if appLogger != nil {
		appLogger.Printf(format, args...)
	}
}

// --- REST Handlers ---

func listAppsHandler(w http.ResponseWriter, r *http.Request) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	apps := []*App{}
	for _, app := range registry {
		apps = append(apps, app)
	}
	json.NewEncoder(w).Encode(apps)
}

type RunToolRequest struct {
	AppID    string          `json:"appId"`
	ToolName string          `json:"toolName"`
	Input    json.RawMessage `json:"input"`
}

func runToolHandler(w http.ResponseWriter, r *http.Request) {
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
	registryMutex.RLock()
	app, ok := registry[req.AppID]
	registryMutex.RUnlock()
	if !ok {
		logAppSubmission("[%s] ERROR: App not found in registry: %s", sessionID, req.AppID)
		logAppSubmission("[%s] RESPONSE: HTTP 404 - App not found", sessionID)
		http.Error(w, "app not found", http.StatusNotFound)
		return
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

	logAppSubmission("[%s] STEP 4: Compiling WASM module", sessionID)
	store := wasmtime.NewStore(wasmEngine)
	module, err := wasmtime.NewModule(wasmEngine, wasmBytes)
	if err != nil {
		logAppSubmission("[%s] ERROR: Failed to compile WASM module: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Module compilation failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to compile module: %v", err), http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] WASM module compiled successfully", sessionID)

	logAppSubmission("[%s] STEP 5: Setting up WASM linker and host functions", sessionID)
	linker := wasmtime.NewLinker(wasmEngine)

	// Define database host functions that WASM can call
	linker.DefineFunc(store, "env", "db_query", func(caller *wasmtime.Caller, queryPtr, queryLen, resultPtrPtr int32) int32 {
		return dbQuery(caller, queryPtr, queryLen, resultPtrPtr)
	})

	linker.DefineFunc(store, "env", "db_exec", func(caller *wasmtime.Caller, stmtPtr, stmtLen int32) int32 {
		return dbExec(caller, stmtPtr, stmtLen)
	})

	linker.DefineFunc(store, "env", "db_prepared_query", func(caller *wasmtime.Caller, stmtPtr, stmtLen, paramsPtr, paramsLen, resultPtrPtr int32) int32 {
		return dbPreparedQuery(caller, stmtPtr, stmtLen, paramsPtr, paramsLen, resultPtrPtr)
	})

	logAppSubmission("[%s] STEP 6: Instantiating WASM module", sessionID)
	instance, err := linker.Instantiate(store, module)
	if err != nil {
		logAppSubmission("[%s] ERROR: Failed to instantiate WASM module: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Module instantiation failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to instantiate module: %v", err), http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] WASM module instantiated successfully", sessionID)

	logAppSubmission("[%s] STEP 7: Verifying required WASM functions", sessionID)
	runFunc := instance.GetFunc(store, "run")
	if runFunc == nil {
		logAppSubmission("[%s] ERROR: WASM module does not export 'run' function", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Missing run function", sessionID)
		http.Error(w, "module does not export 'run' function", http.StatusInternalServerError)
		return
	}

	// Get WASM functions
	allocateFunc := instance.GetFunc(store, "allocate")
	deallocateFunc := instance.GetFunc(store, "deallocate")
	getResultPtrFunc := instance.GetFunc(store, "get_result_ptr")

	if allocateFunc == nil || deallocateFunc == nil || getResultPtrFunc == nil {
		logAppSubmission("[%s] ERROR: WASM module missing required functions (allocate, deallocate, get_result_ptr)", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Missing required functions", sessionID)
		http.Error(w, "module missing required functions (allocate, deallocate, get_result_ptr)", http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] All required WASM functions found", sessionID)

	logAppSubmission("[%s] STEP 8: Preparing input for WASM execution", sessionID)
	// Prepare input string - transform to the format WASM expects
	wasmInput := map[string]interface{}{
		"tool": req.ToolName,
		"data": req.Input,
	}
	inputBytes, _ := json.Marshal(wasmInput)
	inputStr := string(inputBytes)
	inputLen := len(inputStr)
	logAppSubmission("[%s] Input prepared - Tool: %s, InputLength: %d, Input: %s", sessionID, req.ToolName, inputLen, inputStr)

	// Allocate memory in WASM for input
	logAppSubmission("[%s] Allocating WASM memory for input (%d bytes)", sessionID, inputLen)
	inputPtrResult, err := allocateFunc.Call(store, inputLen)
	if err != nil {
		logAppSubmission("[%s] ERROR: Failed to allocate WASM input memory: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Memory allocation failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to allocate input memory: %v", err), http.StatusInternalServerError)
		return
	}
	inputPtr := inputPtrResult.(int32)
	logAppSubmission("[%s] Input memory allocated at pointer: %d", sessionID, inputPtr)

	// Get WASM memory and copy input string
	logAppSubmission("[%s] Copying input data to WASM memory", sessionID)
	memory := instance.GetExport(store, "memory").Memory()
	data := memory.UnsafeData(store)
	copy(data[inputPtr:inputPtr+int32(inputLen)], []byte(inputStr))

	// Call WASM 'run' function with pointer and length
	logAppSubmission("[%s] STEP 9: Executing WASM 'run' function", sessionID)
	startTime := time.Now()
	resultLenResult, err := runFunc.Call(store, inputPtr, inputLen)
	executionTime := time.Since(startTime)
	if err != nil {
		// Clean up allocated memory
		deallocateFunc.Call(store, inputPtr, inputLen)
		logAppSubmission("[%s] ERROR: WASM execution failed after %v: %v", sessionID, executionTime, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Execution failed", sessionID)
		http.Error(w, fmt.Sprintf("execution failed: %v", err), http.StatusInternalServerError)
		return
	}
	resultLen := resultLenResult.(int32)
	logAppSubmission("[%s] WASM execution completed successfully in %v, result length: %d", sessionID, executionTime, resultLen)

	logAppSubmission("[%s] STEP 10: Retrieving execution results", sessionID)
	// Get result pointer
	resultPtrResult, err := getResultPtrFunc.Call(store)
	if err != nil {
		deallocateFunc.Call(store, inputPtr, inputLen)
		logAppSubmission("[%s] ERROR: Failed to get result pointer: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Result retrieval failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to get result pointer: %v", err), http.StatusInternalServerError)
		return
	}
	resultPtr := resultPtrResult.(int32)
	logAppSubmission("[%s] Result pointer: %d", sessionID, resultPtr)

	// Read result from WASM memory
	resultBytes := make([]byte, resultLen)
	copy(resultBytes, data[resultPtr:resultPtr+resultLen])
	resultStr := string(resultBytes)
	logAppSubmission("[%s] Result retrieved (%d bytes), result is: %s", sessionID, len(resultBytes), resultStr)

	// Clean up allocated input memory
	logAppSubmission("[%s] Cleaning up allocated input memory", sessionID)
	deallocateFunc.Call(store, inputPtr, inputLen)

	// Return output
	logAppSubmission("[%s] STEP 11: Sending success response", sessionID)
	output := map[string]interface{}{
		"output": resultStr,
		"status": "success",
	}

	outputJSON, _ := json.Marshal(output)
	logAppSubmission("[%s] Response prepared (%d bytes)", sessionID, len(outputJSON))

	json.NewEncoder(w).Encode(output)
	logAppSubmission("[%s] === TOOL EXECUTION COMPLETED SUCCESSFULLY ===", sessionID)
	logAppSubmission("[%s] Total execution time: %v", sessionID, time.Since(startTime))
}

type AppRequest struct {
	AppID   string     `json:"appId"`
	Version string     `json:"version"`
	Runtime string     `json:"runtime"`
	Tools   []ToolInfo `json:"tools"`
	AppSrc  string     `json:"appSrc"`
}

func submitAppSrcHandler(w http.ResponseWriter, r *http.Request) {
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
	var req AppRequest
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
			http.Error(w, fmt.Sprintf("tool %d (%s): inputFormat is required", i, tool.Name), http.StatusBadRequest)
			return
		}
	}

	logAppSubmission("[%s] Validation passed - AppID: %s, Version: %s, Tools: %v", sessionID, req.AppID, req.Version, req.Tools)

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

	// Step 5: Generate Rust project
	logAppSubmission("[%s] STEP 5: Generating Rust project from trait implementation", sessionID)
	startTime := time.Now()
	if err := generateRustProjectFromTrait(req, buildDir); err != nil {
		logAppSubmission("[%s] ERROR: Failed to generate Rust project: %v", sessionID, err)
		logAppSubmission("[%s] Cleaning up build directory: %s", sessionID, buildDir)
		os.RemoveAll(buildDir)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Rust project generation failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to generate Rust project: %v", err), http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] Rust project generation completed in %v", sessionID, time.Since(startTime))

	// Step 6: Compile to WASM
	logAppSubmission("[%s] STEP 6: Compiling Rust to WASM", sessionID)
	startTime = time.Now()
	wasmPath, err := buildRustToWasm(buildDir)
	if err != nil {
		logAppSubmission("[%s] ERROR: Failed to compile Rust to WASM: %v", sessionID, err)
		logAppSubmission("[%s] Build directory contents before cleanup:", sessionID)

		// Log build directory contents for debugging
		if entries, dirErr := os.ReadDir(buildDir); dirErr == nil {
			for _, entry := range entries {
				logAppSubmission("[%s]   - %s (dir: %t)", sessionID, entry.Name(), entry.IsDir())
			}
		}

		logAppSubmission("[%s] Cleaning up build directory: %s", sessionID, buildDir)
		os.RemoveAll(buildDir)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - WASM compilation failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to compile Rust to WASM: %v", err), http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] WASM compilation completed in %v", sessionID, time.Since(startTime))
	logAppSubmission("[%s] Compiled WASM path: %s", sessionID, wasmPath)

	// Step 7: Register app in registry
	logAppSubmission("[%s] STEP 7: Registering app in registry", sessionID)
	registryMutex.Lock()
	registry[req.AppID] = &App{
		AppID:          req.AppID,
		Version:        req.Version,
		Runtime:        req.Runtime,
		Tools:          req.Tools,
		ArtifactURI:    wasmPath,
		SourceLanguage: "rust",
		Files: []File{
			{
				Name:    "src/lib.rs",
				Content: "Generated from trait implementation",
			},
		},
	}
	registrySize := len(registry)
	registryMutex.Unlock()
	logAppSubmission("[%s] App registered successfully. Registry now contains %d apps", sessionID, registrySize)

	// Step 8: Save registry to file
	logAppSubmission("[%s] STEP 8: Saving registry to file", sessionID)
	if err := saveRegistry(); err != nil {
		logAppSubmission("[%s] WARNING: Failed to save registry to file: %v", sessionID, err)
		log.Printf("Warning: failed to save registry: %v", err)
	} else {
		logAppSubmission("[%s] Registry saved successfully", sessionID)
	}

	// Step 9: Send success response
	logAppSubmission("[%s] STEP 9: Sending success response", sessionID)
	response := map[string]string{
		"status":   "app trait implementation submitted and compiled",
		"wasmPath": wasmPath,
	}
	responseJSON, _ := json.Marshal(response)
	logAppSubmission("[%s] Response: %s", sessionID, string(responseJSON))

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)

	logAppSubmission("[%s] === APP SUBMISSION COMPLETED SUCCESSFULLY ===", sessionID)
	logAppSubmission("[%s] Total processing time: %v", sessionID, time.Since(startTime))
}

func generateRustProjectFromTrait(req AppRequest, buildDir string) error {
	// Read the wrapper template
	templatePath := filepath.Join("wasm_wrapper_template.rs")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read wrapper template: %v", err)
	}

	// Inject the user's trait implementation into the template
	injectedContent := strings.Replace(string(templateContent), "// USER_APP_IMPL_PLACEHOLDER - This will be replaced with user's trait implementation", req.AppSrc, 1)

	// Create src directory
	srcDir := filepath.Join(buildDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return fmt.Errorf("failed to create src directory: %v", err)
	}

	// Write lib.rs with injected code
	libPath := filepath.Join(srcDir, "lib.rs")
	if err := os.WriteFile(libPath, []byte(injectedContent), 0644); err != nil {
		return fmt.Errorf("failed to write lib.rs: %v", err)
	}

	// Create Cargo.toml
	cargoToml := fmt.Sprintf(`[package]
name = "%s"
version = "%s"
edition = "2021"

[lib]
crate-type = ["cdylib"]

[dependencies]
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"

[profile.release]
opt-level = "s"
lto = true
`, req.AppID, req.Version)

	cargoPath := filepath.Join(buildDir, "Cargo.toml")
	if err := os.WriteFile(cargoPath, []byte(cargoToml), 0644); err != nil {
		return fmt.Errorf("failed to write Cargo.toml: %v", err)
	}

	return nil
}

func createFilesFromSpec(files []File, destDir string) error {
	for _, file := range files {
		// Create the full file path
		destPath := filepath.Join(destDir, file.Name)

		// Ensure the file path is within the destination directory (security check)
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", file.Name)
		}

		// Create parent directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %v", destPath, err)
		}

		// Create destination file
		destFile, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %v", destPath, err)
		}
		defer destFile.Close()

		// Write file contents as raw text
		if _, err := destFile.WriteString(file.Content); err != nil {
			return fmt.Errorf("failed to write contents to file %s: %v", file.Name, err)
		}
	}

	return nil
}

func buildRustToWasm(sourceBuildDir string) (string, error) {
	// Check if Cargo.toml exists to confirm it's a Rust project
	cargoPath := filepath.Join(sourceBuildDir, "Cargo.toml")
	if _, err := os.Stat(cargoPath); os.IsNotExist(err) {
		return "", fmt.Errorf("Cargo.toml not found in %s", sourceBuildDir)
	}

	// Create artifacts directory structure to store the compiled WASM
	appID := filepath.Base(filepath.Dir(sourceBuildDir))
	version := filepath.Base(sourceBuildDir)
	artifactsDir := filepath.Join("artifacts", appID)
	wasmFilename := fmt.Sprintf("%s.wasm", version)
	outputWasmPath := filepath.Join(artifactsDir, wasmFilename)

	// Create artifacts directory if it doesn't exist
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create artifacts directory: %v", err)
	}

	// Build the Rust project to WASM
	// Using cargo build with wasm32-unknown-unknown target
	cmd := exec.Command("cargo", "build", "--target", "wasm32-unknown-unknown", "--release")
	cmd.Dir = sourceBuildDir

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cargo build failed: %v\nOutput: %s", err, string(output))
	}

	// Find the compiled WASM file in the target directory
	// The WASM file should be at target/wasm32-unknown-unknown/release/<project_name>.wasm
	targetDir := filepath.Join(sourceBuildDir, "target", "wasm32-unknown-unknown", "release")

	// Read Cargo.toml to get the project name
	projectName, err := getProjectNameFromCargo(cargoPath)
	if err != nil {
		return "", fmt.Errorf("failed to get project name: %v", err)
	}

	compiledWasmPath := filepath.Join(targetDir, projectName+".wasm")

	// Check if the compiled WASM file exists
	if _, err := os.Stat(compiledWasmPath); os.IsNotExist(err) {
		return "", fmt.Errorf("compiled WASM file not found at %s", compiledWasmPath)
	}

	// Copy the compiled WASM to the artifacts directory
	if err := copyFile(compiledWasmPath, outputWasmPath); err != nil {
		return "", fmt.Errorf("failed to copy WASM file: %v", err)
	}

	return outputWasmPath, nil
}

func getProjectNameFromCargo(cargoPath string) (string, error) {
	content, err := os.ReadFile(cargoPath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name") && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				name = strings.Trim(name, `"`)
				// Rust converts hyphens to underscores in binary names
				name = strings.ReplaceAll(name, "-", "_")
				return name, nil
			}
		}
	}
	return "", fmt.Errorf("project name not found in Cargo.toml")
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// --- Database Host Functions for WASM ---

// dbQuery executes a SQL query and returns JSON results
func dbQuery(caller *wasmtime.Caller, queryPtr, queryLen, resultPtrPtr int32) int32 {
	// Get memory instance
	memory := caller.GetExport("memory").Memory()
	data := memory.UnsafeData(caller)

	// Read query string from WASM memory
	queryBytes := data[queryPtr : queryPtr+queryLen]
	query := string(queryBytes)

	dbMutex.RLock()
	defer dbMutex.RUnlock()

	if db == nil {
		return -1 // Database not initialized
	}

	// Execute query
	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Database query error: %v", err)
		return -2 // Query error
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return -3 // Column error
	}

	// Collect results
	var results []map[string]interface{}
	for rows.Next() {
		// Create a slice to hold column values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			log.Printf("Row scan error: %v", err)
			continue
		}

		// Convert to map
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	// Marshal results to JSON
	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return -4 // JSON marshal error
	}

	// Allocate memory in WASM for result
	allocateFunc := caller.GetExport("allocate").Func()
	resultLenResult, err := allocateFunc.Call(caller, len(jsonBytes))
	if err != nil {
		return -5 // Allocation error
	}
	resultPtr := resultLenResult.(int32)

	// Copy result to WASM memory
	copy(data[resultPtr:resultPtr+int32(len(jsonBytes))], jsonBytes)

	// Store result pointer in the provided location
	resultPtrPtrBytes := data[resultPtrPtr : resultPtrPtr+4]
	resultPtrPtrBytes[0] = byte(resultPtr)
	resultPtrPtrBytes[1] = byte(resultPtr >> 8)
	resultPtrPtrBytes[2] = byte(resultPtr >> 16)
	resultPtrPtrBytes[3] = byte(resultPtr >> 24)

	return int32(len(jsonBytes)) // Return result length
}

// dbExec executes a SQL statement (INSERT, UPDATE, DELETE)
func dbExec(caller *wasmtime.Caller, stmtPtr, stmtLen int32) int32 {
	// Get memory instance
	memory := caller.GetExport("memory").Memory()
	data := memory.UnsafeData(caller)

	// Read statement string from WASM memory
	stmtBytes := data[stmtPtr : stmtPtr+stmtLen]
	stmt := string(stmtBytes)

	dbMutex.Lock()
	defer dbMutex.Unlock()

	if db == nil {
		return -1 // Database not initialized
	}

	// Execute statement
	result, err := db.Exec(stmt)
	if err != nil {
		log.Printf("Database exec error: %v", err)
		return -2 // Execution error
	}

	// Return number of affected rows
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return -3 // Could not get affected rows
	}

	return int32(rowsAffected)
}

// dbPreparedQuery executes a prepared statement with parameters
func dbPreparedQuery(caller *wasmtime.Caller, stmtPtr, stmtLen, paramsPtr, paramsLen, resultPtrPtr int32) int32 {
	// Get memory instance
	memory := caller.GetExport("memory").Memory()
	data := memory.UnsafeData(caller)

	// Read statement string
	stmtBytes := data[stmtPtr : stmtPtr+stmtLen]
	stmt := string(stmtBytes)

	// Read parameters JSON
	paramsBytes := data[paramsPtr : paramsPtr+paramsLen]
	var params []interface{}
	if paramsLen > 0 {
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return -1 // Parameter parsing error
		}
	}

	dbMutex.RLock()
	defer dbMutex.RUnlock()

	if db == nil {
		return -2 // Database not initialized
	}

	// Execute prepared statement
	rows, err := db.Query(stmt, params...)
	if err != nil {
		log.Printf("Database prepared query error: %v", err)
		return -3 // Query error
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return -4 // Column error
	}

	// Collect results (same as dbQuery)
	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	// Marshal results to JSON
	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return -5 // JSON marshal error
	}

	// Allocate memory in WASM for result
	allocateFunc := caller.GetExport("allocate").Func()
	resultLenResult, err := allocateFunc.Call(caller, len(jsonBytes))
	if err != nil {
		return -6 // Allocation error
	}
	resultPtr := resultLenResult.(int32)

	// Copy result to WASM memory
	copy(data[resultPtr:resultPtr+int32(len(jsonBytes))], jsonBytes)

	// Store result pointer
	resultPtrPtrBytes := data[resultPtrPtr : resultPtrPtr+4]
	resultPtrPtrBytes[0] = byte(resultPtr)
	resultPtrPtrBytes[1] = byte(resultPtr >> 8)
	resultPtrPtrBytes[2] = byte(resultPtr >> 16)
	resultPtrPtrBytes[3] = byte(resultPtr >> 24)

	return int32(len(jsonBytes))
}

func loadRegistry() error {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	// Check if registry file exists
	if _, err := os.Stat(registryFilePath); os.IsNotExist(err) {
		// File doesn't exist, start with empty registry
		registry = make(map[string]*App)
		log.Printf("Registry file %s not found, starting with empty registry", registryFilePath)
		return nil
	}

	// Read the registry file
	data, err := os.ReadFile(registryFilePath)
	if err != nil {
		return fmt.Errorf("failed to read registry file: %v", err)
	}

	// Unmarshal JSON into registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return fmt.Errorf("failed to parse registry JSON: %v", err)
	}

	log.Printf("Loaded %d apps from registry file", len(registry))
	return nil
}

func saveRegistry() error {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	// Marshal registry to JSON
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry to JSON: %v", err)
	}

	// Write to file
	if err := os.WriteFile(registryFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry file: %v", err)
	}

	return nil
}

// --- Database Management ---

func initDatabase() error {
	var err error

	// Create data directory if it doesn't exist
	if err := os.MkdirAll("data", 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Open SQLite database
	db, err = sql.Open("sqlite3", "data/arcadia.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Printf("Warning: failed to enable WAL mode: %v", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		log.Printf("Warning: failed to enable foreign keys: %v", err)
	}

	// Create default tables for app data storage
	if err := createDefaultTables(); err != nil {
		return fmt.Errorf("failed to create default tables: %v", err)
	}

	log.Println("Database initialized successfully")
	return nil
}

func createDefaultTables() error {
	queries := []string{
		// App data table - for general key-value storage per app
		`CREATE TABLE IF NOT EXISTS app_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_id TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(app_id, key)
		);`,

		// App logs table - for application logging
		`CREATE TABLE IF NOT EXISTS app_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_id TEXT NOT NULL,
			level TEXT NOT NULL,
			message TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// Sessions table - for user session management
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL,
			user_data TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// Create indexes for better performance
		`CREATE INDEX IF NOT EXISTS idx_app_data_app_id ON app_data(app_id);`,
		`CREATE INDEX IF NOT EXISTS idx_app_logs_app_id ON app_logs(app_id);`,
		`CREATE INDEX IF NOT EXISTS idx_app_logs_timestamp ON app_logs(timestamp);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_app_id ON sessions(app_id);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %v", err)
		}
	}

	return nil
}

func closeDatabase() {
	if db != nil {
		db.Close()
		log.Println("Database connection closed")
	}
}

// --- Main ---

func main() {
	// Initialize logging
	if err := initAppLogger(); err != nil {
		log.Fatalf("Failed to initialize app logger: %v", err)
	}
	defer closeAppLogger()

	// Initialize database
	if err := initDatabase(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer closeDatabase()

	// Load registry from file on startup
	if err := loadRegistry(); err != nil {
		log.Fatalf("Failed to load registry: %v", err)
	}

	http.HandleFunc("/list_apps", listAppsHandler)
	http.HandleFunc("/run_tool", runToolHandler)
	http.HandleFunc("/submit_app_src", submitAppSrcHandler)

	port := "8080"
	log.Printf("Starting Phase 0 runtime server on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
