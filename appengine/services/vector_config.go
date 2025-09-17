package services

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

// VectorStoreConfig defines simple configuration for QDRant vector store
type VectorStoreConfig struct {
	QDRant *QDRantConfig `json:"qdrant"`
}

// QDRantConfig defines configuration for QDRant vector store
type QDRantConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	APIKey     string `json:"api_key,omitempty"`
	Collection string `json:"collection"`
	UseHTTPS   bool   `json:"use_https"`
	Timeout    int    `json:"timeout"` // in seconds
}

// DefaultVectorStoreConfig returns default QDRant configuration
func DefaultVectorStoreConfig() VectorStoreConfig {
	return VectorStoreConfig{
		QDRant: DefaultQDRantConfig(),
	}
}

// DefaultQDRantConfig returns default QDRant configuration
func DefaultQDRantConfig() *QDRantConfig {
	return &QDRantConfig{
		Host:       "localhost",
		Port:       6334,
		Collection: "arcadia_vectors",
		UseHTTPS:   false,
		Timeout:    30,
	}
}

// Validate validates the vector store configuration
func (c *VectorStoreConfig) Validate() error {
	if c.QDRant == nil {
		return fmt.Errorf("QDRant configuration is required")
	}
	if c.QDRant.Host == "" {
		return fmt.Errorf("host is required for QDRant vector store")
	}
	if c.QDRant.Port <= 0 {
		return fmt.Errorf("valid port is required for QDRant vector store")
	}
	if c.QDRant.Collection == "" {
		return fmt.Errorf("collection name is required for QDRant vector store")
	}
	if c.QDRant.Timeout <= 0 {
		c.QDRant.Timeout = 30 // Default timeout
	}
	return nil
}

// CreateVectorStore creates a QDRant vector store based on the configuration
func CreateVectorStore(config VectorStoreConfig) (VectorStoreInterface, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid vector store configuration: %v", err)
	}

	log.Printf("[VectorStore] Creating QDRant vector store at %s:%d",
		config.QDRant.Host, config.QDRant.Port)
	return NewQDRantVectorStore(*config.QDRant)
}


// LoadVectorStoreConfigFromEnv loads configuration from environment variables
func LoadVectorStoreConfigFromEnv() *VectorStoreConfig {
	// Start with default QDRant configuration
	config := DefaultVectorStoreConfig()

	// Override with environment variables
	loadConfigFromEnv(&config)

	return &config
}


// loadConfigFromEnv loads configuration overrides from environment variables
func loadConfigFromEnv(config *VectorStoreConfig) {
	// QDRant configuration
	if host := os.Getenv("QDRANT_HOST"); host != "" {
		config.QDRant.Host = host
		log.Printf("[Config] Override QDRant host from env: %s", host)
	}

	if portStr := os.Getenv("QDRANT_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.QDRant.Port = port
			log.Printf("[Config] Override QDRant port from env: %d", port)
		}
	}

	if apiKey := os.Getenv("QDRANT_API_KEY"); apiKey != "" {
		config.QDRant.APIKey = apiKey
		log.Printf("[Config] Override QDRant API key from env: [REDACTED]")
	}

	if collection := os.Getenv("QDRANT_COLLECTION"); collection != "" {
		config.QDRant.Collection = collection
		log.Printf("[Config] Override QDRant collection from env: %s", collection)
	}
}

