package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"arcadia/modules/ai/interfaces"
	"arcadia/modules/ai/models"
)

// OpenAIProvider implements the AI provider interface for OpenAI
type OpenAIProvider struct {
	config         *OpenAIConfig
	logger         interfaces.Logger
	metrics        interfaces.Metrics
	httpClient     *http.Client
	tools          []models.Tool
	contextManager interfaces.ContextManager
	status         string
	startTime      time.Time
}

// OpenAIConfig holds OpenAI-specific configuration
type OpenAIConfig struct {
	APIKey         string  `json:"api_key"`
	BaseURL        string  `json:"base_url"`
	Model          string  `json:"model"`
	MaxTokens      int     `json:"max_tokens"`
	Temperature    float64 `json:"temperature"`
	TimeoutSeconds int     `json:"timeout_seconds"`
	Organization   string  `json:"organization,omitempty"`
	Project        string  `json:"project,omitempty"`
}

// OpenAI API types
type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	Name       string           `json:"name,omitempty"`
}

type OpenAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"` // Always "function"
	Function OpenAIFunctionCall `json:"function"`
}

type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type OpenAITool struct {
	Type     string            `json:"type"` // Always "function"
	Function OpenAIFunctionDef `json:"function"`
}

type OpenAIFunctionDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Tools       []OpenAITool    `json:"tools,omitempty"`
	ToolChoice  any             `json:"tool_choice,omitempty"`
	MaxTokens   int             `json:"max_completion_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(ctx context.Context, config map[string]interface{}) (*OpenAIProvider, error) {
	// Parse OpenAI-specific configuration
	openaiConfig, err := parseOpenAIConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI config: %w", err)
	}

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: time.Duration(openaiConfig.TimeoutSeconds) * time.Second,
	}

	provider := &OpenAIProvider{
		config:     openaiConfig,
		httpClient: httpClient,
		tools:      make([]models.Tool, 0),
		status:     "initialized",
		startTime:  time.Now(),
	}

	return provider, nil
}

// Provider identification
func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) Type() string {
	return "openai"
}

func (p *OpenAIProvider) Version() string {
	return "1.0.0"
}

// Core messaging capabilities
func (p *OpenAIProvider) SendMessage(ctx context.Context, message string) (string, error) {
	return p.SendMessageWithContext(ctx, message, "default")
}

func (p *OpenAIProvider) SendMessageWithContext(ctx context.Context, message string, contextID string) (string, error) {
	if p.logger != nil {
		p.logger.Info("Sending message with context", "context_id", contextID, "message_length", len(message))
	}

	// Get or create context
	var conversationContext *models.ConversationContext
	if p.contextManager != nil {
		var err error
		conversationContext, err = p.contextManager.GetContext(ctx, contextID)
		if err != nil {
			// Create new context if it doesn't exist
			if err := p.contextManager.CreateContext(ctx, contextID); err != nil {
				return "", fmt.Errorf("failed to create context: %w", err)
			}
			conversationContext, err = p.contextManager.GetContext(ctx, contextID)
			if err != nil {
				return "", fmt.Errorf("failed to get context after creation: %w", err)
			}
		}
	} else {
		// Fallback to basic context
		conversationContext = &models.ConversationContext{
			ID:       contextID,
			Messages: []models.Message{},
		}
	}

	// Only add user message if it's not empty (for tool result processing)
	if message != "" {
		userMessage := models.Message{
			Role:      models.RoleUser,
			Content:   message,
			Timestamp: time.Now(),
		}

		if p.contextManager != nil {
			if err := p.contextManager.AddMessage(ctx, contextID, userMessage); err != nil {
				return "", fmt.Errorf("failed to add message to context: %w", err)
			}
			// Refresh context after adding message
			var err error
			conversationContext, err = p.contextManager.GetContext(ctx, contextID)
			if err != nil {
				return "", fmt.Errorf("failed to refresh context: %w", err)
			}
		} else {
			conversationContext.AddMessage(userMessage)
		}
	}

	// Call OpenAI API
	return p.callOpenAIWithContext(ctx, contextID, conversationContext)
}

func (p *OpenAIProvider) SendMessageWithOptions(ctx context.Context, request *models.MessageRequest) (*models.MessageResponse, error) {
	startTime := time.Now()

	response, err := p.SendMessageWithContext(ctx, request.Message, request.ContextID)
	if err != nil {
		return nil, err
	}

	return &models.MessageResponse{
		Response:     response,
		ContextID:    request.ContextID,
		Duration:     float64(time.Since(startTime).Milliseconds()),
		ProviderInfo: p.getProviderInfo(),
		Metadata:     request.Metadata,
		Timestamp:    time.Now(),
	}, nil
}

// Advanced messaging (placeholders for interface compliance)
func (p *OpenAIProvider) GenerateCompletion(ctx context.Context, request *models.CompletionRequest) (*models.CompletionResponse, error) {
	return nil, fmt.Errorf("completion generation not implemented for OpenAI provider")
}

func (p *OpenAIProvider) StreamCompletion(ctx context.Context, request *models.CompletionRequest) (<-chan models.CompletionChunk, error) {
	return nil, fmt.Errorf("streaming completion not implemented for OpenAI provider")
}

// Context management
func (p *OpenAIProvider) CreateContext(ctx context.Context, contextID string) error {
	if p.contextManager != nil {
		return p.contextManager.CreateContext(ctx, contextID)
	}
	return nil // No-op if no context manager
}

func (p *OpenAIProvider) GetContext(ctx context.Context, contextID string) (*models.ConversationContext, error) {
	if p.contextManager != nil {
		return p.contextManager.GetContext(ctx, contextID)
	}
	return nil, fmt.Errorf("context manager not available")
}

func (p *OpenAIProvider) UpdateContext(ctx context.Context, contextID string, message models.Message) error {
	if p.contextManager != nil {
		return p.contextManager.AddMessage(ctx, contextID, message)
	}
	return nil // No-op if no context manager
}

func (p *OpenAIProvider) ClearContext(ctx context.Context, contextID string) error {
	if p.contextManager != nil {
		return p.contextManager.ClearContext(ctx, contextID)
	}
	return nil // No-op if no context manager
}

func (p *OpenAIProvider) GetContextStats(ctx context.Context, contextID string) (*models.ContextStats, error) {
	if p.contextManager != nil {
		return p.contextManager.GetContextStats(ctx, contextID)
	}
	return nil, fmt.Errorf("context manager not available")
}

// Tool integration
func (p *OpenAIProvider) SetTools(ctx context.Context, tools []models.Tool) error {
	p.tools = tools
	if p.logger != nil {
		p.logger.Info("Tools updated", "tool_count", len(tools))
	}
	return nil
}

func (p *OpenAIProvider) GetTools(ctx context.Context) ([]models.Tool, error) {
	return p.tools, nil
}

func (p *OpenAIProvider) ExecuteToolCall(ctx context.Context, toolCall models.ToolCall) (*models.ToolResult, error) {
	return nil, fmt.Errorf("direct tool execution not supported by OpenAI provider")
}

// Configuration and validation
func (p *OpenAIProvider) Configure(ctx context.Context, config map[string]interface{}) error {
	newConfig, err := parseOpenAIConfig(config)
	if err != nil {
		return err
	}
	p.config = newConfig
	return nil
}

func (p *OpenAIProvider) Validate(ctx context.Context) error {
	if p.config.APIKey == "" {
		return models.NewAIError(models.ErrorTypeAuth, "OpenAI API key is required", "openai", 400)
	}
	if p.config.BaseURL == "" {
		return models.NewAIError(models.ErrorTypeValidation, "OpenAI base URL is required", "openai", 400)
	}
	return nil
}

func (p *OpenAIProvider) GetConfiguration(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"api_key":         "***REDACTED***",
		"base_url":        p.config.BaseURL,
		"model":           p.config.Model,
		"max_tokens":      p.config.MaxTokens,
		"temperature":     p.config.Temperature,
		"timeout_seconds": p.config.TimeoutSeconds,
		"organization":    p.config.Organization,
		"project":         p.config.Project,
	}, nil
}

// Provider information and capabilities
func (p *OpenAIProvider) GetProviderInfo(ctx context.Context) (*models.ProviderInfo, error) {
	return p.getProviderInfo(), nil
}

func (p *OpenAIProvider) GetCapabilities(ctx context.Context) (*models.ProviderCapabilities, error) {
	return &models.ProviderCapabilities{
		ProviderName:            "openai",
		SupportsStreaming:       true,
		SupportsVision:          false,
		SupportsEmbeddings:      false,
		SupportsFunctionCalling: true,
		SupportsAsync:           false,
		SupportsBatch:           false,
		MaxTokens:               p.config.MaxTokens,
		MaxContextLength:        128000, // GPT-4 context length
		SupportedModalities:     []string{"text"},
		SupportedModels:         []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"},
		Features:                []string{"tools", "context", "streaming"},
	}, nil
}

func (p *OpenAIProvider) EstimateTokens(ctx context.Context, text string) (int, error) {
	// Rough estimation: ~4 characters per token for English text
	return len(text) / 4, nil
}

// Health and diagnostics
func (p *OpenAIProvider) HealthCheck(ctx context.Context) (*models.HealthStatus, error) {
	status := &models.HealthStatus{
		Status:    models.HealthStatusHealthy,
		Component: "openai_provider",
		Message:   "Provider is operational",
		Timestamp: time.Now(),
		Uptime:    time.Since(p.startTime).Seconds(),
	}

	// Validate configuration
	if err := p.Validate(ctx); err != nil {
		status.Status = models.HealthStatusUnhealthy
		status.Message = fmt.Sprintf("Configuration validation failed: %v", err)
	}

	return status, nil
}

func (p *OpenAIProvider) GetMetrics(ctx context.Context) (*models.ProviderMetrics, error) {
	return &models.ProviderMetrics{
		ProviderName: "openai",
		Status:       p.status,
		Uptime:       time.Since(p.startTime).Seconds(),
	}, nil
}

// Lifecycle management
func (p *OpenAIProvider) Start(ctx context.Context) error {
	p.status = "running"
	if p.logger != nil {
		p.logger.Info("OpenAI provider started")
	}
	return nil
}

func (p *OpenAIProvider) Stop(ctx context.Context) error {
	p.status = "stopped"
	if p.logger != nil {
		p.logger.Info("OpenAI provider stopped")
	}
	return nil
}

func (p *OpenAIProvider) Restart(ctx context.Context) error {
	if err := p.Stop(ctx); err != nil {
		return err
	}
	return p.Start(ctx)
}

// Set dependencies
func (p *OpenAIProvider) SetLogger(logger interfaces.Logger) {
	p.logger = logger
}

func (p *OpenAIProvider) SetMetrics(metrics interfaces.Metrics) {
	p.metrics = metrics
}

func (p *OpenAIProvider) SetContextManager(cm interfaces.ContextManager) {
	p.contextManager = cm
}

// Internal implementation methods

func (p *OpenAIProvider) callOpenAIWithContext(ctx context.Context, contextID string, conversationContext *models.ConversationContext) (string, error) {
	if p.logger != nil {
		p.logger.Debug("Calling OpenAI API", "context_id", contextID, "message_count", len(conversationContext.Messages))
	}

	// Convert messages to OpenAI format
	openaiMessages := p.convertToOpenAIMessages(conversationContext.Messages)

	// Add system message
	systemMessage := p.buildSystemMessage()
	if systemMessage != "" {
		systemMsg := OpenAIMessage{
			Role:    "system",
			Content: systemMessage,
		}
		openaiMessages = append([]OpenAIMessage{systemMsg}, openaiMessages...)
	}

	// Convert tools to OpenAI format
	openaiTools := p.convertToOpenAITools(p.tools)

	// Build request
	request := OpenAIRequest{
		Model:     p.config.Model,
		Messages:  openaiMessages,
		Tools:     openaiTools,
		MaxTokens: p.config.MaxTokens,
		Stream:    false,
	}

	if len(openaiTools) > 0 {
		request.ToolChoice = "auto"
	}

	if p.config.Temperature != 0 {
		request.Temperature = p.config.Temperature
	}

	// Validate message sequence
	if err := p.validateMessageSequence(request.Messages); err != nil {
		return "", fmt.Errorf("invalid message sequence: %w", err)
	}

	// Make API call
	response, err := p.makeAPICall(ctx, request)
	if err != nil {
		return "", err
	}

	// Process response
	return p.processOpenAIResponse(ctx, response, contextID)
}

func (p *OpenAIProvider) makeAPICall(ctx context.Context, request OpenAIRequest) (*OpenAIResponse, error) {
	// Marshal request
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := p.config.BaseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	if p.config.Organization != "" {
		httpReq.Header.Set("OpenAI-Organization", p.config.Organization)
	}
	if p.config.Project != "" {
		httpReq.Header.Set("OpenAI-Project", p.config.Project)
	}

	// Make request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		return nil, p.handleErrorResponse(resp.StatusCode, responseBody)
	}

	// Parse response
	var openaiResp OpenAIResponse
	if err := json.Unmarshal(responseBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &openaiResp, nil
}

func (p *OpenAIProvider) processOpenAIResponse(ctx context.Context, response *OpenAIResponse, contextID string) (string, error) {
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in OpenAI response")
	}

	choice := response.Choices[0]

	// Create assistant message
	assistantMessage := models.Message{
		Role:      models.RoleAssistant,
		Content:   choice.Message.Content,
		Timestamp: time.Now(),
	}

	// Handle tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		// Convert tool calls to generic format
		var toolCalls []models.ToolCall
		for _, tc := range choice.Message.ToolCalls {
			var input any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				if p.logger != nil {
					p.logger.Warn("Failed to unmarshal tool arguments", "tool", tc.Function.Name, "error", err)
				}
			}

			toolCall := models.ToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			}
			toolCalls = append(toolCalls, toolCall)
		}
		assistantMessage.ToolCalls = toolCalls

		// Add assistant message to context
		if p.contextManager != nil {
			if err := p.contextManager.AddMessage(ctx, contextID, assistantMessage); err != nil {
				if p.logger != nil {
					p.logger.Error("Failed to add assistant message to context", "error", err)
				}
			}
		}

		// Note: Tool execution should be handled by the calling module, not the provider
		return choice.Message.Content, nil
	}

	// Add assistant message to context
	if p.contextManager != nil {
		if err := p.contextManager.AddMessage(ctx, contextID, assistantMessage); err != nil {
			if p.logger != nil {
				p.logger.Error("Failed to add assistant message to context", "error", err)
			}
		}
	}

	return choice.Message.Content, nil
}

func (p *OpenAIProvider) convertToOpenAIMessages(messages []models.Message) []OpenAIMessage {
	var openaiMessages []OpenAIMessage

	for _, msg := range messages {
		openaiMsg := OpenAIMessage{
			Role: string(msg.Role),
		}

		// Handle content
		if content, ok := msg.Content.(string); ok {
			openaiMsg.Content = content
		} else if msg.Content != nil {
			// Convert non-string content to JSON
			contentJSON, _ := json.Marshal(msg.Content)
			openaiMsg.Content = string(contentJSON)
		}

		// Handle tool calls for assistant messages
		if len(msg.ToolCalls) > 0 {
			openaiMsg.ToolCalls = p.convertToOpenAIToolCalls(msg.ToolCalls)
		}

		// Handle tool call ID for tool messages
		if msg.Role == models.RoleTool {
			openaiMsg.ToolCallID = msg.ToolCallID
			// Ensure tool messages have non-empty content
			if openaiMsg.Content == "" {
				openaiMsg.Content = "Empty tool response"
			}
		}

		openaiMessages = append(openaiMessages, openaiMsg)
	}

	return openaiMessages
}

func (p *OpenAIProvider) convertToOpenAITools(tools []models.Tool) []OpenAITool {
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

func (p *OpenAIProvider) convertToOpenAIToolCalls(toolCalls []models.ToolCall) []OpenAIToolCall {
	var openaiToolCalls []OpenAIToolCall

	for _, tc := range toolCalls {
		// Convert input to JSON string
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

func (p *OpenAIProvider) buildSystemMessage() string {
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
- Use schedule_app_run to schedule tasks

AVAILABLE CAPABILITIES:
- List registered applications and their tools (use list_apps)
- Execute tools from registered applications (use appId_toolName format)
- Search and retrieve document content (use search_documents)
- Schedule application executions (use schedule_app_run)

Your goal is to help users maximize the potential of the Arcadia platform by providing guidance, executing tools, and facilitating application development and interaction.

REMEMBER: When users ask questions that might be answered by existing documentation or code, ALWAYS use the search_documents tool first.`,
		now.Format("Monday, January 2, 2006 at 3:04 PM MST"),
		now.UTC().Format("2006-01-02 15:04:05 UTC"))
}

