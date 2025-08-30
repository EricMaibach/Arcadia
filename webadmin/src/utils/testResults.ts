// Test data showing the actual wrapper format from your API

// This matches the actual format you showed me
export const testActualResponseFormat = {
  "output": "{\"success\":true,\"data\":{\"daily_totals\":{\"calories\":1080.0,\"carbs_g\":75.0,\"fat_g\":63.0,\"fiber_g\":6.0,\"protein_g\":50.0,\"sodium_mg\":1890.0,\"sugar_g\":6.0},\"date\":\"2025-08-29\",\"meals\":[{\"calories\":540.0,\"carbs_g\":35.0,\"fat_g\":32.0,\"fiber_g\":3.0,\"item_count\":1,\"meal\":\"lunch\",\"protein_g\":25.0,\"sodium_mg\":850.0,\"sugar_g\":6.0},{\"calories\":540.0,\"carbs_g\":40.0,\"fat_g\":31.0,\"fiber_g\":3.0,\"item_count\":1,\"meal\":\"dinner\",\"protein_g\":25.0,\"sodium_mg\":1040.0,\"sugar_g\":0.0}]},\"error\":null}",
  "status": "success"
};

export const testSuccessfulArrayResult = {
  "output": "{\"success\":true,\"data\":[{\"food_name\":\"Apple\",\"calories\":95,\"protein\":0.5,\"date\":\"2025-08-30\"},{\"food_name\":\"Banana\",\"calories\":105,\"protein\":1.3,\"date\":\"2025-08-30\"},{\"food_name\":\"Orange\",\"calories\":62,\"protein\":1.2,\"date\":\"2025-08-30\"}],\"error\":null}",
  "status": "success"
};

export const testSuccessfulObjectResult = {
  "output": "{\"success\":true,\"data\":{\"user_id\":\"user123\",\"total_calories\":1850,\"daily_target\":2000,\"remaining\":150,\"goals\":{\"protein\":150,\"carbs\":200,\"fat\":65},\"last_updated\":\"2025-08-30T12:00:00Z\"},\"error\":null}",
  "status": "success"
};

export const testSuccessfulPrimitiveResult = {
  "output": "{\"success\":true,\"data\":\"Operation completed successfully\",\"error\":null}",
  "status": "success"
};

export const testSuccessfulEmptyResult = {
  "output": "{\"success\":true,\"data\":[],\"error\":null}",
  "status": "success"
};

export const testErrorResult = {
  "output": "{\"success\":false,\"data\":null,\"error\":\"Invalid user ID provided. Please check the user ID and try again.\"}",
  "status": "error"
};

export const testFailureResult = {
  "output": "{\"success\":false,\"data\":null,\"error\":\"Database connection failed\"}",
  "status": "failure"
};

// Legacy format (no wrapper) - should still work
export const testLegacyArrayResult = [
  { "name": "Test Item 1", "value": 100 },
  { "name": "Test Item 2", "value": 200 }
];

export const testLegacyObjectResult = {
  "result": "success",
  "count": 42,
  "message": "Processing complete"
};