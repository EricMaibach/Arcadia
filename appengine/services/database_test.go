package services

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"arcadia/pkg/logging"
)

func newTestLogger(tb testing.TB) logging.Logger {
	tb.Helper()
	l, err := logging.NewLogger(logging.DefaultConfig())
	if err != nil {
		tb.Fatalf("failed to create test logger: %v", err)
	}
	return l
}

func TestDatabaseManager_Initialize(t *testing.T) {
	dm := NewDatabaseManager(newTestLogger(t))

	// This test verifies the DatabaseManager structure
	// In real usage, Initialize() would be called, but we're testing with mocks
	if dm.appDB != nil {
		t.Error("AppDB should be nil initially")
	}
	if dm.systemDB != nil {
		t.Error("SystemDB should be nil initially")
	}
}

func TestMockSystemDB_SaveSchedule(t *testing.T) {
	mockDB := NewMockSystemDB()

	// Test successful save
	schedule := &AppSchedule{
		ID:       "test-schedule-1",
		AppID:    "test-app",
		ToolName: "test-tool",
		IsActive: true,
	}

	err := mockDB.SaveSchedule(schedule)
	if err != nil {
		t.Fatalf("SaveSchedule failed: %v", err)
	}

	// Verify schedule was saved
	if len(mockDB.Schedules) != 1 {
		t.Errorf("Expected 1 schedule, got %d", len(mockDB.Schedules))
	}

	if mockDB.Schedules["test-schedule-1"] == nil {
		t.Error("Schedule was not saved correctly")
	}
}

func TestMockSystemDB_SaveScheduleError(t *testing.T) {
	mockDB := NewMockSystemDB()
	mockDB.SaveError = fmt.Errorf("save error")

	schedule := &AppSchedule{
		ID:       "test-schedule-1",
		AppID:    "test-app",
		ToolName: "test-tool",
		IsActive: true,
	}

	err := mockDB.SaveSchedule(schedule)
	if err == nil {
		t.Error("Expected save error, got nil")
	}

	if err.Error() != "save error" {
		t.Errorf("Expected 'save error', got '%v'", err)
	}
}

func TestMockSystemDB_LoadSchedules(t *testing.T) {
	mockDB := NewMockSystemDB()

	// Add test schedules
	activeSchedule := &AppSchedule{
		ID:       "active-schedule",
		AppID:    "test-app",
		ToolName: "test-tool",
		IsActive: true,
	}
	inactiveSchedule := &AppSchedule{
		ID:       "inactive-schedule",
		AppID:    "test-app",
		ToolName: "test-tool",
		IsActive: false,
	}

	mockDB.Schedules["active-schedule"] = activeSchedule
	mockDB.Schedules["inactive-schedule"] = inactiveSchedule

	// Load schedules (should only return active ones)
	schedules, err := mockDB.LoadSchedules()
	if err != nil {
		t.Fatalf("LoadSchedules failed: %v", err)
	}

	if len(schedules) != 1 {
		t.Errorf("Expected 1 active schedule, got %d", len(schedules))
	}

	if schedules["active-schedule"] == nil {
		t.Error("Active schedule not returned")
	}

	if schedules["inactive-schedule"] != nil {
		t.Error("Inactive schedule should not be returned")
	}
}

func TestMockSystemDB_LoadSchedulesError(t *testing.T) {
	mockDB := NewMockSystemDB()
	mockDB.LoadError = fmt.Errorf("load error")

	schedules, err := mockDB.LoadSchedules()
	if err == nil {
		t.Error("Expected load error, got nil")
	}

	if schedules != nil {
		t.Error("Expected nil schedules on error")
	}
}

func TestMockSystemDB_UpdateSchedule(t *testing.T) {
	mockDB := NewMockSystemDB()

	// Create and save initial schedule
	schedule := &AppSchedule{
		ID:       "test-schedule",
		AppID:    "test-app",
		ToolName: "test-tool",
		RunCount: 0,
		IsActive: true,
	}
	mockDB.SaveSchedule(schedule)

	// Update schedule
	schedule.RunCount = 5
	err := mockDB.UpdateSchedule(schedule)
	if err != nil {
		t.Fatalf("UpdateSchedule failed: %v", err)
	}

	// Verify update
	updated := mockDB.Schedules["test-schedule"]
	if updated.RunCount != 5 {
		t.Errorf("Expected RunCount 5, got %d", updated.RunCount)
	}
}

