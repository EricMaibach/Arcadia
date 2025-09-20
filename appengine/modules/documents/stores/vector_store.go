package stores

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// VectorStoreConfig contains configuration for vector stores
type VectorStoreConfig struct {
	Type         string                 `json:"type"` // "memory", "qdrant", "pinecone", etc.
	Host         string                 `json:"host,omitempty"`
	Port         int                    `json:"port,omitempty"`
	ApiKey       string                 `json:"api_key,omitempty"`
	Collection   string                 `json:"collection,omitempty"`
	Dimension    int                    `json:"dimension"`
	Metric       string                 `json:"metric"` // "cosine", "euclidean", "dot"
	BatchSize    int                    `json:"batch_size"`
	Timeout      int                    `json:"timeout_seconds"`
	MaxRetries   int                    `json:"max_retries"`
	RetryDelay   int                    `json:"retry_delay_seconds"`
	CustomConfig map[string]interface{} `json:"custom_config,omitempty"`
}

// MemoryVectorStore implements VectorStoreInterface using in-memory storage
// This is primarily for testing and development
type MemoryVectorStore struct {
	vectors map[string]*models.VectorEntry
	mutex   sync.RWMutex
	config  VectorStoreConfig
	logger  interfaces.Logger
	metrics interfaces.MetricsCollector
}

// NewMemoryVectorStore creates a new in-memory vector store
func NewMemoryVectorStore(config VectorStoreConfig) *MemoryVectorStore {
	return &MemoryVectorStore{
		vectors: make(map[string]*models.VectorEntry),
		config:  config,
	}
}

// WithLogger adds logging to the memory vector store
func (mvs *MemoryVectorStore) WithLogger(logger interfaces.Logger) *MemoryVectorStore {
	mvs.logger = logger
	return mvs
}

// WithMetrics adds metrics collection to the memory vector store
func (mvs *MemoryVectorStore) WithMetrics(metrics interfaces.MetricsCollector) *MemoryVectorStore {
	mvs.metrics = metrics
	return mvs
}

// Initialize initializes the memory vector store (no-op for memory store)
func (mvs *MemoryVectorStore) Initialize(ctx context.Context) error {
	// Memory store doesn't need initialization - everything is already set up
	if mvs.logger != nil {
		mvs.logger.Debug(ctx, "Memory vector store initialized", "dimension", mvs.config.Dimension)
	}
	return nil
}

// StoreVector stores a vector entry
func (mvs *MemoryVectorStore) StoreVector(ctx context.Context, entry *models.VectorEntry) error {
	if entry == nil {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "vector entry cannot be nil")
	}

	if entry.ID == "" {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "vector entry ID cannot be empty")
	}

	if len(entry.Vector) == 0 {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "vector cannot be empty")
	}

	// Validate dimension
	if mvs.config.Dimension > 0 && len(entry.Vector) != mvs.config.Dimension {
		return models.NewDocumentError(models.ErrVectorStoreFailed,
			fmt.Sprintf("vector dimension mismatch: expected %d, got %d", mvs.config.Dimension, len(entry.Vector)))
	}

	mvs.mutex.Lock()
	defer mvs.mutex.Unlock()

	// Store a copy to avoid external modifications
	entryCopy := *entry
	entryCopy.Vector = make([]float32, len(entry.Vector))
	copy(entryCopy.Vector, entry.Vector)

	mvs.vectors[entry.ID] = &entryCopy

	if mvs.metrics != nil {
		mvs.metrics.IncrementCounter("vector_store.store", map[string]string{"status": "success"})
	}

	return nil
}

// SearchSimilar searches for similar vectors
func (mvs *MemoryVectorStore) SearchSimilar(ctx context.Context, queryVector []float32, topK int) ([]*models.SearchResult, error) {
	if len(queryVector) == 0 {
		return nil, models.NewDocumentError(models.ErrSearchFailed, "query vector cannot be empty")
	}

	if topK <= 0 {
		return []*models.SearchResult{}, nil
	}

	mvs.mutex.RLock()
	defer mvs.mutex.RUnlock()

	startTime := time.Now()
	defer func() {
		if mvs.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			mvs.metrics.RecordTimer("vector_store.search.duration", duration, map[string]string{"type": "memory"})
		}
	}()

	var results []*models.SearchResult

	// Calculate similarity for all vectors
	for _, entry := range mvs.vectors {
		if len(entry.Vector) != len(queryVector) {
			continue // Skip vectors with different dimensions
		}

		similarity := mvs.calculateSimilarity(queryVector, entry.Vector)

		result := &models.SearchResult{
			Entry: entry,
			Score: similarity,
		}

		results = append(results, result)
	}

	// Sort by similarity (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top K results
	if len(results) > topK {
		results = results[:topK]
	}

	if mvs.metrics != nil {
		mvs.metrics.IncrementCounter("vector_store.search", map[string]string{
			"status":  "success",
			"results": fmt.Sprintf("%d", len(results)),
		})
	}

	return results, nil
}

