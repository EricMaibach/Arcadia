#!/bin/bash

# API Testing Commands for Arcadia App Engine
# Run these commands in terminal after starting the server with: go run main.go

echo "🚀 Arcadia App Engine API Test Commands"
echo "========================================"
echo ""

# Server URL - change if running on different port
SERVER_URL="http://localhost:8080"

echo "📋 Available Commands:"
echo "1. test_missing_appsrc    - Test with missing AppSrc (should return 400)"
echo "2. test_missing_appid     - Test with missing AppID (should return 400)"
echo "3. test_simple_app        - Test with simple counter app (should compile)"
echo "4. test_complex_app       - Test with more complex app"
echo "5. test_invalid_json      - Test with invalid JSON"
echo "6. list_apps              - List all registered apps"
echo "7. run_all_tests          - Run all tests sequentially"
echo ""

# Function definitions
test_missing_appsrc() {
    echo "🧪 Testing missing AppSrc (should return 400)..."
    curl -X POST "$SERVER_URL/submit_app_src" \
         -H "Content-Type: application/json" \
         -H "User-Agent: TestScript/1.0" \
         -d '{
           "appId": "test-missing-src",
           "version": "1.0.0",
           "runtime": "wasm",
           "tools": ["test-tool"],
           "appSrc": ""
         }' \
         -w "\nHTTP Status: %{http_code}\n" \
         -s
    echo ""
}

test_missing_appid() {
    echo "🧪 Testing missing AppID (should return 400)..."
    curl -X POST "$SERVER_URL/submit_app_src" \
         -H "Content-Type: application/json" \
         -H "User-Agent: TestScript/1.0" \
         -d '{
           "appId": "",
           "version": "1.0.0", 
           "runtime": "wasm",
           "tools": ["test-tool"],
           "appSrc": "struct TestApp; impl ArcadiaApp for TestApp {}"
         }' \
         -w "\nHTTP Status: %{http_code}\n" \
         -s
    echo ""
}

test_simple_app() {
    echo "🧪 Testing simple counter app..."
    curl -X POST "$SERVER_URL/submit_app_src" \
         -H "Content-Type: application/json" \
         -H "User-Agent: TestScript/1.0" \
         -H "X-Forwarded-For: 10.0.0.100" \
         -d '{
           "appId": "simple-counter",
           "version": "1.0.0",
           "runtime": "wasm",
           "tools": ["increment", "get_count", "reset"],
           "appSrc": "use serde_json::json;\n\nstruct CounterApp {\n    count: i64,\n}\n\nimpl CounterApp {\n    fn new() -> Self {\n        Self { count: 0 }\n    }\n}\n\nimpl ArcadiaApp for CounterApp {\n    fn initialize(&mut self, _db: &DatabaseConnection) -> Result<(), String> {\n        Ok(())\n    }\n    \n    fn handle_tool(&mut self, tool_name: &str, _data: Option<serde_json::Value>, _db: &DatabaseConnection) -> Result<serde_json::Value, String> {\n        match tool_name {\n            \"increment\" => {\n                self.count += 1;\n                Ok(json!({ \"count\": self.count, \"action\": \"incremented\" }))\n            },\n            \"get_count\" => {\n                Ok(json!({ \"count\": self.count }))\n            },\n            \"reset\" => {\n                self.count = 0;\n                Ok(json!({ \"count\": self.count, \"action\": \"reset\" }))\n            },\n            _ => Err(format!(\"Unknown tool: {}\", tool_name))\n        }\n    }\n    \n    fn get_available_tools(&self) -> Vec<&'static str> {\n        vec![\"increment\", \"get_count\", \"reset\"]\n    }\n}"
         }' \
         -w "\nHTTP Status: %{http_code}\n" \
         -s
    echo ""
}

