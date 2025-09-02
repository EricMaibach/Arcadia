# Database and Scheduler Refactoring

This document describes the database and scheduler refactoring that was implemented to enable proper dependency injection and testing with mocks.

## What Changed

### Database Layer
- **Added interfaces**: `Database` interface for generic database operations
- **Created SQLiteDatabase**: Implementation of Database interface with constructor
- **Created DatabaseManager**: Manages both app and system database connections
- **Added mock implementations**: `MockSystemDB` for testing schedules
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

## Testing

Tests use a combination of mocks for schedules and in-memory SQLite databases:

```go
func TestYourFunction(t *testing.T) {
    // For schedule testing - use mock
    mockDB := services.NewMockSystemDB()
    scheduler := services.NewScheduler(mockDB)
    
    // For database functionality testing - use in-memory SQLite
    db, err := services.NewSQLiteDatabase(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()
    
    // Test your function
    schedule, err := scheduler.CreateSchedule(req)
    // ... assertions
}
```

## Benefits

1. **Better Testing**: Tests use in-memory databases and mocks for isolation
2. **Improved Performance**: In-memory database tests run very fast
3. **Interface-based Design**: Generic Database interface supports multiple implementations
4. **Thread Safety**: Proper mutex usage prevents race conditions
5. **Backward Compatibility**: Existing code continues to work
6. **Separation of Concerns**: Clean architecture with proper responsibility boundaries

## Migration Guide

If you want to migrate existing code:

1. Replace `services.InitDatabases()` with `services.InitDatabasesWithManager()`
2. Replace global scheduler functions with instance methods
3. Update tests to use in-memory SQLite databases instead of mocks where appropriate
4. Ensure proper cleanup of database connections

The old functions will continue to work but are now deprecated in favor of the new dependency injection approach.