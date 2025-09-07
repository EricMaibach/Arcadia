package handlers

import (
	"encoding/json"
	"net/http"

	"arcadia/services"
)

// Dependencies for file watcher handlers
var (
	fileWatcher services.FileWatcher
)

// Request/Response types for file watcher endpoints
type AddWatchDirRequest struct {
	Path string `json:"path"`
}

type RemoveWatchDirRequest struct {
	Path string `json:"path"`
}

type WatchedDirResponse struct {
	Directories []string `json:"directories"`
}

// SetFileWatcherDependencies injects the file watcher service
func SetFileWatcherDependencies(fw services.FileWatcher) {
	fileWatcher = fw
}

// AddWatchDirHandler handles POST /filewatcher/add
func AddWatchDirHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AddWatchDirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	if err := fileWatcher.AddDirectory(req.Path); err != nil {
		http.Error(w, "Failed to add directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Directory added to watch list"})
}

// RemoveWatchDirHandler handles POST /filewatcher/remove
func RemoveWatchDirHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RemoveWatchDirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	if err := fileWatcher.RemoveDirectory(req.Path); err != nil {
		http.Error(w, "Failed to remove directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Directory removed from watch list"})
}

// ListWatchedDirsHandler handles GET /filewatcher/list
func ListWatchedDirsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	directories, err := fileWatcher.GetWatchedDirectories()
	if err != nil {
		http.Error(w, "Failed to get watched directories: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := WatchedDirResponse{
		Directories: directories,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}