package models

import (
	"encoding/json"
	"time"
)

// RecurrenceRule defines rules for recurring schedules
type RecurrenceRule struct {
	Interval   int        `json:"interval"`
	Unit       string     `json:"unit"` // "minutes", "hours", "days", "weeks", "months"
	DaysOfWeek []int      `json:"daysOfWeek,omitempty"`
	EndDate    *time.Time `json:"endDate,omitempty"`
}

// ScheduleRequest represents a request to schedule an application run
type ScheduleRequest struct {
	AppID         string                 `json:"appId"`
	ToolName      string                 `json:"toolName"`
	Input         json.RawMessage        `json:"input"`
	ScheduleType  string                 `json:"scheduleType"`
	ScheduledTime FlexTime               `json:"scheduledTime"`
	Recurrence    *RecurrenceRule        `json:"recurrence,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Description   string                 `json:"description,omitempty"`
	Enabled       bool                   `json:"enabled"`
	Timezone      string                 `json:"timezone,omitempty"`
}

// Schedule represents a scheduled task
type Schedule struct {
	ID            string                 `json:"id"`
	AppID         string                 `json:"appId"`
	ToolName      string                 `json:"toolName"`
	Input         json.RawMessage        `json:"input"`
	ScheduleType  string                 `json:"scheduleType"`
	ScheduledTime time.Time              `json:"scheduledTime"`
	Recurrence    *RecurrenceRule        `json:"recurrence,omitempty"`
	NextRun       time.Time              `json:"next_run"`
	LastRun       *time.Time             `json:"last_run,omitempty"`
	RunCount      int                    `json:"run_count"`
	Status        string                 `json:"status"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Description   string                 `json:"description,omitempty"`
	Enabled       bool                   `json:"enabled"`
	Timezone      string                 `json:"timezone,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// ScheduleExecution represents the execution of a scheduled task
type ScheduleExecution struct {
	ID         string                 `json:"id"`
	ScheduleID string                 `json:"schedule_id"`
	AppID      string                 `json:"app_id"`
	ToolName   string                 `json:"tool_name"`
	Input      json.RawMessage        `json:"input"`
	Output     string                 `json:"output,omitempty"`
	Status     string                 `json:"status"`
	Error      string                 `json:"error,omitempty"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    *time.Time             `json:"end_time,omitempty"`
	Duration   float64                `json:"duration_ms,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Schedule type constants
const (
	ScheduleTypeOneTime   = "one-time"
	ScheduleTypeRecurring = "recurring"
)

// Schedule status constants
const (
	ScheduleStatusActive    = "active"
	ScheduleStatusInactive  = "inactive"
	ScheduleStatusCompleted = "completed"
	ScheduleStatusFailed    = "failed"
	ScheduleStatusCancelled = "cancelled"
)

// Execution status constants
const (
	ExecutionStatusPending   = "pending"
	ExecutionStatusRunning   = "running"
	ExecutionStatusCompleted = "completed"
	ExecutionStatusFailed    = "failed"
	ExecutionStatusCancelled = "cancelled"
	ExecutionStatusTimeout   = "timeout"
)

// Recurrence unit constants
const (
	RecurrenceUnitMinutes = "minutes"
	RecurrenceUnitHours   = "hours"
	RecurrenceUnitDays    = "days"
	RecurrenceUnitWeeks   = "weeks"
	RecurrenceUnitMonths  = "months"
)

// ScheduleStats represents statistics about schedules
type ScheduleStats struct {
	TotalSchedules   int        `json:"total_schedules"`
	ActiveSchedules  int        `json:"active_schedules"`
	CompletedToday   int        `json:"completed_today"`
	FailedToday      int        `json:"failed_today"`
	AverageExecution float64    `json:"average_execution_ms"`
	SuccessRate      float64    `json:"success_rate"`
	NextExecution    *time.Time `json:"next_execution,omitempty"`
}

// ScheduleFilter represents filters for querying schedules
type ScheduleFilter struct {
	AppID         string     `json:"app_id,omitempty"`
	ToolName      string     `json:"tool_name,omitempty"`
	Status        string     `json:"status,omitempty"`
	ScheduleType  string     `json:"schedule_type,omitempty"`
	Enabled       *bool      `json:"enabled,omitempty"`
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`
	NextRunAfter  *time.Time `json:"next_run_after,omitempty"`
	NextRunBefore *time.Time `json:"next_run_before,omitempty"`
	Limit         int        `json:"limit,omitempty"`
	Offset        int        `json:"offset,omitempty"`
}

