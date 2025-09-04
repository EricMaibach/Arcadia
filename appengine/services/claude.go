package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// --- Claude Configuration ---

type ClaudeConfig struct {
	APIKey              string `json:"api_key"`
	BaseURL             string `json:"base_url"`
	Model               string `json:"model"`
	MaxTokens           int    `json:"max_tokens"`
	TimeoutSeconds      int    `json:"timeout_seconds"`
	EnableMCP           bool   `json:"enable_mcp"`
	MCPServerCmd        string `json:"mcp_server_cmd"`
	MaxContextMessages  int    `json:"max_context_messages"`  // Max number of messages to retain in context
	ContextCompaction   bool   `json:"context_compaction"`    // Enable smart context compaction
	ContextTTLMinutes   int    `json:"context_ttl_minutes"`   // TTL for context in minutes (0 = no expiry)
}

// --- Claude API structures ---

type ClaudeMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or array of content blocks
}

type ClaudeTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

type ClaudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []ClaudeMessage `json:"messages"`
	Tools     []ClaudeTool    `json:"tools,omitempty"`
}

type ClaudeContent struct {
	Type  string      `json:"type"`
	Text  string      `json:"text,omitempty"`
	ID    string      `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
	Input interface{} `json:"input,omitempty"`
}

type ClaudeResponse struct {
	Content    []ClaudeContent `json:"content"`
	StopReason string          `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// --- Context Management ---

type ConversationContext struct {
	Messages     []ClaudeMessage `json:"messages"`
	LastAccessed time.Time       `json:"last_accessed"`
	TotalTokens  int             `json:"total_tokens"`
}

type ContextManager struct {
	contexts map[string]*ConversationContext // Key is contextID (e.g., "wasm:<appID>", "web:<sessionID>")
	mutex    sync.RWMutex
	config   *ClaudeConfig
}

// --- Claude Service ---

type ClaudeService struct {
	config         ClaudeConfig
	httpClient     *http.Client
	mcpTools       []ClaudeTool
	contextManager *ContextManager
}

// Global service instance (will be set by dependency injection)
var claudeServiceInstance *ClaudeService

// Dependency injection interfaces
type RegistryAccess interface {
	GetRegistry() map[string]interface{}
	GetRegistryMutex() *sync.RWMutex
}

type AppRunner interface {
	ExecuteAppTool(appID, toolName string, input json.RawMessage) (string, error)
}

type AppCreator interface {
	CreateApp(appID, version, runtime string, tools []interface{}, appSrc string, dependencies map[string]string) (string, error)
}

var registryAccess RegistryAccess
var appRunner AppRunner
var appCreator AppCreator

func SetRegistryAccess(ra RegistryAccess) {
	registryAccess = ra
}

func SetAppRunner(ar AppRunner) {
	appRunner = ar
}

func SetAppCreator(ac AppCreator) {
	appCreator = ac
}

func NewClaudeService(config ClaudeConfig) *ClaudeService {
	// Set defaults for context configuration
	if config.MaxContextMessages == 0 {
		config.MaxContextMessages = 20 // Default to retaining 20 messages
	}
	if config.ContextTTLMinutes == 0 {
		config.ContextTTLMinutes = 60 // Default to 1 hour TTL
	}
	
	httpTimeout := time.Duration(config.TimeoutSeconds) * time.Second
	log.Printf("[Claude Service] HTTP client timeout set to: %v", httpTimeout)
	
	service := &ClaudeService{
		config: config,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
		mcpTools: []ClaudeTool{},
		contextManager: &ContextManager{
			contexts: make(map[string]*ConversationContext),
			config:   &config,
		},
	}
	
	if config.EnableMCP {
		service.loadMCPTools()
	}
	
	// Set the global instance
	claudeServiceInstance = service
	
	// Start background goroutine to clean up expired contexts
	if config.ContextTTLMinutes > 0 {
		go service.contextCleanupRoutine()
	}
	
	return service
}

// GetClaudeService returns the global Claude service instance
func GetClaudeService() *ClaudeService {
	return claudeServiceInstance
}

// --- Context Manager Methods ---

func (cm *ContextManager) GetOrCreateContext(contextID string) *ConversationContext {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	context, exists := cm.contexts[contextID]
	if !exists {
		context = &ConversationContext{
			Messages:     []ClaudeMessage{},
			LastAccessed: time.Now(),
			TotalTokens:  0,
		}
		cm.contexts[contextID] = context
	}
	
	context.LastAccessed = time.Now()
	return context
}

func (cm *ContextManager) AddMessage(contextID string, message ClaudeMessage) *ConversationContext {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	context, exists := cm.contexts[contextID]
	if !exists {
		context = &ConversationContext{
			Messages:     []ClaudeMessage{},
			LastAccessed: time.Now(),
			TotalTokens:  0,
		}
		cm.contexts[contextID] = context
	}
	
	context.Messages = append(context.Messages, message)
	context.LastAccessed = time.Now()
	
	// Trim context if needed
	if cm.config.MaxContextMessages > 0 && len(context.Messages) > cm.config.MaxContextMessages {
		if cm.config.ContextCompaction {
			cm.compactContext(context)
		} else {
			// Simple trimming: keep only the most recent messages
			start := len(context.Messages) - cm.config.MaxContextMessages
			context.Messages = context.Messages[start:]
		}
	}
	
	return context
}

func (cm *ContextManager) compactContext(context *ConversationContext) {
	// Smart compaction strategy:
	// 1. Keep the first message (initial context)
	// 2. Keep the last N-1 messages in full
	// 3. Summarize middle messages if needed
	
	maxMessages := cm.config.MaxContextMessages
	if len(context.Messages) <= maxMessages {
		return
	}
	
	// Keep first message and last (maxMessages-1) messages
	keepRecent := maxMessages - 1
	if keepRecent < 1 {
		keepRecent = 1
	}
	
	firstMsg := context.Messages[0]
	recentMsgs := context.Messages[len(context.Messages)-keepRecent:]
	
	// For now, simple strategy: just keep first and recent
	// In production, you might want to summarize the middle messages
	context.Messages = append([]ClaudeMessage{firstMsg}, recentMsgs...)
}

func (cm *ContextManager) UpdateTokenCount(contextID string, additionalTokens int) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	if context, exists := cm.contexts[contextID]; exists {
		context.TotalTokens += additionalTokens
	}
}

