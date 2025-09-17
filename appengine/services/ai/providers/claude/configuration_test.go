package claude

import (
	"testing"

	"arcadia/services/ai"
)

// TestConfigurationValidation tests the AIConfig validation system
func TestConfigurationValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *ai.AIConfig
		expectError bool
		errorType   string
	}{
		{
			name: "Valid configuration",
			config: &ai.AIConfig{
				Provider:       "claude",
				MaxTokens:      4096,
				TimeoutSeconds: 120,
				ProviderSettings: map[string]any{
					"api_key":  "test-key",
					"base_url": "https://api.anthropic.com",
					"model":    "claude-3-5-sonnet-20241022",
				},
			},
			expectError: false,
		},
		{
			name: "Missing provider",
			config: &ai.AIConfig{
				MaxTokens:      4096,
				TimeoutSeconds: 120,
			},
			expectError: true,
			errorType:   "validation",
		},
		{
			name: "Invalid provider",
			config: &ai.AIConfig{
				Provider:   "invalid-provider",
				MaxTokens:  4096,
			},
			expectError: true,
			errorType:   "validation",
		},
		{
			name: "Zero max tokens (should be set to default)",
			config: &ai.AIConfig{
				Provider:       "claude",
				MaxTokens:      0,
				TimeoutSeconds: 120,
				ProviderSettings: map[string]any{
					"api_key": "test-key",
				},
			},
			expectError: false,
		},
		{
			name: "Zero timeout (should be set to default)",
			config: &ai.AIConfig{
				Provider:  "claude",
				MaxTokens: 4096,
				ProviderSettings: map[string]any{
					"api_key": "test-key",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}

			// Check that defaults are set for valid configs
			if !tt.expectError && err == nil {
				if tt.config.MaxTokens <= 0 {
					t.Error("MaxTokens should be set to default value")
				}
				if tt.config.TimeoutSeconds <= 0 {
					t.Error("TimeoutSeconds should be set to default value")
				}
				if tt.config.MaxContextMessages <= 0 {
					t.Error("MaxContextMessages should be set to default value")
				}
				if tt.config.ContextTTLMinutes <= 0 {
					t.Error("ContextTTLMinutes should be set to default value")
				}
			}
		})
	}
}

// TestProviderSpecificConfiguration tests Claude-specific configuration parsing
func TestProviderSpecificConfiguration(t *testing.T) {
	tests := []struct {
		name           string
		providerSettings map[string]any
		expectedAPIKey string
		expectedBaseURL string
		expectedModel  string
		expectError    bool
	}{
		{
			name: "Complete configuration",
			providerSettings: map[string]any{
				"api_key":  "sk-test123",
				"base_url": "https://custom.anthropic.com",
				"model":    "claude-3-haiku",
			},
			expectedAPIKey:  "sk-test123",
			expectedBaseURL: "https://custom.anthropic.com",
			expectedModel:   "claude-3-haiku",
			expectError:     false,
		},
		{
			name: "Minimal configuration with defaults",
			providerSettings: map[string]any{
				"api_key": "sk-test456",
			},
			expectedAPIKey:  "sk-test456",
			expectedBaseURL: "https://api.anthropic.com",
			expectedModel:   "claude-3-5-sonnet-20241022",
			expectError:     false,
		},
		{
			name: "Empty configuration",
			providerSettings: map[string]any{},
			expectedAPIKey:  "",
			expectedBaseURL: "https://api.anthropic.com",
			expectedModel:   "claude-3-5-sonnet-20241022",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ai.AIConfig{
				Provider:         "claude",
				MaxTokens:        4096,
				TimeoutSeconds:   120,
				ProviderSettings: tt.providerSettings,
			}

			claudeConfig, err := parseClaudeConfig(config)

			if tt.expectError && err == nil {
				t.Error("Expected parsing error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no parsing error but got: %v", err)
			}

			if !tt.expectError {
				if claudeConfig.APIKey != tt.expectedAPIKey {
					t.Errorf("Expected API key '%s', got '%s'", tt.expectedAPIKey, claudeConfig.APIKey)
				}
				if claudeConfig.BaseURL != tt.expectedBaseURL {
					t.Errorf("Expected base URL '%s', got '%s'", tt.expectedBaseURL, claudeConfig.BaseURL)
				}
				if claudeConfig.Model != tt.expectedModel {
					t.Errorf("Expected model '%s', got '%s'", tt.expectedModel, claudeConfig.Model)
				}
			}
		})
	}
}

