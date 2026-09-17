package core

import (
	stdctx "context"
	"fmt"
	"strings"
	"time"

	"arcadia/modules/ai/interfaces"
	"arcadia/modules/ai/models"
	"arcadia/pkg/logging"
)

// AutoSearchExecutor executes document searches based on query analysis
type AutoSearchExecutor struct {
	embeddingSearch interfaces.EmbeddingSearch
	config          *AutoSearchConfig
	logger          logging.Logger
	metrics         interfaces.Metrics
}

// AutoSearchConfig contains configuration for auto-search
type AutoSearchConfig struct {
	Enabled         bool    // Global enable/disable
	MaxResults      int     // Max documents to include (default: 3)
	MinConfidence   float64 // Min confidence to trigger (default: 0.6)
	MaxContextSize  int     // Max chars for search results (default: 4000)
	IncludeMetadata bool    // Include file paths, etc
	Timeout         int     // Search timeout in seconds
}

// SearchContext contains the results of an auto-search
type SearchContext struct {
	Query          string
	ResultsFound   int
	Documents      []DocumentSummary
	TotalChars     int
	SearchDuration float64
}

// DocumentSummary contains a summary of a search result document
type DocumentSummary struct {
	ID             string
	FilePath       string
	Score          float64
	ContentPreview string
	Highlights     []string
}

// NewAutoSearchExecutor creates a new auto-search executor
func NewAutoSearchExecutor(
	embeddingSearch interfaces.EmbeddingSearch,
	config *AutoSearchConfig,
	logger logging.Logger,
	metrics interfaces.Metrics,
) *AutoSearchExecutor {
	// Set defaults if not configured
	if config.MaxResults == 0 {
		config.MaxResults = 3
	}
	if config.MinConfidence == 0 {
		config.MinConfidence = 0.6
	}
	if config.MaxContextSize == 0 {
		config.MaxContextSize = 4000
	}
	if config.Timeout == 0 {
		config.Timeout = 5
	}

	return &AutoSearchExecutor{
		embeddingSearch: embeddingSearch,
		config:          config,
		logger:          logger,
		metrics:         metrics,
	}
}

// Execute performs an auto-search based on the analysis result
func (ase *AutoSearchExecutor) Execute(ctx stdctx.Context, analysis *AnalysisResult) (*SearchContext, error) {
	if !ase.config.Enabled {
		return nil, fmt.Errorf("auto-search is disabled")
	}

	if analysis == nil || !analysis.ShouldSearch {
		return nil, fmt.Errorf("analysis indicates search should not be performed")
	}

	if ase.embeddingSearch == nil {
		return nil, fmt.Errorf("embedding search service not available")
	}

	startTime := time.Now()

	// Build search query from terms
	query := ase.buildSearchQuery(analysis)
	if query == "" {
		return nil, fmt.Errorf("could not build search query from analysis")
	}

	if ase.logger != nil {
		ase.logger.Debug(ctx, "Executing auto-search",
			"query", query,
			"max_results", ase.config.MaxResults,
			"confidence", analysis.Confidence)
	}

	// Perform search using enhanced search
	// Note: Timeout is handled by the embedding search service
	searchConfig := models.DefaultSearchConfig()
	results, err := ase.embeddingSearch.SearchDocumentsEnhanced(query, ase.config.MaxResults, searchConfig)
	if err != nil {
		if ase.logger != nil {
			ase.logger.Warn(ctx, "Auto-search failed", "error", err, "query", query)
		}
		return nil, fmt.Errorf("search failed: %w", err)
	}

	duration := time.Since(startTime).Milliseconds()

	// Format results
	searchContext := ase.formatResults(query, results, float64(duration))

	// Apply context size limit
	searchContext = ase.limitContextSize(searchContext, ase.config.MaxContextSize)

	if ase.logger != nil {
		ase.logger.Info(ctx, "Auto-search completed",
			"query", query,
			"results_found", searchContext.ResultsFound,
			"context_chars", searchContext.TotalChars,
			"duration_ms", duration)
	}

	if ase.metrics != nil {
		tags := map[string]string{
			"results_found": fmt.Sprintf("%d", searchContext.ResultsFound),
		}
		ase.metrics.RecordDuration("auto_search.search_duration", float64(duration), tags)
		ase.metrics.RecordValue("auto_search.results_count", float64(searchContext.ResultsFound), nil)
		ase.metrics.RecordValue("auto_search.context_size", float64(searchContext.TotalChars), nil)
	}

	return searchContext, nil
}

