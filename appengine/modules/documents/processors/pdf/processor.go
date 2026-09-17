package pdf

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/processors/base"
)

// PDFProcessor handles processing of PDF documents
type PDFProcessor struct {
	base.BaseProcessor
	config     *PDFConfig
	extractor  *TextExtractor
	ocrHandler *OCRHandler
	validator  *PDFValidator
	logger     interfaces.Logger
}

// NewPDFProcessor creates a new PDF processor with default configuration
func NewPDFProcessor() *PDFProcessor {
	config := DefaultPDFConfig()
	return NewPDFProcessorWithConfig(config)
}

// NewPDFProcessorWithConfig creates a new PDF processor with custom configuration
func NewPDFProcessorWithConfig(config *PDFConfig) *PDFProcessor {
	// Validate configuration
	if err := config.Validate(); err != nil {
		// Use default config if validation fails
		config = DefaultPDFConfig()
	}

	// Create components
	extractor := NewTextExtractor(config)
	ocrHandler := NewOCRHandler(config)
	validator := NewPDFValidator(config)

	// Create base processor
	baseProcessor := base.NewBaseProcessor(
		base.ProcessorTypePDF,
		[]string{"pdf"},
		nil, // logger will be set later
	)
	baseProcessor.SetMaxFileSize(config.MaxFileSize)
	baseProcessor.SetTimeout(config.ProcessingTimeout)

	return &PDFProcessor{
		BaseProcessor: *baseProcessor,
		config:        config,
		extractor:     extractor,
		ocrHandler:    ocrHandler,
		validator:     validator,
	}
}

// WithLogger adds logging to the PDF processor
func (p *PDFProcessor) WithLogger(logger interfaces.Logger) *PDFProcessor {
	p.logger = logger
	p.extractor = p.extractor.WithLogger(logger)
	p.ocrHandler = p.ocrHandler.WithLogger(logger)
	p.validator = p.validator.WithLogger(logger)
	return p
}

// CanProcess determines if this processor can handle the given file
func (p *PDFProcessor) CanProcess(filePath string) bool {
	// Check file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".pdf" {
		return false
	}

	// Use base processor validation
	return p.BaseProcessor.CanProcess(filePath)
}