// GetVector retrieves a vector by ID
func (mvs *MemoryVectorStore) GetVector(ctx context.Context, id string) (*models.VectorEntry, error) {
	if id == "" {
		return nil, models.NewDocumentError(models.ErrVectorStoreFailed, "vector ID cannot be empty")
	}

	mvs.mutex.RLock()
	defer mvs.mutex.RUnlock()

	entry, exists := mvs.vectors[id]
	if !exists {
		return nil, models.NewDocumentError(models.ErrDocumentNotFound, fmt.Sprintf("vector not found: %s", id))
	}

	// Return a copy to avoid external modifications
	entryCopy := *entry
	entryCopy.Vector = make([]float32, len(entry.Vector))
	copy(entryCopy.Vector, entry.Vector)

	return &entryCopy, nil
}

// DeleteVector deletes a vector by ID
func (mvs *MemoryVectorStore) DeleteVector(ctx context.Context, id string) error {
	if id == "" {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "vector ID cannot be empty")
	}

	mvs.mutex.Lock()
	defer mvs.mutex.Unlock()

	if _, exists := mvs.vectors[id]; !exists {
		return models.NewDocumentError(models.ErrDocumentNotFound, fmt.Sprintf("vector not found: %s", id))
	}

	delete(mvs.vectors, id)

	if mvs.metrics != nil {
		mvs.metrics.IncrementCounter("vector_store.delete", map[string]string{"status": "success"})
	}

	return nil
}

// DeleteDocumentVectors deletes all vectors for a document
func (mvs *MemoryVectorStore) DeleteDocumentVectors(ctx context.Context, documentID string) error {
	if documentID == "" {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "document ID cannot be empty")
	}

	mvs.mutex.Lock()
	defer mvs.mutex.Unlock()

	var deletedCount int
	for id, entry := range mvs.vectors {
		if entry.DocumentID == documentID {
			delete(mvs.vectors, id)
			deletedCount++
		}
	}

	if mvs.metrics != nil {
		mvs.metrics.IncrementCounter("vector_store.delete_document", map[string]string{
			"status": "success",
			"count":  fmt.Sprintf("%d", deletedCount),
		})
	}

	return nil
}

// GetDocumentVectors retrieves all vectors for a document
func (mvs *MemoryVectorStore) GetDocumentVectors(ctx context.Context, documentID string) ([]*models.VectorEntry, error) {
	if documentID == "" {
		return nil, models.NewDocumentError(models.ErrVectorStoreFailed, "document ID cannot be empty")
	}

	mvs.mutex.RLock()
	defer mvs.mutex.RUnlock()

	var vectors []*models.VectorEntry

	for _, entry := range mvs.vectors {
		if entry.DocumentID == documentID {
			// Return a copy to avoid external modifications
			entryCopy := *entry
			entryCopy.Vector = make([]float32, len(entry.Vector))
			copy(entryCopy.Vector, entry.Vector)
			vectors = append(vectors, &entryCopy)
		}
	}

	// Sort by chunk index if metadata is available
	sort.Slice(vectors, func(i, j int) bool {
		// Try to extract chunk index from metadata for sorting
		return vectors[i].CreatedAt.Before(vectors[j].CreatedAt)
	})

	return vectors, nil
}

// Clear removes all vectors from the store
func (mvs *MemoryVectorStore) Clear(ctx context.Context) error {
	mvs.mutex.Lock()
	defer mvs.mutex.Unlock()

	count := len(mvs.vectors)
	mvs.vectors = make(map[string]*models.VectorEntry)

	if mvs.logger != nil {
		mvs.logger.Info(ctx, "Cleared vector store", "deleted_count", count)
	}

	if mvs.metrics != nil {
		mvs.metrics.IncrementCounter("vector_store.clear", map[string]string{
			"count": fmt.Sprintf("%d", count),
		})
	}

	return nil
}

