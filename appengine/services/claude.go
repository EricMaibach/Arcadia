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

	"arcadia/services/ai"
)

// --- Claude Configuration ---

type ClaudeConfig struct {
	APIKey             string `json:"api_key"`
	BaseURL            string `json:"base_url"`
	Model              string `json:"model"`
	MaxTokens          int    `json:"max_tokens"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	EnableMCP          bool   `json:"enable_mcp"`
	MCPServerCmd       string `json:"mcp_server_cmd"`
	MaxContextMessages int    `json:"max_context_messages"` // Max number of messages to retain in context
	ContextCompaction  bool   `json:"context_compaction"`   // Enable smart context compaction
	ContextTTLMinutes  int    `json:"context_ttl_minutes"`  // TTL for context in minutes (0 = no expiry)
}

// --- Claude API structures ---

type ClaudeMessage struct {
	Role    string      `json:"role"`
	Content any `json:"content"` // Can be string or array of content blocks
}

type ClaudeTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema any `json:"input_schema"`
}

type ClaudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []ClaudeMessage `json:"messages"`
	Tools     []ClaudeTool    `json:"tools,omitempty"`
	System    string          `json:"system,omitempty"`
}

type ClaudeContent struct {
	Type  string      `json:"type"`
	Text  string      `json:"text,omitempty"`
	ID    string      `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
	Input any `json:"input,omitempty"`
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
	GetRegistry() map[string]any
	GetRegistryMutex() *sync.RWMutex
}

type AppRunner interface {
	ExecuteAppTool(appID, toolName string, input json.RawMessage) (string, error)
}

type AppCreator interface {
	CreateApp(appID, version, runtime string, tools []any, appSrc string, dependencies map[string]string) (string, error)
}

// EmbeddingSearch interface now defined in ai/tool_manager.go

var registryAccess RegistryAccess
var appRunner AppRunner
var appCreator AppCreator
var embeddingSearch ai.EmbeddingSearch

func SetRegistryAccess(ra RegistryAccess) {
	registryAccess = ra
}

func SetAppRunner(ar AppRunner) {
	appRunner = ar
}

func SetAppCreator(ac AppCreator) {
	appCreator = ac
}

