package storage

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/models"
)

// TestTimelineGapFix verifies that pre-existing resources show segments
// from query start even when the first event is after query start
func TestTimelineGapFix(t *testing.T) {
	tests := []struct {
		name                string
		queryStartTimeNs    int64
		events              []models.Event
		expectedSegmentCnt  int
		expectedFirstStart  int64 // in seconds
		expectedLeadingMsg  string
	}{
		{
			name:             "pre-existing resource with gap",
			queryStartTimeNs: 1000 * 1e9, // Query starts at 1000s
			events: []models.Event{
				{
					ID:        "state-test/v1/Pod/default/test-pod-100",
					Timestamp: 1500 * 1e9, // First event at 1500s (500s after query start)
					Type:      models.EventTypeUpdate,
					Resource: models.ResourceMetadata{
						UID:       "test-uid-1",
						Group:     "",
						Version:   "v1",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "test-pod",
					},
					Data: json.RawMessage(`{"status":{"phase":"Running"}}`),
				},
				{
					ID:        "event-2",
					Timestamp: 2000 * 1e9,
					Type:      models.EventTypeUpdate,
					Resource: models.ResourceMetadata{
						UID:       "test-uid-1",
						Group:     "",
						Version:   "v1",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "test-pod",
					},
					Data: json.RawMessage(`{"status":{"phase":"Running"}}`),
				},
			},
			expectedSegmentCnt: 1, // Merged into 1 segment (all have same status)
			expectedFirstStart: 1000,
			expectedLeadingMsg: "Resource existed before query window",
		},
		{
			name:             "non-state event at query start",
			queryStartTimeNs: 1000 * 1e9,
			events: []models.Event{
				{
					ID:        "event-1", // Not a state snapshot
					Timestamp: 1000 * 1e9,
					Type:      models.EventTypeUpdate,
					Resource: models.ResourceMetadata{
						UID:       "test-uid-2",
						Group:     "",
						Version:   "v1",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "test-pod-2",
					},
					Data: json.RawMessage(`{"status":{"phase":"Running"}}`),
				},
			},
			expectedSegmentCnt: 1, // No leading segment needed
			expectedFirstStart: 1000,
			expectedLeadingMsg: "",
		},
		{
			name:             "state event before query start",
			queryStartTimeNs: 2000 * 1e9,
			events: []models.Event{
				{
					ID:        "state-test/v1/Pod/default/test-pod-3-100",
					Timestamp: 1000 * 1e9, // Before query start
					Type:      models.EventTypeUpdate,
					Resource: models.ResourceMetadata{
						UID:       "test-uid-3",
						Group:     "",
						Version:   "v1",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "test-pod-3",
					},
					Data: json.RawMessage(`{"status":{"phase":"Running"}}`),
				},
			},
			expectedSegmentCnt: 1, // No leading segment (state is before query)
			expectedFirstStart: 1000,
			expectedLeadingMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := NewResourceBuilder()
			segments := rb.BuildStatusSegmentsFromEventsWithQueryTime(tt.events, tt.queryStartTimeNs)

			if len(segments) != tt.expectedSegmentCnt {
				t.Errorf("Expected %d segments, got %d", tt.expectedSegmentCnt, len(segments))
			}

			if len(segments) > 0 {
				if segments[0].StartTime != tt.expectedFirstStart {
					t.Errorf("Expected first segment to start at %d, got %d",
						tt.expectedFirstStart, segments[0].StartTime)
				}

				if tt.expectedLeadingMsg != "" {
					if segments[0].Message != tt.expectedLeadingMsg {
						t.Errorf("Expected leading segment message %q, got %q",
							tt.expectedLeadingMsg, segments[0].Message)
					}
				}
			}
		})
	}
}

// TestStateSnapshotTimestampAdjustment verifies that state snapshots
// before query start get adjusted to query start time
func TestStateSnapshotTimestampAdjustment(t *testing.T) {
	queryStartTimeNs := int64(2000 * 1e9) // Query starts at 2000s
	stateTimestamp := int64(1000 * 1e9)   // State is from 1000s (before query start)

	// Simulate what getStateSnapshotEventsWithDeleted does
	eventTimestamp := stateTimestamp
	if eventTimestamp < queryStartTimeNs {
		eventTimestamp = queryStartTimeNs
	}

	// The timestamp should now be at query start
	expectedTimestamp := queryStartTimeNs
	if eventTimestamp != expectedTimestamp {
		t.Errorf("Expected timestamp to be adjusted to %d, got %d",
			expectedTimestamp, eventTimestamp)
	}

	// Build segments from this event
	rb := NewResourceBuilder()
	event := models.Event{
		ID:        "state-test-resource-100",
		Timestamp: eventTimestamp,
		Type:      models.EventTypeUpdate,
		Resource: models.ResourceMetadata{
			UID:       "test-uid",
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "test-pod",
		},
		Data: json.RawMessage(`{"status":{"phase":"Running"}}`),
	}

	segments := rb.BuildStatusSegmentsFromEventsWithQueryTime([]models.Event{event}, queryStartTimeNs)

	if len(segments) != 1 {
		t.Fatalf("Expected 1 segment, got %d", len(segments))
	}

	// The segment should start at query start time (2000s)
	if segments[0].StartTime != queryStartTimeNs/1e9 {
		t.Errorf("Expected segment to start at %d, got %d",
			queryStartTimeNs/1e9, segments[0].StartTime)
	}
}

// TestLeadingSegmentNotCreatedForNonStateEvents ensures we don't create
// leading segments for regular events
func TestLeadingSegmentNotCreatedForNonStateEvents(t *testing.T) {
	queryStartTimeNs := int64(1000 * 1e9)
	rb := NewResourceBuilder()

	// Event that happens after query start but is NOT a state snapshot
	events := []models.Event{
		{
			ID:        "regular-event-1", // No "state-" prefix
			Timestamp: int64(1500 * 1e9),
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				UID:       "test-uid",
				Group:     "",
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test-pod",
			},
			Data: json.RawMessage(`{"status":{"phase":"Running"}}`),
		},
	}

	segments := rb.BuildStatusSegmentsFromEventsWithQueryTime(events, queryStartTimeNs)

	// Should have exactly 1 segment (no leading segment)
	if len(segments) != 1 {
		t.Errorf("Expected 1 segment (no leading), got %d", len(segments))
	}

	// The segment should start at the event time, not query start
	if segments[0].StartTime != 1500 {
		t.Errorf("Expected segment to start at 1500, got %d", segments[0].StartTime)
	}

	// Message should NOT be the leading segment message
	if strings.Contains(segments[0].Message, "existed before query") {
		t.Errorf("Regular event should not have leading segment message")
	}
}
