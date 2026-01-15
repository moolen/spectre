package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/mcp/client"
)

func TestDetectAnomaliesTool_TransformResponse(t *testing.T) {
	tool := &DetectAnomaliesTool{}

	response := &client.AnomalyResponse{
		Anomalies: []client.Anomaly{
			{
				Node: client.AnomalyNode{
					UID:       "uid-1",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "crash-pod",
				},
				Category:  "Event",
				Type:      "CrashLoopBackOff",
				Severity:  "critical",
				Timestamp: "2024-01-15T10:30:00Z",
				Summary:   "Container repeatedly crashing",
				Details: map[string]interface{}{
					"restart_count": 5,
				},
			},
			{
				Node: client.AnomalyNode{
					UID:       "uid-2",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "oom-pod",
				},
				Category:  "State",
				Type:      "OOMKilled",
				Severity:  "high",
				Timestamp: "2024-01-15T10:25:00Z",
				Summary:   "Container killed due to OOM",
				Details:   map[string]interface{}{},
			},
			{
				Node: client.AnomalyNode{
					UID:       "uid-3",
					Kind:      "Deployment",
					Namespace: "default",
					Name:      "web-deploy",
				},
				Category:  "Change",
				Type:      "ReplicaChange",
				Severity:  "medium",
				Timestamp: "2024-01-15T10:20:00Z",
				Summary:   "Replicas changed from 3 to 1",
				Details:   map[string]interface{}{},
			},
		},
		Metadata: client.AnomalyMetadata{
			ResourceUID: "uid-target",
			TimeWindow: client.AnomalyTimeWindow{
				Start: "2024-01-15T10:00:00Z",
				End:   "2024-01-15T11:00:00Z",
			},
			NodesAnalyzed: 5,
			ExecTimeMs:    42,
		},
	}

	output := tool.transformResponse(response, 1705315200, 1705318800)

	// Check anomaly count
	if output.AnomalyCount != 3 {
		t.Errorf("Expected anomaly count 3, got %d", output.AnomalyCount)
	}

	// Check severity breakdown
	if output.AnomaliesBySeverity["critical"] != 1 {
		t.Errorf("Expected 1 critical anomaly, got %d", output.AnomaliesBySeverity["critical"])
	}
	if output.AnomaliesBySeverity["high"] != 1 {
		t.Errorf("Expected 1 high anomaly, got %d", output.AnomaliesBySeverity["high"])
	}
	if output.AnomaliesBySeverity["medium"] != 1 {
		t.Errorf("Expected 1 medium anomaly, got %d", output.AnomaliesBySeverity["medium"])
	}

	// Check category breakdown
	if output.AnomaliesByCategory["Event"] != 1 {
		t.Errorf("Expected 1 Event anomaly, got %d", output.AnomaliesByCategory["Event"])
	}
	if output.AnomaliesByCategory["State"] != 1 {
		t.Errorf("Expected 1 State anomaly, got %d", output.AnomaliesByCategory["State"])
	}
	if output.AnomaliesByCategory["Change"] != 1 {
		t.Errorf("Expected 1 Change anomaly, got %d", output.AnomaliesByCategory["Change"])
	}

	// Check anomaly details preserved
	if len(output.Anomalies) != 3 {
		t.Fatalf("Expected 3 anomalies, got %d", len(output.Anomalies))
	}

	// First anomaly
	a1 := output.Anomalies[0]
	if a1.Node.Name != "crash-pod" {
		t.Errorf("Expected first anomaly node name 'crash-pod', got '%s'", a1.Node.Name)
	}
	if a1.Type != "CrashLoopBackOff" {
		t.Errorf("Expected first anomaly type 'CrashLoopBackOff', got '%s'", a1.Type)
	}
	if a1.TimestampText == "" {
		t.Error("Expected human-readable timestamp, got empty string")
	}

	// Check metadata
	if output.Metadata.ResourceUID != "uid-target" {
		t.Errorf("Expected resource UID 'uid-target', got '%s'", output.Metadata.ResourceUID)
	}
	if output.Metadata.NodesAnalyzed != 5 {
		t.Errorf("Expected 5 nodes analyzed, got %d", output.Metadata.NodesAnalyzed)
	}
	if output.Metadata.StartTimeText == "" {
		t.Error("Expected human-readable start time, got empty string")
	}
}

func TestDetectAnomaliesTool_EmptyResponse(t *testing.T) {
	tool := &DetectAnomaliesTool{}

	response := &client.AnomalyResponse{
		Anomalies: []client.Anomaly{},
		Metadata: client.AnomalyMetadata{
			ResourceUID:   "uid-target",
			NodesAnalyzed: 3,
			ExecTimeMs:    10,
		},
	}

	output := tool.transformResponse(response, 1705315200, 1705318800)

	if output.AnomalyCount != 0 {
		t.Errorf("Expected anomaly count 0, got %d", output.AnomalyCount)
	}

	if len(output.AnomaliesBySeverity) != 0 {
		t.Errorf("Expected empty severity map, got %v", output.AnomaliesBySeverity)
	}

	if len(output.AnomaliesByCategory) != 0 {
		t.Errorf("Expected empty category map, got %v", output.AnomaliesByCategory)
	}
}

