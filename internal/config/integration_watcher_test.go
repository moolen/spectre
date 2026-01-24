package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// createTempConfigFile creates a temporary YAML config file with the given content
func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "integrations.yaml")

	if err := os.WriteFile(tmpFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to create temp config file: %v", err)
	}

	return tmpFile
}

// validConfig returns a valid integrations config for testing
func validConfig() string {
	return `schema_version: v1
instances:
  - name: test-instance
    type: victorialogs
    enabled: true
    config:
      url: "http://localhost:9428"
`
}

// invalidConfig returns an invalid config (bad schema version)
func invalidConfig() string {
	return `schema_version: v999
instances:
  - name: test-instance
    type: victorialogs
    enabled: true
    config:
      url: "http://localhost:9428"
`
}

// TestWatcherStartLoadsInitialConfig verifies that Start() loads the config
// and calls the callback immediately with the initial config.
func TestWatcherStartLoadsInitialConfig(t *testing.T) {
	tmpFile := createTempConfigFile(t, validConfig())

	var callbackCalled atomic.Bool
	var receivedConfig *IntegrationsFile

	callback := func(config *IntegrationsFile) error {
		receivedConfig = config
		callbackCalled.Store(true)
		return nil
	}

	watcher, err := NewIntegrationWatcher(IntegrationWatcherConfig{
		FilePath:       tmpFile,
		DebounceMillis: 100,
	}, callback)
	if err != nil {
		t.Fatalf("NewIntegrationWatcher failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer watcher.Stop()

	// Callback should have been called with initial config
	if !callbackCalled.Load() {
		t.Fatal("callback was not called on Start")
	}

	if receivedConfig == nil {
		t.Fatal("received config is nil")
	}

	if receivedConfig.SchemaVersion != "v1" {
		t.Errorf("expected schema_version v1, got %s", receivedConfig.SchemaVersion)
	}

	if len(receivedConfig.Instances) != 1 {
		t.Errorf("expected 1 instance, got %d", len(receivedConfig.Instances))
	}
}

// TestWatcherDetectsFileChange verifies that the watcher detects when the
// config file is modified and calls the callback.
func TestWatcherDetectsFileChange(t *testing.T) {
	tmpFile := createTempConfigFile(t, validConfig())

	var callCount atomic.Int32
	var mu sync.Mutex
	var lastConfig *IntegrationsFile

	callback := func(config *IntegrationsFile) error {
		mu.Lock()
		lastConfig = config
		mu.Unlock()
		callCount.Add(1)
		return nil
	}

	watcher, err := NewIntegrationWatcher(IntegrationWatcherConfig{
		FilePath:       tmpFile,
		DebounceMillis: 100,
	}, callback)
	if err != nil {
		t.Fatalf("NewIntegrationWatcher failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer watcher.Stop()

	// Initial callback should have been called
	if callCount.Load() != 1 {
		t.Fatalf("expected 1 initial callback, got %d", callCount.Load())
	}

	// Give watcher time to fully initialize
	time.Sleep(50 * time.Millisecond)

	// Modify the file
	newConfig := `schema_version: v1
instances:
  - name: modified-instance
    type: victorialogs
    enabled: true
    config:
      url: "http://modified:9428"
`
	if err := os.WriteFile(tmpFile, []byte(newConfig), 0600); err != nil {
		t.Fatalf("failed to modify config file: %v", err)
	}

	// Wait for debounce + processing time
	time.Sleep(300 * time.Millisecond)

	// Callback should have been called again
	if callCount.Load() != 2 {
		t.Errorf("expected 2 callbacks after file change, got %d", callCount.Load())
	}

	// Verify the new config was received
	mu.Lock()
	if lastConfig == nil || len(lastConfig.Instances) == 0 {
		t.Fatal("no instances in modified config")
	}
	if lastConfig.Instances[0].Name != "modified-instance" {
		t.Errorf("expected instance name 'modified-instance', got %s", lastConfig.Instances[0].Name)
	}
	mu.Unlock()
}

// TestWatcherDebouncing verifies that multiple rapid file modifications
// within the debounce period result in only one callback.
func TestWatcherDebouncing(t *testing.T) {
	tmpFile := createTempConfigFile(t, validConfig())

	var callCount atomic.Int32

	callback := func(config *IntegrationsFile) error {
		callCount.Add(1)
		return nil
	}

	watcher, err := NewIntegrationWatcher(IntegrationWatcherConfig{
		FilePath:       tmpFile,
		DebounceMillis: 200,
	}, callback)
	if err != nil {
		t.Fatalf("NewIntegrationWatcher failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer watcher.Stop()

	// Initial callback
	initialCount := callCount.Load()
	if initialCount != 1 {
		t.Fatalf("expected 1 initial callback, got %d", initialCount)
	}

	// Write to file 5 times rapidly (within 100ms)
	for i := 0; i < 5; i++ {
		content := validConfig() // Use same config (debouncing should work regardless)
		if err := os.WriteFile(tmpFile, []byte(content), 0600); err != nil {
			t.Fatalf("failed to write config file: %v", err)
		}
		time.Sleep(20 * time.Millisecond) // Small delay between writes
	}

	// Wait for debounce period + processing
	time.Sleep(400 * time.Millisecond)

	// Should have been called only once more (not 5 times)
	finalCount := callCount.Load()
	if finalCount != 2 {
		t.Errorf("expected 2 callbacks after debouncing (initial + 1 debounced), got %d", finalCount)
	}
}

// TestWatcherInvalidConfigRejected verifies that when the config file
// is modified to contain invalid data, the callback is NOT called
// and the watcher continues operating.
func TestWatcherInvalidConfigRejected(t *testing.T) {
	tmpFile := createTempConfigFile(t, validConfig())

	var callCount atomic.Int32
	var mu sync.Mutex
	var lastValidConfig *IntegrationsFile

	callback := func(config *IntegrationsFile) error {
		mu.Lock()
		lastValidConfig = config
		mu.Unlock()
		callCount.Add(1)
		return nil
	}

	watcher, err := NewIntegrationWatcher(IntegrationWatcherConfig{
		FilePath:       tmpFile,
		DebounceMillis: 100,
	}, callback)
	if err != nil {
		t.Fatalf("NewIntegrationWatcher failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer watcher.Stop()

	// Initial callback
	if callCount.Load() != 1 {
		t.Fatalf("expected 1 initial callback, got %d", callCount.Load())
	}

	// Verify initial config was valid
	mu.Lock()
	if lastValidConfig == nil || lastValidConfig.Instances[0].Name != "test-instance" {
		t.Fatal("initial config not correct")
	}
	mu.Unlock()

	// Write invalid config
	if err := os.WriteFile(tmpFile, []byte(invalidConfig()), 0600); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	// Wait for debounce + processing
	time.Sleep(300 * time.Millisecond)

	// Callback should NOT have been called again (invalid config rejected)
	if callCount.Load() != 1 {
		t.Errorf("expected callback NOT to be called for invalid config, got %d calls", callCount.Load())
	}

	// Write valid config again
	newValidConfig := `schema_version: v1
instances:
  - name: recovered-instance
    type: victorialogs
    enabled: true
    config:
      url: "http://recovered:9428"
`
	if err := os.WriteFile(tmpFile, []byte(newValidConfig), 0600); err != nil {
		t.Fatalf("failed to write valid config: %v", err)
	}

	// Wait for debounce + processing
	time.Sleep(300 * time.Millisecond)

	// Callback should have been called now
	if callCount.Load() != 2 {
		t.Errorf("expected 2 callbacks after recovery, got %d", callCount.Load())
	}

	// Verify the recovered config was received
	mu.Lock()
	if lastValidConfig == nil || lastValidConfig.Instances[0].Name != "recovered-instance" {
		t.Errorf("expected recovered config, got %v", lastValidConfig)
	}
	mu.Unlock()
}

// TestWatcherCallbackError verifies that when the callback returns an error,
// the watcher logs it but continues watching.
func TestWatcherCallbackError(t *testing.T) {
	tmpFile := createTempConfigFile(t, validConfig())

	var callCount atomic.Int32
	firstCall := true
	var mu sync.Mutex

	callback := func(config *IntegrationsFile) error {
		mu.Lock()
		defer mu.Unlock()
		callCount.Add(1)

		// Return error on first call (initial load)
		// This should cause Start() to fail
		if firstCall {
			firstCall = false
			return nil // Don't error on initial call so Start succeeds
		}

		// Return error on subsequent calls
		return os.ErrNotExist // Arbitrary error
	}

	watcher, err := NewIntegrationWatcher(IntegrationWatcherConfig{
		FilePath:       tmpFile,
		DebounceMillis: 100,
	}, callback)
	if err != nil {
		t.Fatalf("NewIntegrationWatcher failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer watcher.Stop()

	// Initial callback should succeed
	if callCount.Load() != 1 {
		t.Fatalf("expected 1 initial callback, got %d", callCount.Load())
	}

	// Give watcher time to fully initialize
	time.Sleep(50 * time.Millisecond)

	// Modify the file
	newConfig := `schema_version: v1
instances:
  - name: error-test-instance
    type: victorialogs
    enabled: true
    config:
      url: "http://error:9428"
`
	if err := os.WriteFile(tmpFile, []byte(newConfig), 0600); err != nil {
		t.Fatalf("failed to modify config file: %v", err)
	}

	// Wait for debounce + processing
	time.Sleep(300 * time.Millisecond)

	// Callback should have been called (even though it returned error)
	if callCount.Load() != 2 {
		t.Errorf("expected callback to be called despite error, got %d calls", callCount.Load())
	}

	// Watcher should still be running (can modify file again)
	if err := os.WriteFile(tmpFile, []byte(validConfig()), 0600); err != nil {
		t.Fatalf("failed to modify config file again: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// Should have been called at least 3 times (initial + 2 modifications)
	finalCount := callCount.Load()
	if finalCount < 3 {
		t.Errorf("expected watcher to continue after callback error, got only %d calls", finalCount)
	}
}

// TestWatcherStopGraceful verifies that Stop() exits cleanly within the timeout.
func TestWatcherStopGraceful(t *testing.T) {
	tmpFile := createTempConfigFile(t, validConfig())

	callback := func(config *IntegrationsFile) error {
		return nil
	}

	watcher, err := NewIntegrationWatcher(IntegrationWatcherConfig{
		FilePath:       tmpFile,
		DebounceMillis: 100,
	}, callback)
	if err != nil {
		t.Fatalf("NewIntegrationWatcher failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop should complete within timeout
	stopStart := time.Now()
	if err := watcher.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
	stopDuration := time.Since(stopStart)

	// Should complete well before the 5 second timeout
	if stopDuration > 4*time.Second {
		t.Errorf("Stop took too long: %v", stopDuration)
	}
}

// TestNewIntegrationWatcherValidation verifies that the constructor
// validates its inputs properly.
func TestNewIntegrationWatcherValidation(t *testing.T) {
	callback := func(config *IntegrationsFile) error {
		return nil
	}

	// Empty FilePath should error
	_, err := NewIntegrationWatcher(IntegrationWatcherConfig{
		FilePath: "",
	}, callback)
	if err == nil {
		t.Error("expected error for empty FilePath")
	}

	// Nil callback should error
	_, err = NewIntegrationWatcher(IntegrationWatcherConfig{
		FilePath: "/tmp/test.yaml",
	}, nil)
	if err == nil {
		t.Error("expected error for nil callback")
	}

	// Valid config should succeed
	tmpFile := createTempConfigFile(t, validConfig())
	_, err = NewIntegrationWatcher(IntegrationWatcherConfig{
		FilePath: tmpFile,
	}, callback)
	if err != nil {
		t.Errorf("expected success for valid config: %v", err)
	}
}

// TestWatcherDefaultDebounce verifies that DebounceMillis defaults to 500ms
func TestWatcherDefaultDebounce(t *testing.T) {
	tmpFile := createTempConfigFile(t, validConfig())

	callback := func(config *IntegrationsFile) error {
		return nil
	}

	// Create watcher with zero debounce (should default to 500)
	watcher, err := NewIntegrationWatcher(IntegrationWatcherConfig{
		FilePath:       tmpFile,
		DebounceMillis: 0, // Should default to 500
	}, callback)
	if err != nil {
		t.Fatalf("NewIntegrationWatcher failed: %v", err)
	}

	// Check that default was applied
	if watcher.config.DebounceMillis != 500 {
		t.Errorf("expected default debounce 500ms, got %d", watcher.config.DebounceMillis)
	}
}

// TestWatcherDetectsAtomicWrite verifies that the watcher correctly detects
// file changes when using atomic writes (temp file + rename pattern).
// This is critical because atomic writes can cause the inode to change,
// and the watcher must re-add the watch after a Remove/Rename event.
func TestWatcherDetectsAtomicWrite(t *testing.T) {
	tmpFile := createTempConfigFile(t, validConfig())

	var mu sync.Mutex
	var lastConfig *IntegrationsFile
	var callCount atomic.Int32

	callback := func(config *IntegrationsFile) error {
		callCount.Add(1)
		mu.Lock()
		lastConfig = config
		mu.Unlock()
		return nil
	}

	watcher, err := NewIntegrationWatcher(IntegrationWatcherConfig{
		FilePath:       tmpFile,
		DebounceMillis: 100,
	}, callback)
	if err != nil {
		t.Fatalf("NewIntegrationWatcher failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer watcher.Stop()

	// Initial callback should have been called
	if callCount.Load() != 1 {
		t.Fatalf("expected 1 initial callback, got %d", callCount.Load())
	}

	// Give watcher time to fully initialize
	time.Sleep(100 * time.Millisecond)

	// Use WriteIntegrationsFile which does atomic writes (temp file + rename)
	newConfig := &IntegrationsFile{
		SchemaVersion: "v1",
		Instances: []IntegrationConfig{
			{
				Name:    "atomic-write-instance",
				Type:    "victorialogs",
				Enabled: true,
				Config: map[string]interface{}{
					"url": "http://atomic-test:9428",
				},
			},
		},
	}

	if err := WriteIntegrationsFile(tmpFile, newConfig); err != nil {
		t.Fatalf("WriteIntegrationsFile failed: %v", err)
	}

	// Wait for debounce + processing time (longer for atomic writes)
	time.Sleep(500 * time.Millisecond)

	// Callback should have been called again
	if callCount.Load() != 2 {
		t.Errorf("expected 2 callbacks after atomic write, got %d", callCount.Load())
	}

	// Verify the new config was received
	mu.Lock()
	defer mu.Unlock()
	if lastConfig == nil || len(lastConfig.Instances) == 0 {
		t.Fatal("no instances in config after atomic write")
	}
	if lastConfig.Instances[0].Name != "atomic-write-instance" {
		t.Errorf("expected instance name 'atomic-write-instance', got %s", lastConfig.Instances[0].Name)
	}
}
