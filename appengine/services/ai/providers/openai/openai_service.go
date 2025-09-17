package openai

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"arcadia/services/ai"
)

type OpenAIService struct {
	config         *ai.AIConfig
	openaiConfig   OpenAIConfig
	httpClient     *ai.HTTPClient
	contextManager *ai.ContextManager
	toolManager    *ai.ToolManager
}

func NewOpenAIService(config *ai.AIConfig) (*OpenAIService, error) {
	log.Printf("[OpenAI Service] Initializing OpenAI service...")

	// Validate that this is an OpenAI configuration
	if config.Provider != "openai" {
		log.Printf("[OpenAI Service] Error: Invalid provider for OpenAI service: %s", config.Provider)
		return nil, fmt.Errorf("invalid provider for OpenAI service: %s", config.Provider)
	}

	// Parse OpenAI-specific configuration
	openaiConfig, err := parseOpenAIConfig(config)
	if err != nil {
		log.Printf("[OpenAI Service] Error: Failed to parse OpenAI config: %v", err)
		return nil, fmt.Errorf("failed to parse OpenAI config: %w", err)
	}
	log.Printf("[OpenAI Service] Configuration parsed successfully - Model: %s, BaseURL: %s", openaiConfig.Model, openaiConfig.BaseURL)

	// Create HTTP client with OpenAI-specific headers
	log.Printf("[OpenAI Service] Creating HTTP client with timeout: %d seconds", config.TimeoutSeconds)
	httpClient := ai.NewHTTPClient(config.TimeoutSeconds)
	httpClient.SetBaseURL(openaiConfig.BaseURL)
	httpClient.SetHeader("Authorization", "Bearer "+openaiConfig.APIKey)
	httpClient.SetHeader("Content-Type", "application/json")

	if openaiConfig.Organization != "" {
		httpClient.SetHeader("OpenAI-Organization", openaiConfig.Organization)
		log.Printf("[OpenAI Service] Using organization: %s", openaiConfig.Organization)
	}
	if openaiConfig.Project != "" {
		httpClient.SetHeader("OpenAI-Project", openaiConfig.Project)
		log.Printf("[OpenAI Service] Using project: %s", openaiConfig.Project)
	}

	// Create context and tool managers
	log.Printf("[OpenAI Service] Creating context manager with TTL: %d minutes", config.ContextTTLMinutes)
	contextManager := ai.NewContextManager(config)
	log.Printf("[OpenAI Service] Creating tool manager with MCP enabled: %t", config.EnableMCP)
	toolManager := ai.NewToolManager(config)

	service := &OpenAIService{
		config:         config,
		openaiConfig:   openaiConfig,
		httpClient:     httpClient,
		contextManager: contextManager,
		toolManager:    toolManager,
	}

	// Load tools if MCP is enabled
	if config.EnableMCP {
		log.Printf("[OpenAI MCP] Loading MCP tools...")
		toolManager.LoadMCPTools()
		tools := toolManager.GetTools()
		log.Printf("[OpenAI MCP] Loaded %d tools", len(tools))
		for _, tool := range tools {
			log.Printf("[OpenAI MCP] Available tool: %s - %s", tool.Name, tool.Description)
		}
	} else {
		log.Printf("[OpenAI MCP] MCP disabled, no tools loaded")
	}

	// Start background cleanup if TTL is enabled
	if config.ContextTTLMinutes > 0 {
		log.Printf("[OpenAI Service] Starting background context cleanup routine")
		go service.contextCleanupRoutine()
	}

	log.Printf("[OpenAI Service] Service initialization completed successfully")
	return service, nil
}

// Implement AIService interface methods
func (os *OpenAIService) SendMessage(message string) (string, error) {
	log.Printf("[OpenAI Service] Sending message to default context, length: %d chars", len(message))
	return os.SendMessageWithContext(message, "default")
}

