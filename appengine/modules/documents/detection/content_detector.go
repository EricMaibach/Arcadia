package detection

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"arcadia/modules/documents/models"
	"arcadia/modules/documents/processors/base"
)

// ContentDetector detects document type based on file content analysis
type ContentDetector struct {
	priority int
}

// NewContentDetector creates a new content-based detector
func NewContentDetector() *ContentDetector {
	return &ContentDetector{
		priority: 60, // Lower priority, used as fallback
	}
}

// DetectType detects document type based on file content
func (cd *ContentDetector) DetectType(filePath string) (*base.DocumentType, error) {
	if filePath == "" {
		return nil, models.NewDocumentError(models.ErrInvalidInput, "file path cannot be empty").
			WithFilePath(filePath)
	}

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil, models.NewDocumentErrorWithCause(models.ErrFileNotFound, "file does not exist", err).
			WithFilePath(filePath)
	}

	// Skip empty files
	if fileInfo.Size() == 0 {
		return nil, nil
	}

	// Read file content for analysis
	content, err := cd.readFileContent(filePath, 8192) // Read first 8KB for analysis
	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrFileReadError,
			"failed to read file content", err).WithFilePath(filePath)
	}

	// Detect document type based on content
	processorType, confidence := cd.analyzeContent(content, filePath)
	if processorType == "" {
		return nil, nil // Could not determine type
	}

	// Extract file extension for metadata
	ext := strings.ToLower(filepath.Ext(filePath))
	ext = strings.TrimPrefix(ext, ".")

	// Return document type
	return &base.DocumentType{
		Type:       processorType,
		Extension:  ext,
		Confidence: confidence,
		Metadata: map[string]interface{}{
			"detector":      "content",
			"detected_by":   "ContentDetector",
			"original_path": filePath,
			"content_type":  cd.getContentTypeDescription(processorType),
			"file_size":     fileInfo.Size(),
		},
	}, nil
}

// GetConfidence returns confidence level for a given file path
func (cd *ContentDetector) GetConfidence(filePath string) float64 {
	content, err := cd.readFileContent(filePath, 4096) // Smaller read for confidence check
	if err != nil {
		return 0.0
	}

	_, confidence := cd.analyzeContent(content, filePath)
	return confidence
}

// GetPriority returns the priority of this detector
func (cd *ContentDetector) GetPriority() int {
	return cd.priority
}

// readFileContent reads up to maxBytes from the file
func (cd *ContentDetector) readFileContent(filePath string, maxBytes int) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buffer := make([]byte, maxBytes)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return buffer[:n], nil
}

// analyzeContent analyzes file content to determine document type
func (cd *ContentDetector) analyzeContent(content []byte, filePath string) (base.ProcessorType, float64) {
	if len(content) == 0 {
		return "", 0.0
	}

	// Check for PDF signature
	if cd.isPDF(content) {
		return base.ProcessorTypePDF, 0.9
	}

	// Check if content is binary
	if cd.isBinary(content) {
		// Binary files are not supported, but we can try to detect PDF
		return "", 0.0
	}

	// Content appears to be text, analyze further
	textContent := string(content)

	// Check for Markdown indicators
	if cd.isMarkdown(textContent) {
		return base.ProcessorTypeMarkdown, 0.7
	}

	// Check for structured text formats
	if cd.isStructuredText(textContent) {
		return base.ProcessorTypeText, 0.6
	}

	// Default to plain text for readable content
	if cd.isPlainText(content) {
		return base.ProcessorTypeText, 0.5
	}

	return "", 0.0
}

// isPDF checks for PDF file signature
func (cd *ContentDetector) isPDF(content []byte) bool {
	// PDF files start with "%PDF-"
	return len(content) >= 5 && bytes.HasPrefix(content, []byte("%PDF-"))
}

// isBinary determines if content is binary
func (cd *ContentDetector) isBinary(content []byte) bool {
	// Check for null bytes (common in binary files)
	if bytes.Contains(content, []byte{0}) {
		return true
	}

	// Check if content is valid UTF-8 and contains mostly printable characters
	if !utf8.Valid(content) {
		return true
	}

	// Count non-printable characters
	nonPrintable := 0
	for _, b := range content {
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			nonPrintable++
		}
	}

	// If more than 30% is non-printable, consider it binary
	return float64(nonPrintable)/float64(len(content)) > 0.3
}

