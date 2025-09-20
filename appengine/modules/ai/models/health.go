package models

import "time"

// HealthStatus represents the health status of a component
type HealthStatus struct {
	Status       string                   `json:"status"`
	Component    string                   `json:"component"`
	Message      string                   `json:"message,omitempty"`
	Timestamp    time.Time                `json:"timestamp"`
	Duration     float64                  `json:"duration_ms"`
	Checks       map[string]interface{}   `json:"checks,omitempty"`
	Dependencies map[string]*HealthStatus `json:"dependencies,omitempty"`
	Metadata     map[string]interface{}   `json:"metadata,omitempty"`
	Version      string                   `json:"version,omitempty"`
	Uptime       float64                  `json:"uptime_seconds,omitempty"`
	ErrorCount   int                      `json:"error_count,omitempty"`
	WarningCount int                      `json:"warning_count,omitempty"`
}

// ModuleMetrics represents metrics for the AI module
type ModuleMetrics struct {
	ModuleName         string                      `json:"module_name"`
	Version            string                      `json:"version"`
	Timestamp          time.Time                   `json:"timestamp"`
	Uptime             float64                     `json:"uptime_seconds"`
	TotalRequests      int64                       `json:"total_requests"`
	SuccessfulRequests int64                       `json:"successful_requests"`
	FailedRequests     int64                       `json:"failed_requests"`
	AverageLatency     float64                     `json:"average_latency_ms"`
	P95Latency         float64                     `json:"p95_latency_ms"`
	P99Latency         float64                     `json:"p99_latency_ms"`
	ErrorRate          float64                     `json:"error_rate"`
	ThroughputRPS      float64                     `json:"throughput_rps"`
	ActiveContexts     int                         `json:"active_contexts"`
	TotalContexts      int64                       `json:"total_contexts"`
	ToolExecutions     int64                       `json:"tool_executions"`
	ProviderMetrics    map[string]*ProviderMetrics `json:"provider_metrics,omitempty"`
	ToolMetrics        map[string]*ToolMetrics     `json:"tool_metrics,omitempty"`
	CustomMetrics      map[string]interface{}      `json:"custom_metrics,omitempty"`
	MemoryUsage        *MemoryMetrics              `json:"memory_usage,omitempty"`
	ResourceUsage      *ResourceMetrics            `json:"resource_usage,omitempty"`
}

// ProviderMetrics represents metrics for an AI provider
type ProviderMetrics struct {
	ProviderName       string                   `json:"provider_name"`
	Status             string                   `json:"status"`
	TotalRequests      int64                    `json:"total_requests"`
	SuccessfulRequests int64                    `json:"successful_requests"`
	FailedRequests     int64                    `json:"failed_requests"`
	AverageLatency     float64                  `json:"average_latency_ms"`
	P95Latency         float64                  `json:"p95_latency_ms"`
	P99Latency         float64                  `json:"p99_latency_ms"`
	ErrorRate          float64                  `json:"error_rate"`
	ThroughputRPS      float64                  `json:"throughput_rps"`
	TokensUsed         int64                    `json:"tokens_used"`
	TokensRemaining    int64                    `json:"tokens_remaining,omitempty"`
	CostToDate         float64                  `json:"cost_to_date,omitempty"`
	RateLimitHits      int64                    `json:"rate_limit_hits"`
	LastRequest        *time.Time               `json:"last_request,omitempty"`
	Uptime             float64                  `json:"uptime_seconds"`
	Errors             map[string]int64         `json:"errors,omitempty"`
	ModelMetrics       map[string]*ModelMetrics `json:"model_metrics,omitempty"`
	Metadata           map[string]interface{}   `json:"metadata,omitempty"`
}

