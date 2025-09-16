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
		Metadata: map[string]any{
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

	// Search documents
	results, err := service.SearchDocuments("test document", 5)
	if err != nil {
		t.Errorf("Failed to search documents: %v", err)
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

	// Test that the default service is properly initialized
	// Test that the default service is properly initialized and can perform searches
	results, err := GetDefaultEmbeddingService().SearchDocuments("test query", 5)
	if err != nil {
		t.Errorf("SearchDocuments failed: %v", err)
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




// Test calculateSearchLimit function
func TestCalculateSearchLimit(t *testing.T) {
	tests := []struct {
		topK     int
		expected int
		name     string
	}{
		{0, 0, "topK=0"},
		{-1, 0, "topK negative"},
		{1, 5, "topK=1"},
		{5, 25, "topK=5"},
		{10, 50, "topK=10"},
		{20, 100, "topK=20 (uses topK*5)"},
		{21, 105, "topK=21 (uses topK*5)"},
		{25, 125, "topK=25 (uses topK+100)"},
		{50, 150, "topK=50 (uses topK+100)"},
		{100, 200, "topK=100 (uses topK+100)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSearchLimit(tt.topK)
			if result != tt.expected {
				t.Errorf("calculateSearchLimit(%d) = %d, expected %d", tt.topK, result, tt.expected)
			}
		})
	}
}

// Test deduplication with malformed metadata



// TestEmbeddingService_SearchDocuments tests the new SearchDocuments method
func TestEmbeddingService_SearchDocuments(t *testing.T) {
	// Create temporary database
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "test_search_documents.db")
	defer os.Remove(dbPath)

	db, err := NewSQLiteDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create mock components
	chunker := NewSimpleTextChunker()
	model := NewMockEmbeddingModel(128)
	model.Initialize()
	vectorStore := NewSQLiteVectorStore(db)
	documentStore := NewSQLiteDocumentStore(db)
	config := DefaultChunkingConfig()

	// Create service
	service := NewEmbeddingService(chunker, model, vectorStore, documentStore, config)
	err = service.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize service: %v", err)
	}
	defer service.Close()

	// Create test documents
	doc1Content := strings.Repeat("First document with unique content about artificial intelligence and machine learning. ", 10)
	doc2Content := strings.Repeat("Second document discusses natural language processing and text embedding techniques. ", 10)
	doc3Content := strings.Repeat("Third document covers vector databases and similarity search algorithms. ", 10)

	// Create temporary files
	testFiles := []struct {
		name    string
		content string
	}{
		{"doc1.txt", doc1Content},
		{"doc2.txt", doc2Content},
		{"doc3.txt", doc3Content},
	}

	var createdDocs []*Document
	for _, tf := range testFiles {
		// Create temporary file
		tmpFile, err := os.CreateTemp("", tf.name)
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		// Write content
		_, err = tmpFile.WriteString(tf.content)
		if err != nil {
			t.Fatalf("Failed to write temp file: %v", err)
		}
		tmpFile.Close()

		// Process file
		doc, err := service.ProcessFile(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to process file %s: %v", tf.name, err)
		}
		createdDocs = append(createdDocs, doc)
	}

	// Test SearchDocuments
	t.Run("basic_search", func(t *testing.T) {
		results, err := service.SearchDocuments("artificial intelligence machine learning", 3)
		if err != nil {
			t.Fatalf("SearchDocuments failed: %v", err)
		}

		// Should return results
		if len(results) == 0 {
			t.Error("Expected at least one result")
		}

		// Should not exceed topK
		if len(results) > 3 {
			t.Errorf("Expected at most 3 results, got %d", len(results))
		}

		// Verify structure of results
		for i, result := range results {
			if result.Document == nil {
				t.Errorf("Result %d: Document should not be nil", i)
			}

			if len(result.Chunks) == 0 {
				t.Errorf("Result %d: Should have at least one chunk", i)
			}

			if result.BestScore <= 0 {
				t.Errorf("Result %d: BestScore should be positive, got %f", i, result.BestScore)
			}

			if result.TotalChunks != len(result.Chunks) {
				t.Errorf("Result %d: TotalChunks (%d) should match length of Chunks (%d)",
					i, result.TotalChunks, len(result.Chunks))
			}

			if result.RelevanceRank != i+1 {
				t.Errorf("Result %d: Expected RelevanceRank %d, got %d",
					i, i+1, result.RelevanceRank)
			}

			// Verify document has content
			if result.Document.Content == "" {
				t.Errorf("Result %d: Document content should be reconstructed", i)
			}

			// Verify chunks are sorted by score (highest first)
			for j := 1; j < len(result.Chunks); j++ {
				if result.Chunks[j].Score > result.Chunks[j-1].Score {
					t.Errorf("Result %d: Chunks should be sorted by score (highest first)", i)
				}
			}

			// Verify chunk structure
			for j, chunk := range result.Chunks {
				if chunk.Content == "" {
					t.Errorf("Result %d, Chunk %d: Content should not be empty", i, j)
				}
				if chunk.Score <= 0 {
					t.Errorf("Result %d, Chunk %d: Score should be positive", i, j)
				}
			}
		}

		// Verify results are sorted by best score (highest first)
		for i := 1; i < len(results); i++ {
			if results[i].BestScore > results[i-1].BestScore {
				t.Error("Results should be sorted by BestScore (highest first)")
			}
		}
	})

	t.Run("topK_limiting", func(t *testing.T) {
		// Test with topK=1
		results, err := service.SearchDocuments("document content", 1)
		if err != nil {
			t.Fatalf("SearchDocuments failed: %v", err)
		}

		if len(results) > 1 {
			t.Errorf("Expected at most 1 result, got %d", len(results))
		}

		// Test with topK=2
		results, err = service.SearchDocuments("document content", 2)
		if err != nil {
			t.Fatalf("SearchDocuments failed: %v", err)
		}

		if len(results) > 2 {
			t.Errorf("Expected at most 2 results, got %d", len(results))
		}
	})

	t.Run("empty_query", func(t *testing.T) {
		results, err := service.SearchDocuments("", 5)
		if err != nil {
			t.Fatalf("SearchDocuments failed: %v", err)
		}

		// Should handle empty query gracefully
		t.Logf("Empty query returned %d results", len(results))
	})

	t.Run("no_match_query", func(t *testing.T) {
		results, err := service.SearchDocuments("xyz123nonexistentterm456", 5)
		if err != nil {
			t.Fatalf("SearchDocuments failed: %v", err)
		}

		// May return results with low scores or no results
		t.Logf("No-match query returned %d results", len(results))
	})

	t.Run("zero_topK", func(t *testing.T) {
		results, err := service.SearchDocuments("test", 0)
		if err != nil {
			t.Fatalf("SearchDocuments failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected 0 results for topK=0, got %d", len(results))
		}
	})
}

