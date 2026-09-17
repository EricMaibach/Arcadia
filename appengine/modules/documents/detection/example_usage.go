package detection

import (
	"fmt"
	"log"
)

// ExampleTikaDetection demonstrates how to use the Tika detection integration
func ExampleTikaDetection() {
	// Create a default multi-stage detector (includes Tika)
	detector := NewMultiStageDetector()

	// Example Office documents
	testFiles := []string{
		"document.docx",     // Word document
		"spreadsheet.xlsx",  // Excel spreadsheet
		"presentation.pptx", // PowerPoint presentation
		"text.odt",          // OpenDocument text
		"data.csv",          // CSV file (text)
		"manual.pdf",        // PDF document
		"readme.md",         // Markdown file
		"unknown.xyz",       // Unknown extension
	}

	fmt.Println("=== Tika Detection Integration Example ===")
	fmt.Println()

	for _, filePath := range testFiles {
		result, err := detector.DetectDocumentType(filePath)

		fmt.Printf("File: %s\n", filePath)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else if result == nil {
			fmt.Printf("  Detection: No suitable processor found\n")
		} else {
			fmt.Printf("  Processor: %s\n", result.Type)
			fmt.Printf("  Confidence: %.2f\n", result.Confidence)
			fmt.Printf("  Extension: %s\n", result.Extension)

			if result.Metadata != nil {
				if detector := result.Metadata["detected_by"]; detector != nil {
					fmt.Printf("  Detected by: %s\n", detector)
				}
				if tikaMode := result.Metadata["detection_type"]; tikaMode != nil {
					fmt.Printf("  Detection type: %s\n", tikaMode)
				}
			}
		}
		fmt.Println()
	}
}

// ExampleCustomTikaConfig demonstrates how to use custom Tika configuration
func ExampleCustomTikaConfig() {
	// Create custom configuration
	config := DefaultDetectionConfig()
	config.EnableTikaDetection = true
	config.EnableTikaFallback = true    // Enable fallback for unknown files
	config.TikaOfficeConfidence = 0.85  // Custom office confidence
	config.TikaFallbackConfidence = 0.4 // Custom fallback confidence
	config.ConfidenceThreshold = 0.3    // Lower threshold to accept fallback

	// Create detector with custom config
	detector := NewMultiStageDetectorWithConfig(config)

	fmt.Println("=== Custom Tika Configuration Example ===")
	fmt.Println("Configuration:")
	fmt.Printf("  Tika Detection: %v\n", config.EnableTikaDetection)
	fmt.Printf("  Tika Fallback: %v\n", config.EnableTikaFallback)
	fmt.Printf("  Office Confidence: %.2f\n", config.TikaOfficeConfidence)
	fmt.Printf("  Fallback Confidence: %.2f\n", config.TikaFallbackConfidence)
	fmt.Printf("  Confidence Threshold: %.2f\n", config.ConfidenceThreshold)
	fmt.Println()

	// Test with unknown file extensions
	unknownFiles := []string{
		"data.xyz",
		"file.unknown",
		"document", // No extension
		"archive.weird",
	}

	for _, filePath := range unknownFiles {
		result, err := detector.DetectDocumentType(filePath)

		fmt.Printf("File: %s\n", filePath)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else if result == nil {
			fmt.Printf("  Detection: No suitable processor found\n")
		} else {
			fmt.Printf("  Processor: %s (fallback mode)\n", result.Type)
			fmt.Printf("  Confidence: %.2f\n", result.Confidence)

			if result.Metadata != nil {
				if fallback := result.Metadata["fallback_mode"]; fallback != nil {
					fmt.Printf("  Fallback mode: %v\n", fallback)
				}
			}
		}
		fmt.Println()
	}
}

// ExampleTikaOnlyDetector demonstrates using only the Tika detector
func ExampleTikaOnlyDetector() {
	// Create a Tika-only detector
	tikaDetector := NewTikaDetector()

	fmt.Println("=== Tika-Only Detection Example ===")
	fmt.Printf("Priority: %d\n", tikaDetector.GetPriority())
	fmt.Printf("Supported extensions: %v\n", tikaDetector.GetSupportedExtensions()[:10]) // Show first 10
	fmt.Println()

	testFiles := []string{
		"report.docx",
		"data.xlsx",
		"slides.pptx",
		"text.txt",  // Not supported by Tika detector
		"image.jpg", // Not supported by Tika detector
	}

	for _, filePath := range testFiles {
		result, err := tikaDetector.DetectType(filePath)
		confidence := tikaDetector.GetConfidence(filePath)

		fmt.Printf("File: %s\n", filePath)
		fmt.Printf("  Confidence: %.2f\n", confidence)

		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else if result == nil {
			fmt.Printf("  Detection: Not handled by Tika detector\n")
		} else {
			fmt.Printf("  Processor: %s\n", result.Type)
			fmt.Printf("  Extension: %s\n", result.Extension)
		}
		fmt.Println()
	}
}

