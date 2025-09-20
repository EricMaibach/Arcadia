package models

import (
	"fmt"
	"time"
)

// AIError represents an error from an AI service provider
type AIError struct {
	Type     string                 `json:"type"`
	Message  string                 `json:"message"`
	Provider string                 `json:"provider"`
	Code     int                    `json:"code,omitempty"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

// Error implements the error interface
func (e *AIError) Error() string {
	if e.Provider != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Provider, e.Type, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Common error types for AI service operations
const (
	ErrorTypeValidation      = "validation_error"
	ErrorTypeAuth            = "authentication_error"
	ErrorTypeRateLimit       = "rate_limit_error"
	ErrorTypeQuota           = "quota_exceeded"
	ErrorTypeTimeout         = "timeout_error"
	ErrorTypeNetwork         = "network_error"
	ErrorTypeProvider        = "provider_error"
	ErrorTypeConfiguration   = "configuration_error"
	ErrorTypeContextNotFound = "context_not_found"
	ErrorTypeToolNotFound    = "tool_not_found"
	ErrorTypeToolExecution   = "tool_execution_error"
	ErrorTypeInvalidInput    = "invalid_input"
	ErrorTypeInternal        = "internal_error"
	ErrorTypeUnknown         = "unknown_error"
)

// NewAIError creates a new AIError with the specified parameters
func NewAIError(errorType, message, provider string, code int) *AIError {
	return &AIError{
		Type:     errorType,
		Message:  message,
		Provider: provider,
		Code:     code,
		Details:  make(map[string]interface{}),
	}
}

// NewAIErrorWithDetails creates a new AIError with additional details
func NewAIErrorWithDetails(errorType, message, provider string, code int, details map[string]interface{}) *AIError {
	return &AIError{
		Type:     errorType,
		Message:  message,
		Provider: provider,
		Code:     code,
		Details:  details,
	}
}

// WithDetails adds details to an existing error
func (e *AIError) WithDetails(details map[string]interface{}) *AIError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// WithDetail adds a single detail to an existing error
func (e *AIError) WithDetail(key string, value interface{}) *AIError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// IsType checks if the error is of a specific type
func (e *AIError) IsType(errorType string) bool {
	return e.Type == errorType
}

// IsProvider checks if the error is from a specific provider
func (e *AIError) IsProvider(provider string) bool {
	return e.Provider == provider
}

// Common predefined errors
var (
	ErrProviderNotFound      = NewAIError(ErrorTypeProvider, "provider not found", "", 404)
	ErrProviderNotConfigured = NewAIError(ErrorTypeConfiguration, "provider not properly configured", "", 500)
	ErrInvalidConfiguration  = NewAIError(ErrorTypeConfiguration, "invalid configuration", "", 400)
	ErrContextNotFound       = NewAIError(ErrorTypeContextNotFound, "conversation context not found", "", 404)
	ErrToolNotFound          = NewAIError(ErrorTypeToolNotFound, "tool not found", "", 404)
	ErrInvalidInput          = NewAIError(ErrorTypeInvalidInput, "invalid input provided", "", 400)
	ErrRateLimit             = NewAIError(ErrorTypeRateLimit, "rate limit exceeded", "", 429)
	ErrQuotaExceeded         = NewAIError(ErrorTypeQuota, "quota exceeded", "", 429)
	ErrTimeout               = NewAIError(ErrorTypeTimeout, "request timeout", "", 408)
	ErrNetworkError          = NewAIError(ErrorTypeNetwork, "network error", "", 502)
	ErrAuthenticationFailed  = NewAIError(ErrorTypeAuth, "authentication failed", "", 401)
	ErrValidationFailed      = NewAIError(ErrorTypeValidation, "validation failed", "", 400)
	ErrInternalError         = NewAIError(ErrorTypeInternal, "internal error", "", 500)
)

// ValidationError represents a validation error with field-specific details
type ValidationError struct {
	*AIError
	Field string      `json:"field"`
	Value interface{} `json:"value,omitempty"`
	Rule  string      `json:"rule"`
}

// NewValidationError creates a new validation error
func NewValidationError(field, rule, message string, value interface{}) *ValidationError {
	return &ValidationError{
		AIError: NewAIError(ErrorTypeValidation, message, "", 400),
		Field:   field,
		Value:   value,
		Rule:    rule,
	}
}

// Error implements the error interface for ValidationError
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// ProviderError represents a provider-specific error
type ProviderError struct {
	*AIError
	ProviderCode    string `json:"provider_code,omitempty"`
	ProviderMessage string `json:"provider_message,omitempty"`
	Retryable       bool   `json:"retryable"`
}

// NewProviderError creates a new provider error
func NewProviderError(provider, message string, code int, providerCode string, retryable bool) *ProviderError {
	return &ProviderError{
		AIError:         NewAIError(ErrorTypeProvider, message, provider, code),
		ProviderCode:    providerCode,
		ProviderMessage: message,
		Retryable:       retryable,
	}
}

// Error implements the error interface for ProviderError
func (e *ProviderError) Error() string {
	if e.ProviderCode != "" {
		return fmt.Sprintf("[%s:%s] %s", e.Provider, e.ProviderCode, e.Message)
	}
	return e.AIError.Error()
}

// ToolError represents a tool execution error
type ToolError struct {
	*AIError
	ToolName string      `json:"tool_name"`
	Input    interface{} `json:"input,omitempty"`
	Duration float64     `json:"duration_ms,omitempty"`
}

// NewToolError creates a new tool error
func NewToolError(toolName, message string, input interface{}) *ToolError {
	return &ToolError{
		AIError:  NewAIError(ErrorTypeToolExecution, message, "", 500),
		ToolName: toolName,
		Input:    input,
	}
}

// Error implements the error interface for ToolError
func (e *ToolError) Error() string {
	return fmt.Sprintf("tool '%s' error: %s", e.ToolName, e.Message)
}

// ContextError represents a context-related error
type ContextError struct {
	*AIError
	ContextID string `json:"context_id"`
	Operation string `json:"operation"`
}

// NewContextError creates a new context error
func NewContextError(contextID, operation, message string) *ContextError {
	return &ContextError{
		AIError:   NewAIError(ErrorTypeContextNotFound, message, "", 404),
		ContextID: contextID,
		Operation: operation,
	}
}

// Error implements the error interface for ContextError
func (e *ContextError) Error() string {
	return fmt.Sprintf("context '%s' error during %s: %s", e.ContextID, e.Operation, e.Message)
}

// ConfigurationError represents a configuration error
type ConfigurationError struct {
	*AIError
	ConfigKey string      `json:"config_key"`
	Expected  interface{} `json:"expected,omitempty"`
	Actual    interface{} `json:"actual,omitempty"`
}

// NewConfigurationError creates a new configuration error
func NewConfigurationError(configKey, message string, expected, actual interface{}) *ConfigurationError {
	return &ConfigurationError{
		AIError:   NewAIError(ErrorTypeConfiguration, message, "", 400),
		ConfigKey: configKey,
		Expected:  expected,
		Actual:    actual,
	}
}

// Error implements the error interface for ConfigurationError
func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("configuration error for '%s': %s", e.ConfigKey, e.Message)
}

// ErrorList represents a collection of errors
type ErrorList struct {
	Errors []error `json:"errors"`
}

// NewErrorList creates a new error list
func NewErrorList(errors ...error) *ErrorList {
	return &ErrorList{Errors: errors}
}

// Add adds an error to the list
func (el *ErrorList) Add(err error) {
	if err != nil {
		el.Errors = append(el.Errors, err)
	}
}

// HasErrors returns true if there are any errors
func (el *ErrorList) HasErrors() bool {
	return len(el.Errors) > 0
}

// Error implements the error interface for ErrorList
func (el *ErrorList) Error() string {
	if len(el.Errors) == 0 {
		return "no errors"
	}
	if len(el.Errors) == 1 {
		return el.Errors[0].Error()
	}
	return fmt.Sprintf("multiple errors: %d errors occurred", len(el.Errors))
}

// First returns the first error in the list
func (el *ErrorList) First() error {
	if len(el.Errors) > 0 {
		return el.Errors[0]
	}
	return nil
}

// Last returns the last error in the list
func (el *ErrorList) Last() error {
	if len(el.Errors) > 0 {
		return el.Errors[len(el.Errors)-1]
	}
	return nil
}

// ErrorDetails provides detailed error information for debugging
type ErrorDetails struct {
	Error      error                  `json:"error"`
	Timestamp  int64                  `json:"timestamp"`
	Request    interface{}            `json:"request,omitempty"`
	Response   interface{}            `json:"response,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
}

