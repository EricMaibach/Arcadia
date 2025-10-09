package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"arcadia/modules/ai/interfaces"
	"arcadia/modules/ai/models"
)

// ContextManager implements the context management functionality
type ContextManager struct {
	contexts      map[string]*models.ConversationContext
	mutex         sync.RWMutex
	config        *models.Config
	logger        interfaces.Logger
	database      interfaces.Database
	cache         interfaces.Cache
	metrics       interfaces.Metrics
	eventBus      interfaces.EventBus
	cleanupTicker *time.Ticker
	stopCleanup   chan bool
}

// NewContextManager creates a new context manager
func NewContextManager(config *models.Config, deps *interfaces.Dependencies) *ContextManager {
	cm := &ContextManager{
		contexts:    make(map[string]*models.ConversationContext),
		config:      config,
		logger:      deps.Logger,
		database:    deps.Database,
		cache:       deps.Cache,
		metrics:     deps.Metrics,
		eventBus:    deps.EventBus,
		stopCleanup: make(chan bool),
	}

	// Start cleanup routine if TTL is configured
	if config.ContextTTL > 0 {
		cm.startCleanupRoutine()
	}

	return cm
}

// CreateContext creates a new conversation context
func (cm *ContextManager) CreateContext(ctx context.Context, contextID string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Check if context already exists
	if _, exists := cm.contexts[contextID]; exists {
		return models.NewContextError(contextID, "create", "context already exists")
	}

	now := time.Now()
	conversationContext := &models.ConversationContext{
		ID:       contextID,
		Messages: []models.Message{},
		Stats: models.ContextStats{
			Messages:    0,
			Tokens:      0,
			LastUpdated: now,
			CreatedAt:   now,
			Duration:    0,
		},
		Metadata:     make(map[string]interface{}),
		CreatedAt:    now,
		UpdatedAt:    now,
		LastAccessed: now,
	}

	cm.contexts[contextID] = conversationContext

	// Record metrics
	if cm.metrics != nil {
		cm.metrics.IncrementCounter("contexts_created", map[string]string{
			"context_id": contextID,
		})
	}

	// Publish event
	if cm.eventBus != nil {
		event := models.NewContextEvent(contextID, models.EventTypeContextCreated)
		event.Success = true
		cm.publishEvent(ctx, models.NewEvent(models.EventTypeContextCreated, models.EventSourceContext).WithData(event))
	}

	if cm.logger != nil {
		cm.logger.Info("Created conversation context", "context_id", contextID)
	}

	return nil
}

// GetContext retrieves a conversation context
func (cm *ContextManager) GetContext(ctx context.Context, contextID string) (*models.ConversationContext, error) {
	cm.mutex.RLock()
	conversationContext, exists := cm.contexts[contextID]
	cm.mutex.RUnlock()

	if !exists {
		return nil, models.NewContextError(contextID, "get", "context not found")
	}

	// Update last accessed time
	cm.mutex.Lock()
	conversationContext.LastAccessed = time.Now()
	cm.mutex.Unlock()

	// Record metrics
	if cm.metrics != nil {
		cm.metrics.IncrementCounter("context_accessed", map[string]string{
			"context_id": contextID,
		})
	}

	return conversationContext, nil
}

// DeleteContext removes a conversation context
func (cm *ContextManager) DeleteContext(ctx context.Context, contextID string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	conversationContext, exists := cm.contexts[contextID]
	if !exists {
		return models.NewContextError(contextID, "delete", "context not found")
	}

	delete(cm.contexts, contextID)

	// Record metrics
	if cm.metrics != nil {
		cm.metrics.IncrementCounter("contexts_deleted", map[string]string{
			"context_id": contextID,
		})
		cm.metrics.RecordValue("context_lifetime", time.Since(conversationContext.CreatedAt).Seconds(), map[string]string{
			"context_id": contextID,
		})
	}

	// Publish event
	if cm.eventBus != nil {
		event := models.NewContextEvent(contextID, models.EventTypeContextDeleted)
		event.Success = true
		event.MessageCount = len(conversationContext.Messages)
		event.TokenCount = conversationContext.Stats.Tokens
		cm.publishEvent(ctx, models.NewEvent(models.EventTypeContextDeleted, models.EventSourceContext).WithData(event))
	}

	if cm.logger != nil {
		cm.logger.Info("Deleted conversation context", "context_id", contextID)
	}

	return nil
}

