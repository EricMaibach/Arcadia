package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// SearchEngine implements document search functionality
type SearchEngine struct {
	vectorStore     interfaces.VectorStoreInterface
	documentStore   interfaces.DocumentStoreInterface
	embeddingEngine *EmbeddingEngine
	logger          interfaces.Logger
	metrics         interfaces.MetricsCollector
}

// NewSearchEngine creates a new search engine
func NewSearchEngine(
	vectorStore interfaces.VectorStoreInterface,
	documentStore interfaces.DocumentStoreInterface,
	embeddingEngine *EmbeddingEngine,
) *SearchEngine {
	return &SearchEngine{
		vectorStore:     vectorStore,
		documentStore:   documentStore,
		embeddingEngine: embeddingEngine,
	}
}

// WithLogger adds logging to the search engine
func (se *SearchEngine) WithLogger(logger interfaces.Logger) *SearchEngine {
	se.logger = logger
	return se
}

// WithMetrics adds metrics collection to the search engine
func (se *SearchEngine) WithMetrics(metrics interfaces.MetricsCollector) *SearchEngine {
	se.metrics = metrics
	return se
}

// SearchDocuments searches for documents using semantic similarity
func (se *SearchEngine) SearchDocuments(ctx context.Context, query string, topK int) ([]*models.DocumentSearchResult, error) {
	log.Printf("[DEBUG] SearchDocuments called with query='%s', topK=%d", query, topK)

	if query == "" {
		log.Printf("[DEBUG] Empty query, returning empty results")
		return []*models.DocumentSearchResult{}, nil
	}

	startTime := time.Now()
	defer func() {
		if se.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			se.metrics.RecordTimer("search.duration", duration, map[string]string{"type": "basic"})
		}
	}()

	// Generate embedding for query
	log.Printf("[DEBUG] Generating embedding for query")
	queryVector, err := se.embeddingEngine.GenerateEmbedding(ctx, query)
	if err != nil {
		log.Printf("[ERROR] Failed to generate query embedding: %v", err)
		return nil, models.NewDocumentErrorWithCause(models.ErrSearchFailed, "failed to generate query embedding", err)
	}
	log.Printf("[DEBUG] Generated embedding vector of length %d", len(queryVector))

	// Search for similar vectors
	searchTopK := topK * 5 // Get more vectors to group by document
	log.Printf("[DEBUG] Searching for similar vectors with topK=%d", searchTopK)
	vectorResults, err := se.vectorStore.SearchSimilar(ctx, queryVector, searchTopK)
	if err != nil {
		log.Printf("[ERROR] Vector search failed: %v", err)
		return nil, models.NewDocumentErrorWithCause(models.ErrSearchFailed, "vector search failed", err)
	}
	log.Printf("[DEBUG] Vector search returned %d results", len(vectorResults))

	// Group results by document
	documentGroups := make(map[string]*models.DocumentSearchResult)
	for i, result := range vectorResults {
		docID := result.Entry.DocumentID
		log.Printf("[DEBUG] Processing vector result %d: DocID=%s, Score=%f, ContentLength=%d",
			i, docID, result.Score, len(result.Entry.Content))

		if existing, exists := documentGroups[docID]; exists {
			// Add chunk to existing document result
			chunk := &models.ChunkResult{
				Content:    result.Entry.Content,
				Score:      result.Score,
				ChunkIndex: extractChunkIndex(result.Entry.Metadata),
			}
			existing.Chunks = append(existing.Chunks, chunk)

			// Update best score if this chunk has a higher score
			if result.Score > existing.BestScore {
				existing.BestScore = result.Score
			}
			log.Printf("[DEBUG] Added chunk to existing document group for DocID=%s (total chunks: %d)", docID, len(existing.Chunks))
		} else {
			// Create new document result
			documentGroups[docID] = &models.DocumentSearchResult{
				Chunks: []*models.ChunkResult{{
					Content:    result.Entry.Content,
					Score:      result.Score,
					ChunkIndex: extractChunkIndex(result.Entry.Metadata),
				}},
				BestScore:   result.Score,
				TotalChunks: 1,
			}
			log.Printf("[DEBUG] Created new document group for DocID=%s", docID)
		}
	}

	log.Printf("[DEBUG] Grouped results into %d unique documents", len(documentGroups))

	// Fetch document details
	var results []*models.DocumentSearchResult
	for docID, docResult := range documentGroups {
		log.Printf("[DEBUG] Fetching document details for DocID=%s", docID)
		doc, err := se.documentStore.GetDocument(ctx, docID)
		if err != nil {
			log.Printf("[WARN] Failed to fetch document %s: %v", docID, err)
			if se.logger != nil {
				se.logger.Warn(ctx, "Failed to fetch document for search result", "doc_id", docID, "error", err)
			}
			continue
		}

		log.Printf("[DEBUG] Successfully fetched document: ID=%s, FilePath=%s, ContentLength=%d",
			doc.ID, doc.FilePath, len(doc.Content))

		docResult.Document = doc
		docResult.TotalChunks = len(docResult.Chunks)

		// Sort chunks by score (highest first)
		sort.Slice(docResult.Chunks, func(i, j int) bool {
			return docResult.Chunks[i].Score > docResult.Chunks[j].Score
		})

		results = append(results, docResult)
	}

	log.Printf("[DEBUG] Successfully processed %d documents", len(results))

	// Sort results by best score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].BestScore > results[j].BestScore
	})

	// Set relevance ranks
	for i, result := range results {
		result.RelevanceRank = i + 1
	}

	// Limit to topK results
	if len(results) > topK {
		log.Printf("[DEBUG] Limiting results from %d to %d", len(results), topK)
		results = results[:topK]
	}

	if se.metrics != nil {
		se.metrics.IncrementCounter("search.completed", map[string]string{
			"results": fmt.Sprintf("%d", len(results)),
		})
	}

	log.Printf("[DEBUG] SearchDocuments returning %d final results", len(results))
	return results, nil
}