func (p *OpenAIProvider) validateMessageSequence(messages []OpenAIMessage) error {
	pendingToolCalls := make(map[string]bool)

	for i, msg := range messages {
		switch msg.Role {
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					if tc.ID == "" {
						return fmt.Errorf("message %d: assistant tool call missing ID", i)
					}
					pendingToolCalls[tc.ID] = true
				}
			}

		case "tool":
			if msg.ToolCallID == "" {
				return fmt.Errorf("message %d: tool message missing tool_call_id", i)
			}

			if !pendingToolCalls[msg.ToolCallID] {
				return fmt.Errorf("message %d: tool message with tool_call_id '%s' has no corresponding assistant tool call", i, msg.ToolCallID)
			}

			delete(pendingToolCalls, msg.ToolCallID)

			if msg.Content == "" {
				return fmt.Errorf("message %d: tool message has empty content", i)
			}
		}
	}

	if len(pendingToolCalls) > 0 {
		var unfulfilled []string
		for id := range pendingToolCalls {
			unfulfilled = append(unfulfilled, id)
		}
		return fmt.Errorf("unfulfilled tool calls: %v", unfulfilled)
	}

	return nil
}

func (p *OpenAIProvider) handleErrorResponse(statusCode int, body []byte) error {
	var errorResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	// Try to parse error response
	if err := json.Unmarshal(body, &errorResp); err != nil {
		return models.NewAIError(models.ErrorTypeProvider, fmt.Sprintf("HTTP %d: %s", statusCode, string(body)), "openai", statusCode)
	}

	// Map OpenAI error types to our error types
	var errorType string
	switch errorResp.Error.Type {
	case "authentication_error":
		errorType = models.ErrorTypeAuth
	case "rate_limit_error":
		errorType = models.ErrorTypeRateLimit
	case "quota_exceeded":
		errorType = models.ErrorTypeQuota
	case "timeout":
		errorType = models.ErrorTypeTimeout
	default:
		errorType = models.ErrorTypeProvider
	}

	return models.NewAIError(errorType, errorResp.Error.Message, "openai", statusCode)
}

