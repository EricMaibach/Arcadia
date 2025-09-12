package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
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

// ChunkingStrategy defines how text should be chunked
type ChunkingStrategy string

const (
	ChunkingStrategyFixed     ChunkingStrategy = "fixed"
	ChunkingStrategySentence  ChunkingStrategy = "sentence"
	ChunkingStrategyParagraph ChunkingStrategy = "paragraph"
)

// ChunkingConfig defines configuration for text chunking
type ChunkingConfig struct {
	Strategy    ChunkingStrategy `json:"strategy"`
	MaxChunkSize int            `json:"max_chunk_size"`
	ChunkOverlap int            `json:"chunk_overlap"`
}

// DefaultChunkingConfig returns default chunking configuration
func DefaultChunkingConfig() ChunkingConfig {
	return ChunkingConfig{
		Strategy:     ChunkingStrategyFixed,
		MaxChunkSize: 512,  // Good default for all-MiniLM-L6-v2
		ChunkOverlap: 50,   // Small overlap to maintain context
	}
}

// TextChunkerInterface defines the interface for text chunking
type TextChunkerInterface interface {
	ChunkText(content string, config ChunkingConfig) []TextChunk
}

// EmbeddingModelInterface defines the interface for embedding generation
type EmbeddingModelInterface interface {
	GenerateEmbedding(text string) ([]float32, error)
	GetDimension() int
	Initialize() error
	Close() error
}

// VectorStoreInterface defines the interface for vector storage
type VectorStoreInterface interface {
	StoreVector(entry *VectorEntry) error
	SearchSimilar(queryVector []float32, topK int) ([]*SearchResult, error)
	GetVector(id string) (*VectorEntry, error)
	DeleteVector(id string) error
	DeleteDocumentVectors(documentID string) error
	GetDocumentVectors(documentID string) ([]*VectorEntry, error)
	Clear() error
}

// DocumentStoreInterface defines the interface for document storage
type DocumentStoreInterface interface {
	StoreDocument(doc *Document) error
	GetDocument(id string) (*Document, error)
	GetDocumentByPath(filePath string) (*Document, error)
	UpdateDocument(doc *Document) error
	DeleteDocument(id string) error
	ListDocuments() ([]*Document, error)
}

// EmbeddingServiceInterface defines the main interface for the embedding service
type EmbeddingServiceInterface interface {
	ProcessFile(filePath string) (*Document, error)
	SearchSimilar(query string, topK int) ([]*SearchResult, error)
	GetDocument(documentID string) (*Document, error)
	DeleteDocument(documentID string) error
	Initialize() error
	Close() error
}

// EmbeddingService implements the EmbeddingServiceInterface
type EmbeddingService struct {
	chunker       TextChunkerInterface
	model         EmbeddingModelInterface
	vectorStore   VectorStoreInterface
	documentStore DocumentStoreInterface
	config        ChunkingConfig
	mutex         sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
}

// Global embedding service instance
var defaultEmbeddingService *EmbeddingService

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(
	chunker TextChunkerInterface,
	model EmbeddingModelInterface,
	vectorStore VectorStoreInterface,
	documentStore DocumentStoreInterface,
	config ChunkingConfig,
) *EmbeddingService {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &EmbeddingService{
		chunker:       chunker,
		model:         model,
		vectorStore:   vectorStore,
		documentStore: documentStore,
		config:        config,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// generateDocumentID generates a random ID for documents
func generateDocumentID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "doc_" + hex.EncodeToString(bytes)
}

// generateChunkID generates a random ID for chunks
func generateChunkID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "chunk_" + hex.EncodeToString(bytes)
}

// calculateFileHash calculates SHA256 hash of a file
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// generateVectorID generates a random ID for vectors
func generateVectorID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "vec_" + hex.EncodeToString(bytes)
}

// Initialize initializes the embedding service
func (es *EmbeddingService) Initialize() error {
	es.mutex.Lock()
	defer es.mutex.Unlock()

	if es.model != nil {
		if err := es.model.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize embedding model: %v", err)
		}
	}

	return nil
}

// Close closes the embedding service
func (es *EmbeddingService) Close() error {
	es.mutex.Lock()
	defer es.mutex.Unlock()

	if es.cancel != nil {
		es.cancel()
	}

	if es.model != nil {
		if err := es.model.Close(); err != nil {
			return fmt.Errorf("failed to close embedding model: %v", err)
		}
	}

	return nil
}

