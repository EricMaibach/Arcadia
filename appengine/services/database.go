package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/bytecodealliance/wasmtime-go"
	_ "github.com/mattn/go-sqlite3"
)

// Missing type definitions that were lost
type ScheduleType string

const (
	ScheduleTypeOneTime   ScheduleType = "one-time"
	ScheduleTypeRecurring ScheduleType = "recurring"
)

type RecurrenceRule struct {
	Interval    int       `json:"interval"`
	Unit        string    `json:"unit"` // "minutes", "hours", "days", "weeks", "months"
	DaysOfWeek  []int     `json:"daysOfWeek,omitempty"`
	EndDate     *time.Time `json:"endDate,omitempty"`
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

// Database interfaces for dependency injection
type SystemDB interface {
	SaveSchedule(schedule *AppSchedule) error
	LoadSchedules() (map[string]*AppSchedule, error)
	UpdateSchedule(schedule *AppSchedule) error
	DeactivateSchedule(scheduleID string) error
	SaveScheduledRun(run *ScheduledRun) error
	UpdateScheduledRun(run *ScheduledRun) error
}

type AppDB interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	Exec(stmt string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// DatabaseManager manages both app and system database connections
type DatabaseManager struct {
	appDB    *sql.DB
	systemDB *sql.DB
	appMutex    sync.RWMutex
	systemMutex sync.RWMutex
}

// NewDatabaseManager creates a new database manager
func NewDatabaseManager() *DatabaseManager {
	return &DatabaseManager{}
}

// SystemDB implementation for DatabaseManager
func (dm *DatabaseManager) SaveSchedule(schedule *AppSchedule) error {
	dm.systemMutex.Lock()
	defer dm.systemMutex.Unlock()

	if dm.systemDB == nil {
		return fmt.Errorf("system database not initialized")
	}

	return saveScheduleToDatabase(dm.systemDB, schedule)
}

func (dm *DatabaseManager) LoadSchedules() (map[string]*AppSchedule, error) {
	dm.systemMutex.RLock()
	defer dm.systemMutex.RUnlock()

	if dm.systemDB == nil {
		return nil, fmt.Errorf("system database not initialized")
	}

	return loadSchedulesFromDatabase(dm.systemDB)
}

func (dm *DatabaseManager) UpdateSchedule(schedule *AppSchedule) error {
	dm.systemMutex.Lock()
	defer dm.systemMutex.Unlock()

	if dm.systemDB == nil {
		return fmt.Errorf("system database not initialized")
	}

	return updateScheduleInDatabase(dm.systemDB, schedule)
}

func (dm *DatabaseManager) DeactivateSchedule(scheduleID string) error {
	dm.systemMutex.Lock()
	defer dm.systemMutex.Unlock()

	if dm.systemDB == nil {
		return fmt.Errorf("system database not initialized")
	}

	return deactivateScheduleInDatabase(dm.systemDB, scheduleID)
}

func (dm *DatabaseManager) SaveScheduledRun(run *ScheduledRun) error {
	dm.systemMutex.Lock()
	defer dm.systemMutex.Unlock()

	if dm.systemDB == nil {
		return fmt.Errorf("system database not initialized")
	}

	return saveScheduledRunToDatabase(dm.systemDB, run)
}

func (dm *DatabaseManager) UpdateScheduledRun(run *ScheduledRun) error {
	dm.systemMutex.Lock()
	defer dm.systemMutex.Unlock()

	if dm.systemDB == nil {
		return fmt.Errorf("system database not initialized")
	}

	return updateScheduledRunInDatabase(dm.systemDB, run)
}

// AppDB implementation for DatabaseManager
func (dm *DatabaseManager) Query(query string, args ...interface{}) (*sql.Rows, error) {
	dm.appMutex.RLock()
	defer dm.appMutex.RUnlock()

	if dm.appDB == nil {
		return nil, fmt.Errorf("app database not initialized")
	}

	return dm.appDB.Query(query, args...)
}

func (dm *DatabaseManager) Exec(stmt string, args ...interface{}) (sql.Result, error) {
	dm.appMutex.Lock()
	defer dm.appMutex.Unlock()

	if dm.appDB == nil {
		return nil, fmt.Errorf("app database not initialized")
	}

	return dm.appDB.Exec(stmt, args...)
}

func (dm *DatabaseManager) QueryRow(query string, args ...interface{}) *sql.Row {
	dm.appMutex.RLock()
	defer dm.appMutex.RUnlock()

	if dm.appDB == nil {
		// Return a row that will return an error when scanned
		return (&sql.DB{}).QueryRow("SELECT 1 WHERE 0=1")
	}

	return dm.appDB.QueryRow(query, args...)
}

// Initialize initializes both app and system databases
func (dm *DatabaseManager) Initialize() error {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll("data", 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Initialize app database
	if err := dm.initAppDatabase(); err != nil {
		return fmt.Errorf("failed to initialize app database: %v", err)
	}

	// Initialize system database
	if err := dm.initSystemDatabase(); err != nil {
		return fmt.Errorf("failed to initialize system database: %v", err)
	}

	log.Println("Databases initialized successfully")
	return nil
}

func (dm *DatabaseManager) initAppDatabase() error {
	var err error

	// Open app SQLite database
	dm.appDB, err = sql.Open("sqlite3", "data/app_data.db")
	if err != nil {
		return fmt.Errorf("failed to open app database: %v", err)
	}

	// Test the connection
	if err := dm.appDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping app database: %v", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := dm.appDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Printf("Warning: failed to enable WAL mode on app database: %v", err)
	}

	// Enable foreign keys
	if _, err := dm.appDB.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		log.Printf("Warning: failed to enable foreign keys on app database: %v", err)
	}

	// Create default tables for app data storage
	if err := dm.createAppTables(); err != nil {
		return fmt.Errorf("failed to create app tables: %v", err)
	}

	log.Println("App database initialized successfully")
	return nil
}

func (dm *DatabaseManager) initSystemDatabase() error {
	var err error

	// Open system SQLite database
	dm.systemDB, err = sql.Open("sqlite3", "data/system.db")
	if err != nil {
		return fmt.Errorf("failed to open system database: %v", err)
	}

	// Test the connection
	if err := dm.systemDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping system database: %v", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := dm.systemDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Printf("Warning: failed to enable WAL mode on system database: %v", err)
	}

	// Enable foreign keys
	if _, err := dm.systemDB.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		log.Printf("Warning: failed to enable foreign keys on system database: %v", err)
	}

	// Create system tables
	if err := dm.createSystemTables(); err != nil {
		return fmt.Errorf("failed to create system tables: %v", err)
	}

	log.Println("System database initialized successfully")
	return nil
}

