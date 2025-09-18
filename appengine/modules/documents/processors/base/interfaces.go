package base

import (
	"context"
)

// DocumentProcessor defines the interface that all document processors must implement
type DocumentProcessor interface {
	// CanProcess determines if this processor can handle the given file
	CanProcess(filePath string) bool

	// Process extracts content and metadata from the document
	Process(ctx context.Context, filePath string) (*ProcessingResult, error)

	// GetSupportedExtensions returns the file extensions this processor supports
	GetSupportedExtensions() []string

	// GetProcessorType returns the type of processor
	GetProcessorType() ProcessorType
}

// ProcessingResult contains the results of document processing
type ProcessingResult struct {
	// Content is the extracted text content
	Content string

	// Metadata contains document metadata
	Metadata map[string]interface{}

	// ContentType indicates the MIME type of the content
	ContentType string

	// Language is the detected language of the content
	Language string

	// Confidence is the confidence level of the processing (0.0 to 1.0)
	Confidence float64
}