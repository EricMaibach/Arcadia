use std::alloc::{alloc, dealloc, Layout};
use std::ptr;
use serde::{self, Deserialize, Serialize};
use serde_json;

static mut RESULT_PTR: *mut u8 = ptr::null_mut();
static mut RESULT_LEN: usize = 0;

// External database functions
extern "C" {
    fn db_exec(stmt_ptr: *const u8, stmt_len: usize) -> i32;
    fn db_query(query_ptr: *const u8, query_len: usize, result_ptr_ptr: *mut *const u8) -> i32;
}

#[derive(Serialize, Deserialize)]
struct FoodEntry {
    food_name: String,
    calories: u32,
    date_eaten: String, // Format: YYYY-MM-DD
    meal: String, // breakfast, lunch, dinner, snack
    protein: Option<f32>,
    carbohydrates: Option<f32>,
    fat: Option<f32>,
    fiber: Option<f32>,
    sugar: Option<f32>,
    sodium: Option<f32>,
}

#[derive(Serialize, Deserialize)]
struct DateRangeQuery {
    start_date: String,
    end_date: String,
}

#[derive(Serialize, Deserialize)]
struct MealQuery {
    date: String,
    meal: String,
}

#[derive(Serialize, Deserialize)]
struct ToolRequest {
    tool: String,
    data: serde_json::Value,
}

#[derive(Serialize)]
struct ApiResponse {
    success: bool,
    message: String,
    data: Option<serde_json::Value>,
}

impl ApiResponse {
    fn success(with_data: Option<serde_json::Value>) -> Self {
        Self {
            success: true,
            message: "Operation successful".to_string(),
            data: with_data,
        }
    }

    fn error(message: String) -> Self {
        Self {
            success: false,
            message,
            data: None,
        }
    }
}

fn execute_db_query(query: &str) -> Result<String, i32> {
    let query_bytes = query.as_bytes();
    let mut result_ptr: *const u8 = std::ptr::null();

    let result = unsafe {
        db_query(
            query_bytes.as_ptr(),
            query_bytes.len(),
            &mut result_ptr as *mut *const u8,
        )
    };

    if result < 0 {
        return Err(result);
    }

    let json_result = unsafe {
        let slice = std::slice::from_raw_parts(result_ptr, result as usize);
        String::from_utf8_unchecked(slice.to_vec())
    };

    // Free the allocated memory
    unsafe {
        deallocate(result_ptr as *mut u8, result as usize);
    }

    Ok(json_result)
}

fn execute_db_exec(statement: &str) -> Result<i32, i32> {
    let stmt_bytes = statement.as_bytes();
    let result = unsafe {
        db_exec(stmt_bytes.as_ptr(), stmt_bytes.len())
    };

    if result < 0 {
        Err(result)
    } else {
        Ok(result)
    }
}

fn init_database() {
    let create_table_sql = "
        CREATE TABLE IF NOT EXISTS food_entries (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            food_name TEXT NOT NULL,
            calories INTEGER NOT NULL,
            date_eaten TEXT NOT NULL,
            meal TEXT NOT NULL,
            protein REAL,
            carbohydrates REAL,
            fat REAL,
            fiber REAL,
            sugar REAL,
            sodium REAL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );
    ";

    match execute_db_exec(create_table_sql) {
        Ok(_) => {},
        Err(_) => {} // Ignore errors for initialization
    }
}

