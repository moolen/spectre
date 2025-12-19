package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
)

func TestStorageGetInMemoryEvents_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	now := time.Now().Unix()
	query := &models.QueryRequest{
		StartTimestamp: now - 3600,
		EndTimestamp:   now,
		Filters:        models.QueryFilters{},
	}

	events, err := storage.GetInMemoryEvents(query)
	if err != nil {
		t.Fatalf("GetInMemoryEvents failed: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestStorageGetInMemoryEvents_WithBufferedEvents(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events (they'll be buffered in memory)
	now := time.Now()
	baseTime := now.Unix()
	events := []*models.Event{
		createTestEvent("pod1", "default", "Pod", now.UnixNano()),
		createTestEvent("pod2", "default", "Pod", now.Add(1*time.Second).UnixNano()),
		createTestEvent("svc1", "default", "Service", now.Add(2*time.Second).UnixNano()),
	}

	for _, event := range events {
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	query := &models.QueryRequest{
		StartTimestamp: baseTime - 100,
		EndTimestamp:   baseTime + 100,
		Filters:        models.QueryFilters{},
	}

	inMemoryEvents, err := storage.GetInMemoryEvents(query)
	if err != nil {
		t.Fatalf("GetInMemoryEvents failed: %v", err)
	}

	if len(inMemoryEvents) < 3 {
		t.Errorf("expected at least 3 in-memory events, got %d", len(inMemoryEvents))
	}
}

func TestStorageGetInMemoryEvents_WithFilters(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events with different kinds
	now := time.Now()
	baseTime := now.Unix()
	events := []*models.Event{
		createTestEvent("pod1", "default", "Pod", now.UnixNano()),
		createTestEvent("pod2", "default", "Pod", now.Add(1*time.Second).UnixNano()),
		createTestEvent("svc1", "default", "Service", now.Add(2*time.Second).UnixNano()),
	}

	for _, event := range events {
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	query := &models.QueryRequest{
		StartTimestamp: baseTime - 100,
		EndTimestamp:   baseTime + 100,
		Filters:        models.QueryFilters{Kind: "Pod"},
	}

	inMemoryEvents, err := storage.GetInMemoryEvents(query)
	if err != nil {
		t.Fatalf("GetInMemoryEvents failed: %v", err)
	}

	// Should only get Pod events
	for _, event := range inMemoryEvents {
		if event.Resource.Kind != "Pod" {
			t.Errorf("expected Pod, got %s", event.Resource.Kind)
		}
	}
}

func TestStorageGetInMemoryEvents_TimeRangeFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events at different times
	now := time.Now()
	baseTime := now.Unix()
	events := []*models.Event{
		createTestEvent("pod1", "default", "Pod", now.Add(-2*time.Hour).UnixNano()),
		createTestEvent("pod2", "default", "Pod", now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("pod3", "default", "Pod", now.Add(30*time.Minute).UnixNano()),
	}

	for _, event := range events {
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// Query for last hour only
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 3600,
		EndTimestamp:   baseTime,
		Filters:        models.QueryFilters{},
	}

	inMemoryEvents, err := storage.GetInMemoryEvents(query)
	if err != nil {
		t.Fatalf("GetInMemoryEvents failed: %v", err)
	}

	// Should only get events within time range
	for _, event := range inMemoryEvents {
		eventTime := time.Unix(0, event.Timestamp).Unix()
		if eventTime < baseTime-3600 || eventTime > baseTime {
			t.Errorf("event outside time range: %d not in [%d, %d]", eventTime, baseTime-3600, baseTime)
		}
	}
}

func TestStorageGetInMemoryEvents_WithFinalizedSegments(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024) // Small segment size to force finalization
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write enough events to trigger segment finalization
	now := time.Now()
	baseTime := now.Unix()
	for i := 0; i < 100; i++ {
		event := createTestEvent("pod1", "default", "Pod", now.Add(time.Duration(i)*time.Second).UnixNano())
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	query := &models.QueryRequest{
		StartTimestamp: baseTime - 100,
		EndTimestamp:   baseTime + 10000,
		Filters:        models.QueryFilters{},
	}

	inMemoryEvents, err := storage.GetInMemoryEvents(query)
	if err != nil {
		t.Fatalf("GetInMemoryEvents failed: %v", err)
	}

	// Should find events from both finalized segments and current buffer
	if len(inMemoryEvents) == 0 {
		t.Error("expected to find in-memory events")
	}
}

func TestStorageStart(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	if err := storage.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
}

func TestStorageStart_CancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = storage.Start(ctx)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestStorageStop(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Write some events
	now := time.Now()
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod1", "default", "Pod", now.Add(time.Duration(i)*time.Second).UnixNano())
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	ctx := context.Background()
	if err := storage.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestStorageStop_WithTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := storage.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestStorageStop_CancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close() // Ensure cleanup even if Stop fails

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = storage.Stop(ctx)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	// Give the Stop() goroutine time to complete its cleanup
	// This prevents race with test cleanup trying to remove temp directory
	time.Sleep(50 * time.Millisecond)
}

func TestStorageName(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	name := storage.Name()
	if name != "Storage" {
		t.Errorf("expected 'Storage', got '%s'", name)
	}
}

func TestStorageGetStorageStats(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write some events
	now := time.Now()
	for i := 0; i < 10; i++ {
		event := createTestEvent("pod1", "default", "Pod", now.Add(time.Duration(i)*time.Second).UnixNano())
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// Close to finalize files
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Reopen
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage: %v", err)
	}
	defer storage.Close()

	stats, err := storage.GetStorageStats()
	if err != nil {
		t.Fatalf("GetStorageStats failed: %v", err)
	}

	if stats["dataDir"] != tmpDir {
		t.Errorf("expected dataDir %s, got %v", tmpDir, stats["dataDir"])
	}

	if stats["fileCount"].(int) == 0 {
		t.Error("expected at least one file")
	}

	if stats["totalSizeBytes"].(int64) < 0 {
		t.Error("total size should be non-negative")
	}
}

func TestStorageListFiles(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events and close to create files
	now := time.Now()
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod1", "default", "Pod", now.Add(time.Duration(i)*time.Second).UnixNano())
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// Close to finalize
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Reopen
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage: %v", err)
	}
	defer storage.Close()

	files, err := storage.ListFiles()
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	if len(files) == 0 {
		t.Error("expected at least one file")
	}

	// Verify files exist
	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("file %s does not exist: %v", file, err)
		}
	}
}

