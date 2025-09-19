package tika

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"arcadia/modules/documents/processors/base"
	"arcadia/modules/documents/interfaces"
)

// TikaProcessor handles processing of documents using Apache Tika server
type TikaProcessor struct {
	base.BaseProcessor
	client           *TikaClient
	knownOfficeExts  map[string]bool  // High confidence formats
	acceptAllFormats bool             // Fallback mode flag
	config           *TikaConfig
	logger           interfaces.Logger
}

// NewTikaProcessor creates a new Tika processor with default configuration (Office-only mode)
func NewTikaProcessor(logger interfaces.Logger) (*TikaProcessor, error) {
	config := DefaultTikaConfig()
	return NewTikaProcessorWithConfig(config, logger)
}

// NewTikaProcessorWithFallback creates a new Tika processor configured for fallback mode
func NewTikaProcessorWithFallback(logger interfaces.Logger) (*TikaProcessor, error) {
	config := DefaultTikaConfig()
	config.AcceptAllFormats = true // Enable fallback mode
	return NewTikaProcessorWithConfig(config, logger)
}

// NewTikaProcessorWithConfig creates a new Tika processor with custom configuration
func NewTikaProcessorWithConfig(config *TikaConfig, logger interfaces.Logger) (*TikaProcessor, error) {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid tika config: %w", err)
	}

	// Create Tika client
	client := NewTikaClient(config, logger)

	// Create map of known Office extensions for fast lookup
	knownOfficeExts := make(map[string]bool)
	for _, ext := range config.OfficeExtensions {
		knownOfficeExts[strings.ToLower(ext)] = true
	}

	// Determine supported extensions based on mode
	var supportedExtensions []string
	if config.AcceptAllFormats {
		// In fallback mode, we technically support "any" extension, but we need to provide
		// a list for the base processor. We'll use a broad list of common extensions.
		supportedExtensions = []string{
			"doc", "docx", "xls", "xlsx", "ppt", "pptx", "odt", "ods", "odp", "rtf",
			"txt", "csv", "html", "htm", "xml", "json", "pdf", "ps", "eps",
		}
	} else {
		// Office-only mode
		supportedExtensions = config.OfficeExtensions
	}

	// Create base processor
	baseProcessor := base.NewBaseProcessor(
		base.ProcessorTypeTika,
		supportedExtensions,
		logger,
	)
	baseProcessor.SetMaxFileSize(config.MaxFileSize)
	baseProcessor.SetTimeout(config.Timeout)

	processor := &TikaProcessor{
		BaseProcessor:    *baseProcessor,
		client:           client,
		knownOfficeExts:  knownOfficeExts,
		acceptAllFormats: config.AcceptAllFormats,
		config:           config,
		logger:           logger,
	}

	return processor, nil
}

// WithLogger adds logging to the Tika processor
func (p *TikaProcessor) WithLogger(logger interfaces.Logger) *TikaProcessor {
	p.logger = logger
	// Note: We don't recreate the client here as it already has the logger
	return p
}

// CanProcess determines if this processor can handle the given file
func (p *TikaProcessor) CanProcess(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != "" && ext[0] == '.' {
		ext = ext[1:] // Remove the dot
	}

	// In Office-only mode, only accept known Office extensions
	if !p.acceptAllFormats {
		return p.knownOfficeExts[ext]
	}

	// In fallback mode, we can attempt to process any file
	// However, we should still validate basic file properties
	if err := p.BaseProcessor.ValidateFile(filePath); err != nil {
		if p.logger != nil {
			p.logger.Debug(context.Background(), "File validation failed for Tika processing",
				"file", filePath, "error", err.Error())
		}
		return false
	}

	return true
}

