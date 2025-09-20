package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"arcadia/modules/ai/interfaces"
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

// registryAccessImpl implements the interfaces.RegistryAccess interface
type registryAccessImpl struct {
	registry *Registry
}

// GetRegistry returns the registry as a map[string]any for compatibility
func (ra *registryAccessImpl) GetRegistry() map[string]any {
	apps := ra.registry.GetAllApps()
	result := make(map[string]any)
	for k, v := range apps {
		result[k] = v
	}
	return result
}

// GetRegistryMutex returns the registry mutex
func (ra *registryAccessImpl) GetRegistryMutex() *sync.RWMutex {
	return ra.registry.GetMutex()
}

// GetApp retrieves an app by ID
func (ra *registryAccessImpl) GetApp(appID string) (interface{}, bool) {
	return ra.registry.GetApp(appID)
}

// ListApps returns a list of all app IDs
func (ra *registryAccessImpl) ListApps() []string {
	apps := ra.registry.GetAllApps()
	result := make([]string, 0, len(apps))
	for k := range apps {
		result = append(result, k)
	}
	return result
}

// appRunnerImpl implements the interfaces.AppRunner interface
type appRunnerImpl struct {
	executeAppTool func(appID, toolName string, input json.RawMessage) (string, error)
	registry       *Registry
}

// ExecuteAppTool executes a tool from an app
func (ar *appRunnerImpl) ExecuteAppTool(appID, toolName string, input json.RawMessage) (string, error) {
	return ar.executeAppTool(appID, toolName, input)
}

// IsAppAvailable checks if an app is available
func (ar *appRunnerImpl) IsAppAvailable(appID string) bool {
	_, exists := ar.registry.GetApp(appID)
	return exists
}

// GetAppInfo returns information about an app
func (ar *appRunnerImpl) GetAppInfo(appID string) (interface{}, error) {
	app, exists := ar.registry.GetApp(appID)
	if !exists {
		return nil, fmt.Errorf("app not found: %s", appID)
	}
	return app, nil
}

// appCreatorImpl implements the interfaces.AppCreator interface
type appCreatorImpl struct {
	registryManager     *RegistryManager
	appCreationService  AppCreationServiceInterface
}

// CreateApp creates a new app using the centralized app creation service
func (ac *appCreatorImpl) CreateApp(appID, version, runtime string, tools []any, appSrc string, dependencies map[string]string) (string, error) {
	// Convert tools back to the expected format
	toolInfos := make([]ToolInfo, len(tools))
	for i, tool := range tools {
		if toolMap, ok := tool.(map[string]interface{}); ok {
			if name, ok := toolMap["name"].(string); ok {
				inputFormat := ""
				if format, ok := toolMap["inputFormat"].(string); ok {
					inputFormat = format
				}
				toolInfos[i] = ToolInfo{
					Name:        name,
					InputFormat: inputFormat,
				}
			}
		}
	}

	// Create standardized request
	req := AppCreationRequest{
		AppID:        appID,
		Version:      version,
		Runtime:      runtime,
		Tools:        toolInfos,
		AppSrc:       appSrc,
		Dependencies: dependencies,
	}

	// Use centralized service to create the app
	sessionID := fmt.Sprintf("claude_mcp_%d", time.Now().UnixNano())
	return ac.appCreationService.CreateApp(req, sessionID)
}

// UpdateApp updates an existing app
func (ac *appCreatorImpl) UpdateApp(appID string, version string, tools []any, appSrc string, dependencies map[string]string) (string, error) {
	// For now, implement as a create operation
	return ac.CreateApp(appID, version, "wasm", tools, appSrc, dependencies)
}

// DeleteApp deletes an app
func (ac *appCreatorImpl) DeleteApp(appID string) error {
	// Remove from registry
	registry := ac.registryManager.GetRegistry()
	registry.GetMutex().Lock()
	defer registry.GetMutex().Unlock()

	// Note: This is a simplified implementation
	// In a real implementation, we'd need access to the internal registry map
	return fmt.Errorf("app deletion not fully implemented")
}

// ValidateApp validates an app configuration
func (ac *appCreatorImpl) ValidateApp(appID, version, runtime string, tools []any, appSrc string) error {
	// Basic validation
	if appID == "" {
		return fmt.Errorf("appID cannot be empty")
	}
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}
	if runtime == "" {
		return fmt.Errorf("runtime cannot be empty")
	}
	if appSrc == "" {
		return fmt.Errorf("appSrc cannot be empty")
	}
	return nil
}

// Manager manages the registry and provides dependency injection implementations
type RegistryManager struct {
	registry     *Registry
	registryAccess interfaces.RegistryAccess
	appRunner      interfaces.AppRunner
	appCreator     interfaces.AppCreator
}

// NewRegistryManager creates a new registry manager
func NewRegistryManager(executeAppTool func(appID, toolName string, input json.RawMessage) (string, error)) *RegistryManager {
	registry := NewRegistry()
	
	rm := &RegistryManager{
		registry:       registry,
		registryAccess: &registryAccessImpl{registry: registry},
		appRunner:      &appRunnerImpl{executeAppTool: executeAppTool, registry: registry},
	}
	
	// Create app creation service with logging function
	logFunc := func(format string, args ...interface{}) {
		log.Printf(format, args...)
	}
	appCreationService := NewAppCreationService(rm, logFunc)
	
	// Initialize app creator with dependencies
	rm.appCreator = &appCreatorImpl{
		registryManager:    rm,
		appCreationService: appCreationService,
	}
	
	return rm
}

// GetRegistry returns the registry instance
func (rm *RegistryManager) GetRegistry() *Registry {
	return rm.registry
}

// GetRegistryAccess returns the RegistryAccess implementation
func (rm *RegistryManager) GetRegistryAccess() interfaces.RegistryAccess {
	return rm.registryAccess
}

// GetAppRunner returns the AppRunner implementation
func (rm *RegistryManager) GetAppRunner() interfaces.AppRunner {
	return rm.appRunner
}

// GetAppCreator returns the AppCreator implementation
func (rm *RegistryManager) GetAppCreator() interfaces.AppCreator {
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