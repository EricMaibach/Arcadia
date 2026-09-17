package base

import (
	"time"
)

// ProcessorConfig holds configuration for document processors
type ProcessorConfig struct {
	// Enabled determines if this processor should be active
	Enabled bool `yaml:"enabled" json:"enabled"`

	// MaxFileSize is the maximum file size this processor will handle (in bytes)
	MaxFileSize int64 `yaml:"max_file_size" json:"max_file_size"`

	// Timeout is the maximum time allowed for processing a single document
	Timeout time.Duration `yaml:"timeout" json:"timeout"`

	// Concurrency is the maximum number of concurrent processing operations
	Concurrency int `yaml:"concurrency" json:"concurrency"`

	// Extensions is a list of file extensions this processor should handle
	Extensions []string `yaml:"extensions" json:"extensions"`

	// Priority determines the order in which processors are tried (higher = earlier)
	Priority int `yaml:"priority" json:"priority"`

	// Metadata contains processor-specific configuration
	Metadata map[string]interface{} `yaml:"metadata" json:"metadata"`
}

// PDFConfig holds configuration specific to PDF processing
type PDFConfig struct {
	ProcessorConfig `yaml:",inline"`

	// ToolPaths contains paths to external tools used for PDF processing
	ToolPaths PDFToolPaths `yaml:"tool_paths" json:"tool_paths"`

	// OCREnabled determines if OCR should be performed on scanned PDFs
	OCREnabled bool `yaml:"ocr_enabled" json:"ocr_enabled"`

	// PreserveBinary determines if binary data should be preserved in metadata
	PreserveBinary bool `yaml:"preserve_binary" json:"preserve_binary"`
}

// PDFToolPaths contains paths to external PDF processing tools
type PDFToolPaths struct {
	// PopplerPath is the path to the poppler-utils tools directory
	PopplerPath string `yaml:"poppler_path" json:"poppler_path"`

	// PDFToText is the path to the pdftotext utility
	PDFToText string `yaml:"pdf_to_text" json:"pdf_to_text"`

	// PDFInfo is the path to the pdfinfo utility
	PDFInfo string `yaml:"pdf_info" json:"pdf_info"`

	// TesseractPath is the path to the Tesseract OCR engine
	TesseractPath string `yaml:"tesseract_path" json:"tesseract_path"`
}

// TextConfig holds configuration specific to text processing
type TextConfig struct {
	ProcessorConfig `yaml:",inline"`

	// EncodingDetection determines if automatic encoding detection should be used
	EncodingDetection bool `yaml:"encoding_detection" json:"encoding_detection"`

	// DefaultEncoding is the fallback encoding to use if detection fails
	DefaultEncoding string `yaml:"default_encoding" json:"default_encoding"`

	// MaxLineLength is the maximum length of a single line before truncation
	MaxLineLength int `yaml:"max_line_length" json:"max_line_length"`
}

// MarkdownConfig holds configuration specific to Markdown processing
type MarkdownConfig struct {
	ProcessorConfig `yaml:",inline"`

	// ParseFrontMatter determines if YAML/TOML front matter should be parsed
	ParseFrontMatter bool `yaml:"parse_front_matter" json:"parse_front_matter"`

	// RenderHTML determines if Markdown should be rendered to HTML
	RenderHTML bool `yaml:"render_html" json:"render_html"`

	// ExtractLinks determines if links should be extracted as metadata
	ExtractLinks bool `yaml:"extract_links" json:"extract_links"`
}

// PluginRegistryConfig holds configuration for the plugin registry
type PluginRegistryConfig struct {
	// AutoDiscovery determines if plugins should be automatically discovered
	AutoDiscovery bool `yaml:"auto_discovery" json:"auto_discovery"`

	// PluginDirs contains directories to search for plugins
	PluginDirs []string `yaml:"plugin_dirs" json:"plugin_dirs"`

	// DefaultTimeout is the default timeout for all processors
	DefaultTimeout time.Duration `yaml:"default_timeout" json:"default_timeout"`

	// DefaultMaxFileSize is the default maximum file size for all processors
	DefaultMaxFileSize int64 `yaml:"default_max_file_size" json:"default_max_file_size"`

	// DefaultConcurrency is the default concurrency level for all processors
	DefaultConcurrency int `yaml:"default_concurrency" json:"default_concurrency"`
}

// GetDefaultProcessorConfig returns default configuration for a processor
func GetDefaultProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		Enabled:     true,
		MaxFileSize: 100 * 1024 * 1024, // 100MB
		Timeout:     5 * time.Minute,
		Concurrency: 4,
		Extensions:  []string{},
		Priority:    100,
		Metadata:    make(map[string]interface{}),
	}
}

// GetDefaultPDFConfig returns default configuration for PDF processing
func GetDefaultPDFConfig() PDFConfig {
	return PDFConfig{
		ProcessorConfig: ProcessorConfig{
			Enabled:     true,
			MaxFileSize: 100 * 1024 * 1024, // 100MB
			Timeout:     10 * time.Minute,  // PDFs may take longer
			Concurrency: 2,                 // Lower concurrency for PDF processing
			Extensions:  []string{"pdf"},
			Priority:    200,
			Metadata:    make(map[string]interface{}),
		},
		ToolPaths: PDFToolPaths{
			PopplerPath:   "/usr/bin",
			PDFToText:     "/usr/bin/pdftotext",
			PDFInfo:       "/usr/bin/pdfinfo",
			TesseractPath: "/usr/bin/tesseract",
		},
		OCREnabled:     false,
		PreserveBinary: false,
	}
}

// GetDefaultTextConfig returns default configuration for text processing
func GetDefaultTextConfig() TextConfig {
	return TextConfig{
		ProcessorConfig: ProcessorConfig{
			Enabled:     true,
			MaxFileSize: 50 * 1024 * 1024, // 50MB
			Timeout:     2 * time.Minute,
			Concurrency: 8,
			Extensions:  []string{"txt", "log", "csv", "json", "xml", "yaml", "yml"},
			Priority:    50,
			Metadata:    make(map[string]interface{}),
		},
		EncodingDetection: true,
		DefaultEncoding:   "utf-8",
		MaxLineLength:     10000,
	}
}

// GetDefaultMarkdownConfig returns default configuration for Markdown processing
func GetDefaultMarkdownConfig() MarkdownConfig {
	return MarkdownConfig{
		ProcessorConfig: ProcessorConfig{
			Enabled:     true,
			MaxFileSize: 10 * 1024 * 1024, // 10MB
			Timeout:     1 * time.Minute,
			Concurrency: 8,
			Extensions:  []string{"md", "markdown", "mdown", "mkd"},
			Priority:    150,
			Metadata:    make(map[string]interface{}),
		},
		ParseFrontMatter: true,
		RenderHTML:       false,
		ExtractLinks:     true,
	}
}

// GetDefaultPluginRegistryConfig returns default configuration for the plugin registry
func GetDefaultPluginRegistryConfig() PluginRegistryConfig {
	return PluginRegistryConfig{
		AutoDiscovery:      true,
		PluginDirs:         []string{},
		DefaultTimeout:     5 * time.Minute,
		DefaultMaxFileSize: 100 * 1024 * 1024, // 100MB
		DefaultConcurrency: 4,
	}
}
