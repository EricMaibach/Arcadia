package detection

import (
	"testing"

	"arcadia/modules/documents/processors/base"
)

func TestTikaDetector_DetectType(t *testing.T) {
	detector := NewTikaDetector()

	tests := []struct {
		name           string
		filePath       string
		expectedType   base.ProcessorType
		hasExpectedType bool
		expectedConf   float64
		expectError    bool
	}{
		{
			name:            "Word document",
			filePath:        "test.docx",
			expectedType:    base.ProcessorTypeTika,
			hasExpectedType: true,
			expectedConf:    0.9,
			expectError:     false,
		},
		{
			name:            "Excel document",
			filePath:        "test.xlsx",
			expectedType:    base.ProcessorTypeTika,
			hasExpectedType: true,
			expectedConf:    0.9,
			expectError:     false,
		},
		{
			name:            "PowerPoint document",
			filePath:        "test.pptx",
			expectedType:    base.ProcessorTypeTika,
			hasExpectedType: true,
			expectedConf:    0.9,
			expectError:     false,
		},
		{
			name:            "OpenDocument text",
			filePath:        "test.odt",
			expectedType:    base.ProcessorTypeTika,
			hasExpectedType: true,
			expectedConf:    0.9,
			expectError:     false,
		},
		{
			name:            "Rich text format",
			filePath:        "test.rtf",
			expectedType:    base.ProcessorTypeTika,
			hasExpectedType: true,
			expectedConf:    0.9,
			expectError:     false,
		},
		{
			name:            "Text file - no detection",
			filePath:        "test.txt",
			hasExpectedType: false,
			expectedConf:    0.0,
			expectError:     false,
		},
		{
			name:            "No extension - no detection",
			filePath:        "test",
			hasExpectedType: false,
			expectedConf:    0.0,
			expectError:     false,
		},
		{
			name:        "Empty path - error",
			filePath:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := detector.DetectType(tt.filePath)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !tt.hasExpectedType {
				if result != nil {
					t.Errorf("expected nil result but got %+v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("expected result but got nil")
				return
			}

			if result.Type != tt.expectedType {
				t.Errorf("expected type %v but got %v", tt.expectedType, result.Type)
			}

			if result.Confidence != tt.expectedConf {
				t.Errorf("expected confidence %.2f but got %.2f", tt.expectedConf, result.Confidence)
			}

			// Check metadata
			if result.Metadata == nil {
				t.Errorf("expected metadata but got nil")
			} else {
				if result.Metadata["detector"] != "tika" {
					t.Errorf("expected detector=tika but got %v", result.Metadata["detector"])
				}
				if result.Metadata["detected_by"] != "TikaDetector" {
					t.Errorf("expected detected_by=TikaDetector but got %v", result.Metadata["detected_by"])
				}
			}
		})
	}
}

func TestTikaDetector_WithFallback(t *testing.T) {
	detector := NewTikaDetectorWithFallback()

	tests := []struct {
		name         string
		filePath     string
		expectedConf float64
		fallbackMode bool
	}{
		{
			name:         "Office document - high confidence",
			filePath:     "test.docx",
			expectedConf: 0.9,
			fallbackMode: false,
		},
		{
			name:         "Unknown extension - fallback",
			filePath:     "test.xyz",
			expectedConf: 0.3,
			fallbackMode: true,
		},
		{
			name:         "No extension - fallback",
			filePath:     "test",
			expectedConf: 0.3,
			fallbackMode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := detector.DetectType(tt.filePath)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("expected result but got nil")
				return
			}

			if result.Type != base.ProcessorTypeTika {
				t.Errorf("expected type %v but got %v", base.ProcessorTypeTika, result.Type)
			}

			if result.Confidence != tt.expectedConf {
				t.Errorf("expected confidence %.2f but got %.2f", tt.expectedConf, result.Confidence)
			}

			// Check fallback mode in metadata
			if result.Metadata == nil {
				t.Errorf("expected metadata but got nil")
			} else {
				fallbackInMetadata := result.Metadata["fallback_mode"].(bool)
				if fallbackInMetadata != tt.fallbackMode {
					t.Errorf("expected fallback_mode=%v but got %v", tt.fallbackMode, fallbackInMetadata)
				}
			}
		})
	}
}

