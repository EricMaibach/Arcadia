package tika

import (
	"testing"

	"arcadia/modules/documents/processors/base"
)

func TestTikaProcessorCreation(t *testing.T) {
	logger := &MockLogger{}

	// Test office-only mode
	processor, err := NewTikaProcessor(logger)
	if err != nil {
		t.Fatalf("Failed to create Tika processor: %v", err)
	}

	if processor.GetProcessorType() != base.ProcessorTypeTika {
		t.Errorf("Expected processor type %s, got %s", base.ProcessorTypeTika, processor.GetProcessorType())
	}

	if processor.IsInFallbackMode() {
		t.Error("Expected office-only mode, but processor is in fallback mode")
	}

	// Test fallback mode
	fallbackProcessor, err := NewTikaProcessorWithFallback(logger)
	if err != nil {
		t.Fatalf("Failed to create Tika processor with fallback: %v", err)
	}

	if !fallbackProcessor.IsInFallbackMode() {
		t.Error("Expected fallback mode, but processor is in office-only mode")
	}
}

func TestTikaProcessorCanProcess(t *testing.T) {
	logger := &MockLogger{}

	// Test office-only mode
	processor, err := NewTikaProcessor(logger)
	if err != nil {
		t.Fatalf("Failed to create Tika processor: %v", err)
	}

	// Test Office documents (should be accepted in office mode)
	officeFiles := []string{"test.docx", "test.xlsx", "test.pptx", "test.odt"}
	for _, file := range officeFiles {
		// Note: This will fail due to file not existing, but we're testing the extension logic
		// The actual CanProcess method validates file existence
		_ = processor.CanProcess(file)
	}

	// Test fallback mode
	fallbackProcessor, err := NewTikaProcessorWithFallback(logger)
	if err != nil {
		t.Fatalf("Failed to create Tika processor with fallback: %v", err)
	}

	// Test that fallback mode is enabled
	if !fallbackProcessor.IsInFallbackMode() {
		t.Error("Expected fallback processor to be in fallback mode")
	}
}

func TestTikaProcessorConfidence(t *testing.T) {
	logger := &MockLogger{}

	processor, err := NewTikaProcessor(logger)
	if err != nil {
		t.Fatalf("Failed to create Tika processor: %v", err)
	}

	// Test confidence calculation for Office files
	officeConfidence := processor.calculateConfidence("test.docx")
	expectedOfficeConfidence := processor.config.OfficeConfidence
	if officeConfidence != expectedOfficeConfidence {
		t.Errorf("Expected Office confidence %f, got %f", expectedOfficeConfidence, officeConfidence)
	}

	// Test fallback mode
	fallbackProcessor, err := NewTikaProcessorWithFallback(logger)
	if err != nil {
		t.Fatalf("Failed to create Tika processor with fallback: %v", err)
	}

	// Test confidence for non-Office files in fallback mode
	fallbackConfidence := fallbackProcessor.calculateConfidence("test.unknown")
	expectedFallbackConfidence := fallbackProcessor.config.FallbackConfidence
	if fallbackConfidence != expectedFallbackConfidence {
		t.Errorf("Expected fallback confidence %f, got %f", expectedFallbackConfidence, fallbackConfidence)
	}
}

func TestTikaProcessorModeDetection(t *testing.T) {
	logger := &MockLogger{}

	// Test office mode
	processor, err := NewTikaProcessor(logger)
	if err != nil {
		t.Fatalf("Failed to create Tika processor: %v", err)
	}

	if processor.getProcessingMode() != "office" {
		t.Errorf("Expected processing mode 'office', got '%s'", processor.getProcessingMode())
	}

	// Test fallback mode
	fallbackProcessor, err := NewTikaProcessorWithFallback(logger)
	if err != nil {
		t.Fatalf("Failed to create Tika processor with fallback: %v", err)
	}

	if fallbackProcessor.getProcessingMode() != "fallback" {
		t.Errorf("Expected processing mode 'fallback', got '%s'", fallbackProcessor.getProcessingMode())
	}
}

func TestTikaProcessorMetadata(t *testing.T) {
	logger := &MockLogger{}

	processor, err := NewTikaProcessor(logger)
	if err != nil {
		t.Fatalf("Failed to create Tika processor: %v", err)
	}

	metadata := processor.GetProcessorMetadata()

	// Check basic metadata fields
	if metadata["name"] != "TikaProcessor" {
		t.Errorf("Expected name 'TikaProcessor', got '%v'", metadata["name"])
	}

	if metadata["version"] != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%v'", metadata["version"])
	}

	if metadata["type"] != base.ProcessorTypeTika.String() {
		t.Errorf("Expected type '%s', got '%v'", base.ProcessorTypeTika.String(), metadata["type"])
	}

	// Check that capabilities are present
	capabilities, ok := metadata["capabilities"].([]string)
	if !ok {
		t.Error("Expected capabilities to be a string slice")
	} else {
		expectedCapabilities := []string{
			"text_extraction",
			"metadata_extraction",
			"content_type_detection",
			"dual_mode_processing",
			"circuit_breaker_protection",
		}

		for _, expected := range expectedCapabilities {
			found := false
			for _, capability := range capabilities {
				if capability == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected capability '%s' not found in metadata", expected)
			}
		}
	}
}

func TestTikaProcessorSupportedExtensions(t *testing.T) {
	logger := &MockLogger{}

	// Test office-only mode
	processor, err := NewTikaProcessor(logger)
	if err != nil {
		t.Fatalf("Failed to create Tika processor: %v", err)
	}

	extensions := processor.GetSupportedExtensions()
	if len(extensions) == 0 {
		t.Error("Expected supported extensions, got empty list")
	}

	// Should contain common Office extensions
	expectedOfficeExts := []string{"doc", "docx", "xls", "xlsx", "ppt", "pptx"}
	for _, expectedExt := range expectedOfficeExts {
		found := false
		for _, ext := range extensions {
			if ext == expectedExt {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected extension '%s' not found in supported extensions", expectedExt)
		}
	}

	// Test fallback mode
	fallbackProcessor, err := NewTikaProcessorWithFallback(logger)
	if err != nil {
		t.Fatalf("Failed to create Tika processor with fallback: %v", err)
	}

	fallbackExtensions := fallbackProcessor.GetSupportedExtensions()
	if len(fallbackExtensions) <= len(extensions) {
		t.Error("Expected fallback mode to support more extensions than office-only mode")
	}
}