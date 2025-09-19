package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AppCreationRequest represents a standardized app creation request
type AppCreationRequest struct {
	AppID        string            `json:"appId"`
	Version      string            `json:"version"`
	Runtime      string            `json:"runtime"`
	Tools        []ToolInfo        `json:"tools"`
	AppSrc       string            `json:"appSrc"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

// AppCreationServiceInterface defines the interface for app creation services
type AppCreationServiceInterface interface {
	CreateApp(req AppCreationRequest, sessionID string) (string, error)
}

// AppCreationService handles the core logic for creating apps
type AppCreationService struct {
	registryManager *RegistryManager
	logFunc         func(format string, args ...interface{})
}

// NewAppCreationService creates a new app creation service
func NewAppCreationService(registryManager *RegistryManager, logFunc func(format string, args ...interface{})) *AppCreationService {
	return &AppCreationService{
		registryManager: registryManager,
		logFunc:         logFunc,
	}
}

// CreateApp handles the complete app creation process
func (acs *AppCreationService) CreateApp(req AppCreationRequest, sessionID string) (string, error) {
	// Step 1: Validate request
	if err := acs.validateRequest(req, sessionID); err != nil {
		return "", err
	}

	// Step 2: Check for existing WASM
	if err := acs.checkExistingApp(req, sessionID); err != nil {
		return "", err
	}

	// Step 3: Setup build directory
	buildDir, err := acs.setupBuildDirectory(req, sessionID)
	if err != nil {
		return "", err
	}

	// Step 4: Compile to WASM
	wasmPath, err := acs.compileToWasm(req, buildDir, sessionID)
	if err != nil {
		return "", err
	}

	// Step 5: Register app
	if err := acs.registerApp(req, wasmPath, sessionID); err != nil {
		return "", err
	}

	return fmt.Sprintf("App %s version %s created successfully at %s", req.AppID, req.Version, wasmPath), nil
}

// validateRequest validates the app creation request
func (acs *AppCreationService) validateRequest(req AppCreationRequest, sessionID string) error {
	acs.logFunc("[%s] Validating app creation request", sessionID)

	if strings.TrimSpace(req.AppSrc) == "" {
		acs.logFunc("[%s] ERROR: AppSrc field is empty", sessionID)
		return fmt.Errorf("app source code is required")
	}

	if req.AppID == "" {
		acs.logFunc("[%s] ERROR: AppID field is empty", sessionID)
		return fmt.Errorf("appId is required")
	}

	if req.Version == "" {
		acs.logFunc("[%s] ERROR: Version field is empty", sessionID)
		return fmt.Errorf("version is required")
	}

	if req.Runtime != "wasm" {
		acs.logFunc("[%s] ERROR: Runtime is '%s', expected 'wasm'", sessionID, req.Runtime)
		return fmt.Errorf("runtime must be 'wasm'")
	}

	if len(req.Tools) == 0 {
		acs.logFunc("[%s] ERROR: No tools provided", sessionID)
		return fmt.Errorf("at least one tool is required")
	}

	// Validate each tool
	for i, tool := range req.Tools {
		if strings.TrimSpace(tool.Name) == "" {
			acs.logFunc("[%s] ERROR: Tool %d has empty name", sessionID, i)
			return fmt.Errorf("tool %d: name is required", i)
		}
		if strings.TrimSpace(tool.InputFormat) == "" {
			acs.logFunc("[%s] ERROR: Tool %d (%s) has empty input format", sessionID, i, tool.Name)
			return fmt.Errorf("tool %d (%s): input format is required", i, tool.Name)
		}
	}

	acs.logFunc("[%s] Request validation passed", sessionID)
	return nil
}

// checkExistingApp checks if the app version already exists
func (acs *AppCreationService) checkExistingApp(req AppCreationRequest, sessionID string) error {
	acs.logFunc("[%s] Checking for existing app version", sessionID)

	artifactsDir := filepath.Join("artifacts", req.AppID)
	wasmFilename := fmt.Sprintf("%s.wasm", req.Version)
	finalWasmPath := filepath.Join(artifactsDir, wasmFilename)

	if _, err := os.Stat(finalWasmPath); err == nil {
		acs.logFunc("[%s] ERROR: App version already exists at %s", sessionID, finalWasmPath)
		return fmt.Errorf("app %s version %s already exists", req.AppID, req.Version)
	}

	acs.logFunc("[%s] No existing version found - proceeding", sessionID)
	return nil
}

// setupBuildDirectory creates and prepares the build directory
func (acs *AppCreationService) setupBuildDirectory(req AppCreationRequest, sessionID string) (string, error) {
	acs.logFunc("[%s] Setting up build directory", sessionID)

	buildDir := filepath.Join("build", req.AppID, req.Version)

	// Clear existing build directory if it exists
	if _, err := os.Stat(buildDir); err == nil {
		acs.logFunc("[%s] Removing existing build directory: %s", sessionID, buildDir)
		if err := os.RemoveAll(buildDir); err != nil {
			acs.logFunc("[%s] ERROR: Failed to remove build directory: %v", sessionID, err)
			return "", fmt.Errorf("failed to clear build directory: %w", err)
		}
	}

	// Create build directory
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		acs.logFunc("[%s] ERROR: Failed to create build directory: %v", sessionID, err)
		return "", fmt.Errorf("failed to create build directory: %w", err)
	}

	acs.logFunc("[%s] Build directory created: %s", sessionID, buildDir)
	return buildDir, nil
}

// compileToWasm compiles the app source to WASM
func (acs *AppCreationService) compileToWasm(req AppCreationRequest, buildDir, sessionID string) (string, error) {
	acs.logFunc("[%s] Compiling to WASM", sessionID)
	
	startTime := time.Now()
	wasmCompiler := NewWasmCompiler()
	
	// Convert to services.AppRequest format for compatibility with existing compiler
	appRequest := AppRequest{
		AppID:        req.AppID,
		Version:      req.Version,
		Runtime:      req.Runtime,
		Tools:        req.Tools,
		AppSrc:       req.AppSrc,
		Dependencies: req.Dependencies,
	}
	
	wasmPath, err := wasmCompiler.CompileTraitToWasm(appRequest, buildDir)
	if err != nil {
		acs.logFunc("[%s] ERROR: WASM compilation failed: %v", sessionID, err)
		return "", fmt.Errorf("WASM compilation failed: %w", err)
	}

	acs.logFunc("[%s] WASM compilation completed in %v: %s", sessionID, time.Since(startTime), wasmPath)
	return wasmPath, nil
}

// registerApp registers the app in the registry
func (acs *AppCreationService) registerApp(req AppCreationRequest, wasmPath, sessionID string) error {
	acs.logFunc("[%s] Registering app in registry", sessionID)

	registry := acs.registryManager.GetRegistry()
	app := &App{
		AppID:          req.AppID,
		Version:        req.Version,
		Runtime:        req.Runtime,
		Tools:          req.Tools,
		ArtifactURI:    wasmPath,
		SourceLanguage: "rust",
		Files: []File{
			{
				Name:    "src/lib.rs",
				Content: "Generated from trait implementation",
			},
		},
	}

	registry.RegisterApp(app)


	// Save registry to file for persistence
	if err := acs.registryManager.Save(); err != nil {
		acs.logFunc("[%s] WARNING: Failed to save registry: %v", sessionID, err)
		// Don't fail the request, app is already registered in memory
	}

	registrySize := registry.GetAppCount()
	acs.logFunc("[%s] App registered successfully. Registry has %d apps", sessionID, registrySize)
	return nil
}