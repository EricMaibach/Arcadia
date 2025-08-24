package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/bytecodealliance/wasmtime-go"
)

// --- Data structures ---

type App struct {
	AppID       string   `json:"appId"`
	Version     string   `json:"version"`
	Runtime     string   `json:"runtime"`
	Tools       []string `json:"tools"`
	ArtifactURI string   `json:"artifactUri"`
}

var (
	registry      = make(map[string]*App)
	registryMutex sync.RWMutex
	wasmEngine    = wasmtime.NewEngine()
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

type SubmitAppRequest struct {
	Spec        App    `json:"spec"`
	ArtifactURI string `json:"artifactUri"`
}

func submitAppHandler(w http.ResponseWriter, r *http.Request) {
	var req SubmitAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	registryMutex.Lock()
	registry[req.Spec.AppID] = &App{
		AppID:       req.Spec.AppID,
		Version:     req.Spec.Version,
		Runtime:     req.Spec.Runtime,
		Tools:       req.Spec.Tools,
		ArtifactURI: req.ArtifactURI,
	}
	registryMutex.Unlock()

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "app submitted"})
}

// --- Main ---

func main() {
	http.HandleFunc("/list_apps", listAppsHandler)
	http.HandleFunc("/run_tool", runToolHandler)
	http.HandleFunc("/submit_app", submitAppHandler)

	port := "8080"
	log.Printf("Starting Phase 0 runtime server on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
