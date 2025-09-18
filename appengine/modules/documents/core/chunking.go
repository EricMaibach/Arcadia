package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"arcadia/modules/documents/models"
)

// SimpleTextChunker implements TextChunkerInterface with basic chunking strategies
type SimpleTextChunker struct {
	config models.ChunkingConfig
}

// NewSimpleTextChunker creates a new simple text chunker
func NewSimpleTextChunker() *SimpleTextChunker {
	return &SimpleTextChunker{
		config: models.DefaultChunkingConfig(),
	}
}

// NewSimpleTextChunkerWithConfig creates a new simple text chunker with custom configuration
func NewSimpleTextChunkerWithConfig(config models.ChunkingConfig) *SimpleTextChunker {
	return &SimpleTextChunker{
		config: config,
	}
}

// ChunkText chunks text based on the specified strategy
func (c *SimpleTextChunker) ChunkText(content string, config models.ChunkingConfig) []models.TextChunk {
	if content == "" {
		return []models.TextChunk{}
	}

	switch config.Strategy {
	case models.ChunkingStrategySentence:
		return c.chunkBySentence(content, config)
	case models.ChunkingStrategyParagraph:
		return c.chunkByParagraph(content, config)
	default:
		return c.chunkFixed(content, config)
	}
}

// ChunkDocument chunks a document's content
func (c *SimpleTextChunker) ChunkDocument(doc *models.Document, config models.ChunkingConfig) []models.TextChunk {
	chunks := c.ChunkText(doc.Content, config)

	// Set document ID for all chunks
	for i := range chunks {
		chunks[i].DocumentID = doc.ID
	}

	return chunks
}

// ValidateConfig validates a chunking configuration
func (c *SimpleTextChunker) ValidateConfig(config models.ChunkingConfig) error {
	if config.MaxChunkSize <= 0 {
		return models.NewDocumentError(models.ErrInvalidConfig, "max chunk size must be positive")
	}

	if config.ChunkOverlap < 0 {
		return models.NewDocumentError(models.ErrInvalidConfig, "chunk overlap cannot be negative")
	}

	if config.ChunkOverlap >= config.MaxChunkSize {
		return models.NewDocumentError(models.ErrInvalidConfig, "chunk overlap must be less than max chunk size")
	}

	switch config.Strategy {
	case models.ChunkingStrategyFixed, models.ChunkingStrategySentence, models.ChunkingStrategyParagraph:
		// Valid strategies
	default:
		return models.NewDocumentError(models.ErrInvalidConfig, fmt.Sprintf("unsupported chunking strategy: %s", config.Strategy))
	}

	return nil
}

// chunkFixed chunks text into fixed-size chunks with overlap
func (c *SimpleTextChunker) chunkFixed(content string, config models.ChunkingConfig) []models.TextChunk {
	var chunks []models.TextChunk
	contentRunes := []rune(content)
	length := len(contentRunes)

	if length == 0 {
		return chunks
	}

	start := 0
	chunkIndex := 0

	for start < length {
		end := min(start+config.MaxChunkSize, length)

		// Try to break at word boundary if we're not at the end
		if end < length {
			end = c.findWordBoundary(contentRunes, start, end)
		}

		// Create chunk
		chunkContent := string(contentRunes[start:end])
		chunk := models.TextChunk{
			ID:         generateChunkID(),
			Content:    chunkContent,
			StartPos:   start,
			EndPos:     end,
			ChunkIndex: chunkIndex,
			CreatedAt:  time.Now(),
		}
		chunks = append(chunks, chunk)

		// Move to next chunk with overlap
		start += config.MaxChunkSize - config.ChunkOverlap
		if start <= 0 {
			start = config.MaxChunkSize
		}

		// Ensure we're making progress
		if start >= end {
			start = end
		}

		chunkIndex++
	}

	return chunks
}

