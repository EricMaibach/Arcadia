package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"arcadia/pkg/logging"
)

// Schedule-related type definitions
type ScheduleType string

const (
	ScheduleTypeOneTime   ScheduleType = "one-time"
	ScheduleTypeRecurring ScheduleType = "recurring"
)

type RecurrenceRule struct {
	Interval   int        `json:"interval"`
	Unit       string     `json:"unit"` // "minutes", "hours", "days", "weeks", "months"
	DaysOfWeek []int      `json:"daysOfWeek,omitempty"`
	EndDate    *time.Time `json:"endDate,omitempty"`
}

type AppSchedule struct {
	ID            string          `json:"id"`
	AppID         string          `json:"appId"`
	ToolName      string          `json:"toolName"`
	Input         json.RawMessage `json:"input"`
	ScheduleType  ScheduleType    `json:"scheduleType"`
	ScheduledTime time.Time       `json:"scheduledTime"`
	Recurrence    *RecurrenceRule `json:"recurrence,omitempty"`
	IsActive      bool            `json:"isActive"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
	NextRun       *time.Time      `json:"nextRun,omitempty"`
	LastRun       *time.Time      `json:"lastRun,omitempty"`
	RunCount      int             `json:"runCount"`
}

type ScheduledRun struct {
	ID          string          `json:"id"`
	ScheduleID  string          `json:"scheduleId"`
	AppID       string          `json:"appId"`
	ToolName    string          `json:"toolName"`
	Input       json.RawMessage `json:"input"`
	StartedAt   time.Time       `json:"startedAt"`
	CompletedAt *time.Time      `json:"completedAt,omitempty"`
	Status      string          `json:"status"` // "pending", "running", "completed", "failed"
	Output      string          `json:"output,omitempty"`
	Error       string          `json:"error,omitempty"`
}

// ScheduleRepositoryInterface defines the interface for schedule persistence
type ScheduleRepositoryInterface interface {
	SaveSchedule(schedule *AppSchedule) error
	LoadSchedules() (map[string]*AppSchedule, error)
	UpdateSchedule(schedule *AppSchedule) error
	DeactivateSchedule(scheduleID string) error
	SaveScheduledRun(run *ScheduledRun) error
	UpdateScheduledRun(run *ScheduledRun) error
}

// Scheduler manages scheduled app executions
type Scheduler struct {
	repository      ScheduleRepositoryInterface
	schedules       map[string]*AppSchedule
	schedulesMutex  sync.RWMutex
	schedulerCtx    context.Context
	schedulerCancel context.CancelFunc
	executeAppTool  func(appID, toolName string, input json.RawMessage) (string, error)
	logger          logging.Logger
}

// NewScheduler creates a new scheduler with the given repository
func NewScheduler(repository ScheduleRepositoryInterface, logger logging.Logger) *Scheduler {
	return &Scheduler{
		repository: repository,
		schedules:  make(map[string]*AppSchedule),
		logger:     logger,
	}
}

// Global scheduler instance for backward compatibility
var defaultScheduler *Scheduler

// ScheduleRequest represents a request to create a schedule
type ScheduleRequest struct {
	AppID         string          `json:"appId"`
	ToolName      string          `json:"toolName"`
	Input         json.RawMessage `json:"input"`
	ScheduleType  ScheduleType    `json:"scheduleType"`
	ScheduledTime FlexibleTime    `json:"scheduledTime"`
	Recurrence    *RecurrenceRule `json:"recurrence,omitempty"`
}

// FlexibleTime handles multiple datetime formats
type FlexibleTime struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler for flexible datetime parsing
func (ft *FlexibleTime) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}

	// Remove quotes from JSON string
	timeStr := strings.Trim(string(b), `"`)

	// Try multiple common time formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z0700",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
		"01/02/2006 15:04:05",
		"01/02/2006 15:04",
		"01/02/2006",
		"02-01-2006 15:04:05",
		"02-01-2006 15:04",
		"02-01-2006",
		"Jan 2, 2006 3:04:05 PM",
		"Jan 2, 2006 15:04:05",
		"Jan 2, 2006 3:04 PM",
		"Jan 2, 2006 15:04",
		"Jan 2, 2006",
		"January 2, 2006 3:04:05 PM",
		"January 2, 2006 15:04:05",
		"January 2, 2006 3:04 PM",
		"January 2, 2006 15:04",
		"January 2, 2006",
		"2006/01/02 15:04:05",
		"2006/01/02 15:04",
		"2006/01/02",
	}

	var lastErr error
	for _, format := range formats {
		if parsedTime, err := time.ParseInLocation(format, timeStr, time.Local); err == nil {
			ft.Time = parsedTime
			return nil
		} else {
			lastErr = err
		}
	}

	return fmt.Errorf("unable to parse time '%s' using any supported format: %v", timeStr, lastErr)
}