test_complex_app() {
    echo "🧪 Testing complex task manager app..."
    curl -X POST "$SERVER_URL/submit_app_src" \
         -H "Content-Type: application/json" \
         -H "User-Agent: TestScript/1.0" \
         -d '{
           "appId": "task-manager",
           "version": "2.0.0",
           "runtime": "wasm",
           "tools": ["add_task", "complete_task", "list_tasks", "delete_task"],
           "appSrc": "use serde_json::{json, Value};\nuse std::collections::HashMap;\n\n#[derive(Clone)]\nstruct Task {\n    id: u32,\n    title: String,\n    completed: bool,\n}\n\nstruct TaskManagerApp {\n    tasks: HashMap<u32, Task>,\n    next_id: u32,\n}\n\nimpl TaskManagerApp {\n    fn new() -> Self {\n        Self {\n            tasks: HashMap::new(),\n            next_id: 1,\n        }\n    }\n}\n\nimpl ArcadiaApp for TaskManagerApp {\n    fn initialize(&mut self, db: &DatabaseConnection) -> Result<(), String> {\n        // Create tasks table if it doesn'\''t exist\n        db.execute(\"CREATE TABLE IF NOT EXISTS tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL, completed BOOLEAN DEFAULT FALSE)\")?;\n        \n        // Load existing tasks from database\n        let rows = db.query(\"SELECT id, title, completed FROM tasks ORDER BY id\")?;\n        for row in rows {\n            if let (Some(id), Some(title), Some(completed)) = (\n                row[\"id\"].as_i64(),\n                row[\"title\"].as_str(),\n                row[\"completed\"].as_bool()\n            ) {\n                self.tasks.insert(id as u32, Task {\n                    id: id as u32,\n                    title: title.to_string(),\n                    completed,\n                });\n                if id as u32 >= self.next_id {\n                    self.next_id = id as u32 + 1;\n                }\n            }\n        }\n        \n        Ok(())\n    }\n    \n    fn handle_tool(&mut self, tool_name: &str, data: Option<Value>, db: &DatabaseConnection) -> Result<Value, String> {\n        match tool_name {\n            \"add_task\" => {\n                let title = data\n                    .and_then(|d| d[\"title\"].as_str())\n                    .ok_or(\"Missing or invalid title\")?;\n                \n                let task = Task {\n                    id: self.next_id,\n                    title: title.to_string(),\n                    completed: false,\n                };\n                \n                // Save to database\n                db.execute(&format!(\n                    \"INSERT INTO tasks (id, title, completed) VALUES ({}, '{}', false)\",\n                    task.id, title.replace(\"'\", \"''\") // Basic SQL injection protection\n                ))?;\n                \n                self.tasks.insert(self.next_id, task.clone());\n                self.next_id += 1;\n                \n                Ok(json!({\n                    \"success\": true,\n                    \"task\": {\n                        \"id\": task.id,\n                        \"title\": task.title,\n                        \"completed\": task.completed\n                    }\n                }))\n            },\n            \"complete_task\" => {\n                let id = data\n                    .and_then(|d| d[\"id\"].as_u64())\n                    .ok_or(\"Missing or invalid task id\")? as u32;\n                \n                if let Some(task) = self.tasks.get_mut(&id) {\n                    task.completed = true;\n                    \n                    // Update database\n                    db.execute(&format!(\"UPDATE tasks SET completed = true WHERE id = {}\", id))?;\n                    \n                    Ok(json!({\n                        \"success\": true,\n                        \"task\": {\n                            \"id\": task.id,\n                            \"title\": task.title,\n                            \"completed\": task.completed\n                        }\n                    }))\n                } else {\n                    Err(format!(\"Task with id {} not found\", id))\n                }\n            },\n            \"list_tasks\" => {\n                let tasks: Vec<Value> = self.tasks.values()\n                    .map(|task| json!({\n                        \"id\": task.id,\n                        \"title\": task.title,\n                        \"completed\": task.completed\n                    }))\n                    .collect();\n                \n                Ok(json!({\n                    \"success\": true,\n                    \"tasks\": tasks,\n                    \"count\": tasks.len()\n                }))\n            },\n            \"delete_task\" => {\n                let id = data\n                    .and_then(|d| d[\"id\"].as_u64())\n                    .ok_or(\"Missing or invalid task id\")? as u32;\n                \n                if self.tasks.remove(&id).is_some() {\n                    // Remove from database\n                    db.execute(&format!(\"DELETE FROM tasks WHERE id = {}\", id))?;\n                    \n                    Ok(json!({\n                        \"success\": true,\n                        \"message\": format!(\"Task {} deleted\", id)\n                    }))\n                } else {\n                    Err(format!(\"Task with id {} not found\", id))\n                }\n            },\n            _ => Err(format!(\"Unknown tool: {}\", tool_name))\n        }\n    }\n    \n    fn get_available_tools(&self) -> Vec<&'static str> {\n        vec![\"add_task\", \"complete_task\", \"list_tasks\", \"delete_task\"]\n    }\n}"
         }' \
         -w "\nHTTP Status: %{http_code}\n" \
         -s
    echo ""
}

