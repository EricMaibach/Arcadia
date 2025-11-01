package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"arcadia/modules/ai/interfaces"
	"arcadia/modules/ai/models"
	"arcadia/pkg/logging"
)

// Manager implements the tool management functionality
type Manager struct {
	config          *models.Config
	logger          logging.Logger
	metrics         interfaces.Metrics
	eventBus        interfaces.EventBus
	registryAccess  interfaces.RegistryAccess
	appRunner       interfaces.AppRunner
	appCreator      interfaces.AppCreator
	embeddingSearch interfaces.EmbeddingSearch
	schedulerService interfaces.SchedulerService
	tools           map[string]*models.Tool
	toolStats       map[string]*models.ToolStats
	mutex           sync.RWMutex
	registry        *ToolRegistry
}

// ToolRegistry manages tool registration and discovery
type ToolRegistry struct {
	tools    map[string]*models.Tool
	mutex    sync.RWMutex
	logger   logging.Logger
	metrics  interfaces.Metrics
}

// NewManager creates a new tool manager
func NewManager(config *models.Config, deps *interfaces.Dependencies) *Manager {
	registry := &ToolRegistry{
		tools:   make(map[string]*models.Tool),
		logger:  deps.Logger,
		metrics: deps.Metrics,
	}

	tm := &Manager{
		config:          config,
		logger:          deps.Logger,
		metrics:         deps.Metrics,
		eventBus:        deps.EventBus,
		registryAccess:  deps.RegistryAccess,
		appRunner:       deps.AppRunner,
		appCreator:      deps.AppCreator,
		embeddingSearch: deps.EmbeddingSearch,
		schedulerService: deps.SchedulerService,
		tools:           make(map[string]*models.Tool),
		toolStats:       make(map[string]*models.ToolStats),
		registry:        registry,
	}

	// Load initial tools
	if config.EnableMCP {
		tm.LoadMCPTools(context.Background())
	}

	return tm
}

// RegisterTool registers a new tool
func (tm *Manager) RegisterTool(ctx context.Context, tool models.Tool) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Validate tool
	if err := tm.validateTool(&tool); err != nil {
		return err
	}

	// Check if tool already exists
	if _, exists := tm.tools[tool.Name]; exists {
		return models.NewValidationError("tool_name", "unique", "tool already exists", tool.Name)
	}

	// Check if tool is disabled
	if tm.isToolDisabled(tool.Name) {
		return models.NewValidationError("tool_name", "disabled", "tool is disabled in configuration", tool.Name)
	}

	// Set defaults
	if tool.Category == "" {
		tool.Category = "custom"
	}
	if tool.Version == "" {
		tool.Version = "1.0.0"
	}
	tool.Enabled = true

	tm.tools[tool.Name] = &tool

	// Initialize stats
	tm.toolStats[tool.Name] = &models.ToolStats{
		ToolName: tool.Name,
	}

	// Record metrics
	if tm.metrics != nil {
		tm.metrics.IncrementCounter("tools_registered", map[string]string{
			"tool_name": tool.Name,
			"category":  tool.Category,
			"provider":  tool.Provider,
		})
	}

	// Publish event
	if tm.eventBus != nil {
		event := models.NewToolEvent(tool.Name, "tool.registered")
		event.Success = true
		event.Metadata = map[string]interface{}{
			"category": tool.Category,
			"provider": tool.Provider,
		}
		tm.publishEvent(ctx, models.NewEvent("tool.registered", models.EventSourceTool).WithData(event))
	}

	if tm.logger != nil {
		tm.logger.Info(ctx, "Registered tool", "tool_name", tool.Name, "category", tool.Category)
	}

	return nil
}

// UnregisterTool removes a tool
func (tm *Manager) UnregisterTool(ctx context.Context, toolName string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tool, exists := tm.tools[toolName]
	if !exists {
		return models.NewToolError(toolName, "tool not found", nil)
	}

	delete(tm.tools, toolName)
	delete(tm.toolStats, toolName)

	// Record metrics
	if tm.metrics != nil {
		tm.metrics.IncrementCounter("tools_unregistered", map[string]string{
			"tool_name": toolName,
			"category":  tool.Category,
			"provider":  tool.Provider,
		})
	}

	// Publish event
	if tm.eventBus != nil {
		event := models.NewToolEvent(toolName, "tool.unregistered")
		event.Success = true
		tm.publishEvent(ctx, models.NewEvent("tool.unregistered", models.EventSourceTool).WithData(event))
	}

	if tm.logger != nil {
		tm.logger.Info(ctx, "Unregistered tool", "tool_name", toolName)
	}

	return nil
}

