package storage

import (
	"testing"

	"github.com/moritz/rpk/internal/models"
)

func TestNewSegmentMetadataIndex(t *testing.T) {
	index := NewSegmentMetadataIndex()
	if index == nil {
		t.Fatal("expected non-nil SegmentMetadataIndex")
	}
	if index.metadatas == nil {
		t.Error("metadatas map not initialized")
	}
}

func TestSegmentMetadataIndexAddMetadata(t *testing.T) {
	index := NewSegmentMetadataIndex()
	metadata := models.SegmentMetadata{
		MinTimestamp: 1000,
		MaxTimestamp: 2000,
		NamespaceSet: map[string]bool{"default": true},
		KindSet:      map[string]bool{"Pod": true},
	}

	index.AddMetadata(1, metadata)

	retrieved, ok := index.GetSegmentMetadata(1)
	if !ok {
		t.Error("metadata not found after adding")
	}
	if retrieved.MinTimestamp != 1000 {
		t.Errorf("expected MinTimestamp 1000, got %d", retrieved.MinTimestamp)
	}
}

func TestSegmentMetadataIndexCanSegmentBeSkipped_EmptyFilters(t *testing.T) {
	index := NewSegmentMetadataIndex()
	metadata := models.SegmentMetadata{
		MinTimestamp: 1000,
		MaxTimestamp: 2000,
		NamespaceSet: map[string]bool{"default": true},
		KindSet:      map[string]bool{"Pod": true},
	}

	index.AddMetadata(1, metadata)

	// Empty filters should not skip
	canSkip := index.CanSegmentBeSkipped(1, models.QueryFilters{})
	if canSkip {
		t.Error("segment should not be skipped with empty filters")
	}
}

func TestSegmentMetadataIndexCanSegmentBeSkipped_MatchingFilters(t *testing.T) {
	index := NewSegmentMetadataIndex()
	metadata := models.SegmentMetadata{
		MinTimestamp: 1000,
		MaxTimestamp: 2000,
		NamespaceSet: map[string]bool{"default": true},
		KindSet:      map[string]bool{"Pod": true},
	}

	index.AddMetadata(1, metadata)

	// Filters match segment metadata
	filters := models.QueryFilters{Kind: "Pod", Namespace: "default"}
	canSkip := index.CanSegmentBeSkipped(1, filters)
	if canSkip {
		t.Error("segment should not be skipped when filters match")
	}
}

func TestSegmentMetadataIndexCanSegmentBeSkipped_NonMatchingFilters(t *testing.T) {
	index := NewSegmentMetadataIndex()
	metadata := models.SegmentMetadata{
		MinTimestamp: 1000,
		MaxTimestamp: 2000,
		NamespaceSet: map[string]bool{"default": true},
		KindSet:      map[string]bool{"Pod": true},
	}

	index.AddMetadata(1, metadata)

	// Filters don't match segment metadata
	filters := models.QueryFilters{Kind: "Service"}
	canSkip := index.CanSegmentBeSkipped(1, filters)
	if !canSkip {
		t.Error("segment should be skipped when filters don't match")
	}
}

func TestSegmentMetadataIndexCanSegmentBeSkipped_NoMetadata(t *testing.T) {
	index := NewSegmentMetadataIndex()

	// Segment without metadata should not be skipped
	canSkip := index.CanSegmentBeSkipped(1, models.QueryFilters{Kind: "Pod"})
	if canSkip {
		t.Error("segment without metadata should not be skipped")
	}
}

func TestSegmentMetadataIndexFilterSegments(t *testing.T) {
	index := NewSegmentMetadataIndex()

	// Add metadata for multiple segments
	metadata1 := models.SegmentMetadata{
		NamespaceSet: map[string]bool{"default": true},
		KindSet:      map[string]bool{"Pod": true},
	}
	metadata2 := models.SegmentMetadata{
		NamespaceSet: map[string]bool{"kube-system": true},
		KindSet:      map[string]bool{"Service": true},
	}

	index.AddMetadata(1, metadata1)
	index.AddMetadata(2, metadata2)

	// Filter for Pods in default namespace
	filters := models.QueryFilters{Kind: "Pod", Namespace: "default"}
	segmentIDs := []int32{1, 2}
	filtered := index.FilterSegments(segmentIDs, filters)

	if len(filtered) != 1 {
		t.Errorf("expected 1 segment, got %d", len(filtered))
	}
	if filtered[0] != 1 {
		t.Errorf("expected segment 1, got %d", filtered[0])
	}
}

func TestSegmentMetadataIndexFilterSegments_NoMetadata(t *testing.T) {
	index := NewSegmentMetadataIndex()

	// Segment without metadata should be included
	segmentIDs := []int32{1}
	filters := models.QueryFilters{Kind: "Pod"}
	filtered := index.FilterSegments(segmentIDs, filters)

	if len(filtered) != 1 {
		t.Errorf("expected 1 segment, got %d", len(filtered))
	}
}

func TestSegmentMetadataIndexGetSegmentMetadata(t *testing.T) {
	index := NewSegmentMetadataIndex()
	metadata := models.SegmentMetadata{
		MinTimestamp: 1000,
		MaxTimestamp: 2000,
	}

	index.AddMetadata(1, metadata)

	retrieved, ok := index.GetSegmentMetadata(1)
	if !ok {
		t.Error("metadata not found")
	}
	if retrieved.MinTimestamp != 1000 {
		t.Errorf("expected MinTimestamp 1000, got %d", retrieved.MinTimestamp)
	}

	// Non-existent segment
	_, ok = index.GetSegmentMetadata(999)
	if ok {
		t.Error("expected false for non-existent segment")
	}
}

