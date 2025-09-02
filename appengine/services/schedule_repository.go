package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// ScheduleRepository handles persistence of schedules
type ScheduleRepository struct {
	db    Database
	mutex sync.RWMutex
}

// NewScheduleRepository creates a new schedule repository
func NewScheduleRepository(db Database) *ScheduleRepository {
	repo := &ScheduleRepository{
		db: db,
	}
	// Initialize tables
	repo.initializeTables()
	return repo
}

// initializeTables creates the necessary tables if they don't exist
func (r *ScheduleRepository) initializeTables() {
	if r.db == nil {
		return // Skip table creation if database is nil
	}
	
	queries := []string{
		// App schedules table
		`CREATE TABLE IF NOT EXISTS app_schedules (
			id TEXT PRIMARY KEY,
			app_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			input_data TEXT,
			schedule_type TEXT NOT NULL,
			scheduled_time DATETIME NOT NULL,
			recurrence_rule TEXT,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// Scheduled runs table - tracks execution history
		`CREATE TABLE IF NOT EXISTS scheduled_runs (
			id TEXT PRIMARY KEY,
			schedule_id TEXT NOT NULL,
			app_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			input_data TEXT,
			started_at DATETIME NOT NULL,
			completed_at DATETIME,
			status TEXT DEFAULT 'pending',
			output TEXT,
			error TEXT,
			FOREIGN KEY (schedule_id) REFERENCES app_schedules(id)
		);`,
	}

	for _, query := range queries {
		if _, err := r.db.Exec(query); err != nil {
			log.Printf("Warning: failed to create schedule table: %v", err)
		}
	}
}

// SaveSchedule saves a schedule to the database
func (r *ScheduleRepository) SaveSchedule(schedule *AppSchedule) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `INSERT OR REPLACE INTO app_schedules 
		(id, app_id, tool_name, input_data, schedule_type, scheduled_time, recurrence_rule, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`

	var inputData string
	if schedule.Input != nil {
		inputDataBytes, err := json.Marshal(schedule.Input)
		if err != nil {
			return fmt.Errorf("failed to marshal input data: %v", err)
		}
		inputData = string(inputDataBytes)
	}

	var recurrencePattern string
	if schedule.Recurrence != nil {
		recurrenceBytes, err := json.Marshal(schedule.Recurrence)
		if err != nil {
			return fmt.Errorf("failed to marshal recurrence pattern: %v", err)
		}
		recurrencePattern = string(recurrenceBytes)
	}

	_, err := r.db.Exec(query, schedule.ID, schedule.AppID, schedule.ToolName, inputData,
		schedule.ScheduleType, schedule.ScheduledTime, recurrencePattern, schedule.IsActive)
	if err != nil {
		return fmt.Errorf("failed to save schedule to database: %v", err)
	}

	return nil
}

// LoadSchedules loads all active schedules from the database
func (r *ScheduleRepository) LoadSchedules() (map[string]*AppSchedule, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, app_id, tool_name, input_data, schedule_type, scheduled_time, 
		recurrence_rule, is_active FROM app_schedules WHERE is_active = 1`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to load schedules from database: %v", err)
	}
	defer rows.Close()

	schedules := make(map[string]*AppSchedule)
	for rows.Next() {
		var schedule AppSchedule
		var inputData, recurrencePattern sql.NullString
		var scheduledTimeStr string

		err := rows.Scan(&schedule.ID, &schedule.AppID, &schedule.ToolName, &inputData,
			&schedule.ScheduleType, &scheduledTimeStr, &recurrencePattern, &schedule.IsActive)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule row: %v", err)
		}

		// Parse the scheduled time string
		scheduledTime, err := time.Parse(time.RFC3339, scheduledTimeStr)
		if err != nil {
			// Try alternative formats if RFC3339 fails
			formats := []string{
				"2006-01-02 15:04:05",
				"2006-01-02T15:04:05",
				"2006-01-02 15:04:05.999999999-07:00",
				"2006-01-02T15:04:05.999999999Z07:00",
			}
			var parseErr error
			for _, format := range formats {
				if scheduledTime, parseErr = time.Parse(format, scheduledTimeStr); parseErr == nil {
					break
				}
			}
			if parseErr != nil {
				log.Printf("Warning: failed to parse scheduled time '%s' for schedule %s: %v", scheduledTimeStr, schedule.ID, err)
				continue // Skip this schedule
			}
		}
		schedule.ScheduledTime = scheduledTime

		// Unmarshal input data
		if inputData.Valid && inputData.String != "" {
			if err := json.Unmarshal([]byte(inputData.String), &schedule.Input); err != nil {
				log.Printf("Warning: failed to unmarshal input data for schedule %s: %v", schedule.ID, err)
			}
		}

		// Unmarshal recurrence pattern
		if recurrencePattern.Valid && recurrencePattern.String != "" {
			var recurrence RecurrenceRule
			if err := json.Unmarshal([]byte(recurrencePattern.String), &recurrence); err != nil {
				log.Printf("Warning: failed to unmarshal recurrence pattern for schedule %s: %v", schedule.ID, err)
			} else {
				schedule.Recurrence = &recurrence
			}
		}

		schedules[schedule.ID] = &schedule
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating schedule rows: %v", err)
	}

	return schedules, nil
}

