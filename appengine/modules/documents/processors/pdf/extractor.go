package pdf

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// TextExtractor handles text extraction from PDF files using pdftotext
type TextExtractor struct {
	config *PDFConfig
	logger interfaces.Logger
}

// NewTextExtractor creates a new text extractor
func NewTextExtractor(config *PDFConfig) *TextExtractor {
	return &TextExtractor{
		config: config,
	}
}

// WithLogger adds logging to the extractor
func (e *TextExtractor) WithLogger(logger interfaces.Logger) *TextExtractor {
	e.logger = logger
	return e
}

// ExtractText extracts text from a PDF file using pdftotext
func (e *TextExtractor) ExtractText(ctx context.Context, filePath string) (*TextExtractionResult, error) {
	if e.logger != nil {
		e.logger.Debug(ctx, "Starting text extraction", "file_path", filePath)
	}

	startTime := time.Now()

	// Check if pdftotext is available
	if _, err := exec.LookPath(e.config.PDFToTextPath); err != nil {
		return nil, &PDFError{
			Code:    ErrToolNotFound,
			Message: "pdftotext tool not found",
			Cause:   err,
			Context: map[string]interface{}{
				"tool_path": e.config.PDFToTextPath,
			},
		}
	}

	// Create temporary file for output
	tempFile, err := e.createTempFile()
	if err != nil {
		return nil, err
	}
	defer os.Remove(tempFile)

	// Extract text using pdftotext
	text, err := e.runPDFToText(ctx, filePath, tempFile)
	if err != nil {
		return nil, err
	}

	// Analyze text quality
	quality := e.analyzeTextQuality(text)

	duration := time.Since(startTime)

	result := &TextExtractionResult{
		Text:        text,
		Quality:     quality,
		Method:      "pdftotext",
		Duration:    duration,
		ToolVersion: e.getPDFToTextVersion(),
		Metadata: map[string]interface{}{
			"extractor":       "pdftotext",
			"file_path":       filePath,
			"extraction_time": duration.Milliseconds(),
			"quality_score":   quality.Score,
			"word_count":      quality.WordCount,
			"char_count":      quality.CharCount,
		},
	}

	if e.logger != nil {
		e.logger.Info(ctx, "Text extraction completed",
			"file_path", filePath,
			"method", "pdftotext",
			"duration_ms", duration.Milliseconds(),
			"quality_score", quality.Score,
			"word_count", quality.WordCount,
			"char_count", len(text))
	}

	return result, nil
}

// runPDFToText executes pdftotext command
func (e *TextExtractor) runPDFToText(ctx context.Context, inputPath, outputPath string) (string, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, e.config.ProcessingTimeout)
	defer cancel()

	// Build pdftotext command
	args := []string{
		"-layout",       // Preserve layout
		"-enc", "UTF-8", // UTF-8 encoding
		"-eol", "unix", // Unix line endings
		inputPath,
		outputPath,
	}

	cmd := exec.CommandContext(timeoutCtx, e.config.PDFToTextPath, args...)

	// Capture stderr for error analysis
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if e.logger != nil {
		e.logger.Debug(ctx, "Executing pdftotext command",
			"command", e.config.PDFToTextPath,
			"args", args,
			"input_path", inputPath,
			"output_path", outputPath)
	}

	// Execute command
	err := cmd.Run()
	if err != nil {
		stderrStr := stderr.String()

		// Check for specific error conditions
		if strings.Contains(stderrStr, "Incorrect password") ||
			strings.Contains(stderrStr, "Couldn't open file") {
			return "", &PDFError{
				Code:    ErrPDFPasswordProtected,
				Message: "PDF is password protected or corrupted",
				Cause:   err,
				Context: map[string]interface{}{
					"stderr":    stderrStr,
					"file_path": inputPath,
				},
			}
		}

		if strings.Contains(stderrStr, "Couldn't read xref table") ||
			strings.Contains(stderrStr, "PDF file is damaged") {
			return "", &PDFError{
				Code:    ErrPDFCorrupted,
				Message: "PDF file appears to be corrupted",
				Cause:   err,
				Context: map[string]interface{}{
					"stderr":    stderrStr,
					"file_path": inputPath,
				},
			}
		}

		return "", &PDFError{
			Code:    ErrToolExecution,
			Message: "pdftotext execution failed",
			Cause:   err,
			Context: map[string]interface{}{
				"stderr":  stderrStr,
				"command": strings.Join(append([]string{e.config.PDFToTextPath}, args...), " "),
			},
		}
	}

	// Read extracted text
	textBytes, err := ioutil.ReadFile(outputPath)
	if err != nil {
		return "", models.NewDocumentErrorWithCause(models.ErrFileReadError,
			"Cannot read extracted text file", err)
	}

	return string(textBytes), nil
}

