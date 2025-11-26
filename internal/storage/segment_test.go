package storage

import (
	"testing"
	"time"

	"github.com/moritz/rpk/internal/models"
)

func TestNewSegment(t *testing.T) {
	compressor := NewCompressor()
	segment := NewSegment(0, compressor, 1024)

	if segment.ID != 0 {
		t.Errorf("expected segment ID 0, got %d", segment.ID)
	}

	if segment.maxSize != 1024 {
		t.Errorf("expected max size 1024, got %d", segment.maxSize)
	}

	if len(segment.events) != 0 {
		t.Errorf("expected empty events, got %d", len(segment.events))
	}

	if segment.compressor == nil {
		t.Error("expected compressor to be set")
	}
}

func TestSegmentAddEvent(t *testing.T) {
	compressor := NewCompressor()
	segment := NewSegment(0, compressor, 10240)

	event := createTestEvent("pod-1", "default", "Pod", time.Now().UnixNano())

	if err := segment.AddEvent(event); err != nil {
		t.Fatalf("failed to add event: %v", err)
	}

	if segment.GetEventCount() != 1 {
		t.Errorf("expected 1 event, got %d", segment.GetEventCount())
	}

	if segment.startTimestamp == 0 {
		t.Error("expected start timestamp to be set")
	}

	if segment.endTimestamp == 0 {
		t.Error("expected end timestamp to be set")
	}
}

func TestSegmentAddMultipleEvents(t *testing.T) {
	compressor := NewCompressor()
	segment := NewSegment(0, compressor, 10240)

	now := time.Now()
	for i := 0; i < 10; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", now.Add(time.Duration(i)*time.Second).UnixNano())
		if err := segment.AddEvent(event); err != nil {
			t.Fatalf("failed to add event %d: %v", i, err)
		}
	}

	if segment.GetEventCount() != 10 {
		t.Errorf("expected 10 events, got %d", segment.GetEventCount())
	}

	// Verify timestamps
	if segment.startTimestamp > segment.endTimestamp {
		t.Error("start timestamp should be <= end timestamp")
	}
}

func TestSegmentOutOfOrderEvents(t *testing.T) {
	compressor := NewCompressor()
	segment := NewSegment(0, compressor, 10240)

	now := time.Now()
	// Add events in order
	event1 := createTestEvent("pod-1", "default", "Pod", now.UnixNano())
	segment.AddEvent(event1)

	// Add out-of-order event (within reordering window)
	event2 := createTestEvent("pod-2", "default", "Pod", now.Add(-2*time.Second).UnixNano())
	segment.AddEvent(event2)

	// Out-of-order event should be buffered
	if segment.GetBufferedEventCount() == 0 {
		t.Error("expected out-of-order event to be buffered")
	}
}

func TestSegmentFlushBufferedEvents(t *testing.T) {
	compressor := NewCompressor()
	segment := NewSegment(0, compressor, 10240)

	now := time.Now()
	// Add events with some out-of-order
	event1 := createTestEvent("pod-1", "default", "Pod", now.UnixNano())
	segment.AddEvent(event1)

	event2 := createTestEvent("pod-2", "default", "Pod", now.Add(-1*time.Second).UnixNano())
	segment.AddEvent(event2)

	// Force flush
	segment.FlushBufferedEvents()

	if segment.GetBufferedEventCount() != 0 {
		t.Error("expected buffered events to be flushed")
	}

	// Events should be in order now
	events := segment.GetEvents()
	if len(events) < 2 {
		t.Fatal("expected at least 2 events")
	}

	// First event should have earlier timestamp
	if events[0].Timestamp > events[1].Timestamp {
		t.Error("events should be sorted by timestamp")
	}
}

func TestSegmentFinalize(t *testing.T) {
	compressor := NewCompressor()
	segment := NewSegment(0, compressor, 10240)

	// Add events
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		segment.AddEvent(event)
	}

	if err := segment.Finalize(); err != nil {
		t.Fatalf("failed to finalize segment: %v", err)
	}

	if segment.GetUncompressedSize() == 0 {
		t.Error("expected uncompressed size to be set")
	}

	if segment.GetCompressedSize() == 0 {
		t.Error("expected compressed size to be set")
	}

	metadata := segment.GetMetadata()
	if metadata.MinTimestamp == 0 {
		t.Error("expected min timestamp in metadata")
	}
}

func TestSegmentFinalizeEmpty(t *testing.T) {
	compressor := NewCompressor()
	segment := NewSegment(0, compressor, 10240)

	err := segment.Finalize()
	if err == nil {
		t.Error("expected error when finalizing empty segment")
	}
}

func TestSegmentGetDecompressedEvents(t *testing.T) {
	compressor := NewCompressor()
	segment := NewSegment(0, compressor, 10240)

	// Add events
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		segment.AddEvent(event)
	}

	segment.Finalize()

	events, err := segment.GetDecompressedEvents()
	if err != nil {
		t.Fatalf("failed to get decompressed events: %v", err)
	}

	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}
}

func TestSegmentIsReady(t *testing.T) {
	compressor := NewCompressor()
	segment := NewSegment(0, compressor, 100) // Small size

	// Add events until segment is ready
	for i := 0; i < 100; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		segment.AddEvent(event)
		if segment.IsReady() {
			break
		}
	}

	// Segment should be ready when size threshold is reached
	if !segment.IsReady() {
		t.Error("expected segment to be ready")
	}
}

func TestSegmentMatchesFilters(t *testing.T) {
	compressor := NewCompressor()
	segment := NewSegment(0, compressor, 10240)

	// Add events with specific kind
	event := createTestEvent("pod-1", "default", "Pod", time.Now().UnixNano())
	segment.AddEvent(event)
	segment.Finalize()

	// Test matching filter
	filters := models.QueryFilters{Kind: "Pod"}
	if !segment.MatchesFilters(filters) {
		t.Error("expected segment to match Pod filter")
	}

	// Test non-matching filter
	filters = models.QueryFilters{Kind: "Service"}
	if segment.MatchesFilters(filters) {
		t.Error("expected segment to not match Service filter")
	}
}

func TestSegmentIsInTimeRange(t *testing.T) {
	compressor := NewCompressor()
	segment := NewSegment(0, compressor, 10240)

	now := time.Now()
	event := createTestEvent("pod-1", "default", "Pod", now.UnixNano())
	segment.AddEvent(event)

	// Test overlapping time range
	startTime := now.Add(-1 * time.Hour).UnixNano()
	endTime := now.Add(1 * time.Hour).UnixNano()

	if !segment.IsInTimeRange(startTime, endTime) {
		t.Error("expected segment to be in time range")
	}

	// Test non-overlapping time range
	startTime = now.Add(2 * time.Hour).UnixNano()
	endTime = now.Add(3 * time.Hour).UnixNano()

	if segment.IsInTimeRange(startTime, endTime) {
		t.Error("expected segment to not be in time range")
	}
}

func TestSegmentWriteToBuffer(t *testing.T) {
	compressor := NewCompressor()
	segment := NewSegment(0, compressor, 10240)

	// Add events
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		segment.AddEvent(event)
	}

	segment.Finalize()

	buf := segment.WriteToBuffer()
	if buf.Len() == 0 {
		t.Error("expected buffer to have data")
	}
}

func TestSegmentSetReorderingWindow(t *testing.T) {
	compressor := NewCompressor()
	segment := NewSegment(0, compressor, 10240)

	segment.SetReorderingWindow(10000) // 10 seconds

	if segment.reorderingWindowMs != 10000 {
		t.Errorf("expected reordering window 10000, got %d", segment.reorderingWindowMs)
	}
}


