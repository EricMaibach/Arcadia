#!/usr/bin/env python3
"""
MCP Server that exposes the Arcadia App Engine REST APIs.
"""

import asyncio
import logging
import os
from typing import Any, Dict, List
import httpx
from mcp.server.models import InitializationOptions
import mcp.types as types
from mcp.server import NotificationOptions, Server
import mcp.server.stdio
import argparse


# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("arcadia-mcp-server")

# Configuration
class Config:
    def __init__(self):
        # Try environment variable first, then command line args, then default
        self.app_engine_url = os.getenv("ARCADIA_APP_ENGINE_URL", "http://localhost:8080")
    
    def update_from_args(self, args):
        if args.app_engine_url:
            self.app_engine_url = args.app_engine_url

config = Config()
server = Server("arcadia-app-engine")


@server.list_tools()
async def handle_list_tools() -> List[types.Tool]:
    """
    List available tools for interacting with the Arcadia App Engine.
    """
    return [
        types.Tool(
            name="list_apps",
            title="List Apps",
            description="List all registered applications in the Arcadia App Engine",
            inputSchema={
                "type": "object",
                "properties": {}
            }
        ),
        types.Tool(
            name="run_app", 
            title="Run App",
            description="Execute a tool from a registered application",
            inputSchema={
                "type": "object",
                "properties": {
                    "app_id": {
                        "type": "string",
                        "description": "The ID of the application to run"
                    },
                    "tool_name": {
                        "type": "string", 
                        "description": "The name of the tool to execute"
                    },
                    "input_data": {
                        "type": "object",
                        "description": "Input data to pass to the tool"
                    }
                },
                "required": ["app_id", "tool_name", "input_data"]
            }
        ),
        types.Tool(
            name="create_app",
            title="Create App",
            description="""Submit a Rust trait implementation to compile and register a new WASM application.

Implement the ArcadiaApp trait and the system automatically handles all WASM boilerplate, memory management, and database integration.

REQUIRED JSON FORMAT:
```json
{
    "appId": "my-counter-app",
    "version": "1.0.0", 
    "runtime": "wasm",
    "tools": [
        {"name": "increment", "inputFormat": "{}"},
        {"name": "get_count", "inputFormat": "{\"user_id\": \"string\"}"}
    ],
    "appSrc": "/* Your Rust code here */"
}
```

REQUIRED METHODS:
- `fn initialize(&mut self, db: &DatabaseConnection) -> Result<(), String>` (optional)
- `fn handle_tool(&mut self, tool_name: &str, data: Option<serde_json::Value>, db: &DatabaseConnection) -> Result<serde_json::Value, String>` (required)
- `fn get_available_tools(&self) -> Vec<&'static str>` (required)

REQUIRED FACTORY FUNCTION:
- `pub fn create_app() -> Box<dyn ArcadiaApp + Send + Sync>` (required) - Factory function to create your app instance

EXAMPLE COUNTER APP:
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
        db.execute("CREATE TABLE IF NOT EXISTS counter_state (id INTEGER PRIMARY KEY, count INTEGER)")?;
        
        let rows = db.query("SELECT count FROM counter_state WHERE id = 1")?;
        if let Some(row) = rows.first() {
            self.count = row["count"].as_i64().unwrap_or(0);
        }
        
        Ok(())
    }
    
    fn handle_tool(&mut self, tool_name: &str, data: Option<serde_json::Value>, db: &DatabaseConnection) -> Result<serde_json::Value, String> {
        match tool_name {
            "increment" => {
                // Expects empty JSON object: {}
                self.count += 1;
                db.execute(&format!("INSERT OR REPLACE INTO counter_state (id, count) VALUES (1, {})", self.count))?;
                Ok(json!({ "count": self.count }))
            },
            "get_count" => {
                // Expects JSON with user_id: {"user_id": "string"}
                let user_id = data.and_then(|d| d.get("user_id"))
                    .and_then(|u| u.as_str())
                    .unwrap_or("anonymous");
                Ok(json!({ "count": self.count, "user": user_id }))
            },
            _ => Err(format!("Unknown tool: {}", tool_name))
        }
    }
    
    fn get_available_tools(&self) -> Vec<&'static str> {
        vec!["increment", "get_count"]
    }
}

pub fn create_app() -> Box<dyn ArcadiaApp + Send + Sync> {
    Box::new(CounterApp::new())
}
```

INPUT FORMAT SPECIFICATIONS:
The inputFormat field should describe the exact JSON structure each tool expects as input:

EXAMPLES:
- `"{}"` - Tool expects an empty JSON object (no parameters)
- `"{\"name\": \"string\"}"` - Tool expects JSON with a name field
- `"{\"amount\": \"number\", \"currency\": \"string\"}"` - Tool expects amount and currency fields
- `"{\"items\": [\"string\"], \"limit\": \"number?\"}"` - Tool expects array of items, optional limit
- `"{\"query\": \"string\", \"filters\": {\"status\": \"string?\"}}"` - Tool expects nested objects

FORMAT GUIDELINES:
- Use actual JSON structure examples, not just "json"
- Mark optional fields with "?" (e.g., "field_name?")
- Use descriptive type names: "string", "number", "boolean", "array", "object"
- Show nested structures when needed
- Keep it concise but complete

DATABASE ACCESS:
The DatabaseConnection provides these methods:
- `query(&self, sql: &str) -> Result<Vec<serde_json::Value>, String>` - Execute SELECT queries
- `execute(&self, sql: &str) -> Result<i32, String>` - Execute INSERT/UPDATE/DELETE statements
- `prepared_query(&self, sql: &str, params: &[serde_json::Value]) -> Result<Vec<serde_json::Value>, String>` - Execute prepared statements

BENEFITS:
- No WASM boilerplate required
- Automatic memory management
- Built-in database integration
- Type-safe JSON handling
- Easy testing and development
- Input format specification for better API documentation""",
            inputSchema={
                "type": "object",
                "properties": {
                    "appId": {
                        "type": "string",
                        "description": "Unique identifier for the application"
                    },
                    "version": {
                        "type": "string", 
                        "description": "Version of the application"
                    },
                    "runtime": {
                        "type": "string",
                        "description": "Runtime for the application (must be 'wasm')"
                    },
                    "tools": {
                        "type": "array",
                        "items": {
                            "type": "object",
                            "properties": {
                                "name": {
                                    "type": "string",
                                    "description": "The name of the tool"
                                },
                                "inputFormat": {
                                    "type": "string", 
                                    "description": "The expected JSON structure for this tool's input. Examples: '{}' for no input, '{\"name\": \"string\"}' for a name field, '{\"amount\": \"number\", \"currency\": \"string\"}' for multiple fields"
                                }
                            },
                            "required": ["name", "inputFormat"]
                        },
                        "description": "List of tools this application provides with their input format specifications"
                    },
                    "appSrc": {
                        "type": "string",
                        "description": "Rust code implementing the ArcadiaApp trait (including struct definition, impl blocks, and create_app factory function)"
                    }
                },
                "required": ["appId", "version", "runtime", "tools", "appSrc"]
            }
        )
    ]