// Process extracts content and metadata from a document using Tika
func (p *TikaProcessor) Process(ctx context.Context, filePath string) (*base.ProcessingResult, error) {
	if p.logger != nil {
		p.logger.Info(ctx, "Starting Tika processing", "file_path", filePath, "mode", p.getProcessingMode())
	}

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		if p.logger != nil {
			p.logger.Info(ctx, "Tika processing completed",
				"file_path", filePath,
				"duration_ms", duration.Milliseconds())
		}
	}()

	// Create processing context with timeout
	processCtx, cancel := p.BaseProcessor.CreateProcessingContext(ctx)
	defer cancel()

	// Validate the file
	if err := p.BaseProcessor.ValidateFile(filePath); err != nil {
		if p.logger != nil {
			p.logger.Error(ctx, "File validation failed", "file_path", filePath, "error", err)
		}
		return nil, fmt.Errorf("file validation failed: %w", err)
	}

	// Perform health check on Tika server
	if err := p.client.HealthCheck(processCtx); err != nil {
		if p.logger != nil {
			p.logger.Error(ctx, "Tika server health check failed", "error", err)
		}
		return nil, fmt.Errorf("tika server unavailable: %w", err)
	}

	// Extract text content
	content, err := p.client.ExtractText(processCtx, filePath)
	if err != nil {
		if p.logger != nil {
			p.logger.Error(ctx, "Text extraction failed", "file_path", filePath, "error", err)
		}
		return nil, fmt.Errorf("text extraction failed: %w", err)
	}

	// Extract metadata
	metadata, err := p.client.ExtractMetadata(processCtx, filePath)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn(ctx, "Metadata extraction failed, using basic metadata",
				"file_path", filePath, "error", err)
		}
		// Use basic metadata if extraction fails
		metadata = p.getBasicMetadata(filePath)
	}

	// Detect content type
	contentType, err := p.client.DetectType(processCtx, filePath)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn(ctx, "Content type detection failed, using default",
				"file_path", filePath, "error", err)
		}
		contentType = "application/octet-stream" // Default content type
	}

	// Determine confidence based on file extension and processing mode
	confidence := p.calculateConfidence(filePath)

	// Add processor-specific metadata
	metadata["processor_type"] = p.GetProcessorType().String()
	metadata["processor_version"] = "1.0.0"
	metadata["processing_mode"] = p.getProcessingMode()
	metadata["tika_server_url"] = p.config.ServerURL
	metadata["extraction_confidence"] = confidence
	metadata["circuit_breaker_stats"] = p.client.GetCircuitBreakerStats()

	// Add file information
	if fileInfo, err := p.BaseProcessor.GetFileInfo(filePath); err == nil {
		for k, v := range fileInfo {
			metadata["file_"+k] = v
		}
	}

	// Create processing result
	result := &base.ProcessingResult{
		Content:     content,
		Metadata:    metadata,
		ContentType: contentType,
		Language:    p.detectLanguage(content),
		Confidence:  confidence,
	}

	if p.logger != nil {
		p.logger.Info(ctx, "Tika processing successful",
			"file_path", filePath,
			"content_length", len(content),
			"content_type", contentType,
			"confidence", confidence,
			"language", result.Language)
	}

	return result, nil
}

// calculateConfidence determines the confidence level based on file extension and mode
func (p *TikaProcessor) calculateConfidence(filePath string) float64 {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != "" && ext[0] == '.' {
		ext = ext[1:] // Remove the dot
	}

	// High confidence for known Office formats
	if p.knownOfficeExts[ext] {
		return p.config.OfficeConfidence
	}

	// Low confidence for fallback processing
	if p.acceptAllFormats {
		return p.config.FallbackConfidence
	}

	// This shouldn't happen if CanProcess is working correctly
	return 0.1
}

// getProcessingMode returns a string describing the current processing mode
func (p *TikaProcessor) getProcessingMode() string {
	if p.acceptAllFormats {
		return "fallback"
	}
	return "office"
}

