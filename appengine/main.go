package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bytecodealliance/wasmtime-go"
	_ "github.com/mattn/go-sqlite3"
)

// --- Data structures ---

type File struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type ToolInfo struct {
	Name        string `json:"name"`
	InputFormat string `json:"inputFormat"`
}

type App struct {
	AppID          string     `json:"appId"`
	Version        string     `json:"version"`
	Runtime        string     `json:"runtime"`
	Tools          []ToolInfo `json:"tools"`
	ArtifactURI    string     `json:"artifactUri"`
	SourceLanguage string     `json:"sourceLanguage,omitempty"`
	Files          []File     `json:"files,omitempty"`
}

type ScheduleType string

const (
	ScheduleTypeOneTime   ScheduleType = "one-time"
	ScheduleTypeRecurring ScheduleType = "recurring"
)

type RecurrenceRule struct {
	Interval   int        `json:"interval"`             // Number of units between runs
	Unit       string     `json:"unit"`                 // "minutes", "hours", "days", "weeks", "months"
	DaysOfWeek []int      `json:"daysOfWeek,omitempty"` // For weekly: 0=Sunday, 1=Monday, etc.
	EndDate    *time.Time `json:"endDate,omitempty"`    // Optional end date for recurring schedules
}

type AppSchedule struct {
	ID            string          `json:"id"`
	AppID         string          `json:"appId"`
	ToolName      string          `json:"toolName"`
	Input         json.RawMessage `json:"input"`
	ScheduleType  ScheduleType    `json:"scheduleType"`
	ScheduledTime time.Time       `json:"scheduledTime"`
	Recurrence    *RecurrenceRule `json:"recurrence,omitempty"`
	IsActive      bool            `json:"isActive"`
	CreatedAt     time.Time       `json:"createdAt"`
	LastRun       *time.Time      `json:"lastRun,omitempty"`
	NextRun       *time.Time      `json:"nextRun,omitempty"`
	RunCount      int             `json:"runCount"`
}

type ScheduleRequest struct {
	AppID         string          `json:"appId"`
	ToolName      string          `json:"toolName"`
	Input         json.RawMessage `json:"input"`
	ScheduleType  ScheduleType    `json:"scheduleType"`
	ScheduledTime FlexibleTime    `json:"scheduledTime"`
	Recurrence    *RecurrenceRule `json:"recurrence,omitempty"`
}

// FlexibleTime handles multiple datetime formats
type FlexibleTime struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler for flexible datetime parsing
func (ft *FlexibleTime) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}

	// Remove quotes from JSON string
	timeStr := strings.Trim(string(b), `"`)

	// Try multiple common time formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z0700",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
		"01/02/2006 15:04:05",
		"01/02/2006 15:04",
		"01/02/2006",
		"02-01-2006 15:04:05",
		"02-01-2006 15:04",
		"02-01-2006",
		"Jan 2, 2006 3:04:05 PM",
		"Jan 2, 2006 15:04:05",
		"Jan 2, 2006 3:04 PM",
		"Jan 2, 2006 15:04",
		"Jan 2, 2006",
		"January 2, 2006 3:04:05 PM",
		"January 2, 2006 15:04:05",
		"January 2, 2006 3:04 PM",
		"January 2, 2006 15:04",
		"January 2, 2006",
		"2006/01/02 15:04:05",
		"2006/01/02 15:04",
		"2006/01/02",
	}

	var lastErr error
	for _, format := range formats {
		if parsedTime, err := time.ParseInLocation(format, timeStr, time.Local); err == nil {
			ft.Time = parsedTime
			return nil
		} else {
			lastErr = err
		}
	}

	return fmt.Errorf("unable to parse time '%s' using any supported format: %v", timeStr, lastErr)
}

type ScheduledRun struct {
	ID          string          `json:"id"`
	ScheduleID  string          `json:"scheduleId"`
	AppID       string          `json:"appId"`
	ToolName    string          `json:"toolName"`
	Input       json.RawMessage `json:"input"`
	StartedAt   time.Time       `json:"startedAt"`
	CompletedAt *time.Time      `json:"completedAt,omitempty"`
	Status      string          `json:"status"` // "running", "completed", "failed"
	Output      string          `json:"output,omitempty"`
	Error       string          `json:"error,omitempty"`
}

// --- Configuration structures ---

