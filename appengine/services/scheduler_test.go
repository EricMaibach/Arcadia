package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFlexibleTime_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"RFC3339", `"2023-12-01T15:30:00Z"`, false},
		{"ISO format", `"2023-12-01T15:30:00"`, false},
		{"Date only", `"2023-12-01"`, false},
		{"US format", `"12/01/2023 15:30"`, false},
		{"Month name", `"Dec 1, 2023 3:30 PM"`, false},
		{"Full month name", `"December 1, 2023 15:30"`, false},
		{"Slash format", `"2023/12/01 15:30:00"`, false},
		{"Null value", `null`, false},
		{"Invalid format", `"not-a-date"`, true},
		{"Empty string", `""`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ft FlexibleTime
			err := ft.UnmarshalJSON([]byte(tt.input))
			
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.input != `null` && ft.Time.IsZero() {
				t.Error("Expected time to be parsed, but got zero time")
			}
		})
	}
}

func TestGenerateID(t *testing.T) {
	id1, err := GenerateID()
	if err != nil {
		t.Errorf("GenerateID() failed: %v", err)
	}

	if len(id1) == 0 {
		t.Error("GenerateID() returned empty string")
	}

	// Generate a second ID to ensure uniqueness
	id2, err := GenerateID()
	if err != nil {
		t.Errorf("Second GenerateID() failed: %v", err)
	}

	if id1 == id2 {
		t.Error("GenerateID() returned duplicate IDs")
	}

	// Check that ID contains only valid hex characters
	for _, char := range id1 {
		if !strings.ContainsRune("0123456789abcdef", char) {
			t.Errorf("GenerateID() returned invalid hex character: %c", char)
		}
	}
}

