# Simplified Rust Trait App Development

This document shows how to use the simplified trait-based approach for developing Arcadia apps.

## Overview

Instead of providing complete Rust source files with WASM boilerplate, you now only need to implement the `ArcadiaApp` trait. The system automatically injects your implementation into wrapper code that handles all WASM interaction, memory management, and database connectivity.

## The ArcadiaApp Trait

```rust
pub trait ArcadiaApp {
    // Optional: Initialize your app with database access
    fn initialize(&mut self, db: &DatabaseConnection) -> Result<(), String> {
        Ok(()) // Default implementation
    }
    
    // Required: Handle tool requests
    fn handle_tool(&mut self, tool_name: &str, data: Option<serde_json::Value>, db: &DatabaseConnection) -> Result<serde_json::Value, String>;
    
    // Required: List available tools
    fn get_available_tools(&self) -> Vec<&'static str>;
}
```

## Example Implementation

Here's a simple counter app implementation:

```rust
use serde_json::json;

struct CounterApp {
    count: i64,
}

impl CounterApp {
    fn new() -> Self {
        Self { count: 0 }
    }
}

impl ArcadiaApp for CounterApp {
    fn initialize(&mut self, db: &DatabaseConnection) -> Result<(), String> {
        // Create counter table if it doesn't exist
        db.execute("CREATE TABLE IF NOT EXISTS counter_state (id INTEGER PRIMARY KEY, count INTEGER)")?;
        
        // Load existing count from database
        let rows = db.query("SELECT count FROM counter_state WHERE id = 1")?;
        if let Some(row) = rows.first() {
            self.count = row["count"].as_i64().unwrap_or(0);
        }
        
        Ok(())
    }
    
    fn handle_tool(&mut self, tool_name: &str, data: Option<serde_json::Value>, db: &DatabaseConnection) -> Result<serde_json::Value, String> {
        match tool_name {
            "increment" => {
                self.count += 1;
                
                // Save to database
                db.execute(&format!("INSERT OR REPLACE INTO counter_state (id, count) VALUES (1, {})", self.count))?;
                
                Ok(json!({ "count": self.count }))
            },
            "decrement" => {
                self.count -= 1;
                
                // Save to database
                db.execute(&format!("INSERT OR REPLACE INTO counter_state (id, count) VALUES (1, {})", self.count))?;
                
                Ok(json!({ "count": self.count }))
            },
            "get_count" => {
                Ok(json!({ "count": self.count }))
            },
            "reset" => {
                let reset_value = if let Some(data) = data {
                    data["value"].as_i64().unwrap_or(0)
                } else {
                    0
                };
                
                self.count = reset_value;
                db.execute(&format!("INSERT OR REPLACE INTO counter_state (id, count) VALUES (1, {})", self.count))?;
                
                Ok(json!({ "count": self.count }))
            },
            _ => Err(format!("Unknown tool: {}", tool_name))
        }
    }
    
    fn get_available_tools(&self) -> Vec<&'static str> {
        vec!["increment", "decrement", "get_count", "reset"]
    }
}
```

## Submitting Your App

Send a POST request to `/submit_app_src` with this JSON structure:

```json
{
    "appId": "counter-app",
    "version": "1.0.0",
    "runtime": "wasm",
    "tools": ["increment", "decrement", "get_count", "reset"],
    "traitImpl": "use serde_json::json;\n\nstruct CounterApp {\n    count: i64,\n}\n\nimpl CounterApp {\n    fn new() -> Self {\n        Self { count: 0 }\n    }\n}\n\nimpl ArcadiaApp for CounterApp {\n    fn initialize(&mut self, db: &DatabaseConnection) -> Result<(), String> {\n        db.execute(\"CREATE TABLE IF NOT EXISTS counter_state (id INTEGER PRIMARY KEY, count INTEGER)\")?;\n        \n        let rows = db.query(\"SELECT count FROM counter_state WHERE id = 1\")?;\n        if let Some(row) = rows.first() {\n            self.count = row[\"count\"].as_i64().unwrap_or(0);\n        }\n        \n        Ok(())\n    }\n    \n    fn handle_tool(&mut self, tool_name: &str, data: Option<serde_json::Value>, db: &DatabaseConnection) -> Result<serde_json::Value, String> {\n        match tool_name {\n            \"increment\" => {\n                self.count += 1;\n                db.execute(&format!(\"INSERT OR REPLACE INTO counter_state (id, count) VALUES (1, {})\", self.count))?;\n                Ok(json!({ \"count\": self.count }))\n            },\n            \"decrement\" => {\n                self.count -= 1;\n                db.execute(&format!(\"INSERT OR REPLACE INTO counter_state (id, count) VALUES (1, {})\", self.count))?;\n                Ok(json!({ \"count\": self.count }))\n            },\n            \"get_count\" => {\n                Ok(json!({ \"count\": self.count }))\n            },\n            \"reset\" => {\n                let reset_value = if let Some(data) = data {\n                    data[\"value\"].as_i64().unwrap_or(0)\n                } else {\n                    0\n                };\n                \n                self.count = reset_value;\n                db.execute(&format!(\"INSERT OR REPLACE INTO counter_state (id, count) VALUES (1, {})\", self.count))?;\n                \n                Ok(json!({ \"count\": self.count }))\n            },\n            _ => Err(format!(\"Unknown tool: {}\", tool_name))\n        }\n    }\n    \n    fn get_available_tools(&self) -> Vec<&'static str> {\n        vec![\"increment\", \"decrement\", \"get_count\", \"reset\"]\n    }\n}"
}
```

## Database Access

The `DatabaseConnection` provides three methods:

- `query(sql: &str) -> Result<Vec<serde_json::Value>, String>` - Execute SELECT queries
- `execute(sql: &str) -> Result<i32, String>` - Execute INSERT/UPDATE/DELETE statements  
- `prepared_query(sql: &str, params: &[serde_json::Value]) -> Result<Vec<serde_json::Value>, String>` - Execute prepared statements with parameters

## Benefits

1. **Simplified Development**: No need to handle WASM boilerplate, memory management, or FFI
2. **Automatic Database Integration**: Database connection is provided automatically
3. **Type Safety**: Full Rust type safety with serde JSON serialization
4. **Consistent Interface**: All apps follow the same trait pattern
5. **Easy Testing**: Trait implementations can be unit tested independently

## Migration from Old Format

If you have existing apps using the old file-based format, you can extract the core logic from your `run` function and implement it in the `handle_tool` method of the `ArcadiaApp` trait.