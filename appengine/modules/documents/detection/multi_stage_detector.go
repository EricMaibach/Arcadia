package detection

import (
	"fmt"
	"sort"
	"sync"

	"arcadia/modules/documents/models"
	"arcadia/modules/documents/processors/base"
)

// MultiStageDetectorImpl implements the MultiStageDetector interface
type MultiStageDetectorImpl struct {
	detectors         []DocumentDetector
	mu                sync.RWMutex
	confidenceThreshold float64
	enableParallel    bool
}

// DetectionResult holds the result from a single detector
type DetectionResult struct {
	DocumentType *base.DocumentType
	Detector     DocumentDetector
	Error        error
}

// NewMultiStageDetector creates a new multi-stage detector with default detectors
func NewMultiStageDetector() *MultiStageDetectorImpl {
	detector := &MultiStageDetectorImpl{
		detectors:         make([]DocumentDetector, 0),
		confidenceThreshold: 0.5, // Minimum confidence to accept a result
		enableParallel:    false,  // Sequential by default for deterministic results
	}

	// Register default detectors in priority order
	detector.RegisterDetector(NewExtensionDetector())
	detector.RegisterDetector(NewMIMEDetector())
	detector.RegisterDetector(NewTikaDetector())
	detector.RegisterDetector(NewContentDetector())

	return detector
}

// NewMultiStageDetectorWithConfig creates a detector with custom configuration
func NewMultiStageDetectorWithConfig(config *DetectionConfig) *MultiStageDetectorImpl {
	detector := &MultiStageDetectorImpl{
		detectors:         make([]DocumentDetector, 0),
		confidenceThreshold: config.ConfidenceThreshold,
		enableParallel:    config.EnableParallel,
	}

	// Register detectors based on configuration
	if config.EnableExtensionDetection {
		detector.RegisterDetector(NewExtensionDetector())
	}
	if config.EnableMIMEDetection {
		detector.RegisterDetector(NewMIMEDetector())
	}
	if config.EnableTikaDetection {
		tikaDetector := NewTikaDetector()
		if config.EnableTikaFallback {
			tikaDetector.SetFallbackEnabled(true)
		}
		if config.TikaOfficeConfidence > 0 && config.TikaFallbackConfidence > 0 {
			tikaDetector.SetConfidenceLevels(config.TikaOfficeConfidence, config.TikaFallbackConfidence)
		}
		detector.RegisterDetector(tikaDetector)
	}
	if config.EnableContentDetection {
		detector.RegisterDetector(NewContentDetector())
	}

	return detector
}

// DetectDocumentType attempts to detect document type using all registered detectors
func (msd *MultiStageDetectorImpl) DetectDocumentType(filePath string) (*base.DocumentType, error) {
	if filePath == "" {
		return nil, models.NewDocumentError(models.ErrInvalidInput, "file path cannot be empty").
			WithFilePath(filePath)
	}

	msd.mu.RLock()
	detectors := make([]DocumentDetector, len(msd.detectors))
	copy(detectors, msd.detectors)
	msd.mu.RUnlock()

	if len(detectors) == 0 {
		return nil, models.NewDocumentError(models.ErrInvalidConfig, "no detectors registered")
	}

	var results []DetectionResult

	if msd.enableParallel {
		results = msd.detectParallel(filePath, detectors)
	} else {
		results = msd.detectSequential(filePath, detectors)
	}

	// Find the best result
	bestResult := msd.selectBestResult(results)
	if bestResult == nil {
		return nil, models.NewDocumentError(models.ErrUnsupportedFormat,
			"unable to detect document type").WithFilePath(filePath)
	}

	// Check if confidence meets threshold
	if bestResult.DocumentType.Confidence < msd.confidenceThreshold {
		return nil, models.NewDocumentError(models.ErrUnsupportedFormat,
			fmt.Sprintf("detection confidence %.2f below threshold %.2f",
				bestResult.DocumentType.Confidence, msd.confidenceThreshold)).
			WithFilePath(filePath)
	}

	// Add detection metadata
	if bestResult.DocumentType.Metadata == nil {
		bestResult.DocumentType.Metadata = make(map[string]interface{})
	}
	bestResult.DocumentType.Metadata["multi_stage_detector"] = true
	bestResult.DocumentType.Metadata["total_detectors"] = len(detectors)
	bestResult.DocumentType.Metadata["detection_results"] = msd.summarizeResults(results)

	return bestResult.DocumentType, nil
}

