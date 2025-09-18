package stores

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// DocumentStoreConfig contains configuration for document stores
type DocumentStoreConfig struct {
	Type         string                 `json:"type"`          // "sql", "memory"
	TableName    string                 `json:"table_name"`
	BatchSize    int                    `json:"batch_size"`
	Timeout      int                    `json:"timeout_seconds"`
	CustomConfig map[string]interface{} `json:"custom_config,omitempty"`
}

// SQLDocumentStore implements DocumentStoreInterface using SQL database
type SQLDocumentStore struct {
	db        interfaces.DatabaseProvider
	config    DocumentStoreConfig
	logger    interfaces.Logger
	metrics   interfaces.MetricsCollector
	tableName string
}

// NewSQLDocumentStore creates a new SQL document store
func NewSQLDocumentStore(db interfaces.DatabaseProvider, config DocumentStoreConfig) *SQLDocumentStore {
	if config.TableName == "" {
		config.TableName = "documents"
	}

	return &SQLDocumentStore{
		db:        db,
		config:    config,
		tableName: config.TableName,
	}
}

// WithLogger adds logging to the SQL document store
func (sds *SQLDocumentStore) WithLogger(logger interfaces.Logger) *SQLDocumentStore {
	sds.logger = logger
	return sds
}

// WithMetrics adds metrics collection to the SQL document store
func (sds *SQLDocumentStore) WithMetrics(metrics interfaces.MetricsCollector) *SQLDocumentStore {
	sds.metrics = metrics
	return sds
}

// Initialize initializes the SQL document store and creates table if needed
func (sds *SQLDocumentStore) Initialize(ctx context.Context) error {
	// Create table if it doesn't exist
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			file_path TEXT NOT NULL,
			file_hash TEXT,
			content TEXT,
			metadata TEXT,
			chunk_count INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`, sds.tableName)

	if err := sds.db.Execute(ctx, createTableSQL); err != nil {
		return models.NewDocumentErrorWithCause(models.ErrDatabaseError, "failed to create documents table", err)
	}

	// Create indexes separately for SQLite compatibility
	indexes := []string{
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_file_path ON %s (file_path)", sds.tableName, sds.tableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_file_hash ON %s (file_hash)", sds.tableName, sds.tableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_created_at ON %s (created_at)", sds.tableName, sds.tableName),
	}

	for _, indexSQL := range indexes {
		if err := sds.db.Execute(ctx, indexSQL); err != nil {
			// Log warning but don't fail - indexes are optimization
			if sds.logger != nil {
				sds.logger.Warn(ctx, "Failed to create index", "error", err, "sql", indexSQL)
			}
		}
	}

	if sds.logger != nil {
		sds.logger.Info(ctx, "Initialized SQL document store", "table", sds.tableName)
	}

	return nil
}

// StoreDocument stores a document in the SQL database
func (sds *SQLDocumentStore) StoreDocument(ctx context.Context, doc *models.Document) error {
	if doc == nil {
		return models.NewDocumentError(models.ErrDocumentStoreFailed, "document cannot be nil")
	}

	if doc.ID == "" {
		return models.NewDocumentError(models.ErrDocumentStoreFailed, "document ID cannot be empty")
	}

	if sds.db == nil {
		return models.NewDocumentError(models.ErrDatabaseError, "database provider is not available")
	}

	startTime := time.Now()
	defer func() {
		if sds.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			sds.metrics.RecordTimer("document_store.sql.store.duration", duration, nil)
		}
	}()

	// Serialize metadata
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "failed to serialize metadata", err)
	}

	// Insert or update document (SQLite syntax)
	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (id, file_path, file_hash, content, metadata, chunk_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			file_path = excluded.file_path,
			file_hash = excluded.file_hash,
			content = excluded.content,
			metadata = excluded.metadata,
			chunk_count = excluded.chunk_count,
			updated_at = excluded.updated_at
	`, sds.tableName)

	if err := sds.db.Execute(ctx, insertSQL,
		doc.ID,
		doc.FilePath,
		doc.FileHash,
		doc.Content,
		string(metadataJSON),
		doc.ChunkCount,
		doc.CreatedAt,
		doc.UpdatedAt,
	); err != nil {
		if sds.metrics != nil {
			sds.metrics.IncrementCounter("document_store.sql.store.error", nil)
		}
		return models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "failed to store document", err)
	}

	if sds.metrics != nil {
		sds.metrics.IncrementCounter("document_store.sql.store.success", nil)
	}

	return nil
}

