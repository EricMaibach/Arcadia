package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// DocumentProcessor handles document processing operations
type DocumentProcessor struct {
	chunker         interfaces.TextChunkerInterface
	embeddingEngine *EmbeddingEngine
	vectorStore     interfaces.VectorStoreInterface
	documentStore   interfaces.DocumentStoreInterface
	contentAnalyzer interfaces.ContentAnalyzer
	logger          interfaces.Logger
	metrics         interfaces.MetricsCollector

	config ProcessorConfig
}

// ProcessorConfig contains configuration for document processing
type ProcessorConfig struct {
	MaxFileSize       int64    `json:"max_file_size"`        // Maximum file size in bytes
	SupportedFormats  []string `json:"supported_formats"`    // Supported file extensions
	ChunkingConfig    models.ChunkingConfig `json:"chunking_config"`
	ValidateContent   bool     `json:"validate_content"`     // Validate UTF-8 content
	ExtractMetadata   bool     `json:"extract_metadata"`     // Extract file metadata
	SkipDuplicates    bool     `json:"skip_duplicates"`      // Skip files with same hash
	ProcessingTimeout int      `json:"processing_timeout"`   // Timeout in seconds
	RetryAttempts     int      `json:"retry_attempts"`       // Number of retry attempts
	RetryDelay        int      `json:"retry_delay"`          // Delay between retries in seconds
}

// NewDocumentProcessor creates a new document processor
func NewDocumentProcessor(
	chunker interfaces.TextChunkerInterface,
	embeddingEngine *EmbeddingEngine,
	vectorStore interfaces.VectorStoreInterface,
	documentStore interfaces.DocumentStoreInterface,
	config ProcessorConfig,
) *DocumentProcessor {
	return &DocumentProcessor{
		chunker:         chunker,
		embeddingEngine: embeddingEngine,
		vectorStore:     vectorStore,
		documentStore:   documentStore,
		config:          config,
	}
}

// WithContentAnalyzer adds content analysis capabilities
func (dp *DocumentProcessor) WithContentAnalyzer(analyzer interfaces.ContentAnalyzer) *DocumentProcessor {
	dp.contentAnalyzer = analyzer
	return dp
}

// WithLogger adds logging capabilities
func (dp *DocumentProcessor) WithLogger(logger interfaces.Logger) *DocumentProcessor {
	dp.logger = logger
	return dp
}

// WithMetrics adds metrics collection capabilities
func (dp *DocumentProcessor) WithMetrics(metrics interfaces.MetricsCollector) *DocumentProcessor {
	dp.metrics = metrics
	return dp
}