// StoreBatch stores multiple vectors in a batch
func (mvs *MemoryVectorStore) StoreBatch(ctx context.Context, entries []*models.VectorEntry) error {
	if len(entries) == 0 {
		return nil
	}

	mvs.mutex.Lock()
	defer mvs.mutex.Unlock()

	var successCount int

	for _, entry := range entries {
		if entry == nil || entry.ID == "" || len(entry.Vector) == 0 {
			continue
		}

		// Validate dimension
		if mvs.config.Dimension > 0 && len(entry.Vector) != mvs.config.Dimension {
			continue
		}

		// Store a copy to avoid external modifications
		entryCopy := *entry
		entryCopy.Vector = make([]float32, len(entry.Vector))
		copy(entryCopy.Vector, entry.Vector)

		mvs.vectors[entry.ID] = &entryCopy
		successCount++
	}

	if mvs.metrics != nil {
		mvs.metrics.IncrementCounter("vector_store.store_batch", map[string]string{
			"count":  fmt.Sprintf("%d", successCount),
			"total":  fmt.Sprintf("%d", len(entries)),
			"status": "success",
		})
	}

	return nil
}

// SearchWithFilter searches for similar vectors with filtering
func (mvs *MemoryVectorStore) SearchWithFilter(ctx context.Context, queryVector []float32, topK int, filter map[string]interface{}) ([]*models.SearchResult, error) {
	if len(queryVector) == 0 {
		return nil, models.NewDocumentError(models.ErrSearchFailed, "query vector cannot be empty")
	}

	if topK <= 0 {
		return []*models.SearchResult{}, nil
	}

	mvs.mutex.RLock()
	defer mvs.mutex.RUnlock()

	var results []*models.SearchResult

	// Calculate similarity for all vectors that match the filter
	for _, entry := range mvs.vectors {
		if len(entry.Vector) != len(queryVector) {
			continue
		}

		// Apply filter
		if !mvs.matchesFilter(entry, filter) {
			continue
		}

		similarity := mvs.calculateSimilarity(queryVector, entry.Vector)

		result := &models.SearchResult{
			Entry: entry,
			Score: similarity,
		}

		results = append(results, result)
	}

	// Sort by similarity (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top K results
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// HealthCheck performs a health check on the vector store
func (mvs *MemoryVectorStore) HealthCheck(ctx context.Context) error {
	mvs.mutex.RLock()
	defer mvs.mutex.RUnlock()

	// For memory store, just check if we can access the vectors map
	_ = len(mvs.vectors)

	return nil
}

// GetStats returns statistics about the vector store
func (mvs *MemoryVectorStore) GetStats(ctx context.Context) (*VectorStoreStats, error) {
	mvs.mutex.RLock()
	defer mvs.mutex.RUnlock()

	stats := &VectorStoreStats{
		TotalVectors: int64(len(mvs.vectors)),
		Type:         "memory",
		Dimension:    mvs.config.Dimension,
		Metric:       mvs.config.Metric,
	}

	// Count unique documents
	documents := make(map[string]bool)
	var totalSize int64

	for _, entry := range mvs.vectors {
		documents[entry.DocumentID] = true
		totalSize += int64(len(entry.Vector) * 4) // 4 bytes per float32
	}

	stats.UniqueDocuments = int64(len(documents))
	stats.StorageSize = totalSize

	return stats, nil
}

// calculateSimilarity calculates similarity between two vectors based on the configured metric
func (mvs *MemoryVectorStore) calculateSimilarity(a, b []float32) float32 {
	switch mvs.config.Metric {
	case "euclidean":
		// Convert distance to similarity (higher is better)
		distance := euclideanDistance(a, b)
		return 1.0 / (1.0 + distance)
	case "dot":
		return dotProduct(a, b)
	default: // "cosine" or default
		return cosineSimilarity(a, b)
	}
}

// matchesFilter checks if a vector entry matches the given filter
func (mvs *MemoryVectorStore) matchesFilter(entry *models.VectorEntry, filter map[string]interface{}) bool {
	if filter == nil || len(filter) == 0 {
		return true
	}

	// Check document ID filter
	if docID, ok := filter["document_id"]; ok {
		if entry.DocumentID != docID {
			return false
		}
	}

	// Check chunk ID filter
	if chunkID, ok := filter["chunk_id"]; ok {
		if entry.ChunkID != chunkID {
			return false
		}
	}

	// Check content filter (simple substring match)
	if contentFilter, ok := filter["content"]; ok {
		if contentStr, ok := contentFilter.(string); ok {
			if !containsIgnoreCase(entry.Content, contentStr) {
				return false
			}
		}
	}

	return true
}

// VectorStoreStats contains statistics about a vector store
type VectorStoreStats struct {
	TotalVectors    int64  `json:"total_vectors"`
	UniqueDocuments int64  `json:"unique_documents"`
	StorageSize     int64  `json:"storage_size_bytes"`
	Type            string `json:"type"`
	Dimension       int    `json:"dimension"`
	Metric          string `json:"metric"`
}

// Utility functions for vector operations

// cosineSimilarity calculates cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt32(normA) * sqrt32(normB))
}