// SearchDocumentsEnhanced performs enhanced document search with full content
func (se *SearchEngine) SearchDocumentsEnhanced(ctx context.Context, query string, topK int, config models.SearchConfig) ([]*models.EnhancedDocumentSearchResult, error) {
	log.Printf("[DEBUG] SearchDocumentsEnhanced called with query='%s', topK=%d, config=%+v", query, topK, config)

	// First perform basic search
	basicResults, err := se.SearchDocuments(ctx, query, topK)
	if err != nil {
		log.Printf("[ERROR] SearchDocuments failed: %v", err)
		return nil, err
	}

	log.Printf("[DEBUG] SearchDocuments returned %d basic results", len(basicResults))

	// Convert to enhanced results
	enhancedResults := make([]*models.EnhancedDocumentSearchResult, 0, len(basicResults))

	for i, result := range basicResults {
		log.Printf("[DEBUG] Processing basic result %d: Document=%v, BestScore=%f, ChunksCount=%d",
			i, result.Document != nil, result.BestScore, len(result.Chunks))

		if result.Document != nil {
			log.Printf("[DEBUG] Document %d details: ID=%s, FilePath=%s, ContentLength=%d",
				i, result.Document.ID, result.Document.FilePath, len(result.Document.Content))
		}

		enhanced := &models.EnhancedDocumentSearchResult{
			Document:      result.Document,
			BestScore:     result.BestScore,
			RelevanceRank: result.RelevanceRank,
		}

		// Generate context highlights
		enhanced.ContextHighlights = se.generateContextHighlights(result.Chunks, config.MaxHighlights)
		log.Printf("[DEBUG] Generated %d context highlights for result %d", len(enhanced.ContextHighlights), i)

		// Generate content preview
		contentSize := len(result.Document.Content)
		if config.IncludeFullContent || contentSize <= config.MaxDocumentSize {
			enhanced.ContentPreview = result.Document.Content
			enhanced.IsTruncated = false
			log.Printf("[DEBUG] Using full content for result %d (size: %d)", i, contentSize)
		} else {
			// Truncate content and add ellipsis
			enhanced.ContentPreview = result.Document.Content[:config.MaxDocumentSize] + "..."
			enhanced.IsTruncated = true
			log.Printf("[DEBUG] Truncated content for result %d (size: %d -> %d)", i, contentSize, config.MaxDocumentSize)
		}

		enhancedResults = append(enhancedResults, enhanced)
	}

	log.Printf("[DEBUG] SearchDocumentsEnhanced returning %d enhanced results", len(enhancedResults))
	return enhancedResults, nil
}