// ProcessFile processes a single file and stores its embeddings
func (dp *DocumentProcessor) ProcessFile(ctx context.Context, filePath string) (*models.Document, error) {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Seconds() * 1000
		if dp.metrics != nil {
			dp.metrics.RecordTimer("document.process.duration", duration, map[string]string{"operation": "single_file"})
		}
	}()

	// Validate file path
	if filePath == "" {
		return nil, models.NewDocumentError(models.ErrFileNotFound, "file path cannot be empty")
	}

	// Validate and sanitize file path to prevent directory traversal attacks
	cleanPath := filepath.Clean(filePath)
	if cleanPath != filePath {
		if dp.logger != nil {
			dp.logger.Warn(ctx, "File path was sanitized", "original", filePath, "cleaned", cleanPath)
		}
		filePath = cleanPath
	}

	// Check for directory traversal attempts
	if strings.Contains(filePath, "..") {
		return nil, models.NewDocumentError(models.ErrFileNotFound, "invalid file path: directory traversal detected")
	}

	// Ensure path doesn't start with sensitive directories
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrFileNotFound, "failed to resolve absolute path", err)
	}

	// Additional security check: ensure we're not accessing system directories
	if dp.isRestrictedPath(absPath) {
		return nil, models.NewDocumentError(models.ErrFileNotFound, "access to system directories is restricted")
	}

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, models.NewDocumentError(models.ErrFileNotFound, fmt.Sprintf("file not found: %s", filePath)).WithFilePath(filePath)
		}
		return nil, models.NewDocumentErrorWithCause(models.ErrFileReadError, "failed to get file info", err).WithFilePath(filePath)
	}

	// Check file size
	if dp.config.MaxFileSize > 0 && fileInfo.Size() > dp.config.MaxFileSize {
		return nil, models.NewDocumentError(models.ErrDocumentTooBig,
			fmt.Sprintf("file size %d exceeds maximum allowed size %d", fileInfo.Size(), dp.config.MaxFileSize)).WithFilePath(filePath)
	}

	// Check if file is supported text type
	isText, contentType, err := dp.IsTextFile(filePath)
	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrUnsupportedFormat, "failed to analyze file type", err).WithFilePath(filePath)
	}

	if !isText {
		return nil, models.NewDocumentError(models.ErrUnsupportedFormat,
			fmt.Sprintf("unsupported file type: %s (detected: %s)", filepath.Ext(filePath), contentType)).WithFilePath(filePath)
	}

	// Read file content
	content, err := dp.ReadTextFile(filePath)
	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrFileReadError, "failed to read file", err).WithFilePath(filePath)
	}

	// Validate content if configured
	if dp.config.ValidateContent {
		if !utf8.ValidString(content) {
			return nil, models.NewDocumentError(models.ErrDocumentInvalid,
				"file contains invalid UTF-8 content").WithFilePath(filePath)
		}
	}

	// Calculate file hash for change detection
	fileHash, err := dp.CalculateFileHash(filePath)
	if err != nil {
		if dp.logger != nil {
			dp.logger.Warn(ctx, "Failed to calculate file hash", "error", err, "path", filePath)
		}
		fileHash = ""
	}

	// Check for duplicates if configured
	if dp.config.SkipDuplicates && fileHash != "" {
		if existing, err := dp.documentStore.GetDocumentByPath(ctx, filePath); err == nil && existing != nil {
			if existing.FileHash == fileHash {
				if dp.logger != nil {
					dp.logger.Info(ctx, "Skipping duplicate file", "path", filePath, "hash", fileHash)
				}
				return existing, nil
			}
		}
	}

	// Create document
	doc := &models.Document{
		ID:       generateDocumentID(),
		FilePath: filePath,
		FileHash: fileHash,
		Content:  content,
		Metadata: map[string]interface{}{
			"source":       "file",
			"path":         filePath,
			"content_type": contentType,
			"file_ext":     filepath.Ext(filePath),
			"size":         fileInfo.Size(),
			"modified":     fileInfo.ModTime(),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Extract additional metadata if configured
	if dp.config.ExtractMetadata && dp.contentAnalyzer != nil {
		if analysis, err := dp.contentAnalyzer.AnalyzeFile(filePath); err == nil {
			doc.Metadata["analysis"] = analysis
		}
	}

	if dp.logger != nil {
		dp.logger.Info(ctx, "Processing text file", "path", filePath, "type", contentType, "size", len(content))
	}

	// Process the document
	if err := dp.ProcessDocument(ctx, doc, content); err != nil {
		return nil, err
	}

	if dp.metrics != nil {
		dp.metrics.IncrementCounter("document.processed", map[string]string{"status": "success"})
	}

	return doc, nil
}

// ProcessDocument processes a document and generates embeddings
func (dp *DocumentProcessor) ProcessDocument(ctx context.Context, doc *models.Document, content string) error {
	// Chunk the text
	chunks := dp.chunker.ChunkText(content, dp.config.ChunkingConfig)
	doc.ChunkCount = len(chunks)

	// Store the document first
	if err := dp.documentStore.StoreDocument(ctx, doc); err != nil {
		return models.NewDocumentErrorWithCause(models.ErrDocumentStoreFailed, "failed to store document", err).WithDocumentID(doc.ID)
	}

	// Process each chunk
	vectorEntries := make([]*models.VectorEntry, 0, len(chunks))

	for i, chunk := range chunks {
		// Set document ID for chunk
		chunk.DocumentID = doc.ID

		// Generate embedding
		embedding, err := dp.embeddingEngine.GenerateEmbedding(ctx, chunk.Content)
		if err != nil {
			return models.NewDocumentErrorWithCause(models.ErrEmbeddingFailed,
				fmt.Sprintf("failed to generate embedding for chunk %d", i), err).WithDocumentID(doc.ID)
		}

		// Validate embedding
		if err := dp.embeddingEngine.ValidateEmbedding(embedding); err != nil {
			return models.NewDocumentErrorWithCause(models.ErrEmbeddingFailed,
				fmt.Sprintf("invalid embedding for chunk %d", i), err).WithDocumentID(doc.ID)
		}

		// Create vector entry
		entry := &models.VectorEntry{
			ID:         generateVectorID(),
			DocumentID: doc.ID,
			ChunkID:    chunk.ID,
			Vector:     embedding,
			Dimension:  len(embedding),
			Content:    chunk.Content,
			Metadata:   fmt.Sprintf(`{"chunk_index":%d,"start_pos":%d,"end_pos":%d}`, i, chunk.StartPos, chunk.EndPos),
			CreatedAt:  time.Now(),
		}

		vectorEntries = append(vectorEntries, entry)
	}

	// Store vectors in batch if supported
	if batchStore, ok := dp.vectorStore.(interface {
		StoreBatch(ctx context.Context, entries []*models.VectorEntry) error
	}); ok {
		if err := batchStore.StoreBatch(ctx, vectorEntries); err != nil {
			return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, "failed to store vector batch", err).WithDocumentID(doc.ID)
		}
	} else {
		// Store vectors individually
		for i, entry := range vectorEntries {
			if err := dp.vectorStore.StoreVector(ctx, entry); err != nil {
				return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed,
					fmt.Sprintf("failed to store vector for chunk %d", i), err).WithDocumentID(doc.ID)
			}
		}
	}

	return nil
}

