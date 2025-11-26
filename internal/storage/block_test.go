package storage

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/moritz/rpk/internal/models"
)

func TestNewEventBuffer(t *testing.T) {
	buffer := NewEventBuffer(1024)

	if buffer.blockSize != 1024 {
		t.Errorf("expected block size 1024, got %d", buffer.blockSize)
	}

	if buffer.currentSize != 0 {
		t.Errorf("expected current size 0, got %d", buffer.currentSize)
	}

	if len(buffer.events) != 0 {
		t.Errorf("expected empty events, got %d", len(buffer.events))
	}

	if buffer.bloomKinds == nil {
		t.Error("expected bloom filter for kinds")
	}

	if buffer.bloomNamespaces == nil {
		t.Error("expected bloom filter for namespaces")
	}

	if buffer.bloomGroups == nil {
		t.Error("expected bloom filter for groups")
	}
}

func TestEventBufferAddEvent(t *testing.T) {
	buffer := NewEventBuffer(1024)

	event := createTestEvent("pod-1", "default", "Pod", time.Now().UnixNano())
	eventJSON, _ := json.Marshal(event)

	if !buffer.AddEvent(eventJSON) {
		t.Error("expected event to be added")
	}

	if buffer.GetEventCount() != 1 {
		t.Errorf("expected 1 event, got %d", buffer.GetEventCount())
	}

	if buffer.currentSize != int64(len(eventJSON)) {
		t.Errorf("expected current size %d, got %d", len(eventJSON), buffer.currentSize)
	}
}

func TestEventBufferIsFull(t *testing.T) {
	buffer := NewEventBuffer(100)

	// First event should always fit
	event1 := createTestEvent("pod-1", "default", "Pod", time.Now().UnixNano())
	event1JSON, _ := json.Marshal(event1)

	if buffer.IsFull(int64(len(event1JSON))) {
		t.Error("buffer should not be full on first event")
	}

	buffer.AddEvent(event1JSON)

	// Try to add another event that would exceed size
	largeEventJSON := make([]byte, 200)
	if !buffer.IsFull(int64(len(largeEventJSON))) {
		t.Error("buffer should be full when adding would exceed size")
	}
}

func TestEventBufferMetadataTracking(t *testing.T) {
	buffer := NewEventBuffer(10240)

	now := time.Now()
	event1 := createTestEvent("pod-1", "default", "Pod", now.UnixNano())
	event2 := createTestEvent("svc-1", "kube-system", "Service", now.Add(time.Second).UnixNano())
	event3 := createTestEvent("deploy-1", "default", "Deployment", now.Add(2*time.Second).UnixNano())

	event1JSON, _ := json.Marshal(event1)
	event2JSON, _ := json.Marshal(event2)
	event3JSON, _ := json.Marshal(event3)

	buffer.AddEvent(event1JSON)
	buffer.AddEvent(event2JSON)
	buffer.AddEvent(event3JSON)

	// Check timestamp tracking
	if buffer.timestampMin == 0 {
		t.Error("expected timestamp min to be set")
	}
	if buffer.timestampMax == 0 {
		t.Error("expected timestamp max to be set")
	}
	if buffer.timestampMin > buffer.timestampMax {
		t.Error("timestamp min should be <= max")
	}

	// Check kind set
	if len(buffer.kindSet) != 3 {
		t.Errorf("expected 3 unique kinds, got %d", len(buffer.kindSet))
	}
	if !buffer.kindSet["Pod"] || !buffer.kindSet["Service"] || !buffer.kindSet["Deployment"] {
		t.Error("expected all kinds to be tracked")
	}

	// Check namespace set
	if len(buffer.namespaceSet) != 2 {
		t.Errorf("expected 2 unique namespaces, got %d", len(buffer.namespaceSet))
	}
	if !buffer.namespaceSet["default"] || !buffer.namespaceSet["kube-system"] {
		t.Error("expected all namespaces to be tracked")
	}
}

