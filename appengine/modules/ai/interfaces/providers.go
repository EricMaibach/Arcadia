package interfaces

import (
	"context"

	"arcadia/modules/ai/models"
)

// AIProvider interface defines the contract that all AI providers must implement
type AIProvider interface {
	// Provider identification
	Name() string
	Type() string
	Version() string

	// Core messaging capabilities
	SendMessage(ctx context.Context, message string) (string, error)
	SendMessageWithContext(ctx context.Context, message string, contextID string) (string, error)
	SendMessageWithOptions(ctx context.Context, request *models.MessageRequest) (*models.MessageResponse, error)

	// Advanced messaging
	GenerateCompletion(ctx context.Context, request *models.CompletionRequest) (*models.CompletionResponse, error)
	StreamCompletion(ctx context.Context, request *models.CompletionRequest) (<-chan models.CompletionChunk, error)

	// Context management
	CreateContext(ctx context.Context, contextID string) error
	GetContext(ctx context.Context, contextID string) (*models.ConversationContext, error)
	UpdateContext(ctx context.Context, contextID string, message models.Message) error
	ClearContext(ctx context.Context, contextID string) error
	GetContextStats(ctx context.Context, contextID string) (*models.ContextStats, error)

	// Tool integration
	SetTools(ctx context.Context, tools []models.Tool) error
	GetTools(ctx context.Context) ([]models.Tool, error)
	ExecuteToolCall(ctx context.Context, toolCall models.ToolCall) (*models.ToolResult, error)

	// Configuration and validation
	Configure(ctx context.Context, config map[string]interface{}) error
	Validate(ctx context.Context) error
	GetConfiguration(ctx context.Context) (map[string]interface{}, error)

	// Provider information and capabilities
	GetProviderInfo(ctx context.Context) (*models.ProviderInfo, error)
	GetCapabilities(ctx context.Context) (*models.ProviderCapabilities, error)
	EstimateTokens(ctx context.Context, text string) (int, error)

	// Health and diagnostics
	HealthCheck(ctx context.Context) (*models.HealthStatus, error)
	GetMetrics(ctx context.Context) (*models.ProviderMetrics, error)

	// Lifecycle management
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
}

// StreamingProvider interface for providers that support streaming responses
type StreamingProvider interface {
	AIProvider

	// Streaming capabilities
	SupportsStreaming() bool
	StreamMessage(ctx context.Context, request *models.MessageRequest) (<-chan models.MessageChunk, error)
	StreamCompletion(ctx context.Context, request *models.CompletionRequest) (<-chan models.CompletionChunk, error)
}

// VisionProvider interface for providers that support vision/image analysis
type VisionProvider interface {
	AIProvider

	// Vision capabilities
	SupportsVision() bool
	AnalyzeImage(ctx context.Context, imageData []byte, prompt string) (*models.VisionResponse, error)
	AnalyzeImageFromURL(ctx context.Context, imageURL string, prompt string) (*models.VisionResponse, error)
	DescribeImage(ctx context.Context, imageData []byte) (*models.ImageDescription, error)
}

// EmbeddingProvider interface for providers that support embeddings
type EmbeddingProvider interface {
	AIProvider

	// Embedding capabilities
	SupportsEmbeddings() bool
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
	GetEmbeddingDimensions(ctx context.Context) (int, error)
}

// FunctionCallingProvider interface for providers that support function calling
type FunctionCallingProvider interface {
	AIProvider

	// Function calling capabilities
	SupportsFunctionCalling() bool
	CallFunction(ctx context.Context, functionName string, arguments map[string]interface{}) (*models.FunctionResult, error)
	RegisterFunction(ctx context.Context, function *models.Function) error
	UnregisterFunction(ctx context.Context, functionName string) error
	ListFunctions(ctx context.Context) ([]*models.Function, error)
}

// ConfigurableProvider interface for providers with advanced configuration
type ConfigurableProvider interface {
	AIProvider

	// Advanced configuration
	SetTemperature(ctx context.Context, temperature float64) error
	SetMaxTokens(ctx context.Context, maxTokens int) error
	SetTopP(ctx context.Context, topP float64) error
	SetTopK(ctx context.Context, topK int) error
	SetFrequencyPenalty(ctx context.Context, penalty float64) error
	SetPresencePenalty(ctx context.Context, penalty float64) error

	// Model selection
	ListModels(ctx context.Context) ([]string, error)
	SetModel(ctx context.Context, model string) error
	GetCurrentModel(ctx context.Context) (string, error)
}

