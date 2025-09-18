package core

import (
	"path/filepath"
	"strings"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// SimpleContentAnalyzer implements basic content analysis
type SimpleContentAnalyzer struct {
	logger interfaces.Logger
}

// NewSimpleContentAnalyzer creates a new simple content analyzer
func NewSimpleContentAnalyzer() *SimpleContentAnalyzer {
	return &SimpleContentAnalyzer{}
}

// WithLogger adds logging to the content analyzer
func (sca *SimpleContentAnalyzer) WithLogger(logger interfaces.Logger) *SimpleContentAnalyzer {
	sca.logger = logger
	return sca
}

// AnalyzeFile analyzes a file and returns basic analysis
func (sca *SimpleContentAnalyzer) AnalyzeFile(filePath string) (*models.FileAnalysis, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	analysis := &models.FileAnalysis{
		FilePath:    filePath,
		IsText:      isTextByExtension(filePath),
		ContentType: getContentTypeByExtension(ext),
		Metadata:    make(map[string]interface{}),
	}

	// Basic language detection by file extension
	if language := getLanguageByExtension(ext); language != "" {
		analysis.Language = language
		analysis.Confidence = 0.8 // Moderate confidence based on extension
	}

	return analysis, nil
}

// ValidateContent performs basic content validation
func (sca *SimpleContentAnalyzer) ValidateContent(content string) error {
	if len(content) == 0 {
		return models.NewDocumentError(models.ErrDocumentInvalid, "content is empty")
	}

	// Check for extremely large content
	if len(content) > 100*1024*1024 { // 100MB
		return models.NewDocumentError(models.ErrDocumentTooBig, "content exceeds maximum size")
	}

	return nil
}

// ExtractMetadata extracts basic metadata from file path and content
func (sca *SimpleContentAnalyzer) ExtractMetadata(filePath string, content string) (map[string]interface{}, error) {
	metadata := make(map[string]interface{})

	// File-based metadata
	ext := strings.ToLower(filepath.Ext(filePath))
	metadata["file_extension"] = ext
	metadata["file_name"] = filepath.Base(filePath)
	metadata["file_dir"] = filepath.Dir(filePath)

	// Content-based metadata
	metadata["content_length"] = len(content)
	metadata["line_count"] = strings.Count(content, "\n") + 1
	metadata["word_count"] = estimateWordCount(content)

	// Language detection
	if language := getLanguageByExtension(ext); language != "" {
		metadata["language"] = language
	}

	return metadata, nil
}

// DetectLanguage performs basic language detection
func (sca *SimpleContentAnalyzer) DetectLanguage(content string) (string, float64, error) {
	// Very basic language detection
	// In a real implementation, you'd use a proper language detection library

	// Count English indicators
	englishIndicators := []string{"the", "and", "or", "but", "in", "on", "at", "to", "for", "of", "with"}
	englishCount := 0
	words := strings.Fields(strings.ToLower(content))

	for _, word := range words {
		for _, indicator := range englishIndicators {
			if word == indicator {
				englishCount++
				break
			}
		}
	}

	if len(words) == 0 {
		return "unknown", 0.0, nil
	}

	confidence := float64(englishCount) / float64(len(words))
	if confidence > 0.1 {
		return "en", confidence, nil
	}

	return "unknown", 0.0, nil
}

// Helper functions

func getContentTypeByExtension(ext string) string {
	contentTypes := map[string]string{
		".txt":  "text/plain",
		".md":   "text/markdown",
		".json": "application/json",
		".xml":  "application/xml",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".py":   "text/x-python",
		".go":   "text/x-go",
		".java": "text/x-java",
		".c":    "text/x-c",
		".cpp":  "text/x-c++",
		".sql":  "application/sql",
		".yaml": "application/yaml",
		".yml":  "application/yaml",
		".csv":  "text/csv",
		".log":  "text/plain",
	}

	if contentType, exists := contentTypes[ext]; exists {
		return contentType
	}

	return "application/octet-stream"
}

func getLanguageByExtension(ext string) string {
	languages := map[string]string{
		".py":   "python",
		".go":   "go",
		".js":   "javascript",
		".ts":   "typescript",
		".java": "java",
		".c":    "c",
		".cpp":  "cpp",
		".h":    "c",
		".hpp":  "cpp",
		".sql":  "sql",
		".sh":   "bash",
		".bash": "bash",
		".zsh":  "zsh",
		".ps1":  "powershell",
		".rb":   "ruby",
		".php":  "php",
		".pl":   "perl",
		".r":    "r",
		".tex":  "latex",
	}

	return languages[ext]
}

func estimateWordCount(content string) int {
	if content == "" {
		return 0
	}

	words := strings.Fields(content)
	return len(words)
}