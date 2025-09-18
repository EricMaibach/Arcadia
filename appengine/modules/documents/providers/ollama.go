package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// OllamaEmbeddingProvider implements EmbeddingProvider interface using Ollama API
type OllamaEmbeddingProvider struct {
	baseURL     string
	modelName   string
	dimension   int
	timeout     time.Duration
	client      *http.Client
	initialized bool
	logger      interfaces.Logger
	metrics     interfaces.MetricsCollector
	mutex       sync.RWMutex
}

// OllamaEmbeddingRequest represents the request to Ollama API
type OllamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// OllamaEmbeddingResponse represents the response from Ollama API
type OllamaEmbeddingResponse struct {
	Embedding          []float64 `json:"embedding"`
	PromptEvalCount    int       `json:"prompt_eval_count"`
	PromptEvalDuration int64     `json:"prompt_eval_duration"`
}

// NewOllamaEmbeddingProvider creates a new Ollama embedding provider
func NewOllamaEmbeddingProvider(baseURL, modelName string) *OllamaEmbeddingProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if modelName == "" {
		modelName = "embeddinggemma"
	}

	timeout := 60 * time.Second

	return &OllamaEmbeddingProvider{
		baseURL:   baseURL,
		modelName: modelName,
		dimension: 768, // EmbeddingGemma default dimensions
		timeout:   timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// WithLogger adds logging to the Ollama provider
func (p *OllamaEmbeddingProvider) WithLogger(logger interfaces.Logger) *OllamaEmbeddingProvider {
	p.logger = logger
	return p
}

// WithMetrics adds metrics collection to the Ollama provider
func (p *OllamaEmbeddingProvider) WithMetrics(metrics interfaces.MetricsCollector) *OllamaEmbeddingProvider {
	p.metrics = metrics
	return p
}

// Initialize initializes the Ollama embedding provider
func (p *OllamaEmbeddingProvider) Initialize(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.initialized {
		return nil
	}

	// Test connection to Ollama API
	testURL := p.baseURL + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrEmbeddingFailed, "failed to create test request", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrEmbeddingFailed,
			fmt.Sprintf("failed to connect to Ollama at %s", p.baseURL), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.NewDocumentError(models.ErrEmbeddingFailed,
			fmt.Sprintf("Ollama API returned status %d", resp.StatusCode))
	}

	p.initialized = true

	if p.logger != nil {
		p.logger.Info(ctx, "Successfully initialized Ollama embedding provider",
			"base_url", p.baseURL, "model", p.modelName, "dimension", p.dimension)
	}

	return nil
}

// GenerateEmbedding generates an embedding for the given text using Ollama API
func (p *OllamaEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if !p.initialized {
		return nil, models.NewDocumentError(models.ErrEmbeddingFailed, "provider not initialized")
	}

	if text == "" {
		return nil, models.NewDocumentError(models.ErrEmbeddingFailed, "text cannot be empty")
	}

	startTime := time.Now()
	defer func() {
		if p.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			p.metrics.RecordTimer("embedding.provider.ollama.generate.duration", duration, nil)
		}
	}()

	// Prepare request
	request := OllamaEmbeddingRequest{
		Model:  p.modelName,
		Prompt: text,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		if p.metrics != nil {
			p.metrics.IncrementCounter("embedding.provider.ollama.marshal.error", nil)
		}
		return nil, models.NewDocumentErrorWithCause(models.ErrEmbeddingFailed, "failed to marshal request", err)
	}

	// Make API call
	apiURL := p.baseURL + "/api/embeddings"

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrEmbeddingFailed, "failed to create request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		if p.metrics != nil {
			p.metrics.IncrementCounter("embedding.provider.ollama.api.error", nil)
		}
		return nil, models.NewDocumentErrorWithCause(models.ErrEmbeddingFailed, "failed to call Ollama API", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if p.metrics != nil {
			p.metrics.IncrementCounter("embedding.provider.ollama.api.error", nil)
		}
		return nil, models.NewDocumentError(models.ErrEmbeddingFailed,
			fmt.Sprintf("Ollama API returned status %d", resp.StatusCode))
	}

	// Parse response
	var response OllamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		if p.metrics != nil {
			p.metrics.IncrementCounter("embedding.provider.ollama.decode.error", nil)
		}
		return nil, models.NewDocumentErrorWithCause(models.ErrEmbeddingFailed, "failed to decode response", err)
	}

	// Convert float64 to float32
	embedding := make([]float32, len(response.Embedding))
	for i, v := range response.Embedding {
		embedding[i] = float32(v)
	}

	// Update dimension if it changed
	if len(embedding) > 0 && p.dimension != len(embedding) {
		p.dimension = len(embedding)
		if p.logger != nil {
			p.logger.Info(ctx, "Updated embedding dimension", "new_dimension", p.dimension)
		}
	}

	if p.metrics != nil {
		p.metrics.IncrementCounter("embedding.provider.ollama.generate.success", nil)
	}

	return embedding, nil
}