// ListContexts returns a list of all context IDs
func (cm *ContextManager) ListContexts(ctx context.Context) ([]string, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	contextIDs := make([]string, 0, len(cm.contexts))
	for contextID := range cm.contexts {
		contextIDs = append(contextIDs, contextID)
	}

	return contextIDs, nil
}

// AddMessage adds a message to a conversation context
func (cm *ContextManager) AddMessage(ctx context.Context, contextID string, message models.Message) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	conversationContext, exists := cm.contexts[contextID]
	if !exists {
		return models.NewContextError(contextID, "add_message", "context not found")
	}

	// Set timestamp if not provided
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	conversationContext.AddMessage(message)

	// Trim context if needed
	if cm.config.MaxContextMessages > 0 && len(conversationContext.Messages) > cm.config.MaxContextMessages {
		if cm.config.ContextCompaction {
			cm.compactContext(conversationContext)
		} else {
			cm.trimContext(conversationContext, cm.config.MaxContextMessages)
		}
	}

	// Record metrics
	if cm.metrics != nil {
		cm.metrics.IncrementCounter("messages_added", map[string]string{
			"context_id": contextID,
			"role":       string(message.Role),
		})
	}

	// Publish event
	if cm.eventBus != nil {
		event := models.NewContextEvent(contextID, models.EventTypeContextUpdated)
		event.Success = true
		event.MessageCount = len(conversationContext.Messages)
		event.TokenCount = conversationContext.Stats.Tokens
		event.Operation = "add_message"
		cm.publishEvent(ctx, models.NewEvent(models.EventTypeContextUpdated, models.EventSourceContext).WithData(event))
	}

	return nil
}

// GetMessages retrieves all messages from a conversation context
func (cm *ContextManager) GetMessages(ctx context.Context, contextID string) ([]models.Message, error) {
	conversationContext, err := cm.GetContext(ctx, contextID)
	if err != nil {
		return nil, err
	}

	return conversationContext.GetMessages(), nil
}

// GetLastMessages retrieves the last N messages from a conversation context
func (cm *ContextManager) GetLastMessages(ctx context.Context, contextID string, limit int) ([]models.Message, error) {
	conversationContext, err := cm.GetContext(ctx, contextID)
	if err != nil {
		return nil, err
	}

	messages := conversationContext.GetMessages()
	if limit <= 0 || limit >= len(messages) {
		return messages, nil
	}

	return messages[len(messages)-limit:], nil
}

// ClearContext removes all messages from a conversation context
func (cm *ContextManager) ClearContext(ctx context.Context, contextID string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	conversationContext, exists := cm.contexts[contextID]
	if !exists {
		return models.NewContextError(contextID, "clear", "context not found")
	}

	messageCount := len(conversationContext.Messages)
	tokenCount := conversationContext.Stats.Tokens

	conversationContext.Clear()

	// Record metrics
	if cm.metrics != nil {
		cm.metrics.IncrementCounter("contexts_cleared", map[string]string{
			"context_id": contextID,
		})
		cm.metrics.RecordValue("messages_cleared", float64(messageCount), map[string]string{
			"context_id": contextID,
		})
	}

	// Publish event
	if cm.eventBus != nil {
		event := models.NewContextEvent(contextID, models.EventTypeContextUpdated)
		event.Success = true
		event.MessageCount = 0
		event.TokenCount = 0
		event.Operation = "clear"
		event.AddMetadata("previous_message_count", messageCount)
		event.AddMetadata("previous_token_count", tokenCount)
		cm.publishEvent(ctx, models.NewEvent(models.EventTypeContextUpdated, models.EventSourceContext).WithData(event))
	}

	if cm.logger != nil {
		cm.logger.Info("Cleared conversation context", "context_id", contextID, "messages_removed", messageCount)
	}

	return nil
}

