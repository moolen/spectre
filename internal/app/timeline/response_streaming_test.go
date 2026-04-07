package timeline

import (
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestService_BuildTimelineResources_SelectedEntriesOnlyMaterializesRequestedResources(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")
	service := NewService(&mockQueryExecutor{}, logger, tracer)

	resourceResult := &models.QueryResult{
		Events: []models.Event{
			{
				ID:        "pod-a-create",
				Timestamp: 100,
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Version:   "v1",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "pod-a",
					UID:       "pod-a-uid",
				},
				Data: json.RawMessage(`{"status":{"phase":"Running"}}`),
			},
			{
				ID:        "pod-b-create",
				Timestamp: 110,
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Version:   "v1",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "pod-b",
					UID:       "pod-b-uid",
				},
				Data: json.RawMessage(`{"status":{"phase":"Running"}}`),
			},
		},
		Count:           2,
		ExecutionTimeMs: 5,
		QueryStartTime:  100,
		QueryEndTime:    200,
	}
	eventResult := &models.QueryResult{
		Events: []models.Event{
			{
				ID:        "k8s-a",
				Timestamp: 120,
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Version:           "v1",
					Kind:              "Event",
					Namespace:         "default",
					Name:              "pod-a.1",
					UID:               "event-a-uid",
					InvolvedObjectUID: "pod-a-uid",
				},
				Data: json.RawMessage(`{"reason":"Scheduled","message":"Assigned","type":"Normal"}`),
			},
			{
				ID:        "k8s-b",
				Timestamp: 121,
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Version:           "v1",
					Kind:              "Event",
					Namespace:         "default",
					Name:              "pod-b.1",
					UID:               "event-b-uid",
					InvolvedObjectUID: "pod-b-uid",
				},
				Data: json.RawMessage(`{"reason":"Failed","message":"NotScheduled","type":"Warning"}`),
			},
		},
		Count: 2,
	}

	index := service.BuildTimelineIndex(resourceResult, eventResult)
	if index.Count() != 2 {
		t.Fatalf("expected 2 indexed resources, got %d", index.Count())
	}

	var selected *TimelineResourceEntry
	for _, entry := range index.Entries() {
		if entry.Name == "pod-a" {
			selected = entry
			break
		}
	}
	if selected == nil {
		t.Fatal("expected to find indexed entry for pod-a")
	}

	resources := service.BuildTimelineResources(index, []*TimelineResourceEntry{selected})
	if len(resources) != 1 {
		t.Fatalf("expected 1 materialized resource, got %d", len(resources))
	}
	if resources[0].Name != "pod-a" {
		t.Fatalf("expected pod-a resource, got %q", resources[0].Name)
	}
	if len(resources[0].Events) != 1 {
		t.Fatalf("expected 1 attached Kubernetes event, got %d", len(resources[0].Events))
	}
	if resources[0].Events[0].Reason != "Scheduled" {
		t.Fatalf("expected Scheduled event reason, got %q", resources[0].Events[0].Reason)
	}
}