// euclideanDistance calculates Euclidean distance between two vectors
func euclideanDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return float32(1e9) // Large distance for mismatched dimensions
	}

	var sum float32
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return sqrt32(sum)
}

// dotProduct calculates dot product between two vectors
func dotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var product float32
	for i := range a {
		product += a[i] * b[i]
	}

	return product
}

// sqrt32 calculates square root for float32
func sqrt32(x float32) float32 {
	if x <= 0 {
		return 0
	}

	// Newton-Raphson method for square root
	guess := x
	for i := 0; i < 10; i++ {
		guess = (guess + x/guess) / 2
	}
	return guess
}

// containsIgnoreCase checks if s contains substr (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// DefaultVectorStoreConfig returns a default configuration for vector stores
func DefaultVectorStoreConfig() VectorStoreConfig {
	return VectorStoreConfig{
		Type:       "memory",
		Dimension:  384, // Default for all-MiniLM-L6-v2
		Metric:     "cosine",
		BatchSize:  100,
		Timeout:    30,
		MaxRetries: 3,
		RetryDelay: 5,
	}
}

// VectorStoreFactory creates vector stores based on configuration
type VectorStoreFactory struct {
	logger  interfaces.Logger
	metrics interfaces.MetricsCollector
}

// NewVectorStoreFactory creates a new vector store factory
func NewVectorStoreFactory() *VectorStoreFactory {
	return &VectorStoreFactory{}
}

// WithLogger adds logging to the factory
func (vsf *VectorStoreFactory) WithLogger(logger interfaces.Logger) *VectorStoreFactory {
	vsf.logger = logger
	return vsf
}

// WithMetrics adds metrics collection to the factory
func (vsf *VectorStoreFactory) WithMetrics(metrics interfaces.MetricsCollector) *VectorStoreFactory {
	vsf.metrics = metrics
	return vsf
}

// CreateVectorStore creates a vector store based on the configuration
func (vsf *VectorStoreFactory) CreateVectorStore(config VectorStoreConfig) (interfaces.VectorStoreInterface, error) {
	switch config.Type {
	case "memory":
		store := NewMemoryVectorStore(config)
		if vsf.logger != nil {
			store.WithLogger(vsf.logger)
		}
		if vsf.metrics != nil {
			store.WithMetrics(vsf.metrics)
		}
		return store, nil
	case "sqlite":
		// SQLite store requires database connection in CustomConfig
		if config.CustomConfig == nil {
			return nil, models.NewDocumentError(models.ErrInvalidConfig, "SQLite vector store requires database connection in custom config")
		}
		db, ok := config.CustomConfig["database"].(interfaces.DatabaseProvider)
		if !ok {
			return nil, models.NewDocumentError(models.ErrInvalidConfig, "SQLite vector store requires 'database' field in custom config")
		}
		store := NewSQLiteVectorStore(db, config)
		if vsf.logger != nil {
			store.WithLogger(vsf.logger)
		}
		if vsf.metrics != nil {
			store.WithMetrics(vsf.metrics)
		}
		return store, nil
	case "qdrant":
		// Convert VectorStoreConfig to QdrantConfig
		qdrantConfig := QdrantConfig{
			Host:       config.Host,
			Port:       config.Port,
			Collection: config.Collection,
			Dimension:  config.Dimension,
			UseHTTPS:   false, // Default to HTTP
			Timeout:    config.Timeout,
			MaxRetries: config.MaxRetries,
			RetryDelay: config.RetryDelay,
		}

		// Set defaults if not provided
		if qdrantConfig.Host == "" {
			qdrantConfig.Host = "localhost"
		}
		if qdrantConfig.Port == 0 {
			qdrantConfig.Port = 6334
		}
		if qdrantConfig.Collection == "" {
			qdrantConfig.Collection = "documents"
		}
		if qdrantConfig.Timeout == 0 {
			qdrantConfig.Timeout = 30
		}
		if qdrantConfig.MaxRetries == 0 {
			qdrantConfig.MaxRetries = 3
		}
		if qdrantConfig.RetryDelay == 0 {
			qdrantConfig.RetryDelay = 1
		}

		store, err := NewQdrantVectorStore(qdrantConfig)
		if err != nil {
			return nil, models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, "failed to create QDrant vector store", err)
		}

		if vsf.logger != nil {
			store.WithLogger(vsf.logger)
		}
		if vsf.metrics != nil {
			store.WithMetrics(vsf.metrics)
		}

		return store, nil
	default:
		return nil, models.NewDocumentError(models.ErrInvalidConfig, fmt.Sprintf("unsupported vector store type: %s", config.Type))
	}
}

