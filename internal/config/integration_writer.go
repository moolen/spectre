package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// WriteIntegrationsFile atomically writes an IntegrationsFile to disk using
// a temp-file-then-rename pattern to prevent corruption on crashes.
//
// The atomic write process:
// 1. Marshal IntegrationsFile to YAML
// 2. Create temp file in same directory as target
// 3. Write YAML to temp file
// 4. Close temp file to flush to disk
// 5. Atomically rename temp file to target path (POSIX guarantees atomicity)
//
// If any step fails, the temp file is cleaned up and the original file
// remains untouched. This ensures readers never see partial writes.
//
// Returns error if marshaling fails, file operations fail, or rename fails.
func WriteIntegrationsFile(path string, config *IntegrationsFile) error {
	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal integrations config: %w", err)
	}

	// Get directory of target file for temp file creation
	dir := filepath.Dir(path)

	// Create temp file in same directory as target
	// Pattern: .integrations.*.yaml.tmp
	tmpFile, err := os.CreateTemp(dir, ".integrations.*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure cleanup on error
	defer func() {
		// Remove temp file if it still exists (indicates error path)
		if _, err := os.Stat(tmpPath); err == nil {
			os.Remove(tmpPath)
		}
	}()

	// Write YAML data to temp file
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Close temp file to flush to disk
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename from temp to target (POSIX guarantees atomicity)
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file to %q: %w", path, err)
	}

	return nil
}
