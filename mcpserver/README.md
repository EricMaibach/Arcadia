# Arcadia MCP Server

This is a Model Context Protocol (MCP) server that exposes the REST APIs from the Arcadia App Engine as MCP tools.

## Features

The MCP server provides three main tools:

1. **list_apps** - List all registered applications in the App Engine
2. **run_tool** - Execute a tool from a registered application  
3. **submit_app_source** - Submit source code to compile and register a new application

## Installation

1. Install Python dependencies:
```bash
pip install -r requirements.txt
```

2. Make sure the Arcadia App Engine is running (default: http://localhost:8080)

## Usage

Run the MCP server:
```bash
python main.py
```

The server communicates via stdin/stdout using the MCP protocol.

## Tools

### list_apps
Lists all registered applications.

**Parameters:**
- `app_engine_url` (optional): URL of the App Engine server (defaults to http://localhost:8080)

### run_tool  
Executes a tool from a registered application.

**Parameters:**
- `app_id`: The ID of the application to run
- `tool_name`: The name of the tool to execute
- `input_data`: Input data to pass to the tool
- `app_engine_url` (optional): URL of the App Engine server

### submit_app_source
Submits source code to compile and register a new application.

**Parameters:**
- `app_id`: Unique identifier for the application
- `version`: Version of the application
- `runtime`: Runtime for the application (e.g., 'wasm')
- `tools`: List of tool names this application provides
- `source_language`: Source language (e.g., 'rust')
- `files`: Array of source files with name and content
- `app_engine_url` (optional): URL of the App Engine server

## Example Usage

The MCP server is designed to be used by MCP clients (like Claude Desktop) to interact with the Arcadia App Engine through natural language interfaces.