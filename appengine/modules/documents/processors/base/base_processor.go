package base

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"arcadia/modules/documents/interfaces"
)

// BaseProcessor provides common functionality for all document processors
type BaseProcessor struct {
	logger        interfaces.Logger
	processorType ProcessorType
	extensions    []string
	maxFileSize   int64
	timeout       time.Duration
}

// NewBaseProcessor creates a new base processor with the given configuration
func NewBaseProcessor(processorType ProcessorType, extensions []string, logger interfaces.Logger) *BaseProcessor {
	return &BaseProcessor{
		logger:        logger,
		processorType: processorType,
		extensions:    extensions,
		maxFileSize:   100 * 1024 * 1024, // 100MB default
		timeout:       5 * time.Minute,   // 5 minutes default
	}
}

// GetProcessorType returns the type of this processor
func (bp *BaseProcessor) GetProcessorType() ProcessorType {
	return bp.processorType
}

// GetSupportedExtensions returns the file extensions this processor supports
func (bp *BaseProcessor) GetSupportedExtensions() []string {
	return bp.extensions
}

// CanProcess determines if this processor can handle the given file based on extension
func (bp *BaseProcessor) CanProcess(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != "" && ext[0] == '.' {
		ext = ext[1:] // Remove the dot
	}

	for _, supportedExt := range bp.extensions {
		if ext == supportedExt {
			return true
		}
	}
	return false
}

// ValidateFile performs common file validation
func (bp *BaseProcessor) ValidateFile(filePath string) error {
	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", filePath)
		}
		return fmt.Errorf("error accessing file %s: %w", filePath, err)
	}

	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return fmt.Errorf("path is not a regular file: %s", filePath)
	}

	// Check file size
	if info.Size() > bp.maxFileSize {
		return fmt.Errorf("file too large: %d bytes (max: %d bytes)", info.Size(), bp.maxFileSize)
	}

	// Check if we can process this file type
	if !bp.CanProcess(filePath) {
		return fmt.Errorf("unsupported file type: %s", filepath.Ext(filePath))
	}

	return nil
}

// LogProcessingStart logs the start of processing
func (bp *BaseProcessor) LogProcessingStart(ctx context.Context, filePath string) {
	if bp.logger != nil {
		logger := bp.logger.WithFields(map[string]interface{}{
			"file":           filePath,
			"processor_type": bp.processorType,
		})
		logger.Info(ctx, "Starting document processing")
	}
}

// LogProcessingEnd logs the completion of processing
func (bp *BaseProcessor) LogProcessingEnd(ctx context.Context, filePath string, duration time.Duration, err error) {
	if bp.logger != nil {
		fields := map[string]interface{}{
			"file":           filePath,
			"processor_type": bp.processorType,
			"duration_ms":    duration.Milliseconds(),
		}

		logger := bp.logger.WithFields(fields)
		if err != nil {
			logger.Error(ctx, "Document processing failed", "error", err.Error())
		} else {
			logger.Info(ctx, "Document processing completed")
		}
	}
}

// CreateProcessingContext creates a context with timeout for processing
func (bp *BaseProcessor) CreateProcessingContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, bp.timeout)
}

// SetMaxFileSize sets the maximum file size this processor will handle
func (bp *BaseProcessor) SetMaxFileSize(size int64) {
	bp.maxFileSize = size
}

// SetTimeout sets the processing timeout
func (bp *BaseProcessor) SetTimeout(timeout time.Duration) {
	bp.timeout = timeout
}

// GetFileInfo returns basic information about the file
func (bp *BaseProcessor) GetFileInfo(filePath string) (map[string]interface{}, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"size":      info.Size(),
		"mod_time":  info.ModTime(),
		"mode":      info.Mode().String(),
		"extension": filepath.Ext(filePath),
		"base_name": filepath.Base(filePath),
		"processor": bp.processorType,
	}, nil
}
