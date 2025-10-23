package core

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
	"arcadia/modules/documents/providers"
)

// EmbeddingEngine handles embedding generation and similarity calculations
type EmbeddingEngine struct {
	ollama    *providers.OllamaProvider  // Generic Ollama provider
	modelName string                      // Embedding model name
	dimension int                         // Expected embedding dimension
	cache     interfaces.CacheService
	logger    interfaces.Logger
	config    EmbeddingConfig
	metrics   interfaces.MetricsCollector

	// Caching
	cacheEnabled bool
	cacheTTL     int

	mutex sync.RWMutex
}

// EmbeddingConfig contains configuration for the embedding engine
type EmbeddingConfig struct {
	CacheEnabled    bool    `json:"cache_enabled"`
	CacheTTL        int     `json:"cache_ttl_seconds"`
	BatchSize       int     `json:"batch_size"`
	MaxRetries      int     `json:"max_retries"`
	RetryDelay      int     `json:"retry_delay_seconds"`
	TimeoutSeconds  int     `json:"timeout_seconds"`
	ModelName       string  `json:"model_name"`
	Dimension       int     `json:"dimension"`
	SimilarityThreshold float32 `json:"similarity_threshold"`
}

// NewEmbeddingEngine creates a new embedding engine
func NewEmbeddingEngine(ollama *providers.OllamaProvider, modelName string, dimension int, config EmbeddingConfig) *EmbeddingEngine {
	if modelName == "" {
		modelName = "embeddinggemma"
	}
	if dimension == 0 {
		dimension = 768
	}

	return &EmbeddingEngine{
		ollama:       ollama,
		modelName:    modelName,
		dimension:    dimension,
		config:       config,
		cacheEnabled: config.CacheEnabled,
		cacheTTL:     config.CacheTTL,
	}
}

// WithCache adds cache support to the embedding engine
func (ee *EmbeddingEngine) WithCache(cache interfaces.CacheService) *EmbeddingEngine {
	ee.cache = cache
	return ee
}

// WithLogger adds logging support to the embedding engine
func (ee *EmbeddingEngine) WithLogger(logger interfaces.Logger) *EmbeddingEngine {
	ee.logger = logger
	return ee
}

// WithMetrics adds metrics collection support
func (ee *EmbeddingEngine) WithMetrics(metrics interfaces.MetricsCollector) *EmbeddingEngine {
	ee.metrics = metrics
	return ee
}

// GenerateEmbedding generates an embedding for the given text
func (ee *EmbeddingEngine) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, models.NewDocumentError(models.ErrEmbeddingFailed, "text cannot be empty")
	}

	startTime := time.Now()
	defer func() {
		if ee.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			ee.metrics.RecordTimer("embedding.engine.generate.duration", duration, nil)
		}
	}()

	// Check cache first
	if ee.cacheEnabled && ee.cache != nil {
		if cached, err := ee.getCachedEmbedding(ctx, text); err == nil && cached != nil {
			if ee.metrics != nil {
				ee.metrics.IncrementCounter("embedding.cache.hit", nil)
			}
			if ee.logger != nil {
				ee.logger.Debug(ctx, "Cache hit for embedding", "text_length", len(text))
			}
			return cached, nil
		}
	}

	// Call Ollama provider with our model
	embedding64, err := ee.ollama.CallEmbeddings(ctx, ee.modelName, text)
	if err != nil {
		if ee.metrics != nil {
			ee.metrics.IncrementCounter("embedding.generate.error", nil)
		}
		return nil, models.NewDocumentErrorWithCause(models.ErrEmbeddingFailed, "failed to generate embedding", err)
	}

	// Validate dimension
	if len(embedding64) != ee.dimension {
		if ee.logger != nil {
			ee.logger.Warn(ctx, "Embedding dimension mismatch",
				"expected", ee.dimension,
				"got", len(embedding64))
		}
		// Update dimension if it's different
		ee.dimension = len(embedding64)
	}

	// Convert float64 to float32 (embedding-specific logic)
	embedding := make([]float32, len(embedding64))
	for i, v := range embedding64 {
		embedding[i] = float32(v)
	}

	// Validate embedding
	if err := ee.ValidateEmbedding(embedding); err != nil {
		return nil, err
	}

	// Cache the result
	if ee.cacheEnabled && ee.cache != nil {
		ee.cacheEmbedding(ctx, text, embedding)
	}

	if ee.metrics != nil {
		ee.metrics.IncrementCounter("embedding.generate.success", nil)
	}

	if ee.logger != nil {
		ee.logger.Debug(ctx, "Generated embedding",
			"text_length", len(text),
			"dimension", len(embedding),
			"model", ee.modelName)
	}

	return embedding, nil
}

