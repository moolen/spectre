package logging

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// LogLevel represents the logging level
type LogLevel int

const (
	// DEBUG level for detailed debugging information
	DEBUG LogLevel = iota
	// INFO level for informational messages
	INFO
	// WARN level for warning messages
	WARN
	// ERROR level for error messages
	ERROR
)

// Logger provides structured logging throughout the application
type Logger struct {
	level LogLevel
	name  string
}

var globalLogger *Logger

// Initialize initializes the global logger with the specified level
func Initialize(levelStr string) {
	level := INFO
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		level = DEBUG
	case "INFO":
		level = INFO
	case "WARN":
		level = WARN
	case "ERROR":
		level = ERROR
	}

	globalLogger = &Logger{
		level: level,
		name:  "rpk",
	}
}

// GetLogger returns a logger with the specified name
func GetLogger(name string) *Logger {
	if globalLogger == nil {
		Initialize("info")
	}
	return &Logger{
		level: globalLogger.level,
		name:  name,
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	if l.level <= DEBUG {
		l.logf("DEBUG", msg, args...)
	}
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	if l.level <= INFO {
		l.logf("INFO", msg, args...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...interface{}) {
	if l.level <= WARN {
		l.logf("WARN", msg, args...)
	}
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	if l.level <= ERROR {
		l.logf("ERROR", msg, args...)
	}
}

// ErrorWithErr logs an error message with an error object
func (l *Logger) ErrorWithErr(msg string, err error, args ...interface{}) {
	if l.level <= ERROR {
		allArgs := append(args, err)
		l.logf("ERROR", msg+" - %v", allArgs...)
	}
}

// logf is the internal logging function
func (l *Logger) logf(level, msg string, args ...interface{}) {
	timestamp := fmt.Sprintf("[%s]", GetTimestamp())
	logMsg := fmt.Sprintf("%s [%s] %s: %s", timestamp, level, l.name, fmt.Sprintf(msg, args...))
	log.Println(logMsg)

	// Also write to stderr for errors
	if level == "ERROR" {
		fmt.Fprintf(os.Stderr, "%s\n", logMsg)
	}
}

// GetTimestamp returns a formatted timestamp
func GetTimestamp() string {
	return fmt.Sprintf("%v", os.Getenv("LOG_TIMESTAMP"))
}

// WithName returns a new logger with a custom name
func (l *Logger) WithName(name string) *Logger {
	return &Logger{
		level: l.level,
		name:  name,
	}
}
