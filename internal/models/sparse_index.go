package models

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
	if s.TotalSegments < int32(len(s.Entries)) {
		return NewValidationError("totalSegments cannot be less than number of entries")
	}

	return nil
}

// FindSegmentsForTimeRange returns all segment IDs that might contain events in the given time range
// Uses binary search to find the candidate segments
func (s *SparseTimestampIndex) FindSegmentsForTimeRange(startTime, endTime int64) []int32 {
	if len(s.Entries) == 0 {
		return []int32{}
	}

	// Find the first entry with timestamp >= startTime
	startIdx := 0
	for i, entry := range s.Entries {
		if entry.Timestamp >= startTime {
			if i > 0 {
				startIdx = i - 1 // Include the segment before to catch overlaps
			} else {
				startIdx = 0
			}
			break
		}
		if i == len(s.Entries)-1 {
			startIdx = i // If all timestamps are before startTime, use the last one
		}
	}

	// Find the last entry with timestamp <= endTime
	endIdx := len(s.Entries) - 1
	for i := len(s.Entries) - 1; i >= 0; i-- {
		if s.Entries[i].Timestamp <= endTime {
			endIdx = i
			break
		}
		if i == 0 {
			endIdx = 0
		}
	}

	// Ensure start index doesn't exceed end index
	if startIdx > endIdx {
		return []int32{}
	}

	// Collect all segment IDs in range
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
