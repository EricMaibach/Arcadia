package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

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

// OllamaEmbeddingModel implements EmbeddingModelInterface using Ollama API
type OllamaEmbeddingModel struct {
	baseURL     string
	modelName   string
	dimension   int
	timeout     time.Duration
	client      *http.Client
	initialized bool
	mutex       sync.RWMutex
}

// NewOllamaEmbeddingModel creates a new Ollama embedding model
func NewOllamaEmbeddingModel(baseURL, modelName string) *OllamaEmbeddingModel {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if modelName == "" {
		modelName = "embeddinggemma"
	}
	
	return &OllamaEmbeddingModel{
		baseURL:   baseURL,
		modelName: modelName,
		dimension: 768, // EmbeddingGemma default dimensions
		timeout:   30 * time.Second,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewONNXEmbeddingModel creates a new ONNX embedding model (deprecated - use NewOllamaEmbeddingModel)
func NewONNXEmbeddingModel(modelPath string) *OllamaEmbeddingModel {
	// For backward compatibility, create an Ollama model instead
	return NewOllamaEmbeddingModel("", "")
}

// Initialize initializes the Ollama model
func (m *OllamaEmbeddingModel) Initialize() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.initialized {
		return nil
	}

	// Test connection to Ollama API
	testURL := m.baseURL + "/api/tags"
	resp, err := m.client.Get(testURL)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama at %s: %v", m.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}
	
	m.initialized = true
	return nil
}

// Close closes the Ollama model
func (m *OllamaEmbeddingModel) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.initialized {
		return nil
	}

	// Clean up HTTP client resources if needed
	// For now, just mark as not initialized
	m.initialized = false
	return nil
}

// GenerateEmbedding generates an embedding for the given text using Ollama API
func (m *OllamaEmbeddingModel) GenerateEmbedding(text string) ([]float32, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if !m.initialized {
		return nil, fmt.Errorf("model not initialized")
	}

	// Prepare request
	request := OllamaEmbeddingRequest{
		Model:  m.modelName,
		Prompt: text,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Make API call
	apiURL := m.baseURL + "/api/embeddings"
	resp, err := m.client.Post(apiURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}

	// Parse response
	var response OllamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	// Convert float64 to float32
	embedding := make([]float32, len(response.Embedding))
	for i, v := range response.Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// GetDimension returns the embedding dimension
func (m *OllamaEmbeddingModel) GetDimension() int {
	return m.dimension
}


// MockEmbeddingModel is a mock implementation for testing
type MockEmbeddingModel struct {
	dimension   int
	initialized bool
	shouldError bool
	errorMsg    string
}

// NewMockEmbeddingModel creates a new mock embedding model
func NewMockEmbeddingModel(dimension int) *MockEmbeddingModel {
	return &MockEmbeddingModel{
		dimension: dimension,
	}
}

// SetError sets the mock to return errors
func (m *MockEmbeddingModel) SetError(shouldError bool, errorMsg string) {
	m.shouldError = shouldError
	m.errorMsg = errorMsg
}

// Initialize initializes the mock model
func (m *MockEmbeddingModel) Initialize() error {
	if m.shouldError {
		return fmt.Errorf("mock error: %s", m.errorMsg)
	}
	m.initialized = true
	return nil
}

// Close closes the mock model
func (m *MockEmbeddingModel) Close() error {
	if m.shouldError {
		return fmt.Errorf("mock error: %s", m.errorMsg)
	}
	m.initialized = false
	return nil
}

// GenerateEmbedding generates a mock embedding
func (m *MockEmbeddingModel) GenerateEmbedding(text string) ([]float32, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error: %s", m.errorMsg)
	}
	
	if !m.initialized {
		return nil, fmt.Errorf("model not initialized")
	}
	
	// Generate a simple mock embedding
	embedding := make([]float32, m.dimension)
	for i := range embedding {
		embedding[i] = float32(i) / float32(m.dimension)
	}
	
	return embedding, nil
}

// GetDimension returns the embedding dimension
func (m *MockEmbeddingModel) GetDimension() int {
	return m.dimension
}