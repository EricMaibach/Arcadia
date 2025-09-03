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
    "appId": "my-text-analyzer",
    "version": "1.0.0", 
    "runtime": "wasm",
    "tools": [
        {"name": "analyze_text", "inputFormat": "{\"text\": \"string\", \"user_id\": \"string?\"}"},
        {"name": "get_history", "inputFormat": "{\"user_id\": \"string\", \"limit\": \"number?\"}"},
        {"name": "generate_report", "inputFormat": "{\"limit\": \"number?\"}"}
    ],
    "appSrc": "/* Your Rust code here */",
    "dependencies": {
        "regex": "1.9"
    }
}
```

REQUIRED METHODS:
- `fn initialize(&mut self, db: &DatabaseConnection, claude: &ClaudeService) -> Result<(), String>` (optional)
- `fn handle_tool(&mut self, tool_name: &str, data: Option<serde_json::Value>, db: &DatabaseConnection, claude: &ClaudeService) -> Result<serde_json::Value, String>` (required)
- `fn get_available_tools(&self) -> Vec<&'static str>` (required)

REQUIRED FACTORY FUNCTION:
- `pub fn create_app() -> Box<dyn ArcadiaApp + Send + Sync>` (required) - Factory function to create your app instance

EXAMPLE TEXT ANALYZER APP WITH DEPENDENCIES:
```rust
use serde_json::json;
use regex::Regex;
use std::sync::atomic::{AtomicU64, Ordering};

static COUNTER: AtomicU64 = AtomicU64::new(1000);

struct TextAnalyzerApp {
    word_pattern: Regex,
    sentence_pattern: Regex,
}

impl TextAnalyzerApp {
    fn new() -> Self {
        Self { 
            word_pattern: Regex::new(r"\b\w+\b").unwrap(),
            sentence_pattern: Regex::new(r"[.!?]+").unwrap(),
        }
    }
}

impl ArcadiaApp for TextAnalyzerApp {
    fn initialize(&mut self, db: &DatabaseConnection, claude: &ClaudeService) -> Result<(), String> {
        // Create table for storing text analysis results
        db.execute("CREATE TABLE IF NOT EXISTS text_analyses (
            id TEXT PRIMARY KEY,
            user_id TEXT,
            text_content TEXT NOT NULL,
            word_count INTEGER,
            sentence_count INTEGER,
            char_count INTEGER,
            analysis_summary TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )")?;
        
        // Ask Claude to help set up the app
        let welcome = claude.ask("Welcome! This is a text analysis app. Can you give me a brief description of what text analysis is useful for?")?;
        
        // Store the welcome message as an initial analysis using counter-based ID
        let init_id = format!("init_{}", COUNTER.fetch_add(1, Ordering::SeqCst));
        db.execute(&format!(
            "INSERT INTO text_analyses (id, user_id, text_content, word_count, sentence_count, char_count, analysis_summary) 
             VALUES ('{}', 'system', 'App initialized', 2, 1, 14, '{}')",
            init_id, welcome.replace("'", "''")
        ))?;
        
        Ok(())
    }
    
    fn handle_tool(&mut self, tool_name: &str, data: Option<serde_json::Value>, db: &DatabaseConnection, claude: &ClaudeService) -> Result<serde_json::Value, String> {
        match tool_name {
            "analyze_text" => {
                let text = data
                    .as_ref()
                    .and_then(|d| d.get("text"))
                    .and_then(|t| t.as_str())
                    .ok_or("Text is required for analysis")?;
                
                let user_id = data
                    .as_ref()
                    .and_then(|d| d.get("user_id"))
                    .and_then(|u| u.as_str())
                    .unwrap_or("anonymous");
                
                // Use regex to count words and sentences
                let word_count = self.word_pattern.find_iter(text).count() as i32;
                let sentence_count = self.sentence_pattern.find_iter(text).count() as i32;
                let char_count = text.chars().count() as i32;
                
                // Ask Claude to analyze the text content and provide insights
                let claude_prompt = format!(
                    "Please analyze this text and provide insights about its style, tone, and content. 
                     Text: \"{}\"
                     Word count: {}, Sentence count: {}, Character count: {}",
                    text, word_count, sentence_count, char_count
                );
                
                let analysis_summary = claude.ask(&claude_prompt)?;
                
                // Generate unique ID using counter and store in database
                let analysis_id = format!("analysis_{}", COUNTER.fetch_add(1, Ordering::SeqCst));
                
                db.execute(&format!(
                    "INSERT INTO text_analyses (id, user_id, text_content, word_count, sentence_count, char_count, analysis_summary) 
                     VALUES ('{}', '{}', '{}', {}, {}, {}, '{}')",
                    analysis_id, 
                    user_id, 
                    text.replace("'", "''"),
                    word_count,
                    sentence_count, 
                    char_count,
                    analysis_summary.replace("'", "''")
                ))?;
                
                Ok(json!({
                    "id": analysis_id,
                    "user_id": user_id,
                    "word_count": word_count,
                    "sentence_count": sentence_count,
                    "character_count": char_count,
                    "analysis_summary": analysis_summary,
                    "created": "stored in database with CURRENT_TIMESTAMP"
                }))
            },
            "get_history" => {
                let user_id = data
                    .as_ref()
                    .and_then(|d| d.get("user_id"))
                    .and_then(|u| u.as_str())
                    .ok_or("User ID is required")?;
                
                let limit = data
                    .as_ref()
                    .and_then(|d| d.get("limit"))
                    .and_then(|l| l.as_i64())
                    .unwrap_or(10) as i32;
                
                let query = format!(
                    "SELECT id, text_content, word_count, sentence_count, char_count, analysis_summary, created_at
                     FROM text_analyses 
                     WHERE user_id = '{}' 
                     ORDER BY created_at DESC 
                     LIMIT {}",
                    user_id, limit
                );
                
                let results = db.query(&query)?;
                
                Ok(json!({
                    "user_id": user_id,
                    "history": results,
                    "count": results.len()
                }))
            },
            "generate_report" => {
                let limit = data
                    .as_ref()
                    .and_then(|d| d.get("limit"))
                    .and_then(|d| d.as_i64())
                    .unwrap_or(50) as i32;
                
                let query = format!(
                    "SELECT user_id, COUNT(*) as analysis_count, 
                            AVG(word_count) as avg_words, 
                            AVG(sentence_count) as avg_sentences,
                            AVG(char_count) as avg_chars,
                            MIN(created_at) as first_analysis,
                            MAX(created_at) as last_analysis
                     FROM text_analyses 
                     WHERE user_id != 'system'
                     GROUP BY user_id
                     ORDER BY analysis_count DESC
                     LIMIT {}",
                    limit
                );
                
                let stats = db.query(&query)?;
                
                // Use Claude to generate insights from the data
                let claude_prompt = format!(
                    "Generate a summary report for text analysis usage. 
                     Here's the data: {:?}
                     Please provide insights about user engagement, text complexity trends, and recommendations.",
                    stats
                );
                
                let insights = claude.ask(&claude_prompt)?;
                
                let report_id = format!("report_{}", COUNTER.fetch_add(1, Ordering::SeqCst));
                
                Ok(json!({
                    "report_id": report_id,
                    "statistics": stats,
                    "insights": insights,
                    "total_users": stats.len(),
                    "query_limit": limit
                }))
            },
            _ => Err(format!("Unknown tool: {}", tool_name))
        }
    }
    
    fn get_available_tools(&self) -> Vec<&'static str> {
        vec!["analyze_text", "get_history", "generate_report"]
    }
}

pub fn create_app() -> Box<dyn ArcadiaApp + Send + Sync> {
    Box::new(TextAnalyzerApp::new())
}
```

