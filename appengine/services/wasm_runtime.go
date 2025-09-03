package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/bytecodealliance/wasmtime-go"
)

// WasmRuntime handles WASM execution and host function management
type WasmRuntime struct {
	engine          *wasmtime.Engine
	configManager   WasmConfigManager
	registryManager *RegistryManager
	databaseManager *DatabaseManager
}

// WasmConfigManager interface for accessing configuration (renamed to avoid conflict)
type WasmConfigManager interface {
	GetClaudeService() *ClaudeService
}

// NewWasmRuntime creates a new WASM runtime instance
func NewWasmRuntime(configMgr WasmConfigManager, registryMgr *RegistryManager, dbMgr *DatabaseManager) *WasmRuntime {
	return &WasmRuntime{
		engine:          wasmtime.NewEngine(),
		configManager:   configMgr,
		registryManager: registryMgr,
		databaseManager: dbMgr,
	}
}

// GetEngine returns the WASM engine
func (wr *WasmRuntime) GetEngine() *wasmtime.Engine {
	return wr.engine
}

// claudeQuery sends a message to Claude AI and returns the response
func (wr *WasmRuntime) claudeQueryWithAppID(caller *wasmtime.Caller, messagePtr, messageLen, resultPtrPtr int32, appID string) int32 {
	// Get memory instance
	memory := caller.GetExport("memory").Memory()
	data := memory.UnsafeData(caller)

	// Read message string from WASM memory
	messageBytes := data[messagePtr : messagePtr+messageLen]
	message := string(messageBytes)

	log.Printf("[WASM Claude] claudeQuery called from app %s with message: %s", appID, message)

	// Check if Claude service is available
	claudeService := wr.configManager.GetClaudeService()
	if claudeService == nil {
		log.Printf("[WASM Claude] Claude service not initialized")
		return -1 // Claude service not initialized
	}

	// Generate context ID for this WASM app
	contextID := "wasm:" + appID

	// Send message to Claude with app-specific context
	response, err := claudeService.SendMessageWithContext(message, contextID)
	if err != nil {
		log.Printf("[WASM Claude] Claude API error: %v", err)
		return -2 // Claude API error
	}

	// Create response JSON
	responseObj := map[string]interface{}{
		"response": response,
		"success":  true,
	}

	// Marshal response to JSON
	jsonBytes, err := json.Marshal(responseObj)
	if err != nil {
		log.Printf("[WASM Claude] JSON marshal error: %v", err)
		return -3 // JSON marshal error
	}

	// Allocate memory in WASM for result
	allocateFunc := caller.GetExport("allocate").Func()
	resultLenResult, err := allocateFunc.Call(caller, len(jsonBytes))
	if err != nil {
		log.Printf("[WASM Claude] Memory allocation error: %v", err)
		return -4 // Allocation error
	}
	resultPtr := resultLenResult.(int32)

	// Copy result to WASM memory
	copy(data[resultPtr:resultPtr+int32(len(jsonBytes))], jsonBytes)

	// Store result pointer in the output parameter
	resultPtrPtrBytes := data[resultPtrPtr : resultPtrPtr+4]
	resultPtrPtrBytes[0] = byte(resultPtr)
	resultPtrPtrBytes[1] = byte(resultPtr >> 8)
	resultPtrPtrBytes[2] = byte(resultPtr >> 16)
	resultPtrPtrBytes[3] = byte(resultPtr >> 24)

	log.Printf("[WASM Claude] Claude query completed successfully")
	return int32(len(jsonBytes))
}