// ExampleDetectionPriority demonstrates the detection priority system
func ExampleDetectionPriority() {
	detector := NewMultiStageDetector()

	fmt.Println("=== Detection Priority Example ===")
	fmt.Println("Registered detectors (in priority order):")

	detectorInfo := detector.GetRegisteredDetectors()
	for i, info := range detectorInfo {
		fmt.Printf("  %d. %s (priority: %v)\n", i+1, info["type"], info["priority"])
	}
	fmt.Println()

	fmt.Println("How detection works for 'document.docx':")
	fmt.Println("  1. ExtensionDetector (priority 100): Detects .docx -> Tika (confidence 0.95)")
	fmt.Println("  2. MIMEDetector (priority 90): Would need file content")
	fmt.Println("  3. TikaDetector (priority 80): Detects .docx -> Tika (confidence 0.9)")
	fmt.Println("  4. ContentDetector (priority 70): Would need file content")
	fmt.Println()
	fmt.Println("Result: ExtensionDetector wins due to highest priority and confidence")

	// Demonstrate actual detection
	result, err := detector.DetectDocumentType("document.docx")
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("\nActual result for 'document.docx':\n")
	fmt.Printf("  Processor: %s\n", result.Type)
	fmt.Printf("  Confidence: %.2f\n", result.Confidence)
	if result.Metadata != nil {
		fmt.Printf("  Detected by: %s\n", result.Metadata["detected_by"])
	}
}

// ExampleFallbackMode demonstrates the fallback detection mode
func ExampleFallbackMode() {
	// Create detector with fallback enabled
	tikaDetector := NewTikaDetectorWithFallback()

	fmt.Println("=== Fallback Mode Example ===")
	fmt.Printf("Fallback enabled: %v\n", tikaDetector.IsFallbackEnabled())
	fmt.Printf("Priority (fallback mode): %d\n", tikaDetector.GetPriority())
	fmt.Println()

	testFiles := []string{
		"known.docx",  // Known Office format
		"unknown.xyz", // Unknown format (fallback)
		"noextension", // No extension (fallback)
		"data.custom", // Custom format (fallback)
	}

	for _, filePath := range testFiles {
		result, err := tikaDetector.DetectType(filePath)
		confidence := tikaDetector.GetConfidence(filePath)

		fmt.Printf("File: %s\n", filePath)
		fmt.Printf("  Confidence: %.2f\n", confidence)

		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else if result == nil {
			fmt.Printf("  Detection: Not detected\n")
		} else {
			fmt.Printf("  Processor: %s\n", result.Type)
			if result.Metadata != nil {
				detectionType := result.Metadata["detection_type"]
				fallbackMode := result.Metadata["fallback_mode"]
				fmt.Printf("  Detection type: %s\n", detectionType)
				fmt.Printf("  Fallback mode: %v\n", fallbackMode)
			}
		}
		fmt.Println()
	}
}

// ExampleConfigValidation demonstrates configuration validation
func ExampleConfigValidation() {
	fmt.Println("=== Configuration Validation Example ===")

	// Valid configuration
	validConfig := DefaultDetectionConfig()
	fmt.Printf("Default config validation: ")
	if err := validConfig.Validate(); err != nil {
		fmt.Printf("FAILED - %v\n", err)
	} else {
		fmt.Printf("PASSED\n")
	}

	// Invalid configuration examples
	invalidConfigs := []*DetectionConfig{
		{
			EnableTikaDetection:    true,
			TikaOfficeConfidence:   1.5, // Invalid: > 1.0
			TikaFallbackConfidence: 0.3,
		},
		{
			EnableTikaDetection:    true,
			TikaOfficeConfidence:   0.9,
			TikaFallbackConfidence: -0.1, // Invalid: < 0.0
		},
		{
			ConfidenceThreshold:      0.5,
			EnableExtensionDetection: false,
			EnableMIMEDetection:      false,
			EnableContentDetection:   false,
			EnableTikaDetection:      false, // Invalid: No detectors enabled
		},
	}

	for i, config := range invalidConfigs {
		fmt.Printf("Invalid config %d validation: ", i+1)
		if err := config.Validate(); err != nil {
			fmt.Printf("FAILED (as expected) - %v\n", err)
		} else {
			fmt.Printf("PASSED (unexpected!)\n")
		}
	}
}
