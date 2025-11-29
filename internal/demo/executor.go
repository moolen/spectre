package demo

import (
	"sort"
	"time"

	"github.com/moolen/spectre/internal/models"
)

// DemoQueryExecutor implements the QueryExecutor interface for demo mode
// It returns pre-loaded demo events instead of querying actual storage
type DemoQueryExecutor struct{}

// NewDemoQueryExecutor creates a new DemoQueryExecutor with demo data
func NewDemoQueryExecutor() *DemoQueryExecutor {
	return &DemoQueryExecutor{}
}

// Execute executes a query against demo events
func (dqe *DemoQueryExecutor) Execute(query *models.QueryRequest) (*models.QueryResult, error) {
	start := time.Now()

	// Validate query
	if err := query.Validate(); err != nil {
		return nil, err
	}

	events := GetDemoEvents(query.StartTimestamp)
	// Filter events by resource filters
	var matchingEvents []models.Event
	for _, event := range events {
		// Check resource filters
		if !query.Filters.Matches(event.Resource) {
			continue
		}

		matchingEvents = append(matchingEvents, event)
	}

	// Sort by timestamp
	sort.Slice(matchingEvents, func(i, j int) bool {
		return matchingEvents[i].Timestamp < matchingEvents[j].Timestamp
	})

	// Build result
	executionTime := time.Since(start)
	result := &models.QueryResult{
		Events:          matchingEvents,
		Count:           int32(len(matchingEvents)),
		ExecutionTimeMs: int32(executionTime.Milliseconds()),
		SegmentsScanned: int32(len(events)), // For demo, consider all events as scanned
		SegmentsSkipped: 0,                  // No segments to skip in demo
		FilesSearched:   1,                  // Pretend there's 1 "file" (the demo data)
	}

	return result, nil
}
