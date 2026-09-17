package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
)

// Logger defines the standard logging interface used across all modules
type Logger interface {
	Debug(ctx context.Context, msg string, fields ...interface{})
	Info(ctx context.Context, msg string, fields ...interface{})
	Warn(ctx context.Context, msg string, fields ...interface{})
	Error(ctx context.Context, msg string, fields ...interface{})
	Fatal(ctx context.Context, msg string, fields ...interface{})
	WithFields(fields map[string]interface{}) Logger
	WithContext(ctx context.Context) Logger
	WithModule(module string) Logger
	WithComponent(component string) Logger
}

// logger implements the Logger interface using slog
type logger struct {
	slogger   *slog.Logger
	module    string
	component string
	fields    map[string]interface{}
	addSource bool
}

// NewLogger creates a new logger instance from configuration
func NewLogger(config Config) (Logger, error) {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid logger configuration: %w", err)
	}

	// Create writers for all output paths
	writers := make([]io.Writer, 0, len(config.OutputPaths))
	for _, path := range config.OutputPaths {
		if path == "stdout" {
			writers = append(writers, os.Stdout)
		} else if path == "stderr" {
			writers = append(writers, os.Stderr)
		} else {
			// Ensure directory exists
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create log directory %s: %w", dir, err)
			}

			// Open file for appending
			file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to open log file %s: %w", path, err)
			}
			writers = append(writers, file)
		}
	}

	// Combine all writers using MultiWriter
	multiWriter := io.MultiWriter(writers...)

	// Convert log level to slog.Level
	var slogLevel slog.Level
	switch config.Level {
	case LevelDebug:
		slogLevel = slog.LevelDebug
	case LevelInfo:
		slogLevel = slog.LevelInfo
	case LevelWarn:
		slogLevel = slog.LevelWarn
	case LevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	// Create handler options
	// Note: AddSource is set to false because we manually add source info with correct caller depth
	handlerOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slogLevel,
	}

	// Create appropriate handler based on format
	var handler slog.Handler
	switch config.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(multiWriter, handlerOpts)
	case FormatText:
		handler = slog.NewTextHandler(multiWriter, handlerOpts)
	default:
		handler = slog.NewJSONHandler(multiWriter, handlerOpts)
	}

	// Create base slog logger
	slogLogger := slog.New(handler)

	// Add module and component attributes if provided
	if config.ModuleName != "" {
		slogLogger = slogLogger.With("module", config.ModuleName)
	}
	if config.ComponentName != "" {
		slogLogger = slogLogger.With("component", config.ComponentName)
	}

	return &logger{
		slogger:   slogLogger,
		module:    config.ModuleName,
		component: config.ComponentName,
		fields:    make(map[string]interface{}),
		addSource: config.AddSource,
	}, nil
}

// Debug logs a debug-level message
func (l *logger) Debug(ctx context.Context, msg string, fields ...interface{}) {
	l.log(ctx, slog.LevelDebug, msg, fields...)
}

// Info logs an info-level message
func (l *logger) Info(ctx context.Context, msg string, fields ...interface{}) {
	l.log(ctx, slog.LevelInfo, msg, fields...)
}

// Warn logs a warning-level message
func (l *logger) Warn(ctx context.Context, msg string, fields ...interface{}) {
	l.log(ctx, slog.LevelWarn, msg, fields...)
}

// Error logs an error-level message
func (l *logger) Error(ctx context.Context, msg string, fields ...interface{}) {
	l.log(ctx, slog.LevelError, msg, fields...)
}

// Fatal logs an error-level message and then exits the program
func (l *logger) Fatal(ctx context.Context, msg string, fields ...interface{}) {
	l.log(ctx, slog.LevelError, msg, fields...)
	os.Exit(1)
}

