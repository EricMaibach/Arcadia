package detection

import (
	"arcadia/modules/documents/processors/base"
)

// DocumentDetector represents a single document type detection method
type DocumentDetector interface {
	// DetectType attempts to detect the document type from the given file path
	// Returns the detected document type or nil if the detector cannot determine the type
	DetectType(filePath string) (*base.DocumentType, error)

	// GetConfidence returns the confidence level (0.0 to 1.0) for a given file path
	// This is used to determine which detector should take precedence
	GetConfidence(filePath string) float64

	// GetPriority returns the priority of this detector (higher number = higher priority)
	// Used to order detectors when multiple detectors have similar confidence levels
	GetPriority() int
}

// MultiStageDetector orchestrates multiple detection methods to determine document type
type MultiStageDetector interface {
	// DetectDocumentType attempts to detect the document type using all registered detectors
	// Returns the most confident detection result
	DetectDocumentType(filePath string) (*base.DocumentType, error)

	// RegisterDetector adds a new detector to the detection pipeline
	RegisterDetector(detector DocumentDetector) error

	// GetSupportedTypes returns all processor types that can be detected
	GetSupportedTypes() []base.ProcessorType
}
