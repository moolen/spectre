package embeddedstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestCheckpoint_RoundTripProjectionState(t *testing.T) {
	dir := t.TempDir()
	projection, err := BuildProjection([]models.Event{
		{
			ID:        "1",
			Timestamp: 10,
			Resource: models.ResourceMetadata{
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
		},
	})
	require.NoError(t, err)

	meta, err := writeCheckpoint(dir, projection, 123)
	require.NoError(t, err)

	restored, highWaterMark, err := loadCheckpoint(dir, meta)
	require.NoError(t, err)
	require.Equal(t, uint64(123), highWaterMark)

	resource, err := NewAnalysisStore(restored).GetResource(context.Background(), "pod-1")
	require.NoError(t, err)
	require.NotNil(t, resource)
	require.Equal(t, "pod-1", resource.UID)
}

func TestCheckpoint_LoadRejectsCorruptBundle(t *testing.T) {
	dir := t.TempDir()
	projection, err := BuildProjection([]models.Event{
		{
			ID:        "1",
			Timestamp: 10,
			Resource: models.ResourceMetadata{
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
		},
	})
	require.NoError(t, err)

	meta, err := writeCheckpoint(dir, projection, 123)
	require.NoError(t, err)

	checkpointPath := filepath.Join(dir, checkpointsDirName, meta.ID, "meta.json")
	require.NoError(t, os.WriteFile(checkpointPath, []byte("{not-json"), 0o600))

	_, _, err = loadCheckpoint(dir, meta)
	require.Error(t, err)
}

func TestCheckpoint_LoadSupportsLegacyStateFile(t *testing.T) {
	dir := t.TempDir()
	projection, err := BuildProjection([]models.Event{
		{
			ID:        "1",
			Timestamp: 10,
			Resource: models.ResourceMetadata{
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
		},
	})
	require.NoError(t, err)

	checkpointID := newCheckpointID(123)
	checkpointDir := filepath.Join(dir, checkpointsDirName, checkpointID)
	require.NoError(t, os.MkdirAll(checkpointDir, 0o755))

	state := checkpointState{
		FormatVersion: checkpointFormatVersion,
		HighWaterMark: 123,
		Snapshot:      projection.ExportSnapshot(),
	}
	require.NoError(t, writeJSONFile(filepath.Join(checkpointDir, checkpointStateFileV0), state))

	restored, highWaterMark, err := loadCheckpoint(dir, CheckpointMeta{
		ID:            checkpointID,
		HighWaterMark: 123,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(123), highWaterMark)

	resource, err := NewAnalysisStore(restored).GetResource(context.Background(), "pod-1")
	require.NoError(t, err)
	require.NotNil(t, resource)
	require.Equal(t, "pod-1", resource.UID)
}

func TestCheckpoint_LoadSupportsLegacyJSONStreamBundle(t *testing.T) {
	dir := t.TempDir()
	projection, err := BuildProjection([]models.Event{
		{
			ID:        "1",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
			Data: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"}}`),
		},
	})
	require.NoError(t, err)

	checkpointID := newCheckpointID(123)
	checkpointDir := filepath.Join(dir, checkpointsDirName, checkpointID)
	require.NoError(t, os.MkdirAll(checkpointDir, 0o755))
	require.NoError(t, writeJSONFile(filepath.Join(checkpointDir, checkpointStateFile), checkpointState{
		FormatVersion:  checkpointFormatVersionV1,
		HighWaterMark:  123,
		MinTimestampNs: 10,
		MaxTimestampNs: 10,
	}))
	require.NoError(t, writeCheckpointResourcesJSON(filepath.Join(checkpointDir, checkpointResourcesFileV1), projection))
	require.NoError(t, writeCheckpointK8sEventsJSON(filepath.Join(checkpointDir, checkpointK8sEventsFileV1), projection.CheckpointK8sEvents()))

	restored, highWaterMark, err := loadCheckpoint(dir, CheckpointMeta{
		ID:            checkpointID,
		HighWaterMark: 123,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(123), highWaterMark)

	resource, err := NewAnalysisStore(restored).GetResource(context.Background(), "pod-1")
	require.NoError(t, err)
	require.NotNil(t, resource)
	require.Equal(t, "pod-1", resource.UID)
}