// isMarkdown checks for Markdown indicators
func (cd *ContentDetector) isMarkdown(content string) bool {
	lines := strings.Split(content, "\n")
	markdownIndicators := 0
	totalLines := len(lines)

	if totalLines == 0 {
		return false
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for common Markdown patterns
		if cd.isMarkdownLine(line) {
			markdownIndicators++
		}
	}

	// If more than 20% of non-empty lines contain Markdown indicators
	nonEmptyLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
	}

	if nonEmptyLines == 0 {
		return false
	}

	return float64(markdownIndicators)/float64(nonEmptyLines) > 0.2
}

// isMarkdownLine checks if a line contains Markdown indicators
func (cd *ContentDetector) isMarkdownLine(line string) bool {
	line = strings.TrimSpace(line)

	// Headers
	if strings.HasPrefix(line, "#") {
		return true
	}

	// Bold/italic
	if strings.Contains(line, "**") || strings.Contains(line, "__") ||
		strings.Contains(line, "*") || strings.Contains(line, "_") {
		return true
	}

	// Links
	if strings.Contains(line, "](") || strings.Contains(line, "](") {
		return true
	}

	// Code blocks
	if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
		return true
	}

	// Lists
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") ||
		strings.HasPrefix(line, "+ ") {
		return true
	}

	// Numbered lists
	if len(line) > 2 && unicode.IsDigit(rune(line[0])) && line[1] == '.' && line[2] == ' ' {
		return true
	}

	// Blockquotes
	if strings.HasPrefix(line, "> ") {
		return true
	}

	return false
}

// isStructuredText checks for structured text formats (JSON, XML, YAML, etc.)
func (cd *ContentDetector) isStructuredText(content string) bool {
	content = strings.TrimSpace(content)

	// JSON
	if (strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}")) ||
		(strings.HasPrefix(content, "[") && strings.HasSuffix(content, "]")) {
		return true
	}

	// XML
	if strings.HasPrefix(content, "<?xml") || strings.HasPrefix(content, "<") {
		return true
	}

	// YAML
	if cd.looksLikeYAML(content) {
		return true
	}

	// CSV (simple check)
	if cd.looksLikeCSV(content) {
		return true
	}

	return false
}

// looksLikeYAML checks if content looks like YAML
func (cd *ContentDetector) looksLikeYAML(content string) bool {
	lines := strings.Split(content, "\n")
	yamlLines := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// YAML key: value pattern
		if strings.Contains(line, ":") && !strings.HasPrefix(line, "http") {
			yamlLines++
		}

		// YAML list items
		if strings.HasPrefix(line, "- ") {
			yamlLines++
		}
	}

	return yamlLines > 0 && len(lines) > 0 && float64(yamlLines)/float64(len(lines)) > 0.3
}

// looksLikeCSV checks if content looks like CSV
func (cd *ContentDetector) looksLikeCSV(content string) bool {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return false
	}

	firstLine := strings.TrimSpace(lines[0])
	if firstLine == "" {
		return false
	}

	// Count commas in first line
	commas := strings.Count(firstLine, ",")
	if commas == 0 {
		return false
	}

	// Check if subsequent lines have similar comma count
	similarLines := 0
	for i := 1; i < min(len(lines), 10); i++ { // Check first 10 lines
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		lineCommas := strings.Count(line, ",")
		if abs(lineCommas-commas) <= 1 { // Allow for slight variation
			similarLines++
		}
	}

	return similarLines > 0
}

// isPlainText checks if content is readable plain text
func (cd *ContentDetector) isPlainText(content []byte) bool {
	// Must be valid UTF-8
	if !utf8.Valid(content) {
		return false
	}

	// Check for reasonable mix of characters
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		if lineCount > 100 { // Don't analyze too many lines
			break
		}
	}

	return lineCount > 0
}

// getContentTypeDescription returns a human-readable description of the content type
func (cd *ContentDetector) getContentTypeDescription(processorType base.ProcessorType) string {
	switch processorType {
	case base.ProcessorTypeText:
		return "Plain text or structured text"
	case base.ProcessorTypeMarkdown:
		return "Markdown document"
	case base.ProcessorTypePDF:
		return "PDF document"
	default:
		return "Unknown content type"
	}
}

// Helper functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}