func TestTikaDetector_GetConfidence(t *testing.T) {
	detector := NewTikaDetector()

	tests := []struct {
		name         string
		filePath     string
		expectedConf float64
	}{
		{"Office document", "test.docx", 0.9},
		{"Excel document", "test.xlsx", 0.9},
		{"Unknown extension", "test.xyz", 0.0},
		{"No extension", "test", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := detector.GetConfidence(tt.filePath)
			if conf != tt.expectedConf {
				t.Errorf("expected confidence %.2f but got %.2f", tt.expectedConf, conf)
			}
		})
	}
}

func TestTikaDetector_GetConfidence_WithFallback(t *testing.T) {
	detector := NewTikaDetectorWithFallback()

	tests := []struct {
		name         string
		filePath     string
		expectedConf float64
	}{
		{"Office document", "test.docx", 0.9},
		{"Unknown extension with fallback", "test.xyz", 0.3},
		{"No extension with fallback", "test", 0.3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := detector.GetConfidence(tt.filePath)
			if conf != tt.expectedConf {
				t.Errorf("expected confidence %.2f but got %.2f", tt.expectedConf, conf)
			}
		})
	}
}

func TestTikaDetector_Priority(t *testing.T) {
	detector := NewTikaDetector()

	if detector.GetPriority() != 80 {
		t.Errorf("expected priority 80 but got %d", detector.GetPriority())
	}

	fallbackDetector := NewTikaDetectorWithFallback()
	if fallbackDetector.GetPriority() != 10 {
		t.Errorf("expected fallback priority 10 but got %d", fallbackDetector.GetPriority())
	}
}

func TestTikaDetector_SetConfidenceLevels(t *testing.T) {
	detector := NewTikaDetector()

	// Test valid confidence levels
	err := detector.SetConfidenceLevels(0.8, 0.2)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify the changes
	conf := detector.GetConfidence("test.docx")
	if conf != 0.8 {
		t.Errorf("expected office confidence 0.8 but got %.2f", conf)
	}

	// Test invalid confidence levels
	err = detector.SetConfidenceLevels(1.5, 0.2)
	if err == nil {
		t.Errorf("expected error for invalid office confidence")
	}

	err = detector.SetConfidenceLevels(0.8, -0.1)
	if err == nil {
		t.Errorf("expected error for invalid fallback confidence")
	}
}

func TestTikaDetector_GetSupportedExtensions(t *testing.T) {
	detector := NewTikaDetector()

	extensions := detector.GetSupportedExtensions()

	// Check that we have a reasonable number of extensions
	if len(extensions) < 10 {
		t.Errorf("expected at least 10 extensions but got %d", len(extensions))
	}

	// Check for some expected extensions
	expectedExts := []string{"docx", "xlsx", "pptx", "odt", "rtf"}
	found := make(map[string]bool)

	for _, ext := range extensions {
		found[ext] = true
	}

	for _, expected := range expectedExts {
		if !found[expected] {
			t.Errorf("expected extension %s not found in supported extensions", expected)
		}
	}
}

func TestTikaDetector_AddOfficeExtension(t *testing.T) {
	detector := NewTikaDetector()

	// Add a custom extension
	err := detector.AddOfficeExtension("custom")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify it works
	result, err := detector.DetectType("test.custom")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil || result.Type != base.ProcessorTypeTika {
		t.Errorf("expected Tika detection for custom extension")
	}

	// Test empty extension
	err = detector.AddOfficeExtension("")
	if err == nil {
		t.Errorf("expected error for empty extension")
	}
}