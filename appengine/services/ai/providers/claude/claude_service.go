package claude

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"arcadia/services/ai"
)

type ClaudeService struct {
	config         *ai.AIConfig
	claudeConfig   ClaudeConfig
	httpClient     *ai.HTTPClient
	contextManager *ai.ContextManager
	toolManager    *ai.ToolManager
}

// NewClaudeService creates a new Claude AI service with the given configuration
func NewClaudeService(config *ai.AIConfig) (*ClaudeService, error) {
	// Validate that this is a Claude configuration
	if config.Provider != "claude" {
		return nil, fmt.Errorf("invalid provider for Claude service: %s", config.Provider)
	}

	// Parse Claude-specific configuration
	claudeConfig, err := parseClaudeConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Claude config: %w", err)
	}

	// Create HTTP client with Claude-specific headers
	httpClient := ai.NewHTTPClient(config.TimeoutSeconds)
	httpClient.SetBaseURL(claudeConfig.BaseURL)
	httpClient.SetHeader("x-api-key", claudeConfig.APIKey)
	httpClient.SetHeader("anthropic-version", "2023-06-01")

	// Create context and tool managers
	contextManager := ai.NewContextManager(config)
	toolManager := ai.NewToolManager(config)

	service := &ClaudeService{
		config:         config,
		claudeConfig:   claudeConfig,
		httpClient:     httpClient,
		contextManager: contextManager,
		toolManager:    toolManager,
	}

	// Load tools if MCP is enabled
	if config.EnableMCP {
		toolManager.LoadMCPTools()
	}

	// Start background cleanup if TTL is enabled
	if config.ContextTTLMinutes > 0 {
		go service.contextCleanupRoutine()
	}

	return service, nil
}

// Implement AIService interface methods
func (cs *ClaudeService) SendMessage(message string) (string, error) {
	return cs.SendMessageWithContext(message, "default")
}

func (cs *ClaudeService) SendMessageWithContext(message string, contextID string) (string, error) {
	// Add user message to context
	userMessage := ai.Message{
		Role:    ai.RoleUser,
		Content: message,
	}
	cs.contextManager.AddMessage(contextID, userMessage)

	// Call Claude API with context
	return cs.callClaudeWithContext(contextID)
}

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

func (cs *ClaudeService) RefreshTools() {
	cs.toolManager.RefreshTools()
}

func (cs *ClaudeService) TriggerToolRefresh() {
	cs.RefreshTools()
}

func (cs *ClaudeService) GetProviderInfo() ai.ProviderInfo {
	return ai.ProviderInfo{
		Name:     "claude",
		Model:    cs.claudeConfig.Model,
		Features: []string{"tools", "context", "system_prompts"},
	}
}

func (cs *ClaudeService) Validate() error {
	if cs.claudeConfig.APIKey == "" {
		return &ai.AIError{
			Type:     ai.ErrorTypeAuth,
			Message:  "Claude API key is required",
			Provider: "claude",
		}
	}
	if cs.claudeConfig.BaseURL == "" {
		return &ai.AIError{
			Type:     ai.ErrorTypeValidation,
			Message:  "Claude base URL is required",
			Provider: "claude",
		}
	}
	return nil
}

// GetToolManager returns the tool manager for dependency injection
func (cs *ClaudeService) GetToolManager() *ai.ToolManager {
	return cs.toolManager
}

// Claude-specific implementation methods
func (cs *ClaudeService) callClaudeWithContext(contextID string) (string, error) {
	return cs.callClaudeWithContextInternal(contextID, 0)
}

