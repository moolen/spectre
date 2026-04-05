package embeddedstore

import (
	"fmt"
	"path/filepath"
	"testing"

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
