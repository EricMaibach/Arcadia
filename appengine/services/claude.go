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
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go"
)

// --- Claude Configuration ---

type ClaudeConfig struct {
	APIKey         string `json:"api_key"`
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	MaxTokens      int    `json:"max_tokens"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	EnableMCP      bool   `json:"enable_mcp"`
	MCPServerCmd   string `json:"mcp_server_cmd"`
}

// --- Claude API structures ---

type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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

// --- Claude Service ---

type ClaudeService struct {
	config     ClaudeConfig
	httpClient *http.Client
	mcpTools   []ClaudeTool
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
	CreateApp(appID, version, runtime string, tools []interface{}, appSrc string) (string, error)
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
	service := &ClaudeService{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.TimeoutSeconds) * time.Second,
		},
		mcpTools: []ClaudeTool{},
	}
	
	if config.EnableMCP {
		service.loadMCPTools()
	}
	
	// Set the global instance
	claudeServiceInstance = service
	
	return service
}

// GetClaudeService returns the global Claude service instance
func GetClaudeService() *ClaudeService {
	return claudeServiceInstance
}

func (cs *ClaudeService) SendMessage(message string) (string, error) {
	request := ClaudeRequest{
		Model:     cs.config.Model,
		MaxTokens: cs.config.MaxTokens,
		Messages: []ClaudeMessage{
			{
				Role:    "user",
				Content: message,
			},
		},
		Tools: cs.mcpTools,
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

	// Check if Claude wants to use tools
	if claudeResp.StopReason == "tool_use" {
		return cs.handleToolUse(claudeResp, message)
	}

	// Return the first text content
	for _, content := range claudeResp.Content {
		if content.Type == "text" {
			return content.Text, nil
		}
	}

	return "", fmt.Errorf("no text content in Claude response")
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

func (cs *ClaudeService) handleToolUse(response ClaudeResponse, originalMessage string) (string, error) {
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
	
	if len(results) == 0 {
		return "", fmt.Errorf("no tool use content found")
	}
	
	return strings.Join(results, "\n\n"), nil
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
		AppSrc string `json:"appSrc"`
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
		
		return appCreator.CreateApp(createReq.AppID, createReq.Version, createReq.Runtime, tools, createReq.AppSrc)
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
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	response, err := cs.SendMessage(request.Message)
	if err != nil {
		log.Printf("Claude API error: %v", err)
		http.Error(w, "Failed to get Claude response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"response": response})
}

// ClaudeQuery is the WASM host function for Claude AI integration
func ClaudeQuery(caller *wasmtime.Caller, messagePtr, messageLen, resultPtrPtr int32) int32 {
	// Extract message from WASM memory
	message := ""
	if mem := caller.GetExport("memory").Memory(); mem != nil {
		data := mem.UnsafeData(caller)
		if int(messagePtr+messageLen) <= len(data) {
			message = string(data[messagePtr : messagePtr+messageLen])
		} else {
			log.Printf("[WASM Claude] Invalid memory range")
			return -3 // Invalid memory range
		}
	}

	log.Printf("[WASM Claude] claudeQuery called with message: %s", message)

	// Check if Claude service is available
	if claudeServiceInstance == nil {
		log.Printf("[WASM Claude] Claude service not initialized")
		return -1 // Claude service not initialized
	}

	// Send message to Claude
	response, err := claudeServiceInstance.SendMessage(message)
	if err != nil {
		log.Printf("[WASM Claude] Claude API error: %v", err)
		return -2 // Claude API error
	}

	// Create response JSON
	responseJSON := map[string]string{
		"response": response,
	}
	jsonBytes, err := json.Marshal(responseJSON)
	if err != nil {
		log.Printf("[WASM Claude] JSON marshal error: %v", err)
		return -4 // JSON marshal error
	}

	// Allocate memory in WASM for the response
	if mem := caller.GetExport("memory").Memory(); mem != nil {
		allocFunc := caller.GetExport("allocate").Func()
		if allocFunc == nil {
			log.Printf("[WASM Claude] Memory allocation error: allocate function not found")
			return -5 // Memory allocation error
		}
		
		results, err := allocFunc.Call(caller, int32(len(jsonBytes)))
		if err != nil {
			log.Printf("[WASM Claude] Memory allocation failed: %v", err)
			return -5
		}
		
		resultVals, ok := results.([]wasmtime.Val)
		if !ok || len(resultVals) == 0 {
			log.Printf("[WASM Claude] Invalid allocation result")
			return -5
		}
		
		ptr := resultVals[0].I32()
		data := mem.UnsafeData(caller)
		copy(data[ptr:ptr+int32(len(jsonBytes))], jsonBytes)
		
		// Write the pointer to the result
		*(*int32)(unsafe.Pointer(&data[resultPtrPtr])) = ptr
	}

	log.Printf("[WASM Claude] Claude query completed successfully")
	return int32(len(jsonBytes))
}