// dbQuery executes a database query for WASM apps
func (wr *WasmRuntime) dbQuery(caller *wasmtime.Caller, queryPtr, queryLen, resultPtrPtr int32) int32 {
	// Get memory instance
	memory := caller.GetExport("memory").Memory()
	data := memory.UnsafeData(caller)

	// Read query string from WASM memory
	queryBytes := data[queryPtr : queryPtr+queryLen]
	query := string(queryBytes)

	log.Printf("[WASM DB] dbQuery called with query: %s", query)

	if wr.databaseManager == nil {
		log.Printf("[WASM DB] Database manager not initialized")
		return -1 // Database not initialized
	}

	// Execute query using app database
	rows, err := wr.databaseManager.GetAppDB().Query(query)
	if err != nil {
		log.Printf("[WASM DB] Database query error: %v", err)
		return -2 // Query error
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		log.Printf("[WASM DB] Column error: %v", err)
		return -3 // Column error
	}
	log.Printf("[WASM DB] Query columns: %v", columns)

	// Collect results
	results := []map[string]interface{}{}
	log.Printf("[WASM DB] Query returned %+v rows", results)
	rowCount := 0
	for rows.Next() {
		rowCount++
		// Create a slice to hold column values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			log.Printf("[WASM DB] Row scan error: %v", err)
			return -4 // Scan error
		}

		// Convert to map
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if val != nil {
				// Convert byte slices to strings for JSON serialization
				if bytes, ok := val.([]byte); ok {
					val = string(bytes)
				}
			}
			row[col] = val
		}
		results = append(results, row)
		log.Printf("[WASM DB] Row %d: %+v", rowCount, row)
	}

	log.Printf("[WASM DB] Total rows returned: %d", rowCount)

	// Convert results to JSON
	jsonBytes, err := json.Marshal(results)
	if err != nil {
		log.Printf("[WASM DB] JSON marshal error: %v", err)
		return -5 // JSON error
	}

	log.Printf("[WASM DB] JSON result: %s", string(jsonBytes))

	// Allocate memory in WASM for result
	allocateFunc := caller.GetExport("allocate").Func()
	if allocateFunc == nil {
		log.Printf("[WASM DB] allocate function not found")
		return -6 // Allocate function not found
	}

	resultPtrResult, err := allocateFunc.Call(caller, len(jsonBytes))
	if err != nil {
		log.Printf("[WASM DB] Failed to allocate memory: %v", err)
		return -7 // Memory allocation error
	}
	resultPtr := resultPtrResult.(int32)

	// Copy JSON data to WASM memory
	copy(data[resultPtr:], jsonBytes)

	// Write result pointer to specified location
	resultPtrPtrBytes := data[resultPtrPtr : resultPtrPtr+4]
	resultPtrPtrBytes[0] = byte(resultPtr)
	resultPtrPtrBytes[1] = byte(resultPtr >> 8)
	resultPtrPtrBytes[2] = byte(resultPtr >> 16)
	resultPtrPtrBytes[3] = byte(resultPtr >> 24)

	log.Printf("[WASM DB] Query completed successfully")
	return int32(len(jsonBytes))
}

// dbExec executes a database statement for WASM apps
func (wr *WasmRuntime) dbExec(caller *wasmtime.Caller, stmtPtr, stmtLen int32) int32 {
	// Get memory instance
	memory := caller.GetExport("memory").Memory()
	data := memory.UnsafeData(caller)

	// Read statement string from WASM memory
	stmtBytes := data[stmtPtr : stmtPtr+stmtLen]
	stmt := string(stmtBytes)

	if wr.databaseManager == nil {
		return -1 // Database not initialized
	}

	// Execute statement using app database
	result, err := wr.databaseManager.GetAppDB().Exec(stmt)
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

// dbPreparedQuery executes a prepared statement with parameters for WASM apps
func (wr *WasmRuntime) dbPreparedQuery(caller *wasmtime.Caller, stmtPtr, stmtLen, paramsPtr, paramsLen, resultPtrPtr int32) int32 {
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

	if wr.databaseManager == nil {
		return -2 // Database not initialized
	}

	// Execute prepared statement using app database
	rows, err := wr.databaseManager.GetAppDB().Query(stmt, params...)
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
			return -5 // Scan error
		}

		// Convert to map
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if val != nil {
				// Convert byte slices to strings for JSON serialization
				if bytes, ok := val.([]byte); ok {
					val = string(bytes)
				}
			}
			row[col] = val
		}
		results = append(results, row)
	}

	// Convert results to JSON and return (same as dbQuery)
	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return -6 // JSON error
	}

	// Allocate memory in WASM for result
	allocateFunc := caller.GetExport("allocate").Func()
	if allocateFunc == nil {
		return -7 // Allocate function not found
	}

	resultPtrResult, err := allocateFunc.Call(caller, len(jsonBytes))
	if err != nil {
		return -8 // Memory allocation error
	}
	resultPtr := resultPtrResult.(int32)

	// Copy JSON data to WASM memory
	copy(data[resultPtr:], jsonBytes)

	// Write result pointer to specified location
	resultPtrPtrBytes := data[resultPtrPtr : resultPtrPtr+4]
	resultPtrPtrBytes[0] = byte(resultPtr)
	resultPtrPtrBytes[1] = byte(resultPtr >> 8)
	resultPtrPtrBytes[2] = byte(resultPtr >> 16)
	resultPtrPtrBytes[3] = byte(resultPtr >> 24)

	return int32(len(jsonBytes))
}

// setupHostFunctions configures all host functions for WASM execution
func (wr *WasmRuntime) setupHostFunctions(linker *wasmtime.Linker, store *wasmtime.Store, appID string) {
	// Define database host functions
	linker.DefineFunc(store, "env", "db_query", func(caller *wasmtime.Caller, queryPtr, queryLen, resultPtrPtr int32) int32 {
		return wr.dbQuery(caller, queryPtr, queryLen, resultPtrPtr)
	})
	linker.DefineFunc(store, "env", "db_exec", func(caller *wasmtime.Caller, stmtPtr, stmtLen int32) int32 {
		return wr.dbExec(caller, stmtPtr, stmtLen)
	})
	linker.DefineFunc(store, "env", "db_prepared_query", func(caller *wasmtime.Caller, stmtPtr, stmtLen, paramsPtr, paramsLen, resultPtrPtr int32) int32 {
		return wr.dbPreparedQuery(caller, stmtPtr, stmtLen, paramsPtr, paramsLen, resultPtrPtr)
	})

	// Define Claude AI host function with app-specific context
	linker.DefineFunc(store, "env", "claude_query", func(caller *wasmtime.Caller, messagePtr, messageLen, resultPtrPtr int32) int32 {
		return wr.claudeQueryWithAppID(caller, messagePtr, messageLen, resultPtrPtr, appID)
	})
}