// SQLiteVectorStore implements VectorStoreInterface using SQLite database
type SQLiteVectorStore struct {
	db      interfaces.DatabaseProvider
	config  VectorStoreConfig
	logger  interfaces.Logger
	metrics interfaces.MetricsCollector
	mutex   sync.RWMutex
}

// NewSQLiteVectorStore creates a new SQLite vector store
func NewSQLiteVectorStore(db interfaces.DatabaseProvider, config VectorStoreConfig) *SQLiteVectorStore {
	store := &SQLiteVectorStore{
		db:     db,
		config: config,
	}

	// Initialize tables
	store.initializeTables()
	return store
}

// WithLogger adds logging to the SQLite vector store
func (svs *SQLiteVectorStore) WithLogger(logger interfaces.Logger) *SQLiteVectorStore {
	svs.logger = logger
	return svs
}

// WithMetrics adds metrics collection to the SQLite vector store
func (svs *SQLiteVectorStore) WithMetrics(metrics interfaces.MetricsCollector) *SQLiteVectorStore {
	svs.metrics = metrics
	return svs
}

// Initialize initializes the SQLite vector store by creating necessary tables
func (svs *SQLiteVectorStore) Initialize(ctx context.Context) error {
	if svs.db == nil {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "database not initialized")
	}

	// Create tables and indexes if they don't exist
	svs.initializeTables()

	if svs.logger != nil {
		svs.logger.Info(ctx, "SQLite vector store initialized", "dimension", svs.config.Dimension)
	}

	return nil
}

// initializeTables creates the necessary tables if they don't exist
func (svs *SQLiteVectorStore) initializeTables() {
	if svs.db == nil {
		return
	}

	query := `CREATE TABLE IF NOT EXISTS vectors (
		id TEXT PRIMARY KEY,
		document_id TEXT NOT NULL,
		chunk_id TEXT,
		vector TEXT NOT NULL,
		dimension INTEGER NOT NULL,
		content TEXT,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if err := svs.db.Execute(context.Background(), query); err != nil && svs.logger != nil {
		svs.logger.Warn(context.Background(), "Failed to create vectors table", "error", err)
	}

	// Create indexes for better performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_vectors_document_id ON vectors(document_id);",
		"CREATE INDEX IF NOT EXISTS idx_vectors_chunk_id ON vectors(chunk_id);",
		"CREATE INDEX IF NOT EXISTS idx_vectors_dimension ON vectors(dimension);",
	}

	for _, indexQuery := range indexes {
		if err := svs.db.Execute(context.Background(), indexQuery); err != nil && svs.logger != nil {
			svs.logger.Warn(context.Background(), "Failed to create vector index", "error", err)
		}
	}
}

// StoreVector stores a vector entry in SQLite
func (svs *SQLiteVectorStore) StoreVector(ctx context.Context, entry *models.VectorEntry) error {
	if entry == nil {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "vector entry cannot be nil")
	}

	if entry.ID == "" {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "vector entry ID cannot be empty")
	}

	if len(entry.Vector) == 0 {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "vector cannot be empty")
	}

	if svs.db == nil {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "database not initialized")
	}

	svs.mutex.Lock()
	defer svs.mutex.Unlock()

	startTime := time.Now()
	defer func() {
		if svs.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			svs.metrics.RecordTimer("vector_store.sqlite.store.duration", duration, nil)
		}
	}()

	// Serialize vector to JSON
	vectorJSON, err := json.Marshal(entry.Vector)
	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, "failed to serialize vector", err)
	}

	query := `INSERT OR REPLACE INTO vectors
		(id, document_id, chunk_id, vector, dimension, content, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	err = svs.db.Execute(ctx, query, entry.ID, entry.DocumentID, entry.ChunkID,
		string(vectorJSON), entry.Dimension, entry.Content, entry.Metadata, entry.CreatedAt)

	if err != nil {
		if svs.metrics != nil {
			svs.metrics.IncrementCounter("vector_store.sqlite.store.error", nil)
		}
		return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, "failed to store vector in SQLite", err)
	}

	if svs.metrics != nil {
		svs.metrics.IncrementCounter("vector_store.sqlite.store.success", nil)
	}

	return nil
}

