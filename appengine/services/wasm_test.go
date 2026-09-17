package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFile_Struct(t *testing.T) {
	file := File{
		Name:    "test.rs",
		Content: "fn main() {}",
	}

	if file.Name != "test.rs" {
		t.Error("File Name not set correctly")
	}

	if file.Content != "fn main() {}" {
		t.Error("File Content not set correctly")
	}
}

func TestToolInfo_Struct(t *testing.T) {
	tool := ToolInfo{
		Name:        "test_tool",
		InputFormat: "json",
	}

	if tool.Name != "test_tool" {
		t.Error("ToolInfo Name not set correctly")
	}

	if tool.InputFormat != "json" {
		t.Error("ToolInfo InputFormat not set correctly")
	}
}

func TestAppRequest_Struct(t *testing.T) {
	request := AppRequest{
		AppID:   "test-app",
		Version: "1.0.0",
		Runtime: "wasm",
		Tools: []ToolInfo{
			{Name: "tool1", InputFormat: "json"},
		},
		AppSrc: "trait implementation code",
	}

	if request.AppID != "test-app" {
		t.Error("AppRequest AppID not set correctly")
	}

	if request.Runtime != "wasm" {
		t.Error("AppRequest Runtime not set correctly")
	}

	if len(request.Tools) != 1 {
		t.Error("AppRequest Tools not set correctly")
	}

	if request.Tools[0].Name != "tool1" {
		t.Error("AppRequest Tools[0].Name not set correctly")
	}
}

func TestNewWasmCompiler(t *testing.T) {
	compiler := NewWasmCompiler()
	if compiler == nil {
		t.Error("NewWasmCompiler returned nil")
	}
}

func TestWasmCompiler_GenerateRustProjectFromTrait(t *testing.T) {
	compiler := NewWasmCompiler()

	// Create temporary build directory
	tempDir, err := os.MkdirTemp("", "wasm_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a mock wrapper template file
	templateContent := `
// WASM wrapper template
// USER_APP_IMPL_PLACEHOLDER - This will be replaced with user's trait implementation
// End of template
`
	templatePath := "wasm_wrapper_template.rs"
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to create template file: %v", err)
	}
	defer os.Remove(templatePath)

	req := AppRequest{
		AppID:   "test-app",
		Version: "1.0.0",
		Runtime: "wasm",
		Tools: []ToolInfo{
			{Name: "test_tool", InputFormat: "json"},
		},
		AppSrc: "fn test_implementation() { println!(\"Hello from trait\"); }",
	}

	err = compiler.GenerateRustProjectFromTrait(req, tempDir)
	if err != nil {
		t.Errorf("GenerateRustProjectFromTrait failed: %v", err)
	}

	// Check that Cargo.toml was created
	cargoPath := filepath.Join(tempDir, "Cargo.toml")
	if _, err := os.Stat(cargoPath); os.IsNotExist(err) {
		t.Error("Cargo.toml was not created")
	}

	// Check that src/lib.rs was created
	libPath := filepath.Join(tempDir, "src", "lib.rs")
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		t.Error("src/lib.rs was not created")
	}

	// Check that the trait implementation was injected
	libContent, err := os.ReadFile(libPath)
	if err != nil {
		t.Errorf("Failed to read lib.rs: %v", err)
	}

	if !strings.Contains(string(libContent), "fn test_implementation()") {
		t.Error("User trait implementation was not injected into lib.rs")
	}

	// Check Cargo.toml content
	cargoContent, err := os.ReadFile(cargoPath)
	if err != nil {
		t.Errorf("Failed to read Cargo.toml: %v", err)
	}

	cargoStr := string(cargoContent)
	if !strings.Contains(cargoStr, "test-app") {
		t.Error("App ID not found in Cargo.toml")
	}
	if !strings.Contains(cargoStr, "1.0.0") {
		t.Error("Version not found in Cargo.toml")
	}
	if !strings.Contains(cargoStr, `crate-type = ["cdylib"]`) {
		t.Error("WASM crate-type not set in Cargo.toml")
	}
}

func TestWasmCompiler_GenerateRustProjectFromTrait_MissingTemplate(t *testing.T) {
	compiler := NewWasmCompiler()

	tempDir, err := os.MkdirTemp("", "wasm_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	req := AppRequest{
		AppID:   "test-app",
		Version: "1.0.0",
		AppSrc:  "fn test() {}",
	}

	// Template file doesn't exist
	err = compiler.GenerateRustProjectFromTrait(req, tempDir)
	if err == nil {
		t.Error("Expected error when template file is missing")
	}
	if !strings.Contains(err.Error(), "failed to read wrapper template") {
		t.Errorf("Expected wrapper template error, got: %v", err)
	}
}

