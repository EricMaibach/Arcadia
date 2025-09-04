package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"arcadia/services"
)

// ServerConfig represents server configuration
type ServerConfig struct {
	Port string `json:"port"`
	Host string `json:"host"`
}

// Config represents the application configuration
type Config struct {
	Claude services.ClaudeConfig `json:"claude"`
	Server ServerConfig          `json:"server"`
}

// Manager handles configuration loading and management
type Manager struct {
	config        Config
	claudeService *services.ClaudeService
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{}
}

// Load loads the configuration from file and initializes services
func (m *Manager) Load() error {
	configFile := "config.json"

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("config file %s not found. Please create it with your Claude API configuration", configFile)
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

	// Initialize Claude service
	m.claudeService = services.NewClaudeService(m.config.Claude)

	log.Printf("Configuration loaded successfully. Claude model: %s", m.config.Claude.Model)
	return nil
}

// applyDefaultsAndValidate applies default values and validates the configuration
func (m *Manager) applyDefaultsAndValidate() error {
	// Validate required Claude configuration
	if m.config.Claude.APIKey == "" || m.config.Claude.APIKey == "your-claude-api-key-here" {
		return fmt.Errorf("Claude API key not configured in config.json")
	}

	// Apply Claude defaults
	if m.config.Claude.BaseURL == "" {
		m.config.Claude.BaseURL = "https://api.anthropic.com"
	}

	if m.config.Claude.Model == "" {
		m.config.Claude.Model = "claude-3-5-sonnet-20241022"
	}

	if m.config.Claude.MaxTokens == 0 {
		m.config.Claude.MaxTokens = 4096
	}

	if m.config.Claude.TimeoutSeconds == 0 {
		m.config.Claude.TimeoutSeconds = 240
		log.Printf("[Config] Applied default timeout: %d seconds", m.config.Claude.TimeoutSeconds)
	} else {
		log.Printf("[Config] Using configured timeout: %d seconds", m.config.Claude.TimeoutSeconds)
	}

	// Enable MCP by default (now handled directly in Go)
	m.config.Claude.EnableMCP = true

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

// GetClaudeService returns the initialized Claude service
func (m *Manager) GetClaudeService() *services.ClaudeService {
	return m.claudeService
}

// GetServerPort returns the configured server port
func (m *Manager) GetServerPort() string {
	return m.config.Server.Port
}

// GetServerHost returns the configured server host
func (m *Manager) GetServerHost() string {
	return m.config.Server.Host
}

// SetupDependencyInjection sets up dependency injection for services
func (m *Manager) SetupDependencyInjection(
	registryAccess services.RegistryAccess,
	appRunner services.AppRunner,
	appCreator services.AppCreator,
) {
	services.SetRegistryAccess(registryAccess)
	services.SetAppRunner(appRunner)
	services.SetAppCreator(appCreator)
}
