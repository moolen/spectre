package embeddedstore

import (
	"context"
	"encoding/binary"
	"encoding/json"
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

func TestSegment_WriteStoresCompressedEventFrames(t *testing.T) {
	dir := t.TempDir()
	events := []models.Event{
		{
			ID:        "1",
			Timestamp: 10,
			Resource: models.ResourceMetadata{
				Namespace: "default",
				Kind:      "ConfigMap",
				UID:       "cfg-1",
			},
			Data: []byte(`{"kind":"ConfigMap","metadata":{"name":"cfg-1","namespace":"default","uid":"cfg-1"},"data":{"blob":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`),
		},
	}

	meta, err := writeSegment(dir, "seg-001", events)
	require.NoError(t, err)

	payload, err := os.ReadFile(filepath.Join(dir, "segments", "seg-001", "events.bin"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(payload), 4)
	recordLen := binary.BigEndian.Uint32(payload[:4])
	require.NotZero(t, recordLen&(1<<31))

	reader, err := openSegmentReader(dir, meta)
	require.NoError(t, err)
	got, err := reader.ScanTimeRange(context.Background(), 0, 20)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, events[0].ID, got[0].ID)
	require.Equal(t, string(events[0].Data), string(got[0].Data))
}

func TestSegment_ReadsLegacyUncompressedFrames(t *testing.T) {
	dir := t.TempDir()
	events := []models.Event{
		{
			ID:        "legacy-1",
			Timestamp: 10,
			Resource: models.ResourceMetadata{
				Namespace: "default",
				Kind:      "Pod",
				UID:       "pod-1",
			},
		},
	}

	meta, err := writeLegacyUncompressedSegmentForTest(dir, "seg-legacy", events)
	require.NoError(t, err)

	reader, err := openSegmentReader(dir, meta)
	require.NoError(t, err)
	got, err := reader.ScanTimeRange(context.Background(), 0, 20)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "legacy-1", got[0].ID)
}

func writeLegacyUncompressedSegmentForTest(rootDir, segmentID string, events []models.Event) (segmentBundleMeta, error) {
	segmentDir := filepath.Join(rootDir, "segments", segmentID)
	if err := os.MkdirAll(segmentDir, 0o755); err != nil {
		return segmentBundleMeta{}, err
	}

	eventsFile, err := os.OpenFile(filepath.Join(segmentDir, "events.bin"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return segmentBundleMeta{}, err
	}
	defer func() {
		_ = eventsFile.Close()
	}()

	timeIndexEntries := make([]segmentTimeIndexEntry, 0, len(events))
	resourceIndex := segmentResourceIndex{UIDOffsets: map[string][]int64{}}
	associatedIndex := segmentAssociatedIndex{InvolvedUIDOffsets: map[string][]int64{}}
	dimensions := make([]segmentDimensionEntry, 0, len(events))
	namespaceKinds := make(map[string][]string)

	var offset int64
	for i := range events {
		timeIndexEntries = append(timeIndexEntries, segmentTimeIndexEntry{
			Timestamp: events[i].Timestamp,
			Offset:    offset,
		})
		resourceIndex.UIDOffsets[events[i].Resource.UID] = append(resourceIndex.UIDOffsets[events[i].Resource.UID], offset)
		dimensions = append(dimensions, segmentDimensionEntry{
			Namespace: events[i].Resource.Namespace,
			Kind:      events[i].Resource.Kind,
		})
		namespaceKinds[events[i].Resource.Namespace] = append(namespaceKinds[events[i].Resource.Namespace], events[i].Resource.Kind)

		payload, err := json.Marshal(events[i])
		if err != nil {
			return segmentBundleMeta{}, err
		}
		header := make([]byte, 4)
		binary.BigEndian.PutUint32(header, uint32(len(payload)))
		if _, err := eventsFile.Write(header); err != nil {
			return segmentBundleMeta{}, err
		}
		if _, err := eventsFile.Write(payload); err != nil {
			return segmentBundleMeta{}, err
		}
		offset += int64(4 + len(payload))
	}
	if err := eventsFile.Close(); err != nil {
		return segmentBundleMeta{}, err
	}

	meta := segmentBundleMeta{
		ID:             segmentID,
		EventCount:     len(events),
		MinTimestamp:   events[0].Timestamp,
		MaxTimestamp:   events[len(events)-1].Timestamp,
		NamespaceKinds: normalizeNamespaceKinds(namespaceKinds),
	}
	if err := writeTimeIndex(filepath.Join(segmentDir, "time.idx"), timeIndexEntries); err != nil {
		return segmentBundleMeta{}, err
	}
	if err := writeJSONFile(filepath.Join(segmentDir, "resource.idx"), resourceIndex); err != nil {
		return segmentBundleMeta{}, err
	}
	if err := writeJSONFile(filepath.Join(segmentDir, "associated.idx"), associatedIndex); err != nil {
		return segmentBundleMeta{}, err
	}
	if err := writeJSONFile(filepath.Join(segmentDir, "dim.idx"), segmentDimensionIndex{Entries: dimensions}); err != nil {
		return segmentBundleMeta{}, err
	}
	if err := writeJSONFile(filepath.Join(segmentDir, "stats.json"), meta); err != nil {
		return segmentBundleMeta{}, err
	}
	return meta, nil
}
