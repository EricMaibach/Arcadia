package pdf

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// OCRHandler handles OCR processing using ocrmypdf
type OCRHandler struct {
	config *PDFConfig
	logger interfaces.Logger
}

// NewOCRHandler creates a new OCR handler
func NewOCRHandler(config *PDFConfig) *OCRHandler {
	return &OCRHandler{
		config: config,
	}
}

// WithLogger adds logging to the OCR handler
func (o *OCRHandler) WithLogger(logger interfaces.Logger) *OCRHandler {
	o.logger = logger
	return o
}

// ProcessWithOCR processes a PDF file using OCR
func (o *OCRHandler) ProcessWithOCR(ctx context.Context, filePath string) (*OCRResult, error) {
	if o.logger != nil {
		o.logger.Debug(ctx, "Starting OCR processing", "file_path", filePath)
	}

	startTime := time.Now()

	// Check if ocrmypdf is available
	if _, err := exec.LookPath(o.config.OCRMyPDFPath); err != nil {
		return nil, &PDFError{
			Code:    ErrToolNotFound,
			Message: "ocrmypdf tool not found",
			Cause:   err,
			Context: map[string]interface{}{
				"tool_path": o.config.OCRMyPDFPath,
			},
		}
	}

	// Create temporary file for OCR output
	tempFile, err := o.createTempFile(filePath)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tempFile)

	// Run OCR processing
	if err := o.runOCRMyPDF(ctx, filePath, tempFile); err != nil {
		return nil, err
	}

	// Extract text from OCR-processed PDF
	text, err := o.extractTextFromOCRPDF(ctx, tempFile)
	if err != nil {
		return nil, err
	}

	// Analyze OCR quality
	quality := o.analyzeOCRQuality(text)

	duration := time.Since(startTime)

	result := &OCRResult{
		Text:         text,
		Quality:      quality,
		Duration:     duration,
		Languages:    o.config.OCRLanguages,
		QualityLevel: o.config.OCRQualityLevel,
		ToolVersion:  o.getOCRMyPDFVersion(),
		Metadata: map[string]interface{}{
			"processor":       "ocrmypdf",
			"file_path":       filePath,
			"processing_time": duration.Milliseconds(),
			"languages":       o.config.OCRLanguages,
			"quality_level":   o.config.OCRQualityLevel,
			"quality_score":   quality.Score,
			"word_count":      quality.WordCount,
			"char_count":      quality.CharCount,
		},
	}

	if o.logger != nil {
		o.logger.Info(ctx, "OCR processing completed",
			"file_path", filePath,
			"duration_ms", duration.Milliseconds(),
			"quality_score", quality.Score,
			"word_count", quality.WordCount,
			"char_count", len(text),
			"languages", o.config.OCRLanguages)
	}

	return result, nil
}

// runOCRMyPDF executes ocrmypdf command
func (o *OCRHandler) runOCRMyPDF(ctx context.Context, inputPath, outputPath string) error {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, o.config.ProcessingTimeout)
	defer cancel()

	// Build ocrmypdf command
	args := []string{
		"--force-ocr",                                             // Force OCR even if text exists
		"--optimize", fmt.Sprintf("%d", o.config.OCRQualityLevel), // Optimization level
		"--output-type", "pdf", // Output PDF
		"--pdf-renderer", "hocr", // Use hOCR renderer
		"--clean",             // Clean up temporary files
		"--rotate-pages",      // Auto-rotate pages
		"--deskew",            // Deskew pages
		"--remove-background", // Remove background
	}

	// Add language specification
	if len(o.config.OCRLanguages) > 0 {
		langStr := strings.Join(o.config.OCRLanguages, "+")
		args = append(args, "--language", langStr)
	}

	// Add input and output paths
	args = append(args, inputPath, outputPath)

	cmd := exec.CommandContext(timeoutCtx, o.config.OCRMyPDFPath, args...)

	// Capture stderr for error analysis
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if o.logger != nil {
		o.logger.Debug(ctx, "Executing ocrmypdf command",
			"command", o.config.OCRMyPDFPath,
			"args", args,
			"input_path", inputPath,
			"output_path", outputPath)
	}

	// Execute command
	err := cmd.Run()
	if err != nil {
		stderrStr := stderr.String()

		// Check for specific error conditions
		if strings.Contains(stderrStr, "encrypted") ||
			strings.Contains(stderrStr, "password") {
			return &PDFError{
				Code:    ErrPDFPasswordProtected,
				Message: "PDF is password protected",
				Cause:   err,
				Context: map[string]interface{}{
					"stderr":    stderrStr,
					"file_path": inputPath,
				},
			}
		}

		if strings.Contains(stderrStr, "Invalid PDF") ||
			strings.Contains(stderrStr, "corrupted") ||
			strings.Contains(stderrStr, "damaged") {
			return &PDFError{
				Code:    ErrPDFCorrupted,
				Message: "PDF file appears to be corrupted",
				Cause:   err,
				Context: map[string]interface{}{
					"stderr":    stderrStr,
					"file_path": inputPath,
				},
			}
		}

		if strings.Contains(stderrStr, "No images found") ||
			strings.Contains(stderrStr, "already has text") {
			return &PDFError{
				Code:    ErrOCRFailed,
				Message: "OCR processing not needed or failed",
				Cause:   err,
				Context: map[string]interface{}{
					"stderr": stderrStr,
					"reason": "no_images_or_has_text",
				},
			}
		}

		return &PDFError{
			Code:    ErrToolExecution,
			Message: "ocrmypdf execution failed",
			Cause:   err,
			Context: map[string]interface{}{
				"stderr":  stderrStr,
				"command": strings.Join(append([]string{o.config.OCRMyPDFPath}, args...), " "),
			},
		}
	}

	return nil
}