// GetTool retrieves a tool by name
func (tm *Manager) GetTool(ctx context.Context, toolName string) (*models.Tool, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	tool, exists := tm.tools[toolName]
	if !exists {
		return nil, models.NewToolError(toolName, "tool not found", nil)
	}

	return tool, nil
}

// ListTools returns all registered tools
func (tm *Manager) ListTools(ctx context.Context) ([]models.Tool, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	tools := make([]models.Tool, 0, len(tm.tools))
	for _, tool := range tm.tools {
		if tool.Enabled {
			tools = append(tools, *tool)
		}
	}

	return tools, nil
}

// ExecuteTool executes a tool with the given input
func (tm *Manager) ExecuteTool(ctx context.Context, toolName string, input any) (string, error) {
	startTime := time.Now()

	// Check if tool exists and is enabled
	tool, err := tm.GetTool(ctx, toolName)
	if err != nil {
		return "", err
	}

	if !tool.Enabled {
		return "", models.NewToolError(toolName, "tool is disabled", input)
	}

	if tm.logger != nil {
		tm.logger.Debug(ctx, "Executing tool", "tool_name", toolName, "input", input)
	}

	var result string
	var execErr error

	// Publish start event
	if tm.eventBus != nil {
		event := models.NewToolEvent(toolName, models.EventTypeToolExecuted)
		event.Status = models.ExecutionStatusRunning
		event.InputSize = tm.calculateInputSize(input)
		tm.publishEvent(ctx, models.NewEvent(models.EventTypeToolExecuted, models.EventSourceTool).WithData(event))
	}

	// Execute the tool based on its type
	switch {
	case tm.isSystemTool(toolName):
		result, execErr = tm.executeSystemTool(ctx, toolName, input)
	case tm.isAppTool(toolName):
		result, execErr = tm.executeAppTool(ctx, toolName, input)
	default:
		execErr = models.NewToolError(toolName, "unknown tool type", input)
	}

	duration := time.Since(startTime).Milliseconds()
	success := execErr == nil

	// Update stats
	tm.updateToolStats(toolName, float64(duration), success, execErr)

	// Record metrics
	if tm.metrics != nil {
		tags := map[string]string{
			"tool_name": toolName,
			"category":  tool.Category,
			"provider":  tool.Provider,
			"success":   fmt.Sprintf("%t", success),
		}
		tm.metrics.RecordDuration("tool_execution_duration", float64(duration), tags)
		tm.metrics.IncrementCounter("tool_executions", tags)

		if !success {
			tm.metrics.IncrementCounter("tool_errors", tags)
		}
	}

	// Publish end event
	if tm.eventBus != nil {
		event := models.NewToolEvent(toolName, models.EventTypeToolExecuted)
		if success {
			event.Status = models.ExecutionStatusCompleted
		} else {
			event.Status = models.ExecutionStatusFailed
			event.Error = execErr.Error()
		}
		event.Duration = float64(duration)
		event.InputSize = tm.calculateInputSize(input)
		event.OutputSize = int64(len(result))
		event.Success = success
		tm.publishEvent(ctx, models.NewEvent(models.EventTypeToolExecuted, models.EventSourceTool).WithData(event))
	}

	if tm.logger != nil {
		if success {
			tm.logger.Info(ctx, "Tool execution completed", "tool_name", toolName, "duration_ms", duration)
		} else {
			tm.logger.Error(ctx, "Tool execution failed", "tool_name", toolName, "duration_ms", duration, "error", execErr)
		}
	}

	return result, execErr
}

// ValidateToolInput validates tool input against its schema
func (tm *Manager) ValidateToolInput(ctx context.Context, toolName string, input any) error {
	tool, err := tm.GetTool(ctx, toolName)
	if err != nil {
		return err
	}

	// Basic validation - in a real implementation, this would use JSON schema validation
	if tool.InputSchema != nil {
		// Placeholder validation logic
		if input == nil {
			return models.NewValidationError("input", "required", "input is required", input)
		}
	}

	return nil
}

// GetToolSchema returns the input schema for a tool
func (tm *Manager) GetToolSchema(ctx context.Context, toolName string) (json.RawMessage, error) {
	tool, err := tm.GetTool(ctx, toolName)
	if err != nil {
		return nil, err
	}

	if tool.InputSchema == nil {
		return json.RawMessage(`{"type": "object"}`), nil
	}

	schema, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool schema: %w", err)
	}

	return schema, nil
}