// GenerateEmbeddings generates embeddings for multiple texts in batch
func (ee *EmbeddingEngine) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	embeddings := make([][]float32, len(texts))
	var wg sync.WaitGroup
	errChan := make(chan error, len(texts))

	// Process in batches to control concurrency
	batchSize := ee.config.BatchSize
	if batchSize <= 0 {
		batchSize = 10 // Default batch size
	}

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		for j := i; j < end; j++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				embedding, err := ee.GenerateEmbedding(ctx, texts[index])
				if err != nil {
					errChan <- fmt.Errorf("failed to generate embedding for text %d: %w", index, err)
					return
				}
				embeddings[index] = embedding
			}(j)
		}

		wg.Wait()
	}

	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		return nil, <-errChan
	}

	return embeddings, nil
}

// CalculateSimilarity calculates cosine similarity between two vectors
func (ee *EmbeddingEngine) CalculateSimilarity(a, b []float32) (float32, error) {
	if len(a) != len(b) {
		return 0, models.NewDocumentError(models.ErrEmbeddingFailed, "vectors must have the same dimension")
	}

	if len(a) == 0 {
		return 0, models.NewDocumentError(models.ErrEmbeddingFailed, "vectors cannot be empty")
	}

	return CosineSimilarity(a, b), nil
}

// FindMostSimilar finds the most similar vectors to the query vector
func (ee *EmbeddingEngine) FindMostSimilar(queryVector []float32, candidates [][]float32, topK int) ([]SimilarityResult, error) {
	if len(candidates) == 0 {
		return []SimilarityResult{}, nil
	}

	if topK <= 0 {
		topK = len(candidates)
	}

	results := make([]SimilarityResult, 0, len(candidates))

	for i, candidate := range candidates {
		similarity, err := ee.CalculateSimilarity(queryVector, candidate)
		if err != nil {
			continue // Skip invalid vectors
		}

		results = append(results, SimilarityResult{
			Index:      i,
			Similarity: similarity,
			Vector:     candidate,
		})
	}

	// Sort by similarity (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Return top K results
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// ValidateEmbedding validates that an embedding vector is valid
func (ee *EmbeddingEngine) ValidateEmbedding(embedding []float32) error {
	if len(embedding) == 0 {
		return models.NewDocumentError(models.ErrEmbeddingFailed, "embedding cannot be empty")
	}

	// Check for NaN or infinite values
	for i, val := range embedding {
		if math.IsNaN(float64(val)) {
			return models.NewDocumentError(models.ErrEmbeddingFailed, fmt.Sprintf("embedding contains NaN at index %d", i))
		}
		if math.IsInf(float64(val), 0) {
			return models.NewDocumentError(models.ErrEmbeddingFailed, fmt.Sprintf("embedding contains infinity at index %d", i))
		}
	}

	// Check dimension
	if ee.dimension > 0 && len(embedding) != ee.dimension {
		return models.NewDocumentError(models.ErrEmbeddingFailed,
			fmt.Sprintf("embedding dimension mismatch: expected %d, got %d", ee.dimension, len(embedding)))
	}

	return nil
}

// NormalizeEmbedding normalizes an embedding vector to unit length
func (ee *EmbeddingEngine) NormalizeEmbedding(embedding []float32) []float32 {
	magnitude := ee.calculateMagnitude(embedding)
	if magnitude == 0 {
		return embedding
	}

	normalized := make([]float32, len(embedding))
	for i, val := range embedding {
		normalized[i] = val / magnitude
	}

	return normalized
}

// calculateMagnitude calculates the magnitude (L2 norm) of a vector
func (ee *EmbeddingEngine) calculateMagnitude(vector []float32) float32 {
	var sum float32
	for _, val := range vector {
		sum += val * val
	}
	return float32(math.Sqrt(float64(sum)))
}

// getCachedEmbedding retrieves an embedding from cache
func (ee *EmbeddingEngine) getCachedEmbedding(ctx context.Context, text string) ([]float32, error) {
	cacheKey := ee.generateCacheKey(text)

	cached, err := ee.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, err
	}

	if embedding, ok := cached.([]float32); ok {
		return embedding, nil
	}

	return nil, fmt.Errorf("cached value is not a valid embedding")
}

// cacheEmbedding stores an embedding in cache
func (ee *EmbeddingEngine) cacheEmbedding(ctx context.Context, text string, embedding []float32) {
	cacheKey := ee.generateCacheKey(text)

	if err := ee.cache.Set(ctx, cacheKey, embedding, ee.cacheTTL); err != nil {
		if ee.logger != nil {
			ee.logger.Warn(ctx, "Failed to cache embedding", "error", err, "key", cacheKey)
		}
	}
}

