package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/models"
)

//go:embed demo-data.json
var demoDataJSON []byte

// DemoQueryExecutor returns demo data for queries
type DemoQueryExecutor struct {
	demoData *demoDataset
}

// demoDataset represents the embedded demo data structure
type demoDataset struct {
	Version  int `json:"version"`
	TimeRange struct {
		EarliestOffsetSec int64 `json:"earliestOffsetSec"`
		LatestOffsetSec   int64 `json:"latestOffsetSec"`
	} `json:"timeRange"`
	Resources []demoResource `json:"resources"`
	Metadata  struct {
		Namespaces     []string           `json:"namespaces"`
		Kinds          []string           `json:"kinds"`
		Groups         []string           `json:"groups"`
		ResourceCounts map[string]int     `json:"resourceCounts"`
		TotalEvents    int                `json:"totalEvents"`
	} `json:"metadata"`
}

type demoResource struct {
	ID             string             `json:"id"`
	Group          string             `json:"group"`
	Version        string             `json:"version"`
	Kind           string             `json:"kind"`
	Namespace      string             `json:"namespace"`
	Name           string             `json:"name"`
	StatusSegments []demoStatusSegment `json:"statusSegments"`
	Events         []demoEvent        `json:"events"`
}

type demoStatusSegment struct {
	StartOffsetSec int64                  `json:"startOffsetSec"`
	EndOffsetSec   int64                  `json:"endOffsetSec"`
	Status         string                 `json:"status"`
	Message        string                 `json:"message"`
	ResourceData   map[string]interface{} `json:"resourceData"`
}

type demoEvent struct {
	ID                      string `json:"id"`
	TimestampOffsetSec      int64  `json:"timestampOffsetSec"`
	Reason                  string `json:"reason"`
	Message                 string `json:"message"`
	Type                    string `json:"type"`
	Count                   int32  `json:"count"`
	Source                  string `json:"source"`
	FirstTimestampOffsetSec int64  `json:"firstTimestampOffsetSec"`
	LastTimestampOffsetSec  int64  `json:"lastTimestampOffsetSec"`
}

// NewDemoQueryExecutor creates a new demo query executor
func NewDemoQueryExecutor() *DemoQueryExecutor {
	var dataset demoDataset
	if err := json.Unmarshal(demoDataJSON, &dataset); err != nil {
		// This should never happen in production, but we'll fail gracefully
		return &DemoQueryExecutor{
			demoData: &demoDataset{},
		}
	}

	return &DemoQueryExecutor{
		demoData: &dataset,
	}
}

// Execute returns demo data for the requested query
func (d *DemoQueryExecutor) Execute(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, error) {
	startTime := time.Now()

	// Get the demo time range
	demoStart := d.demoData.TimeRange.EarliestOffsetSec

	// Anchor demo data to the query start time
	timeOffset := query.StartTimestamp - demoStart

	// Filter resources based on query filters
	var events []models.Event
	for _, resource := range d.demoData.Resources {
		// Apply filters
		if query.Filters.Namespace != "" && resource.Namespace != query.Filters.Namespace {
			continue
		}
		if query.Filters.Kind != "" && resource.Kind != query.Filters.Kind {
			continue
		}
		if query.Filters.Group != "" && resource.Group != query.Filters.Group {
			continue
		}
		if query.Filters.Version != "" && resource.Version != query.Filters.Version {
			continue
		}

		// Process events from statusSegments
		// Each statusSegment represents a state change, so we create an event at its start time
		for idx, segment := range resource.StatusSegments {
			eventTimeNs := (timeOffset + segment.StartOffsetSec) * int64(time.Second)

			// Check if event falls within query time window (convert to nanoseconds)
			queryStartNs := query.StartTimestamp * int64(time.Second)
			queryEndNs := query.EndTimestamp * int64(time.Second)

			if eventTimeNs < queryStartNs || eventTimeNs > queryEndNs {
				continue
			}

			// Create an event with ResourceMetadata
			resourceMetadata := models.ResourceMetadata{
				Group:     resource.Group,
				Version:   resource.Version,
				Kind:      resource.Kind,
				Namespace: resource.Namespace,
				Name:      resource.Name,
				UID:       resource.ID,
			}

			// Determine event type based on segment position
			var eventType models.EventType
			if idx == 0 {
				eventType = models.EventTypeCreate
			} else {
				eventType = models.EventTypeUpdate
			}

			// Convert resourceData map to byte slice for storage
			var dataBytes []byte
			if segment.ResourceData != nil {
				var err error
				dataBytes, err = json.Marshal(segment.ResourceData)
				if err != nil {
					// Log error but continue processing
					dataBytes = nil
				}
			}

			event := models.Event{
				ID:        fmt.Sprintf("%s-segment-%d", resource.ID, idx),
				Timestamp: eventTimeNs,
				Type:      eventType,
				Resource:  resourceMetadata,
				Data:      dataBytes,
			}
			events = append(events, event)
		}
	}

	executionTime := time.Since(startTime)

	return &models.QueryResult{
		Events:          events,
		Count:           int32(len(events)),                          //nolint:gosec // safe conversion: event count is reasonable
		ExecutionTimeMs: int32(executionTime.Milliseconds()),         //nolint:gosec // safe conversion: execution time is reasonable
		SegmentsScanned: 1,
		SegmentsSkipped: 0,
		FilesSearched:   1,
	}, nil
}

// SetSharedCache is a no-op for demo executor (implements QueryExecutor interface)
func (d *DemoQueryExecutor) SetSharedCache(cache interface{}) {
	// Demo executor doesn't use caching
}

// QueryDistinctMetadata returns distinct namespaces and kinds from demo data
func (d *DemoQueryExecutor) QueryDistinctMetadata(ctx context.Context, startTimeNs, endTimeNs int64) (namespaces []string, kinds []string, minTime int64, maxTime int64, err error) {
	// Use the Execute method to get events
	query := &models.QueryRequest{
		StartTimestamp: startTimeNs / 1e9,
		EndTimestamp:   endTimeNs / 1e9,
		Filters:        models.QueryFilters{},
	}

	result, queryErr := d.Execute(ctx, query)
	if queryErr != nil {
		return nil, nil, 0, 0, queryErr
	}

	// Extract unique namespaces and kinds
	namespacesMap := make(map[string]bool)
	kindsMap := make(map[string]bool)
	minTime = -1
	maxTime = -1

	for _, event := range result.Events {
		namespacesMap[event.Resource.Namespace] = true
		kindsMap[event.Resource.Kind] = true

		if minTime < 0 || event.Timestamp < minTime {
			minTime = event.Timestamp
		}
		if maxTime < 0 || event.Timestamp > maxTime {
			maxTime = event.Timestamp
		}
	}

	// Convert maps to sorted slices
	namespaces = make([]string, 0, len(namespacesMap))
	for ns := range namespacesMap {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	kinds = make([]string, 0, len(kindsMap))
	for kind := range kindsMap {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)

	if minTime < 0 {
		minTime = 0
	}
	if maxTime < 0 {
		maxTime = 0
	}

	return namespaces, kinds, minTime, maxTime, nil
}
