package logprocessing

import (
	"testing"
	"time"
)

func TestGenerateTemplateID_Deterministic(t *testing.T) {
	namespace := "default"
	pattern := "connected to <*>"

	// Generate ID multiple times
	id1 := GenerateTemplateID(namespace, pattern)
	id2 := GenerateTemplateID(namespace, pattern)
	id3 := GenerateTemplateID(namespace, pattern)

	// All IDs should be identical (deterministic)
	if id1 != id2 || id2 != id3 {
		t.Errorf("GenerateTemplateID is not deterministic: %s, %s, %s", id1, id2, id3)
	}

	// ID should be 64 characters (SHA-256 hex encoding)
	if len(id1) != 64 {
		t.Errorf("Expected 64-char hash, got %d chars: %s", len(id1), id1)
	}
}

func TestGenerateTemplateID_NamespaceScoping(t *testing.T) {
	pattern := "user login succeeded"

	// Same pattern in different namespaces should produce different IDs
	id1 := GenerateTemplateID("namespace-a", pattern)
	id2 := GenerateTemplateID("namespace-b", pattern)

	if id1 == id2 {
		t.Error("Same pattern in different namespaces produced identical IDs")
	}
}

func TestGenerateTemplateID_PatternSensitivity(t *testing.T) {
	namespace := "default"

	// Different patterns should produce different IDs
	id1 := GenerateTemplateID(namespace, "connected to <*>")
	id2 := GenerateTemplateID(namespace, "disconnected from <*>")

	if id1 == id2 {
		t.Error("Different patterns produced identical IDs")
	}
}

func TestTemplateList_FindByID(t *testing.T) {
	templates := TemplateList{
		{ID: "id-1", Pattern: "pattern-1"},
		{ID: "id-2", Pattern: "pattern-2"},
		{ID: "id-3", Pattern: "pattern-3"},
	}

	// Find existing template
	found := templates.FindByID("id-2")
	if found == nil {
		t.Fatal("FindByID returned nil for existing ID")
	}
	if found.Pattern != "pattern-2" {
		t.Errorf("Expected pattern-2, got %s", found.Pattern)
	}

	// Find non-existing template
	notFound := templates.FindByID("id-999")
	if notFound != nil {
		t.Error("FindByID returned non-nil for non-existing ID")
	}
}

func TestTemplateList_SortByCount(t *testing.T) {
	templates := TemplateList{
		{ID: "id-1", Count: 10},
		{ID: "id-2", Count: 50},
		{ID: "id-3", Count: 25},
	}

	templates.SortByCount()

	// Should be sorted in descending order
	if templates[0].ID != "id-2" || templates[0].Count != 50 {
		t.Errorf("Expected id-2 (count=50) first, got %s (count=%d)", templates[0].ID, templates[0].Count)
	}
	if templates[1].ID != "id-3" || templates[1].Count != 25 {
		t.Errorf("Expected id-3 (count=25) second, got %s (count=%d)", templates[1].ID, templates[1].Count)
	}
	if templates[2].ID != "id-1" || templates[2].Count != 10 {
		t.Errorf("Expected id-1 (count=10) third, got %s (count=%d)", templates[2].ID, templates[2].Count)
	}
}

func TestTemplateList_SortByLastSeen(t *testing.T) {
	now := time.Now()
	templates := TemplateList{
		{ID: "id-1", LastSeen: now.Add(-1 * time.Hour)},
		{ID: "id-2", LastSeen: now},
		{ID: "id-3", LastSeen: now.Add(-30 * time.Minute)},
	}

	templates.SortByLastSeen()

	// Should be sorted in descending order (most recent first)
	if templates[0].ID != "id-2" {
		t.Errorf("Expected id-2 (most recent) first, got %s", templates[0].ID)
	}
	if templates[1].ID != "id-3" {
		t.Errorf("Expected id-3 (30 min ago) second, got %s", templates[1].ID)
	}
	if templates[2].ID != "id-1" {
		t.Errorf("Expected id-1 (1 hour ago) third, got %s", templates[2].ID)
	}
}

func TestTemplateList_FilterByMinCount(t *testing.T) {
	templates := TemplateList{
		{ID: "id-1", Count: 5},
		{ID: "id-2", Count: 15},
		{ID: "id-3", Count: 10},
		{ID: "id-4", Count: 3},
	}

	// Filter with threshold of 10
	filtered := templates.FilterByMinCount(10)

	// Should only include templates with count >= 10
	if len(filtered) != 2 {
		t.Fatalf("Expected 2 templates after filtering, got %d", len(filtered))
	}

	// Verify correct templates were kept
	foundIDs := make(map[string]bool)
	for _, tmpl := range filtered {
		foundIDs[tmpl.ID] = true
	}

	if !foundIDs["id-2"] || !foundIDs["id-3"] {
		t.Error("FilterByMinCount did not return correct templates")
	}

	if foundIDs["id-1"] || foundIDs["id-4"] {
		t.Error("FilterByMinCount included templates below threshold")
	}
}

func TestTemplate_Structure(t *testing.T) {
	now := time.Now()

	template := Template{
		ID:        GenerateTemplateID("default", "test pattern"),
		Namespace: "default",
		Pattern:   "test pattern",
		Tokens:    []string{"test", "pattern"},
		Count:     42,
		FirstSeen: now.Add(-1 * time.Hour),
		LastSeen:  now,
	}

	// Verify all fields are accessible
	if template.ID == "" {
		t.Error("Template ID is empty")
	}
	if template.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got %s", template.Namespace)
	}
	if template.Pattern != "test pattern" {
		t.Errorf("Expected pattern 'test pattern', got %s", template.Pattern)
	}
	if len(template.Tokens) != 2 {
		t.Errorf("Expected 2 tokens, got %d", len(template.Tokens))
	}
	if template.Count != 42 {
		t.Errorf("Expected count 42, got %d", template.Count)
	}
	if template.FirstSeen.IsZero() {
		t.Error("FirstSeen is zero")
	}
	if template.LastSeen.IsZero() {
		t.Error("LastSeen is zero")
	}
}
