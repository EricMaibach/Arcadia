package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test text chunking strategies
func TestSimpleTextChunker_FixedStrategy(t *testing.T) {
	chunker := NewSimpleTextChunker()
	config := ChunkingConfig{
		Strategy:     ChunkingStrategyFixed,
		MaxChunkSize: 100,
		ChunkOverlap: 20,
	}

	text := "This is a test text that should be chunked into multiple pieces. " +
		"Each chunk should have a maximum size of 100 characters. " +
		"The chunks should also have some overlap to maintain context. " +
		"This helps with better embedding generation and retrieval."

	chunks := chunker.ChunkText(text, config)

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}

	// Check chunk sizes
	for i, chunk := range chunks {
		if len(chunk.Content) > config.MaxChunkSize {
			t.Errorf("Chunk %d exceeds max size: %d > %d", i, len(chunk.Content), config.MaxChunkSize)
		}
		
		if chunk.ChunkIndex != i {
			t.Errorf("Expected chunk index %d, got %d", i, chunk.ChunkIndex)
		}

		if chunk.ID == "" {
			t.Error("Chunk ID should not be empty")
		}
	}

	// Check overlap
	if len(chunks) > 1 {
		// There should be some overlap between consecutive chunks
		firstEnd := chunks[0].EndPos
		secondStart := chunks[1].StartPos
		overlap := firstEnd - secondStart
		
		if overlap <= 0 {
			t.Error("Expected some overlap between chunks")
		}
	}
}

func TestSimpleTextChunker_SentenceStrategy(t *testing.T) {
	chunker := NewSimpleTextChunker()
	config := ChunkingConfig{
		Strategy:     ChunkingStrategySentence,
		MaxChunkSize: 150,
		ChunkOverlap: 0,
	}

	text := "This is the first sentence. This is the second sentence. " +
		"This is the third sentence. This is the fourth sentence."

	chunks := chunker.ChunkText(text, config)

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}

	// Each chunk should contain complete sentences
	for _, chunk := range chunks {
		if chunk.Content == "" {
			t.Error("Chunk content should not be empty")
		}
		
		// Check that content doesn't exceed max size
		if len(chunk.Content) > config.MaxChunkSize {
			t.Errorf("Chunk exceeds max size: %d > %d", len(chunk.Content), config.MaxChunkSize)
		}
	}
}

func TestSimpleTextChunker_ParagraphStrategy(t *testing.T) {
	chunker := NewSimpleTextChunker()
	config := ChunkingConfig{
		Strategy:     ChunkingStrategyParagraph,
		MaxChunkSize: 200,
		ChunkOverlap: 0,
	}

	text := "This is the first paragraph with some content.\n\n" +
		"This is the second paragraph with more content.\n\n" +
		"This is the third paragraph with even more content."

	chunks := chunker.ChunkText(text, config)

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}

	// Check that chunks respect paragraph boundaries
	for _, chunk := range chunks {
		if chunk.Content == "" {
			t.Error("Chunk content should not be empty")
		}
	}
}

func TestSimpleTextChunker_EmptyText(t *testing.T) {
	chunker := NewSimpleTextChunker()
	config := DefaultChunkingConfig()

	chunks := chunker.ChunkText("", config)

	if len(chunks) != 0 {
		t.Error("Expected no chunks for empty text")
	}
}

// Test Ollama embedding model (without requiring Ollama to be running)
func TestOllamaEmbeddingModel_Structure(t *testing.T) {
	model := NewOllamaEmbeddingModel("http://localhost:11434", "embeddinggemma")

	// Test dimension (should be 768 for EmbeddingGemma)
	dim := model.GetDimension()
	if dim != 768 {
		t.Errorf("Expected dimension 768, got %d", dim)
	}

	// Test that model is not initialized before calling Initialize()
	if model.initialized {
		t.Error("Model should not be initialized before calling Initialize()")
	}

	// Test close without initialization (should not error)
	err := model.Close()
	if err != nil {
		t.Errorf("Failed to close uninitialized model: %v", err)
	}

	// Test backward compatibility constructor
	oldModel := NewONNXEmbeddingModel("/deprecated/path")
	if oldModel.GetDimension() != 768 {
		t.Errorf("Backward compatibility model should return 768 dimensions, got %d", oldModel.GetDimension())
	}
}