func TestEventBufferFinalize(t *testing.T) {
	buffer := NewEventBuffer(10240)

	now := time.Now()
	event1 := createTestEvent("pod-1", "default", "Pod", now.UnixNano())
	event2 := createTestEvent("svc-1", "default", "Service", now.Add(time.Second).UnixNano())

	event1JSON, _ := json.Marshal(event1)
	event2JSON, _ := json.Marshal(event2)

	buffer.AddEvent(event1JSON)
	buffer.AddEvent(event2JSON)

	block, err := buffer.Finalize(0, "gzip")
	if err != nil {
		t.Fatalf("failed to finalize buffer: %v", err)
	}

	if block.ID != 0 {
		t.Errorf("expected block ID 0, got %d", block.ID)
	}

	if block.EventCount != 2 {
		t.Errorf("expected 2 events, got %d", block.EventCount)
	}

	if block.Metadata == nil {
		t.Fatal("expected block metadata")
	}

	if len(block.Metadata.KindSet) != 2 {
		t.Errorf("expected 2 kinds in metadata, got %d", len(block.Metadata.KindSet))
	}

	if block.Metadata.BloomFilterKinds == nil {
		t.Error("expected bloom filter for kinds")
	}

	if block.UncompressedLength == 0 {
		t.Error("expected uncompressed length to be set")
	}
}

func TestEventBufferFinalizeEmpty(t *testing.T) {
	buffer := NewEventBuffer(1024)

	_, err := buffer.Finalize(0, "gzip")
	if err == nil {
		t.Error("expected error when finalizing empty buffer")
	}
}

func TestCompressBlock(t *testing.T) {
	buffer := NewEventBuffer(10240)

	event := createTestEvent("pod-1", "default", "Pod", time.Now().UnixNano())
	eventJSON, _ := json.Marshal(event)
	buffer.AddEvent(eventJSON)

	block, err := buffer.Finalize(0, "gzip")
	if err != nil {
		t.Fatalf("failed to finalize: %v", err)
	}

	originalLength := block.UncompressedLength

	compressed, err := CompressBlock(block)
	if err != nil {
		t.Fatalf("failed to compress block: %v", err)
	}

	if compressed.Length == 0 {
		t.Error("expected compressed length to be set")
	}

	if compressed.Length >= originalLength {
		t.Errorf("expected compression to reduce size, original: %d, compressed: %d", originalLength, compressed.Length)
	}

	if compressed.Metadata.CompressedLength != compressed.Length {
		t.Error("metadata compressed length should match block length")
	}
}

func TestDecompressBlock(t *testing.T) {
	buffer := NewEventBuffer(10240)

	event := createTestEvent("pod-1", "default", "Pod", time.Now().UnixNano())
	eventJSON, _ := json.Marshal(event)
	buffer.AddEvent(eventJSON)

	block, err := buffer.Finalize(0, "gzip")
	if err != nil {
		t.Fatalf("failed to finalize: %v", err)
	}

	compressed, err := CompressBlock(block)
	if err != nil {
		t.Fatalf("failed to compress: %v", err)
	}

	decompressed, err := DecompressBlock(compressed)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	if len(decompressed) == 0 {
		t.Error("expected decompressed data")
	}

	// Verify we can parse the events back
	lines := bytes.Split(decompressed, []byte("\n"))
	eventCount := 0
	for _, line := range lines {
		if len(line) > 0 {
			eventCount++
			var e models.Event
			if err := json.Unmarshal(line, &e); err != nil {
				t.Fatalf("failed to unmarshal event: %v", err)
			}
			if e.Resource.Kind != "Pod" {
				t.Errorf("expected Pod, got %s", e.Resource.Kind)
			}
		}
	}

	if eventCount != 1 {
		t.Errorf("expected 1 event after decompression, got %d", eventCount)
	}
}

func TestMapToSlice(t *testing.T) {
	m := map[string]bool{
		"a": true,
		"b": true,
		"c": true,
	}

	slice := mapToSlice(m)
	if len(slice) != 3 {
		t.Errorf("expected 3 elements, got %d", len(slice))
	}

	// Check all elements are present
	seen := make(map[string]bool)
	for _, v := range slice {
		seen[v] = true
	}

	if !seen["a"] || !seen["b"] || !seen["c"] {
		t.Error("expected all elements to be present")
	}
}

func TestMapToSliceEmpty(t *testing.T) {
	slice := mapToSlice(map[string]bool{})
	if slice != nil {
		t.Error("expected nil for empty map")
	}
}