// ProcessBatch processes multiple files in batch
func (dp *DocumentProcessor) ProcessBatch(ctx context.Context, filePaths []string) ([]models.ProcessResult, error) {
	results := make([]models.ProcessResult, len(filePaths))

	for i, path := range filePaths {
		startTime := time.Now()

		doc, err := dp.ProcessFile(ctx, path)
		result := models.ProcessResult{
			FilePath:    path,
			Success:     err == nil,
			Error:       err,
			Document:    doc,
			ProcessedAt: time.Now(),
		}

		if doc != nil {
			result.DocumentID = doc.ID
		}

		results[i] = result

		// Log processing time
		duration := time.Since(startTime)
		if dp.logger != nil {
			if err != nil {
				dp.logger.Error(ctx, "Failed to process file", "path", path, "error", err, "duration", duration)
			} else {
				dp.logger.Info(ctx, "Processed file", "path", path, "duration", duration, "chunks", doc.ChunkCount)
			}
		}
	}

	return results, nil
}

// IsTextFile determines if a file is a text file using multiple detection methods
func (dp *DocumentProcessor) IsTextFile(filePath string) (bool, string, error) {
	// Method 1: Check by file extension first (fast)
	if isTextByExtension(filePath) {
		return true, "text/plain", nil
	}

	// Method 2: Use HTTP content detection with file sample
	file, err := os.Open(filePath)
	if err != nil {
		return false, "", fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Read first 512 bytes for content type detection
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false, "", fmt.Errorf("failed to read file sample: %v", err)
	}

	// Detect content type using HTTP package
	contentType := http.DetectContentType(buffer[:n])

	// Method 3: Check if detected type is text-based
	if isTextContentType(contentType) {
		return true, contentType, nil
	}

	// Method 4: Binary heuristic - check if content is mostly printable UTF-8
	if n > 0 && isLikelyTextContent(buffer[:n]) {
		return true, "text/plain", nil
	}

	return false, contentType, nil
}

