package services

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	AppID        string            `json:"appId"`
	Version      string            `json:"version"`
	Runtime      string            `json:"runtime"`
	Tools        []ToolInfo        `json:"tools"`
	AppSrc       string            `json:"appSrc"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

// WasmCompiler handles the compilation of Rust traits to WASM
type WasmCompiler struct{}

// NewWasmCompiler creates a new WASM compiler instance
func NewWasmCompiler() *WasmCompiler {
	return &WasmCompiler{}
}

// extractDependencies parses Rust code to detect used crates and returns a map of dependencies
func (wc *WasmCompiler) extractDependencies(rustCode string) map[string]string {
	// Start with base dependencies that are always needed
	dependencies := map[string]string{
		"serde":      `{ version = "1.0", features = ["derive"] }`,
		"serde_json": `"1.0"`,
	}

	// Common crate mappings with their Cargo.toml versions
	// This map covers the most commonly used Rust crates
	crateMap := map[string]string{
		// Date and time - NOTE: chrono is problematic in server-side WASM due to wasm-bindgen dependencies
		// Consider using std::time or atomic counters for server-side WASM instead
		"time": `"0.3"`,

		// Text processing
		"regex":       `"1.9"`,
		"lazy_static": `"1.4"`,
		"once_cell":   `"1.19"`,

		// Random and cryptography (server-side WASM compatible)
		"rand": `{ version = "0.8", default-features = false, features = ["small_rng"] }`,
		// Note: uuid with v4 requires randomness that may not work in server-side WASM
		// Consider using timestamp-based IDs instead
		"sha2":   `"0.10"`,
		"sha3":   `"0.10"`,
		"md5":    `"0.7"`,
		"hex":    `"0.4"`,
		"base64": `"0.21"`,
		"bcrypt": `"0.15"`,
		"argon2": `"0.5"`,

		// Async runtime (note: tokio might be too heavy for WASM)
		"tokio":     `{ version = "1.35", features = ["rt", "macros"] }`,
		"async_std": `"1.12"`,
		"futures":   `"0.3"`,

		// HTTP and networking (may not work in WASM context)
		"reqwest": `{ version = "0.11", features = ["json"] }`,
		"hyper":   `"0.14"`,
		"url":     `"2.5"`,

		// Serialization
		"bincode": `"1.3"`,
		"toml":    `"0.8"`,
		"csv":     `"1.3"`,
		"ron":     `"0.8"`,

		// Error handling
		"anyhow":    `"1.0"`,
		"thiserror": `"1.0"`,

		// Logging
		"log":        `"0.4"`,
		"env_logger": `"0.11"`,
		"tracing":    `"0.1"`,

		// Data structures
		"indexmap":  `"2.1"`,
		"hashbrown": `"0.14"`,
		"petgraph":  `"0.6"`,

		// Math and numerics
		"num":      `"0.4"`,
		"nalgebra": `"0.32"`,
		"ndarray":  `"0.15"`,

		// WebAssembly specific (server-side WASM, no browser dependencies)
		"getrandom": `{ version = "0.2", default-features = false }`,
	}

	// Parse use statements and extern crate declarations
	lines := strings.Split(rustCode, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for 'use' statements
		if strings.HasPrefix(line, "use ") {
			// Extract crate name (first part before ::)
			useContent := strings.TrimPrefix(line, "use ")
			useContent = strings.TrimSuffix(useContent, ";")

			// Handle various use patterns:
			// use chrono::DateTime;
			// use std::collections::HashMap;  (skip std)
			// use self::module;  (skip self)
			// use super::module;  (skip super)
			// use crate::module;  (skip crate - internal)

			parts := strings.Split(useContent, "::")
			if len(parts) > 0 {
				crateName := strings.TrimSpace(parts[0])

				// Skip standard library and internal references
				if crateName == "std" || crateName == "core" || crateName == "alloc" ||
					crateName == "self" || crateName == "super" || crateName == "crate" {
					continue
				}

				// Check if it's a known external crate
				if dep, exists := crateMap[crateName]; exists {
					dependencies[crateName] = dep
				}
			}
		}

		// Check for 'extern crate' statements (older Rust style)
		if strings.HasPrefix(line, "extern crate ") {
			crateName := strings.TrimPrefix(line, "extern crate ")
			crateName = strings.TrimSuffix(crateName, ";")
			crateName = strings.TrimSpace(crateName)

			if dep, exists := crateMap[crateName]; exists {
				dependencies[crateName] = dep
			}
		}
	}

	return dependencies
}

// formatDependencyVersion ensures a dependency version is properly formatted for Cargo.toml
func (wc *WasmCompiler) formatDependencyVersion(version string) string {
	version = strings.TrimSpace(version)

	// If it's already a complex dependency specification (starts with { or contains quotes), return as-is
	if strings.HasPrefix(version, "{") || strings.Contains(version, "\"") {
		return version
	}

	// If it looks like a simple version number (digits, dots, and common version chars), quote it
	if matched, _ := regexp.MatchString(`^[0-9]+(\.[0-9]+)*([a-zA-Z0-9\-\.]*)?$`, version); matched {
		return fmt.Sprintf(`"%s"`, version)
	}

	// Otherwise, return as-is (assume user knows what they're doing)
	return version
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

	// Extract dependencies from the Rust code
	codeDeps := wc.extractDependencies(req.AppSrc)

	// Merge with user-provided dependencies (user deps override auto-detected)
	allDeps := make(map[string]string)
	for name, version := range codeDeps {
		allDeps[name] = version
	}
	// User-provided dependencies override auto-detected ones
	if req.Dependencies != nil {
		for name, version := range req.Dependencies {
			// Ensure user-provided versions are properly formatted for Cargo.toml
			allDeps[name] = wc.formatDependencyVersion(version)
		}
	}

	// Build the dependencies section for Cargo.toml
	var depLines []string
	// Sort dependency names for consistent output
	depNames := make([]string, 0, len(allDeps))
	for name := range allDeps {
		depNames = append(depNames, name)
	}
	// Simple alphabetical sort
	for i := 0; i < len(depNames); i++ {
		for j := i + 1; j < len(depNames); j++ {
			if depNames[i] > depNames[j] {
				depNames[i], depNames[j] = depNames[j], depNames[i]
			}
		}
	}

	for _, name := range depNames {
		version := allDeps[name]
		depLines = append(depLines, fmt.Sprintf("%s = %s", name, version))
	}

	// Create Cargo.toml with dynamic dependencies
	cargoToml := fmt.Sprintf(`[package]
name = "%s"
version = "%s"
edition = "2021"

[lib]
crate-type = ["cdylib"]

[dependencies]
%s

[profile.release]
opt-level = "s"
lto = true
`, req.AppID, req.Version, strings.Join(depLines, "\n"))

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
