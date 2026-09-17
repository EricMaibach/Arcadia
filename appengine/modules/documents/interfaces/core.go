package interfaces

import (
	"context"

	"arcadia/modules/documents/models"
)

// VectorStoreInterface defines the interface for vector storage
type VectorStoreInterface interface {
	// Initialization and management
	Initialize(ctx context.Context) error

	// Core vector operations
	StoreVector(ctx context.Context, entry *models.VectorEntry) error
	SearchSimilar(ctx context.Context, queryVector []float32, topK int) ([]*models.SearchResult, error)
	GetVector(ctx context.Context, id string) (*models.VectorEntry, error)
	DeleteVector(ctx context.Context, id string) error
	DeleteDocumentVectors(ctx context.Context, documentID string) error
	GetDocumentVectors(ctx context.Context, documentID string) ([]*models.VectorEntry, error)
	Clear(ctx context.Context) error

	// Enhanced vector store methods
	StoreBatch(ctx context.Context, entries []*models.VectorEntry) error
	SearchWithFilter(ctx context.Context, queryVector []float32, topK int, filter map[string]interface{}) ([]*models.SearchResult, error)
	HealthCheck(ctx context.Context) error
}

// DocumentStoreInterface defines the interface for document storage
type DocumentStoreInterface interface {
	StoreDocument(ctx context.Context, doc *models.Document) error
	GetDocument(ctx context.Context, id string) (*models.Document, error)
	GetDocumentByPath(ctx context.Context, filePath string) (*models.Document, error)
	UpdateDocument(ctx context.Context, doc *models.Document) error
	DeleteDocument(ctx context.Context, id string) error
	ListDocuments(ctx context.Context) ([]*models.Document, error)

	// Enhanced document store methods
	GetDocumentsByIDs(ctx context.Context, ids []string) ([]*models.Document, error)
	SearchDocumentsByMetadata(ctx context.Context, filter map[string]interface{}) ([]*models.Document, error)
	GetDocumentStats(ctx context.Context) (*models.DocumentStats, error)
}

// GraphStoreInterface defines the interface for graph database operations
type GraphStoreInterface interface {
	// Initialization and lifecycle
	Initialize(ctx context.Context) error
	Close() error
	HealthCheck(ctx context.Context) error

	// Basic quad operations
	AddQuad(ctx context.Context, quad models.Quad) error
	AddQuads(ctx context.Context, quads []models.Quad) error
	DeleteQuad(ctx context.Context, quad models.Quad) error
	DeleteQuads(ctx context.Context, quads []models.Quad) error

	// Query operations
	Query(ctx context.Context, query string) ([]map[string]interface{}, error)
	GetNode(ctx context.Context, nodeID string) (*models.GraphNode, error)
	GetEdges(ctx context.Context, nodeID string) ([]models.GraphEdge, error)

	// Document operations
	DeleteDocumentData(ctx context.Context, documentID string) error

	// Statistics
	GetStats(ctx context.Context) (*models.GraphStats, error)
}

// TextChunkerInterface defines the interface for text chunking
type TextChunkerInterface interface {
	ChunkText(content string, config models.ChunkingConfig) []models.TextChunk
	ChunkDocument(doc *models.Document, config models.ChunkingConfig) []models.TextChunk
	ValidateConfig(config models.ChunkingConfig) error
}

// DocumentProcessor defines the interface for processing documents
type DocumentProcessor interface {
	ProcessDocument(ctx context.Context, doc *models.Document, content string) error
	ProcessFile(ctx context.Context, filePath string) (*models.Document, error)
	ProcessFiles(ctx context.Context, filePaths []string) ([]models.ProcessResult, error)

	// Content analysis
	IsTextFile(filePath string) (bool, string, error)
	ReadTextFile(filePath string) (string, error)
	CalculateFileHash(filePath string) (string, error)
}

// SearchEngine defines the interface for document search operations
type SearchEngine interface {
	SearchDocuments(ctx context.Context, query string, topK int) ([]*models.DocumentSearchResult, error)
	SearchDocumentsEnhanced(ctx context.Context, query string, topK int, config models.SearchConfig) ([]*models.EnhancedDocumentSearchResult, error)
	SearchByVector(ctx context.Context, vector []float32, topK int) ([]*models.SearchResult, error)

	// Search utilities
	GenerateQueryEmbedding(ctx context.Context, query string) ([]float32, error)
	ReconstructContentFromChunks(vectors []*models.VectorEntry) string
}

// ContentAnalyzer defines the interface for content analysis
type ContentAnalyzer interface {
	AnalyzeFile(filePath string) (*models.FileAnalysis, error)
	ValidateContent(content string) error
	ExtractMetadata(filePath string, content string) (map[string]interface{}, error)
	DetectLanguage(content string) (string, float64, error)
}

// WorkerPool defines the interface for managing background processing
type WorkerPool interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	SubmitJob(job Job) error
	GetQueueSize() int
	GetActiveWorkers() int
}

// Job represents a unit of work for the worker pool
type Job interface {
	Execute(ctx context.Context) error
	GetID() string
	GetPriority() int
}

// FileWatcher defines the interface for watching file system changes
type FileWatcher interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	AddPath(path string) error
	RemovePath(path string) error
	GetWatchedPaths() []string
}

// FileEventHandler defines the interface for handling file system events
type FileEventHandler interface {
	OnFileCreated(ctx context.Context, filePath string) error
	OnFileModified(ctx context.Context, filePath string) error
	OnFileDeleted(ctx context.Context, filePath string) error
	OnFileRenamed(ctx context.Context, oldPath, newPath string) error
}
