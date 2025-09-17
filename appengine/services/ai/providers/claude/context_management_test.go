package claude

import (
	"testing"
	"time"

	"arcadia/services/ai"
)

// TestContextCreationAndManagement tests basic context operations
func TestContextCreationAndManagement(t *testing.T) {
	config := &ai.AIConfig{
		Provider:           "claude",
		MaxTokens:          4096,
		TimeoutSeconds:     120,
		MaxContextMessages: 10,
		ContextCompaction:  true,
		ContextTTLMinutes:  60,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	service, err := NewClaudeService(config)
	if err != nil {
		t.Fatalf("Failed to create Claude service: %v", err)
	}

	contextID := "test-context"

	// Initially, context should not exist
	messages, tokens, exists := service.GetContextStats(contextID)
	if exists {
		t.Error("Context should not exist initially")
	}
	if messages != 0 || tokens != 0 {
		t.Errorf("Expected 0 messages and tokens, got %d messages, %d tokens", messages, tokens)
	}

	// Add a message to create context (this will fail API call but create context)
	_, err = service.SendMessageWithContext("test message", contextID)
	// We expect this to fail since we're not making real API calls, but context should be created

	// Check that context now exists with the user message
	messages, tokens, exists = service.GetContextStats(contextID)
	if !exists {
		t.Error("Context should exist after sending message")
	}
	if messages != 1 {
		t.Errorf("Expected 1 message, got %d", messages)
	}

	// Clear the context
	service.ClearContext(contextID)

	// Context should no longer exist
	messages, tokens, exists = service.GetContextStats(contextID)
	if exists {
		t.Error("Context should not exist after clearing")
	}
	if messages != 0 || tokens != 0 {
		t.Errorf("Expected 0 messages and tokens after clearing, got %d messages, %d tokens", messages, tokens)
	}
}

// TestContextMessageLimit tests context message limitation and compaction
func TestContextMessageLimit(t *testing.T) {
	config := &ai.AIConfig{
		Provider:           "claude",
		MaxTokens:          4096,
		TimeoutSeconds:     120,
		MaxContextMessages: 3, // Small limit for testing
		ContextCompaction:  false, // Disable compaction for this test
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	contextManager := ai.NewContextManager(config)

	contextID := "test-context"

	// Add messages beyond the limit
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "Message 1"},
		{Role: ai.RoleAssistant, Content: "Response 1"},
		{Role: ai.RoleUser, Content: "Message 2"},
		{Role: ai.RoleAssistant, Content: "Response 2"},
		{Role: ai.RoleUser, Content: "Message 3"}, // This should trigger trimming
	}

	for _, msg := range messages {
		contextManager.AddMessage(contextID, msg)
	}

	// Check that only the most recent messages are kept
	context, exists := contextManager.GetContext(contextID)
	if !exists {
		t.Fatal("Context should exist")
	}

	if len(context.Messages) > 3 {
		t.Errorf("Expected max 3 messages, got %d", len(context.Messages))
	}

	// The last message should be "Message 3"
	lastMessage := context.Messages[len(context.Messages)-1]
	if lastMessage.Content != "Message 3" {
		t.Errorf("Expected last message to be 'Message 3', got '%v'", lastMessage.Content)
	}
}

