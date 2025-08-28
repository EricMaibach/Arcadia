# Arcadia MCP Server

A Model Context Protocol (MCP) server that provides access to the Arcadia App Engine for developing and running WASM applications.

## Available Tools

### 1. `submit_app_trait` (RECOMMENDED)
Submit a Rust trait implementation to create a new WASM application. This is the simplified approach that only requires implementing the `ArcadiaApp` trait.

### 2. `submit_app_source` (LEGACY)
Submit complete Rust source files to create a WASM application. This requires handling all WASM boilerplate manually.

### 3. `list_apps`
List all registered applications in the Arcadia App Engine.

### 4. `run_tool`
Execute a tool from a registered application.

## Installation

1. Install Python dependencies:
```bash
pip install mcp httpx
```

2. Make sure the Arcadia App Engine is running (default: http://localhost:8080)

## Usage

Run the MCP server:
```bash
python main.py --app-engine-url http://localhost:8080
```

The server communicates via stdin/stdout using the MCP protocol.

## Quick Start Example

### Create Your First App with the Trait Approach

Use the `submit_app_trait` tool with a simple trait implementation:

```rust
use serde_json::json;

struct HelloWorldApp;

impl HelloWorldApp {
    fn new() -> Self {
        Self
    }
}

impl ArcadiaApp for HelloWorldApp {
    fn handle_tool(&mut self, tool_name: &str, data: Option<serde_json::Value>, _db: &DatabaseConnection) -> Result<serde_json::Value, String> {
        match tool_name {
            "hello" => {
                let name = data
                    .and_then(|d| d.get("name"))
                    .and_then(|n| n.as_str())
                    .unwrap_or("World");
                Ok(json!({ "message": format!("Hello, {}!", name) }))
            },
            _ => Err(format!("Unknown tool: {}", tool_name))
        }
    }
    
    fn get_available_tools(&self) -> Vec<&'static str> {
        vec!["hello"]
    }
}
```

### Run Your App

```json
{
  "app_id": "hello-app",
  "tool_name": "hello",
  "input_data": {"name": "Claude"}
}
```

## ArcadiaApp Trait Reference

The `ArcadiaApp` trait provides a clean interface for app development:

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

### Database Access

The `DatabaseConnection` provides these methods:

- **`query(&self, sql: &str) -> Result<Vec<serde_json::Value>, String>`**  
  Execute SELECT queries and return results as JSON values.

- **`execute(&self, sql: &str) -> Result<i32, String>`**  
  Execute INSERT/UPDATE/DELETE statements and return affected row count.

- **`prepared_query(&self, sql: &str, params: &[serde_json::Value]) -> Result<Vec<serde_json::Value>, String>`**  
  Execute prepared statements with parameters.

## Benefits of the Trait Approach

- **Simplified Development**: No WASM boilerplate required
- **Automatic Memory Management**: No manual allocation/deallocation
- **Built-in Database Integration**: Database connection provided automatically
- **Type Safety**: Full Rust type safety with serde JSON serialization
- **Easy Testing**: Trait implementations can be unit tested independently
- **Consistent Interface**: All apps follow the same pattern

## Configuration

The server can be configured via:

- **Command line:** `--app-engine-url http://localhost:8080`
- **Environment variable:** `ARCADIA_APP_ENGINE_URL=http://localhost:8080`
- **Default:** `http://localhost:8080`

## Tool Parameters

### `submit_app_trait` Parameters
- `appId`: Unique identifier for the application
- `version`: Version of the application  
- `runtime`: Must be "wasm"
- `tools`: Array of tool names this application provides
- `traitImpl`: Rust code implementing the ArcadiaApp trait

### `run_tool` Parameters
- `app_id`: The ID of the application to run
- `tool_name`: The name of the tool to execute
- `input_data`: Input data to pass to the tool

### `list_apps` Parameters
None required.

### `submit_app_source` Parameters (Legacy)
- `appId`: Unique identifier for the application
- `version`: Version of the application
- `runtime`: Must be "wasm"
- `tools`: Array of tool names this application provides
- `sourceLanguage`: Must be "rust"
- `files`: Array of source files with "name" and "content" fields

## Error Handling

The MCP server provides detailed error information including:
- HTTP status codes and messages
- Compilation errors from the Rust toolchain
- Runtime errors from WASM execution
- Database errors with specific error codes