package core

import (
	"context"
	"regexp"
	"strings"

	"arcadia/modules/ai/interfaces"
	"arcadia/pkg/logging"
)

// QueryAnalyzer analyzes user prompts to determine if document search would be helpful
type QueryAnalyzer struct {
	logger  logging.Logger
	metrics interfaces.Metrics
}

// AnalysisResult contains the results of query analysis
type AnalysisResult struct {
	ShouldSearch   bool     // Whether auto-search should run
	SearchTerms    []string // Extracted search terms
	Confidence     float64  // Confidence score (0.0-1.0)
	Reason         string   // Why this decision was made
	QuotedPhrases  []string // Quoted text from prompt
	DetectedIntent string   // "search", "question", "command", "chitchat"
}

// NewQueryAnalyzer creates a new query analyzer
func NewQueryAnalyzer(logger logging.Logger, metrics interfaces.Metrics) *QueryAnalyzer {
	return &QueryAnalyzer{
		logger:  logger,
		metrics: metrics,
	}
}

// AnalyzeQuery analyzes a user prompt and determines if auto-search should be triggered
func (qa *QueryAnalyzer) AnalyzeQuery(prompt string) *AnalysisResult {
	result := &AnalysisResult{
		ShouldSearch:   false,
		SearchTerms:    make([]string, 0),
		Confidence:     0.0,
		Reason:         "",
		QuotedPhrases:  make([]string, 0),
		DetectedIntent: "unknown",
	}

	// Normalize prompt
	normalized := strings.TrimSpace(strings.ToLower(prompt))
	if len(normalized) == 0 {
		result.Reason = "empty prompt"
		result.DetectedIntent = "empty"
		return result
	}

	// Check if we should skip search (chitchat, greetings, etc.)
	if qa.shouldSkipSearch(normalized) {
		result.Reason = "detected non-search intent (greeting, math, or chitchat)"
		result.DetectedIntent = "chitchat"
		result.Confidence = 0.2
		return result
	}

	// Extract quoted phrases
	result.QuotedPhrases = qa.extractQuotedPhrases(prompt)

	// Detect search intent
	searchIntent := qa.detectSearchIntent(normalized)
	if !searchIntent {
		result.Reason = "no search intent detected"
		result.DetectedIntent = "general"
		result.Confidence = 0.3
		return result
	}

	// Extract keywords and search terms
	keywords := qa.extractKeywords(prompt)

	// Combine quoted phrases and keywords as search terms
	result.SearchTerms = append(result.SearchTerms, result.QuotedPhrases...)
	result.SearchTerms = append(result.SearchTerms, keywords...)
	result.SearchTerms = qa.deduplicateTerms(result.SearchTerms)

	// Calculate confidence based on signals
	confidence := qa.calculateConfidence(normalized, result)
	result.Confidence = confidence

	// Determine if we should search
	if confidence >= 0.6 && len(result.SearchTerms) > 0 {
		result.ShouldSearch = true
		result.DetectedIntent = "search"
		result.Reason = "high confidence search intent with extractable terms"
	} else if len(result.SearchTerms) == 0 {
		result.ShouldSearch = false
		result.DetectedIntent = "question"
		result.Reason = "search intent detected but no specific terms to search"
	} else {
		result.ShouldSearch = false
		result.DetectedIntent = "low_confidence"
		result.Reason = "confidence below threshold"
	}
	ctx := context.Background()

	if qa.logger != nil {
		qa.logger.Debug(ctx, "Query analysis completed",
			"should_search", result.ShouldSearch,
			"confidence", result.Confidence,
			"intent", result.DetectedIntent,
			"terms_count", len(result.SearchTerms))
	}

	return result
}

// shouldSkipSearch checks if the prompt matches skip patterns
func (qa *QueryAnalyzer) shouldSkipSearch(normalized string) bool {
	// Greeting patterns
	greetingPatterns := []string{
		"^(hi|hello|hey|good morning|good afternoon|good evening)",
		"^how are you",
		"^what's up",
		"^greetings",
	}

	// Math patterns
	mathPatterns := []string{
		"what is \\d+",
		"calculate",
		"^\\d+\\s*[+\\-*/]\\s*\\d+",
	}

	// Chitchat patterns
	chitchatPatterns := []string{
		"^thank you",
		"^thanks",
		"^ok$",
		"^okay$",
		"^yes$",
		"^no$",
		"^sure$",
	}

	allPatterns := append(greetingPatterns, mathPatterns...)
	allPatterns = append(allPatterns, chitchatPatterns...)

	for _, pattern := range allPatterns {
		matched, _ := regexp.MatchString(pattern, normalized)
		if matched {
			return true
		}
	}

	return false
}

