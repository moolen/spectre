// Package logging provides structured logging for the Spectre application.
//
// Design Philosophy
//
// This package provides a simple structured logging API optimized for
// incident-time debuggability in Kubernetes controller contexts. It prioritizes
// explicit, boring Go over clever abstractions.
//
// The logger supports multiple log levels (DEBUG, INFO, WARN, ERROR, FATAL)
// and structured logging with key-value fields for better observability.
//
// Basic Usage
//
// Initialize the logger at application startup:
//
//	logging.Initialize("info")
//
// Get a named logger for your component:
//
//	logger := logging.GetLogger("mycomponent")
//	logger.Info("server started")
//	logger.Info("listening on port %d", 8080)
//
// Structured Logging (Recommended)
//
// Use structured fields for better searchability and analysis:
//
//	logger.InfoWithFields("request processed",
//	    logging.Field("duration_ms", duration.Milliseconds()),
//	    logging.Field("status_code", 200),
//	    logging.Field("path", r.URL.Path),
//	)
//
// Context Logger
//
// Create child loggers with persistent fields for request/operation context:
//
//	requestLogger := logger.
//	    WithField("request_id", requestID).
//	    WithField("user", username)
//
//	requestLogger.Info("processing request")
//	requestLogger.Info("querying database")
//	// All logs automatically include request_id and user fields
//
// Context Support
//
// The logger supports Go's context.Context for automatic trace and span ID extraction.
// This is useful for distributed tracing and request correlation:
//
//	// Add trace ID to context (e.g., from incoming request)
//	ctx := context.WithValue(r.Context(), logging.TraceIDKey(), "trace-123")
//	ctx = context.WithValue(ctx, logging.SpanIDKey(), "span-456")
//
//	// Create a context-aware logger
//	ctxLogger := logger.WithContext(ctx)
//	ctxLogger.Info("processing request")
//	// Output includes: trace_id=trace-123 span_id=span-456
//
// Context fields can be combined with persistent fields:
//
//	ctxLogger := logger.
//	    WithContext(ctx).
//	    WithField("user_id", userID)
//
//	ctxLogger.InfoWithFields("request complete",
//	    logging.Field("duration_ms", elapsed),
//	)
//	// Output includes: trace_id, span_id, user_id, and duration_ms
//
// Log Levels
//
// The package supports five log levels in increasing severity:
//
//	DEBUG - Detailed debugging information (verbose)
//	INFO  - Informational messages about normal operations
//	WARN  - Warning messages for potentially problematic situations
//	ERROR - Error messages for failures that don't stop the application
//	FATAL - Critical errors that cause application termination
//
// Set the minimum log level during initialization. Only messages at or above
// the configured level will be output:
//
//	logging.Initialize("warn")  // Only WARN, ERROR, FATAL will be logged
//
// Per-Package Log Levels
//
// You can override the log level for specific packages while keeping others
// at the default level. This is useful for targeted debugging:
//
//	// Set default to INFO, but graph.sync to DEBUG
//	packageLevels := map[string]string{
//	    "graph.sync": "debug",
//	    "graph.*":    "debug",  // Wildcard pattern
//	    "controller": "warn",
//	}
//	logging.Initialize("info", packageLevels)
//
// Package matching supports:
//   - Exact matches: "graph.sync" only matches "graph.sync"
//   - Wildcard patterns: "graph.*" matches "graph.sync", "graph.analyze", etc.
//   - Default fallback: packages not configured use the default level
//
// See LOG_LEVELS.md in the project root for detailed configuration guide.
//
// Error Handling
//
// For logging errors with context:
//
//	if err != nil {
//	    logger.ErrorWithErr("failed to process request", err)
//	}
//
// Or use structured fields for more context:
//
//	logger.ErrorWithFields("database query failed",
//	    logging.Field("query", sql),
//	    logging.Field("error", err.Error()),
//	    logging.Field("duration_ms", elapsed.Milliseconds()),
//	)
//
// Fatal Handling
//
// Fatal logs terminate the application with exit code 1:
//
//	if criticalErr != nil {
//	    logger.Fatal("critical initialization failed: %v", criticalErr)
//	    // Application exits here
//	}
//
// Thread Safety
//
// Logger instances are safe for concurrent use by multiple goroutines.
// The global logger initialization using GetLogger() is protected by sync.Once
// to ensure thread-safe lazy initialization.
//
// Each Logger instance is immutable - methods like WithField() and WithFields()
// return new Logger instances rather than modifying the original, making them
// safe to use across goroutines without coordination.
//
// Testing
//
// The package provides test helpers for capturing log output:
//
//	// Use LOG_TIMESTAMP env var for deterministic timestamps in tests
//	os.Setenv("LOG_TIMESTAMP", "2024-01-01T00:00:00Z")
//
// The global logger can be reinitialized in tests if needed, though GetLogger()
// will automatically initialize with INFO level on first use.
package logging

import (
	"context"
	"os"
	"strings"
	"sync"
)

var (
	globalLogger *Logger
	initOnce     sync.Once
	// exitFunc is the function called by Fatal to terminate the program.
	// Defaults to os.Exit, can be overridden for testing.
	exitFunc = os.Exit
)