// RefreshTools refreshes all tools
func (tm *Manager) RefreshTools(ctx context.Context) error {
	if tm.logger != nil {
		tm.logger.Info(ctx, "Refreshing tools")
	}

	var refreshErr error

	// Refresh MCP tools if enabled
	if tm.config.EnableMCP {
		if err := tm.LoadMCPTools(ctx); err != nil {
			if tm.logger != nil {
				tm.logger.Error(ctx, "Failed to load MCP tools", "error", err)
			}
			refreshErr = err
		}
	}

	// Refresh dynamic app tools
	if err := tm.LoadDynamicTools(ctx); err != nil {
		if tm.logger != nil {
			tm.logger.Error(ctx, "Failed to load dynamic tools", "error", err)
		}
		if refreshErr == nil {
			refreshErr = err
		}
	}

	// Record metrics
	if tm.metrics != nil {
		tm.metrics.IncrementCounter("tools_refreshed", map[string]string{
			"success": fmt.Sprintf("%t", refreshErr == nil),
		})
	}

	if tm.logger != nil {
		if refreshErr == nil {
			tm.logger.Info(ctx, "Tools refreshed successfully", "tool_count", len(tm.tools))
		} else {
			tm.logger.Error(ctx, "Tool refresh completed with errors", "error", refreshErr)
		}
	}

	return refreshErr
}

// LoadMCPTools loads static MCP tools
func (tm *Manager) LoadMCPTools(ctx context.Context) error {
	if tm.logger != nil {
		tm.logger.Debug(ctx, "Loading MCP tools")
	}

	// Static system tools
	staticTools := []models.Tool{
		{
			Name:        "list_apps",
			Description: "List all registered applications in the Arcadia App Engine",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
			Category: "system",
			Provider: "arcadia",
			Version:  "1.0.0",
			Enabled:  true,
		},
		{
			Name:        "schedule_app_run",
			Description: "Schedule an application tool to run at a specific time or recurring interval",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"appId": map[string]any{
						"type":        "string",
						"description": "The ID of the application to schedule",
					},
					"toolName": map[string]any{
						"type":        "string",
						"description": "The name of the tool to execute",
					},
					"input": map[string]any{
						"type":        "object",
						"description": "Input data to pass to the tool",
					},
					"scheduleType": map[string]any{
						"type":        "string",
						"enum":        []string{"one-time", "recurring"},
						"description": "Type of schedule",
					},
					"scheduledTime": map[string]any{
						"type":        "string",
						"description": "When to run the scheduled task (ISO 8601 format or flexible datetime)",
					},
					"recurrence": map[string]any{
						"type":        "object",
						"description": "Recurrence rule for recurring schedules",
						"properties": map[string]any{
							"interval": map[string]any{
								"type":        "integer",
								"description": "Interval between executions",
							},
							"unit": map[string]any{
								"type":        "string",
								"enum":        []string{"minutes", "hours", "days", "weeks", "months"},
								"description": "Time unit for interval",
							},
						},
					},
				},
				"required": []string{"appId", "toolName", "input", "scheduleType", "scheduledTime"},
			},
			Category: "scheduling",
			Provider: "arcadia",
			Version:  "1.0.0",
			Enabled:  true,
		},
		{
			Name:        "list_schedules",
			Description: "List all scheduled application runs, optionally filtered by app ID",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"appId": map[string]any{
						"type":        "string",
						"description": "Optional app ID to filter schedules",
					},
				},
			},
			Category: "scheduling",
			Provider: "arcadia",
			Version:  "1.0.0",
			Enabled:  true,
		},
		{
			Name:        "search_documents",
			Description: "Search for documents with enhanced capabilities including complete document content, context highlights of the top 3 most relevant passages, content previews, and intelligent size management. Returns full documents with highlighted relevant passages for better AI understanding, eliminating fragmented chunk results.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search query text",
					},
					"top_k": map[string]any{
						"type":        "integer",
						"description": "Number of documents to return (default 5)",
						"default":     5,
						"minimum":     1,
						"maximum":     10,
					},
				},
				"required": []string{"query"},
			},
			Category: "search",
			Provider: "arcadia",
			Version:  "1.0.0",
			Enabled:  true,
		},
	}

	// Register static tools
	for _, tool := range staticTools {
		tm.mutex.Lock()
		tm.tools[tool.Name] = &tool
		if tm.toolStats[tool.Name] == nil {
			tm.toolStats[tool.Name] = &models.ToolStats{
				ToolName: tool.Name,
			}
		}
		tm.mutex.Unlock()
	}

	// Load dynamic app tools
	return tm.LoadDynamicTools(ctx)
}