// WithFields returns a new logger with additional fields
func (l *logger) WithFields(fields map[string]interface{}) Logger {
	// Create a copy of existing fields
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	// Add new fields
	for k, v := range fields {
		newFields[k] = v
	}

	// Create new slog logger with these fields
	attrs := make([]any, 0, len(newFields))
	for k, v := range newFields {
		attrs = append(attrs, k, v)
	}

	return &logger{
		slogger:   l.slogger.With(attrs...),
		module:    l.module,
		component: l.component,
		fields:    newFields,
		addSource: l.addSource,
	}
}

// WithContext returns a new logger with context (currently no-op, but enables future context-based features)
func (l *logger) WithContext(ctx context.Context) Logger {
	// For now, this is a pass-through
	// In the future, we could extract trace IDs, request IDs, etc. from context
	return l
}

// WithModule returns a new logger with a module tag
func (l *logger) WithModule(module string) Logger {
	newLogger := l.slogger.With("module", module)
	return &logger{
		slogger:   newLogger,
		module:    module,
		component: l.component,
		fields:    l.fields,
		addSource: l.addSource,
	}
}

// WithComponent returns a new logger with a component tag
func (l *logger) WithComponent(component string) Logger {
	newLogger := l.slogger.With("component", component)
	return &logger{
		slogger:   newLogger,
		module:    l.module,
		component: component,
		fields:    l.fields,
		addSource: l.addSource,
	}
}

// log is the internal logging method that handles all log levels
func (l *logger) log(ctx context.Context, level slog.Level, msg string, fields ...interface{}) {
	// Convert variadic fields to slog attributes
	attrs := l.parseFields(fields)

	// Manually add source location if enabled
	// We use runtime.Caller(2) to skip:
	//   0: this log() function
	//   1: the public method (Info/Debug/Warn/Error)
	//   2: the actual caller we want to capture
	if l.addSource {
		pc, file, line, ok := runtime.Caller(2)
		if ok {
			fn := runtime.FuncForPC(pc)
			source := &slog.Source{
				Function: fn.Name(),
				File:     filepath.Base(file),
				Line:     line,
			}
			attrs = append(attrs, slog.Any(slog.SourceKey, source))
		}
	}

	// Log with context
	l.slogger.LogAttrs(ctx, level, msg, attrs...)
}

// isNilValue reports whether v holds a nil pointer, map, slice, chan, or
// func. A fmt.Stringer implemented on a pointer receiver (e.g. *time.Time)
// still satisfies the interface when the pointer is nil, so calling
// String() on it panics unless callers guard against this case first.
func isNilValue(v interface{}) bool {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}

// parseFields converts variadic key-value pairs into slog.Attr slice
func (l *logger) parseFields(fields []interface{}) []slog.Attr {
	if len(fields) == 0 {
		return nil
	}

	attrs := make([]slog.Attr, 0, len(fields)/2)

	// Process fields as key-value pairs
	for i := 0; i < len(fields); i += 2 {
		if i+1 >= len(fields) {
			// Odd number of fields - add the last one with a placeholder value
			attrs = append(attrs, slog.Any(fmt.Sprint(fields[i]), "MISSING_VALUE"))
			break
		}

		key, ok := fields[i].(string)
		if !ok {
			// Non-string key - convert to string
			key = fmt.Sprint(fields[i])
		}
		value := fields[i+1]

		// Handle special types
		switch v := value.(type) {
		case error:
			attrs = append(attrs, slog.String(key, v.Error()))
		case fmt.Stringer:
			if isNilValue(v) {
				attrs = append(attrs, slog.Any(key, nil))
			} else {
				attrs = append(attrs, slog.String(key, v.String()))
			}
		default:
			attrs = append(attrs, slog.Any(key, v))
		}
	}

	return attrs
}

// GetSlogLogger returns the underlying slog.Logger for advanced use cases
// This is intentionally not part of the Logger interface
func GetSlogLogger(l Logger) *slog.Logger {
	if impl, ok := l.(*logger); ok {
		return impl.slogger
	}
	return nil
}
