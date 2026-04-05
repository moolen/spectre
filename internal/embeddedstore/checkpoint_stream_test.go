package embeddedstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestCheckpoint_StreamRoundTripProjectionState(t *testing.T) {
	dir := t.TempDir()
	projection, err := BuildProjection(makeReplayHeavyEvents(500))
	require.NoError(t, err)

	meta, err := writeCheckpoint(dir, projection, 500)
	require.NoError(t, err)

	restored, highWaterMark, err := loadCheckpoint(dir, meta)
	require.NoError(t, err)
	require.Equal(t, uint64(500), highWaterMark)
	require.Equal(t, projection.ResourceCount(), restored.ResourceCount())
}

func TestCheckpoint_WritesStreamBundleFiles(t *testing.T) {
	dir := t.TempDir()
	projection, err := BuildProjection(makeReplayHeavyEvents(50))
	require.NoError(t, err)

	meta, err := writeCheckpoint(dir, projection, 7)
	require.NoError(t, err)

	checkpointDir := filepath.Join(dir, checkpointsDirName, meta.ID)
	require.FileExists(t, filepath.Join(checkpointDir, "meta.json"))
	require.FileExists(t, filepath.Join(checkpointDir, "resources.ndjson"))
	require.FileExists(t, filepath.Join(checkpointDir, "k8s-events.json"))
}

func TestCheckpoint_StreamBundleMetaOmitsInlineSnapshot(t *testing.T) {
	dir := t.TempDir()
	projection, err := BuildProjection(makeReplayHeavyEvents(25))
	require.NoError(t, err)

	meta, err := writeCheckpoint(dir, projection, 25)
	require.NoError(t, err)

	metaPath := filepath.Join(dir, checkpointsDirName, meta.ID, "meta.json")
	payload, err := os.ReadFile(metaPath)
	require.NoError(t, err)

	var decoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Contains(t, decoded, "format_version")
	require.Contains(t, decoded, "high_water_mark")
	require.Contains(t, decoded, "min_timestamp_ns")
	require.Contains(t, decoded, "max_timestamp_ns")
	require.NotContains(t, decoded, "snapshot")
}

func TestProjection_StreamCheckpointResources_DoesNotHoldLockDuringEmit(t *testing.T) {
	projection, err := BuildProjection(makeReplayHeavyEvents(1))
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		done <- projection.StreamCheckpointResources(func(ProjectionResourceSnapshot) error {
			return projection.Apply(models.Event{
				ID:        "evt-extra",
				Timestamp: 1000,
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Version:   "v1",
					UID:       "pod-extra",
					Namespace: "default",
					Kind:      "Pod",
					Name:      "pod-extra",
				},
				Data: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-extra","namespace":"default","uid":"pod-extra"}}`),
			})
		})
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("StreamCheckpointResources did not return; emit likely ran under projection lock")
	}
}

func makeReplayHeavyEvents(count int) []models.Event {
	events := make([]models.Event, 0, count)
	for i := 0; i < count; i++ {
		uid := fmt.Sprintf("pod-%04d", i)
		events = append(events, models.Event{
			ID:        fmt.Sprintf("evt-%04d", i),
			Timestamp: int64(i + 1),
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       uid,
				Namespace: "default",
				Kind:      "Pod",
				Name:      uid,
			},
			Data: []byte(fmt.Sprintf(
				`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"%s","namespace":"default","uid":"%s"}}`,
				uid,
				uid,
			)),
		})
	}
	return events
}