func (os *OpenAIService) SendMessageWithContext(message string, contextID string) (string, error) {
	log.Printf("[OpenAI Service] Received message for context %s, length: %d chars", contextID, len(message))

	// Add user message to context
	userMessage := ai.Message{
		Role:    ai.RoleUser,
		Content: message,
	}
	os.contextManager.AddMessage(contextID, userMessage)
	log.Printf("[OpenAI Service] Added user message to context %s", contextID)

	// Call OpenAI API with context
	log.Printf("[OpenAI Service] Calling OpenAI API for context %s", contextID)
	return os.callOpenAIWithContext(contextID)
}

func (os *OpenAIService) ClearContext(contextID string) {
	log.Printf("[OpenAI Service] Clearing context: %s", contextID)
	os.contextManager.ClearContext(contextID)
}

func (os *OpenAIService) GetContextStats(contextID string) (messages int, tokens int, exists bool) {
	context, exists := os.contextManager.GetContext(contextID)
	if !exists {
		log.Printf("[OpenAI Service] Context stats requested for non-existent context: %s", contextID)
		return 0, 0, false
	}
	log.Printf("[OpenAI Service] Context stats for %s: %d messages, %d tokens", contextID, len(context.Messages), context.TotalTokens)
	return len(context.Messages), context.TotalTokens, true
}

func (os *OpenAIService) RefreshTools() {
	log.Printf("[OpenAI MCP] Refreshing tools...")
	os.toolManager.RefreshTools()
	tools := os.toolManager.GetTools()
	log.Printf("[OpenAI MCP] Tool refresh completed, %d tools available", len(tools))
}

func (os *OpenAIService) TriggerToolRefresh() {
	log.Printf("[OpenAI MCP] Tool refresh triggered")
	os.RefreshTools()
}

func (os *OpenAIService) GetProviderInfo() ai.ProviderInfo {
	return ai.ProviderInfo{
		Name:     "openai",
		Model:    os.openaiConfig.Model,
		Features: []string{"tools", "context", "streaming"},
	}
}

func (os *OpenAIService) Validate() error {
	log.Printf("[OpenAI Service] Validating OpenAI service configuration")

	if os.openaiConfig.APIKey == "" {
		log.Printf("[OpenAI Service] Validation failed: API key is missing")
		return &ai.AIError{
			Type:     ai.ErrorTypeAuth,
			Message:  "OpenAI API key is required",
			Provider: "openai",
		}
	}
	if os.openaiConfig.BaseURL == "" {
		log.Printf("[OpenAI Service] Validation failed: Base URL is missing")
		return &ai.AIError{
			Type:     ai.ErrorTypeValidation,
			Message:  "OpenAI base URL is required",
			Provider: "openai",
		}
	}

	log.Printf("[OpenAI Service] Validation successful")
	return nil
}

// GetToolManager returns the tool manager for dependency injection
func (os *OpenAIService) GetToolManager() *ai.ToolManager {
	return os.toolManager
}