type ClaudeConfig struct {
	APIKey         string `json:"api_key"`
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	MaxTokens      int    `json:"max_tokens"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	EnableMCP      bool   `json:"enable_mcp"`
	MCPServerCmd   string `json:"mcp_server_cmd"`
}

type ServerConfig struct {
	Port string `json:"port"`
	Host string `json:"host"`
}

type Config struct {
	Claude ClaudeConfig `json:"claude"`
	Server ServerConfig `json:"server"`
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
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
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
	
	return service
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
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"name": map[string]interface{}{
									"type":        "string",
									"description": "The name of the tool",
								},
								"inputFormat": map[string]interface{}{
									"type":        "string",
									"description": "The expected JSON structure for this tool's input",
								},
							},
							"required": []string{"name", "inputFormat"},
						},
						"description": "List of tools this application provides",
					},
					"appSrc": map[string]interface{}{
						"type":        "string",
						"description": "Rust code implementing the ArcadiaApp trait",
					},
				},
				"required": []string{"appId", "version", "runtime", "tools", "appSrc"},
			},
		},
		{
			Name:        "schedule_app_run",
			Description: "Schedule an app tool to run at a specific time, either once or on a recurring basis",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"appId": map[string]interface{}{
						"type":        "string",
						"description": "ID of the application to schedule",
					},
					"toolName": map[string]interface{}{
						"type":        "string",
						"description": "Name of the tool to execute",
					},
					"input": map[string]interface{}{
						"type":        "object",
						"description": "Input data to pass to the tool",
					},
					"scheduleType": map[string]interface{}{
						"type":        "string",
						"description": "Type of schedule: 'one-time' or 'recurring'",
					},
					"scheduledTime": map[string]interface{}{
						"type":        "string",
						"format":      "date-time",
						"description": "When to run the tool (ISO 8601 format)",
					},
					"recurrence": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"interval": map[string]interface{}{
								"type":        "integer",
								"description": "Number of units between runs",
							},
							"unit": map[string]interface{}{
								"type":        "string",
								"description": "Unit of time for recurrence",
							},
							"daysOfWeek": map[string]interface{}{
								"type":        "array",
								"description": "Days of week for weekly recurrence (0=Sunday, 1=Monday, etc.)",
							},
							"endDate": map[string]interface{}{
								"type":        "string",
								"format":      "date-time",
								"description": "Optional end date for recurring schedules",
							},
						},
						"description": "Recurrence pattern (required for recurring schedules)",
					},
				},
				"required": []string{"appId", "toolName", "input", "scheduleType", "scheduledTime"},
			},
		},
		{
			Name:        "list_schedules",
			Description: "List all scheduled app runs",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"appId": map[string]interface{}{
						"type":        "string",
						"description": "Optional: filter schedules by app ID",
					},
				},
			},
		},
	}
	
	log.Printf("Loaded %d MCP tools directly in Go", len(cs.mcpTools))
}






func (cs *ClaudeService) handleToolUse(response ClaudeResponse, originalMessage string) (string, error) {
	var toolResults []string
	
	// Execute each tool call
	for _, content := range response.Content {
		if content.Type == "tool_use" {
			result, err := cs.executeMCPTool(content.Name, content.Input)
			if err != nil {
				toolResults = append(toolResults, fmt.Sprintf("Tool %s failed: %v", content.Name, err))
			} else {
				toolResults = append(toolResults, fmt.Sprintf("Tool %s result: %s", content.Name, result))
			}
		}
	}
	
	if len(toolResults) == 0 {
		return "I tried to use tools but no tool results were available.", nil
	}
	
	// Return the combined results
	return strings.Join(toolResults, "\n\n"), nil
}

func (cs *ClaudeService) executeMCPTool(toolName string, input interface{}) (string, error) {
	// Execute tools directly using Go methods
	log.Printf("Executing MCP tool: %s", toolName)
	return cs.executeMCPToolDirect(toolName, input)
}



func (cs *ClaudeService) executeMCPToolDirect(toolName string, input interface{}) (string, error) {
	// Execute tools directly using local Go functions instead of HTTP calls
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

// executeListApps handles the list_apps tool
func (cs *ClaudeService) executeListApps() (string, error) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	apps := []*App{}
	for _, app := range registry {
		apps = append(apps, app)
	}
	
	result, err := json.MarshalIndent(apps, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal apps: %w", err)
	}
	
	return string(result), nil
}

// executeRunApp handles the run_app tool
func (cs *ClaudeService) executeRunApp(input interface{}) (string, error) {
	// Parse input
	inputMap, ok := input.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid input format for run_app")
	}
	
	appID, ok := inputMap["app_id"].(string)
	if !ok {
		return "", fmt.Errorf("app_id is required")
	}
	
	toolName, ok := inputMap["tool_name"].(string)
	if !ok {
		return "", fmt.Errorf("tool_name is required")
	}
	
	inputData := inputMap["input_data"]
	inputJSON, err := json.Marshal(inputData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input_data: %w", err)
	}
	
	// Execute the app tool
	output, err := executeAppTool(appID, toolName, json.RawMessage(inputJSON))
	if err != nil {
		return "", fmt.Errorf("failed to execute app tool: %w", err)
	}
	
	// Return formatted result
	result := map[string]interface{}{
		"output": output,
		"status": "success",
	}
	
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	
	return string(resultJSON), nil
}

// executeCreateApp handles the create_app tool
func (cs *ClaudeService) executeCreateApp(input interface{}) (string, error) {
	// Convert input to AppRequest
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}
	
	var req AppRequest
	if err := json.Unmarshal(inputJSON, &req); err != nil {
		return "", fmt.Errorf("failed to parse app request: %w", err)
	}
	
	// Validate required fields
	if strings.TrimSpace(req.AppSrc) == "" {
		return "", fmt.Errorf("trait implementation is required")
	}
	if req.AppID == "" {
		return "", fmt.Errorf("appId is required")
	}
	if req.Version == "" {
		return "", fmt.Errorf("version is required")
	}
	if len(req.Tools) == 0 {
		return "", fmt.Errorf("at least one tool is required")
	}
	
	// Check for existing compiled WASM
	artifactsDir := filepath.Join("artifacts", req.AppID)
	wasmFilename := fmt.Sprintf("%s.wasm", req.Version)
	finalWasmPath := filepath.Join(artifactsDir, wasmFilename)
	
	if _, err := os.Stat(finalWasmPath); err == nil {
		return "", fmt.Errorf("app %s version %s already compiled and exists", req.AppID, req.Version)
	}
	
	// Prepare build directory
	buildDir := filepath.Join("build", req.AppID, req.Version)
	if _, err := os.Stat(buildDir); err == nil {
		if err := os.RemoveAll(buildDir); err != nil {
			return "", fmt.Errorf("failed to clear existing build directory: %w", err)
		}
	}
	
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create build directory: %w", err)
	}
	
	// Generate Rust project from trait
	if err := generateRustProjectFromTrait(req, buildDir); err != nil {
		os.RemoveAll(buildDir)
		return "", fmt.Errorf("failed to generate Rust project: %w", err)
	}
	
	// Compile to WASM
	wasmPath, err := buildRustToWasm(buildDir)
	if err != nil {
		return "", fmt.Errorf("failed to compile Rust to WASM: %w", err)
	}
	
	// Register app in registry
	registryMutex.Lock()
	registry[req.AppID] = &App{
		AppID:          req.AppID,
		Version:        req.Version,
		Runtime:        req.Runtime,
		Tools:          req.Tools,
		ArtifactURI:    wasmPath,
		SourceLanguage: "rust",
		Files: []File{
			{
				Name:    "src/lib.rs",
				Content: "Generated from trait implementation",
			},
		},
	}
	registryMutex.Unlock()
	
	// Save registry to file
	if err := saveRegistry(); err != nil {
		log.Printf("Warning: failed to save registry: %v", err)
	}
	
	// Return success response
	response := map[string]string{
		"status":   "app trait implementation submitted and compiled",
		"wasmPath": wasmPath,
	}
	
	resultJSON, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}
	
	return string(resultJSON), nil
}

// executeScheduleAppRun handles the schedule_app_run tool
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
	
	// Verify app exists and tool is valid
	registryMutex.RLock()
	app, ok := registry[req.AppID]
	registryMutex.RUnlock()
	if !ok {
		return "", fmt.Errorf("app not found: %s", req.AppID)
	}
	
	toolFound := false
	for _, tool := range app.Tools {
		if tool.Name == req.ToolName {
			toolFound = true
			break
		}
	}
	if !toolFound {
		return "", fmt.Errorf("tool '%s' not found in app", req.ToolName)
	}
	
	// Generate schedule ID and create schedule
	scheduleID, err := generateID()
	if err != nil {
		return "", fmt.Errorf("failed to generate schedule ID: %w", err)
	}
	
	now := time.Now()
	schedule := &AppSchedule{
		ID:            scheduleID,
		AppID:         req.AppID,
		ToolName:      req.ToolName,
		Input:         req.Input,
		ScheduleType:  req.ScheduleType,
		ScheduledTime: req.ScheduledTime.Time,
		Recurrence:    req.Recurrence,
		IsActive:      true,
		CreatedAt:     now,
		RunCount:      0,
	}
	
	nextRun := req.ScheduledTime.Time
	schedule.NextRun = &nextRun
	
	// Store schedule in database
	if err := saveScheduleToDatabase(schedule); err != nil {
		return "", fmt.Errorf("failed to save schedule: %w", err)
	}
	
	// Store schedule in memory
	schedulesMutex.Lock()
	schedules[scheduleID] = schedule
	schedulesMutex.Unlock()
	
	// Return success response
	response := map[string]interface{}{
		"status":     "schedule created successfully",
		"scheduleId": scheduleID,
		"nextRun":    schedule.NextRun,
	}
	
	resultJSON, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}
	
	return string(resultJSON), nil
}

// executeListSchedules handles the list_schedules tool
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
	
	schedulesMutex.RLock()
	defer schedulesMutex.RUnlock()
	
	scheduleList := []*AppSchedule{}
	for _, schedule := range schedules {
		// Apply app ID filter if specified
		if appIdFilter != "" && schedule.AppID != appIdFilter {
			continue
		}
		scheduleList = append(scheduleList, schedule)
	}
	
	result, err := json.MarshalIndent(scheduleList, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal schedules: %w", err)
	}
	
	return string(result), nil
}

// --- REST API Handler for future use ---

func (cs *ClaudeService) HandleClaudeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	response, err := cs.SendMessage(req.Message)
	if err != nil {
		log.Printf("Claude API error: %v", err)
		http.Error(w, "Failed to get Claude response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"response": response})
}

const registryFilePath = "app_registry.json"

var (
	registry      = make(map[string]*App)
	registryMutex sync.RWMutex
	wasmEngine    = wasmtime.NewEngine()

	// App database - available to WASM apps for their data
	appDB      *sql.DB
	appDBMutex sync.RWMutex

	// System database - for app engine internal data (schedules, etc.)
	systemDB      *sql.DB
	systemDBMutex sync.RWMutex

	appLogger *log.Logger
	
	// Configuration and Claude service
	appConfig     Config
	claudeService *ClaudeService
	logFile   *os.File

	// Scheduler globals
	schedules       = make(map[string]*AppSchedule)
	schedulesMutex  sync.RWMutex
	schedulerCtx    context.Context
	schedulerCancel context.CancelFunc
)

// --- Logging Setup ---

func initAppLogger() error {
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Create log file with timestamp
	logFileName := fmt.Sprintf("logs/app_submissions_%s.log", time.Now().Format("2006-01-02"))
	var err error
	logFile, err = os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to create log file: %v", err)
	}

	// Create logger that writes to both file and stdout
	appLogger = log.New(io.MultiWriter(os.Stdout, logFile), "[APP_SUBMISSION] ", log.LstdFlags|log.Lmicroseconds)

	appLogger.Println("=== App Submission Logger Initialized ===")
	return nil
}

func closeAppLogger() {
	if logFile != nil {
		appLogger.Println("=== App Submission Logger Closing ===")
		logFile.Close()
	}
}

// Safe logging helper
func logAppSubmission(format string, args ...interface{}) {
	if appLogger != nil {
		appLogger.Printf(format, args...)
	}
}

// --- REST Handlers ---

func listAppsHandler(w http.ResponseWriter, r *http.Request) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	apps := []*App{}
	for _, app := range registry {
		apps = append(apps, app)
	}
	json.NewEncoder(w).Encode(apps)
}

type RunToolRequest struct {
	AppID    string          `json:"appId"`
	ToolName string          `json:"toolName"`
	Input    json.RawMessage `json:"input"`
}

func runToolHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := fmt.Sprintf("tool_session_%d", time.Now().UnixNano())
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = forwarded
	}

	logAppSubmission("=== NEW TOOL EXECUTION REQUEST [%s] ===", sessionID)
	logAppSubmission("[%s] Client IP: %s", sessionID, clientIP)
	logAppSubmission("[%s] Request Method: %s", sessionID, r.Method)
	logAppSubmission("[%s] Request URL: %s", sessionID, r.URL.String())
	logAppSubmission("[%s] User-Agent: %s", sessionID, r.Header.Get("User-Agent"))
	logAppSubmission("[%s] Content-Type: %s", sessionID, r.Header.Get("Content-Type"))

	logAppSubmission("[%s] STEP 1: Parsing tool execution request", sessionID)
	var req RunToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logAppSubmission("[%s] ERROR: Failed to decode request body: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Invalid JSON", sessionID)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	logAppSubmission("[%s] Parsed request - AppID: %s, ToolName: %s, InputLength: %d", sessionID, req.AppID, req.ToolName, len(req.Input))

	logAppSubmission("[%s] STEP 2: Looking up app in registry", sessionID)
	registryMutex.RLock()
	app, ok := registry[req.AppID]
	registryMutex.RUnlock()
	if !ok {
		logAppSubmission("[%s] ERROR: App not found in registry: %s", sessionID, req.AppID)
		logAppSubmission("[%s] RESPONSE: HTTP 404 - App not found", sessionID)
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}
	logAppSubmission("[%s] App found - Version: %s, Runtime: %s, ArtifactURI: %s", sessionID, app.Version, app.Runtime, app.ArtifactURI)

	// Validate that the requested tool exists in the app
	toolFound := false
	for _, tool := range app.Tools {
		if tool.Name == req.ToolName {
			toolFound = true
			logAppSubmission("[%s] Tool '%s' found with input format: %s", sessionID, req.ToolName, tool.InputFormat)
			break
		}
	}
	if !toolFound {
		logAppSubmission("[%s] ERROR: Tool '%s' not found in app. Available tools: %v", sessionID, req.ToolName, app.Tools)
		logAppSubmission("[%s] RESPONSE: HTTP 404 - Tool not found", sessionID)
		http.Error(w, fmt.Sprintf("tool '%s' not found in app", req.ToolName), http.StatusNotFound)
		return
	}

	logAppSubmission("[%s] STEP 3: Loading WASM module from: %s", sessionID, app.ArtifactURI)
	wasmBytes, err := os.ReadFile(app.ArtifactURI)
	if err != nil {
		logAppSubmission("[%s] ERROR: Failed to load WASM artifact: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Artifact load failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to load artifact: %v", err), http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] WASM module loaded successfully (%d bytes)", sessionID, len(wasmBytes))

	logAppSubmission("[%s] STEP 4: Compiling WASM module", sessionID)
	store := wasmtime.NewStore(wasmEngine)
	module, err := wasmtime.NewModule(wasmEngine, wasmBytes)
	if err != nil {
		logAppSubmission("[%s] ERROR: Failed to compile WASM module: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Module compilation failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to compile module: %v", err), http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] WASM module compiled successfully", sessionID)

	logAppSubmission("[%s] STEP 5: Setting up WASM linker and host functions", sessionID)
	linker := wasmtime.NewLinker(wasmEngine)

	// Define database host functions that WASM can call
	linker.DefineFunc(store, "env", "db_query", func(caller *wasmtime.Caller, queryPtr, queryLen, resultPtrPtr int32) int32 {
		return dbQuery(caller, queryPtr, queryLen, resultPtrPtr)
	})

	linker.DefineFunc(store, "env", "db_exec", func(caller *wasmtime.Caller, stmtPtr, stmtLen int32) int32 {
		return dbExec(caller, stmtPtr, stmtLen)
	})

	linker.DefineFunc(store, "env", "db_prepared_query", func(caller *wasmtime.Caller, stmtPtr, stmtLen, paramsPtr, paramsLen, resultPtrPtr int32) int32 {
		return dbPreparedQuery(caller, stmtPtr, stmtLen, paramsPtr, paramsLen, resultPtrPtr)
	})

	// Define Claude AI host function that WASM can call
	linker.DefineFunc(store, "env", "claude_query", func(caller *wasmtime.Caller, messagePtr, messageLen, resultPtrPtr int32) int32 {
		return claudeQuery(caller, messagePtr, messageLen, resultPtrPtr)
	})

	logAppSubmission("[%s] STEP 6: Instantiating WASM module", sessionID)
	instance, err := linker.Instantiate(store, module)
	if err != nil {
		logAppSubmission("[%s] ERROR: Failed to instantiate WASM module: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Module instantiation failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to instantiate module: %v", err), http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] WASM module instantiated successfully", sessionID)

	logAppSubmission("[%s] STEP 7: Verifying required WASM functions", sessionID)
	runFunc := instance.GetFunc(store, "run")
	if runFunc == nil {
		logAppSubmission("[%s] ERROR: WASM module does not export 'run' function", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Missing run function", sessionID)
		http.Error(w, "module does not export 'run' function", http.StatusInternalServerError)
		return
	}

	// Get WASM functions
	allocateFunc := instance.GetFunc(store, "allocate")
	deallocateFunc := instance.GetFunc(store, "deallocate")
	getResultPtrFunc := instance.GetFunc(store, "get_result_ptr")

	if allocateFunc == nil || deallocateFunc == nil || getResultPtrFunc == nil {
		logAppSubmission("[%s] ERROR: WASM module missing required functions (allocate, deallocate, get_result_ptr)", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Missing required functions", sessionID)
		http.Error(w, "module missing required functions (allocate, deallocate, get_result_ptr)", http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] All required WASM functions found", sessionID)

	logAppSubmission("[%s] STEP 8: Preparing input for WASM execution", sessionID)
	// Prepare input string - transform to the format WASM expects
	wasmInput := map[string]interface{}{
		"tool": req.ToolName,
		"data": req.Input,
	}
	inputBytes, _ := json.Marshal(wasmInput)
	inputStr := string(inputBytes)
	inputLen := len(inputStr)
	logAppSubmission("[%s] Input prepared - Tool: %s, InputLength: %d, Input: %s", sessionID, req.ToolName, inputLen, inputStr)

	// Allocate memory in WASM for input
	logAppSubmission("[%s] Allocating WASM memory for input (%d bytes)", sessionID, inputLen)
	inputPtrResult, err := allocateFunc.Call(store, inputLen)
	if err != nil {
		logAppSubmission("[%s] ERROR: Failed to allocate WASM input memory: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Memory allocation failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to allocate input memory: %v", err), http.StatusInternalServerError)
		return
	}
	inputPtr := inputPtrResult.(int32)
	logAppSubmission("[%s] Input memory allocated at pointer: %d", sessionID, inputPtr)

	// Get WASM memory and copy input string
	logAppSubmission("[%s] Copying input data to WASM memory", sessionID)
	memory := instance.GetExport(store, "memory").Memory()
	data := memory.UnsafeData(store)
	copy(data[inputPtr:inputPtr+int32(inputLen)], []byte(inputStr))

	// Call WASM 'run' function with pointer and length
	logAppSubmission("[%s] STEP 9: Executing WASM 'run' function", sessionID)
	startTime := time.Now()
	resultLenResult, err := runFunc.Call(store, inputPtr, inputLen)
	executionTime := time.Since(startTime)
	if err != nil {
		// Clean up allocated memory
		deallocateFunc.Call(store, inputPtr, inputLen)
		logAppSubmission("[%s] ERROR: WASM execution failed after %v: %v", sessionID, executionTime, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Execution failed", sessionID)
		http.Error(w, fmt.Sprintf("execution failed: %v", err), http.StatusInternalServerError)
		return
	}
	resultLen := resultLenResult.(int32)
	logAppSubmission("[%s] WASM execution completed successfully in %v, result length: %d", sessionID, executionTime, resultLen)

	logAppSubmission("[%s] STEP 10: Retrieving execution results", sessionID)
	// Get result pointer
	resultPtrResult, err := getResultPtrFunc.Call(store)
	if err != nil {
		deallocateFunc.Call(store, inputPtr, inputLen)
		logAppSubmission("[%s] ERROR: Failed to get result pointer: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Result retrieval failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to get result pointer: %v", err), http.StatusInternalServerError)
		return
	}
	resultPtr := resultPtrResult.(int32)
	logAppSubmission("[%s] Result pointer: %d", sessionID, resultPtr)

	// Read result from WASM memory
	resultBytes := make([]byte, resultLen)
	copy(resultBytes, data[resultPtr:resultPtr+resultLen])
	resultStr := string(resultBytes)
	logAppSubmission("[%s] Result retrieved (%d bytes), result is: %s", sessionID, len(resultBytes), resultStr)

	// Clean up allocated input memory
	logAppSubmission("[%s] Cleaning up allocated input memory", sessionID)
	deallocateFunc.Call(store, inputPtr, inputLen)

	// Return output
	logAppSubmission("[%s] STEP 11: Sending success response", sessionID)
	output := map[string]interface{}{
		"output": resultStr,
		"status": "success",
	}

	outputJSON, _ := json.Marshal(output)
	logAppSubmission("[%s] Response prepared (%d bytes)", sessionID, len(outputJSON))

	json.NewEncoder(w).Encode(output)
	logAppSubmission("[%s] === TOOL EXECUTION COMPLETED SUCCESSFULLY ===", sessionID)
	logAppSubmission("[%s] Total execution time: %v", sessionID, time.Since(startTime))
}

type AppRequest struct {
	AppID   string     `json:"appId"`
	Version string     `json:"version"`
	Runtime string     `json:"runtime"`
	Tools   []ToolInfo `json:"tools"`
	AppSrc  string     `json:"appSrc"`
}

func submitAppSrcHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = forwarded
	}

	logAppSubmission("=== NEW APP SUBMISSION REQUEST [%s] ===", sessionID)
	logAppSubmission("[%s] Client IP: %s", sessionID, clientIP)
	logAppSubmission("[%s] Request Method: %s", sessionID, r.Method)
	logAppSubmission("[%s] Request URL: %s", sessionID, r.URL.String())
	logAppSubmission("[%s] User-Agent: %s", sessionID, r.Header.Get("User-Agent"))
	logAppSubmission("[%s] Content-Type: %s", sessionID, r.Header.Get("Content-Type"))

	// Step 1: Parse and validate request
	logAppSubmission("[%s] STEP 1: Parsing request body", sessionID)
	var req AppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logAppSubmission("[%s] ERROR: Failed to decode request body: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Invalid JSON", sessionID)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Log the parsed request (sanitized)
	reqJSON, _ := json.MarshalIndent(map[string]interface{}{
		"appId":        req.AppID,
		"version":      req.Version,
		"runtime":      req.Runtime,
		"tools":        req.Tools,
		"appSrcLength": len(req.AppSrc),
		"appSrcPreview": func() string {
			if len(req.AppSrc) > 100 {
				return req.AppSrc[:100] + "..."
			}
			return req.AppSrc
		}(),
	}, "", "  ")
	logAppSubmission("[%s] Parsed request: %s", sessionID, string(reqJSON))

	// Step 2: Validate required fields
	logAppSubmission("[%s] STEP 2: Validating request fields", sessionID)
	if strings.TrimSpace(req.AppSrc) == "" {
		logAppSubmission("[%s] ERROR: AppSrc field is empty or whitespace only", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Missing app source", sessionID)
		http.Error(w, "trait implementation is required", http.StatusBadRequest)
		return
	}

	if req.AppID == "" {
		logAppSubmission("[%s] ERROR: AppID field is empty", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Missing AppID", sessionID)
		http.Error(w, "appId is required", http.StatusBadRequest)
		return
	}

	if req.Version == "" {
		logAppSubmission("[%s] ERROR: Version field is empty", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Missing Version", sessionID)
		http.Error(w, "version is required", http.StatusBadRequest)
		return
	}

	if len(req.Tools) == 0 {
		logAppSubmission("[%s] ERROR: Tools array is empty", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Missing tools", sessionID)
		http.Error(w, "at least one tool is required", http.StatusBadRequest)
		return
	}

	// Validate each tool has both name and input format
	for i, tool := range req.Tools {
		if strings.TrimSpace(tool.Name) == "" {
			logAppSubmission("[%s] ERROR: Tool %d has empty name", sessionID, i)
			logAppSubmission("[%s] RESPONSE: HTTP 400 - Invalid tool name", sessionID)
			http.Error(w, fmt.Sprintf("tool %d: name is required", i), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(tool.InputFormat) == "" {
			logAppSubmission("[%s] ERROR: Tool %d (%s) has empty input format", sessionID, i, tool.Name)
			logAppSubmission("[%s] RESPONSE: HTTP 400 - Invalid tool input format", sessionID)
			http.Error(w, fmt.Sprintf("tool %d (%s): inputFormat is required", i, tool.Name), http.StatusBadRequest)
			return
		}
	}

	logAppSubmission("[%s] Validation passed - AppID: %s, Version: %s, Tools: %v", sessionID, req.AppID, req.Version, req.Tools)

	// Step 3: Check for existing compiled WASM
	logAppSubmission("[%s] STEP 3: Checking for existing compiled WASM", sessionID)
	artifactsDir := filepath.Join("artifacts", req.AppID)
	wasmFilename := fmt.Sprintf("%s.wasm", req.Version)
	finalWasmPath := filepath.Join(artifactsDir, wasmFilename)
	logAppSubmission("[%s] Checking path: %s", sessionID, finalWasmPath)

	if _, err := os.Stat(finalWasmPath); err == nil {
		logAppSubmission("[%s] ERROR: WASM file already exists at %s", sessionID, finalWasmPath)
		logAppSubmission("[%s] RESPONSE: HTTP 409 - Conflict", sessionID)
		http.Error(w, fmt.Sprintf("app %s version %s already compiled and exists", req.AppID, req.Version), http.StatusConflict)
		return
	}
	logAppSubmission("[%s] No existing WASM found - proceeding with compilation", sessionID)

	// Step 4: Prepare build directory
	logAppSubmission("[%s] STEP 4: Setting up build directory", sessionID)
	buildDir := filepath.Join("build", req.AppID, req.Version)
	logAppSubmission("[%s] Build directory: %s", sessionID, buildDir)

	// Clear build directory if it exists
	if _, err := os.Stat(buildDir); err == nil {
		logAppSubmission("[%s] Existing build directory found, removing: %s", sessionID, buildDir)
		if err := os.RemoveAll(buildDir); err != nil {
			logAppSubmission("[%s] ERROR: Failed to remove existing build directory: %v", sessionID, err)
			logAppSubmission("[%s] RESPONSE: HTTP 500 - Build directory cleanup failed", sessionID)
			http.Error(w, fmt.Sprintf("failed to clear existing build directory: %v", err), http.StatusInternalServerError)
			return
		}
		logAppSubmission("[%s] Successfully removed existing build directory", sessionID)
	}

	// Create build directory
	logAppSubmission("[%s] Creating build directory: %s", sessionID, buildDir)
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		logAppSubmission("[%s] ERROR: Failed to create build directory: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Build directory creation failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to create build directory: %v", err), http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] Build directory created successfully", sessionID)

	// Step 5: Generate Rust project
	logAppSubmission("[%s] STEP 5: Generating Rust project from trait implementation", sessionID)
	startTime := time.Now()
	if err := generateRustProjectFromTrait(req, buildDir); err != nil {
		logAppSubmission("[%s] ERROR: Failed to generate Rust project: %v", sessionID, err)
		logAppSubmission("[%s] Cleaning up build directory: %s", sessionID, buildDir)
		os.RemoveAll(buildDir)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Rust project generation failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to generate Rust project: %v", err), http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] Rust project generation completed in %v", sessionID, time.Since(startTime))

	// Step 6: Compile to WASM
	logAppSubmission("[%s] STEP 6: Compiling Rust to WASM", sessionID)
	startTime = time.Now()
	wasmPath, err := buildRustToWasm(buildDir)
	if err != nil {
		logAppSubmission("[%s] ERROR: Failed to compile Rust to WASM: %v", sessionID, err)
		logAppSubmission("[%s] Build directory contents before cleanup:", sessionID)

		// Log build directory contents for debugging
		if entries, dirErr := os.ReadDir(buildDir); dirErr == nil {
			for _, entry := range entries {
				logAppSubmission("[%s]   - %s (dir: %t)", sessionID, entry.Name(), entry.IsDir())
			}
		}

		logAppSubmission("[%s] Cleaning up build directory: %s", sessionID, buildDir)
		// os.RemoveAll(buildDir)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - WASM compilation failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to compile Rust to WASM: %v", err), http.StatusInternalServerError)
		return
	}
	logAppSubmission("[%s] WASM compilation completed in %v", sessionID, time.Since(startTime))
	logAppSubmission("[%s] Compiled WASM path: %s", sessionID, wasmPath)

	// Step 7: Register app in registry
	logAppSubmission("[%s] STEP 7: Registering app in registry", sessionID)
	registryMutex.Lock()
	registry[req.AppID] = &App{
		AppID:          req.AppID,
		Version:        req.Version,
		Runtime:        req.Runtime,
		Tools:          req.Tools,
		ArtifactURI:    wasmPath,
		SourceLanguage: "rust",
		Files: []File{
			{
				Name:    "src/lib.rs",
				Content: "Generated from trait implementation",
			},
		},
	}
	registrySize := len(registry)
	registryMutex.Unlock()
	logAppSubmission("[%s] App registered successfully. Registry now contains %d apps", sessionID, registrySize)

	// Step 8: Save registry to file
	logAppSubmission("[%s] STEP 8: Saving registry to file", sessionID)
	if err := saveRegistry(); err != nil {
		logAppSubmission("[%s] WARNING: Failed to save registry to file: %v", sessionID, err)
		log.Printf("Warning: failed to save registry: %v", err)
	} else {
		logAppSubmission("[%s] Registry saved successfully", sessionID)
	}

	// Step 9: Send success response
	logAppSubmission("[%s] STEP 9: Sending success response", sessionID)
	response := map[string]string{
		"status":   "app trait implementation submitted and compiled",
		"wasmPath": wasmPath,
	}
	responseJSON, _ := json.Marshal(response)
	logAppSubmission("[%s] Response: %s", sessionID, string(responseJSON))

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)

	logAppSubmission("[%s] === APP SUBMISSION COMPLETED SUCCESSFULLY ===", sessionID)
	logAppSubmission("[%s] Total processing time: %v", sessionID, time.Since(startTime))
}

func scheduleAppRunHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := fmt.Sprintf("schedule_session_%d", time.Now().UnixNano())
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = forwarded
	}

	logAppSubmission("=== NEW APP SCHEDULE REQUEST [%s] ===", sessionID)
	logAppSubmission("[%s] Client IP: %s", sessionID, clientIP)
	logAppSubmission("[%s] Request Method: %s", sessionID, r.Method)
	logAppSubmission("[%s] Request URL: %s", sessionID, r.URL.String())
	logAppSubmission("[%s] User-Agent: %s", sessionID, r.Header.Get("User-Agent"))
	logAppSubmission("[%s] Content-Type: %s", sessionID, r.Header.Get("Content-Type"))

	logAppSubmission("[%s] STEP 1: Parsing schedule request", sessionID)
	var req ScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logAppSubmission("[%s] ERROR: Failed to decode request body: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Invalid JSON", sessionID)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	logAppSubmission("[%s] Parsed request - AppID: %s, ToolName: %s, ScheduleType: %s", sessionID, req.AppID, req.ToolName, req.ScheduleType)

	// Step 2: Validate request fields
	logAppSubmission("[%s] STEP 2: Validating schedule request", sessionID)
	if req.AppID == "" {
		logAppSubmission("[%s] ERROR: AppID is required", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Missing AppID", sessionID)
		http.Error(w, "appId is required", http.StatusBadRequest)
		return
	}

	if req.ToolName == "" {
		logAppSubmission("[%s] ERROR: ToolName is required", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Missing ToolName", sessionID)
		http.Error(w, "toolName is required", http.StatusBadRequest)
		return
	}

	if req.ScheduleType != ScheduleTypeOneTime && req.ScheduleType != ScheduleTypeRecurring {
		logAppSubmission("[%s] ERROR: Invalid schedule type: %s", sessionID, req.ScheduleType)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Invalid schedule type", sessionID)
		http.Error(w, "scheduleType must be 'one-time' or 'recurring'", http.StatusBadRequest)
		return
	}

	if req.ScheduledTime.Time.IsZero() {
		logAppSubmission("[%s] ERROR: ScheduledTime is required", sessionID)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Missing ScheduledTime", sessionID)
		http.Error(w, "scheduledTime is required", http.StatusBadRequest)
		return
	}

	// Validate scheduled time is in the future
	if req.ScheduledTime.Time.Before(time.Now()) {
		logAppSubmission("[%s] ERROR: ScheduledTime is in the past: %v", sessionID, req.ScheduledTime.Time)
		logAppSubmission("[%s] RESPONSE: HTTP 400 - Past scheduled time", sessionID)
		http.Error(w, "scheduledTime must be in the future", http.StatusBadRequest)
		return
	}

	// Validate recurrence for recurring schedules
	if req.ScheduleType == ScheduleTypeRecurring {
		if req.Recurrence == nil {
			logAppSubmission("[%s] ERROR: Recurrence is required for recurring schedules", sessionID)
			logAppSubmission("[%s] RESPONSE: HTTP 400 - Missing recurrence", sessionID)
			http.Error(w, "recurrence is required for recurring schedules", http.StatusBadRequest)
			return
		}
		if err := validateRecurrence(req.Recurrence); err != nil {
			logAppSubmission("[%s] ERROR: Invalid recurrence: %v", sessionID, err)
			logAppSubmission("[%s] RESPONSE: HTTP 400 - Invalid recurrence", sessionID)
			http.Error(w, fmt.Sprintf("invalid recurrence: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Step 3: Verify app exists and tool is valid
	logAppSubmission("[%s] STEP 3: Verifying app and tool exist", sessionID)
	registryMutex.RLock()
	app, ok := registry[req.AppID]
	registryMutex.RUnlock()
	if !ok {
		logAppSubmission("[%s] ERROR: App not found in registry: %s", sessionID, req.AppID)
		logAppSubmission("[%s] RESPONSE: HTTP 404 - App not found", sessionID)
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}

	// Validate that the requested tool exists in the app
	toolFound := false
	for _, tool := range app.Tools {
		if tool.Name == req.ToolName {
			toolFound = true
			logAppSubmission("[%s] Tool '%s' found in app", sessionID, req.ToolName)
			break
		}
	}
	if !toolFound {
		logAppSubmission("[%s] ERROR: Tool '%s' not found in app. Available tools: %v", sessionID, req.ToolName, app.Tools)
		logAppSubmission("[%s] RESPONSE: HTTP 404 - Tool not found", sessionID)
		http.Error(w, fmt.Sprintf("tool '%s' not found in app", req.ToolName), http.StatusNotFound)
		return
	}

	// Step 4: Generate schedule ID and create schedule
	logAppSubmission("[%s] STEP 4: Creating schedule", sessionID)
	scheduleID, err := generateID()
	if err != nil {
		logAppSubmission("[%s] ERROR: Failed to generate schedule ID: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - ID generation failed", sessionID)
		http.Error(w, "failed to generate schedule ID", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	schedule := &AppSchedule{
		ID:            scheduleID,
		AppID:         req.AppID,
		ToolName:      req.ToolName,
		Input:         req.Input,
		ScheduleType:  req.ScheduleType,
		ScheduledTime: req.ScheduledTime.Time,
		Recurrence:    req.Recurrence,
		IsActive:      true,
		CreatedAt:     now,
		RunCount:      0,
	}

	// Calculate next run time
	nextRun := req.ScheduledTime.Time
	schedule.NextRun = &nextRun

	// Step 5: Store schedule in database
	logAppSubmission("[%s] STEP 5: Storing schedule in database", sessionID)
	if err := saveScheduleToDatabase(schedule); err != nil {
		logAppSubmission("[%s] ERROR: Failed to save schedule to database: %v", sessionID, err)
		logAppSubmission("[%s] RESPONSE: HTTP 500 - Database save failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to save schedule: %v", err), http.StatusInternalServerError)
		return
	}

	// Step 6: Store schedule in memory
	schedulesMutex.Lock()
	schedules[scheduleID] = schedule
	scheduleCount := len(schedules)
	schedulesMutex.Unlock()
	logAppSubmission("[%s] Schedule stored in memory. Total schedules: %d", sessionID, scheduleCount)

	// Step 7: Send response
	logAppSubmission("[%s] STEP 6: Sending success response", sessionID)
	response := map[string]interface{}{
		"status":     "schedule created successfully",
		"scheduleId": scheduleID,
		"nextRun":    schedule.NextRun,
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
	logAppSubmission("[%s] === APP SCHEDULE CREATED SUCCESSFULLY ===", sessionID)
}

func listSchedulesHandler(w http.ResponseWriter, r *http.Request) {
	appIdFilter := r.URL.Query().Get("appId")

	schedulesMutex.RLock()
	defer schedulesMutex.RUnlock()

	scheduleList := []*AppSchedule{}
	for _, schedule := range schedules {
		// Apply app ID filter if specified
		if appIdFilter != "" && schedule.AppID != appIdFilter {
			continue
		}
		scheduleList = append(scheduleList, schedule)
	}

	json.NewEncoder(w).Encode(scheduleList)
}

func getScheduleHandler(w http.ResponseWriter, r *http.Request) {
	scheduleID := r.URL.Query().Get("id")
	if scheduleID == "" {
		http.Error(w, "schedule id is required", http.StatusBadRequest)
		return
	}

	schedulesMutex.RLock()
	schedule, exists := schedules[scheduleID]
	schedulesMutex.RUnlock()

	if !exists {
		http.Error(w, "schedule not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(schedule)
}

func deleteScheduleHandler(w http.ResponseWriter, r *http.Request) {
	scheduleID := r.URL.Query().Get("id")
	if scheduleID == "" {
		http.Error(w, "schedule id is required", http.StatusBadRequest)
		return
	}

	schedulesMutex.Lock()
	schedule, exists := schedules[scheduleID]
	if !exists {
		schedulesMutex.Unlock()
		http.Error(w, "schedule not found", http.StatusNotFound)
		return
	}

	// Mark as inactive in database
	schedule.IsActive = false
	delete(schedules, scheduleID)
	schedulesMutex.Unlock()

	// Update in database
	if err := deactivateScheduleInDatabase(scheduleID); err != nil {
		log.Printf("Failed to deactivate schedule in database: %v", err)
		http.Error(w, "failed to delete schedule", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status": "schedule deleted successfully",
	}
	json.NewEncoder(w).Encode(response)
}

func updateScheduleHandler(w http.ResponseWriter, r *http.Request) {
	scheduleID := r.URL.Query().Get("id")
	if scheduleID == "" {
		http.Error(w, "schedule id is required", http.StatusBadRequest)
		return
	}

	var updateReq struct {
		IsActive      *bool           `json:"isActive,omitempty"`
		ScheduledTime *time.Time      `json:"scheduledTime,omitempty"`
		Input         json.RawMessage `json:"input,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	schedulesMutex.Lock()
	schedule, exists := schedules[scheduleID]
	if !exists {
		schedulesMutex.Unlock()
		http.Error(w, "schedule not found", http.StatusNotFound)
		return
	}

	// Update fields
	updated := false
	if updateReq.IsActive != nil {
		schedule.IsActive = *updateReq.IsActive
		updated = true
	}
	if updateReq.ScheduledTime != nil {
		schedule.ScheduledTime = *updateReq.ScheduledTime
		// Recalculate next run
		schedule.NextRun = calculateNextRun(schedule)
		updated = true
	}
	if updateReq.Input != nil {
		schedule.Input = updateReq.Input
		updated = true
	}
	schedulesMutex.Unlock()

	if !updated {
		http.Error(w, "no fields to update", http.StatusBadRequest)
		return
	}

	// Update in database
	if err := updateScheduleInDatabase(schedule); err != nil {
		log.Printf("Failed to update schedule in database: %v", err)
		http.Error(w, "failed to update schedule", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":   "schedule updated successfully",
		"schedule": schedule,
	}
	json.NewEncoder(w).Encode(response)
}

func listScheduledRunsHandler(w http.ResponseWriter, r *http.Request) {
	scheduleID := r.URL.Query().Get("schedule_id")

	systemDBMutex.RLock()
	defer systemDBMutex.RUnlock()

	if systemDB == nil {
		http.Error(w, "system database not initialized", http.StatusInternalServerError)
		return
	}

	var query string
	var args []interface{}

	if scheduleID != "" {
		query = `SELECT 
			id, schedule_id, app_id, tool_name, input_data, started_at, 
			completed_at, status, output, error
		FROM scheduled_runs WHERE schedule_id = ? ORDER BY started_at DESC`
		args = []interface{}{scheduleID}
	} else {
		query = `SELECT 
			id, schedule_id, app_id, tool_name, input_data, started_at, 
			completed_at, status, output, error
		FROM scheduled_runs ORDER BY started_at DESC`
	}

	rows, err := systemDB.Query(query, args...)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query runs: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var runs []*ScheduledRun

	for rows.Next() {
		run := &ScheduledRun{}
		var inputData, startedAt string
		var completedAtStr sql.NullString
		var output, errorStr sql.NullString

		err := rows.Scan(
			&run.ID, &run.ScheduleID, &run.AppID, &run.ToolName, &inputData,
			&startedAt, &completedAtStr, &run.Status, &output, &errorStr,
		)
		if err != nil {
			log.Printf("Error scanning run row: %v", err)
			continue
		}

		// Parse JSON input
		if err := json.Unmarshal([]byte(inputData), &run.Input); err != nil {
			log.Printf("Error parsing input data for run %s: %v", run.ID, err)
		}

		// Parse times
		if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
			run.StartedAt = t
		}
		if completedAtStr.Valid {
			if t, err := time.Parse(time.RFC3339, completedAtStr.String); err == nil {
				run.CompletedAt = &t
			}
		}

		if output.Valid {
			run.Output = output.String
		}
		if errorStr.Valid {
			run.Error = errorStr.String
		}

		runs = append(runs, run)
	}

	json.NewEncoder(w).Encode(runs)
}

