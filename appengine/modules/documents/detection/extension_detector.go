package detection

import (
	"path/filepath"
	"strings"

	"arcadia/modules/documents/models"
	"arcadia/modules/documents/processors/base"
)

// ExtensionDetector detects document type based on file extension
type ExtensionDetector struct {
	extensionMap map[string]base.ProcessorType
	priority     int
}

// NewExtensionDetector creates a new extension-based detector
func NewExtensionDetector() *ExtensionDetector {
	return &ExtensionDetector{
		extensionMap: getDefaultExtensionMapping(),
		priority:     100, // High priority as extensions are most reliable
	}
}

// DetectType detects document type based on file extension
func (ed *ExtensionDetector) DetectType(filePath string) (*base.DocumentType, error) {
	if filePath == "" {
		return nil, models.NewDocumentError(models.ErrInvalidInput, "file path cannot be empty").
			WithFilePath(filePath)
	}

	// Extract extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return nil, nil // No extension, can't detect
	}

	// Remove the dot from extension
	ext = strings.TrimPrefix(ext, ".")

	// Look up processor type
	processorType, exists := ed.extensionMap[ext]
	if !exists {
		return nil, nil // Unknown extension
	}

	// Return document type with high confidence
	return &base.DocumentType{
		Type:       processorType,
		Extension:  ext,
		Confidence: 0.95, // High confidence for known extensions
		Metadata: map[string]interface{}{
			"detector":        "extension",
			"detected_by":     "ExtensionDetector",
			"original_path":   filePath,
			"file_extension":  ext,
		},
	}, nil
}

// GetConfidence returns confidence level for a given file path
func (ed *ExtensionDetector) GetConfidence(filePath string) float64 {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return 0.0
	}

	ext = strings.TrimPrefix(ext, ".")
	if _, exists := ed.extensionMap[ext]; exists {
		return 0.95
	}

	return 0.0
}

// GetPriority returns the priority of this detector
func (ed *ExtensionDetector) GetPriority() int {
	return ed.priority
}

// AddExtensionMapping adds a new extension to processor type mapping
func (ed *ExtensionDetector) AddExtensionMapping(extension string, processorType base.ProcessorType) error {
	if extension == "" {
		return models.NewDocumentError(models.ErrInvalidInput, "extension cannot be empty")
	}

	if !processorType.IsValid() {
		return models.NewDocumentError(models.ErrInvalidInput, "invalid processor type").
			WithContext(map[string]interface{}{"processor_type": processorType})
	}

	// Normalize extension (remove dot, lowercase)
	ext := strings.ToLower(strings.TrimPrefix(extension, "."))
	ed.extensionMap[ext] = processorType

	return nil
}

// GetSupportedExtensions returns all supported file extensions
func (ed *ExtensionDetector) GetSupportedExtensions() []string {
	extensions := make([]string, 0, len(ed.extensionMap))
	for ext := range ed.extensionMap {
		extensions = append(extensions, ext)
	}
	return extensions
}

