package pdf

import (
	"fmt"
	"os/exec"
	"time"
)

// PDFConfig holds configuration for PDF processing
type PDFConfig struct {
	// Tool paths
	PDFToTextPath string `json:"pdftotext_path" yaml:"pdftotext_path"`
	OCRMyPDFPath  string `json:"ocrmypdf_path" yaml:"ocrmypdf_path"`

	// Processing limits
	MaxFileSize      int64         `json:"max_file_size" yaml:"max_file_size"`           // Maximum file size in bytes
	ProcessingTimeout time.Duration `json:"processing_timeout" yaml:"processing_timeout"` // Processing timeout

	// Text extraction settings
	TextQualityThreshold float64 `json:"text_quality_threshold" yaml:"text_quality_threshold"` // Minimum text quality to skip OCR
	MinWordsPerPage      int     `json:"min_words_per_page" yaml:"min_words_per_page"`         // Minimum words per page for quality check

	// OCR settings
	OCRLanguages      []string `json:"ocr_languages" yaml:"ocr_languages"`           // OCR languages (e.g., "eng", "fra", "spa")
	OCRQualityLevel   int      `json:"ocr_quality_level" yaml:"ocr_quality_level"`   // OCR quality level (1-3)
	EnableOCRFallback bool     `json:"enable_ocr_fallback" yaml:"enable_ocr_fallback"` // Enable OCR fallback for poor text extraction

	// Tool validation
	RequireTools bool `json:"require_tools" yaml:"require_tools"` // Require CLI tools to be available

	// Security settings
	AllowPasswordProtected bool `json:"allow_password_protected" yaml:"allow_password_protected"` // Allow password-protected PDFs
	MaxPages               int  `json:"max_pages" yaml:"max_pages"`                               // Maximum number of pages to process
}

// DefaultPDFConfig returns a default PDF configuration
func DefaultPDFConfig() *PDFConfig {
	return &PDFConfig{
		PDFToTextPath:          "pdftotext", // Assume in PATH
		OCRMyPDFPath:           "ocrmypdf",  // Assume in PATH
		MaxFileSize:            100 * 1024 * 1024, // 100MB
		ProcessingTimeout:      5 * time.Minute,
		TextQualityThreshold:   0.3,  // 30% quality threshold
		MinWordsPerPage:        10,   // Minimum 10 words per page
		OCRLanguages:           []string{"eng"}, // English by default
		OCRQualityLevel:        2,    // Medium quality
		EnableOCRFallback:      true,
		RequireTools:           false, // Don't require tools by default
		AllowPasswordProtected: false,
		MaxPages:               1000, // Maximum 1000 pages
	}
}

// Validate validates the PDF configuration
func (c *PDFConfig) Validate() error {
	if c.MaxFileSize <= 0 {
		return &PDFError{
			Code:    ErrConfigInvalid,
			Message: "max_file_size must be positive",
		}
	}

	if c.ProcessingTimeout <= 0 {
		return &PDFError{
			Code:    ErrConfigInvalid,
			Message: "processing_timeout must be positive",
		}
	}

	if c.TextQualityThreshold < 0 || c.TextQualityThreshold > 1 {
		return &PDFError{
			Code:    ErrConfigInvalid,
			Message: "text_quality_threshold must be between 0 and 1",
		}
	}

	if c.OCRQualityLevel < 1 || c.OCRQualityLevel > 3 {
		return &PDFError{
			Code:    ErrConfigInvalid,
			Message: "ocr_quality_level must be between 1 and 3",
		}
	}

	if c.MaxPages <= 0 {
		return &PDFError{
			Code:    ErrConfigInvalid,
			Message: "max_pages must be positive",
		}
	}

	if len(c.OCRLanguages) == 0 {
		return &PDFError{
			Code:    ErrConfigInvalid,
			Message: "at least one OCR language must be specified",
		}
	}

	return nil
}

// CheckToolsAvailable checks if required CLI tools are available
func (c *PDFConfig) CheckToolsAvailable() error {
	// Check pdftotext
	if _, err := exec.LookPath(c.PDFToTextPath); err != nil {
		if c.RequireTools {
			return &PDFError{
				Code:    ErrToolNotFound,
				Message: "pdftotext tool not found",
				Context: map[string]interface{}{
					"tool_path": c.PDFToTextPath,
					"error":     err.Error(),
				},
			}
		}
	}

	// Check ocrmypdf
	if _, err := exec.LookPath(c.OCRMyPDFPath); err != nil {
		if c.RequireTools {
			return &PDFError{
				Code:    ErrToolNotFound,
				Message: "ocrmypdf tool not found",
				Context: map[string]interface{}{
					"tool_path": c.OCRMyPDFPath,
					"error":     err.Error(),
				},
			}
		}
	}

	return nil
}

// GetToolsStatus returns the availability status of CLI tools
func (c *PDFConfig) GetToolsStatus() map[string]bool {
	status := make(map[string]bool)

	// Check pdftotext
	if _, err := exec.LookPath(c.PDFToTextPath); err == nil {
		status["pdftotext"] = true
	} else {
		status["pdftotext"] = false
	}

	// Check ocrmypdf
	if _, err := exec.LookPath(c.OCRMyPDFPath); err == nil {
		status["ocrmypdf"] = true
	} else {
		status["ocrmypdf"] = false
	}

	return status
}

// PDFErrorCode represents PDF-specific error codes
type PDFErrorCode string

const (
	ErrConfigInvalid     PDFErrorCode = "PDF_CONFIG_INVALID"
	ErrToolNotFound      PDFErrorCode = "PDF_TOOL_NOT_FOUND"
	ErrToolExecution     PDFErrorCode = "PDF_TOOL_EXECUTION_FAILED"
	ErrPDFCorrupted      PDFErrorCode = "PDF_CORRUPTED"
	ErrPDFPasswordProtected PDFErrorCode = "PDF_PASSWORD_PROTECTED"
	ErrPDFTooBig         PDFErrorCode = "PDF_TOO_BIG"
	ErrPDFTooManyPages   PDFErrorCode = "PDF_TOO_MANY_PAGES"
	ErrTextExtractionFailed PDFErrorCode = "PDF_TEXT_EXTRACTION_FAILED"
	ErrOCRFailed         PDFErrorCode = "PDF_OCR_FAILED"
	ErrQualityTooLow     PDFErrorCode = "PDF_QUALITY_TOO_LOW"
)

// PDFError represents a PDF processing error
type PDFError struct {
	Code    PDFErrorCode    `json:"code"`
	Message string          `json:"message"`
	Cause   error           `json:"cause,omitempty"`
	Context map[string]interface{} `json:"context,omitempty"`
}

// Error implements the error interface
func (e *PDFError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *PDFError) Unwrap() error {
	return e.Cause
}

// IsCode checks if the error has the specified error code
func (e *PDFError) IsCode(code PDFErrorCode) bool {
	return e.Code == code
}