// ProcessFile processes a file and stores its embeddings
func (es *EmbeddingService) ProcessFile(filePath string) (*Document, error) {
	es.mutex.Lock()
	defer es.mutex.Unlock()

	// Check if file is supported text type
	isText, contentType, err := es.isTextFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze file type: %v", err)
	}

	if !isText {
		log.Printf("[Embedding] Skipping unsupported file type %s for file: %s", contentType, filePath)
		return nil, fmt.Errorf("unsupported file type: %s (detected: %s)", filepath.Ext(filePath), contentType)
	}

	// Read file content
	content, err := es.readTextFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// Validate that content is valid UTF-8 text
	if !utf8.ValidString(content) {
		log.Printf("[Embedding] Skipping file with invalid UTF-8 content: %s", filePath)
		return nil, fmt.Errorf("file contains invalid UTF-8 content: %s", filePath)
	}

	// Calculate file hash for change detection
	fileHash, err := calculateFileHash(filePath)
	if err != nil {
		log.Printf("[Embedding] Warning: failed to calculate file hash for %s: %v", filePath, err)
		fileHash = ""
	}

	// Create document
	doc := &Document{
		ID:       generateDocumentID(),
		FilePath: filePath,
		FileHash: fileHash,
		Metadata: map[string]interface{}{
			"source":      "file",
			"path":        filePath,
			"content_type": contentType,
			"file_ext":    filepath.Ext(filePath),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	log.Printf("[Embedding] Processing text file: %s (type: %s)", filePath, contentType)

	// Process the document
	if err := es.processDocument(doc, content); err != nil {
		return nil, err
	}

	return doc, nil
}


// processDocument processes a document and generates embeddings
func (es *EmbeddingService) processDocument(doc *Document, content string) error {
	// Chunk the text
	chunks := es.chunker.ChunkText(content, es.config)
	doc.ChunkCount = len(chunks)

	// Store the document
	if err := es.documentStore.StoreDocument(doc); err != nil {
		return fmt.Errorf("failed to store document: %v", err)
	}

	// Process each chunk
	for i, chunk := range chunks {
		chunk.DocumentID = doc.ID
		chunk.ChunkIndex = i

		// Generate embedding
		embedding, err := es.model.GenerateEmbedding(chunk.Content)
		if err != nil {
			return fmt.Errorf("failed to generate embedding for chunk %d: %v", i, err)
		}

		// Create vector entry
		entry := &VectorEntry{
			ID:         generateVectorID(),
			DocumentID: doc.ID,
			ChunkID:    chunk.ID,
			Vector:     embedding,
			Dimension:  len(embedding),
			Content:    chunk.Content,
			Metadata:   fmt.Sprintf(`{"chunk_index":%d,"start_pos":%d,"end_pos":%d}`, i, chunk.StartPos, chunk.EndPos),
			CreatedAt:  time.Now(),
		}

		// Store the vector
		if err := es.vectorStore.StoreVector(entry); err != nil {
			return fmt.Errorf("failed to store vector for chunk %d: %v", i, err)
		}
	}

	return nil
}

// SearchSimilar searches for similar content
func (es *EmbeddingService) SearchSimilar(query string, topK int) ([]*SearchResult, error) {
	es.mutex.RLock()
	defer es.mutex.RUnlock()

	// Generate query embedding
	queryEmbedding, err := es.model.GenerateEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %v", err)
	}

	// Search for similar vectors
	results, err := es.vectorStore.SearchSimilar(queryEmbedding, topK)
	if err != nil {
		return nil, fmt.Errorf("failed to search similar vectors: %v", err)
	}

	// Enrich results with document information
	for _, result := range results {
		if result.Entry != nil && result.Entry.DocumentID != "" {
			doc, _ := es.documentStore.GetDocument(result.Entry.DocumentID)
			result.Document = doc
		}
	}

	return results, nil
}

// GetDocument retrieves a document by ID
func (es *EmbeddingService) GetDocument(documentID string) (*Document, error) {
	es.mutex.RLock()
	defer es.mutex.RUnlock()

	return es.documentStore.GetDocument(documentID)
}

// DeleteDocument deletes a document and its associated vectors
func (es *EmbeddingService) DeleteDocument(documentID string) error {
	es.mutex.Lock()
	defer es.mutex.Unlock()

	// Delete vectors
	if err := es.vectorStore.DeleteDocumentVectors(documentID); err != nil {
		return fmt.Errorf("failed to delete document vectors: %v", err)
	}

	// Delete document
	if err := es.documentStore.DeleteDocument(documentID); err != nil {
		return fmt.Errorf("failed to delete document: %v", err)
	}

	return nil
}

// isTextFile determines if a file is a text file using multiple detection methods
func (es *EmbeddingService) isTextFile(filePath string) (bool, string, error) {
	// Method 1: Check by file extension first (fast)
	if isTextByExtension(filePath) {
		return true, "text/plain", nil
	}

	// Method 2: Use HTTP content detection with file sample
	file, err := os.Open(filePath)
	if err != nil {
		return false, "", fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Read first 512 bytes for content type detection
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false, "", fmt.Errorf("failed to read file sample: %v", err)
	}

	// Detect content type using HTTP package
	contentType := http.DetectContentType(buffer[:n])
	
	// Method 3: Check if detected type is text-based
	if isTextContentType(contentType) {
		return true, contentType, nil
	}

	// Method 4: Binary heuristic - check if content is mostly printable UTF-8
	if n > 0 && isLikelyTextContent(buffer[:n]) {
		return true, "text/plain", nil
	}

	return false, contentType, nil
}