// LoadDynamicTools loads tools from registered applications
func (tm *Manager) LoadDynamicTools(ctx context.Context) error {
	if tm.registryAccess == nil {
		if tm.logger != nil {
			tm.logger.Warn(ctx, "Registry access not configured, skipping dynamic tool loading")
		}
		return nil
	}

	registry := tm.registryAccess.GetRegistry()
	mutex := tm.registryAccess.GetRegistryMutex()

	mutex.RLock()
	defer mutex.RUnlock()

	if tm.logger != nil {
		tm.logger.Debug(ctx, "Loading dynamic tools from registry", "app_count", len(registry))
	}

	toolsLoaded := 0

	for appID, appInterface := range registry {
		// Handle *App struct
		if app, ok := appInterface.(*models.App); ok {
			for _, appTool := range app.Tools {
				toolName := fmt.Sprintf("%s_%s", appID, appTool.Name)

				// Create tool definition
				tool := models.Tool{
					Name:        toolName,
					Description: tm.buildToolDescription(appID, appTool),
					InputSchema: tm.buildInputSchema(appTool),
					Category:    tm.getToolCategory(appTool),
					Provider:    appID,
					Version:     app.Version,
					Enabled:     !tm.isToolDisabled(toolName),
					Metadata: map[string]interface{}{
						"app_id":      appID,
						"app_version": app.Version,
						"app_runtime": app.Runtime,
						"tool_name":   appTool.Name,
					},
				}

				tm.mutex.Lock()
				tm.tools[toolName] = &tool
				if tm.toolStats[toolName] == nil {
					tm.toolStats[toolName] = &models.ToolStats{
						ToolName: toolName,
					}
				}
				tm.mutex.Unlock()

				toolsLoaded++
				if tm.logger != nil {
					tm.logger.Debug(ctx, "Loaded dynamic tool", "tool_name", toolName, "app_id", appID)
				}
			}
		}
	}

	if tm.logger != nil {
		tm.logger.Info(ctx, "Loaded dynamic tools", "tools_loaded", toolsLoaded)
	}

	return nil
}

// GetToolsByCategory returns tools filtered by category
func (tm *Manager) GetToolsByCategory(ctx context.Context, category string) ([]models.Tool, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	var tools []models.Tool
	for _, tool := range tm.tools {
		if tool.Enabled && tool.Category == category {
			tools = append(tools, *tool)
		}
	}

	return tools, nil
}

// GetToolsByProvider returns tools filtered by provider
func (tm *Manager) GetToolsByProvider(ctx context.Context, provider string) ([]models.Tool, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	var tools []models.Tool
	for _, tool := range tm.tools {
		if tool.Enabled && tool.Provider == provider {
			tools = append(tools, *tool)
		}
	}

	return tools, nil
}

// IsToolEnabled checks if a tool is enabled
func (tm *Manager) IsToolEnabled(ctx context.Context, toolName string) (bool, error) {
	tool, err := tm.GetTool(ctx, toolName)
	if err != nil {
		return false, err
	}

	return tool.Enabled, nil
}

// GetToolUsageStats returns usage statistics for all tools
func (tm *Manager) GetToolUsageStats(ctx context.Context) (*models.ToolUsageStats, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	stats := &models.ToolUsageStats{
		TotalTools:  len(tm.tools),
		ActiveTools: 0,
		TopTools:    make([]*models.ToolUsage, 0),
		ToolsByCategory: make(map[string]*models.CategoryStats),
	}

	var totalExecutions int64
	var successfulExecutions int64
	var failedExecutions int64
	var totalExecutionTime float64

	for _, tool := range tm.tools {
		if tool.Enabled {
			stats.ActiveTools++
		}

		if toolStats, exists := tm.toolStats[tool.Name]; exists {
			totalExecutions += toolStats.ExecutionCount
			successfulExecutions += toolStats.SuccessCount
			failedExecutions += toolStats.FailureCount
			totalExecutionTime += toolStats.AverageRuntime * float64(toolStats.ExecutionCount)

			// Create tool usage entry
			usage := &models.ToolUsage{
				ToolName:       tool.Name,
				ExecutionCount: toolStats.ExecutionCount,
				SuccessRate:    100.0 - toolStats.ErrorRate,
				AverageRuntime: toolStats.AverageRuntime,
				LastUsed:       toolStats.LastExecution,
				Category:       tool.Category,
				Provider:       tool.Provider,
			}
			stats.TopTools = append(stats.TopTools, usage)

			// Update category stats
			if categoryStats, exists := stats.ToolsByCategory[tool.Category]; exists {
				categoryStats.ToolCount++
				categoryStats.ExecutionCount += toolStats.ExecutionCount
				categoryStats.AverageRuntime = (categoryStats.AverageRuntime + toolStats.AverageRuntime) / 2
			} else {
				stats.ToolsByCategory[tool.Category] = &models.CategoryStats{
					Category:       tool.Category,
					ToolCount:      1,
					ExecutionCount: toolStats.ExecutionCount,
					SuccessRate:    100.0 - toolStats.ErrorRate,
					AverageRuntime: toolStats.AverageRuntime,
				}
			}
		}
	}

	stats.TotalExecutions = totalExecutions
	stats.SuccessfulExecutions = successfulExecutions
	stats.FailedExecutions = failedExecutions

	if totalExecutions > 0 {
		stats.AverageExecutionTime = totalExecutionTime / float64(totalExecutions)
	}

	return stats, nil
}

