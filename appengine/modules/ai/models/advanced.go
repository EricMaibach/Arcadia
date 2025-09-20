package models

import (
	"time"
)

// ProviderCapabilities represents the capabilities of an AI provider
type ProviderCapabilities struct {
	ProviderName            string     `json:"provider_name"`
	SupportsStreaming       bool       `json:"supports_streaming"`
	SupportsVision          bool       `json:"supports_vision"`
	SupportsEmbeddings      bool       `json:"supports_embeddings"`
	SupportsFunctionCalling bool       `json:"supports_function_calling"`
	SupportsAsync           bool       `json:"supports_async"`
	SupportsBatch           bool       `json:"supports_batch"`
	MaxTokens               int        `json:"max_tokens"`
	MaxContextLength        int        `json:"max_context_length"`
	SupportedModalities     []string   `json:"supported_modalities"`
	SupportedModels         []string   `json:"supported_models"`
	RateLimit               *RateLimit `json:"rate_limit,omitempty"`
	Features                []string   `json:"features"`
}

// VisionResponse represents a response from vision/image analysis
type VisionResponse struct {
	Description    string                 `json:"description"`
	Objects        []DetectedObject       `json:"objects,omitempty"`
	Text           string                 `json:"text,omitempty"`
	Confidence     float64                `json:"confidence"`
	ProcessingTime float64                `json:"processing_time_ms"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// DetectedObject represents an object detected in an image
type DetectedObject struct {
	Label       string                 `json:"label"`
	Confidence  float64                `json:"confidence"`
	BoundingBox *BoundingBox           `json:"bounding_box,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
}

// BoundingBox represents coordinates of a detected object
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ImageDescription represents a description of an image
type ImageDescription struct {
	Description         string                 `json:"description"`
	DetailedDescription string                 `json:"detailed_description,omitempty"`
	Tags                []string               `json:"tags,omitempty"`
	Confidence          float64                `json:"confidence"`
	Language            string                 `json:"language,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// Function represents a function that can be called by AI
type Function struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Required    []string               `json:"required,omitempty"`
	Examples    []FunctionExample      `json:"examples,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Deprecated  bool                   `json:"deprecated,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// FunctionExample represents an example of function usage
type FunctionExample struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      interface{}            `json:"output"`
}

