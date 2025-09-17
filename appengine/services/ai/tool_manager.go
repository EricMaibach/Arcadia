package ai

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Dependency injection interfaces (copied from claude.go)
type RegistryAccess interface {
	GetRegistry() map[string]any
	GetRegistryMutex() *sync.RWMutex
}

type AppRunner interface {
	ExecuteAppTool(appID, toolName string, input json.RawMessage) (string, error)
}

type AppCreator interface {
	CreateApp(appID, version, runtime string, tools []any, appSrc string, dependencies map[string]string) (string, error)
}

type EmbeddingSearch interface {
	SearchDocuments(query string, topK int) ([]*DocumentSearchResult, error)
	SearchDocumentsEnhanced(query string, topK int, config SearchConfig) ([]*EnhancedDocumentSearchResult, error)
	GetDocument(documentID string) (*Document, error)
}

// ToolManager manages tools for AI services including MCP tools and dynamic app tools
type ToolManager struct {
	config          *AIConfig
	tools           []Tool
	registryAccess  RegistryAccess
	appRunner       AppRunner
	appCreator      AppCreator
	embeddingSearch EmbeddingSearch
	mutex           sync.RWMutex
}

// NewToolManager creates a new tool manager with the given configuration
func NewToolManager(config *AIConfig) *ToolManager {
	return &ToolManager{
		config: config,
		tools:  []Tool{},
	}
}

// SetRegistryAccess sets the registry access for dynamic app tools
func (tm *ToolManager) SetRegistryAccess(ra RegistryAccess) {
	tm.registryAccess = ra
}

// SetAppRunner sets the app runner for executing app tools
func (tm *ToolManager) SetAppRunner(ar AppRunner) {
	tm.appRunner = ar
}

// SetAppCreator sets the app creator for creating new apps
func (tm *ToolManager) SetAppCreator(ac AppCreator) {
	tm.appCreator = ac
}

// SetEmbeddingSearch sets the embedding search service for document search
func (tm *ToolManager) SetEmbeddingSearch(es EmbeddingSearch) {
	tm.embeddingSearch = es
}

// LoadMCPTools loads static MCP tools and dynamic app tools
func (tm *ToolManager) LoadMCPTools() {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Start with static tools
	staticTools := []Tool{
		{
			Name:        "list_apps",
			Description: "List all registered applications in the Arcadia App Engine",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
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
							"daysOfWeek": map[string]any{
								"type":        "array",
								"description": "Days of week for weekly recurrence (0=Sunday, 6=Saturday)",
								"items": map[string]any{
									"type":    "integer",
									"minimum": 0,
									"maximum": 6,
								},
							},
							"endDate": map[string]any{
								"type":        "string",
								"description": "End date for recurrence (ISO 8601 format)",
							},
						},
					},
				},
				"required": []string{"appId", "toolName", "input", "scheduleType", "scheduledTime"},
			},
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
		},
	}

	// Add dynamic tools from registered apps
	tm.tools = append(staticTools, tm.LoadDynamicAppTools()...)
}