// GenerateID generates a unique ID
func GenerateID() (string, error) {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ValidateRecurrence validates a recurrence rule
func ValidateRecurrence(r *RecurrenceRule) error {
	if r == nil {
		return fmt.Errorf("recurrence cannot be nil")
	}

	if r.Interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}

	validUnits := map[string]bool{
		"minutes": true,
		"hours":   true,
		"days":    true,
		"weeks":   true,
		"months":  true,
	}

	if !validUnits[r.Unit] {
		return fmt.Errorf("unit must be one of: minutes, hours, days, weeks, months")
	}

	// Validate days of week for weekly recurrence
	if r.Unit == "weeks" && len(r.DaysOfWeek) > 0 {
		for _, day := range r.DaysOfWeek {
			if day < 0 || day > 6 {
				return fmt.Errorf("daysOfWeek must be between 0 (Sunday) and 6 (Saturday)")
			}
		}
	}

	// Validate end date is in the future if provided
	if r.EndDate != nil && r.EndDate.Before(time.Now()) {
		return fmt.Errorf("endDate must be in the future")
	}

	return nil
}

// CalculateNextRun calculates the next run time for a schedule
func CalculateNextRun(schedule *AppSchedule) *time.Time {
	if schedule.ScheduleType == ScheduleTypeOneTime {
		// One-time schedules don't have a "next run" after completion
		if schedule.RunCount > 0 {
			return nil
		}
		return &schedule.ScheduledTime
	}

	if schedule.Recurrence == nil {
		return nil
	}

	// Start from the last run time, or scheduled time if never run
	baseTime := schedule.ScheduledTime
	if schedule.LastRun != nil {
		baseTime = *schedule.LastRun
	}

	var nextRun time.Time

	switch schedule.Recurrence.Unit {
	case "minutes":
		nextRun = baseTime.Add(time.Duration(schedule.Recurrence.Interval) * time.Minute)
	case "hours":
		nextRun = baseTime.Add(time.Duration(schedule.Recurrence.Interval) * time.Hour)
	case "days":
		nextRun = baseTime.AddDate(0, 0, schedule.Recurrence.Interval)
	case "weeks":
		if len(schedule.Recurrence.DaysOfWeek) == 0 {
			// Simple weekly interval
			nextRun = baseTime.AddDate(0, 0, 7*schedule.Recurrence.Interval)
		} else {
			// Find next occurrence on specified days of week
			nextRun = findNextWeeklyOccurrence(baseTime, schedule.Recurrence)
		}
	case "months":
		nextRun = baseTime.AddDate(0, schedule.Recurrence.Interval, 0)
	default:
		return nil
	}

	// Check if we've passed the end date
	if schedule.Recurrence.EndDate != nil && nextRun.After(*schedule.Recurrence.EndDate) {
		return nil
	}

	return &nextRun
}

func findNextWeeklyOccurrence(baseTime time.Time, recurrence *RecurrenceRule) time.Time {
	current := baseTime.AddDate(0, 0, 1) // Start from the day after base time

	for i := 0; i < 14; i++ { // Look ahead maximum 2 weeks
		currentWeekday := int(current.Weekday())
		for _, day := range recurrence.DaysOfWeek {
			if currentWeekday == day {
				return current
			}
		}
		current = current.AddDate(0, 0, 1)
	}

	// Fallback to simple weekly interval if no matching day found
	return baseTime.AddDate(0, 0, 7*recurrence.Interval)
}

// Start starts the scheduler service
func (s *Scheduler) Start() error {
	ctx := context.Background()
	s.schedulerCtx, s.schedulerCancel = context.WithCancel(context.Background())

	// Load schedules from repository
	loadedSchedules, err := s.repository.LoadSchedules()
	if err != nil {
		return fmt.Errorf("failed to load schedules: %v", err)
	}
	s.schedulesMutex.Lock()
	s.schedules = loadedSchedules
	s.schedulesMutex.Unlock()

	// Start scheduler goroutine
	go s.schedulerLoop()
	if s.logger != nil {
		s.logger.Info(ctx, "Scheduler started")
	}
	return nil
}

