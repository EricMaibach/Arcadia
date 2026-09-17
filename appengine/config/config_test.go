package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcadia/pkg/logging"
)

func newTestLogger(t *testing.T) logging.Logger {
	t.Helper()
	l, err := logging.NewLogger(logging.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create test logger: %v", err)
	}
	return l
}

func TestNewManager(t *testing.T) {
	manager := NewManager(newTestLogger(t))

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManager_Load_ConfigFileNotExists(t *testing.T) {
	manager := NewManager(newTestLogger(t))

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

	manager := NewManager(newTestLogger(t))
	err = manager.Load()

	if err != nil {
		t.Errorf("Load failed with valid config: %v", err)
	}

	// Test that configuration was loaded properly
	config := manager.GetConfig()
	if config.Server.Port != "8080" {
		t.Error("Server port not loaded correctly")
	}
	if config.Server.Host != "localhost" {
		t.Error("Server host not loaded correctly")
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
	invalidJSON := `{"server": "invalid json structure"`
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

	manager := NewManager(newTestLogger(t))
	err = manager.Load()

	if err == nil {
		t.Error("Expected error with invalid JSON")
	}
}

func TestManager_applyDefaultsAndValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		wantErr   bool
		checkFunc func(*testing.T, *Manager)
	}{
		{
			name:    "empty config with defaults applied",
			config:  Config{},
			wantErr: false,
			checkFunc: func(t *testing.T, m *Manager) {
				config := m.GetConfig()
				if config.Server.Port != "8080" {
					t.Error("Server port default not applied")
				}
			},
		},
		{
			name: "custom server values preserved",
			config: Config{
				Server: ServerConfig{
					Port: "9000",
					Host: "0.0.0.0",
				},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, m *Manager) {
				config := m.GetConfig()
				if config.Server.Port != "9000" {
					t.Error("Custom server port not preserved")
				}
				if config.Server.Host != "0.0.0.0" {
					t.Error("Custom server host not preserved")
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
	manager := NewManager(newTestLogger(t))

	// Set a test config
	testConfig := Config{
		Server: ServerConfig{
			Port: "8080",
			Host: "localhost",
		},
	}
	manager.config = testConfig

	retrievedConfig := manager.GetConfig()

	if retrievedConfig.Server.Port != "8080" {
		t.Error("GetConfig returned incorrect server port")
	}

	if retrievedConfig.Server.Host != "localhost" {
		t.Error("GetConfig returned incorrect server host")
	}
}

func TestManager_GetServerPort(t *testing.T) {
	manager := NewManager(newTestLogger(t))
	manager.config.Server.Port = "9999"

	port := manager.GetServerPort()
	if port != "9999" {
		t.Errorf("Expected port '9999', got '%s'", port)
	}
}

func TestManager_GetServerHost(t *testing.T) {
	manager := NewManager(newTestLogger(t))
	manager.config.Server.Host = "test-host"

	host := manager.GetServerHost()
	if host != "test-host" {
		t.Errorf("Expected host 'test-host', got '%s'", host)
	}
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
