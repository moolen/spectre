package embeddedstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	checkpointsDirName      = "checkpoints"
	checkpointStateFile     = "checkpoint.json"
	checkpointFormatVersion = 1
)

type checkpointState struct {
	FormatVersion int                `json:"format_version"`
	HighWaterMark uint64             `json:"high_water_mark"`
	Snapshot      ProjectionSnapshot `json:"snapshot"`
}

func writeCheckpoint(rootDir string, projection *Projection, highWaterMark uint64) (CheckpointMeta, error) {
	if rootDir == "" {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: root dir is empty")
	}
	if projection == nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: projection is nil")
	}

	checkpointsRoot := filepath.Join(rootDir, checkpointsDirName)
	if err := ensureDirWithParentSync(checkpointsRoot); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: ensure checkpoints dir: %w", err)
	}

	tmpRoot := filepath.Join(rootDir, segmentTempDirName)
	if err := ensureDirWithParentSync(tmpRoot); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: ensure temp dir: %w", err)
	}

	checkpointID := newCheckpointID(highWaterMark)
	finalCheckpointDir := filepath.Join(checkpointsRoot, checkpointID)
	if _, err := os.Stat(finalCheckpointDir); err == nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: checkpoint %q already exists", checkpointID)
	} else if !os.IsNotExist(err) {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: stat existing checkpoint path: %w", err)
	}

	tmpCheckpointDir, err := os.MkdirTemp(tmpRoot, checkpointID+".tmp-*")
	if err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: create temp checkpoint dir: %w", err)
	}

	cleanupTmpDir := true
	defer func() {
		if cleanupTmpDir {
			_ = os.RemoveAll(tmpCheckpointDir)
		}
	}()

	state := checkpointState{
		FormatVersion: checkpointFormatVersion,
		HighWaterMark: highWaterMark,
		Snapshot:      projection.ExportSnapshot(),
	}
	if err := writeJSONFile(filepath.Join(tmpCheckpointDir, checkpointStateFile), state); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: state file: %w", err)
	}
	if err := syncPath(tmpCheckpointDir); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: sync temp checkpoint dir: %w", err)
	}
	if err := os.Rename(tmpCheckpointDir, finalCheckpointDir); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: move checkpoint into place: %w", err)
	}
	if err := syncPath(checkpointsRoot); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: sync checkpoints dir: %w", err)
	}

	cleanupTmpDir = false
	return CheckpointMeta{
		ID:            checkpointID,
		HighWaterMark: highWaterMark,
	}, nil
}

func loadCheckpoint(rootDir string, meta CheckpointMeta) (*Projection, uint64, error) {
	if rootDir == "" {
		return nil, 0, fmt.Errorf("load checkpoint: root dir is empty")
	}
	if meta.ID == "" {
		return nil, 0, fmt.Errorf("load checkpoint: checkpoint id is empty")
	}

	statePath := filepath.Join(rootDir, checkpointsDirName, meta.ID, checkpointStateFile)
	payload, err := os.ReadFile(statePath)
	if err != nil {
		return nil, 0, fmt.Errorf("load checkpoint: read state file: %w", err)
	}

	var state checkpointState
	if err := json.Unmarshal(payload, &state); err != nil {
		return nil, 0, fmt.Errorf("load checkpoint: decode state file: %w", err)
	}
	if state.FormatVersion != checkpointFormatVersion {
		return nil, 0, fmt.Errorf(
			"load checkpoint: unsupported checkpoint format version %d (expected %d)",
			state.FormatVersion,
			checkpointFormatVersion,
		)
	}
	if meta.HighWaterMark != 0 && meta.HighWaterMark != state.HighWaterMark {
		return nil, 0, fmt.Errorf(
			"load checkpoint: high water mark mismatch for %q: manifest=%d checkpoint=%d",
			meta.ID,
			meta.HighWaterMark,
			state.HighWaterMark,
		)
	}

	projection, err := ProjectionFromSnapshot(state.Snapshot)
	if err != nil {
		return nil, 0, fmt.Errorf("load checkpoint: restore projection: %w", err)
	}
	return projection, state.HighWaterMark, nil
}

func newCheckpointID(highWaterMark uint64) string {
	return fmt.Sprintf("chk-%020d-%d", highWaterMark, time.Now().UTC().UnixNano())
}
