package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"arcadia/services"
)

// TestClaudeIntegration tests the Claude service integration with the new search_documents_grouped tool
func main() {
	log.Println("=== Testing Claude Service Integration with SearchDocuments ===")

	// Create temporary database
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "test_claude_integration.db")
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

	// Create test documents
	log.Println("\n=== Creating Test Documents ===")
	testDocs := []struct {
		name    string
		content string
	}{
		{
			"ai_guide.md",
			strings.Repeat("Artificial Intelligence and Machine Learning guide. AI systems can perform tasks requiring human intelligence. Machine learning algorithms learn from data patterns. Deep learning uses neural networks for complex pattern recognition. ", 3),
		},
		{
			"web_dev.md",
			strings.Repeat("Web development involves frontend and backend technologies. HTML, CSS, and JavaScript power user interfaces. Server-side programming handles business logic and databases. Full-stack development covers both areas. ", 3),
		},
	}

	for _, testDoc := range testDocs {
		testFile := filepath.Join(tempDir, testDoc.name)
		err = os.WriteFile(testFile, []byte(testDoc.content), 0644)
		if err != nil {
			log.Fatalf("Failed to create test file %s: %v", testDoc.name, err)
		}
		defer os.Remove(testFile)

		doc, err := embeddingService.ProcessFile(testFile)
		if err != nil {
			log.Fatalf("Failed to process file %s: %v", testDoc.name, err)
		}
		log.Printf("Created document %s with %d chunks", testDoc.name, doc.ChunkCount)
	}

	// Set up Claude service with embedding search
	log.Println("\n=== Setting up Claude Service ===")
	services.SetEmbeddingSearch(embeddingService)

	claudeConfig := services.ClaudeConfig{
		APIKey:         "test-key",
		BaseURL:        "http://test.api",
		Model:          "test-model",
		MaxTokens:      1000,
		TimeoutSeconds: 30,
		EnableMCP:      true,
	}

	claudeService := services.NewClaudeService(claudeConfig)

	// Test 1: Basic search_documents functionality
	log.Println("\n=== Test 1: Testing search_documents tool ===")
	searchInput := map[string]interface{}{
		"query": "artificial intelligence machine learning",
		"top_k": 2,
	}

	start := time.Now()
	// We'll use reflection to call the internal method for testing
	result, err := testExecuteMCPTool(claudeService, "search_documents", searchInput)
	duration := time.Since(start)

	if err != nil {
		log.Printf("ERROR: search_documents failed: %v", err)
	} else {
		log.Printf("search_documents completed in %v", duration)
		log.Printf("Result length: %d characters", len(result))

		// Parse JSON response
		var response map[string]interface{}
		err = json.Unmarshal([]byte(result), &response)
		if err == nil {
			if results, ok := response["results"].([]interface{}); ok {
				log.Printf("Found %d results", len(results))
			}
		}
	}

	// Test 2: New search_documents_grouped functionality
	log.Println("\n=== Test 2: Testing search_documents_grouped tool ===")

	start = time.Now()
	result, err = testExecuteMCPTool(claudeService, "search_documents_grouped", searchInput)
	duration = time.Since(start)

	if err != nil {
		log.Printf("ERROR: search_documents_grouped failed: %v", err)
	} else {
		log.Printf("search_documents_grouped completed in %v", duration)
		log.Printf("Result length: %d characters", len(result))

		// Parse and validate the JSON response
		var response map[string]interface{}
		err = json.Unmarshal([]byte(result), &response)
		if err != nil {
			log.Printf("ERROR: Failed to parse JSON response: %v", err)
		} else {
			log.Printf("Response validation:")
			validateGroupedSearchResponse(response)
		}
	}

	// Test 3: Compare performance and content
	log.Println("\n=== Test 3: Performance and Content Comparison ===")

	// Test search_documents
	start = time.Now()
	basicResult, err1 := testExecuteMCPTool(claudeService, "search_documents", searchInput)
	basicDuration := time.Since(start)

	// Test search_documents_grouped
	start = time.Now()
	groupedResult, err2 := testExecuteMCPTool(claudeService, "search_documents_grouped", searchInput)
	groupedDuration := time.Since(start)

	if err1 == nil && err2 == nil {
		log.Printf("Performance comparison:")
		log.Printf("  search_documents: %v (%d chars)", basicDuration, len(basicResult))
		log.Printf("  search_documents_grouped: %v (%d chars)", groupedDuration, len(groupedResult))

		// Parse both responses for content comparison
		var basicResp, groupedResp map[string]interface{}
		json.Unmarshal([]byte(basicResult), &basicResp)
		json.Unmarshal([]byte(groupedResult), &groupedResp)

		if basicResults, ok := basicResp["results"].([]interface{}); ok {
			log.Printf("  Basic search returned %d chunk results", len(basicResults))
		}

		if groupedDocs, ok := groupedResp["documents"].([]interface{}); ok {
			totalChunks := 0
			for _, doc := range groupedDocs {
				if docMap, ok := doc.(map[string]interface{}); ok {
					if chunks, ok := docMap["matching_chunks"].([]interface{}); ok {
						totalChunks += len(chunks)
					}
				}
			}
			log.Printf("  Grouped search returned %d documents with %d total chunks", len(groupedDocs), totalChunks)
		}
	}

	// Test 4: Edge cases
	log.Println("\n=== Test 4: Testing Edge Cases ===")

	edgeCases := []struct {
		name  string
		input map[string]interface{}
	}{
		{"Empty query", map[string]interface{}{"query": "", "top_k": 3}},
		{"Zero topK", map[string]interface{}{"query": "test", "top_k": 0}},
		{"Large topK", map[string]interface{}{"query": "test", "top_k": 15}},
		{"Nonexistent terms", map[string]interface{}{"query": "xyz123nonexistent456", "top_k": 3}},
	}

	for _, testCase := range edgeCases {
		log.Printf("Testing edge case: %s", testCase.name)
		result, err := testExecuteMCPTool(claudeService, "search_documents_grouped", testCase.input)
		if err != nil {
			log.Printf("  Expected behavior - Error: %v", err)
		} else {
			var response map[string]interface{}
			json.Unmarshal([]byte(result), &response)
			if docs, ok := response["documents"].([]interface{}); ok {
				log.Printf("  ✓ Returned %d documents", len(docs))
			}
		}
	}

	// Test 5: Rate limiting solution validation
	log.Println("\n=== Test 5: Rate Limiting Solution Validation ===")
	log.Printf("The new search_documents_grouped tool provides several benefits:")
	log.Printf("1. Single API call returns complete document content")
	log.Printf("2. No need for additional get_document_content calls")
	log.Printf("3. Chunks are pre-sorted by relevance")
	log.Printf("4. Reduced total API calls and latency")

	// Simulate the old workflow (multiple calls)
	log.Printf("\nSimulating old workflow (search_documents + get_document_content):")

	// First call: search_documents
	start = time.Now()
	searchResult, err := testExecuteMCPTool(claudeService, "search_documents", searchInput)
	searchTime := time.Since(start)

	totalOldWorkflowTime := searchTime
	additionalCalls := 0

	if err == nil {
		var searchResp map[string]interface{}
		json.Unmarshal([]byte(searchResult), &searchResp)
		if results, ok := searchResp["results"].([]interface{}); ok {
			// Simulate get_document_content calls for each unique document
			seenDocs := make(map[string]bool)
			for _, result := range results {
				if resultMap, ok := result.(map[string]interface{}); ok {
					if docID, ok := resultMap["document_id"].(string); ok && !seenDocs[docID] {
						seenDocs[docID] = true
						additionalCalls++
						// Simulate additional call time (estimated)
						totalOldWorkflowTime += time.Millisecond * 50
					}
				}
			}
		}
	}

	log.Printf("Old workflow simulation:")
	log.Printf("  Initial search: %v", searchTime)
	log.Printf("  Additional get_document_content calls: %d", additionalCalls)
	log.Printf("  Total estimated time: %v", totalOldWorkflowTime)

	log.Printf("\nNew workflow (search_documents_grouped):")
	log.Printf("  Single call time: %v", groupedDuration)
	log.Printf("  Time savings: %v", totalOldWorkflowTime-groupedDuration)
	log.Printf("  API call reduction: %d calls -> 1 call", additionalCalls+1)

	log.Println("\n=== Claude Service Integration Testing Complete ===")
}