// GetToolMetrics returns metrics for a specific tool
func (tm *Manager) GetToolMetrics(ctx context.Context, toolName string) (*models.ToolMetrics, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	tool, exists := tm.tools[toolName]
	if !exists {
		return nil, models.NewToolError(toolName, "tool not found", nil)
	}

	toolStats, exists := tm.toolStats[toolName]
	if !exists {
		// Return empty metrics if no stats exist
		return &models.ToolMetrics{
			ToolName: toolName,
		}, nil
	}

	metrics := &models.ToolMetrics{
		ToolName:       toolStats.ToolName,
		ExecutionCount: toolStats.ExecutionCount,
		SuccessCount:   toolStats.SuccessCount,
		FailureCount:   toolStats.FailureCount,
		AverageLatency: toolStats.AverageRuntime,
		ErrorRate:      toolStats.ErrorRate,
		LastExecution:  toolStats.LastExecution,
		Errors:         make(map[string]int64),
		Metadata: map[string]interface{}{
			"category": tool.Category,
			"provider": tool.Provider,
			"enabled":  tool.Enabled,
		},
	}

	// Calculate additional metrics
	if toolStats.ExecutionCount > 0 {
		metrics.ThroughputRPS = float64(toolStats.ExecutionCount) / time.Since(tool.Metadata["created_at"].(time.Time)).Seconds()
	}

	return metrics, nil
}

// RecordToolExecution records tool execution metrics
func (tm *Manager) RecordToolExecution(ctx context.Context, toolName string, duration float64, success bool) error {
	tm.updateToolStats(toolName, duration, success, nil)
	return nil
}

// UpdateDependencies updates the tool manager's dependencies after initialization
func (tm *Manager) UpdateDependencies(ctx context.Context, deps *interfaces.Dependencies) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Update the dependencies
	if deps.RegistryAccess != nil {
		tm.registryAccess = deps.RegistryAccess
	}
	if deps.AppRunner != nil {
		tm.appRunner = deps.AppRunner
	}
	if deps.AppCreator != nil {
		tm.appCreator = deps.AppCreator
	}
	if deps.EmbeddingSearch != nil {
		tm.embeddingSearch = deps.EmbeddingSearch
	}
	if deps.SchedulerService != nil {
		tm.schedulerService = deps.SchedulerService
	}

	if tm.logger != nil {
		tm.logger.Info(ctx, "Tool manager dependencies updated", "embedding_search_available", tm.embeddingSearch != nil)
	}

	return nil
}

// Internal helper methods

// isToolDisabled checks if a tool is disabled in the configuration
func (tm *Manager) isToolDisabled(toolName string) bool {
	if tm.config == nil || tm.config.DisabledTools == nil {
		return false
	}

	for _, disabledTool := range tm.config.DisabledTools {
		if disabledTool == toolName {
			return true
		}
	}
	return false
}

func (tm *Manager) validateTool(tool *models.Tool) error {
	if tool.Name == "" {
		return models.NewValidationError("name", "required", "tool name is required", tool.Name)
	}

	// Validate tool name format
	for _, char := range tool.Name {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '_') {
			return models.NewValidationError("name", "format", "tool name must contain only alphanumeric characters and underscores", tool.Name)
		}
	}

	if tool.Description == "" {
		return models.NewValidationError("description", "required", "tool description is required", tool.Description)
	}

	return nil
}

