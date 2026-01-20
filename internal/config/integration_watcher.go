package config

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ReloadCallback is called when the integration config file is successfully reloaded.
// If the callback returns an error, it is logged but the watcher continues watching.
type ReloadCallback func(config *IntegrationsFile) error

// IntegrationWatcherConfig holds configuration for the IntegrationWatcher.
type IntegrationWatcherConfig struct {
	// FilePath is the path to the integrations YAML file to watch
	FilePath string

	// DebounceMillis is the debounce period in milliseconds
	// Multiple file change events within this period will be coalesced into a single reload
	// Default: 500ms
	DebounceMillis int
}

// IntegrationWatcher watches an integrations config file for changes and triggers
// reload callbacks with debouncing to prevent reload storms from editor save sequences.
//
// Invalid configs during reload are logged but do not crash the watcher - it continues
// watching with the previous valid config.
type IntegrationWatcher struct {
	config   IntegrationWatcherConfig
	callback ReloadCallback
	cancel   context.CancelFunc
	stopped  chan struct{}
	mu       sync.Mutex

	// debounceTimer is used to coalesce multiple file change events
	debounceTimer *time.Timer
}

// NewIntegrationWatcher creates a new watcher for the given config file.
// The callback will be invoked when the file changes and the new config is valid.
//
// Returns an error if FilePath is empty.
func NewIntegrationWatcher(config IntegrationWatcherConfig, callback ReloadCallback) (*IntegrationWatcher, error) {
	if config.FilePath == "" {
		return nil, fmt.Errorf("FilePath cannot be empty")
	}

	if callback == nil {
		return nil, fmt.Errorf("callback cannot be nil")
	}

	// Set default debounce if not specified
	if config.DebounceMillis == 0 {
		config.DebounceMillis = 500
	}

	return &IntegrationWatcher{
		config:   config,
		callback: callback,
		stopped:  make(chan struct{}),
	}, nil
}

// Start begins watching the config file for changes.
// It loads the initial config, calls the callback, and then watches for file changes.
//
// This method blocks until Stop() is called or the context is cancelled.
// Returns an error if initial config load fails or callback returns error.
func (w *IntegrationWatcher) Start(ctx context.Context) error {
	// Load initial config
	initialConfig, err := LoadIntegrationsFile(w.config.FilePath)
	if err != nil {
		return fmt.Errorf("failed to load initial config: %w", err)
	}

	// Call callback with initial config (fail fast if callback errors)
	if err := w.callback(initialConfig); err != nil {
		return fmt.Errorf("initial callback failed: %w", err)
	}

	log.Printf("IntegrationWatcher: loaded initial config from %s", w.config.FilePath)

	// Create watcher context
	watchCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	// Start watching in a goroutine
	go w.watchLoop(watchCtx)

	return nil
}

// watchLoop is the main file watching loop
func (w *IntegrationWatcher) watchLoop(ctx context.Context) {
	defer close(w.stopped)

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("IntegrationWatcher: failed to create file watcher: %v", err)
		return
	}
	defer watcher.Close()

	// Add file to watcher
	if err := watcher.Add(w.config.FilePath); err != nil {
		log.Printf("IntegrationWatcher: failed to watch file %s: %v", w.config.FilePath, err)
		return
	}

	log.Printf("IntegrationWatcher: watching %s for changes (debounce: %dms)",
		w.config.FilePath, w.config.DebounceMillis)

	for {
		select {
		case <-ctx.Done():
			log.Printf("IntegrationWatcher: context cancelled, stopping")
			return

		case event, ok := <-watcher.Events:
			if !ok {
				log.Printf("IntegrationWatcher: watcher events channel closed")
				return
			}

			// Check if this is a relevant event (Write or Create)
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				w.handleFileChange(ctx)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				log.Printf("IntegrationWatcher: watcher errors channel closed")
				return
			}
			log.Printf("IntegrationWatcher: watcher error: %v", err)
		}
	}
}

// handleFileChange is called when a file change event is detected.
// It implements debouncing by resetting a timer on each event.
func (w *IntegrationWatcher) handleFileChange(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Reset the debounce timer if it exists
	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
	}

	// Create new timer that will trigger reload after debounce period
	w.debounceTimer = time.AfterFunc(
		time.Duration(w.config.DebounceMillis)*time.Millisecond,
		func() {
			w.reloadConfig(ctx)
		},
	)
}

// reloadConfig reloads the config file and calls the callback if successful.
// Invalid configs are logged but don't crash the watcher.
func (w *IntegrationWatcher) reloadConfig(ctx context.Context) {
	log.Printf("IntegrationWatcher: reloading config from %s", w.config.FilePath)

	// Load new config
	newConfig, err := LoadIntegrationsFile(w.config.FilePath)
	if err != nil {
		// Log error but continue watching with previous config
		log.Printf("IntegrationWatcher: failed to load config (keeping previous config): %v", err)
		return
	}

	// Call callback with new config
	if err := w.callback(newConfig); err != nil {
		// Log error but continue watching
		log.Printf("IntegrationWatcher: callback error (continuing to watch): %v", err)
		return
	}

	log.Printf("IntegrationWatcher: config reloaded successfully")
}

// Stop gracefully stops the file watcher.
// Waits for the watch loop to exit with a timeout of 5 seconds.
// Returns an error if the timeout is exceeded.
func (w *IntegrationWatcher) Stop() error {
	if w.cancel != nil {
		w.cancel()
	}

	// Wait for stopped signal with timeout
	timeout := time.After(5 * time.Second)
	select {
	case <-w.stopped:
		log.Printf("IntegrationWatcher: stopped gracefully")
		return nil
	case <-timeout:
		return fmt.Errorf("timeout waiting for watcher to stop")
	}
}