// LoadDynamicAppTools loads tools from registered applications
func (tm *ToolManager) LoadDynamicAppTools() []Tool {
	var dynamicTools []Tool

	// Get registry access
	if tm.registryAccess == nil {
		log.Printf("[AI Tools] Warning: registryAccess is nil, no dynamic tools will be loaded")
		return dynamicTools
	}

	registry := tm.registryAccess.GetRegistry()
	mutex := tm.registryAccess.GetRegistryMutex()

	mutex.RLock()
	defer mutex.RUnlock()

	log.Printf("[AI Tools] Loading dynamic tools from registry with %d registered apps", len(registry))

	// Iterate through all registered apps
	for appID, appInterface := range registry {
		// Try to handle as *App struct first (the actual format)
		if app, ok := appInterface.(*App); ok {
			log.Printf("[AI Tools] App '%s' has %d tools", appID, len(app.Tools))
			for _, tool := range app.Tools {
				// Create namespaced tool name
				namespacedName := fmt.Sprintf("%s_%s", appID, tool.Name)

				// Extract description and input format
				description := fmt.Sprintf("Execute %s tool from %s application", tool.Name, appID)

				// Create input schema from inputFormat if available
				inputSchema := map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				}

				if tool.InputFormat != "" {
					// Try to parse the input format as JSON schema
					// For now, use a generic object schema with description
					inputSchema = map[string]any{
						"type":        "object",
						"description": fmt.Sprintf("Input data for %s. Expected format: %s", tool.Name, tool.InputFormat),
						"properties":  map[string]any{},
					}
				}

				// Create the dynamic tool
				dynamicTool := Tool{
					Name:        namespacedName,
					Description: description,
					InputSchema: inputSchema,
				}

				log.Printf("[AI Tools] Created dynamic tool: %s", namespacedName)
				dynamicTools = append(dynamicTools, dynamicTool)
			}
		} else if appData, ok := appInterface.(map[string]any); ok {
			// Fallback: handle as map[string]any (for backward compatibility)
			if toolsInterface, exists := appData["tools"]; exists {
				if tools, ok := toolsInterface.([]any); ok {
					log.Printf("[AI Tools] App '%s' has %d tools (map format)", appID, len(tools))
					for _, toolInterface := range tools {
						if tool, ok := toolInterface.(map[string]any); ok {
							// Extract tool information
							toolName, hasName := tool["name"].(string)
							if !hasName {
								log.Printf("[AI Tools] Skipping tool in app '%s' - no name found", appID)
								continue
							}

							// Create namespaced tool name
							namespacedName := fmt.Sprintf("%s_%s", appID, toolName)

							// Extract description and input format
							description := fmt.Sprintf("Execute %s tool from %s application", toolName, appID)
							if desc, ok := tool["description"].(string); ok && desc != "" {
								description = desc
							}

							// Create input schema from inputFormat if available
							inputSchema := map[string]any{
								"type":       "object",
								"properties": map[string]any{},
							}

							if inputFormat, ok := tool["inputFormat"].(string); ok && inputFormat != "" {
								// Try to parse the input format as JSON schema
								// For now, use a generic object schema with description
								inputSchema = map[string]any{
									"type":        "object",
									"description": fmt.Sprintf("Input data for %s. Expected format: %s", toolName, inputFormat),
									"properties":  map[string]any{},
								}
							}

							// Create the dynamic tool
							dynamicTool := Tool{
								Name:        namespacedName,
								Description: description,
								InputSchema: inputSchema,
							}

							log.Printf("[AI Tools] Created dynamic tool: %s", namespacedName)
							dynamicTools = append(dynamicTools, dynamicTool)
						} else {
							log.Printf("[AI Tools] Skipping invalid tool in app '%s' - not a map", appID)
						}
					}
				} else {
					log.Printf("[AI Tools] App '%s' tools field is not an array", appID)
				}
			} else {
				log.Printf("[AI Tools] App '%s' has no tools field", appID)
			}
		} else {
			log.Printf("[AI Tools] App '%s' data is neither *App nor map (type: %T)", appID, appInterface)
		}
	}

	log.Printf("[AI Tools] Loaded %d dynamic tools total", len(dynamicTools))

	return dynamicTools
}

// RefreshTools refreshes the tools list
func (tm *ToolManager) RefreshTools() {
	if tm.config.EnableMCP {
		tm.LoadMCPTools()
	}
}

// GetTools returns the current list of tools
func (tm *ToolManager) GetTools() []Tool {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	return tm.tools
}

// ExecuteTool executes a tool with the given name and input
func (tm *ToolManager) ExecuteTool(toolName string, input any) (string, error) {
	log.Printf("[AI Tools] Executing tool: %s with input: %+v", toolName, input)

	switch toolName {
	case "list_apps":
		return tm.ExecuteSystemTool(toolName, input)
	case "create_app":
		return tm.ExecuteSystemTool(toolName, input)
	case "schedule_app_run":
		return tm.ExecuteSystemTool(toolName, input)
	case "list_schedules":
		return tm.ExecuteSystemTool(toolName, input)
	case "search_documents":
		return tm.ExecuteSystemTool(toolName, input)
	case "get_document_content":
		return tm.ExecuteSystemTool(toolName, input)
	default:
		// Check if this is a dynamic app tool (format: appId_toolName)
		if strings.Contains(toolName, "_") {
			parts := strings.SplitN(toolName, "_", 2)
			if len(parts) == 2 {
				appID := parts[0]
				appToolName := parts[1]
				return tm.ExecuteDynamicAppTool(appID, appToolName, input)
			}
		}
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

// ExecuteSystemTool executes a system tool
func (tm *ToolManager) ExecuteSystemTool(toolName string, input any) (string, error) {
	switch toolName {
	case "list_apps":
		return tm.executeListApps()
	case "create_app":
		return tm.executeCreateApp(input)
	case "schedule_app_run":
		return tm.executeScheduleAppRun(input)
	case "list_schedules":
		return tm.executeListSchedules(input)
	case "search_documents":
		return tm.executeSearchDocuments(input)
	case "get_document_content":
		return tm.executeGetDocumentContent(input)
	default:
		return "", fmt.Errorf("unknown system tool: %s", toolName)
	}
}

// ExecuteDynamicAppTool executes a dynamic app tool
func (tm *ToolManager) ExecuteDynamicAppTool(appID, toolName string, input any) (string, error) {
	if tm.appRunner == nil {
		return "", fmt.Errorf("app runner not configured")
	}

	// Convert input to JSON for the app runner
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}

	// Execute the app tool directly
	return tm.appRunner.ExecuteAppTool(appID, toolName, inputJSON)
}

// System tool implementations
func (tm *ToolManager) executeListApps() (string, error) {
	if tm.registryAccess == nil {
		return "", fmt.Errorf("registry access not configured")
	}

	registry := tm.registryAccess.GetRegistry()
	mutex := tm.registryAccess.GetRegistryMutex()

	mutex.RLock()
	defer mutex.RUnlock()

	result, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal apps: %w", err)
	}

	return string(result), nil
}

