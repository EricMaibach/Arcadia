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
                let user_id = data
                  .as_ref()
                  .and_then(|d| d.get("user_id"))
                  .and_then(|uid| uid.as_str())
                  .map(|s| s.to_string())
                  .unwrap_or_else(|| "".to_string());

                Ok(json!({ "count": self.count, "user_id": user_id }))
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