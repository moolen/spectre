package storage

import (
	"testing"

	"github.com/moritz/rpk/internal/models"
	"github.com/moritz/rpk/internal/storage"
)

// TestNewFilterEngine tests filter engine creation
func TestNewFilterEngine(t *testing.T) {
	fe := storage.NewFilterEngine()
	if fe == nil {
		t.Error("NewFilterEngine returned nil")
	}
}

// TestFilterEventsEmpty tests filtering with empty filter
func TestFilterEventsEmpty(t *testing.T) {
	fe := storage.NewFilterEngine()

	events := []models.Event{
		{
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "pod1",
			},
		},
		{
			Resource: models.ResourceMetadata{
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "deploy1",
			},
		},
	}

	filters := models.QueryFilters{} // Empty filters

	result := fe.FilterEvents(events, filters)
	if len(result) != len(events) {
		t.Errorf("Expected %d events, got %d", len(events), len(result))
	}
}

// TestFilterEventsByKind tests filtering by kind
func TestFilterEventsByKind(t *testing.T) {
	fe := storage.NewFilterEngine()

	events := []models.Event{
		{
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Namespace: "default",
			},
		},
		{
			Resource: models.ResourceMetadata{
				Kind:      "Deployment",
				Namespace: "default",
			},
		},
		{
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Namespace: "kube-system",
			},
		},
	}

	filters := models.QueryFilters{Kind: "Pod"}

	result := fe.FilterEvents(events, filters)
	if len(result) != 2 {
		t.Errorf("Expected 2 Pod events, got %d", len(result))
	}

	for _, event := range result {
		if event.Resource.Kind != "Pod" {
			t.Errorf("Expected Kind=Pod, got %s", event.Resource.Kind)
		}
	}
}

// TestFilterEventsByNamespace tests filtering by namespace
func TestFilterEventsByNamespace(t *testing.T) {
	fe := storage.NewFilterEngine()

	events := []models.Event{
		{
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Namespace: "default",
			},
		},
		{
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Namespace: "kube-system",
			},
		},
		{
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Namespace: "default",
			},
		},
	}

	filters := models.QueryFilters{Namespace: "default"}

	result := fe.FilterEvents(events, filters)
	if len(result) != 2 {
		t.Errorf("Expected 2 events in default namespace, got %d", len(result))
	}

	for _, event := range result {
		if event.Resource.Namespace != "default" {
			t.Errorf("Expected Namespace=default, got %s", event.Resource.Namespace)
		}
	}
}

// TestFilterEventsByMultipleDimensions tests filtering by multiple dimensions
func TestFilterEventsByMultipleDimensions(t *testing.T) {
	fe := storage.NewFilterEngine()

	events := []models.Event{
		{
			Resource: models.ResourceMetadata{
				Group:     "apps",
				Version:   "v1",
				Kind:      "Deployment",
				Namespace: "default",
			},
		},
		{
			Resource: models.ResourceMetadata{
				Group:     "apps",
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "default",
			},
		},
		{
			Resource: models.ResourceMetadata{
				Group:     "apps",
				Version:   "v1",
				Kind:      "Deployment",
				Namespace: "kube-system",
			},
		},
	}

	filters := models.QueryFilters{
		Group:     "apps",
		Version:   "v1",
		Kind:      "Deployment",
		Namespace: "default",
	}

	result := fe.FilterEvents(events, filters)
	if len(result) != 1 {
		t.Errorf("Expected 1 matching event, got %d", len(result))
	}

	event := result[0]
	if event.Resource.Group != "apps" || event.Resource.Kind != "Deployment" ||
		event.Resource.Namespace != "default" {
		t.Error("Filtered event doesn't match all criteria")
	}
}

// TestFilterByTimeRange tests filtering by timestamp range
func TestFilterByTimeRange(t *testing.T) {
	fe := storage.NewFilterEngine()

	events := []models.Event{
		{Timestamp: 1000},
		{Timestamp: 2000},
		{Timestamp: 3000},
		{Timestamp: 4000},
		{Timestamp: 5000},
	}

	result := fe.FilterByTimeRange(events, 2000, 4000)
	if len(result) != 3 {
		t.Errorf("Expected 3 events in time range, got %d", len(result))
	}

	for _, event := range result {
		if event.Timestamp < 2000 || event.Timestamp > 4000 {
			t.Errorf("Event %d is outside time range", event.Timestamp)
		}
	}
}

// TestFilterByType tests filtering by event type
func TestFilterByType(t *testing.T) {
	fe := storage.NewFilterEngine()

	events := []models.Event{
		{Type: models.EventTypeCreate},
		{Type: models.EventTypeUpdate},
		{Type: models.EventTypeDelete},
		{Type: models.EventTypeCreate},
	}

	result := fe.FilterByType(events, models.EventTypeCreate)
	if len(result) != 2 {
		t.Errorf("Expected 2 CREATE events, got %d", len(result))
	}

	for _, event := range result {
		if event.Type != models.EventTypeCreate {
			t.Errorf("Expected Type=CREATE, got %s", event.Type)
		}
	}
}

