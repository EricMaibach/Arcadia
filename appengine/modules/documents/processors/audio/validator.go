package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// AudioValidator validates audio files before processing
type AudioValidator struct {
	config *AudioConfig
	logger interfaces.Logger
}

// NewAudioValidator creates a new audio validator
func NewAudioValidator(config *AudioConfig) *AudioValidator {
	return &AudioValidator{
		config: config,
	}
}

// WithLogger adds logging to the validator
func (av *AudioValidator) WithLogger(logger interfaces.Logger) *AudioValidator {
	av.logger = logger
	return av
}

// ValidateFile performs comprehensive validation on an audio file
func (av *AudioValidator) ValidateFile(filePath string) error {
	// 1. Security: Check for path traversal
	if err := av.validatePath(filePath); err != nil {
		return models.NewDocumentError(models.ErrInvalidConfig, fmt.Sprintf("path validation failed: %v", err))
	}

	// 2. Check file exists and is accessible
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return models.NewDocumentError(models.ErrDocumentNotFound, fmt.Sprintf("file not found: %s", filePath))
		}
		return models.NewDocumentError(models.ErrProcessingFailed, fmt.Sprintf("cannot access file: %v", err))
	}

	// 3. Check it's a file, not a directory
	if fileInfo.IsDir() {
		return models.NewDocumentError(models.ErrInvalidConfig, fmt.Sprintf("path is a directory, not a file: %s", filePath))
	}

	// 4. Validate file extension
	if err := av.validateExtension(filePath); err != nil {
		return err
	}

	// 5. Validate file size
	if err := av.validateFileSize(fileInfo.Size()); err != nil {
		return err
	}

	// 6. Validate file format via magic bytes (optional but recommended)
	if err := av.validateAudioFormat(filePath); err != nil {
		if av.logger != nil {
			av.logger.Warn(nil, "Audio format validation failed, but proceeding",
				"file_path", filePath,
				"error", err)
		}
		// Don't fail validation on magic bytes check - just log warning
		// Some valid audio files might not have standard magic bytes
	}

	return nil
}

// validatePath checks for path traversal attempts and restricted paths
func (av *AudioValidator) validatePath(filePath string) error {
	// Check for path traversal patterns
	if strings.Contains(filePath, "..") {
		return fmt.Errorf("path traversal detected: %s", filePath)
	}

	// Convert to absolute path for additional checks
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check for common restricted directories (Unix-based systems)
	restrictedDirs := []string{"/etc", "/sys", "/proc", "/dev", "/root"}
	for _, restricted := range restrictedDirs {
		if strings.HasPrefix(absPath, restricted) {
			return fmt.Errorf("access to restricted directory denied: %s", restricted)
		}
	}

	return nil
}

// validateExtension checks if the file extension is supported
func (av *AudioValidator) validateExtension(filePath string) error {
	ext := strings.ToLower(filepath.Ext(filePath))

	if ext == "" {
		return models.NewDocumentError(models.ErrInvalidConfig, "file has no extension")
	}

	if !av.config.IsFormatSupported(ext) {
		return models.NewDocumentError(
			models.ErrInvalidConfig,
			fmt.Sprintf("unsupported audio format: %s (supported: %v)", ext, av.config.SupportedFormats),
		)
	}

	return nil
}

// validateFileSize checks if the file size is within acceptable limits
func (av *AudioValidator) validateFileSize(size int64) error {
	if size == 0 {
		return models.NewDocumentError(models.ErrInvalidConfig, "file is empty (0 bytes)")
	}

	if size > av.config.MaxAudioFileSize {
		return models.NewDocumentError(
			models.ErrInvalidConfig,
			fmt.Sprintf("file size (%d bytes) exceeds maximum allowed size (%d bytes)",
				size, av.config.MaxAudioFileSize),
		)
	}

	return nil
}

// validateAudioFormat validates the file is actually an audio file by checking magic bytes
func (av *AudioValidator) validateAudioFormat(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("cannot open file for format validation: %w", err)
	}
	defer file.Close()

	// Read first 12 bytes for magic number detection
	header := make([]byte, 12)
	n, err := file.Read(header)
	if err != nil {
		return fmt.Errorf("cannot read file header: %w", err)
	}
	if n < 4 {
		return fmt.Errorf("file too small to validate format (less than 4 bytes)")
	}

	// Check magic bytes for common audio formats
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".mp3":
		// MP3 files can start with ID3 tag (0x49 0x44 0x33) or MPEG frame sync (0xFF 0xFB or 0xFF 0xFA)
		if (n >= 3 && header[0] == 0x49 && header[1] == 0x44 && header[2] == 0x33) ||
			(n >= 2 && header[0] == 0xFF && (header[1] == 0xFB || header[1] == 0xFA || header[1] == 0xF3 || header[1] == 0xF2)) {
			return nil
		}
		return fmt.Errorf("file does not appear to be a valid MP3 (invalid magic bytes)")

	case ".wav":
		// WAV files start with "RIFF" and contain "WAVE"
		if n >= 12 && string(header[0:4]) == "RIFF" && string(header[8:12]) == "WAVE" {
			return nil
		}
		return fmt.Errorf("file does not appear to be a valid WAV (expected RIFF...WAVE header)")

	case ".flac":
		// FLAC files start with "fLaC"
		if n >= 4 && string(header[0:4]) == "fLaC" {
			return nil
		}
		return fmt.Errorf("file does not appear to be a valid FLAC (expected fLaC header)")

	case ".ogg":
		// OGG files start with "OggS"
		if n >= 4 && string(header[0:4]) == "OggS" {
			return nil
		}
		return fmt.Errorf("file does not appear to be a valid OGG (expected OggS header)")

	case ".m4a", ".aac":
		// M4A/AAC files start with specific patterns
		// M4A: typically starts with 0x00 0x00 0x00 and then "ftyp" or "ftypM4A"
		if n >= 8 && header[4] == 0x66 && header[5] == 0x74 && header[6] == 0x79 && header[7] == 0x70 {
			return nil
		}
		// AAC ADTS: starts with 0xFF 0xF1 or 0xFF 0xF9
		if n >= 2 && header[0] == 0xFF && (header[1] == 0xF1 || header[1] == 0xF9) {
			return nil
		}
		return fmt.Errorf("file does not appear to be a valid M4A/AAC")

	default:
		// For other formats, skip magic byte check
		return nil
	}
}

// GetFileInfo returns basic information about a file
func (av *AudioValidator) GetFileInfo(filePath string) (*AudioFileInfo, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot stat file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))

	return &AudioFileInfo{
		Path:      filePath,
		Size:      fileInfo.Size(),
		Extension: ext,
		ModTime:   fileInfo.ModTime(),
	}, nil
}

// AudioFileInfo contains information about an audio file
type AudioFileInfo struct {
	Path      string
	Size      int64
	Extension string
	ModTime   time.Time
}
