package logging

import (
	"fmt"
	"strings"
)

// LogLevel represents logging severity levels
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// OutputFormat represents log output format
type OutputFormat string

const (
	FormatJSON OutputFormat = "json"
	FormatText OutputFormat = "text"
)

// Config holds logging configuration
type Config struct {
	// Output configuration
	Level       LogLevel     // debug, info, warn, error
	OutputPaths []string     // stdout, file paths
	Format      OutputFormat // json, text

	// Feature flags
	AddSource    bool // Include file:line:function
	AddTimestamp bool // Include timestamp

	// Module identification
	ModuleName    string
	ComponentName string

	// File rotation (for future use)
	MaxSize    int // MB
	MaxBackups int
	MaxAge     int // days
}

// DefaultConfig returns sensible defaults for logging configuration
func DefaultConfig() Config {
	return Config{
		Level:        LevelInfo,
		OutputPaths:  []string{"stdout"},
		Format:       FormatJSON,
		AddSource:    true,
		AddTimestamp: true,
		ModuleName:   "",
		ComponentName: "",
		MaxSize:      100,  // 100MB
		MaxBackups:   3,
		MaxAge:       7, // 7 days
	}
}

// Validate checks configuration validity and returns an error if invalid
func (c Config) Validate() error {
	// Validate log level
	switch c.Level {
	case LevelDebug, LevelInfo, LevelWarn, LevelError:
		// Valid
	default:
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.Level)
	}

	// Validate output format
	switch c.Format {
	case FormatJSON, FormatText:
		// Valid
	default:
		return fmt.Errorf("invalid output format: %s (must be json or text)", c.Format)
	}

	// Validate output paths
	if len(c.OutputPaths) == 0 {
		return fmt.Errorf("at least one output path must be specified")
	}

	// Validate output paths are not empty strings
	for i, path := range c.OutputPaths {
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("output path at index %d is empty", i)
		}
	}

	// Validate file rotation settings (if provided)
	if c.MaxSize < 0 {
		return fmt.Errorf("MaxSize must be non-negative, got %d", c.MaxSize)
	}
	if c.MaxBackups < 0 {
		return fmt.Errorf("MaxBackups must be non-negative, got %d", c.MaxBackups)
	}
	if c.MaxAge < 0 {
		return fmt.Errorf("MaxAge must be non-negative, got %d", c.MaxAge)
	}

	return nil
}