// Process extracts content and metadata from a PDF document
func (p *PDFProcessor) Process(ctx context.Context, filePath string) (*base.ProcessingResult, error) {
	if p.logger != nil {
		p.logger.Info(ctx, "Starting PDF processing", "file_path", filePath)
	}

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		if p.logger != nil {
			p.logger.Info(ctx, "PDF processing completed",
				"file_path", filePath,
				"duration_ms", duration.Milliseconds())
		}
	}()

	// Create processing context with timeout
	processCtx, cancel := p.BaseProcessor.CreateProcessingContext(ctx)
	defer cancel()

	// Validate the PDF file
	if err := p.validator.ValidateFile(processCtx, filePath); err != nil {
		if p.logger != nil {
			p.logger.Error(ctx, "PDF validation failed", "file_path", filePath, "error", err)
		}
		return nil, fmt.Errorf("PDF validation failed: %w", err)
	}

	// Attempt text extraction first
	textResult, textErr := p.extractText(processCtx, filePath)

	var finalText string
	var finalConfidence float64
	var extractionMethod string
	var extractionMetadata map[string]interface{}

	// Check if text extraction was successful and quality is good
	if textErr == nil && textResult != nil &&
		textResult.Quality.IsGoodQuality(p.config.TextQualityThreshold) {

		// Use direct text extraction
		finalText = textResult.Text
		finalConfidence = textResult.Quality.Confidence
		extractionMethod = "pdftotext"
		extractionMetadata = textResult.Metadata

		if p.logger != nil {
			p.logger.Info(ctx, "Using direct text extraction",
				"file_path", filePath,
				"quality_score", textResult.Quality.Score,
				"word_count", textResult.Quality.WordCount)
		}

	} else {
		// Try OCR as fallback
		if p.config.EnableOCRFallback {
			ocrResult, ocrErr := p.processWithOCR(processCtx, filePath)

			if ocrErr == nil && ocrResult != nil {
				// Compare OCR result with text extraction result
				useOCR := false

				if textErr != nil || textResult == nil {
					// Text extraction failed, use OCR
					useOCR = true
				} else if ocrResult.Quality.IsBetterThan(textResult.Quality) {
					// OCR produced better quality text
					useOCR = true
				}

				if useOCR {
					finalText = ocrResult.Text
					finalConfidence = ocrResult.Quality.Confidence
					extractionMethod = "ocrmypdf"
					extractionMetadata = ocrResult.Metadata

					if p.logger != nil {
						p.logger.Info(ctx, "Using OCR extraction",
							"file_path", filePath,
							"quality_score", ocrResult.Quality.Score,
							"word_count", ocrResult.Quality.WordCount)
					}
				} else {
					// Use text extraction despite lower quality
					finalText = textResult.Text
					finalConfidence = textResult.Quality.Confidence
					extractionMethod = "pdftotext"
					extractionMetadata = textResult.Metadata

					if p.logger != nil {
						p.logger.Info(ctx, "Using text extraction despite OCR availability",
							"file_path", filePath,
							"text_quality", textResult.Quality.Score,
							"ocr_quality", ocrResult.Quality.Score)
					}
				}
			} else {
				// OCR failed, try to use text extraction even if poor quality
				if textResult != nil {
					finalText = textResult.Text
					finalConfidence = textResult.Quality.Confidence * 0.7 // Lower confidence due to poor quality
					extractionMethod = "pdftotext_fallback"
					extractionMetadata = textResult.Metadata

					if p.logger != nil {
						p.logger.Warn(ctx, "Using poor quality text extraction as OCR failed",
							"file_path", filePath,
							"text_error", textErr,
							"ocr_error", ocrErr)
					}
				} else {
					// Both methods failed
					return nil, fmt.Errorf("both text extraction and OCR failed: text_error=%v, ocr_error=%v", textErr, ocrErr)
				}
			}
		} else {
			// OCR disabled, use text extraction even if poor quality
			if textResult != nil {
				finalText = textResult.Text
				finalConfidence = textResult.Quality.Confidence * 0.7
				extractionMethod = "pdftotext_fallback"
				extractionMetadata = textResult.Metadata
			} else {
				return nil, fmt.Errorf("text extraction failed and OCR is disabled: %w", textErr)
			}
		}
	}

	// Extract metadata
	metadata, err := p.extractMetadata(processCtx, filePath)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn(ctx, "Failed to extract metadata, using basic metadata",
				"file_path", filePath, "error", err)
		}
		// Use basic metadata if extraction fails
		metadata = p.getBasicMetadata(filePath)
	}

	// Merge extraction metadata
	if extractionMetadata != nil {
		for k, v := range extractionMetadata {
			metadata[k] = v
		}
	}

	// Add processor-specific metadata
	metadata["processor_type"] = p.GetProcessorType().String()
	metadata["processor_version"] = "1.0.0"
	metadata["extraction_method"] = extractionMethod
	metadata["extraction_confidence"] = finalConfidence
	metadata["tools_available"] = p.config.GetToolsStatus()

	// Create processing result
	result := &base.ProcessingResult{
		Content:     finalText,
		Metadata:    metadata,
		ContentType: "application/pdf",
		Language:    p.detectLanguage(finalText),
		Confidence:  finalConfidence,
	}

	if p.logger != nil {
		p.logger.Info(ctx, "PDF processing successful",
			"file_path", filePath,
			"content_length", len(finalText),
			"extraction_method", extractionMethod,
			"confidence", finalConfidence,
			"language", result.Language)
	}

	return result, nil
}

// extractText attempts to extract text using pdftotext
func (p *PDFProcessor) extractText(ctx context.Context, filePath string) (*TextExtractionResult, error) {
	if p.logger != nil {
		p.logger.Debug(ctx, "Attempting text extraction", "file_path", filePath)
	}

	return p.extractor.ExtractText(ctx, filePath)
}

// processWithOCR attempts to process PDF using OCR
func (p *PDFProcessor) processWithOCR(ctx context.Context, filePath string) (*OCRResult, error) {
	if p.logger != nil {
		p.logger.Debug(ctx, "Attempting OCR processing", "file_path", filePath)
	}

	// Check if OCR is available
	if !p.ocrHandler.CanProcessOCR() {
		return nil, &PDFError{
			Code:    ErrToolNotFound,
			Message: "OCR tool (ocrmypdf) not available",
		}
	}

	return p.ocrHandler.ProcessWithOCR(ctx, filePath)
}

// extractMetadata extracts metadata from the PDF file
func (p *PDFProcessor) extractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error) {
	// Get basic file metadata
	metadata, err := p.BaseProcessor.GetFileInfo(filePath)
	if err != nil {
		return nil, err
	}

	// Add PDF-specific metadata
	metadata["content_type"] = "application/pdf"
	metadata["file_type"] = "pdf"

	// Get validation metadata
	validationInfo, err := p.validator.GetFileInfo(filePath)
	if err == nil {
		for k, v := range validationInfo {
			metadata["validation_"+k] = v
		}
	}

	return metadata, nil
}