func (tm *Manager) isSystemTool(toolName string) bool {
	systemTools := []string{"list_apps", "schedule_app_run", "list_schedules", "search_documents", "get_document_content"}
	for _, systemTool := range systemTools {
		if toolName == systemTool {
			return true
		}
	}
	return false
}

func (tm *Manager) isAppTool(toolName string) bool {
	return strings.Contains(toolName, "_") && !tm.isSystemTool(toolName)
}

func (tm *Manager) executeSystemTool(ctx context.Context, toolName string, input any) (string, error) {
	switch toolName {
	case "list_apps":
		return tm.executeListApps(ctx)
	case "schedule_app_run":
		return tm.executeScheduleAppRun(ctx, input)
	case "list_schedules":
		return tm.executeListSchedules(ctx, input)
	case "search_documents":
		return tm.executeSearchDocuments(ctx, input)
	case "get_document_content":
		return tm.executeGetDocumentContent(ctx, input)
	default:
		return "", models.NewToolError(toolName, "unknown system tool", input)
	}
}

func (tm *Manager) executeAppTool(ctx context.Context, toolName string, input any) (string, error) {
	if tm.appRunner == nil {
		return "", models.NewConfigurationError("app_runner", "app runner not configured", nil, nil)
	}

	// Parse app tool name (format: appId_toolName)
	parts := strings.SplitN(toolName, "_", 2)
	if len(parts) != 2 {
		return "", models.NewToolError(toolName, "invalid app tool name format", input)
	}

	appID := parts[0]
	appToolName := parts[1]

	// Convert input to JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", models.NewToolError(toolName, fmt.Sprintf("failed to marshal input: %v", err), input)
	}

	// Execute the app tool
	return tm.appRunner.ExecuteAppTool(appID, appToolName, inputJSON)
}

func (tm *Manager) executeListApps(ctx context.Context) (string, error) {
	if tm.registryAccess == nil {
		return "", models.NewConfigurationError("registry_access", "registry access not configured", nil, nil)
	}

	registry := tm.registryAccess.GetRegistry()
	mutex := tm.registryAccess.GetRegistryMutex()

	mutex.RLock()
	defer mutex.RUnlock()

	result, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return "", models.NewToolError("list_apps", fmt.Sprintf("failed to marshal apps: %v", err), nil)
	}

	return string(result), nil
}

func (tm *Manager) executeScheduleAppRun(ctx context.Context, input any) (string, error) {
	if tm.schedulerService == nil {
		return "", models.NewConfigurationError("scheduler_service", "scheduler service not configured", nil, nil)
	}

	// Convert input to ScheduleRequest
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", models.NewToolError("schedule_app_run", fmt.Sprintf("failed to marshal input: %v", err), input)
	}

	var req models.ScheduleRequest
	if err := json.Unmarshal(inputJSON, &req); err != nil {
		return "", models.NewToolError("schedule_app_run", fmt.Sprintf("failed to parse schedule request: %v", err), input)
	}

	// Validate request
	if err := req.Validate(); err != nil {
		return "", models.NewToolError("schedule_app_run", fmt.Sprintf("invalid schedule request: %v", err), input)
	}

	// Create schedule
	schedule, err := tm.schedulerService.CreateSchedule(req)
	if err != nil {
		return "", models.NewToolError("schedule_app_run", fmt.Sprintf("failed to create schedule: %v", err), input)
	}

	// Return success response
	response := map[string]any{
		"status":     "schedule created successfully",
		"scheduleId": schedule.ID,
		"nextRun":    schedule.NextRun,
	}

	resultJSON, err := json.Marshal(response)
	if err != nil {
		return "", models.NewToolError("schedule_app_run", fmt.Sprintf("failed to marshal response: %v", err), input)
	}

	return string(resultJSON), nil
}

func (tm *Manager) executeListSchedules(ctx context.Context, input any) (string, error) {
	if tm.schedulerService == nil {
		return "", models.NewConfigurationError("scheduler_service", "scheduler service not configured", nil, nil)
	}

	// Parse optional appId filter
	var appIdFilter string
	if input != nil {
		if inputMap, ok := input.(map[string]any); ok {
			if appId, ok := inputMap["appId"].(string); ok {
				appIdFilter = appId
			}
		}
	}

	scheduleList := tm.schedulerService.GetAllSchedules(appIdFilter)

	result, err := json.MarshalIndent(scheduleList, "", "  ")
	if err != nil {
		return "", models.NewToolError("list_schedules", fmt.Sprintf("failed to marshal schedules: %v", err), input)
	}

	return string(result), nil
}

