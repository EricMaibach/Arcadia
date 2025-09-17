package services

import (
	"os"
	"testing"
)

func TestVectorStoreConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  VectorStoreConfig
		wantErr bool
	}{
		{
			name: "Valid QDRant config",
			config: VectorStoreConfig{
				QDRant: &QDRantConfig{
					Host:       "localhost",
					Port:       6333,
					Collection: "test",
					Timeout:    30,
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid QDRant config - empty host",
			config: VectorStoreConfig{
				QDRant: &QDRantConfig{
					Host:       "",
					Port:       6333,
					Collection: "test",
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid QDRant config - missing port",
			config: VectorStoreConfig{
				QDRant: &QDRantConfig{
					Host:       "localhost",
					Port:       0,
					Collection: "test",
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid QDRant config - missing collection",
			config: VectorStoreConfig{
				QDRant: &QDRantConfig{
					Host: "localhost",
					Port: 6333,
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid config - nil QDRant",
			config: VectorStoreConfig{
				QDRant: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("VectorStoreConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultConfigs(t *testing.T) {
	// Test default vector store config
	config := DefaultVectorStoreConfig()

	if config.QDRant == nil {
		t.Error("DefaultVectorStoreConfig() QDRant config is nil")
	}

	// Test default QDRant config
	qdrantConfig := DefaultQDRantConfig()

	if qdrantConfig.Host != "localhost" {
		t.Errorf("DefaultQDRantConfig() host = %v, want localhost", qdrantConfig.Host)
	}

	if qdrantConfig.Port != 6334 {
		t.Errorf("DefaultQDRantConfig() port = %v, want 6334", qdrantConfig.Port)
	}

	if qdrantConfig.Collection != "arcadia_vectors" {
		t.Errorf("DefaultQDRantConfig() collection = %v, want arcadia_vectors", qdrantConfig.Collection)
	}
}

func TestLoadVectorStoreConfigFromEnv(t *testing.T) {
	// Save original env vars
	originalHost := os.Getenv("QDRANT_HOST")
	originalPort := os.Getenv("QDRANT_PORT")
	originalCollection := os.Getenv("QDRANT_COLLECTION")
	originalAPIKey := os.Getenv("QDRANT_API_KEY")

	// Clean up after test
	defer func() {
		if originalHost != "" {
			os.Setenv("QDRANT_HOST", originalHost)
		} else {
			os.Unsetenv("QDRANT_HOST")
		}
		if originalPort != "" {
			os.Setenv("QDRANT_PORT", originalPort)
		} else {
			os.Unsetenv("QDRANT_PORT")
		}
		if originalCollection != "" {
			os.Setenv("QDRANT_COLLECTION", originalCollection)
		} else {
			os.Unsetenv("QDRANT_COLLECTION")
		}
		if originalAPIKey != "" {
			os.Setenv("QDRANT_API_KEY", originalAPIKey)
		} else {
			os.Unsetenv("QDRANT_API_KEY")
		}
	}()

	// Test with environment variables
	os.Setenv("QDRANT_HOST", "test-host")
	os.Setenv("QDRANT_PORT", "9999")
	os.Setenv("QDRANT_COLLECTION", "test-collection")
	os.Setenv("QDRANT_API_KEY", "test-key")

	config := LoadVectorStoreConfigFromEnv()

	if config.QDRant.Host != "test-host" {
		t.Errorf("LoadVectorStoreConfigFromEnv() host = %v, want test-host", config.QDRant.Host)
	}

	if config.QDRant.Port != 9999 {
		t.Errorf("LoadVectorStoreConfigFromEnv() port = %v, want 9999", config.QDRant.Port)
	}

	if config.QDRant.Collection != "test-collection" {
		t.Errorf("LoadVectorStoreConfigFromEnv() collection = %v, want test-collection", config.QDRant.Collection)
	}

	if config.QDRant.APIKey != "test-key" {
		t.Errorf("LoadVectorStoreConfigFromEnv() API key = %v, want test-key", config.QDRant.APIKey)
	}
}

func TestCreateVectorStore(t *testing.T) {
	// Test QDRant creation (will fail to connect, but should create the object)
	config := VectorStoreConfig{
		QDRant: &QDRantConfig{
			Host:       "localhost",
			Port:       6333,
			Collection: "test",
			Timeout:    30,
		},
	}

	store, err := CreateVectorStore(config)
	if err != nil {
		t.Errorf("CreateVectorStore() error = %v", err)
		return
	}

	if store == nil {
		t.Error("CreateVectorStore() returned nil store")
		return
	}

	// Test validation of invalid config
	invalidConfig := VectorStoreConfig{
		QDRant: &QDRantConfig{
			Host: "", // invalid
			Port: 6333,
		},
	}

	_, err = CreateVectorStore(invalidConfig)
	if err == nil {
		t.Error("CreateVectorStore() should fail with invalid config")
	}
}