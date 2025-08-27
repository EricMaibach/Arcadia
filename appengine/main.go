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

	"github.com/bytecodealliance/wasmtime-go"
	_ "github.com/mattn/go-sqlite3"
)

// --- Data structures ---

type File struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type App struct {
	AppID          string   `json:"appId"`
	Version        string   `json:"version"`
	Runtime        string   `json:"runtime"`
	Tools          []string `json:"tools"`
	ArtifactURI    string   `json:"artifactUri"`
	SourceLanguage string   `json:"sourceLanguage,omitempty"`
	Files          []File   `json:"files,omitempty"`
}

const registryFilePath = "app_registry.json"

var (
	registry      = make(map[string]*App)
	registryMutex sync.RWMutex
	wasmEngine    = wasmtime.NewEngine()
	db            *sql.DB
	dbMutex       sync.RWMutex
)

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
	var req RunToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	registryMutex.RLock()
	app, ok := registry[req.AppID]
	registryMutex.RUnlock()
	if !ok {
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}

	// Load WASM module
	wasmBytes, err := os.ReadFile(app.ArtifactURI)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load artifact: %v", err), http.StatusInternalServerError)
		return
	}

	store := wasmtime.NewStore(wasmEngine)
	module, err := wasmtime.NewModule(wasmEngine, wasmBytes)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to compile module: %v", err), http.StatusInternalServerError)
		return
	}

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

	instance, err := linker.Instantiate(store, module)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to instantiate module: %v", err), http.StatusInternalServerError)
		return
	}

	runFunc := instance.GetFunc(store, "run")
	if runFunc == nil {
		http.Error(w, "module does not export 'run' function", http.StatusInternalServerError)
		return
	}

	// Get WASM functions
	allocateFunc := instance.GetFunc(store, "allocate")
	deallocateFunc := instance.GetFunc(store, "deallocate")
	getResultPtrFunc := instance.GetFunc(store, "get_result_ptr")

	if allocateFunc == nil || deallocateFunc == nil || getResultPtrFunc == nil {
		http.Error(w, "module missing required functions (allocate, deallocate, get_result_ptr)", http.StatusInternalServerError)
		return
	}

	// Prepare input string
	inputBytes, _ := json.Marshal(req.Input)
	inputStr := string(inputBytes)
	inputLen := len(inputStr)

	// Allocate memory in WASM for input
	inputPtrResult, err := allocateFunc.Call(store, inputLen)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to allocate input memory: %v", err), http.StatusInternalServerError)
		return
	}
	inputPtr := inputPtrResult.(int32)

	// Get WASM memory and copy input string
	memory := instance.GetExport(store, "memory").Memory()
	data := memory.UnsafeData(store)
	copy(data[inputPtr:inputPtr+int32(inputLen)], []byte(inputStr))

	// Call WASM 'run' function with pointer and length
	resultLenResult, err := runFunc.Call(store, inputPtr, inputLen)
	if err != nil {
		// Clean up allocated memory
		deallocateFunc.Call(store, inputPtr, inputLen)
		http.Error(w, fmt.Sprintf("execution failed: %v", err), http.StatusInternalServerError)
		return
	}
	resultLen := resultLenResult.(int32)

	// Get result pointer
	resultPtrResult, err := getResultPtrFunc.Call(store)
	if err != nil {
		deallocateFunc.Call(store, inputPtr, inputLen)
		http.Error(w, fmt.Sprintf("failed to get result pointer: %v", err), http.StatusInternalServerError)
		return
	}
	resultPtr := resultPtrResult.(int32)

	// Read result from WASM memory
	resultBytes := make([]byte, resultLen)
	copy(resultBytes, data[resultPtr:resultPtr+resultLen])
	resultStr := string(resultBytes)

	// Clean up allocated input memory
	deallocateFunc.Call(store, inputPtr, inputLen)

	// Return output
	output := map[string]interface{}{
		"output": resultStr,
		"status": "success",
	}
	json.NewEncoder(w).Encode(output)
}

func submitAppSrcHandler(w http.ResponseWriter, r *http.Request) {
	var app App
	if err := json.NewDecoder(r.Body).Decode(&app); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate source language
	if strings.ToLower(app.SourceLanguage) != "rust" {
		http.Error(w, "only Rust source code is supported", http.StatusBadRequest)
		return
	}

	// Validate that files are provided
	if len(app.Files) == 0 {
		http.Error(w, "no source files provided", http.StatusBadRequest)
		return
	}

	// Check if compiled WASM already exists in artifacts
	artifactsDir := filepath.Join("artifacts", app.AppID)
	wasmFilename := fmt.Sprintf("%s.wasm", app.Version)
	finalWasmPath := filepath.Join(artifactsDir, wasmFilename)

	if _, err := os.Stat(finalWasmPath); err == nil {
		http.Error(w, fmt.Sprintf("app %s version %s already compiled and exists", app.AppID, app.Version), http.StatusConflict)
		return
	}

	// Create temporary build directory structure
	buildDir := filepath.Join("build", app.AppID, app.Version)

	// Clear build directory if it exists (allow resubmission for build issues)
	if _, err := os.Stat(buildDir); err == nil {
		if err := os.RemoveAll(buildDir); err != nil {
			http.Error(w, fmt.Sprintf("failed to clear existing build directory: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Create build directory
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		http.Error(w, fmt.Sprintf("failed to create build directory: %v", err), http.StatusInternalServerError)
		return
	}

	// Create files from JSON spec
	if err := createFilesFromSpec(app.Files, buildDir); err != nil {
		// Clean up on error
		os.RemoveAll(buildDir)
		http.Error(w, fmt.Sprintf("failed to create source files: %v", err), http.StatusInternalServerError)
		return
	}

	// Compile the Rust source code to WASM
	wasmPath, err := buildRustToWasm(buildDir)
	if err != nil {
		// Clean up on error
		os.RemoveAll(buildDir)
		http.Error(w, fmt.Sprintf("failed to compile Rust to WASM: %v", err), http.StatusInternalServerError)
		return
	}

	// Register the app in the registry with the compiled WASM path
	registryMutex.Lock()
	registry[app.AppID] = &App{
		AppID:          app.AppID,
		Version:        app.Version,
		Runtime:        app.Runtime,
		Tools:          app.Tools,
		ArtifactURI:    wasmPath,
		SourceLanguage: app.SourceLanguage,
		Files:          app.Files,
	}
	registryMutex.Unlock()

	// Save registry to file
	if err := saveRegistry(); err != nil {
		log.Printf("Warning: failed to save registry: %v", err)
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "app source submitted and compiled",
		"wasmPath": wasmPath,
	})
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
