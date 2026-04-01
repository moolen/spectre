package importexport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

func TestImportInChunks(t *testing.T) {
	tmpDir := t.TempDir()

	writeEventsFile := func(path string, events []models.Event) {
		t.Helper()
		payload, err := json.Marshal(BatchEventImportRequest{Events: events})
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			t.Fatalf("write file failed: %v", err)
		}
	}

	aPath := filepath.Join(tmpDir, "a.json")
	bPath := filepath.Join(tmpDir, "b.json")

	writeEventsFile(aPath, []models.Event{
		{
			ID:        "a-1",
			Timestamp: 1000,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind: "Deployment",
				Name: "a-1",
			},
		},
		{
			ID:        "a-2",
			Timestamp: 1001,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind: "Deployment",
				Name: "a-2",
			},
		},
		{
			ID:        "a-3",
			Timestamp: 1002,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind: "Deployment",
				Name: "a-3",
			},
		},
	})
	writeEventsFile(bPath, []models.Event{
		{
			ID:        "b-1",
			Timestamp: 2000,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind: "Deployment",
				Name: "b-1",
			},
		},
		{
			ID:        "b-2",
			Timestamp: 2001,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind: "Deployment",
				Name: "b-2",
			},
		},
	})

	var chunkSizes []int
	var seenIDs []string

	err := ImportInChunks(FromDirectory(tmpDir), 2, func(chunk []models.Event) error {
		chunkSizes = append(chunkSizes, len(chunk))
		for _, event := range chunk {
			seenIDs = append(seenIDs, event.ID)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ImportInChunks failed: %v", err)
	}

	wantChunkSizes := []int{2, 2, 1}
	if fmt.Sprint(chunkSizes) != fmt.Sprint(wantChunkSizes) {
		t.Fatalf("chunk sizes = %v, want %v", chunkSizes, wantChunkSizes)
	}

	// Directory imports should be deterministic so chunk boundaries are stable.
	wantIDs := []string{"a-1", "a-2", "a-3", "b-1", "b-2"}
	if fmt.Sprint(seenIDs) != fmt.Sprint(wantIDs) {
		t.Fatalf("event order = %v, want %v", seenIDs, wantIDs)
	}
}

func TestImportInChunksValidationAndEnrichment(t *testing.T) {
	input := `{
		"events": [
			{
				"id": "k8s-event",
				"timestamp": 1234567890000000000,
				"type": "CREATE",
				"resource": {
					"group": "",
					"version": "v1",
					"kind": "Event",
					"namespace": "default",
					"name": "test-event",
					"uid": "event-uid-1"
				},
				"data": {
					"involvedObject": {
						"uid": "pod-uid-123"
					}
				}
			},
			{
				"id": "",
				"timestamp": 1234567891000000000,
				"type": "CREATE",
				"resource": {
					"kind": "Deployment",
					"name": "invalid"
				}
			}
		]
	}`

	var chunks [][]models.Event
	err := ImportInChunks(FromReader(strings.NewReader(input)), 2, func(chunk []models.Event) error {
		chunks = append(chunks, append([]models.Event(nil), chunk...))
		return nil
	})
	if err != nil {
		t.Fatalf("ImportInChunks failed: %v", err)
	}

	if len(chunks) != 1 || len(chunks[0]) != 1 {
		t.Fatalf("chunks = %#v, want one chunk with one valid event", chunks)
	}

	if chunks[0][0].Resource.InvolvedObjectUID != "pod-uid-123" {
		t.Fatalf("InvolvedObjectUID = %q, want %q", chunks[0][0].Resource.InvolvedObjectUID, "pod-uid-123")
	}
}

func TestImportInChunksStreamsBeforeLaterParseError(t *testing.T) {
	input := `{
		"events": [
			{
				"id": "event-1",
				"timestamp": 1,
				"type": "CREATE",
				"resource": {"kind": "Deployment", "name": "a"}
			},
			{
				"id": "event-2",
				"timestamp": 2,
				"type": "CREATE",
				"resource": {"kind": "Deployment", "name": "b"}
			},
			{
				"id": "broken-event",
				"timestamp":
			}
		]
	}`

	var gotChunks [][]models.Event
	err := ImportInChunks(FromReader(strings.NewReader(input)), 2, func(chunk []models.Event) error {
		gotChunks = append(gotChunks, append([]models.Event(nil), chunk...))
		return nil
	})
	if err == nil {
		t.Fatalf("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse JSON") {
		t.Fatalf("error = %v, want failed-to-parse message", err)
	}
	if len(gotChunks) != 1 || len(gotChunks[0]) != 2 {
		t.Fatalf("got chunks = %#v, want one emitted chunk of size 2 before parse failure", gotChunks)
	}
}

func TestImportUsesStreamingWhenAvailable(t *testing.T) {
	source := &streamPreferredSource{}

	events, err := Import(source, WithLogger(logging.GetLogger("test")))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if len(events) != 1 || events[0].ID != "streamed" {
		t.Fatalf("events = %#v, want one streamed event", events)
	}
	if source.loadCalled {
		t.Fatalf("expected legacy Load not to be called when streaming is available")
	}
}

type streamPreferredSource struct {
	loadCalled bool
}

func (s *streamPreferredSource) Load(_ *logging.Logger) ([]models.Event, error) {
	s.loadCalled = true
	return nil, fmt.Errorf("legacy load should not be called")
}

func (s *streamPreferredSource) streamLoad(_ *logging.Logger, _ int, onChunk ChunkCallback) error {
	return onChunk([]models.Event{
		{
			ID:        "streamed",
			Timestamp: 1,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind: "Deployment",
				Name: "streamed",
			},
		},
	})
}