// buildSearchQuery builds a search query from the analysis result
func (ase *AutoSearchExecutor) buildSearchQuery(analysis *AnalysisResult) string {
	if len(analysis.SearchTerms) == 0 {
		return ""
	}

	// Use quoted phrases first (highest priority)
	if len(analysis.QuotedPhrases) > 0 {
		return strings.Join(analysis.QuotedPhrases, " ")
	}

	// Otherwise use all search terms
	return strings.Join(analysis.SearchTerms, " ")
}

// formatResults formats search results into SearchContext
func (ase *AutoSearchExecutor) formatResults(query string, results []*models.EnhancedDocumentSearchResult, duration float64) *SearchContext {
	context := &SearchContext{
		Query:          query,
		ResultsFound:   len(results),
		Documents:      make([]DocumentSummary, 0),
		TotalChars:     0,
		SearchDuration: duration,
	}

	for _, result := range results {
		if result.Document == nil {
			continue
		}

		summary := DocumentSummary{
			ID:         result.Document.ID,
			FilePath:   result.Document.FilePath,
			Score:      float64(result.BestScore),
			Highlights: result.ContextHighlights,
		}

		// Build content preview from highlights or document content
		if len(result.ContextHighlights) > 0 {
			// Use context highlights as preview (already relevant passages)
			summary.ContentPreview = strings.Join(result.ContextHighlights, "\n\n")
		} else {
			// Fallback to document content preview
			contentPreview := result.Document.Content
			if len(contentPreview) > 500 {
				contentPreview = contentPreview[:500] + "..."
			}
			summary.ContentPreview = contentPreview
		}

		context.TotalChars += len(summary.ContentPreview)
		if ase.config.IncludeMetadata {
			context.TotalChars += len(summary.FilePath) + 50 // Rough estimate for metadata
		}

		context.Documents = append(context.Documents, summary)
	}

	return context
}

// limitContextSize truncates the context if it exceeds the maximum size
func (ase *AutoSearchExecutor) limitContextSize(context *SearchContext, maxSize int) *SearchContext {
	if context.TotalChars <= maxSize {
		return context
	}

	ctx := stdctx.Background()
	if ase.logger != nil {
		ase.logger.Debug(ctx, "Limiting context size",
			"original_chars", context.TotalChars,
			"max_chars", maxSize,
			"original_docs", len(context.Documents))
	}

	// Truncate by removing documents from the end or shortening previews
	currentSize := 0
	truncatedDocs := make([]DocumentSummary, 0)

	for i, doc := range context.Documents {
		docSize := len(doc.ContentPreview)
		if ase.config.IncludeMetadata {
			docSize += len(doc.FilePath) + 50
		}

		if currentSize+docSize > maxSize {
			// Try to include a truncated version of this document
			remainingSpace := maxSize - currentSize
			if remainingSpace > 200 && i == 0 {
				// At least include first document, even if truncated
				truncatedPreview := doc.ContentPreview
				if len(truncatedPreview) > remainingSpace-100 {
					truncatedPreview = truncatedPreview[:remainingSpace-100] + "... [truncated]"
				}
				doc.ContentPreview = truncatedPreview
				truncatedDocs = append(truncatedDocs, doc)
			}
			break
		}

		truncatedDocs = append(truncatedDocs, doc)
		currentSize += docSize
	}

	context.Documents = truncatedDocs
	context.TotalChars = currentSize
	context.ResultsFound = len(truncatedDocs)

	if ase.logger != nil {
		ase.logger.Debug(ctx, "Context size limited",
			"new_chars", context.TotalChars,
			"new_docs", len(context.Documents))
	}

	return context
}