func deactivateScheduleInDatabase(scheduleID string) error {
	systemDBMutex.Lock()
	defer systemDBMutex.Unlock()

	if systemDB == nil {
		return fmt.Errorf("system database not initialized")
	}

	query := `UPDATE app_schedules SET is_active = 0 WHERE id = ?`
	_, err := systemDB.Exec(query, scheduleID)
	return err
}

func generateRustProjectFromTrait(req AppRequest, buildDir string) error {
	// Read the wrapper template
	templatePath := filepath.Join("wasm_wrapper_template.rs")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read wrapper template: %v", err)
	}

	// Inject the user's trait implementation into the template
	injectedContent := strings.Replace(string(templateContent), "// USER_APP_IMPL_PLACEHOLDER - This will be replaced with user's trait implementation", req.AppSrc, 1)

	// Create src directory
	srcDir := filepath.Join(buildDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return fmt.Errorf("failed to create src directory: %v", err)
	}

	// Write lib.rs with injected code
	libPath := filepath.Join(srcDir, "lib.rs")
	if err := os.WriteFile(libPath, []byte(injectedContent), 0644); err != nil {
		return fmt.Errorf("failed to write lib.rs: %v", err)
	}

	// Create Cargo.toml
	cargoToml := fmt.Sprintf(`[package]
name = "%s"
version = "%s"
edition = "2021"

[lib]
crate-type = ["cdylib"]

[dependencies]
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"

[profile.release]
opt-level = "s"
lto = true
`, req.AppID, req.Version)

	cargoPath := filepath.Join(buildDir, "Cargo.toml")
	if err := os.WriteFile(cargoPath, []byte(cargoToml), 0644); err != nil {
		return fmt.Errorf("failed to write Cargo.toml: %v", err)
	}

	return nil
}