// SearchSimilar searches for similar vectors using cosine similarity
func (svs *SQLiteVectorStore) SearchSimilar(ctx context.Context, queryVector []float32, topK int) ([]*models.SearchResult, error) {
	if len(queryVector) == 0 {
		return nil, models.NewDocumentError(models.ErrSearchFailed, "query vector cannot be empty")
	}

	if topK <= 0 {
		return []*models.SearchResult{}, nil
	}

	if svs.db == nil {
		return nil, models.NewDocumentError(models.ErrSearchFailed, "database not initialized")
	}

	svs.mutex.RLock()
	defer svs.mutex.RUnlock()

	startTime := time.Now()
	defer func() {
		if svs.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			svs.metrics.RecordTimer("vector_store.sqlite.search.duration", duration, nil)
		}
	}()

	// Get all vectors from database and calculate similarity in memory
	// For large datasets, this should be optimized with proper vector indexing
	query := `SELECT id, document_id, chunk_id, vector, dimension, content, metadata, created_at
		FROM vectors ORDER BY created_at DESC`

	rows, err := svs.db.Query(ctx, query)
	if err != nil {
		if svs.metrics != nil {
			svs.metrics.IncrementCounter("vector_store.sqlite.search.error", nil)
		}
		return nil, models.NewDocumentErrorWithCause(models.ErrSearchFailed, "failed to query vectors from SQLite", err)
	}
	defer rows.Close()

	var results []*models.SearchResult

	for rows.Next() {
		var entry models.VectorEntry
		var vectorJSON string

		err := rows.Scan(&entry.ID, &entry.DocumentID, &entry.ChunkID,
			&vectorJSON, &entry.Dimension, &entry.Content, &entry.Metadata, &entry.CreatedAt)
		if err != nil {
			if svs.logger != nil {
				svs.logger.Warn(ctx, "Failed to scan vector row", "error", err)
			}
			continue
		}

		// Deserialize vector
		if err := json.Unmarshal([]byte(vectorJSON), &entry.Vector); err != nil {
			if svs.logger != nil {
				svs.logger.Warn(ctx, "Failed to deserialize vector", "id", entry.ID, "error", err)
			}
			continue
		}

		// Calculate similarity
		if len(entry.Vector) != len(queryVector) {
			continue // Skip vectors with different dimensions
		}

		similarity := svs.calculateSimilarity(queryVector, entry.Vector)

		result := &models.SearchResult{
			Entry: &entry,
			Score: similarity,
		}

		results = append(results, result)
	}

	// Sort by similarity (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top K results
	if len(results) > topK {
		results = results[:topK]
	}

	if svs.metrics != nil {
		svs.metrics.IncrementCounter("vector_store.sqlite.search.success", map[string]string{
			"results": fmt.Sprintf("%d", len(results)),
		})
	}

	return results, nil
}

// GetVector retrieves a vector by ID from SQLite
func (svs *SQLiteVectorStore) GetVector(ctx context.Context, id string) (*models.VectorEntry, error) {
	if id == "" {
		return nil, models.NewDocumentError(models.ErrVectorStoreFailed, "vector ID cannot be empty")
	}

	if svs.db == nil {
		return nil, models.NewDocumentError(models.ErrVectorStoreFailed, "database not initialized")
	}

	svs.mutex.RLock()
	defer svs.mutex.RUnlock()

	query := `SELECT id, document_id, chunk_id, vector, dimension, content, metadata, created_at
		FROM vectors WHERE id = ?`

	rows, err := svs.db.Query(ctx, query, id)
	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrDocumentNotFound, "failed to query vector from SQLite", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, models.NewDocumentError(models.ErrDocumentNotFound, fmt.Sprintf("vector not found: %s", id))
	}

	var entry models.VectorEntry
	var vectorJSON string

	err = rows.Scan(&entry.ID, &entry.DocumentID, &entry.ChunkID,
		&vectorJSON, &entry.Dimension, &entry.Content, &entry.Metadata, &entry.CreatedAt)
	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, "failed to scan vector row", err)
	}

	// Deserialize vector
	if err := json.Unmarshal([]byte(vectorJSON), &entry.Vector); err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, "failed to deserialize vector", err)
	}

	return &entry, nil
}

