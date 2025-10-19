package audio

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/processors/base"
)

// AudioProcessor handles processing of audio files with Whisper transcription
type AudioProcessor struct {
	base.BaseProcessor
	config    *AudioConfig
	validator *AudioValidator
	client    *WhisperClient
	logger    interfaces.Logger
	metrics   interfaces.MetricsCollector
}

// NewAudioProcessor creates a new audio processor with default configuration
func NewAudioProcessor() *AudioProcessor {
	config := DefaultAudioConfig()
	return NewAudioProcessorWithConfig(config)
}

// NewAudioProcessorWithConfig creates a new audio processor with custom configuration
func NewAudioProcessorWithConfig(config AudioConfig) *AudioProcessor {
	// Validate configuration
	if err := config.Validate(); err != nil {
		// Use default config if validation fails
		config = DefaultAudioConfig()
	}

	// Create components
	validator := NewAudioValidator(&config)
	client := NewWhisperClient(&config)

	// Create base processor
	baseProcessor := base.NewBaseProcessor(
		base.ProcessorTypeAudio,
		config.SupportedFormats,
		nil, // logger will be set later
	)
	baseProcessor.SetMaxFileSize(config.MaxAudioFileSize)
	baseProcessor.SetTimeout(time.Duration(config.WhisperTimeout) * time.Second)

	return &AudioProcessor{
		BaseProcessor: *baseProcessor,
		config:        &config,
		validator:     validator,
		client:        client,
	}
}

// WithLogger adds logging to the audio processor
func (ap *AudioProcessor) WithLogger(logger interfaces.Logger) *AudioProcessor {
	ap.logger = logger
	ap.validator = ap.validator.WithLogger(logger)
	ap.client = ap.client.WithLogger(logger)
	return ap
}

// WithMetrics adds metrics collection to the audio processor
func (ap *AudioProcessor) WithMetrics(metrics interfaces.MetricsCollector) *AudioProcessor {
	ap.metrics = metrics
	ap.client = ap.client.WithMetrics(metrics)
	return ap
}

// CanProcess determines if this processor can handle the given file
func (ap *AudioProcessor) CanProcess(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return false
	}

	return ap.config.IsFormatSupported(ext)
}

// Process extracts content and metadata from an audio file by transcribing it
func (ap *AudioProcessor) Process(ctx context.Context, filePath string) (*base.ProcessingResult, error) {
	if ap.logger != nil {
		ap.logger.Info(ctx, "Starting audio processing",
			"file_path", filePath,
			"processor_type", "audio")
	}

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		if ap.logger != nil {
			ap.logger.Info(ctx, "Audio processing completed",
				"file_path", filePath,
				"duration_ms", duration.Milliseconds())
		}
		if ap.metrics != nil {
			ap.metrics.RecordTimer("audio.processing.duration", duration.Seconds()*1000, nil)
		}
	}()

	// Create processing context with timeout
	processCtx, cancel := context.WithTimeout(ctx, ap.config.GetWhisperTimeoutDuration())
	defer cancel()

	// Step 1: Validate the audio file
	if err := ap.validator.ValidateFile(filePath); err != nil {
		if ap.logger != nil {
			ap.logger.Error(ctx, "Audio validation failed",
				"file_path", filePath,
				"error", err)
		}
		if ap.metrics != nil {
			ap.metrics.IncrementCounter("audio.processing.failed", map[string]string{
				"reason": "validation_failed",
			})
		}
		return nil, fmt.Errorf("audio validation failed: %w", err)
	}

	// Step 2: Get file information
	fileInfo, err := ap.validator.GetFileInfo(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Step 3: Transcribe audio using Whisper
	transcription, transcribeErr := ap.client.Transcribe(processCtx, filePath)

	// Step 4: Handle transcription result or fallback
	if transcribeErr != nil {
		// Check if graceful degradation is enabled
		if ap.config.EnableGracefulDegradation {
			if ap.logger != nil {
				ap.logger.Warn(ctx, "Transcription failed, using graceful degradation",
					"file_path", filePath,
					"error", transcribeErr)
			}
			return ap.createFallbackResult(filePath, fileInfo, transcribeErr)
		}

		// Graceful degradation disabled, return error
		if ap.logger != nil {
			ap.logger.Error(ctx, "Transcription failed",
				"file_path", filePath,
				"error", transcribeErr)
		}
		if ap.metrics != nil {
			ap.metrics.IncrementCounter("audio.processing.failed", map[string]string{
				"reason": "transcription_failed",
			})
		}
		return nil, fmt.Errorf("audio transcription failed: %w", transcribeErr)
	}

	// Step 5: Create processing result with transcription
	result := ap.createSuccessResult(filePath, fileInfo, transcription)

	if ap.metrics != nil {
		ap.metrics.IncrementCounter("audio.processing.success", nil)
	}

	if ap.logger != nil {
		ap.logger.Info(ctx, "Audio processing successful",
			"file_path", filePath,
			"language", transcription.Language,
			"word_count", transcription.WordCount,
			"content_length", len(transcription.Text))
	}

	return result, nil
}