// TestConfigurationHelpers tests the helper methods for accessing provider settings
func TestConfigurationHelpers(t *testing.T) {
	config := &ai.AIConfig{
		Provider: "claude",
		ProviderSettings: map[string]any{
			"string_value":  "test-string",
			"bool_value":    true,
			"int_value":     42,
			"float_value":   3.14,
			"invalid_value": []string{"not", "supported"},
		},
	}

	// Test GetProviderString
	t.Run("GetProviderString", func(t *testing.T) {
		// Valid string
		if val := config.GetProviderString("string_value"); val != "test-string" {
			t.Errorf("Expected 'test-string', got '%s'", val)
		}

		// Non-existent key
		if val := config.GetProviderString("non_existent"); val != "" {
			t.Errorf("Expected empty string for non-existent key, got '%s'", val)
		}

		// Wrong type
		if val := config.GetProviderString("bool_value"); val != "" {
			t.Errorf("Expected empty string for wrong type, got '%s'", val)
		}
	})

	// Test GetProviderBool
	t.Run("GetProviderBool", func(t *testing.T) {
		// Valid bool
		if val := config.GetProviderBool("bool_value"); !val {
			t.Error("Expected true, got false")
		}

		// Non-existent key
		if val := config.GetProviderBool("non_existent"); val {
			t.Error("Expected false for non-existent key, got true")
		}

		// Wrong type
		if val := config.GetProviderBool("string_value"); val {
			t.Error("Expected false for wrong type, got true")
		}
	})

	// Test GetProviderInt
	t.Run("GetProviderInt", func(t *testing.T) {
		// Valid int
		if val := config.GetProviderInt("int_value"); val != 42 {
			t.Errorf("Expected 42, got %d", val)
		}

		// Valid float (should convert)
		if val := config.GetProviderInt("float_value"); val != 3 {
			t.Errorf("Expected 3 (converted from float), got %d", val)
		}

		// Non-existent key
		if val := config.GetProviderInt("non_existent"); val != 0 {
			t.Errorf("Expected 0 for non-existent key, got %d", val)
		}

		// Wrong type
		if val := config.GetProviderInt("string_value"); val != 0 {
			t.Errorf("Expected 0 for wrong type, got %d", val)
		}
	})
}

// TestServiceValidation tests the Claude service validation logic
func TestServiceValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *ai.AIConfig
		expectError bool
		errorType   string
	}{
		{
			name: "Valid service configuration",
			config: &ai.AIConfig{
				Provider: "claude",
				ProviderSettings: map[string]any{
					"api_key":  "sk-test123",
					"base_url": "https://api.anthropic.com",
				},
			},
			expectError: false,
		},
		{
			name: "Missing API key",
			config: &ai.AIConfig{
				Provider: "claude",
				ProviderSettings: map[string]any{
					"base_url": "https://api.anthropic.com",
				},
			},
			expectError: true,
			errorType:   ai.ErrorTypeAuth,
		},
		{
			name: "Empty API key",
			config: &ai.AIConfig{
				Provider: "claude",
				ProviderSettings: map[string]any{
					"api_key":  "",
					"base_url": "https://api.anthropic.com",
				},
			},
			expectError: true,
			errorType:   ai.ErrorTypeAuth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewClaudeService(tt.config)
			if err != nil {
				t.Fatalf("Failed to create service: %v", err)
			}

			err = service.Validate()

			if tt.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}

			if tt.expectError && err != nil {
				if aiErr, ok := err.(*ai.AIError); ok {
					if aiErr.Type != tt.errorType {
						t.Errorf("Expected error type '%s', got '%s'", tt.errorType, aiErr.Type)
					}
					if aiErr.Provider != "claude" {
						t.Errorf("Expected provider 'claude', got '%s'", aiErr.Provider)
					}
				} else {
					t.Error("Expected AIError type")
				}
			}
		})
	}
}