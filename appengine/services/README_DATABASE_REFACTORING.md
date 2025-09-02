# Database and Scheduler Refactoring

This document describes the database and scheduler refactoring that was implemented to enable proper dependency injection and testing with mocks.

## What Changed

### Database Layer
- **Added interfaces**: `SystemDB` and `AppDB` interfaces for dependency injection
- **Created DatabaseManager**: Struct that manages database connections and implements the interfaces
- **Added mock implementations**: `MockSystemDB` and `MockAppDB` for testing
- **Maintained backward compatibility**: All existing global functions still work

### Scheduler Layer
- **Refactored to use dependency injection**: `Scheduler` struct now accepts a `SystemDB` interface
- **Added default scheduler**: Maintains backward compatibility with existing code
- **Improved testability**: All database operations now use mocked interfaces

## New Usage Pattern (Recommended)

For new code, use the dependency injection approach:

```go
// Initialize database manager
dm, err := services.InitDatabasesWithManager()
if err != nil {
    log.Fatal("Failed to initialize database:", err)
}

// Create scheduler with database dependency
scheduler := services.NewScheduler(dm)
scheduler.SetExecuteAppTool(yourExecuteFunction)

// Start scheduler
if err := scheduler.Start(); err != nil {
    log.Fatal("Failed to start scheduler:", err)
}
```

## Old Usage Pattern (Still Supported)

Existing code continues to work without changes:

```go
// Initialize databases (global approach)
if err := services.InitDatabases(); err != nil {
    log.Fatal("Failed to initialize database:", err)
}

// Start scheduler (global approach)  
if err := services.StartScheduler(); err != nil {
    log.Fatal("Failed to start scheduler:", err)
}
```

## Testing with Mocks

Tests now use mocks instead of real databases:

```go
func TestYourFunction(t *testing.T) {
    // Create mock database
    mockDB := services.NewMockSystemDB()
    
    // Create scheduler with mock
    scheduler := services.NewScheduler(mockDB)
    
    // Test your function
    schedule, err := scheduler.CreateSchedule(req)
    // ... assertions
}
```

## Benefits

1. **Better Testing**: Tests are isolated and don't require real databases
2. **Improved Performance**: Mock tests run much faster
3. **Dependency Injection**: Makes the code more modular and testable
4. **Thread Safety**: Proper mutex usage prevents race conditions
5. **Backward Compatibility**: Existing code continues to work

## Migration Guide

If you want to migrate existing code:

1. Replace `services.InitDatabases()` with `services.InitDatabasesWithManager()`
2. Replace global scheduler functions with instance methods
3. Update tests to use mock implementations
4. Ensure proper cleanup of database connections

The old functions will continue to work but are now deprecated in favor of the new dependency injection approach.