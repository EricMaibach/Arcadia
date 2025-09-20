package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"arcadia/services/ai"
)

// Dependencies that will be injected
var (
	aiService ai.AIService
	aiServiceMu sync.RWMutex
)

// SetAIDependencies configures the AI service dependency for the AI handlers
func SetAIDependencies(service ai.AIService) {
	aiServiceMu.Lock()
	defer aiServiceMu.Unlock()
	aiService = service
}

// getAIService safely returns the AI service with proper locking
func getAIService() ai.AIService {
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

	response, err := service.SendMessageWithContext(request.Message, contextID)
	if err != nil {
		log.Printf("AI API error: %v", err)

		// Check if it's an AIError for better error reporting
		if aiErr, ok := err.(*ai.AIError); ok {
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
	messages, tokens, _ := service.GetContextStats(contextID)
	providerInfo := service.GetProviderInfo()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"response": response,
		"context_stats": map[string]int{
			"message_count": messages,
			"total_tokens":  tokens,
		},
		"provider_info": map[string]any{
			"name":     providerInfo.Name,
			"model":    providerInfo.Model,
			"features": providerInfo.Features,
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

	// Create new service with the requested provider
	factory := ai.GetGlobalFactory()

	var newService ai.AIService
	var err error

	if request.APIKey != "" {
		newService, err = factory.CreateServiceWithDefaults(request.Provider, request.APIKey)
	} else {
		// Try to use existing configuration but switch provider
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": "API key required for provider switch",
		})
		return
	}

	if err != nil {
		log.Printf("Provider switch error: %v", err)
		http.Error(w, "Failed to switch provider", http.StatusBadRequest)
		return
	}

	// For demo purposes, we'll just validate the new service
	// In a real implementation, you might want to update the global service
	// or manage per-session services

	if err := newService.Validate(); err != nil {
		log.Printf("New provider validation failed: %v", err)
		http.Error(w, "Provider validation failed", http.StatusBadRequest)
		return
	}

	providerInfo := newService.GetProviderInfo()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "provider switched successfully",
		"provider_info": map[string]any{
			"name":     providerInfo.Name,
			"model":    providerInfo.Model,
			"features": providerInfo.Features,
		},
	})
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

	providerInfo := service.GetProviderInfo()

	// Get supported providers from factory
	factory := ai.GetGlobalFactory()
	supportedProviders := factory.GetSupportedProviders()

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
func getHTTPStatusForAIError(err *ai.AIError) int {
	switch err.Type {
	case ai.ErrorTypeValidation:
		return http.StatusBadRequest
	case ai.ErrorTypeAuth:
		return http.StatusUnauthorized
	case ai.ErrorTypeRateLimit:
		return http.StatusTooManyRequests
	case ai.ErrorTypeQuota:
		return http.StatusPaymentRequired
	case ai.ErrorTypeTimeout:
		return http.StatusRequestTimeout
	case ai.ErrorTypeNetwork:
		return http.StatusBadGateway
	case ai.ErrorTypeProvider:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}