func TestMockEmbeddingModel(t *testing.T) {
	model := NewMockEmbeddingModel(128)

	// Test without initialization
	_, err := model.GenerateEmbedding("test")
	if err == nil {
		t.Error("Expected error when generating embedding without initialization")
	}

	// Initialize
	err = model.Initialize()
	if err != nil {
		t.Errorf("Failed to initialize mock model: %v", err)
	}

	// Test embedding generation
	embedding, err := model.GenerateEmbedding("test text")
	if err != nil {
		t.Errorf("Failed to generate embedding: %v", err)
	}

	if len(embedding) != 128 {
		t.Errorf("Expected embedding dimension 128, got %d", len(embedding))
	}

	// Test error simulation
	model.SetError(true, "test error")
	_, err = model.GenerateEmbedding("test")
	if err == nil {
		t.Error("Expected error when error flag is set")
	}
}

// Test vector store
func TestSQLiteVectorStore(t *testing.T) {
	// Create temporary database
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "test_vectors.db")
	defer os.Remove(dbPath)

	db, err := NewSQLiteDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	store := NewSQLiteVectorStore(db)

	// Create test vector entry
	entry := &VectorEntry{
		ID:         "vec1",
		DocumentID: "doc1",
		ChunkID:    "chunk1",
		Vector:     []float32{0.1, 0.2, 0.3, 0.4},
		Dimension:  4,
		Content:    "test content",
		Metadata:   `{"test": "metadata"}`,
		CreatedAt:  time.Now(),
	}

	// Test store vector
	err = store.StoreVector(entry)
	if err != nil {
		t.Errorf("Failed to store vector: %v", err)
	}

	// Test get vector
	retrieved, err := store.GetVector("vec1")
	if err != nil {
		t.Errorf("Failed to get vector: %v", err)
	}

	if retrieved == nil {
		t.Error("Expected to retrieve vector")
	} else {
		if retrieved.ID != entry.ID {
			t.Errorf("Expected ID %s, got %s", entry.ID, retrieved.ID)
		}
		if len(retrieved.Vector) != len(entry.Vector) {
			t.Errorf("Expected vector length %d, got %d", len(entry.Vector), len(retrieved.Vector))
		}
	}

	// Test search similar
	queryVector := []float32{0.1, 0.2, 0.3, 0.4}
	results, err := store.SearchSimilar(queryVector, 5)
	if err != nil {
		t.Errorf("Failed to search similar: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected at least one search result")
	} else {
		// Should find exact match with high similarity
		if results[0].Score < 0.99 {
			t.Errorf("Expected high similarity score for exact match, got %f", results[0].Score)
		}
	}

	// Test get document vectors
	docVectors, err := store.GetDocumentVectors("doc1")
	if err != nil {
		t.Errorf("Failed to get document vectors: %v", err)
	}

	if len(docVectors) != 1 {
		t.Errorf("Expected 1 document vector, got %d", len(docVectors))
	}

	// Test delete vector
	err = store.DeleteVector("vec1")
	if err != nil {
		t.Errorf("Failed to delete vector: %v", err)
	}

	retrieved, err = store.GetVector("vec1")
	if err != nil {
		t.Errorf("Failed to get vector after delete: %v", err)
	}
	if retrieved != nil {
		t.Error("Vector should be deleted")
	}
}

// Test document store
func TestSQLiteDocumentStore(t *testing.T) {
	// Create temporary database
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "test_documents.db")
	defer os.Remove(dbPath)

	db, err := NewSQLiteDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	store := NewSQLiteDocumentStore(db)

	// Create test document
	doc := &Document{
		ID:       "doc1",
		FilePath: "/test/path.txt",
		FileHash: "abc123hash",
		Metadata: map[string]interface{}{
			"author": "test",
			"date":   "2024-01-01",
		},
		ChunkCount: 5,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Test store document
	err = store.StoreDocument(doc)
	if err != nil {
		t.Errorf("Failed to store document: %v", err)
	}

	// Test get document
	retrieved, err := store.GetDocument("doc1")
	if err != nil {
		t.Errorf("Failed to get document: %v", err)
	}

	if retrieved == nil {
		t.Error("Expected to retrieve document")
	} else {
		if retrieved.ID != doc.ID {
			t.Errorf("Expected ID %s, got %s", doc.ID, retrieved.ID)
		}
		if retrieved.FilePath != doc.FilePath {
			t.Errorf("Expected file path %s, got %s", doc.FilePath, retrieved.FilePath)
		}
		if retrieved.ChunkCount != doc.ChunkCount {
			t.Errorf("Expected chunk count %d, got %d", doc.ChunkCount, retrieved.ChunkCount)
		}
	}

	// Test get document by path
	retrieved, err = store.GetDocumentByPath("/test/path.txt")
	if err != nil {
		t.Errorf("Failed to get document by path: %v", err)
	}

	if retrieved == nil {
		t.Error("Expected to retrieve document by path")
	}

	// Test update document
	doc.FileHash = "updated_hash"
	err = store.UpdateDocument(doc)
	if err != nil {
		t.Errorf("Failed to update document: %v", err)
	}

	// Test list documents
	docs, err := store.ListDocuments()
	if err != nil {
		t.Errorf("Failed to list documents: %v", err)
	}

	if len(docs) != 1 {
		t.Errorf("Expected 1 document, got %d", len(docs))
	}

	// Test delete document
	err = store.DeleteDocument("doc1")
	if err != nil {
		t.Errorf("Failed to delete document: %v", err)
	}

	retrieved, err = store.GetDocument("doc1")
	if err != nil {
		t.Errorf("Failed to get document after delete: %v", err)
	}
	if retrieved != nil {
		t.Error("Document should be deleted")
	}
}