// detectSequential runs detectors one by one, stopping at first high-confidence result
func (msd *MultiStageDetectorImpl) detectSequential(filePath string, detectors []DocumentDetector) []DetectionResult {
	results := make([]DetectionResult, 0, len(detectors))

	// Sort detectors by priority (highest first)
	sortedDetectors := make([]DocumentDetector, len(detectors))
	copy(sortedDetectors, detectors)
	sort.Slice(sortedDetectors, func(i, j int) bool {
		return sortedDetectors[i].GetPriority() > sortedDetectors[j].GetPriority()
	})

	for _, detector := range sortedDetectors {
		docType, err := detector.DetectType(filePath)

		result := DetectionResult{
			DocumentType: docType,
			Detector:     detector,
			Error:        err,
		}
		results = append(results, result)

		// If we have a high-confidence result, we can stop early
		if err == nil && docType != nil && docType.Confidence >= 0.9 {
			break
		}
	}

	return results
}

// detectParallel runs all detectors in parallel
func (msd *MultiStageDetectorImpl) detectParallel(filePath string, detectors []DocumentDetector) []DetectionResult {
	results := make([]DetectionResult, len(detectors))
	var wg sync.WaitGroup

	for i, detector := range detectors {
		wg.Add(1)
		go func(index int, det DocumentDetector) {
			defer wg.Done()

			docType, err := det.DetectType(filePath)
			results[index] = DetectionResult{
				DocumentType: docType,
				Detector:     det,
				Error:        err,
			}
		}(i, detector)
	}

	wg.Wait()
	return results
}

// selectBestResult chooses the best detection result from all detector results
func (msd *MultiStageDetectorImpl) selectBestResult(results []DetectionResult) *DetectionResult {
	var bestResult *DetectionResult
	var bestScore float64

	for _, result := range results {
		// Skip failed detections
		if result.Error != nil || result.DocumentType == nil {
			continue
		}

		// Calculate score based on confidence and detector priority
		score := result.DocumentType.Confidence * (float64(result.Detector.GetPriority()) / 100.0)

		if bestResult == nil || score > bestScore {
			bestResult = &result
			bestScore = score
		}
	}

	return bestResult
}

// summarizeResults creates a summary of all detection results for metadata
func (msd *MultiStageDetectorImpl) summarizeResults(results []DetectionResult) []map[string]interface{} {
	summary := make([]map[string]interface{}, 0, len(results))

	for _, result := range results {
		entry := map[string]interface{}{
			"detector_type": fmt.Sprintf("%T", result.Detector),
			"priority":      result.Detector.GetPriority(),
		}

		if result.Error != nil {
			entry["error"] = result.Error.Error()
		} else if result.DocumentType != nil {
			entry["detected_type"] = result.DocumentType.Type
			entry["confidence"] = result.DocumentType.Confidence
			entry["mime_type"] = result.DocumentType.MimeType
			entry["extension"] = result.DocumentType.Extension
		} else {
			entry["result"] = "no_detection"
		}

		summary = append(summary, entry)
	}

	return summary
}

// RegisterDetector adds a new detector to the detection pipeline
func (msd *MultiStageDetectorImpl) RegisterDetector(detector DocumentDetector) error {
	if detector == nil {
		return models.NewDocumentError(models.ErrInvalidInput, "detector cannot be nil")
	}

	msd.mu.Lock()
	defer msd.mu.Unlock()

	// Check for duplicate detector types
	for _, existing := range msd.detectors {
		if fmt.Sprintf("%T", existing) == fmt.Sprintf("%T", detector) {
			return models.NewDocumentError(models.ErrDocumentExists,
				fmt.Sprintf("detector of type %T already registered", detector))
		}
	}

	msd.detectors = append(msd.detectors, detector)

	// Sort detectors by priority
	sort.Slice(msd.detectors, func(i, j int) bool {
		return msd.detectors[i].GetPriority() > msd.detectors[j].GetPriority()
	})

	return nil
}

