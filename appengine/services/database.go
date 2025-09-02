package services

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)


// AppDB interface for application database operations
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

// GetSystemDB returns the system database connection
func (dm *DatabaseManager) GetSystemDB() *sql.DB {
	dm.systemMutex.RLock()
	defer dm.systemMutex.RUnlock()
	return dm.systemDB
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
			error TEXT,
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


// InitDatabasesWithManager initializes databases using the new DatabaseManager
func InitDatabasesWithManager() (*DatabaseManager, error) {
	dm := NewDatabaseManager()
	if err := dm.Initialize(); err != nil {
		return nil, err
	}
	
	// Initialize the schedule repository with the system database
	InitScheduleRepository(dm.systemDB)
	
	// Initialize the default scheduler with the schedule repository
	if repo := GetScheduleRepository(); repo != nil {
		InitDefaultScheduler(repo)
	}
	
	return dm, nil
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