// createSuccessResult creates a successful processing result with transcription
func (ap *AudioProcessor) createSuccessResult(filePath string, fileInfo *AudioFileInfo, transcription *TranscriptionResult) *base.ProcessingResult {
	// Create metadata
	metadata := NewProcessingMetadata(filePath, fileInfo.Extension, fileInfo.Size)
	metadata.TranscriptionLanguage = transcription.Language
	metadata.TranscriptionDuration = transcription.ProcessingTime.Seconds()
	metadata.WordCount = transcription.WordCount
	metadata.WhisperURL = ap.config.WhisperURL
	metadata.WhisperModel = "whisper-base" // Default model
	metadata.LanguageConfidence = transcription.LanguageConfidence

	// Convert to map for ProcessingResult
	metadataMap := map[string]interface{}{
		"original_format":         metadata.OriginalFormat,
		"original_size":           metadata.OriginalSize,
		"file_name":               metadata.FileName,
		"file_path":               metadata.FilePath,
		"transcription_language":  metadata.TranscriptionLanguage,
		"transcription_duration":  metadata.TranscriptionDuration,
		"word_count":              metadata.WordCount,
		"processed_at":            metadata.ProcessedAt,
		"processor_version":       metadata.ProcessorVersion,
		"whisper_url":             metadata.WhisperURL,
		"whisper_model":           metadata.WhisperModel,
		"language_confidence":     metadata.LanguageConfidence,
		"audio_duration":          transcription.Duration,
		"transcribed_at":          transcription.TranscribedAt,
		"processor_type":          "audio",
	}

	// Create processing result
	return &base.ProcessingResult{
		Content:     transcription.Text,
		Metadata:    metadataMap,
		ContentType: "text/plain", // Transcribed text is plain text
		Language:    transcription.Language,
		Confidence:  0.9, // Whisper transcriptions are generally high confidence
	}
}

// createFallbackResult creates a fallback result when transcription fails
func (ap *AudioProcessor) createFallbackResult(filePath string, fileInfo *AudioFileInfo, transcribeErr error) (*base.ProcessingResult, error) {
	if ap.logger != nil {
		ap.logger.Warn(context.Background(), "Creating fallback result for audio file",
			"file_path", filePath,
			"error", transcribeErr)
	}

	// Create basic metadata without transcription
	metadata := map[string]interface{}{
		"original_format":      fileInfo.Extension,
		"original_size":        fileInfo.Size,
		"file_name":            filepath.Base(filePath),
		"file_path":            filePath,
		"processed_at":         time.Now(),
		"processor_version":    "1.0.0",
		"processor_type":       "audio",
		"transcription_error":  transcribeErr.Error(),
		"fallback_mode":        true,
		"language":             ap.config.FallbackLanguage,
	}

	// Create a basic content description
	content := fmt.Sprintf("[Audio file: %s, Format: %s, Size: %d bytes. Transcription unavailable: %v]",
		filepath.Base(filePath),
		fileInfo.Extension,
		fileInfo.Size,
		transcribeErr)

	if ap.metrics != nil {
		ap.metrics.IncrementCounter("audio.processing.fallback", nil)
	}

	return &base.ProcessingResult{
		Content:     content,
		Metadata:    metadata,
		ContentType: "text/plain",
		Language:    ap.config.FallbackLanguage,
		Confidence:  0.1, // Low confidence for fallback
	}, nil
}

// GetSupportedExtensions returns the file extensions this processor supports
func (ap *AudioProcessor) GetSupportedExtensions() []string {
	return ap.config.SupportedFormats
}

// GetProcessorType returns the type of this processor
func (ap *AudioProcessor) GetProcessorType() base.ProcessorType {
	return base.ProcessorTypeAudio
}

// GetCircuitBreakerState returns the current circuit breaker state
func (ap *AudioProcessor) GetCircuitBreakerState() CircuitBreakerState {
	return ap.client.GetCircuitBreakerState()
}

// ResetCircuitBreaker manually resets the circuit breaker
func (ap *AudioProcessor) ResetCircuitBreaker() {
	ap.client.ResetCircuitBreaker()
	if ap.logger != nil {
		ap.logger.Info(context.Background(), "Audio processor circuit breaker reset")
	}
}

// HealthCheck checks if the processor and Whisper service are healthy
func (ap *AudioProcessor) HealthCheck(ctx context.Context) error {
	// Check Whisper service health
	if err := ap.client.HealthCheck(ctx); err != nil {
		return fmt.Errorf("whisper service health check failed: %w", err)
	}

	// Check circuit breaker state
	state := ap.client.GetCircuitBreakerState()
	if state == CircuitBreakerOpen {
		return fmt.Errorf("circuit breaker is open")
	}

	return nil
}

// GetConfig returns the processor configuration
func (ap *AudioProcessor) GetConfig() *AudioConfig {
	return ap.config
}
