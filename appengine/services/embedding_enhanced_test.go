package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSearchConfig tests the SearchConfig structure and defaults
func TestSearchConfig(t *testing.T) {
	t.Run("default_config", func(t *testing.T) {
		config := DefaultSearchConfig()

		if config.MaxDocumentSize != 50*1024 {
			t.Errorf("Expected MaxDocumentSize 51200, got %d", config.MaxDocumentSize)
		}

		if config.MaxHighlights != 3 {
			t.Errorf("Expected MaxHighlights 3, got %d", config.MaxHighlights)
		}

		if !config.IncludeFullContent {
			t.Error("Expected IncludeFullContent true")
		}
	})

	t.Run("custom_config", func(t *testing.T) {
		config := SearchConfig{
			MaxDocumentSize:   10000,
			MaxHighlights:     5,
			IncludeFullContent: false,
		}

		if config.MaxDocumentSize != 10000 {
			t.Errorf("Expected MaxDocumentSize 10000, got %d", config.MaxDocumentSize)
		}

		if config.MaxHighlights != 5 {
			t.Errorf("Expected MaxHighlights 5, got %d", config.MaxHighlights)
		}

		if config.IncludeFullContent {
			t.Error("Expected IncludeFullContent false")
		}
	})
}

// TestEnhancedDocumentSearchResult tests the enhanced result structure
func TestEnhancedDocumentSearchResult(t *testing.T) {
	doc := &Document{
		ID:       "test-doc",
		FilePath: "/test/path.txt",
		Content:  "Test document content",
		Metadata: map[string]any{
			"author": "test",
		},
		ChunkCount: 2,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	result := &EnhancedDocumentSearchResult{
		Document:          doc,
		BestScore:         0.95,
		RelevanceRank:     1,
		ContextHighlights: []string{"highlight 1", "highlight 2"},
		ContentPreview:    "Test document...",
		IsTruncated:       false,
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal enhanced result: %v", err)
	}

	var unmarshaled EnhancedDocumentSearchResult
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal enhanced result: %v", err)
	}

	if unmarshaled.Document.ID != doc.ID {
		t.Errorf("Expected document ID %s, got %s", doc.ID, unmarshaled.Document.ID)
	}

	if unmarshaled.BestScore != result.BestScore {
		t.Errorf("Expected BestScore %f, got %f", result.BestScore, unmarshaled.BestScore)
	}

	if len(unmarshaled.ContextHighlights) != len(result.ContextHighlights) {
		t.Errorf("Expected %d context highlights, got %d",
			len(result.ContextHighlights), len(unmarshaled.ContextHighlights))
	}
}

// TestTruncateContent tests content truncation functionality
func TestTruncateContent(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		maxSize    int
		expectTruncated bool
	}{
		{
			name:       "no_truncation_needed",
			content:    "Short content",
			maxSize:    100,
			expectTruncated: false,
		},
		{
			name:       "exact_size",
			content:    "12345",
			maxSize:    5,
			expectTruncated: false,
		},
		{
			name:       "truncation_needed",
			content:    "This is a long content that needs to be truncated",
			maxSize:    10,
			expectTruncated: true,
		},
		{
			name:       "empty_content",
			content:    "",
			maxSize:    100,
			expectTruncated: false,
		},
		{
			name:       "zero_max_size",
			content:    "content",
			maxSize:    0,
			expectTruncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, truncated := truncateContent(tt.content, tt.maxSize)

			if truncated != tt.expectTruncated {
				t.Errorf("Expected truncated=%v, got %v", tt.expectTruncated, truncated)
			}

			if len(result) > tt.maxSize {
				t.Errorf("Result length %d exceeds max size %d", len(result), tt.maxSize)
			}

			if !truncated && result != tt.content {
				t.Error("Non-truncated result should match original content")
			}
		})
	}
}