// OpenAI-specific implementation methods
func (os *OpenAIService) callOpenAIWithContext(contextID string) (string, error) {
	log.Printf("[OpenAI Service] Processing request for context: %s", contextID)

	// Get context
	context, exists := os.contextManager.GetContext(contextID)
	if !exists {
		log.Printf("[OpenAI Service] Error: Context not found: %s", contextID)
		return "", fmt.Errorf("context not found: %s", contextID)
	}
	log.Printf("[OpenAI Service] Context found with %d messages, %d tokens", len(context.Messages), context.TotalTokens)

	// Convert generic messages to OpenAI format
	log.Printf("[OpenAI Service] Converting %d messages to OpenAI format", len(context.Messages))
	openaiMessages := os.convertToOpenAIMessages(context.Messages)

	// Add system message if needed (OpenAI puts system context as first message)
	systemMessage := os.buildSystemMessage()
	if systemMessage != "" {
		systemMsg := OpenAIMessage{
			Role:    "system",
			Content: systemMessage,
		}
		openaiMessages = append([]OpenAIMessage{systemMsg}, openaiMessages...)
	}

	// Build OpenAI request
	tools := os.toolManager.GetTools()
	log.Printf("[OpenAI Tools] Converting %d tools to OpenAI format", len(tools))
	for i, tool := range tools {
		log.Printf("[OpenAI Tools] Tool %d: %s - %s", i+1, tool.Name, tool.Description)
	}
	openaiTools := os.convertToOpenAITools(tools)

	request := OpenAIRequest{
		Model:     os.openaiConfig.Model,
		Messages:  openaiMessages,
		Tools:     openaiTools,
		MaxTokens: os.config.MaxTokens,
		Stream:    false,
	}

	// Set tool_choice to auto to let OpenAI choose when to use tools
	if len(openaiTools) > 0 {
		request.ToolChoice = "auto"
		log.Printf("[OpenAI Tools] Set tool_choice to 'auto' for %d available tools", len(openaiTools))
	}

	// Only include temperature if it's not the default value of 1.0
	if os.openaiConfig.Temperature != 1.0 {
		request.Temperature = os.openaiConfig.Temperature
	}

	// Validate message sequence before sending to OpenAI
	if err := os.validateMessageSequence(request.Messages); err != nil {
		log.Printf("[OpenAI Error] Message sequence validation failed: %v", err)
		return "", fmt.Errorf("invalid message sequence for OpenAI: %w", err)
	}

	// Debug: Log the message sequence to ensure proper ordering
	log.Printf("[OpenAI Debug] Message sequence validation passed:")
	for i, msg := range request.Messages {
		if msg.Role == "tool" {
			log.Printf("[OpenAI Debug] Message %d: role=%s, tool_call_id=%s, content_length=%d", i, msg.Role, msg.ToolCallID, len(msg.Content))
		} else if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			log.Printf("[OpenAI Debug] Message %d: role=%s, tool_calls_count=%d", i, msg.Role, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				log.Printf("[OpenAI Debug]   Tool call %d: id=%s, name=%s", j, tc.ID, tc.Function.Name)
			}
		} else {
			log.Printf("[OpenAI Debug] Message %d: role=%s, content_length=%d", i, msg.Role, len(msg.Content))
		}
	}

	// Debug: Log the full request being sent to OpenAI (but truncate for readability)
	requestJSON, _ := json.Marshal(request)
	if len(requestJSON) > 2000 {
		log.Printf("[OpenAI Debug] Full request (truncated): %s...", string(requestJSON[:2000]))
	} else {
		log.Printf("[OpenAI Debug] Full request: %s", string(requestJSON))
	}

	// Make API call
	log.Printf("[OpenAI Service] Making API call to /v1/chat/completions with %d messages, %d tools", len(request.Messages), len(request.Tools))
	resp, err := os.httpClient.PostJSON("/v1/chat/completions", request)
	if err != nil {
		log.Printf("[OpenAI Service] API call failed: %v", err)
		return "", err
	}
	defer resp.Body.Close()
	log.Printf("[OpenAI Service] API call completed with status: %d", resp.StatusCode)

	if err := os.httpClient.HandleErrorResponse(resp, "openai"); err != nil {
		log.Printf("[OpenAI Service] Error response from API: %v", err)
		return "", err
	}

	var openaiResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		log.Printf("[OpenAI Service] Failed to decode response: %v", err)
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	log.Printf("[OpenAI Service] Response decoded successfully with %d choices", len(openaiResp.Choices))

	// Debug: Log the full response from OpenAI
	responseJSON, _ := json.Marshal(openaiResp)
	log.Printf("[OpenAI Debug] Full response: %s", string(responseJSON))

	// Process response and handle tool calls
	log.Printf("[OpenAI Service] Processing OpenAI response for context %s", contextID)
	return os.processOpenAIResponse(openaiResp, contextID)
}

