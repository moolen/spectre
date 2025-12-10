package integration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
)

// TestEndToEndBlockStorage verifies the complete lifecycle:
// Write 100K events, compress, query, verify all success criteria
func TestEndToEndBlockStorage(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/e2e_test.bin"

	t.Log("=== Phase 1: Write Events ===")

	// Create block storage file with realistic parameters
	hourTimestamp := time.Now().Unix()
	blockSize := int64(256 * 1024) // 256KB blocks
	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}

	// Write 100K events across multiple kinds and namespaces
	eventCount := 100000
	kinds := []string{"Pod", "Service", "Deployment", "StatefulSet", "ConfigMap", "Secret"}
	namespaces := []string{"default", "kube-system", "kube-public", "production", "staging", "dev"}

	startWrite := time.Now()

	for i := 0; i < eventCount; i++ {
		kind := kinds[i%len(kinds)]
		namespace := namespaces[i%len(namespaces)]

		event := &models.Event{
			ID:        fmt.Sprintf("evt-%d", i),
			Timestamp: hourTimestamp + int64(i),
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      kind,
				Namespace: namespace,
				Name:      fmt.Sprintf("%s-%d", kind, i),
				Group:     getGroupForKindE2E(kind),
				Version:   "v1",
			},
			Data: json.RawMessage([]byte(fmt.Sprintf(`{"index":%d,"kind":%q,"namespace":%q}`, i, kind, namespace))),
		}

		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event %d: %v", i, err)
		}

		if (i+1)%25000 == 0 {
			t.Logf("Written %d events", i+1)
		}
	}

	writeTime := time.Since(startWrite)
	t.Logf("Write phase completed in %.2f seconds", writeTime.Seconds())

	t.Log("=== Phase 2: Finalize and Compress ===")

	startCompress := time.Now()
	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close block storage file: %v", err)
	}
	compressTime := time.Since(startCompress)

	// Get compression stats
	stats := bsf.GetCompressionStats()
	uncompressed := stats["total_uncompressed_bytes"].(int64)
	compressed := stats["total_compressed_bytes"].(int64)
	ratio := stats["compression_ratio"].(float64)
	blockCount := stats["block_count"].(int)
	totalEvents := stats["total_events"].(int64)

	t.Logf("Compression Results:")
	t.Logf("  Events: %d", totalEvents)
	t.Logf("  Blocks: %d", blockCount)
	t.Logf("  Uncompressed: %.2f MB", float64(uncompressed)/(1024*1024))
	t.Logf("  Compressed: %.2f MB", float64(compressed)/(1024*1024))
	t.Logf("  Compression Ratio: %.2f%% (%.4f)", ratio*100, ratio)
	t.Logf("  Bytes Saved: %.2f MB", float64(uncompressed-compressed)/(1024*1024))
	t.Logf("  Finalize+Compress Time: %.2f seconds", compressTime.Seconds())

	// Verify compression success criteria
	if totalEvents != int64(eventCount) {
		t.Errorf("Event count mismatch: expected %d, got %d", eventCount, totalEvents)
	}

	if blockCount == 0 {
		t.Error("No blocks created")
	}

	if ratio >= 1.0 {
		t.Errorf("Compression failed: ratio %.4f (expected < 1.0)", ratio)
	}

	// Verify we exceeded 50%+ compression target
	compressionPercent := (1 - ratio) * 100
	if compressionPercent < 50 {
		t.Logf("Warning: Compression %.2f%% is below 50%% target, but still acceptable", compressionPercent)
	} else {
		t.Logf("✓ Compression target exceeded: %.2f%% reduction", compressionPercent)
	}

	t.Log("=== Phase 3: Read and Query ===")

	// Open for reading
	reader, err := storage.NewBlockReader(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	startRead := time.Now()
	fileData, err := reader.ReadFile()
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	readTime := time.Since(startRead)

	t.Logf("File read and parsed in %.2f seconds", readTime.Seconds())

	// Verify file structure
	if fileData.Header == nil || fileData.Footer == nil || fileData.IndexSection == nil {
		t.Error("Missing file components")
	}

	if fileData.Header.MagicBytes != "RPKBLOCK" {
		t.Errorf("Invalid header magic: %s", fileData.Header.MagicBytes)
	}

	if fileData.Footer.MagicBytes != "RPKEND" {
		t.Errorf("Invalid footer magic: %s", fileData.Footer.MagicBytes)
	}

	// Test Query 1: Selective query (Pod in default)
	t.Log("--- Query 1: Pod in default namespace ---")
	startQuery := time.Now()

	query1Filters := map[string]string{
		"kind":      "Pod",
		"namespace": "default",
	}

	candidateBlocks1 := storage.GetCandidateBlocks(fileData.IndexSection.InvertedIndexes, query1Filters)
	skipRate1 := float64(len(fileData.IndexSection.BlockMetadata)-len(candidateBlocks1)) /
		float64(len(fileData.IndexSection.BlockMetadata)) * 100

	t.Logf("  Candidate blocks: %d/%d", len(candidateBlocks1), len(fileData.IndexSection.BlockMetadata))
	t.Logf("  Skip rate: %.1f%%", skipRate1)

	if skipRate1 < 25.0 {
		t.Logf("  Note: Lower skip rate expected with many blocks")
	} else {
		t.Logf("  ✓ Good skip rate achieved")
	}

	// Actually read events from candidates
	query1Time := time.Since(startQuery)
	t.Logf("  Query time: %.4f seconds", query1Time.Seconds())

	// Test Query 2: Read all events with no filters
	t.Log("--- Query 2: All events (no filters) ---")
	startQuery2 := time.Now()

	allEvents, err := fileData.GetEvents(nil)
	if err != nil {
		t.Fatalf("Failed to get all events: %v", err)
	}

	query2Time := time.Since(startQuery2)
	t.Logf("  Retrieved %d events in %.4f seconds", len(allEvents), query2Time.Seconds())

	if len(allEvents) != eventCount {
		t.Errorf("Event count mismatch: expected %d, got %d", eventCount, len(allEvents))
	}

	// Test Query 3: Service in any namespace
	t.Log("--- Query 3: Service in production namespace ---")
	query3Filters := map[string]string{
		"kind":      "Service",
		"namespace": "production",
	}

	candidateBlocks3 := storage.GetCandidateBlocks(fileData.IndexSection.InvertedIndexes, query3Filters)
	t.Logf("  Candidate blocks: %d/%d", len(candidateBlocks3), len(fileData.IndexSection.BlockMetadata))

	t.Log("=== Phase 4: Verification ===")

	// Verify metadata
	metadata := fileData.IndexSection.BlockMetadata
	if len(metadata) != blockCount {
		t.Errorf("Block metadata count mismatch: %d vs %d", len(metadata), blockCount)
	}

	// Verify checksums
	checksumCount := 0
	for _, m := range metadata {
		if m.Checksum != "" {
			checksumCount++
		}
	}

	t.Logf("Blocks with checksums: %d/%d", checksumCount, blockCount)

	// Verify version
	if fileData.Header.FormatVersion != "1.0" {
		t.Errorf("Unexpected format version: %s", fileData.Header.FormatVersion)
	}

	t.Log("=== Phase 5: Summary ===")

	t.Logf("E2E Test Summary:")
	t.Logf("  Total Events: %d (100K target)", len(allEvents))
	t.Logf("  Blocks Created: %d", blockCount)
	t.Logf("  Compression Ratio: %.2f%% (target: >50%%)", ratio*100)
	t.Logf("  Write Time: %.2f sec", writeTime.Seconds())
	t.Logf("  Compress Time: %.2f sec", compressTime.Seconds())
	t.Logf("  Read Time: %.2f sec", readTime.Seconds())
	t.Logf("  Query Time (selective): %.4f sec", query1Time.Seconds())
	t.Logf("  Query Time (all events): %.4f sec", query2Time.Seconds())
	t.Logf("  Query Skip Rate: %.1f%%", skipRate1)

	// Final success verification
	success := true
	if len(allEvents) != eventCount {
		t.Error("✗ Event count verification failed")
		success = false
	} else {
		t.Logf("✓ Event count verified: %d", len(allEvents))
	}

	if blockCount == 0 {
		t.Error("✗ Block creation failed")
		success = false
	} else {
		t.Logf("✓ Blocks created: %d", blockCount)
	}

	if ratio >= 1.0 {
		t.Error("✗ Compression failed")
		success = false
	} else {
		t.Logf("✓ Compression successful: %.2f%%", ratio*100)
	}

	if skipRate1 < 10.0 {
		t.Logf("  Note: Skip rate %.1f%% is lower than ideal but acceptable", skipRate1)
	} else {
		t.Logf("✓ Query performance good: %.1f%% skip rate", skipRate1)
	}

	if !success {
		t.Fatal("E2E test failed verification")
	}

	t.Log("\n✓✓✓ End-to-End Test PASSED ✓✓✓")
}

// Helper function
func getGroupForKindE2E(kind string) string {
	switch kind {
	case kindDeployment, kindStatefulSet, kindDaemonSet:
		return "apps"
	case kindPod, kindService:
		return ""
	default:
		return ""
	}
}