// TestExtractContentPreview tests content preview extraction
func TestExtractContentPreview(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		expectLen  int
		expectDots bool
	}{
		{
			name:       "short_content",
			content:    "Short content",
			expectLen:  13,
			expectDots: false,
		},
		{
			name:       "exactly_500_chars",
			content:    strings.Repeat("a", 500),
			expectLen:  500,
			expectDots: false,
		},
		{
			name:       "long_content",
			content:    strings.Repeat("a", 1000),
			expectLen:  503, // 500 + "..."
			expectDots: true,
		},
		{
			name:       "empty_content",
			content:    "",
			expectLen:  0,
			expectDots: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preview := extractContentPreview(tt.content)

			if len(preview) != tt.expectLen {
				t.Errorf("Expected preview length %d, got %d", tt.expectLen, len(preview))
			}

			if tt.expectDots && !strings.HasSuffix(preview, "...") {
				t.Error("Expected preview to end with '...'")
			}

			if !tt.expectDots && strings.HasSuffix(preview, "...") {
				t.Error("Did not expect preview to end with '...'")
			}
		})
	}
}

// TestExtractContextHighlights tests context highlight extraction
func TestExtractContextHighlights(t *testing.T) {
	chunks := []*ChunkResult{
		{Content: "First chunk with high score", Score: 0.9, ChunkIndex: 0},
		{Content: "Second chunk with medium score", Score: 0.7, ChunkIndex: 1},
		{Content: "Third chunk with low score", Score: 0.5, ChunkIndex: 2},
		{Content: "Fourth chunk with very low score", Score: 0.3, ChunkIndex: 3},
		{Content: "   ", Score: 0.1, ChunkIndex: 4}, // Empty content (whitespace)
	}

	t.Run("extract_top_3", func(t *testing.T) {
		highlights := extractContextHighlights(chunks, 3)

		if len(highlights) != 3 {
			t.Errorf("Expected 3 highlights, got %d", len(highlights))
		}

		// Should be sorted by score (highest first)
		expected := []string{
			"First chunk with high score",
			"Second chunk with medium score",
			"Third chunk with low score",
		}

		for i, expected := range expected {
			if i < len(highlights) && highlights[i] != expected {
				t.Errorf("Highlight %d: expected '%s', got '%s'", i, expected, highlights[i])
			}
		}
	})

	t.Run("extract_more_than_available", func(t *testing.T) {
		highlights := extractContextHighlights(chunks, 10)

		// Should not include empty/whitespace-only chunks
		if len(highlights) != 4 {
			t.Errorf("Expected 4 highlights (excluding empty), got %d", len(highlights))
		}
	})

	t.Run("extract_zero", func(t *testing.T) {
		highlights := extractContextHighlights(chunks, 0)

		if len(highlights) != 0 {
			t.Errorf("Expected 0 highlights, got %d", len(highlights))
		}
	})

	t.Run("empty_chunks", func(t *testing.T) {
		highlights := extractContextHighlights([]*ChunkResult{}, 3)

		if len(highlights) != 0 {
			t.Errorf("Expected 0 highlights for empty chunks, got %d", len(highlights))
		}
	})
}

