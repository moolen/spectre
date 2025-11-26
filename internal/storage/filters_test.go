package storage

import (
	"testing"
	"time"

	"github.com/moritz/rpk/internal/models"
)

func TestNewFilterEngine(t *testing.T) {
	engine := NewFilterEngine()
	if engine == nil {
		t.Fatal("expected non-nil filter engine")
	}
	if engine.logger == nil {
		t.Error("expected logger to be initialized")
	}
}

func TestFilterEngineFilterEvents(t *testing.T) {
	engine := NewFilterEngine()

	events := []models.Event{
		createEventWithResource("pod-1", "default", "Pod", ""),
		createEventWithResource("svc-1", "default", "Service", ""),
		createEventWithResource("pod-2", "kube-system", "Pod", ""),
	}

	// Test with empty filters (should return all)
	filtered := engine.FilterEvents(events, models.QueryFilters{})
	if len(filtered) != 3 {
		t.Errorf("expected 3 events with empty filters, got %d", len(filtered))
	}

	// Test with kind filter
	filters := models.QueryFilters{Kind: "Pod"}
	filtered = engine.FilterEvents(events, filters)
	if len(filtered) != 2 {
		t.Errorf("expected 2 Pod events, got %d", len(filtered))
	}

	// Test with namespace filter
	filters = models.QueryFilters{Namespace: "default"}
	filtered = engine.FilterEvents(events, filters)
	if len(filtered) != 2 {
		t.Errorf("expected 2 events in default namespace, got %d", len(filtered))
	}

	// Test with multiple filters
	filters = models.QueryFilters{Kind: "Pod", Namespace: "default"}
	filtered = engine.FilterEvents(events, filters)
	if len(filtered) != 1 {
		t.Errorf("expected 1 Pod in default namespace, got %d", len(filtered))
	}
}

func TestFilterEngineMatchesFilters(t *testing.T) {
	engine := NewFilterEngine()

	event := createEventWithResource("pod-1", "default", "Pod", "")

	// Test empty filters
	if !engine.MatchesFilters(&event, models.QueryFilters{}) {
		t.Error("expected event to match empty filters")
	}

	// Test matching filter
	filters := models.QueryFilters{Kind: "Pod"}
	if !engine.MatchesFilters(&event, filters) {
		t.Error("expected event to match Pod filter")
	}

	// Test non-matching filter
	filters = models.QueryFilters{Kind: "Service"}
	if engine.MatchesFilters(&event, filters) {
		t.Error("expected event to not match Service filter")
	}
}

func TestFilterEngineFilterByNamespace(t *testing.T) {
	engine := NewFilterEngine()

	events := []models.Event{
		createEventWithResource("pod-1", "default", "Pod", ""),
		createEventWithResource("svc-1", "default", "Service", ""),
		createEventWithResource("pod-2", "kube-system", "Pod", ""),
	}

	// Test with namespace filter
	filtered := engine.FilterByNamespace(events, "default")
	if len(filtered) != 2 {
		t.Errorf("expected 2 events in default namespace, got %d", len(filtered))
	}

	// Test with empty namespace (should return all)
	filtered = engine.FilterByNamespace(events, "")
	if len(filtered) != 3 {
		t.Errorf("expected 3 events with empty namespace filter, got %d", len(filtered))
	}
}

func TestFilterEngineFilterByKind(t *testing.T) {
	engine := NewFilterEngine()

	events := []models.Event{
		createEventWithResource("pod-1", "default", "Pod", ""),
		createEventWithResource("svc-1", "default", "Service", ""),
		createEventWithResource("pod-2", "kube-system", "Pod", ""),
	}

	// Test with kind filter
	filtered := engine.FilterByKind(events, "Pod")
	if len(filtered) != 2 {
		t.Errorf("expected 2 Pod events, got %d", len(filtered))
	}

	// Test with empty kind (should return all)
	filtered = engine.FilterByKind(events, "")
	if len(filtered) != 3 {
		t.Errorf("expected 3 events with empty kind filter, got %d", len(filtered))
	}
}

func TestFilterEngineFilterByGroup(t *testing.T) {
	engine := NewFilterEngine()

	events := []models.Event{
		createEventWithResource("pod-1", "default", "Pod", ""),
		createEventWithResource("deploy-1", "default", "Deployment", "apps"),
		createEventWithResource("svc-1", "default", "Service", ""),
	}

	// Test with group filter
	filtered := engine.FilterByGroup(events, "apps")
	if len(filtered) != 1 {
		t.Errorf("expected 1 event in apps group, got %d", len(filtered))
	}

	// Test with empty group (should return all)
	filtered = engine.FilterByGroup(events, "")
	if len(filtered) != 3 {
		t.Errorf("expected 3 events with empty group filter, got %d", len(filtered))
	}
}

