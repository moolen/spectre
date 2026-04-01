package grafana

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

// MatchSnapshot compares actual output against a golden file.
// If UPDATE_GOLDEN=true environment variable is set, updates the golden file instead.
// Timestamps are normalized before comparison to ensure deterministic results.
func MatchSnapshot(t *testing.T, goldenPath string, actual any) {
	t.Helper()

	// Marshal actual to JSON
	actualJSON, err := json.MarshalIndent(actual, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal actual value: %v", err)
	}

	// Normalize timestamps in actual
	actualNormalized := NormalizeTimestamps(actualJSON)

	// Check if we should update golden files
	if os.Getenv("UPDATE_GOLDEN") == "true" {
		if err := os.WriteFile(goldenPath, actualNormalized, 0644); err != nil {
			t.Fatalf("Failed to update golden file %s: %v", goldenPath, err)
		}
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	// Read expected golden file
	expectedJSON, err := os.ReadFile(goldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("Golden file not found: %s\nRun with UPDATE_GOLDEN=true to create it.\n\nActual output:\n%s", goldenPath, string(actualNormalized))
		}
		t.Fatalf("Failed to read golden file %s: %v", goldenPath, err)
	}

	// Normalize expected as well (in case it was edited manually)
	expectedNormalized := NormalizeTimestamps(expectedJSON)

	// Compare
	if string(actualNormalized) != string(expectedNormalized) {
		t.Errorf("Output does not match golden file %s\n\nExpected:\n%s\n\nActual:\n%s\n\nRun with UPDATE_GOLDEN=true to update.",
			goldenPath, string(expectedNormalized), string(actualNormalized))
	}
}

// NormalizeTimestamps replaces RFC3339 timestamps with "NORMALIZED" for deterministic comparison.
// This handles the "timestamp" field in Observatory tool responses.
func NormalizeTimestamps(data []byte) []byte {
	// Match RFC3339 timestamps like "2024-01-15T10:30:00Z" or "2024-01-15T10:30:00+00:00"
	timestampPattern := regexp.MustCompile(`"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:Z|[+-]\d{2}:\d{2})"`)
	return timestampPattern.ReplaceAll(data, []byte(`"NORMALIZED"`))
}

// NormalizeFloats rounds floating point numbers to a fixed precision for comparison.
// This helps avoid floating point precision issues in comparisons.
func NormalizeFloats(data []byte) []byte {
	// Parse JSON, walk structure, round floats, re-marshal
	var obj any
	if err := json.Unmarshal(data, &obj); err != nil {
		return data // Return original if can't parse
	}

	normalizeValue(obj)

	result, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return data
	}
	return result
}

// normalizeValue recursively normalizes floating point values in a JSON structure
func normalizeValue(v any) {
	switch val := v.(type) {
	case map[string]any:
		for k, v := range val {
			if f, ok := v.(float64); ok {
				// Round to 4 decimal places
				val[k] = float64(int(f*10000+0.5)) / 10000
			} else {
				normalizeValue(v)
			}
		}
	case []any:
		for i, v := range val {
			if f, ok := v.(float64); ok {
				val[i] = float64(int(f*10000+0.5)) / 10000
			} else {
				normalizeValue(v)
			}
		}
	}
}

// AssertJSONEquals compares two JSON values for equality, ignoring formatting differences.
func AssertJSONEquals(t *testing.T, expected, actual []byte) {
	t.Helper()

	var expectedObj, actualObj any
	if err := json.Unmarshal(expected, &expectedObj); err != nil {
		t.Fatalf("Failed to parse expected JSON: %v", err)
	}
	if err := json.Unmarshal(actual, &actualObj); err != nil {
		t.Fatalf("Failed to parse actual JSON: %v", err)
	}

	expectedNorm, _ := json.Marshal(expectedObj)
	actualNorm, _ := json.Marshal(actualObj)

	if string(expectedNorm) != string(actualNorm) {
		t.Errorf("JSON values differ\n\nExpected:\n%s\n\nActual:\n%s",
			string(expected), string(actual))
	}
}
