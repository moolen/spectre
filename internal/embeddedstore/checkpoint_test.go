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
