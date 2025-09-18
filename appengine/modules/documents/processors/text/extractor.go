package text

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"arcadia/modules/documents/models"
)

// TextExtractor handles extraction and validation of text content from files
type TextExtractor struct{}

// NewTextExtractor creates a new text extractor
func NewTextExtractor() *TextExtractor {
	return &TextExtractor{}
}

// ExtractFromFile reads and validates text content from a file
func (te *TextExtractor) ExtractFromFile(filePath string) (string, error) {
	// Validate file path
	if filePath == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", filePath)
	}

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// Get file info for size validation
	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to get file info for %s: %w", filePath, err)
	}

	// Check file size (100MB limit)
	const maxFileSize = 100 * 1024 * 1024
	if fileInfo.Size() > maxFileSize {
		return "", models.NewDocumentError(models.ErrDocumentTooBig,
			fmt.Sprintf("file size %d exceeds maximum size %d", fileInfo.Size(), maxFileSize))
	}

	// Read content
	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Convert to string
	contentStr := string(content)

	// Validate content
	if err := te.ValidateContent(contentStr); err != nil {
		return "", err
	}

	return contentStr, nil
}

// ValidateContent performs validation on text content
func (te *TextExtractor) ValidateContent(content string) error {
	// Check for empty content
	if len(content) == 0 {
		return models.NewDocumentError(models.ErrDocumentInvalid, "content is empty")
	}

	// Check for extremely large content
	if len(content) > 100*1024*1024 { // 100MB
		return models.NewDocumentError(models.ErrDocumentTooBig, "content exceeds maximum size")
	}

	// Validate UTF-8 encoding
	if !utf8.ValidString(content) {
		return models.NewDocumentError(models.ErrDocumentInvalid, "content is not valid UTF-8")
	}

	return nil
}

// IsTextFile determines if a file is a text file based on its extension
func (te *TextExtractor) IsTextFile(filePath string) bool {
	return isTextByExtension(filePath)
}

// GetContentType returns the MIME type for a file based on its extension
func (te *TextExtractor) GetContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	return getContentTypeByExtension(ext)
}

// EstimateWordCount estimates the number of words in the content
func (te *TextExtractor) EstimateWordCount(content string) int {
	if content == "" {
		return 0
	}

	words := strings.Fields(content)
	return len(words)
}

// Helper functions extracted from analyzer.go

func isTextByExtension(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	textExtensions := map[string]bool{
		".txt":  true,
		".md":   true,
		".json": true,
		".xml":  true,
		".html": true,
		".htm":  true,
		".css":  true,
		".js":   true,
		".ts":   true,
		".jsx":  true,
		".tsx":  true,
		".py":   true,
		".go":   true,
		".java": true,
		".c":    true,
		".cpp":  true,
		".h":    true,
		".hpp":  true,
		".sql":  true,
		".yaml": true,
		".yml":  true,
		".csv":  true,
		".log":  true,
		".ini":  true,
		".conf": true,
		".config": true,
		".properties": true,
		".sh":   true,
		".bash": true,
		".zsh":  true,
		".ps1":  true,
		".rb":   true,
		".php":  true,
		".pl":   true,
		".r":    true,
		".tex":  true,
		".ltx":  true,
		".rst":  true,
		".asciidoc": true,
		".adoc": true,
		".org":  true,
		".wiki": true,
	}

	return textExtensions[ext]
}

func getContentTypeByExtension(ext string) string {
	contentTypes := map[string]string{
		".txt":  "text/plain",
		".md":   "text/markdown",
		".json": "application/json",
		".xml":  "application/xml",
		".html": "text/html",
		".htm":  "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".ts":   "application/typescript",
		".jsx":  "text/jsx",
		".tsx":  "text/tsx",
		".py":   "text/x-python",
		".go":   "text/x-go",
		".java": "text/x-java",
		".c":    "text/x-c",
		".cpp":  "text/x-c++",
		".h":    "text/x-c",
		".hpp":  "text/x-c++",
		".sql":  "application/sql",
		".yaml": "application/yaml",
		".yml":  "application/yaml",
		".csv":  "text/csv",
		".log":  "text/plain",
		".ini":  "text/plain",
		".conf": "text/plain",
		".config": "text/plain",
		".properties": "text/plain",
		".sh":   "text/x-shellscript",
		".bash": "text/x-shellscript",
		".zsh":  "text/x-shellscript",
		".ps1":  "text/x-powershell",
		".rb":   "text/x-ruby",
		".php":  "text/x-php",
		".pl":   "text/x-perl",
		".r":    "text/x-r",
		".tex":  "text/x-tex",
		".ltx":  "text/x-tex",
		".rst":  "text/x-rst",
		".asciidoc": "text/x-asciidoc",
		".adoc": "text/x-asciidoc",
		".org":  "text/x-org",
		".wiki": "text/x-wiki",
	}

	if contentType, exists := contentTypes[ext]; exists {
		return contentType
	}

	return "application/octet-stream"
}