func (cm *ContextManager) ClearContext(contextID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	delete(cm.contexts, contextID)
}

func (cm *ContextManager) GetContext(contextID string) (*ConversationContext, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	
	context, exists := cm.contexts[contextID]
	return context, exists
}

func (cm *ContextManager) CleanupExpiredContexts(ttlMinutes int) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	if ttlMinutes <= 0 {
		return
	}
	
	expiry := time.Now().Add(-time.Duration(ttlMinutes) * time.Minute)
	
	for id, context := range cm.contexts {
		if context.LastAccessed.Before(expiry) {
			delete(cm.contexts, id)
			log.Printf("[Claude Context] Expired context: %s", id)
		}
	}
}

// Background cleanup routine
func (cs *ClaudeService) contextCleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute) // Run every 5 minutes
	defer ticker.Stop()
	
	for range ticker.C {
		cs.contextManager.CleanupExpiredContexts(cs.config.ContextTTLMinutes)
	}
}

// Public methods for managing contexts

func (cs *ClaudeService) ClearContext(contextID string) {
	cs.contextManager.ClearContext(contextID)
}

func (cs *ClaudeService) GetContextStats(contextID string) (messages int, tokens int, exists bool) {
	context, exists := cs.contextManager.GetContext(contextID)
	if !exists {
		return 0, 0, false
	}
	return len(context.Messages), context.TotalTokens, true
}

func (cs *ClaudeService) SendMessage(message string) (string, error) {
	// Use a default context ID for backward compatibility
	return cs.SendMessageWithContext(message, "default")
}

func (cs *ClaudeService) SendMessageWithContext(message string, contextID string) (string, error) {
	// Get or create context
	_ = cs.contextManager.GetOrCreateContext(contextID)
	
	// Add the new user message to context
	userMessage := ClaudeMessage{
		Role:    "user",
		Content: message,
	}
	cs.contextManager.AddMessage(contextID, userMessage)
	
	// Call Claude API with context
	return cs.callClaudeWithContext(contextID)
}

func (cs *ClaudeService) callClaudeWithContext(contextID string) (string, error) {
	return cs.callClaudeWithContextInternal(contextID, 0)
}