// ToolMetrics represents metrics for a specific tool
type ToolMetrics struct {
	ToolName        string                 `json:"tool_name"`
	ExecutionCount  int64                  `json:"execution_count"`
	SuccessCount    int64                  `json:"success_count"`
	FailureCount    int64                  `json:"failure_count"`
	AverageLatency  float64                `json:"average_latency_ms"`
	P95Latency      float64                `json:"p95_latency_ms"`
	P99Latency      float64                `json:"p99_latency_ms"`
	ErrorRate       float64                `json:"error_rate"`
	ThroughputRPS   float64                `json:"throughput_rps"`
	LastExecution   *time.Time             `json:"last_execution,omitempty"`
	Errors          map[string]int64       `json:"errors,omitempty"`
	InputSizeStats  *SizeStats             `json:"input_size_stats,omitempty"`
	OutputSizeStats *SizeStats             `json:"output_size_stats,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ModelMetrics represents metrics for a specific model
type ModelMetrics struct {
	ModelName               string                 `json:"model_name"`
	TotalRequests           int64                  `json:"total_requests"`
	SuccessfulRequests      int64                  `json:"successful_requests"`
	FailedRequests          int64                  `json:"failed_requests"`
	AverageLatency          float64                `json:"average_latency_ms"`
	TokensUsed              int64                  `json:"tokens_used"`
	AverageTokensPerRequest float64                `json:"average_tokens_per_request"`
	CostPerRequest          float64                `json:"cost_per_request,omitempty"`
	LastUsed                *time.Time             `json:"last_used,omitempty"`
	Metadata                map[string]interface{} `json:"metadata,omitempty"`
}

// MemoryMetrics represents memory usage metrics
type MemoryMetrics struct {
	AllocatedBytes    int64   `json:"allocated_bytes"`
	UsedBytes         int64   `json:"used_bytes"`
	FreeBytes         int64   `json:"free_bytes"`
	GCPauses          int64   `json:"gc_pauses"`
	GCDuration        float64 `json:"gc_duration_ms"`
	HeapSize          int64   `json:"heap_size_bytes"`
	StackSize         int64   `json:"stack_size_bytes"`
	MemoryUtilization float64 `json:"memory_utilization_percent"`
}

// ResourceMetrics represents general resource usage metrics
type ResourceMetrics struct {
	CPUUsage        float64                `json:"cpu_usage_percent"`
	MemoryUsage     float64                `json:"memory_usage_percent"`
	DiskUsage       float64                `json:"disk_usage_percent"`
	NetworkBytesIn  int64                  `json:"network_bytes_in"`
	NetworkBytesOut int64                  `json:"network_bytes_out"`
	FileDescriptors int                    `json:"file_descriptors"`
	Goroutines      int                    `json:"goroutines"`
	Connections     int                    `json:"connections"`
	CustomResources map[string]interface{} `json:"custom_resources,omitempty"`
}

// SizeStats represents statistics about data sizes
type SizeStats struct {
	MinSize     int64   `json:"min_size"`
	MaxSize     int64   `json:"max_size"`
	AverageSize float64 `json:"average_size"`
	MedianSize  int64   `json:"median_size"`
	TotalSize   int64   `json:"total_size"`
	Count       int64   `json:"count"`
}

// ToolUsageStats represents usage statistics for tools
type ToolUsageStats struct {
	TotalTools           int                       `json:"total_tools"`
	ActiveTools          int                       `json:"active_tools"`
	TotalExecutions      int64                     `json:"total_executions"`
	SuccessfulExecutions int64                     `json:"successful_executions"`
	FailedExecutions     int64                     `json:"failed_executions"`
	AverageExecutionTime float64                   `json:"average_execution_time_ms"`
	TopTools             []*ToolUsage              `json:"top_tools,omitempty"`
	ToolsByCategory      map[string]*CategoryStats `json:"tools_by_category,omitempty"`
	RecentExecutions     []*ToolExecution          `json:"recent_executions,omitempty"`
	ErrorSummary         map[string]int64          `json:"error_summary,omitempty"`
}

// ToolUsage represents usage information for a single tool
type ToolUsage struct {
	ToolName       string     `json:"tool_name"`
	ExecutionCount int64      `json:"execution_count"`
	SuccessRate    float64    `json:"success_rate"`
	AverageRuntime float64    `json:"average_runtime_ms"`
	LastUsed       *time.Time `json:"last_used,omitempty"`
	Category       string     `json:"category,omitempty"`
	Provider       string     `json:"provider,omitempty"`
}

// CategoryStats represents statistics for a tool category
type CategoryStats struct {
	Category       string  `json:"category"`
	ToolCount      int     `json:"tool_count"`
	ExecutionCount int64   `json:"execution_count"`
	SuccessRate    float64 `json:"success_rate"`
	AverageRuntime float64 `json:"average_runtime_ms"`
}

// ToolExecution represents a tool execution record
type ToolExecution struct {
	ID         string                 `json:"id"`
	ToolName   string                 `json:"tool_name"`
	Status     string                 `json:"status"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    *time.Time             `json:"end_time,omitempty"`
	Duration   float64                `json:"duration_ms,omitempty"`
	InputSize  int64                  `json:"input_size_bytes,omitempty"`
	OutputSize int64                  `json:"output_size_bytes,omitempty"`
	Error      string                 `json:"error,omitempty"`
	ContextID  string                 `json:"context_id,omitempty"`
	UserID     string                 `json:"user_id,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ContextUsageStats represents usage statistics for conversation contexts
type ContextUsageStats struct {
	TotalContexts   int64             `json:"total_contexts"`
	ActiveContexts  int               `json:"active_contexts"`
	ExpiredContexts int64             `json:"expired_contexts"`
	AverageMessages float64           `json:"average_messages_per_context"`
	AverageLifetime float64           `json:"average_lifetime_seconds"`
	TotalMessages   int64             `json:"total_messages"`
	TotalTokens     int64             `json:"total_tokens"`
	ContextsByAge   map[string]int    `json:"contexts_by_age,omitempty"`
	ContextsBySize  map[string]int    `json:"contexts_by_size,omitempty"`
	RecentContexts  []*ContextSummary `json:"recent_contexts,omitempty"`
}

// ContextMetrics represents metrics for a specific context
type ContextMetrics struct {
	ContextID           string                 `json:"context_id"`
	MessageCount        int                    `json:"message_count"`
	TokenCount          int                    `json:"token_count"`
	CreatedAt           time.Time              `json:"created_at"`
	LastAccessed        time.Time              `json:"last_accessed"`
	Lifetime            float64                `json:"lifetime_seconds"`
	TotalInteractions   int                    `json:"total_interactions"`
	ToolCalls           int                    `json:"tool_calls"`
	Errors              int                    `json:"errors"`
	AverageResponseTime float64                `json:"average_response_time_ms"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// ContextSummary represents a summary of a conversation context
type ContextSummary struct {
	ContextID    string    `json:"context_id"`
	MessageCount int       `json:"message_count"`
	TokenCount   int       `json:"token_count"`
	CreatedAt    time.Time `json:"created_at"`
	LastAccessed time.Time `json:"last_accessed"`
	IsActive     bool      `json:"is_active"`
	Summary      string    `json:"summary,omitempty"`
}

// HealthAlert represents a health alert
type HealthAlert struct {
	ID          string                 `json:"id"`
	Component   string                 `json:"component"`
	Level       string                 `json:"level"` // "warning", "error", "critical"
	Message     string                 `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	Resolved    bool                   `json:"resolved"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
	Threshold   float64                `json:"threshold,omitempty"`
	ActualValue float64                `json:"actual_value,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// CacheStats represents cache statistics
type CacheStats struct {
	HitCount    int64   `json:"hit_count"`
	MissCount   int64   `json:"miss_count"`
	HitRate     float64 `json:"hit_rate"`
	TotalSize   int64   `json:"total_size_bytes"`
	EntryCount  int64   `json:"entry_count"`
	Evictions   int64   `json:"evictions"`
	AverageSize float64 `json:"average_entry_size_bytes"`
	MemoryUsage float64 `json:"memory_usage_percent"`
}

// CleanupResult represents the result of a cleanup operation
type CleanupResult struct {
	Operation         string                 `json:"operation"`
	StartTime         time.Time              `json:"start_time"`
	EndTime           time.Time              `json:"end_time"`
	Duration          float64                `json:"duration_ms"`
	ItemsProcessed    int64                  `json:"items_processed"`
	ItemsRemoved      int64                  `json:"items_removed"`
	BytesReclaimed    int64                  `json:"bytes_reclaimed"`
	ErrorsEncountered int                    `json:"errors_encountered"`
	Success           bool                   `json:"success"`
	Message           string                 `json:"message,omitempty"`
	Details           map[string]interface{} `json:"details,omitempty"`
}

// ModuleState represents the current state of the module
type ModuleState struct {
	Status         string                 `json:"status"`
	StartTime      time.Time              `json:"start_time"`
	LastUpdate     time.Time              `json:"last_update"`
	ActiveProvider string                 `json:"active_provider"`
	ContextCount   int                    `json:"context_count"`
	ToolCount      int                    `json:"tool_count"`
	ProviderCount  int                    `json:"provider_count"`
	ConfigVersion  string                 `json:"config_version"`
	Features       []string               `json:"features"`
	HealthScore    float64                `json:"health_score"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// Health status constants
const (
	HealthStatusHealthy   = "healthy"
	HealthStatusWarning   = "warning"
	HealthStatusUnhealthy = "unhealthy"
	HealthStatusUnknown   = "unknown"
)

// Module status constants
const (
	ModuleStatusStarting = "starting"
	ModuleStatusRunning  = "running"
	ModuleStatusStopping = "stopping"
	ModuleStatusStopped  = "stopped"
	ModuleStatusError    = "error"
)

// Alert level constants
const (
	AlertLevelInfo     = "info"
	AlertLevelWarning  = "warning"
	AlertLevelError    = "error"
	AlertLevelCritical = "critical"
)

// IsHealthy checks if the health status is healthy
func (hs *HealthStatus) IsHealthy() bool {
	return hs.Status == HealthStatusHealthy
}

// HasWarnings checks if the health status has warnings
func (hs *HealthStatus) HasWarnings() bool {
	return hs.Status == HealthStatusWarning
}

// IsUnhealthy checks if the health status is unhealthy
func (hs *HealthStatus) IsUnhealthy() bool {
	return hs.Status == HealthStatusUnhealthy
}

// AddCheck adds a health check result
func (hs *HealthStatus) AddCheck(name string, result interface{}) {
	if hs.Checks == nil {
		hs.Checks = make(map[string]interface{})
	}
	hs.Checks[name] = result
}

// AddDependency adds a dependency health status
func (hs *HealthStatus) AddDependency(name string, status *HealthStatus) {
	if hs.Dependencies == nil {
		hs.Dependencies = make(map[string]*HealthStatus)
	}
	hs.Dependencies[name] = status
}

// CalculateOverallHealth calculates overall health including dependencies
func (hs *HealthStatus) CalculateOverallHealth() string {
	if hs.Status == HealthStatusUnhealthy {
		return HealthStatusUnhealthy
	}

	hasWarnings := hs.Status == HealthStatusWarning

	for _, dep := range hs.Dependencies {
		if dep.IsUnhealthy() {
			return HealthStatusUnhealthy
		}
		if dep.HasWarnings() {
			hasWarnings = true
		}
	}

	if hasWarnings {
		return HealthStatusWarning
	}

	return HealthStatusHealthy
}

// CalculateSuccessRate calculates success rate for metrics
func (mm *ModuleMetrics) CalculateSuccessRate() {
	if mm.TotalRequests > 0 {
		mm.ErrorRate = float64(mm.FailedRequests) / float64(mm.TotalRequests) * 100
	} else {
		mm.ErrorRate = 0
	}
}

// CalculateThroughput calculates throughput in requests per second
func (mm *ModuleMetrics) CalculateThroughput() {
	if mm.Uptime > 0 {
		mm.ThroughputRPS = float64(mm.TotalRequests) / mm.Uptime
	} else {
		mm.ThroughputRPS = 0
	}
}

// UpdateMetrics updates metrics with new request data
func (mm *ModuleMetrics) UpdateMetrics(latency float64, success bool) {
	mm.TotalRequests++
	if success {
		mm.SuccessfulRequests++
	} else {
		mm.FailedRequests++
	}

	// Update average latency using incremental average
	mm.AverageLatency = mm.AverageLatency + (latency-mm.AverageLatency)/float64(mm.TotalRequests)

	mm.CalculateSuccessRate()
	mm.CalculateThroughput()
	mm.Timestamp = time.Now()
}

// IsHealthy checks if the module state is healthy
func (ms *ModuleState) IsHealthy() bool {
	return ms.Status == ModuleStatusRunning && ms.HealthScore >= 0.8
}

// UpdateHealthScore updates the health score based on various factors
func (ms *ModuleState) UpdateHealthScore(metrics *ModuleMetrics) {
	score := 1.0

	// Factor in error rate
	if metrics.ErrorRate > 10 {
		score -= 0.3
	} else if metrics.ErrorRate > 5 {
		score -= 0.1
	}

	// Factor in latency
	if metrics.AverageLatency > 5000 { // 5 seconds
		score -= 0.2
	} else if metrics.AverageLatency > 2000 { // 2 seconds
		score -= 0.1
	}

	// Factor in provider health
	healthyProviders := 0
	totalProviders := len(metrics.ProviderMetrics)
	for _, pm := range metrics.ProviderMetrics {
		if pm.Status == HealthStatusHealthy {
			healthyProviders++
		}
	}

	if totalProviders > 0 {
		providerHealthRatio := float64(healthyProviders) / float64(totalProviders)
		if providerHealthRatio < 0.5 {
			score -= 0.3
		} else if providerHealthRatio < 0.8 {
			score -= 0.1
		}
	}

	if score < 0 {
		score = 0
	}

	ms.HealthScore = score
	ms.LastUpdate = time.Now()
}
