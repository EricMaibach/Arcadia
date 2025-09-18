package pdf

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// PDFValidator handles PDF file validation and security checks
type PDFValidator struct {
	config *PDFConfig
	logger interfaces.Logger
}

// NewPDFValidator creates a new PDF validator
func NewPDFValidator(config *PDFConfig) *PDFValidator {
	return &PDFValidator{
		config: config,
	}
}

// WithLogger adds logging to the validator
func (v *PDFValidator) WithLogger(logger interfaces.Logger) *PDFValidator {
	v.logger = logger
	return v
}

// ValidateFile performs comprehensive PDF file validation
func (v *PDFValidator) ValidateFile(ctx context.Context, filePath string) error {
	if v.logger != nil {
		v.logger.Debug(ctx, "Starting PDF validation", "file_path", filePath)
	}

	// Basic file validation
	if err := v.validateBasicFile(filePath); err != nil {
		return err
	}

	// PDF structure validation
	if err := v.validatePDFStructure(filePath); err != nil {
		return err
	}

	// Security checks
	if err := v.performSecurityChecks(ctx, filePath); err != nil {
		return err
	}

	if v.logger != nil {
		v.logger.Debug(ctx, "PDF validation completed successfully", "file_path", filePath)
	}

	return nil
}

// validateBasicFile performs basic file system validation
func (v *PDFValidator) validateBasicFile(filePath string) error {
	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return models.NewDocumentErrorWithCause(models.ErrFileNotFound,
				"PDF file does not exist", err).WithFilePath(filePath)
		}
		return models.NewDocumentErrorWithCause(models.ErrFileReadError,
			"Cannot access PDF file", err).WithFilePath(filePath)
	}

	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return models.NewDocumentError(models.ErrDocumentInvalid,
			"Path is not a regular file").WithFilePath(filePath)
	}

	// Check file size
	if info.Size() > v.config.MaxFileSize {
		return &PDFError{
			Code:    ErrPDFTooBig,
			Message: fmt.Sprintf("PDF file too large: %d bytes (max: %d bytes)",
				info.Size(), v.config.MaxFileSize),
			Context: map[string]interface{}{
				"file_size": info.Size(),
				"max_size":  v.config.MaxFileSize,
				"file_path": filePath,
			},
		}
	}

	// Check if file is empty
	if info.Size() == 0 {
		return models.NewDocumentError(models.ErrDocumentInvalid,
			"PDF file is empty").WithFilePath(filePath)
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".pdf" {
		return models.NewDocumentError(models.ErrUnsupportedFormat,
			"File does not have .pdf extension").WithFilePath(filePath)
	}

	return nil
}

// validatePDFStructure validates the PDF file structure and format
func (v *PDFValidator) validatePDFStructure(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrFileReadError,
			"Cannot open PDF file for validation", err).WithFilePath(filePath)
	}
	defer file.Close()

	// Check PDF header
	if err := v.validatePDFHeader(file); err != nil {
		return err
	}

	// Check PDF footer (look for %%EOF)
	if err := v.validatePDFFooter(file); err != nil {
		return err
	}

	return nil
}

// validatePDFHeader checks for valid PDF header
func (v *PDFValidator) validatePDFHeader(file *os.File) error {
	// Read first 8 bytes to check PDF header
	header := make([]byte, 8)
	n, err := file.Read(header)
	if err != nil {
		return &PDFError{
			Code:    ErrPDFCorrupted,
			Message: "Cannot read PDF header",
			Cause:   err,
		}
	}

	if n < 5 {
		return &PDFError{
			Code:    ErrPDFCorrupted,
			Message: "File too short to be a valid PDF",
		}
	}

	// Check for PDF magic number
	headerStr := string(header[:5])
	if headerStr != "%PDF-" {
		return &PDFError{
			Code:    ErrPDFCorrupted,
			Message: "Invalid PDF header - missing PDF magic number",
			Context: map[string]interface{}{
				"found_header": headerStr,
			},
		}
	}

	return nil
}

// validatePDFFooter checks for valid PDF footer
func (v *PDFValidator) validatePDFFooter(file *os.File) error {
	// Seek to end of file
	stat, err := file.Stat()
	if err != nil {
		return &PDFError{
			Code:    ErrPDFCorrupted,
			Message: "Cannot get file statistics",
			Cause:   err,
		}
	}

	fileSize := stat.Size()

	// Read last 1024 bytes (or entire file if smaller)
	readSize := int64(1024)
	if fileSize < readSize {
		readSize = fileSize
	}

	seekPos := fileSize - readSize
	if _, err := file.Seek(seekPos, 0); err != nil {
		return &PDFError{
			Code:    ErrPDFCorrupted,
			Message: "Cannot seek to end of file",
			Cause:   err,
		}
	}

	// Read the footer portion
	footer := make([]byte, readSize)
	n, err := file.Read(footer)
	if err != nil {
		return &PDFError{
			Code:    ErrPDFCorrupted,
			Message: "Cannot read PDF footer",
			Cause:   err,
		}
	}

	// Look for %%EOF marker
	footerStr := string(footer[:n])
	if !strings.Contains(footerStr, "%%EOF") {
		return &PDFError{
			Code:    ErrPDFCorrupted,
			Message: "Invalid PDF footer - missing %%EOF marker",
		}
	}

	return nil
}