// GetDocument retrieves a document by ID
func (sds *SQLDocumentStore) GetDocument(ctx context.Context, id string) (*models.Document, error) {
	if id == "" {
		return nil, models.NewDocumentError(models.ErrDocumentStoreFailed, "document ID cannot be empty")
	}

	if sds.db == nil {
		return nil, models.NewDocumentError(models.ErrDatabaseError, "database provider is not available")
	}

	selectSQL := fmt.Sprintf(`
		SELECT id, file_path, file_hash, content, metadata, chunk_count, created_at, updated_at
		FROM %s
		WHERE id = ?
	`, sds.tableName)

	row := sds.db.QueryRow(ctx, selectSQL, id)
	if row == nil {
		return nil, models.NewDocumentError(models.ErrDatabaseError, "database query returned nil row")
	}

	var doc models.Document
	var metadataJSON string

	// Add defensive programming to catch potential nil pointer issues
	defer func() {
		if r := recover(); r != nil {
			if sds.logger != nil {
				sds.logger.Error(ctx, "Panic in GetDocument scan operation", "panic", r, "id", id)
			}
		}
	}()

	err := row.Scan(
		&doc.ID,
		&doc.FilePath,
		&doc.FileHash,
		&doc.Content,
		&metadataJSON,
		&doc.ChunkCount,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.NewDocumentError(models.ErrDocumentNotFound, fmt.Sprintf("document not found: %s", id))
		}
		return nil, models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "failed to get document", err)
	}

	// Deserialize metadata
	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &doc.Metadata); err != nil {
			if sds.logger != nil {
				sds.logger.Warn(ctx, "Failed to deserialize document metadata", "id", id, "error", err)
			}
			doc.Metadata = make(map[string]interface{})
		}
	} else {
		doc.Metadata = make(map[string]interface{})
	}

	return &doc, nil
}

// GetDocumentByPath retrieves a document by file path
func (sds *SQLDocumentStore) GetDocumentByPath(ctx context.Context, filePath string) (*models.Document, error) {
	if filePath == "" {
		return nil, models.NewDocumentError(models.ErrDocumentStoreFailed, "file path cannot be empty")
	}

	if sds.db == nil {
		return nil, models.NewDocumentError(models.ErrDatabaseError, "database provider is not available")
	}

	selectSQL := fmt.Sprintf(`
		SELECT id, file_path, file_hash, content, metadata, chunk_count, created_at, updated_at
		FROM %s
		WHERE file_path = ?
	`, sds.tableName)

	row := sds.db.QueryRow(ctx, selectSQL, filePath)
	if row == nil {
		return nil, models.NewDocumentError(models.ErrDatabaseError, "database query returned nil row")
	}

	var doc models.Document
	var metadataJSON string

	// Add defensive programming to catch potential nil pointer issues
	defer func() {
		if r := recover(); r != nil {
			if sds.logger != nil {
				sds.logger.Error(ctx, "Panic in GetDocumentByPath scan operation", "panic", r, "path", filePath)
			}
		}
	}()

	err := row.Scan(
		&doc.ID,
		&doc.FilePath,
		&doc.FileHash,
		&doc.Content,
		&metadataJSON,
		&doc.ChunkCount,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.NewDocumentError(models.ErrDocumentNotFound, fmt.Sprintf("document not found: %s", filePath))
		}
		return nil, models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "failed to get document by path", err)
	}

	// Deserialize metadata
	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &doc.Metadata); err != nil {
			if sds.logger != nil {
				sds.logger.Warn(ctx, "Failed to deserialize document metadata", "path", filePath, "error", err)
			}
			doc.Metadata = make(map[string]interface{})
		}
	} else {
		doc.Metadata = make(map[string]interface{})
	}

	return &doc, nil
}