func (tm *Manager) executeSearchDocuments(ctx context.Context, input any) (string, error) {
	log.Printf("[DEBUG] executeSearchDocuments called with input: %+v", input)

	if tm.embeddingSearch == nil {
		log.Printf("[ERROR] embedding search service not configured")
		return "", models.NewConfigurationError("embedding_search", "embedding search service not configured", nil, nil)
	}

	// Convert input to search request
	inputJSON, err := json.Marshal(input)
	if err != nil {
		log.Printf("[ERROR] failed to marshal input: %v", err)
		return "", models.NewToolError("search_documents", fmt.Sprintf("failed to marshal input: %v", err), input)
	}
	log.Printf("[DEBUG] input JSON: %s", string(inputJSON))

	var searchReq struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}

	if err := json.Unmarshal(inputJSON, &searchReq); err != nil {
		log.Printf("[ERROR] failed to parse search request: %v", err)
		return "", models.NewToolError("search_documents", fmt.Sprintf("failed to parse search request: %v", err), input)
	}
	log.Printf("[DEBUG] parsed search request - Query: '%s', TopK: %d", searchReq.Query, searchReq.TopK)

	// Validate required fields
	if searchReq.Query == "" {
		log.Printf("[ERROR] query is empty")
		return "", models.NewToolError("search_documents", "query is required", input)
	}

	// Set default top_k if not provided
	if searchReq.TopK == 0 {
		searchReq.TopK = 5
		log.Printf("[DEBUG] set default TopK to 5")
	}

	// Validate top_k range
	if searchReq.TopK < 1 || searchReq.TopK > 10 {
		log.Printf("[ERROR] invalid TopK: %d", searchReq.TopK)
		return "", models.NewToolError("search_documents", "top_k must be between 1 and 10", input)
	}

	// Perform search
	config := models.DefaultSearchConfig() // Use default search config
	log.Printf("[DEBUG] performing search with query '%s', topK %d, config: %+v", searchReq.Query, searchReq.TopK, config)

	results, err := tm.embeddingSearch.SearchDocumentsEnhanced(searchReq.Query, searchReq.TopK, config)
	if err != nil {
		log.Printf("[ERROR] search failed: %v", err)
		return "", models.NewToolError("search_documents", fmt.Sprintf("search failed: %v", err), input)
	}
	log.Printf("[DEBUG] search completed, got %d results", len(results))

	// Format results
	var formattedResults []map[string]any
	for i, result := range results {
		log.Printf("[DEBUG] processing result %d: Document=%v, BestScore=%f", i, result.Document != nil, result.BestScore)

		if result.Document == nil {
			log.Printf("[DEBUG] skipping result %d - document is nil", i)
			continue
		}

		log.Printf("[DEBUG] result %d document - ID: %s, FilePath: %s, ContentLength: %d",
			i, result.Document.ID, result.Document.FilePath, len(result.Document.Content))

		// Use context highlights from enhanced result
		contextHighlights := result.ContextHighlights
		if len(contextHighlights) == 0 {
			// Fallback to content preview if no highlights available
			contentPreview := result.Document.Content
			if len(contentPreview) > 200 {
				contentPreview = contentPreview[:200]
			}
			contextHighlights = []string{contentPreview}
			log.Printf("[DEBUG] no context highlights, using content preview (length: %d)", len(contentPreview))
		} else {
			log.Printf("[DEBUG] using %d context highlights", len(contextHighlights))
		}

		// Create content preview
		contentPreview := result.Document.Content
		isTruncated := false
		if len(contentPreview) > 500 {
			contentPreview = contentPreview[:500]
			isTruncated = true
		}

		formattedResult := map[string]any{
			"document_id":        result.Document.ID,
			"file_path":          result.Document.FilePath,
			"full_content":       result.Document.Content,
			"context_highlights": contextHighlights,
			"content_preview":    contentPreview,
			"is_truncated":       isTruncated,
			"relevance_score":    result.BestScore,
			"relevance_rank":     result.RelevanceRank,
			"metadata":           result.Document.Metadata,
		}

		formattedResults = append(formattedResults, formattedResult)
		log.Printf("[DEBUG] added formatted result %d", i)
	}

	response := map[string]any{
		"query":           searchReq.Query,
		"documents":       formattedResults,
		"total_documents": len(formattedResults),
		"usage_note":      "Documents include full content with highlighted relevant passages for better AI understanding.",
	}

	log.Printf("[DEBUG] final response has %d documents", len(formattedResults))

	resultJSON, err := json.Marshal(response)
	if err != nil {
		log.Printf("[ERROR] failed to marshal response: %v", err)
		return "", models.NewToolError("search_documents", fmt.Sprintf("failed to marshal response: %v", err), input)
	}

	log.Printf("[DEBUG] returning JSON response (length: %d)", len(resultJSON))
	log.Printf("[DEBUG] response JSON: %s", string(resultJSON))

	return string(resultJSON), nil
}