func (cs *ClaudeService) callClaudeWithContextInternal(contextID string, depth int) (string, error) {
	// Get context
	context, exists := cs.contextManager.GetContext(contextID)
	if !exists {
		return "", fmt.Errorf("context not found: %s", contextID)
	}
	
	// Build the request with full context
	request := ClaudeRequest{
		Model:     cs.config.Model,
		MaxTokens: cs.config.MaxTokens,
		Messages:  context.Messages,
		Tools:     cs.mcpTools,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", cs.config.BaseURL+"/v1/messages", strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cs.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	log.Printf("[Claude Service] Making request with timeout: %v", cs.httpClient.Timeout)
	resp, err := cs.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Claude API error (status %d): %s", resp.StatusCode, string(body))
	}

	var claudeResp ClaudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("no content in Claude response")
	}

	// Build assistant message content (may include both text and tool use)
	var assistantContent []interface{}
	var responseText string
	var hasToolUse bool
	
	for _, content := range claudeResp.Content {
		if content.Type == "text" {
			responseText = content.Text
			assistantContent = append(assistantContent, map[string]interface{}{
				"type": "text",
				"text": content.Text,
			})
		} else if content.Type == "tool_use" {
			hasToolUse = true
			assistantContent = append(assistantContent, map[string]interface{}{
				"type":  "tool_use",
				"id":    content.ID,
				"name":  content.Name,
				"input": content.Input,
			})
		}
	}
	
	// Add assistant's response to context (including tool use)
	if len(assistantContent) > 0 {
		// Store as content array for tool use, string for plain text
		var messageContent interface{}
		if hasToolUse {
			messageContent = assistantContent
		} else if responseText != "" {
			messageContent = responseText
		}
		
		if messageContent != nil {
			assistantMessage := ClaudeMessage{
				Role:    "assistant",
				Content: messageContent,
			}
			cs.contextManager.AddMessage(contextID, assistantMessage)
		}
		
		// Update token count if available
		if claudeResp.Usage.InputTokens > 0 || claudeResp.Usage.OutputTokens > 0 {
			cs.contextManager.UpdateTokenCount(contextID, claudeResp.Usage.InputTokens+claudeResp.Usage.OutputTokens)
		}
	}

	// If Claude wants to use tools, handle them and continue conversation
	if hasToolUse || claudeResp.StopReason == "tool_use" {
		return cs.handleToolUseWithLoopInternal(claudeResp, contextID, depth)
	}

	return responseText, nil
}

func (cs *ClaudeService) loadMCPTools() {
	// Define tools directly in Go, no longer using Python MCP server
	cs.mcpTools = []ClaudeTool{
		{
			Name:        "list_apps",
			Description: "List all registered applications in the Arcadia App Engine",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "run_app",
			Description: "Execute a tool from a registered application",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"app_id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the application to run",
					},
					"tool_name": map[string]interface{}{
						"type":        "string",
						"description": "The name of the tool to execute",
					},
					"input_data": map[string]interface{}{
						"type":        "object",
						"description": "Input data to pass to the tool",
					},
				},
				"required": []string{"app_id", "tool_name", "input_data"},
			},
		},
		{
			Name:        "create_app",
			Description: "Submit a Rust trait implementation to compile and register a new WASM application",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"appId": map[string]interface{}{
						"type":        "string",
						"description": "Unique identifier for the application",
					},
					"version": map[string]interface{}{
						"type":        "string",
						"description": "Version of the application",
					},
					"runtime": map[string]interface{}{
						"type":        "string",
						"description": "Runtime for the application (must be 'wasm')",
					},
					"tools": map[string]interface{}{
						"type": "array",
						"description": "Array of tool definitions",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"name": map[string]interface{}{
									"type":        "string",
									"description": "Name of the tool",
								},
								"input_format": map[string]interface{}{
									"type":        "string",
									"description": "Input format (json, xml, etc.)",
								},
							},
							"required": []string{"name", "input_format"},
						},
					},
					"appSrc": map[string]interface{}{
						"type":        "string",
						"description": "Rust trait implementation source code",
					},
					"dependencies": map[string]interface{}{
						"type":        "object",
						"description": "Optional Rust crate dependencies to include in Cargo.toml (e.g., {\"chrono\": \"0.4\", \"regex\": \"1.9\"}). Common dependencies are auto-detected from 'use' statements.",
						"additionalProperties": map[string]interface{}{
							"type":        "string",
							"description": "Version specification for the crate",
						},
					},
				},
				"required": []string{"appId", "version", "runtime", "tools", "appSrc"},
			},
		},
		{
			Name:        "schedule_app_run",
			Description: "Schedule an application tool to run at a specific time or recurring interval",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"appId": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the application to schedule",
					},
					"toolName": map[string]interface{}{
						"type":        "string",
						"description": "The name of the tool to execute",
					},
					"input": map[string]interface{}{
						"type":        "object",
						"description": "Input data to pass to the tool",
					},
					"scheduleType": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"one-time", "recurring"},
						"description": "Type of schedule",
					},
					"scheduledTime": map[string]interface{}{
						"type":        "string",
						"description": "When to run the scheduled task (ISO 8601 format or flexible datetime)",
					},
					"recurrence": map[string]interface{}{
						"type":        "object",
						"description": "Recurrence rule for recurring schedules",
						"properties": map[string]interface{}{
							"interval": map[string]interface{}{
								"type":        "integer",
								"description": "Interval between executions",
							},
							"unit": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"minutes", "hours", "days", "weeks", "months"},
								"description": "Time unit for interval",
							},
							"daysOfWeek": map[string]interface{}{
								"type":        "array",
								"description": "Days of week for weekly recurrence (0=Sunday, 6=Saturday)",
								"items": map[string]interface{}{
									"type":    "integer",
									"minimum": 0,
									"maximum": 6,
								},
							},
							"endDate": map[string]interface{}{
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
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"appId": map[string]interface{}{
						"type":        "string",
						"description": "Optional app ID to filter schedules",
					},
				},
			},
		},
	}
}

