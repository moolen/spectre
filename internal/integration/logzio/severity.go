package logzio

// Severity classification patterns for log analysis.
// These patterns are designed to match error and warning indicators across
// multiple programming languages and logging frameworks.
//
// Pattern Design Notes:
// - Uses (?i) for case-insensitive matching
// - Avoids leading wildcards for Elasticsearch performance
// - Groups related patterns for maintainability
// - Balances precision vs. recall (prefers catching errors over missing them)

// ErrorPattern is a regex pattern that matches error-level log messages.
// Optimized for Elasticsearch while covering the most common error indicators.
//
// Categories covered:
// 1. Explicit log levels: level=error, ERROR:
// 2. Common exceptions: Exception, panic
// 3. Kubernetes errors: CrashLoopBackOff, OOMKilled
const ErrorPattern = `(?i)(` +
	`level=error|ERROR:|` +
	`Exception|panic:|` +
	`CrashLoopBackOff|OOMKilled` +
	`)`

// WarningPattern is a regex pattern that matches warning-level log messages.
// Optimized for Elasticsearch while covering the most common warning indicators.
//
// Categories covered:
// 1. Explicit log levels: level=warn, WARN:, WARNING:
// 2. Warning keywords: deprecated
// 3. Health indicators: unhealthy
const WarningPattern = `(?i)(` +
	`level=warn|WARN:|WARNING:|` +
	`deprecated|unhealthy` +
	`)`

// GetErrorPattern returns the error classification regex pattern.
func GetErrorPattern() string {
	return ErrorPattern
}

// GetWarningPattern returns the warning classification regex pattern.
func GetWarningPattern() string {
	return WarningPattern
}