// Test cosine similarity
func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{-1, 0, 0},
			expected: -1.0,
		},
		{
			name:     "different lengths",
			a:        []float32{1, 0},
			b:        []float32{1, 0, 0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CosineSimilarity(tt.a, tt.b)
			if result < tt.expected-0.01 || result > tt.expected+0.01 {
				t.Errorf("Expected similarity %f, got %f", tt.expected, result)
			}
		})
	}
}

// Integration test for embedding service
func TestEmbeddingService_Integration(t *testing.T) {
	// Create temporary database
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "test_embedding.db")
	defer os.Remove(dbPath)

	db, err := NewSQLiteDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create components
	chunker := NewSimpleTextChunker()
	model := NewMockEmbeddingModel(128)
	vectorStore := NewSQLiteVectorStore(db)
	documentStore := NewSQLiteDocumentStore(db)
	config := ChunkingConfig{
		Strategy:     ChunkingStrategyFixed,
		MaxChunkSize: 100,
		ChunkOverlap: 20,
	}

	// Create service
	service := NewEmbeddingService(chunker, model, vectorStore, documentStore, config)

	// Initialize
	err = service.Initialize()
	if err != nil {
		t.Errorf("Failed to initialize service: %v", err)
	}

	// Create temporary file for testing
	content := "This is a test document with some content. " +
		"It should be chunked and embedded properly. " +
		"Each chunk will have its own embedding vector."
	
	testFile := filepath.Join(tempDir, "test_integration.txt")
	err = os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	// Process file
	doc, err := service.ProcessFile(testFile)

	if err != nil {
		t.Errorf("Failed to process file: %v", err)
	}

	if doc == nil {
		t.Fatal("Expected document to be created")
	}

	if doc.ChunkCount == 0 {
		t.Error("Expected chunks to be created")
	}

	// Search similar content
	results, err := service.SearchSimilar("test document", 5)
	if err != nil {
		t.Errorf("Failed to search similar: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected search results")
	}

	// Get document
	retrieved, err := service.GetDocument(doc.ID)
	if err != nil {
		t.Errorf("Failed to get document: %v", err)
	}

	if retrieved == nil {
		t.Error("Expected to retrieve document")
	}

	// Delete document
	err = service.DeleteDocument(doc.ID)
	if err != nil {
		t.Errorf("Failed to delete document: %v", err)
	}

	// Verify deletion
	retrieved, err = service.GetDocument(doc.ID)
	if err != nil {
		t.Errorf("Failed to get document after delete: %v", err)
	}
	if retrieved != nil {
		t.Error("Document should be deleted")
	}

	// Close service
	err = service.Close()
	if err != nil {
		t.Errorf("Failed to close service: %v", err)
	}
}

// Test global functions
func TestEmbeddingService_GlobalFunctions(t *testing.T) {
	// Reset global service
	defaultEmbeddingService = nil

	// Test without initialization
	_, err := ProcessFileWithEmbeddings("/test/file.txt")
	if err == nil {
		t.Error("Expected error when service not initialized")
	}

	_, err = SearchSimilarContent("query", 5)
	if err == nil {
		t.Error("Expected error when service not initialized")
	}

	// Create temporary database
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "test_global_embedding.db")
	defer os.Remove(dbPath)

	db, err := NewSQLiteDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Initialize global service
	chunker := NewSimpleTextChunker()
	model := NewMockEmbeddingModel(128)
	model.Initialize()
	vectorStore := NewSQLiteVectorStore(db)
	documentStore := NewSQLiteDocumentStore(db)
	config := DefaultChunkingConfig()

	InitDefaultEmbeddingService(chunker, model, vectorStore, documentStore, config)

	if GetDefaultEmbeddingService() == nil {
		t.Error("Expected default service to be initialized")
	}

	// Initialize the service
	GetDefaultEmbeddingService().Initialize()

	// Test global functions (ProcessFileWithEmbeddings would fail due to file reading)
	// But SearchSimilarContent should work
	results, err := SearchSimilarContent("test query", 5)
	if err != nil {
		t.Errorf("SearchSimilarContent failed: %v", err)
	}

	// Results might be nil or empty, both are acceptable for an empty database
	// The important thing is that the function doesn't error
	_ = results
}