// ScheduleUpdate represents updates to a schedule
type ScheduleUpdate struct {
	ScheduledTime *time.Time             `json:"scheduled_time,omitempty"`
	Recurrence    *RecurrenceRule        `json:"recurrence,omitempty"`
	Enabled       *bool                  `json:"enabled,omitempty"`
	Description   *string                `json:"description,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// Validate validates a schedule request
func (sr *ScheduleRequest) Validate() error {
	if sr.AppID == "" {
		return NewValidationError("appId", "required", "appId is required", sr.AppID)
	}

	if sr.ToolName == "" {
		return NewValidationError("toolName", "required", "toolName is required", sr.ToolName)
	}

	if sr.ScheduleType != ScheduleTypeOneTime && sr.ScheduleType != ScheduleTypeRecurring {
		return NewValidationError("scheduleType", "enum", "scheduleType must be 'one-time' or 'recurring'", sr.ScheduleType)
	}

	if sr.ScheduledTime.Time.IsZero() {
		return NewValidationError("scheduledTime", "required", "scheduledTime is required", sr.ScheduledTime)
	}

	if sr.ScheduledTime.Time.Before(time.Now()) {
		return NewValidationError("scheduledTime", "future", "scheduledTime must be in the future", sr.ScheduledTime)
	}

	if sr.ScheduleType == ScheduleTypeRecurring && sr.Recurrence == nil {
		return NewValidationError("recurrence", "required", "recurrence is required for recurring schedules", sr.Recurrence)
	}

	if sr.Recurrence != nil {
		if err := sr.Recurrence.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Validate validates a recurrence rule
func (rr *RecurrenceRule) Validate() error {
	if rr.Interval <= 0 {
		return NewValidationError("interval", "positive", "interval must be positive", rr.Interval)
	}

	validUnits := []string{RecurrenceUnitMinutes, RecurrenceUnitHours, RecurrenceUnitDays, RecurrenceUnitWeeks, RecurrenceUnitMonths}
	validUnit := false
	for _, unit := range validUnits {
		if rr.Unit == unit {
			validUnit = true
			break
		}
	}
	if !validUnit {
		return NewValidationError("unit", "enum", "invalid recurrence unit", rr.Unit)
	}

	if rr.Unit == RecurrenceUnitWeeks && len(rr.DaysOfWeek) == 0 {
		return NewValidationError("daysOfWeek", "required", "daysOfWeek is required for weekly recurrence", rr.DaysOfWeek)
	}

	for _, day := range rr.DaysOfWeek {
		if day < 0 || day > 6 {
			return NewValidationError("daysOfWeek", "range", "days of week must be 0-6", day)
		}
	}

	if rr.EndDate != nil && rr.EndDate.Before(time.Now()) {
		return NewValidationError("endDate", "future", "endDate must be in the future", rr.EndDate)
	}

	return nil
}

// CalculateNextRun calculates the next run time for a schedule
func (s *Schedule) CalculateNextRun() time.Time {
	if s.ScheduleType == ScheduleTypeOneTime {
		return s.ScheduledTime
	}

	if s.Recurrence == nil {
		return s.ScheduledTime
	}

	now := time.Now()
	next := s.ScheduledTime

	// If the scheduled time is in the past, start from the last run or current time
	if next.Before(now) {
		if s.LastRun != nil {
			next = *s.LastRun
		} else {
			next = now
		}
	}

	// Add the recurrence interval
	switch s.Recurrence.Unit {
	case RecurrenceUnitMinutes:
		next = next.Add(time.Duration(s.Recurrence.Interval) * time.Minute)
	case RecurrenceUnitHours:
		next = next.Add(time.Duration(s.Recurrence.Interval) * time.Hour)
	case RecurrenceUnitDays:
		next = next.AddDate(0, 0, s.Recurrence.Interval)
	case RecurrenceUnitWeeks:
		if len(s.Recurrence.DaysOfWeek) > 0 {
			// Find the next occurrence on one of the specified days
			for i := 0; i < 7*s.Recurrence.Interval; i++ {
				candidate := next.AddDate(0, 0, i)
				for _, day := range s.Recurrence.DaysOfWeek {
					if int(candidate.Weekday()) == day {
						next = candidate
						break
					}
				}
				if !next.Equal(s.ScheduledTime) {
					break
				}
			}
		} else {
			next = next.AddDate(0, 0, 7*s.Recurrence.Interval)
		}
	case RecurrenceUnitMonths:
		next = next.AddDate(0, s.Recurrence.Interval, 0)
	}

	// Check if we've exceeded the end date
	if s.Recurrence.EndDate != nil && next.After(*s.Recurrence.EndDate) {
		return time.Time{} // No more runs
	}

	return next
}

// IsExpired checks if a schedule is expired
func (s *Schedule) IsExpired() bool {
	if s.ScheduleType == ScheduleTypeOneTime {
		return s.LastRun != nil || time.Now().After(s.ScheduledTime)
	}

	if s.Recurrence != nil && s.Recurrence.EndDate != nil {
		return time.Now().After(*s.Recurrence.EndDate)
	}

	return false
}

// ShouldRun checks if a schedule should run now
func (s *Schedule) ShouldRun() bool {
	if !s.Enabled || s.IsExpired() {
		return false
	}

	return time.Now().After(s.NextRun) || time.Now().Equal(s.NextRun)
}

// UpdateNextRun updates the next run time for a schedule
func (s *Schedule) UpdateNextRun() {
	s.NextRun = s.CalculateNextRun()
	s.UpdatedAt = time.Now()
}

// MarkAsRun marks a schedule as having been run
func (s *Schedule) MarkAsRun(success bool) {
	now := time.Now()
	s.LastRun = &now
	s.RunCount++
	s.UpdatedAt = now

	if success {
		s.Status = ScheduleStatusActive
	} else {
		s.Status = ScheduleStatusFailed
	}

	// Update next run time for recurring schedules
	if s.ScheduleType == ScheduleTypeRecurring && !s.IsExpired() {
		s.UpdateNextRun()
	} else if s.ScheduleType == ScheduleTypeOneTime {
		s.Status = ScheduleStatusCompleted
	}
}