func createFilesFromSpec(files []File, destDir string) error {
	for _, file := range files {
		// Create the full file path
		destPath := filepath.Join(destDir, file.Name)

		// Ensure the file path is within the destination directory (security check)
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", file.Name)
		}

		// Create parent directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %v", destPath, err)
		}

		// Create destination file
		destFile, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %v", destPath, err)
		}
		defer destFile.Close()

		// Write file contents as raw text
		if _, err := destFile.WriteString(file.Content); err != nil {
			return fmt.Errorf("failed to write contents to file %s: %v", file.Name, err)
		}
	}

	return nil
}

func buildRustToWasm(sourceBuildDir string) (string, error) {
	// Check if Cargo.toml exists to confirm it's a Rust project
	cargoPath := filepath.Join(sourceBuildDir, "Cargo.toml")
	if _, err := os.Stat(cargoPath); os.IsNotExist(err) {
		return "", fmt.Errorf("Cargo.toml not found in %s", sourceBuildDir)
	}

	// Create artifacts directory structure to store the compiled WASM
	appID := filepath.Base(filepath.Dir(sourceBuildDir))
	version := filepath.Base(sourceBuildDir)
	artifactsDir := filepath.Join("artifacts", appID)
	wasmFilename := fmt.Sprintf("%s.wasm", version)
	outputWasmPath := filepath.Join(artifactsDir, wasmFilename)

	// Create artifacts directory if it doesn't exist
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create artifacts directory: %v", err)
	}

	// Build the Rust project to WASM
	// Using cargo build with wasm32-unknown-unknown target
	cmd := exec.Command("cargo", "build", "--target", "wasm32-unknown-unknown", "--release")
	cmd.Dir = sourceBuildDir

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cargo build failed: %v\nOutput: %s", err, string(output))
	}

	// Find the compiled WASM file in the target directory
	// The WASM file should be at target/wasm32-unknown-unknown/release/<project_name>.wasm
	targetDir := filepath.Join(sourceBuildDir, "target", "wasm32-unknown-unknown", "release")

	// Read Cargo.toml to get the project name
	projectName, err := getProjectNameFromCargo(cargoPath)
	if err != nil {
		return "", fmt.Errorf("failed to get project name: %v", err)
	}

	compiledWasmPath := filepath.Join(targetDir, projectName+".wasm")

	// Check if the compiled WASM file exists
	if _, err := os.Stat(compiledWasmPath); os.IsNotExist(err) {
		return "", fmt.Errorf("compiled WASM file not found at %s", compiledWasmPath)
	}

	// Copy the compiled WASM to the artifacts directory
	if err := copyFile(compiledWasmPath, outputWasmPath); err != nil {
		return "", fmt.Errorf("failed to copy WASM file: %v", err)
	}

	return outputWasmPath, nil
}