// GetSupportedTypes returns all processor types that can be detected
func (msd *MultiStageDetectorImpl) GetSupportedTypes() []base.ProcessorType {
	msd.mu.RLock()
	defer msd.mu.RUnlock()

	typeSet := make(map[base.ProcessorType]bool)

	// Collect all supported types from all detectors
	for _, detector := range msd.detectors {
		// For extension detector, we can get supported types
		if extDetector, ok := detector.(*ExtensionDetector); ok {
			for _, ext := range extDetector.GetSupportedExtensions() {
				if docType, err := extDetector.DetectType("dummy." + ext); err == nil && docType != nil {
					typeSet[docType.Type] = true
				}
			}
		}
		// For MIME detector
		if mimeDetector, ok := detector.(*MIMEDetector); ok {
			for range mimeDetector.GetSupportedMIMETypes() {
				// This is a bit hacky, but we can create a temporary mapping
				for _, processorType := range []base.ProcessorType{
					base.ProcessorTypeText, base.ProcessorTypeMarkdown, base.ProcessorTypePDF} {
					typeSet[processorType] = true
				}
			}
		}
		// For content detector, assume it supports all basic types
		if _, ok := detector.(*ContentDetector); ok {
			typeSet[base.ProcessorTypeText] = true
			typeSet[base.ProcessorTypeMarkdown] = true
			typeSet[base.ProcessorTypePDF] = true
		}
		// For Tika detector
		if tikaDetector, ok := detector.(*TikaDetector); ok {
			for _, ext := range tikaDetector.GetSupportedExtensions() {
				if docType, err := tikaDetector.DetectType("dummy." + ext); err == nil && docType != nil {
					typeSet[docType.Type] = true
				}
			}
			// Also add Tika type directly since it might support fallback mode
			typeSet[base.ProcessorTypeTika] = true
		}
	}

	// Convert set to slice
	types := make([]base.ProcessorType, 0, len(typeSet))
	for processorType := range typeSet {
		types = append(types, processorType)
	}

	return types
}

// SetConfidenceThreshold sets the minimum confidence threshold for accepting results
func (msd *MultiStageDetectorImpl) SetConfidenceThreshold(threshold float64) error {
	if threshold < 0.0 || threshold > 1.0 {
		return models.NewDocumentError(models.ErrInvalidInput,
			"confidence threshold must be between 0.0 and 1.0")
	}

	msd.mu.Lock()
	defer msd.mu.Unlock()
	msd.confidenceThreshold = threshold

	return nil
}

// GetConfidenceThreshold returns the current confidence threshold
func (msd *MultiStageDetectorImpl) GetConfidenceThreshold() float64 {
	msd.mu.RLock()
	defer msd.mu.RUnlock()
	return msd.confidenceThreshold
}

// SetParallelDetection enables or disables parallel detection
func (msd *MultiStageDetectorImpl) SetParallelDetection(enable bool) {
	msd.mu.Lock()
	defer msd.mu.Unlock()
	msd.enableParallel = enable
}

// IsParallelDetectionEnabled returns whether parallel detection is enabled
func (msd *MultiStageDetectorImpl) IsParallelDetectionEnabled() bool {
	msd.mu.RLock()
	defer msd.mu.RUnlock()
	return msd.enableParallel
}

// GetRegisteredDetectors returns information about all registered detectors
func (msd *MultiStageDetectorImpl) GetRegisteredDetectors() []map[string]interface{} {
	msd.mu.RLock()
	defer msd.mu.RUnlock()

	info := make([]map[string]interface{}, len(msd.detectors))
	for i, detector := range msd.detectors {
		info[i] = map[string]interface{}{
			"type":     fmt.Sprintf("%T", detector),
			"priority": detector.GetPriority(),
		}
	}

	return info
}