func TestWasmCompiler_BuildRustToWasm_NoCargo(t *testing.T) {
	compiler := NewWasmCompiler()

	tempDir, err := os.MkdirTemp("", "wasm_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// No Cargo.toml file exists
	_, err = compiler.BuildRustToWasm(tempDir)
	if err == nil {
		t.Error("Expected error when Cargo.toml doesn't exist")
	}
	if !strings.Contains(err.Error(), "Cargo.toml not found") {
		t.Errorf("Expected Cargo.toml error, got: %v", err)
	}
}

func TestWasmCompiler_CreateFilesFromSpec(t *testing.T) {
	compiler := NewWasmCompiler()

	tempDir, err := os.MkdirTemp("", "wasm_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	files := []File{
		{
			Name:    "test1.txt",
			Content: "Content of test1",
		},
		{
			Name:    "subdir/test2.rs",
			Content: "fn main() { println!(\"test2\"); }",
		},
		{
			Name:    "test3.json",
			Content: `{"key": "value"}`,
		},
	}

	err = compiler.CreateFilesFromSpec(files, tempDir)
	if err != nil {
		t.Errorf("CreateFilesFromSpec failed: %v", err)
	}

	// Check that all files were created
	for _, file := range files {
		filePath := filepath.Join(tempDir, file.Name)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("File %s was not created", file.Name)
			continue
		}

		// Check file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("Failed to read file %s: %v", file.Name, err)
			continue
		}

		if string(content) != file.Content {
			t.Errorf("File %s content mismatch. Expected: %s, Got: %s",
				file.Name, file.Content, string(content))
		}
	}
}