// UpdateDocument updates an existing document
func (sds *SQLDocumentStore) UpdateDocument(ctx context.Context, doc *models.Document) error {
	if doc == nil {
		return models.NewDocumentError(models.ErrDocumentStoreFailed, "document cannot be nil")
	}

	if doc.ID == "" {
		return models.NewDocumentError(models.ErrDocumentStoreFailed, "document ID cannot be empty")
	}

	// Serialize metadata
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "failed to serialize metadata", err)
	}

	// Update document
	updateSQL := fmt.Sprintf(`
		UPDATE %s
		SET file_path = ?, file_hash = ?, content = ?, metadata = ?, chunk_count = ?, updated_at = ?
		WHERE id = ?
	`, sds.tableName)

	doc.UpdatedAt = time.Now()

	if err := sds.db.Execute(ctx, updateSQL,
		doc.FilePath,
		doc.FileHash,
		doc.Content,
		string(metadataJSON),
		doc.ChunkCount,
		doc.UpdatedAt,
		doc.ID,
	); err != nil {
		return models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "failed to update document", err)
	}

	return nil
}

// DeleteDocument deletes a document by ID
func (sds *SQLDocumentStore) DeleteDocument(ctx context.Context, id string) error {
	if id == "" {
		return models.NewDocumentError(models.ErrDocumentStoreFailed, "document ID cannot be empty")
	}

	deleteSQL := fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, sds.tableName)

	if err := sds.db.Execute(ctx, deleteSQL, id); err != nil {
		return models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "failed to delete document", err)
	}

	if sds.metrics != nil {
		sds.metrics.IncrementCounter("document_store.sql.delete.success", nil)
	}

	return nil
}

// ListDocuments lists all documents
func (sds *SQLDocumentStore) ListDocuments(ctx context.Context) ([]*models.Document, error) {
	if sds.db == nil {
		return nil, models.NewDocumentError(models.ErrDatabaseError, "database provider is not available")
	}

	selectSQL := fmt.Sprintf(`
		SELECT id, file_path, file_hash, content, metadata, chunk_count, created_at, updated_at
		FROM %s
		ORDER BY created_at DESC
	`, sds.tableName)

	rows, err := sds.db.Query(ctx, selectSQL)
	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "failed to list documents", err)
	}
	defer rows.Close()

	var documents []*models.Document

	for rows.Next() {
		var doc models.Document
		var metadataJSON string

		err := rows.Scan(
			&doc.ID,
			&doc.FilePath,
			&doc.FileHash,
			&doc.Content,
			&metadataJSON,
			&doc.ChunkCount,
			&doc.CreatedAt,
			&doc.UpdatedAt,
		)

		if err != nil {
			if sds.logger != nil {
				sds.logger.Warn(ctx, "Failed to scan document row", "error", err)
			}
			continue
		}

		// Deserialize metadata
		if metadataJSON != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &doc.Metadata); err != nil {
				if sds.logger != nil {
					sds.logger.Warn(ctx, "Failed to deserialize document metadata", "id", doc.ID, "error", err)
				}
				doc.Metadata = make(map[string]interface{})
			}
		} else {
			doc.Metadata = make(map[string]interface{})
		}

		documents = append(documents, &doc)
	}

	if err := rows.Err(); err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "error iterating document rows", err)
	}

	return documents, nil
}