// Helper methods for type conversion and message processing
func (os *OpenAIService) convertToOpenAIMessages(messages []ai.Message) []OpenAIMessage {
	var openaiMessages []OpenAIMessage

	log.Printf("[OpenAI Conversion] Converting %d messages to OpenAI format", len(messages))

	for i, msg := range messages {
		log.Printf("[OpenAI Conversion] Message %d: role=%s, tool_calls=%d, tool_call_id=%s",
			i, string(msg.Role), len(msg.ToolCalls), msg.ToolCallID)

		openaiMsg := OpenAIMessage{
			Role: string(msg.Role),
		}

		// Handle content - can be string or structured
		if content, ok := msg.Content.(string); ok {
			openaiMsg.Content = content
		} else if contentArray, ok := msg.Content.([]any); ok {
			// Handle structured content (tool calls, etc.)
			os.processStructuredContent(&openaiMsg, contentArray)
		} else if msg.Content != nil {
			// Convert any other content type to JSON string
			contentJSON, _ := json.Marshal(msg.Content)
			openaiMsg.Content = string(contentJSON)
		}

		// Handle tool calls for assistant messages
		if len(msg.ToolCalls) > 0 {
			log.Printf("[OpenAI Conversion] Message %d: converting %d tool calls", i, len(msg.ToolCalls))
			openaiMsg.ToolCalls = os.convertToOpenAIToolCalls(msg.ToolCalls)
			for j, tc := range openaiMsg.ToolCalls {
				log.Printf("[OpenAI Conversion] Message %d tool call %d: id=%s, function=%s",
					i, j, tc.ID, tc.Function.Name)
			}
		}

		// Handle tool call ID for tool role messages (required by OpenAI)
		if msg.Role == ai.RoleTool {
			if msg.ToolCallID == "" {
				log.Printf("[OpenAI Conversion] WARNING: Message %d tool message missing tool_call_id", i)
			} else {
				openaiMsg.ToolCallID = msg.ToolCallID
				log.Printf("[OpenAI Conversion] Message %d: setting tool_call_id=%s for tool message", i, msg.ToolCallID)
			}

			// Ensure tool messages have non-empty content
			if openaiMsg.Content == "" {
				openaiMsg.Content = "Empty tool response"
				log.Printf("[OpenAI Conversion] Message %d: tool message had empty content, setting default", i)
			}
		}

		openaiMessages = append(openaiMessages, openaiMsg)
	}

	log.Printf("[OpenAI Conversion] Conversion completed: %d messages converted", len(openaiMessages))
	return openaiMessages
}

func (os *OpenAIService) convertToOpenAITools(tools []ai.Tool) []OpenAITool {
	var openaiTools []OpenAITool

	for _, tool := range tools {
		openaiTool := OpenAITool{
			Type: "function",
			Function: OpenAIFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
		openaiTools = append(openaiTools, openaiTool)
	}

	return openaiTools
}

func (os *OpenAIService) convertToOpenAIToolCalls(toolCalls []ai.ToolCall) []OpenAIToolCall {
	var openaiToolCalls []OpenAIToolCall

	for _, tc := range toolCalls {
		// Convert input to JSON string as required by OpenAI
		inputJSON, _ := json.Marshal(tc.Input)

		openaiTC := OpenAIToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: OpenAIFunctionCall{
				Name:      tc.Name,
				Arguments: string(inputJSON),
			},
		}
		openaiToolCalls = append(openaiToolCalls, openaiTC)
	}

	return openaiToolCalls
}