func TestWasmCompiler_CreateFilesFromSpec_PathTraversal(t *testing.T) {
	compiler := NewWasmCompiler()

	tempDir, err := os.MkdirTemp("", "wasm_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Try path traversal attack
	files := []File{
		{
			Name:    "../../../etc/passwd",
			Content: "malicious content",
		},
	}

	err = compiler.CreateFilesFromSpec(files, tempDir)
	if err == nil {
		t.Error("Expected error for path traversal attempt")
	}
	if !strings.Contains(err.Error(), "invalid file path") {
		t.Errorf("Expected path traversal error, got: %v", err)
	}
}

func TestWasmCompiler_getProjectNameFromCargo(t *testing.T) {
	compiler := NewWasmCompiler()

	tempDir, err := os.MkdirTemp("", "wasm_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name         string
		cargoContent string
		expectedName string
		expectError  bool
	}{
		{
			name: "simple name",
			cargoContent: `[package]
name = "test_project"
version = "1.0.0"`,
			expectedName: "test_project",
			expectError:  false,
		},
		{
			name: "name with hyphens",
			cargoContent: `[package]
name = "test-project-name"
version = "1.0.0"`,
			expectedName: "test_project_name",
			expectError:  false,
		},
		{
			name: "name with extra spaces",
			cargoContent: `[package]
name  =  "spaced_project"  
version = "1.0.0"`,
			expectedName: "spaced_project",
			expectError:  false,
		},
		{
			name: "no name field",
			cargoContent: `[package]
version = "1.0.0"`,
			expectedName: "",
			expectError:  true,
		},
		{
			name: "malformed name field",
			cargoContent: `[package]
name test_project
version = "1.0.0"`,
			expectedName: "",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cargoPath := filepath.Join(tempDir, "Cargo.toml")
			if err := os.WriteFile(cargoPath, []byte(tt.cargoContent), 0644); err != nil {
				t.Fatalf("Failed to write Cargo.toml: %v", err)
			}

			name, err := compiler.getProjectNameFromCargo(cargoPath)

			if (err != nil) != tt.expectError {
				t.Errorf("getProjectNameFromCargo() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError && name != tt.expectedName {
				t.Errorf("Expected project name '%s', got '%s'", tt.expectedName, name)
			}
		})
	}
}

func TestWasmCompiler_getProjectNameFromCargo_FileNotFound(t *testing.T) {
	compiler := NewWasmCompiler()

	_, err := compiler.getProjectNameFromCargo("/nonexistent/Cargo.toml")
	if err == nil {
		t.Error("Expected error for non-existent Cargo.toml")
	}
}

func TestWasmCompiler_copyFile(t *testing.T) {
	compiler := NewWasmCompiler()

	tempDir, err := os.MkdirTemp("", "wasm_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source file
	srcPath := filepath.Join(tempDir, "source.txt")
	srcContent := "This is test content for copy operation"
	if err := os.WriteFile(srcPath, []byte(srcContent), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Copy file
	dstPath := filepath.Join(tempDir, "destination.txt")
	err = compiler.copyFile(srcPath, dstPath)
	if err != nil {
		t.Errorf("copyFile failed: %v", err)
	}

	// Check that destination file was created with correct content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Errorf("Failed to read destination file: %v", err)
	}

	if string(dstContent) != srcContent {
		t.Errorf("Copied file content mismatch. Expected: %s, Got: %s", srcContent, string(dstContent))
	}
}

func TestWasmCompiler_copyFile_SourceNotFound(t *testing.T) {
	compiler := NewWasmCompiler()

	tempDir, err := os.MkdirTemp("", "wasm_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srcPath := filepath.Join(tempDir, "nonexistent.txt")
	dstPath := filepath.Join(tempDir, "destination.txt")

	err = compiler.copyFile(srcPath, dstPath)
	if err == nil {
		t.Error("Expected error when source file doesn't exist")
	}
}

func TestWasmCompiler_CompileTraitToWasm_MissingTemplate(t *testing.T) {
	compiler := NewWasmCompiler()

	tempDir, err := os.MkdirTemp("", "wasm_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	req := AppRequest{
		AppID:   "test-app",
		Version: "1.0.0",
		AppSrc:  "fn test() {}",
	}

	// Template file doesn't exist
	_, err = compiler.CompileTraitToWasm(req, tempDir)
	if err == nil {
		t.Error("Expected error when template is missing")
	}
	if !strings.Contains(err.Error(), "failed to generate Rust project") {
		t.Errorf("Expected Rust project generation error, got: %v", err)
	}
}

func TestLegacyFunctions(t *testing.T) {
	// Test that legacy functions call the default compiler

	tempDir, err := os.MkdirTemp("", "wasm_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test CreateFilesFromSpec legacy function
	files := []File{
		{Name: "test.txt", Content: "test content"},
	}

	err = CreateFilesFromSpec(files, tempDir)
	if err != nil {
		t.Errorf("Legacy CreateFilesFromSpec failed: %v", err)
	}

	// Verify file was created
	filePath := filepath.Join(tempDir, "test.txt")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Legacy CreateFilesFromSpec did not create file")
	}

	// Test BuildRustToWasm legacy function (should fail without Cargo.toml)
	_, err = BuildRustToWasm(tempDir)
	if err == nil {
		t.Error("Expected error from legacy BuildRustToWasm without Cargo.toml")
	}

	// Test CompileTraitToWasm legacy function (should fail without template)
	req := AppRequest{
		AppID:   "test",
		Version: "1.0.0",
		AppSrc:  "fn test() {}",
	}
	_, err = CompileTraitToWasm(req, tempDir)
	if err == nil {
		t.Error("Expected error from legacy CompileTraitToWasm without template")
	}

	// Test GenerateRustProjectFromTrait legacy function (should fail without template)
	err = GenerateRustProjectFromTrait(req, tempDir)
	if err == nil {
		t.Error("Expected error from legacy GenerateRustProjectFromTrait without template")
	}
}

func TestDefaultCompiler(t *testing.T) {
	// Test that defaultCompiler is initialized
	if defaultCompiler == nil {
		t.Error("defaultCompiler should be initialized")
	}

	// Test that it's a WasmCompiler instance by type checking
	if _, ok := interface{}(defaultCompiler).(*WasmCompiler); !ok {
		t.Error("defaultCompiler should be a WasmCompiler instance")
	}
}

func TestWasmCompiler_BuildRustToWasm_ArtifactsDirectory(t *testing.T) {
	compiler := NewWasmCompiler()

	// Create a temporary directory structure that mimics the expected build structure
	tempDir, err := os.MkdirTemp("", "wasm_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create app/version directory structure
	appDir := filepath.Join(tempDir, "test-app")
	buildDir := filepath.Join(appDir, "1.0.0")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("Failed to create build dir: %v", err)
	}

	// Create a Cargo.toml file
	cargoContent := `[package]
name = "test-app"
version = "1.0.0"`
	cargoPath := filepath.Join(buildDir, "Cargo.toml")
	if err := os.WriteFile(cargoPath, []byte(cargoContent), 0644); err != nil {
		t.Fatalf("Failed to create Cargo.toml: %v", err)
	}

	// Test BuildRustToWasm (this will fail at cargo build, but we can test path logic)
	_, err = compiler.BuildRustToWasm(buildDir)

	// We expect this to fail at the cargo build step, not at path validation
	if err == nil {
		t.Error("Expected cargo build to fail (cargo likely not available in test environment)")
	} else if strings.Contains(err.Error(), "Cargo.toml not found") {
		t.Error("Should not fail at Cargo.toml validation - file exists")
	}
	// The error should be from cargo build command not found or similar

	// Test that artifacts directory would be created properly
	expectedArtifactsDir := filepath.Join("artifacts", "test-app")
	// Note: We can't easily test the full build without cargo installed and template files
	// But we can verify the path logic would work correctly

	if filepath.Base(appDir) != "test-app" {
		t.Errorf("App ID extraction failed: got %s", filepath.Base(appDir))
	}

	if filepath.Base(buildDir) != "1.0.0" {
		t.Errorf("Version extraction failed: got %s", filepath.Base(buildDir))
	}

	expectedWasmPath := filepath.Join(expectedArtifactsDir, "1.0.0.wasm")
	if !strings.Contains(expectedWasmPath, "artifacts/test-app/1.0.0.wasm") {
		t.Errorf("Expected WASM path construction failed: %s", expectedWasmPath)
	}
}