// GetDocumentsByIDs retrieves multiple documents by their IDs
func (sds *SQLDocumentStore) GetDocumentsByIDs(ctx context.Context, ids []string) ([]*models.Document, error) {
	if len(ids) == 0 {
		return []*models.Document{}, nil
	}

	// Create placeholders for the IN clause
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma

	selectSQL := fmt.Sprintf(`
		SELECT id, file_path, file_hash, content, metadata, chunk_count, created_at, updated_at
		FROM %s
		WHERE id IN (%s)
		ORDER BY created_at DESC
	`, sds.tableName, placeholders)

	// Convert string slice to interface slice for query parameters
	params := make([]interface{}, len(ids))
	for i, id := range ids {
		params[i] = id
	}

	rows, err := sds.db.Query(ctx, selectSQL, params...)
	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "failed to get documents by IDs", err)
	}
	defer rows.Close()

	var documents []*models.Document

	for rows.Next() {
		var doc models.Document
		var metadataJSON string

		err := rows.Scan(
			&doc.ID,
			&doc.FilePath,
			&doc.FileHash,
			&doc.Content,
			&metadataJSON,
			&doc.ChunkCount,
			&doc.CreatedAt,
			&doc.UpdatedAt,
		)

		if err != nil {
			if sds.logger != nil {
				sds.logger.Warn(ctx, "Failed to scan document row", "error", err)
			}
			continue
		}

		// Deserialize metadata
		if metadataJSON != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &doc.Metadata); err != nil {
				if sds.logger != nil {
					sds.logger.Warn(ctx, "Failed to deserialize document metadata", "id", doc.ID, "error", err)
				}
				doc.Metadata = make(map[string]interface{})
			}
		} else {
			doc.Metadata = make(map[string]interface{})
		}

		documents = append(documents, &doc)
	}

	if err := rows.Err(); err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "error iterating document rows", err)
	}

	return documents, nil
}

// SearchDocumentsByMetadata searches documents by metadata filters
func (sds *SQLDocumentStore) SearchDocumentsByMetadata(ctx context.Context, filter map[string]interface{}) ([]*models.Document, error) {
	if len(filter) == 0 {
		return sds.ListDocuments(ctx)
	}

	// Build WHERE clause for JSON searches
	var conditions []string
	var params []interface{}

	for key, value := range filter {
		conditions = append(conditions, "JSON_EXTRACT(metadata, ?) = ?")
		params = append(params, "$."+key, value)
	}

	whereClause := strings.Join(conditions, " AND ")

	selectSQL := fmt.Sprintf(`
		SELECT id, file_path, file_hash, content, metadata, chunk_count, created_at, updated_at
		FROM %s
		WHERE %s
		ORDER BY created_at DESC
	`, sds.tableName, whereClause)

	rows, err := sds.db.Query(ctx, selectSQL, params...)
	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "failed to search documents by metadata", err)
	}
	defer rows.Close()

	var documents []*models.Document

	for rows.Next() {
		var doc models.Document
		var metadataJSON string

		err := rows.Scan(
			&doc.ID,
			&doc.FilePath,
			&doc.FileHash,
			&doc.Content,
			&metadataJSON,
			&doc.ChunkCount,
			&doc.CreatedAt,
			&doc.UpdatedAt,
		)

		if err != nil {
			if sds.logger != nil {
				sds.logger.Warn(ctx, "Failed to scan document row", "error", err)
			}
			continue
		}

		// Deserialize metadata
		if metadataJSON != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &doc.Metadata); err != nil {
				if sds.logger != nil {
					sds.logger.Warn(ctx, "Failed to deserialize document metadata", "id", doc.ID, "error", err)
				}
				doc.Metadata = make(map[string]interface{})
			}
		} else {
			doc.Metadata = make(map[string]interface{})
		}

		documents = append(documents, &doc)
	}

	if err := rows.Err(); err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "error iterating document rows", err)
	}

	return documents, nil
}

// GetDocumentStats returns statistics about the documents
func (sds *SQLDocumentStore) GetDocumentStats(ctx context.Context) (*models.DocumentStats, error) {
	statsSQL := fmt.Sprintf(`
		SELECT
			COUNT(*) as total_documents,
			COALESCE(SUM(chunk_count), 0) as total_chunks,
			COALESCE(AVG(chunk_count), 0) as avg_chunk_count,
			COALESCE(SUM(LENGTH(content)), 0) as total_content_size
		FROM %s
	`, sds.tableName)

	row := sds.db.QueryRow(ctx, statsSQL)

	var stats models.DocumentStats
	var avgChunkCount float64
	var totalContentSize int64

	err := row.Scan(
		&stats.TotalDocuments,
		&stats.TotalChunks,
		&avgChunkCount,
		&totalContentSize,
	)

	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "failed to get document stats", err)
	}

	stats.AverageChunkSize = avgChunkCount
	stats.TotalSize = totalContentSize

	// Get file type distribution
	fileTypesSQL := fmt.Sprintf(`
		SELECT
			JSON_EXTRACT(metadata, '$.file_ext') as file_ext,
			COUNT(*) as count
		FROM %s
		WHERE JSON_EXTRACT(metadata, '$.file_ext') IS NOT NULL
		GROUP BY JSON_EXTRACT(metadata, '$.file_ext')
	`, sds.tableName)

	rows, err := sds.db.Query(ctx, fileTypesSQL)
	if err == nil {
		defer rows.Close()

		stats.FileTypes = make(map[string]int64)

		for rows.Next() {
			var fileExt string
			var count int64

			if err := rows.Scan(&fileExt, &count); err == nil {
				stats.FileTypes[fileExt] = count
			}
		}
	}

	stats.ProcessingStats = make(map[string]interface{})

	return &stats, nil
}