// createTempFile creates a temporary file for text output
func (e *TextExtractor) createTempFile() (string, error) {
	tempFile, err := ioutil.TempFile("", "pdf_extract_*.txt")
	if err != nil {
		return "", models.NewDocumentErrorWithCause(models.ErrProcessingFailed,
			"Cannot create temporary file", err)
	}

	tempPath := tempFile.Name()
	tempFile.Close() // Close immediately, pdftotext will write to it

	return tempPath, nil
}

// analyzeTextQuality analyzes the quality of extracted text
func (e *TextExtractor) analyzeTextQuality(text string) *TextQuality {
	if text == "" {
		return &TextQuality{
			Score:      0.0,
			WordCount:  0,
			CharCount:  0,
			LineCount:  0,
			IsGarbled:  true,
			HasContent: false,
			Confidence: 0.0,
		}
	}

	lines := strings.Split(text, "\n")
	words := strings.Fields(text)
	chars := len(text)

	// Count valid words (containing alphabetic characters)
	validWords := 0
	totalWordLength := 0
	for _, word := range words {
		if e.isValidWord(word) {
			validWords++
			totalWordLength += len(word)
		}
	}

	// Calculate quality metrics
	avgWordLength := 0.0
	if validWords > 0 {
		avgWordLength = float64(totalWordLength) / float64(validWords)
	}

	// Detect garbled text patterns
	isGarbled := e.detectGarbledText(text)

	// Calculate quality score
	score := e.calculateQualityScore(validWords, len(words), avgWordLength, isGarbled)

	// Determine confidence
	confidence := e.calculateConfidence(score, validWords, chars)

	return &TextQuality{
		Score:         score,
		WordCount:     len(words),
		ValidWords:    validWords,
		CharCount:     chars,
		LineCount:     len(lines),
		AvgWordLength: avgWordLength,
		IsGarbled:     isGarbled,
		HasContent:    validWords > 0,
		Confidence:    confidence,
	}
}

// isValidWord checks if a word contains alphabetic characters
func (e *TextExtractor) isValidWord(word string) bool {
	if len(word) == 0 {
		return false
	}

	// Remove common punctuation
	cleaned := strings.Trim(word, ".,!?;:()[]{}\"'")
	if len(cleaned) == 0 {
		return false
	}

	// Check if word contains at least one letter
	hasLetter := false
	for _, r := range cleaned {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}

	// Word should be reasonable length and contain letters
	return hasLetter && len(cleaned) >= 1 && len(cleaned) <= 50
}

// hasRepeatedCharacters checks for consecutive repeated letters
func (e *TextExtractor) hasRepeatedCharacters(text string, minLength int) bool {
	if len(text) < minLength {
		return false
	}

	consecutiveCount := 1
	var lastChar rune

	for _, char := range text {
		if unicode.IsLetter(char) {
			if char == lastChar {
				consecutiveCount++
				if consecutiveCount >= minLength {
					return true
				}
			} else {
				consecutiveCount = 1
			}
			lastChar = char
		} else {
			consecutiveCount = 1
			lastChar = 0
		}
	}
	return false
}