// MonitorableProvider interface for providers that support detailed monitoring
type MonitorableProvider interface {
	AIProvider

	// Monitoring capabilities
	GetUsageStats(ctx context.Context) (*models.UsageStats, error)
	GetLatencyStats(ctx context.Context) (*models.LatencyStats, error)
	GetErrorStats(ctx context.Context) (*models.ErrorStats, error)
	GetTokenUsage(ctx context.Context) (*models.TokenUsage, error)

	// Performance monitoring
	StartMonitoring(ctx context.Context) error
	StopMonitoring(ctx context.Context) error
	IsMonitoring(ctx context.Context) bool
}

// CachingProvider interface for providers that support response caching
type CachingProvider interface {
	AIProvider

	// Caching capabilities
	SupportsCaching() bool
	EnableCaching(ctx context.Context, enabled bool) error
	ClearCache(ctx context.Context) error
	GetCacheStats(ctx context.Context) (*models.CacheStats, error)
	SetCacheTTL(ctx context.Context, ttl int) error
}

// RateLimitedProvider interface for providers with rate limiting
type RateLimitedProvider interface {
	AIProvider

	// Rate limiting
	GetRateLimit(ctx context.Context) (*models.RateLimit, error)
	GetRemainingRequests(ctx context.Context) (int, error)
	WaitForRateLimit(ctx context.Context) error
}

// ProviderFactory interface for creating provider instances
type ProviderFactory interface {
	// Factory methods
	CreateProvider(ctx context.Context, providerType string, config map[string]interface{}) (AIProvider, error)
	GetSupportedProviders() []string
	ValidateProviderConfig(providerType string, config map[string]interface{}) error

	// Provider registration
	RegisterProviderType(providerType string, constructor ProviderConstructor) error
	UnregisterProviderType(providerType string) error
}

// ProviderConstructor function type for creating provider instances
type ProviderConstructor func(ctx context.Context, config map[string]interface{}) (AIProvider, error)

// ProviderRegistry interface for managing provider instances
type ProviderRegistry interface {
	// Provider management
	RegisterProvider(ctx context.Context, name string, provider AIProvider) error
	UnregisterProvider(ctx context.Context, name string) error
	GetProvider(ctx context.Context, name string) (AIProvider, error)
	ListProviders(ctx context.Context) ([]string, error)

	// Default provider management
	SetDefaultProvider(ctx context.Context, name string) error
	GetDefaultProvider(ctx context.Context) (AIProvider, error)
	GetDefaultProviderName(ctx context.Context) (string, error)

	// Provider discovery
	FindProvidersByCapability(ctx context.Context, capability string) ([]AIProvider, error)
	FindProvidersByType(ctx context.Context, providerType string) ([]AIProvider, error)

	// Health and status
	CheckAllProviders(ctx context.Context) (map[string]*models.HealthStatus, error)
	GetProviderMetrics(ctx context.Context) (map[string]*models.ProviderMetrics, error)
}

// ProviderLoadBalancer interface for load balancing across providers
type ProviderLoadBalancer interface {
	// Load balancing
	SelectProvider(ctx context.Context, request *models.MessageRequest) (AIProvider, error)
	GetProviderForModel(ctx context.Context, model string) (AIProvider, error)

	// Load balancing strategies
	SetStrategy(ctx context.Context, strategy string) error
	GetStrategy(ctx context.Context) (string, error)
	GetSupportedStrategies() []string

	// Provider weighting
	SetProviderWeight(ctx context.Context, providerName string, weight float64) error
	GetProviderWeight(ctx context.Context, providerName string) (float64, error)

	// Health-based routing
	EnableHealthBasedRouting(ctx context.Context, enabled bool) error
	IsHealthBasedRoutingEnabled(ctx context.Context) bool
}

// ProviderCircuitBreaker interface for circuit breaker pattern
type ProviderCircuitBreaker interface {
	// Circuit breaker management
	OpenCircuit(ctx context.Context, providerName string) error
	CloseCircuit(ctx context.Context, providerName string) error
	IsCircuitOpen(ctx context.Context, providerName string) (bool, error)

	// Circuit breaker configuration
	SetFailureThreshold(ctx context.Context, providerName string, threshold int) error
	SetRecoveryTimeout(ctx context.Context, providerName string, timeout int) error
	GetCircuitState(ctx context.Context, providerName string) (*models.CircuitState, error)

	// Circuit breaker monitoring
	GetCircuitStats(ctx context.Context, providerName string) (*models.CircuitStats, error)
	OnCircuitStateChange(ctx context.Context, handler CircuitStateChangeHandler) error
}