func TestMockSystemDB_DeactivateSchedule(t *testing.T) {
	mockDB := NewMockSystemDB()

	// Create and save active schedule
	schedule := &AppSchedule{
		ID:       "test-schedule",
		AppID:    "test-app",
		ToolName: "test-tool",
		IsActive: true,
	}
	mockDB.SaveSchedule(schedule)

	// Deactivate schedule
	err := mockDB.DeactivateSchedule("test-schedule")
	if err != nil {
		t.Fatalf("DeactivateSchedule failed: %v", err)
	}

	// Verify schedule is deactivated
	deactivated := mockDB.Schedules["test-schedule"]
	if deactivated.IsActive {
		t.Error("Schedule should be deactivated")
	}
}

func TestMockSystemDB_SaveScheduledRun(t *testing.T) {
	mockDB := NewMockSystemDB()

	run := &ScheduledRun{
		ID:         "test-run",
		ScheduleID: "test-schedule",
		AppID:      "test-app",
		ToolName:   "test-tool",
		Status:     "running",
		StartedAt:  time.Now(),
	}

	err := mockDB.SaveScheduledRun(run)
	if err != nil {
		t.Fatalf("SaveScheduledRun failed: %v", err)
	}

	// Verify run was saved
	if len(mockDB.ScheduledRuns) != 1 {
		t.Errorf("Expected 1 run, got %d", len(mockDB.ScheduledRuns))
	}

	if mockDB.ScheduledRuns["test-run"] == nil {
		t.Error("Run was not saved correctly")
	}
}

func TestMockSystemDB_UpdateScheduledRun(t *testing.T) {
	mockDB := NewMockSystemDB()

	// Create and save initial run
	run := &ScheduledRun{
		ID:         "test-run",
		ScheduleID: "test-schedule",
		AppID:      "test-app",
		ToolName:   "test-tool",
		Status:     "running",
		StartedAt:  time.Now(),
	}
	mockDB.SaveScheduledRun(run)

	// Update run
	completedAt := time.Now()
	run.Status = "completed"
	run.CompletedAt = &completedAt
	run.Output = "test output"

	err := mockDB.UpdateScheduledRun(run)
	if err != nil {
		t.Fatalf("UpdateScheduledRun failed: %v", err)
	}

	// Verify update
	updated := mockDB.ScheduledRuns["test-run"]
	if updated.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", updated.Status)
	}
	if updated.Output != "test output" {
		t.Errorf("Expected output 'test output', got '%s'", updated.Output)
	}
}

func TestSQLiteDatabase_Exec(t *testing.T) {
	// Use in-memory SQLite database for testing
	db, err := NewSQLiteDatabase(":memory:", newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create a test table
	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Test successful execution
	result, err := db.Exec("INSERT INTO test (name) VALUES (?)", "test-value")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	// Check result values
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected failed: %v", err)
	}
	if rowsAffected != 1 {
		t.Errorf("Expected 1 row affected, got %d", rowsAffected)
	}

	lastInsertId, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId failed: %v", err)
	}
	if lastInsertId != 1 {
		t.Errorf("Expected LastInsertId 1, got %d", lastInsertId)
	}
}