func TestStorageDeleteOldFiles(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a file with an old hour timestamp (48 hours ago)
	oldTime := time.Now().Add(-48 * time.Hour)
	oldFileName := fmt.Sprintf("%04d-%02d-%02d-%02d.bin",
		oldTime.Year(), oldTime.Month(), oldTime.Day(), oldTime.Hour())
	oldFile := filepath.Join(tmpDir, oldFileName)
	file, err := os.Create(oldFile)
	if err != nil {
		t.Fatalf("failed to create old file: %v", err)
	}
	file.Close()

	// Delete files older than 24 hours
	if err := storage.DeleteOldFiles(24); err != nil {
		t.Fatalf("DeleteOldFiles failed: %v", err)
	}

	// Verify old file was deleted
	if _, err := os.Stat(oldFile); err == nil {
		t.Error("expected old file to be deleted")
	}
}

func TestStorageDeleteOldFiles_KeepRecent(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a file with a recent hour timestamp (1 hour ago)
	recentTime := time.Now().Add(-1 * time.Hour)
	recentFileName := fmt.Sprintf("%04d-%02d-%02d-%02d.bin",
		recentTime.Year(), recentTime.Month(), recentTime.Day(), recentTime.Hour())
	recentFile := filepath.Join(tmpDir, recentFileName)
	file, err := os.Create(recentFile)
	if err != nil {
		t.Fatalf("failed to create recent file: %v", err)
	}
	file.Close()

	// Delete files older than 24 hours
	if err := storage.DeleteOldFiles(24); err != nil {
		t.Fatalf("DeleteOldFiles failed: %v", err)
	}

	// Verify recent file was kept
	if _, err := os.Stat(recentFile); err != nil {
		t.Error("expected recent file to be kept")
	}
}