// StartScheduler starts the scheduler service (backward compatibility)
func StartScheduler() error {
	if defaultScheduler == nil {
		return fmt.Errorf("default scheduler not initialized - use InitDefaultScheduler first")
	}
	return defaultScheduler.Start()
}

// Stop stops the scheduler service
func (s *Scheduler) Stop() {
	ctx := context.Background()
	if s.schedulerCancel != nil {
		s.schedulerCancel()
		if s.logger != nil {
			s.logger.Info(ctx, "Scheduler stopped")
		}
	}
}

// StopScheduler stops the scheduler service (backward compatibility)
func StopScheduler() {
	if defaultScheduler != nil {
		defaultScheduler.Stop()
	}
}

func (s *Scheduler) schedulerLoop() {
	ctx := context.Background()
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	if s.logger != nil {
		s.logger.Info(ctx, "Scheduler loop started")
	}

	for {
		select {
		case <-s.schedulerCtx.Done():
			if s.logger != nil {
				s.logger.Info(ctx, "Scheduler loop terminating")
			}
			return
		case <-ticker.C:
			s.CheckAndExecuteSchedules()
		}
	}
}

// CheckAndExecuteSchedules checks for schedules that are ready to run and executes them
func (s *Scheduler) CheckAndExecuteSchedules() {
	ctx := context.Background()
	now := time.Now()
	s.schedulesMutex.RLock()
	var schedulesToRun []*AppSchedule

	for _, schedule := range s.schedules {
		if schedule.IsActive && schedule.NextRun != nil && schedule.NextRun.Before(now.Add(time.Minute)) {
			schedulesToRun = append(schedulesToRun, schedule)
		}
	}
	s.schedulesMutex.RUnlock()

	if len(schedulesToRun) > 0 {
		if s.logger != nil {
			s.logger.Info(ctx, "Found schedules ready to run", "count", len(schedulesToRun))
		}
	}

	for _, schedule := range schedulesToRun {
		go s.ExecuteScheduledRun(schedule)
	}
}

// CheckAndExecuteSchedules backward compatibility function
func CheckAndExecuteSchedules() {
	if defaultScheduler != nil {
		defaultScheduler.CheckAndExecuteSchedules()
	}
}

// ExecuteScheduledRun executes a scheduled run
func (s *Scheduler) ExecuteScheduledRun(schedule *AppSchedule) {
	ctx := context.Background()
	runID, err := GenerateID()
	if err != nil {
		if s.logger != nil {
			s.logger.Error(ctx, "Failed to generate run ID for schedule", "error", err, "schedule_id", schedule.ID)
		}
		return
	}

	startTime := time.Now()
	if s.logger != nil {
		s.logger.Info(ctx, "Starting scheduled run", "run_id", runID, "schedule_id", schedule.ID, "app_id", schedule.AppID, "tool_name", schedule.ToolName)
	}

	// Create scheduled run record
	run := &ScheduledRun{
		ID:         runID,
		ScheduleID: schedule.ID,
		AppID:      schedule.AppID,
		ToolName:   schedule.ToolName,
		Input:      schedule.Input,
		StartedAt:  startTime,
		Status:     "running",
	}

	// Save run record to repository
	if err := s.repository.SaveScheduledRun(run); err != nil {
		if s.logger != nil {
			s.logger.Error(ctx, "Failed to save scheduled run record", "error", err)
		}
		return
	}

	// Execute the app tool using the injected function
	var output string
	if s.executeAppTool != nil {
		output, err = s.executeAppTool(schedule.AppID, schedule.ToolName, schedule.Input)
	} else {
		err = fmt.Errorf("executeAppTool function not set")
	}

	completedAt := time.Now()
	run.CompletedAt = &completedAt

	if err != nil {
		run.Status = "failed"
		run.Error = err.Error()
		if s.logger != nil {
			s.logger.Error(ctx, "Scheduled run failed", "run_id", runID, "error", err)
		}
	} else {
		run.Status = "completed"
		run.Output = output
		if s.logger != nil {
			s.logger.Info(ctx, "Scheduled run completed successfully", "run_id", runID, "duration", completedAt.Sub(startTime))
		}
	}

	// Update run record in repository
	if err := s.repository.UpdateScheduledRun(run); err != nil {
		if s.logger != nil {
			s.logger.Error(ctx, "Failed to update scheduled run record", "error", err)
		}
	}

	// Update schedule's last run and calculate next run
	s.schedulesMutex.Lock()
	schedule.LastRun = &startTime
	schedule.RunCount++
	schedule.NextRun = CalculateNextRun(schedule)
	s.schedulesMutex.Unlock()

	// Update schedule in repository
	if err := s.repository.UpdateSchedule(schedule); err != nil {
		if s.logger != nil {
			s.logger.Error(ctx, "Failed to update schedule in database", "error", err)
		}
	}

	if s.logger != nil {
		s.logger.Info(ctx, "Scheduled run processing complete", "run_id", runID, "next_run", schedule.NextRun)
	}
}

