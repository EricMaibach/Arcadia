package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"arcadia/pkg/logging"
)

// ServerConfig represents server configuration
type ServerConfig struct {
	Port string `json:"port"`
	Host string `json:"host"`
}

// Config represents the application configuration
type Config struct {
	Server ServerConfig `json:"server"`
}

// Manager handles configuration loading and management
type Manager struct {
	config Config
	logger logging.Logger
}

// NewManager creates a new configuration manager
func NewManager(logger logging.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// Load loads the configuration from file and initializes services
func (m *Manager) Load() error {
	configFile := "config.json"

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("config file %s not found. Please create it with your server configuration", configFile)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, &m.config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults and validate
	if err := m.applyDefaultsAndValidate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	ctx := context.Background()
	if m.logger != nil {
		m.logger.Info(ctx, "Configuration loaded successfully", "config_file", configFile, "port", m.config.Server.Port)
	}
	return nil
}

// applyDefaultsAndValidate applies default values and validates the configuration
func (m *Manager) applyDefaultsAndValidate() error {
	// Apply server defaults
	if m.config.Server.Port == "" {
		m.config.Server.Port = "8080"
	}

	return nil
}

// GetConfig returns the loaded configuration
func (m *Manager) GetConfig() Config {
	return m.config
}


// GetServerPort returns the configured server port
func (m *Manager) GetServerPort() string {
	return m.config.Server.Port
}

// GetServerHost returns the configured server host
func (m *Manager) GetServerHost() string {
	return m.config.Server.Host
}

