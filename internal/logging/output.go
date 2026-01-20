package logging

import (
	"fmt"
	"log"
	"os"
	"time"
)

const levelFatal = "FATAL"

// writeLog is the unified internal logging function that handles all output
// It formats the message with optional fields and routes to appropriate stream:
// - DEBUG/INFO/WARN: stdout (via log.Println)
// - ERROR/FATAL: stderr (via fmt.Fprintf)
func (l *Logger) writeLog(level, msg string, fields map[string]interface{}) {
	timestamp := fmt.Sprintf("[%s]", GetTimestamp())
	logMsg := fmt.Sprintf("%s [%s] %s: %s", timestamp, level, l.name, msg)

	// Add structured fields if present
	if len(fields) > 0 {
		logMsg += " |"
		for k, v := range fields {
			logMsg += fmt.Sprintf(" %s=%v", k, v)
		}
	}

	// Route output based on severity level
	// ERROR and FATAL go to stderr only
	// All other levels (DEBUG/INFO/WARN) go to stdout
	if level == strError || level == levelFatal {
		fmt.Fprintf(os.Stderr, "%s\n", logMsg)
	} else {
		log.Println(logMsg)
	}
}

// logf is the internal logging function for formatted messages
func (l *Logger) logf(level, msg string, args ...interface{}) {
	formattedMsg := fmt.Sprintf(msg, args...)

	// Extract context fields if available and merge with logger fields
	contextFields := extractContextFields(l.ctx)
	var mergedFields map[string]interface{}

	if contextFields != nil || len(l.fields) > 0 {
		mergedFields = make(map[string]interface{})

		// Add context fields first (lowest priority)
		for k, v := range contextFields {
			mergedFields[k] = v
		}

		// Add logger's persistent fields (higher priority)
		for k, v := range l.fields {
			mergedFields[k] = v
		}
	}

	l.writeLog(level, formattedMsg, mergedFields)
}

// GetTimestamp returns a formatted timestamp
// Uses RFC3339 format for sortability and timezone awareness
// Can be overridden via LOG_TIMESTAMP env var for testing
func GetTimestamp() string {
	// Allow test override for deterministic output
	if override := os.Getenv("LOG_TIMESTAMP"); override != "" {
		return override
	}
	return time.Now().Format(time.RFC3339)
}