DEPENDENCIES:
The system automatically detects common Rust crate usage from 'use' statements.
You can also explicitly specify dependencies in the request:

```json
"dependencies": {
    "regex": "1.9",            // Regular expressions
    "rand": "0.8",             // Random number generation
    "base64": "0.21"           // Base64 encoding/decoding
}
```

Common dependencies are auto-detected when you use them:
- regex, rand, base64, hex, sha2, md5, bcrypt
- serde and serde_json are always included
- Dependencies are configured for server-side WASM compatibility
- User-provided dependencies override auto-detected versions

NOTE: For unique ID generation in server-side WASM, use atomic counters instead of timestamps:
```rust
use std::sync::atomic::{AtomicU64, Ordering};
static COUNTER: AtomicU64 = AtomicU64::new(1000);
let unique_id = format!("id_{}", COUNTER.fetch_add(1, Ordering::SeqCst));
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

CLAUDE AI INTEGRATION:
The ClaudeService provides these methods:
- `query(&self, message: &str) -> Result<String, String>` - Send any message to Claude AI
- `ask(&self, question: &str) -> Result<String, String>` - Semantic alias for asking questions

CLAUDE AI EXAMPLES:
```rust
// Basic Claude interaction
let response = claude.ask("Explain this data structure")?;

// Combine database and AI analysis
let data = db.query("SELECT * FROM sales_data")?;
let analysis = claude.query(&format!("Analyze this sales data: {:?}", data))?;