func (os *OpenAIService) buildSystemMessage() string {
	// Create system message similar to Claude's system prompt
	now := time.Now()
	return fmt.Sprintf(`Current date and time: %s (UTC: %s).

You are Arcadia, an AI-powered digital ecosystem where applications grow and flourish together.

ABOUT ARCADIA:
Arcadia is a digital ecosystem where applications grow and flourish together. It's a WASM-based application platform that allows developers to create and deploy applications that can interact with each other, access shared databases, and leverage AI capabilities.

IMPORTANT INSTRUCTIONS:
1. When users ask you to create, build, develop, or implement applications, use the appropriate tools to accomplish their goals
2. You have access to an application registry that contains all registered applications and their available tools
3. You can search through documents that may contain relevant information to help users - ALWAYS use the search_documents tool when users ask questions about existing code, files, or documentation
4. Always be helpful and provide accurate information about the Arcadia platform and its capabilities
5. When working with WASM applications, ensure proper tool definitions and input formats

TOOL USAGE GUIDELINES:
- ALWAYS use search_documents when users ask about existing content, files, documentation, or code
- Use list_apps to show available applications
- Use appropriate app tools (appId_toolName format) to execute application functionality
- Use create_app to build new WASM applications
- Use schedule_app_run to schedule tasks

AVAILABLE CAPABILITIES:
- List registered applications and their tools (use list_apps)
- Execute tools from registered applications (use appId_toolName format)
- Search and retrieve document content (use search_documents)
- Schedule application executions (use schedule_app_run)
- Create new WASM applications (use create_app)

Your goal is to help users maximize the potential of the Arcadia platform by providing guidance, executing tools, and facilitating application development and interaction.

REMEMBER: When users ask questions that might be answered by existing documentation or code, ALWAYS use the search_documents tool first.`,
		now.Format("Monday, January 2, 2006 at 3:04 PM MST"),
		now.UTC().Format("2006-01-02 15:04:05 UTC"))
}

func (os *OpenAIService) processStructuredContent(openaiMsg *OpenAIMessage, contentArray []any) {
	// For now, convert structured content to string representation
	// This handles cases where content might be more complex than just text
	contentParts := make([]string, len(contentArray))
	for i, part := range contentArray {
		if partStr, ok := part.(string); ok {
			contentParts[i] = partStr
		} else {
			// Convert complex content to JSON string
			partJSON, _ := json.Marshal(part)
			contentParts[i] = string(partJSON)
		}
	}
	openaiMsg.Content = strings.Join(contentParts, " ")
}

func (os *OpenAIService) processOpenAIResponse(response OpenAIResponse, contextID string) (string, error) {
	if len(response.Choices) == 0 {
		log.Printf("[OpenAI Service] Error: No choices in OpenAI response")
		return "", fmt.Errorf("no choices in OpenAI response")
	}

	choice := response.Choices[0]
	log.Printf("[OpenAI Service] Processing choice with %d tool calls", len(choice.Message.ToolCalls))
	log.Printf("[OpenAI Debug] Choice finish_reason: %s", choice.FinishReason)
	log.Printf("[OpenAI Debug] Choice message content length: %d", len(choice.Message.Content))

	if len(choice.Message.ToolCalls) == 0 {
		log.Printf("[OpenAI Debug] No tool calls found - OpenAI chose to respond directly")
		contentLen := len(choice.Message.Content)
		previewLen := 200
		if contentLen < previewLen {
			previewLen = contentLen
		}
		log.Printf("[OpenAI Debug] Message content preview (first %d chars): %s", previewLen, choice.Message.Content[:previewLen])
	} else {
		log.Printf("[OpenAI Debug] Tool calls found!")
		for i, tc := range choice.Message.ToolCalls {
			log.Printf("[OpenAI Debug] Tool call %d: %s (ID: %s) Args: %s", i+1, tc.Function.Name, tc.ID, tc.Function.Arguments)
		}
	}

	// Add assistant's response to context
	assistantMessage := ai.Message{
		Role:    ai.RoleAssistant,
		Content: choice.Message.Content,
	}

	// Handle tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		log.Printf("[OpenAI Tools] Processing %d tool calls", len(choice.Message.ToolCalls))

		// Convert OpenAI tool calls to generic format
		var toolCalls []ai.ToolCall
		for _, tc := range choice.Message.ToolCalls {
			log.Printf("[OpenAI Tools] Converting tool call: %s (ID: %s)", tc.Function.Name, tc.ID)
			var input any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				log.Printf("[OpenAI Tools] Warning: Failed to unmarshal tool arguments for %s: %v", tc.Function.Name, err)
			}

			toolCall := ai.ToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			}
			toolCalls = append(toolCalls, toolCall)
		}
		assistantMessage.ToolCalls = toolCalls

		// Add assistant message with tool calls to context
		log.Printf("[OpenAI Service] Adding assistant message with tool calls to context %s", contextID)
		os.contextManager.AddMessage(contextID, assistantMessage)

		// Execute tools and continue conversation
		log.Printf("[OpenAI Tools] Executing %d tool calls", len(toolCalls))
		return os.handleToolCalls(toolCalls, contextID)
	}

	log.Printf("[OpenAI Service] Adding assistant message to context %s", contextID)
	os.contextManager.AddMessage(contextID, assistantMessage)

	// Update token count
	if response.Usage.TotalTokens > 0 {
		log.Printf("[OpenAI Service] Updating token count for context %s: %d tokens", contextID, response.Usage.TotalTokens)
		os.contextManager.UpdateTokenCount(contextID, response.Usage.TotalTokens)
	}

	log.Printf("[OpenAI Service] Returning response for context %s, length: %d chars", contextID, len(choice.Message.Content))
	return choice.Message.Content, nil
}