// NewErrorDetails creates new error details
func NewErrorDetails(err error) *ErrorDetails {
	return &ErrorDetails{
		Error:     err,
		Timestamp: time.Now().Unix(),
		Context:   make(map[string]interface{}),
	}
}

// WithRequest adds request information to error details
func (ed *ErrorDetails) WithRequest(request interface{}) *ErrorDetails {
	ed.Request = request
	return ed
}

// WithResponse adds response information to error details
func (ed *ErrorDetails) WithResponse(response interface{}) *ErrorDetails {
	ed.Response = response
	return ed
}

// WithContext adds context information to error details
func (ed *ErrorDetails) WithContext(key string, value interface{}) *ErrorDetails {
	if ed.Context == nil {
		ed.Context = make(map[string]interface{})
	}
	ed.Context[key] = value
	return ed
}

// WithStackTrace adds stack trace to error details
func (ed *ErrorDetails) WithStackTrace(stackTrace string) *ErrorDetails {
	ed.StackTrace = stackTrace
	return ed
}

// Helper functions for error checking

// IsAIError checks if an error is an AIError
func IsAIError(err error) bool {
	_, ok := err.(*AIError)
	return ok
}

// IsValidationError checks if an error is a ValidationError
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// IsProviderError checks if an error is a ProviderError
func IsProviderError(err error) bool {
	_, ok := err.(*ProviderError)
	return ok
}

// IsToolError checks if an error is a ToolError
func IsToolError(err error) bool {
	_, ok := err.(*ToolError)
	return ok
}

// IsContextError checks if an error is a ContextError
func IsContextError(err error) bool {
	_, ok := err.(*ContextError)
	return ok
}

// IsConfigurationError checks if an error is a ConfigurationError
func IsConfigurationError(err error) bool {
	_, ok := err.(*ConfigurationError)
	return ok
}

// GetErrorType returns the error type if it's an AIError
func GetErrorType(err error) string {
	if aiErr, ok := err.(*AIError); ok {
		return aiErr.Type
	}
	return ErrorTypeUnknown
}

// GetErrorProvider returns the provider if it's an AIError
func GetErrorProvider(err error) string {
	if aiErr, ok := err.(*AIError); ok {
		return aiErr.Provider
	}
	return ""
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if provErr, ok := err.(*ProviderError); ok {
		return provErr.Retryable
	}

	// Check error type for retryability
	errorType := GetErrorType(err)
	switch errorType {
	case ErrorTypeTimeout, ErrorTypeNetwork, ErrorTypeRateLimit:
		return true
	case ErrorTypeAuth, ErrorTypeValidation, ErrorTypeConfiguration:
		return false
	default:
		return false
	}
}