func TestValidateRecurrence(t *testing.T) {
	tests := []struct {
		name    string
		rule    *RecurrenceRule
		wantErr bool
	}{
		{
			name:    "Nil rule",
			rule:    nil,
			wantErr: true,
		},
		{
			name: "Valid minutes",
			rule: &RecurrenceRule{
				Interval: 30,
				Unit:     "minutes",
			},
			wantErr: false,
		},
		{
			name: "Valid hours",
			rule: &RecurrenceRule{
				Interval: 2,
				Unit:     "hours",
			},
			wantErr: false,
		},
		{
			name: "Valid days",
			rule: &RecurrenceRule{
				Interval: 1,
				Unit:     "days",
			},
			wantErr: false,
		},
		{
			name: "Valid weeks with days",
			rule: &RecurrenceRule{
				Interval:   1,
				Unit:       "weeks",
				DaysOfWeek: []int{1, 3, 5}, // Mon, Wed, Fri
			},
			wantErr: false,
		},
		{
			name: "Valid months",
			rule: &RecurrenceRule{
				Interval: 3,
				Unit:     "months",
			},
			wantErr: false,
		},
		{
			name: "Invalid interval (zero)",
			rule: &RecurrenceRule{
				Interval: 0,
				Unit:     "days",
			},
			wantErr: true,
		},
		{
			name: "Invalid interval (negative)",
			rule: &RecurrenceRule{
				Interval: -1,
				Unit:     "days",
			},
			wantErr: true,
		},
		{
			name: "Invalid unit",
			rule: &RecurrenceRule{
				Interval: 1,
				Unit:     "invalid",
			},
			wantErr: true,
		},
		{
			name: "Invalid day of week",
			rule: &RecurrenceRule{
				Interval:   1,
				Unit:       "weeks",
				DaysOfWeek: []int{0, 7}, // 7 is invalid
			},
			wantErr: true,
		},
		{
			name: "End date in past",
			rule: &RecurrenceRule{
				Interval: 1,
				Unit:     "days",
				EndDate:  timePtr(time.Now().Add(-24 * time.Hour)),
			},
			wantErr: true,
		},
		{
			name: "End date in future",
			rule: &RecurrenceRule{
				Interval: 1,
				Unit:     "days",
				EndDate:  timePtr(time.Now().Add(24 * time.Hour)),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRecurrence(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRecurrence() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestCalculateNextRun(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		schedule *AppSchedule
		expected *time.Time
	}{
		{
			name: "One-time unrun schedule",
			schedule: &AppSchedule{
				ScheduleType:  ScheduleTypeOneTime,
				ScheduledTime: now.Add(time.Hour),
				RunCount:      0,
			},
			expected: timePtr(now.Add(time.Hour)),
		},
		{
			name: "One-time completed schedule",
			schedule: &AppSchedule{
				ScheduleType:  ScheduleTypeOneTime,
				ScheduledTime: now.Add(time.Hour),
				RunCount:      1,
			},
			expected: nil,
		},
		{
			name: "Recurring daily from scheduled time",
			schedule: &AppSchedule{
				ScheduleType:  ScheduleTypeRecurring,
				ScheduledTime: now,
				Recurrence: &RecurrenceRule{
					Interval: 1,
					Unit:     "days",
				},
				LastRun: nil,
			},
			expected: timePtr(now.AddDate(0, 0, 1)),
		},
		{
			name: "Recurring daily from last run",
			schedule: &AppSchedule{
				ScheduleType:  ScheduleTypeRecurring,
				ScheduledTime: now,
				Recurrence: &RecurrenceRule{
					Interval: 1,
					Unit:     "days",
				},
				LastRun: timePtr(now.Add(time.Hour)),
			},
			expected: timePtr(now.Add(time.Hour).AddDate(0, 0, 1)),
		},
		{
			name: "Recurring hourly",
			schedule: &AppSchedule{
				ScheduleType:  ScheduleTypeRecurring,
				ScheduledTime: now,
				Recurrence: &RecurrenceRule{
					Interval: 2,
					Unit:     "hours",
				},
				LastRun: nil,
			},
			expected: timePtr(now.Add(2 * time.Hour)),
		},
		{
			name: "Recurring minutes",
			schedule: &AppSchedule{
				ScheduleType:  ScheduleTypeRecurring,
				ScheduledTime: now,
				Recurrence: &RecurrenceRule{
					Interval: 30,
					Unit:     "minutes",
				},
				LastRun: nil,
			},
			expected: timePtr(now.Add(30 * time.Minute)),
		},
		{
			name: "Recurring weekly",
			schedule: &AppSchedule{
				ScheduleType:  ScheduleTypeRecurring,
				ScheduledTime: now,
				Recurrence: &RecurrenceRule{
					Interval: 2,
					Unit:     "weeks",
				},
				LastRun: nil,
			},
			expected: timePtr(now.AddDate(0, 0, 14)),
		},
		{
			name: "Recurring monthly",
			schedule: &AppSchedule{
				ScheduleType:  ScheduleTypeRecurring,
				ScheduledTime: now,
				Recurrence: &RecurrenceRule{
					Interval: 3,
					Unit:     "months",
				},
				LastRun: nil,
			},
			expected: timePtr(now.AddDate(0, 3, 0)),
		},
		{
			name: "Recurring past end date",
			schedule: &AppSchedule{
				ScheduleType:  ScheduleTypeRecurring,
				ScheduledTime: now.Add(-2 * time.Hour),
				Recurrence: &RecurrenceRule{
					Interval: 1,
					Unit:     "hours",
					EndDate:  timePtr(now.Add(-30 * time.Minute)),
				},
				LastRun: timePtr(now.Add(-time.Hour)),
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateNextRun(tt.schedule)
			
			if tt.expected == nil && result != nil {
				t.Errorf("Expected nil, got %v", result)
			} else if tt.expected != nil && result == nil {
				t.Errorf("Expected %v, got nil", tt.expected)
			} else if tt.expected != nil && result != nil {
				// Allow small time difference due to test execution time
				diff := result.Sub(*tt.expected)
				if diff < -time.Second || diff > time.Second {
					t.Errorf("Expected %v, got %v (diff: %v)", tt.expected, result, diff)
				}
			}
		})
	}
}

func TestScheduler_CreateSchedule(t *testing.T) {
	mockRepo := NewMockScheduleRepository()
	scheduler := NewScheduler(mockRepo)

	futureTime := time.Now().Add(time.Hour)

	tests := []struct {
		name    string
		req     ScheduleRequest
		wantErr bool
		setup   func()
	}{
		{
			name: "Valid one-time schedule",
			req: ScheduleRequest{
				AppID:         "test-app",
				ToolName:      "test-tool",
				Input:         json.RawMessage(`{"key": "value"}`),
				ScheduleType:  ScheduleTypeOneTime,
				ScheduledTime: FlexibleTime{Time: futureTime},
			},
			wantErr: false,
		},
		{
			name: "Valid recurring schedule",
			req: ScheduleRequest{
				AppID:         "test-app",
				ToolName:      "test-tool",
				Input:         json.RawMessage(`{"key": "value"}`),
				ScheduleType:  ScheduleTypeRecurring,
				ScheduledTime: FlexibleTime{Time: futureTime},
				Recurrence: &RecurrenceRule{
					Interval: 1,
					Unit:     "hours",
				},
			},
			wantErr: false,
		},
		{
			name: "Missing app ID",
			req: ScheduleRequest{
				ToolName:      "test-tool",
				ScheduleType:  ScheduleTypeOneTime,
				ScheduledTime: FlexibleTime{Time: futureTime},
			},
			wantErr: true,
		},
		{
			name: "Missing tool name",
			req: ScheduleRequest{
				AppID:         "test-app",
				ScheduleType:  ScheduleTypeOneTime,
				ScheduledTime: FlexibleTime{Time: futureTime},
			},
			wantErr: true,
		},
		{
			name: "Invalid schedule type",
			req: ScheduleRequest{
				AppID:         "test-app",
				ToolName:      "test-tool",
				ScheduleType:  ScheduleType("invalid"),
				ScheduledTime: FlexibleTime{Time: futureTime},
			},
			wantErr: true,
		},
		{
			name: "Past scheduled time",
			req: ScheduleRequest{
				AppID:         "test-app",
				ToolName:      "test-tool",
				ScheduleType:  ScheduleTypeOneTime,
				ScheduledTime: FlexibleTime{Time: time.Now().Add(-time.Hour)},
			},
			wantErr: true,
		},
		{
			name: "Recurring without recurrence rule",
			req: ScheduleRequest{
				AppID:         "test-app",
				ToolName:      "test-tool",
				ScheduleType:  ScheduleTypeRecurring,
				ScheduledTime: FlexibleTime{Time: futureTime},
			},
			wantErr: true,
		},
		{
			name: "Database save error",
			req: ScheduleRequest{
				AppID:         "test-app",
				ToolName:      "test-tool",
				ScheduleType:  ScheduleTypeOneTime,
				ScheduledTime: FlexibleTime{Time: futureTime},
			},
			setup: func() {
				mockRepo.SaveError = fmt.Errorf("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock state and scheduler
			mockRepo.SaveError = nil
			mockRepo.Schedules = make(map[string]*AppSchedule)
			scheduler.schedules = make(map[string]*AppSchedule)
			
			if tt.setup != nil {
				tt.setup()
			}

			schedule, err := scheduler.CreateSchedule(tt.req)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSchedule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if schedule == nil {
					t.Error("Expected schedule to be created")
					return
				}

				if schedule.AppID != tt.req.AppID {
					t.Errorf("Expected AppID %s, got %s", tt.req.AppID, schedule.AppID)
				}

				if schedule.ToolName != tt.req.ToolName {
					t.Errorf("Expected ToolName %s, got %s", tt.req.ToolName, schedule.ToolName)
				}

				if !schedule.IsActive {
					t.Error("Expected schedule to be active")
				}

				if schedule.RunCount != 0 {
					t.Errorf("Expected RunCount 0, got %d", schedule.RunCount)
				}

				// Check that schedule was saved to mock database
				if len(mockRepo.Schedules) != 1 {
					t.Errorf("Expected 1 schedule in database, got %d", len(mockRepo.Schedules))
				}

				// Check that schedule was added to scheduler's in-memory map
				if len(scheduler.schedules) != 1 {
					t.Errorf("Expected 1 schedule in scheduler memory, got %d", len(scheduler.schedules))
				}
			}
		})
	}
}

func TestScheduler_GetAllSchedules(t *testing.T) {
	mockRepo := NewMockScheduleRepository()
	scheduler := NewScheduler(mockRepo)

	// Add test schedules to scheduler
	schedule1 := &AppSchedule{
		ID:       "schedule1",
		AppID:    "app1",
		ToolName: "tool1",
		IsActive: true,
	}
	schedule2 := &AppSchedule{
		ID:       "schedule2",
		AppID:    "app2",
		ToolName: "tool2",
		IsActive: true,
	}
	schedule3 := &AppSchedule{
		ID:       "schedule3",
		AppID:    "app1",
		ToolName: "tool3",
		IsActive: true,
	}

	scheduler.schedules = map[string]*AppSchedule{
		"schedule1": schedule1,
		"schedule2": schedule2,
		"schedule3": schedule3,
	}

	// Test getting all schedules
	allSchedules := scheduler.GetAllSchedules("")
	if len(allSchedules) != 3 {
		t.Errorf("Expected 3 schedules, got %d", len(allSchedules))
	}

	// Test filtering by app ID
	app1Schedules := scheduler.GetAllSchedules("app1")
	if len(app1Schedules) != 2 {
		t.Errorf("Expected 2 schedules for app1, got %d", len(app1Schedules))
	}

	// Verify that the filtered schedules have the correct app ID
	for _, schedule := range app1Schedules {
		if schedule.AppID != "app1" {
			t.Errorf("Expected AppID app1, got %s", schedule.AppID)
		}
	}

	// Test filtering with non-existent app ID
	noSchedules := scheduler.GetAllSchedules("nonexistent")
	if len(noSchedules) != 0 {
		t.Errorf("Expected 0 schedules for nonexistent app, got %d", len(noSchedules))
	}
}

func TestScheduler_GetSchedule(t *testing.T) {
	mockRepo := NewMockScheduleRepository()
	scheduler := NewScheduler(mockRepo)

	// Add test schedule
	testSchedule := &AppSchedule{
		ID:       "test-schedule",
		AppID:    "test-app",
		ToolName: "test-tool",
		IsActive: true,
	}
	scheduler.schedules["test-schedule"] = testSchedule

	// Test getting existing schedule
	schedule, exists := scheduler.GetSchedule("test-schedule")
	if !exists {
		t.Error("Expected schedule to exist")
	}
	if schedule == nil {
		t.Error("Expected non-nil schedule")
	}
	if schedule.ID != "test-schedule" {
		t.Errorf("Expected schedule ID 'test-schedule', got '%s'", schedule.ID)
	}

	// Test getting non-existent schedule
	_, exists = scheduler.GetSchedule("nonexistent")
	if exists {
		t.Error("Expected schedule to not exist")
	}
}

func TestScheduler_DeleteSchedule(t *testing.T) {
	mockRepo := NewMockScheduleRepository()
	scheduler := NewScheduler(mockRepo)

	// Add test schedule
	testSchedule := &AppSchedule{
		ID:       "test-schedule",
		AppID:    "test-app",
		ToolName: "test-tool",
		IsActive: true,
	}
	scheduler.schedules["test-schedule"] = testSchedule

	// Test deleting existing schedule
	err := scheduler.DeleteSchedule("test-schedule")
	if err != nil {
		t.Errorf("DeleteSchedule failed: %v", err)
	}

	// Verify schedule was removed from memory
	if len(scheduler.schedules) != 0 {
		t.Errorf("Expected 0 schedules in memory, got %d", len(scheduler.schedules))
	}

	// Verify deactivate was called on database
	// (The mock doesn't store deactivated schedules, so we can't verify the exact state)

	// Test deleting non-existent schedule
	err = scheduler.DeleteSchedule("nonexistent")
	if err == nil {
		t.Error("Expected error when deleting nonexistent schedule")
	}

	// Test database error during deletion
	scheduler.schedules["error-schedule"] = &AppSchedule{
		ID:       "error-schedule",
		AppID:    "test-app",
		IsActive: true,
	}
	mockRepo.DeactivateError = fmt.Errorf("database error")

	err = scheduler.DeleteSchedule("error-schedule")
	if err == nil {
		t.Error("Expected error when database fails")
	}
}

func TestScheduler_UpdateSchedule(t *testing.T) {
	mockRepo := NewMockScheduleRepository()
	scheduler := NewScheduler(mockRepo)

	// Add test schedule
	testSchedule := &AppSchedule{
		ID:            "test-schedule",
		AppID:         "test-app",
		ToolName:      "test-tool",
		ScheduledTime: time.Now().Add(time.Hour),
		IsActive:      true,
		Input:         json.RawMessage(`{"old": "value"}`),
		RunCount:      0,
	}
	scheduler.schedules["test-schedule"] = testSchedule
	mockRepo.Schedules["test-schedule"] = testSchedule

	tests := []struct {
		name      string
		updateReq UpdateScheduleRequest
		wantErr   bool
		setup     func()
	}{
		{
			name: "Update IsActive",
			updateReq: UpdateScheduleRequest{
				IsActive: boolPtr(false),
			},
			wantErr: false,
		},
		{
			name: "Update ScheduledTime",
			updateReq: UpdateScheduleRequest{
				ScheduledTime: timePtr(time.Now().Add(2 * time.Hour)),
			},
			wantErr: false,
		},
		{
			name: "Update Input",
			updateReq: UpdateScheduleRequest{
				Input: json.RawMessage(`{"new": "value"}`),
			},
			wantErr: false,
		},
		{
			name: "Update multiple fields",
			updateReq: UpdateScheduleRequest{
				IsActive: boolPtr(false),
				Input:    json.RawMessage(`{"updated": "value"}`),
			},
			wantErr: false,
		},
		{
			name:      "No fields to update",
			updateReq: UpdateScheduleRequest{},
			wantErr:   true,
		},
		{
			name: "Database update error",
			updateReq: UpdateScheduleRequest{
				IsActive: boolPtr(false),
			},
			setup: func() {
				mockRepo.UpdateError = fmt.Errorf("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock state and add test schedule
			mockRepo.UpdateError = nil
			testSched := &AppSchedule{
				ID:            "test-schedule",
				AppID:         "test-app",
				ToolName:      "test-tool",
				ScheduledTime: time.Now().Add(time.Hour),
				IsActive:      true,
				Input:         json.RawMessage(`{"old": "value"}`),
				RunCount:      0,
			}
			scheduler.schedules["test-schedule"] = testSched
			mockRepo.Schedules["test-schedule"] = testSched

			if tt.setup != nil {
				tt.setup()
			}

			updatedSchedule, err := scheduler.UpdateSchedule("test-schedule", tt.updateReq)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateSchedule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if updatedSchedule == nil {
					t.Error("Expected updated schedule to be returned")
					return
				}

				// Verify updates were applied
				if tt.updateReq.IsActive != nil && updatedSchedule.IsActive != *tt.updateReq.IsActive {
					t.Errorf("Expected IsActive %v, got %v", *tt.updateReq.IsActive, updatedSchedule.IsActive)
				}

				if tt.updateReq.Input != nil && string(updatedSchedule.Input) != string(tt.updateReq.Input) {
					t.Errorf("Expected Input %s, got %s", tt.updateReq.Input, updatedSchedule.Input)
				}
			}
		})
	}

	// Test updating non-existent schedule
	_, err := scheduler.UpdateSchedule("nonexistent", UpdateScheduleRequest{
		IsActive: boolPtr(false),
	})
	if err == nil {
		t.Error("Expected error when updating nonexistent schedule")
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func TestScheduler_Start(t *testing.T) {
	mockRepo := NewMockScheduleRepository()
	scheduler := NewScheduler(mockRepo)

	// Add a test schedule to the mock database
	testSchedule := &AppSchedule{
		ID:       "test-schedule",
		AppID:    "test-app",
		ToolName: "test-tool",
		IsActive: true,
	}
	mockRepo.Schedules["test-schedule"] = testSchedule

	// Test successful start
	err := scheduler.Start()
	if err != nil {
		t.Errorf("Start() failed: %v", err)
	}

	// Verify that schedules were loaded
	if len(scheduler.schedules) != 1 {
		t.Errorf("Expected 1 schedule loaded, got %d", len(scheduler.schedules))
	}

	// Stop the scheduler
	scheduler.Stop()

	// Test start with database error
	mockRepo.LoadError = fmt.Errorf("database error")
	scheduler2 := NewScheduler(mockRepo)
	
	err = scheduler2.Start()
	if err == nil {
		t.Error("Expected error when database fails to load schedules")
	}
}

func TestScheduler_SetExecuteAppTool(t *testing.T) {
	mockRepo := NewMockScheduleRepository()
	scheduler := NewScheduler(mockRepo)

	// Test setting execute function
	var executeCalled bool
	var executeAppID, executeToolName string

	executeFunc := func(appID, toolName string, input json.RawMessage) (string, error) {
		executeCalled = true
		executeAppID = appID
		executeToolName = toolName
		return "test output", nil
	}

	scheduler.SetExecuteAppTool(executeFunc)

	// Verify the function was set by testing execution
	if scheduler.executeAppTool == nil {
		t.Error("Execute function was not set")
	}

	// Create a test scheduled run to verify the function works
	testSchedule := &AppSchedule{
		ID:       "test-schedule",
		AppID:    "test-app",
		ToolName: "test-tool",
		Input:    json.RawMessage(`{"test": "data"}`),
		IsActive: true,
	}

	// This would normally be called by ExecuteScheduledRun, but we'll test the function directly
	if scheduler.executeAppTool != nil {
		output, err := scheduler.executeAppTool(testSchedule.AppID, testSchedule.ToolName, testSchedule.Input)
		if err != nil {
			t.Errorf("Execute function failed: %v", err)
		}
		if output != "test output" {
			t.Errorf("Expected 'test output', got '%s'", output)
		}
		if !executeCalled {
			t.Error("Execute function was not called")
		}
		if executeAppID != "test-app" {
			t.Errorf("Expected AppID 'test-app', got '%s'", executeAppID)
		}
		if executeToolName != "test-tool" {
			t.Errorf("Expected ToolName 'test-tool', got '%s'", executeToolName)
		}
	}
}

// Test backward compatibility functions
func TestBackwardCompatibilityFunctions(t *testing.T) {
	// Initialize default scheduler
	mockRepo := NewMockScheduleRepository()
	InitDefaultScheduler(mockRepo)

	// Test StartScheduler
	err := StartScheduler()
	if err != nil {
		t.Errorf("StartScheduler() failed: %v", err)
	}

	// Test CreateSchedule (global function)
	futureTime := time.Now().Add(time.Hour)
	req := ScheduleRequest{
		AppID:         "test-app",
		ToolName:      "test-tool",
		Input:         json.RawMessage(`{"key": "value"}`),
		ScheduleType:  ScheduleTypeOneTime,
		ScheduledTime: FlexibleTime{Time: futureTime},
	}
	
	schedule, err := CreateSchedule(req)
	if err != nil {
		t.Errorf("CreateSchedule() failed: %v", err)
	}
	if schedule == nil {
		t.Error("Expected schedule to be created")
	}

	// Test GetAllSchedules (global function)
	schedules := GetAllSchedules("")
	if len(schedules) != 1 {
		t.Errorf("Expected 1 schedule, got %d", len(schedules))
	}

	// Test GetSchedule (global function)
	if schedule != nil {
		gotSchedule, exists := GetSchedule(schedule.ID)
		if !exists {
			t.Error("Expected schedule to exist")
		}
		if gotSchedule.ID != schedule.ID {
			t.Errorf("Expected schedule ID %s, got %s", schedule.ID, gotSchedule.ID)
		}
	}

	// Test UpdateSchedule (global function)
	if schedule != nil {
		updateReq := UpdateScheduleRequest{
			IsActive: boolPtr(false),
		}
		updatedSchedule, err := UpdateSchedule(schedule.ID, updateReq)
		if err != nil {
			t.Errorf("UpdateSchedule() failed: %v", err)
		}
		if updatedSchedule.IsActive {
			t.Error("Expected schedule to be inactive")
		}
	}

	// Test DeleteSchedule (global function)
	if schedule != nil {
		err = DeleteSchedule(schedule.ID)
		if err != nil {
			t.Errorf("DeleteSchedule() failed: %v", err)
		}
		
		_, exists := GetSchedule(schedule.ID)
		if exists {
			t.Error("Expected schedule to be deleted")
		}
	}

	// Test SetExecuteAppTool (global function)
	executeFunc := func(appID, toolName string, input json.RawMessage) (string, error) {
		return "test output", nil
	}
	SetExecuteAppTool(executeFunc)
	
	// Verify it was set in the default scheduler
	if ExecuteAppTool == nil {
		t.Error("ExecuteAppTool was not set")
	}
	
	// Test CheckAndExecuteSchedules (global function)
	CheckAndExecuteSchedules()
	
	// Test StopScheduler (global function)
	StopScheduler()
}

func TestScheduler_ExecuteScheduledRun(t *testing.T) {
	mockRepo := NewMockScheduleRepository()
	scheduler := NewScheduler(mockRepo)
	
	// Set up execute function
	var executedAppID, executedToolName string
	scheduler.SetExecuteAppTool(func(appID, toolName string, input json.RawMessage) (string, error) {
		executedAppID = appID
		executedToolName = toolName
		return "test output", nil
	})
	
	// Create a test schedule
	schedule := &AppSchedule{
		ID:            "test-schedule",
		AppID:         "test-app",
		ToolName:      "test-tool",
		Input:         json.RawMessage(`{"test": "data"}`),
		ScheduleType:  ScheduleTypeOneTime,
		ScheduledTime: time.Now().Add(time.Hour),
		IsActive:      true,
		RunCount:      0,
	}
	
	// Add schedule to scheduler
	scheduler.schedules[schedule.ID] = schedule
	
	// Execute the scheduled run
	scheduler.ExecuteScheduledRun(schedule)
	
	// Verify the execution
	if executedAppID != "test-app" {
		t.Errorf("Expected AppID 'test-app', got '%s'", executedAppID)
	}
	if executedToolName != "test-tool" {
		t.Errorf("Expected ToolName 'test-tool', got '%s'", executedToolName)
	}
	
	// Check that a run was saved
	if len(mockRepo.ScheduledRuns) != 1 {
		t.Errorf("Expected 1 scheduled run, got %d", len(mockRepo.ScheduledRuns))
	}
	
	// Check that the schedule was updated
	if schedule.RunCount != 1 {
		t.Errorf("Expected RunCount 1, got %d", schedule.RunCount)
	}
	if schedule.LastRun == nil {
		t.Error("Expected LastRun to be set")
	}
	
	// Test with execution error
	scheduler.SetExecuteAppTool(func(appID, toolName string, input json.RawMessage) (string, error) {
		return "", fmt.Errorf("execution failed")
	})
	
	schedule2 := &AppSchedule{
		ID:            "test-schedule-2",
		AppID:         "test-app",
		ToolName:      "test-tool",
		Input:         json.RawMessage(`{"test": "data"}`),
		ScheduleType:  ScheduleTypeOneTime,
		ScheduledTime: time.Now().Add(time.Hour),
		IsActive:      true,
		RunCount:      0,
	}
	scheduler.schedules[schedule2.ID] = schedule2
	
	// Execute with error
	scheduler.ExecuteScheduledRun(schedule2)
	
	// Check that the run was marked as failed
	// Find the run for schedule2
	var failedRun *ScheduledRun
	for _, run := range mockRepo.ScheduledRuns {
		if run.ScheduleID == schedule2.ID {
			failedRun = run
			break
		}
	}
	if failedRun == nil {
		t.Error("Expected to find a run for schedule2")
	} else if failedRun.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", failedRun.Status)
	}
	
	// Test with nil executeAppTool
	scheduler.executeAppTool = nil
	schedule3 := &AppSchedule{
		ID:            "test-schedule-3",
		AppID:         "test-app",
		ToolName:      "test-tool",
		Input:         json.RawMessage(`{"test": "data"}`),
		ScheduleType:  ScheduleTypeOneTime,
		ScheduledTime: time.Now().Add(time.Hour),
		IsActive:      true,
		RunCount:      0,
	}
	scheduler.schedules[schedule3.ID] = schedule3
	
	// Execute with nil function
	scheduler.ExecuteScheduledRun(schedule3)
	
	// Should still create a run but it will be failed
	if len(mockRepo.ScheduledRuns) < 3 {
		t.Error("Expected at least 3 scheduled runs")
	}
}

func TestScheduler_CheckAndExecuteSchedules(t *testing.T) {
	mockRepo := NewMockScheduleRepository()
	scheduler := NewScheduler(mockRepo)
	
	// Set up execute function
	executionCount := 0
	scheduler.SetExecuteAppTool(func(appID, toolName string, input json.RawMessage) (string, error) {
		executionCount++
		return "test output", nil
	})
	
	// Create schedules that should run
	now := time.Now()
	schedule1 := &AppSchedule{
		ID:            "ready-schedule",
		AppID:         "test-app",
		ToolName:      "test-tool",
		Input:         json.RawMessage(`{}`),
		ScheduleType:  ScheduleTypeOneTime,
		ScheduledTime: now.Add(-time.Minute), // Past due
		IsActive:      true,
		NextRun:       timePtr(now.Add(-time.Minute)),
		RunCount:      0,
	}
	
	// Create schedule that should not run yet
	schedule2 := &AppSchedule{
		ID:            "future-schedule",
		AppID:         "test-app",
		ToolName:      "test-tool",
		Input:         json.RawMessage(`{}`),
		ScheduleType:  ScheduleTypeOneTime,
		ScheduledTime: now.Add(2 * time.Hour), // Future
		IsActive:      true,
		NextRun:       timePtr(now.Add(2 * time.Hour)),
		RunCount:      0,
	}
	
	// Create inactive schedule
	schedule3 := &AppSchedule{
		ID:            "inactive-schedule",
		AppID:         "test-app",
		ToolName:      "test-tool",
		Input:         json.RawMessage(`{}`),
		ScheduleType:  ScheduleTypeOneTime,
		ScheduledTime: now.Add(-time.Minute),
		IsActive:      false, // Inactive
		NextRun:       timePtr(now.Add(-time.Minute)),
		RunCount:      0,
	}
	
	scheduler.schedules = map[string]*AppSchedule{
		schedule1.ID: schedule1,
		schedule2.ID: schedule2,
		schedule3.ID: schedule3,
	}
	
	// Check and execute schedules
	scheduler.CheckAndExecuteSchedules()
	
	// Give goroutines time to execute
	time.Sleep(100 * time.Millisecond)
	
	// Only schedule1 should have been executed
	if executionCount != 1 {
		t.Errorf("Expected 1 execution, got %d", executionCount)
	}
}

func TestCalculateNextRunEdgeCases(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name     string
		schedule *AppSchedule
		expected *time.Time
	}{
		{
			name: "Recurring with nil recurrence",
			schedule: &AppSchedule{
				ScheduleType:  ScheduleTypeRecurring,
				ScheduledTime: now,
				Recurrence:    nil,
			},
			expected: nil,
		},
		{
			name: "Weekly with specific days",
			schedule: &AppSchedule{
				ScheduleType:  ScheduleTypeRecurring,
				ScheduledTime: now,
				Recurrence: &RecurrenceRule{
					Interval:   1,
					Unit:       "weeks",
					DaysOfWeek: []int{1, 3, 5}, // Mon, Wed, Fri
				},
				LastRun: nil,
			},
			expected: nil, // Will be calculated based on next weekday
		},
		{
			name: "Invalid recurrence unit",
			schedule: &AppSchedule{
				ScheduleType:  ScheduleTypeRecurring,
				ScheduledTime: now,
				Recurrence: &RecurrenceRule{
					Interval: 1,
					Unit:     "invalid",
				},
				LastRun: nil,
			},
			expected: nil,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateNextRun(tt.schedule)
			
			if tt.name == "Weekly with specific days" {
				// For weekly with specific days, just check it returns something
				if result == nil {
					t.Error("Expected a next run time for weekly with specific days")
				}
			} else if tt.expected == nil && result != nil {
				t.Errorf("Expected nil, got %v", result)
			} else if tt.expected != nil && result == nil {
				t.Errorf("Expected %v, got nil", tt.expected)
			}
		})
	}
}