// performSecurityChecks performs security validation
func (v *PDFValidator) performSecurityChecks(ctx context.Context, filePath string) error {
	// Check for password protection
	if err := v.checkPasswordProtected(filePath); err != nil {
		return err
	}

	// Check for path traversal attacks
	if err := v.validateFilePath(filePath); err != nil {
		return err
	}

	// Estimate page count for very large PDFs
	if err := v.validatePageCount(ctx, filePath); err != nil {
		return err
	}

	return nil
}

// checkPasswordProtection checks if PDF is password protected
func (v *PDFValidator) checkPasswordProtected(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrFileReadError,
			"Cannot open PDF for password check", err).WithFilePath(filePath)
	}
	defer file.Close()

	// Simple check: scan for encryption indicators
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	lineCount := 0
	for scanner.Scan() && lineCount < 100 { // Only check first 100 lines
		line := scanner.Text()
		if strings.Contains(line, "/Encrypt") ||
		   strings.Contains(line, "/Filter/Standard") ||
		   strings.Contains(line, "UserPassword") {
			if !v.config.AllowPasswordProtected {
				return &PDFError{
					Code:    ErrPDFPasswordProtected,
					Message: "PDF is password protected and not allowed",
					Context: map[string]interface{}{
						"file_path": filePath,
					},
				}
			}
			break
		}
		lineCount++
	}

	return nil
}

// validateFilePath checks for path traversal and other security issues
func (v *PDFValidator) validateFilePath(filePath string) error {
	// Clean the path
	cleanPath := filepath.Clean(filePath)

	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return models.NewDocumentError(models.ErrInvalidInput,
			"Invalid file path - path traversal detected").WithFilePath(filePath)
	}

	// Ensure path is absolute to avoid relative path issues
	if !filepath.IsAbs(cleanPath) {
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return models.NewDocumentErrorWithCause(models.ErrInvalidInput,
				"Cannot resolve absolute path", err).WithFilePath(filePath)
		}

		// Update the path reference (this is informational)
		if v.logger != nil {
			v.logger.Debug(context.Background(), "Resolved relative path to absolute",
				"original_path", filePath,
				"absolute_path", absPath)
		}
	}

	return nil
}

// validatePageCount estimates and validates PDF page count
func (v *PDFValidator) validatePageCount(ctx context.Context, filePath string) error {
	// This is a simple heuristic - count /Page occurrences
	file, err := os.Open(filePath)
	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrFileReadError,
			"Cannot open PDF for page count check", err).WithFilePath(filePath)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanWords)

	pageCount := 0
	wordCount := 0
	maxWordsToScan := 10000 // Limit scanning to avoid performance issues

	for scanner.Scan() && wordCount < maxWordsToScan {
		word := scanner.Text()
		if word == "/Page" || strings.Contains(word, "/Type/Page") {
			pageCount++
		}
		wordCount++
	}

	// This is a rough estimate, so be conservative
	if pageCount > v.config.MaxPages {
		return &PDFError{
			Code:    ErrPDFTooManyPages,
			Message: fmt.Sprintf("PDF has too many pages: estimated %d (max: %d)",
				pageCount, v.config.MaxPages),
			Context: map[string]interface{}{
				"estimated_pages": pageCount,
				"max_pages":       v.config.MaxPages,
				"file_path":       filePath,
			},
		}
	}

	if v.logger != nil {
		v.logger.Debug(ctx, "PDF page count validation completed",
			"file_path", filePath,
			"estimated_pages", pageCount,
			"max_allowed", v.config.MaxPages)
	}

	return nil
}

// GetFileInfo returns validation-related file information
func (v *PDFValidator) GetFileInfo(filePath string) (map[string]interface{}, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"size":           info.Size(),
		"mod_time":       info.ModTime(),
		"extension":      filepath.Ext(filePath),
		"base_name":      filepath.Base(filePath),
		"is_regular":     info.Mode().IsRegular(),
		"max_file_size":  v.config.MaxFileSize,
		"size_valid":     info.Size() <= v.config.MaxFileSize,
	}

	// Add tool availability
	result["tools_status"] = v.config.GetToolsStatus()

	return result, nil
}