func (dm *DatabaseManager) createAppTables() error {
	queries := []string{
		// App data table - for general key-value storage per app
		`CREATE TABLE IF NOT EXISTS app_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_id TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(app_id, key)
		);`,

		// App logs table - for application logging
		`CREATE TABLE IF NOT EXISTS app_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_id TEXT NOT NULL,
			level TEXT NOT NULL,
			message TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// App metrics table - for performance monitoring
		`CREATE TABLE IF NOT EXISTS app_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_id TEXT NOT NULL,
			metric_name TEXT NOT NULL,
			metric_value REAL NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, query := range queries {
		if _, err := dm.appDB.Exec(query); err != nil {
			return fmt.Errorf("failed to create app table: %v", err)
		}
	}

	return nil
}

func (dm *DatabaseManager) createSystemTables() error {
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
			error_message TEXT,
			FOREIGN KEY (schedule_id) REFERENCES app_schedules(id)
		);`,
	}

	for _, query := range queries {
		if _, err := dm.systemDB.Exec(query); err != nil {
			return fmt.Errorf("failed to create system table: %v", err)
		}
	}

	return nil
}

// Close closes both database connections
func (dm *DatabaseManager) Close() {
	dm.appMutex.Lock()
	dm.systemMutex.Lock()
	defer dm.appMutex.Unlock()
	defer dm.systemMutex.Unlock()

	if dm.appDB != nil {
		dm.appDB.Close()
		dm.appDB = nil
	}
	if dm.systemDB != nil {
		dm.systemDB.Close()
		dm.systemDB = nil
	}
}

// Helper functions for database operations (reconstructed based on usage)
func saveScheduleToDatabase(db *sql.DB, schedule *AppSchedule) error {
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

	_, err := db.Exec(query, schedule.ID, schedule.AppID, schedule.ToolName, inputData, 
		schedule.ScheduleType, schedule.ScheduledTime, recurrencePattern, schedule.IsActive)
	if err != nil {
		return fmt.Errorf("failed to save schedule to database: %v", err)
	}

	return nil
}

func loadSchedulesFromDatabase(db *sql.DB) (map[string]*AppSchedule, error) {
	query := `SELECT id, app_id, tool_name, input_data, schedule_type, scheduled_time, 
		recurrence_rule, is_active FROM app_schedules WHERE is_active = 1`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to load schedules from database: %v", err)
	}
	defer rows.Close()

	schedules := make(map[string]*AppSchedule)
	for rows.Next() {
		var schedule AppSchedule
		var inputData, recurrencePattern sql.NullString
		var scheduledTime time.Time

		err := rows.Scan(&schedule.ID, &schedule.AppID, &schedule.ToolName, &inputData,
			&schedule.ScheduleType, &scheduledTime, &recurrencePattern, &schedule.IsActive)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule row: %v", err)
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

func updateScheduleInDatabase(db *sql.DB, schedule *AppSchedule) error {
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

	_, err := db.Exec(query, schedule.AppID, schedule.ToolName, inputData, schedule.ScheduleType,
		schedule.ScheduledTime, recurrencePattern, schedule.IsActive, schedule.ID)
	if err != nil {
		return fmt.Errorf("failed to update schedule in database: %v", err)
	}

	return nil
}

func deactivateScheduleInDatabase(db *sql.DB, scheduleID string) error {
	_, err := db.Exec("UPDATE app_schedules SET is_active = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?", scheduleID)
	return err
}

func saveScheduledRunToDatabase(db *sql.DB, run *ScheduledRun) error {
	query := `INSERT OR REPLACE INTO scheduled_runs 
		(id, schedule_id, app_id, tool_name, input_data, started_at, completed_at, status, output, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var inputData string
	if run.Input != nil {
		inputDataBytes, err := json.Marshal(run.Input)
		if err != nil {
			return fmt.Errorf("failed to marshal input data: %v", err)
		}
		inputData = string(inputDataBytes)
	}

	_, err := db.Exec(query, run.ID, run.ScheduleID, run.AppID, run.ToolName, inputData,
		run.StartedAt, nullableTime(run.CompletedAt), run.Status, run.Output, run.Error)
	if err != nil {
		return fmt.Errorf("failed to save scheduled run to database: %v", err)
	}

	return nil
}

func updateScheduledRunInDatabase(db *sql.DB, run *ScheduledRun) error {
	query := `UPDATE scheduled_runs SET 
		started_at = ?, completed_at = ?, status = ?, output = ?, error_message = ?
		WHERE id = ?`

	_, err := db.Exec(query, run.StartedAt, nullableTime(run.CompletedAt),
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

// InitDatabasesWithManager initializes databases using the new DatabaseManager and sets up scheduler
func InitDatabasesWithManager() (*DatabaseManager, error) {
	dm := NewDatabaseManager()
	if err := dm.Initialize(); err != nil {
		return nil, err
	}

	// Initialize the default scheduler with the new database interfaces
	InitDefaultScheduler(dm)
	
	return dm, nil
}

// Legacy functions - kept for backward compatibility
func InitDatabases() error {
	log.Println("WARNING: InitDatabases() is deprecated. Use InitDatabasesWithManager() instead.")
	// For backward compatibility, create a DatabaseManager
	dm, err := InitDatabasesWithManager()
	if err != nil {
		return err
	}
	_ = dm // Keep reference so it doesn't get garbage collected
	return nil
}

func GetAppDB() *sql.DB {
	log.Println("WARNING: GetAppDB() is deprecated. Use DatabaseManager instead.")
	return nil
}

func GetSystemDB() *sql.DB {
	log.Println("WARNING: GetSystemDB() is deprecated. Use DatabaseManager instead.")
	return nil
}

// Legacy WASM host functions - deprecated but kept for compatibility
func DbQuery(caller *wasmtime.Caller, queryPtr, queryLen, resultPtrPtr int32) int32 {
	log.Printf("WARNING: DbQuery() is deprecated. Use WasmRuntime.dbQuery() instead.")
	return -1
}

func DbExec(caller *wasmtime.Caller, stmtPtr, stmtLen int32) int32 {
	log.Printf("WARNING: DbExec() is deprecated. Use WasmRuntime.dbExec() instead.")
	return -1
}

func DbPreparedQuery(caller *wasmtime.Caller, stmtPtr, stmtLen, paramsPtr, paramsLen, resultPtrPtr int32) int32 {
	log.Printf("WARNING: DbPreparedQuery() is deprecated. Use WasmRuntime.dbPreparedQuery() instead.")
	return -1
}

// Mock implementations for testing
type MockSystemDB struct {
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

func NewMockSystemDB() *MockSystemDB {
	return &MockSystemDB{
		Schedules:     make(map[string]*AppSchedule),
		ScheduledRuns: make(map[string]*ScheduledRun),
		ShouldError:   false,
	}
}

func (m *MockSystemDB) SaveSchedule(schedule *AppSchedule) error {
	if m.SaveError != nil {
		return m.SaveError
	}
	if m.ShouldError {
		return fmt.Errorf("mock error")
	}
	m.Schedules[schedule.ID] = schedule
	return nil
}

func (m *MockSystemDB) LoadSchedules() (map[string]*AppSchedule, error) {
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

func (m *MockSystemDB) UpdateSchedule(schedule *AppSchedule) error {
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

func (m *MockSystemDB) DeactivateSchedule(scheduleID string) error {
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

func (m *MockSystemDB) SaveScheduledRun(run *ScheduledRun) error {
	if m.SaveRunError != nil {
		return m.SaveRunError
	}
	if m.ShouldError {
		return fmt.Errorf("mock error")
	}
	m.ScheduledRuns[run.ID] = run
	return nil
}

func (m *MockSystemDB) UpdateScheduledRun(run *ScheduledRun) error {
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

// Mock App Database for testing
type MockAppDB struct {
	ShouldError bool
	QueryError  error
	ExecError   error
}

func NewMockAppDB() *MockAppDB {
	return &MockAppDB{}
}

func (m *MockAppDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if m.QueryError != nil {
		return nil, m.QueryError
	}
	if m.ShouldError {
		return nil, fmt.Errorf("mock query error")
	}
	// Return nil rows - in real tests you'd need more sophisticated mocking
	return nil, nil
}

func (m *MockAppDB) Exec(stmt string, args ...interface{}) (sql.Result, error) {
	if m.ExecError != nil {
		return nil, m.ExecError
	}
	if m.ShouldError {
		return nil, fmt.Errorf("mock exec error")
	}
	return &MockResult{RowsAffectedCount: 1, LastInsertIdVal: 1}, nil
}

func (m *MockAppDB) QueryRow(query string, args ...interface{}) *sql.Row {
	// Return nil - MockRow is too complex to implement properly
	// In real tests, you'd use a proper mocking library
	return nil
}

// Mock SQL types for testing
type MockResult struct {
	RowsAffectedCount   int64
	LastInsertIdVal     int64
	LastInsertIdError   error
	RowsAffectedError   error
}

func (m *MockResult) LastInsertId() (int64, error) {
	if m.LastInsertIdError != nil {
		return 0, m.LastInsertIdError
	}
	return m.LastInsertIdVal, nil
}

func (m *MockResult) RowsAffected() (int64, error) {
	if m.RowsAffectedError != nil {
		return 0, m.RowsAffectedError
	}
	return m.RowsAffectedCount, nil
}

type MockRow struct {
	ScanFunc func(dest ...interface{}) error
	ScanErr  error
}

func (m *MockRow) Scan(dest ...interface{}) error {
	if m.ScanFunc != nil {
		return m.ScanFunc(dest...)
	}
	if m.ScanErr != nil {
		return m.ScanErr
	}
	return nil
}