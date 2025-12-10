package models

import "sort"

// SparseTimestampIndex maps timestamp ranges to segment offsets for fast temporal filtering
type SparseTimestampIndex struct {
	// Entries is an array of index entries sorted by timestamp
	Entries []IndexEntry `json:"entries"`

	// TotalSegments is the number of segments in the file
	TotalSegments int32 `json:"totalSegments"`
}

// IndexEntry is a single entry in the sparse timestamp index
type IndexEntry struct {
	// Timestamp is a representative timestamp for this entry
	Timestamp int64 `json:"timestamp"`

	// SegmentID points to the segment with events near this time
	SegmentID int32 `json:"segmentId"`

	// Offset is the byte offset of the segment in the file
	Offset int64 `json:"offset"`
}

// Validate checks that the index is well-formed
func (s *SparseTimestampIndex) Validate() error {
	// Index must have at least one entry
	if len(s.Entries) == 0 {
		return NewValidationError("entries must not be empty")
	}

	// Entries should be sorted by timestamp
	for i := 1; i < len(s.Entries); i++ {
		if s.Entries[i].Timestamp < s.Entries[i-1].Timestamp {
			return NewValidationError("entries must be sorted by timestamp")
		}
	}

	// TotalSegments must match number of unique segment IDs
	// (This is a loose check; strict validation would require all segment IDs to be present)
	if s.TotalSegments < int32(len(s.Entries)) { //nolint:gosec // safe conversion: entry count is reasonable
		return NewValidationError("totalSegments cannot be less than number of entries")
	}

	return nil
}

// FindSegmentsForTimeRange returns all segment IDs that might contain events in the given time range
// Uses binary search for O(log n) performance on large indices
func (s *SparseTimestampIndex) FindSegmentsForTimeRange(startTime, endTime int64) []int32 {
	if len(s.Entries) == 0 {
		return []int32{}
	}

	// Use binary search to find the first entry with timestamp >= startTime
	startIdx := sort.Search(len(s.Entries), func(i int) bool {
		return s.Entries[i].Timestamp >= startTime
	})

	// If no exact match, include the segment before to catch overlaps
	if startIdx > 0 {
		startIdx--
	}

	// Use binary search to find the last entry with timestamp <= endTime
	// search for the first entry > endTime, then subtract 1
	endIdx := sort.Search(len(s.Entries), func(i int) bool {
		return s.Entries[i].Timestamp > endTime
	})

	// If search found an element, move to previous; if not found, use last element
	if endIdx > 0 {
		endIdx--
	} else if len(s.Entries) > 0 {
		// If no entries found > endTime, use the last entry
		endIdx = len(s.Entries) - 1
	}

	// Ensure start index doesn't exceed end index
	if startIdx > endIdx {
		return []int32{}
	}

	// Collect all segment IDs in range, avoiding duplicates
	segmentIDs := make([]int32, 0)
	seen := make(map[int32]bool)
	for i := startIdx; i <= endIdx; i++ {
		segmentID := s.Entries[i].SegmentID
		if !seen[segmentID] {
			segmentIDs = append(segmentIDs, segmentID)
			seen[segmentID] = true
		}
	}

	return segmentIDs
}

// GetSegmentOffset returns the byte offset of a specific segment
// Linear search is acceptable here since segment IDs are not necessarily ordered
func (s *SparseTimestampIndex) GetSegmentOffset(segmentID int32) int64 {
	for _, entry := range s.Entries {
		if entry.SegmentID == segmentID {
			return entry.Offset
		}
	}
	return -1 // Segment not found
}

// IsValid checks if the index is valid
func (s *SparseTimestampIndex) IsValid() bool {
	return s.Validate() == nil
}

// GetMinTimestamp returns the earliest timestamp in the index
func (s *SparseTimestampIndex) GetMinTimestamp() int64 {
	if len(s.Entries) == 0 {
		return 0
	}
	return s.Entries[0].Timestamp
}

// GetMaxTimestamp returns the latest timestamp in the index
func (s *SparseTimestampIndex) GetMaxTimestamp() int64 {
	if len(s.Entries) == 0 {
		return 0
	}
	return s.Entries[len(s.Entries)-1].Timestamp
}