// extractTextFromOCRPDF extracts text from the OCR-processed PDF
func (o *OCRHandler) extractTextFromOCRPDF(ctx context.Context, ocrPDFPath string) (string, error) {
	// Use pdftotext to extract text from the OCR-processed PDF
	extractor := NewTextExtractor(o.config)
	if o.logger != nil {
		extractor = extractor.WithLogger(o.logger)
	}

	result, err := extractor.ExtractText(ctx, ocrPDFPath)
	if err != nil {
		return "", &PDFError{
			Code:    ErrTextExtractionFailed,
			Message: "Failed to extract text from OCR-processed PDF",
			Cause:   err,
			Context: map[string]interface{}{
				"ocr_pdf_path": ocrPDFPath,
			},
		}
	}

	return result.Text, nil
}

// createTempFile creates a temporary file for OCR output
func (o *OCRHandler) createTempFile(originalPath string) (string, error) {
	// Create temp file with PDF extension
	baseName := filepath.Base(originalPath)
	nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	tempFile, err := ioutil.TempFile("", fmt.Sprintf("ocr_%s_*.pdf", nameWithoutExt))
	if err != nil {
		return "", models.NewDocumentErrorWithCause(models.ErrProcessingFailed,
			"Cannot create temporary file for OCR", err)
	}

	tempPath := tempFile.Name()
	tempFile.Close() // Close immediately, ocrmypdf will write to it

	return tempPath, nil
}

// analyzeOCRQuality analyzes the quality of OCR-extracted text
func (o *OCRHandler) analyzeOCRQuality(text string) *OCRQuality {
	if text == "" {
		return &OCRQuality{
			Score:      0.0,
			WordCount:  0,
			CharCount:  0,
			LineCount:  0,
			HasContent: false,
			Confidence: 0.0,
			Languages:  o.config.OCRLanguages,
		}
	}

	lines := strings.Split(text, "\n")
	words := strings.Fields(text)
	chars := len(text)

	// Count valid words
	validWords := 0
	totalWordLength := 0
	for _, word := range words {
		if o.isValidOCRWord(word) {
			validWords++
			totalWordLength += len(word)
		}
	}

	// Calculate average word length
	avgWordLength := 0.0
	if validWords > 0 {
		avgWordLength = float64(totalWordLength) / float64(validWords)
	}

	// Calculate quality score
	score := o.calculateOCRQualityScore(validWords, len(words), avgWordLength, text)

	// Calculate confidence
	confidence := o.calculateOCRConfidence(score, validWords, chars)

	return &OCRQuality{
		Score:         score,
		WordCount:     len(words),
		ValidWords:    validWords,
		CharCount:     chars,
		LineCount:     len(lines),
		AvgWordLength: avgWordLength,
		HasContent:    validWords > 0,
		Confidence:    confidence,
		Languages:     o.config.OCRLanguages,
	}
}

// isValidOCRWord checks if a word appears to be valid OCR output
func (o *OCRHandler) isValidOCRWord(word string) bool {
	if len(word) == 0 {
		return false
	}

	// Remove common punctuation
	cleaned := strings.Trim(word, ".,!?;:()[]{}\"'")
	if len(cleaned) == 0 {
		return false
	}

	// OCR words should be reasonable length
	if len(cleaned) > 30 {
		return false
	}

	// Check for obvious OCR errors (lots of special characters)
	specialCount := 0
	for _, r := range cleaned {
		if !('a' <= r && r <= 'z') && !('A' <= r && r <= 'Z') && !('0' <= r && r <= '9') {
			specialCount++
		}
	}

	// Too many special characters indicates OCR error
	if len(cleaned) > 0 && float64(specialCount)/float64(len(cleaned)) > 0.4 {
		return false
	}

	return len(cleaned) >= 1
}