func TestSQLiteDatabase_ExecError(t *testing.T) {
	// Use in-memory SQLite database for testing
	db, err := NewSQLiteDatabase(":memory:", newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Test execution error with invalid SQL
	result, err := db.Exec("INVALID SQL STATEMENT")
	if err == nil {
		t.Error("Expected exec error, got nil")
	}
	if result != nil {
		t.Error("Expected nil result on error")
	}
}

func TestDatabaseManagerIntegration(t *testing.T) {
	// Test the DatabaseManager with mock system database
	mockSystemDB := NewMockSystemDB()

	// Create a test schedule
	schedule := &AppSchedule{
		ID:            "integration-test",
		AppID:         "test-app",
		ToolName:      "test-tool",
		Input:         json.RawMessage(`{"key": "value"}`),
		ScheduleType:  ScheduleTypeOneTime,
		ScheduledTime: time.Now().Add(time.Hour),
		IsActive:      true,
		CreatedAt:     time.Now(),
		RunCount:      0,
	}

	// Test save
	err := mockSystemDB.SaveSchedule(schedule)
	if err != nil {
		t.Fatalf("Failed to save schedule: %v", err)
	}

	// Test load
	schedules, err := mockSystemDB.LoadSchedules()
	if err != nil {
		t.Fatalf("Failed to load schedules: %v", err)
	}

	if len(schedules) != 1 {
		t.Errorf("Expected 1 schedule, got %d", len(schedules))
	}

	loadedSchedule := schedules["integration-test"]
	if loadedSchedule == nil {
		t.Fatal("Schedule not loaded")
	}

	if loadedSchedule.AppID != "test-app" {
		t.Errorf("Expected AppID 'test-app', got '%s'", loadedSchedule.AppID)
	}
}

func TestSQLiteDatabase_Query(t *testing.T) {
	// Use in-memory SQLite database for testing
	db, err := NewSQLiteDatabase(":memory:", newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create a test table with data
	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	_, err = db.Exec("INSERT INTO test (name) VALUES (?)", "test-value1")
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	_, err = db.Exec("INSERT INTO test (name) VALUES (?)", "test-value2")
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Test successful query
	rows, err := db.Query("SELECT id, name FROM test ORDER BY id")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int
		var name string
		err = rows.Scan(&id, &name)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		count++

		if count == 1 && name != "test-value1" {
			t.Errorf("Expected first name 'test-value1', got '%s'", name)
		}
		if count == 2 && name != "test-value2" {
			t.Errorf("Expected second name 'test-value2', got '%s'", name)
		}
	}

	if count != 2 {
		t.Errorf("Expected 2 rows, got %d", count)
	}
}

func TestComplexScheduleOperations(t *testing.T) {
	mockDB := NewMockSystemDB()

	// Test with recurring schedule
	recurrence := &RecurrenceRule{
		Interval: 1,
		Unit:     "hours",
	}

	schedule := &AppSchedule{
		ID:            "recurring-test",
		AppID:         "test-app",
		ToolName:      "hourly-tool",
		Input:         json.RawMessage(`{"frequency": "hourly"}`),
		ScheduleType:  ScheduleTypeRecurring,
		ScheduledTime: time.Now().Add(time.Hour),
		Recurrence:    recurrence,
		IsActive:      true,
		CreatedAt:     time.Now(),
		RunCount:      0,
	}

	// Save recurring schedule
	err := mockDB.SaveSchedule(schedule)
	if err != nil {
		t.Fatalf("Failed to save recurring schedule: %v", err)
	}

	// Simulate multiple runs
	for i := 0; i < 3; i++ {
		// Create scheduled run
		run := &ScheduledRun{
			ID:         fmt.Sprintf("run-%d", i),
			ScheduleID: "recurring-test",
			AppID:      "test-app",
			ToolName:   "hourly-tool",
			Input:      schedule.Input,
			StartedAt:  time.Now(),
			Status:     "running",
		}

		// Save run
		err = mockDB.SaveScheduledRun(run)
		if err != nil {
			t.Fatalf("Failed to save run %d: %v", i, err)
		}

		// Complete run
		completedAt := time.Now()
		run.Status = "completed"
		run.CompletedAt = &completedAt
		run.Output = fmt.Sprintf("Run %d completed", i)

		err = mockDB.UpdateScheduledRun(run)
		if err != nil {
			t.Fatalf("Failed to update run %d: %v", i, err)
		}

		// Update schedule run count
		schedule.RunCount++
		lastRun := time.Now()
		schedule.LastRun = &lastRun

		err = mockDB.UpdateSchedule(schedule)
		if err != nil {
			t.Fatalf("Failed to update schedule after run %d: %v", i, err)
		}
	}

	// Verify final state
	if len(mockDB.ScheduledRuns) != 3 {
		t.Errorf("Expected 3 runs, got %d", len(mockDB.ScheduledRuns))
	}

	finalSchedule := mockDB.Schedules["recurring-test"]
	if finalSchedule.RunCount != 3 {
		t.Errorf("Expected RunCount 3, got %d", finalSchedule.RunCount)
	}

	// Verify all runs are completed
	for i := 0; i < 3; i++ {
		run := mockDB.ScheduledRuns[fmt.Sprintf("run-%d", i)]
		if run == nil {
			t.Errorf("Run %d not found", i)
			continue
		}
		if run.Status != "completed" {
			t.Errorf("Run %d status expected 'completed', got '%s'", i, run.Status)
		}
	}
}

func TestScheduleRepository_Methods(t *testing.T) {
	// Test that schedule repository methods return errors when database is nil
	repo := NewScheduleRepository(nil, newTestLogger(t))

	t.Run("SaveSchedule without db", func(t *testing.T) {
		schedule := &AppSchedule{ID: "test"}
		err := repo.SaveSchedule(schedule)
		if err == nil {
			t.Error("Expected error when database not initialized")
		}
	})

	t.Run("LoadSchedules without db", func(t *testing.T) {
		_, err := repo.LoadSchedules()
		if err == nil {
			t.Error("Expected error when database not initialized")
		}
	})

	t.Run("UpdateSchedule without db", func(t *testing.T) {
		schedule := &AppSchedule{ID: "test"}
		err := repo.UpdateSchedule(schedule)
		if err == nil {
			t.Error("Expected error when database not initialized")
		}
	})

	t.Run("DeactivateSchedule without db", func(t *testing.T) {
		err := repo.DeactivateSchedule("test")
		if err == nil {
			t.Error("Expected error when database not initialized")
		}
	})

	t.Run("SaveScheduledRun without db", func(t *testing.T) {
		run := &ScheduledRun{ID: "test"}
		err := repo.SaveScheduledRun(run)
		if err == nil {
			t.Error("Expected error when database not initialized")
		}
	})

	t.Run("UpdateScheduledRun without db", func(t *testing.T) {
		run := &ScheduledRun{ID: "test"}
		err := repo.UpdateScheduledRun(run)
		if err == nil {
			t.Error("Expected error when database not initialized")
		}
	})
}

func TestDatabaseManager_Methods(t *testing.T) {
	dm := NewDatabaseManager(newTestLogger(t))

	t.Run("GetAppDB without init", func(t *testing.T) {
		db := dm.GetAppDB()
		if db != nil {
			t.Error("Expected nil app database when not initialized")
		}
	})

	t.Run("GetSystemDB without init", func(t *testing.T) {
		db := dm.GetSystemDB()
		if db != nil {
			t.Error("Expected nil system database when not initialized")
		}
	})

	t.Run("Database interface test", func(t *testing.T) {
		// Test that we can create a temporary SQLite database
		// This validates our Database interface works
		tempDB, err := NewSQLiteDatabase(":memory:", newTestLogger(t))
		if err != nil {
			t.Fatalf("Failed to create in-memory SQLite database: %v", err)
		}
		defer tempDB.Close()

		// Test basic operations
		_, err = tempDB.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
		if err != nil {
			t.Fatalf("Failed to create test table: %v", err)
		}

		_, err = tempDB.Exec("INSERT INTO test (name) VALUES (?)", "test-value")
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	})

	// Test Close method
	t.Run("Close without init", func(t *testing.T) {
		// Should not panic
		dm.Close()
	})
}

func TestErrorHandling(t *testing.T) {
	mockDB := NewMockSystemDB()

	// Test all error conditions
	testSchedule := &AppSchedule{
		ID:       "error-test",
		AppID:    "test-app",
		IsActive: true,
	}

	testRun := &ScheduledRun{
		ID:         "error-run",
		ScheduleID: "error-test",
		AppID:      "test-app",
		Status:     "running",
		StartedAt:  time.Now(),
	}

	// Test save errors
	mockDB.SaveError = fmt.Errorf("save failed")
	err := mockDB.SaveSchedule(testSchedule)
	if err == nil {
		t.Error("Expected save error")
	}

	mockDB.SaveRunError = fmt.Errorf("save run failed")
	err = mockDB.SaveScheduledRun(testRun)
	if err == nil {
		t.Error("Expected save run error")
	}

	// Reset and add data for other error tests
	mockDB.SaveError = nil
	mockDB.SaveRunError = nil
	mockDB.SaveSchedule(testSchedule)
	mockDB.SaveScheduledRun(testRun)

	// Test load error
	mockDB.LoadError = fmt.Errorf("load failed")
	schedules, err := mockDB.LoadSchedules()
	if err == nil {
		t.Error("Expected load error")
	}
	if schedules != nil {
		t.Error("Expected nil schedules on load error")
	}

	// Test update errors
	mockDB.LoadError = nil
	mockDB.UpdateError = fmt.Errorf("update failed")
	err = mockDB.UpdateSchedule(testSchedule)
	if err == nil {
		t.Error("Expected update error")
	}

	mockDB.UpdateRunError = fmt.Errorf("update run failed")
	err = mockDB.UpdateScheduledRun(testRun)
	if err == nil {
		t.Error("Expected update run error")
	}

	// Test deactivate error
	mockDB.DeactivateError = fmt.Errorf("deactivate failed")
	err = mockDB.DeactivateSchedule("error-test")
	if err == nil {
		t.Error("Expected deactivate error")
	}
}