// MemoryDocumentStore implements DocumentStoreInterface using in-memory storage
// This is primarily for testing and development
type MemoryDocumentStore struct {
	documents map[string]*models.Document
	pathIndex map[string]string // Maps file path to document ID
	mutex     sync.RWMutex
	config    DocumentStoreConfig
	logger    interfaces.Logger
	metrics   interfaces.MetricsCollector
}

// NewMemoryDocumentStore creates a new in-memory document store
func NewMemoryDocumentStore(config DocumentStoreConfig) *MemoryDocumentStore {
	return &MemoryDocumentStore{
		documents: make(map[string]*models.Document),
		pathIndex: make(map[string]string),
		config:    config,
	}
}

// WithLogger adds logging to the memory document store
func (mds *MemoryDocumentStore) WithLogger(logger interfaces.Logger) *MemoryDocumentStore {
	mds.logger = logger
	return mds
}

// WithMetrics adds metrics collection to the memory document store
func (mds *MemoryDocumentStore) WithMetrics(metrics interfaces.MetricsCollector) *MemoryDocumentStore {
	mds.metrics = metrics
	return mds
}

// StoreDocument stores a document in memory
func (mds *MemoryDocumentStore) StoreDocument(ctx context.Context, doc *models.Document) error {
	if doc == nil {
		return models.NewDocumentError(models.ErrDocumentStoreFailed, "document cannot be nil")
	}

	if doc.ID == "" {
		return models.NewDocumentError(models.ErrDocumentStoreFailed, "document ID cannot be empty")
	}

	mds.mutex.Lock()
	defer mds.mutex.Unlock()

	// Create a copy to avoid external modifications
	docCopy := *doc
	if doc.Metadata != nil {
		docCopy.Metadata = make(map[string]interface{})
		for k, v := range doc.Metadata {
			docCopy.Metadata[k] = v
		}
	}

	mds.documents[doc.ID] = &docCopy
	mds.pathIndex[doc.FilePath] = doc.ID

	if mds.metrics != nil {
		mds.metrics.IncrementCounter("document_store.memory.store.success", nil)
	}

	return nil
}

// GetDocument retrieves a document by ID from memory
func (mds *MemoryDocumentStore) GetDocument(ctx context.Context, id string) (*models.Document, error) {
	if id == "" {
		return nil, models.NewDocumentError(models.ErrDocumentStoreFailed, "document ID cannot be empty")
	}

	mds.mutex.RLock()
	defer mds.mutex.RUnlock()

	doc, exists := mds.documents[id]
	if !exists {
		return nil, models.NewDocumentError(models.ErrDocumentNotFound, fmt.Sprintf("document not found: %s", id))
	}

	// Return a copy to avoid external modifications
	docCopy := *doc
	if doc.Metadata != nil {
		docCopy.Metadata = make(map[string]interface{})
		for k, v := range doc.Metadata {
			docCopy.Metadata[k] = v
		}
	}

	return &docCopy, nil
}