func (os *OpenAIService) handleToolCalls(toolCalls []ai.ToolCall, contextID string) (string, error) {
	log.Printf("[OpenAI Tools] Handling %d tool calls for context %s", len(toolCalls), contextID)

	// Execute each tool call and collect results
	var toolResults []ai.Message

	for _, tc := range toolCalls {
		log.Printf("[OpenAI Tools] Executing tool: %s (ID: %s)", tc.Name, tc.ID)

		// Create tool result message with proper format
		toolResult := ai.Message{
			Role:       ai.RoleTool,
			ToolCallID: tc.ID, // Must match the exact tool call ID from OpenAI
		}

		// Safely execute the tool with error handling
		result, err := os.toolManager.ExecuteTool(tc.Name, tc.Input)
		if err != nil {
			log.Printf("[OpenAI Tools] Tool %s failed: %v", tc.Name, err)
			// Provide error in a format OpenAI can understand
			toolResult.Content = fmt.Sprintf("Error executing tool %s: %v", tc.Name, err)
		} else {
			log.Printf("[OpenAI Tools] Tool %s succeeded, result length: %d", tc.Name, len(result))
			toolResult.Content = result
		}

		// Ensure content is always a non-empty string (OpenAI requirement)
		if toolResult.Content == nil || toolResult.Content == "" {
			toolResult.Content = "Tool executed successfully with no output"
		}

		// Ensure content is a string, not complex object
		if contentStr, ok := toolResult.Content.(string); ok {
			toolResult.Content = contentStr
		} else {
			// Convert non-string content to JSON string
			contentJSON, _ := json.Marshal(toolResult.Content)
			toolResult.Content = string(contentJSON)
		}

		toolResults = append(toolResults, toolResult)
	}

	// Add all tool results to context in order (maintaining OpenAI message sequence requirements)
	log.Printf("[OpenAI Tools] Adding %d tool results to context %s", len(toolResults), contextID)
	for i, tr := range toolResults {
		log.Printf("[OpenAI Tools] Adding tool result %d: tool_call_id=%s, content_length=%d", i+1, tr.ToolCallID, len(tr.Content.(string)))
		os.contextManager.AddMessage(contextID, tr)
	}

	// Call OpenAI again with tool results
	log.Printf("[OpenAI Tools] Calling OpenAI again with tool results for context %s", contextID)
	return os.callOpenAIWithContext(contextID)
}

