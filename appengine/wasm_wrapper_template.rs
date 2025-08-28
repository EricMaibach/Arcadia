use std::alloc::{alloc, dealloc, Layout};
use std::ptr;
use serde::{Deserialize, Serialize};

static mut RESULT_PTR: *mut u8 = ptr::null_mut();
static mut RESULT_LEN: usize = 0;

// External database functions available to apps
extern "C" {
    fn db_query(query_ptr: *const u8, query_len: usize, result_ptr_ptr: *mut *const u8) -> i32;
    fn db_exec(stmt_ptr: *const u8, stmt_len: usize) -> i32;
    fn db_prepared_query(stmt_ptr: *const u8, stmt_len: usize, params_ptr: *const u8, params_len: usize, result_ptr_ptr: *mut *const u8) -> i32;
}

#[derive(Serialize, Deserialize, Debug)]
pub struct ToolRequest {
    pub tool: String,
    pub data: Option<serde_json::Value>,
}

#[derive(Serialize, Deserialize, Debug)]
pub struct ToolResponse {
    pub success: bool,
    pub data: Option<serde_json::Value>,
    pub error: Option<String>,
}

pub struct DatabaseConnection;

impl DatabaseConnection {
    pub fn query(&self, query: &str) -> Result<Vec<serde_json::Value>, String> {
        let query_bytes = query.as_bytes();
        let mut result_ptr: *const u8 = ptr::null();
        
        let result = unsafe {
            db_query(
                query_bytes.as_ptr(),
                query_bytes.len(),
                &mut result_ptr as *mut *const u8,
            )
        };
        
        if result < 0 {
            return Err(format!("Database query failed with error code: {}", result));
        }
        
        let json_result = unsafe {
            let slice = std::slice::from_raw_parts(result_ptr, result as usize);
            String::from_utf8_unchecked(slice.to_vec())
        };
        
        unsafe { deallocate(result_ptr as *mut u8, result as usize) };
        
        serde_json::from_str(&json_result)
            .map_err(|e| format!("JSON parsing error: {}", e))
    }
    
    pub fn execute(&self, stmt: &str) -> Result<i32, String> {
        let stmt_bytes = stmt.as_bytes();
        let result = unsafe { db_exec(stmt_bytes.as_ptr(), stmt_bytes.len()) };
        
        if result < 0 {
            Err(format!("Database execution failed with error code: {}", result))
        } else {
            Ok(result)
        }
    }
    
    pub fn prepared_query(&self, stmt: &str, params: &[serde_json::Value]) -> Result<Vec<serde_json::Value>, String> {
        let stmt_bytes = stmt.as_bytes();
        let params_json = serde_json::to_string(params)
            .map_err(|e| format!("Parameter serialization error: {}", e))?;
        let params_bytes = params_json.as_bytes();
        let mut result_ptr: *const u8 = ptr::null();
        
        let result = unsafe {
            db_prepared_query(
                stmt_bytes.as_ptr(),
                stmt_bytes.len(),
                params_bytes.as_ptr(),
                params_bytes.len(),
                &mut result_ptr as *mut *const u8,
            )
        };
        
        if result < 0 {
            return Err(format!("Database prepared query failed with error code: {}", result));
        }
        
        let json_result = unsafe {
            let slice = std::slice::from_raw_parts(result_ptr, result as usize);
            String::from_utf8_unchecked(slice.to_vec())
        };
        
        unsafe { deallocate(result_ptr as *mut u8, result as usize) };
        
        serde_json::from_str(&json_result)
            .map_err(|e| format!("JSON parsing error: {}", e))
    }
}

pub trait ArcadiaApp {
    fn initialize(&mut self, db: &DatabaseConnection) -> Result<(), String> {
        // Default implementation - apps can override if they need initialization
        Ok(())
    }
    
    fn handle_tool(&mut self, tool_name: &str, data: Option<serde_json::Value>, db: &DatabaseConnection) -> Result<serde_json::Value, String>;
    
    fn get_available_tools(&self) -> Vec<&'static str>;
}

// USER_APP_IMPL_PLACEHOLDER - This will be replaced with user's trait implementation

static mut APP_INSTANCE: Option<Box<dyn ArcadiaApp + Send + Sync>> = None;
static mut DB_CONNECTION: Option<DatabaseConnection> = None;

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
pub extern "C" fn get_result_ptr() -> *const u8 {
    unsafe { RESULT_PTR }
}

fn store_result(output: &str) -> usize {
    let output_bytes = output.as_bytes();
    let output_len = output_bytes.len();
    
    unsafe {
        if !RESULT_PTR.is_null() {
            deallocate(RESULT_PTR, RESULT_LEN);
        }
        RESULT_PTR = allocate(output_len);
        RESULT_LEN = output_len;
        ptr::copy_nonoverlapping(output_bytes.as_ptr(), RESULT_PTR, output_len);
        output_len
    }
}

#[no_mangle]
pub extern "C" fn run(input_ptr: *const u8, input_len: usize) -> usize {
    let input = unsafe {
        let slice = std::slice::from_raw_parts(input_ptr, input_len);
        std::str::from_utf8_unchecked(slice)
    };
    
    // Parse input
    let tool_request: ToolRequest = match serde_json::from_str(input) {
        Ok(input) => input,
        Err(e) => {
            let response = ToolResponse {
                success: false,
                data: None,
                error: Some(format!("Invalid input JSON: {}", e)),
            };
            let output = serde_json::to_string(&response).unwrap();
            return store_result(&output);
        }
    };
    
    unsafe {
        // Initialize database connection if not already done
        if DB_CONNECTION.is_none() {
            DB_CONNECTION = Some(DatabaseConnection);
        }
        
        // Initialize app instance if not already done
        if APP_INSTANCE.is_none() {
            let mut app = create_app();
            if let Some(ref db) = &DB_CONNECTION {
                if let Err(e) = app.initialize(db) {
                    let response = ToolResponse {
                        success: false,
                        data: None,
                        error: Some(format!("App initialization failed: {}", e)),
                    };
                    let output = serde_json::to_string(&response).unwrap();
                    return store_result(&output);
                }
            }
            APP_INSTANCE = Some(app);
        }
        
        // Handle the tool request
        if let (Some(ref mut app), Some(ref db)) = (&mut APP_INSTANCE, &DB_CONNECTION) {
            let result = app.handle_tool(&tool_request.tool, tool_request.data, db);
            
            let response = match result {
                Ok(data) => ToolResponse {
                    success: true,
                    data: Some(data),
                    error: None,
                },
                Err(error) => ToolResponse {
                    success: false,
                    data: None,
                    error: Some(error),
                },
            };
            
            let output = serde_json::to_string(&response).unwrap();
            store_result(&output)
        } else {
            let response = ToolResponse {
                success: false,
                data: None,
                error: Some("Failed to initialize app or database connection".to_string()),
            };
            let output = serde_json::to_string(&response).unwrap();
            store_result(&output)
        }
    }
}