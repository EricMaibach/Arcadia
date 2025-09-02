package services

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// File represents a file structure
type File struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// ToolInfo represents tool information
type ToolInfo struct {
	Name        string `json:"name"`
	InputFormat string `json:"inputFormat"`
}

// AppRequest represents a request to create an application from Rust trait
type AppRequest struct {
	AppID   string     `json:"appId"`
	Version string     `json:"version"`
	Runtime string     `json:"runtime"`
	Tools   []ToolInfo `json:"tools"`
	AppSrc  string     `json:"appSrc"`
}

// WasmCompiler handles the compilation of Rust traits to WASM
type WasmCompiler struct{}

// NewWasmCompiler creates a new WASM compiler instance
func NewWasmCompiler() *WasmCompiler {
	return &WasmCompiler{}
}

// GenerateRustProjectFromTrait creates a Rust project from a trait implementation
func (wc *WasmCompiler) GenerateRustProjectFromTrait(req AppRequest, buildDir string) error {
	// Read the wrapper template
	templatePath := filepath.Join("wasm_wrapper_template.rs")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read wrapper template: %v", err)
	}

	// Inject the user's trait implementation into the template
	injectedContent := strings.Replace(string(templateContent), "// USER_APP_IMPL_PLACEHOLDER - This will be replaced with user's trait implementation", req.AppSrc, 1)

	// Create src directory
	srcDir := filepath.Join(buildDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return fmt.Errorf("failed to create src directory: %v", err)
	}

	// Write lib.rs with injected code
	libPath := filepath.Join(srcDir, "lib.rs")
	if err := os.WriteFile(libPath, []byte(injectedContent), 0644); err != nil {
		return fmt.Errorf("failed to write lib.rs: %v", err)
	}

	// Create Cargo.toml
	cargoToml := fmt.Sprintf(`[package]
name = "%s"
version = "%s"
edition = "2021"

[lib]
crate-type = ["cdylib"]

[dependencies]
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"

[profile.release]
opt-level = "s"
lto = true
`, req.AppID, req.Version)

	cargoPath := filepath.Join(buildDir, "Cargo.toml")
	if err := os.WriteFile(cargoPath, []byte(cargoToml), 0644); err != nil {
		return fmt.Errorf("failed to write Cargo.toml: %v", err)
	}

	return nil
}

// BuildRustToWasm compiles a Rust project to WASM
func (wc *WasmCompiler) BuildRustToWasm(sourceBuildDir string) (string, error) {
	// Check if Cargo.toml exists to confirm it's a Rust project
	cargoPath := filepath.Join(sourceBuildDir, "Cargo.toml")
	if _, err := os.Stat(cargoPath); os.IsNotExist(err) {
		return "", fmt.Errorf("Cargo.toml not found in %s", sourceBuildDir)
	}

	// Create artifacts directory structure to store the compiled WASM
	appID := filepath.Base(filepath.Dir(sourceBuildDir))
	version := filepath.Base(sourceBuildDir)
	artifactsDir := filepath.Join("artifacts", appID)
	wasmFilename := fmt.Sprintf("%s.wasm", version)
	outputWasmPath := filepath.Join(artifactsDir, wasmFilename)

	// Create artifacts directory if it doesn't exist
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create artifacts directory: %v", err)
	}

	// Build the Rust project to WASM
	// Using cargo build with wasm32-unknown-unknown target
	cmd := exec.Command("cargo", "build", "--target", "wasm32-unknown-unknown", "--release")
	cmd.Dir = sourceBuildDir

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cargo build failed: %v\nOutput: %s", err, string(output))
	}

	// Find the compiled WASM file in the target directory
	// The WASM file should be at target/wasm32-unknown-unknown/release/<project_name>.wasm
	targetDir := filepath.Join(sourceBuildDir, "target", "wasm32-unknown-unknown", "release")

	// Read Cargo.toml to get the project name
	projectName, err := wc.getProjectNameFromCargo(cargoPath)
	if err != nil {
		return "", fmt.Errorf("failed to get project name: %v", err)
	}

	compiledWasmPath := filepath.Join(targetDir, projectName+".wasm")

	// Check if the compiled WASM file exists
	if _, err := os.Stat(compiledWasmPath); os.IsNotExist(err) {
		return "", fmt.Errorf("compiled WASM file not found at %s", compiledWasmPath)
	}

	// Copy the compiled WASM to the artifacts directory
	if err := wc.copyFile(compiledWasmPath, outputWasmPath); err != nil {
		return "", fmt.Errorf("failed to copy WASM file: %v", err)
	}

	return outputWasmPath, nil
}

// CreateFilesFromSpec creates files from a specification
func (wc *WasmCompiler) CreateFilesFromSpec(files []File, destDir string) error {
	for _, file := range files {
		// Create the full file path
		destPath := filepath.Join(destDir, file.Name)

		// Ensure the file path is within the destination directory (security check)
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", file.Name)
		}

		// Create parent directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %v", destPath, err)
		}

		// Create destination file
		destFile, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %v", destPath, err)
		}
		defer destFile.Close()

		// Write file contents as raw text
		if _, err := destFile.WriteString(file.Content); err != nil {
			return fmt.Errorf("failed to write contents to file %s: %v", file.Name, err)
		}
	}

	return nil
}

// getProjectNameFromCargo extracts the project name from Cargo.toml
func (wc *WasmCompiler) getProjectNameFromCargo(cargoPath string) (string, error) {
	content, err := os.ReadFile(cargoPath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name") && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				name = strings.Trim(name, `"`)
				// Rust converts hyphens to underscores in binary names
				name = strings.ReplaceAll(name, "-", "_")
				return name, nil
			}
		}
	}
	return "", fmt.Errorf("project name not found in Cargo.toml")
}

// copyFile copies a file from src to dst
func (wc *WasmCompiler) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// CompileTraitToWasm compiles a Rust trait implementation to WASM
// This is a high-level function that orchestrates the entire compilation process
func (wc *WasmCompiler) CompileTraitToWasm(req AppRequest, buildDir string) (wasmPath string, err error) {
	// Step 1: Generate Rust project from trait
	if err := wc.GenerateRustProjectFromTrait(req, buildDir); err != nil {
		return "", fmt.Errorf("failed to generate Rust project: %v", err)
	}

	// Step 2: Build Rust to WASM
	wasmPath, err = wc.BuildRustToWasm(buildDir)
	if err != nil {
		return "", fmt.Errorf("failed to build Rust to WASM: %v", err)
	}

	return wasmPath, nil
}

// Global compiler instance for backward compatibility
var defaultCompiler = NewWasmCompiler()

// Legacy functions for backward compatibility

// GenerateRustProjectFromTrait generates a Rust project from trait (legacy)
func GenerateRustProjectFromTrait(req AppRequest, buildDir string) error {
	return defaultCompiler.GenerateRustProjectFromTrait(req, buildDir)
}

// BuildRustToWasm builds Rust to WASM (legacy)
func BuildRustToWasm(sourceBuildDir string) (string, error) {
	return defaultCompiler.BuildRustToWasm(sourceBuildDir)
}

// CreateFilesFromSpec creates files from spec (legacy)
func CreateFilesFromSpec(files []File, destDir string) error {
	return defaultCompiler.CreateFilesFromSpec(files, destDir)
}

// CompileTraitToWasm compiles trait to WASM (legacy)
func CompileTraitToWasm(req AppRequest, buildDir string) (string, error) {
	return defaultCompiler.CompileTraitToWasm(req, buildDir)
}