// ReadTextFile reads a text file from the filesystem
func (dp *DocumentProcessor) ReadTextFile(filePath string) (string, error) {
	// Validate and sanitize file path
	cleanPath := filepath.Clean(filePath)
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("invalid file path: directory traversal detected in %s", filePath)
	}

	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for %s: %v", filePath, err)
	}

	if dp.isRestrictedPath(absPath) {
		return "", fmt.Errorf("access to system directories is restricted: %s", filePath)
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %v", cleanPath, err)
	}

	return string(data), nil
}

// CalculateFileHash calculates SHA256 hash of a file
func (dp *DocumentProcessor) CalculateFileHash(filePath string) (string, error) {
	// Validate and sanitize file path
	cleanPath := filepath.Clean(filePath)
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("invalid file path: directory traversal detected in %s", filePath)
	}

	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for %s: %v", filePath, err)
	}

	if dp.isRestrictedPath(absPath) {
		return "", fmt.Errorf("access to system directories is restricted: %s", filePath)
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// generateDocumentID generates a random ID for documents
func generateDocumentID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "doc_" + hex.EncodeToString(bytes)
}

// generateVectorID generates a random UUID for vectors
func generateVectorID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	// Set version (4) and variant bits
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// isTextByExtension checks if file extension indicates text content
func isTextByExtension(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))

	textExtensions := map[string]bool{
		".txt":      true,
		".md":       true,
		".markdown": true,
		".rst":      true,
		".csv":      true,
		".tsv":      true,
		".log":      true,
		".conf":     true,
		".cfg":      true,
		".ini":      true,
		".yaml":     true,
		".yml":      true,
		".json":     true,
		".xml":      true,
		".html":     true,
		".htm":      true,
		".css":      true,
		".js":       true,
		".ts":       true,
		".py":       true,
		".go":       true,
		".java":     true,
		".c":        true,
		".cpp":      true,
		".h":        true,
		".hpp":      true,
		".sh":       true,
		".bash":     true,
		".zsh":      true,
		".fish":     true,
		".ps1":      true,
		".sql":      true,
		".r":        true,
		".rb":       true,
		".php":      true,
		".pl":       true,
		".tex":      true,
		".bib":      true,
		"":          true, // files without extension might be text
	}

	return textExtensions[ext]
}

// isTextContentType checks if HTTP-detected content type is text-based
func isTextContentType(contentType string) bool {
	// Split off charset if present
	mainType := strings.Split(contentType, ";")[0]
	mainType = strings.TrimSpace(strings.ToLower(mainType))

	textTypes := map[string]bool{
		"text/plain":             true,
		"text/html":              true,
		"text/css":               true,
		"text/javascript":        true,
		"text/csv":               true,
		"text/xml":               true,
		"application/json":       true,
		"application/xml":        true,
		"application/javascript": true,
		"application/x-sh":       true,
		"application/x-python":   true,
		"application/x-perl":     true,
		"application/x-ruby":     true,
		"application/x-php":      true,
		"application/sql":        true,
		"application/yaml":       true,
		"application/x-yaml":     true,
	}

	// Also check if it starts with "text/"
	if strings.HasPrefix(mainType, "text/") {
		return true
	}

	return textTypes[mainType]
}