func getProjectNameFromCargo(cargoPath string) (string, error) {
	content, err := os.ReadFile(cargoPath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name") && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				name = strings.Trim(name, `"`)
				// Rust converts hyphens to underscores in binary names
				name = strings.ReplaceAll(name, "-", "_")
				return name, nil
			}
		}
	}
	return "", fmt.Errorf("project name not found in Cargo.toml")
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// --- Database Host Functions for WASM ---

// dbQuery executes a SQL query and returns JSON results
func dbQuery(caller *wasmtime.Caller, queryPtr, queryLen, resultPtrPtr int32) int32 {
	// Get memory instance
	memory := caller.GetExport("memory").Memory()
	data := memory.UnsafeData(caller)

	// Read query string from WASM memory
	queryBytes := data[queryPtr : queryPtr+queryLen]
	query := string(queryBytes)

	log.Printf("[WASM DB] dbQuery called with query: %s", query)

	appDBMutex.RLock()
	defer appDBMutex.RUnlock()

	if appDB == nil {
		log.Printf("[WASM DB] Database not initialized")
		return -1 // Database not initialized
	}

	// Execute query
	rows, err := appDB.Query(query)
	if err != nil {
		log.Printf("[WASM DB] Database query error: %v", err)
		return -2 // Query error
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		log.Printf("[WASM DB] Column error: %v", err)
		return -3 // Column error
	}
	log.Printf("[WASM DB] Query columns: %v", columns)

	// Collect results
	results := []map[string]interface{}{}
	log.Printf("[WASM DB] Query returned %+v rows", results)
	rowCount := 0
	for rows.Next() {
		rowCount++
		// Create a slice to hold column values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			log.Printf("[WASM DB] Row scan error: %v", err)
			continue
		}

		// Convert to map
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}
	log.Printf("[WASM DB] Query returned %d rows", rowCount)

	// Marshal results to JSON
	jsonBytes, err := json.Marshal(results)
	if err != nil {
		log.Printf("[WASM DB] JSON marshal error: %v", err)
		return -4 // JSON marshal error
	}
	jsonString := string(jsonBytes)
	log.Printf("[WASM DB] Marshaled JSON result (%d bytes): %s", len(jsonBytes), jsonString)

	// Allocate memory in WASM for result
	allocateFunc := caller.GetExport("allocate").Func()
	resultLenResult, err := allocateFunc.Call(caller, len(jsonBytes))
	if err != nil {
		log.Printf("[WASM DB] Allocation error: %v", err)
		return -5 // Allocation error
	}
	resultPtr := resultLenResult.(int32)
	log.Printf("[WASM DB] Allocated result at pointer %d", resultPtr)

	// Copy result to WASM memory
	copy(data[resultPtr:resultPtr+int32(len(jsonBytes))], jsonBytes)

	// Store result pointer in the provided location
	resultPtrPtrBytes := data[resultPtrPtr : resultPtrPtr+4]
	resultPtrPtrBytes[0] = byte(resultPtr)
	resultPtrPtrBytes[1] = byte(resultPtr >> 8)
	resultPtrPtrBytes[2] = byte(resultPtr >> 16)
	resultPtrPtrBytes[3] = byte(resultPtr >> 24)

	log.Printf("[WASM DB] dbQuery completed successfully, returning %d bytes", len(jsonBytes))
	return int32(len(jsonBytes)) // Return result length
}

// dbExec executes a SQL statement (INSERT, UPDATE, DELETE)
func dbExec(caller *wasmtime.Caller, stmtPtr, stmtLen int32) int32 {
	// Get memory instance
	memory := caller.GetExport("memory").Memory()
	data := memory.UnsafeData(caller)

	// Read statement string from WASM memory
	stmtBytes := data[stmtPtr : stmtPtr+stmtLen]
	stmt := string(stmtBytes)

	appDBMutex.Lock()
	defer appDBMutex.Unlock()

	if appDB == nil {
		return -1 // Database not initialized
	}

	// Execute statement
	result, err := appDB.Exec(stmt)
	if err != nil {
		log.Printf("Database exec error: %v", err)
		return -2 // Execution error
	}

	// Return number of affected rows
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return -3 // Could not get affected rows
	}

	return int32(rowsAffected)
}

// dbPreparedQuery executes a prepared statement with parameters
func dbPreparedQuery(caller *wasmtime.Caller, stmtPtr, stmtLen, paramsPtr, paramsLen, resultPtrPtr int32) int32 {
	// Get memory instance
	memory := caller.GetExport("memory").Memory()
	data := memory.UnsafeData(caller)

	// Read statement string
	stmtBytes := data[stmtPtr : stmtPtr+stmtLen]
	stmt := string(stmtBytes)

	// Read parameters JSON
	paramsBytes := data[paramsPtr : paramsPtr+paramsLen]
	var params []interface{}
	if paramsLen > 0 {
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return -1 // Parameter parsing error
		}
	}

	appDBMutex.RLock()
	defer appDBMutex.RUnlock()

	if appDB == nil {
		return -2 // Database not initialized
	}

	// Execute prepared statement
	rows, err := appDB.Query(stmt, params...)
	if err != nil {
		log.Printf("Database prepared query error: %v", err)
		return -3 // Query error
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return -4 // Column error
	}

	// Collect results (same as dbQuery)
	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	// Marshal results to JSON
	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return -5 // JSON marshal error
	}

	// Allocate memory in WASM for result
	allocateFunc := caller.GetExport("allocate").Func()
	resultLenResult, err := allocateFunc.Call(caller, len(jsonBytes))
	if err != nil {
		return -6 // Allocation error
	}
	resultPtr := resultLenResult.(int32)

	// Copy result to WASM memory
	copy(data[resultPtr:resultPtr+int32(len(jsonBytes))], jsonBytes)

	// Store result pointer
	resultPtrPtrBytes := data[resultPtrPtr : resultPtrPtr+4]
	resultPtrPtrBytes[0] = byte(resultPtr)
	resultPtrPtrBytes[1] = byte(resultPtr >> 8)
	resultPtrPtrBytes[2] = byte(resultPtr >> 16)
	resultPtrPtrBytes[3] = byte(resultPtr >> 24)

	return int32(len(jsonBytes))
}

// claudeQuery sends a message to Claude AI and returns the response
func claudeQuery(caller *wasmtime.Caller, messagePtr, messageLen, resultPtrPtr int32) int32 {
	// Get memory instance
	memory := caller.GetExport("memory").Memory()
	data := memory.UnsafeData(caller)

	// Read message string from WASM memory
	messageBytes := data[messagePtr : messagePtr+messageLen]
	message := string(messageBytes)

	log.Printf("[WASM Claude] claudeQuery called with message: %s", message)

	// Check if Claude service is available
	if claudeService == nil {
		log.Printf("[WASM Claude] Claude service not initialized")
		return -1 // Claude service not initialized
	}

	// Send message to Claude
	response, err := claudeService.SendMessage(message)
	if err != nil {
		log.Printf("[WASM Claude] Claude API error: %v", err)
		return -2 // Claude API error
	}

	// Create response JSON
	responseObj := map[string]interface{}{
		"response": response,
		"success":  true,
	}

	// Marshal response to JSON
	jsonBytes, err := json.Marshal(responseObj)
	if err != nil {
		log.Printf("[WASM Claude] JSON marshal error: %v", err)
		return -3 // JSON marshal error
	}

	// Allocate memory in WASM for result
	allocateFunc := caller.GetExport("allocate").Func()
	resultLenResult, err := allocateFunc.Call(caller, len(jsonBytes))
	if err != nil {
		log.Printf("[WASM Claude] Memory allocation error: %v", err)
		return -4 // Allocation error
	}
	resultPtr := resultLenResult.(int32)

	// Copy result to WASM memory
	copy(data[resultPtr:resultPtr+int32(len(jsonBytes))], jsonBytes)

	// Store result pointer in the output parameter
	resultPtrPtrBytes := data[resultPtrPtr : resultPtrPtr+4]
	resultPtrPtrBytes[0] = byte(resultPtr)
	resultPtrPtrBytes[1] = byte(resultPtr >> 8)
	resultPtrPtrBytes[2] = byte(resultPtr >> 16)
	resultPtrPtrBytes[3] = byte(resultPtr >> 24)

	log.Printf("[WASM Claude] Claude query completed successfully")
	return int32(len(jsonBytes))
}

// --- Schedule Management ---

func generateID() (string, error) {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func validateRecurrence(r *RecurrenceRule) error {
	if r == nil {
		return fmt.Errorf("recurrence cannot be nil")
	}

	if r.Interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}

	validUnits := map[string]bool{
		"minutes": true,
		"hours":   true,
		"days":    true,
		"weeks":   true,
		"months":  true,
	}

	if !validUnits[r.Unit] {
		return fmt.Errorf("unit must be one of: minutes, hours, days, weeks, months")
	}

	// Validate days of week for weekly recurrence
	if r.Unit == "weeks" && len(r.DaysOfWeek) > 0 {
		for _, day := range r.DaysOfWeek {
			if day < 0 || day > 6 {
				return fmt.Errorf("daysOfWeek must be between 0 (Sunday) and 6 (Saturday)")
			}
		}
	}

	// Validate end date is in the future if provided
	if r.EndDate != nil && r.EndDate.Before(time.Now()) {
		return fmt.Errorf("endDate must be in the future")
	}

	return nil
}

func saveScheduleToDatabase(schedule *AppSchedule) error {
	systemDBMutex.Lock()
	defer systemDBMutex.Unlock()

	if systemDB == nil {
		return fmt.Errorf("system database not initialized")
	}

	// Convert struct fields to JSON for storage
	inputJSON, _ := json.Marshal(schedule.Input)
	recurrenceJSON := []byte("null")
	if schedule.Recurrence != nil {
		recurrenceJSON, _ = json.Marshal(schedule.Recurrence)
	}

	var lastRunStr, nextRunStr interface{}
	if schedule.LastRun != nil {
		lastRunStr = schedule.LastRun.Format(time.RFC3339)
	}
	if schedule.NextRun != nil {
		nextRunStr = schedule.NextRun.Format(time.RFC3339)
	}

	query := `INSERT INTO app_schedules (
		id, app_id, tool_name, input_data, schedule_type, scheduled_time, 
		recurrence_rule, is_active, created_at, last_run, next_run, run_count
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := systemDB.Exec(query,
		schedule.ID,
		schedule.AppID,
		schedule.ToolName,
		string(inputJSON),
		string(schedule.ScheduleType),
		schedule.ScheduledTime.Format(time.RFC3339),
		string(recurrenceJSON),
		schedule.IsActive,
		schedule.CreatedAt.Format(time.RFC3339),
		lastRunStr,
		nextRunStr,
		schedule.RunCount,
	)

	return err
}

func calculateNextRun(schedule *AppSchedule) *time.Time {
	if schedule.ScheduleType == ScheduleTypeOneTime {
		// One-time schedules don't have a "next run" after completion
		if schedule.RunCount > 0 {
			return nil
		}
		return &schedule.ScheduledTime
	}

	if schedule.Recurrence == nil {
		return nil
	}

	// Start from the last run time, or scheduled time if never run
	baseTime := schedule.ScheduledTime
	if schedule.LastRun != nil {
		baseTime = *schedule.LastRun
	}

	var nextRun time.Time

	switch schedule.Recurrence.Unit {
	case "minutes":
		nextRun = baseTime.Add(time.Duration(schedule.Recurrence.Interval) * time.Minute)
	case "hours":
		nextRun = baseTime.Add(time.Duration(schedule.Recurrence.Interval) * time.Hour)
	case "days":
		nextRun = baseTime.AddDate(0, 0, schedule.Recurrence.Interval)
	case "weeks":
		if len(schedule.Recurrence.DaysOfWeek) == 0 {
			// Simple weekly interval
			nextRun = baseTime.AddDate(0, 0, 7*schedule.Recurrence.Interval)
		} else {
			// Find next occurrence on specified days of week
			nextRun = findNextWeeklyOccurrence(baseTime, schedule.Recurrence)
		}
	case "months":
		nextRun = baseTime.AddDate(0, schedule.Recurrence.Interval, 0)
	default:
		return nil
	}

	// Check if we've passed the end date
	if schedule.Recurrence.EndDate != nil && nextRun.After(*schedule.Recurrence.EndDate) {
		return nil
	}

	return &nextRun
}

func findNextWeeklyOccurrence(baseTime time.Time, recurrence *RecurrenceRule) time.Time {
	current := baseTime.AddDate(0, 0, 1) // Start from the day after base time

	for i := 0; i < 14; i++ { // Look ahead maximum 2 weeks
		currentWeekday := int(current.Weekday())
		for _, day := range recurrence.DaysOfWeek {
			if currentWeekday == day {
				return current
			}
		}
		current = current.AddDate(0, 0, 1)
	}

	// Fallback to simple weekly interval if no matching day found
	return baseTime.AddDate(0, 0, 7*recurrence.Interval)
}

func loadSchedulesFromDatabase() error {
	systemDBMutex.RLock()
	defer systemDBMutex.RUnlock()

	if systemDB == nil {
		return fmt.Errorf("system database not initialized")
	}

	query := `SELECT 
		id, app_id, tool_name, input_data, schedule_type, scheduled_time,
		recurrence_rule, is_active, created_at, last_run, next_run, run_count
	FROM app_schedules WHERE is_active = 1`

	rows, err := systemDB.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query schedules: %v", err)
	}
	defer rows.Close()

	schedulesMutex.Lock()
	defer schedulesMutex.Unlock()
	schedules = make(map[string]*AppSchedule)

	for rows.Next() {
		schedule := &AppSchedule{}
		var inputData, scheduleType, scheduledTime, recurrenceRule, createdAt string
		var lastRunStr, nextRunStr sql.NullString

		err := rows.Scan(
			&schedule.ID, &schedule.AppID, &schedule.ToolName, &inputData,
			&scheduleType, &scheduledTime, &recurrenceRule, &schedule.IsActive,
			&createdAt, &lastRunStr, &nextRunStr, &schedule.RunCount,
		)
		if err != nil {
			log.Printf("Error scanning schedule row: %v", err)
			continue
		}

		// Parse JSON fields
		if err := json.Unmarshal([]byte(inputData), &schedule.Input); err != nil {
			log.Printf("Error parsing input data for schedule %s: %v", schedule.ID, err)
			continue
		}

		if recurrenceRule != "null" && recurrenceRule != "" {
			var recurrence RecurrenceRule
			if err := json.Unmarshal([]byte(recurrenceRule), &recurrence); err != nil {
				log.Printf("Error parsing recurrence rule for schedule %s: %v", schedule.ID, err)
			} else {
				schedule.Recurrence = &recurrence
			}
		}

		// Parse time fields
		schedule.ScheduleType = ScheduleType(scheduleType)
		if t, err := time.Parse(time.RFC3339, scheduledTime); err == nil {
			schedule.ScheduledTime = t
		}
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			schedule.CreatedAt = t
		}
		if lastRunStr.Valid {
			if t, err := time.Parse(time.RFC3339, lastRunStr.String); err == nil {
				schedule.LastRun = &t
			}
		}
		if nextRunStr.Valid {
			if t, err := time.Parse(time.RFC3339, nextRunStr.String); err == nil {
				schedule.NextRun = &t
			}
		}

		schedules[schedule.ID] = schedule
	}

	log.Printf("Loaded %d active schedules from database", len(schedules))
	return nil
}