// DeleteVector deletes a vector by ID from SQLite
func (svs *SQLiteVectorStore) DeleteVector(ctx context.Context, id string) error {
	if id == "" {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "vector ID cannot be empty")
	}

	if svs.db == nil {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "database not initialized")
	}

	svs.mutex.Lock()
	defer svs.mutex.Unlock()

	query := `DELETE FROM vectors WHERE id = ?`
	err := svs.db.Execute(ctx, query, id)

	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, "failed to delete vector from SQLite", err)
	}

	if svs.metrics != nil {
		svs.metrics.IncrementCounter("vector_store.sqlite.delete.success", nil)
	}

	return nil
}

// DeleteDocumentVectors deletes all vectors for a document from SQLite
func (svs *SQLiteVectorStore) DeleteDocumentVectors(ctx context.Context, documentID string) error {
	if documentID == "" {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "document ID cannot be empty")
	}

	if svs.db == nil {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "database not initialized")
	}

	svs.mutex.Lock()
	defer svs.mutex.Unlock()

	query := `DELETE FROM vectors WHERE document_id = ?`
	err := svs.db.Execute(ctx, query, documentID)

	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, "failed to delete document vectors from SQLite", err)
	}

	if svs.metrics != nil {
		svs.metrics.IncrementCounter("vector_store.sqlite.delete_document.success", nil)
	}

	return nil
}

// GetDocumentVectors retrieves all vectors for a document from SQLite
func (svs *SQLiteVectorStore) GetDocumentVectors(ctx context.Context, documentID string) ([]*models.VectorEntry, error) {
	if documentID == "" {
		return nil, models.NewDocumentError(models.ErrVectorStoreFailed, "document ID cannot be empty")
	}

	if svs.db == nil {
		return nil, models.NewDocumentError(models.ErrVectorStoreFailed, "database not initialized")
	}

	svs.mutex.RLock()
	defer svs.mutex.RUnlock()

	query := `SELECT id, document_id, chunk_id, vector, dimension, content, metadata, created_at
		FROM vectors WHERE document_id = ? ORDER BY created_at ASC`

	rows, err := svs.db.Query(ctx, query, documentID)
	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, "failed to query document vectors from SQLite", err)
	}
	defer rows.Close()

	var vectors []*models.VectorEntry

	for rows.Next() {
		var entry models.VectorEntry
		var vectorJSON string

		err := rows.Scan(&entry.ID, &entry.DocumentID, &entry.ChunkID,
			&vectorJSON, &entry.Dimension, &entry.Content, &entry.Metadata, &entry.CreatedAt)
		if err != nil {
			if svs.logger != nil {
				svs.logger.Warn(ctx, "Failed to scan vector row for document", "document_id", documentID, "error", err)
			}
			continue
		}

		// Deserialize vector
		if err := json.Unmarshal([]byte(vectorJSON), &entry.Vector); err != nil {
			if svs.logger != nil {
				svs.logger.Warn(ctx, "Failed to deserialize vector for document", "id", entry.ID, "document_id", documentID, "error", err)
			}
			continue
		}

		vectors = append(vectors, &entry)
	}

	return vectors, nil
}

// Clear removes all vectors from the SQLite store
func (svs *SQLiteVectorStore) Clear(ctx context.Context) error {
	if svs.db == nil {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "database not initialized")
	}

	svs.mutex.Lock()
	defer svs.mutex.Unlock()

	query := `DELETE FROM vectors`
	err := svs.db.Execute(ctx, query)

	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, "failed to clear vectors from SQLite", err)
	}

	if svs.logger != nil {
		svs.logger.Info(ctx, "Cleared SQLite vector store")
	}

	return nil
}