// Configuration parsing helper
func parseOpenAIConfig(config *ai.AIConfig) (OpenAIConfig, error) {
	openaiConfig := OpenAIConfig{
		MaxTokens:      config.MaxTokens,
		TimeoutSeconds: config.TimeoutSeconds,
	}

	// Extract OpenAI-specific settings
	openaiConfig.APIKey = config.GetProviderString("api_key")
	openaiConfig.BaseURL = config.GetProviderString("base_url")
	openaiConfig.Model = config.GetProviderString("model")
	openaiConfig.Organization = config.GetProviderString("organization")
	openaiConfig.Project = config.GetProviderString("project")

	// Handle temperature (might be stored as float64 or string)
	if tempVal, exists := config.ProviderSettings["temperature"]; exists {
		if temp, ok := tempVal.(float64); ok {
			openaiConfig.Temperature = temp
		} else if tempStr, ok := tempVal.(string); ok {
			fmt.Sscanf(tempStr, "%f", &openaiConfig.Temperature)
		}
	}

	// Set defaults
	if openaiConfig.BaseURL == "" {
		openaiConfig.BaseURL = "https://api.openai.com"
	}
	if openaiConfig.Model == "" {
		openaiConfig.Model = "gpt-4"
	}
	if openaiConfig.Temperature == 0 {
		openaiConfig.Temperature = 1.0
	}

	return openaiConfig, nil
}

// Background cleanup routine
func (os *OpenAIService) contextCleanupRoutine() {
	log.Printf("[OpenAI Service] Starting context cleanup routine with %d minute TTL", os.config.ContextTTLMinutes)
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		log.Printf("[OpenAI Service] Running context cleanup...")
		os.contextManager.CleanupExpiredContexts(os.config.ContextTTLMinutes)
	}
}

// HTTP Handler adapter (for consistency with Claude implementation)
func (os *OpenAIService) HandleOpenAIAPI(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("[OpenAI Service] HTTP API request for context %s, message length: %d", contextID, len(request.Message))
	response, err := os.SendMessageWithContext(request.Message, contextID)
	if err != nil {
		log.Printf("[OpenAI Service] HTTP API error for context %s: %v", contextID, err)
		http.Error(w, "Failed to get OpenAI response", http.StatusInternalServerError)
		return
	}
	log.Printf("[OpenAI Service] HTTP API response for context %s, length: %d", contextID, len(response))

	// Include context stats in response
	messages, tokens, _ := os.GetContextStats(contextID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"response": response,
		"context_stats": map[string]int{
			"message_count": messages,
			"total_tokens":  tokens,
		},
	})
}

// SetRegistryAccess allows setting the registry access for the tool manager
func (os *OpenAIService) SetRegistryAccess(ra ai.RegistryAccess) {
	log.Printf("[OpenAI MCP] Setting registry access for tool manager")
	os.toolManager.SetRegistryAccess(ra)
}

// SetAppRunner allows setting the app runner for the tool manager
func (os *OpenAIService) SetAppRunner(ar ai.AppRunner) {
	log.Printf("[OpenAI MCP] Setting app runner for tool manager")
	os.toolManager.SetAppRunner(ar)
}

// SetAppCreator allows setting the app creator for the tool manager
func (os *OpenAIService) SetAppCreator(ac ai.AppCreator) {
	log.Printf("[OpenAI MCP] Setting app creator for tool manager")
	os.toolManager.SetAppCreator(ac)
}

// SetEmbeddingSearch allows setting the embedding search service for the tool manager
func (os *OpenAIService) SetEmbeddingSearch(es ai.EmbeddingSearch) {
	log.Printf("[OpenAI MCP] Setting embedding search for tool manager")
	os.toolManager.SetEmbeddingSearch(es)
}

// GetTools returns the current list of tools (for debugging and testing)
func (os *OpenAIService) GetTools() []ai.Tool {
	if os.toolManager != nil {
		return os.toolManager.GetTools()
	}
	return []ai.Tool{}
}

// GetContextManager returns the context manager (for testing)
func (os *OpenAIService) GetContextManager() *ai.ContextManager {
	return os.contextManager
}

// ConvertToOpenAIMessages converts generic AI messages to OpenAI format (for testing)
func (os *OpenAIService) ConvertToOpenAIMessages(messages []ai.Message) []OpenAIMessage {
	return os.convertToOpenAIMessages(messages)
}

