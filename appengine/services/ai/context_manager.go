package ai

import (
	"log"
	"sync"
	"time"
)

// AIConversationContext represents a conversation with message history and metadata for AI services
type AIConversationContext struct {
	Messages     []Message `json:"messages"`
	LastAccessed time.Time `json:"last_accessed"`
	TotalTokens  int       `json:"total_tokens"`
}

// ContextManager manages conversation contexts for AI services
type ContextManager struct {
	contexts map[string]*AIConversationContext // Key is contextID (e.g., "wasm:<appID>", "web:<sessionID>")
	mutex    sync.RWMutex
	config   *AIConfig
}

// NewContextManager creates a new context manager with the given configuration
func NewContextManager(config *AIConfig) *ContextManager {
	return &ContextManager{
		contexts: make(map[string]*AIConversationContext),
		config:   config,
	}
}

// GetOrCreateContext retrieves an existing context or creates a new one
func (cm *ContextManager) GetOrCreateContext(contextID string) *AIConversationContext {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	context, exists := cm.contexts[contextID]
	if !exists {
		context = &AIConversationContext{
			Messages:     []Message{},
			LastAccessed: time.Now(),
			TotalTokens:  0,
		}
		cm.contexts[contextID] = context
	}

	context.LastAccessed = time.Now()
	return context
}

// AddMessage adds a message to the specified context and manages context size
func (cm *ContextManager) AddMessage(contextID string, message Message) *AIConversationContext {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	context, exists := cm.contexts[contextID]
	if !exists {
		context = &AIConversationContext{
			Messages:     []Message{},
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
			// Simple trimming: keep only the most recent messages, but ensure tool sequences aren't broken
			log.Printf("[AI Context] Simple trimming context from %d to %d messages", len(context.Messages), cm.config.MaxContextMessages)
			safeMessages := cm.getTailWithCompleteToolSequences(context.Messages, cm.config.MaxContextMessages)
			log.Printf("[AI Context] Simple trimming result: kept %d messages", len(safeMessages))
			context.Messages = safeMessages
		}
	}

	return context
}

// compactContext implements smart context compaction strategy that preserves tool call sequences
func (cm *ContextManager) compactContext(context *AIConversationContext) {
	// Smart compaction strategy:
	// 1. Keep the first message (initial context)
	// 2. Keep the last N-1 messages in full
	// 3. Ensure tool call sequences are not broken (assistant + tool responses)

	maxMessages := cm.config.MaxContextMessages
	if len(context.Messages) <= maxMessages {
		return
	}

	log.Printf("[AI Context] Compacting context from %d to %d messages", len(context.Messages), maxMessages)

	// Find a safe cut-off point that doesn't break tool call sequences
	keepRecent := max(maxMessages-1, 1)
	startIndex := len(context.Messages) - keepRecent

	// Look backwards from the desired start to find a safe cut point
	// We need to ensure we don't cut between assistant tool calls and their responses
	safeStartIndex := cm.findSafeCutPoint(context.Messages, startIndex)

	firstMsg := context.Messages[0]
	recentMsgs := context.Messages[safeStartIndex:]

	// If including the safe recent messages would still exceed limit,
	// we need to be more aggressive but still preserve tool sequences
	if len(recentMsgs)+1 > maxMessages {
		// Keep only the most recent complete tool sequences that fit
		recentMsgs = cm.getTailWithCompleteToolSequences(context.Messages, maxMessages-1)
	}

	log.Printf("[AI Context] Compaction result: keeping first message + %d recent messages", len(recentMsgs))
	context.Messages = append([]Message{firstMsg}, recentMsgs...)
}

// findSafeCutPoint finds a safe point to cut the message history without breaking tool call sequences
func (cm *ContextManager) findSafeCutPoint(messages []Message, desiredStart int) int {
	// If desired start is already at the beginning, use it
	if desiredStart <= 0 {
		return 0
	}

	// Look backwards from desired start to find a safe cut point
	for i := desiredStart; i >= 0; i-- {
		// A safe cut point is right after a complete conversation turn
		// i.e., not in the middle of assistant tool calls + tool responses
		if cm.isSafeCutPoint(messages, i) {
			return i
		}
	}

	// If no safe point found, cut at the beginning
	return 0
}

