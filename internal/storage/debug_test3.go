package storage

import (
	"testing"
	"time"
)

func TestDebugStateCarryover(t *testing.T) {
	tmpDir := t.TempDir()

	// Hour 1: Create long-lived-pod
	baseTime := time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC)

	storage, _ := New(tmpDir, 1024*1024)
	event1 := createTestEvent("long-lived-pod", "default", "Pod", baseTime.UnixNano())
	storage.WriteEvent(event1)
	storage.Close()

	t.Logf("Hour 1: Created long-lived-pod")

	// Check Hour 1 file
	files, _ := storage.getStorageFiles()
	reader, _ := NewBlockReader(files[0])
	fileData, _ := reader.ReadFile()
	reader.Close()
	t.Logf("Hour 1 file: %d final states", len(fileData.IndexSection.FinalResourceStates))
	for key := range fileData.IndexSection.FinalResourceStates {
		t.Logf("  - %s", key)
	}

	// Hour 2: Create unrelated-pod
	storage, _ = New(tmpDir, 1024*1024)
	event2 := createTestEvent("unrelated-pod", "default", "Pod", baseTime.Add(1*time.Hour).UnixNano())
	storage.WriteEvent(event2)
	storage.Close()

	t.Logf("\nHour 2: Created unrelated-pod")

	// Check Hour 2 file
	files, _ = storage.getStorageFiles()
	reader, _ = NewBlockReader(files[len(files)-1])
	fileData, _ = reader.ReadFile()
	reader.Close()
	t.Logf("Hour 2 file: %d final states", len(fileData.IndexSection.FinalResourceStates))
	for key := range fileData.IndexSection.FinalResourceStates {
		t.Logf("  - %s", key)
	}

	// Hour 3: Create another-pod
	storage, _ = New(tmpDir, 1024*1024)
	event3 := createTestEvent("another-pod", "default", "Pod", baseTime.Add(2*time.Hour).UnixNano())
	storage.WriteEvent(event3)
	storage.Close()

	t.Logf("\nHour 3: Created another-pod")

	// Check Hour 3 file
	files, _ = storage.getStorageFiles()
	reader, _ = NewBlockReader(files[len(files)-1])
	fileData, _ = reader.ReadFile()
	reader.Close()
	t.Logf("Hour 3 file: %d final states", len(fileData.IndexSection.FinalResourceStates))
	for key := range fileData.IndexSection.FinalResourceStates {
		t.Logf("  - %s", key)
	}

	// Verify long-lived-pod is still present
	longLivedKey := "/v1/Pod/default/long-lived-pod"
	_, hasLongLived := fileData.IndexSection.FinalResourceStates[longLivedKey]

	if !hasLongLived {
		t.Error("long-lived pod from hour 1 should be carried through to hour 3")
		t.Logf("Expected key: %s", longLivedKey)
		t.Logf("Available keys:")
		for key := range fileData.IndexSection.FinalResourceStates {
			t.Logf("  - %s", key)
		}
	}
}