// StoreBatch stores multiple vectors in a batch
func (svs *SQLiteVectorStore) StoreBatch(ctx context.Context, entries []*models.VectorEntry) error {
	if len(entries) == 0 {
		return nil
	}

	if svs.db == nil {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "database not initialized")
	}

	svs.mutex.Lock()
	defer svs.mutex.Unlock()

	startTime := time.Now()
	defer func() {
		if svs.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			svs.metrics.RecordTimer("vector_store.sqlite.store_batch.duration", duration, nil)
		}
	}()

	// Use transaction for batch insert
	var successCount int

	err := svs.db.Transaction(ctx, func(tx *sql.Tx) error {
		query := `INSERT OR REPLACE INTO vectors
			(id, document_id, chunk_id, vector, dimension, content, metadata, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

		stmt, err := tx.Prepare(query)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, entry := range entries {
			if entry == nil || entry.ID == "" || len(entry.Vector) == 0 {
				continue
			}

			// Serialize vector to JSON
			vectorJSON, err := json.Marshal(entry.Vector)
			if err != nil {
				if svs.logger != nil {
					svs.logger.Warn(ctx, "Failed to serialize vector in batch", "id", entry.ID, "error", err)
				}
				continue
			}

			_, err = stmt.Exec(entry.ID, entry.DocumentID, entry.ChunkID,
				string(vectorJSON), entry.Dimension, entry.Content, entry.Metadata, entry.CreatedAt)
			if err != nil {
				if svs.logger != nil {
					svs.logger.Warn(ctx, "Failed to insert vector in batch", "id", entry.ID, "error", err)
				}
				continue
			}

			successCount++
		}

		return nil
	})

	if err != nil {
		if svs.metrics != nil {
			svs.metrics.IncrementCounter("vector_store.sqlite.store_batch.error", nil)
		}
		return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, "failed to execute batch transaction", err)
	}

	if svs.metrics != nil {
		svs.metrics.IncrementCounter("vector_store.sqlite.store_batch.success", map[string]string{
			"count": fmt.Sprintf("%d", successCount),
			"total": fmt.Sprintf("%d", len(entries)),
		})
	}

	return nil
}

// SearchWithFilter searches for similar vectors with filtering
func (svs *SQLiteVectorStore) SearchWithFilter(ctx context.Context, queryVector []float32, topK int, filter map[string]interface{}) ([]*models.SearchResult, error) {
	if len(queryVector) == 0 {
		return nil, models.NewDocumentError(models.ErrSearchFailed, "query vector cannot be empty")
	}

	if topK <= 0 {
		return []*models.SearchResult{}, nil
	}

	if svs.db == nil {
		return nil, models.NewDocumentError(models.ErrSearchFailed, "database not initialized")
	}

	svs.mutex.RLock()
	defer svs.mutex.RUnlock()

	// Build query with filter conditions
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`SELECT id, document_id, chunk_id, vector, dimension, content, metadata, created_at FROM vectors`)

	var args []interface{}
	var conditions []string

	if filter != nil && len(filter) > 0 {
		if docID, ok := filter["document_id"]; ok {
			conditions = append(conditions, "document_id = ?")
			args = append(args, docID)
		}
		if chunkID, ok := filter["chunk_id"]; ok {
			conditions = append(conditions, "chunk_id = ?")
			args = append(args, chunkID)
		}
		if content, ok := filter["content"]; ok {
			conditions = append(conditions, "content LIKE ?")
			args = append(args, fmt.Sprintf("%%%s%%", content))
		}
	}

	if len(conditions) > 0 {
		queryBuilder.WriteString(" WHERE ")
		queryBuilder.WriteString(strings.Join(conditions, " AND "))
	}

	queryBuilder.WriteString(" ORDER BY created_at DESC")

	rows, err := svs.db.Query(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrSearchFailed, "failed to query vectors with filter from SQLite", err)
	}
	defer rows.Close()

	var results []*models.SearchResult

	for rows.Next() {
		var entry models.VectorEntry
		var vectorJSON string

		err := rows.Scan(&entry.ID, &entry.DocumentID, &entry.ChunkID,
			&vectorJSON, &entry.Dimension, &entry.Content, &entry.Metadata, &entry.CreatedAt)
		if err != nil {
			continue
		}

		// Deserialize vector
		if err := json.Unmarshal([]byte(vectorJSON), &entry.Vector); err != nil {
			continue
		}

		// Calculate similarity
		if len(entry.Vector) != len(queryVector) {
			continue
		}

		similarity := svs.calculateSimilarity(queryVector, entry.Vector)

		result := &models.SearchResult{
			Entry: &entry,
			Score: similarity,
		}

		results = append(results, result)
	}

	// Sort by similarity (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top K results
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// HealthCheck performs a health check on the SQLite vector store
func (svs *SQLiteVectorStore) HealthCheck(ctx context.Context) error {
	if svs.db == nil {
		return models.NewDocumentError(models.ErrStorageFailed, "database not initialized")
	}

	// Simple query to check database connectivity
	_, err := svs.db.Query(ctx, "SELECT COUNT(*) FROM vectors LIMIT 1")
	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrStorageFailed, "failed to query SQLite database", err)
	}

	return nil
}

// calculateSimilarity calculates similarity between two vectors based on the configured metric
func (svs *SQLiteVectorStore) calculateSimilarity(a, b []float32) float32 {
	switch svs.config.Metric {
	case "euclidean":
		// Convert distance to similarity (higher is better)
		distance := euclideanDistance(a, b)
		return 1.0 / (1.0 + distance)
	case "dot":
		return dotProduct(a, b)
	default: // "cosine" or default
		return cosineSimilarity(a, b)
	}
}