// GenerateEmbeddings generates embeddings for multiple texts in batch
func (p *OllamaEmbeddingProvider) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// Ollama doesn't have native batch support, so we'll process individually
	// This could be optimized with concurrent requests in the future
	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		embedding, err := p.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, models.NewDocumentErrorWithCause(models.ErrEmbeddingFailed,
				fmt.Sprintf("failed to generate embedding for text %d", i), err)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

// GetModelName returns the model name
func (p *OllamaEmbeddingProvider) GetModelName() string {
	return p.modelName
}

// GetDimension returns the embedding dimension
func (p *OllamaEmbeddingProvider) GetDimension() int {
	return p.dimension
}

// HealthCheck performs a health check on the provider
func (p *OllamaEmbeddingProvider) HealthCheck(ctx context.Context) error {
	// Test with a simple text
	testText := "health check test"
	_, err := p.GenerateEmbedding(ctx, testText)
	return err
}

// Close closes the Ollama provider
func (p *OllamaEmbeddingProvider) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.initialized {
		return nil
	}

	// Clean up HTTP client resources if needed
	// For now, just mark as not initialized
	p.initialized = false
	return nil
}

// MockEmbeddingProvider is a mock implementation for testing
type MockEmbeddingProvider struct {
	modelName   string
	dimension   int
	initialized bool
	shouldError bool
	errorMsg    string
	logger      interfaces.Logger
	metrics     interfaces.MetricsCollector
}

// NewMockEmbeddingProvider creates a new mock embedding provider
func NewMockEmbeddingProvider(modelName string, dimension int) *MockEmbeddingProvider {
	if modelName == "" {
		modelName = "mock-model"
	}
	if dimension <= 0 {
		dimension = 384
	}

	return &MockEmbeddingProvider{
		modelName: modelName,
		dimension: dimension,
	}
}

// WithLogger adds logging to the mock provider
func (p *MockEmbeddingProvider) WithLogger(logger interfaces.Logger) *MockEmbeddingProvider {
	p.logger = logger
	return p
}

// WithMetrics adds metrics collection to the mock provider
func (p *MockEmbeddingProvider) WithMetrics(metrics interfaces.MetricsCollector) *MockEmbeddingProvider {
	p.metrics = metrics
	return p
}

// SetError sets the mock to return errors
func (p *MockEmbeddingProvider) SetError(shouldError bool, errorMsg string) {
	p.shouldError = shouldError
	p.errorMsg = errorMsg
}

// Initialize initializes the mock provider
func (p *MockEmbeddingProvider) Initialize(ctx context.Context) error {
	if p.shouldError {
		return models.NewDocumentError(models.ErrEmbeddingFailed, fmt.Sprintf("mock error: %s", p.errorMsg))
	}
	p.initialized = true
	return nil
}

// GenerateEmbedding generates a mock embedding
func (p *MockEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if p.shouldError {
		return nil, models.NewDocumentError(models.ErrEmbeddingFailed, fmt.Sprintf("mock error: %s", p.errorMsg))
	}

	if !p.initialized {
		return nil, models.NewDocumentError(models.ErrEmbeddingFailed, "provider not initialized")
	}

	if text == "" {
		return nil, models.NewDocumentError(models.ErrEmbeddingFailed, "text cannot be empty")
	}

	// Generate a simple mock embedding based on text content
	embedding := make([]float32, p.dimension)

	// Create a deterministic embedding based on the text
	hash := simpleHash(text)
	for i := range embedding {
		// Create deterministic but varied values
		embedding[i] = float32((hash*uint32(i+1))%1000) / 1000.0 - 0.5
	}

	// Normalize the embedding
	magnitude := float32(0)
	for _, val := range embedding {
		magnitude += val * val
	}
	if magnitude > 0 {
		magnitude = float32(1.0 / (magnitude + 0.001)) // Add small value to avoid division by zero
		for i := range embedding {
			embedding[i] *= magnitude
		}
	}

	if p.metrics != nil {
		p.metrics.IncrementCounter("embedding.provider.mock.generate.success", nil)
	}

	return embedding, nil
}

// GenerateEmbeddings generates mock embeddings for multiple texts
func (p *MockEmbeddingProvider) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		embedding, err := p.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

// GetModelName returns the model name
func (p *MockEmbeddingProvider) GetModelName() string {
	return p.modelName
}

// GetDimension returns the embedding dimension
func (p *MockEmbeddingProvider) GetDimension() int {
	return p.dimension
}

// HealthCheck performs a health check on the mock provider
func (p *MockEmbeddingProvider) HealthCheck(ctx context.Context) error {
	if p.shouldError {
		return models.NewDocumentError(models.ErrEmbeddingFailed, fmt.Sprintf("mock error: %s", p.errorMsg))
	}
	return nil
}

// Close closes the mock provider
func (p *MockEmbeddingProvider) Close() error {
	if p.shouldError {
		return models.NewDocumentError(models.ErrEmbeddingFailed, fmt.Sprintf("mock error: %s", p.errorMsg))
	}
	p.initialized = false
	return nil
}

// simpleHash provides a simple hash function for deterministic mock embeddings
func simpleHash(s string) uint32 {
	h := uint32(2166136261) // FNV offset basis
	for _, c := range s {
		h ^= uint32(c)
		h *= 16777619 // FNV prime
	}
	return h
}