// UpdateSchedule updates an existing schedule in the database
func (r *ScheduleRepository) UpdateSchedule(schedule *AppSchedule) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `UPDATE app_schedules SET 
		app_id = ?, tool_name = ?, input_data = ?, schedule_type = ?, 
		scheduled_time = ?, recurrence_rule = ?, is_active = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`

	var inputData string
	if schedule.Input != nil {
		inputDataBytes, err := json.Marshal(schedule.Input)
		if err != nil {
			return fmt.Errorf("failed to marshal input data: %v", err)
		}
		inputData = string(inputDataBytes)
	}

	var recurrencePattern string
	if schedule.Recurrence != nil {
		recurrenceBytes, err := json.Marshal(schedule.Recurrence)
		if err != nil {
			return fmt.Errorf("failed to marshal recurrence pattern: %v", err)
		}
		recurrencePattern = string(recurrenceBytes)
	}

	_, err := r.db.Exec(query, schedule.AppID, schedule.ToolName, inputData, schedule.ScheduleType,
		schedule.ScheduledTime, recurrencePattern, schedule.IsActive, schedule.ID)
	if err != nil {
		return fmt.Errorf("failed to update schedule in database: %v", err)
	}

	return nil
}

// DeactivateSchedule deactivates a schedule in the database
func (r *ScheduleRepository) DeactivateSchedule(scheduleID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, err := r.db.Exec("UPDATE app_schedules SET is_active = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?", scheduleID)
	return err
}

