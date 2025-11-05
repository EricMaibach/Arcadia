package models

import (
	"time"
)

// TextChunk represents a chunk of text with metadata
type TextChunk struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	Content    string    `json:"content"`
	StartPos   int       `json:"start_pos"`
	EndPos     int       `json:"end_pos"`
	ChunkIndex int       `json:"chunk_index"`
	CreatedAt  time.Time `json:"created_at"`
}

// Document represents a document in the system
type Document struct {
	ID         string                 `json:"id"`
	FilePath   string                 `json:"file_path"`
	FileHash   string                 `json:"file_hash"`
	Content    string                 `json:"content"`               // Reconstructed from chunks
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	ChunkCount int                    `json:"chunk_count"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// VectorEntry represents an embedding vector with metadata
type VectorEntry struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	ChunkID    string    `json:"chunk_id"`
	Vector     []float32 `json:"vector"`
	Dimension  int       `json:"dimension"`
	Content    string    `json:"content"`
	Metadata   string    `json:"metadata,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// SearchResult represents a vector search result
type SearchResult struct {
	Entry      *VectorEntry `json:"entry"`
	Score      float32      `json:"score"`
	Document   *Document    `json:"document,omitempty"`
	ChunkIndex int          `json:"chunk_index"`
}

// DocumentSearchResult represents a document with its matching chunks
type DocumentSearchResult struct {
	Document      *Document      `json:"document"`
	Chunks        []*ChunkResult `json:"chunks"`
	BestScore     float32        `json:"best_score"`
	TotalChunks   int            `json:"total_chunks"`
	RelevanceRank int            `json:"relevance_rank"`
}

// ChunkResult represents a matching chunk with its score
type ChunkResult struct {
	Content    string  `json:"content"`
	Score      float32 `json:"score"`
	ChunkIndex int     `json:"chunk_index"`
}

// EnhancedDocumentSearchResult represents an enhanced document search result with full content
type EnhancedDocumentSearchResult struct {
	Document          *Document `json:"document"`
	BestScore         float32   `json:"best_score"`
	RelevanceRank     int       `json:"relevance_rank"`
	ContextHighlights []string  `json:"context_highlights"`
	ContentPreview    string    `json:"content_preview"` // First 500 chars
	IsTruncated       bool      `json:"is_truncated"`
}

// SearchConfig defines configuration for document search
type SearchConfig struct {
	MaxDocumentSize    int  `json:"max_document_size"`    // 50KB default
	MaxHighlights      int  `json:"max_highlights"`       // 3 default
	IncludeFullContent bool `json:"include_full_content"` // true default
}

// ChunkingStrategy defines how text should be chunked
type ChunkingStrategy string

const (
	ChunkingStrategyFixed     ChunkingStrategy = "fixed"
	ChunkingStrategySentence  ChunkingStrategy = "sentence"
	ChunkingStrategyParagraph ChunkingStrategy = "paragraph"
)

// ChunkingConfig defines configuration for text chunking
type ChunkingConfig struct {
	Strategy     ChunkingStrategy `json:"strategy"`
	MaxChunkSize int              `json:"max_chunk_size"`
	ChunkOverlap int              `json:"chunk_overlap"`
}

// ProcessResult represents the result of processing a file
type ProcessResult struct {
	DocumentID string     `json:"document_id"`
	FilePath   string     `json:"file_path"`
	Success    bool       `json:"success"`
	Error      error      `json:"error,omitempty"`
	Document   *Document  `json:"document,omitempty"`
	ProcessedAt time.Time `json:"processed_at"`
}

// HealthStatus represents the health status of the module
type HealthStatus struct {
	Status     string                 `json:"status"`     // "healthy", "degraded", "unhealthy"
	Timestamp  time.Time              `json:"timestamp"`
	Components map[string]interface{} `json:"components"` // component-specific health info
	Message    string                 `json:"message,omitempty"`
}

// ModuleMetrics contains metrics for the documents module
type ModuleMetrics struct {
	TotalDocuments     int64                  `json:"total_documents"`
	TotalChunks        int64                  `json:"total_chunks"`
	TotalVectors       int64                  `json:"total_vectors"`
	ProcessingQueue    int                    `json:"processing_queue"`
	LastProcessedAt    *time.Time             `json:"last_processed_at,omitempty"`
	AverageChunkSize   float64                `json:"average_chunk_size"`
	ProcessingLatency  map[string]float64     `json:"processing_latency"` // percentiles
	ErrorRate          float64                `json:"error_rate"`
	CustomMetrics      map[string]interface{} `json:"custom_metrics,omitempty"`
}

// DefaultChunkingConfig returns default chunking configuration
func DefaultChunkingConfig() ChunkingConfig {
	return ChunkingConfig{
		Strategy:     ChunkingStrategyFixed,
		MaxChunkSize: 512, // Good default for all-MiniLM-L6-v2
		ChunkOverlap: 50,  // Small overlap to maintain context
	}
}

// DocumentStats contains statistics about documents
type DocumentStats struct {
	TotalDocuments    int64                  `json:"total_documents"`
	TotalChunks       int64                  `json:"total_chunks"`
	TotalVectors      int64                  `json:"total_vectors"`
	AverageChunkSize  float64                `json:"average_chunk_size"`
	TotalSize         int64                  `json:"total_size_bytes"`
	FileTypes         map[string]int64       `json:"file_types"`
	ProcessingStats   map[string]interface{} `json:"processing_stats"`
}

// FileAnalysis contains the result of analyzing a file
type FileAnalysis struct {
	FilePath     string                 `json:"file_path"`
	Size         int64                  `json:"size"`
	ContentType  string                 `json:"content_type"`
	Language     string                 `json:"language,omitempty"`
	Confidence   float64                `json:"confidence,omitempty"`
	IsText       bool                   `json:"is_text"`
	Encoding     string                 `json:"encoding,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Summary      string                 `json:"summary,omitempty"`
}

// ContentAnalysis contains the result of analyzing content
type ContentAnalysis struct {
	Language      string                 `json:"language,omitempty"`
	Confidence    float64                `json:"confidence,omitempty"`
	WordCount     int                    `json:"word_count"`
	CharCount     int                    `json:"char_count"`
	SentenceCount int                    `json:"sentence_count"`
	Keywords      []string               `json:"keywords,omitempty"`
	Topics        []string               `json:"topics,omitempty"`
	Sentiment     *SentimentAnalysis     `json:"sentiment,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// SentimentAnalysis contains sentiment analysis results
type SentimentAnalysis struct {
	Score      float64 `json:"score"`       // -1.0 to 1.0
	Magnitude  float64 `json:"magnitude"`   // 0.0 to 1.0
	Label      string  `json:"label"`       // "positive", "negative", "neutral"
	Confidence float64 `json:"confidence"`  // 0.0 to 1.0
}

// ProcessingRecord contains a record of document processing
type ProcessingRecord struct {
	ID          string                 `json:"id"`
	DocumentID  string                 `json:"document_id"`
	FilePath    string                 `json:"file_path"`
	Status      string                 `json:"status"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Duration    float64                `json:"duration_ms"`
	ChunkCount  int                    `json:"chunk_count"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// SearchRecord contains a record of search queries
type SearchRecord struct {
	ID           string    `json:"id"`
	Query        string    `json:"query"`
	ResultCount  int       `json:"result_count"`
	Duration     float64   `json:"duration_ms"`
	Timestamp    time.Time `json:"timestamp"`
	UserID       string    `json:"user_id,omitempty"`
	SessionID    string    `json:"session_id,omitempty"`
	SearchConfig SearchConfig `json:"search_config"`
}

// IntegrityReport contains the result of data integrity validation
type IntegrityReport struct {
	TotalDocuments      int64                  `json:"total_documents"`
	OrphanedChunks      int64                  `json:"orphaned_chunks"`
	OrphanedVectors     int64                  `json:"orphaned_vectors"`
	MissingVectors      int64                  `json:"missing_vectors"`
	InconsistentHashes  int64                  `json:"inconsistent_hashes"`
	CorruptedDocuments  []string               `json:"corrupted_documents,omitempty"`
	Issues              []IntegrityIssue       `json:"issues,omitempty"`
	RecommendedActions  []string               `json:"recommended_actions,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// IntegrityIssue represents a specific data integrity issue
type IntegrityIssue struct {
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	DocumentID  string                 `json:"document_id,omitempty"`
	ChunkID     string                 `json:"chunk_id,omitempty"`
	VectorID    string                 `json:"vector_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DefaultSearchConfig returns default search configuration
func DefaultSearchConfig() SearchConfig {
	return SearchConfig{
		MaxDocumentSize:    50 * 1024, // 50KB
		MaxHighlights:      10,
		IncludeFullContent: false,
	}
}