// detectGarbledText detects if text appears to be garbled (OCR artifacts, etc.)
func (e *TextExtractor) detectGarbledText(text string) bool {
	// Patterns that indicate garbled text
	garbledPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[^\w\s]{5,}`), // Long sequences of non-word characters
		regexp.MustCompile(`\w{20,}`),     // Very long words
		regexp.MustCompile(`[0-9]{10,}`),  // Very long numbers
	}

	for _, pattern := range garbledPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}

	// Check for repeated characters (5+ consecutive same letters)
	if e.hasRepeatedCharacters(text, 5) {
		return true
	}

	// Check ratio of special characters to total characters
	specialChars := 0
	totalChars := 0
	for _, r := range text {
		if !unicode.IsSpace(r) {
			totalChars++
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				specialChars++
			}
		}
	}

	if totalChars > 0 {
		specialRatio := float64(specialChars) / float64(totalChars)
		if specialRatio > 0.3 { // More than 30% special characters
			return true
		}
	}

	return false
}

// calculateQualityScore calculates overall text quality score
func (e *TextExtractor) calculateQualityScore(validWords, totalWords int, avgWordLength float64, isGarbled bool) float64 {
	if totalWords == 0 {
		return 0.0
	}

	// Base score from valid word ratio
	validRatio := float64(validWords) / float64(totalWords)
	score := validRatio

	// Adjust for average word length (optimal range: 3-8 characters)
	if avgWordLength >= 3 && avgWordLength <= 8 {
		score *= 1.1 // Boost for good word length
	} else if avgWordLength < 2 || avgWordLength > 15 {
		score *= 0.8 // Penalty for poor word length
	}

	// Heavy penalty for garbled text
	if isGarbled {
		score *= 0.3
	}

	// Ensure score stays within bounds
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// calculateConfidence calculates confidence in the extraction result
func (e *TextExtractor) calculateConfidence(score float64, validWords, chars int) float64 {
	confidence := score

	// Boost confidence for substantial content
	if validWords > 50 && chars > 500 {
		confidence *= 1.1
	} else if validWords < 10 || chars < 100 {
		confidence *= 0.7
	}

	// Ensure confidence stays within bounds
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// getPDFToTextVersion gets the version of pdftotext tool
func (e *TextExtractor) getPDFToTextVersion() string {
	cmd := exec.Command(e.config.PDFToTextPath, "-v")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "unknown"
	}

	// Extract version from output
	versionStr := string(output)
	lines := strings.Split(versionStr, "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}

	return "unknown"
}

// TextExtractionResult holds the results of text extraction
type TextExtractionResult struct {
	Text        string                 `json:"text"`
	Quality     *TextQuality           `json:"quality"`
	Method      string                 `json:"method"`
	Duration    time.Duration          `json:"duration"`
	ToolVersion string                 `json:"tool_version"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// TextQuality holds quality metrics for extracted text
type TextQuality struct {
	Score         float64 `json:"score"`           // Overall quality score (0-1)
	WordCount     int     `json:"word_count"`      // Total word count
	ValidWords    int     `json:"valid_words"`     // Count of valid words
	CharCount     int     `json:"char_count"`      // Total character count
	LineCount     int     `json:"line_count"`      // Total line count
	AvgWordLength float64 `json:"avg_word_length"` // Average word length
	IsGarbled     bool    `json:"is_garbled"`      // Whether text appears garbled
	HasContent    bool    `json:"has_content"`     // Whether text has meaningful content
	Confidence    float64 `json:"confidence"`      // Confidence in the result (0-1)
}

// IsGoodQuality returns true if the text quality meets the configured threshold
func (tq *TextQuality) IsGoodQuality(threshold float64) bool {
	return tq.Score >= threshold && tq.HasContent && !tq.IsGarbled
}

// ShouldUseOCR determines if OCR should be used based on text quality
func (tq *TextQuality) ShouldUseOCR(threshold float64) bool {
	return !tq.IsGoodQuality(threshold) || tq.WordCount < 10
}

// GetQualityLevel returns a human-readable quality level
func (tq *TextQuality) GetQualityLevel() string {
	switch {
	case tq.Score >= 0.8:
		return "excellent"
	case tq.Score >= 0.6:
		return "good"
	case tq.Score >= 0.4:
		return "fair"
	case tq.Score >= 0.2:
		return "poor"
	default:
		return "very_poor"
	}
}