// isTextByExtension checks if file extension indicates text content
func isTextByExtension(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	textExtensions := map[string]bool{
		".txt":      true,
		".md":       true,
		".markdown": true,
		".rst":      true,
		".csv":      true,
		".tsv":      true,
		".log":      true,
		".conf":     true,
		".cfg":      true,
		".ini":      true,
		".yaml":     true,
		".yml":      true,
		".json":     true,
		".xml":      true,
		".html":     true,
		".htm":      true,
		".css":      true,
		".js":       true,
		".ts":       true,
		".py":       true,
		".go":       true,
		".java":     true,
		".c":        true,
		".cpp":      true,
		".h":        true,
		".hpp":      true,
		".sh":       true,
		".bash":     true,
		".zsh":      true,
		".fish":     true,
		".ps1":      true,
		".sql":      true,
		".r":        true,
		".rb":       true,
		".php":      true,
		".pl":       true,
		".tex":      true,
		".bib":      true,
		"":          true, // files without extension might be text
	}
	
	return textExtensions[ext]
}

// isTextContentType checks if HTTP-detected content type is text-based
func isTextContentType(contentType string) bool {
	// Split off charset if present
	mainType := strings.Split(contentType, ";")[0]
	mainType = strings.TrimSpace(strings.ToLower(mainType))
	
	textTypes := map[string]bool{
		"text/plain":             true,
		"text/html":              true,
		"text/css":               true,
		"text/javascript":        true,
		"text/csv":               true,
		"text/xml":               true,
		"application/json":       true,
		"application/xml":        true,
		"application/javascript": true,
		"application/x-sh":       true,
		"application/x-python":   true,
		"application/x-perl":     true,
		"application/x-ruby":     true,
		"application/x-php":      true,
		"application/sql":        true,
		"application/yaml":       true,
		"application/x-yaml":     true,
	}
	
	// Also check if it starts with "text/"
	if strings.HasPrefix(mainType, "text/") {
		return true
	}
	
	return textTypes[mainType]
}

// isLikelyTextContent uses heuristics to determine if binary data is likely text
func isLikelyTextContent(data []byte) bool {
	if len(data) == 0 {
		return true
	}

	// Check if content is valid UTF-8
	if !utf8.Valid(data) {
		return false
	}

	// Reject files with null bytes (common in binary files)
	for _, b := range data {
		if b == 0 {
			return false
		}
	}

	// Count printable vs non-printable characters
	printableCount := 0
	controlCount := 0
	
	for _, b := range data {
		switch {
		case b >= 32 && b <= 126: // ASCII printable
			printableCount++
		case b == '\t' || b == '\n' || b == '\r': // Common whitespace
			printableCount++
		case b < 32: // Control characters
			controlCount++
		}
	}
	
	// If more than 85% of characters are printable, consider it text
	totalChars := len(data)
	if totalChars == 0 {
		return true
	}
	
	printableRatio := float64(printableCount) / float64(totalChars)
	return printableRatio > 0.85
}

// readTextFile reads a text file from the filesystem
func (es *EmbeddingService) readTextFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %v", filePath, err)
	}
	
	return string(data), nil
}

// SimpleTextChunker implements TextChunkerInterface with basic chunking strategies
type SimpleTextChunker struct{}

// NewSimpleTextChunker creates a new simple text chunker
func NewSimpleTextChunker() *SimpleTextChunker {
	return &SimpleTextChunker{}
}

// ChunkText chunks text based on the specified strategy
func (c *SimpleTextChunker) ChunkText(content string, config ChunkingConfig) []TextChunk {
	switch config.Strategy {
	case ChunkingStrategySentence:
		return c.chunkBySentence(content, config)
	case ChunkingStrategyParagraph:
		return c.chunkByParagraph(content, config)
	default:
		return c.chunkFixed(content, config)
	}
}

// chunkFixed chunks text into fixed-size chunks with overlap
func (c *SimpleTextChunker) chunkFixed(content string, config ChunkingConfig) []TextChunk {
	var chunks []TextChunk
	contentRunes := []rune(content)
	length := len(contentRunes)
	
	if length == 0 {
		return chunks
	}

	start := 0
	chunkIndex := 0
	
	for start < length {
		end := start + config.MaxChunkSize
		if end > length {
			end = length
		}

		// Create chunk
		chunk := TextChunk{
			ID:         generateChunkID(),
			Content:    string(contentRunes[start:end]),
			StartPos:   start,
			EndPos:     end,
			ChunkIndex: chunkIndex,
			CreatedAt:  time.Now(),
		}
		chunks = append(chunks, chunk)

		// Move to next chunk with overlap
		start += config.MaxChunkSize - config.ChunkOverlap
		if start < 0 {
			start = 0
		}
		chunkIndex++
	}

	return chunks
}

