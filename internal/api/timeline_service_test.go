package api_test

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/trace/noop"
)

type stubQueryExecutor struct{}

func (stubQueryExecutor) Execute(context.Context, *models.QueryRequest) (*models.QueryResult, error) {
	return &models.QueryResult{}, nil
}

func (stubQueryExecutor) SetSharedCache(interface{}) {}

func TestBuildTimelineResponse_ClampsPreExistingSegmentToQueryStart(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")
	timelineService := api.NewTimelineService(stubQueryExecutor{}, logger, tracer)

	queryStart := time.Unix(1_700_000_000, 0)
	queryEnd := queryStart.Add(90 * time.Minute)
	preWindowTime := queryStart.Add(-15 * time.Minute)
	inRangeTime := queryStart.Add(10 * time.Minute)
	resourceUID := "pod-uid-1"

	resourceResult := &models.QueryResult{
		Events: []models.Event{
			{
				ID:          "pre-window",
				Timestamp:   preWindowTime.UnixNano(),
				Type:        models.EventTypeCreate,
				PreExisting: true,
				Resource: models.ResourceMetadata{
					Kind:      "Pod",
					Namespace: "default",
					Name:      "example-pod",
					UID:       resourceUID,
				},
				Data: []byte(`{"status":{"phase":"Running"}}`),
			},
			{
				ID:        "in-range",
				Timestamp: inRangeTime.UnixNano(),
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					Kind:      "Pod",
					Namespace: "default",
					Name:      "example-pod",
					UID:       resourceUID,
				},
				Data: []byte(`{"status":{"phase":"Running"}}`),
			},
		},
		Count:           2,
		ExecutionTimeMs: 10,
		QueryStartTime:  queryStart.UnixNano(),
		QueryEndTime:    queryEnd.UnixNano(),
	}

	response := timelineService.BuildTimelineResponse(resourceResult, &models.QueryResult{Events: []models.Event{}})

	if response.Count != 1 {
		t.Fatalf("expected 1 resource, got %d", response.Count)
	}

	resource := response.Resources[0]
	if !resource.PreExisting {
		t.Fatalf("expected resource to be marked pre-existing")
	}
	if len(resource.StatusSegments) != 2 {
		t.Fatalf("expected 2 status segments, got %d", len(resource.StatusSegments))
	}

	firstSegment := resource.StatusSegments[0]
	if firstSegment.StartTime != queryStart.UnixNano() {
		t.Fatalf("expected first segment to start at query start %d, got %d", queryStart.UnixNano(), firstSegment.StartTime)
	}
	if firstSegment.EndTime != inRangeTime.UnixNano() {
		t.Fatalf("expected first segment to end at first in-range event %d, got %d", inRangeTime.UnixNano(), firstSegment.EndTime)
	}

	secondSegment := resource.StatusSegments[1]
	if secondSegment.StartTime != inRangeTime.UnixNano() {
		t.Fatalf("expected second segment to start at in-range event %d, got %d", inRangeTime.UnixNano(), secondSegment.StartTime)
	}
	if secondSegment.EndTime != queryEnd.UnixNano() {
		t.Fatalf("expected second segment to end at query end %d, got %d", queryEnd.UnixNano(), secondSegment.EndTime)
	}
}

func TestBuildTimelineResponse_PreExistingOnlyResourceSpansWindow(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")
	timelineService := api.NewTimelineService(stubQueryExecutor{}, logger, tracer)

	queryStart := time.Unix(1_700_100_000, 0)
	queryEnd := queryStart.Add(30 * time.Minute)
	preWindowTime := queryStart.Add(-5 * time.Minute)
	resourceUID := "pod-uid-2"

	resourceResult := &models.QueryResult{
		Events: []models.Event{
			{
				ID:          "pre-window-only",
				Timestamp:   preWindowTime.UnixNano(),
				Type:        models.EventTypeCreate,
				PreExisting: true,
				Resource: models.ResourceMetadata{
					Kind:      "Pod",
					Namespace: "default",
					Name:      "steady-pod",
					UID:       resourceUID,
				},
				Data: []byte(`{"status":{"phase":"Running"}}`),
			},
		},
		Count:           1,
		ExecutionTimeMs: 7,
		QueryStartTime:  queryStart.UnixNano(),
		QueryEndTime:    queryEnd.UnixNano(),
	}

	response := timelineService.BuildTimelineResponse(resourceResult, &models.QueryResult{Events: []models.Event{}})

	if response.Count != 1 {
		t.Fatalf("expected 1 resource, got %d", response.Count)
	}

	resource := response.Resources[0]
	if !resource.PreExisting {
		t.Fatalf("expected resource to be marked pre-existing")
	}
	if len(resource.StatusSegments) != 1 {
		t.Fatalf("expected 1 status segment, got %d", len(resource.StatusSegments))
	}

	segment := resource.StatusSegments[0]
	if segment.StartTime != queryStart.UnixNano() {
		t.Fatalf("expected segment start %d, got %d", queryStart.UnixNano(), segment.StartTime)
	}
	if segment.EndTime != queryEnd.UnixNano() {
		t.Fatalf("expected segment end %d, got %d", queryEnd.UnixNano(), segment.EndTime)
	}
}