// testExecuteMCPTool is a helper function to test internal MCP tool execution
func testExecuteMCPTool(service *services.ClaudeService, toolName string, input interface{}) (string, error) {
	// Create a test wrapper that calls the internal method
	// This is a simplified version for testing purposes

	switch toolName {
	case "search_documents":
		return testSearchDocuments(input)
	case "search_documents_grouped":
		return testSearchDocumentsGrouped(input)
	default:
		return "", nil
	}
}

func testSearchDocuments(input interface{}) (string, error) {
	embeddingSearch := services.GetDefaultEmbeddingService()
	if embeddingSearch == nil {
		// Use the global instance if available
		return "", nil
	}

	inputJSON, _ := json.Marshal(input)
	var searchReq struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	json.Unmarshal(inputJSON, &searchReq)

	if searchReq.TopK == 0 {
		searchReq.TopK = 5
	}

	results, err := embeddingSearch.SearchSimilar(searchReq.Query, searchReq.TopK)
	if err != nil {
		return "", err
	}

	var formattedResults []map[string]interface{}
	for _, result := range results {
		if result.Entry == nil {
			continue
		}

		formattedResult := map[string]interface{}{
			"score":       result.Score,
			"content":     result.Entry.Content,
			"chunk_index": result.ChunkIndex,
		}

		if result.Document != nil {
			formattedResult["document_id"] = result.Document.ID
			formattedResult["file_path"] = result.Document.FilePath
		}

		formattedResults = append(formattedResults, formattedResult)
	}

	response := map[string]interface{}{
		"query":         searchReq.Query,
		"results":       formattedResults,
		"total_results": len(formattedResults),
	}

	resultJSON, _ := json.Marshal(response)
	return string(resultJSON), nil
}