// TestSearchDocumentsEnhanced_EdgeCases tests edge cases for enhanced search
func TestSearchDocumentsEnhanced_EdgeCases(t *testing.T) {
	// Create temporary database
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "test_enhanced_edge_cases.db")
	defer os.Remove(dbPath)

	db, err := NewSQLiteDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create service with mock components
	chunker := NewSimpleTextChunker()
	model := NewMockEmbeddingModel(128)
	model.Initialize()
	vectorStore := NewSQLiteVectorStore(db)
	documentStore := NewSQLiteDocumentStore(db)
	config := DefaultChunkingConfig()

	service := NewEmbeddingService(chunker, model, vectorStore, documentStore, config)
	err = service.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize service: %v", err)
	}
	defer service.Close()

	t.Run("negative_topK", func(t *testing.T) {
		config := DefaultSearchConfig()
		results, err := service.SearchDocumentsEnhanced("test", -1, config)
		if err != nil {
			t.Fatalf("SearchDocumentsEnhanced failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected 0 results for negative topK, got %d", len(results))
		}
	})

	t.Run("zero_max_highlights", func(t *testing.T) {
		// Create a test document first
		testContent := "This is test content for highlight testing."
		tmpFile, err := os.CreateTemp("", "test_highlights.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		tmpFile.WriteString(testContent)
		tmpFile.Close()

		_, err = service.ProcessFile(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to process file: %v", err)
		}

		config := SearchConfig{
			MaxDocumentSize:   50000,
			MaxHighlights:     0,
			IncludeFullContent: true,
		}

		results, err := service.SearchDocumentsEnhanced("test content", 1, config)
		if err != nil {
			t.Fatalf("SearchDocumentsEnhanced failed: %v", err)
		}

		if len(results) > 0 && len(results[0].ContextHighlights) != 0 {
			t.Errorf("Expected 0 highlights with MaxHighlights=0, got %d",
				len(results[0].ContextHighlights))
		}
	})

	t.Run("zero_max_document_size", func(t *testing.T) {
		searchConfig := SearchConfig{
			MaxDocumentSize:   0,
			MaxHighlights:     3,
			IncludeFullContent: true,
		}

		results, err := service.SearchDocumentsEnhanced("test content", 1, searchConfig)
		if err != nil {
			t.Fatalf("SearchDocumentsEnhanced failed: %v", err)
		}

		// Should handle zero size limit gracefully
		for _, result := range results {
			if len(result.Document.Content) > 0 {
				t.Errorf("Expected truncated content with zero size limit, got %d chars",
					len(result.Document.Content))
			}
			if !result.IsTruncated {
				t.Error("Expected IsTruncated=true with zero size limit")
			}
		}
	})
}

// TestClaudeServiceIntegration tests the integration with Claude service
func TestClaudeServiceIntegration_SearchDocuments(t *testing.T) {
	// This tests the Claude service's executeSearchDocuments method
	// We'll create a mock embedding service that implements the EmbeddingSearch interface

	mockService := &MockEmbeddingSearchService{}

	// Test the Claude service integration
	claudeService := &ClaudeService{}

	// Set the global embedding search service
	SetEmbeddingSearch(mockService)
	defer SetEmbeddingSearch(nil) // Cleanup

	t.Run("execute_search_documents_enhanced", func(t *testing.T) {
		input := map[string]any{
			"query": "test query",
			"top_k": 3,
		}

		result, err := claudeService.executeSearchDocuments(input)
		if err != nil {
			t.Fatalf("executeSearchDocuments failed: %v", err)
		}

		// Verify the result is valid JSON
		var response map[string]any
		err = json.Unmarshal([]byte(result), &response)
		if err != nil {
			t.Fatalf("Failed to parse JSON response: %v", err)
		}

		// Verify required fields
		if response["query"] != "test query" {
			t.Errorf("Expected query 'test query', got %v", response["query"])
		}

		if response["total_documents"] == nil {
			t.Error("Expected total_documents field")
		}

		if response["documents"] == nil {
			t.Error("Expected documents field")
		}

		if response["usage_note"] == nil {
			t.Error("Expected usage_note field")
		}
	})

	t.Run("execute_search_documents_missing_query", func(t *testing.T) {
		input := map[string]any{
			"top_k": 3,
		}

		_, err := claudeService.executeSearchDocuments(input)
		if err == nil {
			t.Error("Expected error for missing query")
		}

		if !strings.Contains(err.Error(), "query is required") {
			t.Errorf("Expected 'query is required' error, got: %v", err)
		}
	})

	t.Run("execute_search_documents_invalid_topk", func(t *testing.T) {
		input := map[string]any{
			"query": "test",
			"top_k": 15, // Above limit
		}

		_, err := claudeService.executeSearchDocuments(input)
		if err == nil {
			t.Error("Expected error for invalid top_k")
		}

		if !strings.Contains(err.Error(), "top_k must be between 1 and 10") {
			t.Errorf("Expected top_k validation error, got: %v", err)
		}
	})
}

// MockEmbeddingSearchService for testing Claude integration
type MockEmbeddingSearchService struct{}

