package tika

import (
	"context"
	"fmt"
	"log"

	"arcadia/modules/documents/interfaces"
)

// ExampleLogger is a simple logger implementation for examples
type ExampleLogger struct{}

func (e *ExampleLogger) Debug(ctx context.Context, msg string, fields ...interface{}) {
	log.Printf("DEBUG: %s %v", msg, fields)
}

func (e *ExampleLogger) Info(ctx context.Context, msg string, fields ...interface{}) {
	log.Printf("INFO: %s %v", msg, fields)
}

func (e *ExampleLogger) Warn(ctx context.Context, msg string, fields ...interface{}) {
	log.Printf("WARN: %s %v", msg, fields)
}

func (e *ExampleLogger) Error(ctx context.Context, msg string, fields ...interface{}) {
	log.Printf("ERROR: %s %v", msg, fields)
}

func (e *ExampleLogger) Fatal(ctx context.Context, msg string, fields ...interface{}) {
	log.Fatalf("FATAL: %s %v", msg, fields)
}

func (e *ExampleLogger) WithFields(fields map[string]interface{}) interfaces.Logger {
	return e
}

func (e *ExampleLogger) WithContext(ctx context.Context) interfaces.Logger {
	return e
}

func (e *ExampleLogger) WithModule(module string) interfaces.Logger {
	return e
}

func (e *ExampleLogger) WithComponent(component string) interfaces.Logger {
	return e
}

// ExampleOfficeDocumentProcessing demonstrates processing Office documents with high confidence
func ExampleOfficeDocumentProcessing() {
	logger := &ExampleLogger{}

	// Create Tika processor in Office-only mode (default)
	processor, err := NewTikaProcessor(logger)
	if err != nil {
		log.Fatalf("Failed to create Tika processor: %v", err)
	}

	// Process an Office document
	ctx := context.Background()
	result, err := processor.Process(ctx, "/path/to/document.docx")
	if err != nil {
		log.Printf("Processing failed: %v", err)
		return
	}

	fmt.Printf("Extracted content: %s\n", result.Content)
	fmt.Printf("Content type: %s\n", result.ContentType)
	fmt.Printf("Confidence: %.2f\n", result.Confidence)
	fmt.Printf("Language: %s\n", result.Language)
}

// ExampleFallbackProcessing demonstrates using Tika as a fallback processor
func ExampleFallbackProcessing() {
	logger := &ExampleLogger{}

	// Create Tika processor in fallback mode
	processor, err := NewTikaProcessorWithFallback(logger)
	if err != nil {
		log.Fatalf("Failed to create Tika processor with fallback: %v", err)
	}

	// Process an unknown format document
	ctx := context.Background()
	result, err := processor.Process(ctx, "/path/to/unknown-format.file")
	if err != nil {
		log.Printf("Processing failed: %v", err)
		return
	}

	fmt.Printf("Extracted content: %s\n", result.Content)
	fmt.Printf("Content type: %s\n", result.ContentType)
	fmt.Printf("Confidence: %.2f (low confidence for unknown formats)\n", result.Confidence)
}

// ExampleCustomConfiguration demonstrates creating a processor with custom configuration
func ExampleCustomConfiguration() {
	logger := &ExampleLogger{}

	// Create custom configuration
	config := DefaultTikaConfig()
	config.ServerURL = "http://custom-tika-server:9998"
	config.Timeout = 60 * 1000 // 60 seconds
	config.AcceptAllFormats = true
	config.OfficeConfidence = 0.95
	config.FallbackConfidence = 0.2

	// Create processor with custom configuration
	processor, err := NewTikaProcessorWithConfig(config, logger)
	if err != nil {
		log.Fatalf("Failed to create Tika processor with custom config: %v", err)
	}

	// Get processor metadata
	metadata := processor.GetProcessorMetadata()
	fmt.Printf("Processor configuration: %+v\n", metadata["configuration"])
}

// ExampleCircuitBreakerUsage demonstrates circuit breaker functionality
func ExampleCircuitBreakerUsage() {
	logger := &ExampleLogger{}

	processor, err := NewTikaProcessor(logger)
	if err != nil {
		log.Fatalf("Failed to create Tika processor: %v", err)
	}

	// Get circuit breaker statistics
	stats := processor.GetCircuitBreakerStats()
	fmt.Printf("Circuit breaker state: %s\n", stats.State)
	fmt.Printf("Failure count: %d\n", stats.Failures)
	fmt.Printf("Half-open requests: %d\n", stats.HalfOpenRequests)

	// Reset circuit breaker if needed
	if stats.State == StateOpen {
		processor.ResetCircuitBreaker()
		fmt.Println("Circuit breaker reset")
	}
}

// ExampleDualModeComparison demonstrates the difference between office and fallback modes
func ExampleDualModeComparison() {
	logger := &ExampleLogger{}

	// Office-only processor
	officeProcessor, _ := NewTikaProcessor(logger)
	fmt.Printf("Office mode - Supported extensions: %v\n", officeProcessor.GetSupportedExtensions())
	fmt.Printf("Office mode - Can process .docx: %t\n", officeProcessor.CanProcess("test.docx"))
	fmt.Printf("Office mode - Can process .txt: %t\n", officeProcessor.CanProcess("test.txt"))

	// Fallback processor
	fallbackProcessor, _ := NewTikaProcessorWithFallback(logger)
	fmt.Printf("Fallback mode - Supported extensions: %v\n", fallbackProcessor.GetSupportedExtensions())
	fmt.Printf("Fallback mode - Can process .docx: %t\n", fallbackProcessor.CanProcess("test.docx"))
	fmt.Printf("Fallback mode - Can process .txt: %t\n", fallbackProcessor.CanProcess("test.txt"))

	// Mode detection
	fmt.Printf("Office processor mode: %s\n", officeProcessor.getProcessingMode())
	fmt.Printf("Fallback processor mode: %s\n", fallbackProcessor.getProcessingMode())
}