// SearchByVector searches for documents using a pre-computed vector
func (se *SearchEngine) SearchByVector(ctx context.Context, vector []float32, topK int) ([]*models.SearchResult, error) {
	if len(vector) == 0 {
		return []*models.SearchResult{}, nil
	}

	return se.vectorStore.SearchSimilar(ctx, vector, topK)
}

// GenerateQueryEmbedding generates an embedding for a search query
func (se *SearchEngine) GenerateQueryEmbedding(ctx context.Context, query string) ([]float32, error) {
	return se.embeddingEngine.GenerateEmbedding(ctx, query)
}

// ReconstructContentFromChunks reconstructs content from vector entries
func (se *SearchEngine) ReconstructContentFromChunks(vectors []*models.VectorEntry) string {
	if len(vectors) == 0 {
		return ""
	}

	// Parse chunk metadata and create sortable structure
	type chunkInfo struct {
		content    string
		chunkIndex int
		startPos   int
		endPos     int
	}

	var chunks []chunkInfo
	for _, vector := range vectors {
		// Use the enhanced extractChunkIndex that supports overlap handling
		chunkIndex, startPos, endPos := extractChunkMetadata(vector.Metadata)

		chunks = append(chunks, chunkInfo{
			content:    vector.Content,
			chunkIndex: chunkIndex,
			startPos:   startPos,
			endPos:     endPos,
		})
	}

	// Sort chunks by chunk index
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].chunkIndex < chunks[j].chunkIndex
	})

	// Reconstruct content handling overlap
	var result strings.Builder
	for i, chunk := range chunks {
		if i == 0 {
			// First chunk - add full content
			result.WriteString(chunk.content)
		} else {
			// For subsequent chunks, we need to handle overlap
			prevChunk := chunks[i-1]

			// If we have position information, use it to determine overlap
			if chunk.startPos > 0 && prevChunk.endPos > 0 {
				overlap := prevChunk.endPos - chunk.startPos
				if overlap > 0 && overlap < len(chunk.content) {
					// Remove the overlapping part from current chunk
					result.WriteString(chunk.content[overlap:])
				} else {
					// No valid overlap info, just append
					result.WriteString(chunk.content)
				}
			} else {
				// No position info available, assume default 50-char overlap
				overlapSize := 50
				if overlapSize < len(chunk.content) {
					result.WriteString(chunk.content[overlapSize:])
				} else {
					// Chunk is smaller than overlap, this shouldn't happen normally
					result.WriteString(chunk.content)
				}
			}
		}
	}

	return result.String()
}

// generateContextHighlights generates context highlights from chunk results
func (se *SearchEngine) generateContextHighlights(chunks []*models.ChunkResult, maxHighlights int) []string {
	if len(chunks) == 0 || maxHighlights <= 0 {
		return []string{}
	}

	highlights := make([]string, 0, min(len(chunks), maxHighlights))

	for i, chunk := range chunks {
		if i >= maxHighlights {
			break
		}

		// Truncate long chunks for highlights
		content := chunk.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}

		highlights = append(highlights, content)
	}

	return highlights
}

// extractChunkIndex extracts chunk index from metadata JSON string
func extractChunkIndex(metadata string) int {
	index, _, _ := extractChunkMetadata(metadata)
	return index
}

// extractChunkMetadata extracts chunk metadata including positions from JSON string
func extractChunkMetadata(metadata string) (int, int, int) {
	if metadata == "" {
		return 0, 0, 0
	}

	// Try to parse JSON metadata for chunk info
	var metaData struct {
		ChunkIndex int `json:"chunk_index"`
		StartPos   int `json:"start_pos"`
		EndPos     int `json:"end_pos"`
	}

	if err := json.Unmarshal([]byte(metadata), &metaData); err != nil {
		// Fallback: try to extract chunk index from simple patterns
		// This handles cases where metadata might be in different formats
		return 0, 0, 0
	}

	return metaData.ChunkIndex, metaData.StartPos, metaData.EndPos
}