// SetExecuteAppTool sets the app tool execution function for this scheduler
func (s *Scheduler) SetExecuteAppTool(fn func(appID, toolName string, input json.RawMessage) (string, error)) {
	s.executeAppTool = fn
}

// Global function for backward compatibility
var ExecuteAppTool func(appID, toolName string, input json.RawMessage) (string, error)

// SetExecuteAppTool sets the app tool execution function (backward compatibility)
func SetExecuteAppTool(fn func(appID, toolName string, input json.RawMessage) (string, error)) {
	ExecuteAppTool = fn
	if defaultScheduler != nil {
		defaultScheduler.SetExecuteAppTool(fn)
	}
}

// Schedule management functions

// CreateSchedule creates and stores a new schedule
func (s *Scheduler) CreateSchedule(req ScheduleRequest) (*AppSchedule, error) {
	// Validate request
	if req.AppID == "" {
		return nil, fmt.Errorf("appId is required")
	}
	if req.ToolName == "" {
		return nil, fmt.Errorf("toolName is required")
	}
	if req.ScheduleType != ScheduleTypeOneTime && req.ScheduleType != ScheduleTypeRecurring {
		return nil, fmt.Errorf("scheduleType must be 'one-time' or 'recurring'")
	}
	if req.ScheduledTime.Time.IsZero() {
		return nil, fmt.Errorf("scheduledTime is required")
	}
	if req.ScheduledTime.Time.Before(time.Now()) {
		return nil, fmt.Errorf("scheduledTime must be in the future")
	}

	// Validate recurrence for recurring schedules
	if req.ScheduleType == ScheduleTypeRecurring {
		if req.Recurrence == nil {
			return nil, fmt.Errorf("recurrence is required for recurring schedules")
		}
		if err := ValidateRecurrence(req.Recurrence); err != nil {
			return nil, fmt.Errorf("invalid recurrence: %v", err)
		}
	}

	// Generate schedule ID and create schedule
	scheduleID, err := GenerateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate schedule ID: %w", err)
	}

	now := time.Now()
	schedule := &AppSchedule{
		ID:            scheduleID,
		AppID:         req.AppID,
		ToolName:      req.ToolName,
		Input:         req.Input,
		ScheduleType:  req.ScheduleType,
		ScheduledTime: req.ScheduledTime.Time,
		Recurrence:    req.Recurrence,
		IsActive:      true,
		CreatedAt:     now,
		RunCount:      0,
	}

	nextRun := req.ScheduledTime.Time
	schedule.NextRun = &nextRun

	// Store schedule in repository
	if err := s.repository.SaveSchedule(schedule); err != nil {
		return nil, fmt.Errorf("failed to save schedule: %w", err)
	}

	// Store schedule in memory
	s.schedulesMutex.Lock()
	s.schedules[scheduleID] = schedule
	s.schedulesMutex.Unlock()

	return schedule, nil
}

// GetAllSchedules returns all schedules, optionally filtered by app ID
func (s *Scheduler) GetAllSchedules(appIdFilter string) []*AppSchedule {
	s.schedulesMutex.RLock()
	defer s.schedulesMutex.RUnlock()

	scheduleList := []*AppSchedule{}
	for _, schedule := range s.schedules {
		// Apply app ID filter if specified
		if appIdFilter != "" && schedule.AppID != appIdFilter {
			continue
		}
		scheduleList = append(scheduleList, schedule)
	}

	return scheduleList
}