func startScheduler() error {
	schedulerCtx, schedulerCancel = context.WithCancel(context.Background())

	// Load schedules from database
	if err := loadSchedulesFromDatabase(); err != nil {
		return fmt.Errorf("failed to load schedules: %v", err)
	}

	// Start scheduler goroutine
	go schedulerLoop(schedulerCtx)
	log.Println("Scheduler started")
	return nil
}

func stopScheduler() {
	if schedulerCancel != nil {
		schedulerCancel()
		log.Println("Scheduler stopped")
	}
}

func schedulerLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	log.Println("Scheduler loop started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Scheduler loop terminating")
			return
		case <-ticker.C:
			checkAndExecuteSchedules()
		}
	}
}

func checkAndExecuteSchedules() {
	now := time.Now()
	schedulesMutex.RLock()
	var schedulesToRun []*AppSchedule

	for _, schedule := range schedules {
		if schedule.IsActive && schedule.NextRun != nil && schedule.NextRun.Before(now.Add(time.Minute)) {
			schedulesToRun = append(schedulesToRun, schedule)
		}
	}
	schedulesMutex.RUnlock()

	if len(schedulesToRun) > 0 {
		log.Printf("Found %d schedules ready to run", len(schedulesToRun))
	}

	for _, schedule := range schedulesToRun {
		go executeScheduledRun(schedule)
	}
}

