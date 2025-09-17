# AI Service Factory Usage

This document describes how to use the new AI service factory system.

## Basic Setup

1. First, register the providers (do this once during application initialization):

```go
import "arcadia/services/airegistry"

// Register all providers
airegistry.SetupProviders()
```

2. Create an AI service using the factory:

```go
import "arcadia/services/ai"

// Using the configuration builder
config := ai.NewConfigBuilder("claude").
    WithAPIKey("your-api-key").
    WithModel("claude-3-5-sonnet-20241022").
    WithMaxTokens(4096).
    Build()

// Create service through global factory
factory := ai.GetGlobalFactory()
service, err := factory.CreateService(config)
if err != nil {
    log.Fatal(err)
}

// Or initialize as global service
err = ai.InitializeGlobalService(config)
if err != nil {
    log.Fatal(err)
}

// Use global service
service = ai.GetGlobalService()
```

## Configuration Options

### Using Configuration Builder

```go
config := ai.NewConfigBuilder("openai").
    WithAPIKey("your-openai-key").
    WithModel("gpt-4").
    WithMaxTokens(2048).
    WithTimeout(60).
    WithContextSettings(30, true, 120).
    WithMCP(true, "mcp-server-command").
    WithProviderSetting("temperature", 0.7).
    Build()
```

### Loading from Environment

```go
// Set environment variables:
// AI_PROVIDER=claude
// AI_API_KEY=your-key
// AI_MODEL=claude-3-sonnet
// AI_BASE_URL=https://api.anthropic.com

config, err := ai.LoadConfigFromEnv()
if err != nil {
    log.Fatal(err)
}
```

### Loading from File

```go
config, err := ai.LoadConfigFromFile("ai-config.json")
if err != nil {
    log.Fatal(err)
}
```

## Provider-Specific Settings

### Claude Settings

```go
config := ai.NewConfigBuilder("claude").
    WithAPIKey("your-claude-key").
    WithProviderSetting("base_url", "https://api.anthropic.com").
    WithProviderSetting("model", "claude-3-5-sonnet-20241022").
    Build()
```

### OpenAI Settings

```go
config := ai.NewConfigBuilder("openai").
    WithAPIKey("your-openai-key").
    WithProviderSetting("base_url", "https://api.openai.com").
    WithProviderSetting("model", "gpt-4").
    WithProviderSetting("temperature", 0.7).
    WithProviderSetting("organization", "your-org-id"). // optional
    WithProviderSetting("project", "your-project-id").  // optional
    Build()
```

## Using Services

```go
// Send a simple message
response, err := service.SendMessage("Hello, how are you?")

// Send with context (maintains conversation history)
response, err := service.SendMessageWithContext("Follow up question", "conversation-1")

// Clear context
service.ClearContext("conversation-1")

// Get context statistics
messages, tokens, exists := service.GetContextStats("conversation-1")

// Refresh tools (for MCP-enabled services)
service.RefreshTools()

// Get provider information
info := service.GetProviderInfo()
fmt.Printf("Using %s with model %s\n", info.Name, info.Model)
```

## Factory Methods

```go
factory := ai.GetGlobalFactory()

// Check supported providers
providers := factory.GetSupportedProviders()

// Validate a provider
err := factory.ValidateProvider("claude")

// Create with defaults
service, err := factory.CreateServiceWithDefaults("claude", "your-api-key")
```

## Error Handling

The system uses structured errors:

```go
service, err := factory.CreateService(config)
if err != nil {
    if aiErr, ok := err.(*ai.AIError); ok {
        fmt.Printf("AI Error: Type=%s, Provider=%s, Message=%s\n",
            aiErr.Type, aiErr.Provider, aiErr.Message)
    }
}
```

## Migration from Legacy Claude Service

If you were using the old Claude-specific service:

```go
// Old way
claudeService := services.GetClaudeService()
response, err := claudeService.SendMessage("Hello")

// New way
ai.InitializeGlobalService(config) // Do this once
service := ai.GetGlobalService()
response, err := service.SendMessage("Hello")

// Or using convenience functions
response, err := ai.SendGlobalMessage("Hello")
response, err := ai.SendGlobalMessageWithContext("Hello", "context-id")
ai.TriggerGlobalToolRefresh()
```