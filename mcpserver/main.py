#!/usr/bin/env python3
"""
MCP Server that exposes the Arcadia App Engine REST APIs.
"""

import asyncio
import json
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
            description="List all registered applications in the Arcadia App Engine",
            inputSchema={
                "type": "object",
                "properties": {}
            }
        ),
        types.Tool(
            name="run_tool", 
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
            name="submit_app_source",
            description="""Submit source code to compile and register a new application. Currently only the Rust programming language is supported. The application will be compiled into a server-side WASM application that runs on a Golang runtime.

REQUIRED WASM INTERFACE:
Your Rust application MUST export these exact C functions for the WASM runtime to work:

1. `allocate(len: usize) -> *mut u8` - Allocate memory in WASM
2. `deallocate(ptr: *mut u8, len: usize)` - Deallocate memory in WASM  
3. `run(input_ptr: *const u8, input_len: usize) -> usize` - Main execution function that takes JSON input as pointer/length and returns result length
4. `get_result_ptr() -> *const u8` - Returns pointer to result data

RUST PROJECT STRUCTURE:
- Must include a `Cargo.toml` with `crate-type = ["cdylib"]` in [lib] section
- Must have a `src/lib.rs` file (not main.rs) containing the exported functions
- The exported functions must use `#[no_mangle]` and `pub extern "C"`

ENCODING REQUIREMENTS TO AVOID JSON ESCAPING ISSUES:
- STRONGLY RECOMMENDED: Base64 encode all file contents to prevent JSON escaping problems
- Set "encoding": "base64" for each file when using Base64 (this is the default)  
- This completely eliminates shell special characters (!@#$%^&*) causing JSON parsing errors
- Plain text is supported but may cause issues with special characters

EXAMPLES:
Base64 encoded (RECOMMENDED):
{
  "name": "src/lib.rs",
  "content": "dXNlIHN0ZDo6YWxsb2M6Ont7YWxsb2MsIGRlYWxsb2MsIExheW91dH07",
  "encoding": "base64"
}

Plain text (use with caution):
{
  "name": "Cargo.toml", 
  "content": "[package]\\nname = \\"hello\\"\\nversion = \\"0.1.0\\"",
  "encoding": "plain"
}

EXAMPLE MINIMAL IMPLEMENTATION (with safe JSON escaping):
```rust
use std::alloc::{alloc, dealloc, Layout};
use std::ptr;

static mut RESULT_PTR: *mut u8 = ptr::null_mut();
static mut RESULT_LEN: usize = 0;

#[no_mangle]
pub extern "C" fn allocate(len: usize) -> *mut u8 {
    let layout = Layout::from_size_align(len, 1).unwrap();
    unsafe { alloc(layout) }
}

#[no_mangle]
pub extern "C" fn deallocate(ptr: *mut u8, len: usize) {
    let layout = Layout::from_size_align(len, 1).unwrap();
    unsafe { dealloc(ptr, layout) }
}

#[no_mangle]
pub extern "C" fn run(input_ptr: *const u8, input_len: usize) -> usize {
    // Read input JSON from memory
    let input = unsafe {
        let slice = std::slice::from_raw_parts(input_ptr, input_len);
        std::str::from_utf8_unchecked(slice)
    };
    
    // Process input and create output
    let output = format!("Processed: {}", input);
    let output_bytes = output.into_bytes();
    let output_len = output_bytes.len();
    
    // Store result in global memory - using is_null() check instead of !
    unsafe {
        if RESULT_PTR.is_null() == false {
            deallocate(RESULT_PTR, RESULT_LEN);
        }
        RESULT_PTR = allocate(output_len);
        RESULT_LEN = output_len;
        ptr::copy_nonoverlapping(output_bytes.as_ptr(), RESULT_PTR, output_len);
        output_len
    }
}

#[no_mangle]
pub extern "C" fn get_result_ptr() -> *const u8 {
    unsafe { RESULT_PTR }
}
```

The application will be compiled using `cargo build --target wasm32-unknown-unknown --release` and registered for tool execution.""", 
            inputSchema={
                "type": "object",
                "properties": {
                    "spec": {
                        "type": "object",
                        "description": "Application specification that matches the Go server's SubmitAppSrcRequest.Spec struct",
                        "properties": {
                            "appId": {
                                "type": "string",
                                "description": "Unique identifier for the application (camelCase, NOT app_id)"
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
                                "items": {"type": "string"},
                                "description": "List of tool names this application provides"
                            },
                            "sourceLanguage": {
                                "type": "string",
                                "description": "Source language - must be 'rust' (camelCase, NOT source_language)"
                            },
                            "files": {
                                "type": "array",
                                "items": {
                                    "type": "object",
                                    "properties": {
                                        "name": {"type": "string", "description": "File name with path"},
                                        "content": {"type": "string", "description": "File contents (Base64 encoded recommended to avoid JSON escaping issues)"},
                                        "encoding": {"type": "string", "description": "Content encoding format", "enum": ["base64", "plain"], "default": "base64"}
                                    },
                                    "required": ["name", "content"]
                                },
                                "description": "Source files for the application"
                            }
                        },
                        "required": ["appId", "version", "runtime", "tools", "sourceLanguage", "files"]
                    }
                },
                "required": ["spec"]
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
            elif name == "run_tool":
                return await handle_run_tool(client, app_engine_url, arguments)
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
        result += f"Tools: {', '.join(app.get('tools', []))}\n"
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


async def handle_submit_app_source(
    client: httpx.AsyncClient,
    app_engine_url: str, 
    arguments: Dict[str, Any]
) -> List[types.TextContent]:
    """Handle the submit_app_source tool."""
    # The arguments now contain the spec directly
    payload = {
        "spec": arguments["spec"]
    }
    
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