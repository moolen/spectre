package storage

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
)

func TestDebugMultipleEvents(t *testing.T) {
	tmpDir := t.TempDir()

	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	t.Logf("Current time: %s", time.Now().Format("2006-01-02 15:04:05"))
	t.Logf("Two hours ago: %s", twoHoursAgo.Format("2006-01-02 15:04:05"))
	t.Logf("Two hours ago hour file: %s", twoHoursAgo.Format("2006-01-02-15"))

	// Create 3 events
	pod := createTestEvent("web-pod", "default", "Pod", twoHoursAgo.UnixNano())
	pod.Data = json.RawMessage(`{"status": {"phase": "Running"}}`)
	t.Logf("Pod event time: %s", time.Unix(0, pod.Timestamp).Format("2006-01-02 15:04:05"))

	deployment := &models.Event{
		ID:        "event-deploy1",
		Timestamp: twoHoursAgo.Add(5 * time.Minute).UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "apps",
			Version:   "v1",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "web-app",
			UID:       "uid-deploy1",
		},
		Data: json.RawMessage(`{"spec": {"replicas": 3}}`),
	}
	t.Logf("Deployment event time: %s", time.Unix(0, deployment.Timestamp).Format("2006-01-02 15:04:05"))

	service := createTestEvent("web-svc", "default", "Service", twoHoursAgo.Add(10*time.Minute).UnixNano())
	service.Data = json.RawMessage(`{"spec": {"type": "ClusterIP"}}`)
	t.Logf("Service event time: %s", time.Unix(0, service.Timestamp).Format("2006-01-02 15:04:05"))

	// Write events
	if err := storage.WriteEvent(pod); err != nil {
		t.Fatalf("failed to write pod: %v", err)
	}
	if err := storage.WriteEvent(deployment); err != nil {
		t.Fatalf("failed to write deployment: %v", err)
	}
	if err := storage.WriteEvent(service); err != nil {
		t.Fatalf("failed to write service: %v", err)
	}

	// Close storage
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Check files
	files, _ := storage.getStorageFiles()
	t.Logf("Number of files created: %d", len(files))
	for i, file := range files {
		t.Logf("File %d: %s", i, file)
		reader, _ := NewBlockReader(file)
		fileData, _ := reader.ReadFile()
		_ = reader.Close()
		t.Logf("  - Events: %d", fileData.IndexSection.Statistics.TotalEvents)
		t.Logf("  - Final resource states: %d", len(fileData.IndexSection.FinalResourceStates))
		for key := range fileData.IndexSection.FinalResourceStates {
			t.Logf("    - %s", key)
		}
	}
}
