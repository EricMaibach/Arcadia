package claude

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"arcadia/services/ai"
)

// TestHTTPClientCreation tests basic HTTP client creation and configuration
func TestHTTPClientCreation(t *testing.T) {
	client := ai.NewHTTPClient(30)
	if client == nil {
		t.Fatal("HTTP client should not be nil")
	}

	// Test timeout setting
	if client.GetTimeout() != 30*time.Second {
		t.Errorf("Expected 30 second timeout, got %v", client.GetTimeout())
	}

	// Test base URL setting
	client.SetBaseURL("https://api.example.com")

	// Test header setting
	client.SetHeader("Authorization", "Bearer test-token")
	client.SetHeader("Content-Type", "application/json")

	// Test multiple headers
	headers := map[string]string{
		"User-Agent":    "Test-Agent/1.0",
		"Custom-Header": "test-value",
	}
	client.SetHeaders(headers)
}

// TestHTTPClientTimeout tests timeout functionality
func TestHTTPClientTimeout(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("slow response"))
	}))
	defer server.Close()

	// Create client with short timeout
	client := ai.NewHTTPClient(1) // 1 second timeout
	client.SetBaseURL(server.URL)

	// Test GET request timeout
	_, err := client.Get("/test")
	if err == nil {
		t.Error("Expected timeout error")
	}

	// Test POST request timeout
	_, err = client.PostJSON("/test", map[string]string{"test": "data"})
	if err == nil {
		t.Error("Expected timeout error")
	}
}

// TestHTTPErrorHandling tests error response handling
func TestHTTPErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedError  string
	}{
		{
			name:          "Unauthorized",
			statusCode:    http.StatusUnauthorized,
			responseBody:  `{"error": "invalid api key"}`,
			expectedError: ai.ErrorTypeAuth,
		},
		{
			name:          "Rate Limited",
			statusCode:    http.StatusTooManyRequests,
			responseBody:  `{"error": "rate limit exceeded"}`,
			expectedError: ai.ErrorTypeRateLimit,
		},
		{
			name:          "Payment Required",
			statusCode:    http.StatusPaymentRequired,
			responseBody:  `{"error": "quota exceeded"}`,
			expectedError: ai.ErrorTypeQuota,
		},
		{
			name:          "Timeout",
			statusCode:    http.StatusRequestTimeout,
			responseBody:  `{"error": "request timeout"}`,
			expectedError: ai.ErrorTypeTimeout,
		},
		{
			name:          "Server Error",
			statusCode:    http.StatusInternalServerError,
			responseBody:  `{"error": "internal server error"}`,
			expectedError: ai.ErrorTypeProvider,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := ai.NewHTTPClient(30)
			client.SetBaseURL(server.URL)

			resp, err := client.Get("/test")
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			err = client.ValidateResponse(resp, "test-provider")
			if err == nil {
				t.Error("Expected error response")
			}

			if aiErr, ok := err.(*ai.AIError); ok {
				if aiErr.Type != tt.expectedError {
					t.Errorf("Expected error type '%s', got '%s'", tt.expectedError, aiErr.Type)
				}
				if aiErr.Provider != "test-provider" {
					t.Errorf("Expected provider 'test-provider', got '%s'", aiErr.Provider)
				}
				if aiErr.Code != tt.statusCode {
					t.Errorf("Expected code %d, got %d", tt.statusCode, aiErr.Code)
				}
			} else {
				t.Error("Expected AIError type")
			}
		})
	}
}

// TestHTTPSuccessfulRequests tests successful HTTP operations
func TestHTTPSuccessfulRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back request info (for potential future use)
		_ = map[string]string{
			"method":      r.Method,
			"path":        r.URL.Path,
			"content-type": r.Header.Get("Content-Type"),
			"auth":        r.Header.Get("Authorization"),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	client := ai.NewHTTPClient(30)
	client.SetBaseURL(server.URL)
	client.SetHeader("Authorization", "Bearer test-token")

	// Test GET request
	t.Run("GET request", func(t *testing.T) {
		resp, err := client.Get("/test")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 status, got %d", resp.StatusCode)
		}

		// Test response validation
		err = client.ValidateResponse(resp, "test-provider")
		if err != nil {
			t.Errorf("Response validation failed: %v", err)
		}
	})

	// Test POST JSON request
	t.Run("POST JSON request", func(t *testing.T) {
		payload := map[string]any{
			"message": "test message",
			"tokens":  100,
		}

		resp, err := client.PostJSON("/test", payload)
		if err != nil {
			t.Fatalf("POST JSON request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 status, got %d", resp.StatusCode)
		}

		// Verify content type was set
		if resp.Request.Header.Get("Content-Type") != "application/json" {
			t.Error("Expected Content-Type to be application/json")
		}
	})

	// Test POST with string payload
	t.Run("POST string request", func(t *testing.T) {
		resp, err := client.Post("/test", "test payload", "text/plain")
		if err != nil {
			t.Fatalf("POST string request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 status, got %d", resp.StatusCode)
		}
	})
}

