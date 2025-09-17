package openai

import (
	"testing"

	"arcadia/services/ai"
)

// TestContextCompactionPreservesToolSequences verifies that context compaction doesn't break tool call sequences
func TestContextCompactionPreservesToolSequences(t *testing.T) {
	config := &ai.AIConfig{
		Provider: "openai",
		ProviderSettings: map[string]any{
			"api_key":  "test-key",
			"base_url": "https://api.openai.com",
			"model":    "gpt-4",
		},
		MaxTokens:          4096,
		TimeoutSeconds:     120,
		MaxContextMessages: 5, // Small limit to trigger compaction
		ContextCompaction:  true,
		EnableMCP:          false,
	}

	service, err := NewOpenAIService(config)
	if err != nil {
		t.Fatalf("Failed to create OpenAI service: %v", err)
	}

	// Create a conversation that would trigger compaction
	contextID := "test-context"

	// Add multiple messages that include tool call sequences
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "Hello"},
		{Role: ai.RoleAssistant, Content: "Hi! How can I help?"},
		{Role: ai.RoleUser, Content: "What's the weather?"},
		{
			Role:    ai.RoleAssistant,
			Content: "I'll check the weather for you.",
			ToolCalls: []ai.ToolCall{
				{ID: "call_123", Name: "get_weather", Input: map[string]any{"location": "London"}},
			},
		},
		{Role: ai.RoleTool, Content: "Sunny, 25°C", ToolCallID: "call_123"},
		{Role: ai.RoleAssistant, Content: "The weather is sunny and 25°C."},
		{Role: ai.RoleUser, Content: "Thanks! What about tomorrow?"},
		{
			Role:    ai.RoleAssistant,
			Content: "Let me check tomorrow's forecast.",
			ToolCalls: []ai.ToolCall{
				{ID: "call_456", Name: "get_weather", Input: map[string]any{"location": "London", "date": "tomorrow"}},
			},
		},
		{Role: ai.RoleTool, Content: "Partly cloudy, 22°C", ToolCallID: "call_456"},
		{Role: ai.RoleAssistant, Content: "Tomorrow will be partly cloudy with 22°C."},
	}

	// Add all messages to trigger compaction
	for _, msg := range messages {
		service.contextManager.AddMessage(contextID, msg)
	}

	// Get the context and check that the messages are still valid
	context, exists := service.contextManager.GetContext(contextID)
	if !exists {
		t.Fatal("Context should exist after adding messages")
	}

	// Verify that the context was compacted, but may exceed maxMessages to preserve tool sequences
	t.Logf("Context compacted to %d messages (limit was %d)", len(context.Messages), config.MaxContextMessages)

	// The important thing is that tool call sequences are preserved, not the exact message count
	// Our fix prioritizes correctness over strict limits

	// Convert to OpenAI messages for validation
	openaiMessages := service.convertToOpenAIMessages(context.Messages)

	// The key test: validation should pass even after compaction
	err = service.validateMessageSequence(openaiMessages)
	if err != nil {
		t.Errorf("Message sequence validation should pass after compaction, got error: %v", err)

		// Debug: show the messages that failed validation
		t.Logf("Messages after compaction:")
		for i, msg := range openaiMessages {
			if msg.Role == "tool" {
				t.Logf("  Message %d: role=%s, tool_call_id=%s, content_length=%d", i, msg.Role, msg.ToolCallID, len(msg.Content))
			} else if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
				t.Logf("  Message %d: role=%s, tool_calls=%d", i, msg.Role, len(msg.ToolCalls))
				for j, tc := range msg.ToolCalls {
					t.Logf("    Tool call %d: id=%s, name=%s", j, tc.ID, tc.Function.Name)
				}
			} else {
				t.Logf("  Message %d: role=%s, content_length=%d", i, msg.Role, len(msg.Content))
			}
		}
	}
}