// Initialize initializes the global logger with the specified default level
// and optional per-package log level overrides.
// packageLevels is a map of package patterns to level strings.
// Example: {"graph.*": "DEBUG", "controller": "WARN"}
func Initialize(levelStr string, packageLevels ...map[string]string) error {
	var level LogLevel
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		level = DEBUG
	case "INFO":
		level = INFO
	case "WARN":
		level = WARN
	case strError:
		level = ERROR
	case "FATAL":
		level = FATAL
	default:
		level = INFO
	}

	globalLogger = &Logger{
		level: level,
		name:  "spectre",
	}

	// Set up per-package log levels if provided
	if len(packageLevels) > 0 && packageLevels[0] != nil {
		if err := SetPackageLogLevels(packageLevels[0]); err != nil {
			return err
		}
	}

	return nil
}

// GetLogger returns a logger with the specified name
// Thread-safe: uses sync.Once to ensure single initialization
func GetLogger(name string) *Logger {
	initOnce.Do(func() {
		if globalLogger == nil {
			_ = Initialize("info")
		}
	})
	return &Logger{
		level:  globalLogger.level,
		name:   name,
		fields: make(map[string]interface{}),
	}
}

// shouldLog checks if a log message at the given level should be output
// Considers both the logger's level and any per-package level overrides
func (l *Logger) shouldLog(level LogLevel) bool {
	// Check package-specific overrides first
	if pkgLevel := GetPackageLogLevel(l.name); pkgLevel >= 0 {
		return level >= pkgLevel
	}
	// Fall back to logger's level
	return level >= l.level
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	if l.shouldLog(DEBUG) {
		l.logf("DEBUG", msg, args...)
	}
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	if l.shouldLog(INFO) {
		l.logf("INFO", msg, args...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...interface{}) {
	if l.shouldLog(WARN) {
		l.logf("WARN", msg, args...)
	}
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	if l.shouldLog(ERROR) {
		l.logf(strError, msg, args...)
	}
}

// Fatal logs a fatal message and exits the program with code 1
func (l *Logger) Fatal(msg string, args ...interface{}) {
	if l.shouldLog(FATAL) {
		l.logf("FATAL", msg, args...)
		exitFunc(1)
	}
}

// FatalWithFields logs a fatal message with structured fields and exits the program with code 1
func (l *Logger) FatalWithFields(msg string, fields ...LogField) {
	if l.shouldLog(FATAL) {
		l.logWithFields("FATAL", msg, fields...)
		exitFunc(1)
	}
}

// ErrorWithErr logs an error message with an error object
func (l *Logger) ErrorWithErr(msg string, err error, args ...interface{}) {
	if l.shouldLog(ERROR) {
		args = append(args, err)
		l.logf("ERROR", msg+" - %v", args...)
	}
}

// WithName returns a new logger with a custom name
func (l *Logger) WithName(name string) *Logger {
	return &Logger{
		level:  l.level,
		name:   name,
		fields: make(map[string]interface{}),
		ctx:    l.ctx,
	}
}

// WithField adds a structured field to the logger
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := &Logger{
		level:  l.level,
		name:   l.name,
		fields: cloneFields(l.fields),
		ctx:    l.ctx,
	}
	newLogger.fields[key] = value
	return newLogger
}

// WithFields adds multiple structured fields to the logger
func (l *Logger) WithFields(fields ...LogField) *Logger {
	newLogger := &Logger{
		level:  l.level,
		name:   l.name,
		fields: cloneFields(l.fields),
		ctx:    l.ctx,
	}
	// Add new fields
	for _, f := range fields {
		newLogger.fields[f.Key] = f.Value
	}
	return newLogger
}

// WithContext returns a new logger with the provided context attached.
// The context is used to extract trace_id and span_id values if present.
// These fields are automatically included in all log messages from the returned logger.
// If ctx is nil, this method returns a logger without context support.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{
		level:  l.level,
		name:   l.name,
		fields: cloneFields(l.fields),
		ctx:    ctx,
	}
}

// DebugWithFields logs a debug message with structured fields
func (l *Logger) DebugWithFields(msg string, fields ...LogField) {
	if l.shouldLog(DEBUG) {
		l.logWithFields("DEBUG", msg, fields...)
	}
}

// InfoWithFields logs an info message with structured fields
func (l *Logger) InfoWithFields(msg string, fields ...LogField) {
	if l.shouldLog(INFO) {
		l.logWithFields("INFO", msg, fields...)
	}
}

// WarnWithFields logs a warning message with structured fields
func (l *Logger) WarnWithFields(msg string, fields ...LogField) {
	if l.shouldLog(WARN) {
		l.logWithFields("WARN", msg, fields...)
	}
}

// ErrorWithFields logs an error message with structured fields
func (l *Logger) ErrorWithFields(msg string, fields ...LogField) {
	if l.shouldLog(ERROR) {
		l.logWithFields("ERROR", msg, fields...)
	}
}

// logWithFields logs a message with structured fields
func (l *Logger) logWithFields(level, msg string, fields ...LogField) {
	// Extract context fields (trace_id, span_id) if context is present
	contextFields := extractContextFields(l.ctx)

	// Merge all fields: context fields, logger fields, and method-specific fields
	// Priority order (last wins): context fields < logger fields < method fields
	var mergedFields map[string]interface{}
	if contextFields != nil || len(l.fields) > 0 || len(fields) > 0 {
		mergedFields = make(map[string]interface{})

		// Add context fields first (lowest priority)
		for k, v := range contextFields {
			mergedFields[k] = v
		}

		// Add logger's persistent fields (medium priority)
		for k, v := range l.fields {
			mergedFields[k] = v
		}

		// Add method-specific fields last (highest priority)
		for _, f := range fields {
			mergedFields[f.Key] = f.Value
		}
	}

	l.writeLog(level, msg, mergedFields)
}