// detectSearchIntent checks if the prompt indicates search intent
func (qa *QueryAnalyzer) detectSearchIntent(normalized string) bool {
	// Question words
	questionWords := []string{
		"what", "when", "where", "who", "why", "how",
		"which", "whose", "whom",
	}

	// Command words
	commandWords := []string{
		"find", "search", "show", "get", "list",
		"tell me about", "explain", "describe",
		"summarize", "give me", "provide",
	}

	// Document reference words
	docWords := []string{
		"document", "file", "note", "meeting",
		"contract", "report", "email", "message",
		"presentation", "spreadsheet", "pdf",
	}

	// Check for question words
	for _, word := range questionWords {
		if strings.HasPrefix(normalized, word+" ") || strings.Contains(normalized, " "+word+" ") {
			return true
		}
	}

	// Check for command words
	for _, word := range commandWords {
		if strings.HasPrefix(normalized, word+" ") || strings.Contains(normalized, " "+word+" ") {
			return true
		}
	}

	// Check for document reference words
	for _, word := range docWords {
		if strings.Contains(normalized, word) {
			return true
		}
	}

	// Check if it's a question (ends with ?)
	if strings.HasSuffix(strings.TrimSpace(normalized), "?") && len(normalized) > 10 {
		return true
	}

	return false
}

// extractQuotedPhrases extracts text within quotes
func (qa *QueryAnalyzer) extractQuotedPhrases(prompt string) []string {
	phrases := make([]string, 0)

	// Match both double and single quotes
	patterns := []string{
		`"([^"]+)"`,
		`'([^']+)'`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(prompt, -1)
		for _, match := range matches {
			if len(match) > 1 && len(match[1]) > 0 {
				phrases = append(phrases, match[1])
			}
		}
	}

	return phrases
}

// extractKeywords extracts important keywords from the prompt
func (qa *QueryAnalyzer) extractKeywords(prompt string) []string {
	keywords := make([]string, 0)

	// Stop words to ignore
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "from": true, "by": true,
		"is": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "must": true,
		"can": true, "this": true, "that": true, "these": true, "those": true,
		"i": true, "you": true, "he": true, "she": true, "it": true,
		"we": true, "they": true, "what": true, "when": true, "where": true,
		"who": true, "why": true, "how": true, "which": true,
		"me": true, "my": true, "our": true, "your": true,
	}

	// Split into words
	words := strings.FieldsFunc(prompt, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_')
	})

	for _, word := range words {
		normalized := strings.ToLower(strings.TrimSpace(word))

		// Skip if too short, is stop word, or all numbers
		if len(normalized) < 3 {
			continue
		}
		if stopWords[normalized] {
			continue
		}
		if regexp.MustCompile(`^\d+$`).MatchString(normalized) {
			continue
		}

		// Keep capitalized words (likely proper nouns) or longer words
		if len(word) >= 4 || (len(word) >= 2 && word[0] >= 'A' && word[0] <= 'Z') {
			keywords = append(keywords, word)
		}
	}

	// Limit to top keywords (by length and position)
	if len(keywords) > 5 {
		keywords = keywords[:5]
	}

	return keywords
}

// calculateConfidence calculates a confidence score for search intent
func (qa *QueryAnalyzer) calculateConfidence(normalized string, result *AnalysisResult) float64 {
	confidence := 0.0

	// Base confidence for having search terms
	if len(result.SearchTerms) > 0 {
		confidence += 0.3
	}

	// Boost for quoted phrases (high confidence)
	if len(result.QuotedPhrases) > 0 {
		confidence += 0.3
	}

	// Boost for question marks
	if strings.Contains(normalized, "?") {
		confidence += 0.1
	}

	// Boost for document-related words
	docWords := []string{"document", "file", "contract", "report", "note", "meeting", "email"}
	for _, word := range docWords {
		if strings.Contains(normalized, word) {
			confidence += 0.15
			break
		}
	}

	// Boost for specific action words
	actionWords := []string{"find", "search", "show me", "get", "what did"}
	for _, word := range actionWords {
		if strings.Contains(normalized, word) {
			confidence += 0.15
			break
		}
	}

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// deduplicateTerms removes duplicate terms
func (qa *QueryAnalyzer) deduplicateTerms(terms []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, term := range terms {
		normalized := strings.ToLower(strings.TrimSpace(term))
		if !seen[normalized] && len(normalized) > 0 {
			seen[normalized] = true
			result = append(result, term)
		}
	}

	return result
}