// GetDocumentByPath retrieves a document by file path from memory
func (mds *MemoryDocumentStore) GetDocumentByPath(ctx context.Context, filePath string) (*models.Document, error) {
	if filePath == "" {
		return nil, models.NewDocumentError(models.ErrDocumentStoreFailed, "file path cannot be empty")
	}

	mds.mutex.RLock()
	defer mds.mutex.RUnlock()

	docID, exists := mds.pathIndex[filePath]
	if !exists {
		return nil, models.NewDocumentError(models.ErrDocumentNotFound, fmt.Sprintf("document not found: %s", filePath))
	}

	return mds.GetDocument(ctx, docID)
}

// UpdateDocument updates a document in memory
func (mds *MemoryDocumentStore) UpdateDocument(ctx context.Context, doc *models.Document) error {
	if doc == nil {
		return models.NewDocumentError(models.ErrDocumentStoreFailed, "document cannot be nil")
	}

	if doc.ID == "" {
		return models.NewDocumentError(models.ErrDocumentStoreFailed, "document ID cannot be empty")
	}

	mds.mutex.Lock()
	defer mds.mutex.Unlock()

	if _, exists := mds.documents[doc.ID]; !exists {
		return models.NewDocumentError(models.ErrDocumentNotFound, fmt.Sprintf("document not found: %s", doc.ID))
	}

	// Update the document
	doc.UpdatedAt = time.Now()

	// Create a copy to avoid external modifications
	docCopy := *doc
	if doc.Metadata != nil {
		docCopy.Metadata = make(map[string]interface{})
		for k, v := range doc.Metadata {
			docCopy.Metadata[k] = v
		}
	}

	// Update path index if path changed
	oldDoc := mds.documents[doc.ID]
	if oldDoc.FilePath != doc.FilePath {
		delete(mds.pathIndex, oldDoc.FilePath)
		mds.pathIndex[doc.FilePath] = doc.ID
	}

	mds.documents[doc.ID] = &docCopy

	return nil
}

// DeleteDocument deletes a document from memory
func (mds *MemoryDocumentStore) DeleteDocument(ctx context.Context, id string) error {
	if id == "" {
		return models.NewDocumentError(models.ErrDocumentStoreFailed, "document ID cannot be empty")
	}

	mds.mutex.Lock()
	defer mds.mutex.Unlock()

	doc, exists := mds.documents[id]
	if !exists {
		return models.NewDocumentError(models.ErrDocumentNotFound, fmt.Sprintf("document not found: %s", id))
	}

	delete(mds.documents, id)
	delete(mds.pathIndex, doc.FilePath)

	if mds.metrics != nil {
		mds.metrics.IncrementCounter("document_store.memory.delete.success", nil)
	}

	return nil
}

// ListDocuments lists all documents from memory
func (mds *MemoryDocumentStore) ListDocuments(ctx context.Context) ([]*models.Document, error) {
	mds.mutex.RLock()
	defer mds.mutex.RUnlock()

	documents := make([]*models.Document, 0, len(mds.documents))

	for _, doc := range mds.documents {
		// Return a copy to avoid external modifications
		docCopy := *doc
		if doc.Metadata != nil {
			docCopy.Metadata = make(map[string]interface{})
			for k, v := range doc.Metadata {
				docCopy.Metadata[k] = v
			}
		}
		documents = append(documents, &docCopy)
	}

	return documents, nil
}

// GetDocumentsByIDs retrieves multiple documents by their IDs from memory
func (mds *MemoryDocumentStore) GetDocumentsByIDs(ctx context.Context, ids []string) ([]*models.Document, error) {
	mds.mutex.RLock()
	defer mds.mutex.RUnlock()

	documents := make([]*models.Document, 0, len(ids))

	for _, id := range ids {
		if doc, exists := mds.documents[id]; exists {
			// Return a copy to avoid external modifications
			docCopy := *doc
			if doc.Metadata != nil {
				docCopy.Metadata = make(map[string]interface{})
				for k, v := range doc.Metadata {
					docCopy.Metadata[k] = v
				}
			}
			documents = append(documents, &docCopy)
		}
	}

	return documents, nil
}