@server.call_tool()
async def handle_call_tool(
    name: str, arguments: Dict[str, Any]
) -> List[types.TextContent | types.ImageContent | types.EmbeddedResource]:
    """
    Handle tool execution requests.
    """
    app_engine_url = config.app_engine_url
    
    try:
        async with httpx.AsyncClient(timeout=30.0) as client:
            if name == "list_apps":
                return await handle_list_apps(client, app_engine_url)
            elif name == "run_app":
                return await handle_run_tool(client, app_engine_url, arguments)
            elif name == "create_app":
                return await handle_submit_app_trait(client, app_engine_url, arguments)
            elif name == "submit_app_source":
                return await handle_submit_app_source(client, app_engine_url, arguments)
            else:
                raise ValueError(f"Unknown tool: {name}")
                
    except Exception as e:
        logger.error(f"Error executing tool {name}: {str(e)}")
        return [
            types.TextContent(
                type="text",
                text=f"Error: {str(e)}"
            )
        ]


async def handle_list_apps(
    client: httpx.AsyncClient, 
    app_engine_url: str
) -> List[types.TextContent]:
    """Handle the list_apps tool."""
    response = await client.get(f"{app_engine_url}/list_apps")
    response.raise_for_status()
    
    apps = response.json()
    
    if not apps:
        return [types.TextContent(type="text", text="No applications registered.")]
    
    # Format the response nicely
    result = "Registered Applications:\n\n"
    for app in apps:
        result += f"App ID: {app.get('appId', 'Unknown')}\n"
        result += f"Version: {app.get('version', 'Unknown')}\n"
        result += f"Runtime: {app.get('runtime', 'Unknown')}\n"
        
        # Format tools with input format information
        tools = app.get('tools', [])
        if tools:
            result += "Tools:\n"
            for tool in tools:
                if isinstance(tool, dict):
                    # New format: {name: "tool_name", inputFormat: "json"}
                    tool_name = tool.get('name', 'Unknown')
                    input_format = tool.get('inputFormat', 'Unknown')
                    result += f"  • {tool_name} (input: {input_format})\n"
                else:
                    # Legacy format: just tool name as string
                    result += f"  • {tool} (input: unknown)\n"
        else:
            result += "Tools: None\n"
        
        result += f"Source Language: {app.get('sourceLanguage', 'Unknown')}\n"
        result += f"Artifact: {app.get('artifactUri', 'Unknown')}\n"
        if app.get('files'):
            result += f"Source Files: {len(app['files'])} files\n"
        result += "-" * 50 + "\n"
    
    return [types.TextContent(type="text", text=result)]