// chunkBySentence chunks text by sentences
func (c *SimpleTextChunker) chunkBySentence(content string, config models.ChunkingConfig) []models.TextChunk {
	sentences := c.splitIntoSentences(content)
	var chunks []models.TextChunk
	var currentChunk strings.Builder
	currentStart := 0
	chunkIndex := 0
	sentenceStart := 0

	for _, sentence := range sentences {
		// Check if adding this sentence would exceed max chunk size
		if currentChunk.Len()+len(sentence)+1 > config.MaxChunkSize && currentChunk.Len() > 0 {
			// Save current chunk
			chunkContent := strings.TrimSpace(currentChunk.String())
			if chunkContent != "" {
				chunk := models.TextChunk{
					ID:         generateChunkID(),
					Content:    chunkContent,
					StartPos:   currentStart,
					EndPos:     currentStart + currentChunk.Len(),
					ChunkIndex: chunkIndex,
					CreatedAt:  time.Now(),
				}
				chunks = append(chunks, chunk)
			}

			// Start new chunk with overlap
			currentStart = c.calculateOverlapPosition(content, currentStart, currentChunk.Len(), config.ChunkOverlap)
			currentChunk.Reset()

			// Add overlap content if applicable
			if config.ChunkOverlap > 0 {
				overlapContent := c.getOverlapContent(sentences, len(chunks), config.ChunkOverlap)
				if overlapContent != "" {
					currentChunk.WriteString(overlapContent)
					currentChunk.WriteString(" ")
				}
			}

			chunkIndex++
		}

		// Add sentence to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString(" ")
		}
		currentChunk.WriteString(sentence)

		if currentStart == 0 {
			currentStart = sentenceStart
		}

		sentenceStart += len(sentence) + 1 // +1 for space
	}

	// Add remaining chunk
	if currentChunk.Len() > 0 {
		chunkContent := strings.TrimSpace(currentChunk.String())
		if chunkContent != "" {
			chunk := models.TextChunk{
				ID:         generateChunkID(),
				Content:    chunkContent,
				StartPos:   currentStart,
				EndPos:     currentStart + currentChunk.Len(),
				ChunkIndex: chunkIndex,
				CreatedAt:  time.Now(),
			}
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// chunkByParagraph chunks text by paragraphs
func (c *SimpleTextChunker) chunkByParagraph(content string, config models.ChunkingConfig) []models.TextChunk {
	paragraphs := c.splitIntoParagraphs(content)
	var chunks []models.TextChunk
	var currentChunk strings.Builder
	currentStart := 0
	chunkIndex := 0
	paragraphStart := 0

	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		// Check if adding this paragraph would exceed max chunk size
		if currentChunk.Len()+len(paragraph)+2 > config.MaxChunkSize && currentChunk.Len() > 0 {
			// Save current chunk
			chunkContent := strings.TrimSpace(currentChunk.String())
			if chunkContent != "" {
				chunk := models.TextChunk{
					ID:         generateChunkID(),
					Content:    chunkContent,
					StartPos:   currentStart,
					EndPos:     currentStart + currentChunk.Len(),
					ChunkIndex: chunkIndex,
					CreatedAt:  time.Now(),
				}
				chunks = append(chunks, chunk)
			}

			// Start new chunk with overlap
			currentStart = c.calculateOverlapPosition(content, currentStart, currentChunk.Len(), config.ChunkOverlap)
			currentChunk.Reset()

			// Add overlap content if applicable
			if config.ChunkOverlap > 0 {
				overlapContent := c.getParagraphOverlapContent(paragraphs, len(chunks), config.ChunkOverlap)
				if overlapContent != "" {
					currentChunk.WriteString(overlapContent)
					currentChunk.WriteString("\n\n")
				}
			}

			chunkIndex++
		}

		// Add paragraph to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(paragraph)

		if currentStart == 0 {
			currentStart = paragraphStart
		}

		paragraphStart += len(paragraph) + 2 // +2 for paragraph separation
	}

	// Add remaining chunk
	if currentChunk.Len() > 0 {
		chunkContent := strings.TrimSpace(currentChunk.String())
		if chunkContent != "" {
			chunk := models.TextChunk{
				ID:         generateChunkID(),
				Content:    chunkContent,
				StartPos:   currentStart,
				EndPos:     currentStart + currentChunk.Len(),
				ChunkIndex: chunkIndex,
				CreatedAt:  time.Now(),
			}
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// findWordBoundary finds the nearest word boundary before the given position
func (c *SimpleTextChunker) findWordBoundary(runes []rune, start, maxEnd int) int {
	// Look backwards from maxEnd to find a word boundary
	for i := maxEnd - 1; i > start; i-- {
		if i < len(runes) && (runes[i] == ' ' || runes[i] == '\n' || runes[i] == '\t' || runes[i] == '.') {
			return i + 1
		}
	}
	// If no word boundary found, use maxEnd
	return maxEnd
}

// splitIntoSentences splits text into sentences using simple heuristics
func (c *SimpleTextChunker) splitIntoSentences(content string) []string {
	// Simple sentence splitting (can be improved with better NLP)
	sentences := strings.Split(content, ". ")

	// Clean up sentences
	for i := range sentences {
		sentences[i] = strings.TrimSpace(sentences[i])
		// Add period back if not the last sentence and doesn't end with punctuation
		if i < len(sentences)-1 && !strings.HasSuffix(sentences[i], ".") &&
		   !strings.HasSuffix(sentences[i], "!") && !strings.HasSuffix(sentences[i], "?") {
			sentences[i] += "."
		}
	}

	// Remove empty sentences
	var cleaned []string
	for _, sentence := range sentences {
		if strings.TrimSpace(sentence) != "" {
			cleaned = append(cleaned, sentence)
		}
	}

	return cleaned
}

// splitIntoParagraphs splits text into paragraphs
func (c *SimpleTextChunker) splitIntoParagraphs(content string) []string {
	// Split by double newline for paragraphs
	paragraphs := strings.Split(content, "\n\n")

	// Clean up paragraphs
	var cleaned []string
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph != "" {
			cleaned = append(cleaned, paragraph)
		}
	}

	return cleaned
}

// calculateOverlapPosition calculates the starting position for overlap
func (c *SimpleTextChunker) calculateOverlapPosition(content string, currentStart, chunkLen, overlap int) int {
	if overlap <= 0 {
		return currentStart + chunkLen
	}

	overlapStart := currentStart + chunkLen - overlap
	if overlapStart < currentStart {
		overlapStart = currentStart
	}

	return overlapStart
}

// getOverlapContent gets overlap content from previous sentences
func (c *SimpleTextChunker) getOverlapContent(sentences []string, chunkIndex, overlapSize int) string {
	if chunkIndex == 0 || overlapSize <= 0 {
		return ""
	}

	// Get the last few characters from previous chunk as overlap
	var overlap strings.Builder
	currentSize := 0

	// Work backwards from the end of previous sentences
	for i := len(sentences) - 1; i >= 0 && currentSize < overlapSize; i-- {
		sentence := sentences[i]
		if currentSize + len(sentence) <= overlapSize {
			if overlap.Len() > 0 {
				overlap.WriteString(" ")
			}
			overlap.WriteString(sentence)
			currentSize += len(sentence) + 1
		} else {
			// Take partial sentence
			remaining := overlapSize - currentSize
			if remaining > 0 {
				partial := sentence[len(sentence)-remaining:]
				if overlap.Len() > 0 {
					overlap.WriteString(" ")
				}
				overlap.WriteString(partial)
			}
			break
		}
	}

	return overlap.String()
}

// getParagraphOverlapContent gets overlap content from previous paragraphs
func (c *SimpleTextChunker) getParagraphOverlapContent(paragraphs []string, chunkIndex, overlapSize int) string {
	if chunkIndex == 0 || overlapSize <= 0 {
		return ""
	}

	// Similar logic as sentence overlap but for paragraphs
	var overlap strings.Builder
	currentSize := 0

	for i := len(paragraphs) - 1; i >= 0 && currentSize < overlapSize; i-- {
		paragraph := paragraphs[i]
		if currentSize + len(paragraph) <= overlapSize {
			if overlap.Len() > 0 {
				overlap.WriteString("\n\n")
			}
			overlap.WriteString(paragraph)
			currentSize += len(paragraph) + 2
		} else {
			remaining := overlapSize - currentSize
			if remaining > 0 {
				partial := paragraph[len(paragraph)-remaining:]
				if overlap.Len() > 0 {
					overlap.WriteString("\n\n")
				}
				overlap.WriteString(partial)
			}
			break
		}
	}

	return overlap.String()
}

// generateChunkID generates a random ID for chunks
func generateChunkID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "chunk_" + hex.EncodeToString(bytes)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// AdaptiveChunker implements more sophisticated chunking strategies
type AdaptiveChunker struct {
	config models.ChunkingConfig
}

// NewAdaptiveChunker creates a new adaptive text chunker
func NewAdaptiveChunker(config models.ChunkingConfig) *AdaptiveChunker {
	return &AdaptiveChunker{
		config: config,
	}
}

// ChunkText implements adaptive chunking based on content structure
func (ac *AdaptiveChunker) ChunkText(content string, config models.ChunkingConfig) []models.TextChunk {
	// Analyze content structure
	structure := ac.analyzeContentStructure(content)

	// Choose best chunking strategy based on analysis
	switch structure.Type {
	case "code":
		return ac.chunkCode(content, config)
	case "structured":
		return ac.chunkStructured(content, config)
	case "narrative":
		return ac.chunkNarrative(content, config)
	default:
		// Fall back to simple chunker
		simple := NewSimpleTextChunker()
		return simple.ChunkText(content, config)
	}
}

// ChunkDocument chunks a document using adaptive strategy
func (ac *AdaptiveChunker) ChunkDocument(doc *models.Document, config models.ChunkingConfig) []models.TextChunk {
	chunks := ac.ChunkText(doc.Content, config)

	// Set document ID for all chunks
	for i := range chunks {
		chunks[i].DocumentID = doc.ID
	}

	return chunks
}

// ValidateConfig validates configuration for adaptive chunker
func (ac *AdaptiveChunker) ValidateConfig(config models.ChunkingConfig) error {
	simple := NewSimpleTextChunker()
	return simple.ValidateConfig(config)
}

// ContentStructure represents the analyzed structure of content
type ContentStructure struct {
	Type           string  // "code", "structured", "narrative", "unknown"
	Confidence     float64 // 0.0 to 1.0
	HasHeaders     bool
	HasLists       bool
	HasCode        bool
	LineBreakRatio float64
	AverageLineLength float64
}

// analyzeContentStructure analyzes the structure of content to determine best chunking strategy
func (ac *AdaptiveChunker) analyzeContentStructure(content string) *ContentStructure {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)
	totalChars := len(content)

	structure := &ContentStructure{}

	if totalLines == 0 {
		structure.Type = "unknown"
		return structure
	}

	// Calculate metrics
	structure.LineBreakRatio = float64(totalLines) / float64(totalChars)
	structure.AverageLineLength = float64(totalChars) / float64(totalLines)

	// Detect patterns
	headerCount := 0
	listCount := 0
	codeIndicators := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Header detection (markdown-style)
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "##") {
			headerCount++
		}

		// List detection
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") ||
		   strings.HasPrefix(line, "+ ") || strings.Contains(line, ". ") {
			listCount++
		}

		// Code indicators
		if strings.Contains(line, "{") || strings.Contains(line, "}") ||
		   strings.Contains(line, "function") || strings.Contains(line, "class") ||
		   strings.Contains(line, "import") || strings.Contains(line, "def ") {
			codeIndicators++
		}
	}

	structure.HasHeaders = headerCount > 0
	structure.HasLists = listCount > 0
	structure.HasCode = codeIndicators > totalLines/10 // More than 10% code indicators

	// Determine content type
	if structure.HasCode {
		structure.Type = "code"
		structure.Confidence = float64(codeIndicators) / float64(totalLines)
	} else if structure.HasHeaders || structure.HasLists {
		structure.Type = "structured"
		structure.Confidence = float64(headerCount+listCount) / float64(totalLines)
	} else if structure.AverageLineLength > 50 {
		structure.Type = "narrative"
		structure.Confidence = 0.7 // Moderate confidence for narrative
	} else {
		structure.Type = "unknown"
		structure.Confidence = 0.0
	}

	return structure
}

// chunkCode chunks code content preserving function/class boundaries
func (ac *AdaptiveChunker) chunkCode(content string, config models.ChunkingConfig) []models.TextChunk {
	// TODO: Implement sophisticated code chunking
	// For now, fall back to simple chunking
	simple := NewSimpleTextChunker()
	return simple.ChunkText(content, config)
}

// chunkStructured chunks structured content preserving section boundaries
func (ac *AdaptiveChunker) chunkStructured(content string, config models.ChunkingConfig) []models.TextChunk {
	// TODO: Implement structured content chunking
	// For now, fall back to paragraph chunking
	simple := NewSimpleTextChunker()
	config.Strategy = models.ChunkingStrategyParagraph
	return simple.ChunkText(content, config)
}

// chunkNarrative chunks narrative content optimizing for readability
func (ac *AdaptiveChunker) chunkNarrative(content string, config models.ChunkingConfig) []models.TextChunk {
	// For narrative content, sentence-based chunking often works best
	simple := NewSimpleTextChunker()
	config.Strategy = models.ChunkingStrategySentence
	return simple.ChunkText(content, config)
}