// SearchDocumentsByMetadata searches documents by metadata filters in memory
func (mds *MemoryDocumentStore) SearchDocumentsByMetadata(ctx context.Context, filter map[string]interface{}) ([]*models.Document, error) {
	mds.mutex.RLock()
	defer mds.mutex.RUnlock()

	var documents []*models.Document

	for _, doc := range mds.documents {
		if mds.matchesFilter(doc, filter) {
			// Return a copy to avoid external modifications
			docCopy := *doc
			if doc.Metadata != nil {
				docCopy.Metadata = make(map[string]interface{})
				for k, v := range doc.Metadata {
					docCopy.Metadata[k] = v
				}
			}
			documents = append(documents, &docCopy)
		}
	}

	return documents, nil
}

// GetDocumentStats returns statistics about the documents in memory
func (mds *MemoryDocumentStore) GetDocumentStats(ctx context.Context) (*models.DocumentStats, error) {
	mds.mutex.RLock()
	defer mds.mutex.RUnlock()

	stats := &models.DocumentStats{
		FileTypes:       make(map[string]int64),
		ProcessingStats: make(map[string]interface{}),
	}

	var totalChunks int64
	var totalSize int64

	for _, doc := range mds.documents {
		stats.TotalDocuments++
		totalChunks += int64(doc.ChunkCount)
		totalSize += int64(len(doc.Content))

		// Count file types
		if fileExt, ok := doc.Metadata["file_ext"].(string); ok {
			stats.FileTypes[fileExt]++
		}
	}

	stats.TotalChunks = totalChunks
	stats.TotalSize = totalSize

	if stats.TotalDocuments > 0 {
		stats.AverageChunkSize = float64(totalChunks) / float64(stats.TotalDocuments)
	}

	return stats, nil
}

// matchesFilter checks if a document matches the given filter
func (mds *MemoryDocumentStore) matchesFilter(doc *models.Document, filter map[string]interface{}) bool {
	if filter == nil || len(filter) == 0 {
		return true
	}

	for key, expectedValue := range filter {
		if actualValue, exists := doc.Metadata[key]; !exists || actualValue != expectedValue {
			return false
		}
	}

	return true
}

// DefaultDocumentStoreConfig returns a default configuration for document stores
func DefaultDocumentStoreConfig() DocumentStoreConfig {
	return DocumentStoreConfig{
		Type:      "sql",
		TableName: "documents",
		BatchSize: 100,
		Timeout:   30,
	}
}

// DocumentStoreFactory creates document stores based on configuration
type DocumentStoreFactory struct {
	db      interfaces.DatabaseProvider
	logger  interfaces.Logger
	metrics interfaces.MetricsCollector
}

// NewDocumentStoreFactory creates a new document store factory
func NewDocumentStoreFactory(db interfaces.DatabaseProvider) *DocumentStoreFactory {
	return &DocumentStoreFactory{
		db: db,
	}
}

// WithLogger adds logging to the factory
func (dsf *DocumentStoreFactory) WithLogger(logger interfaces.Logger) *DocumentStoreFactory {
	dsf.logger = logger
	return dsf
}

// WithMetrics adds metrics collection to the factory
func (dsf *DocumentStoreFactory) WithMetrics(metrics interfaces.MetricsCollector) *DocumentStoreFactory {
	dsf.metrics = metrics
	return dsf
}

// CreateDocumentStore creates a document store based on the configuration
func (dsf *DocumentStoreFactory) CreateDocumentStore(config DocumentStoreConfig) (interfaces.DocumentStoreInterface, error) {
	switch config.Type {
	case "sql":
		if dsf.db == nil {
			return nil, models.NewDocumentError(models.ErrDependencyMissing, "database provider is required for SQL document store")
		}
		store := NewSQLDocumentStore(dsf.db, config)
		if dsf.logger != nil {
			store.WithLogger(dsf.logger)
		}
		if dsf.metrics != nil {
			store.WithMetrics(dsf.metrics)
		}
		return store, nil
	case "memory":
		store := NewMemoryDocumentStore(config)
		if dsf.logger != nil {
			store.WithLogger(dsf.logger)
		}
		if dsf.metrics != nil {
			store.WithMetrics(dsf.metrics)
		}
		return store, nil
	default:
		return nil, models.NewDocumentError(models.ErrInvalidConfig, fmt.Sprintf("unsupported document store type: %s", config.Type))
	}
}