// TestHTTPClientCloning tests HTTP client cloning functionality
func TestHTTPClientCloning(t *testing.T) {
	original := ai.NewHTTPClient(60)
	original.SetBaseURL("https://api.example.com")
	original.SetHeader("Authorization", "Bearer original-token")
	original.SetHeader("User-Agent", "Original-Client/1.0")

	// Clone the client
	cloned := original.Clone()

	// Verify timeout is preserved
	if cloned.GetTimeout() != original.GetTimeout() {
		t.Error("Cloned client should have same timeout")
	}

	// Modify cloned client
	cloned.SetHeader("Authorization", "Bearer cloned-token")
	cloned.SetTimeout(30)

	// Verify original is not affected
	if original.GetTimeout() == cloned.GetTimeout() {
		t.Error("Original client should not be affected by cloned client changes")
	}
}

// TestClaudeServiceHTTPIntegration tests Claude service HTTP integration
func TestClaudeServiceHTTPIntegration(t *testing.T) {
	// Mock Claude API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("x-api-key") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": {"type": "authentication_error", "message": "Invalid API key"}}`))
			return
		}

		if r.Header.Get("anthropic-version") != "2023-06-01" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": {"type": "invalid_request_error", "message": "Missing anthropic-version header"}}`))
			return
		}

		// Mock successful response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"content": [
				{
					"type": "text",
					"text": "Hello! This is a test response."
				}
			],
			"stop_reason": "end_turn",
			"usage": {
				"input_tokens": 10,
				"output_tokens": 8
			}
		}`))
	}))
	defer server.Close()

	config := &ai.AIConfig{
		Provider:       "claude",
		MaxTokens:      4096,
		TimeoutSeconds: 30,
		ProviderSettings: map[string]any{
			"api_key":  "test-api-key",
			"base_url": server.URL,
			"model":    "claude-3-5-sonnet-20241022",
		},
	}

	service, err := NewClaudeService(config)
	if err != nil {
		t.Fatalf("Failed to create Claude service: %v", err)
	}

	// Test successful API call
	t.Run("Successful API call", func(t *testing.T) {
		response, err := service.SendMessage("Hello, Claude!")
		if err != nil {
			t.Errorf("SendMessage failed: %v", err)
		} else {
			if response != "Hello! This is a test response." {
				t.Errorf("Unexpected response: %s", response)
			}
		}
	})

	// Test authentication error
	t.Run("Authentication error", func(t *testing.T) {
		// Create service with invalid API key
		badConfig := &ai.AIConfig{
			Provider:       "claude",
			MaxTokens:      4096,
			TimeoutSeconds: 30,
			ProviderSettings: map[string]any{
				"api_key":  "", // Empty API key
				"base_url": server.URL,
				"model":    "claude-3-5-sonnet-20241022",
			},
		}

		badService, err := NewClaudeService(badConfig)
		if err != nil {
			t.Fatalf("Failed to create Claude service: %v", err)
		}

		_, err = badService.SendMessage("Hello, Claude!")
		if err == nil {
			t.Error("Expected authentication error")
		}
	})
}

// TestHTTPResponseProcessing tests response processing functionality
func TestHTTPResponseProcessing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "test response", "status": "success"}`))
	}))
	defer server.Close()

	client := ai.NewHTTPClient(30)
	client.SetBaseURL(server.URL)

	resp, err := client.Get("/test")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	// Test JSON decoding
	t.Run("JSON decoding", func(t *testing.T) {
		var result map[string]any
		err := client.DecodeJSONResponse(resp, &result)
		if err != nil {
			t.Errorf("JSON decoding failed: %v", err)
		}

		if result["message"] != "test response" {
			t.Errorf("Expected 'test response', got '%v'", result["message"])
		}
	})

	// Test reading response body
	t.Run("Read response body", func(t *testing.T) {
		// Make another request since body was consumed
		resp2, err := client.Get("/test")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		body, err := client.ReadResponseBody(resp2)
		if err != nil {
			t.Errorf("Reading response body failed: %v", err)
		}

		if body != `{"message": "test response", "status": "success"}` {
			t.Errorf("Unexpected response body: %s", body)
		}
	})
}