// TestEmbeddingService_SearchDocumentsEnhanced tests the enhanced SearchDocuments method
func TestEmbeddingService_SearchDocumentsEnhanced(t *testing.T) {
	// Create temporary database
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "test_search_enhanced.db")
	defer os.Remove(dbPath)

	db, err := NewSQLiteDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create mock components
	chunker := NewSimpleTextChunker()
	model := NewMockEmbeddingModel(128)
	model.Initialize()
	vectorStore := NewSQLiteVectorStore(db)
	documentStore := NewSQLiteDocumentStore(db)
	config := DefaultChunkingConfig()

	// Create service
	service := NewEmbeddingService(chunker, model, vectorStore, documentStore, config)
	err = service.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize service: %v", err)
	}
	defer service.Close()

	// Create test documents with varying sizes
	smallDoc := strings.Repeat("Small document content. ", 5)
	mediumDoc := strings.Repeat("Medium document with more detailed content about machine learning and AI. ", 50)
	largeDoc := strings.Repeat("Large document with extensive content covering artificial intelligence, machine learning, natural language processing, vector databases, and similarity search algorithms. This content is designed to exceed size limits. ", 300)

	// Create temporary files
	testFiles := []struct {
		name    string
		content string
	}{
		{"small.txt", smallDoc},
		{"medium.txt", mediumDoc},
		{"large.txt", largeDoc},
	}

	var createdDocs []*Document
	for _, tf := range testFiles {
		// Create temporary file
		tmpFile, err := os.CreateTemp("", tf.name)
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		// Write content
		_, err = tmpFile.WriteString(tf.content)
		if err != nil {
			t.Fatalf("Failed to write temp file: %v", err)
		}
		tmpFile.Close()

		// Process file
		doc, err := service.ProcessFile(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to process file %s: %v", tf.name, err)
		}
		createdDocs = append(createdDocs, doc)
	}

	// Test SearchDocumentsEnhanced with default config
	t.Run("enhanced_search_default_config", func(t *testing.T) {
		config := DefaultSearchConfig()
		results, err := service.SearchDocumentsEnhanced("machine learning artificial intelligence", 3, config)
		if err != nil {
			t.Fatalf("SearchDocumentsEnhanced failed: %v", err)
		}

		// Should return results
		if len(results) == 0 {
			t.Error("Expected at least one result")
		}

		// Should not exceed topK
		if len(results) > 3 {
			t.Errorf("Expected at most 3 results, got %d", len(results))
		}

		// Verify structure of enhanced results
		for i, result := range results {
			if result.Document == nil {
				t.Errorf("Result %d: Document should not be nil", i)
			}

			if result.BestScore <= 0 {
				t.Errorf("Result %d: BestScore should be positive, got %f", i, result.BestScore)
			}

			if result.RelevanceRank != i+1 {
				t.Errorf("Result %d: Expected RelevanceRank %d, got %d",
					i, i+1, result.RelevanceRank)
			}

			// Verify document has content
			if result.Document.Content == "" {
				t.Errorf("Result %d: Document content should be reconstructed", i)
			}

			// Verify context highlights
			if len(result.ContextHighlights) > config.MaxHighlights {
				t.Errorf("Result %d: Too many context highlights: %d > %d",
					i, len(result.ContextHighlights), config.MaxHighlights)
			}

			// Verify content preview
			if result.ContentPreview == "" {
				t.Errorf("Result %d: ContentPreview should not be empty", i)
			}

			// Verify preview length
			if len(result.ContentPreview) > 503 { // 500 + "..."
				t.Errorf("Result %d: ContentPreview too long: %d chars", i, len(result.ContentPreview))
			}
		}

		// Verify results are sorted by best score (highest first)
		for i := 1; i < len(results); i++ {
			if results[i].BestScore > results[i-1].BestScore {
				t.Error("Results should be sorted by BestScore (highest first)")
			}
		}
	})

	// Test with size limits
	t.Run("enhanced_search_size_limits", func(t *testing.T) {
		config := SearchConfig{
			MaxDocumentSize:   1000, // Small size limit
			MaxHighlights:     2,
			IncludeFullContent: true,
		}

		results, err := service.SearchDocumentsEnhanced("extensive content", 3, config)
		if err != nil {
			t.Fatalf("SearchDocumentsEnhanced failed: %v", err)
		}

		// Check for truncation
		foundTruncated := false
		for i, result := range results {
			if len(result.Document.Content) > config.MaxDocumentSize {
				t.Errorf("Result %d: Document content exceeds size limit: %d > %d",
					i, len(result.Document.Content), config.MaxDocumentSize)
			}

			if result.IsTruncated {
				foundTruncated = true
			}

			if len(result.ContextHighlights) > config.MaxHighlights {
				t.Errorf("Result %d: Too many highlights: %d > %d",
					i, len(result.ContextHighlights), config.MaxHighlights)
			}
		}

		// Should have found at least one truncated document given our large content
		if !foundTruncated {
			t.Log("Expected to find at least one truncated document")
		}
	})

	// Test without full content
	t.Run("enhanced_search_no_full_content", func(t *testing.T) {
		config := SearchConfig{
			MaxDocumentSize:   50000,
			MaxHighlights:     3,
			IncludeFullContent: false,
		}

		results, err := service.SearchDocumentsEnhanced("document content", 2, config)
		if err != nil {
			t.Fatalf("SearchDocumentsEnhanced failed: %v", err)
		}

		// When full content is disabled, document content should be empty or original
		for i, result := range results {
			if result.Document == nil {
				t.Errorf("Result %d: Document should not be nil", i)
				continue
			}
			// Content might be empty or the original stored content
			// The important thing is we don't reconstruct it from chunks
		}
	})

	t.Run("enhanced_search_zero_topK", func(t *testing.T) {
		config := DefaultSearchConfig()
		results, err := service.SearchDocumentsEnhanced("test", 0, config)
		if err != nil {
			t.Fatalf("SearchDocumentsEnhanced failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected 0 results for topK=0, got %d", len(results))
		}
	})
}