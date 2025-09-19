package detection

import (
	"arcadia/modules/documents/models"
)

// DetectionConfig holds configuration for the document detection system
type DetectionConfig struct {
	// ConfidenceThreshold is the minimum confidence level (0.0-1.0) required to accept a detection result
	ConfidenceThreshold float64 `json:"confidence_threshold" yaml:"confidence_threshold"`

	// EnableParallel determines whether detectors should run in parallel or sequentially
	EnableParallel bool `json:"enable_parallel" yaml:"enable_parallel"`

	// EnableExtensionDetection enables/disables file extension-based detection
	EnableExtensionDetection bool `json:"enable_extension_detection" yaml:"enable_extension_detection"`

	// EnableMIMEDetection enables/disables MIME type-based detection
	EnableMIMEDetection bool `json:"enable_mime_detection" yaml:"enable_mime_detection"`

	// EnableContentDetection enables/disables content analysis-based detection
	EnableContentDetection bool `json:"enable_content_detection" yaml:"enable_content_detection"`

	// EnableTikaDetection enables/disables Tika-based detection for Office documents
	EnableTikaDetection bool `json:"enable_tika_detection" yaml:"enable_tika_detection"`

	// EnableTikaFallback enables Tika as a fallback for unknown document types
	EnableTikaFallback bool `json:"enable_tika_fallback" yaml:"enable_tika_fallback"`

	// TikaOfficeConfidence is the confidence level for known Office format detection
	TikaOfficeConfidence float64 `json:"tika_office_confidence" yaml:"tika_office_confidence"`

	// TikaFallbackConfidence is the confidence level for fallback detection
	TikaFallbackConfidence float64 `json:"tika_fallback_confidence" yaml:"tika_fallback_confidence"`

	// ExtensionDetectorConfig holds configuration specific to extension-based detection
	ExtensionDetectorConfig ExtensionDetectorConfig `json:"extension_detector" yaml:"extension_detector"`

	// MIMEDetectorConfig holds configuration specific to MIME type detection
	MIMEDetectorConfig MIMEDetectorConfig `json:"mime_detector" yaml:"mime_detector"`

	// ContentDetectorConfig holds configuration specific to content analysis
	ContentDetectorConfig ContentDetectorConfig `json:"content_detector" yaml:"content_detector"`
}

// ExtensionDetectorConfig holds configuration for extension-based detection
type ExtensionDetectorConfig struct {
	// Priority sets the priority level for extension detection (higher = more important)
	Priority int `json:"priority" yaml:"priority"`

	// CustomMappings allows adding custom file extension to processor type mappings
	CustomMappings map[string]string `json:"custom_mappings" yaml:"custom_mappings"`

	// OverrideDefaults determines if custom mappings should override default mappings
	OverrideDefaults bool `json:"override_defaults" yaml:"override_defaults"`
}

// MIMEDetectorConfig holds configuration for MIME type detection
type MIMEDetectorConfig struct {
	// Priority sets the priority level for MIME detection
	Priority int `json:"priority" yaml:"priority"`

	// CustomMappings allows adding custom MIME type to processor type mappings
	CustomMappings map[string]string `json:"custom_mappings" yaml:"custom_mappings"`

	// OverrideDefaults determines if custom mappings should override default mappings
	OverrideDefaults bool `json:"override_defaults" yaml:"override_defaults"`

	// MaxReadBytes is the maximum number of bytes to read for MIME detection
	MaxReadBytes int `json:"max_read_bytes" yaml:"max_read_bytes"`
}

// ContentDetectorConfig holds configuration for content analysis detection
type ContentDetectorConfig struct {
	// Priority sets the priority level for content detection
	Priority int `json:"priority" yaml:"priority"`

	// MaxAnalysisBytes is the maximum number of bytes to analyze for content detection
	MaxAnalysisBytes int `json:"max_analysis_bytes" yaml:"max_analysis_bytes"`

	// MarkdownConfidenceThreshold is the minimum threshold for markdown detection
	MarkdownConfidenceThreshold float64 `json:"markdown_confidence_threshold" yaml:"markdown_confidence_threshold"`

	// BinaryDetectionThreshold is the threshold for considering content as binary
	BinaryDetectionThreshold float64 `json:"binary_detection_threshold" yaml:"binary_detection_threshold"`
}

