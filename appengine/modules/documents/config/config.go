package config

import (
	"arcadia/modules/documents/processors/audio"
	"arcadia/modules/documents/processors/base"
)

// ProcessingConfig contains configuration for the new plugin-based document processing system
type ProcessingConfig struct {
	// PluginRegistry configures the plugin registry system
	PluginRegistry base.PluginRegistryConfig `yaml:"plugin_registry" json:"plugin_registry"`

	// PDF contains configuration for PDF processing
	PDF base.PDFConfig `yaml:"pdf" json:"pdf"`

	// Text contains configuration for text processing
	Text base.TextConfig `yaml:"text" json:"text"`

	// Markdown contains configuration for Markdown processing
	Markdown base.MarkdownConfig `yaml:"markdown" json:"markdown"`

	// Audio contains configuration for audio processing
	Audio audio.AudioConfig `yaml:"audio" json:"audio"`
}

// GetDefaultProcessingConfig returns default configuration for document processing
func GetDefaultProcessingConfig() ProcessingConfig {
	return ProcessingConfig{
		PluginRegistry: base.GetDefaultPluginRegistryConfig(),
		PDF:            base.GetDefaultPDFConfig(),
		Text:           base.GetDefaultTextConfig(),
		Markdown:       base.GetDefaultMarkdownConfig(),
		Audio:          audio.DefaultAudioConfig(),
	}
}