// CompactContext implements smart context compaction
func (cm *ContextManager) CompactContext(ctx context.Context, contextID string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	conversationContext, exists := cm.contexts[contextID]
	if !exists {
		return models.NewContextError(contextID, "compact", "context not found")
	}

	originalMessageCount := len(conversationContext.Messages)
	cm.compactContext(conversationContext)
	compactedMessageCount := len(conversationContext.Messages)

	// Record metrics
	if cm.metrics != nil {
		cm.metrics.IncrementCounter("contexts_compacted", map[string]string{
			"context_id": contextID,
		})
		cm.metrics.RecordValue("messages_removed_by_compaction", float64(originalMessageCount-compactedMessageCount), map[string]string{
			"context_id": contextID,
		})
	}

	// Publish event
	if cm.eventBus != nil {
		event := models.NewContextEvent(contextID, models.EventTypeContextUpdated)
		event.Success = true
		event.MessageCount = compactedMessageCount
		event.TokenCount = conversationContext.Stats.Tokens
		event.Operation = "compact"
		event.AddMetadata("original_message_count", originalMessageCount)
		event.AddMetadata("compacted_message_count", compactedMessageCount)
		cm.publishEvent(ctx, models.NewEvent(models.EventTypeContextUpdated, models.EventSourceContext).WithData(event))
	}

	if cm.logger != nil {
		cm.logger.Info("Compacted conversation context", "context_id", contextID,
			"original_messages", originalMessageCount, "compacted_messages", compactedMessageCount)
	}

	return nil
}

// TrimContext trims context to a maximum number of messages
func (cm *ContextManager) TrimContext(ctx context.Context, contextID string, maxMessages int) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	conversationContext, exists := cm.contexts[contextID]
	if !exists {
		return models.NewContextError(contextID, "trim", "context not found")
	}

	originalMessageCount := len(conversationContext.Messages)
	cm.trimContext(conversationContext, maxMessages)
	trimmedMessageCount := len(conversationContext.Messages)

	// Record metrics
	if cm.metrics != nil {
		cm.metrics.IncrementCounter("contexts_trimmed", map[string]string{
			"context_id": contextID,
		})
		cm.metrics.RecordValue("messages_removed_by_trimming", float64(originalMessageCount-trimmedMessageCount), map[string]string{
			"context_id": contextID,
		})
	}

	// Publish event
	if cm.eventBus != nil {
		event := models.NewContextEvent(contextID, models.EventTypeContextUpdated)
		event.Success = true
		event.MessageCount = trimmedMessageCount
		event.TokenCount = conversationContext.Stats.Tokens
		event.Operation = "trim"
		event.AddMetadata("original_message_count", originalMessageCount)
		event.AddMetadata("trimmed_message_count", trimmedMessageCount)
		cm.publishEvent(ctx, models.NewEvent(models.EventTypeContextUpdated, models.EventSourceContext).WithData(event))
	}

	if cm.logger != nil {
		cm.logger.Info("Trimmed conversation context", "context_id", contextID,
			"original_messages", originalMessageCount, "trimmed_messages", trimmedMessageCount)
	}

	return nil
}

// GetContextStats returns statistics about a context
func (cm *ContextManager) GetContextStats(ctx context.Context, contextID string) (*models.ContextStats, error) {
	conversationContext, err := cm.GetContext(ctx, contextID)
	if err != nil {
		return nil, err
	}

	stats := conversationContext.GetStats()
	// Update duration
	stats.Duration = time.Since(conversationContext.CreatedAt).Seconds()

	return &stats, nil
}

// UpdateTokenCount updates the token count for a context
func (cm *ContextManager) UpdateTokenCount(ctx context.Context, contextID string, additionalTokens int) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	conversationContext, exists := cm.contexts[contextID]
	if !exists {
		return models.NewContextError(contextID, "update_tokens", "context not found")
	}

	conversationContext.UpdateTokenCount(additionalTokens)

	// Record metrics
	if cm.metrics != nil {
		cm.metrics.RecordValue("tokens_used", float64(additionalTokens), map[string]string{
			"context_id": contextID,
		})
	}

	return nil
}

