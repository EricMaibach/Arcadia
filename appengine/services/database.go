package services

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// Database interface defines generic database operations
type Database interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	Exec(stmt string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Close() error
}

// SQLiteDatabase implements the Database interface for SQLite
type SQLiteDatabase struct {
	db    *sql.DB
	mutex sync.RWMutex
}

// NewSQLiteDatabase creates a new SQLite database instance
func NewSQLiteDatabase(filepath string) (*SQLiteDatabase, error) {
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
		log.Printf("Warning: failed to enable WAL mode: %v", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		log.Printf("Warning: failed to enable foreign keys: %v", err)
	}

	return &SQLiteDatabase{
		db: db,
	}, nil
}

// Query executes a query that returns rows
func (s *SQLiteDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.db.Query(query, args...)
}

// Exec executes a query without returning any rows
func (s *SQLiteDatabase) Exec(stmt string, args ...interface{}) (sql.Result, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.db.Exec(stmt, args...)
}

// QueryRow executes a query that is expected to return at most one row
func (s *SQLiteDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.db.QueryRow(query, args...)
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
}

// NewDatabaseManager creates a new database manager
func NewDatabaseManager() *DatabaseManager {
	return &DatabaseManager{}
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
	dm.appDB, err = NewSQLiteDatabase("data/app_data.db")
	if err != nil {
		return fmt.Errorf("failed to initialize app database: %v", err)
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
	dm.systemDB, err = NewSQLiteDatabase("data/system.db")
	if err != nil {
		return fmt.Errorf("failed to initialize system database: %v", err)
	}

	// Create system tables
	if err := dm.createSystemTables(); err != nil {
		return fmt.Errorf("failed to create system tables: %v", err)
	}

	log.Println("System database initialized successfully")
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
func InitDatabasesWithManager() (*DatabaseManager, error) {
	dm := NewDatabaseManager()
	if err := dm.Initialize(); err != nil {
		return nil, err
	}
	
	// Initialize the schedule repository with the system database
	InitScheduleRepository(dm.GetSystemDB())
	
	// Initialize the default scheduler with the schedule repository
	if repo := GetScheduleRepository(); repo != nil {
		InitDefaultScheduler(repo)
	}
	
	return dm, nil
}