// isLikelyTextContent uses heuristics to determine if binary data is likely text
func isLikelyTextContent(data []byte) bool {
	if len(data) == 0 {
		return true
	}

	// Check if content is valid UTF-8
	if !utf8.Valid(data) {
		return false
	}

	// Reject files with null bytes (common in binary files)
	if slices.Contains(data, 0) {
		return false
	}

	// Count printable vs non-printable characters
	printableCount := 0
	controlCount := 0

	for _, b := range data {
		switch {
		case b >= 32 && b <= 126: // ASCII printable
			printableCount++
		case b == '\t' || b == '\n' || b == '\r': // Common whitespace
			printableCount++
		case b < 32: // Control characters
			controlCount++
		}
	}

	// If more than 85% of characters are printable, consider it text
	totalChars := len(data)
	if totalChars == 0 {
		return true
	}

	printableRatio := float64(printableCount) / float64(totalChars)
	return printableRatio > 0.85
}

// DefaultProcessorConfig returns a default processor configuration
func DefaultProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		MaxFileSize:       50 * 1024 * 1024, // 50MB
		SupportedFormats:  []string{".txt", ".md", ".json", ".csv", ".log"},
		ChunkingConfig:    models.DefaultChunkingConfig(),
		ValidateContent:   true,
		ExtractMetadata:   true,
		SkipDuplicates:    true,
		ProcessingTimeout: 300, // 5 minutes
		RetryAttempts:     3,
		RetryDelay:        5,
	}
}

// FileProcessor handles file system operations for document processing
type FileProcessor struct {
	processor *DocumentProcessor
	logger    interfaces.Logger
}

// NewFileProcessor creates a new file processor
func NewFileProcessor(processor *DocumentProcessor) *FileProcessor {
	return &FileProcessor{
		processor: processor,
	}
}

// WithLogger adds logging to the file processor
func (fp *FileProcessor) WithLogger(logger interfaces.Logger) *FileProcessor {
	fp.logger = logger
	return fp
}

// ProcessDirectory processes all supported files in a directory
func (fp *FileProcessor) ProcessDirectory(ctx context.Context, dirPath string, recursive bool) ([]models.ProcessResult, error) {
	var filePaths []string

	if recursive {
		err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() && fp.isSupportedFile(path) {
				filePaths = append(filePaths, path)
			}

			return nil
		})

		if err != nil {
			return nil, models.NewDocumentErrorWithCause(models.ErrProcessingFailed, "failed to walk directory", err)
		}
	} else {
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return nil, models.NewDocumentErrorWithCause(models.ErrProcessingFailed, "failed to read directory", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				path := filepath.Join(dirPath, entry.Name())
				if fp.isSupportedFile(path) {
					filePaths = append(filePaths, path)
				}
			}
		}
	}

	if fp.logger != nil {
		fp.logger.Info(ctx, "Processing directory", "path", dirPath, "files", len(filePaths), "recursive", recursive)
	}

	return fp.processor.ProcessBatch(ctx, filePaths)
}

// isSupportedFile checks if a file is supported for processing
func (fp *FileProcessor) isSupportedFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))

	for _, supportedExt := range fp.processor.config.SupportedFormats {
		if ext == supportedExt {
			return true
		}
	}

	return isTextByExtension(filePath)
}

// isRestrictedPath checks if the path accesses restricted system directories
func (dp *DocumentProcessor) isRestrictedPath(absPath string) bool {
	restrictedPaths := []string{
		"/etc",
		"/usr/bin",
		"/usr/sbin",
		"/bin",
		"/sbin",
		"/boot",
		"/dev",
		"/proc",
		"/sys",
		"/root",
		"/var/log",
		"/var/lib",
		"/var/run",
	}

	// Normalize path separators for cross-platform compatibility
	normalizedPath := filepath.ToSlash(strings.ToLower(absPath))

	for _, restricted := range restrictedPaths {
		if strings.HasPrefix(normalizedPath, strings.ToLower(restricted)) {
			return true
		}
	}

	// Check for Windows system directories
	if strings.HasPrefix(normalizedPath, "c:/windows") ||
		strings.HasPrefix(normalizedPath, "c:/program files") ||
		strings.HasPrefix(normalizedPath, "c:/system32") {
		return true
	}

	return false
}