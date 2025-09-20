package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// App represents an application in the registry
type App struct {
	AppID          string                 `json:"appId"`
	Version        string                 `json:"version"`
	Runtime        string                 `json:"runtime"`
	Tools          []AppTool              `json:"tools"`
	ArtifactURI    string                 `json:"artifactUri"`
	SourceLanguage string                 `json:"sourceLanguage,omitempty"`
	Files          []File                 `json:"files,omitempty"`
	Description    string                 `json:"description,omitempty"`
	Author         string                 `json:"author,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
	Dependencies   map[string]string      `json:"dependencies,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	Status         string                 `json:"status"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// AppTool represents a tool provided by an application
type AppTool struct {
	Name        string                 `json:"name"`
	InputFormat string                 `json:"inputFormat"`
	Description string                 `json:"description,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
	Examples    []ToolExample          `json:"examples,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// File represents a file in an application
type File struct {
	Name     string                 `json:"name"`
	Content  string                 `json:"content"`
	Type     string                 `json:"type,omitempty"`
	Size     int64                  `json:"size,omitempty"`
	Hash     string                 `json:"hash,omitempty"`
	Encoding string                 `json:"encoding,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolExample represents an example of tool usage
type ToolExample struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Input       interface{} `json:"input"`
	Output      string      `json:"output"`
}

// AppStats represents statistics about an application
type AppStats struct {
	AppID           string                `json:"app_id"`
	TotalExecutions int64                 `json:"total_executions"`
	SuccessfulRuns  int64                 `json:"successful_runs"`
	FailedRuns      int64                 `json:"failed_runs"`
	AverageRuntime  float64               `json:"average_runtime_ms"`
	LastExecution   *time.Time            `json:"last_execution,omitempty"`
	ToolStats       map[string]*ToolStats `json:"tool_stats,omitempty"`
	ErrorRate       float64               `json:"error_rate"`
	Uptime          float64               `json:"uptime_percentage"`
}

// ToolStats represents statistics about a specific tool
type ToolStats struct {
	ToolName       string     `json:"tool_name"`
	ExecutionCount int64      `json:"execution_count"`
	SuccessCount   int64      `json:"success_count"`
	FailureCount   int64      `json:"failure_count"`
	AverageRuntime float64    `json:"average_runtime_ms"`
	LastExecution  *time.Time `json:"last_execution,omitempty"`
	ErrorRate      float64    `json:"error_rate"`
	CommonErrors   []string   `json:"common_errors,omitempty"`
}

// AppExecution represents the execution of an application tool
type AppExecution struct {
	ID        string                 `json:"id"`
	AppID     string                 `json:"app_id"`
	ToolName  string                 `json:"tool_name"`
	Input     json.RawMessage        `json:"input"`
	Output    string                 `json:"output,omitempty"`
	Status    string                 `json:"status"`
	Error     string                 `json:"error,omitempty"`
	StartTime time.Time              `json:"start_time"`
	EndTime   *time.Time             `json:"end_time,omitempty"`
	Duration  float64                `json:"duration_ms,omitempty"`
	UserID    string                 `json:"user_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	ContextID string                 `json:"context_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// AppRegistry represents the application registry
type AppRegistry struct {
	Apps         map[string]*App `json:"apps"`
	LastUpdated  time.Time       `json:"last_updated"`
	Version      string          `json:"version"`
	TotalApps    int             `json:"total_apps"`
	ActiveApps   int             `json:"active_apps"`
	InactiveApps int             `json:"inactive_apps"`
}

// AppCreateRequest represents a request to create an application
type AppCreateRequest struct {
	AppID          string                 `json:"appId"`
	Version        string                 `json:"version"`
	Runtime        string                 `json:"runtime"`
	Tools          []AppTool              `json:"tools"`
	AppSrc         string                 `json:"appSrc"`
	Dependencies   map[string]string      `json:"dependencies,omitempty"`
	Description    string                 `json:"description,omitempty"`
	Author         string                 `json:"author,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
	SourceLanguage string                 `json:"sourceLanguage,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// AppUpdateRequest represents a request to update an application
type AppUpdateRequest struct {
	Version      *string                `json:"version,omitempty"`
	Tools        []AppTool              `json:"tools,omitempty"`
	AppSrc       *string                `json:"appSrc,omitempty"`
	Dependencies map[string]string      `json:"dependencies,omitempty"`
	Description  *string                `json:"description,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	Status       *string                `json:"status,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// AppFilter represents filters for querying applications
type AppFilter struct {
	Runtime       string     `json:"runtime,omitempty"`
	Status        string     `json:"status,omitempty"`
	Author        string     `json:"author,omitempty"`
	Tags          []string   `json:"tags,omitempty"`
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`
	HasTool       string     `json:"has_tool,omitempty"`
	Limit         int        `json:"limit,omitempty"`
	Offset        int        `json:"offset,omitempty"`
}

// App status constants
const (
	AppStatusActive     = "active"
	AppStatusInactive   = "inactive"
	AppStatusDeveloping = "developing"
	AppStatusDeprecated = "deprecated"
	AppStatusError      = "error"
)

// App runtime constants
const (
	RuntimeWASM       = "wasm"
	RuntimeJavaScript = "javascript"
	RuntimePython     = "python"
	RuntimeGo         = "go"
)

// Validate validates an app create request
func (acr *AppCreateRequest) Validate() error {
	if acr.AppID == "" {
		return NewValidationError("appId", "required", "appId is required", acr.AppID)
	}

	if acr.Version == "" {
		return NewValidationError("version", "required", "version is required", acr.Version)
	}

	if acr.Runtime == "" {
		return NewValidationError("runtime", "required", "runtime is required", acr.Runtime)
	}

	validRuntimes := []string{RuntimeWASM, RuntimeJavaScript, RuntimePython, RuntimeGo}
	validRuntime := false
	for _, runtime := range validRuntimes {
		if acr.Runtime == runtime {
			validRuntime = true
			break
		}
	}
	if !validRuntime {
		return NewValidationError("runtime", "enum", "invalid runtime", acr.Runtime)
	}

	if acr.AppSrc == "" {
		return NewValidationError("appSrc", "required", "appSrc is required", acr.AppSrc)
	}

	if len(acr.Tools) == 0 {
		return NewValidationError("tools", "required", "at least one tool is required", acr.Tools)
	}

	for i, tool := range acr.Tools {
		if tool.Name == "" {
			return NewValidationError(fmt.Sprintf("tools[%d].name", i), "required", "tool name is required", tool.Name)
		}
	}

	return nil
}

// Validate validates an app tool
func (at *AppTool) Validate() error {
	if at.Name == "" {
		return NewValidationError("name", "required", "tool name is required", at.Name)
	}

	// Validate tool name format (alphanumeric and underscores only)
	for _, char := range at.Name {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '_') {
			return NewValidationError("name", "format", "tool name must contain only alphanumeric characters and underscores", at.Name)
		}
	}

	return nil
}

// GetTool returns a tool by name from the app
func (a *App) GetTool(toolName string) (*AppTool, bool) {
	for _, tool := range a.Tools {
		if tool.Name == toolName {
			return &tool, true
		}
	}
	return nil, false
}

// HasTool checks if the app has a specific tool
func (a *App) HasTool(toolName string) bool {
	_, exists := a.GetTool(toolName)
	return exists
}

// AddTool adds a tool to the app
func (a *App) AddTool(tool AppTool) error {
	if err := tool.Validate(); err != nil {
		return err
	}

	// Check for duplicate tool names
	if a.HasTool(tool.Name) {
		return NewValidationError("name", "unique", "tool name already exists", tool.Name)
	}

	a.Tools = append(a.Tools, tool)
	a.UpdatedAt = time.Now()
	return nil
}

// RemoveTool removes a tool from the app
func (a *App) RemoveTool(toolName string) bool {
	for i, tool := range a.Tools {
		if tool.Name == toolName {
			a.Tools = append(a.Tools[:i], a.Tools[i+1:]...)
			a.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// UpdateTool updates a tool in the app
func (a *App) UpdateTool(toolName string, updatedTool AppTool) error {
	if err := updatedTool.Validate(); err != nil {
		return err
	}

	for i, tool := range a.Tools {
		if tool.Name == toolName {
			a.Tools[i] = updatedTool
			a.UpdatedAt = time.Now()
			return nil
		}
	}

	return NewValidationError("toolName", "not_found", "tool not found", toolName)
}

// GetToolNames returns a list of tool names
func (a *App) GetToolNames() []string {
	names := make([]string, len(a.Tools))
	for i, tool := range a.Tools {
		names[i] = tool.Name
	}
	return names
}

// SetStatus sets the app status and updates the timestamp
func (a *App) SetStatus(status string) {
	a.Status = status
	a.UpdatedAt = time.Now()
}

// IsActive checks if the app is active
func (a *App) IsActive() bool {
	return a.Status == AppStatusActive
}

// AddMetadata adds metadata to the app
func (a *App) AddMetadata(key string, value interface{}) {
	if a.Metadata == nil {
		a.Metadata = make(map[string]interface{})
	}
	a.Metadata[key] = value
	a.UpdatedAt = time.Now()
}

// GetMetadata gets metadata from the app
func (a *App) GetMetadata(key string) (interface{}, bool) {
	if a.Metadata == nil {
		return nil, false
	}
	value, exists := a.Metadata[key]
	return value, exists
}

// CalculateSuccessRate calculates the success rate for app stats
func (as *AppStats) CalculateSuccessRate() {
	if as.TotalExecutions > 0 {
		as.ErrorRate = float64(as.FailedRuns) / float64(as.TotalExecutions) * 100
	} else {
		as.ErrorRate = 0
	}
}

// CalculateUptime calculates the uptime percentage for app stats
func (as *AppStats) CalculateUptime() {
	// This would typically be based on actual uptime monitoring
	// For now, we'll calculate it based on success rate
	if as.TotalExecutions > 0 {
		as.Uptime = float64(as.SuccessfulRuns) / float64(as.TotalExecutions) * 100
	} else {
		as.Uptime = 100 // No executions means no failures
	}
}

// Update updates app stats with a new execution
func (as *AppStats) Update(execution *AppExecution) {
	as.TotalExecutions++
	as.LastExecution = &execution.StartTime

	if execution.Status == ExecutionStatusCompleted {
		as.SuccessfulRuns++
	} else if execution.Status == ExecutionStatusFailed {
		as.FailedRuns++
	}

	if execution.Duration > 0 {
		// Update average runtime using incremental average
		newAverage := as.AverageRuntime + (execution.Duration-as.AverageRuntime)/float64(as.TotalExecutions)
		as.AverageRuntime = newAverage
	}

	// Update tool stats
	if as.ToolStats == nil {
		as.ToolStats = make(map[string]*ToolStats)
	}

	toolStats, exists := as.ToolStats[execution.ToolName]
	if !exists {
		toolStats = &ToolStats{
			ToolName: execution.ToolName,
		}
		as.ToolStats[execution.ToolName] = toolStats
	}

	toolStats.Update(execution)
	as.CalculateSuccessRate()
	as.CalculateUptime()
}

// Update updates tool stats with a new execution
func (ts *ToolStats) Update(execution *AppExecution) {
	ts.ExecutionCount++
	ts.LastExecution = &execution.StartTime

	if execution.Status == ExecutionStatusCompleted {
		ts.SuccessCount++
	} else if execution.Status == ExecutionStatusFailed {
		ts.FailureCount++
		if execution.Error != "" && !contains(ts.CommonErrors, execution.Error) {
			ts.CommonErrors = append(ts.CommonErrors, execution.Error)
		}
	}

	if execution.Duration > 0 {
		// Update average runtime using incremental average
		newAverage := ts.AverageRuntime + (execution.Duration-ts.AverageRuntime)/float64(ts.ExecutionCount)
		ts.AverageRuntime = newAverage
	}

	if ts.ExecutionCount > 0 {
		ts.ErrorRate = float64(ts.FailureCount) / float64(ts.ExecutionCount) * 100
	} else {
		ts.ErrorRate = 0
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
