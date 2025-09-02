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

// NewAppLogger creates a new application logger instance
func NewAppLogger() *AppLogger {
	return &AppLogger{}
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

// Close closes the log file and shuts down the logger
func (al *AppLogger) Close() {
	if al.logFile != nil && al.logger != nil {
		al.logger.Println("=== App Submission Logger Closing ===")
		al.logFile.Close()
		al.logFile = nil
		al.logger = nil
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

// Global logger instance for backward compatibility
var globalAppLogger *AppLogger

// InitializeAppLogger initializes the global application logger
func InitializeAppLogger() error {
	globalAppLogger = NewAppLogger()
	return globalAppLogger.Initialize()
}

// CloseAppLogger closes the global application logger
func CloseAppLogger() {
	if globalAppLogger != nil {
		globalAppLogger.Close()
		globalAppLogger = nil
	}
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