// DefaultDetectionConfig returns a configuration with sensible defaults
func DefaultDetectionConfig() *DetectionConfig {
	return &DetectionConfig{
		ConfidenceThreshold:      0.5,
		EnableParallel:           false,
		EnableExtensionDetection: true,
		EnableMIMEDetection:      true,
		EnableContentDetection:   true,
		EnableTikaDetection:      true,
		EnableTikaFallback:       false,
		TikaOfficeConfidence:     0.9,
		TikaFallbackConfidence:   0.3,

		ExtensionDetectorConfig: ExtensionDetectorConfig{
			Priority:         100,
			CustomMappings:   make(map[string]string),
			OverrideDefaults: false,
		},

		MIMEDetectorConfig: MIMEDetectorConfig{
			Priority:         80,
			CustomMappings:   make(map[string]string),
			OverrideDefaults: false,
			MaxReadBytes:     512,
		},

		ContentDetectorConfig: ContentDetectorConfig{
			Priority:                    60,
			MaxAnalysisBytes:            8192,
			MarkdownConfidenceThreshold: 0.2,
			BinaryDetectionThreshold:    0.3,
		},
	}
}

// HighPerformanceDetectionConfig returns a configuration optimized for performance
func HighPerformanceDetectionConfig() *DetectionConfig {
	config := DefaultDetectionConfig()

	// Enable parallel detection for speed
	config.EnableParallel = true

	// Reduce content analysis scope
	config.ContentDetectorConfig.MaxAnalysisBytes = 4096
	config.MIMEDetectorConfig.MaxReadBytes = 256

	// Higher confidence threshold to avoid expensive fallbacks
	config.ConfidenceThreshold = 0.7

	return config
}

// HighAccuracyDetectionConfig returns a configuration optimized for accuracy
func HighAccuracyDetectionConfig() *DetectionConfig {
	config := DefaultDetectionConfig()

	// Sequential detection for more thorough analysis
	config.EnableParallel = false

	// Lower confidence threshold to use more detectors
	config.ConfidenceThreshold = 0.3

	// More thorough content analysis
	config.ContentDetectorConfig.MaxAnalysisBytes = 16384
	config.MIMEDetectorConfig.MaxReadBytes = 1024

	// Lower thresholds for more sensitive detection
	config.ContentDetectorConfig.MarkdownConfidenceThreshold = 0.15
	config.ContentDetectorConfig.BinaryDetectionThreshold = 0.2

	return config
}

// Validate checks if the configuration is valid
func (c *DetectionConfig) Validate() error {
	// Validate confidence threshold
	if c.ConfidenceThreshold < 0.0 || c.ConfidenceThreshold > 1.0 {
		return models.NewDocumentError(models.ErrInvalidConfig,
			"confidence threshold must be between 0.0 and 1.0")
	}

	// At least one detector must be enabled
	if !c.EnableExtensionDetection && !c.EnableMIMEDetection && !c.EnableContentDetection && !c.EnableTikaDetection {
		return models.NewDocumentError(models.ErrInvalidConfig,
			"at least one detector must be enabled")
	}

	// Validate extension detector config
	if c.EnableExtensionDetection {
		if c.ExtensionDetectorConfig.Priority < 0 {
			return models.NewDocumentError(models.ErrInvalidConfig,
				"extension detector priority must be non-negative")
		}
	}

	// Validate MIME detector config
	if c.EnableMIMEDetection {
		if c.MIMEDetectorConfig.Priority < 0 {
			return models.NewDocumentError(models.ErrInvalidConfig,
				"MIME detector priority must be non-negative")
		}
		if c.MIMEDetectorConfig.MaxReadBytes <= 0 {
			return models.NewDocumentError(models.ErrInvalidConfig,
				"MIME detector max read bytes must be positive")
		}
	}

	// Validate content detector config
	if c.EnableContentDetection {
		if c.ContentDetectorConfig.Priority < 0 {
			return models.NewDocumentError(models.ErrInvalidConfig,
				"content detector priority must be non-negative")
		}
		if c.ContentDetectorConfig.MaxAnalysisBytes <= 0 {
			return models.NewDocumentError(models.ErrInvalidConfig,
				"content detector max analysis bytes must be positive")
		}
		if c.ContentDetectorConfig.MarkdownConfidenceThreshold < 0.0 ||
			c.ContentDetectorConfig.MarkdownConfidenceThreshold > 1.0 {
			return models.NewDocumentError(models.ErrInvalidConfig,
				"markdown confidence threshold must be between 0.0 and 1.0")
		}
		if c.ContentDetectorConfig.BinaryDetectionThreshold < 0.0 ||
			c.ContentDetectorConfig.BinaryDetectionThreshold > 1.0 {
			return models.NewDocumentError(models.ErrInvalidConfig,
				"binary detection threshold must be between 0.0 and 1.0")
		}
	}

	// Validate Tika detector config
	if c.EnableTikaDetection {
		if c.TikaOfficeConfidence < 0.0 || c.TikaOfficeConfidence > 1.0 {
			return models.NewDocumentError(models.ErrInvalidConfig,
				"Tika office confidence must be between 0.0 and 1.0")
		}
		if c.TikaFallbackConfidence < 0.0 || c.TikaFallbackConfidence > 1.0 {
			return models.NewDocumentError(models.ErrInvalidConfig,
				"Tika fallback confidence must be between 0.0 and 1.0")
		}
	}

	return nil
}

