package api

import (
	"strconv"

	dps "github.com/markusmobius/go-dateparser"
)

// ParseTimestamp parses a timestamp string, supporting both Unix timestamps and human-readable dates.
// Returns Unix timestamp in seconds.
// fieldName is used for error messages (e.g., "start", "end").
func ParseTimestamp(timestampStr, fieldName string) (int64, error) {
	if timestampStr == "" {
		return 0, NewValidationError("%s timestamp is required", fieldName)
	}

	// First, try parsing as Unix timestamp (for backward compatibility)
	if unixTimestamp, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
		if unixTimestamp < 0 {
			return 0, NewValidationError("%s timestamp must be non-negative", fieldName)
		}
		return unixTimestamp, nil
	}

	// If not a valid integer, try parsing as human-readable date
	parser := dps.Parser{}
	cfg := &dps.Configuration{
		// Use CurrentPeriod as default to interpret dates like "March" as current period
		// This is more intuitive for search queries
		PreferredDateSource: dps.CurrentPeriod,
	}

	parsedDate, err := parser.Parse(cfg, timestampStr)
	if err != nil {
		return 0, NewValidationError("%s must be a valid Unix timestamp or human-readable date: %v", fieldName, err)
	}

	if parsedDate.IsZero() {
		return 0, NewValidationError("%s could not be parsed as a valid date: %s", fieldName, timestampStr)
	}

	// Convert to Unix seconds
	return parsedDate.Time.Unix(), nil
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