func (tm *Manager) executeGetDocumentContent(ctx context.Context, input any) (string, error) {
	if tm.embeddingSearch == nil {
		return "", models.NewConfigurationError("embedding_search", "embedding search service not configured", nil, nil)
	}

	// Convert input to document request
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", models.NewToolError("get_document_content", fmt.Sprintf("failed to marshal input: %v", err), input)
	}

	var docReq struct {
		DocumentID string `json:"document_id"`
	}

	if err := json.Unmarshal(inputJSON, &docReq); err != nil {
		return "", models.NewToolError("get_document_content", fmt.Sprintf("failed to parse document request: %v", err), input)
	}

	// Validate required fields
	if docReq.DocumentID == "" {
		return "", models.NewToolError("get_document_content", "document_id is required", input)
	}

	// Get the document
	document, err := tm.embeddingSearch.GetDocument(docReq.DocumentID)
	if err != nil {
		return "", models.NewToolError("get_document_content", fmt.Sprintf("failed to get document: %v", err), input)
	}

	if document == nil {
		return "", models.NewToolError("get_document_content", fmt.Sprintf("document not found: %s", docReq.DocumentID), input)
	}

	response := map[string]any{
		"document_id": document.ID,
		"file_path":   document.FilePath,
		"file_hash":   document.FileHash,
		"content":     document.Content,
		"chunk_count": document.ChunkCount,
		"metadata":    document.Metadata,
		"created_at":  document.CreatedAt,
		"updated_at":  document.UpdatedAt,
	}

	resultJSON, err := json.Marshal(response)
	if err != nil {
		return "", models.NewToolError("get_document_content", fmt.Sprintf("failed to marshal response: %v", err), input)
	}

	return string(resultJSON), nil
}

func (tm *Manager) updateToolStats(toolName string, duration float64, success bool, err error) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	stats, exists := tm.toolStats[toolName]
	if !exists {
		stats = &models.ToolStats{
			ToolName: toolName,
		}
		tm.toolStats[toolName] = stats
	}

	now := time.Now()
	stats.ExecutionCount++
	stats.LastExecution = &now

	if success {
		stats.SuccessCount++
	} else {
		stats.FailureCount++
		if err != nil && !contains(stats.CommonErrors, err.Error()) {
			stats.CommonErrors = append(stats.CommonErrors, err.Error())
		}
	}

	// Update average runtime using incremental average
	if duration > 0 {
		newAverage := stats.AverageRuntime + (duration-stats.AverageRuntime)/float64(stats.ExecutionCount)
		stats.AverageRuntime = newAverage
	}

	// Calculate error rate
	if stats.ExecutionCount > 0 {
		stats.ErrorRate = float64(stats.FailureCount) / float64(stats.ExecutionCount) * 100
	}
}

func (tm *Manager) buildToolDescription(appID string, appTool models.AppTool) string {
	if appTool.Description != "" {
		return appTool.Description
	}
	return fmt.Sprintf("Execute %s tool from %s application", appTool.Name, appID)
}

func (tm *Manager) buildInputSchema(appTool models.AppTool) any {
	if appTool.Schema != nil {
		return appTool.Schema
	}

	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}

	if appTool.InputFormat != "" {
		schema["description"] = fmt.Sprintf("Input data for %s. Expected format: %s", appTool.Name, appTool.InputFormat)
	}

	return schema
}

func (tm *Manager) getToolCategory(appTool models.AppTool) string {
	if appTool.Category != "" {
		return appTool.Category
	}
	return "app"
}

func (tm *Manager) calculateInputSize(input any) int64 {
	if input == nil {
		return 0
	}

	data, err := json.Marshal(input)
	if err != nil {
		return 0
	}

	return int64(len(data))
}

func (tm *Manager) publishEvent(ctx context.Context, event *models.Event) {
	if tm.eventBus != nil {
		if err := tm.eventBus.PublishAsync(ctx, event); err != nil {
			if tm.logger != nil {
				tm.logger.Error(ctx, "Failed to publish tool event", "event_type", event.Type, "error", err)
			}
		}
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