func TestFilterEngineFilterByVersion(t *testing.T) {
	engine := NewFilterEngine()

	events := []models.Event{
		createEventWithResource("pod-1", "default", "Pod", ""),
		createEventWithResource("deploy-1", "default", "Deployment", "apps"),
	}

	// Test with version filter
	filtered := engine.FilterByVersion(events, "v1")
	if len(filtered) != 2 {
		t.Errorf("expected 2 events with v1 version, got %d", len(filtered))
	}

	// Test with empty version (should return all)
	filtered = engine.FilterByVersion(events, "")
	if len(filtered) != 2 {
		t.Errorf("expected 2 events with empty version filter, got %d", len(filtered))
	}
}

func TestFilterEngineFilterByTimeRange(t *testing.T) {
	engine := NewFilterEngine()

	now := time.Now()
	events := []models.Event{
		createEventWithTimestamp(now.Add(-2 * time.Hour).UnixNano()),
		createEventWithTimestamp(now.Add(-1 * time.Hour).UnixNano()),
		createEventWithTimestamp(now.UnixNano()),
		createEventWithTimestamp(now.Add(1 * time.Hour).UnixNano()),
	}

	startTime := now.Add(-90 * time.Minute).UnixNano()
	endTime := now.Add(30 * time.Minute).UnixNano()

	filtered := engine.FilterByTimeRange(events, startTime, endTime)
	if len(filtered) != 2 {
		t.Errorf("expected 2 events in time range, got %d", len(filtered))
	}
}

func TestFilterEngineFilterByType(t *testing.T) {
	engine := NewFilterEngine()

	events := []models.Event{
		{Type: models.EventTypeCreate, Resource: models.ResourceMetadata{Kind: "Pod"}},
		{Type: models.EventTypeUpdate, Resource: models.ResourceMetadata{Kind: "Pod"}},
		{Type: models.EventTypeDelete, Resource: models.ResourceMetadata{Kind: "Pod"}},
		{Type: models.EventTypeCreate, Resource: models.ResourceMetadata{Kind: "Service"}},
	}

	// Test with CREATE filter
	filtered := engine.FilterByType(events, models.EventTypeCreate)
	if len(filtered) != 2 {
		t.Errorf("expected 2 CREATE events, got %d", len(filtered))
	}

	// Test with empty type (should return all)
	filtered = engine.FilterByType(events, "")
	if len(filtered) != 4 {
		t.Errorf("expected 4 events with empty type filter, got %d", len(filtered))
	}
}

func TestFilterEngineGetFilterSummary(t *testing.T) {
	engine := NewFilterEngine()

	// Test with all filters
	filters := models.QueryFilters{
		Group:     "apps",
		Version:   "v1",
		Kind:      "Pod",
		Namespace: "default",
	}

	summary := engine.GetFilterSummary(filters)
	if len(summary) != 4 {
		t.Errorf("expected 4 filter entries, got %d", len(summary))
	}

	if summary["group"] != "apps" {
		t.Errorf("expected group=apps, got %s", summary["group"])
	}

	// Test with empty filters
	summary = engine.GetFilterSummary(models.QueryFilters{})
	if summary["filters"] != "none" {
		t.Errorf("expected 'none' for empty filters, got %s", summary["filters"])
	}
}

func TestFilterEngineAreFiltersCompatible(t *testing.T) {
	engine := NewFilterEngine()

	// Test compatible filters
	filters1 := models.QueryFilters{Kind: "Pod"}
	filters2 := models.QueryFilters{Kind: "Pod", Namespace: "default"}
	if !engine.AreFiltersCompatible(filters1, filters2) {
		t.Error("expected compatible filters")
	}

	// Test incompatible filters (different kinds)
	filters1 = models.QueryFilters{Kind: "Pod"}
	filters2 = models.QueryFilters{Kind: "Service"}
	if engine.AreFiltersCompatible(filters1, filters2) {
		t.Error("expected incompatible filters")
	}

	// Test compatible filters (one empty)
	filters1 = models.QueryFilters{}
	filters2 = models.QueryFilters{Kind: "Pod"}
	if !engine.AreFiltersCompatible(filters1, filters2) {
		t.Error("expected compatible filters (one empty)")
	}
}

func createEventWithResource(name, namespace, kind, group string) models.Event {
	return models.Event{
		ID:        "test-id-" + name,
		Timestamp: time.Now().UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     group,
			Version:   "v1",
			Kind:      kind,
			Namespace: namespace,
			Name:      name,
		},
	}
}

func createEventWithTimestamp(timestamp int64) models.Event {
	return models.Event{
		ID:        "test-id",
		Timestamp: timestamp,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "test",
		},
	}
}

