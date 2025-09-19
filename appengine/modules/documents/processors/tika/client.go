package tika

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"arcadia/modules/documents/interfaces"
)

// TikaClient provides a robust HTTP client for Apache Tika server
type TikaClient struct {
	config           *TikaConfig
	httpClient       *http.Client
	circuitBreaker   *CircuitBreaker
	logger           interfaces.Logger
	allowedExtensions map[string]bool // Map for fast extension lookup
}

// NewTikaClient creates a new Tika client with the given configuration
func NewTikaClient(config *TikaConfig, logger interfaces.Logger) *TikaClient {
	if config == nil {
		config = DefaultTikaConfig()
	}

	// Create HTTP client with connection pooling and timeouts
	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        config.MaxConnections,
			MaxIdleConnsPerHost: config.MaxConnections,
			IdleConnTimeout:     config.IdleConnTimeout,
			DisableCompression:  true, // Let Tika handle compression
		},
	}

	// Create circuit breaker
	circuitBreaker := NewCircuitBreaker(config.CircuitBreaker)

	// Build allowed extensions map
	allowedExtensions := make(map[string]bool)

	// Default allowed extensions for security
	defaultExtensions := []string{".pdf", ".doc", ".docx", ".txt", ".rtf", ".odt", ".ppt", ".pptx", ".xls", ".xlsx", ".csv", ".html", ".htm", ".xml", ".json"}

	// If AcceptAllFormats is enabled, be more permissive
	if config.AcceptAllFormats {
		// Add more extensions for fallback mode
		extendedExtensions := []string{".ps", ".eps", ".wpd", ".wps", ".pages", ".numbers", ".key", ".odg", ".svg"}
		defaultExtensions = append(defaultExtensions, extendedExtensions...)
	}

	// Build the map for fast lookup
	for _, ext := range defaultExtensions {
		allowedExtensions[strings.ToLower(ext)] = true
	}

	client := &TikaClient{
		config:           config,
		httpClient:       httpClient,
		circuitBreaker:   circuitBreaker,
		logger:           logger,
		allowedExtensions: allowedExtensions,
	}

	return client
}

// validateFilePath performs comprehensive security validation on file paths
func (c *TikaClient) validateFilePath(filePath string) error {
	// Clean and validate the path
	cleanPath := filepath.Clean(filePath)

	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		if c.logger != nil {
			c.logger.Warn(context.Background(), "Path traversal attempt detected", "path", cleanPath)
		}
		return fmt.Errorf("invalid file path")
	}

	// Get absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		if c.logger != nil {
			c.logger.Warn(context.Background(), "Failed to resolve absolute path", "path", cleanPath, "error", err.Error())
		}
		return fmt.Errorf("invalid file path")
	}

	// Check against restricted directories
	restrictedDirs := []string{"/etc", "/proc", "/sys", "/dev", "/root", "/usr/bin", "/usr/sbin", "/boot", "/var/log"}
	for _, restricted := range restrictedDirs {
		if strings.HasPrefix(absPath, restricted) {
			if c.logger != nil {
				c.logger.Warn(context.Background(), "Attempted access to restricted directory", "restricted_dir", restricted, "attempted_path", absPath)
			}
			return fmt.Errorf("access denied")
		}
	}

	// Verify file exists and is accessible
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found")
		}
		if os.IsPermission(err) {
			if c.logger != nil {
				c.logger.Warn(context.Background(), "Permission denied accessing file", "path", absPath)
			}
			return fmt.Errorf("access denied")
		}
		return fmt.Errorf("file not accessible")
	}

	// Check if it's a regular file
	if !fileInfo.Mode().IsRegular() {
		if c.logger != nil {
			c.logger.Warn(context.Background(), "Attempted to process non-regular file", "path", absPath, "mode", fileInfo.Mode().String())
		}
		return fmt.Errorf("invalid file type")
	}

	// Check file size against limits
	if c.config.MaxFileSize > 0 && fileInfo.Size() > c.config.MaxFileSize {
		if c.logger != nil {
			c.logger.Warn(context.Background(), "File size exceeds limit", "size", fileInfo.Size(), "limit", c.config.MaxFileSize)
		}
		return fmt.Errorf("file too large")
	}

	// Basic file type validation
	ext := strings.ToLower(filepath.Ext(absPath))

	// Check if extension is allowed
	if ext != "" && !c.allowedExtensions[ext] {
		if c.logger != nil {
			c.logger.Warn(context.Background(), "Unsupported file extension", "extension", ext, "path", absPath)
		}
		return fmt.Errorf("unsupported file type")
	}

	return nil
}

// ExtractText extracts text content from a file using Tika
func (c *TikaClient) ExtractText(ctx context.Context, filePath string) (string, error) {
	// Validate file path for security
	if err := c.validateFilePath(filePath); err != nil {
		return "", err
	}

	endpoint := "/tika"

	var content string
	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		result, err := c.makeRequest(ctx, "PUT", endpoint, filePath, "text/plain")
		if err != nil {
			return err
		}
		content = result
		return nil
	})

	return content, err
}

