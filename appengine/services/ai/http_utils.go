package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient provides common HTTP functionality for AI services
type HTTPClient struct {
	client  *http.Client
	baseURL string
	headers map[string]string
}

// NewHTTPClient creates a new HTTP client with the specified timeout
func NewHTTPClient(timeoutSeconds int) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
		headers: make(map[string]string),
	}
}

// SetBaseURL sets the base URL for all requests
func (hc *HTTPClient) SetBaseURL(url string) {
	hc.baseURL = url
}

// SetHeader sets a header that will be included in all requests
func (hc *HTTPClient) SetHeader(key, value string) {
	hc.headers[key] = value
}

// SetHeaders sets multiple headers that will be included in all requests
func (hc *HTTPClient) SetHeaders(headers map[string]string) {
	for key, value := range headers {
		hc.headers[key] = value
	}
}

// PostJSON performs a POST request with JSON payload
func (hc *HTTPClient) PostJSON(endpoint string, payload any) (*http.Response, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", hc.baseURL+endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range hc.headers {
		req.Header.Set(key, value)
	}

	return hc.client.Do(req)
}

// Post performs a POST request with string payload
func (hc *HTTPClient) Post(endpoint string, payload string, contentType string) (*http.Response, error) {
	req, err := http.NewRequest("POST", hc.baseURL+endpoint, bytes.NewReader([]byte(payload)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	for key, value := range hc.headers {
		req.Header.Set(key, value)
	}

	return hc.client.Do(req)
}

// Get performs a GET request
func (hc *HTTPClient) Get(endpoint string) (*http.Response, error) {
	req, err := http.NewRequest("GET", hc.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range hc.headers {
		req.Header.Set(key, value)
	}

	return hc.client.Do(req)
}

// HandleErrorResponse checks response status and creates appropriate AI errors
func (hc *HTTPClient) HandleErrorResponse(resp *http.Response, provider string) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)

	// Determine error type based on status code
	var errorType string
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		errorType = ErrorTypeAuth
	case http.StatusTooManyRequests:
		errorType = ErrorTypeRateLimit
	case http.StatusPaymentRequired, http.StatusForbidden:
		errorType = ErrorTypeQuota
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		errorType = ErrorTypeTimeout
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		errorType = ErrorTypeNetwork
	default:
		errorType = ErrorTypeProvider
	}

	return &AIError{
		Type:     errorType,
		Message:  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
		Provider: provider,
		Code:     resp.StatusCode,
	}
}

// DecodeJSONResponse decodes a JSON response into the provided structure
func (hc *HTTPClient) DecodeJSONResponse(resp *http.Response, target any) error {
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode JSON response: %w", err)
	}

	return nil
}

// ReadResponseBody reads the entire response body as a string
func (hc *HTTPClient) ReadResponseBody(resp *http.Response) (string, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// ValidateResponse checks if the response is successful and handles errors
func (hc *HTTPClient) ValidateResponse(resp *http.Response, provider string) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return hc.HandleErrorResponse(resp, provider)
}

// SetTimeout updates the HTTP client timeout
func (hc *HTTPClient) SetTimeout(timeoutSeconds int) {
	hc.client.Timeout = time.Duration(timeoutSeconds) * time.Second
}

// GetTimeout returns the current timeout setting
func (hc *HTTPClient) GetTimeout() time.Duration {
	return hc.client.Timeout
}

// Clone creates a copy of the HTTP client with the same configuration
func (hc *HTTPClient) Clone() *HTTPClient {
	newHeaders := make(map[string]string)
	for k, v := range hc.headers {
		newHeaders[k] = v
	}

	return &HTTPClient{
		client: &http.Client{
			Timeout: hc.client.Timeout,
		},
		baseURL: hc.baseURL,
		headers: newHeaders,
	}
}
