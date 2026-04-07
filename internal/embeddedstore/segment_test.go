package embeddedstore

import (
	"context"
	"fmt"
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
	for _, name := range []string{"events.bin", "time.idx", "resource.idx", "associated.idx", "dim.idx", "stats.json"} {
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

func TestSegment_ScanTimeRangeIncludesAllDuplicateTimestampsAcrossSparseIndex(t *testing.T) {
	dir := t.TempDir()

	events := make([]models.Event, 0, 96)
	for i := 0; i < 96; i++ {
		events = append(events, models.Event{
			ID:        fmt.Sprintf("dup-%03d", i),
			Timestamp: 100,
			Resource: models.ResourceMetadata{
				Namespace: "default",
				Kind:      "Pod",
				UID:       fmt.Sprintf("pod-%03d", i),
			},
		})
	}

	meta, err := writeSegment(dir, "seg-001", events)
	require.NoError(t, err)

	reader, err := openSegmentReader(dir, meta)
	require.NoError(t, err)

	got, err := reader.ScanTimeRange(context.Background(), 100, 100)
	require.NoError(t, err)
	require.Len(t, got, len(events))
	for i := range got {
		require.Equal(t, fmt.Sprintf("dup-%03d", i), got[i].ID)
	}
}

func TestSegment_ScanUIDPreservesWriteOrderForSameTimestamp(t *testing.T) {
	dir := t.TempDir()
	events := []models.Event{
		{
			ID:        "uid-z",
			Timestamp: 200,
			Resource: models.ResourceMetadata{
				Namespace: "default",
				Kind:      "Pod",
				UID:       "pod-1",
			},
		},
		{
			ID:        "uid-a",
			Timestamp: 200,
			Resource: models.ResourceMetadata{
				Namespace: "default",
				Kind:      "Pod",
				UID:       "pod-1",
			},
		},
		{
			ID:        "uid-m",
			Timestamp: 200,
			Resource: models.ResourceMetadata{
				Namespace: "default",
				Kind:      "Pod",
				UID:       "pod-1",
			},
		},
	}

	meta, err := writeSegment(dir, "seg-001", events)
	require.NoError(t, err)

	reader, err := openSegmentReader(dir, meta)
	require.NoError(t, err)

	got, err := reader.ScanUID(context.Background(), "pod-1")
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, "uid-z", got[0].ID)
	require.Equal(t, "uid-a", got[1].ID)
	require.Equal(t, "uid-m", got[2].ID)
}

func TestSegment_MayContainUIDDoesNotHydrateFullResourceIndex(t *testing.T) {
	dir := t.TempDir()
	events := []models.Event{
		{
			ID:        "uid-1",
			Timestamp: 10,
			Resource: models.ResourceMetadata{
				Namespace: "default",
				Kind:      "Pod",
				UID:       "pod-1",
			},
		},
		{
			ID:        "uid-2",
			Timestamp: 20,
			Resource: models.ResourceMetadata{
				Namespace: "default",
				Kind:      "Pod",
				UID:       "pod-2",
			},
		},
	}

	meta, err := writeSegment(dir, "seg-001", events)
	require.NoError(t, err)

	reader, err := openSegmentReader(dir, meta)
	require.NoError(t, err)
	require.Nil(t, reader.resourceIndex)

	require.True(t, reader.MayContainUID("pod-1"))
	require.Nil(t, reader.resourceIndex)

	got, err := reader.ScanUID(context.Background(), "pod-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "uid-1", got[0].ID)
	require.NotNil(t, reader.resourceIndex)
}