func (m *MockEmbeddingSearchService) SearchDocuments(query string, topK int) ([]*DocumentSearchResult, error) {
	// Return mock results
	doc := &Document{
		ID:       "mock-doc-1",
		FilePath: "/mock/path.txt",
		Content:  "Mock document content for testing",
		Metadata: map[string]any{"type": "mock"},
		ChunkCount: 1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	chunk := &ChunkResult{
		Content:    "Mock chunk content",
		Score:      0.85,
		ChunkIndex: 0,
	}

	result := &DocumentSearchResult{
		Document:      doc,
		Chunks:        []*ChunkResult{chunk},
		BestScore:     0.85,
		TotalChunks:   1,
		RelevanceRank: 1,
	}

	return []*DocumentSearchResult{result}, nil
}

func (m *MockEmbeddingSearchService) SearchDocumentsEnhanced(query string, topK int, config SearchConfig) ([]*EnhancedDocumentSearchResult, error) {
	// Return mock enhanced results
	doc := &Document{
		ID:       "mock-doc-enhanced-1",
		FilePath: "/mock/enhanced/path.txt",
		Content:  "Mock enhanced document content for testing with more details",
		Metadata: map[string]any{"type": "mock_enhanced"},
		ChunkCount: 2,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	result := &EnhancedDocumentSearchResult{
		Document:          doc,
		BestScore:         0.92,
		RelevanceRank:     1,
		ContextHighlights: []string{"enhanced content", "testing details", "mock highlights"},
		ContentPreview:    "Mock enhanced document content for testing...",
		IsTruncated:       false,
	}

	return []*EnhancedDocumentSearchResult{result}, nil
}

func (m *MockEmbeddingSearchService) GetDocument(documentID string) (*Document, error) {
	return &Document{
		ID:       documentID,
		FilePath: fmt.Sprintf("/mock/%s.txt", documentID),
		Content:  fmt.Sprintf("Mock content for document %s", documentID),
		Metadata: map[string]any{"type": "mock"},
		ChunkCount: 1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

// TestPerformanceComparison compares performance between original and enhanced search
func TestPerformanceComparison(t *testing.T) {
	// Skip this test in short mode as it's performance-focused
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create temporary database
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "test_performance.db")
	defer os.Remove(dbPath)

	db, err := NewSQLiteDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create service
	chunker := NewSimpleTextChunker()
	model := NewMockEmbeddingModel(128)
	model.Initialize()
	vectorStore := NewSQLiteVectorStore(db)
	documentStore := NewSQLiteDocumentStore(db)
	config := DefaultChunkingConfig()

	service := NewEmbeddingService(chunker, model, vectorStore, documentStore, config)
	err = service.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize service: %v", err)
	}
	defer service.Close()

	// Create test documents
	for i := 0; i < 5; i++ {
		content := fmt.Sprintf("Document %d with some test content that should be searchable. %s",
			i, strings.Repeat("Additional content for testing. ", 20))

		tmpFile, err := os.CreateTemp("", fmt.Sprintf("perf_test_%d.txt", i))
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		tmpFile.WriteString(content)
		tmpFile.Close()

		_, err = service.ProcessFile(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to process file: %v", err)
		}
	}

	query := "test content searchable"
	topK := 3
	iterations := 10

	// Benchmark original SearchDocuments
	originalStart := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := service.SearchDocuments(query, topK)
		if err != nil {
			t.Fatalf("Original search failed: %v", err)
		}
	}
	originalDuration := time.Since(originalStart)

	// Benchmark enhanced SearchDocumentsEnhanced
	enhancedStart := time.Now()
	searchConfig := DefaultSearchConfig()
	for i := 0; i < iterations; i++ {
		_, err := service.SearchDocumentsEnhanced(query, topK, searchConfig)
		if err != nil {
			t.Fatalf("Enhanced search failed: %v", err)
		}
	}
	enhancedDuration := time.Since(enhancedStart)

	t.Logf("Original search: %v (%v per operation)", originalDuration, originalDuration/time.Duration(iterations))
	t.Logf("Enhanced search: %v (%v per operation)", enhancedDuration, enhancedDuration/time.Duration(iterations))

	// The enhanced version should not be significantly slower (allow 2x overhead)
	if enhancedDuration > originalDuration*2 {
		t.Logf("Warning: Enhanced search is significantly slower (%v vs %v)",
			enhancedDuration, originalDuration)
	}
}