# Arcadia AI Module

The AI module is a comprehensive artificial intelligence service for the Arcadia application platform. It provides a modular, provider-agnostic interface for AI operations, context management, tool execution, and conversation handling.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Context Management](#context-management)
- [Tool System](#tool-system)
- [API Reference](#api-reference)
- [Provider Support](#provider-support)
- [Development](#development)
- [Troubleshooting](#troubleshooting)

## Overview

The AI module follows Arcadia's modular monolith pattern, providing a clean facade interface while maintaining internal modularity. It supports multiple AI providers, conversation context management, dynamic tool loading, and comprehensive monitoring.

### Key Features

- **Multiple AI Providers**: OpenAI, Claude (Anthropic), and extensible provider system
- **Context Management**: Persistent conversation contexts with TTL and compaction
- **Tool Integration**: Dynamic tool loading from MCP servers and Arcadia apps
- **Provider Switching**: Runtime provider switching with conversation preservation
- **Metrics & Monitoring**: Comprehensive metrics collection and health checks
- **Configuration**: Flexible configuration via environment variables and config files

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        AI Module Facade                         │
├─────────────────────────────────────────────────────────────────┤
│  Core Components                                                │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌──────────┐  │
│  │   Context   │ │    Tool     │ │  Provider   │ │ Metrics  │  │
│  │  Manager    │ │  Manager    │ │   Manager   │ │ Manager  │  │
│  └─────────────┘ └─────────────┘ └─────────────┘ └──────────┘  │
├─────────────────────────────────────────────────────────────────┤
│  Providers                                                      │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐              │
│  │   OpenAI    │ │   Claude    │ │   Custom    │              │
│  │  Provider   │ │  Provider   │ │  Providers  │              │
│  └─────────────┘ └─────────────┘ └─────────────┘              │
├─────────────────────────────────────────────────────────────────┤
│  External Dependencies                                          │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐              │
│  │  App        │ │   MCP       │ │ Embedding   │              │
│  │ Registry    │ │  Servers    │ │   Search    │              │
│  └─────────────┘ └─────────────┘ └─────────────┘              │
└─────────────────────────────────────────────────────────────────┘
```

### File Structure

```
modules/ai/
├── README.md              # This documentation
├── facade.go              # Public API interface
├── module.go              # Main module implementation
├── global.go              # Global service management
├── config.go              # Configuration management
├── version.go             # Version information
├── core/
│   └── context.go         # Context management implementation
├── providers/
│   ├── openai.go          # OpenAI provider implementation
│   └── claude.go          # Claude provider implementation
├── tools/
│   └── manager.go         # Tool management system
├── models/
│   ├── types.go           # Core data types
│   ├── advanced.go        # Advanced model types
│   └── apps.go            # App-related models
└── interfaces/
    └── interfaces.go      # Interface definitions
```

## Getting Started

### Basic Usage

```go
import "arcadia/modules/ai"

// Initialize the global AI service
err := ai.InitializeGlobalServiceFromConfig()
if err != nil {
    log.Fatal("Failed to initialize AI service:", err)
}

// Get the AI service
aiService := ai.GetGlobalService()

// Send a simple message
response, err := aiService.SendMessage(ctx, "Hello, AI!")
if err != nil {
    log.Error("AI request failed:", err)
    return
}
fmt.Println("AI Response:", response)
```

### With Context Management

```go
// Create a conversation context
contextID := "user-session-123"
err := aiService.CreateConversation(ctx, contextID)
if err != nil {
    log.Error("Failed to create conversation:", err)
    return
}

// Send messages with context
response, err := aiService.SendMessageWithContext(ctx, "What is the weather?", contextID)
// Follow-up message will remember the conversation
response2, err := aiService.SendMessageWithContext(ctx, "What about tomorrow?", contextID)
```

## Configuration

The AI module supports configuration through environment variables and configuration files.

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `OPENAI_API_KEY` | OpenAI API key | - | Yes (for OpenAI) |
| `CLAUDE_API_KEY` | Claude API key | - | Yes (for Claude) |
| `AI_PROVIDER` | Default AI provider | `openai` | No |
| `AI_MODEL` | Default model | `gpt-4` | No |
| `AI_BASE_URL` | Custom API base URL | - | No |

### Configuration Example

```go
config := ai.DefaultConfig()
config.Provider = "openai"
config.DefaultMaxTokens = 4096
config.EnableMCP = true
config.EnableMetrics = true
config.ContextTTL = 60 * time.Minute

// Provider-specific configuration
config.Providers["openai"] = map[string]interface{}{
    "api_key": os.Getenv("OPENAI_API_KEY"),
    "model":   "gpt-4",
    "temperature": 0.7,
}
```

### Configuration Options

- **Provider Settings**: API keys, models, temperature, max tokens
- **Context Management**: TTL, max messages, compaction settings
- **Tool Configuration**: MCP servers, allowed domains, disabled tools
- **Performance**: Concurrent requests, rate limiting, timeout settings
- **Features**: Enable/disable MCP, persistence, metrics, caching

## Context Management

The AI module provides sophisticated conversation context management to maintain coherent, multi-turn conversations.

### How Context Works

1. **Context Creation**: Each conversation gets a unique context ID
2. **Message Storage**: All messages are stored in the context with metadata
3. **Context Retrieval**: Previous messages are included in new requests
4. **TTL Management**: Contexts automatically expire after the configured TTL
5. **Compaction**: Long conversations are automatically summarized to stay within token limits

### Context Lifecycle

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Create        │───▶│   Active        │───▶│   Expired       │
│   Context       │    │   Context       │    │   Context       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │   Compacted     │
                       │   Context       │
                       └─────────────────┘
```

### Context API

```go
// Create a new conversation context
err := aiService.CreateConversation(ctx, "user-123")

// Send message with context
response, err := aiService.SendMessageWithContext(ctx, "Hello", "user-123")

// Get context statistics
stats, err := aiService.GetContextStats(ctx, "user-123")
fmt.Printf("Messages: %d, Tokens: %d\n", stats.Messages, stats.Tokens)

// Clear context when done
err = aiService.ClearContext(ctx, "user-123")
```

### Context Configuration

```go
config.MaxContextMessages = 20          // Max messages before compaction
config.ContextCompaction = true         // Enable automatic compaction
config.ContextTTL = 60 * time.Minute   // Context expires after 1 hour
```

### Context Features

- **Automatic Cleanup**: Expired contexts are automatically removed
- **Memory Management**: Context compaction prevents token limit issues
- **Persistence**: Contexts can be persisted to database (when enabled)
- **Statistics**: Track message count, token usage, and last activity
- **Thread Safety**: Safe for concurrent access across multiple requests

## Tool System

The AI module includes a comprehensive tool system that allows AI models to execute functions and interact with external systems.

### Tool Types

1. **MCP Tools**: Tools from Model Context Protocol servers
2. **App Tools**: Tools from registered Arcadia applications
3. **System Tools**: Built-in tools for document search, scheduling, etc.
4. **Custom Tools**: User-defined tools with custom implementations

### How Tools Work

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  AI Request     │───▶│  Tool Manager   │───▶│  Tool           │
│  with Tools     │    │  Routes Call    │    │  Execution      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │                        │
                              ▼                        ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │  Tool Registry  │    │  Response       │
                       │  Discovery      │    │  to AI          │
                       └─────────────────┘    └─────────────────┘
```

### Tool Configuration

```go
config.EnableMCP = true
config.MCPServerCmd = "node mcp-server.js"
config.AllowedToolDomains = []string{"arcadia.ai", "tools.local"}
config.DisabledTools = []string{"dangerous-tool"}
```

### Available Tools

#### Document Search Tools
- `search_documents`: Semantic search across document collection
- `get_document`: Retrieve specific document by ID
- `list_documents`: List available documents

#### App Management Tools
- `list_apps`: List registered Arcadia applications
- `run_app_tool`: Execute tools from Arcadia apps
- `get_app_info`: Get information about specific apps

#### Scheduling Tools
- `schedule_task`: Schedule one-time or recurring tasks
- `list_schedules`: View scheduled tasks
- `cancel_schedule`: Cancel scheduled tasks

### Tool API

```go
// Get available tools
tools, err := aiService.GetTools(ctx)
for _, tool := range tools {
    fmt.Printf("Tool: %s - %s\n", tool.Name, tool.Description)
}

// Execute a tool directly
result, err := aiService.ExecuteTool(ctx, "search_documents", map[string]any{
    "query": "artificial intelligence",
    "limit": 5,
})

// Refresh tools (loads new MCP servers and apps)
err = aiService.RefreshTools(ctx)
```

### MCP Integration

The module supports Model Context Protocol (MCP) for dynamic tool loading:

```bash
# Environment variable for MCP server
export MCP_SERVER_CMD="node /path/to/mcp-server.js"

# Or in configuration
config.MCPServerCmd = "python mcp_server.py"
```

## API Reference

### Core Interface

```go
type AIModule interface {
    // Core AI operations
    SendMessage(ctx context.Context, message string) (string, error)
    SendMessageWithContext(ctx context.Context, message string, contextID string) (string, error)

    // Context management
    CreateConversation(ctx context.Context, contextID string) error
    ClearContext(ctx context.Context, contextID string) error
    GetContextStats(ctx context.Context, contextID string) (*ContextStats, error)

    // Tool management
    GetTools(ctx context.Context) ([]Tool, error)
    ExecuteTool(ctx context.Context, toolName string, input any) (string, error)
    RefreshTools(ctx context.Context) error

    // Provider management
    GetProviderInfo(ctx context.Context) (*ProviderInfo, error)
    SwitchProvider(ctx context.Context, providerName string) error

    // Health and monitoring
    HealthCheck(ctx context.Context) (*HealthStatus, error)
    GetMetrics(ctx context.Context) (*ModuleMetrics, error)
}
```

### HTTP Endpoints

The module exposes REST API endpoints:

- `POST /api/ai/v2/chat` - Send AI chat messages
- `GET /api/ai/provider/status` - Get current provider status
- `POST /api/ai/provider/switch` - Switch AI providers

### Request/Response Format

#### Chat Request
```json
{
    "message": "What is the weather like?",
    "context_id": "user-session-123",
    "provider": "openai"
}
```

#### Chat Response
```json
{
    "response": "I need more information about your location to check the weather.",
    "context_stats": {
        "message_count": 3,
        "total_tokens": 150
    },
    "provider_info": {
        "name": "openai",
        "model": "gpt-4",
        "features": ["tools", "context", "streaming"]
    }
}
```

## Provider Support

### OpenAI Provider

```go
config.Providers["openai"] = map[string]interface{}{
    "api_key":     os.Getenv("OPENAI_API_KEY"),
    "model":       "gpt-4",
    "temperature": 0.7,
    "max_tokens":  4096,
    "base_url":    "https://api.openai.com/v1", // Optional custom URL
}
```

**Supported Models**: GPT-4, GPT-4 Turbo, GPT-3.5 Turbo

### Claude Provider

```go
config.Providers["claude"] = map[string]interface{}{
    "api_key":     os.Getenv("CLAUDE_API_KEY"),
    "model":       "claude-3-sonnet-20240229",
    "temperature": 0.7,
    "max_tokens":  4096,
}
```

**Supported Models**: Claude 3 Opus, Claude 3 Sonnet, Claude 3 Haiku

### Provider Features

| Feature | OpenAI | Claude | Notes |
|---------|--------|--------|-------|
| Chat Completion | ✅ | ✅ | Basic chat functionality |
| Function Calling | ✅ | ✅ | Tool execution support |
| Streaming | ✅ | ✅ | Real-time responses |
| Context Windows | ✅ | ✅ | Large context support |
| Vision | ✅ | ✅ | Image understanding |

## Development

### Adding a New Provider

1. **Create Provider Implementation**:
```go
// providers/custom.go
type CustomProvider struct {
    config *CustomConfig
    client *CustomClient
}

func (p *CustomProvider) SendMessage(ctx context.Context, messages []Message) (*Response, error) {
    // Implementation
}
```

2. **Register Provider**:
```go
// In module initialization
providerManager.RegisterProvider("custom", NewCustomProvider(config))
```

3. **Add Configuration**:
```go
type CustomConfig struct {
    APIKey    string `json:"api_key"`
    BaseURL   string `json:"base_url"`
    Model     string `json:"model"`
}
```

### Adding Custom Tools

```go
// Define tool
tool := &models.Tool{
    Name:        "custom_tool",
    Description: "Performs custom operation",
    Parameters: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "input": {"type": "string", "description": "Input parameter"},
        },
    },
}

// Register tool
toolManager.RegisterTool(tool, func(ctx context.Context, input any) (string, error) {
    // Tool implementation
    return "result", nil
})
```

### Testing

```bash
# Run tests
go test ./modules/ai/...

# Run with coverage
go test -cover ./modules/ai/...

# Integration tests
go test -tags=integration ./modules/ai/...
```

## Troubleshooting

### Common Issues

#### API Key Not Found
```
[WARN] OPENAI_API_KEY environment variable is not set
```
**Solution**: Set the appropriate environment variable:
```bash
export OPENAI_API_KEY="your-api-key"
```

#### Context Not Found
```
Error: context not found: user-123
```
**Solution**: Create the context before using it:
```go
err := aiService.CreateConversation(ctx, "user-123")
```

#### Tool Execution Failed
```
Error: tool not found: custom_tool
```
**Solution**: Refresh tools or check tool registration:
```go
err := aiService.RefreshTools(ctx)
```

#### Provider Switch Failed
```
Error: provider not available: claude
```
**Solution**: Ensure provider is configured and API key is set.

### Debug Mode

Enable debug logging:
```go
config.LogLevel = "debug"
```

### Health Checks

```go
health, err := aiService.HealthCheck(ctx)
if err != nil {
    log.Error("Health check failed:", err)
}
fmt.Printf("Status: %s, Uptime: %s\n", health.Status, health.Uptime)
```

### Metrics

```go
metrics, err := aiService.GetMetrics(ctx)
if err != nil {
    log.Error("Failed to get metrics:", err)
}
fmt.Printf("Requests: %d, Errors: %d\n", metrics.TotalRequests, metrics.TotalErrors)
```

---

## Support

For issues and questions:
- Check the [troubleshooting section](#troubleshooting)
- Review logs for error details
- Ensure all environment variables are properly set
- Verify provider API keys are valid and have sufficient credits

## License

Part of the Arcadia application platform.