async def handle_run_tool(
    client: httpx.AsyncClient, 
    app_engine_url: str, 
    arguments: Dict[str, Any]
) -> List[types.TextContent]:
    """Handle the run_tool tool."""
    payload = {
        "appId": arguments["app_id"],
        "toolName": arguments["tool_name"], 
        "input": arguments["input_data"]
    }
    
    response = await client.post(
        f"{app_engine_url}/run_tool",
        json=payload,
        headers={"Content-Type": "application/json"}
    )
    response.raise_for_status()
    
    result = response.json()
    
    # Format the response
    output_text = f"Tool Execution Result:\n"
    output_text += f"Status: {result.get('status', 'Unknown')}\n"
    output_text += f"Output: {result.get('output', 'No output')}\n"
    
    return [types.TextContent(type="text", text=output_text)]


async def handle_submit_app_trait(
    client: httpx.AsyncClient,
    app_engine_url: str, 
    arguments: Dict[str, Any]
) -> List[types.TextContent]:
    """Handle the submit_app_trait tool."""
    # Send the trait data directly to the new endpoint
    payload = arguments
    
    response = await client.post(
        f"{app_engine_url}/submit_app_src",
        json=payload,
        headers={"Content-Type": "application/json"}
    )
    
    # Check for HTTP errors and provide detailed error information
    if not response.is_success:
        error_text = f"Trait Application Submission Failed:\n"
        error_text += f"HTTP Status: {response.status_code} {response.reason_phrase}\n"
        
        try:
            # Try to parse error response as JSON
            error_data = response.json()
            if isinstance(error_data, dict):
                if error_data.get('error'):
                    error_text += f"Error: {error_data['error']}\n"
                if error_data.get('message'):
                    error_text += f"Message: {error_data['message']}\n"
                if error_data.get('details'):
                    error_text += f"Details: {error_data['details']}\n"
                # Include any other fields from the error response
                for key, value in error_data.items():
                    if key not in ['error', 'message', 'details']:
                        error_text += f"{key}: {value}\n"
            else:
                error_text += f"Error Response: {error_data}\n"
        except Exception:
            # If JSON parsing fails, include the raw response text
            error_text += f"Response Text: {response.text}\n"
        
        return [types.TextContent(type="text", text=error_text)]
    
    result = response.json()
    
    # Format the response
    output_text = f"Trait Application Submission Result:\n"
    output_text += f"Status: {result.get('status', 'Unknown')}\n"
    if result.get('wasmPath'):
        output_text += f"WASM Path: {result['wasmPath']}\n"
    output_text += f"\nYour trait implementation has been compiled and registered successfully!\n"
    output_text += f"You can now use run_tool with appId: {arguments.get('appId', 'unknown')}\n"
    
    return [types.TextContent(type="text", text=output_text)]


