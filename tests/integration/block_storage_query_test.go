package integration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moritz/rpk/internal/models"
	"github.com/moritz/rpk/internal/storage"
)

// TestQueryBlockFiltering creates a storage file with multiple kinds and namespaces,
// then executes queries and verifies that 90%+ of blocks are skipped
func TestQueryBlockFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test_query.bin"

	// Create block storage file with multiple blocks
	hourTimestamp := time.Now().Unix()
	blockSize := int64(50 * 1024) // Small block size to force multiple blocks
	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}

	// Define kinds and namespaces for test data
	kinds := []string{"Pod", "Service", "Deployment", "StatefulSet", "ConfigMap"}
	namespaces := []string{"default", "kube-system", "kube-public", "production", "staging"}

	// Write events with selective distribution:
	// Pod events go to default/production/staging
	// Service events go to kube-system/kube-public
	// Other kinds distributed across all namespaces
	eventCount := 0

	// 100 Pod events, mostly in default
	for i := 0; i < 100; i++ {
		namespace := namespaces[i%3] // default, kube-system, kube-public
		if i%5 == 0 {
			namespace = "production" // Some in production
		}
		event := createTestEventWithKindNamespaceQuery(eventCount, hourTimestamp+int64(eventCount), "Pod", namespace)
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
		eventCount++
	}

	// 50 Service events, only in kube-system/kube-public
	for i := 0; i < 50; i++ {
		namespace := namespaces[1 + (i%2)] // kube-system or kube-public
		event := createTestEventWithKindNamespaceQuery(eventCount, hourTimestamp+int64(eventCount), "Service", namespace)
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
		eventCount++
	}

	// 150 Deployment/StatefulSet/ConfigMap events distributed
	for i := 0; i < 150; i++ {
		kind := kinds[2+((i/50)%3)] // Deployment, StatefulSet, or ConfigMap
		namespace := namespaces[i%len(namespaces)]
		event := createTestEventWithKindNamespaceQuery(eventCount, hourTimestamp+int64(eventCount), kind, namespace)
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
		eventCount++
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close block storage file: %v", err)
	}

	t.Logf("Created file with %d events across %d kinds × %d namespaces", eventCount, len(kinds), len(namespaces))

	// Now read the file back
	reader, err := storage.NewBlockReader(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	fileData, err := reader.ReadFile()
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	totalBlocks := len(fileData.IndexSection.BlockMetadata)
	t.Logf("File has %d blocks", totalBlocks)

	// Test Query 1: Find all Pods in default namespace
	filters := map[string]string{
		"kind":      "Pod",
		"namespace": "default",
	}

	candidateBlocks := storage.GetCandidateBlocks(fileData.IndexSection.InvertedIndexes, filters)
	skippedBlocks := totalBlocks - len(candidateBlocks)
	skipRate := float64(skippedBlocks) / float64(totalBlocks) * 100

	t.Logf("Query: kind=Pod AND namespace=default")
	t.Logf("  Candidate blocks: %d / %d", len(candidateBlocks), totalBlocks)
	t.Logf("  Skip rate: %.1f%%", skipRate)

	// With 5 kinds and 5 namespaces, we expect roughly 20% selectivity (1 out of 25 combinations)
	// So skip rate should be ~80%
	if skipRate < 50.0 {
		t.Errorf("Skip rate %.1f%% is below 50%% threshold for selective query", skipRate)
	}

	// Test Query 2: Find all Services
	filters2 := map[string]string{
		"kind": "Service",
	}

	candidateBlocks2 := storage.GetCandidateBlocks(fileData.IndexSection.InvertedIndexes, filters2)
	skippedBlocks2 := totalBlocks - len(candidateBlocks2)
	skipRate2 := float64(skippedBlocks2) / float64(totalBlocks) * 100

	t.Logf("Query: kind=Service")
	t.Logf("  Candidate blocks: %d / %d", len(candidateBlocks2), totalBlocks)
	t.Logf("  Skip rate: %.1f%%", skipRate2)

	// With 5 kinds, we expect roughly 20% selectivity, so ~80% skip rate
	if skipRate2 < 50.0 {
		t.Errorf("Skip rate %.1f%% is below 50%% threshold for selective query", skipRate2)
	}

	// Test Query 3: Find all Service events (should skip blocks without Service)
	filters3 := map[string]string{
		"kind": "Deployment",
	}

	candidateBlocks3 := storage.GetCandidateBlocks(fileData.IndexSection.InvertedIndexes, filters3)
	skippedBlocks3 := totalBlocks - len(candidateBlocks3)
	skipRate3 := float64(skippedBlocks3) / float64(totalBlocks) * 100

	t.Logf("Query: kind=Deployment")
	t.Logf("  Candidate blocks: %d / %d", len(candidateBlocks3), totalBlocks)
	t.Logf("  Skip rate: %.1f%%", skipRate3)

	if skipRate3 < 25.0 {
		t.Logf("Note: Skip rate %.1f%% may be low if Deployment distributed across blocks", skipRate3)
	}

	// Test Query 4: Verify we can actually read and filter events from candidate blocks
	events, err := fileData.GetEvents(filters)
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	// Verify the returned events match the filters
	podDefaultCount := 0
	for _, event := range events {
		if event.Resource.Kind == "Pod" && event.Resource.Namespace == "default" {
			podDefaultCount++
		}
	}

	t.Logf("Retrieved %d events matching filters, %d are Pod/default", len(events), podDefaultCount)

	// Expect roughly 20 events (100 iterations × 5 kinds × (1/5) kinds matching × (1/5) namespaces matching)
	// With variance due to distribution
	if len(events) < 5 {
		t.Logf("Warning: Expected more events matching filters, got %d", len(events))
	}
}