func (cs *ClaudeService) handleToolUseWithLoop(response ClaudeResponse, contextID string) (string, error) {
	return cs.handleToolUseWithLoopInternal(response, contextID, 0)
}

func (cs *ClaudeService) handleToolUseWithLoopInternal(response ClaudeResponse, contextID string, depth int) (string, error) {
	// Prevent infinite loops - limit recursion depth
	const maxDepth = 10
	if depth >= maxDepth {
		log.Printf("[Claude MCP] Warning: Max tool use depth reached (%d), stopping recursion", maxDepth)
		// Return tool results as final response to avoid infinite loop
		var results []string
		for _, content := range response.Content {
			if content.Type == "tool_use" {
				result, err := cs.executeMCPTool(content.Name, content.Input)
				if err != nil {
					results = append(results, fmt.Sprintf("Tool %s failed: %v", content.Name, err))
				} else {
					results = append(results, fmt.Sprintf("Tool %s result: %s", content.Name, result))
				}
			}
		}
		return strings.Join(results, "\n\n"), nil
	}
	
	var toolResults []map[string]interface{}
	
	log.Printf("[Claude MCP] Handling tool use response with %d content items (depth: %d)", len(response.Content), depth)
	
	// Execute each tool and collect results
	for _, content := range response.Content {
		if content.Type == "tool_use" {
			log.Printf("[Claude MCP] Executing tool: %s", content.Name)
			result, err := cs.executeMCPTool(content.Name, content.Input)
			
			// Create tool result message
			toolResult := map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": content.ID,
			}
			
			if err != nil {
				toolResult["is_error"] = true
				toolResult["content"] = fmt.Sprintf("Tool execution failed: %v", err)
				log.Printf("[Claude MCP] Tool %s failed: %v", content.Name, err)
			} else {
				toolResult["content"] = result
				log.Printf("[Claude MCP] Tool %s succeeded, result length: %d", content.Name, len(result))
			}
			
			toolResults = append(toolResults, toolResult)
		}
	}
	
	if len(toolResults) == 0 {
		return "", fmt.Errorf("no tool use content found")
	}
	
	log.Printf("[Claude MCP] Sending %d tool results back to Claude", len(toolResults))
	
	// Add tool results to context as a user message with content array
	toolResultMessage := ClaudeMessage{
		Role:    "user",
		Content: toolResults,
	}
	cs.contextManager.AddMessage(contextID, toolResultMessage)
	
	// Call Claude again with the tool results to get the final response
	log.Printf("[Claude MCP] Calling Claude again with tool results...")
	return cs.callClaudeWithContextInternal(contextID, depth+1)
}

func (cs *ClaudeService) executeMCPTool(toolName string, input interface{}) (string, error) {
	log.Printf("[Claude MCP] Executing tool: %s with input: %+v", toolName, input)
	
	return cs.executeMCPToolDirect(toolName, input)
}

