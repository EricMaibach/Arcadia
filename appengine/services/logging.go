package services

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// AppLogger handles application logging functionality
type AppLogger struct {
	logger  *log.Logger
	logFile *os.File
}

// GeneralLogger handles general application logging for all log.Printf calls
type GeneralLogger struct {
	logger  *log.Logger
	logFile *os.File
	originalOutput io.Writer
}

// NewAppLogger creates a new application logger instance
func NewAppLogger() *AppLogger {
	return &AppLogger{}
}

// NewGeneralLogger creates a new general application logger instance
func NewGeneralLogger() *GeneralLogger {
	return &GeneralLogger{}
}

// Initialize sets up the application logger with file and stdout output
func (al *AppLogger) Initialize() error {
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Create log file with timestamp
	logFileName := fmt.Sprintf("logs/app_submissions_%s.log", time.Now().Format("2006-01-02"))
	var err error
	al.logFile, err = os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to create log file: %v", err)
	}

	// Create logger that writes to both file and stdout
	al.logger = log.New(io.MultiWriter(os.Stdout, al.logFile), "[APP_SUBMISSION] ", log.LstdFlags|log.Lmicroseconds)

	al.logger.Println("=== App Submission Logger Initialized ===")
	return nil
}

// Initialize sets up the general logger with file and stdout output
func (gl *GeneralLogger) Initialize() error {
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Create log file with timestamp for general application logs
	logFileName := fmt.Sprintf("logs/arcadia_%s.log", time.Now().Format("2006-01-02"))
	var err error
	gl.logFile, err = os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to create general log file: %v", err)
	}

	// Store the original output to restore later if needed
	gl.originalOutput = os.Stdout

	// Create a multi-writer that writes to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, gl.logFile)

	// Set the global log output to write to both file and console
	log.SetOutput(multiWriter)

	// Also create a logger instance for internal use
	gl.logger = log.New(multiWriter, "", log.LstdFlags|log.Lmicroseconds)

	gl.logger.Println("=== General Application Logger Initialized ===")
	return nil
}

// Close closes the log file and shuts down the logger
func (al *AppLogger) Close() {
	if al.logFile != nil && al.logger != nil {
		al.logger.Println("=== App Submission Logger Closing ===")
		al.logFile.Close()
		al.logFile = nil
		al.logger = nil
	}
}

// Close closes the general log file and restores original output
func (gl *GeneralLogger) Close() {
	if gl.logFile != nil && gl.logger != nil {
		gl.logger.Println("=== General Application Logger Closing ===")

		// Restore original output
		if gl.originalOutput != nil {
			log.SetOutput(gl.originalOutput)
		}

		gl.logFile.Close()
		gl.logFile = nil
		gl.logger = nil
	}
}

// LogAppSubmission logs an app submission message with formatting
func (al *AppLogger) LogAppSubmission(format string, args ...interface{}) {
	if al.logger != nil {
		al.logger.Printf(format, args...)
	}
}

// GetLogFunc returns a function that can be used for dependency injection
func (al *AppLogger) GetLogFunc() func(format string, args ...interface{}) {
	return al.LogAppSubmission
}

// Global logger instances
var globalAppLogger *AppLogger
var globalGeneralLogger *GeneralLogger

// InitializeAppLogger initializes the global application logger
func InitializeAppLogger() error {
	globalAppLogger = NewAppLogger()
	return globalAppLogger.Initialize()
}

// InitializeGeneralLogger initializes the global general logger
func InitializeGeneralLogger() error {
	globalGeneralLogger = NewGeneralLogger()
	return globalGeneralLogger.Initialize()
}

// InitializeAllLoggers initializes both app submission and general logging
func InitializeAllLoggers() error {
	// Initialize general logger first to capture all subsequent log output
	if err := InitializeGeneralLogger(); err != nil {
		return fmt.Errorf("failed to initialize general logger: %v", err)
	}

	// Then initialize app submission logger
	if err := InitializeAppLogger(); err != nil {
		return fmt.Errorf("failed to initialize app logger: %v", err)
	}

	return nil
}

// CloseAppLogger closes the global application logger
func CloseAppLogger() {
	if globalAppLogger != nil {
		globalAppLogger.Close()
		globalAppLogger = nil
	}
}

// CloseGeneralLogger closes the global general logger
func CloseGeneralLogger() {
	if globalGeneralLogger != nil {
		globalGeneralLogger.Close()
		globalGeneralLogger = nil
	}
}

// CloseAllLoggers closes both app submission and general loggers
func CloseAllLoggers() {
	CloseAppLogger()
	CloseGeneralLogger()
}

// LogAppSubmission logs using the global logger (legacy compatibility)
func LogAppSubmission(format string, args ...interface{}) {
	if globalAppLogger != nil {
		globalAppLogger.LogAppSubmission(format, args...)
	}
}

// GetGlobalAppLogger returns the global logger instance
func GetGlobalAppLogger() *AppLogger {
	return globalAppLogger
}

// GetAppLogFunc returns the global log function for dependency injection
func GetAppLogFunc() func(format string, args ...interface{}) {
	if globalAppLogger != nil {
		return globalAppLogger.GetLogFunc()
	}
	return func(format string, args ...interface{}) {
		// No-op if logger not initialized
	}
}