// ExtractMetadata extracts metadata from a file using Tika
func (c *TikaClient) ExtractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error) {
	// Validate file path for security
	if err := c.validateFilePath(filePath); err != nil {
		return nil, err
	}

	endpoint := "/meta"

	var metadata map[string]interface{}
	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		result, err := c.makeRequest(ctx, "PUT", endpoint, filePath, "application/json")
		if err != nil {
			return err
		}

		// Parse JSON metadata
		parsed, err := parseJSONMetadata(result)
		if err != nil {
			return fmt.Errorf("failed to parse metadata JSON: %w", err)
		}

		metadata = parsed
		return nil
	})

	return metadata, err
}

// DetectType detects the MIME type of a file using Tika
func (c *TikaClient) DetectType(ctx context.Context, filePath string) (string, error) {
	// Validate file path for security
	if err := c.validateFilePath(filePath); err != nil {
		return "", err
	}

	endpoint := "/detect/stream"

	var mimeType string
	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		result, err := c.makeRequest(ctx, "PUT", endpoint, filePath, "text/plain")
		if err != nil {
			return err
		}
		mimeType = strings.TrimSpace(result)
		return nil
	})

	return mimeType, err
}

// HealthCheck checks if the Tika server is available and responsive
func (c *TikaClient) HealthCheck(ctx context.Context) error {
	return c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, "GET", c.config.ServerURL+"/tika", nil)
		if err != nil {
			return fmt.Errorf("failed to create health check request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("health check request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
		}

		return nil
	})
}

// makeRequest makes an HTTP request to Tika server with file streaming
// Note: filePath is already validated by calling methods for security
func (c *TikaClient) makeRequest(ctx context.Context, method, endpoint, filePath, acceptType string) (string, error) {
	// Get file info (file already validated by caller)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		// Sanitize error message to avoid path disclosure
		return "", fmt.Errorf("file access error")
	}

	// Open file for streaming
	file, err := os.Open(filePath)
	if err != nil {
		// Sanitize error message to avoid path disclosure
		return "", fmt.Errorf("file access error")
	}

	// Ensure file is closed properly even on context cancellation
	defer func() {
		if file != nil {
			file.Close()
		}
	}()

	// Create request with proper context handling
	url := c.config.ServerURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, file)
	if err != nil {
		return "", fmt.Errorf("failed to create request")
	}

	// Set headers
	req.Header.Set("Accept", acceptType)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))

	// Set Tika-specific headers for better processing
	req.Header.Set("X-Tika-PDFextractInlineImages", "true")
	req.Header.Set("X-Tika-PDFOcrStrategy", "ocr_only")
	req.Header.Set("X-Tika-OCRLanguage", "eng")

	// Execute request with retries and proper context handling
	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		// Check for context cancellation before each attempt
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		if attempt > 0 {
			// Reset file position for retry
			_, err := file.Seek(0, io.SeekStart)
			if err != nil {
				return "", fmt.Errorf("file read error")
			}

			// Wait before retry with context cancellation check
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		resp, lastErr = c.httpClient.Do(req)
		if lastErr != nil {
			// Check if error is due to context cancellation
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
				continue
			}
		}

		if resp.StatusCode == http.StatusOK {
			break
		}

		resp.Body.Close()
		lastErr = fmt.Errorf("server error")
	}

	if lastErr != nil {
		return "", fmt.Errorf("request failed after retries")
	}

	// Ensure response body is closed even on context cancellation
	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()

	// Read response body with context cancellation handling
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// Check if error is due to context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			return "", fmt.Errorf("response read error")
		}
	}

	return string(body), nil
}

// GetCircuitBreakerStats returns current circuit breaker statistics
func (c *TikaClient) GetCircuitBreakerStats() CircuitBreakerStats {
	return c.circuitBreaker.GetStats()
}

// ResetCircuitBreaker manually resets the circuit breaker
func (c *TikaClient) ResetCircuitBreaker() {
	c.circuitBreaker.Reset()
}

// Close closes the HTTP client and cleans up resources
func (c *TikaClient) Close() error {
	// Close idle connections
	c.httpClient.CloseIdleConnections()
	return nil
}

// parseJSONMetadata parses JSON metadata response from Tika
func parseJSONMetadata(jsonStr string) (map[string]interface{}, error) {
	var metadata map[string]interface{}

	err := json.Unmarshal([]byte(jsonStr), &metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON metadata: %w", err)
	}

	return metadata, nil
}

// TikaError represents an error from the Tika server
type TikaError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func (e TikaError) Error() string {
	return fmt.Sprintf("tika error (status %d): %s", e.Status, e.Message)
}

// IsTikaError checks if an error is a TikaError
func IsTikaError(err error) (*TikaError, bool) {
	tikaErr, ok := err.(TikaError)
	return &tikaErr, ok
}