func (cs *ClaudeService) callClaudeWithContextInternal(contextID string, depth int) (string, error) {
	// Get context
	context, exists := cs.contextManager.GetContext(contextID)
	if !exists {
		return "", fmt.Errorf("context not found: %s", contextID)
	}

	// Convert generic messages to Claude format
	claudeMessages := cs.convertToClaudeMessages(context.Messages)

	// Build Claude request
	now := time.Now()
	systemPrompt := cs.buildSystemPrompt(now)

	claudeTools := cs.convertToClaudeTools(cs.toolManager.GetTools())

	request := ClaudeRequest{
		Model:     cs.claudeConfig.Model,
		MaxTokens: cs.config.MaxTokens,
		Messages:  claudeMessages,
		Tools:     claudeTools,
		System:    systemPrompt,
	}

	// Make API call
	resp, err := cs.httpClient.PostJSON("/v1/messages", request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := cs.httpClient.ValidateResponse(resp, "claude"); err != nil {
		return "", err
	}

	var claudeResp ClaudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Process response and handle tool calls
	return cs.processClaudeResponse(claudeResp, contextID, depth)
}

// Helper methods for type conversion and message processing
func (cs *ClaudeService) convertToClaudeMessages(messages []ai.Message) []ClaudeMessage {
	claudeMessages := make([]ClaudeMessage, len(messages))
	for i, msg := range messages {
		claudeMessages[i] = ClaudeMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}
	return claudeMessages
}

func (cs *ClaudeService) convertToClaudeTools(tools []ai.Tool) []ClaudeTool {
	claudeTools := make([]ClaudeTool, len(tools))
	for i, tool := range tools {
		claudeTools[i] = ClaudeTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
	}
	return claudeTools
}

func (cs *ClaudeService) buildSystemPrompt(now time.Time) string {
	return fmt.Sprintf(`Current date and time: %s (UTC: %s).

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
}

func (cs *ClaudeService) processClaudeResponse(response ClaudeResponse, contextID string, depth int) (string, error) {
	if len(response.Content) == 0 {
		return "", fmt.Errorf("no content in Claude response")
	}

	// Build assistant message content (may include both text and tool use)
	var assistantContent []any
	var responseText string
	var hasToolUse bool

	for _, content := range response.Content {
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
			assistantMessage := ai.Message{
				Role:    ai.RoleAssistant,
				Content: messageContent,
			}
			cs.contextManager.AddMessage(contextID, assistantMessage)
		}

		// Update token count if available
		if response.Usage.InputTokens > 0 || response.Usage.OutputTokens > 0 {
			cs.contextManager.UpdateTokenCount(contextID, response.Usage.InputTokens+response.Usage.OutputTokens)
		}
	}

	// If Claude wants to use tools, handle them and continue conversation
	if hasToolUse || response.StopReason == "tool_use" {
		return cs.handleToolUseWithLoopInternal(response, contextID, depth)
	}

	return responseText, nil
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
				result, err := cs.toolManager.ExecuteTool(content.Name, content.Input)
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
			result, err := cs.toolManager.ExecuteTool(content.Name, content.Input)

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
	toolResultMessage := ai.Message{
		Role:    ai.RoleUser,
		Content: toolResults,
	}
	cs.contextManager.AddMessage(contextID, toolResultMessage)

	// Call Claude again with the tool results to get the final response
	log.Printf("[Claude MCP] Calling Claude again with tool results...")
	return cs.callClaudeWithContextInternal(contextID, depth+1)
}

// Configuration parsing helper
func parseClaudeConfig(config *ai.AIConfig) (ClaudeConfig, error) {
	claudeConfig := ClaudeConfig{
		MaxTokens:          config.MaxTokens,
		TimeoutSeconds:     config.TimeoutSeconds,
		EnableMCP:          config.EnableMCP,
		MCPServerCmd:       config.MCPServerCmd,
		MaxContextMessages: config.MaxContextMessages,
		ContextCompaction:  config.ContextCompaction,
		ContextTTLMinutes:  config.ContextTTLMinutes,
	}

	// Extract Claude-specific settings
	claudeConfig.APIKey = config.GetProviderString("api_key")
	claudeConfig.BaseURL = config.GetProviderString("base_url")
	claudeConfig.Model = config.GetProviderString("model")

	// Set defaults
	if claudeConfig.BaseURL == "" {
		claudeConfig.BaseURL = "https://api.anthropic.com"
	}
	if claudeConfig.Model == "" {
		claudeConfig.Model = "claude-3-5-sonnet-20241022"
	}

	return claudeConfig, nil
}

// Background cleanup routine
func (cs *ClaudeService) contextCleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cs.contextManager.CleanupExpiredContexts(cs.config.ContextTTLMinutes)
	}
}

// HTTP Handler Adapter for backwards compatibility
func (cs *ClaudeService) HandleClaudeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Message   string `json:"message"`
		SessionID string `json:"session_id"`
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

// SetDependencies allows injection of tool manager dependencies
func (cs *ClaudeService) SetDependencies(registryAccess ai.RegistryAccess, appRunner ai.AppRunner, appCreator ai.AppCreator, embeddingSearch ai.EmbeddingSearch) {
	cs.toolManager.SetRegistryAccess(registryAccess)
	cs.toolManager.SetAppRunner(appRunner)
	cs.toolManager.SetAppCreator(appCreator)
	cs.toolManager.SetEmbeddingSearch(embeddingSearch)
}

// CreateClaudeConfig creates a new AI config for Claude with reasonable defaults
func CreateClaudeConfig(apiKey, model string, maxTokens int) *ai.AIConfig {
	config := &ai.AIConfig{
		Provider:           "claude",
		MaxTokens:          maxTokens,
		TimeoutSeconds:     120,
		MaxContextMessages: 20,
		ContextCompaction:  true,
		ContextTTLMinutes:  60,
		EnableMCP:          true,
		ProviderSettings: map[string]any{
			"api_key":  apiKey,
			"base_url": "https://api.anthropic.com",
			"model":    model,
		},
	}

	// Set default model if not provided
	if model == "" {
		config.ProviderSettings["model"] = "claude-3-5-sonnet-20241022"
	}

	return config
}
