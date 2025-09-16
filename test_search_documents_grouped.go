package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"arcadia/services"
)

// TestSearchDocumentsGrouped performs comprehensive testing of the new SearchDocuments functionality
func main() {
	log.Println("=== Testing SearchDocuments Grouped Functionality ===")

	// Create temporary database
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "test_search_docs_grouped.db")
	defer os.Remove(dbPath)

	db, err := services.NewSQLiteDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create components
	chunker := services.NewSimpleTextChunker()
	model := services.NewMockEmbeddingModel(128)
	err = model.Initialize()
	if err != nil {
		log.Fatalf("Failed to initialize model: %v", err)
	}
	defer model.Close()

	vectorStore := services.NewSQLiteVectorStore(db)
	documentStore := services.NewSQLiteDocumentStore(db)
	config := services.ChunkingConfig{
		Strategy:     services.ChunkingStrategyFixed,
		MaxChunkSize: 200,
		ChunkOverlap: 50,
	}

	// Create embedding service
	embeddingService := services.NewEmbeddingService(chunker, model, vectorStore, documentStore, config)
	err = embeddingService.Initialize()
	if err != nil {
		log.Fatalf("Failed to initialize embedding service: %v", err)
	}
	defer embeddingService.Close()

	// Test Case 1: Create test documents with varying content
	log.Println("\n=== Test Case 1: Creating Test Documents ===")
	testDocs := []struct {
		name    string
		content string
	}{
		{
			"ai_ml_guide.md",
			strings.Repeat("Artificial Intelligence and Machine Learning are transformative technologies. AI involves creating systems that can perform tasks typically requiring human intelligence. Machine learning is a subset of AI that enables systems to learn from data without explicit programming. Deep learning uses neural networks to model complex patterns. ", 5),
		},
		{
			"web_development.md",
			strings.Repeat("Web development involves creating websites and web applications using various technologies. Frontend development focuses on user interfaces using HTML, CSS, and JavaScript. Backend development handles server-side logic and database management. Full-stack developers work on both frontend and backend systems. ", 4),
		},
		{
			"data_science.md",
			strings.Repeat("Data science combines statistics, programming, and domain expertise to extract insights from data. Data scientists use Python and R for analysis. Machine learning algorithms help predict future trends. Data visualization helps communicate findings effectively. Big data technologies handle large datasets efficiently. ", 6),
		},
	}

	var createdDocs []*services.Document
	for _, testDoc := range testDocs {
		// Create temporary file
		testFile := filepath.Join(tempDir, testDoc.name)
		err = os.WriteFile(testFile, []byte(testDoc.content), 0644)
		if err != nil {
			log.Fatalf("Failed to create test file %s: %v", testDoc.name, err)
		}
		defer os.Remove(testFile)

		// Process file
		doc, err := embeddingService.ProcessFile(testFile)
		if err != nil {
			log.Fatalf("Failed to process file %s: %v", testDoc.name, err)
		}
		log.Printf("Created document %s with %d chunks", testDoc.name, doc.ChunkCount)
		createdDocs = append(createdDocs, doc)
	}

	// Test Case 2: Test SearchDocuments functionality
	log.Println("\n=== Test Case 2: Testing SearchDocuments Method ===")

	// Test with different queries
	testQueries := []struct {
		query string
		topK  int
	}{
		{"artificial intelligence machine learning", 2},
		{"web development frontend backend", 3},
		{"data science python statistics", 1},
		{"programming technologies", 3},
	}

	for _, query := range testQueries {
		log.Printf("\nTesting query: '%s' (topK=%d)", query.query, query.topK)

		start := time.Now()
		results, err := embeddingService.SearchDocuments(query.query, query.topK)
		duration := time.Since(start)

		if err != nil {
			log.Printf("ERROR: SearchDocuments failed: %v", err)
			continue
		}

		log.Printf("Found %d documents in %v", len(results), duration)

		for i, result := range results {
			log.Printf("  Document %d:", i+1)
			log.Printf("    File: %s", result.Document.FilePath)
			log.Printf("    Relevance Rank: %d", result.RelevanceRank)
			log.Printf("    Best Score: %.4f", result.BestScore)
			log.Printf("    Total Chunks: %d", result.TotalChunks)
			log.Printf("    Matching Chunks: %d", len(result.Chunks))
			log.Printf("    Content Length: %d characters", len(result.Document.Content))

			// Verify chunks are sorted by score
			for j := 1; j < len(result.Chunks); j++ {
				if result.Chunks[j].Score > result.Chunks[j-1].Score {
					log.Printf("WARNING: Chunks not sorted by score in document %d", i+1)
				}
			}
		}

		// Verify results are sorted by best score
		for i := 1; i < len(results); i++ {
			if results[i].BestScore > results[i-1].BestScore {
				log.Printf("WARNING: Results not sorted by BestScore")
			}
		}
	}

	// Test Case 3: Compare SearchDocuments vs SearchSimilar
	log.Println("\n=== Test Case 3: Comparing SearchDocuments vs SearchSimilar ===")

	testQuery := "machine learning artificial intelligence"

	start := time.Now()
	similarResults, err := embeddingService.SearchSimilar(testQuery, 3)
	similarDuration := time.Since(start)

	if err != nil {
		log.Printf("ERROR: SearchSimilar failed: %v", err)
	} else {
		log.Printf("SearchSimilar found %d results in %v", len(similarResults), similarDuration)
	}

	start = time.Now()
	docResults, err := embeddingService.SearchDocuments(testQuery, 3)
	docDuration := time.Since(start)

	if err != nil {
		log.Printf("ERROR: SearchDocuments failed: %v", err)
	} else {
		log.Printf("SearchDocuments found %d results in %v", len(docResults), docDuration)

		// Calculate total content returned
		totalContentLength := 0
		totalChunks := 0
		for _, result := range docResults {
			totalContentLength += len(result.Document.Content)
			totalChunks += len(result.Chunks)
		}
		log.Printf("SearchDocuments returned %d total characters of content across %d chunks",
			totalContentLength, totalChunks)
	}

	// Test Case 4: Test Claude Service Integration
	log.Println("\n=== Test Case 4: Testing Claude Service Integration ===")

	// Set up Claude service with embedding search
	services.SetEmbeddingSearch(embeddingService)

	config := services.ClaudeConfig{
		APIKey:         "test-key",
		BaseURL:        "http://test.api",
		Model:          "test-model",
		MaxTokens:      1000,
		TimeoutSeconds: 30,
		EnableMCP:      true,
	}

	claudeService := services.NewClaudeService(config)

	// Test the executeSearchDocumentsGrouped method directly
	log.Printf("Testing executeSearchDocumentsGrouped...")

	searchInput := map[string]interface{}{
		"query": "artificial intelligence machine learning",
		"top_k": 2,
	}

	start = time.Now()
	result, err := claudeService.ExecuteMCPToolDirect("search_documents_grouped", searchInput)
	duration = time.Since(start)

	if err != nil {
		log.Printf("ERROR: executeSearchDocumentsGrouped failed: %v", err)
	} else {
		log.Printf("executeSearchDocumentsGrouped completed in %v", duration)
		log.Printf("Result length: %d characters", len(result))

		// Parse and validate the JSON response
		var response map[string]interface{}
		err = json.Unmarshal([]byte(result), &response)
		if err != nil {
			log.Printf("ERROR: Failed to parse JSON response: %v", err)
		} else {
			log.Printf("Response structure validation:")
			if query, ok := response["query"].(string); ok {
				log.Printf("  ✓ Query: %s", query)
			} else {
				log.Printf("  ✗ Missing or invalid query field")
			}

			if docs, ok := response["documents"].([]interface{}); ok {
				log.Printf("  ✓ Documents: %d found", len(docs))

				// Validate first document structure
				if len(docs) > 0 {
					if doc, ok := docs[0].(map[string]interface{}); ok {
						requiredFields := []string{"document_id", "file_path", "full_content", "matching_chunks", "best_score", "relevance_rank"}
						for _, field := range requiredFields {
							if _, exists := doc[field]; exists {
								log.Printf("    ✓ Field '%s' present", field)
							} else {
								log.Printf("    ✗ Field '%s' missing", field)
							}
						}

						// Check matching_chunks structure
						if chunks, ok := doc["matching_chunks"].([]interface{}); ok {
							log.Printf("    ✓ Matching chunks: %d", len(chunks))
							if len(chunks) > 0 {
								if chunk, ok := chunks[0].(map[string]interface{}); ok {
									chunkFields := []string{"content", "score", "chunk_index"}
									for _, field := range chunkFields {
										if _, exists := chunk[field]; exists {
											log.Printf("      ✓ Chunk field '%s' present", field)
										} else {
											log.Printf("      ✗ Chunk field '%s' missing", field)
										}
									}
								}
							}
						}
					}
				}
			} else {
				log.Printf("  ✗ Missing or invalid documents field")
			}

			if usage, ok := response["usage_note"].(string); ok {
				log.Printf("  ✓ Usage note: %s", usage)
			}
		}
	}

	// Test Case 5: Performance and Rate Limiting Analysis
	log.Println("\n=== Test Case 5: Performance and Rate Limiting Analysis ===")

	// Test multiple consecutive calls to simulate rate limiting scenario
	log.Printf("Testing multiple consecutive calls...")
	totalTime := time.Duration(0)
	successCount := 0

	for i := 0; i < 5; i++ {
		start := time.Now()
		result, err := claudeService.ExecuteMCPToolDirect("search_documents_grouped", searchInput)
		duration := time.Since(start)
		totalTime += duration

		if err != nil {
			log.Printf("  Call %d failed: %v", i+1, err)
		} else {
			successCount++
			log.Printf("  Call %d: %v (%d chars)", i+1, duration, len(result))
		}
	}

	avgTime := totalTime / 5
	log.Printf("Performance summary: %d/5 calls successful, average time: %v", successCount, avgTime)

	// Test Case 6: Edge Cases
	log.Println("\n=== Test Case 6: Testing Edge Cases ===")

	edgeCases := []struct {
		name  string
		input map[string]interface{}
	}{
		{
			"Empty query",
			map[string]interface{}{"query": "", "top_k": 3},
		},
		{
			"Zero topK",
			map[string]interface{}{"query": "test", "top_k": 0},
		},
		{
			"Large topK",
			map[string]interface{}{"query": "test", "top_k": 100},
		},
		{
			"Non-existent terms",
			map[string]interface{}{"query": "zxcvbnm123456789", "top_k": 3},
		},
	}

	for _, testCase := range edgeCases {
		log.Printf("Testing: %s", testCase.name)
		result, err := claudeService.ExecuteMCPToolDirect("search_documents_grouped", testCase.input)
		if err != nil {
			log.Printf("  ERROR: %v", err)
		} else {
			var response map[string]interface{}
			json.Unmarshal([]byte(result), &response)
			if docs, ok := response["documents"].([]interface{}); ok {
				log.Printf("  ✓ Returned %d documents", len(docs))
			} else {
				log.Printf("  ✗ Invalid response format")
			}
		}
	}

	log.Println("\n=== SearchDocuments Grouped Testing Complete ===")
}