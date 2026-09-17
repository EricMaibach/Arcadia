package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"arcadia/services"
)

// Dependencies for schedule handlers
var (
	scheduleLogFunc         func(format string, args ...interface{})
	scheduleRegistryManager *services.RegistryManager
)

// SetScheduleDependencies configures the dependencies for schedule handlers
func SetScheduleDependencies(
	logFunc func(format string, args ...interface{}),
	registryManager *services.RegistryManager,
) {
	scheduleLogFunc = logFunc
	scheduleRegistryManager = registryManager
}

// ScheduleAppRunHandler handles POST /schedule_app_run
func ScheduleAppRunHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := fmt.Sprintf("schedule_session_%d", time.Now().UnixNano())
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = forwarded
	}

	scheduleLogFunc("=== NEW APP SCHEDULE REQUEST [%s] ===", sessionID)
	scheduleLogFunc("[%s] Client IP: %s", sessionID, clientIP)
	scheduleLogFunc("[%s] Request Method: %s", sessionID, r.Method)
	scheduleLogFunc("[%s] Request URL: %s", sessionID, r.URL.String())
	scheduleLogFunc("[%s] User-Agent: %s", sessionID, r.Header.Get("User-Agent"))
	scheduleLogFunc("[%s] Content-Type: %s", sessionID, r.Header.Get("Content-Type"))

	scheduleLogFunc("[%s] STEP 1: Parsing schedule request", sessionID)
	var req services.ScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		scheduleLogFunc("[%s] ERROR: Failed to decode request body: %v", sessionID, err)
		scheduleLogFunc("[%s] RESPONSE: HTTP 400 - Invalid JSON", sessionID)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	scheduleLogFunc("[%s] Parsed request - AppID: %s, ToolName: %s, ScheduleType: %s", sessionID, req.AppID, req.ToolName, req.ScheduleType)

	// Step 2: Validate request fields
	scheduleLogFunc("[%s] STEP 2: Validating schedule request", sessionID)
	if req.AppID == "" {
		scheduleLogFunc("[%s] ERROR: AppID is required", sessionID)
		scheduleLogFunc("[%s] RESPONSE: HTTP 400 - Missing AppID", sessionID)
		http.Error(w, "appId is required", http.StatusBadRequest)
		return
	}

	if req.ToolName == "" {
		scheduleLogFunc("[%s] ERROR: ToolName is required", sessionID)
		scheduleLogFunc("[%s] RESPONSE: HTTP 400 - Missing ToolName", sessionID)
		http.Error(w, "toolName is required", http.StatusBadRequest)
		return
	}

	if req.ScheduleType != services.ScheduleTypeOneTime && req.ScheduleType != services.ScheduleTypeRecurring {
		scheduleLogFunc("[%s] ERROR: Invalid schedule type: %s", sessionID, req.ScheduleType)
		scheduleLogFunc("[%s] RESPONSE: HTTP 400 - Invalid schedule type", sessionID)
		http.Error(w, "scheduleType must be 'one-time' or 'recurring'", http.StatusBadRequest)
		return
	}

	if req.ScheduledTime.Time.IsZero() {
		scheduleLogFunc("[%s] ERROR: ScheduledTime is required", sessionID)
		scheduleLogFunc("[%s] RESPONSE: HTTP 400 - Missing ScheduledTime", sessionID)
		http.Error(w, "scheduledTime is required", http.StatusBadRequest)
		return
	}

	// Validate scheduled time is in the future
	if req.ScheduledTime.Time.Before(time.Now()) {
		scheduleLogFunc("[%s] ERROR: ScheduledTime is in the past: %v", sessionID, req.ScheduledTime.Time)
		scheduleLogFunc("[%s] RESPONSE: HTTP 400 - Past scheduled time", sessionID)
		http.Error(w, "scheduledTime must be in the future", http.StatusBadRequest)
		return
	}

	// Validate recurrence for recurring schedules
	if req.ScheduleType == services.ScheduleTypeRecurring {
		if req.Recurrence == nil {
			scheduleLogFunc("[%s] ERROR: Recurrence is required for recurring schedules", sessionID)
			scheduleLogFunc("[%s] RESPONSE: HTTP 400 - Missing recurrence", sessionID)
			http.Error(w, "recurrence is required for recurring schedules", http.StatusBadRequest)
			return
		}
		if err := services.ValidateRecurrence(req.Recurrence); err != nil {
			scheduleLogFunc("[%s] ERROR: Invalid recurrence: %v", sessionID, err)
			scheduleLogFunc("[%s] RESPONSE: HTTP 400 - Invalid recurrence", sessionID)
			http.Error(w, fmt.Sprintf("invalid recurrence: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Step 3: Verify app exists and tool is valid
	scheduleLogFunc("[%s] STEP 3: Verifying app and tool exist", sessionID)
	registry := scheduleRegistryManager.GetRegistry()
	serviceApp, ok := registry.GetApp(req.AppID)
	if !ok {
		scheduleLogFunc("[%s] ERROR: App not found in registry: %s", sessionID, req.AppID)
		scheduleLogFunc("[%s] RESPONSE: HTTP 404 - App not found", sessionID)
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}

	// Validate that the requested tool exists in the app
	toolFound := false
	for _, tool := range serviceApp.Tools {
		if tool.Name == req.ToolName {
			toolFound = true
			scheduleLogFunc("[%s] Tool '%s' found in app", sessionID, req.ToolName)
			break
		}
	}
	if !toolFound {
		scheduleLogFunc("[%s] ERROR: Tool '%s' not found in app. Available tools: %v", sessionID, req.ToolName, serviceApp.Tools)
		scheduleLogFunc("[%s] RESPONSE: HTTP 404 - Tool not found", sessionID)
		http.Error(w, fmt.Sprintf("tool '%s' not found in app", req.ToolName), http.StatusNotFound)
		return
	}

	// Step 4: Create schedule using services
	scheduleLogFunc("[%s] STEP 4: Creating schedule", sessionID)
	schedule, err := services.CreateSchedule(req)
	if err != nil {
		scheduleLogFunc("[%s] ERROR: Failed to create schedule: %v", sessionID, err)
		scheduleLogFunc("[%s] RESPONSE: HTTP 500 - Schedule creation failed", sessionID)
		http.Error(w, fmt.Sprintf("failed to create schedule: %v", err), http.StatusInternalServerError)
		return
	}

	scheduleLogFunc("[%s] Schedule created successfully with ID: %s", sessionID, schedule.ID)

	// Step 5: Send response
	scheduleLogFunc("[%s] STEP 5: Sending success response", sessionID)
	response := map[string]interface{}{
		"status":     "schedule created successfully",
		"scheduleId": schedule.ID,
		"nextRun":    schedule.NextRun,
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
	scheduleLogFunc("[%s] === APP SCHEDULE CREATED SUCCESSFULLY ===", sessionID)
}

// ListSchedulesHandler handles GET /list_schedules
func ListSchedulesHandler(w http.ResponseWriter, r *http.Request) {
	appIdFilter := r.URL.Query().Get("appId")
	scheduleList := services.GetAllSchedules(appIdFilter)
	json.NewEncoder(w).Encode(scheduleList)
}

// GetScheduleHandler handles GET /get_schedule
func GetScheduleHandler(w http.ResponseWriter, r *http.Request) {
	scheduleID := r.URL.Query().Get("id")
	if scheduleID == "" {
		http.Error(w, "schedule id is required", http.StatusBadRequest)
		return
	}

	schedule, exists := services.GetSchedule(scheduleID)
	if !exists {
		http.Error(w, "schedule not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(schedule)
}

// DeleteScheduleHandler handles DELETE /delete_schedule
func DeleteScheduleHandler(w http.ResponseWriter, r *http.Request) {
	scheduleID := r.URL.Query().Get("id")
	if scheduleID == "" {
		http.Error(w, "schedule id is required", http.StatusBadRequest)
		return
	}

	if err := services.DeleteSchedule(scheduleID); err != nil {
		http.Error(w, "failed to delete schedule", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status": "schedule deleted successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// UpdateScheduleHandler handles PUT /update_schedule
func UpdateScheduleHandler(w http.ResponseWriter, r *http.Request) {
	scheduleID := r.URL.Query().Get("id")
	if scheduleID == "" {
		http.Error(w, "schedule id is required", http.StatusBadRequest)
		return
	}

	var updateReq services.UpdateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	schedule, err := services.UpdateSchedule(scheduleID, updateReq)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("failed to update schedule: %v", err), http.StatusInternalServerError)
		}
		return
	}

	response := map[string]interface{}{
		"status":   "schedule updated successfully",
		"schedule": schedule,
	}
	json.NewEncoder(w).Encode(response)
}

// ListScheduledRunsHandler handles GET /list_scheduled_runs
func ListScheduledRunsHandler(w http.ResponseWriter, r *http.Request) {
	// For now, return empty list until GetScheduledRuns is implemented in services
	// Future implementation would parse query parameters: appId, status, scheduleId
	response := []interface{}{}
	json.NewEncoder(w).Encode(response)
}
