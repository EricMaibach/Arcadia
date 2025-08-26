use std::alloc::{alloc, dealloc, Layout};
use std::ptr;
use serde::{Deserialize, Serialize};

static mut RESULT_PTR: *mut u8 = ptr::null_mut();
static mut RESULT_LEN: usize = 0;

#[derive(Deserialize)]
struct GreetInput {
    text: String,
}

#[derive(Serialize)]
struct GreetOutput {
    message: String,
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
    // Read input JSON from memory
    let input = unsafe {
        let slice = std::slice::from_raw_parts(input_ptr, input_len);
        std::str::from_utf8_unchecked(slice)
    };
    
    // Parse input
    let greet_input: GreetInput = match serde_json::from_str(input) {
        Ok(input) => input,
        Err(_) => {
            // If JSON parsing fails, treat the entire input as text
            GreetInput {
                text: input.to_string(),
            }
        }
    };
    
    // Create output
    let output = GreetOutput {
        message: format!("Hello {}", greet_input.text),
    };
    
    // Serialize output to JSON
    let output_json = serde_json::to_string(&output).unwrap();
    let output_bytes = output_json.into_bytes();
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