// Use Claude for intelligent decision making
let recommendation = claude.ask("Based on the current count, what should the next action be?")?;
```

BENEFITS:
- No WASM boilerplate required
- Automatic memory management
- Built-in database integration
- Claude AI integration for intelligent features
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
                    },
                    "dependencies": {
                        "type": "object",
                        "description": "Optional Rust crate dependencies to include in Cargo.toml. Common dependencies like chrono, regex, rand are auto-detected from 'use' statements. Format: {\"crate_name\": \"version_spec\"}",
                        "additionalProperties": {
                            "type": "string",
                            "description": "Crate version specification (e.g., \"1.0\", \"{ version = \\\"1.0\\\", features = [\\\"feature1\\\"] }\")"
                        }
                    }
                },
                "required": ["appId", "version", "runtime", "tools", "appSrc"]
            }
        ),
        types.Tool(
            name="schedule_app_run",
            title="Schedule App Run",
            description="Schedule an app tool to run at a specific time, either once or on a recurring basis",
            inputSchema={
                "type": "object",
                "properties": {
                    "appId": {
                        "type": "string",
                        "description": "ID of the application to schedule"
                    },
                    "toolName": {
                        "type": "string",
                        "description": "Name of the tool to execute"
                    },
                    "input": {
                        "type": "object",
                        "description": "Input data to pass to the tool"
                    },
                    "scheduleType": {
                        "type": "string",
                        "enum": ["one-time", "recurring"],
                        "description": "Type of schedule: 'one-time' or 'recurring'"
                    },
                    "scheduledTime": {
                        "type": "string",
                        "format": "date-time",
                        "description": "When to run the tool (ISO 8601 format without a time zone configuration)"
                    },
                    "recurrence": {
                        "type": "object",
                        "properties": {
                            "interval": {
                                "type": "integer",
                                "description": "Number of units between runs"
                            },
                            "unit": {
                                "type": "string",
                                "enum": ["minutes", "hours", "days", "weeks", "months"],
                                "description": "Unit of time for recurrence"
                            },
                            "daysOfWeek": {
                                "type": "array",
                                "items": {"type": "integer", "minimum": 0, "maximum": 6},
                                "description": "Days of week for weekly recurrence (0=Sunday, 1=Monday, etc.)"
                            },
                            "endDate": {
                                "type": "string",
                                "format": "date-time",
                                "description": "Optional end date for recurring schedules"
                            }
                        },
                        "required": ["interval", "unit"],
                        "description": "Recurrence pattern (required for recurring schedules)"
                    }
                },
                "required": ["appId", "toolName", "input", "scheduleType", "scheduledTime"]
            }
        ),
        types.Tool(
            name="list_schedules",
            title="List Schedules",
            description="List all scheduled app runs",
            inputSchema={
                "type": "object",
                "properties": {
                    "appId": {
                        "type": "string",
                        "description": "Optional: filter schedules by app ID"
                    }
                }
            }
        ),
        types.Tool(
            name="get_schedule",
            title="Get Schedule",
            description="Get details of a specific schedule",
            inputSchema={
                "type": "object",
                "properties": {
                    "scheduleId": {
                        "type": "string",
                        "description": "ID of the schedule to retrieve"
                    }
                },
                "required": ["scheduleId"]
            }
        ),
        types.Tool(
            name="delete_schedule",
            title="Delete Schedule",
            description="Delete a scheduled app run",
            inputSchema={
                "type": "object",
                "properties": {
                    "scheduleId": {
                        "type": "string",
                        "description": "ID of the schedule to delete"
                    }
                },
                "required": ["scheduleId"]
            }
        ),
        types.Tool(
            name="update_schedule",
            title="Update Schedule",
            description="Update an existing schedule",
            inputSchema={
                "type": "object",
                "properties": {
                    "scheduleId": {
                        "type": "string",
                        "description": "ID of the schedule to update"
                    },
                    "appId": {
                        "type": "string",
                        "description": "ID of the application to schedule"
                    },
                    "toolName": {
                        "type": "string",
                        "description": "Name of the tool to execute"
                    },
                    "input": {
                        "type": "object",
                        "description": "Input data to pass to the tool"
                    },
                    "scheduleType": {
                        "type": "string",
                        "enum": ["one-time", "recurring"],
                        "description": "Type of schedule: 'one-time' or 'recurring'"
                    },
                    "scheduledTime": {
                        "type": "string",
                        "format": "date-time",
                        "description": "When to run the tool (ISO 8601 format)"
                    },
                    "recurrence": {
                        "type": "object",
                        "properties": {
                            "interval": {
                                "type": "integer",
                                "description": "Number of units between runs"
                            },
                            "unit": {
                                "type": "string",
                                "enum": ["minutes", "hours", "days", "weeks", "months"],
                                "description": "Unit of time for recurrence"
                            },
                            "daysOfWeek": {
                                "type": "array",
                                "items": {"type": "integer", "minimum": 0, "maximum": 6},
                                "description": "Days of week for weekly recurrence (0=Sunday, 1=Monday, etc.)"
                            },
                            "endDate": {
                                "type": "string",
                                "format": "date-time",
                                "description": "Optional end date for recurring schedules"
                            }
                        },
                        "required": ["interval", "unit"],
                        "description": "Recurrence pattern (required for recurring schedules)"
                    },
                    "isActive": {
                        "type": "boolean",
                        "description": "Whether the schedule is active"
                    }
                },
                "required": ["scheduleId"]
            }
        ),
        types.Tool(
            name="list_scheduled_runs",
            title="List Scheduled Runs",
            description="List execution history of scheduled app runs",
            inputSchema={
                "type": "object",
                "properties": {
                    "scheduleId": {
                        "type": "string",
                        "description": "Optional: filter runs by schedule ID"
                    },
                    "appId": {
                        "type": "string",
                        "description": "Optional: filter runs by app ID"
                    },
                    "status": {
                        "type": "string",
                        "enum": ["running", "completed", "failed"],
                        "description": "Optional: filter runs by status"
                    }
                }
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
            elif name == "schedule_app_run":
                return await handle_schedule_app_run(client, app_engine_url, arguments)
            elif name == "list_schedules":
                return await handle_list_schedules(client, app_engine_url, arguments)
            elif name == "get_schedule":
                return await handle_get_schedule(client, app_engine_url, arguments)
            elif name == "delete_schedule":
                return await handle_delete_schedule(client, app_engine_url, arguments)
            elif name == "update_schedule":
                return await handle_update_schedule(client, app_engine_url, arguments)
            elif name == "list_scheduled_runs":
                return await handle_list_scheduled_runs(client, app_engine_url, arguments)
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


async def handle_schedule_app_run(
    client: httpx.AsyncClient,
    app_engine_url: str,
    arguments: Dict[str, Any]
) -> List[types.TextContent]:
    """Handle the schedule_app_run tool."""
    payload = arguments
    
    response = await client.post(
        f"{app_engine_url}/schedule_app_run",
        json=payload,
        headers={"Content-Type": "application/json"}
    )
    
    if not response.is_success:
        error_text = f"Schedule App Run Failed:\n"
        error_text += f"HTTP Status: {response.status_code} {response.reason_phrase}\n"
        
        try:
            error_data = response.json()
            if isinstance(error_data, dict):
                if error_data.get('error'):
                    error_text += f"Error: {error_data['error']}\n"
                if error_data.get('message'):
                    error_text += f"Message: {error_data['message']}\n"
            else:
                error_text += f"Error Response: {error_data}\n"
        except Exception:
            error_text += f"Response Text: {response.text}\n"
        
        return [types.TextContent(type="text", text=error_text)]
    
    result = response.json()
    
    output_text = f"App Run Scheduled Successfully:\n"
    output_text += f"Schedule ID: {result.get('scheduleId', 'Unknown')}\n"
    output_text += f"App ID: {result.get('appId', 'Unknown')}\n"
    output_text += f"Tool: {result.get('toolName', 'Unknown')}\n"
    output_text += f"Schedule Type: {result.get('scheduleType', 'Unknown')}\n"
    output_text += f"Scheduled Time: {result.get('scheduledTime', 'Unknown')}\n"
    if result.get('nextRun'):
        output_text += f"Next Run: {result.get('nextRun')}\n"
    
    return [types.TextContent(type="text", text=output_text)]


async def handle_list_schedules(
    client: httpx.AsyncClient,
    app_engine_url: str,
    arguments: Dict[str, Any]
) -> List[types.TextContent]:
    """Handle the list_schedules tool."""
    params = {}
    if arguments.get("appId"):
        params["appId"] = arguments["appId"]
    
    response = await client.get(
        f"{app_engine_url}/list_schedules",
        params=params
    )
    response.raise_for_status()
    
    schedules = response.json()
    
    if not schedules:
        return [types.TextContent(type="text", text="No schedules found.")]
    
    result = "Scheduled App Runs:\n\n"
    for schedule in schedules:
        result += f"Schedule ID: {schedule.get('id', 'Unknown')}\n"
        result += f"App ID: {schedule.get('appId', 'Unknown')}\n"
        result += f"Tool: {schedule.get('toolName', 'Unknown')}\n"
        result += f"Schedule Type: {schedule.get('scheduleType', 'Unknown')}\n"
        result += f"Scheduled Time: {schedule.get('scheduledTime', 'Unknown')}\n"
        result += f"Active: {schedule.get('isActive', False)}\n"
        result += f"Run Count: {schedule.get('runCount', 0)}\n"
        
        if schedule.get('lastRun'):
            result += f"Last Run: {schedule['lastRun']}\n"
        if schedule.get('nextRun'):
            result += f"Next Run: {schedule['nextRun']}\n"
        if schedule.get('recurrence'):
            rec = schedule['recurrence']
            result += f"Recurrence: Every {rec.get('interval', 1)} {rec.get('unit', 'unknown')}\n"
            if rec.get('daysOfWeek'):
                days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
                day_names = [days[d] for d in rec['daysOfWeek'] if 0 <= d <= 6]
                result += f"Days of Week: {', '.join(day_names)}\n"
            if rec.get('endDate'):
                result += f"End Date: {rec['endDate']}\n"
        
        result += "-" * 50 + "\n"
    
    return [types.TextContent(type="text", text=result)]


async def handle_get_schedule(
    client: httpx.AsyncClient,
    app_engine_url: str,
    arguments: Dict[str, Any]
) -> List[types.TextContent]:
    """Handle the get_schedule tool."""
    schedule_id = arguments["scheduleId"]
    
    response = await client.get(
        f"{app_engine_url}/get_schedule",
        params={"scheduleId": schedule_id}
    )
    
    if response.status_code == 404:
        return [types.TextContent(type="text", text=f"Schedule not found: {schedule_id}")]
    
    response.raise_for_status()
    schedule = response.json()
    
    result = f"Schedule Details:\n\n"
    result += f"Schedule ID: {schedule.get('id', 'Unknown')}\n"
    result += f"App ID: {schedule.get('appId', 'Unknown')}\n"
    result += f"Tool: {schedule.get('toolName', 'Unknown')}\n"
    result += f"Schedule Type: {schedule.get('scheduleType', 'Unknown')}\n"
    result += f"Scheduled Time: {schedule.get('scheduledTime', 'Unknown')}\n"
    result += f"Active: {schedule.get('isActive', False)}\n"
    result += f"Created At: {schedule.get('createdAt', 'Unknown')}\n"
    result += f"Run Count: {schedule.get('runCount', 0)}\n"
    
    if schedule.get('input'):
        result += f"Input Data: {schedule['input']}\n"
    if schedule.get('lastRun'):
        result += f"Last Run: {schedule['lastRun']}\n"
    if schedule.get('nextRun'):
        result += f"Next Run: {schedule['nextRun']}\n"
        
    if schedule.get('recurrence'):
        rec = schedule['recurrence']
        result += f"\nRecurrence Pattern:\n"
        result += f"- Every {rec.get('interval', 1)} {rec.get('unit', 'unknown')}\n"
        if rec.get('daysOfWeek'):
            days = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
            day_names = [days[d] for d in rec['daysOfWeek'] if 0 <= d <= 6]
            result += f"- Days of Week: {', '.join(day_names)}\n"
        if rec.get('endDate'):
            result += f"- End Date: {rec['endDate']}\n"
    
    return [types.TextContent(type="text", text=result)]


async def handle_delete_schedule(
    client: httpx.AsyncClient,
    app_engine_url: str,
    arguments: Dict[str, Any]
) -> List[types.TextContent]:
    """Handle the delete_schedule tool."""
    schedule_id = arguments["scheduleId"]
    
    response = await client.delete(
        f"{app_engine_url}/delete_schedule",
        params={"scheduleId": schedule_id}
    )
    
    if response.status_code == 404:
        return [types.TextContent(type="text", text=f"Schedule not found: {schedule_id}")]
    
    if not response.is_success:
        error_text = f"Delete Schedule Failed:\n"
        error_text += f"HTTP Status: {response.status_code} {response.reason_phrase}\n"
        
        try:
            error_data = response.json()
            if isinstance(error_data, dict) and error_data.get('error'):
                error_text += f"Error: {error_data['error']}\n"
            else:
                error_text += f"Error Response: {error_data}\n"
        except Exception:
            error_text += f"Response Text: {response.text}\n"
        
        return [types.TextContent(type="text", text=error_text)]
    
    response.raise_for_status()
    return [types.TextContent(type="text", text=f"Schedule deleted successfully: {schedule_id}")]


async def handle_update_schedule(
    client: httpx.AsyncClient,
    app_engine_url: str,
    arguments: Dict[str, Any]
) -> List[types.TextContent]:
    """Handle the update_schedule tool."""
    payload = arguments
    
    response = await client.put(
        f"{app_engine_url}/update_schedule",
        json=payload,
        headers={"Content-Type": "application/json"}
    )
    
    if response.status_code == 404:
        return [types.TextContent(type="text", text=f"Schedule not found: {arguments.get('scheduleId', 'Unknown')}")]
    
    if not response.is_success:
        error_text = f"Update Schedule Failed:\n"
        error_text += f"HTTP Status: {response.status_code} {response.reason_phrase}\n"
        
        try:
            error_data = response.json()
            if isinstance(error_data, dict):
                if error_data.get('error'):
                    error_text += f"Error: {error_data['error']}\n"
                if error_data.get('message'):
                    error_text += f"Message: {error_data['message']}\n"
            else:
                error_text += f"Error Response: {error_data}\n"
        except Exception:
            error_text += f"Response Text: {response.text}\n"
        
        return [types.TextContent(type="text", text=error_text)]
    
    result = response.json()
    
    output_text = f"Schedule Updated Successfully:\n"
    output_text += f"Schedule ID: {result.get('id', 'Unknown')}\n"
    output_text += f"App ID: {result.get('appId', 'Unknown')}\n"
    output_text += f"Tool: {result.get('toolName', 'Unknown')}\n"
    output_text += f"Schedule Type: {result.get('scheduleType', 'Unknown')}\n"
    output_text += f"Active: {result.get('isActive', False)}\n"
    if result.get('nextRun'):
        output_text += f"Next Run: {result.get('nextRun')}\n"
    
    return [types.TextContent(type="text", text=output_text)]


async def handle_list_scheduled_runs(
    client: httpx.AsyncClient,
    app_engine_url: str,
    arguments: Dict[str, Any]
) -> List[types.TextContent]:
    """Handle the list_scheduled_runs tool."""
    params = {}
    if arguments.get("scheduleId"):
        params["scheduleId"] = arguments["scheduleId"]
    if arguments.get("appId"):
        params["appId"] = arguments["appId"]
    if arguments.get("status"):
        params["status"] = arguments["status"]
    
    response = await client.get(
        f"{app_engine_url}/list_scheduled_runs",
        params=params
    )
    response.raise_for_status()
    
    runs = response.json()
    
    if not runs:
        return [types.TextContent(type="text", text="No scheduled runs found.")]
    
    result = "Scheduled Run History:\n\n"
    for run in runs:
        result += f"Run ID: {run.get('id', 'Unknown')}\n"
        result += f"Schedule ID: {run.get('scheduleId', 'Unknown')}\n"
        result += f"App ID: {run.get('appId', 'Unknown')}\n"
        result += f"Tool: {run.get('toolName', 'Unknown')}\n"
        result += f"Status: {run.get('status', 'Unknown')}\n"
        result += f"Started At: {run.get('startedAt', 'Unknown')}\n"
        
        if run.get('completedAt'):
            result += f"Completed At: {run['completedAt']}\n"
        if run.get('output'):
            result += f"Output: {run['output']}\n"
        if run.get('error'):
            result += f"Error: {run['error']}\n"
        
        result += "-" * 50 + "\n"
    
    return [types.TextContent(type="text", text=result)]


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