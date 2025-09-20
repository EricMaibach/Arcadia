package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"arcadia/modules/ai"
	"arcadia/modules/ai/models"
)

// Dependencies that will be injected
var (
	aiService ai.AIModule
	aiServiceMu sync.RWMutex
)

// SetAIDependencies configures the AI service dependency for the AI handlers
func SetAIDependencies(service ai.AIModule) {
	aiServiceMu.Lock()
	defer aiServiceMu.Unlock()
	aiService = service
}

// getAIService safely returns the AI service with proper locking
func getAIService() ai.AIModule {
	aiServiceMu.RLock()
	defer aiServiceMu.RUnlock()
	return aiService
}

// HandleAIAPI handles AI chat requests (provider-agnostic)
func HandleAIAPI(w http.ResponseWriter, r *http.Request) {
	service := getAIService()
	if service == nil {
		http.Error(w, "AI service not initialized", http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Message   string `json:"message"`
		ContextID string `json:"context_id"`
		SessionID string `json:"session_id"` // deprecated, for backward compatibility
		Provider  string `json:"provider,omitempty"` // Optional provider override
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if request.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Use context_id if present, fall back to session_id for backward compatibility
	contextParam := request.ContextID
	if contextParam == "" && request.SessionID != "" {
		contextParam = request.SessionID
	}

	// Generate context ID for web users
	contextID := "web:default"
	if contextParam != "" {
		contextID = "web:" + contextParam
	}

	response, err := service.SendMessageWithContext(r.Context(), request.Message, contextID)
	if err != nil {
		log.Printf("AI API error: %v", err)

		// Check if it's an AIError for better error reporting
		if aiErr, ok := err.(*models.AIError); ok {
			w.WriteHeader(getHTTPStatusForAIError(aiErr))
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"type":     aiErr.Type,
					"message":  aiErr.Message,
					"provider": aiErr.Provider,
				},
			})
			return
		}

		http.Error(w, "Failed to get AI response", http.StatusInternalServerError)
		return
	}

	// Include context stats and provider info in response
	ctxStats, _ := service.GetContextStats(r.Context(), contextID)
	providerInfo, _ := service.GetProviderInfo(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"response": response,
		"context_stats": map[string]interface{}{
			"message_count": func() int {
				if ctxStats != nil {
					return ctxStats.Messages
				}
				return 0
			}(),
			"total_tokens": func() int {
				if ctxStats != nil {
					return ctxStats.Tokens
				}
				return 0
			}(),
		},
		"provider_info": map[string]interface{}{
			"name": func() string {
				if providerInfo != nil {
					return providerInfo.Name
				}
				return "unknown"
			}(),
			"model": func() string {
				if providerInfo != nil {
					return providerInfo.Model
				}
				return "unknown"
			}(),
			"features": func() []string {
				if providerInfo != nil {
					return providerInfo.Features
				}
				return []string{}
			}(),
		},
	})
}

// HandleProviderSwitch allows switching providers mid-conversation
func HandleProviderSwitch(w http.ResponseWriter, r *http.Request) {
	service := getAIService()
	if service == nil {
		http.Error(w, "AI service not initialized", http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Provider  string `json:"provider"`
		APIKey    string `json:"api_key,omitempty"`
		ContextID string `json:"context_id"`
		SessionID string `json:"session_id"` // deprecated, for backward compatibility
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Provider switching functionality - simplified for new architecture
	// In the new module architecture, provider switching should be handled
	// through the module's SwitchProvider method
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": "Provider switching not yet implemented in new architecture",
		"message": "This feature will be available in a future update",
	})
	return

}

// HandleProviderStatus returns current provider information
func HandleProviderStatus(w http.ResponseWriter, r *http.Request) {
	service := getAIService()
	if service == nil {
		http.Error(w, "AI service not initialized", http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	providerInfo, _ := service.GetProviderInfo(r.Context())

	// Get supported providers - simplified for new architecture
	supportedProviders := []string{"openai"} // Would come from module configuration

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"current_provider": map[string]any{
			"name":     providerInfo.Name,
			"model":    providerInfo.Model,
			"features": providerInfo.Features,
		},
		"supported_providers": supportedProviders,
	})
}

// Helper function to map AIError types to HTTP status codes
func getHTTPStatusForAIError(err *models.AIError) int {
	switch err.Type {
	case models.ErrorTypeValidation:
		return http.StatusBadRequest
	case models.ErrorTypeAuth:
		return http.StatusUnauthorized
	case models.ErrorTypeRateLimit:
		return http.StatusTooManyRequests
	case models.ErrorTypeQuota:
		return http.StatusPaymentRequired
	case models.ErrorTypeTimeout:
		return http.StatusRequestTimeout
	case models.ErrorTypeNetwork:
		return http.StatusBadGateway
	case models.ErrorTypeProvider:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}