func (cs *ClaudeService) executeMCPToolDirect(toolName string, input interface{}) (string, error) {
	switch toolName {
	case "list_apps":
		return cs.executeListApps()
	case "run_app":
		return cs.executeRunApp(input)
	case "create_app":
		return cs.executeCreateApp(input)
	case "schedule_app_run":
		return cs.executeScheduleAppRun(input)
	case "list_schedules":
		return cs.executeListSchedules(input)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (cs *ClaudeService) executeListApps() (string, error) {
	if registryAccess == nil {
		return "", fmt.Errorf("registry access not configured")
	}
	
	registry := registryAccess.GetRegistry()
	mutex := registryAccess.GetRegistryMutex()
	
	mutex.RLock()
	defer mutex.RUnlock()
	
	result, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal apps: %w", err)
	}
	
	return string(result), nil
}

func (cs *ClaudeService) executeRunApp(input interface{}) (string, error) {
	if appRunner == nil {
		return "", fmt.Errorf("app runner not configured")
	}
	
	// Convert input to expected structure
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}
	
	var runReq struct {
		AppID     string          `json:"app_id"`
		ToolName  string          `json:"tool_name"`
		InputData json.RawMessage `json:"input_data"`
	}
	
	if err := json.Unmarshal(inputJSON, &runReq); err != nil {
		return "", fmt.Errorf("failed to parse run request: %w", err)
	}
	
	// Execute the app tool
	return appRunner.ExecuteAppTool(runReq.AppID, runReq.ToolName, runReq.InputData)
}

func (cs *ClaudeService) executeCreateApp(input interface{}) (string, error) {
	// Convert input to expected structure
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}
	
	var createReq struct {
		AppID   string `json:"appId"`
		Version string `json:"version"`
		Runtime string `json:"runtime"`
		Tools   []struct {
			Name        string `json:"name"`
			InputFormat string `json:"input_format"`
		} `json:"tools"`
		AppSrc       string            `json:"appSrc"`
		Dependencies map[string]string `json:"dependencies,omitempty"`
	}
	
	if err := json.Unmarshal(inputJSON, &createReq); err != nil {
		return "", fmt.Errorf("failed to parse create app request: %w", err)
	}
	
	// Validate required fields
	if createReq.AppID == "" {
		return "", fmt.Errorf("appId is required")
	}
	if createReq.Version == "" {
		return "", fmt.Errorf("version is required")
	}
	if createReq.Runtime != "wasm" {
		return "", fmt.Errorf("runtime must be 'wasm'")
	}
	if createReq.AppSrc == "" {
		return "", fmt.Errorf("appSrc is required")
	}
	if len(createReq.Tools) == 0 {
		return "", fmt.Errorf("at least one tool is required")
	}
	
	// Use dependency injection to create the app
	if appCreator != nil {
		// Convert tools to interface{} slice
		tools := make([]interface{}, len(createReq.Tools))
		for i, tool := range createReq.Tools {
			tools[i] = map[string]interface{}{
				"name":         tool.Name,
				"input_format": tool.InputFormat,
			}
		}
		
		return appCreator.CreateApp(createReq.AppID, createReq.Version, createReq.Runtime, tools, createReq.AppSrc, createReq.Dependencies)
	}
	
	return fmt.Sprintf("App creation request received for %s (version %s)", createReq.AppID, createReq.Version), nil
}

func (cs *ClaudeService) executeScheduleAppRun(input interface{}) (string, error) {
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
	response := map[string]interface{}{
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

func (cs *ClaudeService) executeListSchedules(input interface{}) (string, error) {
	// Parse optional appId filter
	var appIdFilter string
	if input != nil {
		if inputMap, ok := input.(map[string]interface{}); ok {
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

func (cs *ClaudeService) HandleClaudeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Message   string `json:"message"`
		SessionID string `json:"session_id"` // Optional session ID from web client
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Generate context ID for web users
	contextID := "web:default"
	if request.SessionID != "" {
		contextID = "web:" + request.SessionID
	}

	response, err := cs.SendMessageWithContext(request.Message, contextID)
	if err != nil {
		log.Printf("Claude API error: %v", err)
		http.Error(w, "Failed to get Claude response", http.StatusInternalServerError)
		return
	}

	// Include context stats in response
	messages, tokens, _ := cs.GetContextStats(contextID)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": response,
		"context_stats": map[string]int{
			"message_count": messages,
			"total_tokens":  tokens,
		},
	})
}

