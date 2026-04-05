package embeddedstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const storageFormatVersion = 1

const manifestFileName = "manifest.json"

type Manifest struct {
	FormatVersion      int              `json:"format_version"`
	ActiveSegments     []SegmentMeta    `json:"active_segments"`
	ActiveCheckpoint   CheckpointMeta   `json:"active_checkpoint"`
	ActiveTail         TailJournalMeta  `json:"active_tail"`
	Checkpoints        []CheckpointMeta `json:"checkpoints"`
	FlushHighWaterMark uint64           `json:"flush_high_water_mark"`
}

type SegmentMeta struct {
	ID            string `json:"id"`
	HighWaterMark uint64 `json:"high_water_mark"`
}

type CheckpointMeta struct {
	ID            string `json:"id"`
	HighWaterMark uint64 `json:"high_water_mark"`
}

type TailJournalMeta struct {
	ID                string `json:"id"`
	BaseHighWaterMark uint64 `json:"base_high_water_mark"`
	LastHighWaterMark uint64 `json:"last_high_water_mark"`
	EventCount        int    `json:"event_count"`
	SizeBytes         int64  `json:"size_bytes"`
}

func loadOrCreateManifest(dir string) (Manifest, error) {
	if err := ensureManifestDir(dir); err != nil {
		return Manifest{}, err
	}

	manifestPath := filepath.Join(dir, manifestFileName)
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return Manifest{}, fmt.Errorf("load manifest: read %s: %w", manifestPath, err)
		}

		manifest := Manifest{
			FormatVersion:  storageFormatVersion,
			ActiveSegments: []SegmentMeta{},
			Checkpoints:    []CheckpointMeta{},
		}
		if err := storeManifest(dir, manifest); err != nil {
			return Manifest{}, err
		}
		return manifest, nil
	}

	var manifest Manifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("load manifest: decode %s: %w", manifestPath, err)
	}

	manifest = normalizeManifest(manifest)
	if err := validateManifestVersion(manifest, "load manifest"); err != nil {
		return Manifest{}, err
	}
	manifest, err = reconcileManifestCheckpoints(dir, manifest)
	if err != nil {
		return Manifest{}, err
	}

	return manifest, nil
}

func storeManifest(dir string, manifest Manifest) error {
	if err := ensureManifestDir(dir); err != nil {
		return err
	}

	manifest = normalizeManifest(manifest)
	if manifest.FormatVersion == 0 {
		manifest.FormatVersion = storageFormatVersion
	}
	if err := validateManifestVersion(manifest, "store manifest"); err != nil {
		return err
	}

	payload, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("store manifest: encode payload: %w", err)
	}

	manifestPath := filepath.Join(dir, manifestFileName)
	tmpFile, err := os.CreateTemp(dir, manifestFileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("store manifest: create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	written := 0
	for written < len(payload) {
		n, err := tmpFile.Write(payload[written:])
		if err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("store manifest: write temp file: %w", err)
		}
		if n == 0 {
			_ = tmpFile.Close()
			return fmt.Errorf("store manifest: write temp file: %w", io.ErrShortWrite)
		}
		written += n
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("store manifest: sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("store manifest: close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, manifestPath); err != nil {
		return fmt.Errorf("store manifest: replace manifest: %w", err)
	}
	if err := syncPath(dir); err != nil {
		return fmt.Errorf("store manifest: sync manifest dir: %w", err)
	}

	return nil
}

func ensureManifestDir(dir string) error {
	if dir == "" {
		return fmt.Errorf("manifest dir is empty")
	}

	created, err := pathCreated(dir)
	if err != nil {
		return fmt.Errorf("ensure manifest dir: stat dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure manifest dir: create dir: %w", err)
	}
	if created {
		if err := syncPath(filepath.Dir(dir)); err != nil {
			return fmt.Errorf("ensure manifest dir: sync parent dir: %w", err)
		}
	}

	return nil
}

func normalizeManifest(manifest Manifest) Manifest {
	if manifest.ActiveSegments == nil {
		manifest.ActiveSegments = []SegmentMeta{}
	}
	if manifest.Checkpoints == nil {
		manifest.Checkpoints = []CheckpointMeta{}
	}
	if manifest.ActiveCheckpoint.ID == "" && len(manifest.Checkpoints) > 0 {
		manifest.ActiveCheckpoint = latestCheckpointMeta(manifest.Checkpoints)
	}
	if manifest.ActiveTail == (TailJournalMeta{}) {
		manifest.ActiveTail.BaseHighWaterMark = manifest.ActiveCheckpoint.HighWaterMark
		manifest.ActiveTail.LastHighWaterMark = manifest.ActiveCheckpoint.HighWaterMark
	}
	return manifest
}

func validateManifestVersion(manifest Manifest, operation string) error {
	if manifest.FormatVersion != storageFormatVersion {
		return fmt.Errorf(
			"%s: unsupported manifest format version %d (expected %d)",
			operation,
			manifest.FormatVersion,
			storageFormatVersion,
		)
	}
	return nil
}

func reconcileManifestCheckpoints(dir string, manifest Manifest) (Manifest, error) {
	checkpointsRoot := filepath.Join(dir, checkpointsDirName)
	entries, err := os.ReadDir(checkpointsRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return manifest, nil
		}
		return Manifest{}, fmt.Errorf("load manifest: list checkpoints: %w", err)
	}

	known := make(map[string]struct{}, len(manifest.Checkpoints))
	for i := range manifest.Checkpoints {
		known[manifest.Checkpoints[i].ID] = struct{}{}
	}

	changed := false
	for i := range entries {
		if !entries[i].IsDir() {
			continue
		}
		checkpointID := entries[i].Name()
		if _, exists := known[checkpointID]; exists {
			continue
		}

		highWaterMark, ok := parseCheckpointHighWaterMark(checkpointID)
		if !ok {
			continue
		}

		manifest.Checkpoints = append(manifest.Checkpoints, CheckpointMeta{
			ID:            checkpointID,
			HighWaterMark: highWaterMark,
		})
		known[checkpointID] = struct{}{}
		changed = true
	}

	if !changed {
		return manifest, nil
	}

	slices.SortFunc(manifest.Checkpoints, func(a, b CheckpointMeta) int {
		switch {
		case a.HighWaterMark < b.HighWaterMark:
			return -1
		case a.HighWaterMark > b.HighWaterMark:
			return 1
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
	if err := storeManifest(dir, manifest); err != nil {
		return Manifest{}, err
	}

	return manifest, nil
}

func parseCheckpointHighWaterMark(checkpointID string) (uint64, bool) {
	if !strings.HasPrefix(checkpointID, "chk-") {
		return 0, false
	}

	rest := strings.TrimPrefix(checkpointID, "chk-")
	parts := strings.SplitN(rest, "-", 2)
	if len(parts) != 2 || parts[0] == "" {
		return 0, false
	}

	highWaterMark, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, false
	}

	return highWaterMark, true
}
