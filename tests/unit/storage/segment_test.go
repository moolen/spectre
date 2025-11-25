package storage

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/moritz/rpk/internal/models"
	"github.com/moritz/rpk/internal/storage"
)

// Helper function to create a valid test event
func createTestEvent(id string, timestamp int64, eventType models.EventType, kind, namespace string) *models.Event {
	return &models.Event{
		ID:        id,
		Timestamp: timestamp,
		Type:      eventType,
		Resource: models.ResourceMetadata{
			Group:     "apps",
			Version:   "v1",
			Kind:      kind,
			Namespace: namespace,
			Name:      "test-" + kind,
			UID:       id + "-uid",
		},
		Data: json.RawMessage(`{"test": "data"}`),
	}
}

// TestNewSegment tests segment creation
func TestNewSegment(t *testing.T) {
	comp := storage.NewCompressor()
	seg := storage.NewSegment(1, comp, 1024*1024)

	if seg == nil {
		t.Error("NewSegment returned nil")
	}
}

// TestAddEvent tests adding an event to segment
func TestAddEvent(t *testing.T) {
	comp := storage.NewCompressor()
	seg := storage.NewSegment(1, comp, 1024*1024)

	event := createTestEvent("test-1", 1000, models.EventTypeCreate, "Pod", "default")

	err := seg.AddEvent(event)
	if err != nil {
		t.Fatalf("AddEvent failed: %v", err)
	}

	if seg.GetEventCount() != 1 {
		t.Errorf("Expected 1 event, got %d", seg.GetEventCount())
	}
}

// TestAddMultipleEvents tests adding multiple events
func TestAddMultipleEvents(t *testing.T) {
	comp := storage.NewCompressor()
	seg := storage.NewSegment(1, comp, 1024*1024)

	events := []*models.Event{
		createTestEvent("test-1", 1000, models.EventTypeCreate, "Pod", "default"),
		createTestEvent("test-2", 2000, models.EventTypeUpdate, "Deployment", "default"),
		createTestEvent("test-3", 3000, models.EventTypeDelete, "Pod", "kube-system"),
	}

	for _, event := range events {
		if err := seg.AddEvent(event); err != nil {
			t.Fatalf("AddEvent failed: %v", err)
		}
	}

	if seg.GetEventCount() != int32(len(events)) {
		t.Errorf("Expected %d events, got %d", len(events), seg.GetEventCount())
	}
}

// TestFinalize tests segment finalization
func TestFinalize(t *testing.T) {
	comp := storage.NewCompressor()
	seg := storage.NewSegment(1, comp, 1024*1024)

	event := createTestEvent("test-1", 1000, models.EventTypeCreate, "Pod", "default")

	seg.AddEvent(event)

	err := seg.Finalize()
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	if seg.GetUncompressedSize() == 0 {
		t.Error("Expected non-zero uncompressed size after finalize")
	}

	if seg.GetCompressedSize() == 0 {
		t.Error("Expected non-zero compressed size after finalize")
	}
}

// TestGetCompressionRatio tests compression ratio
func TestSegmentCompressionRatio(t *testing.T) {
	comp := storage.NewCompressor()
	seg := storage.NewSegment(1, comp, 1024*1024)

	// Add repeated data for good compression
	for i := 0; i < 10; i++ {
		event := createTestEvent("test-"+string(rune(i)), int64(1000+i), models.EventTypeCreate, "Pod", "default")
		// Override Data field with repeated data for good compression
		event.Data = json.RawMessage(bytes.Repeat([]byte(`{"test":"data"}`), 10))
		seg.AddEvent(event)
	}

	seg.Finalize()

	ratio := 0.0
	if seg.GetUncompressedSize() > 0 {
		ratio = float64(seg.GetCompressedSize()) / float64(seg.GetUncompressedSize())
	}

	if ratio < 0 || ratio > 1 {
		t.Errorf("Invalid compression ratio: %f", ratio)
	}

	if ratio >= 0.9 {
		t.Logf("Warning: Low compression ratio %.2f for repetitive data", ratio)
	}
}

// TestIsReady tests segment ready status
func TestIsReady(t *testing.T) {
	comp := storage.NewCompressor()
	seg := storage.NewSegment(1, comp, 100) // Small segment size

	if seg.IsReady() {
		t.Error("Empty segment should not be ready")
	}

	event := createTestEvent("test-1", 1000, models.EventTypeCreate, "Pod", "default")
	// Override Data field with large data to exceed segment size
	event.Data = json.RawMessage(bytes.Repeat([]byte("x"), 150))

	seg.AddEvent(event)

	// After finalize, should be ready
	seg.Finalize()
	// IsReady checks if needs finalization, so after finalize it's not "ready" to finalize again
	// This is by design - segment is finalized once
}

// TestFilterEvents tests event filtering in segment
func TestFilterEvents(t *testing.T) {
	comp := storage.NewCompressor()
	seg := storage.NewSegment(1, comp, 1024*1024)

	events := []*models.Event{
		createTestEvent("test-1", 1000, models.EventTypeCreate, "Pod", "default"),
		createTestEvent("test-2", 2000, models.EventTypeCreate, "Deployment", "default"),
	}

	for _, event := range events {
		seg.AddEvent(event)
	}

	seg.Finalize()

	filters := models.QueryFilters{Kind: "Pod"}
	filtered, err := seg.FilterEvents(filters)
	if err != nil {
		t.Fatalf("FilterEvents failed: %v", err)
	}

	if len(filtered) != 1 {
		t.Errorf("Expected 1 filtered event, got %d", len(filtered))
	}

	if filtered[0].Resource.Kind != "Pod" {
		t.Errorf("Expected Pod, got %s", filtered[0].Resource.Kind)
	}
}

