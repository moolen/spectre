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
	checkpointStateFile     = "meta.json"
	checkpointResourcesFile = "resources.ndjson"
	checkpointK8sEventsFile = "k8s-events.json"
	checkpointFormatVersion = 1
)

type checkpointState struct {
	FormatVersion  int                `json:"format_version"`
	HighWaterMark  uint64             `json:"high_water_mark"`
	MinTimestampNs int64              `json:"min_timestamp_ns"`
	MaxTimestampNs int64              `json:"max_timestamp_ns"`
	Snapshot       ProjectionSnapshot `json:"snapshot,omitempty"`
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

	state := projection.CheckpointState(highWaterMark)
	if err := writeJSONFile(filepath.Join(tmpCheckpointDir, checkpointStateFile), state); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: state file: %w", err)
	}
	if err := writeCheckpointResources(filepath.Join(tmpCheckpointDir, checkpointResourcesFile), projection); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: resources stream: %w", err)
	}
	if err := writeJSONFile(filepath.Join(tmpCheckpointDir, checkpointK8sEventsFile), projection.CheckpointK8sEvents()); err != nil {
		return CheckpointMeta{}, fmt.Errorf("write checkpoint: k8s events file: %w", err)
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

	checkpointDir := filepath.Join(rootDir, checkpointsDirName, meta.ID)
	statePath := filepath.Join(checkpointDir, checkpointStateFile)
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

	if len(state.Snapshot.Resources) > 0 || len(state.Snapshot.K8sEventsByInvolvedUID) > 0 || len(state.Snapshot.Events) > 0 {
		projection, err := ProjectionFromSnapshot(state.Snapshot)
		if err != nil {
			return nil, 0, fmt.Errorf("load checkpoint: restore projection: %w", err)
		}
		return projection, state.HighWaterMark, nil
	}

	resourcesFile, err := os.Open(filepath.Join(checkpointDir, checkpointResourcesFile))
	if err != nil {
		return nil, 0, fmt.Errorf("load checkpoint: open resources stream: %w", err)
	}
	defer func() {
		_ = resourcesFile.Close()
	}()

	k8sEventsFile, err := os.Open(filepath.Join(checkpointDir, checkpointK8sEventsFile))
	if err != nil {
		return nil, 0, fmt.Errorf("load checkpoint: open k8s events file: %w", err)
	}
	defer func() {
		_ = k8sEventsFile.Close()
	}()

	projection, err := ProjectionFromCheckpointStream(state, resourcesFile, k8sEventsFile)
	if err != nil {
		return nil, 0, fmt.Errorf("load checkpoint: restore projection: %w", err)
	}
	return projection, state.HighWaterMark, nil
}

func newCheckpointID(highWaterMark uint64) string {
	return fmt.Sprintf("chk-%020d-%d", highWaterMark, time.Now().UTC().UnixNano())
}

func writeCheckpointResources(path string, projection *Projection) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	encoder := json.NewEncoder(file)
	if err := projection.StreamCheckpointResources(func(snapshot ProjectionResourceSnapshot) error {
		return encoder.Encode(snapshot)
	}); err != nil {
		return fmt.Errorf("encode resources: %w", err)
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close file: %w", err)
	}
	return nil
}
