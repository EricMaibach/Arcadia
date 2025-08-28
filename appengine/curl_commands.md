# Terminal Commands for Testing Arcadia App Engine

## Prerequisites
1. Start the server: `go run main.go`
2. Server should be running on `http://localhost:8080`

## Quick Test Commands

### 1. Test Missing AppSrc (Should return 400)
```bash
curl -X POST http://localhost:8080/submit_app_src \
  -H "Content-Type: application/json" \
  -d '{"appId":"test-app","version":"1.0.0","runtime":"wasm","tools":["test"],"appSrc":""}' \
  -w "\nStatus: %{http_code}\n"
```

### 2. Test Missing AppID (Should return 400)
```bash
curl -X POST http://localhost:8080/submit_app_src \
  -H "Content-Type: application/json" \
  -d '{"appId":"","version":"1.0.0","runtime":"wasm","tools":["test"],"appSrc":"struct App; impl ArcadiaApp for App {}"}' \
  -w "\nStatus: %{http_code}\n"
```

### 3. Test Simple Counter App (Should compile and succeed)
```bash
curl -X POST http://localhost:8080/submit_app_src \
  -H "Content-Type: application/json" \
  -H "X-Forwarded-For: 192.168.1.100" \
  -d '{
    "appId": "counter-app",
    "version": "1.0.0",
    "runtime": "wasm",
    "tools": ["increment", "get_count"],
    "appSrc": "use serde_json::json;\n\nstruct CounterApp {\n    count: i64,\n}\n\nimpl CounterApp {\n    fn new() -> Self {\n        Self { count: 0 }\n    }\n}\n\nimpl ArcadiaApp for CounterApp {\n    fn initialize(&mut self, _db: &DatabaseConnection) -> Result<(), String> {\n        Ok(())\n    }\n    \n    fn handle_tool(&mut self, tool_name: &str, _data: Option<serde_json::Value>, _db: &DatabaseConnection) -> Result<serde_json::Value, String> {\n        match tool_name {\n            \"increment\" => {\n                self.count += 1;\n                Ok(json!({ \"count\": self.count }))\n            },\n            \"get_count\" => {\n                Ok(json!({ \"count\": self.count }))\n            },\n            _ => Err(format!(\"Unknown tool: {}\", tool_name))\n        }\n    }\n    \n    fn get_available_tools(&self) -> Vec<&'static str> {\n        vec![\"increment\", \"get_count\"]\n    }\n}"
  }' \
  -w "\nStatus: %{http_code}\n"
```

### 4. List All Registered Apps
```bash
curl -X GET http://localhost:8080/list_apps \
  -H "Content-Type: application/json" \
  -w "\nStatus: %{http_code}\n" | jq .
```

### 5. Test Calculator App
```bash
curl -X POST http://localhost:8080/submit_app_src \
  -H "Content-Type: application/json" \
  -d '{
    "appId": "calculator",
    "version": "1.0.0", 
    "runtime": "wasm",
    "tools": ["add", "subtract", "multiply", "divide"],
    "appSrc": "use serde_json::{json, Value};\n\nstruct CalculatorApp;\n\nimpl CalculatorApp {\n    fn new() -> Self {\n        Self\n    }\n}\n\nimpl ArcadiaApp for CalculatorApp {\n    fn initialize(&mut self, _db: &DatabaseConnection) -> Result<(), String> {\n        Ok(())\n    }\n    \n    fn handle_tool(&mut self, tool_name: &str, data: Option<Value>, _db: &DatabaseConnection) -> Result<Value, String> {\n        let data = data.ok_or(\"No input data provided\")?;\n        let a = data[\"a\"].as_f64().ok_or(\"Missing or invalid parameter a\")?;\n        let b = data[\"b\"].as_f64().ok_or(\"Missing or invalid parameter b\")?;\n        \n        let result = match tool_name {\n            \"add\" => a + b,\n            \"subtract\" => a - b,\n            \"multiply\" => a * b,\n            \"divide\" => {\n                if b == 0.0 {\n                    return Err(\"Division by zero\".to_string());\n                }\n                a / b\n            },\n            _ => return Err(format!(\"Unknown operation: {}\", tool_name))\n        };\n        \n        Ok(json!({\n            \"operation\": tool_name,\n            \"a\": a,\n            \"b\": b,\n            \"result\": result\n        }))\n    }\n    \n    fn get_available_tools(&self) -> Vec<&'static str> {\n        vec![\"add\", \"subtract\", \"multiply\", \"divide\"]\n    }\n}"
  }' \
  -w "\nStatus: %{http_code}\n"
```

### 6. Test Running a Tool (After successful app submission)
```bash
curl -X POST http://localhost:8080/run_tool \
  -H "Content-Type: application/json" \
  -d '{
    "appId": "calculator",
    "toolName": "add", 
    "input": {"a": 5, "b": 3}
  }' \
  -w "\nStatus: %{http_code}\n"
```

## Using the Test Script

For easier testing, use the provided script:

```bash
# Make it executable (already done)
chmod +x test_api_commands.sh

# Run interactively
./test_api_commands.sh

# Or run specific tests
./test_api_commands.sh test_simple_app
./test_api_commands.sh list_apps
./test_api_commands.sh run_all_tests

# See all options
./test_api_commands.sh help
```

## What to Watch For

1. **Server Logs**: Watch the console where you ran `go run main.go` for detailed processing logs
2. **Log Files**: Check `logs/app_submissions_YYYY-MM-DD.log` for detailed request logging
3. **HTTP Status Codes**:
   - `201`: Successfully created and compiled
   - `400`: Bad request (missing fields, invalid JSON)
   - `409`: Conflict (app version already exists)
   - `500`: Server error (compilation failed, etc.)

## Monitoring Logs in Real-Time

In a separate terminal, you can watch the log file:
```bash
tail -f logs/app_submissions_$(date +%Y-%m-%d).log
```

## Notes

- The compilation process may take a few seconds, especially for the first request as Rust dependencies are downloaded
- Failed compilations will show detailed error messages in both the response and the logs
- Each request gets a unique session ID for tracking in the logs
- The script includes both simple and complex app examples to test different scenarios