package models

import (
	"strings"
	"time"
)

// Document represents a document in the system
type Document struct {
	ID         string         `json:"id"`
	FilePath   string         `json:"file_path"`
	FileHash   string         `json:"file_hash"`
	Content    string         `json:"content"`
	ChunkCount int            `json:"chunk_count"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// ChunkResult represents a search result chunk
type ChunkResult struct {
	Content    string                 `json:"content"`
	Score      float32                `json:"score"`
	ChunkIndex int                    `json:"chunk_index"`
	DocumentID string                 `json:"document_id,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// DocumentSearchResult represents a document search result
type DocumentSearchResult struct {
	Document      *Document      `json:"document"`
	Chunks        []*ChunkResult `json:"chunks"`
	BestScore     float32        `json:"best_score"`
	TotalChunks   int            `json:"total_chunks"`
	RelevanceRank int            `json:"relevance_rank"`
	Highlights    []string       `json:"highlights,omitempty"`
}

// EnhancedDocumentSearchResult represents an enhanced document search result
type EnhancedDocumentSearchResult struct {
	Document          *Document              `json:"document"`
	ContextHighlights []string               `json:"context_highlights"`
	ContentPreview    string                 `json:"content_preview"`
	IsTruncated       bool                   `json:"is_truncated"`
	BestScore         float32                `json:"best_score"`
	RelevanceRank     int                    `json:"relevance_rank"`
	Summary           string                 `json:"summary,omitempty"`
	Keywords          []string               `json:"keywords,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// SearchResult represents a generic search result
type SearchResult struct {
	ID         string                 `json:"id"`
	Content    string                 `json:"content"`
	Score      float32                `json:"score"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Highlights []string               `json:"highlights,omitempty"`
	DocumentID string                 `json:"document_id,omitempty"`
	ChunkIndex int                    `json:"chunk_index,omitempty"`
}

// TextChunk represents a chunk of text
type TextChunk struct {
	ID         string                 `json:"id"`
	Content    string                 `json:"content"`
	Index      int                    `json:"index"`
	StartPos   int                    `json:"start_pos"`
	EndPos     int                    `json:"end_pos"`
	TokenCount int                    `json:"token_count"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ChunkingConfig represents configuration for text chunking
type ChunkingConfig struct {
	ChunkSize     int      `json:"chunk_size"`
	ChunkOverlap  int      `json:"chunk_overlap"`
	SplitMethod   string   `json:"split_method"`
	PreserveWords bool     `json:"preserve_words"`
	MinChunkSize  int      `json:"min_chunk_size"`
	MaxChunkSize  int      `json:"max_chunk_size"`
	Separators    []string `json:"separators,omitempty"`
}

// ProcessResult represents the result of document processing
type ProcessResult struct {
	DocumentID     string                 `json:"document_id"`
	FilePath       string                 `json:"file_path"`
	Success        bool                   `json:"success"`
	ChunksCreated  int                    `json:"chunks_created"`
	TokenCount     int                    `json:"token_count"`
	ProcessingTime float64                `json:"processing_time_ms"`
	Error          string                 `json:"error,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
}

// ContentAnalysis represents analysis of document content
type ContentAnalysis struct {
	DocumentID   string                 `json:"document_id"`
	WordCount    int                    `json:"word_count"`
	CharCount    int                    `json:"char_count"`
	Language     string                 `json:"language,omitempty"`
	Keywords     []string               `json:"keywords,omitempty"`
	Topics       []string               `json:"topics,omitempty"`
	Sentiment    string                 `json:"sentiment,omitempty"`
	ReadingLevel string                 `json:"reading_level,omitempty"`
	Summary      string                 `json:"summary,omitempty"`
	Entities     []string               `json:"entities,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// DocumentStats represents statistics about documents
type DocumentStats struct {
	TotalDocuments      int        `json:"total_documents"`
	TotalChunks         int        `json:"total_chunks"`
	TotalTokens         int64      `json:"total_tokens"`
	AverageChunksPerDoc float64    `json:"average_chunks_per_doc"`
	AverageTokensPerDoc float64    `json:"average_tokens_per_doc"`
	ProcessedToday      int        `json:"processed_today"`
	ProcessedThisWeek   int        `json:"processed_this_week"`
	ProcessedThisMonth  int        `json:"processed_this_month"`
	StorageSize         int64      `json:"storage_size_bytes"`
	LastProcessed       *time.Time `json:"last_processed,omitempty"`
}

// ProcessingRecord represents a record of document processing
type ProcessingRecord struct {
	ID            string                 `json:"id"`
	DocumentID    string                 `json:"document_id"`
	FilePath      string                 `json:"file_path"`
	Operation     string                 `json:"operation"`
	Status        string                 `json:"status"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	Duration      float64                `json:"duration_ms,omitempty"`
	ChunksCreated int                    `json:"chunks_created,omitempty"`
	TokenCount    int                    `json:"token_count,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// SearchRecord represents a record of search operations
type SearchRecord struct {
	ID          string                 `json:"id"`
	Query       string                 `json:"query"`
	ResultCount int                    `json:"result_count"`
	Duration    float64                `json:"duration_ms"`
	SearchType  string                 `json:"search_type"`
	Filters     map[string]interface{} `json:"filters,omitempty"`
	TopK        int                    `json:"top_k"`
	Timestamp   time.Time              `json:"timestamp"`
	UserID      string                 `json:"user_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// IntegrityReport represents a data integrity report
type IntegrityReport struct {
	Timestamp          time.Time              `json:"timestamp"`
	TotalDocuments     int                    `json:"total_documents"`
	TotalChunks        int                    `json:"total_chunks"`
	OrphanedChunks     int                    `json:"orphaned_chunks"`
	MissingDocuments   []string               `json:"missing_documents,omitempty"`
	CorruptedChunks    []string               `json:"corrupted_chunks,omitempty"`
	InconsistentMeta   []string               `json:"inconsistent_metadata,omitempty"`
	Issues             []string               `json:"issues,omitempty"`
	HealthScore        float64                `json:"health_score"`
	RecommendedActions []string               `json:"recommended_actions,omitempty"`
	Details            map[string]interface{} `json:"details,omitempty"`
}

// FileChange represents a file system change
type FileChange struct {
	Path      string                 `json:"path"`
	Operation string                 `json:"operation"` // "create", "modify", "delete"
	Size      int64                  `json:"size,omitempty"`
	ModTime   time.Time              `json:"mod_time"`
	Hash      string                 `json:"hash,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ConversationSearchResult represents a search result from conversation history
type ConversationSearchResult struct {
	ContextID    string                 `json:"context_id"`
	MessageIndex int                    `json:"message_index"`
	Message      *Message               `json:"message"`
	Score        float32                `json:"score"`
	Highlights   []string               `json:"highlights,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// DocumentFilter represents filters for document queries
type DocumentFilter struct {
	FileExtensions []string               `json:"file_extensions,omitempty"`
	CreatedAfter   *time.Time             `json:"created_after,omitempty"`
	CreatedBefore  *time.Time             `json:"created_before,omitempty"`
	MinSize        *int64                 `json:"min_size,omitempty"`
	MaxSize        *int64                 `json:"max_size,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Limit          int                    `json:"limit,omitempty"`
	Offset         int                    `json:"offset,omitempty"`
}

// SearchFilter represents filters for search operations
type SearchFilter struct {
	DocumentIDs    []string               `json:"document_ids,omitempty"`
	FileTypes      []string               `json:"file_types,omitempty"`
	DateRange      *DateRange             `json:"date_range,omitempty"`
	ScoreThreshold float32                `json:"score_threshold,omitempty"`
	MaxResults     int                    `json:"max_results,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// DateRange represents a date range filter
type DateRange struct {
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
}

// DocumentUpdate represents updates to a document
type DocumentUpdate struct {
	Content   *string                `json:"content,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// Default chunking configuration
func DefaultChunkingConfig() ChunkingConfig {
	return ChunkingConfig{
		ChunkSize:     1000,
		ChunkOverlap:  200,
		SplitMethod:   "recursive",
		PreserveWords: true,
		MinChunkSize:  100,
		MaxChunkSize:  2000,
		Separators:    []string{"\n\n", "\n", ". ", "! ", "? ", " "},
	}
}

// Validate validates a chunking configuration
func (cc *ChunkingConfig) Validate() error {
	if cc.ChunkSize <= 0 {
		return NewValidationError("chunk_size", "positive", "chunk size must be positive", cc.ChunkSize)
	}

	if cc.ChunkOverlap < 0 {
		return NewValidationError("chunk_overlap", "non_negative", "chunk overlap must be non-negative", cc.ChunkOverlap)
	}

	if cc.ChunkOverlap >= cc.ChunkSize {
		return NewValidationError("chunk_overlap", "less_than_chunk_size", "chunk overlap must be less than chunk size", cc.ChunkOverlap)
	}

	if cc.MinChunkSize < 0 {
		return NewValidationError("min_chunk_size", "non_negative", "min chunk size must be non-negative", cc.MinChunkSize)
	}

	if cc.MaxChunkSize > 0 && cc.MaxChunkSize < cc.ChunkSize {
		return NewValidationError("max_chunk_size", "greater_than_chunk_size", "max chunk size must be greater than chunk size", cc.MaxChunkSize)
	}

	validMethods := []string{"recursive", "character", "token", "sentence", "paragraph"}
	validMethod := false
	for _, method := range validMethods {
		if cc.SplitMethod == method {
			validMethod = true
			break
		}
	}
	if !validMethod {
		return NewValidationError("split_method", "enum", "invalid split method", cc.SplitMethod)
	}

	return nil
}

// GetEffectiveChunkSize returns the effective chunk size considering min/max limits
func (cc *ChunkingConfig) GetEffectiveChunkSize() int {
	size := cc.ChunkSize

	if cc.MinChunkSize > 0 && size < cc.MinChunkSize {
		size = cc.MinChunkSize
	}

	if cc.MaxChunkSize > 0 && size > cc.MaxChunkSize {
		size = cc.MaxChunkSize
	}

	return size
}

// CalculateScore calculates a relevance score for a search result
func (sr *SearchResult) CalculateScore(query string) float32 {
	// This is a placeholder implementation
	// In a real system, this would use sophisticated scoring algorithms
	return sr.Score
}

// IsRelevant checks if a search result meets a relevance threshold
func (sr *SearchResult) IsRelevant(threshold float32) bool {
	return sr.Score >= threshold
}

// GetHighlightedContent returns content with highlights applied
func (sr *SearchResult) GetHighlightedContent() string {
	content := sr.Content
	// This is a placeholder implementation
	// In a real system, this would apply highlighting markup
	return content
}

// AddMetadata adds metadata to a document
func (d *Document) AddMetadata(key string, value interface{}) {
	if d.Metadata == nil {
		d.Metadata = make(map[string]any)
	}
	d.Metadata[key] = value
}

// GetMetadata gets metadata from a document
func (d *Document) GetMetadata(key string) (interface{}, bool) {
	if d.Metadata == nil {
		return nil, false
	}
	value, exists := d.Metadata[key]
	return value, exists
}

// UpdateContent updates the document content and timestamp
func (d *Document) UpdateContent(content string) {
	d.Content = content
	d.UpdatedAt = time.Now()
}

// GetWordCount returns the approximate word count of the document
func (d *Document) GetWordCount() int {
	return len(strings.Fields(d.Content))
}

// GetCharCount returns the character count of the document
func (d *Document) GetCharCount() int {
	return len(d.Content)
}
