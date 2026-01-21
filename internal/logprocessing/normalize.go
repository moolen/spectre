package logprocessing

import (
	"encoding/json"
	"strings"
)

// ExtractMessage extracts the semantic message from a log entry.
// For JSON logs, it attempts to extract common message field names.
// For plain text logs, it returns the log as-is.
//
// User decision from CONTEXT.md: "For JSON logs, extract and template the message/msg field only (ignore JSON structure)"
func ExtractMessage(rawLog string) string {
	// Try parsing as JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(rawLog), &parsed); err != nil {
		// Not JSON, use as-is
		return rawLog
	}

	// Try common message field names (order matters - most specific first)
	messageFields := []string{
		"message", // Standard field name
		"msg",     // Common shorthand
		"log",     // Kubernetes container logs
		"text",    // Alternative name
		"_raw",    // Fluentd convention
		"event",   // Event-based logging
	}

	for _, field := range messageFields {
		if value, ok := parsed[field]; ok {
			if msg, ok := value.(string); ok && msg != "" {
				return msg
			}
		}
	}

	// No message field found - return full rawLog
	// This might be a structured event log where all fields are meaningful
	return rawLog
}

// PreProcess normalizes a log message for Drain clustering.
// It extracts the message from JSON if applicable, converts to lowercase,
// and trims whitespace. Variable masking is NOT done here - that happens
// post-clustering.
//
// User decision from CONTEXT.md: "masking AFTER Drain clustering"
func PreProcess(rawLog string) string {
	// Extract semantic message from JSON or use as-is
	message := ExtractMessage(rawLog)

	// Convert to lowercase for case-insensitive clustering
	message = strings.ToLower(message)

	// Trim whitespace
	message = strings.TrimSpace(message)

	// DO NOT mask variables yet - that happens post-clustering
	return message
}
