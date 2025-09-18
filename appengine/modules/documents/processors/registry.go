package processors

import (
	"context"
	"fmt"
	"sync"

	"arcadia/modules/documents/processors/base"
	"arcadia/modules/documents/processors/text"
	"arcadia/modules/documents/processors/pdf"
	"arcadia/modules/documents/detection"
	"arcadia/modules/documents/interfaces"
)

// Registry manages document processors and handles document processing
type Registry struct {
	processors map[base.ProcessorType]base.DocumentProcessor
	detector   detection.MultiStageDetector
	logger     interfaces.Logger
	metrics    interfaces.MetricsCollector
	mu         sync.RWMutex
}

// NewRegistry creates a new processor registry
func NewRegistry(detector detection.MultiStageDetector) *Registry {
	return &Registry{
		processors: make(map[base.ProcessorType]base.DocumentProcessor),
		detector:   detector,
		mu:         sync.RWMutex{},
	}
}

// WithLogger adds logging to the registry
func (r *Registry) WithLogger(logger interfaces.Logger) *Registry {
	r.logger = logger
	return r
}

// WithMetrics adds metrics collection to the registry
func (r *Registry) WithMetrics(metrics interfaces.MetricsCollector) *Registry {
	r.metrics = metrics
	return r
}

// RegisterProcessor registers a new document processor
func (r *Registry) RegisterProcessor(processorType base.ProcessorType, processor base.DocumentProcessor) error {
	if processor == nil {
		return fmt.Errorf("processor cannot be nil")
	}

	if !processorType.IsValid() {
		return fmt.Errorf("invalid processor type: %s", processorType)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.processors[processorType]; exists {
		if r.logger != nil {
			r.logger.Warn(context.Background(), "Overwriting existing processor", "processor_type", processorType)
		}
	}

	r.processors[processorType] = processor

	if r.logger != nil {
		r.logger.Info(context.Background(), "Registered processor",
			"processor_type", processorType,
			"supported_extensions", processor.GetSupportedExtensions())
	}

	if r.metrics != nil {
		r.metrics.IncrementCounter("processors_registered", map[string]string{
			"processor_type": processorType.String(),
		})
	}

	return nil
}

// GetProcessor retrieves a processor by type
func (r *Registry) GetProcessor(processorType base.ProcessorType) (base.DocumentProcessor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	processor, exists := r.processors[processorType]
	return processor, exists
}

// GetSupportedTypes returns all processor types that are registered
func (r *Registry) GetSupportedTypes() []base.ProcessorType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]base.ProcessorType, 0, len(r.processors))
	for processorType := range r.processors {
		types = append(types, processorType)
	}

	return types
}

// ProcessDocument processes a document using the appropriate processor
func (r *Registry) ProcessDocument(ctx context.Context, filePath string) (*base.ProcessingResult, error) {
	if r.metrics != nil {
		timer := r.metrics.StartTimer("document_processing_duration", map[string]string{
			"operation": "process_document",
		})
		defer timer.Stop()
	}

	// Detect document type
	docType, err := r.detector.DetectDocumentType(filePath)
	if err != nil {
		if r.logger != nil {
			r.logger.Error(ctx, "Failed to detect document type",
				"file_path", filePath,
				"error", err)
		}
		if r.metrics != nil {
			r.metrics.IncrementCounter("document_processing_errors", map[string]string{
				"error_type": "detection_failed",
			})
		}
		return nil, fmt.Errorf("failed to detect document type for %s: %w", filePath, err)
	}

	if docType == nil {
		if r.logger != nil {
			r.logger.Warn(ctx, "Document type could not be determined", "file_path", filePath)
		}
		if r.metrics != nil {
			r.metrics.IncrementCounter("document_processing_errors", map[string]string{
				"error_type": "unsupported_type",
			})
		}
		return nil, fmt.Errorf("unsupported document type for %s", filePath)
	}

	// Get the appropriate processor
	processor, exists := r.GetProcessor(docType.Type)
	if !exists {
		if r.logger != nil {
			r.logger.Error(ctx, "No processor found for document type",
				"file_path", filePath,
				"processor_type", docType.Type)
		}
		if r.metrics != nil {
			r.metrics.IncrementCounter("document_processing_errors", map[string]string{
				"error_type": "processor_not_found",
				"processor_type": docType.Type.String(),
			})
		}
		return nil, fmt.Errorf("no processor registered for type %s", docType.Type)
	}

	// Double-check that the processor can handle this file
	if !processor.CanProcess(filePath) {
		if r.logger != nil {
			r.logger.Error(ctx, "Processor cannot handle file",
				"file_path", filePath,
				"processor_type", docType.Type)
		}
		if r.metrics != nil {
			r.metrics.IncrementCounter("document_processing_errors", map[string]string{
				"error_type": "processor_cannot_handle",
				"processor_type": docType.Type.String(),
			})
		}
		return nil, fmt.Errorf("processor %s cannot handle file %s", docType.Type, filePath)
	}

	// Process the document
	if r.logger != nil {
		r.logger.Debug(ctx, "Processing document",
			"file_path", filePath,
			"processor_type", docType.Type,
			"detection_confidence", docType.Confidence)
	}

	result, err := processor.Process(ctx, filePath)
	if err != nil {
		if r.logger != nil {
			r.logger.Error(ctx, "Failed to process document",
				"file_path", filePath,
				"processor_type", docType.Type,
				"error", err)
		}
		if r.metrics != nil {
			r.metrics.IncrementCounter("document_processing_errors", map[string]string{
				"error_type": "processing_failed",
				"processor_type": docType.Type.String(),
			})
		}
		return nil, fmt.Errorf("failed to process document %s with processor %s: %w", filePath, docType.Type, err)
	}

	if r.logger != nil {
		r.logger.Info(ctx, "Successfully processed document",
			"file_path", filePath,
			"processor_type", docType.Type,
			"content_length", len(result.Content),
			"confidence", result.Confidence)
	}

	if r.metrics != nil {
		r.metrics.IncrementCounter("documents_processed", map[string]string{
			"processor_type": docType.Type.String(),
		})
		r.metrics.RecordHistogram("document_content_length", float64(len(result.Content)), map[string]string{
			"processor_type": docType.Type.String(),
		})
	}

	return result, nil
}

