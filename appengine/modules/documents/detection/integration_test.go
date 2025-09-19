package detection

import (
	"testing"

	"arcadia/modules/documents/processors/base"
)

func TestMultiStageDetector_TikaIntegration(t *testing.T) {
	// Test default detector includes Tika
	detector := NewMultiStageDetector()

	tests := []struct {
		name         string
		filePath     string
		expectedType base.ProcessorType
	}{
		{"Word document", "test.docx", base.ProcessorTypeTika},
		{"Excel document", "test.xlsx", base.ProcessorTypeTika},
		{"PowerPoint document", "test.pptx", base.ProcessorTypeTika},
		{"OpenDocument text", "test.odt", base.ProcessorTypeTika},
		{"Rich text format", "test.rtf", base.ProcessorTypeTika},
		{"PDF document", "test.pdf", base.ProcessorTypePDF},
		{"Markdown document", "test.md", base.ProcessorTypeMarkdown},
		{"Text document", "test.txt", base.ProcessorTypeText},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := detector.DetectDocumentType(tt.filePath)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("expected result but got nil")
				return
			}

			if result.Type != tt.expectedType {
				t.Errorf("expected type %v but got %v", tt.expectedType, result.Type)
			}

			// Verify confidence is reasonable
			if result.Confidence <= 0.0 {
				t.Errorf("expected positive confidence but got %.2f", result.Confidence)
			}
		})
	}
}

func TestMultiStageDetector_TikaConfigIntegration(t *testing.T) {
	// Test with custom configuration
	config := DefaultDetectionConfig()
	config.EnableTikaDetection = true
	config.EnableTikaFallback = true
	config.TikaOfficeConfidence = 0.85
	config.TikaFallbackConfidence = 0.6  // Above the default threshold
	config.ConfidenceThreshold = 0.5      // Ensure consistent threshold

	detector := NewMultiStageDetectorWithConfig(config)

	// Test Office document detection
	result, err := detector.DetectDocumentType("test.docx")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Errorf("expected result but got nil")
	} else {
		if result.Type != base.ProcessorTypeTika {
			t.Errorf("expected Tika type but got %v", result.Type)
		}
		// Note: The confidence might not match exactly due to ExtensionDetector having higher priority
		// but we can verify it's using Tika
	}

	// Test fallback detection for unknown extension
	result, err = detector.DetectDocumentType("test.unknown")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Errorf("expected result but got nil with fallback enabled")
	} else {
		if result.Type != base.ProcessorTypeTika {
			t.Errorf("expected Tika type for fallback but got %v", result.Type)
		}
	}
}

func TestMultiStageDetector_TikaDisabled(t *testing.T) {
	// Test with Tika disabled
	config := DefaultDetectionConfig()
	config.EnableTikaDetection = false

	detector := NewMultiStageDetectorWithConfig(config)

	// Office documents should still be detected by ExtensionDetector as Tika type
	// but since no Tika processor would be registered, this would fail at processing time
	// The detection itself will still return Tika type from ExtensionDetector
	result, err := detector.DetectDocumentType("test.docx")

	// ExtensionDetector will still map .docx to Tika, so we expect it to be detected
	// The actual error would occur later during processing when no Tika processor is available
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Errorf("expected result but got nil")
	} else if result.Type != base.ProcessorTypeTika {
		t.Errorf("expected Tika type from ExtensionDetector but got %v", result.Type)
	}

	// The key difference is that TikaDetector should not be registered
	detectors := detector.GetRegisteredDetectors()
	tikaDetectorFound := false
	for _, detectorInfo := range detectors {
		if detectorInfo["type"] == "*detection.TikaDetector" {
			tikaDetectorFound = true
			break
		}
	}

	if tikaDetectorFound {
		t.Errorf("expected TikaDetector to not be registered when disabled")
	}
}

func TestMultiStageDetector_GetSupportedTypes_IncludesTika(t *testing.T) {
	detector := NewMultiStageDetector()

	supportedTypes := detector.GetSupportedTypes()

	// Should include Tika type
	found := false
	for _, typeVal := range supportedTypes {
		if typeVal == base.ProcessorTypeTika {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected Tika processor type in supported types but not found")
	}
}

func TestDetectionConfig_TikaValidation(t *testing.T) {
	config := DefaultDetectionConfig()

	// Test valid configuration
	err := config.Validate()
	if err != nil {
		t.Errorf("expected valid default config but got error: %v", err)
	}

	// Test invalid office confidence
	config.TikaOfficeConfidence = 1.5
	err = config.Validate()
	if err == nil {
		t.Errorf("expected error for invalid office confidence")
	}

	// Reset and test invalid fallback confidence
	config.TikaOfficeConfidence = 0.9
	config.TikaFallbackConfidence = -0.1
	err = config.Validate()
	if err == nil {
		t.Errorf("expected error for invalid fallback confidence")
	}
}

func TestDetectionConfig_TikaCloneAndMerge(t *testing.T) {
	// Test Clone includes Tika settings
	config := DefaultDetectionConfig()
	config.EnableTikaDetection = true
	config.EnableTikaFallback = true
	config.TikaOfficeConfidence = 0.8
	config.TikaFallbackConfidence = 0.2

	clone := config.Clone()

	if clone.EnableTikaDetection != config.EnableTikaDetection {
		t.Errorf("clone EnableTikaDetection mismatch")
	}
	if clone.EnableTikaFallback != config.EnableTikaFallback {
		t.Errorf("clone EnableTikaFallback mismatch")
	}
	if clone.TikaOfficeConfidence != config.TikaOfficeConfidence {
		t.Errorf("clone TikaOfficeConfidence mismatch")
	}
	if clone.TikaFallbackConfidence != config.TikaFallbackConfidence {
		t.Errorf("clone TikaFallbackConfidence mismatch")
	}

	// Test MergeWith includes Tika settings
	other := DefaultDetectionConfig()
	other.EnableTikaDetection = false
	other.EnableTikaFallback = false
	other.TikaOfficeConfidence = 0.7
	other.TikaFallbackConfidence = 0.1

	config.MergeWith(other)

	if config.EnableTikaDetection != other.EnableTikaDetection {
		t.Errorf("merge EnableTikaDetection mismatch")
	}
	if config.EnableTikaFallback != other.EnableTikaFallback {
		t.Errorf("merge EnableTikaFallback mismatch")
	}
	if config.TikaOfficeConfidence != other.TikaOfficeConfidence {
		t.Errorf("merge TikaOfficeConfidence mismatch")
	}
	if config.TikaFallbackConfidence != other.TikaFallbackConfidence {
		t.Errorf("merge TikaFallbackConfidence mismatch")
	}
}