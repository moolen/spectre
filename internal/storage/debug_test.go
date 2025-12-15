package storage

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
)

func TestDebugFinalResourceStates(t *testing.T) {
	tmpDir := t.TempDir()

	// Create storage
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Write an event
	now := time.Now()
	event := &models.Event{
		ID:        "event-test",
		Timestamp: now.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			UID:       "uid-test",
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "test-pod",
		},
		Data: json.RawMessage(`{"status":"Running"}`),
	}

	if err := storage.WriteEvent(event); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

	// Close storage
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Get the file path
	hourTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	filename := hourTime.Format("2006-01-02-15") + ".bin"
	filePath := tmpDir + "/" + filename

	t.Logf("Reading file: %s", filePath)

	// Read the file back
	reader, err := NewBlockReader(filePath)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}
	defer reader.Close()

	fileData, err := reader.ReadFile()
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	t.Logf("File has %d blocks", len(fileData.IndexSection.BlockMetadata))
	t.Logf("File has %d final resource states", len(fileData.IndexSection.FinalResourceStates))

	if len(fileData.IndexSection.FinalResourceStates) == 0 {
		t.Error("FinalResourceStates is empty! This is the bug.")
	} else {
		for key, state := range fileData.IndexSection.FinalResourceStates {
			t.Logf("State: %s -> EventType=%s, UID=%s, Timestamp=%d",
				key, state.EventType, state.UID, state.Timestamp)
		}
	}
}
