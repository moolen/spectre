package embedded

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/models"
)

func makeEvent(ts int64, kind, namespace, name, uid string, eventType models.EventType) models.Event {
	return models.Event{
		ID:        uid + "-" + name + "-" + kind + "-" + namespace + "-" + string(eventType),
		Timestamp: ts,
		Type:      eventType,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      kind,
			Namespace: namespace,
			Name:      name,
			UID:       uid,
		},
		Data: json.RawMessage(`{"ok":true}`),
	}
}

func makeK8sEvent(ts int64, namespace, name, uid, involvedUID string) models.Event {
	return models.Event{
		ID:        uid + "-" + name + "-Event-" + namespace,
		Timestamp: ts,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:             "",
			Version:           "v1",
			Kind:              "Event",
			Namespace:         namespace,
			Name:              name,
			UID:               uid,
			InvolvedObjectUID: involvedUID,
		},
		Data: json.RawMessage(`{"reason":"Test","message":"msg","type":"Normal"}`),
	}
}

func TestQueryExecutor_ExecuteFiltersResources(t *testing.T) {
	events := []models.Event{
		makeEvent(10*1e9, "Pod", "ns-a", "pod-a", "uid-a", models.EventTypeCreate),
		makeEvent(12*1e9, "Pod", "ns-a", "pod-a", "uid-a", models.EventTypeUpdate),
		makeEvent(11*1e9, "Deployment", "ns-b", "dep-b", "uid-b", models.EventTypeCreate),
		makeEvent(13*1e9, "Service", "ns-a", "svc-a", "uid-c", models.EventTypeCreate),
	}

	executor, err := NewQueryExecutor(events)
	if err != nil {
		t.Fatalf("NewQueryExecutor error: %v", err)
	}

	query := &models.QueryRequest{
		StartTimestamp: 9,
		EndTimestamp:   13,
		Filters: models.QueryFilters{
			Kinds:      []string{"Pod"},
			Namespaces: []string{"ns-a"},
		},
	}

	result, err := executor.Execute(context.Background(), query)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if got := len(result.Events); got != 2 {
		t.Fatalf("expected 2 events, got %d", got)
	}
	for _, event := range result.Events {
		if event.Resource.Kind != "Pod" || event.Resource.Namespace != "ns-a" {
			t.Fatalf("unexpected event resource: kind=%s ns=%s", event.Resource.Kind, event.Resource.Namespace)
		}
	}
}

func TestQueryExecutor_ExecuteAddsPreExistingAnchor(t *testing.T) {
	events := []models.Event{
		makeEvent(1*1e9, "Pod", "ns-a", "pod-a", "uid-a", models.EventTypeCreate),
		makeEvent(6*1e9, "Pod", "ns-a", "pod-a", "uid-a", models.EventTypeUpdate),
	}

	executor, err := NewQueryExecutor(events)
	if err != nil {
		t.Fatalf("NewQueryExecutor error: %v", err)
	}

	query := &models.QueryRequest{
		StartTimestamp: 5,
		EndTimestamp:   7,
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(context.Background(), query)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if got := len(result.Events); got != 2 {
		t.Fatalf("expected 2 events, got %d", got)
	}

	if !result.Events[0].PreExisting {
		t.Fatalf("expected first event to be preExisting")
	}
	if result.Events[1].PreExisting {
		t.Fatalf("did not expect second event to be preExisting")
	}
}

func TestQueryExecutor_ExecutePaginatedByResource(t *testing.T) {
	events := []models.Event{
		makeEvent(10*1e9, "Pod", "ns-a", "pod-a", "uid-a", models.EventTypeCreate),
		makeEvent(11*1e9, "Pod", "ns-a", "pod-a", "uid-a", models.EventTypeUpdate),
		makeEvent(12*1e9, "Pod", "ns-a", "pod-b", "uid-b", models.EventTypeCreate),
		makeEvent(13*1e9, "Deployment", "ns-a", "dep-c", "uid-c", models.EventTypeCreate),
	}

	executor, err := NewQueryExecutor(events)
	if err != nil {
		t.Fatalf("NewQueryExecutor error: %v", err)
	}

	query := &models.QueryRequest{
		StartTimestamp: 9,
		EndTimestamp:   14,
		Filters:        models.QueryFilters{},
	}

	page1, page1Resp, err := executor.ExecutePaginated(context.Background(), query, &models.PaginationRequest{
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("ExecutePaginated page1 error: %v", err)
	}

	if got := len(page1.Events); got != 3 {
		t.Fatalf("expected 3 events on page1, got %d", got)
	}
	if page1Resp == nil || !page1Resp.HasMore || page1Resp.NextCursor == "" {
		t.Fatalf("expected pagination response with hasMore and cursor")
	}

	page2, page2Resp, err := executor.ExecutePaginated(context.Background(), query, &models.PaginationRequest{
		PageSize: 2,
		Cursor:   page1Resp.NextCursor,
	})
	if err != nil {
		t.Fatalf("ExecutePaginated page2 error: %v", err)
	}
	if got := len(page2.Events); got != 1 {
		t.Fatalf("expected 1 event on page2, got %d", got)
	}
	if page2Resp == nil || page2Resp.HasMore || page2Resp.NextCursor != "" {
		t.Fatalf("expected pagination response without more pages")
	}
}

func TestQueryExecutor_QueryDistinctMetadata(t *testing.T) {
	events := []models.Event{
		makeEvent(1*1e9, "Pod", "ns-a", "pod-a", "uid-a", models.EventTypeCreate),
		makeEvent(2*1e9, "Deployment", "ns-b", "dep-b", "uid-b", models.EventTypeCreate),
		makeEvent(3*1e9, "Event", "ns-a", "evt-a", "uid-e", models.EventTypeCreate),
		makeK8sEvent(3*1e9, "ns-a", "evt-b", "uid-f", "uid-a"),
	}

	executor, err := NewQueryExecutor(events)
	if err != nil {
		t.Fatalf("NewQueryExecutor error: %v", err)
	}

	namespaces, kinds, minTime, maxTime, err := executor.QueryDistinctMetadata(context.Background(), 0, 5*1e9)
	if err != nil {
		t.Fatalf("QueryDistinctMetadata error: %v", err)
	}

	if len(namespaces) != 2 || namespaces[0] != "ns-a" || namespaces[1] != "ns-b" {
		t.Fatalf("unexpected namespaces: %v", namespaces)
	}
	if len(kinds) != 2 || kinds[0] != "Deployment" || kinds[1] != "Pod" {
		t.Fatalf("unexpected kinds: %v", kinds)
	}
	if minTime != 1*1e9 || maxTime != 2*1e9 {
		t.Fatalf("unexpected time range: min=%d max=%d", minTime, maxTime)
	}
}