func (p *OpenAIProvider) getProviderInfo() *models.ProviderInfo {
	return &models.ProviderInfo{
		Name:     "openai",
		Type:     "openai",
		Version:  "1.0.0",
		Model:    p.config.Model,
		Features: []string{"tools", "context", "streaming"},
		Status:   p.status,
	}
}

// Helper function to parse OpenAI configuration
func parseOpenAIConfig(config map[string]interface{}) (*OpenAIConfig, error) {
	openaiConfig := &OpenAIConfig{
		BaseURL:        "https://api.openai.com",
		Model:          "gpt-4",
		MaxTokens:      4096,
		Temperature:    1.0,
		TimeoutSeconds: 120,
	}

	// Extract configuration values
	if val, ok := config["api_key"].(string); ok {
		openaiConfig.APIKey = val
	}

	if val, ok := config["base_url"].(string); ok {
		openaiConfig.BaseURL = val
	}

	if val, ok := config["model"].(string); ok {
		openaiConfig.Model = val
	}

	if val, ok := config["max_tokens"]; ok {
		switch v := val.(type) {
		case int:
			openaiConfig.MaxTokens = v
		case float64:
			openaiConfig.MaxTokens = int(v)
		}
	}

	if val, ok := config["temperature"]; ok {
		switch v := val.(type) {
		case float64:
			openaiConfig.Temperature = v
		case int:
			openaiConfig.Temperature = float64(v)
		}
	}

	if val, ok := config["timeout_seconds"]; ok {
		switch v := val.(type) {
		case int:
			openaiConfig.TimeoutSeconds = v
		case float64:
			openaiConfig.TimeoutSeconds = int(v)
		}
	}

	if val, ok := config["organization"].(string); ok {
		openaiConfig.Organization = val
	}

	if val, ok := config["project"].(string); ok {
		openaiConfig.Project = val
	}

	return openaiConfig, nil
}