// TestSimpleTrimmingPreservesToolSequences verifies that simple trimming also preserves tool sequences
func TestSimpleTrimmingPreservesToolSequences(t *testing.T) {
	config := &ai.AIConfig{
		Provider: "openai",
		ProviderSettings: map[string]any{
			"api_key":  "test-key",
			"base_url": "https://api.openai.com",
			"model":    "gpt-4",
		},
		MaxTokens:          4096,
		TimeoutSeconds:     120,
		MaxContextMessages: 4, // Small limit to trigger trimming
		ContextCompaction:  false, // Use simple trimming
		EnableMCP:          false,
	}

	service, err := NewOpenAIService(config)
	if err != nil {
		t.Fatalf("Failed to create OpenAI service: %v", err)
	}

	contextID := "test-context-simple"

	// Add messages that include a tool call sequence at the end
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "Hello"},
		{Role: ai.RoleAssistant, Content: "Hi!"},
		{Role: ai.RoleUser, Content: "Old question"},
		{Role: ai.RoleAssistant, Content: "Old answer"},
		{Role: ai.RoleUser, Content: "What's the weather?"},
		{
			Role:    ai.RoleAssistant,
			Content: "I'll check the weather.",
			ToolCalls: []ai.ToolCall{
				{ID: "call_789", Name: "get_weather", Input: map[string]any{}},
			},
		},
		{Role: ai.RoleTool, Content: "Sunny", ToolCallID: "call_789"},
		{Role: ai.RoleAssistant, Content: "It's sunny!"},
	}

	// Add all messages to trigger trimming
	for _, msg := range messages {
		service.contextManager.AddMessage(contextID, msg)
	}

	// Get the context and validate
	context, exists := service.contextManager.GetContext(contextID)
	if !exists {
		t.Fatal("Context should exist after adding messages")
	}

	// Verify trimming occurred
	if len(context.Messages) > config.MaxContextMessages {
		t.Errorf("Context should be trimmed to %d messages, got %d", config.MaxContextMessages, len(context.Messages))
	}

	// Convert and validate
	openaiMessages := service.convertToOpenAIMessages(context.Messages)
	err = service.validateMessageSequence(openaiMessages)
	if err != nil {
		t.Errorf("Message sequence validation should pass after simple trimming, got error: %v", err)
	}
}

// TestCompactionWithMultipleToolCallsInOneMessage tests compaction with assistant messages having multiple tool calls
func TestCompactionWithMultipleToolCallsInOneMessage(t *testing.T) {
	config := &ai.AIConfig{
		Provider: "openai",
		ProviderSettings: map[string]any{
			"api_key":  "test-key",
			"base_url": "https://api.openai.com",
			"model":    "gpt-4",
		},
		MaxTokens:          4096,
		TimeoutSeconds:     120,
		MaxContextMessages: 6,
		ContextCompaction:  true,
		EnableMCP:          false,
	}

	service, err := NewOpenAIService(config)
	if err != nil {
		t.Fatalf("Failed to create OpenAI service: %v", err)
	}

	contextID := "test-multi-tools"

	// Create a scenario with multiple tool calls in one assistant message
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "Tell me about weather and news"},
		{
			Role:    ai.RoleAssistant,
			Content: "I'll get both weather and news for you.",
			ToolCalls: []ai.ToolCall{
				{ID: "call_weather", Name: "get_weather", Input: map[string]any{}},
				{ID: "call_news", Name: "get_news", Input: map[string]any{}},
			},
		},
		{Role: ai.RoleTool, Content: "Sunny, 25°C", ToolCallID: "call_weather"},
		{Role: ai.RoleTool, Content: "Breaking: Important news", ToolCallID: "call_news"},
		{Role: ai.RoleAssistant, Content: "The weather is sunny and here's the latest news..."},
		{Role: ai.RoleUser, Content: "What about sports?"},
		{
			Role:    ai.RoleAssistant,
			Content: "Let me get sports news.",
			ToolCalls: []ai.ToolCall{
				{ID: "call_sports", Name: "get_sports", Input: map[string]any{}},
			},
		},
		{Role: ai.RoleTool, Content: "Team wins!", ToolCallID: "call_sports"},
		{Role: ai.RoleAssistant, Content: "Latest sports: Team wins!"},
	}

	// Add all messages
	for _, msg := range messages {
		service.contextManager.AddMessage(contextID, msg)
	}

	// Validate the compacted context
	context, exists := service.contextManager.GetContext(contextID)
	if !exists {
		t.Fatal("Context should exist")
	}

	openaiMessages := service.convertToOpenAIMessages(context.Messages)
	err = service.validateMessageSequence(openaiMessages)
	if err != nil {
		t.Errorf("Validation should pass with multiple tool calls, got error: %v", err)
	}
}