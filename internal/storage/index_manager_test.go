package storage

import (
	"testing"
)

func TestNewIndexManager(t *testing.T) {
	manager := NewIndexManager()
	if manager == nil {
		t.Fatal("expected non-nil IndexManager")
	}
	if manager.entries == nil {
		t.Error("entries map not initialized")
	}
}

func TestIndexManagerAddEntry(t *testing.T) {
	manager := NewIndexManager()

	manager.AddEntry(1, 1000, 0)
	manager.AddEntry(2, 2000, 100)

	if manager.GetSegmentCount() != 2 {
		t.Errorf("expected 2 entries, got %d", manager.GetSegmentCount())
	}

	offset, err := manager.GetSegmentOffset(1)
	if err != nil {
		t.Fatalf("GetSegmentOffset failed: %v", err)
	}
	if offset != 0 {
		t.Errorf("expected offset 0, got %d", offset)
	}

	offset, err = manager.GetSegmentOffset(2)
	if err != nil {
		t.Fatalf("GetSegmentOffset failed: %v", err)
	}
	if offset != 100 {
		t.Errorf("expected offset 100, got %d", offset)
	}
}

func TestIndexManagerGetSegmentOffset_NotFound(t *testing.T) {
	manager := NewIndexManager()

	_, err := manager.GetSegmentOffset(999)
	if err == nil {
		t.Error("expected error for non-existent segment")
	}
}

func TestIndexManagerFindSegmentsInTimeRange(t *testing.T) {
	manager := NewIndexManager()

	// Add entries at different timestamps
	manager.AddEntry(1, 1000, 0)
	manager.AddEntry(2, 2000, 100)
	manager.AddEntry(3, 3000, 200)
	manager.AddEntry(4, 4000, 300)

	// Find segments in range [1500, 3500]
	segments := manager.FindSegmentsInTimeRange(1500, 3500)
	if len(segments) < 2 {
		t.Errorf("expected at least 2 segments, got %d", len(segments))
	}

	// Should include segments 2 and 3
	found2 := false
	found3 := false
	for _, segID := range segments {
		if segID == 2 {
			found2 = true
		}
		if segID == 3 {
			found3 = true
		}
	}

	if !found2 {
		t.Error("expected to find segment 2")
	}
	if !found3 {
		t.Error("expected to find segment 3")
	}
}

func TestIndexManagerFindSegmentsInTimeRange_BeforeStart(t *testing.T) {
	manager := NewIndexManager()

	manager.AddEntry(1, 1000, 0)
	manager.AddEntry(2, 2000, 100)

	// Find segments before range start (should include earlier segments)
	segments := manager.FindSegmentsInTimeRange(1500, 3000)
	if len(segments) == 0 {
		t.Error("expected to find segments")
	}
}

func TestIndexManagerFindSegmentsInTimeRange_Empty(t *testing.T) {
	manager := NewIndexManager()

	segments := manager.FindSegmentsInTimeRange(1000, 2000)
	if len(segments) != 0 {
		t.Errorf("expected 0 segments, got %d", len(segments))
	}
}

func TestIndexManagerGetSegmentCount(t *testing.T) {
	manager := NewIndexManager()

	if manager.GetSegmentCount() != 0 {
		t.Errorf("expected 0 segments, got %d", manager.GetSegmentCount())
	}

	manager.AddEntry(1, 1000, 0)
	manager.AddEntry(2, 2000, 100)

	if manager.GetSegmentCount() != 2 {
		t.Errorf("expected 2 segments, got %d", manager.GetSegmentCount())
	}
}

func TestIndexManagerBuildSparseIndex(t *testing.T) {
	manager := NewIndexManager()

	manager.AddEntry(1, 1000, 0)
	manager.AddEntry(2, 2000, 100)
	manager.AddEntry(3, 3000, 200)

	index := manager.BuildSparseIndex()
	if index.TotalSegments != 3 {
		t.Errorf("expected 3 segments, got %d", index.TotalSegments)
	}
	if len(index.Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(index.Entries))
	}
}

func TestIndexManagerGetEntriesInRange(t *testing.T) {
	manager := NewIndexManager()

	manager.AddEntry(1, 1000, 0)
	manager.AddEntry(2, 2000, 100)
	manager.AddEntry(3, 3000, 200)
	manager.AddEntry(4, 4000, 300)

	entries := manager.GetEntriesInRange(1500, 3500)
	if len(entries) < 2 {
		t.Errorf("expected at least 2 entries, got %d", len(entries))
	}

	// Verify entries are in range
	for _, entry := range entries {
		if entry.Timestamp < 1500 || entry.Timestamp > 3500 {
			t.Errorf("entry timestamp %d outside range [1500, 3500]", entry.Timestamp)
		}
	}
}

func TestIndexManagerGetEntriesInRange_Empty(t *testing.T) {
	manager := NewIndexManager()

	entries := manager.GetEntriesInRange(1000, 2000)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestIndexManagerClear(t *testing.T) {
	manager := NewIndexManager()

	manager.AddEntry(1, 1000, 0)
	manager.AddEntry(2, 2000, 100)

	if manager.GetSegmentCount() != 2 {
		t.Errorf("expected 2 segments, got %d", manager.GetSegmentCount())
	}

	manager.Clear()

	if manager.GetSegmentCount() != 0 {
		t.Errorf("expected 0 segments after clear, got %d", manager.GetSegmentCount())
	}

	_, err := manager.GetSegmentOffset(1)
	if err == nil {
		t.Error("expected error after clear")
	}
}

func TestIndexManagerGetAllEntries(t *testing.T) {
	manager := NewIndexManager()

	manager.AddEntry(1, 1000, 0)
	manager.AddEntry(2, 2000, 100)
	manager.AddEntry(3, 3000, 200)

	entries := manager.GetAllEntries()
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// Verify all entries are present
	segmentIDs := make(map[int32]bool)
	for _, entry := range entries {
		segmentIDs[entry.SegmentID] = true
	}

	if !segmentIDs[1] || !segmentIDs[2] || !segmentIDs[3] {
		t.Error("missing expected segment IDs")
	}
}

func TestIndexManagerGetAllEntries_Empty(t *testing.T) {
	manager := NewIndexManager()

	entries := manager.GetAllEntries()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestIndexManagerMultipleEntries_SameSegmentID(t *testing.T) {
	manager := NewIndexManager()

	// Adding entry with same segment ID should overwrite
	manager.AddEntry(1, 1000, 0)
	manager.AddEntry(1, 2000, 100)

	if manager.GetSegmentCount() != 1 {
		t.Errorf("expected 1 entry (overwritten), got %d", manager.GetSegmentCount())
	}

	offset, err := manager.GetSegmentOffset(1)
	if err != nil {
		t.Fatalf("GetSegmentOffset failed: %v", err)
	}
	if offset != 100 {
		t.Errorf("expected offset 100 (from second entry), got %d", offset)
	}
}