// ExportContext exports a context to a portable format
func (cm *ContextManager) ExportContext(ctx context.Context, contextID string) (*models.ContextExport, error) {
	conversationContext, err := cm.GetContext(ctx, contextID)
	if err != nil {
		return nil, err
	}

	export := &models.ContextExport{
		ContextID:  contextID,
		Messages:   conversationContext.Messages,
		Metadata:   conversationContext.Metadata,
		ExportedAt: time.Now(),
		Version:    "1.0",
	}

	// Calculate checksum
	data, err := json.Marshal(export)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal export data: %w", err)
	}

	// Simple checksum (in a real implementation, you'd use a proper hash)
	export.Checksum = fmt.Sprintf("%x", len(data))

	return export, nil
}

// ImportContext imports a context from an export
func (cm *ContextManager) ImportContext(ctx context.Context, contextID string, export *models.ContextExport) error {
	// Validate export
	if export.ContextID != contextID {
		return models.NewValidationError("context_id", "mismatch", "export context ID does not match target context ID", export.ContextID)
	}

	// Create context if it doesn't exist
	if err := cm.CreateContext(ctx, contextID); err != nil {
		// If context already exists, that's okay for import
		if !models.IsContextError(err) {
			return err
		}
	}

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	conversationContext := cm.contexts[contextID]

	// Import messages
	conversationContext.Messages = export.Messages
	conversationContext.Metadata = export.Metadata
	conversationContext.UpdatedAt = time.Now()
	conversationContext.LastAccessed = time.Now()

	// Recalculate stats
	conversationContext.Stats.Messages = len(export.Messages)
	conversationContext.Stats.LastUpdated = time.Now()

	if cm.logger != nil {
		cm.logger.Info("Imported conversation context", "context_id", contextID, "messages", len(export.Messages))
	}

	return nil
}

// CleanupExpiredContexts removes contexts that haven't been accessed within the TTL
func (cm *ContextManager) CleanupExpiredContexts(ctx context.Context) (*models.CleanupResult, error) {
	if cm.config.ContextTTL <= 0 {
		return &models.CleanupResult{
			Operation: "cleanup_expired_contexts",
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Success:   true,
			Message:   "Context TTL not configured, no cleanup performed",
		}, nil
	}

	startTime := time.Now()
	expiry := time.Now().Add(-cm.config.ContextTTL)

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	var expiredContexts []string
	for contextID, conversationContext := range cm.contexts {
		if conversationContext.LastAccessed.Before(expiry) {
			expiredContexts = append(expiredContexts, contextID)
		}
	}

	// Remove expired contexts
	for _, contextID := range expiredContexts {
		delete(cm.contexts, contextID)

		// Publish event
		if cm.eventBus != nil {
			event := models.NewContextEvent(contextID, models.EventTypeContextExpired)
			event.Success = true
			cm.publishEvent(ctx, models.NewEvent(models.EventTypeContextExpired, models.EventSourceContext).WithData(event))
		}

		if cm.logger != nil {
			cm.logger.Debug("Expired context", "context_id", contextID)
		}
	}

	endTime := time.Now()

	// Record metrics
	if cm.metrics != nil {
		cm.metrics.RecordValue("contexts_expired", float64(len(expiredContexts)), map[string]string{})
		cm.metrics.RecordDuration("cleanup_duration", endTime.Sub(startTime).Seconds()*1000, map[string]string{})
	}

	result := &models.CleanupResult{
		Operation:      "cleanup_expired_contexts",
		StartTime:      startTime,
		EndTime:        endTime,
		Duration:       endTime.Sub(startTime).Seconds() * 1000,
		ItemsProcessed: int64(len(cm.contexts) + len(expiredContexts)),
		ItemsRemoved:   int64(len(expiredContexts)),
		Success:        true,
		Message:        fmt.Sprintf("Cleaned up %d expired contexts", len(expiredContexts)),
	}

	if cm.logger != nil {
		cm.logger.Info("Cleaned up expired contexts", "expired_count", len(expiredContexts), "total_contexts", len(cm.contexts))
	}

	return result, nil
}

