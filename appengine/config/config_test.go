package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"arcadia/services"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()
	
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManager_Load_ConfigFileNotExists(t *testing.T) {
	manager := NewManager()
	
	// Test loading when config file doesn't exist
	err := manager.Load()
	
	if err == nil {
		t.Error("Expected error when config file doesn't exist")
	}
	
	if !strings.Contains(err.Error(), "config file") {
		t.Errorf("Expected error about config file, got: %v", err)
	}
}

func TestManager_Load_ValidConfig(t *testing.T) {
	// Create a temporary valid config file
	validConfig := Config{
		Claude: services.ClaudeConfig{
			APIKey:         "test-api-key",
			BaseURL:        "https://api.anthropic.com",
			Model:          "claude-3-5-sonnet-20241022",
			MaxTokens:      4096,
			TimeoutSeconds: 30,
			EnableMCP:      true,
		},
		Server: ServerConfig{
			Port: "8080",
			Host: "localhost",
		},
	}
	
	tmpFile, err := ioutil.TempFile("", "config_test*.json")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	
	data, err := json.MarshalIndent(validConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	
	err = ioutil.WriteFile(tmpFile.Name(), data, 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	
	// Change working directory temporarily
	originalWd, _ := os.Getwd()
	tmpDir := filepath.Dir(tmpFile.Name())
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)
	
	// Rename temp file to config.json
	configPath := "config.json"
	os.Rename(tmpFile.Name(), configPath)
	defer os.Remove(configPath)
	
	manager := NewManager()
	err = manager.Load()
	
	if err != nil {
		t.Errorf("Load failed with valid config: %v", err)
	}
	
	// Test that Claude service was initialized
	claudeService := manager.GetClaudeService()
	if claudeService == nil {
		t.Error("Claude service not initialized")
	}
}

func TestManager_Load_InvalidJSON(t *testing.T) {
	// Create a temporary file with invalid JSON
	tmpFile, err := ioutil.TempFile("", "config_test*.json")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	
	// Write invalid JSON
	invalidJSON := `{"claude": "invalid json structure"`
	err = ioutil.WriteFile(tmpFile.Name(), []byte(invalidJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}
	
	// Change working directory temporarily
	originalWd, _ := os.Getwd()
	tmpDir := filepath.Dir(tmpFile.Name())
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)
	
	// Rename temp file to config.json
	configPath := "config.json"
	os.Rename(tmpFile.Name(), configPath)
	defer os.Remove(configPath)
	
	manager := NewManager()
	err = manager.Load()
	
	if err == nil {
		t.Error("Expected error with invalid JSON")
	}
}

func TestManager_applyDefaultsAndValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantErr     bool
		wantErrMsg  string
		checkFunc   func(*testing.T, *Manager)
	}{
		{
			name: "missing API key",
			config: Config{
				Claude: services.ClaudeConfig{
					APIKey: "",
				},
			},
			wantErr:    true,
			wantErrMsg: "Claude API key not configured",
		},
		{
			name: "placeholder API key",
			config: Config{
				Claude: services.ClaudeConfig{
					APIKey: "your-claude-api-key-here",
				},
			},
			wantErr:    true,
			wantErrMsg: "Claude API key not configured",
		},
		{
			name: "valid config with defaults applied",
			config: Config{
				Claude: services.ClaudeConfig{
					APIKey: "valid-api-key",
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, m *Manager) {
				config := m.GetConfig()
				if config.Claude.BaseURL != "https://api.anthropic.com" {
					t.Error("BaseURL default not applied")
				}
				if config.Claude.Model != "claude-3-5-sonnet-20241022" {
					t.Error("Model default not applied")
				}
				if config.Claude.MaxTokens != 4096 {
					t.Error("MaxTokens default not applied")
				}
				if config.Claude.TimeoutSeconds != 240 {
					t.Error("TimeoutSeconds default not applied")
				}
				if !config.Claude.EnableMCP {
					t.Error("EnableMCP should default to true")
				}
				if config.Server.Port != "8080" {
					t.Error("Server port default not applied")
				}
			},
		},
		{
			name: "custom values preserved",
			config: Config{
				Claude: services.ClaudeConfig{
					APIKey:         "valid-api-key",
					BaseURL:        "https://custom-api.com",
					Model:          "custom-model",
					MaxTokens:      8192,
					TimeoutSeconds: 60,
				},
				Server: ServerConfig{
					Port: "9000",
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, m *Manager) {
				config := m.GetConfig()
				if config.Claude.BaseURL != "https://custom-api.com" {
					t.Error("Custom BaseURL not preserved")
				}
				if config.Claude.Model != "custom-model" {
					t.Error("Custom Model not preserved")
				}
				if config.Claude.MaxTokens != 8192 {
					t.Error("Custom MaxTokens not preserved")
				}
				if config.Claude.TimeoutSeconds != 60 {
					t.Error("Custom TimeoutSeconds not preserved")
				}
				if config.Server.Port != "9000" {
					t.Error("Custom server port not preserved")
				}
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{config: tt.config}
			err := manager.applyDefaultsAndValidate()
			
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("Expected error containing '%s', got '%v'", tt.wantErrMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.checkFunc != nil {
					tt.checkFunc(t, manager)
				}
			}
		})
	}
}

func TestManager_GetConfig(t *testing.T) {
	manager := NewManager()
	
	// Set a test config
	testConfig := Config{
		Claude: services.ClaudeConfig{
			APIKey: "test-key",
			Model:  "test-model",
		},
		Server: ServerConfig{
			Port: "8080",
		},
	}
	manager.config = testConfig
	
	retrievedConfig := manager.GetConfig()
	
	if retrievedConfig.Claude.APIKey != "test-key" {
		t.Error("GetConfig returned incorrect Claude API key")
	}
	
	if retrievedConfig.Server.Port != "8080" {
		t.Error("GetConfig returned incorrect server port")
	}
}

func TestManager_GetClaudeService(t *testing.T) {
	manager := NewManager()
	
	// Test when Claude service is nil
	claudeService := manager.GetClaudeService()
	if claudeService != nil {
		t.Error("Expected nil Claude service before initialization")
	}
	
	// Mock a Claude service
	mockService := &services.ClaudeService{}
	manager.claudeService = mockService
	
	retrievedService := manager.GetClaudeService()
	if retrievedService != mockService {
		t.Error("GetClaudeService returned different service")
	}
}

func TestManager_GetServerPort(t *testing.T) {
	manager := NewManager()
	manager.config.Server.Port = "9999"
	
	port := manager.GetServerPort()
	if port != "9999" {
		t.Errorf("Expected port '9999', got '%s'", port)
	}
}

func TestManager_GetServerHost(t *testing.T) {
	manager := NewManager()
	manager.config.Server.Host = "test-host"
	
	host := manager.GetServerHost()
	if host != "test-host" {
		t.Errorf("Expected host 'test-host', got '%s'", host)
	}
}

func TestManager_SetupDependencyInjection(t *testing.T) {
	manager := NewManager()
	
	// Create mock implementations
	mockRegistryAccess := &MockRegistryAccess{}
	mockAppRunner := &MockAppRunner{}
	mockAppCreator := &MockAppCreator{}
	
	// This test ensures the method doesn't crash
	manager.SetupDependencyInjection(mockRegistryAccess, mockAppRunner, mockAppCreator)
}

// Mock implementations for testing
type MockRegistryAccess struct{}

func (m *MockRegistryAccess) GetRegistry() map[string]interface{} {
	return make(map[string]interface{})
}

func (m *MockRegistryAccess) GetRegistryMutex() *sync.RWMutex {
	return &sync.RWMutex{}
}

type MockAppRunner struct{}

func (m *MockAppRunner) ExecuteAppTool(appID, toolName string, input json.RawMessage) (string, error) {
	return "mock result", nil
}

type MockAppCreator struct{}

func (m *MockAppCreator) CreateApp(appID, version, runtime string, tools []interface{}, appSrc string, dependencies map[string]string) (string, error) {
	return "test app created", nil
}

func TestServerConfig(t *testing.T) {
	config := ServerConfig{
		Port: "8080",
		Host: "localhost",
	}
	
	if config.Port != "8080" {
		t.Error("ServerConfig Port not set correctly")
	}
	
	if config.Host != "localhost" {
		t.Error("ServerConfig Host not set correctly")
	}
}