// chunkBySentence chunks text by sentences
func (c *SimpleTextChunker) chunkBySentence(content string, config ChunkingConfig) []TextChunk {
	// Simple sentence splitting (can be improved with better NLP)
	sentences := strings.Split(content, ". ")
	var chunks []TextChunk
	var currentChunk strings.Builder
	currentStart := 0
	chunkIndex := 0

	for i, sentence := range sentences {
		// Add period back if not the last sentence
		if i < len(sentences)-1 {
			sentence += "."
		}

		// Check if adding this sentence would exceed max chunk size
		if currentChunk.Len()+len(sentence)+1 > config.MaxChunkSize && currentChunk.Len() > 0 {
			// Save current chunk
			chunk := TextChunk{
				ID:         generateChunkID(),
				Content:    currentChunk.String(),
				StartPos:   currentStart,
				EndPos:     currentStart + currentChunk.Len(),
				ChunkIndex: chunkIndex,
				CreatedAt:  time.Now(),
			}
			chunks = append(chunks, chunk)
			
			// Start new chunk
			currentStart += currentChunk.Len()
			currentChunk.Reset()
			chunkIndex++
		}

		// Add sentence to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString(" ")
		}
		currentChunk.WriteString(sentence)
	}

	// Add remaining chunk
	if currentChunk.Len() > 0 {
		chunk := TextChunk{
			ID:         generateChunkID(),
			Content:    currentChunk.String(),
			StartPos:   currentStart,
			EndPos:     currentStart + currentChunk.Len(),
			ChunkIndex: chunkIndex,
			CreatedAt:  time.Now(),
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

// chunkByParagraph chunks text by paragraphs
func (c *SimpleTextChunker) chunkByParagraph(content string, config ChunkingConfig) []TextChunk {
	// Split by double newline for paragraphs
	paragraphs := strings.Split(content, "\n\n")
	var chunks []TextChunk
	var currentChunk strings.Builder
	currentStart := 0
	chunkIndex := 0

	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		// Check if adding this paragraph would exceed max chunk size
		if currentChunk.Len()+len(paragraph)+2 > config.MaxChunkSize && currentChunk.Len() > 0 {
			// Save current chunk
			chunk := TextChunk{
				ID:         generateChunkID(),
				Content:    currentChunk.String(),
				StartPos:   currentStart,
				EndPos:     currentStart + currentChunk.Len(),
				ChunkIndex: chunkIndex,
				CreatedAt:  time.Now(),
			}
			chunks = append(chunks, chunk)
			
			// Start new chunk
			currentStart += currentChunk.Len()
			currentChunk.Reset()
			chunkIndex++
		}

		// Add paragraph to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(paragraph)
	}

	// Add remaining chunk
	if currentChunk.Len() > 0 {
		chunk := TextChunk{
			ID:         generateChunkID(),
			Content:    currentChunk.String(),
			StartPos:   currentStart,
			EndPos:     currentStart + currentChunk.Len(),
			ChunkIndex: chunkIndex,
			CreatedAt:  time.Now(),
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

// CosineSimilarity calculates cosine similarity between two vectors
func CosineSimilarity(a, b []float32) float32 {
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

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// Global functions for backward compatibility

// InitDefaultEmbeddingService initializes the default embedding service
func InitDefaultEmbeddingService(
	chunker TextChunkerInterface,
	model EmbeddingModelInterface,
	vectorStore VectorStoreInterface,
	documentStore DocumentStoreInterface,
	config ChunkingConfig,
) {
	defaultEmbeddingService = NewEmbeddingService(chunker, model, vectorStore, documentStore, config)
}

// GetDefaultEmbeddingService returns the default embedding service
func GetDefaultEmbeddingService() *EmbeddingService {
	return defaultEmbeddingService
}

// ProcessFileWithEmbeddings processes a file using the default service
func ProcessFileWithEmbeddings(filePath string) (*Document, error) {
	if defaultEmbeddingService == nil {
		return nil, fmt.Errorf("default embedding service not initialized")
	}
	return defaultEmbeddingService.ProcessFile(filePath)
}

// SearchSimilarContent searches for similar content using the default service
func SearchSimilarContent(query string, topK int) ([]*SearchResult, error) {
	if defaultEmbeddingService == nil {
		return nil, fmt.Errorf("default embedding service not initialized")
	}
	return defaultEmbeddingService.SearchSimilar(query, topK)
}