// calculateOCRQualityScore calculates quality score for OCR text
func (o *OCRHandler) calculateOCRQualityScore(validWords, totalWords int, avgWordLength float64, text string) float64 {
	if totalWords == 0 {
		return 0.0
	}

	// Base score from valid word ratio
	validRatio := float64(validWords) / float64(totalWords)
	score := validRatio * 0.8 // OCR typically has lower base quality

	// Adjust for word length (OCR can produce fragmented words)
	if avgWordLength >= 2 && avgWordLength <= 10 {
		score *= 1.1
	} else if avgWordLength < 1.5 {
		score *= 0.7 // Penalty for very short words (fragmentation)
	}

	// Check for common OCR patterns that indicate quality
	if o.hasGoodOCRPatterns(text) {
		score *= 1.1
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

// hasGoodOCRPatterns checks for patterns that indicate good OCR quality
func (o *OCRHandler) hasGoodOCRPatterns(text string) bool {
	// Look for complete sentences (capital letter + period)
	lines := strings.Split(text, "\n")
	completeLines := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 10 &&
			len(line) < 200 &&
			strings.HasSuffix(line, ".") &&
			len(line) > 0 &&
			(line[0] >= 'A' && line[0] <= 'Z') {
			completeLines++
		}
	}

	// If we have several complete-looking lines, OCR quality is probably good
	return completeLines >= 3
}

// calculateOCRConfidence calculates confidence in OCR results
func (o *OCRHandler) calculateOCRConfidence(score float64, validWords, chars int) float64 {
	confidence := score * 0.9 // OCR confidence is inherently lower

	// Boost confidence for substantial content
	if validWords > 30 && chars > 300 {
		confidence *= 1.1
	} else if validWords < 5 || chars < 50 {
		confidence *= 0.5
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

// getOCRMyPDFVersion gets the version of ocrmypdf tool
func (o *OCRHandler) getOCRMyPDFVersion() string {
	cmd := exec.Command(o.config.OCRMyPDFPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "unknown"
	}

	versionStr := string(output)
	return strings.TrimSpace(versionStr)
}

// CanProcessOCR checks if OCR processing is possible
func (o *OCRHandler) CanProcessOCR() bool {
	_, err := exec.LookPath(o.config.OCRMyPDFPath)
	return err == nil
}

// OCRResult holds the results of OCR processing
type OCRResult struct {
	Text         string                 `json:"text"`
	Quality      *OCRQuality            `json:"quality"`
	Duration     time.Duration          `json:"duration"`
	Languages    []string               `json:"languages"`
	QualityLevel int                    `json:"quality_level"`
	ToolVersion  string                 `json:"tool_version"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// OCRQuality holds quality metrics for OCR text
type OCRQuality struct {
	Score         float64  `json:"score"`           // Overall quality score (0-1)
	WordCount     int      `json:"word_count"`      // Total word count
	ValidWords    int      `json:"valid_words"`     // Count of valid words
	CharCount     int      `json:"char_count"`      // Total character count
	LineCount     int      `json:"line_count"`      // Total line count
	AvgWordLength float64  `json:"avg_word_length"` // Average word length
	HasContent    bool     `json:"has_content"`     // Whether text has meaningful content
	Confidence    float64  `json:"confidence"`      // Confidence in OCR result (0-1)
	Languages     []string `json:"languages"`       // OCR languages used
}

// IsGoodQuality returns true if the OCR quality is acceptable
func (oq *OCRQuality) IsGoodQuality(threshold float64) bool {
	return oq.Score >= threshold && oq.HasContent && oq.ValidWords > 5
}

// GetQualityLevel returns a human-readable quality level
func (oq *OCRQuality) GetQualityLevel() string {
	switch {
	case oq.Score >= 0.7:
		return "good"
	case oq.Score >= 0.5:
		return "fair"
	case oq.Score >= 0.3:
		return "poor"
	default:
		return "very_poor"
	}
}

// IsBetterThan compares this OCR quality with another quality result
func (oq *OCRQuality) IsBetterThan(other *TextQuality) bool {
	if other == nil {
		return oq.HasContent
	}

	// Compare scores, but give slight preference to OCR if original quality was poor
	if other.Score < 0.3 {
		return oq.Score > other.Score*0.8 // Lower threshold for OCR
	}

	return oq.Score > other.Score
}
