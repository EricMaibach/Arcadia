package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)


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

	if doc == nil {
		return fmt.Errorf("document is nil")
	}

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

	if id == "" {
		return nil, fmt.Errorf("document ID is empty")
	}

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

	if filePath == "" {
		return nil, fmt.Errorf("file path is empty")
	}

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
	if doc == nil {
		return fmt.Errorf("document is nil")
	}
	doc.UpdatedAt = time.Now()
	return s.StoreDocument(doc)
}

// DeleteDocument deletes a document by ID
func (s *SQLiteDocumentStore) DeleteDocument(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if id == "" {
		return fmt.Errorf("document ID is empty")
	}

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

// SetVectorStore sets the default vector store (used for migration and A/B testing)
func SetVectorStore(store VectorStoreInterface) {
	defaultVectorStore = store
}