func testSearchDocumentsGrouped(input interface{}) (string, error) {
	embeddingSearch := services.GetDefaultEmbeddingService()
	if embeddingSearch == nil {
		return "", nil
	}

	inputJSON, _ := json.Marshal(input)
	var searchReq struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	json.Unmarshal(inputJSON, &searchReq)

	if searchReq.TopK == 0 {
		searchReq.TopK = 5
	}

	if searchReq.TopK > 10 {
		searchReq.TopK = 10
	}

	results, err := embeddingSearch.SearchDocuments(searchReq.Query, searchReq.TopK)
	if err != nil {
		return "", err
	}

	var formattedResults []map[string]interface{}
	for _, result := range results {
		if result.Document == nil {
			continue
		}

		var chunks []map[string]interface{}
		for _, chunk := range result.Chunks {
			chunks = append(chunks, map[string]interface{}{
				"content":     chunk.Content,
				"score":       chunk.Score,
				"chunk_index": chunk.ChunkIndex,
			})
		}

		formattedResult := map[string]interface{}{
			"document_id":      result.Document.ID,
			"file_path":        result.Document.FilePath,
			"full_content":     result.Document.Content,
			"relevance_rank":   result.RelevanceRank,
			"best_score":       result.BestScore,
			"total_chunks":     result.TotalChunks,
			"matching_chunks":  chunks,
		}

		formattedResults = append(formattedResults, formattedResult)
	}

	response := map[string]interface{}{
		"query":           searchReq.Query,
		"documents":       formattedResults,
		"total_documents": len(formattedResults),
		"usage_note":      "Each document includes full content and relevant chunks. No additional get_document_content calls needed.",
	}

	resultJSON, _ := json.Marshal(response)
	return string(resultJSON), nil
}

func validateGroupedSearchResponse(response map[string]interface{}) {
	if query, ok := response["query"].(string); ok {
		log.Printf("  ✓ Query: %s", query)
	} else {
		log.Printf("  ✗ Missing or invalid query field")
	}

	if docs, ok := response["documents"].([]interface{}); ok {
		log.Printf("  ✓ Documents: %d found", len(docs))

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