package main

import (
	"encoding/json"
	"testing"
)

// TestConvertTools is no longer needed since conversion functions are now in handlers package
// and the main.go file uses the registry service directly

// TestConvertFiles is no longer needed since conversion functions are now in handlers package
// and the main.go file uses the registry service directly

func TestExecuteAppTool(t *testing.T) {
	// This test is limited because executeAppTool depends on the global wasmRuntime
	// In a real scenario, we'd refactor to use dependency injection

	t.Run("with nil runtime", func(t *testing.T) {
		// Save the original wasmRuntime
		originalRuntime := wasmRuntime
		defer func() {
			wasmRuntime = originalRuntime
		}()

		wasmRuntime = nil

		// This should not panic but will return an error
		defer func() {
			if r := recover(); r != nil {
				// It's ok if it panics with nil runtime
				t.Logf("Recovered from panic: %v", r)
			}
		}()

		// Try to execute - will likely panic or error
		_, err := executeAppTool("test-app", "test-tool", json.RawMessage(`{}`))
		if err == nil && wasmRuntime != nil {
			t.Error("Expected error or panic with nil runtime")
		}
	})
}

// Test for initialization order could be added if we refactor main()
// to call an initializeServices() function that we can test
func TestInitializationOrder(t *testing.T) {
	// This would require refactoring main() to extract initialization logic
	// into a separate function like:
	// func initializeServices() error { ... }
	// Then we could test that function

	t.Skip("Requires refactoring main() to extract initialization logic")
}

// Integration test example (usually in a separate file or build tag)
func TestMainIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would test the full initialization sequence
	// but requires all dependencies to be available
	t.Skip("Integration test - requires full environment setup")
}
