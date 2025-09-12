package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// SQLiteVectorStore implements VectorStoreInterface using SQLite
type SQLiteVectorStore struct {
	db    Database
	mutex sync.RWMutex
}

// NewSQLiteVectorStore creates a new SQLite vector store
func NewSQLiteVectorStore(db Database) *SQLiteVectorStore {
	store := &SQLiteVectorStore{
		db: db,
	}
	// Initialize tables
	store.initializeTables()
	return store
}

// initializeTables creates the necessary tables if they don't exist
func (s *SQLiteVectorStore) initializeTables() {
	if s.db == nil {
		return
	}

	query := `CREATE TABLE IF NOT EXISTS vector_entries (
		id TEXT PRIMARY KEY,
		document_id TEXT NOT NULL,
		chunk_id TEXT,
		vector_data TEXT NOT NULL,
		dimension INTEGER NOT NULL,
		content TEXT,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := s.db.Exec(query); err != nil {
		log.Printf("Warning: failed to create vector_entries table: %v", err)
	}

	// Create index for document_id for efficient lookups
	indexQuery := `CREATE INDEX IF NOT EXISTS idx_vector_document 
		ON vector_entries(document_id);`

	if _, err := s.db.Exec(indexQuery); err != nil {
		log.Printf("Warning: failed to create vector index: %v", err)
	}
}

// StoreVector stores a vector entry
func (s *SQLiteVectorStore) StoreVector(entry *VectorEntry) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Serialize vector to JSON
	vectorJSON, err := json.Marshal(entry.Vector)
	if err != nil {
		return fmt.Errorf("failed to serialize vector: %v", err)
	}

	query := `INSERT OR REPLACE INTO vector_entries 
		(id, document_id, chunk_id, vector_data, dimension, content, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = s.db.Exec(query, entry.ID, entry.DocumentID, entry.ChunkID,
		string(vectorJSON), entry.Dimension, entry.Content, entry.Metadata, entry.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to store vector: %v", err)
	}

	return nil
}

// SearchSimilar searches for similar vectors using cosine similarity
func (s *SQLiteVectorStore) SearchSimilar(queryVector []float32, topK int) ([]*SearchResult, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Retrieve all vectors (for small-medium datasets this is acceptable)
	// For larger datasets, consider using a specialized vector database
	query := `SELECT id, document_id, chunk_id, vector_data, dimension, content, metadata, created_at
		FROM vector_entries`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query vectors: %v", err)
	}
	defer rows.Close()

	var results []*SearchResult

	for rows.Next() {
		entry, err := s.scanVectorEntry(rows)
		if err != nil {
			continue
		}

		// Calculate cosine similarity
		score := CosineSimilarity(queryVector, entry.Vector)

		result := &SearchResult{
			Entry: entry,
			Score: score,
		}

		// Parse metadata to get chunk index
		if entry.Metadata != "" {
			var meta map[string]interface{}
			if err := json.Unmarshal([]byte(entry.Metadata), &meta); err == nil {
				if chunkIdx, ok := meta["chunk_index"].(float64); ok {
					result.ChunkIndex = int(chunkIdx)
				}
			}
		}

		results = append(results, result)
	}

	// Sort by score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top K results
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// GetVector retrieves a vector by ID
func (s *SQLiteVectorStore) GetVector(id string) (*VectorEntry, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, document_id, chunk_id, vector_data, dimension, content, metadata, created_at
		FROM vector_entries WHERE id = ?`

	rows, err := s.db.Query(query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query vector: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	return s.scanVectorEntry(rows)
}

// DeleteVector deletes a vector by ID
func (s *SQLiteVectorStore) DeleteVector(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `DELETE FROM vector_entries WHERE id = ?`
	_, err := s.db.Exec(query, id)
	
	if err != nil {
		return fmt.Errorf("failed to delete vector: %v", err)
	}

	return nil
}

// DeleteDocumentVectors deletes all vectors for a document
func (s *SQLiteVectorStore) DeleteDocumentVectors(documentID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `DELETE FROM vector_entries WHERE document_id = ?`
	_, err := s.db.Exec(query, documentID)
	
	if err != nil {
		return fmt.Errorf("failed to delete document vectors: %v", err)
	}

	return nil
}

// GetDocumentVectors retrieves all vectors for a document
func (s *SQLiteVectorStore) GetDocumentVectors(documentID string) ([]*VectorEntry, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, document_id, chunk_id, vector_data, dimension, content, metadata, created_at
		FROM vector_entries WHERE document_id = ? ORDER BY created_at`

	rows, err := s.db.Query(query, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query document vectors: %v", err)
	}
	defer rows.Close()

	var vectors []*VectorEntry
	for rows.Next() {
		entry, err := s.scanVectorEntry(rows)
		if err != nil {
			continue
		}
		vectors = append(vectors, entry)
	}

	return vectors, nil
}

// Clear removes all vectors from the store
func (s *SQLiteVectorStore) Clear() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `DELETE FROM vector_entries`
	_, err := s.db.Exec(query)
	
	if err != nil {
		return fmt.Errorf("failed to clear vector store: %v", err)
	}

	return nil
}

// scanVectorEntry scans a row into a VectorEntry
func (s *SQLiteVectorStore) scanVectorEntry(rows *sql.Rows) (*VectorEntry, error) {
	var entry VectorEntry
	var vectorData string
	var chunkID sql.NullString
	var content sql.NullString
	var metadata sql.NullString

	err := rows.Scan(&entry.ID, &entry.DocumentID, &chunkID, &vectorData,
		&entry.Dimension, &content, &metadata, &entry.CreatedAt)
	
	if err != nil {
		return nil, fmt.Errorf("failed to scan vector entry: %v", err)
	}

	// Deserialize vector
	if err := json.Unmarshal([]byte(vectorData), &entry.Vector); err != nil {
		return nil, fmt.Errorf("failed to deserialize vector: %v", err)
	}

	if chunkID.Valid {
		entry.ChunkID = chunkID.String
	}

	if content.Valid {
		entry.Content = content.String
	}

	if metadata.Valid {
		entry.Metadata = metadata.String
	}

	return &entry, nil
}