// FunctionResult represents the result of a function call
type FunctionResult struct {
	FunctionName string                 `json:"function_name"`
	Success      bool                   `json:"success"`
	Result       interface{}            `json:"result,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Duration     float64                `json:"duration_ms"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ToolName   string                 `json:"tool_name"`
	Success    bool                   `json:"success"`
	Result     string                 `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Duration   float64                `json:"duration_ms"`
	InputSize  int64                  `json:"input_size_bytes,omitempty"`
	OutputSize int64                  `json:"output_size_bytes,omitempty"`
	Cached     bool                   `json:"cached,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// UsageStats represents usage statistics
type UsageStats struct {
	TotalRequests      int64      `json:"total_requests"`
	SuccessfulRequests int64      `json:"successful_requests"`
	FailedRequests     int64      `json:"failed_requests"`
	TotalTokens        int64      `json:"total_tokens"`
	InputTokens        int64      `json:"input_tokens"`
	OutputTokens       int64      `json:"output_tokens"`
	AverageLatency     float64    `json:"average_latency_ms"`
	P95Latency         float64    `json:"p95_latency_ms"`
	P99Latency         float64    `json:"p99_latency_ms"`
	ErrorRate          float64    `json:"error_rate"`
	FirstRequest       *time.Time `json:"first_request,omitempty"`
	LastRequest        *time.Time `json:"last_request,omitempty"`
}

// LatencyStats represents latency statistics
type LatencyStats struct {
	MinLatency     float64            `json:"min_latency_ms"`
	MaxLatency     float64            `json:"max_latency_ms"`
	AverageLatency float64            `json:"average_latency_ms"`
	MedianLatency  float64            `json:"median_latency_ms"`
	P95Latency     float64            `json:"p95_latency_ms"`
	P99Latency     float64            `json:"p99_latency_ms"`
	SampleCount    int64              `json:"sample_count"`
	Percentiles    map[string]float64 `json:"percentiles,omitempty"`
}

// ErrorStats represents error statistics
type ErrorStats struct {
	TotalErrors  int64            `json:"total_errors"`
	ErrorsByType map[string]int64 `json:"errors_by_type"`
	ErrorsByCode map[string]int64 `json:"errors_by_code"`
	RecentErrors []string         `json:"recent_errors,omitempty"`
	ErrorRate    float64          `json:"error_rate"`
	FirstError   *time.Time       `json:"first_error,omitempty"`
	LastError    *time.Time       `json:"last_error,omitempty"`
}

// TokenUsage represents token usage information
type TokenUsage struct {
	InputTokens             int64   `json:"input_tokens"`
	OutputTokens            int64   `json:"output_tokens"`
	TotalTokens             int64   `json:"total_tokens"`
	CachedTokens            int64   `json:"cached_tokens,omitempty"`
	Cost                    float64 `json:"cost,omitempty"`
	Currency                string  `json:"currency,omitempty"`
	Model                   string  `json:"model,omitempty"`
	RequestCount            int64   `json:"request_count"`
	AverageTokensPerRequest float64 `json:"average_tokens_per_request"`
}

// RateLimit represents rate limiting information
type RateLimit struct {
	RequestsPerMinute int       `json:"requests_per_minute"`
	RequestsPerHour   int       `json:"requests_per_hour"`
	RequestsPerDay    int       `json:"requests_per_day"`
	TokensPerMinute   int64     `json:"tokens_per_minute,omitempty"`
	RemainingRequests int       `json:"remaining_requests"`
	RemainingTokens   int64     `json:"remaining_tokens,omitempty"`
	ResetTime         time.Time `json:"reset_time"`
	RetryAfter        int       `json:"retry_after_seconds,omitempty"`
}

// AsyncResult represents the result of an async operation
type AsyncResult struct {
	ID        string                 `json:"id"`
	Status    string                 `json:"status"`
	Result    interface{}            `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Progress  float64                `json:"progress,omitempty"`
	StartTime time.Time              `json:"start_time"`
	EndTime   *time.Time             `json:"end_time,omitempty"`
	Duration  float64                `json:"duration_ms,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// AsyncOperation represents an async operation
type AsyncOperation struct {
	ID                  string                 `json:"id"`
	Type                string                 `json:"type"`
	Status              string                 `json:"status"`
	Progress            float64                `json:"progress"`
	Message             string                 `json:"message,omitempty"`
	StartTime           time.Time              `json:"start_time"`
	UpdateTime          time.Time              `json:"update_time"`
	EstimatedCompletion *time.Time             `json:"estimated_completion,omitempty"`
	CanCancel           bool                   `json:"can_cancel"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// AsyncStatus represents the status of an async operation
type AsyncStatus struct {
	ID         string                 `json:"id"`
	Status     string                 `json:"status"`
	Progress   float64                `json:"progress"`
	Message    string                 `json:"message,omitempty"`
	UpdateTime time.Time              `json:"update_time"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// BatchStatus represents the status of a batch operation
type BatchStatus struct {
	ID                  string                 `json:"id"`
	Status              string                 `json:"status"`
	TotalItems          int                    `json:"total_items"`
	CompletedItems      int                    `json:"completed_items"`
	FailedItems         int                    `json:"failed_items"`
	Progress            float64                `json:"progress"`
	StartTime           time.Time              `json:"start_time"`
	UpdateTime          time.Time              `json:"update_time"`
	EstimatedCompletion *time.Time             `json:"estimated_completion,omitempty"`
	Results             []interface{}          `json:"results,omitempty"`
	Errors              []string               `json:"errors,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// MultiModalInput represents input with multiple modalities
type MultiModalInput struct {
	Text      string                 `json:"text,omitempty"`
	Images    []ImageInput           `json:"images,omitempty"`
	Audio     []AudioInput           `json:"audio,omitempty"`
	Video     []VideoInput           `json:"video,omitempty"`
	Documents []DocumentInput        `json:"documents,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// MultiModalResponse represents a response with multiple modalities
type MultiModalResponse struct {
	Text       string                 `json:"text,omitempty"`
	Images     []ImageOutput          `json:"images,omitempty"`
	Audio      []AudioOutput          `json:"audio,omitempty"`
	Video      []VideoOutput          `json:"video,omitempty"`
	Documents  []DocumentOutput       `json:"documents,omitempty"`
	Confidence float64                `json:"confidence"`
	Duration   float64                `json:"duration_ms"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ImageInput represents image input
type ImageInput struct {
	Data        []byte                 `json:"data,omitempty"`
	URL         string                 `json:"url,omitempty"`
	Format      string                 `json:"format"`
	Width       int                    `json:"width,omitempty"`
	Height      int                    `json:"height,omitempty"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AudioInput represents audio input
type AudioInput struct {
	Data        []byte                 `json:"data,omitempty"`
	URL         string                 `json:"url,omitempty"`
	Format      string                 `json:"format"`
	Duration    float64                `json:"duration_seconds,omitempty"`
	SampleRate  int                    `json:"sample_rate,omitempty"`
	Channels    int                    `json:"channels,omitempty"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// VideoInput represents video input
type VideoInput struct {
	Data        []byte                 `json:"data,omitempty"`
	URL         string                 `json:"url,omitempty"`
	Format      string                 `json:"format"`
	Duration    float64                `json:"duration_seconds,omitempty"`
	Width       int                    `json:"width,omitempty"`
	Height      int                    `json:"height,omitempty"`
	FrameRate   float64                `json:"frame_rate,omitempty"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DocumentInput represents document input
type DocumentInput struct {
	Data        []byte                 `json:"data,omitempty"`
	URL         string                 `json:"url,omitempty"`
	Format      string                 `json:"format"`
	Title       string                 `json:"title,omitempty"`
	Author      string                 `json:"author,omitempty"`
	Language    string                 `json:"language,omitempty"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ImageOutput represents image output
type ImageOutput struct {
	Data        []byte                 `json:"data,omitempty"`
	URL         string                 `json:"url,omitempty"`
	Format      string                 `json:"format"`
	Width       int                    `json:"width,omitempty"`
	Height      int                    `json:"height,omitempty"`
	Description string                 `json:"description,omitempty"`
	Confidence  float64                `json:"confidence,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AudioOutput represents audio output
type AudioOutput struct {
	Data       []byte                 `json:"data,omitempty"`
	URL        string                 `json:"url,omitempty"`
	Format     string                 `json:"format"`
	Duration   float64                `json:"duration_seconds,omitempty"`
	SampleRate int                    `json:"sample_rate,omitempty"`
	Channels   int                    `json:"channels,omitempty"`
	Transcript string                 `json:"transcript,omitempty"`
	Confidence float64                `json:"confidence,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// VideoOutput represents video output
type VideoOutput struct {
	Data        []byte                 `json:"data,omitempty"`
	URL         string                 `json:"url,omitempty"`
	Format      string                 `json:"format"`
	Duration    float64                `json:"duration_seconds,omitempty"`
	Width       int                    `json:"width,omitempty"`
	Height      int                    `json:"height,omitempty"`
	FrameRate   float64                `json:"frame_rate,omitempty"`
	Description string                 `json:"description,omitempty"`
	Confidence  float64                `json:"confidence,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DocumentOutput represents document output
type DocumentOutput struct {
	Data       []byte                 `json:"data,omitempty"`
	URL        string                 `json:"url,omitempty"`
	Format     string                 `json:"format"`
	Title      string                 `json:"title,omitempty"`
	Summary    string                 `json:"summary,omitempty"`
	Language   string                 `json:"language,omitempty"`
	WordCount  int                    `json:"word_count,omitempty"`
	Confidence float64                `json:"confidence,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// RetryPolicy represents a retry policy configuration
type RetryPolicy struct {
	MaxRetries        int           `json:"max_retries"`
	InitialDelay      time.Duration `json:"initial_delay"`
	MaxDelay          time.Duration `json:"max_delay"`
	BackoffMultiplier float64       `json:"backoff_multiplier"`
	RetryableErrors   []string      `json:"retryable_errors"`
	Jitter            bool          `json:"jitter"`
}

// RetryStats represents retry statistics
type RetryStats struct {
	TotalRetries      int64            `json:"total_retries"`
	SuccessfulRetries int64            `json:"successful_retries"`
	FailedRetries     int64            `json:"failed_retries"`
	AverageRetries    float64          `json:"average_retries"`
	RetryRate         float64          `json:"retry_rate"`
	RetriesByError    map[string]int64 `json:"retries_by_error"`
}

// EncryptedRequest represents an encrypted request
type EncryptedRequest struct {
	EncryptedData []byte                 `json:"encrypted_data"`
	Algorithm     string                 `json:"algorithm"`
	KeyID         string                 `json:"key_id,omitempty"`
	IV            []byte                 `json:"iv,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// EncryptedResponse represents an encrypted response
type EncryptedResponse struct {
	EncryptedData []byte                 `json:"encrypted_data"`
	Algorithm     string                 `json:"algorithm"`
	KeyID         string                 `json:"key_id,omitempty"`
	IV            []byte                 `json:"iv,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID         string                 `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	UserID     string                 `json:"user_id"`
	Action     string                 `json:"action"`
	Resource   string                 `json:"resource"`
	ResourceID string                 `json:"resource_id,omitempty"`
	Success    bool                   `json:"success"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	UserAgent  string                 `json:"user_agent,omitempty"`
	Changes    map[string]interface{} `json:"changes,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
	Duration   float64                `json:"duration_ms,omitempty"`
}

// Claims represents JWT claims
type Claims struct {
	UserID    string                 `json:"user_id"`
	Username  string                 `json:"username,omitempty"`
	Email     string                 `json:"email,omitempty"`
	Roles     []string               `json:"roles,omitempty"`
	Scopes    []string               `json:"scopes,omitempty"`
	IssuedAt  time.Time              `json:"issued_at"`
	ExpiresAt time.Time              `json:"expires_at"`
	Issuer    string                 `json:"issuer,omitempty"`
	Audience  string                 `json:"audience,omitempty"`
	Custom    map[string]interface{} `json:"custom,omitempty"`
}

// Notification represents a notification
type Notification struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Recipients  []string               `json:"recipients"`
	Subject     string                 `json:"subject"`
	Message     string                 `json:"message"`
	Priority    string                 `json:"priority"`
	Channel     string                 `json:"channel"`
	Status      string                 `json:"status"`
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
	SentAt      *time.Time             `json:"sent_at,omitempty"`
	DeliveredAt *time.Time             `json:"delivered_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NotificationStatus represents the status of a notification
type NotificationStatus struct {
	ID          string                 `json:"id"`
	Status      string                 `json:"status"`
	SentAt      *time.Time             `json:"sent_at,omitempty"`
	DeliveredAt *time.Time             `json:"delivered_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Attempts    int                    `json:"attempts"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ContextExport represents an exported conversation context
type ContextExport struct {
	ContextID  string                 `json:"context_id"`
	Messages   []Message              `json:"messages"`
	Metadata   map[string]interface{} `json:"metadata"`
	ExportedAt time.Time              `json:"exported_at"`
	Version    string                 `json:"version"`
	Checksum   string                 `json:"checksum,omitempty"`
}

// RepairResult represents the result of a data repair operation
type RepairResult struct {
	Operation      string                 `json:"operation"`
	StartTime      time.Time              `json:"start_time"`
	EndTime        time.Time              `json:"end_time"`
	Duration       float64                `json:"duration_ms"`
	ItemsProcessed int64                  `json:"items_processed"`
	ItemsRepaired  int64                  `json:"items_repaired"`
	ItemsSkipped   int64                  `json:"items_skipped"`
	Success        bool                   `json:"success"`
	Message        string                 `json:"message,omitempty"`
	Errors         []string               `json:"errors,omitempty"`
	Details        map[string]interface{} `json:"details,omitempty"`
}

// Async operation status constants
const (
	AsyncStatusPending   = "pending"
	AsyncStatusRunning   = "running"
	AsyncStatusCompleted = "completed"
	AsyncStatusFailed    = "failed"
	AsyncStatusCancelled = "cancelled"
)

// Batch status constants
const (
	BatchStatusPending    = "pending"
	BatchStatusProcessing = "processing"
	BatchStatusCompleted  = "completed"
	BatchStatusFailed     = "failed"
	BatchStatusCancelled  = "cancelled"
)

// Notification status constants
const (
	NotificationStatusPending   = "pending"
	NotificationStatusSent      = "sent"
	NotificationStatusDelivered = "delivered"
	NotificationStatusFailed    = "failed"
	NotificationStatusCancelled = "cancelled"
)

// Notification channel constants
const (
	NotificationChannelEmail   = "email"
	NotificationChannelSlack   = "slack"
	NotificationChannelWebhook = "webhook"
	NotificationChannelSMS     = "sms"
	NotificationChannelPush    = "push"
)

// Helper methods for capabilities
func (pc *ProviderCapabilities) HasCapability(capability string) bool {
	for _, feature := range pc.Features {
		if feature == capability {
			return true
		}
	}
	return false
}

// Helper methods for rate limits
func (rl *RateLimit) IsExceeded() bool {
	return rl.RemainingRequests <= 0 || (rl.RemainingTokens > 0 && rl.RemainingTokens <= 0)
}

func (rl *RateLimit) TimeUntilReset() time.Duration {
	return time.Until(rl.ResetTime)
}

// Helper methods for async operations
func (ao *AsyncOperation) IsCompleted() bool {
	return ao.Status == AsyncStatusCompleted || ao.Status == AsyncStatusFailed || ao.Status == AsyncStatusCancelled
}

func (ao *AsyncOperation) IsRunning() bool {
	return ao.Status == AsyncStatusRunning
}

// Helper methods for batch operations
func (bs *BatchStatus) IsCompleted() bool {
	return bs.Status == BatchStatusCompleted || bs.Status == BatchStatusFailed || bs.Status == BatchStatusCancelled
}

func (bs *BatchStatus) CalculateProgress() {
	if bs.TotalItems > 0 {
		bs.Progress = float64(bs.CompletedItems+bs.FailedItems) / float64(bs.TotalItems) * 100
	}
}

// Helper methods for notifications
func (n *Notification) IsDelivered() bool {
	return n.Status == NotificationStatusDelivered && n.DeliveredAt != nil
}

func (n *Notification) IsFailed() bool {
	return n.Status == NotificationStatusFailed || n.Error != ""
}

// Helper methods for retry policies
func (rp *RetryPolicy) ShouldRetry(attempt int, err error) bool {
	if attempt >= rp.MaxRetries {
		return false
	}

	if len(rp.RetryableErrors) == 0 {
		return true
	}

	errorType := GetErrorType(err)
	for _, retryableError := range rp.RetryableErrors {
		if errorType == retryableError {
			return true
		}
	}

	return false
}

func (rp *RetryPolicy) CalculateDelay(attempt int) time.Duration {
	delay := rp.InitialDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * rp.BackoffMultiplier)
		if delay > rp.MaxDelay {
			delay = rp.MaxDelay
			break
		}
	}
	return delay
}