// GetActiveContextCount returns the number of active contexts
func (cm *ContextManager) GetActiveContextCount(ctx context.Context) (int, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return len(cm.contexts), nil
}

// ValidateContextIntegrity validates the integrity of a context
func (cm *ContextManager) ValidateContextIntegrity(ctx context.Context, contextID string) error {
	conversationContext, err := cm.GetContext(ctx, contextID)
	if err != nil {
		return err
	}

	// Validate message sequence
	for i, message := range conversationContext.Messages {
		if message.Timestamp.IsZero() {
			return fmt.Errorf("message %d has invalid timestamp", i)
		}

		if i > 0 && message.Timestamp.Before(conversationContext.Messages[i-1].Timestamp) {
			return fmt.Errorf("message %d timestamp is before previous message", i)
		}

		// Validate tool call sequences
		if message.Role == models.RoleAssistant && len(message.ToolCalls) > 0 {
			for _, toolCall := range message.ToolCalls {
				if toolCall.ID == "" {
					return fmt.Errorf("message %d has tool call with empty ID", i)
				}
				if toolCall.Name == "" {
					return fmt.Errorf("message %d has tool call with empty name", i)
				}
			}
		}

		if message.Role == models.RoleTool && message.ToolCallID == "" {
			return fmt.Errorf("message %d is a tool message without tool call ID", i)
		}
	}

	// Validate stats consistency
	actualMessageCount := len(conversationContext.Messages)
	if conversationContext.Stats.Messages != actualMessageCount {
		return fmt.Errorf("stats message count (%d) does not match actual count (%d)",
			conversationContext.Stats.Messages, actualMessageCount)
	}

	return nil
}

// Internal helper methods

func (cm *ContextManager) compactContext(conversationContext *models.ConversationContext) {
	maxMessages := cm.config.MaxContextMessages
	if len(conversationContext.Messages) <= maxMessages {
		return
	}

	if cm.logger != nil {
		cm.logger.Debug("Compacting context", "context_id", conversationContext.ID,
			"current_messages", len(conversationContext.Messages), "target_messages", maxMessages)
	}

	// Smart compaction strategy: keep the first message and the last N-1 messages
	// while ensuring tool call sequences are not broken
	keepRecent := max(maxMessages-1, 1)
	startIndex := len(conversationContext.Messages) - keepRecent

	// Find a safe cut point that doesn't break tool call sequences
	safeStartIndex := cm.findSafeCutPoint(conversationContext.Messages, startIndex)

	firstMsg := conversationContext.Messages[0]
	recentMsgs := conversationContext.Messages[safeStartIndex:]

	// If including the safe recent messages would still exceed limit,
	// use the tail with complete tool sequences
	if len(recentMsgs)+1 > maxMessages {
		recentMsgs = cm.getTailWithCompleteToolSequences(conversationContext.Messages, maxMessages-1)
	}

	conversationContext.Messages = append([]models.Message{firstMsg}, recentMsgs...)
	conversationContext.Stats.Messages = len(conversationContext.Messages)
	conversationContext.Stats.LastUpdated = time.Now()
	conversationContext.UpdatedAt = time.Now()
}

func (cm *ContextManager) trimContext(conversationContext *models.ConversationContext, maxMessages int) {
	if len(conversationContext.Messages) <= maxMessages {
		return
	}

	if cm.logger != nil {
		cm.logger.Debug("Trimming context", "context_id", conversationContext.ID,
			"current_messages", len(conversationContext.Messages), "target_messages", maxMessages)
	}

	// Keep the most recent messages while preserving tool sequences
	safeMessages := cm.getTailWithCompleteToolSequences(conversationContext.Messages, maxMessages)
	conversationContext.Messages = safeMessages
	conversationContext.Stats.Messages = len(conversationContext.Messages)
	conversationContext.Stats.LastUpdated = time.Now()
	conversationContext.UpdatedAt = time.Now()
}