// GetSchedule returns a specific schedule by ID
func (s *Scheduler) GetSchedule(scheduleID string) (*AppSchedule, bool) {
	s.schedulesMutex.RLock()
	defer s.schedulesMutex.RUnlock()

	schedule, exists := s.schedules[scheduleID]
	return schedule, exists
}

// DeleteSchedule deactivates and removes a schedule
func (s *Scheduler) DeleteSchedule(scheduleID string) error {
	s.schedulesMutex.Lock()
	schedule, exists := s.schedules[scheduleID]
	if !exists {
		s.schedulesMutex.Unlock()
		return fmt.Errorf("schedule not found")
	}

	// Mark as inactive in database
	schedule.IsActive = false
	delete(s.schedules, scheduleID)
	s.schedulesMutex.Unlock()

	// Update in repository
	if err := s.repository.DeactivateSchedule(scheduleID); err != nil {
		return fmt.Errorf("failed to deactivate schedule in database: %v", err)
	}

	return nil
}

// UpdateScheduleRequest represents an update to a schedule
type UpdateScheduleRequest struct {
	IsActive      *bool           `json:"isActive,omitempty"`
	ScheduledTime *time.Time      `json:"scheduledTime,omitempty"`
	Input         json.RawMessage `json:"input,omitempty"`
}

// UpdateSchedule updates an existing schedule
func (s *Scheduler) UpdateSchedule(scheduleID string, updateReq UpdateScheduleRequest) (*AppSchedule, error) {
	s.schedulesMutex.Lock()
	schedule, exists := s.schedules[scheduleID]
	if !exists {
		s.schedulesMutex.Unlock()
		return nil, fmt.Errorf("schedule not found")
	}

	// Update fields
	updated := false
	if updateReq.IsActive != nil {
		schedule.IsActive = *updateReq.IsActive
		updated = true
	}
	if updateReq.ScheduledTime != nil {
		schedule.ScheduledTime = *updateReq.ScheduledTime
		// Recalculate next run
		schedule.NextRun = CalculateNextRun(schedule)
		updated = true
	}
	if updateReq.Input != nil {
		schedule.Input = updateReq.Input
		updated = true
	}
	s.schedulesMutex.Unlock()

	if !updated {
		return nil, fmt.Errorf("no fields to update")
	}

	// Update in repository
	if err := s.repository.UpdateSchedule(schedule); err != nil {
		return nil, fmt.Errorf("failed to update schedule in database: %v", err)
	}

	return schedule, nil
}

// Backward compatibility functions - delegate to default scheduler

// InitDefaultScheduler initializes the default scheduler with the given repository
func InitDefaultScheduler(repository ScheduleRepositoryInterface, logger logging.Logger) {
	defaultScheduler = NewScheduler(repository, logger)
}

// CreateSchedule creates and stores a new schedule (backward compatibility)
func CreateSchedule(req ScheduleRequest) (*AppSchedule, error) {
	if defaultScheduler == nil {
		return nil, fmt.Errorf("default scheduler not initialized")
	}
	return defaultScheduler.CreateSchedule(req)
}

// GetAllSchedules returns all schedules, optionally filtered by app ID (backward compatibility)
func GetAllSchedules(appIdFilter string) []*AppSchedule {
	if defaultScheduler == nil {
		return []*AppSchedule{}
	}
	return defaultScheduler.GetAllSchedules(appIdFilter)
}

// GetSchedule returns a specific schedule by ID (backward compatibility)
func GetSchedule(scheduleID string) (*AppSchedule, bool) {
	if defaultScheduler == nil {
		return nil, false
	}
	return defaultScheduler.GetSchedule(scheduleID)
}

// DeleteSchedule deactivates and removes a schedule (backward compatibility)
func DeleteSchedule(scheduleID string) error {
	if defaultScheduler == nil {
		return fmt.Errorf("default scheduler not initialized")
	}
	return defaultScheduler.DeleteSchedule(scheduleID)
}

// UpdateSchedule updates an existing schedule (backward compatibility)
func UpdateSchedule(scheduleID string, updateReq UpdateScheduleRequest) (*AppSchedule, error) {
	if defaultScheduler == nil {
		return nil, fmt.Errorf("default scheduler not initialized")
	}
	return defaultScheduler.UpdateSchedule(scheduleID, updateReq)
}