// CircuitStateChangeHandler function type for circuit state changes
type CircuitStateChangeHandler func(ctx context.Context, providerName string, oldState, newState string) error

// ProviderFallback interface for fallback provider management
type ProviderFallback interface {
	// Fallback configuration
	SetFallbackProvider(ctx context.Context, primaryProvider, fallbackProvider string) error
	GetFallbackProvider(ctx context.Context, primaryProvider string) (string, error)
	RemoveFallbackProvider(ctx context.Context, primaryProvider string) error

	// Fallback execution
	ExecuteWithFallback(ctx context.Context, primaryProvider string, request *models.MessageRequest) (*models.MessageResponse, error)

	// Fallback monitoring
	GetFallbackStats(ctx context.Context) (*models.FallbackStats, error)
	OnFallbackTriggered(ctx context.Context, handler FallbackHandler) error
}

// FallbackHandler function type for fallback events
type FallbackHandler func(ctx context.Context, primaryProvider, fallbackProvider string, error error) error

// ProviderPoolManager interface for managing pools of providers
type ProviderPoolManager interface {
	// Pool management
	CreatePool(ctx context.Context, poolName string, providers []string) error
	DeletePool(ctx context.Context, poolName string) error
	GetPool(ctx context.Context, poolName string) ([]string, error)
	ListPools(ctx context.Context) ([]string, error)

	// Pool operations
	AddProviderToPool(ctx context.Context, poolName, providerName string) error
	RemoveProviderFromPool(ctx context.Context, poolName, providerName string) error
	GetProviderFromPool(ctx context.Context, poolName string) (AIProvider, error)

	// Pool monitoring
	GetPoolStats(ctx context.Context, poolName string) (*models.PoolStats, error)
	GetPoolHealth(ctx context.Context, poolName string) (*models.PoolHealth, error)
}

// AsyncProvider interface for providers that support async operations
type AsyncProvider interface {
	AIProvider

	// Async capabilities
	SupportsAsync() bool
	SendMessageAsync(ctx context.Context, request *models.MessageRequest) (*models.AsyncResult, error)
	GetAsyncResult(ctx context.Context, resultID string) (*models.AsyncResult, error)
	CancelAsyncOperation(ctx context.Context, operationID string) error

	// Async monitoring
	ListAsyncOperations(ctx context.Context) ([]*models.AsyncOperation, error)
	GetAsyncOperationStatus(ctx context.Context, operationID string) (*models.AsyncStatus, error)
}

// BatchProvider interface for providers that support batch operations
type BatchProvider interface {
	AIProvider

	// Batch capabilities
	SupportsBatch() bool
	SendBatchMessages(ctx context.Context, requests []*models.MessageRequest) ([]*models.MessageResponse, error)
	GetBatchStatus(ctx context.Context, batchID string) (*models.BatchStatus, error)

	// Batch configuration
	GetMaxBatchSize(ctx context.Context) (int, error)
	SetBatchTimeout(ctx context.Context, timeout int) error
}

// MultiModalProvider interface for providers that support multiple modalities
type MultiModalProvider interface {
	AIProvider
	VisionProvider
	EmbeddingProvider

	// Multi-modal capabilities
	GetSupportedModalities() []string
	ProcessMultiModalInput(ctx context.Context, input *models.MultiModalInput) (*models.MultiModalResponse, error)
}

// RetryableProvider interface for providers with retry capabilities
type RetryableProvider interface {
	AIProvider

	// Retry configuration
	SetRetryPolicy(ctx context.Context, policy *models.RetryPolicy) error
	GetRetryPolicy(ctx context.Context) (*models.RetryPolicy, error)
	EnableRetries(ctx context.Context, enabled bool) error

	// Retry monitoring
	GetRetryStats(ctx context.Context) (*models.RetryStats, error)
}

// SecureProvider interface for providers with security features
type SecureProvider interface {
	AIProvider

	// Security features
	EncryptRequest(ctx context.Context, request *models.MessageRequest) (*models.EncryptedRequest, error)
	DecryptResponse(ctx context.Context, response *models.EncryptedResponse) (*models.MessageResponse, error)
	ValidateAPIKey(ctx context.Context, apiKey string) (bool, error)

	// Audit logging
	EnableAuditLogging(ctx context.Context, enabled bool) error
	GetAuditLogs(ctx context.Context) ([]*models.AuditLog, error)
}