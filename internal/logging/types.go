package logging

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	// FATAL level for fatal messages
	FATAL
)

const (
	strError = "ERROR"
)

// LogField represents a structured logging field
type LogField struct {
	Key   string
	Value interface{}
}

// Field creates a structured logging field
func Field(key string, value interface{}) LogField {
	return LogField{Key: key, Value: value}
}

// Logger provides structured logging throughout the application
type Logger struct {
	level  LogLevel
	name   string
	fields map[string]interface{} // Structured fields
	ctx    context.Context        // Optional context for trace/span ID extraction
}

// packageLogLevels stores per-package log level overrides
// Key format: "package.name" or "pattern.*" for wildcard matching
// Supports both exact matches and prefix patterns
var (
	packageLogLevels = make(map[string]LogLevel)
	packageLogMutex  sync.RWMutex
)

// SetPackageLogLevels configures per-package log levels
// Supports patterns like "graph.*" to match "graph.sync", "graph.analyze", etc.
// Input format: map["package.name"]="DEBUG" or map["graph.*"]="INFO"
// Returns error if level names are invalid
func SetPackageLogLevels(levels map[string]string) error {
	if levels == nil {
		return nil
	}

	packageLogMutex.Lock()
	defer packageLogMutex.Unlock()

	// Clear and rebuild
	packageLogLevels = make(map[string]LogLevel)

	for pkg, levelStr := range levels {
		level, err := parseLevel(levelStr)
		if err != nil {
			return fmt.Errorf("invalid log level for package %q: %w", pkg, err)
		}
		packageLogLevels[pkg] = level
	}

	return nil
}

// GetPackageLogLevel returns the effective log level for a package name
// Searches in order: exact match, wildcard patterns (sorted by specificity), default (-1 if not found)
func GetPackageLogLevel(packageName string) LogLevel {
	packageLogMutex.RLock()
	defer packageLogMutex.RUnlock()

	// Check for exact match first
	if level, exists := packageLogLevels[packageName]; exists {
		return level
	}

	// Sort patterns by specificity (longest first = most specific)
	// and return the first match
	var patterns []string
	for pattern := range packageLogLevels {
		if matchesPattern(packageName, pattern) {
			patterns = append(patterns, pattern)
		}
	}

	// Simple sort: by length descending (longest = most specific)
	for i := 0; i < len(patterns); i++ {
		for j := i + 1; j < len(patterns); j++ {
			if len(patterns[j]) > len(patterns[i]) {
				patterns[i], patterns[j] = patterns[j], patterns[i]
			}
		}
	}

	if len(patterns) > 0 {
		return packageLogLevels[patterns[0]]
	}

	// Not found
	return LogLevel(-1)
}

// matchesPattern returns true if packageName matches the pattern
// Supports wildcard patterns like "graph.*"
// Examples:
//   matchesPattern("graph.sync", "graph.sync") -> true (exact)
//   matchesPattern("graph.sync", "graph.*") -> true (wildcard)
//   matchesPattern("controller", "graph.*") -> false (no match)
func matchesPattern(packageName, pattern string) bool {
	// Exact match
	if packageName == pattern {
		return true
	}

	// Wildcard match: "graph.*" matches anything starting with "graph."
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(packageName, prefix+".")
	}

	return false
}

// parseLevel converts a string level to LogLevel enum
func parseLevel(levelStr string) (LogLevel, error) {
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return DEBUG, nil
	case "INFO":
		return INFO, nil
	case "WARN":
		return WARN, nil
	case "ERROR":
		return ERROR, nil
	case "FATAL":
		return FATAL, nil
	default:
		return -1, fmt.Errorf("invalid level: %s (must be DEBUG, INFO, WARN, ERROR, or FATAL)", levelStr)
	}
}