func SetEmbeddingSearch(es ai.EmbeddingSearch) {
	embeddingSearch = es
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
	keepRecent := max(maxMessages-1, 1)

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

// TriggerToolRefresh refreshes the MCP tools list from the current registry state
// This should be called whenever apps are added, removed, or updated
func (cs *ClaudeService) TriggerToolRefresh() {
	log.Printf("[Claude MCP] Refreshing MCP tools due to registry change")
	cs.RefreshMCPTools()

	// Log the updated tool count for debugging
	log.Printf("[Claude MCP] Tool refresh complete, now have %d total tools", len(cs.mcpTools))
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

	// Build the request with full context and current date/time
	now := time.Now()
	systemPrompt := fmt.Sprintf(`Current date and time: %s (UTC: %s).

You are Arcadia, an AI-powered digital ecosystem where applications grow and flourish together.

ABOUT ARCADIA:
Arcadia is a digital ecosystem where applications grow and flourish together. It's a WASM-based application platform that allows developers to create and deploy applications that can interact with each other, access shared databases, and leverage AI capabilities.

AVAILABLE TOOLS:
You have access to three types of tools through the MCP (Model Context Protocol):

1. SYSTEM TOOLS (Arcadia platform functionality):
   - list_apps: List all registered applications in the ecosystem
   - create_app: Submit new Rust code to compile and register WASM applications
   - schedule_app_run: Schedule application tools to run at specific times
   - list_schedules: List all scheduled application runs

2. DOCUMENT SEARCH TOOLS (RAG capabilities):
   - search_documents: Enhanced semantic search with complete document content, context highlights, and intelligent content management

   Use this tool to access existing code, documentation, and files in the Arcadia ecosystem.
   Returns complete document content with highlighted relevant passages for optimal AI understanding.

   ENHANCED FEATURES:
   - Complete document content with size management (up to 50KB)
   - Context highlights: Top 3 most relevant passages for each document
   - Content previews: First 500 characters with truncation indicators
   - Better deduplication and relevance ranking
   - Eliminates fragmented chunk results

   WHEN TO USE:
   - Questions about existing code, files, or documentation
   - Finding relevant examples or implementations
   - Understanding how components work
   - User mentions "show me examples" or "how does X work"

   INCORPORATING RETRIEVED CONTEXT:
   - Always cite source file paths when referencing content
   - Explain relevance to the user's question
   - Use context highlights to focus on most relevant passages
   - Documents ranked by relevance with complete content included

3. APP TOOLS (from registered WASM applications):
   App tools are dynamically loaded and follow the naming pattern: "appId_toolName"
   Examples: "food-tracker_log_food", "hello-text_add_hello"

   These tools represent functionality exposed by individual applications in the ecosystem. Each app can expose multiple tools for different purposes.
   
   FORMATTING GUIDELINES:
   When displaying app names and tool names to users, always format them in a human-readable way:
   - Convert kebab-case (hyphen-separated) to Title Case
   - "food-tracker" → "Food Tracker"  
   - "hello-text" → "Hello Text"
   - "log_food" → "Log Food"
   - "get_daily_summary" → "Get Daily Summary"
   - "analyze_nutrition" → "Analyze Nutrition"
   
   Example: Instead of saying "food-tracker_log_food", say "Food Tracker's Log Food tool" or "the Log Food tool from Food Tracker"

CAPABILITIES:
- Create new applications by writing Rust code that implements the ArcadiaApp trait
- List and interact with existing applications and their tools
- Schedule automated runs of application tools
- Search and retrieve documents from the Arcadia codebase using semantic similarity
- Access full content of specific documents to provide detailed code explanations
- Help users understand and navigate the Arcadia ecosystem by referencing existing implementations
- Provide insights about application functionality and data based on actual codebase content

As Arcadia, your role is to help users create, manage, and interact with applications within your digital ecosystem.`,
		now.Format("Monday, January 2, 2006 at 3:04 PM MST"),
		now.UTC().Format("2006-01-02 15:04:05 UTC"))

	request := ClaudeRequest{
		Model:     cs.config.Model,
		MaxTokens: cs.config.MaxTokens,
		Messages:  context.Messages,
		Tools:     cs.mcpTools,
		System:    systemPrompt,
	}

	// Log tools being sent to Claude for debugging
	// log.Printf("[Claude Tools] Sending %d tools to Claude:", len(cs.mcpTools))
	// for i, tool := range cs.mcpTools {
	// 	log.Printf("[Claude Tools] %d. %s - %s", i+1, tool.Name, tool.Description)
	// }

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
		return "", fmt.Errorf("claude API error (status %d): %s", resp.StatusCode, string(body))
	}

	var claudeResp ClaudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("no content in Claude response")
	}

	// Build assistant message content (may include both text and tool use)
	var assistantContent []any
	var responseText string
	var hasToolUse bool

	for _, content := range claudeResp.Content {
		switch content.Type {
		case "text":
			responseText = content.Text
			assistantContent = append(assistantContent, map[string]any{
				"type": "text",
				"text": content.Text,
			})
		case "tool_use":
			hasToolUse = true
			assistantContent = append(assistantContent, map[string]any{
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
		var messageContent any
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

func (cs *ClaudeService) RefreshMCPTools() {
	if cs.config.EnableMCP {
		cs.loadMCPTools()
	}
}


func (cs *ClaudeService) loadMCPTools() {
	// Start with static tools
	staticTools := []ClaudeTool{
		{
			Name:        "list_apps",
			Description: "List all registered applications in the Arcadia App Engine",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		// {
		// 	Name:        "create_app",
		// 	Description: loadCreateAppDescription(),
		// 	InputSchema: map[string]any{
		// 		"type": "object",
		// 		"properties": map[string]any{
		// 			"appId": map[string]any{
		// 				"type":        "string",
		// 				"description": "Unique identifier for the application",
		// 			},
		// 			"version": map[string]any{
		// 				"type":        "string",
		// 				"description": "Version of the application",
		// 			},
		// 			"runtime": map[string]any{
		// 				"type":        "string",
		// 				"description": "Runtime for the application (must be 'wasm')",
		// 			},
		// 			"tools": map[string]any{
		// 				"type":        "array",
		// 				"description": "Array of tool definitions",
		// 				"items": map[string]any{
		// 					"type": "object",
		// 					"properties": map[string]any{
		// 						"name": map[string]any{
		// 							"type":        "string",
		// 							"description": "Name of the tool",
		// 						},
		// 						"input_format": map[string]any{
		// 							"type":        "string",
		// 							"description": "Input format (json, xml, etc.)",
		// 						},
		// 					},
		// 					"required": []string{"name", "input_format"},
		// 				},
		// 			},
		// 			"appSrc": map[string]any{
		// 				"type":        "string",
		// 				"description": "Rust trait implementation source code",
		// 			},
		// 			"dependencies": map[string]any{
		// 				"type":        "object",
		// 				"description": "Optional Rust crate dependencies to include in Cargo.toml (e.g., {\"chrono\": \"0.4\", \"regex\": \"1.9\"}). Common dependencies are auto-detected from 'use' statements.",
		// 				"additionalProperties": map[string]any{
		// 					"type":        "string",
		// 					"description": "Version specification for the crate",
		// 				},
		// 			},
		// 		},
		// 		"required": []string{"appId", "version", "runtime", "tools", "appSrc"},
		// 	},
		// },
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
		// {
		//	Name:        "get_document_content",
		//	Description: "Retrieve full content of a specific document by its ID",
		//	InputSchema: map[string]any{
		//		"type": "object",
		//		"properties": map[string]any{
		//			"document_id": map[string]any{
		//				"type":        "string",
		//				"description": "Document ID to retrieve",
		//			},
		//		},
		//		"required": []string{"document_id"},
		//	},
		// },
	}

	// Add dynamic tools from registered apps
	cs.mcpTools = append(staticTools, cs.loadDynamicAppTools()...)
}

func (cs *ClaudeService) loadDynamicAppTools() []ClaudeTool {
	var dynamicTools []ClaudeTool

	// Get registry access
	if registryAccess == nil {
		log.Printf("[Claude Tools] Warning: registryAccess is nil, no dynamic tools will be loaded")
		return dynamicTools
	}

	registry := registryAccess.GetRegistry()
	mutex := registryAccess.GetRegistryMutex()

	mutex.RLock()
	defer mutex.RUnlock()

	log.Printf("[Claude Tools] Loading dynamic tools from registry with %d registered apps", len(registry))

	// Iterate through all registered apps
	for appID, appInterface := range registry {
		// Try to handle as *App struct first (the actual format)
		if app, ok := appInterface.(*App); ok {
			log.Printf("[Claude Tools] App '%s' has %d tools", appID, len(app.Tools))
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
				dynamicTool := ClaudeTool{
					Name:        namespacedName,
					Description: description,
					InputSchema: inputSchema,
				}

				log.Printf("[Claude Tools] Created dynamic tool: %s", namespacedName)
				dynamicTools = append(dynamicTools, dynamicTool)
			}
		} else if appData, ok := appInterface.(map[string]any); ok {
			// Fallback: handle as map[string]any (for backward compatibility)
			if toolsInterface, exists := appData["tools"]; exists {
				if tools, ok := toolsInterface.([]any); ok {
					log.Printf("[Claude Tools] App '%s' has %d tools (map format)", appID, len(tools))
					for _, toolInterface := range tools {
						if tool, ok := toolInterface.(map[string]any); ok {
							// Extract tool information
							toolName, hasName := tool["name"].(string)
							if !hasName {
								log.Printf("[Claude Tools] Skipping tool in app '%s' - no name found", appID)
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
							dynamicTool := ClaudeTool{
								Name:        namespacedName,
								Description: description,
								InputSchema: inputSchema,
							}

							log.Printf("[Claude Tools] Created dynamic tool: %s", namespacedName)
							dynamicTools = append(dynamicTools, dynamicTool)
						} else {
							log.Printf("[Claude Tools] Skipping invalid tool in app '%s' - not a map", appID)
						}
					}
				} else {
					log.Printf("[Claude Tools] App '%s' tools field is not an array", appID)
				}
			} else {
				log.Printf("[Claude Tools] App '%s' has no tools field", appID)
			}
		} else {
			log.Printf("[Claude Tools] App '%s' data is neither *App nor map (type: %T)", appID, appInterface)
		}
	}

	log.Printf("[Claude Tools] Loaded %d dynamic tools total", len(dynamicTools))

	return dynamicTools
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

	var toolResults []map[string]any

	log.Printf("[Claude MCP] Handling tool use response with %d content items (depth: %d)", len(response.Content), depth)

	// Execute each tool and collect results
	for _, content := range response.Content {
		if content.Type == "tool_use" {
			log.Printf("[Claude MCP] Executing tool: %s", content.Name)
			result, err := cs.executeMCPTool(content.Name, content.Input)

			// Create tool result message
			toolResult := map[string]any{
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

func (cs *ClaudeService) executeMCPTool(toolName string, input any) (string, error) {
	log.Printf("[Claude MCP] Executing tool: %s with input: %+v", toolName, input)

	return cs.executeMCPToolDirect(toolName, input)
}

func (cs *ClaudeService) executeMCPToolDirect(toolName string, input any) (string, error) {
	switch toolName {
	case "list_apps":
		return cs.executeListApps()
	case "create_app":
		return cs.executeCreateApp(input)
	case "schedule_app_run":
		return cs.executeScheduleAppRun(input)
	case "list_schedules":
		return cs.executeListSchedules(input)
	case "search_documents":
		return cs.executeSearchDocuments(input)
	case "get_document_content":
		return cs.executeGetDocumentContent(input)
	default:
		// Check if this is a dynamic app tool (format: appId_toolName)
		if strings.Contains(toolName, "_") {
			parts := strings.SplitN(toolName, "_", 2)
			if len(parts) == 2 {
				appID := parts[0]
				appToolName := parts[1]
				return cs.executeDynamicAppTool(appID, appToolName, input)
			}
		}
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

func (cs *ClaudeService) executeDynamicAppTool(appID, toolName string, input any) (string, error) {
	if appRunner == nil {
		return "", fmt.Errorf("app runner not configured")
	}

	// Convert input to JSON for the app runner
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}

	// Execute the app tool directly
	return appRunner.ExecuteAppTool(appID, toolName, inputJSON)
}

func (cs *ClaudeService) executeCreateApp(input any) (string, error) {
	log.Printf("[Claude MCP] executeCreateApp called with input type: %T", input)

	// Convert input to expected structure
	inputJSON, err := json.Marshal(input)
	if err != nil {
		log.Printf("[Claude MCP] Failed to marshal input: %v", err)
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}
	log.Printf("[Claude MCP] Input JSON: %s", string(inputJSON))

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
		log.Printf("[Claude MCP] Failed to parse create app request: %v", err)
		return "", fmt.Errorf("failed to parse create app request: %w", err)
	}

	log.Printf("[Claude MCP] Parsed request - AppID: %s, Version: %s, Runtime: %s, Tools: %d, Dependencies: %d",
		createReq.AppID, createReq.Version, createReq.Runtime, len(createReq.Tools), len(createReq.Dependencies))

	// Validate required fields
	if createReq.AppID == "" {
		log.Printf("[Claude MCP] Validation failed: appId is empty")
		return "", fmt.Errorf("appId is required")
	}
	if createReq.Version == "" {
		log.Printf("[Claude MCP] Validation failed: version is empty")
		return "", fmt.Errorf("version is required")
	}
	if createReq.Runtime != "wasm" {
		log.Printf("[Claude MCP] Validation failed: runtime is '%s', expected 'wasm'", createReq.Runtime)
		return "", fmt.Errorf("runtime must be 'wasm'")
	}
	if createReq.AppSrc == "" {
		log.Printf("[Claude MCP] Validation failed: appSrc is empty")
		return "", fmt.Errorf("appSrc is required")
	}
	if len(createReq.Tools) == 0 {
		log.Printf("[Claude MCP] Validation failed: no tools provided")
		return "", fmt.Errorf("at least one tool is required")
	}

	log.Printf("[Claude MCP] All validations passed, checking appCreator dependency")

	// Use dependency injection to create the app
	if appCreator != nil {
		log.Printf("[Claude MCP] appCreator is available, proceeding with app creation")

		// Convert tools to any slice
		tools := make([]any, len(createReq.Tools))
		for i, tool := range createReq.Tools {
			tools[i] = map[string]any{
				"name":        tool.Name,
				"inputFormat": tool.InputFormat,
			}
		}

		log.Printf("[Claude MCP] Calling appCreator.CreateApp with %d tools", len(tools))
		result, err := appCreator.CreateApp(createReq.AppID, createReq.Version, createReq.Runtime, tools, createReq.AppSrc, createReq.Dependencies)
		if err != nil {
			log.Printf("[Claude MCP] appCreator.CreateApp failed: %v", err)
			return "", fmt.Errorf("app creation failed: %w", err)
		}

		log.Printf("[Claude MCP] App creation successful: %s", result)
		return result, nil
	}

	log.Printf("[Claude MCP] appCreator is nil - dependency injection not properly configured")
	return fmt.Sprintf("App creation request received for %s (version %s)", createReq.AppID, createReq.Version), nil
}

func (cs *ClaudeService) executeScheduleAppRun(input any) (string, error) {
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

func (cs *ClaudeService) executeListSchedules(input any) (string, error) {
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


func (cs *ClaudeService) executeGetDocumentContent(input any) (string, error) {
	if embeddingSearch == nil {
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
	document, err := embeddingSearch.GetDocument(docReq.DocumentID)
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

func (cs *ClaudeService) executeSearchDocuments(input any) (string, error) {
	if embeddingSearch == nil {
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

	// Use the EmbeddingSearch interface directly (embeddingSearch is now EmbeddingSearchAdapter)
	if embeddingSearch != nil {
		// Use empty config since ai.SearchConfig is an empty interface
		var defaultConfig ai.SearchConfig

		// Call the enhanced method through the interface
		results, err := embeddingSearch.SearchDocumentsEnhanced(searchReq.Query, searchReq.TopK, defaultConfig)
		if err != nil {
			return "", fmt.Errorf("enhanced search failed: %w", err)
		}

		// Format enhanced results for Claude
		var formattedResults []map[string]any
		for _, result := range results {
			if result.Document == nil {
				continue
			}

			formattedResult := map[string]any{
				"document_id":        result.Document.ID,
				"file_path":          result.Document.FilePath,
				"full_content":       result.Document.Content,
				"context_highlights": result.ContextHighlights,
				"content_preview":    result.ContentPreview,
				"is_truncated":       result.IsTruncated,
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

	// Fallback to original method if enhanced method is not available
	results, err := embeddingSearch.SearchDocuments(searchReq.Query, searchReq.TopK)
	if err != nil {
		return "", fmt.Errorf("fallback search failed: %w", err)
	}

	// Format results for Claude with complete document information
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
	json.NewEncoder(w).Encode(map[string]any{
		"response": response,
		"context_stats": map[string]int{
			"message_count": messages,
			"total_tokens":  tokens,
		},
	})
}

// Global function to trigger MCP tool refresh - can be called from other services
func TriggerClaudeToolRefresh() {
	if claudeServiceInstance != nil {
		claudeServiceInstance.TriggerToolRefresh()
	} else {
		log.Printf("[Claude MCP] Warning: Claude service not initialized, cannot refresh tools")
	}
}
