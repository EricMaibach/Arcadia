Submit a Rust trait implementation to compile and register a new server side WASM application.  Since it is server side WASM the Rust code does not have access to system functions, so you can't use libraries like chrono that need that, or make system calls like SystemTime.  Write coded that is server side WASM compatible.

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
                
                // Generate unique ID based on text hash to avoid collisions
                use std::collections::hash_map::DefaultHasher;
                use std::hash::{Hash, Hasher};
                let mut hasher = DefaultHasher::new();
                format!("{}{}{}", user_id, text, word_count).hash(&mut hasher);
                let analysis_id = format!("analysis_{:x}", hasher.finish());
                
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
                
                // Generate unique report ID using statistics hash
                use std::collections::hash_map::DefaultHasher;
                use std::hash::{Hash, Hasher};
                let mut hasher = DefaultHasher::new();
                format!("{:?}{}", stats, limit).hash(&mut hasher);
                let report_id = format!("report_{:x}", hasher.finish());
                
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
The system automatically detects common Rust crate usage from 'use' statements.  The code will run as a server side WASM package.  So do not use any packages that cannot run as server side WASM with not system access.
You can also explicitly specify dependencies in the request:

```json
"dependencies": {
    "regex": "1.9",            // Regular expressions
    "base64": "0.21"           // Base64 encoding/decoding
}
```

Common dependencies are auto-detected when you use them:
- regex, rand, base64, hex, sha2, md5, bcrypt
- serde and serde_json are always included
- Dependencies are configured for server-side WASM compatibility
- User-provided dependencies override auto-detected versions

NOTE: For unique ID generation in WASM apps, use content hashing for uniqueness across module instances:
```rust
use std::collections::hash_map::DefaultHasher;
use std::hash::{Hash, Hasher};
let mut hasher = DefaultHasher::new();
format!("{}{}", user_data, content).hash(&mut hasher);
let unique_id = format!("id_{:x}", hasher.finish());
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
- Input format specification for better API documentation