package models

import "time"

// QueryRequest represents an API query for historical events
type QueryRequest struct {
	// StartTimestamp is the start of the time window (Unix seconds, inclusive)
	StartTimestamp int64 `json:"startTimestamp"`

	// EndTimestamp is the end of the time window (Unix seconds, inclusive)
	EndTimestamp int64 `json:"endTimestamp"`

	// Filters specifies which events to return based on resource dimensions
	Filters QueryFilters `json:"filters"`

	// Pagination contains optional pagination parameters
	// If nil, defaults to DefaultPageSize (100)
	Pagination *PaginationRequest `json:"pagination,omitempty"`
}

// Validate checks that the query request is well-formed
func (q *QueryRequest) Validate() error {
	// Validate timestamps
	if q.StartTimestamp < 0 || q.EndTimestamp < 0 {
		return NewValidationError("timestamps must be non-negative")
	}

	if q.StartTimestamp > q.EndTimestamp {
		return NewValidationError("startTimestamp must be less than or equal to endTimestamp")
	}

	return nil
}

// GetStartTime returns the start timestamp as a time.Time
func (q *QueryRequest) GetStartTime() time.Time {
	return time.Unix(q.StartTimestamp, 0)
}

// GetEndTime returns the end timestamp as a time.Time
func (q *QueryRequest) GetEndTime() time.Time {
	return time.Unix(q.EndTimestamp, 0)
}

// GetDuration returns the duration of the query time window
func (q *QueryRequest) GetDuration() time.Duration {
	return time.Duration((q.EndTimestamp - q.StartTimestamp) * int64(time.Second))
}

// IsValid checks if the query request is valid
func (q *QueryRequest) IsValid() bool {
	return q.Validate() == nil
}

// HasFilters checks if any filters are specified
func (q *QueryRequest) HasFilters() bool {
	return !q.Filters.IsEmpty()
}
