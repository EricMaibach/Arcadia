package core

import (
	"context"
	"fmt"
	"strings"

	"arcadia/pkg/logging"
)

// ContextBuilder builds enriched prompts with search context
type ContextBuilder struct {
	logger logging.Logger
}

// EnrichedPrompt contains the original prompt enriched with search context
type EnrichedPrompt struct {
	OriginalPrompt string
	SearchContext  *SearchContext
	FinalPrompt    string
	ContextAdded   bool
	TotalTokens    int // Estimated
}

// NewContextBuilder creates a new context builder
func NewContextBuilder(logger logging.Logger) *ContextBuilder {
	return &ContextBuilder{
		logger: logger,
	}
}

// BuildEnrichedPrompt builds an enriched prompt with search context
func (cb *ContextBuilder) BuildEnrichedPrompt(originalPrompt string, searchContext *SearchContext) *EnrichedPrompt {
	result := &EnrichedPrompt{
		OriginalPrompt: originalPrompt,
		SearchContext:  searchContext,
		ContextAdded:   false,
		TotalTokens:    0,
	}

	// If no search context or no results, return original prompt
	if searchContext == nil || searchContext.ResultsFound == 0 {
		result.FinalPrompt = originalPrompt
		result.TotalTokens = cb.estimateTokens(originalPrompt)
		return result
	}

	// Build the search context section
	contextSection := cb.formatSearchContext(searchContext)

	// Combine context with original prompt
	result.FinalPrompt = fmt.Sprintf("%s\n\n%s", contextSection, originalPrompt)
	result.ContextAdded = true
	result.TotalTokens = cb.estimateTokens(result.FinalPrompt)

	ctx := context.Background()
	if cb.logger != nil {
		cb.logger.Debug(ctx, "Built enriched prompt",
			"original_length", len(originalPrompt),
			"context_length", len(contextSection),
			"final_length", len(result.FinalPrompt),
			"estimated_tokens", result.TotalTokens)
	}

	return result
}

// formatSearchContext formats the search context into a readable section
func (cb *ContextBuilder) formatSearchContext(context *SearchContext) string {
	var builder strings.Builder

	// Header
	builder.WriteString("[DOCUMENT SEARCH RESULTS]\n")
	builder.WriteString(fmt.Sprintf("Found %d relevant document(s) for your query:\n\n", context.ResultsFound))

	// Add each document
	for i, doc := range context.Documents {
		builder.WriteString(fmt.Sprintf("Document %d: %s\n", i+1, doc.FilePath))
		builder.WriteString(fmt.Sprintf("Relevance Score: %.2f\n", doc.Score))
		builder.WriteString("---\n")

		// Add highlights if available
		if len(doc.Highlights) > 0 {
			builder.WriteString("Relevant passages:\n")
			for j, highlight := range doc.Highlights {
				// Limit highlights to top 3
				if j >= 3 {
					break
				}
				builder.WriteString(fmt.Sprintf("\n[Passage %d]\n%s\n", j+1, highlight))
			}
		} else {
			// Fallback to content preview
			builder.WriteString("Content:\n")
			builder.WriteString(doc.ContentPreview)
			builder.WriteString("\n")
		}

		builder.WriteString("\n")
	}

	// Footer
	builder.WriteString("[END DOCUMENT SEARCH RESULTS]\n")
	builder.WriteString("\nPlease use the above documents to answer the following question:\n")

	return builder.String()
}

// estimateTokens provides a rough token estimate (chars / 4)
func (cb *ContextBuilder) estimateTokens(text string) int {
	// Rough approximation: 1 token ≈ 4 characters
	return len(text) / 4
}