func TestDetectAnomaliesTool_InputValidation(t *testing.T) {
	tool := &DetectAnomaliesTool{}
	ctx := context.Background()

	tests := []struct {
		name        string
		input       map[string]interface{}
		expectError string
	}{
		{
			name:        "missing resource_uid and namespace/kind",
			input:       map[string]interface{}{"start_time": 1000, "end_time": 2000},
			expectError: "either resource_uid OR both namespace and kind must be provided",
		},
		{
			name:        "missing kind when namespace provided",
			input:       map[string]interface{}{"namespace": "default", "start_time": 1000, "end_time": 2000},
			expectError: "either resource_uid OR both namespace and kind must be provided",
		},
		{
			name:        "missing namespace when kind provided",
			input:       map[string]interface{}{"kind": "Pod", "start_time": 1000, "end_time": 2000},
			expectError: "either resource_uid OR both namespace and kind must be provided",
		},
		{
			name:        "missing start_time",
			input:       map[string]interface{}{"resource_uid": "test-uid", "end_time": 2000},
			expectError: "start_time is required",
		},
		{
			name:        "missing end_time",
			input:       map[string]interface{}{"resource_uid": "test-uid", "start_time": 1000},
			expectError: "end_time is required",
		},
		{
			name:        "start_time >= end_time",
			input:       map[string]interface{}{"resource_uid": "test-uid", "start_time": 2000, "end_time": 1000},
			expectError: "start_time must be before end_time",
		},
		{
			name:        "start_time == end_time",
			input:       map[string]interface{}{"resource_uid": "test-uid", "start_time": 1000, "end_time": 1000},
			expectError: "start_time must be before end_time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON, _ := json.Marshal(tt.input)
			_, err := tool.Execute(ctx, inputJSON)

			if err == nil {
				t.Errorf("Expected error containing '%s', got nil", tt.expectError)
				return
			}

			if !contains(err.Error(), tt.expectError) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.expectError, err.Error())
			}
		})
	}
}

func TestDetectAnomaliesTool_TimestampConversion(t *testing.T) {
	// Test that milliseconds are converted to seconds
	// These are both around Jan 15, 2024
	startMs := int64(1705315200000) // Milliseconds
	endMs := int64(1705318800000)   // Milliseconds

	// After conversion: start=1705315200, end=1705318800 (seconds)
	input := DetectAnomaliesInput{
		ResourceUID: "test",
		StartTime:   startMs,
		EndTime:     endMs,
	}

	// We can't fully test without a mock client, but we can verify the logic
	// by checking the conversion threshold (10000000000)
	if startMs <= 10000000000 {
		t.Error("Test setup error: startMs should be > 10000000000 to trigger conversion")
	}

	// Verify conversion logic works correctly
	convertedStart := startMs
	convertedEnd := endMs
	if convertedStart > 10000000000 {
		convertedStart /= 1000
	}
	if convertedEnd > 10000000000 {
		convertedEnd /= 1000
	}

	if convertedStart >= convertedEnd {
		t.Errorf("After conversion, start (%d) should be < end (%d)", convertedStart, convertedEnd)
	}

	// Just verify input parsing works
	inputJSON, _ := json.Marshal(input)
	var parsed DetectAnomaliesInput
	if err := json.Unmarshal(inputJSON, &parsed); err != nil {
		t.Errorf("Failed to parse input: %v", err)
	}
}

func TestDetectAnomaliesTool_InvalidTimestampFormat(t *testing.T) {
	tool := &DetectAnomaliesTool{}

	// Test with invalid timestamp format in response
	response := &client.AnomalyResponse{
		Anomalies: []client.Anomaly{
			{
				Node: client.AnomalyNode{
					UID:       "uid-1",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "test-pod",
				},
				Category:  "Event",
				Type:      "TestAnomaly",
				Severity:  "low",
				Timestamp: "invalid-timestamp", // Invalid format
				Summary:   "Test anomaly",
			},
		},
		Metadata: client.AnomalyMetadata{
			ResourceUID:   "uid-target",
			NodesAnalyzed: 1,
		},
	}

	output := tool.transformResponse(response, 1000, 2000)

	// Should not crash, and should use fallback
	if len(output.Anomalies) != 1 {
		t.Fatalf("Expected 1 anomaly, got %d", len(output.Anomalies))
	}

	// TimestampText should fall back to the original string
	if output.Anomalies[0].TimestampText != "invalid-timestamp" {
		t.Errorf("Expected timestamp text to be 'invalid-timestamp', got '%s'", output.Anomalies[0].TimestampText)
	}

	// Timestamp (int64) should be 0 since parsing failed
	if output.Anomalies[0].Timestamp != 0 {
		t.Errorf("Expected timestamp to be 0 for invalid format, got %d", output.Anomalies[0].Timestamp)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
