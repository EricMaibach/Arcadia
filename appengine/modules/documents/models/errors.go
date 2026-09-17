package models

import "fmt"

// ErrorCode represents a specific error type for the documents module
type ErrorCode string

const (
	// Document-related errors
	ErrDocumentNotFound     ErrorCode = "DOC_NOT_FOUND"
	ErrDocumentExists       ErrorCode = "DOC_EXISTS"
	ErrDocumentInvalid      ErrorCode = "DOC_INVALID"
	ErrDocumentTooBig       ErrorCode = "DOC_TOO_BIG"
	ErrDocumentDeleteFailed ErrorCode = "DOC_DELETE_FAILED"
	ErrInvalidInput         ErrorCode = "INVALID_INPUT"

	// File-related errors
	ErrUnsupportedFormat ErrorCode = "UNSUPPORTED_FORMAT"
	ErrFileNotFound      ErrorCode = "FILE_NOT_FOUND"
	ErrFileReadError     ErrorCode = "FILE_READ_ERROR"
	ErrFileHashError     ErrorCode = "FILE_HASH_ERROR"

	// Processing errors
	ErrChunkingFailed    ErrorCode = "CHUNKING_FAILED"
	ErrEmbeddingFailed   ErrorCode = "EMBEDDING_FAILED"
	ErrProcessingFailed  ErrorCode = "PROCESSING_FAILED"
	ErrProcessingTimeout ErrorCode = "PROCESSING_TIMEOUT"

	// Storage errors
	ErrStorageFailed       ErrorCode = "STORAGE_FAILED"
	ErrVectorStoreFailed   ErrorCode = "VECTOR_STORE_FAILED"
	ErrDocumentStoreFailed ErrorCode = "DOCUMENT_STORE_FAILED"
	ErrDatabaseError       ErrorCode = "DATABASE_ERROR"

	// Resource errors
	ErrQuotaExceeded       ErrorCode = "QUOTA_EXCEEDED"
	ErrRateLimitExceeded   ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrResourceUnavailable ErrorCode = "RESOURCE_UNAVAILABLE"

	// Configuration errors
	ErrInvalidConfig ErrorCode = "INVALID_CONFIG"
	ErrMissingConfig ErrorCode = "MISSING_CONFIG"

	// Search errors
	ErrSearchFailed  ErrorCode = "SEARCH_FAILED"
	ErrQueryInvalid  ErrorCode = "QUERY_INVALID"
	ErrSearchTimeout ErrorCode = "SEARCH_TIMEOUT"

	// Module errors
	ErrModuleNotInitialized ErrorCode = "MODULE_NOT_INITIALIZED"
	ErrModuleShuttingDown   ErrorCode = "MODULE_SHUTTING_DOWN"
	ErrDependencyMissing    ErrorCode = "DEPENDENCY_MISSING"
)

// DocumentError represents an error specific to the documents module
type DocumentError struct {
	Code       ErrorCode   `json:"code"`
	Message    string      `json:"message"`
	Cause      error       `json:"cause,omitempty"`
	DocumentID string      `json:"document_id,omitempty"`
	FilePath   string      `json:"file_path,omitempty"`
	Context    interface{} `json:"context,omitempty"`
}

// Error implements the error interface
func (e *DocumentError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *DocumentError) Unwrap() error {
	return e.Cause
}

// IsCode checks if the error has the specified error code
func (e *DocumentError) IsCode(code ErrorCode) bool {
	return e.Code == code
}

// NewDocumentError creates a new DocumentError
func NewDocumentError(code ErrorCode, message string) *DocumentError {
	return &DocumentError{
		Code:    code,
		Message: message,
	}
}

// NewDocumentErrorWithCause creates a new DocumentError with an underlying cause
func NewDocumentErrorWithCause(code ErrorCode, message string, cause error) *DocumentError {
	return &DocumentError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// WithDocumentID adds a document ID to the error context
func (e *DocumentError) WithDocumentID(id string) *DocumentError {
	e.DocumentID = id
	return e
}

// WithFilePath adds a file path to the error context
func (e *DocumentError) WithFilePath(path string) *DocumentError {
	e.FilePath = path
	return e
}

// WithContext adds additional context to the error
func (e *DocumentError) WithContext(ctx interface{}) *DocumentError {
	e.Context = ctx
	return e
}

// IsDocumentError checks if an error is a DocumentError
func IsDocumentError(err error) bool {
	_, ok := err.(*DocumentError)
	return ok
}

// GetErrorCode extracts the error code from a DocumentError, returns empty string for other errors
func GetErrorCode(err error) ErrorCode {
	if docErr, ok := err.(*DocumentError); ok {
		return docErr.Code
	}
	return ""
}

// IsRetryable returns true if the error indicates a retryable condition
func IsRetryable(err error) bool {
	if docErr, ok := err.(*DocumentError); ok {
		switch docErr.Code {
		case ErrProcessingTimeout, ErrSearchTimeout, ErrResourceUnavailable, ErrRateLimitExceeded:
			return true
		default:
			return false
		}
	}
	return false
}

// IsTemporary returns true if the error indicates a temporary condition
func IsTemporary(err error) bool {
	if docErr, ok := err.(*DocumentError); ok {
		switch docErr.Code {
		case ErrStorageFailed, ErrVectorStoreFailed, ErrDocumentStoreFailed,
			ErrDatabaseError, ErrResourceUnavailable, ErrRateLimitExceeded,
			ErrProcessingTimeout, ErrSearchTimeout:
			return true
		default:
			return false
		}
	}
	return false
}

// IsPermanent returns true if the error indicates a permanent condition that won't resolve with retry
func IsPermanent(err error) bool {
	if docErr, ok := err.(*DocumentError); ok {
		switch docErr.Code {
		case ErrDocumentNotFound, ErrUnsupportedFormat, ErrFileNotFound,
			ErrDocumentInvalid, ErrDocumentTooBig, ErrInvalidConfig,
			ErrMissingConfig, ErrQueryInvalid, ErrQuotaExceeded:
			return true
		default:
			return false
		}
	}
	return false
}
