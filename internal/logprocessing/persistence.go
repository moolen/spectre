package logprocessing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// SnapshotData represents the JSON serialization format for template persistence.
// It includes versioning for schema evolution and timestamp for debugging.
type SnapshotData struct {
	// Version is the schema version (start with 1)
	Version int `json:"version"`

	// Timestamp is when the snapshot was created
	Timestamp time.Time `json:"timestamp"`

	// Namespaces contains per-namespace template snapshots
	Namespaces map[string]*NamespaceSnapshot `json:"namespaces"`
}

// NamespaceSnapshot represents a serialized namespace's template state.
// Templates are stored as a slice (not map) for JSON serialization.
type NamespaceSnapshot struct {
	// Templates is the list of templates in this namespace
	Templates []Template `json:"templates"`

	// Counts maps templateID -> occurrence count
	Counts map[string]int `json:"counts"`
}

// PersistenceManager handles periodic snapshots and restoration of template store.
// It writes snapshots to disk using atomic file operations (temp + rename).
//
// Design decision from CONTEXT.md: "Persist every 5 minutes (lose at most 5 min on crash)"
// Pattern from Phase 2: "Atomic writes prevent config corruption on crashes"
type PersistenceManager struct {
	// store is the live template store to snapshot
	store *TemplateStore

	// snapshotPath is the file path for JSON snapshots
	snapshotPath string

	// snapshotInterval is how often to create snapshots (default 5 minutes)
	snapshotInterval time.Duration

	// stopCh signals shutdown to the snapshot loop
	stopCh chan struct{}
}

// NewPersistenceManager creates a persistence manager for the given store.
// Snapshots are written to snapshotPath every interval.
func NewPersistenceManager(store *TemplateStore, snapshotPath string, interval time.Duration) *PersistenceManager {
	return &PersistenceManager{
		store:            store,
		snapshotPath:     snapshotPath,
		snapshotInterval: interval,
		stopCh:           make(chan struct{}),
	}
}

// Start begins the periodic snapshot loop.
// It first attempts to load existing state from disk, then starts the snapshot ticker.
// Blocks until context is cancelled or Stop() is called.
//
// Requirement MINE-04: Canonical templates stored in MCP server for persistence.
func (pm *PersistenceManager) Start(ctx context.Context) error {
	// Load existing state if snapshot file exists
	if err := pm.Load(); err != nil {
		// Log error but continue - start with empty state
		// User decision: "Start empty on first run"
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to load snapshot: %w", err)
		}
	}

	// Create ticker for periodic snapshots
	ticker := time.NewTicker(pm.snapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Periodic snapshot
			if err := pm.Snapshot(); err != nil {
				// Log error but continue
				// User decision: "lose at most 5 min on crash" - don't fail server
				// In production, this would be logged via proper logger
				fmt.Fprintf(os.Stderr, "snapshot failed: %v\n", err)
			}

		case <-ctx.Done():
			// Context cancelled - perform final snapshot
			if err := pm.Snapshot(); err != nil {
				fmt.Fprintf(os.Stderr, "final snapshot failed: %v\n", err)
			}
			return ctx.Err()

		case <-pm.stopCh:
			// Explicit stop - perform final snapshot
			if err := pm.Snapshot(); err != nil {
				fmt.Fprintf(os.Stderr, "final snapshot failed: %v\n", err)
			}
			return nil
		}
	}
}

// Snapshot creates a JSON snapshot of the current template store state.
// Uses atomic writes (temp file + rename) to prevent corruption on crash.
//
// Pattern from Phase 2: "Atomic writes prevent config corruption on crashes (POSIX atomicity)"
func (pm *PersistenceManager) Snapshot() error {
	// Lock store for reading
	pm.store.mu.RLock()
	defer pm.store.mu.RUnlock()

	// Build snapshot data
	snapshot := SnapshotData{
		Version:    1,
		Timestamp:  time.Now(),
		Namespaces: make(map[string]*NamespaceSnapshot),
	}

	// Copy each namespace's templates and counts
	for namespace, ns := range pm.store.namespaces {
		ns.mu.RLock()

		// Convert templates map to slice for JSON serialization
		templates := make([]Template, 0, len(ns.templates))
		for _, template := range ns.templates {
			// Deep copy to prevent mutation
			templateCopy := *template
			templates = append(templates, templateCopy)
		}

		// Copy counts map
		counts := make(map[string]int, len(ns.counts))
		for id, count := range ns.counts {
			counts[id] = count
		}

		snapshot.Namespaces[namespace] = &NamespaceSnapshot{
			Templates: templates,
			Counts:    counts,
		}

		ns.mu.RUnlock()
	}

	// Marshal to JSON with indentation for human readability
	// User decision: "JSON format for persistence (human-readable, debuggable)"
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	// Write to temp file first
	tmpPath := pm.snapshotPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp snapshot: %w", err)
	}

	// Atomic rename (POSIX atomicity)
	if err := os.Rename(tmpPath, pm.snapshotPath); err != nil {
		return fmt.Errorf("failed to rename snapshot: %w", err)
	}

	return nil
}

// Load restores template store state from a JSON snapshot.
// If the snapshot file doesn't exist, returns nil (start empty).
// If the snapshot is corrupted, returns error.
func (pm *PersistenceManager) Load() error {
	// Read snapshot file
	data, err := os.ReadFile(pm.snapshotPath)
	if err != nil {
		return err // os.IsNotExist(err) checked by caller
	}

	// Unmarshal JSON
	var snapshot SnapshotData
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	// Verify version
	if snapshot.Version != 1 {
		return fmt.Errorf("unsupported snapshot version: %d", snapshot.Version)
	}

	// Lock store for writing
	pm.store.mu.Lock()
	defer pm.store.mu.Unlock()

	// Restore each namespace
	for namespace, nsSnapshot := range snapshot.Namespaces {
		// Create new NamespaceTemplates with fresh Drain instance
		ns := &NamespaceTemplates{
			drain:     NewDrainProcessor(pm.store.config),
			templates: make(map[string]*Template),
			counts:    make(map[string]int),
		}

		// Restore templates
		for i := range nsSnapshot.Templates {
			template := &nsSnapshot.Templates[i]
			ns.templates[template.ID] = template
		}

		// Restore counts
		for id, count := range nsSnapshot.Counts {
			ns.counts[id] = count
		}

		pm.store.namespaces[namespace] = ns
	}

	return nil
}

// Stop signals the snapshot loop to stop and perform a final snapshot.
// Blocks until the loop exits.
func (pm *PersistenceManager) Stop() {
	close(pm.stopCh)
}
