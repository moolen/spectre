package parsing

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	dps "github.com/markusmobius/go-dateparser"
)

// ParseTimestamp parses a timestamp string, supporting both Unix timestamps and human-readable dates.
// Returns Unix timestamp in seconds.
// fieldName is used for error messages (e.g., "start", "end").
//
// Supported formats:
//   - Unix timestamps: "1609459200", "0"
//   - Human-readable dates: "now", "2h ago", "yesterday", "last week", "2024-01-01", etc.
//   - Composite format: "now-2h", "now-30m", "now-1d" (subtract duration from now)
//
// Parsing is done relative to the server's current time in UTC.
// For backward compatibility, numeric strings are always parsed as Unix timestamps first.
func ParseTimestamp(timestampStr, fieldName string) (int64, error) {
	if timestampStr == "" {
		return 0, NewParsingError("%s timestamp is required", fieldName)
	}

	// First, try parsing as Unix timestamp (for backward compatibility)
	if unixTimestamp, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
		if unixTimestamp < 0 {
			return 0, NewParsingError("%s timestamp must be non-negative", fieldName)
		}
		return unixTimestamp, nil
	}

	// Try parsing "now-<duration>" format (e.g., "now-2h", "now-30m")
	trimmed := strings.TrimSpace(timestampStr)
	// Check if input looks like "now-..." format (case-insensitive)
	nowMinusPattern := regexp.MustCompile(`(?i)^\s*now\s*-`)
	if nowMinusPattern.MatchString(trimmed) {
		// This looks like a "now-..." format, so parse it or return error (don't fall back)
		result, err := parseNowMinusDuration(trimmed, fieldName)
		if err != nil {
			return 0, err // Return error directly, don't fall back to go-dateparser
		}
		return result, nil
	}

	// If not a valid integer or "now-<duration>", try parsing as human-readable date
	parser := dps.Parser{}
	cfg := &dps.Configuration{
		// Use CurrentPeriod as default to interpret dates like "March" as current period
		// This is more intuitive for search queries
		PreferredDateSource: dps.CurrentPeriod,
	}

	parsedDate, err := parser.Parse(cfg, timestampStr)
	if err != nil {
		return 0, NewParsingError("%s must be a valid Unix timestamp or human-readable date: %v", fieldName, err)
	}

	if parsedDate.IsZero() {
		return 0, NewParsingError("%s could not be parsed as a valid date: %s", fieldName, timestampStr)
	}

	// Convert to Unix seconds
	return parsedDate.Time.Unix(), nil
}

// parseNowMinusDuration parses the "now-<duration>" format.
// Examples: "now-2h", "now-30m", "now-1d"
// Returns the Unix timestamp in seconds, or an error if the format is invalid.
// Note: This function is only called when input is confirmed to match "now-..." pattern.
func parseNowMinusDuration(input, fieldName string) (int64, error) {
	// Pattern: "now" (case-insensitive) followed by "-" followed by duration
	// Trim whitespace around "now" and "-"
	pattern := regexp.MustCompile(`(?i)^\s*now\s*-\s*(.+)$`)
	matches := pattern.FindStringSubmatch(input)
	if len(matches) != 2 {
		return 0, NewParsingError("%s: duration is required after 'now-'", fieldName)
	}

	durationStr := strings.TrimSpace(matches[1])
	if durationStr == "" {
		return 0, NewParsingError("%s: duration is required after 'now-'", fieldName)
	}

	// Parse duration: number followed by unit (h, m, d, etc.)
	// Support: h/hr/hrs/hour/hours, m/min/mins/minute/minutes, d/day/days
	durationPattern := regexp.MustCompile(`(?i)^(\d+)\s*(h|hr|hrs|hour|hours|m|min|mins|minute|minutes|d|day|days)$`)
	durationMatches := durationPattern.FindStringSubmatch(durationStr)
	if len(durationMatches) != 3 {
		return 0, NewParsingError("%s: invalid duration format in 'now-<duration>'. Expected format: 'now-<number><unit>' (e.g., 'now-2h', 'now-30m')", fieldName)
	}

	amount, err := strconv.ParseInt(durationMatches[1], 10, 64)
	if err != nil {
		return 0, NewParsingError("%s: invalid number in duration: %s", fieldName, durationMatches[1])
	}

	unit := strings.ToLower(durationMatches[2])
	now := time.Now().UTC()

	var result time.Time
	switch {
	case strings.HasPrefix(unit, "h"):
		// Hours
		result = now.Add(-time.Duration(amount) * time.Hour)
	case strings.HasPrefix(unit, "m"):
		// Minutes
		result = now.Add(-time.Duration(amount) * time.Minute)
	case strings.HasPrefix(unit, "d"):
		// Days
		result = now.AddDate(0, 0, -int(amount))
	default:
		return 0, NewParsingError("%s: unsupported duration unit: %s. Supported units: h, m, d", fieldName, unit)
	}

	return result.Unix(), nil
}

// ParseOptionalTimestamp parses an optional timestamp string.
// If the string is empty, returns defaultVal.
// If the string is provided but invalid, returns an error.
// Supports both Unix timestamps and human-readable dates.
func ParseOptionalTimestamp(timestampStr string, defaultVal int64) (int64, error) {
	if timestampStr == "" {
		return defaultVal, nil
	}

	return ParseTimestamp(timestampStr, "timestamp")
}

// ParseTimestampRange parses start and end timestamp strings
// Returns (start, end, error)
func ParseTimestampRange(startStr, endStr string) (int64, int64, error) {
	start, err := ParseTimestamp(startStr, "start")
	if err != nil {
		return 0, 0, err
	}

	end, err := ParseTimestamp(endStr, "end")
	if err != nil {
		return 0, 0, err
	}

	if start > end {
		return 0, 0, NewParsingError("start timestamp must be less than or equal to end timestamp")
	}

	return start, end, nil
}
