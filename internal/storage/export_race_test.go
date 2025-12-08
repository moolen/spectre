package storage

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExportWithConcurrentWrites reproduces the race condition where:
// 1. Export closes the current file with include_open_hour=true
// 2. A concurrent WriteEvent reopens the file for appending
// 3. Export tries to read the file while it's being written
// This causes a "size mismatch" error
func TestExportWithConcurrentWrites(t *testing.T) {
	// Create a temporary storage instance
	storage, err := New(t.TempDir(), 64*1024)
	require.NoError(t, err)
	defer storage.Close()

	// Write some initial events to create a file
	for i := 0; i < 100; i++ {
		event := &models.Event{
			ID:        "test-event-1",
			Timestamp: time.Now().UnixNano(),
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test-pod",
				UID:       "test-uid",
			},
			Data:     []byte(`{"test":"data"}`),
			DataSize: 15,
		}
		err := storage.WriteEvent(event)
		require.NoError(t, err)
	}

	// Set up concurrent operations
	var wg sync.WaitGroup
	exportErr := make(chan error, 1)
	writeErr := make(chan error, 1)

	// Start export in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		var buf bytes.Buffer
		now := time.Now().Unix()
		opts := ExportOptions{
			StartTime:       now - 3600,
			EndTime:         now + 60,
			IncludeOpenHour: true,
			Compression:     true,
		}
		err := storage.Export(&buf, opts)
		exportErr <- err
	}()

	// Start concurrent writes immediately
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Give export a tiny head start to close the file
		time.Sleep(10 * time.Millisecond)

		// Write many events to increase chance of hitting the race
		for i := 0; i < 100; i++ {
			event := &models.Event{
				ID:        "test-event-2",
				Timestamp: time.Now().UnixNano(),
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					Kind:      "Pod",
					Namespace: "default",
					Name:      "test-pod-concurrent",
					UID:       "test-uid-2",
				},
				Data:     []byte(`{"test":"concurrent"}`),
				DataSize: 21,
			}
			if err := storage.WriteEvent(event); err != nil {
				writeErr <- err
				return
			}
		}
		writeErr <- nil
	}()

	// Wait for both operations to complete
	wg.Wait()
	close(exportErr)
	close(writeErr)

	// Check results
	expErr := <-exportErr
	wrtErr := <-writeErr

	// Write should always succeed
	assert.NoError(t, wrtErr, "concurrent writes should succeed")

	// Export should not have a size mismatch error
	// Before the fix, this would sometimes fail with:
	// "size mismatch: wrote X bytes but file size is Y"
	if expErr != nil {
		assert.NotContains(t, expErr.Error(), "size mismatch",
			"export should not fail with size mismatch due to race condition")
	}
}

// TestExportIncludeOpenHourStability runs multiple iterations to catch race conditions
func TestExportIncludeOpenHourStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stability test in short mode")
	}

	// Run multiple iterations to increase chance of catching the race
	for iteration := 0; iteration < 10; iteration++ {
		t.Run("iteration", func(t *testing.T) {
			storage, err := New(t.TempDir(), 64*1024)
			require.NoError(t, err)
			defer storage.Close()

			// Write some events
			for i := 0; i < 50; i++ {
				event := &models.Event{
					ID:        "test-event",
					Timestamp: time.Now().UnixNano(),
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Kind:      "Deployment",
						Namespace: "test-ns",
						Name:      "test-deploy",
						UID:       "test-uid",
					},
					Data:     []byte(`{"spec":{"replicas":1}}`),
					DataSize: 23,
				}
				require.NoError(t, storage.WriteEvent(event))
			}

			// Simulate concurrent export and writes
			done := make(chan bool)
			var exportErr error

			go func() {
				var buf bytes.Buffer
				now := time.Now().Unix()
				exportErr = storage.Export(&buf, ExportOptions{
					StartTime:       now - 3600,
					EndTime:         now + 60,
					IncludeOpenHour: true,
					Compression:     true,
				})
				done <- true
			}()

			// Concurrent writes
			for i := 0; i < 20; i++ {
				event := &models.Event{
					ID:        "concurrent-event",
					Timestamp: time.Now().UnixNano(),
					Type:      models.EventTypeUpdate,
					Resource: models.ResourceMetadata{
						Kind:      "Deployment",
						Namespace: "test-ns",
						Name:      "concurrent-deploy",
						UID:       "concurrent-uid",
					},
					Data:     []byte(`{"status":"running"}`),
					DataSize: 21,
				}
				storage.WriteEvent(event)
				time.Sleep(1 * time.Millisecond)
			}

			<-done

			// Export should complete without size mismatch
			if exportErr != nil {
				assert.NotContains(t, exportErr.Error(), "size mismatch")
			}
		})
	}
}
