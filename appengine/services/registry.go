package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
)

const registryFilePath = "app_registry.json"

// App represents an application in the registry
// Note: ToolInfo and File types are defined in wasm.go
type App struct {
	AppID          string     `json:"appId"`
	Version        string     `json:"version"`
	Runtime        string     `json:"runtime"`
	Tools          []ToolInfo `json:"tools"`
	ArtifactURI    string     `json:"artifactUri"`
	SourceLanguage string     `json:"sourceLanguage,omitempty"`
	Files          []File     `json:"files,omitempty"`
}

// Registry manages application registration and storage
type Registry struct {
	apps  map[string]*App
	mutex sync.RWMutex
}

// NewRegistry creates a new registry instance
func NewRegistry() *Registry {
	return &Registry{
		apps: make(map[string]*App),
	}
}

// Load loads the registry from file
func (r *Registry) Load() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if registry file exists
	if _, err := os.Stat(registryFilePath); os.IsNotExist(err) {
		// File doesn't exist, start with empty registry
		r.apps = make(map[string]*App)
		log.Printf("Registry file %s not found, starting with empty registry", registryFilePath)
		return nil
	}

	// Read the registry file
	data, err := os.ReadFile(registryFilePath)
	if err != nil {
		return fmt.Errorf("failed to read registry file: %v", err)
	}

	// Unmarshal JSON into registry
	if err := json.Unmarshal(data, &r.apps); err != nil {
		return fmt.Errorf("failed to parse registry JSON: %v", err)
	}

	log.Printf("Loaded %d apps from registry file", len(r.apps))
	return nil
}

// Save saves the registry to file
func (r *Registry) Save() error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Marshal registry to JSON
	data, err := json.MarshalIndent(r.apps, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry to JSON: %v", err)
	}

	// Write to file
	if err := os.WriteFile(registryFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry file: %v", err)
	}

	return nil
}

// GetApp retrieves an app by ID
func (r *Registry) GetApp(appID string) (*App, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	app, exists := r.apps[appID]
	return app, exists
}

// RegisterApp registers a new app in the registry
func (r *Registry) RegisterApp(app *App) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.apps[app.AppID] = app
}

// GetAllApps returns all registered apps
func (r *Registry) GetAllApps() map[string]*App {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	// Return a copy to prevent external modifications
	result := make(map[string]*App)
	for k, v := range r.apps {
		result[k] = v
	}
	return result
}

// GetAppCount returns the number of registered apps
func (r *Registry) GetAppCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.apps)
}

// GetMutex returns the registry mutex for external synchronization
func (r *Registry) GetMutex() *sync.RWMutex {
	return &r.mutex
}

// registryAccessImpl implements the services.RegistryAccess interface
type registryAccessImpl struct {
	registry *Registry
}

// GetRegistry returns the registry as a map[string]interface{} for compatibility
func (ra *registryAccessImpl) GetRegistry() map[string]interface{} {
	apps := ra.registry.GetAllApps()
	result := make(map[string]interface{})
	for k, v := range apps {
		result[k] = v
	}
	return result
}

// GetRegistryMutex returns the registry mutex
func (ra *registryAccessImpl) GetRegistryMutex() *sync.RWMutex {
	return ra.registry.GetMutex()
}

// appRunnerImpl implements the services.AppRunner interface
type appRunnerImpl struct {
	executeAppTool func(appID, toolName string, input json.RawMessage) (string, error)
}

// ExecuteAppTool executes a tool from an app
func (ar *appRunnerImpl) ExecuteAppTool(appID, toolName string, input json.RawMessage) (string, error) {
	return ar.executeAppTool(appID, toolName, input)
}

// appCreatorImpl implements the services.AppCreator interface (if it exists)
type appCreatorImpl struct{}

// CreateApp creates a new app (placeholder implementation)
func (ac *appCreatorImpl) CreateApp(appID, version, runtime string, tools []interface{}, appSrc string, dependencies map[string]string) (string, error) {
	// Convert tools back to the expected format
	toolInfos := make([]ToolInfo, len(tools))
	for i, tool := range tools {
		if toolMap, ok := tool.(map[string]interface{}); ok {
			toolInfos[i] = ToolInfo{
				Name:        toolMap["name"].(string),
				InputFormat: toolMap["input_format"].(string),
			}
		}
	}
	
	// Process the app creation (this would need to call the actual app creation logic)
	// For now, return a placeholder response
	depCount := 0
	if dependencies != nil {
		depCount = len(dependencies)
	}
	return fmt.Sprintf("App %s created successfully with %d tools and %d dependencies", appID, len(toolInfos), depCount), nil
}

// Manager manages the registry and provides dependency injection implementations
type RegistryManager struct {
	registry     *Registry
	registryAccess RegistryAccess
	appRunner      AppRunner
	appCreator     interface{}
}

// NewRegistryManager creates a new registry manager
func NewRegistryManager(executeAppTool func(appID, toolName string, input json.RawMessage) (string, error)) *RegistryManager {
	registry := NewRegistry()
	
	return &RegistryManager{
		registry:       registry,
		registryAccess: &registryAccessImpl{registry: registry},
		appRunner:      &appRunnerImpl{executeAppTool: executeAppTool},
		appCreator:     &appCreatorImpl{},
	}
}

// GetRegistry returns the registry instance
func (rm *RegistryManager) GetRegistry() *Registry {
	return rm.registry
}

// GetRegistryAccess returns the RegistryAccess implementation
func (rm *RegistryManager) GetRegistryAccess() RegistryAccess {
	return rm.registryAccess
}

// GetAppRunner returns the AppRunner implementation
func (rm *RegistryManager) GetAppRunner() AppRunner {
	return rm.appRunner
}

// GetAppCreator returns the AppCreator implementation
func (rm *RegistryManager) GetAppCreator() interface{} {
	return rm.appCreator
}

// Load loads the registry from file
func (rm *RegistryManager) Load() error {
	return rm.registry.Load()
}

// Save saves the registry to file
func (rm *RegistryManager) Save() error {
	return rm.registry.Save()
}

// Global registry manager instance (for backward compatibility)
var globalRegistryManager *RegistryManager

// InitializeRegistry initializes the global registry manager
func InitializeRegistry(executeAppTool func(appID, toolName string, input json.RawMessage) (string, error)) {
	globalRegistryManager = NewRegistryManager(executeAppTool)
}

// GetGlobalRegistry returns the global registry instance
func GetGlobalRegistry() *Registry {
	if globalRegistryManager == nil {
		panic("Registry not initialized. Call InitializeRegistry first.")
	}
	return globalRegistryManager.GetRegistry()
}

// GetGlobalRegistryManager returns the global registry manager
func GetGlobalRegistryManager() *RegistryManager {
	if globalRegistryManager == nil {
		panic("Registry not initialized. Call InitializeRegistry first.")
	}
	return globalRegistryManager
}