func (tm *ToolManager) executeCreateApp(input any) (string, error) {
	log.Printf("[AI Tools] executeCreateApp called with input type: %T", input)

	// Convert input to expected structure
	inputJSON, err := json.Marshal(input)
	if err != nil {
		log.Printf("[AI Tools] Failed to marshal input: %v", err)
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}
	log.Printf("[AI Tools] Input JSON: %s", string(inputJSON))

	var createReq struct {
		AppID   string `json:"appId"`
		Version string `json:"version"`
		Runtime string `json:"runtime"`
		Tools   []struct {
			Name        string `json:"name"`
			InputFormat string `json:"inputFormat"`
		} `json:"tools"`
		AppSrc       string            `json:"appSrc"`
		Dependencies map[string]string `json:"dependencies,omitempty"`
	}

	if err := json.Unmarshal(inputJSON, &createReq); err != nil {
		log.Printf("[AI Tools] Failed to parse create app request: %v", err)
		return "", fmt.Errorf("failed to parse create app request: %w", err)
	}

	log.Printf("[AI Tools] Parsed request - AppID: %s, Version: %s, Runtime: %s, Tools: %d, Dependencies: %d",
		createReq.AppID, createReq.Version, createReq.Runtime, len(createReq.Tools), len(createReq.Dependencies))

	// Validate required fields
	if createReq.AppID == "" {
		log.Printf("[AI Tools] Validation failed: appId is empty")
		return "", fmt.Errorf("appId is required")
	}
	if createReq.Version == "" {
		log.Printf("[AI Tools] Validation failed: version is empty")
		return "", fmt.Errorf("version is required")
	}
	if createReq.Runtime != "wasm" {
		log.Printf("[AI Tools] Validation failed: runtime is '%s', expected 'wasm'", createReq.Runtime)
		return "", fmt.Errorf("runtime must be 'wasm'")
	}
	if createReq.AppSrc == "" {
		log.Printf("[AI Tools] Validation failed: appSrc is empty")
		return "", fmt.Errorf("appSrc is required")
	}
	if len(createReq.Tools) == 0 {
		log.Printf("[AI Tools] Validation failed: no tools provided")
		return "", fmt.Errorf("at least one tool is required")
	}

	log.Printf("[AI Tools] All validations passed, checking appCreator dependency")

	// Use dependency injection to create the app
	if tm.appCreator != nil {
		log.Printf("[AI Tools] appCreator is available, proceeding with app creation")

		// Convert tools to any slice
		tools := make([]any, len(createReq.Tools))
		for i, tool := range createReq.Tools {
			tools[i] = map[string]any{
				"name":        tool.Name,
				"inputFormat": tool.InputFormat,
			}
		}

		log.Printf("[AI Tools] Calling appCreator.CreateApp with %d tools", len(tools))
		result, err := tm.appCreator.CreateApp(createReq.AppID, createReq.Version, createReq.Runtime, tools, createReq.AppSrc, createReq.Dependencies)
		if err != nil {
			log.Printf("[AI Tools] appCreator.CreateApp failed: %v", err)
			return "", fmt.Errorf("app creation failed: %w", err)
		}

		log.Printf("[AI Tools] App creation successful: %s", result)
		return result, nil
	}

	log.Printf("[AI Tools] appCreator is nil - dependency injection not properly configured")
	return fmt.Sprintf("App creation request received for %s (version %s)", createReq.AppID, createReq.Version), nil
}