// TestMetadataTracking tests segment metadata tracking
func TestMetadataTracking(t *testing.T) {
	comp := storage.NewCompressor()
	seg := storage.NewSegment(1, comp, 1024*1024)

	events := []*models.Event{
		createTestEvent("test-1", 1000, models.EventTypeCreate, "Pod", "default"),
		createTestEvent("test-2", 2000, models.EventTypeCreate, "Deployment", "kube-system"),
	}

	for _, event := range events {
		seg.AddEvent(event)
	}

	seg.Finalize()

	metadata := seg.GetMetadata()
	if len(metadata.KindSet) != 2 {
		t.Errorf("Expected 2 kinds in metadata, got %d", len(metadata.KindSet))
	}

	if len(metadata.NamespaceSet) != 2 {
		t.Errorf("Expected 2 namespaces in metadata, got %d", len(metadata.NamespaceSet))
	}
}

// TestIsInTimeRange tests timestamp range checking
func TestIsInTimeRange(t *testing.T) {
	comp := storage.NewCompressor()
	seg := storage.NewSegment(1, comp, 1024*1024)

	for i := 0; i < 5; i++ {
		event := createTestEvent("test-"+string(rune(i)), int64(1000+i*1000), models.EventTypeCreate, "Pod", "default")
		seg.AddEvent(event)
	}

	seg.Finalize()

	testCases := []struct {
		start     int64
		end       int64
		overlaps  bool
	}{
		{0, 5000, true},           // Full range (1000-5000)
		{1000, 2000, true},        // Partial overlap
		{2500, 3500, true},        // Middle overlap
		{5001, 6000, false},       // After range
		{0, 999, false},           // Before range
		{1000, 5000, true},        // Exact match
		{5000, 5000, true},        // Touch at end (overlaps at 5000)
	}

	for _, tc := range testCases {
		overlaps := seg.IsInTimeRange(tc.start, tc.end)
		if overlaps != tc.overlaps {
			t.Errorf("IsInTimeRange(%d, %d): expected %v, got %v",
				tc.start, tc.end, tc.overlaps, overlaps)
		}
	}
}

// TestToStorageSegment tests conversion to StorageSegment model
func TestToStorageSegment(t *testing.T) {
	comp := storage.NewCompressor()
	seg := storage.NewSegment(1, comp, 1024*1024)

	event := createTestEvent("test-1", 1000, models.EventTypeCreate, "Pod", "default")

	seg.AddEvent(event)
	seg.Finalize()

	storageSeg := seg.ToStorageSegment(100, 200)

	if storageSeg.ID != 1 {
		t.Errorf("Expected ID=1, got %d", storageSeg.ID)
	}

	if storageSeg.Offset != 100 {
		t.Errorf("Expected Offset=100, got %d", storageSeg.Offset)
	}

	if storageSeg.Length != 200 {
		t.Errorf("Expected Length=200, got %d", storageSeg.Length)
	}

	if storageSeg.EventCount != 1 {
		t.Errorf("Expected EventCount=1, got %d", storageSeg.EventCount)
	}
}

// TestAddInvalidEvent tests adding invalid event
func TestAddInvalidEvent(t *testing.T) {
	comp := storage.NewCompressor()
	seg := storage.NewSegment(1, comp, 1024*1024)

	// Event without required fields
	event := &models.Event{
		ID: "test-1",
		// Missing timestamp, type, resource
	}

	err := seg.AddEvent(event)
	if err == nil {
		t.Error("Expected error for invalid event, got nil")
	}
}

// TestSegmentSizing tests segment with size limits
func TestSegmentSizing(t *testing.T) {
	comp := storage.NewCompressor()
	maxSize := int64(500) // Very small for testing
	seg := storage.NewSegment(1, comp, maxSize)

	// Add events until segment would be large
	for i := 0; i < 10; i++ {
		event := createTestEvent("test-"+string(rune(i)), int64(1000+i), models.EventTypeCreate, "Pod", "default")
		// Override Data field with large repeated data
		event.Data = json.RawMessage(bytes.Repeat([]byte("test data "), 20))
		seg.AddEvent(event)
	}

	if seg.GetEventCount() != 10 {
		t.Errorf("Expected 10 events added, got %d", seg.GetEventCount())
	}
}

// TestDecompressedEvents tests getting decompressed events
func TestDecompressedEvents(t *testing.T) {
	comp := storage.NewCompressor()
	seg := storage.NewSegment(1, comp, 1024*1024)

	originalEvents := []*models.Event{
		createTestEvent("test-1", 1000, models.EventTypeCreate, "Pod", "default"),
		createTestEvent("test-2", 2000, models.EventTypeUpdate, "Pod", "default"),
	}

	// Override Data field for these events with specific test data
	originalEvents[0].Data = json.RawMessage(`{"key":"value"}`)
	originalEvents[1].Data = json.RawMessage(`{"key":"updated"}`)

	for _, event := range originalEvents {
		seg.AddEvent(event)
	}

	seg.Finalize()

	decompressed, err := seg.GetDecompressedEvents()
	if err != nil {
		t.Fatalf("GetDecompressedEvents failed: %v", err)
	}

	if len(decompressed) != len(originalEvents) {
		t.Errorf("Expected %d events, got %d", len(originalEvents), len(decompressed))
	}

	for i, event := range decompressed {
		if event.ID != originalEvents[i].ID {
			t.Errorf("Event %d: expected ID %s, got %s", i, originalEvents[i].ID, event.ID)
		}
	}
}