async def handle_submit_app_source(
    client: httpx.AsyncClient,
    app_engine_url: str, 
    arguments: Dict[str, Any]
) -> List[types.TextContent]:
    """Handle the submit_app_source tool."""
    # Validate that all files are raw text only
    if "files" in arguments:
        for file in arguments["files"]:
            if "encoding" in file:
                return [types.TextContent(
                    type="text", 
                    text="Error: Files must be submitted as raw text only. The 'encoding' field is not supported."
                )]
    
    # Send the app data directly
    payload = arguments
    
    response = await client.post(
        f"{app_engine_url}/submit_app_src",
        json=payload,
        headers={"Content-Type": "application/json"}
    )
    
    # Check for HTTP errors and provide detailed error information
    if not response.is_success:
        error_text = f"Application Submission Failed:\n"
        error_text += f"HTTP Status: {response.status_code} {response.reason_phrase}\n"
        
        try:
            # Try to parse error response as JSON
            error_data = response.json()
            if isinstance(error_data, dict):
                if error_data.get('error'):
                    error_text += f"Error: {error_data['error']}\n"
                if error_data.get('message'):
                    error_text += f"Message: {error_data['message']}\n"
                if error_data.get('details'):
                    error_text += f"Details: {error_data['details']}\n"
                # Include any other fields from the error response
                for key, value in error_data.items():
                    if key not in ['error', 'message', 'details']:
                        error_text += f"{key}: {value}\n"
            else:
                error_text += f"Error Response: {error_data}\n"
        except Exception:
            # If JSON parsing fails, include the raw response text
            error_text += f"Response Text: {response.text}\n"
        
        return [types.TextContent(type="text", text=error_text)]
    
    result = response.json()
    
    # Format the response
    output_text = f"Application Submission Result:\n"
    output_text += f"Status: {result.get('status', 'Unknown')}\n"
    if result.get('wasmPath'):
        output_text += f"WASM Path: {result['wasmPath']}\n"
    
    return [types.TextContent(type="text", text=output_text)]


async def main():
    # Parse command line arguments
    parser = argparse.ArgumentParser(description="Arcadia App Engine MCP Server")
    parser.add_argument(
        "--app-engine-url",
        type=str,
        help="URL of the App Engine server (default: http://localhost:8080 or ARCADIA_APP_ENGINE_URL env var)"
    )
    args = parser.parse_args()
    
    # Update config with command line arguments
    config.update_from_args(args)
    
    logger.info(f"Using App Engine URL: {config.app_engine_url}")
    
    # Run the server using stdin/stdout streams
    async with mcp.server.stdio.stdio_server() as (read_stream, write_stream):
        await server.run(
            read_stream,
            write_stream,
            InitializationOptions(
                server_name="arcadia-app-engine",
                server_version="0.1.0",
                capabilities=server.get_capabilities(
                    notification_options=NotificationOptions(),
                    experimental_capabilities={},
                ),
            ),
        )


if __name__ == "__main__":
    asyncio.run(main())