func executeScheduledRun(schedule *AppSchedule) {
	runID, err := generateID()
	if err != nil {
		log.Printf("Failed to generate run ID for schedule %s: %v", schedule.ID, err)
		return
	}

	startTime := time.Now()
	log.Printf("Starting scheduled run %s for schedule %s (app: %s, tool: %s)", runID, schedule.ID, schedule.AppID, schedule.ToolName)

	// Create scheduled run record
	run := &ScheduledRun{
		ID:         runID,
		ScheduleID: schedule.ID,
		AppID:      schedule.AppID,
		ToolName:   schedule.ToolName,
		Input:      schedule.Input,
		StartedAt:  startTime,
		Status:     "running",
	}

	// Save run record to database
	if err := saveScheduledRunToDatabase(run); err != nil {
		log.Printf("Failed to save scheduled run record: %v", err)
		return
	}

	// Execute the app tool
	output, err := executeAppTool(schedule.AppID, schedule.ToolName, schedule.Input)

	completedAt := time.Now()
	run.CompletedAt = &completedAt

	if err != nil {
		run.Status = "failed"
		run.Error = err.Error()
		log.Printf("Scheduled run %s failed: %v", runID, err)
	} else {
		run.Status = "completed"
		run.Output = output
		log.Printf("Scheduled run %s completed successfully in %v", runID, completedAt.Sub(startTime))
	}

	// Update run record in database
	if err := updateScheduledRunInDatabase(run); err != nil {
		log.Printf("Failed to update scheduled run record: %v", err)
	}

	// Update schedule's last run and calculate next run
	schedulesMutex.Lock()
	schedule.LastRun = &startTime
	schedule.RunCount++
	schedule.NextRun = calculateNextRun(schedule)
	schedulesMutex.Unlock()

	// Update schedule in database
	if err := updateScheduleInDatabase(schedule); err != nil {
		log.Printf("Failed to update schedule in database: %v", err)
	}

	log.Printf("Scheduled run %s processing complete. Next run: %v", runID, schedule.NextRun)
}

func saveScheduledRunToDatabase(run *ScheduledRun) error {
	systemDBMutex.Lock()
	defer systemDBMutex.Unlock()

	if systemDB == nil {
		return fmt.Errorf("system database not initialized")
	}

	inputJSON, _ := json.Marshal(run.Input)

	query := `INSERT INTO scheduled_runs (
		id, schedule_id, app_id, tool_name, input_data, started_at, status
	) VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := systemDB.Exec(query,
		run.ID, run.ScheduleID, run.AppID, run.ToolName,
		string(inputJSON), run.StartedAt.Format(time.RFC3339), run.Status,
	)

	return err
}

func updateScheduledRunInDatabase(run *ScheduledRun) error {
	systemDBMutex.Lock()
	defer systemDBMutex.Unlock()

	if systemDB == nil {
		return fmt.Errorf("system database not initialized")
	}

	var completedAtStr interface{}
	if run.CompletedAt != nil {
		completedAtStr = run.CompletedAt.Format(time.RFC3339)
	}

	query := `UPDATE scheduled_runs SET 
		completed_at = ?, status = ?, output = ?, error = ?
		WHERE id = ?`

	_, err := systemDB.Exec(query,
		completedAtStr, run.Status, run.Output, run.Error, run.ID,
	)

	return err
}

func updateScheduleInDatabase(schedule *AppSchedule) error {
	systemDBMutex.Lock()
	defer systemDBMutex.Unlock()

	if systemDB == nil {
		return fmt.Errorf("system database not initialized")
	}

	inputJSON, _ := json.Marshal(schedule.Input)

	var lastRunStr, nextRunStr interface{}
	if schedule.LastRun != nil {
		lastRunStr = schedule.LastRun.Format(time.RFC3339)
	}
	if schedule.NextRun != nil {
		nextRunStr = schedule.NextRun.Format(time.RFC3339)
	}

	query := `UPDATE app_schedules SET 
		input_data = ?, scheduled_time = ?, is_active = ?,
		last_run = ?, next_run = ?, run_count = ?
		WHERE id = ?`

	_, err := systemDB.Exec(query,
		string(inputJSON), schedule.ScheduledTime.Format(time.RFC3339), schedule.IsActive,
		lastRunStr, nextRunStr, schedule.RunCount, schedule.ID,
	)

	return err
}

func executeAppTool(appID, toolName string, input json.RawMessage) (string, error) {
	// This reuses the existing runTool logic but without HTTP request/response
	registryMutex.RLock()
	app, ok := registry[appID]
	registryMutex.RUnlock()
	if !ok {
		return "", fmt.Errorf("app not found: %s", appID)
	}

	// Validate tool exists
	toolFound := false
	for _, tool := range app.Tools {
		if tool.Name == toolName {
			toolFound = true
			break
		}
	}
	if !toolFound {
		return "", fmt.Errorf("tool '%s' not found in app", toolName)
	}

	// Load and compile WASM module (similar to runToolHandler)
	wasmBytes, err := os.ReadFile(app.ArtifactURI)
	if err != nil {
		return "", fmt.Errorf("failed to load WASM artifact: %v", err)
	}

	store := wasmtime.NewStore(wasmEngine)
	module, err := wasmtime.NewModule(wasmEngine, wasmBytes)
	if err != nil {
		return "", fmt.Errorf("failed to compile WASM module: %v", err)
	}

	linker := wasmtime.NewLinker(wasmEngine)

	// Define database host functions
	linker.DefineFunc(store, "env", "db_query", func(caller *wasmtime.Caller, queryPtr, queryLen, resultPtrPtr int32) int32 {
		return dbQuery(caller, queryPtr, queryLen, resultPtrPtr)
	})
	linker.DefineFunc(store, "env", "db_exec", func(caller *wasmtime.Caller, stmtPtr, stmtLen int32) int32 {
		return dbExec(caller, stmtPtr, stmtLen)
	})
	linker.DefineFunc(store, "env", "db_prepared_query", func(caller *wasmtime.Caller, stmtPtr, stmtLen, paramsPtr, paramsLen, resultPtrPtr int32) int32 {
		return dbPreparedQuery(caller, stmtPtr, stmtLen, paramsPtr, paramsLen, resultPtrPtr)
	})

	// Define Claude AI host function
	linker.DefineFunc(store, "env", "claude_query", func(caller *wasmtime.Caller, messagePtr, messageLen, resultPtrPtr int32) int32 {
		return claudeQuery(caller, messagePtr, messageLen, resultPtrPtr)
	})

	instance, err := linker.Instantiate(store, module)
	if err != nil {
		return "", fmt.Errorf("failed to instantiate WASM module: %v", err)
	}

	runFunc := instance.GetFunc(store, "run")
	if runFunc == nil {
		return "", fmt.Errorf("WASM module does not export 'run' function")
	}

	allocateFunc := instance.GetFunc(store, "allocate")
	deallocateFunc := instance.GetFunc(store, "deallocate")
	getResultPtrFunc := instance.GetFunc(store, "get_result_ptr")

	if allocateFunc == nil || deallocateFunc == nil || getResultPtrFunc == nil {
		return "", fmt.Errorf("WASM module missing required functions")
	}

	// Prepare input
	wasmInput := map[string]interface{}{
		"tool": toolName,
		"data": input,
	}
	inputBytes, _ := json.Marshal(wasmInput)
	inputStr := string(inputBytes)
	inputLen := len(inputStr)

	// Allocate memory and copy input
	inputPtrResult, err := allocateFunc.Call(store, inputLen)
	if err != nil {
		return "", fmt.Errorf("failed to allocate input memory: %v", err)
	}
	inputPtr := inputPtrResult.(int32)

	memory := instance.GetExport(store, "memory").Memory()
	data := memory.UnsafeData(store)
	copy(data[inputPtr:inputPtr+int32(inputLen)], []byte(inputStr))

	// Execute
	resultLenResult, err := runFunc.Call(store, inputPtr, inputLen)
	if err != nil {
		deallocateFunc.Call(store, inputPtr, inputLen)
		return "", fmt.Errorf("execution failed: %v", err)
	}
	resultLen := resultLenResult.(int32)

	// Get result
	resultPtrResult, err := getResultPtrFunc.Call(store)
	if err != nil {
		deallocateFunc.Call(store, inputPtr, inputLen)
		return "", fmt.Errorf("failed to get result pointer: %v", err)
	}
	resultPtr := resultPtrResult.(int32)

	resultBytes := make([]byte, resultLen)
	copy(resultBytes, data[resultPtr:resultPtr+resultLen])
	resultStr := string(resultBytes)

	// Clean up
	deallocateFunc.Call(store, inputPtr, inputLen)

	return resultStr, nil
}

func loadRegistry() error {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	// Check if registry file exists
	if _, err := os.Stat(registryFilePath); os.IsNotExist(err) {
		// File doesn't exist, start with empty registry
		registry = make(map[string]*App)
		log.Printf("Registry file %s not found, starting with empty registry", registryFilePath)
		return nil
	}

	// Read the registry file
	data, err := os.ReadFile(registryFilePath)
	if err != nil {
		return fmt.Errorf("failed to read registry file: %v", err)
	}

	// Unmarshal JSON into registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return fmt.Errorf("failed to parse registry JSON: %v", err)
	}

	log.Printf("Loaded %d apps from registry file", len(registry))
	return nil
}

func saveRegistry() error {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	// Marshal registry to JSON
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry to JSON: %v", err)
	}

	// Write to file
	if err := os.WriteFile(registryFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry file: %v", err)
	}

	return nil
}

// --- Database Management ---

func initDatabases() error {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll("data", 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Initialize app database (for WASM apps to use)
	if err := initAppDatabase(); err != nil {
		return fmt.Errorf("failed to initialize app database: %v", err)
	}

	// Initialize system database (for app engine internal data)
	if err := initSystemDatabase(); err != nil {
		return fmt.Errorf("failed to initialize system database: %v", err)
	}

	log.Println("Databases initialized successfully")
	return nil
}

func initAppDatabase() error {
	var err error

	// Open app SQLite database
	appDB, err = sql.Open("sqlite3", "data/app_data.db")
	if err != nil {
		return fmt.Errorf("failed to open app database: %v", err)
	}

	// Test the connection
	if err := appDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping app database: %v", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := appDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Printf("Warning: failed to enable WAL mode on app database: %v", err)
	}

	// Enable foreign keys
	if _, err := appDB.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		log.Printf("Warning: failed to enable foreign keys on app database: %v", err)
	}

	// Create default tables for app data storage
	if err := createAppTables(); err != nil {
		return fmt.Errorf("failed to create app tables: %v", err)
	}

	log.Println("App database initialized successfully")
	return nil
}

func initSystemDatabase() error {
	var err error

	// Open system SQLite database
	systemDB, err = sql.Open("sqlite3", "data/system.db")
	if err != nil {
		return fmt.Errorf("failed to open system database: %v", err)
	}

	// Test the connection
	if err := systemDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping system database: %v", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := systemDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Printf("Warning: failed to enable WAL mode on system database: %v", err)
	}

	// Enable foreign keys
	if _, err := systemDB.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		log.Printf("Warning: failed to enable foreign keys on system database: %v", err)
	}

	// Create system tables (schedules, etc.)
	if err := createSystemTables(); err != nil {
		return fmt.Errorf("failed to create system tables: %v", err)
	}

	log.Println("System database initialized successfully")
	return nil
}

func createAppTables() error {
	queries := []string{
		// App data table - for general key-value storage per app
		`CREATE TABLE IF NOT EXISTS app_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_id TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(app_id, key)
		);`,

		// App logs table - for application logging
		`CREATE TABLE IF NOT EXISTS app_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_id TEXT NOT NULL,
			level TEXT NOT NULL,
			message TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// Sessions table - for user session management
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL,
			user_data TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// Create indexes for better performance
		`CREATE INDEX IF NOT EXISTS idx_app_data_app_id ON app_data(app_id);`,
		`CREATE INDEX IF NOT EXISTS idx_app_logs_app_id ON app_logs(app_id);`,
		`CREATE INDEX IF NOT EXISTS idx_app_logs_timestamp ON app_logs(timestamp);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_app_id ON sessions(app_id);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);`,
	}

	for _, query := range queries {
		if _, err := appDB.Exec(query); err != nil {
			return fmt.Errorf("failed to execute app table query: %v", err)
		}
	}

	return nil
}