// isSafeCutPoint checks if cutting at the given index would break tool call sequences
func (cm *ContextManager) isSafeCutPoint(messages []Message, index int) bool {
	if index >= len(messages) {
		return true
	}

	// Look backwards from this index to see if we're in the middle of a tool sequence
	pendingToolCalls := make(map[string]bool)

	// Scan backwards to track any incomplete tool sequences
	for i := index - 1; i >= 0; i-- {
		msg := messages[i]

		if msg.Role == RoleAssistant && len(msg.ToolCalls) > 0 {
			// Found assistant with tool calls - track them
			for _, tc := range msg.ToolCalls {
				pendingToolCalls[tc.ID] = true
			}
		} else if msg.Role == RoleTool && msg.ToolCallID != "" {
			// Found tool response - remove from pending
			delete(pendingToolCalls, msg.ToolCallID)
		}
	}

	// If there are pending tool calls at this cut point, it's not safe
	return len(pendingToolCalls) == 0
}

// getTailWithCompleteToolSequences gets the tail of messages ensuring complete tool sequences
func (cm *ContextManager) getTailWithCompleteToolSequences(messages []Message, maxCount int) []Message {
	if len(messages) <= maxCount {
		return messages
	}

	// Start from the end and work backwards, keeping complete sequences
	kept := []Message{}
	pendingToolCalls := make(map[string]bool)

	// Process messages in reverse order
	// Note: We allow exceeding maxCount temporarily to ensure tool call sequences are complete
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]

		if msg.Role == RoleTool && msg.ToolCallID != "" {
			// Tool response - we MUST keep this and find its assistant call
			kept = append([]Message{msg}, kept...)
			pendingToolCalls[msg.ToolCallID] = true
		} else if msg.Role == RoleAssistant && len(msg.ToolCalls) > 0 {
			// Assistant with tool calls - check if any are pending
			hasNeededCalls := false
			for _, tc := range msg.ToolCalls {
				if pendingToolCalls[tc.ID] {
					hasNeededCalls = true
					delete(pendingToolCalls, tc.ID)
				}
			}

			if hasNeededCalls {
				// MUST keep this assistant message to complete tool sequence
				kept = append([]Message{msg}, kept...)
			} else if len(pendingToolCalls) == 0 && len(kept) < maxCount {
				// Regular assistant with tool calls, keep if we have space
				kept = append([]Message{msg}, kept...)
			}
		} else {
			// Regular message - keep if no pending tool calls and we have space
			if len(pendingToolCalls) == 0 && len(kept) < maxCount {
				kept = append([]Message{msg}, kept...)
			}
		}

		// If we have no pending tool calls and we've reached our target, we can stop
		// (unless we have more tool messages that need their assistant calls)
		if len(pendingToolCalls) == 0 && len(kept) >= maxCount {
			// Check if any remaining messages are tool messages
			hasMoreToolMessages := false
			for j := i - 1; j >= 0; j-- {
				if messages[j].Role == RoleTool && messages[j].ToolCallID != "" {
					hasMoreToolMessages = true
					break
				}
			}
			if !hasMoreToolMessages {
				break
			}
		}
	}

	// If we have any pending tool calls, we have an incomplete sequence
	// This shouldn't happen with proper tool call management, but log it for debugging
	if len(pendingToolCalls) > 0 {
		log.Printf("[AI Context] WARNING: Incomplete tool sequences remain: %v", pendingToolCalls)
	}

	return kept
}

// UpdateTokenCount updates the token count for a context
func (cm *ContextManager) UpdateTokenCount(contextID string, additionalTokens int) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if context, exists := cm.contexts[contextID]; exists {
		context.TotalTokens += additionalTokens
	}
}

// ClearContext removes a context completely
func (cm *ContextManager) ClearContext(contextID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	delete(cm.contexts, contextID)
}

// GetContext retrieves a context if it exists
func (cm *ContextManager) GetContext(contextID string) (*AIConversationContext, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	context, exists := cm.contexts[contextID]
	return context, exists
}

// CleanupExpiredContexts removes contexts that haven't been accessed within the TTL
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
			log.Printf("[AI Context] Expired context: %s", id)
		}
	}
}

// GetContextStats returns statistics about a context
func (cm *ContextManager) GetContextStats(contextID string) (messages int, tokens int, exists bool) {
	context, exists := cm.GetContext(contextID)
	if !exists {
		return 0, 0, false
	}
	return len(context.Messages), context.TotalTokens, true
}

// StartCleanupRoutine starts a background goroutine to clean up expired contexts
func (cm *ContextManager) StartCleanupRoutine(ttlMinutes int) {
	if ttlMinutes <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(5 * time.Minute) // Run every 5 minutes
		defer ticker.Stop()

		for range ticker.C {
			cm.CleanupExpiredContexts(ttlMinutes)
		}
	}()
}

// Helper function for Go versions that don't have max built-in
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
