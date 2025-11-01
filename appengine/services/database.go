package services

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"

	"arcadia/pkg/logging"

	_ "github.com/mattn/go-sqlite3"
)

// Database interface defines generic database operations
type Database interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Exec(stmt string, args ...interface{}) (sql.Result, error)
	Close() error
}

// SQLiteDatabase implements the Database interface for SQLite
type SQLiteDatabase struct {
	db     *sql.DB
	mutex  sync.RWMutex
	logger logging.Logger
}

// NewSQLiteDatabase creates a new SQLite database instance
func NewSQLiteDatabase(filepath string, logger logging.Logger) (*SQLiteDatabase, error) {
	ctx := context.Background()

	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		if logger != nil {
			logger.Warn(ctx, "Failed to enable WAL mode", "error", err, "filepath", filepath)
		}
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		if logger != nil {
			logger.Warn(ctx, "Failed to enable foreign keys", "error", err, "filepath", filepath)
		}
	}

	return &SQLiteDatabase{
		db:     db,
		logger: logger,
	}, nil
}

// Query executes a query that returns rows
func (s *SQLiteDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.db.Query(query, args...)
}

// QueryRow executes a query that is expected to return at most one row
func (s *SQLiteDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.db.QueryRow(query, args...)
}

// Exec executes a query without returning any rows
func (s *SQLiteDatabase) Exec(stmt string, args ...interface{}) (sql.Result, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.db.Exec(stmt, args...)
}

// Close closes the database connection
func (s *SQLiteDatabase) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		return err
	}
	return nil
}

// DatabaseManager manages both app and system database connections
type DatabaseManager struct {
	appDB    Database
	systemDB Database
	logger   logging.Logger
}

// NewDatabaseManager creates a new database manager
func NewDatabaseManager(logger logging.Logger) *DatabaseManager {
	return &DatabaseManager{
		logger: logger,
	}
}

// GetAppDB returns the app database instance
func (dm *DatabaseManager) GetAppDB() Database {
	return dm.appDB
}

// GetSystemDB returns the system database instance
func (dm *DatabaseManager) GetSystemDB() Database {
	return dm.systemDB
}

// Initialize initializes both app and system databases
func (dm *DatabaseManager) Initialize() error {
	ctx := context.Background()

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

	if dm.logger != nil {
		dm.logger.Info(ctx, "Databases initialized successfully")
	}
	return nil
}

func (dm *DatabaseManager) initAppDatabase() error {
	ctx := context.Background()
	var err error
	dm.appDB, err = NewSQLiteDatabase("data/app_data.db", dm.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize app database: %v", err)
	}

	// Create default tables for app data storage
	if err := dm.createAppTables(); err != nil {
		return fmt.Errorf("failed to create app tables: %v", err)
	}

	if dm.logger != nil {
		dm.logger.Info(ctx, "App database initialized successfully", "db_path", "data/app_data.db")
	}
	return nil
}

func (dm *DatabaseManager) initSystemDatabase() error {
	ctx := context.Background()
	var err error
	dm.systemDB, err = NewSQLiteDatabase("data/system.db", dm.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize system database: %v", err)
	}

	// Create system tables
	if err := dm.createSystemTables(); err != nil {
		return fmt.Errorf("failed to create system tables: %v", err)
	}

	if dm.logger != nil {
		dm.logger.Info(ctx, "System database initialized successfully", "db_path", "data/system.db")
	}
	return nil
}

func (dm *DatabaseManager) createAppTables() error {
	// Currently no app-specific tables are needed
	// Tables are created as needed by the application
	return nil
}

func (dm *DatabaseManager) createSystemTables() error {
	// System tables are now created by the schedule repository when needed
	// This keeps database concerns separate from business logic
	return nil
}

// Close closes both database connections
func (dm *DatabaseManager) Close() {
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
func InitDatabasesWithManager(logger logging.Logger) (*DatabaseManager, error) {
	dm := NewDatabaseManager(logger)
	if err := dm.Initialize(); err != nil {
		return nil, err
	}

	// Initialize the schedule repository with the system database
	InitScheduleRepository(dm.GetSystemDB(), logger)

	// Initialize the default scheduler with the schedule repository
	if repo := GetScheduleRepository(); repo != nil {
		InitDefaultScheduler(repo, logger)
	}

	// Initialize the queue repository with the system database
	InitQueueRepository(dm.GetSystemDB(), logger)

	// Initialize the default queue with the queue repository
	if queueRepo := GetQueueRepository(); queueRepo != nil {
		InitDefaultQueue(queueRepo)
	}

	// Legacy vector and document store initialization removed - using documents module

	return dm, nil
}