// ExecuteAppTool executes a tool from an app using WASM runtime
func (wr *WasmRuntime) ExecuteAppTool(appID, toolName string, input json.RawMessage) (string, error) {
	// Get app from registry
	registry := wr.registryManager.GetRegistry()
	app, ok := registry.GetApp(appID)
	if !ok {
		return "", fmt.Errorf("app not found: %s", appID)
	}

	// Validate tool exists
	toolFound := false
	for _, tool := range app.Tools {
		if tool.Name == toolName {
			toolFound = true
			break
		}
	}
	if !toolFound {
		return "", fmt.Errorf("tool '%s' not found in app", toolName)
	}

	// Load and compile WASM module
	wasmBytes, err := os.ReadFile(app.ArtifactURI)
	if err != nil {
		return "", fmt.Errorf("failed to load WASM artifact: %v", err)
	}

	store := wasmtime.NewStore(wr.engine)
	module, err := wasmtime.NewModule(wr.engine, wasmBytes)
	if err != nil {
		return "", fmt.Errorf("failed to compile WASM module: %v", err)
	}

	linker := wasmtime.NewLinker(wr.engine)

	// Setup all host functions with app ID for context isolation
	wr.setupHostFunctions(linker, store, appID)

	instance, err := linker.Instantiate(store, module)
	if err != nil {
		return "", fmt.Errorf("failed to instantiate WASM module: %v", err)
	}

	runFunc := instance.GetFunc(store, "run")
	if runFunc == nil {
		return "", fmt.Errorf("WASM module does not export 'run' function")
	}

	allocateFunc := instance.GetFunc(store, "allocate")
	deallocateFunc := instance.GetFunc(store, "deallocate")
	getResultPtrFunc := instance.GetFunc(store, "get_result_ptr")

	if allocateFunc == nil || deallocateFunc == nil || getResultPtrFunc == nil {
		return "", fmt.Errorf("WASM module missing required functions")
	}

	// Prepare input
	wasmInput := map[string]interface{}{
		"tool": toolName,
		"data": input,
	}
	inputBytes, _ := json.Marshal(wasmInput)
	inputStr := string(inputBytes)
	inputLen := len(inputStr)

	// Allocate memory and copy input
	inputPtrResult, err := allocateFunc.Call(store, inputLen)
	if err != nil {
		return "", fmt.Errorf("failed to allocate input memory: %v", err)
	}
	inputPtr := inputPtrResult.(int32)

	memory := instance.GetExport(store, "memory").Memory()
	data := memory.UnsafeData(store)
	copy(data[inputPtr:inputPtr+int32(inputLen)], []byte(inputStr))

	// Execute
	resultLenResult, err := runFunc.Call(store, inputPtr, inputLen)
	if err != nil {
		deallocateFunc.Call(store, inputPtr, inputLen)
		return "", fmt.Errorf("execution failed: %v", err)
	}
	resultLen := resultLenResult.(int32)

	// Get result
	resultPtrResult, err := getResultPtrFunc.Call(store)
	if err != nil {
		deallocateFunc.Call(store, inputPtr, inputLen)
		return "", fmt.Errorf("failed to get result pointer: %v", err)
	}
	resultPtr := resultPtrResult.(int32)

	resultBytes := make([]byte, resultLen)
	copy(resultBytes, data[resultPtr:resultPtr+resultLen])
	resultStr := string(resultBytes)

	// Clean up
	deallocateFunc.Call(store, inputPtr, inputLen)

	return resultStr, nil
}

// Global runtime instance for backward compatibility
var globalRuntime *WasmRuntime

// InitializeWasmRuntime initializes the global WASM runtime
func InitializeWasmRuntime(configMgr WasmConfigManager, registryMgr *RegistryManager, dbMgr *DatabaseManager) {
	globalRuntime = NewWasmRuntime(configMgr, registryMgr, dbMgr)
}

// GetGlobalWasmRuntime returns the global WASM runtime instance
func GetGlobalWasmRuntime() *WasmRuntime {
	if globalRuntime == nil {
		panic("WASM runtime not initialized. Call InitializeWasmRuntime first.")
	}
	return globalRuntime
}

// Legacy compatibility wrapper for scheduler and other services
func WasmExecuteAppTool(appID, toolName string, input json.RawMessage) (string, error) {
	return GetGlobalWasmRuntime().ExecuteAppTool(appID, toolName, input)
}