// getBasicMetadata returns basic metadata when full extraction fails
func (p *PDFProcessor) getBasicMetadata(filePath string) map[string]interface{} {
	metadata := map[string]interface{}{
		"file_path":    filePath,
		"file_name":    filepath.Base(filePath),
		"content_type": "application/pdf",
		"file_type":    "pdf",
		"processor":    "PDFProcessor",
	}

	// Try to get basic file info
	if info, err := p.BaseProcessor.GetFileInfo(filePath); err == nil {
		for k, v := range info {
			metadata[k] = v
		}
	}

	return metadata
}

// detectLanguage performs simple language detection on the extracted text
func (p *PDFProcessor) detectLanguage(text string) string {
	if text == "" {
		return "unknown"
	}

	// Simple heuristic based on character patterns
	// This is a basic implementation - could be enhanced with proper language detection

	// Count character patterns
	englishChars := 0
	totalChars := 0

	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			englishChars++
		}
		if r > 32 && r < 127 { // printable ASCII
			totalChars++
		}
	}

	if totalChars > 0 {
		englishRatio := float64(englishChars) / float64(totalChars)
		if englishRatio > 0.7 {
			return "en"
		}
	}

	return "unknown"
}

// GetSupportedExtensions returns the file extensions this processor supports
func (p *PDFProcessor) GetSupportedExtensions() []string {
	return []string{"pdf"}
}

// GetProcessorType returns the type of this processor
func (p *PDFProcessor) GetProcessorType() base.ProcessorType {
	return base.ProcessorTypePDF
}

// GetProcessorMetadata returns metadata about this processor
func (p *PDFProcessor) GetProcessorMetadata() map[string]interface{} {
	return map[string]interface{}{
		"name":        "PDFProcessor",
		"version":     "1.0.0",
		"type":        p.GetProcessorType().String(),
		"description": "Processes PDF files using pdftotext and ocrmypdf for text extraction",
		"capabilities": []string{
			"text_extraction",
			"ocr_processing",
			"metadata_extraction",
			"quality_assessment",
			"multi_method_extraction",
		},
		"supported_extensions": p.GetSupportedExtensions(),
		"max_file_size":        p.config.MaxFileSize,
		"processing_timeout":   p.config.ProcessingTimeout.Milliseconds(),
		"features": map[string]bool{
			"pdftotext_extraction": true,
			"ocr_fallback":         p.config.EnableOCRFallback,
			"quality_assessment":   true,
			"security_validation":  true,
			"metadata_extraction":  true,
		},
		"tools": map[string]interface{}{
			"pdftotext": map[string]interface{}{
				"path":      p.config.PDFToTextPath,
				"available": p.config.GetToolsStatus()["pdftotext"],
			},
			"ocrmypdf": map[string]interface{}{
				"path":      p.config.OCRMyPDFPath,
				"available": p.config.GetToolsStatus()["ocrmypdf"],
				"languages": p.config.OCRLanguages,
				"quality":   p.config.OCRQualityLevel,
			},
		},
		"configuration": map[string]interface{}{
			"text_quality_threshold": p.config.TextQualityThreshold,
			"min_words_per_page":     p.config.MinWordsPerPage,
			"enable_ocr_fallback":    p.config.EnableOCRFallback,
			"max_pages":              p.config.MaxPages,
		},
	}
}

// EstimateProcessingComplexity estimates the computational complexity of processing a PDF
func (p *PDFProcessor) EstimateProcessingComplexity(filePath string) (int, error) {
	// Get file info
	info, err := p.BaseProcessor.GetFileInfo(filePath)
	if err != nil {
		return 0, err
	}

	fileSize, ok := info["size"].(int64)
	if !ok {
		return 5, nil // Default medium complexity
	}

	// Complexity based on file size and available tools
	var complexity int

	switch {
	case fileSize < 1024*1024: // < 1MB
		complexity = 2
	case fileSize < 5*1024*1024: // < 5MB
		complexity = 4
	case fileSize < 20*1024*1024: // < 20MB
		complexity = 6
	case fileSize < 50*1024*1024: // < 50MB
		complexity = 8
	default: // >= 50MB
		complexity = 10
	}

	// Adjust based on available tools
	toolsStatus := p.config.GetToolsStatus()
	if !toolsStatus["pdftotext"] {
		complexity += 2 // Harder without pdftotext
	}
	if p.config.EnableOCRFallback && toolsStatus["ocrmypdf"] {
		complexity += 1 // OCR adds complexity
	}

	// Ensure complexity stays within bounds
	if complexity > 10 {
		complexity = 10
	}
	if complexity < 1 {
		complexity = 1
	}

	return complexity, nil
}