// getBasicMetadata returns basic metadata when full extraction fails
func (p *TikaProcessor) getBasicMetadata(filePath string) map[string]interface{} {
	metadata := map[string]interface{}{
		"file_path":    filePath,
		"file_name":    filepath.Base(filePath),
		"processor":    "TikaProcessor",
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
func (p *TikaProcessor) detectLanguage(text string) string {
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
func (p *TikaProcessor) GetSupportedExtensions() []string {
	return p.BaseProcessor.GetSupportedExtensions()
}

// GetProcessorType returns the type of this processor
func (p *TikaProcessor) GetProcessorType() base.ProcessorType {
	return base.ProcessorTypeTika
}

// GetProcessorMetadata returns metadata about this processor
func (p *TikaProcessor) GetProcessorMetadata() map[string]interface{} {
	return map[string]interface{}{
		"name":        "TikaProcessor",
		"version":     "1.0.0",
		"type":        p.GetProcessorType().String(),
		"description": "Processes documents using Apache Tika server with dual-mode support",
		"capabilities": []string{
			"text_extraction",
			"metadata_extraction",
			"content_type_detection",
			"dual_mode_processing",
			"circuit_breaker_protection",
		},
		"supported_extensions": p.GetSupportedExtensions(),
		"max_file_size":       p.config.MaxFileSize,
		"processing_timeout":  p.config.Timeout.Milliseconds(),
		"features": map[string]bool{
			"office_mode":            !p.acceptAllFormats,
			"fallback_mode":         p.acceptAllFormats,
			"circuit_breaker":       true,
			"metadata_extraction":   true,
			"content_type_detection": true,
			"retry_mechanism":       true,
		},
		"configuration": map[string]interface{}{
			"server_url":          p.config.ServerURL,
			"accept_all_formats":  p.acceptAllFormats,
			"office_confidence":   p.config.OfficeConfidence,
			"fallback_confidence": p.config.FallbackConfidence,
			"max_retries":         p.config.MaxRetries,
			"processing_mode":     p.getProcessingMode(),
		},
		"circuit_breaker": map[string]interface{}{
			"failure_threshold":      p.config.CircuitBreaker.FailureThreshold,
			"reset_timeout_seconds":  p.config.CircuitBreaker.ResetTimeout.Seconds(),
			"half_open_max_requests": p.config.CircuitBreaker.HalfOpenMaxRequests,
		},
	}
}

// EstimateProcessingComplexity estimates the computational complexity of processing a document
func (p *TikaProcessor) EstimateProcessingComplexity(filePath string) (int, error) {
	// Get file info
	info, err := p.BaseProcessor.GetFileInfo(filePath)
	if err != nil {
		return 0, err
	}

	fileSize, ok := info["size"].(int64)
	if !ok {
		return 5, nil // Default medium complexity
	}

	// Complexity based on file size and processing mode
	var complexity int

	switch {
	case fileSize < 1024*1024: // < 1MB
		complexity = 3
	case fileSize < 5*1024*1024: // < 5MB
		complexity = 5
	case fileSize < 20*1024*1024: // < 20MB
		complexity = 7
	case fileSize < 50*1024*1024: // < 50MB
		complexity = 9
	default: // >= 50MB
		complexity = 10
	}

	// Adjust based on processing mode
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != "" && ext[0] == '.' {
		ext = ext[1:]
	}

	if p.knownOfficeExts[ext] {
		// Office documents are generally well-structured and easier to process
		complexity = complexity - 1
	} else if p.acceptAllFormats {
		// Unknown formats might be more challenging
		complexity = complexity + 1
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

// SetAcceptAllFormats enables or disables fallback mode
func (p *TikaProcessor) SetAcceptAllFormats(accept bool) {
	p.acceptAllFormats = accept
	p.config.AcceptAllFormats = accept
}

// IsInFallbackMode returns true if the processor is in fallback mode
func (p *TikaProcessor) IsInFallbackMode() bool {
	return p.acceptAllFormats
}

// GetCircuitBreakerStats returns current circuit breaker statistics
func (p *TikaProcessor) GetCircuitBreakerStats() CircuitBreakerStats {
	return p.client.GetCircuitBreakerStats()
}

// ResetCircuitBreaker manually resets the circuit breaker
func (p *TikaProcessor) ResetCircuitBreaker() {
	p.client.ResetCircuitBreaker()
}