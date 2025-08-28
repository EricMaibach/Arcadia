# Arcadia App Engine Tests

This directory contains comprehensive unit tests for the Arcadia App Engine.

## Test Files

### `main_test.go`
Core unit tests for the application including:
- **HTTP Handler Tests**: Tests for all REST endpoints (`listAppsHandler`, `runToolHandler`, `submitAppSrcHandler`)
- **Utility Function Tests**: Tests for Rust project generation, file operations, and Cargo.toml parsing
- **Database Tests**: Tests for database initialization and operations
- **Registry Tests**: Tests for app registry loading/saving

### `json_test.go`
JSON serialization/deserialization tests:
- Tests for all data structures (`App`, `AppRequest`, `RunToolRequest`)
- Validation of JSON field names and tags
- Ensures proper camelCase formatting in JSON output

### `benchmark_test.go`
Performance benchmarks for critical operations:
- HTTP handler performance
- JSON marshaling/unmarshaling performance
- File parsing performance

## Running Tests

### Quick Test Run
```bash
go test
```

### Verbose Output
```bash
go test -v
```

### With Coverage
```bash
go test -cover
```

### Benchmarks
```bash
go test -bench=.
```

### Race Condition Detection
```bash
go test -race
```

### Complete Test Suite
```bash
./test.sh
```

## Test Coverage

Current test coverage: ~25% of statements

The tests cover the most critical paths:
- All HTTP endpoints
- Core utility functions
- Database operations
- JSON handling
- Registry management

## Test Structure

### Setup/Teardown Helpers
- `setupTestDB()`: Creates in-memory SQLite database for testing
- `setupTestRegistry()`: Populates test app registry
- `cleanupTestDB()`: Cleans up test database
- `cleanupTestRegistry()`: Resets registry to empty state

### Test Categories

1. **Unit Tests**: Test individual functions in isolation
2. **Integration Tests**: Test HTTP endpoints with full request/response cycle
3. **Benchmark Tests**: Performance testing for critical operations
4. **JSON Tests**: Serialization validation and field naming

## Test Data

Tests use temporary directories and in-memory databases to avoid affecting the actual application data. No real files or databases are modified during testing.

## Adding New Tests

When adding new functionality:

1. Add unit tests in `main_test.go`
2. Add JSON tests in `json_test.go` if new data structures are introduced
3. Add benchmarks in `benchmark_test.go` for performance-critical code
4. Follow existing naming conventions and structure

### Example Test Function
```go
func TestNewFeature(t *testing.T) {
    tests := []struct {
        name           string
        input          InputType
        expectedOutput ExpectedType
        expectError    bool
    }{
        {
            name:           "valid input",
            input:          validInput,
            expectedOutput: expectedOutput,
            expectError:    false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := functionUnderTest(tt.input)
            
            if tt.expectError && err == nil {
                t.Error("Expected error but got none")
            }
            if !tt.expectError && err != nil {
                t.Errorf("Unexpected error: %v", err)
            }
            if result != tt.expectedOutput {
                t.Errorf("Expected %v, got %v", tt.expectedOutput, result)
            }
        })
    }
}
```

## Continuous Integration

These tests are designed to be run in CI/CD pipelines. The `test.sh` script provides a comprehensive test suite suitable for automated builds.