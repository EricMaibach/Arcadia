# Claude AI Service Implementation

This package provides a Claude AI service implementation that follows the common `AIService` interface defined in the parent `ai` package.

## Structure

- `claude_types.go` - Claude-specific types and data structures
- `claude_service.go` - Main Claude service implementation
- `claude_test.go` - Comprehensive test suite

## Key Features

### ✅ Complete AIService Interface Implementation
- `SendMessage(message string) (string, error)`
- `SendMessageWithContext(message string, contextID string) (string, error)`
- `ClearContext(contextID string)`
- `GetContextStats(contextID string) (messages int, tokens int, exists bool)`
- `RefreshTools()`
- `TriggerToolRefresh()`
- `GetProviderInfo() ai.ProviderInfo`
- `Validate() error`

### ✅ Claude-Specific Features Preserved
- **System Prompts**: Complete Arcadia system prompt with current date/time
- **Tool Support**: Full MCP tool integration including:
  - System tools (list_apps, schedule_app_run, list_schedules)
  - Document search tools (search_documents)
  - Dynamic app tools (loaded from registry)
- **Context Management**: Smart context compaction and TTL-based cleanup
- **Error Handling**: Structured AI errors with proper typing
- **Tool Execution Loop**: Recursive tool calling with depth limits
- **Token Tracking**: Input/output token counting and context stats

### ✅ Backward Compatibility
- `HandleClaudeAPI(w http.ResponseWriter, r *http.Request)` - HTTP handler for existing web API
- `SetDependencies()` - Dependency injection for registry, app runner, embedding search

### ✅ Configuration and Convenience
- `CreateClaudeConfig(apiKey, model string, maxTokens int)` - Easy configuration creation
- Automatic defaults for base URL, model, timeouts, context settings
- Provider validation and settings parsing

## Usage

### Basic Usage
```go
// Create configuration
config := claude.CreateClaudeConfig("your-api-key", "claude-3-5-sonnet-20241022", 4096)

// Create service
service, err := claude.NewClaudeService(config)
if err != nil {
    log.Fatal(err)
}

// Use as AIService interface
var aiService ai.AIService = service

// Send messages
response, err := aiService.SendMessage("Hello, Claude!")
```

### With Dependencies
```go
// Set up dependencies for tool execution
service.SetDependencies(registryAccess, appRunner, appCreator, embeddingSearch)

// Now tools will work properly
response, err := service.SendMessage("List all apps in the ecosystem")
```

### HTTP API (Backward Compatibility)
```go
// Use existing HTTP handler
http.HandleFunc("/api/claude", service.HandleClaudeAPI)
```

## Architecture

The Claude service uses the extracted common functionality:

- **ContextManager**: Manages conversation contexts with TTL and compaction
- **ToolManager**: Handles MCP tools and dynamic app tools
- **HTTPClient**: Provides HTTP utilities with proper error handling
- **Types**: Common AI service types and interfaces

All complex logic from the original `claude.go` has been preserved and adapted to work with the new architecture.

## Testing

Comprehensive test suite covering:
- Service creation and configuration
- Interface compliance
- Configuration parsing and defaults
- Validation and error handling
- Provider info and features

Run tests:
```bash
go test ./services/ai/providers/claude/ -v
```

## Integration

This implementation serves as the reference for other AI provider implementations and validates that the `AIService` interface is complete and practical for real-world usage.