func (tm *ToolManager) executeScheduleAppRun(input any) (string, error) {
	// Convert input to ScheduleRequest
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}

	var req ScheduleRequest
	if err := json.Unmarshal(inputJSON, &req); err != nil {
		return "", fmt.Errorf("failed to parse schedule request: %w", err)
	}

	// Validate request
	if req.AppID == "" {
		return "", fmt.Errorf("appId is required")
	}
	if req.ToolName == "" {
		return "", fmt.Errorf("toolName is required")
	}
	if req.ScheduleType != ScheduleTypeOneTime && req.ScheduleType != ScheduleTypeRecurring {
		return "", fmt.Errorf("scheduleType must be 'one-time' or 'recurring'")
	}
	if req.ScheduledTime.Time.IsZero() {
		return "", fmt.Errorf("scheduledTime is required")
	}
	if req.ScheduledTime.Time.Before(time.Now()) {
		return "", fmt.Errorf("scheduledTime must be in the future")
	}

	// Create schedule using services
	schedule, err := CreateSchedule(req)
	if err != nil {
		return "", fmt.Errorf("failed to create schedule: %w", err)
	}

	// Return success response
	response := map[string]any{
		"status":     "schedule created successfully",
		"scheduleId": schedule.ID,
		"nextRun":    schedule.NextRun,
	}

	resultJSON, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(resultJSON), nil
}

func (tm *ToolManager) executeListSchedules(input any) (string, error) {
	// Parse optional appId filter
	var appIdFilter string
	if input != nil {
		if inputMap, ok := input.(map[string]any); ok {
			if appId, ok := inputMap["appId"].(string); ok {
				appIdFilter = appId
			}
		}
	}

	scheduleList := GetAllSchedules(appIdFilter)

	result, err := json.MarshalIndent(scheduleList, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal schedules: %w", err)
	}

	return string(result), nil
}

func (tm *ToolManager) executeGetDocumentContent(input any) (string, error) {
	if tm.embeddingSearch == nil {
		return "", fmt.Errorf("embedding search service not configured")
	}

	// Convert input to expected structure
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}

	var docReq struct {
		DocumentID string `json:"document_id"`
	}

	if err := json.Unmarshal(inputJSON, &docReq); err != nil {
		return "", fmt.Errorf("failed to parse document request: %w", err)
	}

	// Validate required fields
	if docReq.DocumentID == "" {
		return "", fmt.Errorf("document_id is required")
	}

	// Get the document with reconstructed content
	document, err := tm.embeddingSearch.GetDocument(docReq.DocumentID)
	if err != nil {
		return "", fmt.Errorf("failed to get document: %w", err)
	}

	if document == nil {
		return "", fmt.Errorf("document not found: %s", docReq.DocumentID)
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
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(resultJSON), nil
}

func (tm *ToolManager) executeSearchDocuments(input any) (string, error) {
	if tm.embeddingSearch == nil {
		return "", fmt.Errorf("embedding search service not configured")
	}

	// Convert input to expected structure
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}

	var searchReq struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}

	if err := json.Unmarshal(inputJSON, &searchReq); err != nil {
		return "", fmt.Errorf("failed to parse search request: %w", err)
	}

	// Validate required fields
	if searchReq.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	// Set default top_k if not provided
	if searchReq.TopK == 0 {
		searchReq.TopK = 5
	}

	// Validate top_k range (lower max to prevent rate limiting)
	if searchReq.TopK < 1 || searchReq.TopK > 10 {
		return "", fmt.Errorf("top_k must be between 1 and 10")
	}

	// Try enhanced search first, fallback to basic search
	results, err := tm.embeddingSearch.SearchDocuments(searchReq.Query, searchReq.TopK)
	if err != nil {
		return "", fmt.Errorf("fallback search failed: %w", err)
	}

	// Format results for AI with complete document information
	var formattedResults []map[string]any
	for _, result := range results {
		if result.Document == nil {
			continue
		}

		// Format chunks for this document as context highlights
		var contextHighlights []string
		maxHighlights := 3
		for i, chunk := range result.Chunks {
			if i >= maxHighlights {
				break
			}
			contextHighlights = append(contextHighlights, chunk.Content)
		}

		// Create content preview (first 500 chars)
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
	}

	response := map[string]any{
		"query":           searchReq.Query,
		"documents":       formattedResults,
		"total_documents": len(formattedResults),
		"usage_note":      "Documents include full content with highlighted relevant passages for better AI understanding.",
	}

	resultJSON, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(resultJSON), nil
}
