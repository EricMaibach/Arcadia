package text

import (
	"context"
	"fmt"

	"arcadia/modules/documents/processors/base"
	"arcadia/modules/documents/interfaces"
)

// TextProcessor handles processing of plain text files
type TextProcessor struct {
	extractor         *TextExtractor
	languageDetector  *LanguageDetector
	metadataExtractor *MetadataExtractor
	logger            interfaces.Logger
}

// NewTextProcessor creates a new text processor
func NewTextProcessor() *TextProcessor {
	return &TextProcessor{
		extractor:         NewTextExtractor(),
		languageDetector:  NewLanguageDetector(),
		metadataExtractor: NewMetadataExtractor(),
	}
}

// WithLogger adds logging to the text processor
func (tp *TextProcessor) WithLogger(logger interfaces.Logger) *TextProcessor {
	tp.logger = logger
	return tp
}

// CanProcess determines if this processor can handle the given file
func (tp *TextProcessor) CanProcess(filePath string) bool {
	return tp.extractor.IsTextFile(filePath)
}

// Process extracts content and metadata from a text document
func (tp *TextProcessor) Process(ctx context.Context, filePath string) (*base.ProcessingResult, error) {
	if tp.logger != nil {
		tp.logger.Debug(ctx, "Processing text file", "file_path", filePath)
	}

	// Check if we can process this file
	if !tp.CanProcess(filePath) {
		return nil, fmt.Errorf("text processor cannot handle file: %s", filePath)
	}

	// Extract text content from file
	content, err := tp.extractor.ExtractFromFile(filePath)
	if err != nil {
		if tp.logger != nil {
			tp.logger.Error(ctx, "Failed to extract content from file",
				"file_path", filePath,
				"error", err)
		}
		return nil, fmt.Errorf("failed to extract content from %s: %w", filePath, err)
	}

	// Extract metadata
	metadata, err := tp.metadataExtractor.ExtractFromFile(filePath, content)
	if err != nil {
		if tp.logger != nil {
			tp.logger.Warn(ctx, "Failed to extract metadata, using basic metadata",
				"file_path", filePath,
				"error", err)
		}
		// Fall back to basic metadata if full extraction fails
		metadata = tp.metadataExtractor.ExtractBasicMetadata(filePath, len(content))
	}

	// Detect language and confidence
	language, confidence := tp.languageDetector.DetectLanguage(filePath, content)

	// Get content type
	contentType := tp.extractor.GetContentType(filePath)

	// Create processing result
	result := &base.ProcessingResult{
		Content:     content,
		Metadata:    metadata,
		ContentType: contentType,
		Language:    language,
		Confidence:  confidence,
	}

	// Add processor-specific metadata
	result.Metadata["processor_type"] = tp.GetProcessorType().String()
	result.Metadata["processor_version"] = "1.0.0"

	if tp.logger != nil {
		tp.logger.Info(ctx, "Successfully processed text file",
			"file_path", filePath,
			"content_length", len(content),
			"language", language,
			"confidence", confidence,
			"content_type", contentType)
	}

	return result, nil
}

// GetSupportedExtensions returns the file extensions this processor supports
func (tp *TextProcessor) GetSupportedExtensions() []string {
	return []string{
		// Plain text
		".txt", ".text", ".log", ".out", ".err",

		// Documentation
		".md", ".markdown", ".rst", ".asciidoc", ".adoc", ".org", ".wiki", ".tex", ".ltx",

		// Configuration
		".ini", ".conf", ".config", ".properties", ".toml", ".env", ".cfg",

		// Data formats
		".json", ".yaml", ".yml", ".xml", ".csv", ".tsv", ".sql",

		// Web technologies
		".html", ".htm", ".css", ".js", ".ts", ".jsx", ".tsx",

		// Programming languages
		".py", ".go", ".java", ".c", ".cpp", ".h", ".hpp", ".cs", ".php", ".rb", ".pl", ".r",
		".swift", ".kt", ".scala", ".clj", ".hs", ".ml", ".fs", ".dart", ".lua", ".tcl",
		".m", ".mm", ".vb", ".pas", ".f", ".f90", ".ada", ".cob", ".cobol",

		// Scripts
		".sh", ".bash", ".zsh", ".fish", ".csh", ".tcsh", ".ksh", ".ps1", ".psm1", ".psd1",
		".bat", ".cmd",

		// Other text formats
		".rtf", ".diff", ".patch", ".gitignore", ".dockerignore",
	}
}

// GetProcessorType returns the type of this processor
func (tp *TextProcessor) GetProcessorType() base.ProcessorType {
	return base.ProcessorTypeText
}

// ValidateContent validates text content using the extractor
func (tp *TextProcessor) ValidateContent(content string) error {
	return tp.extractor.ValidateContent(content)
}

// EstimateProcessingComplexity estimates the computational complexity of processing a file
func (tp *TextProcessor) EstimateProcessingComplexity(filePath string) (int, error) {
	// For text files, complexity is primarily based on file size
	content, err := tp.extractor.ExtractFromFile(filePath)
	if err != nil {
		return 0, err
	}

	// Simple complexity calculation based on content length
	// Complexity score ranges from 1 (very simple) to 10 (very complex)
	contentLength := len(content)

	switch {
	case contentLength < 1024: // < 1KB
		return 1, nil
	case contentLength < 10*1024: // < 10KB
		return 2, nil
	case contentLength < 100*1024: // < 100KB
		return 3, nil
	case contentLength < 1024*1024: // < 1MB
		return 5, nil
	case contentLength < 10*1024*1024: // < 10MB
		return 7, nil
	case contentLength < 50*1024*1024: // < 50MB
		return 9, nil
	default: // >= 50MB
		return 10, nil
	}
}

// GetProcessorMetadata returns metadata about this processor
func (tp *TextProcessor) GetProcessorMetadata() map[string]interface{} {
	return map[string]interface{}{
		"name":        "TextProcessor",
		"version":     "1.0.0",
		"type":        tp.GetProcessorType().String(),
		"description": "Processes plain text files and extracts content with metadata",
		"capabilities": []string{
			"text_extraction",
			"language_detection",
			"metadata_extraction",
			"content_validation",
		},
		"supported_extensions": tp.GetSupportedExtensions(),
		"max_file_size":       100 * 1024 * 1024, // 100MB
		"features": map[string]bool{
			"utf8_validation":     true,
			"language_detection":  true,
			"metadata_extraction": true,
			"file_categorization": true,
			"content_statistics":  true,
		},
	}
}