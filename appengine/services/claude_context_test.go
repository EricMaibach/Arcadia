package services

import (
	"testing"
	"time"
)

func TestContextManager(t *testing.T) {
	config := ClaudeConfig{
		MaxContextMessages: 6,
		ContextCompaction:  false,
		ContextTTLMinutes:  60,
	}
	
	cm := &ContextManager{
		contexts: make(map[string]*ConversationContext),
		config:   &config,
	}
	
	// Test GetOrCreateContext
	ctx1 := cm.GetOrCreateContext("test1")
	if ctx1 == nil {
		t.Fatal("Expected context to be created")
	}
	if len(ctx1.Messages) != 0 {
		t.Fatal("Expected empty messages for new context")
	}
	
	// Test AddMessage
	msg1 := ClaudeMessage{Role: "user", Content: "Hello"}
	ctx1 = cm.AddMessage("test1", msg1)
	if len(ctx1.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(ctx1.Messages))
	}
	
	// Test message trimming
	for i := 0; i < 10; i++ {
		msg := ClaudeMessage{Role: "user", Content: "Message " + string(rune(i))}
		cm.AddMessage("test1", msg)
	}
	
	ctx1, _ = cm.GetContext("test1")
	if len(ctx1.Messages) != config.MaxContextMessages {
		t.Fatalf("Expected %d messages after trimming, got %d", 
			config.MaxContextMessages, len(ctx1.Messages))
	}
	
	// Test context isolation
	ctx2 := cm.GetOrCreateContext("test2")
	msg2 := ClaudeMessage{Role: "user", Content: "Different context"}
	cm.AddMessage("test2", msg2)
	
	ctx1, _ = cm.GetContext("test1")
	ctx2, _ = cm.GetContext("test2")
	
	if len(ctx1.Messages) == len(ctx2.Messages) {
		t.Fatal("Contexts should be isolated")
	}
	
	// Test ClearContext
	cm.ClearContext("test1")
	_, exists := cm.GetContext("test1")
	if exists {
		t.Fatal("Context should be cleared")
	}
	
	// Test UpdateTokenCount
	cm.UpdateTokenCount("test2", 100)
	ctx2, _ = cm.GetContext("test2")
	if ctx2.TotalTokens != 100 {
		t.Fatalf("Expected 100 tokens, got %d", ctx2.TotalTokens)
	}
}

func TestContextCompaction(t *testing.T) {
	config := ClaudeConfig{
		MaxContextMessages: 6,
		ContextCompaction:  true, // Enable compaction
		ContextTTLMinutes:  60,
	}
	
	cm := &ContextManager{
		contexts: make(map[string]*ConversationContext),
		config:   &config,
	}
	
	// Add more messages than max
	for i := 0; i < 10; i++ {
		msg := ClaudeMessage{
			Role:    "user",
			Content: "Message " + string(rune('A'+i)),
		}
		cm.AddMessage("compact_test", msg)
	}
	
	ctx, _ := cm.GetContext("compact_test")
	
	// Should have max messages
	if len(ctx.Messages) != config.MaxContextMessages {
		t.Fatalf("Expected %d messages after compaction, got %d",
			config.MaxContextMessages, len(ctx.Messages))
	}
	
	// First message should be preserved (Message A)
	if ctx.Messages[0].Content != "Message A" {
		t.Fatalf("Expected first message to be preserved, got %s", ctx.Messages[0].Content)
	}
	
	// Last messages should be the most recent ones
	lastMsg := ctx.Messages[len(ctx.Messages)-1]
	if lastMsg.Content != "Message J" {
		t.Fatalf("Expected last message to be 'Message J', got %s", lastMsg.Content)
	}
}

func TestContextExpiry(t *testing.T) {
	config := ClaudeConfig{
		MaxContextMessages: 10,
		ContextTTLMinutes:  1, // 1 minute TTL for testing
	}
	
	cm := &ContextManager{
		contexts: make(map[string]*ConversationContext),
		config:   &config,
	}
	
	// Create a context
	cm.GetOrCreateContext("expire_test")
	
	// Manually set LastAccessed to past
	cm.contexts["expire_test"].LastAccessed = time.Now().Add(-2 * time.Minute)
	
	// Run cleanup
	cm.CleanupExpiredContexts(1)
	
	// Context should be gone
	_, exists := cm.GetContext("expire_test")
	if exists {
		t.Fatal("Expired context should be removed")
	}
}