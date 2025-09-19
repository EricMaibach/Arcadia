package detection

import (
	"path/filepath"
	"strings"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
	"arcadia/modules/documents/processors/base"
)

// TikaDetector detects document types that should be processed with Apache Tika
type TikaDetector struct {
	knownOfficeExts    map[string]bool
	fallbackEnabled    bool
	officeConfidence   float64
	fallbackConfidence float64
	priority           int
	logger             interfaces.Logger
}

// NewTikaDetector creates a new Tika detector for Office documents only
func NewTikaDetector() *TikaDetector {
	return &TikaDetector{
		knownOfficeExts:    getKnownOfficeExtensions(),
		fallbackEnabled:    false,
		officeConfidence:   0.9,
		fallbackConfidence: 0.3,
		priority:           80, // Between extension/MIME (100,90) and content (70)
	}
}

// NewTikaDetectorWithFallback creates a new Tika detector with fallback mode enabled
func NewTikaDetectorWithFallback() *TikaDetector {
	detector := NewTikaDetector()
	detector.fallbackEnabled = true
	detector.priority = 10 // Low priority for fallback mode
	return detector
}

// DetectType attempts to detect document type for Tika processing
func (d *TikaDetector) DetectType(filePath string) (*base.DocumentType, error) {
	if filePath == "" {
		return nil, models.NewDocumentError(models.ErrInvalidInput, "file path cannot be empty").
			WithFilePath(filePath)
	}

	// Extract extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" && !d.fallbackEnabled {
		return nil, nil // No extension and fallback disabled
	}

	// Remove the dot from extension
	if ext != "" {
		ext = strings.TrimPrefix(ext, ".")
	}

	// Check for known Office formats (high confidence)
	if ext != "" && d.knownOfficeExts[ext] {
		if d.logger != nil {
			d.logger.Debug(nil, "Detected Office document format",
				"file_path", filePath,
				"extension", ext,
				"confidence", d.officeConfidence)
		}

		return &base.DocumentType{
			Type:       base.ProcessorTypeTika,
			Extension:  ext,
			Confidence: d.officeConfidence,
			Metadata: map[string]interface{}{
				"detector":        "tika",
				"detected_by":     "TikaDetector",
				"original_path":   filePath,
				"file_extension":  ext,
				"detection_type":  "office_format",
				"fallback_mode":   false,
			},
		}, nil
	}

	// Check for fallback mode (any unknown extension)
	if d.fallbackEnabled {
		if d.logger != nil {
			d.logger.Debug(nil, "Using Tika fallback detection",
				"file_path", filePath,
				"extension", ext,
				"confidence", d.fallbackConfidence)
		}

		return &base.DocumentType{
			Type:       base.ProcessorTypeTika,
			Extension:  ext,
			Confidence: d.fallbackConfidence,
			Metadata: map[string]interface{}{
				"detector":        "tika",
				"detected_by":     "TikaDetector",
				"original_path":   filePath,
				"file_extension":  ext,
				"detection_type":  "fallback",
				"fallback_mode":   true,
			},
		}, nil
	}

	// Not detected by this detector
	return nil, nil
}

// GetConfidence returns confidence level for a given file path
func (d *TikaDetector) GetConfidence(filePath string) float64 {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		if d.fallbackEnabled {
			return d.fallbackConfidence
		}
		return 0.0
	}

	ext = strings.TrimPrefix(ext, ".")

	// High confidence for known Office formats
	if d.knownOfficeExts[ext] {
		return d.officeConfidence
	}

	// Fallback confidence for unknown extensions
	if d.fallbackEnabled {
		return d.fallbackConfidence
	}

	return 0.0
}

// GetPriority returns the priority of this detector
func (d *TikaDetector) GetPriority() int {
	return d.priority
}

// WithLogger sets the logger for this detector
func (d *TikaDetector) WithLogger(logger interfaces.Logger) *TikaDetector {
	d.logger = logger
	return d
}

// SetFallbackEnabled enables or disables fallback mode
func (d *TikaDetector) SetFallbackEnabled(enabled bool) {
	d.fallbackEnabled = enabled
	if enabled {
		d.priority = 10 // Low priority for fallback
	} else {
		d.priority = 80 // Normal priority for Office detection
	}
}

// IsFallbackEnabled returns whether fallback mode is enabled
func (d *TikaDetector) IsFallbackEnabled() bool {
	return d.fallbackEnabled
}

// SetConfidenceLevels sets the confidence levels for detection
func (d *TikaDetector) SetConfidenceLevels(officeConfidence, fallbackConfidence float64) error {
	if officeConfidence < 0.0 || officeConfidence > 1.0 {
		return models.NewDocumentError(models.ErrInvalidInput,
			"office confidence must be between 0.0 and 1.0")
	}
	if fallbackConfidence < 0.0 || fallbackConfidence > 1.0 {
		return models.NewDocumentError(models.ErrInvalidInput,
			"fallback confidence must be between 0.0 and 1.0")
	}

	d.officeConfidence = officeConfidence
	d.fallbackConfidence = fallbackConfidence
	return nil
}

// GetSupportedExtensions returns all Office extensions supported by this detector
func (d *TikaDetector) GetSupportedExtensions() []string {
	extensions := make([]string, 0, len(d.knownOfficeExts))
	for ext := range d.knownOfficeExts {
		extensions = append(extensions, ext)
	}
	return extensions
}

// AddOfficeExtension adds a new Office extension to the known list
func (d *TikaDetector) AddOfficeExtension(extension string) error {
	if extension == "" {
		return models.NewDocumentError(models.ErrInvalidInput, "extension cannot be empty")
	}

	// Normalize extension (remove dot, lowercase)
	ext := strings.ToLower(strings.TrimPrefix(extension, "."))
	d.knownOfficeExts[ext] = true

	if d.logger != nil {
		d.logger.Debug(nil, "Added Office extension to Tika detector", "extension", ext)
	}

	return nil
}

// getKnownOfficeExtensions returns the default set of Office document extensions
func getKnownOfficeExtensions() map[string]bool {
	return map[string]bool{
		// Microsoft Office formats
		"doc":  true, // Word 97-2003
		"docx": true, // Word 2007+
		"xls":  true, // Excel 97-2003
		"xlsx": true, // Excel 2007+
		"ppt":  true, // PowerPoint 97-2003
		"pptx": true, // PowerPoint 2007+

		// Microsoft Office alternative formats
		"docm": true, // Word macro-enabled
		"xlsm": true, // Excel macro-enabled
		"pptm": true, // PowerPoint macro-enabled
		"xlsb": true, // Excel binary format
		"xltx": true, // Excel template
		"xltm": true, // Excel macro-enabled template
		"potx": true, // PowerPoint template
		"potm": true, // PowerPoint macro-enabled template

		// OpenDocument formats
		"odt": true, // OpenDocument Text
		"ods": true, // OpenDocument Spreadsheet
		"odp": true, // OpenDocument Presentation
		"odg": true, // OpenDocument Graphics
		"odf": true, // OpenDocument Formula

		// LibreOffice/OpenOffice legacy formats
		"sxw": true, // StarOffice Writer
		"sxc": true, // StarOffice Calc
		"sxi": true, // StarOffice Impress

		// Rich Text Format
		"rtf": true, // Rich Text Format

		// Other office-like formats that Tika handles well
		"wpd": true, // WordPerfect
		"wps": true, // Microsoft Works Word Processor
		"pub": true, // Microsoft Publisher
		"vsd": true, // Microsoft Visio
		"msg": true, // Outlook message format
	}
}