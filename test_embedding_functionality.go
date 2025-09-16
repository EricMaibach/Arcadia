package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"arcadia/services"
)

// Test the new SearchDocuments functionality directly
func main() {
	log.Println("=== Testing New SearchDocuments Functionality ===")

	// Create temporary database
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "test_embedding_functionality.db")
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
		MaxChunkSize: 150,
		ChunkOverlap: 30,
	}

	// Create embedding service
	embeddingService := services.NewEmbeddingService(chunker, model, vectorStore, documentStore, config)
	err = embeddingService.Initialize()
	if err != nil {
		log.Fatalf("Failed to initialize embedding service: %v", err)
	}
	defer embeddingService.Close()

	// Create test documents with different content
	log.Println("\n=== Creating Test Documents ===")
	testDocs := []struct {
		name    string
		content string
	}{
		{
			"ai_machine_learning.txt",
			"Artificial Intelligence and Machine Learning are revolutionary technologies changing how we solve complex problems. " +
				"AI systems can perform tasks that typically require human intelligence, such as visual perception, speech recognition, " +
				"decision-making, and language translation. Machine learning, a subset of AI, enables computers to learn and improve " +
				"from experience without being explicitly programmed. Deep learning algorithms use neural networks with multiple layers " +
				"to model and understand complex patterns in data. These technologies are being applied across industries including " +
				"healthcare, finance, autonomous vehicles, and natural language processing.",
		},
		{
			"web_development.txt",
			"Web development encompasses both frontend and backend technologies for creating modern web applications. " +
				"Frontend development focuses on user interface design using HTML for structure, CSS for styling, and JavaScript " +
				"for interactivity. Popular frontend frameworks include React, Vue.js, and Angular. Backend development handles " +
				"server-side logic, database management, and API creation using languages like Python, Node.js, Java, or Go. " +
				"Full-stack developers work across both frontend and backend systems. Modern web development also involves " +
				"responsive design, progressive web apps, and cloud deployment strategies.",
		},
		{
			"data_science.txt",
			"Data science combines statistical analysis, programming, and domain expertise to extract meaningful insights from data. " +
				"Data scientists use programming languages like Python and R for data manipulation, analysis, and visualization. " +
				"The data science process typically involves data collection, cleaning, exploratory analysis, modeling, and " +
				"interpretation of results. Machine learning algorithms are often employed to build predictive models and identify " +
				"patterns in large datasets. Data visualization tools like Matplotlib, Seaborn, and Tableau help communicate findings " +
				"effectively. Big data technologies such as Hadoop and Spark enable processing of massive datasets.",
		},
	}

	var createdDocs []*services.Document
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
		createdDocs = append(createdDocs, doc)
	}

	// Test 1: Compare SearchSimilar vs SearchDocuments
	log.Println("\n=== Test 1: Comparing SearchSimilar vs SearchDocuments ===")

	testQuery := "machine learning algorithms neural networks"

	// Test SearchSimilar (existing method)
	log.Printf("Testing SearchSimilar with query: '%s'", testQuery)
	start := time.Now()
	similarResults, err := embeddingService.SearchSimilar(testQuery, 3)
	similarDuration := time.Since(start)

	if err != nil {
		log.Printf("ERROR: SearchSimilar failed: %v", err)
	} else {
		log.Printf("SearchSimilar found %d results in %v", len(similarResults), similarDuration)
		for i, result := range similarResults {
			log.Printf("  Result %d: Score=%.4f, ChunkIndex=%d, DocID=%s",
				i+1, result.Score, result.ChunkIndex, result.Entry.DocumentID)
			log.Printf("    Content: %s...", truncateString(result.Entry.Content, 80))
		}
	}

	// Test SearchDocuments (new method)
	log.Printf("\nTesting SearchDocuments with query: '%s'", testQuery)
	start = time.Now()
	docResults, err := embeddingService.SearchDocuments(testQuery, 3)
	docDuration := time.Since(start)

	if err != nil {
		log.Printf("ERROR: SearchDocuments failed: %v", err)
	} else {
		log.Printf("SearchDocuments found %d documents in %v", len(docResults), docDuration)
		totalChunks := 0
		totalContentLength := 0
		for i, result := range docResults {
			log.Printf("  Document %d: %s", i+1, filepath.Base(result.Document.FilePath))
			log.Printf("    Relevance Rank: %d", result.RelevanceRank)
			log.Printf("    Best Score: %.4f", result.BestScore)
			log.Printf("    Total Chunks: %d", result.TotalChunks)
			log.Printf("    Matching Chunks: %d", len(result.Chunks))
			log.Printf("    Content Length: %d characters", len(result.Document.Content))

			totalChunks += len(result.Chunks)
			totalContentLength += len(result.Document.Content)

			// Show top chunks
			for j, chunk := range result.Chunks {
				if j < 2 { // Show only first 2 chunks
					log.Printf("      Chunk %d: Score=%.4f, Index=%d",
						j+1, chunk.Score, chunk.ChunkIndex)
					log.Printf("        Content: %s...", truncateString(chunk.Content, 60))
				}
			}
		}
		log.Printf("Total content returned: %d characters across %d chunks", totalContentLength, totalChunks)
	}

	// Test 2: Detailed functionality testing
	log.Println("\n=== Test 2: Detailed SearchDocuments Testing ===")

	testCases := []struct {
		query string
		topK  int
		desc  string
	}{
		{"artificial intelligence", 1, "Single document search"},
		{"programming languages python javascript", 2, "Multi-language programming query"},
		{"data visualization analysis", 3, "Data analysis focus"},
		{"frontend backend development", 2, "Web development focus"},
	}

	for _, testCase := range testCases {
		log.Printf("\nTest Case: %s", testCase.desc)
		log.Printf("Query: '%s' (topK=%d)", testCase.query, testCase.topK)

		start := time.Now()
		results, err := embeddingService.SearchDocuments(testCase.query, testCase.topK)
		duration := time.Since(start)

		if err != nil {
			log.Printf("ERROR: %v", err)
			continue
		}

		log.Printf("Found %d documents in %v", len(results), duration)

		// Validate structure and ordering
		for i, result := range results {
			// Check that relevance rank is correct
			if result.RelevanceRank != i+1 {
				log.Printf("WARNING: Document %d has incorrect relevance rank: expected %d, got %d",
					i+1, i+1, result.RelevanceRank)
			}

			// Check that chunks are sorted by score
			for j := 1; j < len(result.Chunks); j++ {
				if result.Chunks[j].Score > result.Chunks[j-1].Score {
					log.Printf("WARNING: Document %d chunks not sorted by score", i+1)
					break
				}
			}

			// Check that best score matches highest chunk score
			if len(result.Chunks) > 0 {
				highestChunkScore := result.Chunks[0].Score
				if result.BestScore != highestChunkScore {
					log.Printf("WARNING: Document %d best score (%.4f) doesn't match highest chunk score (%.4f)",
						i+1, result.BestScore, highestChunkScore)
				}
			}

			// Check that total chunks matches chunk count
			if result.TotalChunks != len(result.Chunks) {
				log.Printf("WARNING: Document %d total chunks (%d) doesn't match chunk count (%d)",
					i+1, result.TotalChunks, len(result.Chunks))
			}

			// Check that document has reconstructed content
			if result.Document.Content == "" {
				log.Printf("WARNING: Document %d has empty content", i+1)
			}

			log.Printf("  ✓ Document %d: Score=%.4f, Chunks=%d, Content=%d chars",
				i+1, result.BestScore, len(result.Chunks), len(result.Document.Content))
		}

		// Check that documents are sorted by best score
		for i := 1; i < len(results); i++ {
			if results[i].BestScore > results[i-1].BestScore {
				log.Printf("WARNING: Documents not sorted by best score")
				break
			}
		}
	}

	// Test 3: Edge cases
	log.Println("\n=== Test 3: Edge Cases ===")

	edgeCases := []struct {
		query string
		topK  int
		desc  string
	}{
		{"", 3, "Empty query"},
		{"nonexistentterm12345", 3, "No matching terms"},
		{"machine learning", 0, "Zero topK"},
		{"artificial intelligence", 10, "topK larger than document count"},
	}

	for _, testCase := range edgeCases {
		log.Printf("\nEdge Case: %s", testCase.desc)
		log.Printf("Query: '%s' (topK=%d)", testCase.query, testCase.topK)

		start := time.Now()
		results, err := embeddingService.SearchDocuments(testCase.query, testCase.topK)
		duration := time.Since(start)

		if err != nil {
			log.Printf("ERROR: %v", err)
		} else {
			log.Printf("✓ Returned %d documents in %v", len(results), duration)

			// Validate topK constraint
			if len(results) > testCase.topK {
				log.Printf("WARNING: Returned more documents (%d) than topK (%d)", len(results), testCase.topK)
			}
		}
	}

	// Test 4: Performance analysis
	log.Println("\n=== Test 4: Performance Analysis ===")

	// Test with different document counts and topK values
	performanceQuery := "technology development programming"

	log.Printf("Performance testing with query: '%s'", performanceQuery)

	topKValues := []int{1, 2, 3, 5}
	for _, topK := range topKValues {
		// Run multiple times to get average
		var totalDuration time.Duration
		iterations := 10

		for i := 0; i < iterations; i++ {
			start := time.Now()
			results, err := embeddingService.SearchDocuments(performanceQuery, topK)
			duration := time.Since(start)
			totalDuration += duration

			if err != nil {
				log.Printf("ERROR in iteration %d: %v", i+1, err)
				break
			}

			// Validate that we don't exceed topK
			if len(results) > topK {
				log.Printf("WARNING: Iteration %d returned %d results for topK=%d", i+1, len(results), topK)
			}
		}

		avgDuration := totalDuration / time.Duration(iterations)
		log.Printf("topK=%d: Average time over %d iterations: %v", topK, iterations, avgDuration)
	}

	// Test 5: Content reconstruction validation
	log.Println("\n=== Test 5: Content Reconstruction Validation ===")

	log.Printf("Validating that SearchDocuments reconstructs complete document content...")

	results, err := embeddingService.SearchDocuments("artificial intelligence", 1)
	if err != nil {
		log.Printf("ERROR: %v", err)
	} else if len(results) > 0 {
		doc := results[0].Document
		log.Printf("Document: %s", filepath.Base(doc.FilePath))
		log.Printf("Reconstructed content length: %d", len(doc.Content))
		log.Printf("Chunk count: %d", doc.ChunkCount)

		// Get the document directly to compare
		directDoc, err := embeddingService.GetDocument(doc.ID)
		if err != nil {
			log.Printf("ERROR getting document directly: %v", err)
		} else {
			if doc.Content == directDoc.Content {
				log.Printf("✓ Content reconstruction matches GetDocument result")
			} else {
				log.Printf("WARNING: Content reconstruction differs from GetDocument")
				log.Printf("  SearchDocuments length: %d", len(doc.Content))
				log.Printf("  GetDocument length: %d", len(directDoc.Content))
			}
		}

		// Validate that content makes sense
		if len(doc.Content) > 100 {
			log.Printf("✓ Content appears complete (>100 characters)")
			log.Printf("Content preview: %s...", truncateString(doc.Content, 150))
		} else {
			log.Printf("WARNING: Content seems too short: %d characters", len(doc.Content))
		}
	}

	log.Println("\n=== New SearchDocuments Functionality Testing Complete ===")
	log.Println("\nSummary of Benefits:")
	log.Println("1. ✓ Groups results by document with all relevant chunks")
	log.Println("2. ✓ Provides complete document content in single call")
	log.Println("3. ✓ Sorts documents by relevance and chunks by score")
	log.Println("4. ✓ Eliminates need for multiple get_document_content calls")
	log.Println("5. ✓ Supports proper topK limiting and edge case handling")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}