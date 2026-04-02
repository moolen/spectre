package embeddedstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestSegment_WriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	events := []models.Event{
		{
			ID:        "2",
			Timestamp: 20,
			Resource: models.ResourceMetadata{
				Namespace: "default",
				Kind:      "Service",
				UID:       "svc-1",
			},
		},
		{
			ID:        "1",
			Timestamp: 10,
			Resource: models.ResourceMetadata{
				Namespace: "default",
				Kind:      "Pod",
				UID:       "pod-1",
			},
		},
	}

	meta, err := writeSegment(dir, "seg-001", events)
	require.NoError(t, err)

	segmentDir := filepath.Join(dir, "segments", "seg-001")
	for _, name := range []string{"events.bin", "time.idx", "resource.idx", "dim.idx", "stats.json"} {
		_, err := os.Stat(filepath.Join(segmentDir, name))
		require.NoError(t, err)
	}

	reader, err := openSegmentReader(dir, meta)
	require.NoError(t, err)

	got, err := reader.ScanTimeRange(context.Background(), 0, 30)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "1", got[0].ID)
	require.Equal(t, "2", got[1].ID)

	uidEvents, err := reader.ScanUID(context.Background(), "pod-1")
	require.NoError(t, err)
	require.Len(t, uidEvents, 1)
	require.Equal(t, "1", uidEvents[0].ID)
}

func TestSegment_PrunesByNamespaceKindStats(t *testing.T) {
	dir := t.TempDir()
	events := []models.Event{
		{
			ID:        "1",
			Timestamp: 10,
			Resource: models.ResourceMetadata{
				Namespace: "flux-system",
				Kind:      "HelmRelease",
				UID:       "hr-1",
			},
		},
	}

	meta, err := writeSegment(dir, "seg-001", events)
	require.NoError(t, err)
	require.True(t, meta.MayContain("flux-system", "HelmRelease"))
	require.False(t, meta.MayContain("default", "Pod"))

	reader, err := openSegmentReader(dir, meta)
	require.NoError(t, err)
	require.True(t, reader.MayContain("flux-system", "HelmRelease"))
	require.False(t, reader.MayContain("default", "Pod"))
}
