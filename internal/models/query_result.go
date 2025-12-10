package models

import "time"

// QueryResult is the response containing matching events from a query
type QueryResult struct {
	// Events is the array of matching events
	Events []Event `json:"events"`

	// Count is the number of events returned
	Count int32 `json:"count"`

	// ExecutionTimeMs is the query execution duration in milliseconds
	ExecutionTimeMs int32 `json:"executionTimeMs"`

	// SegmentsScanned is the total number of segments examined
	SegmentsScanned int32 `json:"segmentsScanned"`

	// SegmentsSkipped is the number of segments skipped via metadata filtering
	SegmentsSkipped int32 `json:"segmentsSkipped"`

	// FilesSearched is the number of hourly files searched
	FilesSearched int32 `json:"filesSearched"`
}

// Validate checks that the query result is well-formed
func (r *QueryResult) Validate() error {
	// Count must match the length of events
	if r.Count != int32(len(r.Events)) { //nolint:gosec // safe conversion: event count is reasonable
		return NewValidationError("count does not match the number of events")
	}

	// Execution time must be non-negative
	if r.ExecutionTimeMs < 0 {
		return NewValidationError("executionTimeMs must be non-negative")
	}

	// Segment counts must be non-negative
	if r.SegmentsScanned < 0 {
		return NewValidationError("segmentsScanned must be non-negative")
	}
	if r.SegmentsSkipped < 0 {
		return NewValidationError("segmentsSkipped must be non-negative")
	}

	// Files searched must be non-negative
	if r.FilesSearched < 0 {
		return NewValidationError("filesSearched must be non-negative")
	}

	// All events must be valid
	for i, event := range r.Events {
		if err := event.Validate(); err != nil {
			return NewValidationError("event %d is invalid: %v", i, err)
		}
	}

	return nil
}

// IsEmpty checks if the result contains no events
func (r *QueryResult) IsEmpty() bool {
	return r.Count == 0 || len(r.Events) == 0
}

// GetExecutionTime returns the execution time as a time.Duration
func (r *QueryResult) GetExecutionTime() time.Duration {
	return time.Duration(r.ExecutionTimeMs) * time.Millisecond
}

// GetTotalSegmentsExamined returns the total segments examined (scanned + skipped)
func (r *QueryResult) GetTotalSegmentsExamined() int32 {
	return r.SegmentsScanned + r.SegmentsSkipped
}

// GetSegmentSkipRatio returns the percentage of segments skipped (0.0 to 1.0)
func (r *QueryResult) GetSegmentSkipRatio() float64 {
	total := r.GetTotalSegmentsExamined()
	if total == 0 {
		return 0.0
	}
	return float64(r.SegmentsSkipped) / float64(total)
}

// IsValid checks if the query result is valid
func (r *QueryResult) IsValid() bool {
	return r.Validate() == nil
}