test_invalid_json() {
    echo "🧪 Testing invalid JSON (should return 400)..."
    curl -X POST "$SERVER_URL/submit_app_src" \
         -H "Content-Type: application/json" \
         -H "User-Agent: TestScript/1.0" \
         -d '{
           "appId": "invalid-json",
           "version": "1.0.0",
           "runtime": "wasm",
           "tools": ["test"],
           "appSrc": "invalid rust code { missing quotes
         }' \
         -w "\nHTTP Status: %{http_code}\n" \
         -s 2>/dev/null || echo "❌ Invalid JSON as expected"
    echo ""
}

list_apps() {
    echo "📋 Listing all registered apps..."
    curl -X GET "$SERVER_URL/list_apps" \
         -H "User-Agent: TestScript/1.0" \
         -w "\nHTTP Status: %{http_code}\n" \
         -s | jq . 2>/dev/null || curl -X GET "$SERVER_URL/list_apps" -H "User-Agent: TestScript/1.0" -w "\nHTTP Status: %{http_code}\n" -s
    echo ""
}

run_all_tests() {
    echo "🏃 Running all tests..."
    test_missing_appsrc
    sleep 1
    test_missing_appid  
    sleep 1
    test_invalid_json
    sleep 1
    test_simple_app
    sleep 2  # Give more time for compilation
    list_apps
    sleep 1
    test_complex_app
    sleep 2
    list_apps
    echo "✅ All tests completed!"
}

# Handle command line arguments
case "${1:-}" in
    "test_missing_appsrc"|"1")
        test_missing_appsrc
        ;;
    "test_missing_appid"|"2")
        test_missing_appid
        ;;
    "test_simple_app"|"3")
        test_simple_app
        ;;
    "test_complex_app"|"4")
        test_complex_app
        ;;
    "test_invalid_json"|"5")
        test_invalid_json
        ;;
    "list_apps"|"6")
        list_apps
        ;;
    "run_all_tests"|"7")
        run_all_tests
        ;;
    "help"|"-h"|"--help")
        echo "Usage: ./test_api_commands.sh [command]"
        echo ""
        echo "Available commands:"
        echo "  test_missing_appsrc  - Test missing AppSrc field"
        echo "  test_missing_appid   - Test missing AppID field"
        echo "  test_simple_app      - Test simple counter app"
        echo "  test_complex_app     - Test complex task manager app"
        echo "  test_invalid_json    - Test invalid JSON"
        echo "  list_apps            - List registered apps"
        echo "  run_all_tests        - Run all tests"
        echo "  help                 - Show this help"
        echo ""
        echo "Or run without arguments for interactive menu."
        ;;
    "")
        # Interactive mode
        echo "Enter command number or name:"
        read -p "> " choice
        case "$choice" in
            "1"|"test_missing_appsrc") test_missing_appsrc ;;
            "2"|"test_missing_appid") test_missing_appid ;;
            "3"|"test_simple_app") test_simple_app ;;
            "4"|"test_complex_app") test_complex_app ;;
            "5"|"test_invalid_json") test_invalid_json ;;
            "6"|"list_apps") list_apps ;;
            "7"|"run_all_tests") run_all_tests ;;
            *) echo "❌ Invalid choice: $choice" ;;
        esac
        ;;
    *)
        echo "❌ Unknown command: $1"
        echo "Run './test_api_commands.sh help' for usage information."
        exit 1
        ;;
esac