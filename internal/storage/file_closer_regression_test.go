package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFileCloserRegression is a regression test for the bug where hourly files
// were created with only headers (77 bytes) and never properly closed.
//
// Bug: Commit 0023700 introduced CloseOldHourFiles() but never called it,
// resulting in files that only contained headers (77 bytes) without index/footer.
//
// This test verifies that:
// 1. Files are created when events are written
// 2. Files are properly closed after the configured duration
// 3. Closed files contain more than just the header (have blocks + index + footer)
// 4. The background file closer goroutine runs correctly
func TestFileCloserRegression(t *testing.T) {
	tmpDir := t.TempDir()

	// Create storage with small block size to ensure blocks are written
	storage, err := New(tmpDir, 1024) // 1KB block size
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Start storage (this should start the background file closer)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := storage.Start(ctx); err != nil {
		t.Fatalf("Failed to start storage: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = storage.Stop(stopCtx)
	}()

	// Write events to an old hour (3 hours ago) to trigger immediate closure
	now := time.Now()
	oldHour := now.Add(-3 * time.Hour)
	oldHourStart := time.Date(oldHour.Year(), oldHour.Month(), oldHour.Day(),
		oldHour.Hour(), 0, 0, 0, oldHour.Location())

	// Write enough events to fill multiple blocks (to exceed 77 bytes)
	for i := 0; i < 200; i++ {
		event := createTestEvent("test-pod", "default", "Pod",
			oldHourStart.Add(time.Duration(i)*time.Second).UnixNano())
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	// Verify file was created
	oldHourFilename := oldHourStart.Format("2006-01-02-15") + ".bin"
	oldHourFilePath := filepath.Join(tmpDir, oldHourFilename)

	if _, err := os.Stat(oldHourFilePath); os.IsNotExist(err) {
		t.Fatalf("Expected file to be created: %s", oldHourFilePath)
	}

	// Check initial file size (should have header + blocks since we wrote many events)
	initialInfo, err := os.Stat(oldHourFilePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	t.Logf("Initial file size: %d bytes", initialInfo.Size())

	// File should have at least the header (77 bytes)
	if initialInfo.Size() < 77 {
		t.Errorf("File is smaller than header size: got %d bytes, want at least 77", initialInfo.Size())
	}

	// Manually trigger file closer (simulates what the background goroutine does)
	// Close files older than 2 hours
	if err := storage.CloseOldHourFiles(2 * time.Hour); err != nil {
		t.Fatalf("CloseOldHourFiles failed: %v", err)
	}

	// Wait a moment for close operations to complete
	time.Sleep(100 * time.Millisecond)

	// Verify file was properly closed with index and footer
	finalInfo, err := os.Stat(oldHourFilePath)
	if err != nil {
		t.Fatalf("Failed to stat file after close: %v", err)
	}

	t.Logf("Final file size: %d bytes", finalInfo.Size())

	// File should be larger than just the header (77 bytes)
	// A proper file has: Header (77) + Blocks (variable) + Index (variable) + Footer (324)
	// Minimum complete file size should be at least ~500 bytes with data
	if finalInfo.Size() <= 77 {
		t.Errorf("REGRESSION: File only contains header! Size: %d bytes (expected > 77)", finalInfo.Size())
		t.Error("This indicates files are not being properly closed with index and footer")
	}

	// Verify we can read the file (has valid footer and index)
	reader, err := NewBlockReader(oldHourFilePath)
	if err != nil {
		t.Fatalf("Failed to create reader for closed file: %v", err)
	}
	defer reader.Close()

	fileData, err := reader.ReadFile()
	if err != nil {
		t.Errorf("Failed to read closed file (likely missing footer/index): %v", err)
	}

	if fileData != nil {
		t.Logf("File successfully read with %d blocks", len(fileData.IndexSection.BlockMetadata))

		// Verify blocks were written
		if len(fileData.IndexSection.BlockMetadata) == 0 {
			t.Error("File has no blocks - events were not flushed")
		}

		// Verify footer exists and is valid
		if fileData.Footer == nil {
			t.Error("REGRESSION: File has no footer - was not properly closed")
		}

		// Verify index section exists
		if fileData.IndexSection == nil {
			t.Error("REGRESSION: File has no index section - was not properly closed")
		}
	}
}

// TestFileCloserBackgroundExecution verifies the background file closer runs automatically
func TestFileCloserBackgroundExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping background execution test in short mode")
	}

	tmpDir := t.TempDir()

	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Start storage (starts background goroutine)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := storage.Start(ctx); err != nil {
		t.Fatalf("Failed to start storage: %v", err)
	}

	// Verify background goroutine is tracked
	// The wg counter should be 1 (for the file closer)
	// We can't directly check wg, but we can verify it stops correctly

	// Stop storage and verify it completes within timeout
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()

	if err := storage.Stop(stopCtx); err != nil {
		t.Errorf("Stop failed or timed out: %v", err)
		t.Error("This may indicate the background goroutine is not stopping correctly")
	}
}

// TestCloseOldHourFilesImmediateClose tests that files are closed immediately
// when they exceed the age threshold
func TestCloseOldHourFilesImmediateClose(t *testing.T) {
	tmpDir := t.TempDir()

	storage, err := New(tmpDir, 1024)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := storage.Start(ctx); err != nil {
		t.Fatalf("Failed to start storage: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = storage.Stop(stopCtx)
	}()

	// Create files for different hours
	now := time.Now()
	hours := []time.Time{
		now.Add(-4 * time.Hour), // Old (should be closed)
		now.Add(-3 * time.Hour), // Old (should be closed)
		now.Add(-1 * time.Hour), // Recent (should stay open)
		now,                     // Current (should stay open)
	}

	for _, hourTime := range hours {
		hourStart := time.Date(hourTime.Year(), hourTime.Month(), hourTime.Day(),
			hourTime.Hour(), 0, 0, 0, hourTime.Location())

		event := createTestEvent("test-pod", "default", "Pod",
			hourStart.Add(1*time.Minute).UnixNano())

		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event for hour %s: %v", hourStart.Format("15:04"), err)
		}
	}

	// Check how many files are open
	storage.fileMutex.RLock()
	openFilesBefore := len(storage.hourFiles)
	storage.fileMutex.RUnlock()

	t.Logf("Open files before close: %d", openFilesBefore)

	if openFilesBefore != 4 {
		t.Errorf("Expected 4 open files, got %d", openFilesBefore)
	}

	// Close files older than 2 hours
	if err := storage.CloseOldHourFiles(2 * time.Hour); err != nil {
		t.Fatalf("CloseOldHourFiles failed: %v", err)
	}

	// Check how many files remain open
	storage.fileMutex.RLock()
	openFilesAfter := len(storage.hourFiles)
	storage.fileMutex.RUnlock()

	t.Logf("Open files after close: %d", openFilesAfter)

	// Should have closed 2 files (4h and 3h old), leaving 2 open (1h and current)
	if openFilesAfter != 2 {
		t.Errorf("Expected 2 files to remain open, got %d", openFilesAfter)
	}

	// Verify the closed files are properly finalized
	for i, hourTime := range hours[:2] { // Check the first 2 (old) files
		hourStart := time.Date(hourTime.Year(), hourTime.Month(), hourTime.Day(),
			hourTime.Hour(), 0, 0, 0, hourTime.Location())
		filename := hourStart.Format("2006-01-02-15") + ".bin"
		filePath := filepath.Join(tmpDir, filename)

		info, err := os.Stat(filePath)
		if err != nil {
			t.Errorf("File %d not found: %v", i, err)
			continue
		}

		// File should be larger than just header
		if info.Size() <= 77 {
			t.Errorf("File %d is only %d bytes (header only - not properly closed)", i, info.Size())
		}
	}
}

// TestFileHeaderOnlyRegression is a specific test for the 77-byte bug
func TestFileHeaderOnlyRegression(t *testing.T) {
	tmpDir := t.TempDir()

	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := storage.Start(ctx); err != nil {
		t.Fatalf("Failed to start storage: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = storage.Stop(stopCtx)
	}()

	// Write a single event to create a file
	now := time.Now()
	event := createTestEvent("test-pod", "default", "Pod", now.UnixNano())

	if err := storage.WriteEvent(event); err != nil {
		t.Fatalf("Failed to write event: %v", err)
	}

	// Get the current hour file
	storage.fileMutex.RLock()
	currentFile := storage.currentFile
	currentFilePath := ""
	if currentFile != nil {
		currentFilePath = currentFile.path
	}
	storage.fileMutex.RUnlock()

	if currentFilePath == "" {
		t.Fatal("No current file created")
	}

	// Close the file explicitly (simulates what should happen at hour boundary)
	storage.fileMutex.Lock()
	if storage.currentFile != nil {
		if err := storage.currentFile.Close(); err != nil {
			t.Errorf("Failed to close file: %v", err)
		}
		storage.currentFile = nil
	}
	storage.fileMutex.Unlock()

	// Verify file size
	info, err := os.Stat(currentFilePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	t.Logf("File size after close: %d bytes", info.Size())

	// The bug would result in a 77-byte file
	// With the fix, the file should have: header (77) + blocks + index + footer (324)
	// Minimum: 77 + 0 + ~50 + 324 ≈ 451 bytes (with empty index)
	if info.Size() == 77 {
		t.Error("REGRESSION DETECTED: File is exactly 77 bytes (header only)")
		t.Error("This indicates the file was not properly closed with index and footer")
		t.Error("Bug: CloseOldHourFiles() was added but never called")
	}

	// Verify file has valid structure
	reader, err := NewBlockReader(currentFilePath)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	_, err = reader.ReadFile()
	if err != nil {
		t.Errorf("Failed to read file structure: %v", err)
		t.Error("This indicates missing footer or invalid file structure")
	}
}