func (cm *ContextManager) findSafeCutPoint(messages []models.Message, desiredStart int) int {
	if desiredStart <= 0 {
		return 0
	}

	// Look backwards from desired start to find a safe cut point
	for i := desiredStart; i >= 0; i-- {
		if cm.isSafeCutPoint(messages, i) {
			return i
		}
	}

	return 0
}

func (cm *ContextManager) isSafeCutPoint(messages []models.Message, index int) bool {
	if index >= len(messages) {
		return true
	}

	// Track any incomplete tool sequences
	pendingToolCalls := make(map[string]bool)

	// Scan backwards from this index to check for incomplete tool sequences
	for i := index - 1; i >= 0; i-- {
		msg := messages[i]

		if msg.Role == models.RoleAssistant && len(msg.ToolCalls) > 0 {
			// Found assistant with tool calls - track them
			for _, tc := range msg.ToolCalls {
				pendingToolCalls[tc.ID] = true
			}
		} else if msg.Role == models.RoleTool && msg.ToolCallID != "" {
			// Found tool response - remove from pending
			delete(pendingToolCalls, msg.ToolCallID)
		}
	}

	// If there are pending tool calls at this cut point, it's not safe
	return len(pendingToolCalls) == 0
}

func (cm *ContextManager) getTailWithCompleteToolSequences(messages []models.Message, maxCount int) []models.Message {
	if len(messages) <= maxCount {
		return messages
	}

	// Start from the end and work backwards, keeping complete sequences
	kept := []models.Message{}
	pendingToolCalls := make(map[string]bool)

	// Process messages in reverse order
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]

		if msg.Role == models.RoleTool && msg.ToolCallID != "" {
			// Tool response - we MUST keep this and find its assistant call
			kept = append([]models.Message{msg}, kept...)
			pendingToolCalls[msg.ToolCallID] = true
		} else if msg.Role == models.RoleAssistant && len(msg.ToolCalls) > 0 {
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
				kept = append([]models.Message{msg}, kept...)
			} else if len(pendingToolCalls) == 0 && len(kept) < maxCount {
				// Regular assistant with tool calls, keep if we have space
				kept = append([]models.Message{msg}, kept...)
			}
		} else {
			// Regular message - keep if no pending tool calls and we have space
			if len(pendingToolCalls) == 0 && len(kept) < maxCount {
				kept = append([]models.Message{msg}, kept...)
			}
		}

		// If we have no pending tool calls and we've reached our target, we can stop
		if len(pendingToolCalls) == 0 && len(kept) >= maxCount {
			// Check if any remaining messages are tool messages
			hasMoreToolMessages := false
			for j := i - 1; j >= 0; j-- {
				if messages[j].Role == models.RoleTool && messages[j].ToolCallID != "" {
					hasMoreToolMessages = true
					break
				}
			}
			if !hasMoreToolMessages {
				break
			}
		}
	}

	return kept
}

func (cm *ContextManager) startCleanupRoutine() {
	cm.cleanupTicker = time.NewTicker(5 * time.Minute) // Run every 5 minutes

	go func() {
		for {
			select {
			case <-cm.cleanupTicker.C:
				ctx := context.Background()
				if result, err := cm.CleanupExpiredContexts(ctx); err != nil {
					if cm.logger != nil {
						cm.logger.Error("Context cleanup failed", "error", err)
					}
				} else if cm.logger != nil && result.ItemsRemoved > 0 {
					cm.logger.Info("Context cleanup completed", "expired_contexts", result.ItemsRemoved)
				}
			case <-cm.stopCleanup:
				cm.cleanupTicker.Stop()
				return
			}
		}
	}()
}

func (cm *ContextManager) publishEvent(ctx context.Context, event *models.Event) {
	if cm.eventBus != nil {
		if err := cm.eventBus.PublishAsync(ctx, event); err != nil {
			if cm.logger != nil {
				cm.logger.Error("Failed to publish context event", "event_type", event.Type, "error", err)
			}
		}
	}
}

// Stop stops the context manager and cleanup routines
func (cm *ContextManager) Stop() {
	if cm.cleanupTicker != nil {
		close(cm.stopCleanup)
	}
}

// Helper function for Go versions that don't have max built-in
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