// SaveScheduledRun saves a scheduled run to the database
func (r *ScheduleRepository) SaveScheduledRun(run *ScheduledRun) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `INSERT OR REPLACE INTO scheduled_runs 
		(id, schedule_id, app_id, tool_name, input_data, started_at, completed_at, status, output, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var inputData string
	if run.Input != nil {
		inputDataBytes, err := json.Marshal(run.Input)
		if err != nil {
			return fmt.Errorf("failed to marshal input data: %v", err)
		}
		inputData = string(inputDataBytes)
	}

	_, err := r.db.Exec(query, run.ID, run.ScheduleID, run.AppID, run.ToolName, inputData,
		run.StartedAt, nullableTime(run.CompletedAt), run.Status, run.Output, run.Error)
	if err != nil {
		return fmt.Errorf("failed to save scheduled run to database: %v", err)
	}

	return nil
}

// UpdateScheduledRun updates a scheduled run in the database
func (r *ScheduleRepository) UpdateScheduledRun(run *ScheduledRun) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `UPDATE scheduled_runs SET 
		started_at = ?, completed_at = ?, status = ?, output = ?, error = ?
		WHERE id = ?`

	_, err := r.db.Exec(query, run.StartedAt, nullableTime(run.CompletedAt),
		run.Status, run.Output, run.Error, run.ID)
	if err != nil {
		return fmt.Errorf("failed to update scheduled run in database: %v", err)
	}

	return nil
}

func nullableTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}

// Global repository instance for backward compatibility
var defaultScheduleRepository *ScheduleRepository

// InitScheduleRepository initializes the default schedule repository
func InitScheduleRepository(db Database) {
	defaultScheduleRepository = NewScheduleRepository(db)
}

// GetScheduleRepository returns the default schedule repository
func GetScheduleRepository() *ScheduleRepository {
	return defaultScheduleRepository
}

// Mock implementations for testing
type MockScheduleRepository struct {
	Schedules        map[string]*AppSchedule
	ScheduledRuns    map[string]*ScheduledRun
	ShouldError      bool
	SaveError        error
	LoadError        error
	SaveRunError     error
	UpdateError      error
	UpdateRunError   error
	DeactivateError  error
}

func NewMockScheduleRepository() *MockScheduleRepository {
	return &MockScheduleRepository{
		Schedules:     make(map[string]*AppSchedule),
		ScheduledRuns: make(map[string]*ScheduledRun),
		ShouldError:   false,
	}
}

func (m *MockScheduleRepository) SaveSchedule(schedule *AppSchedule) error {
	if m.SaveError != nil {
		return m.SaveError
	}
	if m.ShouldError {
		return fmt.Errorf("mock error")
	}
	m.Schedules[schedule.ID] = schedule
	return nil
}

func (m *MockScheduleRepository) LoadSchedules() (map[string]*AppSchedule, error) {
	if m.LoadError != nil {
		return nil, m.LoadError
	}
	if m.ShouldError {
		return nil, fmt.Errorf("mock error")
	}
	// Filter to only return active schedules (like the real implementation)
	activeSchedules := make(map[string]*AppSchedule)
	for id, schedule := range m.Schedules {
		if schedule.IsActive {
			activeSchedules[id] = schedule
		}
	}
	return activeSchedules, nil
}

func (m *MockScheduleRepository) UpdateSchedule(schedule *AppSchedule) error {
	if m.UpdateError != nil {
		return m.UpdateError
	}
	if m.ShouldError {
		return fmt.Errorf("mock error")
	}
	if _, exists := m.Schedules[schedule.ID]; !exists {
		return fmt.Errorf("schedule not found")
	}
	m.Schedules[schedule.ID] = schedule
	return nil
}

func (m *MockScheduleRepository) DeactivateSchedule(scheduleID string) error {
	if m.DeactivateError != nil {
		return m.DeactivateError
	}
	if m.ShouldError {
		return fmt.Errorf("mock error")
	}
	if schedule, exists := m.Schedules[scheduleID]; exists {
		schedule.IsActive = false
	}
	return nil
}

func (m *MockScheduleRepository) SaveScheduledRun(run *ScheduledRun) error {
	if m.SaveRunError != nil {
		return m.SaveRunError
	}
	if m.ShouldError {
		return fmt.Errorf("mock error")
	}
	m.ScheduledRuns[run.ID] = run
	return nil
}

func (m *MockScheduleRepository) UpdateScheduledRun(run *ScheduledRun) error {
	if m.UpdateRunError != nil {
		return m.UpdateRunError
	}
	if m.ShouldError {
		return fmt.Errorf("mock error")
	}
	if _, exists := m.ScheduledRuns[run.ID]; !exists {
		return fmt.Errorf("scheduled run not found")
	}
	m.ScheduledRuns[run.ID] = run
	return nil
}

// Legacy compatibility - create a mock that conforms to the old SystemDB interface
type MockSystemDB struct {
	*MockScheduleRepository
}

func NewMockSystemDB() *MockSystemDB {
	return &MockSystemDB{
		MockScheduleRepository: NewMockScheduleRepository(),
	}
}