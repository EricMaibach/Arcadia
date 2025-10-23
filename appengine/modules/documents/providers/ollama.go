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
)

// OllamaProvider is a generic provider for Ollama API
// It knows about Ollama endpoints but not about specific models or use cases
type OllamaProvider struct {
	baseURL     string
	timeout     time.Duration
	client      *http.Client
	initialized bool
	logger      interfaces.Logger
	metrics     interfaces.MetricsCollector
	mutex       sync.RWMutex
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(baseURL string, timeout time.Duration) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &OllamaProvider{
		baseURL: baseURL,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// WithLogger adds logging to the Ollama provider
func (p *OllamaProvider) WithLogger(logger interfaces.Logger) *OllamaProvider {
	p.logger = logger
	return p
}

// WithMetrics adds metrics collection to the Ollama provider
func (p *OllamaProvider) WithMetrics(metrics interfaces.MetricsCollector) *OllamaProvider {
	p.metrics = metrics
	return p
}

// Initialize initializes the Ollama provider
func (p *OllamaProvider) Initialize(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.initialized {
		return nil
	}

	// Test connection to Ollama API
	testURL := p.baseURL + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create test request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama at %s: %w", p.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}

	p.initialized = true

	if p.logger != nil {
		p.logger.Info(ctx, "Successfully initialized Ollama provider",
			"base_url", p.baseURL)
	}

	return nil
}

// CallEmbeddings calls the /api/embeddings endpoint
// Caller specifies which embedding model to use
func (p *OllamaProvider) CallEmbeddings(ctx context.Context, model string, prompt string) ([]float64, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if !p.initialized {
		return nil, fmt.Errorf("provider not initialized")
	}

	if prompt == "" {
		return nil, fmt.Errorf("prompt cannot be empty")
	}

	startTime := time.Now()
	defer func() {
		if p.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			p.metrics.RecordTimer("ollama.embeddings.duration", duration, map[string]string{
				"model": model,
			})
		}
	}()

	// Prepare request
	request := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		if p.metrics != nil {
			p.metrics.IncrementCounter("ollama.embeddings.marshal.error", nil)
		}
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make API call to /api/embeddings
	apiURL := p.baseURL + "/api/embeddings"

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		if p.metrics != nil {
			p.metrics.IncrementCounter("ollama.embeddings.api.error", nil)
		}
		return nil, fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if p.metrics != nil {
			p.metrics.IncrementCounter("ollama.embeddings.api.error", nil)
		}
		return nil, fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}

	// Parse response
	var response struct {
		Embedding          []float64 `json:"embedding"`
		PromptEvalCount    int       `json:"prompt_eval_count"`
		PromptEvalDuration int64     `json:"prompt_eval_duration"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		if p.metrics != nil {
			p.metrics.IncrementCounter("ollama.embeddings.decode.error", nil)
		}
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if p.metrics != nil {
		p.metrics.IncrementCounter("ollama.embeddings.success", map[string]string{
			"model": model,
		})
	}

	return response.Embedding, nil
}

// CallGenerate calls the /api/generate endpoint
// Caller specifies which generation model to use and any options
func (p *OllamaProvider) CallGenerate(ctx context.Context, model string, prompt string, options map[string]interface{}) (string, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if !p.initialized {
		return "", fmt.Errorf("provider not initialized")
	}

	if prompt == "" {
		return "", fmt.Errorf("prompt cannot be empty")
	}

	startTime := time.Now()
	defer func() {
		if p.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			p.metrics.RecordTimer("ollama.generate.duration", duration, map[string]string{
				"model": model,
			})
		}
	}()

	// Prepare request
	request := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	}

	// Merge caller-provided options
	for k, v := range options {
		request[k] = v
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		if p.metrics != nil {
			p.metrics.IncrementCounter("ollama.generate.marshal.error", nil)
		}
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make API call to /api/generate
	apiURL := p.baseURL + "/api/generate"

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		if p.metrics != nil {
			p.metrics.IncrementCounter("ollama.generate.api.error", nil)
		}
		return "", fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if p.metrics != nil {
			p.metrics.IncrementCounter("ollama.generate.api.error", nil)
		}
		return "", fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}

	// Parse response
	var response struct {
		Response string `json:"response"`
		Done     bool   `json:"done"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		if p.metrics != nil {
			p.metrics.IncrementCounter("ollama.generate.decode.error", nil)
		}
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if p.metrics != nil {
		p.metrics.IncrementCounter("ollama.generate.success", map[string]string{
			"model": model,
		})
	}

	return response.Response, nil
}

// HealthCheck performs a health check on the provider
func (p *OllamaProvider) HealthCheck(ctx context.Context) error {
	// Just check if we can connect
	return p.Initialize(ctx)
}

// GetBaseURL returns the base URL
func (p *OllamaProvider) GetBaseURL() string {
	return p.baseURL
}

// Close closes the Ollama provider
func (p *OllamaProvider) Close() error {
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