fn add_food(food: FoodEntry) -> ApiResponse {
    let insert_sql = format!(
        "INSERT INTO food_entries (food_name, calories, date_eaten, meal, protein, carbohydrates, fat, fiber, sugar, sodium) VALUES ('{}', {}, '{}', '{}', {}, {}, {}, {}, {}, {})",
        food.food_name.replace("'", "''"), // Escape single quotes for SQL
        food.calories,
        food.date_eaten,
        food.meal,
        food.protein.map_or("NULL".to_string(), |val| val.to_string()),
        food.carbohydrates.map_or("NULL".to_string(), |val| val.to_string()),
        food.fat.map_or("NULL".to_string(), |val| val.to_string()),
        food.fiber.map_or("NULL".to_string(), |val| val.to_string()),
        food.sugar.map_or("NULL".to_string(), |val| val.to_string()),
        food.sodium.map_or("NULL".to_string(), |val| val.to_string())
    );

    match execute_db_exec(&insert_sql) {
        Ok(rows_affected) => {
            if rows_affected > 0 {
                ApiResponse::success(None)
            } else {
                ApiResponse::error("Failed to insert food entry".to_string())
            }
        }
        Err(error_code) => {
            ApiResponse::error(format!("Database error: {}", error_code))
        }
    }
}

fn get_foods_for_date_range(query: DateRangeQuery) -> ApiResponse {
    let select_sql = format!(
        "SELECT * FROM food_entries WHERE date_eaten BETWEEN '{}' AND '{}' ORDER BY date_eaten, created_at",
        query.start_date,
        query.end_date
    );

    match execute_db_query(&select_sql) {
        Ok(json_result) => {
            if let Ok(data) = serde_json::from_str(&json_result) {
                ApiResponse::success(Some(data))
            } else {
                ApiResponse::error("Failed to parse query result".to_string())
            }
        }
        Err(error_code) => {
            ApiResponse::error(format!("Database error: {}", error_code))
        }
    }
}

fn get_foods_by_meal(query: MealQuery) -> ApiResponse {
    let select_sql = format!(
        "SELECT * FROM food_entries WHERE date_eaten = '{}' AND meal = '{}' ORDER BY created_at",
        query.date,
        query.meal
    );

    match execute_db_query(&select_sql) {
        Ok(json_result) => {
            if let Ok(data) = serde_json::from_str(&json_result) {
                ApiResponse::success(Some(data))
            } else {
                ApiResponse::error("Failed to parse query result".to_string())
            }
        }
        Err(error_code) => {
            ApiResponse::error(format!("Database error: {}", error_code))
        }
    }
}

fn handle_tool_request(input: &str) -> String {
    // Parse the input JSON
    let request: ToolRequest = match serde_json::from_str(input) {
        Ok(req) => req,
        Err(e) => {
            let error_response = ApiResponse::error(format!("Invalid JSON input: {}", e));
            return serde_json::to_string(&error_response).unwrap_or_else(|e| format!("{{\"success\":false,\"message\":\"Serialization error: {}\"}}", e));
        }
    };

    // Process the data and call appropriate function
    let response = match request.tool.as_str() {
        "addFood" => {
            match serde_json::from_value::<FoodEntry>(request.data) {
                Ok(food) => add_food(food),
                Err(e) => ApiResponse::error(format!("Invalid food data: {}", e)),
            }
        }
        "getFoodsForDateRange" => {
            match serde_json::from_value::<DateRangeQuery>(request.data) {
                Ok(query) => get_foods_for_date_range(query),
                Err(e) => ApiResponse::error(format!("Invalid date range data: {}", e)),
            }
        }
        "getFoodsByMeal" => {
            match serde_json::from_value::<MealQuery>(request.data) {
                Ok(query) => get_foods_by_meal(query),
                Err(e) => ApiResponse::error(format!("Invalid meal query data: {}", e)),
            }
        }
        _ => ApiResponse::error(format!("Unknown tool: {}", request.tool)),
    };

    serde_json::to_string(&response).unwrap_or_else(|e| {
        format!("{{\"success\":false,\"message\":\"Response serialization error: {}\"}}", e)
    })
}

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
    // Initialize database on first run
    init_database();

    // Read input JSON from memory
    let input = unsafe {
        let slice = std::slice::from_raw_parts(input_ptr, input_len);
        std::str::from_utf8_unchecked(slice)
    };

    // Process input and create output
    let output = handle_tool_request(input);
    let output_bytes = output.into_bytes();
    let output_len = output_bytes.len();

    // Store result in global memory
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
pub extern "C" fn get_result_ptr() -> *const u8 {
    unsafe { RESULT_PTR }
}