func createSystemTables() error {
	queries := []string{
		// App schedules table - for scheduled app runs
		`CREATE TABLE IF NOT EXISTS app_schedules (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			input_data TEXT NOT NULL,
			schedule_type TEXT NOT NULL CHECK(schedule_type IN ('one-time', 'recurring')),
			scheduled_time TEXT NOT NULL,
			recurrence_rule TEXT,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			last_run TEXT,
			next_run TEXT,
			run_count INTEGER NOT NULL DEFAULT 0
		);`,

		// Scheduled runs table - for tracking individual scheduled executions
		`CREATE TABLE IF NOT EXISTS scheduled_runs (
			id TEXT PRIMARY KEY,
			schedule_id TEXT NOT NULL,
			app_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			input_data TEXT NOT NULL,
			started_at TEXT NOT NULL,
			completed_at TEXT,
			status TEXT NOT NULL CHECK(status IN ('running', 'completed', 'failed')),
			output TEXT,
			error TEXT,
			FOREIGN KEY (schedule_id) REFERENCES app_schedules(id) ON DELETE CASCADE
		);`,

		// Create indexes for better performance
		`CREATE INDEX IF NOT EXISTS idx_app_schedules_app_id ON app_schedules(app_id);`,
		`CREATE INDEX IF NOT EXISTS idx_app_schedules_next_run ON app_schedules(next_run);`,
		`CREATE INDEX IF NOT EXISTS idx_app_schedules_active ON app_schedules(is_active);`,
		`CREATE INDEX IF NOT EXISTS idx_scheduled_runs_schedule_id ON scheduled_runs(schedule_id);`,
		`CREATE INDEX IF NOT EXISTS idx_scheduled_runs_status ON scheduled_runs(status);`,
		`CREATE INDEX IF NOT EXISTS idx_scheduled_runs_started_at ON scheduled_runs(started_at);`,
	}

	for _, query := range queries {
		if _, err := systemDB.Exec(query); err != nil {
			return fmt.Errorf("failed to execute system table query: %v", err)
		}
	}

	return nil
}

func closeDatabases() {
	if appDB != nil {
		appDB.Close()
		log.Println("App database connection closed")
	}
	if systemDB != nil {
		systemDB.Close()
		log.Println("System database connection closed")
	}
}

// --- Configuration Loading ---

func loadConfig() error {
	configFile := "config.json"
	
	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("config file %s not found. Please create it with your Claude API configuration", configFile)
	}
	
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	
	if err := json.Unmarshal(data, &appConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}
	
	// Validate required Claude configuration
	if appConfig.Claude.APIKey == "" || appConfig.Claude.APIKey == "your-claude-api-key-here" {
		return fmt.Errorf("Claude API key not configured in %s", configFile)
	}
	
	if appConfig.Claude.BaseURL == "" {
		appConfig.Claude.BaseURL = "https://api.anthropic.com"
	}
	
	if appConfig.Claude.Model == "" {
		appConfig.Claude.Model = "claude-3-5-sonnet-20241022"
	}
	
	if appConfig.Claude.MaxTokens == 0 {
		appConfig.Claude.MaxTokens = 4096
	}
	
	if appConfig.Claude.TimeoutSeconds == 0 {
		appConfig.Claude.TimeoutSeconds = 30
	}
	
	// Enable MCP by default (now handled directly in Go)
	appConfig.Claude.EnableMCP = true
	
	// Initialize Claude service
	claudeService = NewClaudeService(appConfig.Claude)
	
	log.Printf("Configuration loaded successfully. Claude model: %s", appConfig.Claude.Model)
	return nil
}

// --- Main ---

func main() {
	// Initialize logging
	if err := initAppLogger(); err != nil {
		log.Fatalf("Failed to initialize app logger: %v", err)
	}
	defer closeAppLogger()

	// Load configuration
	if err := loadConfig(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize databases (separate app and system databases)
	if err := initDatabases(); err != nil {
		log.Fatalf("Failed to initialize databases: %v", err)
	}
	defer closeDatabases()

	// Load registry from file on startup
	if err := loadRegistry(); err != nil {
		log.Fatalf("Failed to load registry: %v", err)
	}

	// Start the scheduler
	if err := startScheduler(); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}
	defer stopScheduler()

	// CORS middleware
	corsHandler := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		}
	}

	// App management endpoints
	http.HandleFunc("/list_apps", corsHandler(listAppsHandler))
	http.HandleFunc("/run_tool", corsHandler(runToolHandler))
	http.HandleFunc("/submit_app_src", corsHandler(submitAppSrcHandler))

	// Schedule management endpoints
	http.HandleFunc("/schedule_app_run", corsHandler(scheduleAppRunHandler))
	http.HandleFunc("/list_schedules", corsHandler(listSchedulesHandler))
	http.HandleFunc("/get_schedule", corsHandler(getScheduleHandler))
	http.HandleFunc("/delete_schedule", corsHandler(deleteScheduleHandler))
	http.HandleFunc("/update_schedule", corsHandler(updateScheduleHandler))
	http.HandleFunc("/list_scheduled_runs", corsHandler(listScheduledRunsHandler))

	// AI Integration endpoints
	http.HandleFunc("/claude", corsHandler(claudeService.HandleClaudeAPI))

	port := appConfig.Server.Port
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting Arcadia App Engine server on port %s...", port)
	log.Printf("Available endpoints:")
	log.Printf("  /list_apps - List all registered apps")
	log.Printf("  /run_tool - Execute a tool from an app")
	log.Printf("  /submit_app_src - Submit new app source code")
	log.Printf("  /schedule_app_run - Schedule app runs (one-time or recurring)")
	log.Printf("  /list_schedules - List all schedules")
	log.Printf("  /get_schedule?id=<id> - Get specific schedule")
	log.Printf("  /delete_schedule?id=<id> - Delete a schedule")
	log.Printf("  /update_schedule?id=<id> - Update a schedule")
	log.Printf("  /list_scheduled_runs[?schedule_id=<id>] - List scheduled runs")
	log.Printf("  /claude - Send message to Claude AI (POST {\"message\": \"your message\"})")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