// SQLiteDocumentStore implements DocumentStoreInterface using SQLite
type SQLiteDocumentStore struct {
	db    Database
	mutex sync.RWMutex
}

// NewSQLiteDocumentStore creates a new SQLite document store
func NewSQLiteDocumentStore(db Database) *SQLiteDocumentStore {
	store := &SQLiteDocumentStore{
		db: db,
	}
	// Initialize tables
	store.initializeTables()
	return store
}

// initializeTables creates the necessary tables if they don't exist
func (s *SQLiteDocumentStore) initializeTables() {
	if s.db == nil {
		return
	}

	query := `CREATE TABLE IF NOT EXISTS documents (
		id TEXT PRIMARY KEY,
		file_path TEXT,
		file_hash TEXT,
		metadata TEXT,
		chunk_count INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := s.db.Exec(query); err != nil {
		log.Printf("Warning: failed to create documents table: %v", err)
	}

	// Create index for file_path for efficient lookups
	indexQuery := `CREATE UNIQUE INDEX IF NOT EXISTS idx_document_filepath 
		ON documents(file_path) WHERE file_path IS NOT NULL;`

	if _, err := s.db.Exec(indexQuery); err != nil {
		log.Printf("Warning: failed to create document index: %v", err)
	}
}

// StoreDocument stores a document
func (s *SQLiteDocumentStore) StoreDocument(doc *Document) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Serialize metadata to JSON
	var metadataJSON string
	if doc.Metadata != nil {
		data, err := json.Marshal(doc.Metadata)
		if err != nil {
			return fmt.Errorf("failed to serialize metadata: %v", err)
		}
		metadataJSON = string(data)
	}

	query := `INSERT OR REPLACE INTO documents 
		(id, file_path, file_hash, metadata, chunk_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(query, doc.ID, doc.FilePath, doc.FileHash,
		metadataJSON, doc.ChunkCount, doc.CreatedAt, doc.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to store document: %v", err)
	}

	return nil
}

// GetDocument retrieves a document by ID
func (s *SQLiteDocumentStore) GetDocument(id string) (*Document, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, file_path, file_hash, metadata, chunk_count, created_at, updated_at
		FROM documents WHERE id = ?`

	rows, err := s.db.Query(query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query document: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	return s.scanDocument(rows)
}

// GetDocumentByPath retrieves a document by file path
func (s *SQLiteDocumentStore) GetDocumentByPath(filePath string) (*Document, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, file_path, file_hash, metadata, chunk_count, created_at, updated_at
		FROM documents WHERE file_path = ?`

	rows, err := s.db.Query(query, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to query document by path: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	return s.scanDocument(rows)
}

// UpdateDocument updates a document
func (s *SQLiteDocumentStore) UpdateDocument(doc *Document) error {
	doc.UpdatedAt = time.Now()
	return s.StoreDocument(doc)
}

// DeleteDocument deletes a document by ID
func (s *SQLiteDocumentStore) DeleteDocument(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `DELETE FROM documents WHERE id = ?`
	_, err := s.db.Exec(query, id)
	
	if err != nil {
		return fmt.Errorf("failed to delete document: %v", err)
	}

	return nil
}

// ListDocuments lists all documents
func (s *SQLiteDocumentStore) ListDocuments() ([]*Document, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, file_path, file_hash, metadata, chunk_count, created_at, updated_at
		FROM documents ORDER BY created_at DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents: %v", err)
	}
	defer rows.Close()

	var documents []*Document
	for rows.Next() {
		doc, err := s.scanDocument(rows)
		if err != nil {
			continue
		}
		documents = append(documents, doc)
	}

	return documents, nil
}

// scanDocument scans a row into a Document
func (s *SQLiteDocumentStore) scanDocument(rows *sql.Rows) (*Document, error) {
	var doc Document
	var filePath sql.NullString
	var fileHash sql.NullString
	var metadataJSON sql.NullString

	err := rows.Scan(&doc.ID, &filePath, &fileHash, &metadataJSON,
		&doc.ChunkCount, &doc.CreatedAt, &doc.UpdatedAt)
	
	if err != nil {
		return nil, fmt.Errorf("failed to scan document: %v", err)
	}

	if filePath.Valid {
		doc.FilePath = filePath.String
	}

	if fileHash.Valid {
		doc.FileHash = fileHash.String
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &doc.Metadata); err != nil {
			// Log warning but don't fail
			log.Printf("Warning: failed to deserialize document metadata: %v", err)
		}
	}

	return &doc, nil
}

// Global repository instances
var (
	defaultVectorStore   VectorStoreInterface
	defaultDocumentStore DocumentStoreInterface
)

// InitVectorStore initializes the default vector store
func InitVectorStore(db Database) {
	defaultVectorStore = NewSQLiteVectorStore(db)
}

// GetVectorStore returns the default vector store
func GetVectorStore() VectorStoreInterface {
	return defaultVectorStore
}

// InitDocumentStore initializes the default document store
func InitDocumentStore(db Database) {
	defaultDocumentStore = NewSQLiteDocumentStore(db)
}

// GetDocumentStore returns the default document store
func GetDocumentStore() DocumentStoreInterface {
	return defaultDocumentStore
}