package tools

import (
	"strings"
	"testing"
)

func TestTruncateResult_NilResult(t *testing.T) {
	result := truncateResult(nil, MaxToolResponseBytes)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestTruncateResult_NilData(t *testing.T) {
	original := &Result{
		Success: true,
		Summary: "test",
	}
	result := truncateResult(original, MaxToolResponseBytes)
	if result != original {
		t.Errorf("expected original result to be returned unchanged")
	}
}

func TestTruncateResult_SmallData(t *testing.T) {
	original := &Result{
		Success: true,
		Data:    map[string]string{"key": "value"},
		Summary: "small data",
	}
	result := truncateResult(original, MaxToolResponseBytes)
	if result != original {
		t.Errorf("expected original result to be returned unchanged for small data")
	}
}

func TestTruncateResult_LargeData(t *testing.T) {
	// Create data larger than 1KB (using small limit for testing)
	largeString := strings.Repeat("x", 2000)
	original := &Result{
		Success:         true,
		Data:            map[string]string{"large": largeString},
		Summary:         "large data",
		ExecutionTimeMs: 100,
	}

	maxBytes := 1024 // 1KB limit for test
	result := truncateResult(original, maxBytes)

	// Should be a different result
	if result == original {
		t.Error("expected truncated result to be different from original")
	}

	// Should still be successful
	if !result.Success {
		t.Error("expected success to be preserved")
	}

	// Should have execution time preserved
	if result.ExecutionTimeMs != 100 {
		t.Errorf("expected execution time 100, got %d", result.ExecutionTimeMs)
	}

	// Summary should indicate truncation
	if !strings.Contains(result.Summary, "TRUNCATED") {
		t.Errorf("expected summary to contain TRUNCATED, got %s", result.Summary)
	}

	// Data should be truncatedData type
	truncated, ok := result.Data.(*truncatedData)
	if !ok {
		t.Fatalf("expected data to be *truncatedData, got %T", result.Data)
	}

	if !truncated.Truncated {
		t.Error("expected Truncated flag to be true")
	}

	if truncated.OriginalBytes <= maxBytes {
		t.Errorf("expected OriginalBytes > %d, got %d", maxBytes, truncated.OriginalBytes)
	}

	if truncated.TruncatedBytes != maxBytes {
		t.Errorf("expected TruncatedBytes = %d, got %d", maxBytes, truncated.TruncatedBytes)
	}

	if truncated.PartialData == "" {
		t.Error("expected PartialData to contain partial content")
	}

	if truncated.TruncationNote == "" {
		t.Error("expected TruncationNote to be set")
	}
}

func TestTruncateResult_PreservesError(t *testing.T) {
	largeString := strings.Repeat("x", 2000)
	original := &Result{
		Success: false,
		Data:    map[string]string{"large": largeString},
		Error:   "some error",
		Summary: "error case",
	}

	result := truncateResult(original, 1024)

	if result.Error != "some error" {
		t.Errorf("expected error to be preserved, got %s", result.Error)
	}

	if result.Success {
		t.Error("expected Success=false to be preserved")
	}
}

func TestTruncateResult_EmptySummary(t *testing.T) {
	largeString := strings.Repeat("x", 2000)
	original := &Result{
		Success: true,
		Data:    map[string]string{"large": largeString},
		Summary: "",
	}

	result := truncateResult(original, 1024)

	if !strings.Contains(result.Summary, "TRUNCATED") {
		t.Errorf("expected summary to contain TRUNCATED even when original was empty, got %s", result.Summary)
	}
}

func TestTruncateResult_ExactLimit(t *testing.T) {
	// Create data that's exactly at the limit
	// This is tricky because JSON marshaling adds overhead
	original := &Result{
		Success: true,
		Data:    "x",
		Summary: "at limit",
	}

	// Should not be truncated
	result := truncateResult(original, 100)
	if result != original {
		t.Error("expected result at limit to not be truncated")
	}
}