// Clone creates a deep copy of the configuration
func (c *DetectionConfig) Clone() *DetectionConfig {
	clone := &DetectionConfig{
		ConfidenceThreshold:      c.ConfidenceThreshold,
		EnableParallel:           c.EnableParallel,
		EnableExtensionDetection: c.EnableExtensionDetection,
		EnableMIMEDetection:      c.EnableMIMEDetection,
		EnableContentDetection:   c.EnableContentDetection,
		EnableTikaDetection:      c.EnableTikaDetection,
		EnableTikaFallback:       c.EnableTikaFallback,
		TikaOfficeConfidence:     c.TikaOfficeConfidence,
		TikaFallbackConfidence:   c.TikaFallbackConfidence,

		ExtensionDetectorConfig: ExtensionDetectorConfig{
			Priority:         c.ExtensionDetectorConfig.Priority,
			CustomMappings:   make(map[string]string),
			OverrideDefaults: c.ExtensionDetectorConfig.OverrideDefaults,
		},

		MIMEDetectorConfig: MIMEDetectorConfig{
			Priority:         c.MIMEDetectorConfig.Priority,
			CustomMappings:   make(map[string]string),
			OverrideDefaults: c.MIMEDetectorConfig.OverrideDefaults,
			MaxReadBytes:     c.MIMEDetectorConfig.MaxReadBytes,
		},

		ContentDetectorConfig: ContentDetectorConfig{
			Priority:                    c.ContentDetectorConfig.Priority,
			MaxAnalysisBytes:            c.ContentDetectorConfig.MaxAnalysisBytes,
			MarkdownConfidenceThreshold: c.ContentDetectorConfig.MarkdownConfidenceThreshold,
			BinaryDetectionThreshold:    c.ContentDetectorConfig.BinaryDetectionThreshold,
		},
	}

	// Deep copy custom mappings
	for k, v := range c.ExtensionDetectorConfig.CustomMappings {
		clone.ExtensionDetectorConfig.CustomMappings[k] = v
	}
	for k, v := range c.MIMEDetectorConfig.CustomMappings {
		clone.MIMEDetectorConfig.CustomMappings[k] = v
	}

	return clone
}

// MergeWith merges another configuration into this one, with the other config taking precedence
func (c *DetectionConfig) MergeWith(other *DetectionConfig) {
	if other == nil {
		return
	}

	// Merge basic settings
	c.ConfidenceThreshold = other.ConfidenceThreshold
	c.EnableParallel = other.EnableParallel
	c.EnableExtensionDetection = other.EnableExtensionDetection
	c.EnableMIMEDetection = other.EnableMIMEDetection
	c.EnableContentDetection = other.EnableContentDetection
	c.EnableTikaDetection = other.EnableTikaDetection
	c.EnableTikaFallback = other.EnableTikaFallback
	c.TikaOfficeConfidence = other.TikaOfficeConfidence
	c.TikaFallbackConfidence = other.TikaFallbackConfidence

	// Merge detector-specific configs
	c.ExtensionDetectorConfig.Priority = other.ExtensionDetectorConfig.Priority
	c.ExtensionDetectorConfig.OverrideDefaults = other.ExtensionDetectorConfig.OverrideDefaults

	c.MIMEDetectorConfig.Priority = other.MIMEDetectorConfig.Priority
	c.MIMEDetectorConfig.OverrideDefaults = other.MIMEDetectorConfig.OverrideDefaults
	c.MIMEDetectorConfig.MaxReadBytes = other.MIMEDetectorConfig.MaxReadBytes

	c.ContentDetectorConfig.Priority = other.ContentDetectorConfig.Priority
	c.ContentDetectorConfig.MaxAnalysisBytes = other.ContentDetectorConfig.MaxAnalysisBytes
	c.ContentDetectorConfig.MarkdownConfidenceThreshold = other.ContentDetectorConfig.MarkdownConfidenceThreshold
	c.ContentDetectorConfig.BinaryDetectionThreshold = other.ContentDetectorConfig.BinaryDetectionThreshold

	// Merge custom mappings
	for k, v := range other.ExtensionDetectorConfig.CustomMappings {
		c.ExtensionDetectorConfig.CustomMappings[k] = v
	}
	for k, v := range other.MIMEDetectorConfig.CustomMappings {
		c.MIMEDetectorConfig.CustomMappings[k] = v
	}
}