// TestMatchesFilters tests individual event matching
func TestMatchesFilters(t *testing.T) {
	fe := storage.NewFilterEngine()

	event := &models.Event{
		Resource: models.ResourceMetadata{
			Group:     "apps",
			Version:   "v1",
			Kind:      "Deployment",
			Namespace: "default",
		},
	}

	testCases := []struct {
		name    string
		filters models.QueryFilters
		matches bool
	}{
		{
			name:    "empty filters",
			filters: models.QueryFilters{},
			matches: true,
		},
		{
			name: "matching kind",
			filters: models.QueryFilters{Kind: "Deployment"},
			matches: true,
		},
		{
			name: "non-matching kind",
			filters: models.QueryFilters{Kind: "Pod"},
			matches: false,
		},
		{
			name: "matching namespace",
			filters: models.QueryFilters{Namespace: "default"},
			matches: true,
		},
		{
			name: "non-matching namespace",
			filters: models.QueryFilters{Namespace: "kube-system"},
			matches: false,
		},
		{
			name: "matching group and kind",
			filters: models.QueryFilters{Group: "apps", Kind: "Deployment"},
			matches: true,
		},
		{
			name: "partial match (group matches, kind doesn't)",
			filters: models.QueryFilters{Group: "apps", Kind: "Pod"},
			matches: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := fe.MatchesFilters(event, tc.filters)
			if result != tc.matches {
				t.Errorf("Expected %v, got %v", tc.matches, result)
			}
		})
	}
}

// TestGetFilterSummary tests filter summary generation
func TestGetFilterSummary(t *testing.T) {
	fe := storage.NewFilterEngine()

	testCases := []struct {
		name     string
		filters  models.QueryFilters
		minItems int
	}{
		{
			name:     "empty filters",
			filters:  models.QueryFilters{},
			minItems: 1,
		},
		{
			name: "single filter",
			filters: models.QueryFilters{
				Kind: "Pod",
			},
			minItems: 1,
		},
		{
			name: "multiple filters",
			filters: models.QueryFilters{
				Kind:      "Pod",
				Namespace: "default",
				Group:     "apps",
			},
			minItems: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			summary := fe.GetFilterSummary(tc.filters)
			if len(summary) < tc.minItems {
				t.Errorf("Expected at least %d items, got %d", tc.minItems, len(summary))
			}
		})
	}
}

// TestAreFiltersCompatible tests filter compatibility checking
func TestAreFiltersCompatible(t *testing.T) {
	fe := storage.NewFilterEngine()

	testCases := []struct {
		name        string
		filters1    models.QueryFilters
		filters2    models.QueryFilters
		compatible  bool
	}{
		{
			name:       "both empty",
			filters1:   models.QueryFilters{},
			filters2:   models.QueryFilters{},
			compatible: true,
		},
		{
			name: "non-conflicting filters",
			filters1: models.QueryFilters{
				Kind: "Pod",
			},
			filters2: models.QueryFilters{
				Namespace: "default",
			},
			compatible: true,
		},
		{
			name: "matching same dimension",
			filters1: models.QueryFilters{
				Kind: "Pod",
			},
			filters2: models.QueryFilters{
				Kind: "Pod",
			},
			compatible: true,
		},
		{
			name: "conflicting same dimension",
			filters1: models.QueryFilters{
				Kind: "Pod",
			},
			filters2: models.QueryFilters{
				Kind: "Deployment",
			},
			compatible: false,
		},
		{
			name: "one empty filter compatible",
			filters1: models.QueryFilters{
				Kind: "Pod",
			},
			filters2: models.QueryFilters{},
			compatible: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := fe.AreFiltersCompatible(tc.filters1, tc.filters2)
			if result != tc.compatible {
				t.Errorf("Expected %v, got %v", tc.compatible, result)
			}
		})
	}
}

// TestFilterEventsByGroup tests filtering by API group
func TestFilterEventsByGroup(t *testing.T) {
	fe := storage.NewFilterEngine()

	events := []models.Event{
		{
			Resource: models.ResourceMetadata{
				Group: "apps",
				Kind: "Deployment",
			},
		},
		{
			Resource: models.ResourceMetadata{
				Group: "batch",
				Kind: "Job",
			},
		},
		{
			Resource: models.ResourceMetadata{
				Group: "apps",
				Kind: "StatefulSet",
			},
		},
	}

	result := fe.FilterByGroup(events, "apps")
	if len(result) != 2 {
		t.Errorf("Expected 2 events in apps group, got %d", len(result))
	}

	for _, event := range result {
		if event.Resource.Group != "apps" {
			t.Errorf("Expected Group=apps, got %s", event.Resource.Group)
		}
	}
}

// TestFilterEventsByVersion tests filtering by API version
func TestFilterEventsByVersion(t *testing.T) {
	fe := storage.NewFilterEngine()

	events := []models.Event{
		{
			Resource: models.ResourceMetadata{
				Version: "v1",
				Kind: "Pod",
			},
		},
		{
			Resource: models.ResourceMetadata{
				Version: "v1beta1",
				Kind: "Pod",
			},
		},
		{
			Resource: models.ResourceMetadata{
				Version: "v1",
				Kind: "Deployment",
			},
		},
	}

	result := fe.FilterByVersion(events, "v1")
	if len(result) != 2 {
		t.Errorf("Expected 2 events with v1, got %d", len(result))
	}

	for _, event := range result {
		if event.Resource.Version != "v1" {
			t.Errorf("Expected Version=v1, got %s", event.Resource.Version)
		}
	}
}