// getDefaultExtensionMapping returns the default mapping of file extensions to processor types
func getDefaultExtensionMapping() map[string]base.ProcessorType {
	return map[string]base.ProcessorType{
		// Text files
		"txt":  base.ProcessorTypeText,
		"text": base.ProcessorTypeText,
		"log":  base.ProcessorTypeText,
		"csv":  base.ProcessorTypeText,
		"tsv":  base.ProcessorTypeText,
		"json": base.ProcessorTypeText,
		"xml":  base.ProcessorTypeText,
		"yaml": base.ProcessorTypeText,
		"yml":  base.ProcessorTypeText,
		"ini":  base.ProcessorTypeText,
		"cfg":  base.ProcessorTypeText,
		"conf": base.ProcessorTypeText,

		// Markdown files
		"md":       base.ProcessorTypeMarkdown,
		"markdown": base.ProcessorTypeMarkdown,
		"mdown":    base.ProcessorTypeMarkdown,
		"mkd":      base.ProcessorTypeMarkdown,
		"mdwn":     base.ProcessorTypeMarkdown,
		"mdtxt":    base.ProcessorTypeMarkdown,
		"mdtext":   base.ProcessorTypeMarkdown,

		// PDF files
		"pdf": base.ProcessorTypePDF,

		// Code files (treated as text)
		"go":   base.ProcessorTypeText,
		"js":   base.ProcessorTypeText,
		"ts":   base.ProcessorTypeText,
		"py":   base.ProcessorTypeText,
		"java": base.ProcessorTypeText,
		"c":    base.ProcessorTypeText,
		"cpp":  base.ProcessorTypeText,
		"h":    base.ProcessorTypeText,
		"hpp":  base.ProcessorTypeText,
		"cs":   base.ProcessorTypeText,
		"php":  base.ProcessorTypeText,
		"rb":   base.ProcessorTypeText,
		"rs":   base.ProcessorTypeText,
		"kt":   base.ProcessorTypeText,
		"swift": base.ProcessorTypeText,
		"dart": base.ProcessorTypeText,
		"sh":   base.ProcessorTypeText,
		"bash": base.ProcessorTypeText,
		"zsh":  base.ProcessorTypeText,
		"fish": base.ProcessorTypeText,
		"ps1":  base.ProcessorTypeText,
		"bat":  base.ProcessorTypeText,
		"cmd":  base.ProcessorTypeText,

		// Web files (treated as text)
		"html": base.ProcessorTypeText,
		"htm":  base.ProcessorTypeText,
		"css":  base.ProcessorTypeText,
		"scss": base.ProcessorTypeText,
		"sass": base.ProcessorTypeText,
		"less": base.ProcessorTypeText,

		// Configuration and data files (treated as text)
		"toml":       base.ProcessorTypeText,
		"properties": base.ProcessorTypeText,
		"env":        base.ProcessorTypeText,
		"gitignore":  base.ProcessorTypeText,
		"dockerfile": base.ProcessorTypeText,
		"makefile":   base.ProcessorTypeText,
		"cmake":      base.ProcessorTypeText,
		"gradle":     base.ProcessorTypeText,
		"pom":        base.ProcessorTypeText,
		"sql":        base.ProcessorTypeText,

		// Microsoft Office formats -> Tika
		"doc":  base.ProcessorTypeTika,
		"docx": base.ProcessorTypeTika,
		"xls":  base.ProcessorTypeTika,
		"xlsx": base.ProcessorTypeTika,
		"ppt":  base.ProcessorTypeTika,
		"pptx": base.ProcessorTypeTika,
		"docm": base.ProcessorTypeTika,
		"xlsm": base.ProcessorTypeTika,
		"pptm": base.ProcessorTypeTika,
		"xlsb": base.ProcessorTypeTika,
		"xltx": base.ProcessorTypeTika,
		"xltm": base.ProcessorTypeTika,
		"potx": base.ProcessorTypeTika,
		"potm": base.ProcessorTypeTika,

		// OpenDocument formats -> Tika
		"odt": base.ProcessorTypeTika,
		"ods": base.ProcessorTypeTika,
		"odp": base.ProcessorTypeTika,
		"odg": base.ProcessorTypeTika,
		"odf": base.ProcessorTypeTika,

		// LibreOffice/OpenOffice legacy formats -> Tika
		"sxw": base.ProcessorTypeTika,
		"sxc": base.ProcessorTypeTika,
		"sxi": base.ProcessorTypeTika,

		// Other office-like formats -> Tika
		"rtf": base.ProcessorTypeTika,
		"wpd": base.ProcessorTypeTika,
		"wps": base.ProcessorTypeTika,
		"pub": base.ProcessorTypeTika,
		"vsd": base.ProcessorTypeTika,
		"msg": base.ProcessorTypeTika,
	}
}