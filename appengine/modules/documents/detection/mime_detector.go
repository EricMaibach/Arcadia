package detection

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"arcadia/modules/documents/models"
	"arcadia/modules/documents/processors/base"
)

// MIMEDetector detects document type based on MIME type analysis
type MIMEDetector struct {
	mimeMap  map[string]base.ProcessorType
	priority int
}

// NewMIMEDetector creates a new MIME-based detector
func NewMIMEDetector() *MIMEDetector {
	return &MIMEDetector{
		mimeMap:  getDefaultMIMEMapping(),
		priority: 80, // Medium-high priority, less reliable than extension but more than content
	}
}

// DetectType detects document type based on MIME type
func (md *MIMEDetector) DetectType(filePath string) (*base.DocumentType, error) {
	if filePath == "" {
		return nil, models.NewDocumentError(models.ErrInvalidInput, "file path cannot be empty").
			WithFilePath(filePath)
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, models.NewDocumentErrorWithCause(models.ErrFileNotFound, "file does not exist", err).
			WithFilePath(filePath)
	}

	// Read file header to detect MIME type
	mimeType, err := md.detectMIMEType(filePath)
	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrFileReadError,
			"failed to detect MIME type", err).WithFilePath(filePath)
	}

	if mimeType == "" {
		return nil, nil // Could not detect MIME type
	}

	// Look up processor type
	processorType, exists := md.mimeMap[mimeType]
	if !exists {
		return nil, nil // Unknown MIME type
	}

	// Extract file extension for metadata
	ext := strings.ToLower(filepath.Ext(filePath))
	ext = strings.TrimPrefix(ext, ".")

	// Return document type with medium-high confidence
	return &base.DocumentType{
		Type:       processorType,
		MimeType:   mimeType,
		Extension:  ext,
		Confidence: 0.85, // Good confidence for MIME detection
		Metadata: map[string]interface{}{
			"detector":     "mime",
			"detected_by":  "MIMEDetector",
			"original_path": filePath,
			"mime_type":    mimeType,
		},
	}, nil
}

// GetConfidence returns confidence level for a given file path
func (md *MIMEDetector) GetConfidence(filePath string) float64 {
	mimeType, err := md.detectMIMEType(filePath)
	if err != nil || mimeType == "" {
		return 0.0
	}

	if _, exists := md.mimeMap[mimeType]; exists {
		return 0.85
	}

	return 0.0
}

// GetPriority returns the priority of this detector
func (md *MIMEDetector) GetPriority() int {
	return md.priority
}

// detectMIMEType detects MIME type by reading file header
func (md *MIMEDetector) detectMIMEType(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Read first 512 bytes for MIME detection (http.DetectContentType requirement)
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}

	// Detect MIME type using Go's built-in detector
	mimeType := http.DetectContentType(buffer[:n])

	// Clean up MIME type (remove charset and other parameters)
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	return mimeType, nil
}

// AddMIMEMapping adds a new MIME type to processor type mapping
func (md *MIMEDetector) AddMIMEMapping(mimeType string, processorType base.ProcessorType) error {
	if mimeType == "" {
		return models.NewDocumentError(models.ErrInvalidInput, "MIME type cannot be empty")
	}

	if !processorType.IsValid() {
		return models.NewDocumentError(models.ErrInvalidInput, "invalid processor type").
			WithContext(map[string]interface{}{"processor_type": processorType})
	}

	md.mimeMap[mimeType] = processorType
	return nil
}

// GetSupportedMIMETypes returns all supported MIME types
func (md *MIMEDetector) GetSupportedMIMETypes() []string {
	mimeTypes := make([]string, 0, len(md.mimeMap))
	for mimeType := range md.mimeMap {
		mimeTypes = append(mimeTypes, mimeType)
	}
	return mimeTypes
}

// getDefaultMIMEMapping returns the default mapping of MIME types to processor types
func getDefaultMIMEMapping() map[string]base.ProcessorType {
	return map[string]base.ProcessorType{
		// Text files
		"text/plain":       base.ProcessorTypeText,
		"text/csv":         base.ProcessorTypeText,
		"text/tab-separated-values": base.ProcessorTypeText,
		"application/json": base.ProcessorTypeText,
		"application/xml":  base.ProcessorTypeText,
		"text/xml":         base.ProcessorTypeText,
		"application/yaml": base.ProcessorTypeText,
		"text/yaml":        base.ProcessorTypeText,

		// Markdown files
		"text/markdown":       base.ProcessorTypeMarkdown,
		"text/x-markdown":     base.ProcessorTypeMarkdown,
		"application/markdown": base.ProcessorTypeMarkdown,

		// PDF files
		"application/pdf": base.ProcessorTypePDF,

		// Web files (treated as text)
		"text/html":       base.ProcessorTypeText,
		"application/xhtml+xml": base.ProcessorTypeText,
		"text/css":        base.ProcessorTypeText,

		// Code files (treated as text)
		"text/x-go":           base.ProcessorTypeText,
		"application/x-go":    base.ProcessorTypeText,
		"text/javascript":     base.ProcessorTypeText,
		"application/javascript": base.ProcessorTypeText,
		"application/x-javascript": base.ProcessorTypeText,
		"text/x-python":       base.ProcessorTypeText,
		"application/x-python": base.ProcessorTypeText,
		"text/x-java-source":  base.ProcessorTypeText,
		"text/x-c":           base.ProcessorTypeText,
		"text/x-c++":         base.ProcessorTypeText,
		"text/x-csharp":      base.ProcessorTypeText,
		"text/x-php":         base.ProcessorTypeText,
		"text/x-ruby":        base.ProcessorTypeText,
		"text/x-rust":        base.ProcessorTypeText,
		"text/x-swift":       base.ProcessorTypeText,
		"text/x-shellscript": base.ProcessorTypeText,
		"application/x-sh":   base.ProcessorTypeText,

		// Configuration files (treated as text)
		"application/toml":    base.ProcessorTypeText,
		"text/x-properties":  base.ProcessorTypeText,
		"application/x-wine-extension-ini": base.ProcessorTypeText,

		// Generic fallbacks
		"application/octet-stream": base.ProcessorTypeText, // Binary files default to text processor

		// Audio MIME types
		"audio/mpeg":      base.ProcessorTypeAudio, // MP3
		"audio/wav":       base.ProcessorTypeAudio, // WAV
		"audio/x-wav":     base.ProcessorTypeAudio, // WAV (alternate)
		"audio/wave":      base.ProcessorTypeAudio, // WAV (alternate)
		"audio/mp4":       base.ProcessorTypeAudio, // M4A
		"audio/x-m4a":     base.ProcessorTypeAudio, // M4A (alternate)
		"audio/flac":      base.ProcessorTypeAudio, // FLAC
		"audio/x-flac":    base.ProcessorTypeAudio, // FLAC (alternate)
		"audio/ogg":       base.ProcessorTypeAudio, // OGG
		"audio/aac":       base.ProcessorTypeAudio, // AAC
		"audio/aacp":      base.ProcessorTypeAudio, // AAC+ (alternate)
	}
}