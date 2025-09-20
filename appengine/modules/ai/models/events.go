package models

import (
	"time"
)

// Event represents a module event
type Event struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Source      string                 `json:"source"`
	Subject     string                 `json:"subject,omitempty"`
	Data        interface{}            `json:"data,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Version     string                 `json:"version,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Correlation string                 `json:"correlation_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
}

// ProviderEvent represents a provider-specific event
type ProviderEvent struct {
	ProviderName string                 `json:"provider_name"`
	EventType    string                 `json:"event_type"`
	Status       string                 `json:"status,omitempty"`
	Model        string                 `json:"model,omitempty"`
	TokensUsed   int                    `json:"tokens_used,omitempty"`
	Duration     float64                `json:"duration_ms,omitempty"`
	Error        string                 `json:"error,omitempty"`
	RequestID    string                 `json:"request_id,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ContextEvent represents a context-related event
type ContextEvent struct {
	ContextID    string                 `json:"context_id"`
	EventType    string                 `json:"event_type"`
	MessageCount int                    `json:"message_count,omitempty"`
	TokenCount   int                    `json:"token_count,omitempty"`
	Operation    string                 `json:"operation,omitempty"`
	Duration     float64                `json:"duration_ms,omitempty"`
	Success      bool                   `json:"success"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ToolEvent represents a tool execution event
type ToolEvent struct {
	ToolName    string                 `json:"tool_name"`
	EventType   string                 `json:"event_type"`
	ExecutionID string                 `json:"execution_id,omitempty"`
	Status      string                 `json:"status"`
	Duration    float64                `json:"duration_ms,omitempty"`
	InputSize   int64                  `json:"input_size_bytes,omitempty"`
	OutputSize  int64                  `json:"output_size_bytes,omitempty"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	ContextID   string                 `json:"context_id,omitempty"`
	AppID       string                 `json:"app_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ModuleEvent represents a module lifecycle event
type ModuleEvent struct {
	EventType      string                 `json:"event_type"`
	Status         string                 `json:"status"`
	PreviousStatus string                 `json:"previous_status,omitempty"`
	Version        string                 `json:"version,omitempty"`
	ConfigVersion  string                 `json:"config_version,omitempty"`
	Duration       float64                `json:"duration_ms,omitempty"`
	Reason         string                 `json:"reason,omitempty"`
	Error          string                 `json:"error,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ErrorEvent represents an error event
type ErrorEvent struct {
	ErrorType  string                 `json:"error_type"`
	ErrorCode  string                 `json:"error_code,omitempty"`
	Message    string                 `json:"message"`
	Component  string                 `json:"component"`
	Operation  string                 `json:"operation,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
	Severity   string                 `json:"severity"`
	Recovery   string                 `json:"recovery,omitempty"`
	UserID     string                 `json:"user_id,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
}

// MetricsEvent represents a metrics event
type MetricsEvent struct {
	MetricName string                 `json:"metric_name"`
	MetricType string                 `json:"metric_type"` // "counter", "gauge", "histogram"
	Value      float64                `json:"value"`
	Unit       string                 `json:"unit,omitempty"`
	Tags       map[string]string      `json:"tags,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// NotificationEvent represents a notification event
type NotificationEvent struct {
	NotificationType string                 `json:"notification_type"`
	Recipients       []string               `json:"recipients"`
	Subject          string                 `json:"subject"`
	Message          string                 `json:"message"`
	Priority         string                 `json:"priority"` // "low", "normal", "high", "critical"
	Channel          string                 `json:"channel"`  // "email", "slack", "webhook"
	Status           string                 `json:"status"`
	DeliveredAt      *time.Time             `json:"delivered_at,omitempty"`
	Error            string                 `json:"error,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// AuditEvent represents an audit event
type AuditEvent struct {
	Action     string                 `json:"action"`
	Resource   string                 `json:"resource"`
	ResourceID string                 `json:"resource_id,omitempty"`
	UserID     string                 `json:"user_id"`
	UserAgent  string                 `json:"user_agent,omitempty"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	Success    bool                   `json:"success"`
	Reason     string                 `json:"reason,omitempty"`
	Changes    map[string]interface{} `json:"changes,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// EventFilter represents filters for querying events
type EventFilter struct {
	EventTypes []string   `json:"event_types,omitempty"`
	Source     string     `json:"source,omitempty"`
	Subject    string     `json:"subject,omitempty"`
	UserID     string     `json:"user_id,omitempty"`
	SessionID  string     `json:"session_id,omitempty"`
	StartTime  *time.Time `json:"start_time,omitempty"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	Severity   string     `json:"severity,omitempty"`
	Success    *bool      `json:"success,omitempty"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
}

// EventStream represents a stream of events
type EventStream struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Filter      *EventFilter           `json:"filter,omitempty"`
	Subscribers []EventSubscriber      `json:"subscribers"`
	BufferSize  int                    `json:"buffer_size"`
	MaxRetries  int                    `json:"max_retries"`
	RetryDelay  time.Duration          `json:"retry_delay"`
	Status      string                 `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// EventSubscriber represents a subscriber to an event stream
type EventSubscriber struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Endpoint      string                 `json:"endpoint"`
	Method        string                 `json:"method"` // "POST", "PUT", etc.
	Headers       map[string]string      `json:"headers,omitempty"`
	Active        bool                   `json:"active"`
	LastDelivery  *time.Time             `json:"last_delivery,omitempty"`
	DeliveryCount int64                  `json:"delivery_count"`
	ErrorCount    int64                  `json:"error_count"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// EventBatch represents a batch of events
type EventBatch struct {
	ID          string                 `json:"id"`
	Events      []*Event               `json:"events"`
	Size        int                    `json:"size"`
	CreatedAt   time.Time              `json:"created_at"`
	ProcessedAt *time.Time             `json:"processed_at,omitempty"`
	Status      string                 `json:"status"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Event type constants
const (
	EventTypeMessageSent      = "message.sent"
	EventTypeMessageReceived  = "message.received"
	EventTypeMessageFailed    = "message.failed"
	EventTypeToolExecuted     = "tool.executed"
	EventTypeToolFailed       = "tool.failed"
	EventTypeContextCreated   = "context.created"
	EventTypeContextUpdated   = "context.updated"
	EventTypeContextDeleted   = "context.deleted"
	EventTypeContextExpired   = "context.expired"
	EventTypeProviderStarted  = "provider.started"
	EventTypeProviderStopped  = "provider.stopped"
	EventTypeProviderFailed   = "provider.failed"
	EventTypeProviderSwitched = "provider.switched"
	EventTypeModuleStarted    = "module.started"
	EventTypeModuleStopped    = "module.stopped"
	EventTypeModuleRestarted  = "module.restarted"
	EventTypeModuleError      = "module.error"
	EventTypeConfigUpdated    = "config.updated"
	EventTypeHealthCheck      = "health.check"
	EventTypeMetricsCollected = "metrics.collected"
	EventTypeErrorOccurred    = "error.occurred"
	EventTypeAuditAction      = "audit.action"
	EventTypeNotificationSent = "notification.sent"
)

// Event source constants
const (
	EventSourceModule       = "ai.module"
	EventSourceProvider     = "ai.provider"
	EventSourceContext      = "ai.context"
	EventSourceTool         = "ai.tool"
	EventSourceConfig       = "ai.config"
	EventSourceHealth       = "ai.health"
	EventSourceMetrics      = "ai.metrics"
	EventSourceAudit        = "ai.audit"
	EventSourceNotification = "ai.notification"
)

// Event status constants
const (
	EventStatusPending    = "pending"
	EventStatusProcessing = "processing"
	EventStatusProcessed  = "processed"
	EventStatusFailed     = "failed"
	EventStatusRetrying   = "retrying"
)

// Severity constants
const (
	SeverityDebug    = "debug"
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityError    = "error"
	SeverityCritical = "critical"
)

// Priority constants
const (
	PriorityLow      = "low"
	PriorityNormal   = "normal"
	PriorityHigh     = "high"
	PriorityCritical = "critical"
)

// NewEvent creates a new event with basic information
func NewEvent(eventType, source string) *Event {
	return &Event{
		Type:      eventType,
		Source:    source,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
}

// NewProviderEvent creates a new provider event
func NewProviderEvent(providerName, eventType string) *ProviderEvent {
	return &ProviderEvent{
		ProviderName: providerName,
		EventType:    eventType,
		Metadata:     make(map[string]interface{}),
	}
}

// NewContextEvent creates a new context event
func NewContextEvent(contextID, eventType string) *ContextEvent {
	return &ContextEvent{
		ContextID: contextID,
		EventType: eventType,
		Metadata:  make(map[string]interface{}),
	}
}

// AddMetadata adds metadata to the context event
func (ce *ContextEvent) AddMetadata(key string, value interface{}) {
	if ce.Metadata == nil {
		ce.Metadata = make(map[string]interface{})
	}
	ce.Metadata[key] = value
}

// NewToolEvent creates a new tool event
func NewToolEvent(toolName, eventType string) *ToolEvent {
	return &ToolEvent{
		ToolName:  toolName,
		EventType: eventType,
		Metadata:  make(map[string]interface{}),
	}
}

// NewModuleEvent creates a new module event
func NewModuleEvent(eventType string) *ModuleEvent {
	return &ModuleEvent{
		EventType: eventType,
		Metadata:  make(map[string]interface{}),
	}
}

// NewErrorEvent creates a new error event
func NewErrorEvent(errorType, component, message string) *ErrorEvent {
	return &ErrorEvent{
		ErrorType: errorType,
		Component: component,
		Message:   message,
		Context:   make(map[string]interface{}),
	}
}

// NewMetricsEvent creates a new metrics event
func NewMetricsEvent(metricName, metricType string, value float64) *MetricsEvent {
	return &MetricsEvent{
		MetricName: metricName,
		MetricType: metricType,
		Value:      value,
		Timestamp:  time.Now(),
		Tags:       make(map[string]string),
		Metadata:   make(map[string]interface{}),
	}
}

// AddMetadata adds metadata to an event
func (e *Event) AddMetadata(key string, value interface{}) {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
}

// WithCorrelation sets the correlation ID for an event
func (e *Event) WithCorrelation(correlationID string) *Event {
	e.Correlation = correlationID
	return e
}

// WithUser sets the user ID for an event
func (e *Event) WithUser(userID string) *Event {
	e.UserID = userID
	return e
}

// WithSession sets the session ID for an event
func (e *Event) WithSession(sessionID string) *Event {
	e.SessionID = sessionID
	return e
}

// WithSubject sets the subject for an event
func (e *Event) WithSubject(subject string) *Event {
	e.Subject = subject
	return e
}

// WithData sets the data for an event
func (e *Event) WithData(data interface{}) *Event {
	e.Data = data
	return e
}

// IsSuccess checks if the event represents a successful operation
func (e *Event) IsSuccess() bool {
	if success, ok := e.Metadata["success"].(bool); ok {
		return success
	}
	return !e.IsError()
}

// IsError checks if the event represents an error
func (e *Event) IsError() bool {
	return e.Type == EventTypeMessageFailed ||
		e.Type == EventTypeToolFailed ||
		e.Type == EventTypeProviderFailed ||
		e.Type == EventTypeModuleError ||
		e.Type == EventTypeErrorOccurred
}

// GetDuration returns the duration if available in metadata
func (e *Event) GetDuration() float64 {
	if duration, ok := e.Metadata["duration_ms"].(float64); ok {
		return duration
	}
	return 0
}

// Validate validates an event filter
func (ef *EventFilter) Validate() error {
	if ef.StartTime != nil && ef.EndTime != nil && ef.StartTime.After(*ef.EndTime) {
		return NewValidationError("time_range", "invalid", "start time must be before end time", ef.StartTime)
	}

	if ef.Limit < 0 {
		return NewValidationError("limit", "non_negative", "limit must be non-negative", ef.Limit)
	}

	if ef.Offset < 0 {
		return NewValidationError("offset", "non_negative", "offset must be non-negative", ef.Offset)
	}

	return nil
}

// Matches checks if an event matches the filter criteria
func (ef *EventFilter) Matches(event *Event) bool {
	// Check event types
	if len(ef.EventTypes) > 0 {
		match := false
		for _, eventType := range ef.EventTypes {
			if event.Type == eventType {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	// Check source
	if ef.Source != "" && event.Source != ef.Source {
		return false
	}

	// Check subject
	if ef.Subject != "" && event.Subject != ef.Subject {
		return false
	}

	// Check user ID
	if ef.UserID != "" && event.UserID != ef.UserID {
		return false
	}

	// Check session ID
	if ef.SessionID != "" && event.SessionID != ef.SessionID {
		return false
	}

	// Check time range
	if ef.StartTime != nil && event.Timestamp.Before(*ef.StartTime) {
		return false
	}

	if ef.EndTime != nil && event.Timestamp.After(*ef.EndTime) {
		return false
	}

	// Check success status
	if ef.Success != nil && event.IsSuccess() != *ef.Success {
		return false
	}

	return true
}

// AddTag adds a tag to a metrics event
func (me *MetricsEvent) AddTag(key, value string) {
	if me.Tags == nil {
		me.Tags = make(map[string]string)
	}
	me.Tags[key] = value
}

// WithUnit sets the unit for a metrics event
func (me *MetricsEvent) WithUnit(unit string) *MetricsEvent {
	me.Unit = unit
	return me
}

// IsDelivered checks if a notification event was delivered
func (ne *NotificationEvent) IsDelivered() bool {
	return ne.DeliveredAt != nil && ne.Status == "delivered"
}

// IsFailed checks if a notification event failed
func (ne *NotificationEvent) IsFailed() bool {
	return ne.Status == "failed" || ne.Error != ""
}
