package ai

import (
	"log"
	"net/http"
)

// Backward compatibility functions for existing Claude-specific code

// HandleClaudeAPI provides backward compatibility for existing Claude API endpoints
func HandleClaudeAPI(w http.ResponseWriter, r *http.Request) {
	service := GetGlobalService()
	if service == nil {
		log.Printf("[AI Compatibility] Warning: Global AI service not initialized")
		http.Error(w, "AI service not available", http.StatusServiceUnavailable)
		return
	}

	handler := NewHTTPHandler(service)
	handler.HandleAIAPI(w, r)
}

// TriggerClaudeToolRefresh provides backward compatibility for tool refresh
func TriggerClaudeToolRefresh() {
	TriggerGlobalToolRefresh()
}

// GetClaudeService provides backward compatibility (returns generic AI service)
// Note: This now returns AIService instead of *ClaudeService
func GetClaudeService() AIService {
	return GetGlobalService()
}

// ClaudeServiceCompat provides a compatibility wrapper that mimics the old ClaudeService interface
// This allows existing code that depends on the specific ClaudeService methods to continue working
type ClaudeServiceCompat struct {
	aiService AIService
}

// NewClaudeServiceCompat creates a new compatibility wrapper
func NewClaudeServiceCompat(aiService AIService) *ClaudeServiceCompat {
	return &ClaudeServiceCompat{
		aiService: aiService,
	}
}

// SendMessage implements the old ClaudeService.SendMessage method
func (c *ClaudeServiceCompat) SendMessage(message string) (string, error) {
	if c.aiService == nil {
		return "", &AIError{
			Type:     ErrorTypeValidation,
			Message:  "AI service not initialized",
			Provider: "compatibility",
		}
	}
	return c.aiService.SendMessage(message)
}

// SendMessageWithContext implements the old ClaudeService.SendMessageWithContext method
func (c *ClaudeServiceCompat) SendMessageWithContext(message string, contextID string) (string, error) {
	if c.aiService == nil {
		return "", &AIError{
			Type:     ErrorTypeValidation,
			Message:  "AI service not initialized",
			Provider: "compatibility",
		}
	}
	return c.aiService.SendMessageWithContext(message, contextID)
}

// ClearContext implements the old ClaudeService.ClearContext method
func (c *ClaudeServiceCompat) ClearContext(contextID string) {
	if c.aiService != nil {
		c.aiService.ClearContext(contextID)
	}
}

// GetContextStats implements the old ClaudeService.GetContextStats method
func (c *ClaudeServiceCompat) GetContextStats(contextID string) (messages int, tokens int, exists bool) {
	if c.aiService == nil {
		return 0, 0, false
	}
	return c.aiService.GetContextStats(contextID)
}

// TriggerToolRefresh implements the old ClaudeService.TriggerToolRefresh method
func (c *ClaudeServiceCompat) TriggerToolRefresh() {
	if c.aiService != nil {
		c.aiService.TriggerToolRefresh()
	}
}

// RefreshMCPTools provides backward compatibility for the old RefreshMCPTools method
func (c *ClaudeServiceCompat) RefreshMCPTools() {
	if c.aiService != nil {
		c.aiService.RefreshTools()
	}
}

// HandleClaudeAPI implements the old ClaudeService.HandleClaudeAPI method
func (c *ClaudeServiceCompat) HandleClaudeAPI(w http.ResponseWriter, r *http.Request) {
	if c.aiService == nil {
		log.Printf("[AI Compatibility] Warning: AI service not initialized")
		http.Error(w, "AI service not available", http.StatusServiceUnavailable)
		return
	}

	handler := NewHTTPHandler(c.aiService)
	handler.HandleAIAPI(w, r)
}

// GetGlobalClaudeServiceCompat returns a compatibility wrapper for the global AI service
// This function can be used by existing code that expects a ClaudeService-like interface
func GetGlobalClaudeServiceCompat() *ClaudeServiceCompat {
	service := GetGlobalService()
	if service == nil {
		return &ClaudeServiceCompat{aiService: nil}
	}
	return NewClaudeServiceCompat(service)
}

// Legacy function aliases for backward compatibility
// These functions maintain the exact same signatures as the old Claude service

// LegacySendMessage sends a message using the global AI service (backward compatibility)
func LegacySendMessage(message string) (string, error) {
	return SendGlobalMessage(message)
}

// LegacySendMessageWithContext sends a message with context using the global AI service (backward compatibility)
func LegacySendMessageWithContext(message string, contextID string) (string, error) {
	return SendGlobalMessageWithContext(message, contextID)
}

// LegacyClearContext clears a context using the global AI service (backward compatibility)
func LegacyClearContext(contextID string) {
	service := GetGlobalService()
	if service != nil {
		service.ClearContext(contextID)
	}
}

// LegacyGetContextStats gets context statistics using the global AI service (backward compatibility)
func LegacyGetContextStats(contextID string) (messages int, tokens int, exists bool) {
	service := GetGlobalService()
	if service == nil {
		return 0, 0, false
	}
	return service.GetContextStats(contextID)
}

// LegacyTriggerToolRefresh triggers tool refresh using the global AI service (backward compatibility)
func LegacyTriggerToolRefresh() {
	TriggerGlobalToolRefresh()
}