// Test file type detection
func TestFileExtensionDetection(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"test.txt", true},
		{"test.md", true},
		{"test.json", true},
		{"test.py", true},
		{"test.go", true},
		{"test.pdf", false},
		{"test.jpg", false},
		{"test.png", false},
		{"test.mp4", false},
		{"test.exe", false},
		{"README", true}, // no extension
		{"test.unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := isTextByExtension(tt.filename)
			if result != tt.expected {
				t.Errorf("Expected %v for %s, got %v", tt.expected, tt.filename, result)
			}
		})
	}
}

func TestContentTypeDetection(t *testing.T) {
	tests := []struct {
		contentType string
		expected    bool
	}{
		{"text/plain", true},
		{"text/html", true},
		{"text/css", true},
		{"application/json", true},
		{"application/xml", true},
		{"application/javascript", true},
		{"text/plain; charset=utf-8", true},
		{"image/jpeg", false},
		{"image/png", false},
		{"application/pdf", false},
		{"video/mp4", false},
		{"application/octet-stream", false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			result := isTextContentType(tt.contentType)
			if result != tt.expected {
				t.Errorf("Expected %v for %s, got %v", tt.expected, tt.contentType, result)
			}
		})
	}
}

func TestBinaryHeuristic(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{"plain text", []byte("This is plain text content."), true},
		{"text with newlines", []byte("Line 1\nLine 2\nLine 3"), true},
		{"text with tabs", []byte("Column1\tColumn2\tColumn3"), true},
		{"binary data", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, false},
		{"mostly text with some binary", []byte("Text content with some binary \x00\x01"), false},
		{"empty data", []byte{}, true},
		{"JSON content", []byte(`{"key": "value", "number": 123}`), true},
		{"XML content", []byte(`<?xml version="1.0"?><root><item>test</item></root>`), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLikelyTextContent(tt.data)
			if result != tt.expected {
				t.Errorf("Expected %v for %s, got %v", tt.expected, tt.name, result)
			}
		})
	}
}

func TestProcessFile_UnsupportedFileType(t *testing.T) {
	// Create temporary binary file
	tempDir := os.TempDir()
	binaryFile := filepath.Join(tempDir, "test.bin")
	binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
	err := os.WriteFile(binaryFile, binaryData, 0644)
	if err != nil {
		t.Fatalf("Failed to create test binary file: %v", err)
	}
	defer os.Remove(binaryFile)

	// Create embedding service
	chunker := NewSimpleTextChunker()
	model := NewMockEmbeddingModel(128)
	service := NewEmbeddingService(chunker, model, nil, nil, DefaultChunkingConfig())

	// Try to process binary file
	_, err = service.ProcessFile(binaryFile)
	if err == nil {
		t.Error("Expected error when processing binary file")
	}

	expectedError := "unsupported file type"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got: %v", expectedError, err)
	}
}

func TestProcessFile_TextFile(t *testing.T) {
	// Create temporary database
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "test_file_processing.db")
	defer os.Remove(dbPath)

	db, err := NewSQLiteDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create test text file
	textFile := filepath.Join(tempDir, "test.txt")
	textContent := "This is a test document.\nIt has multiple lines.\nAnd should be processed correctly."
	err = os.WriteFile(textFile, []byte(textContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test text file: %v", err)
	}
	defer os.Remove(textFile)

	// Create components
	chunker := NewSimpleTextChunker()
	model := NewMockEmbeddingModel(128)
	model.Initialize()
	vectorStore := NewSQLiteVectorStore(db)
	documentStore := NewSQLiteDocumentStore(db)
	config := DefaultChunkingConfig()

	service := NewEmbeddingService(chunker, model, vectorStore, documentStore, config)
	service.Initialize()

	// Process the text file
	doc, err := service.ProcessFile(textFile)
	if err != nil {
		t.Errorf("Failed to process text file: %v", err)
	}

	if doc == nil {
		t.Error("Expected document to be created")
	} else {
		if doc.FilePath != textFile {
			t.Errorf("Expected file path %s, got %s", textFile, doc.FilePath)
		}
		if doc.FileHash == "" {
			t.Errorf("Expected file hash to be calculated")
		}
		if doc.ChunkCount == 0 {
			t.Error("Expected chunks to be created")
		}
		
		// Check metadata
		if doc.Metadata["content_type"] == nil {
			t.Error("Expected content_type in metadata")
		}
		if doc.Metadata["file_ext"] != ".txt" {
			t.Errorf("Expected file_ext .txt, got %v", doc.Metadata["file_ext"])
		}
	}
}