// TestContextCompaction tests smart context compaction
func TestContextCompaction(t *testing.T) {
	config := &ai.AIConfig{
		Provider:           "claude",
		MaxTokens:          4096,
		TimeoutSeconds:     120,
		MaxContextMessages: 3, // Small limit for testing
		ContextCompaction:  true, // Enable compaction
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	contextManager := ai.NewContextManager(config)

	contextID := "test-context"

	// Add messages beyond the limit
	messages := []ai.Message{
		{Role: ai.RoleSystem, Content: "System prompt"}, // This should be kept
		{Role: ai.RoleUser, Content: "Message 1"},
		{Role: ai.RoleAssistant, Content: "Response 1"},
		{Role: ai.RoleUser, Content: "Message 2"},
		{Role: ai.RoleAssistant, Content: "Response 2"}, // This should be kept (recent)
		{Role: ai.RoleUser, Content: "Message 3"},       // This should be kept (most recent)
	}

	for _, msg := range messages {
		contextManager.AddMessage(contextID, msg)
	}

	// Check that compaction works
	context, exists := contextManager.GetContext(contextID)
	if !exists {
		t.Fatal("Context should exist")
	}

	if len(context.Messages) > 3 {
		t.Errorf("Expected max 3 messages after compaction, got %d", len(context.Messages))
	}

	// First message should still be the system prompt
	if len(context.Messages) > 0 && context.Messages[0].Content != "System prompt" {
		t.Errorf("Expected first message to be system prompt, got '%v'", context.Messages[0].Content)
	}

	// Last message should be the most recent
	if len(context.Messages) > 0 {
		lastMessage := context.Messages[len(context.Messages)-1]
		if lastMessage.Content != "Message 3" {
			t.Errorf("Expected last message to be 'Message 3', got '%v'", lastMessage.Content)
		}
	}
}

// TestContextTTLAndCleanup tests context expiration and cleanup
func TestContextTTLAndCleanup(t *testing.T) {
	config := &ai.AIConfig{
		Provider:          "claude",
		MaxTokens:         4096,
		TimeoutSeconds:    120,
		ContextTTLMinutes: 1, // 1 minute for testing
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	contextManager := ai.NewContextManager(config)

	contextID1 := "test-context-1"
	contextID2 := "test-context-2"

	// Add messages to both contexts
	contextManager.AddMessage(contextID1, ai.Message{Role: ai.RoleUser, Content: "Message 1"})
	contextManager.AddMessage(contextID2, ai.Message{Role: ai.RoleUser, Content: "Message 2"})

	// Both contexts should exist
	_, exists1 := contextManager.GetContext(contextID1)
	_, exists2 := contextManager.GetContext(contextID2)
	if !exists1 || !exists2 {
		t.Fatal("Both contexts should exist")
	}

	// Manually expire one context by modifying its last accessed time
	if context1, exists := contextManager.GetContext(contextID1); exists {
		context1.LastAccessed = time.Now().Add(-2 * time.Minute) // 2 minutes ago
	}

	// Run cleanup
	contextManager.CleanupExpiredContexts(1) // 1 minute TTL

	// Only context2 should exist now
	_, exists1 = contextManager.GetContext(contextID1)
	_, exists2 = contextManager.GetContext(contextID2)

	if exists1 {
		t.Error("Context 1 should have been cleaned up")
	}
	if !exists2 {
		t.Error("Context 2 should still exist")
	}
}

// TestTokenCountTracking tests token count updates
func TestTokenCountTracking(t *testing.T) {
	config := &ai.AIConfig{
		Provider:       "claude",
		MaxTokens:      4096,
		TimeoutSeconds: 120,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	contextManager := ai.NewContextManager(config)

	contextID := "test-context"

	// Add a message
	contextManager.AddMessage(contextID, ai.Message{Role: ai.RoleUser, Content: "Test message"})

	// Initial token count should be 0
	_, tokens, exists := contextManager.GetContextStats(contextID)
	if !exists {
		t.Fatal("Context should exist")
	}
	if tokens != 0 {
		t.Errorf("Expected 0 initial tokens, got %d", tokens)
	}

	// Update token count
	contextManager.UpdateTokenCount(contextID, 100)

	// Check updated token count
	_, tokens, exists = contextManager.GetContextStats(contextID)
	if !exists {
		t.Fatal("Context should still exist")
	}
	if tokens != 100 {
		t.Errorf("Expected 100 tokens, got %d", tokens)
	}

	// Add more tokens
	contextManager.UpdateTokenCount(contextID, 50)

	// Check cumulative token count
	_, tokens, exists = contextManager.GetContextStats(contextID)
	if !exists {
		t.Fatal("Context should still exist")
	}
	if tokens != 150 {
		t.Errorf("Expected 150 tokens, got %d", tokens)
	}
}

// TestConcurrentContextAccess tests thread safety of context operations
func TestConcurrentContextAccess(t *testing.T) {
	config := &ai.AIConfig{
		Provider:       "claude",
		MaxTokens:      4096,
		TimeoutSeconds: 120,
		ProviderSettings: map[string]any{
			"api_key": "test-key",
		},
	}

	contextManager := ai.NewContextManager(config)

	// Run concurrent operations
	done := make(chan bool, 10)

	// Start multiple goroutines doing context operations
	for i := 0; i < 10; i++ {
		go func(id int) {
			contextID := "test-context"

			// Add messages
			for j := 0; j < 5; j++ {
				msg := ai.Message{
					Role:    ai.RoleUser,
					Content: "Message from goroutine " + string(rune(id)) + " iteration " + string(rune(j)),
				}
				contextManager.AddMessage(contextID, msg)
			}

			// Read stats
			_, _, _ = contextManager.GetContextStats(contextID)

			// Update tokens
			contextManager.UpdateTokenCount(contextID, 10)

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify context still exists and has some messages
	messages, tokens, exists := contextManager.GetContextStats("test-context")
	if !exists {
		t.Error("Context should exist after concurrent operations")
	}
	if messages == 0 {
		t.Error("Context should have some messages after concurrent operations")
	}
	if tokens == 0 {
		t.Error("Context should have some tokens after concurrent operations")
	}

	t.Logf("Final context stats: %d messages, %d tokens", messages, tokens)
}