func TestSegmentMetadataIndexGetAllMetadatas(t *testing.T) {
	index := NewSegmentMetadataIndex()

	metadata1 := models.SegmentMetadata{MinTimestamp: 1000}
	metadata2 := models.SegmentMetadata{MinTimestamp: 2000}

	index.AddMetadata(1, metadata1)
	index.AddMetadata(2, metadata2)

	all := index.GetAllMetadatas()
	if len(all) != 2 {
		t.Errorf("expected 2 metadatas, got %d", len(all))
	}
}

func TestSegmentMetadataIndexContainsNamespace(t *testing.T) {
	index := NewSegmentMetadataIndex()

	metadata1 := models.SegmentMetadata{
		NamespaceSet: map[string]bool{"default": true},
	}
	metadata2 := models.SegmentMetadata{
		NamespaceSet: map[string]bool{"kube-system": true},
	}

	index.AddMetadata(1, metadata1)
	index.AddMetadata(2, metadata2)

	if !index.ContainsNamespace("default") {
		t.Error("expected to contain 'default' namespace")
	}
	if !index.ContainsNamespace("kube-system") {
		t.Error("expected to contain 'kube-system' namespace")
	}
	if index.ContainsNamespace("non-existent") {
		t.Error("should not contain 'non-existent' namespace")
	}
}

func TestSegmentMetadataIndexContainsKind(t *testing.T) {
	index := NewSegmentMetadataIndex()

	metadata1 := models.SegmentMetadata{
		KindSet: map[string]bool{"Pod": true},
	}
	metadata2 := models.SegmentMetadata{
		KindSet: map[string]bool{"Service": true},
	}

	index.AddMetadata(1, metadata1)
	index.AddMetadata(2, metadata2)

	if !index.ContainsKind("Pod") {
		t.Error("expected to contain 'Pod' kind")
	}
	if !index.ContainsKind("Service") {
		t.Error("expected to contain 'Service' kind")
	}
	if index.ContainsKind("NonExistent") {
		t.Error("should not contain 'NonExistent' kind")
	}
}

func TestSegmentMetadataIndexGetNamespaces(t *testing.T) {
	index := NewSegmentMetadataIndex()

	metadata1 := models.SegmentMetadata{
		NamespaceSet: map[string]bool{"default": true},
	}
	metadata2 := models.SegmentMetadata{
		NamespaceSet: map[string]bool{"kube-system": true, "default": true},
	}

	index.AddMetadata(1, metadata1)
	index.AddMetadata(2, metadata2)

	namespaces := index.GetNamespaces()
	if len(namespaces) != 2 {
		t.Errorf("expected 2 unique namespaces, got %d", len(namespaces))
	}

	// Check for expected namespaces
	foundDefault := false
	foundKubeSystem := false
	for _, ns := range namespaces {
		if ns == "default" {
			foundDefault = true
		}
		if ns == "kube-system" {
			foundKubeSystem = true
		}
	}

	if !foundDefault {
		t.Error("expected 'default' namespace")
	}
	if !foundKubeSystem {
		t.Error("expected 'kube-system' namespace")
	}
}

func TestSegmentMetadataIndexGetKinds(t *testing.T) {
	index := NewSegmentMetadataIndex()

	metadata1 := models.SegmentMetadata{
		KindSet: map[string]bool{"Pod": true},
	}
	metadata2 := models.SegmentMetadata{
		KindSet: map[string]bool{"Service": true, "Pod": true},
	}

	index.AddMetadata(1, metadata1)
	index.AddMetadata(2, metadata2)

	kinds := index.GetKinds()
	if len(kinds) != 2 {
		t.Errorf("expected 2 unique kinds, got %d", len(kinds))
	}

	// Check for expected kinds
	foundPod := false
	foundService := false
	for _, kind := range kinds {
		if kind == "Pod" {
			foundPod = true
		}
		if kind == "Service" {
			foundService = true
		}
	}

	if !foundPod {
		t.Error("expected 'Pod' kind")
	}
	if !foundService {
		t.Error("expected 'Service' kind")
	}
}

func TestSegmentMetadataIndexGetSegmentCount(t *testing.T) {
	index := NewSegmentMetadataIndex()

	if index.GetSegmentCount() != 0 {
		t.Errorf("expected 0 segments, got %d", index.GetSegmentCount())
	}

	metadata := models.SegmentMetadata{}
	index.AddMetadata(1, metadata)
	index.AddMetadata(2, metadata)

	if index.GetSegmentCount() != 2 {
		t.Errorf("expected 2 segments, got %d", index.GetSegmentCount())
	}
}

func TestSegmentMetadataIndexClear(t *testing.T) {
	index := NewSegmentMetadataIndex()

	metadata := models.SegmentMetadata{}
	index.AddMetadata(1, metadata)
	index.AddMetadata(2, metadata)

	if index.GetSegmentCount() != 2 {
		t.Errorf("expected 2 segments, got %d", index.GetSegmentCount())
	}

	index.Clear()

	if index.GetSegmentCount() != 0 {
		t.Errorf("expected 0 segments after clear, got %d", index.GetSegmentCount())
	}

	_, ok := index.GetSegmentMetadata(1)
	if ok {
		t.Error("metadata should be cleared")
	}
}