// generateCacheKey generates a cache key for the given text
func (ee *EmbeddingEngine) generateCacheKey(text string) string {
	// Use a hash of the text to create a consistent cache key
	return fmt.Sprintf("embedding:%s:%x", ee.config.ModelName, simpleHash(text))
}

// GetModelName returns the model name being used
func (ee *EmbeddingEngine) GetModelName() string {
	return ee.modelName
}

// GetDimension returns the expected embedding dimension
func (ee *EmbeddingEngine) GetDimension() int {
	return ee.dimension
}

// GetProviderInfo returns information about the embedding provider
func (ee *EmbeddingEngine) GetProviderInfo() ProviderInfo {
	return ProviderInfo{
		Name:      ee.modelName,
		Dimension: ee.dimension,
		Config:    ee.config,
	}
}

// HealthCheck performs a health check on the embedding engine
func (ee *EmbeddingEngine) HealthCheck(ctx context.Context) error {
	// Test with a simple text
	testText := "health check test"
	_, err := ee.GenerateEmbedding(ctx, testText)
	return err
}

// SimilarityResult represents a similarity search result
type SimilarityResult struct {
	Index      int       `json:"index"`
	Similarity float32   `json:"similarity"`
	Vector     []float32 `json:"vector,omitempty"`
}

// ProviderInfo contains information about the embedding provider
type ProviderInfo struct {
	Name      string          `json:"name"`
	Dimension int             `json:"dimension"`
	Config    EmbeddingConfig `json:"config"`
}

// CosineSimilarity calculates cosine similarity between two vectors
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// EuclideanDistance calculates Euclidean distance between two vectors
func EuclideanDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return float32(math.Inf(1))
	}

	var sum float32
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return float32(math.Sqrt(float64(sum)))
}

// DotProduct calculates dot product between two vectors
func DotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var product float32
	for i := range a {
		product += a[i] * b[i]
	}

	return product
}

// simpleHash provides a simple hash function for cache keys
func simpleHash(s string) uint32 {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}

// DefaultEmbeddingConfig returns a default embedding configuration
func DefaultEmbeddingConfig() EmbeddingConfig {
	return EmbeddingConfig{
		CacheEnabled:    true,
		CacheTTL:        3600, // 1 hour
		BatchSize:       10,
		MaxRetries:      3,
		RetryDelay:      5,
		TimeoutSeconds:  30,
		ModelName:       "default",
		Dimension:       0, // Will be determined by provider
		SimilarityThreshold: 0.7,
	}
}

// EmbeddingBatchProcessor handles batch processing of embeddings
type EmbeddingBatchProcessor struct {
	engine    *EmbeddingEngine
	batchSize int
	workers   int
}

// NewEmbeddingBatchProcessor creates a new batch processor
func NewEmbeddingBatchProcessor(engine *EmbeddingEngine, batchSize, workers int) *EmbeddingBatchProcessor {
	return &EmbeddingBatchProcessor{
		engine:    engine,
		batchSize: batchSize,
		workers:   workers,
	}
}

// ProcessBatch processes a batch of texts and returns their embeddings
func (ebp *EmbeddingBatchProcessor) ProcessBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	embeddings := make([][]float32, len(texts))
	jobs := make(chan int, len(texts))
	results := make(chan batchResult, len(texts))

	// Start workers
	var wg sync.WaitGroup
	for w := 0; w < ebp.workers; w++ {
		wg.Add(1)
		go ebp.worker(ctx, &wg, texts, embeddings, jobs, results)
	}

	// Send jobs
	for i := range texts {
		jobs <- i
	}
	close(jobs)

	// Wait for completion
	wg.Wait()
	close(results)

	// Check for errors
	var lastError error
	for result := range results {
		if result.err != nil {
			lastError = result.err
		}
	}

	if lastError != nil {
		return nil, lastError
	}

	return embeddings, nil
}

// batchResult represents the result of processing a single item in a batch
type batchResult struct {
	index int
	err   error
}

// worker processes embedding jobs
func (ebp *EmbeddingBatchProcessor) worker(ctx context.Context, wg *sync.WaitGroup, texts []string, embeddings [][]float32, jobs <-chan int, results chan<- batchResult) {
	defer wg.Done()

	for index := range jobs {
		embedding, err := ebp.engine.GenerateEmbedding(ctx, texts[index])
		if err != nil {
			results <- batchResult{index: index, err: err}
			continue
		}

		embeddings[index] = embedding
		results <- batchResult{index: index, err: nil}
	}
}