// ValidateMessageSequence validates message sequence (for testing)
func (os *OpenAIService) ValidateMessageSequence(messages []OpenAIMessage) error {
	return os.validateMessageSequence(messages)
}

// validateMessageSequence ensures the message sequence follows OpenAI's requirements
func (os *OpenAIService) validateMessageSequence(messages []OpenAIMessage) error {
	log.Printf("[OpenAI Validation] Starting validation of %d messages", len(messages))

	// Track tool call IDs that need corresponding tool responses
	pendingToolCalls := make(map[string]bool)

	for i, msg := range messages {
		log.Printf("[OpenAI Validation] Message %d: role=%s", i, msg.Role)

		switch msg.Role {
		case "assistant":
			// If assistant has tool calls, track their IDs
			if len(msg.ToolCalls) > 0 {
				log.Printf("[OpenAI Validation] Message %d: found %d tool calls", i, len(msg.ToolCalls))
				for j, tc := range msg.ToolCalls {
					if tc.ID == "" {
						log.Printf("[OpenAI Validation] ERROR: Message %d tool call %d missing ID", i, j)
						return fmt.Errorf("message %d: assistant tool call missing ID", i)
					}
					pendingToolCalls[tc.ID] = true
					log.Printf("[OpenAI Validation] Message %d: tracking tool call ID: %s (function: %s)", i, tc.ID, tc.Function.Name)
				}
			} else {
				log.Printf("[OpenAI Validation] Message %d: assistant message with no tool calls", i)
			}

		case "tool":
			// Tool messages must have tool_call_id and respond to a pending tool call
			log.Printf("[OpenAI Validation] Message %d: tool message with tool_call_id=%s", i, msg.ToolCallID)

			if msg.ToolCallID == "" {
				log.Printf("[OpenAI Validation] ERROR: Message %d tool message missing tool_call_id", i)
				return fmt.Errorf("message %d: tool message missing tool_call_id", i)
			}

			// Debug: Show current pending tool calls
			log.Printf("[OpenAI Validation] Current pending tool calls: %v", pendingToolCalls)

			if !pendingToolCalls[msg.ToolCallID] {
				log.Printf("[OpenAI Validation] ERROR: Message %d tool_call_id '%s' not found in pending calls", i, msg.ToolCallID)
				// Show all available tool call IDs for debugging
				availableIDs := make([]string, 0, len(pendingToolCalls))
				for id := range pendingToolCalls {
					availableIDs = append(availableIDs, id)
				}
				log.Printf("[OpenAI Validation] Available tool call IDs: %v", availableIDs)
				return fmt.Errorf("message %d: tool message with tool_call_id '%s' has no corresponding assistant tool call", i, msg.ToolCallID)
			}

			// Mark this tool call as fulfilled
			delete(pendingToolCalls, msg.ToolCallID)
			log.Printf("[OpenAI Validation] Message %d: fulfilled tool call ID: %s", i, msg.ToolCallID)

			// Tool messages must have non-empty content
			if msg.Content == "" {
				log.Printf("[OpenAI Validation] ERROR: Message %d tool message has empty content", i)
				return fmt.Errorf("message %d: tool message has empty content", i)
			}

		default:
			log.Printf("[OpenAI Validation] Message %d: role=%s (content length: %d)", i, msg.Role, len(msg.Content))
		}
	}

	// Check if there are any unfulfilled tool calls
	if len(pendingToolCalls) > 0 {
		unfulfilled := make([]string, 0, len(pendingToolCalls))
		for id := range pendingToolCalls {
			unfulfilled = append(unfulfilled, id)
		}
		log.Printf("[OpenAI Validation] ERROR: Unfulfilled tool calls: %v", unfulfilled)
		return fmt.Errorf("unfulfilled tool calls: %v", unfulfilled)
	}

	log.Printf("[OpenAI Validation] Message sequence validation successful - all tool calls properly matched")
	return nil
}