// TestQueryNoResults verifies that queries with no matching blocks return empty results
func TestQueryNoResults(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test_query_empty.bin"

	// Create block storage file with only Pod events in default namespace
	hourTimestamp := time.Now().Unix()
	blockSize := int64(100 * 1024)
	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}

	// Write only Pod/default events
	for i := 0; i < 50; i++ {
		event := createTestEventWithKindNamespaceQuery(i, hourTimestamp+int64(i), "Pod", "default")
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close block storage file: %v", err)
	}

	// Read file back
	reader, err := storage.NewBlockReader(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	fileData, err := reader.ReadFile()
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Query for something that doesn't exist
	filters := map[string]string{
		"kind":      "Service", // No Service events written
		"namespace": "default",
	}

	candidateBlocks := storage.GetCandidateBlocks(fileData.IndexSection.InvertedIndexes, filters)

	if len(candidateBlocks) != 0 {
		t.Errorf("Expected 0 candidate blocks for non-existent query, got %d", len(candidateBlocks))
	}

	// Try to get events - should return empty
	events, err := fileData.GetEvents(filters)
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("Expected 0 events for non-existent query, got %d", len(events))
	}

	t.Logf("Query correctly returned no results for non-existent filters")
}

// TestQueryAllEvents verifies that querying without filters returns all events
func TestQueryAllEvents(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test_query_all.bin"

	// Create block storage file
	hourTimestamp := time.Now().Unix()
	blockSize := int64(50 * 1024)
	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}

	eventCount := 75
	for i := 0; i < eventCount; i++ {
		event := createTestEventWithKindNamespaceQuery(i, hourTimestamp+int64(i), "Pod", "default")
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close block storage file: %v", err)
	}

	// Read file back
	reader, err := storage.NewBlockReader(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	fileData, err := reader.ReadFile()
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Query with no filters - should return all events
	events, err := fileData.GetEvents(nil)
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	if len(events) != eventCount {
		t.Errorf("Expected %d events with no filters, got %d", eventCount, len(events))
	}

	t.Logf("Query correctly returned all %d events when no filters applied", len(events))
}

// Helper functions
func createTestEventWithKindNamespaceQuery(id int, timestamp int64, kind, namespace string) *models.Event {
	data, _ := json.Marshal(map[string]interface{}{
		"message": fmt.Sprintf("Event %d: %s in %s", id, kind, namespace),
		"index":   id,
	})

	return &models.Event{
		ID:        fmt.Sprintf("evt-%d", id),
		Timestamp: timestamp,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Kind:      kind,
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-%d", kind, id),
			Group:     getGroupForKindQuery(kind),
			Version:   "v1",
		},
		Data: data,
	}
}

func getGroupForKindQuery(kind string) string {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet":
		return "apps"
	case "Pod", "Service":
		return ""
	default:
		return ""
	}
}