// GetProcessorStats returns statistics about registered processors
func (r *Registry) GetProcessorStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_processors"] = len(r.processors)

	processorInfo := make(map[string]interface{})
	for processorType, processor := range r.processors {
		processorInfo[processorType.String()] = map[string]interface{}{
			"supported_extensions": processor.GetSupportedExtensions(),
			"processor_type":       processor.GetProcessorType().String(),
		}
	}
	stats["processors"] = processorInfo

	return stats
}

// CanProcessFile checks if any registered processor can handle the given file
func (r *Registry) CanProcessFile(filePath string) bool {
	docType, err := r.detector.DetectDocumentType(filePath)
	if err != nil || docType == nil {
		return false
	}

	processor, exists := r.GetProcessor(docType.Type)
	if !exists {
		return false
	}

	return processor.CanProcess(filePath)
}

// GetProcessorForFile returns the processor that would be used for the given file
func (r *Registry) GetProcessorForFile(filePath string) (base.DocumentProcessor, base.ProcessorType, error) {
	docType, err := r.detector.DetectDocumentType(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to detect document type: %w", err)
	}

	if docType == nil {
		return nil, "", fmt.Errorf("unsupported document type")
	}

	processor, exists := r.GetProcessor(docType.Type)
	if !exists {
		return nil, docType.Type, fmt.Errorf("no processor registered for type %s", docType.Type)
	}

	return processor, docType.Type, nil
}

// InitializeWithDefaults initializes the registry with default processors
func (r *Registry) InitializeWithDefaults() error {
	// Register text processor
	textProcessor := text.NewTextProcessor()
	if r.logger != nil {
		textProcessor = textProcessor.WithLogger(r.logger)
	}

	if err := r.RegisterProcessor(base.ProcessorTypeText, textProcessor); err != nil {
		return fmt.Errorf("failed to register text processor: %w", err)
	}

	// Register PDF processor
	pdfProcessor := pdf.NewPDFProcessor()
	if r.logger != nil {
		pdfProcessor = pdfProcessor.WithLogger(r.logger)
	}

	if err := r.RegisterProcessor(base.ProcessorTypePDF, pdfProcessor); err != nil {
		return fmt.Errorf("failed to register PDF processor: %w", err)
	}

	if r.logger != nil {
		r.logger.Info(context.Background(), "Registry initialized with default processors",
			"processors_count", len(r.processors))
	}

	return nil
}

// NewRegistryWithDefaults creates a registry initialized with default processors
func NewRegistryWithDefaults(detector detection.MultiStageDetector) (*Registry, error) {
	registry := NewRegistry(detector)

	if err := registry.InitializeWithDefaults(); err != nil {
		return nil, fmt.Errorf("failed to initialize registry with defaults: %w", err)
	}

	return registry, nil
}