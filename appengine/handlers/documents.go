package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"arcadia/modules/documents"
	"arcadia/modules/documents/models"
	"arcadia/pkg/logging"
)

// Dependencies for document handlers
var (
	documentsService   documents.DocumentsModule
	documentsServiceMu sync.RWMutex
	documentsLogger    logging.Logger
	documentsLoggerMu  sync.RWMutex
)

// SetDocumentsDependencies configures the documents service dependency for the document handlers
func SetDocumentsDependencies(service documents.DocumentsModule, logger logging.Logger) {
	documentsServiceMu.Lock()
	documentsService = service
	documentsServiceMu.Unlock()

	documentsLoggerMu.Lock()
	documentsLogger = logger
	documentsLoggerMu.Unlock()
}

// getDocumentsService safely returns the documents service with proper locking
func getDocumentsService() documents.DocumentsModule {
	documentsServiceMu.RLock()
	defer documentsServiceMu.RUnlock()
	return documentsService
}

// getDocumentsLogger safely returns the logger with proper locking
func getDocumentsLogger() logging.Logger {
	documentsLoggerMu.RLock()
	defer documentsLoggerMu.RUnlock()
	return documentsLogger
}

// HandleSearchDocuments handles basic document search requests
// POST /api/documents/v1/search
func HandleSearchDocuments(w http.ResponseWriter, r *http.Request) {
	service := getDocumentsService()
	if service == nil {
		http.Error(w, "Documents service not initialized", http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate query
	if request.Query == "" {
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}

	// Default and validate top_k
	if request.TopK == 0 {
		request.TopK = 5
	}
	if request.TopK < 1 || request.TopK > 10 {
		http.Error(w, "top_k must be between 1 and 10", http.StatusBadRequest)
		return
	}

	// Log the search request
	if logger := getDocumentsLogger(); logger != nil {
		logger.Info(r.Context(), "Document search request",
			"query", request.Query,
			"top_k", request.TopK,
			"client_ip", r.RemoteAddr)
	}

	// Perform search with timing
	startTime := time.Now()
	results, err := service.SearchDocuments(r.Context(), request.Query, request.TopK)
	searchTime := time.Since(startTime).Milliseconds()

	if err != nil {
		if logger := getDocumentsLogger(); logger != nil {
			logger.Error(r.Context(), "Search failed",
				"query", request.Query,
				"top_k", request.TopK,
				"error", err,
				"duration_ms", searchTime)
		}
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	// Log successful search
	if logger := getDocumentsLogger(); logger != nil {
		logger.Info(r.Context(), "Search completed",
			"query", request.Query,
			"results_count", len(results),
			"duration_ms", searchTime)
	}

	// Format response
	response := map[string]interface{}{
		"query":           request.Query,
		"results":         results,
		"total_documents": len(results),
		"search_time_ms":  searchTime,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		if logger := getDocumentsLogger(); logger != nil {
			logger.Error(r.Context(), "Failed to encode response", "error", err)
		}
	}
}

// HandleSearchDocumentsEnhanced handles enhanced document search with configuration
// POST /api/documents/v1/search/enhanced
func HandleSearchDocumentsEnhanced(w http.ResponseWriter, r *http.Request) {
	service := getDocumentsService()
	if service == nil {
		http.Error(w, "Documents service not initialized", http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Query  string              `json:"query"`
		TopK   int                 `json:"top_k"`
		Config models.SearchConfig `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate query
	if request.Query == "" {
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}

	// Default and validate top_k
	if request.TopK == 0 {
		request.TopK = 5
	}
	if request.TopK < 1 || request.TopK > 10 {
		http.Error(w, "top_k must be between 1 and 10", http.StatusBadRequest)
		return
	}

	// Use default config if not provided or has zero values
	searchConfig := request.Config
	if searchConfig.MaxDocumentSize == 0 && searchConfig.MaxHighlights == 0 {
		searchConfig = models.DefaultSearchConfig()
	} else {
		// Fill in any missing values with defaults
		defaultConfig := models.DefaultSearchConfig()
		if searchConfig.MaxDocumentSize == 0 {
			searchConfig.MaxDocumentSize = defaultConfig.MaxDocumentSize
		}
		if searchConfig.MaxHighlights == 0 {
			searchConfig.MaxHighlights = defaultConfig.MaxHighlights
		}
		// IncludeFullContent defaults to false, so it's fine if not specified
	}

	// Log the enhanced search request
	if logger := getDocumentsLogger(); logger != nil {
		logger.Info(r.Context(), "Enhanced document search request",
			"query", request.Query,
			"top_k", request.TopK,
			"max_document_size", searchConfig.MaxDocumentSize,
			"max_highlights", searchConfig.MaxHighlights,
			"include_full_content", searchConfig.IncludeFullContent,
			"client_ip", r.RemoteAddr)
	}

	// Perform enhanced search with timing
	startTime := time.Now()
	results, err := service.SearchDocumentsEnhanced(
		r.Context(),
		request.Query,
		request.TopK,
		searchConfig,
	)
	searchTime := time.Since(startTime).Milliseconds()

	if err != nil {
		if logger := getDocumentsLogger(); logger != nil {
			logger.Error(r.Context(), "Enhanced search failed",
				"query", request.Query,
				"top_k", request.TopK,
				"error", err,
				"duration_ms", searchTime)
		}
		http.Error(w, "Enhanced search failed", http.StatusInternalServerError)
		return
	}

	// Log successful enhanced search
	if logger := getDocumentsLogger(); logger != nil {
		logger.Info(r.Context(), "Enhanced search completed",
			"query", request.Query,
			"results_count", len(results),
			"duration_ms", searchTime)
	}

	// Format response
	response := map[string]interface{}{
		"query":           request.Query,
		"results":         results,
		"total_documents": len(results),
		"search_time_ms":  searchTime,
		"search_config":   searchConfig,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		if logger := getDocumentsLogger(); logger != nil {
			logger.Error(r.Context(), "Failed to encode response", "error", err)
		}
	}
}
