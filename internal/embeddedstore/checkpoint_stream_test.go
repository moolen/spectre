package embeddedstore

import (
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	require.FileExists(t, filepath.Join(checkpointDir, "resources.gob"))
	require.FileExists(t, filepath.Join(checkpointDir, "k8s-events.gob"))
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

func TestCheckpoint_WritesRawJSONResourcePayloads(t *testing.T) {
	dir := t.TempDir()
	projection, err := BuildProjection([]models.Event{
		{
			ID:        "pod-create",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
			Data: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"},"spec":{"containers":[{"name":"app","image":"nginx:1.29"}]}}`),
		},
	})
	require.NoError(t, err)

	meta, err := writeCheckpoint(dir, projection, 10)
	require.NoError(t, err)

	resourcesPath := filepath.Join(dir, checkpointsDirName, meta.ID, checkpointResourcesFile)
	file, err := os.Open(resourcesPath)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, file.Close())
	}()

	var snapshot ProjectionResourceSnapshot
	require.NoError(t, gob.NewDecoder(file).Decode(&snapshot))
	require.Len(t, snapshot.Versions, 1)
	require.JSONEq(
		t,
		`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"},"spec":{"containers":[{"name":"app","image":"nginx:1.29"}]}}`,
		string(snapshot.Versions[0].Data),
	)
}

func TestProjectionFromCheckpointStream_LoadsLegacyBase64Payloads(t *testing.T) {
	resourceJSON := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"}}`)
	resourcesPayload := fmt.Sprintf(
		`{"uid":"pod-1","versions":[{"event_id":"pod-create","timestamp":10,"event_type":"CREATE","identity":{"uid":"pod-1","kind":"Pod","apiGroup":"","version":"v1","namespace":"default","name":"pod-1","labels":{"app":"demo"},"firstSeen":10,"lastSeen":10,"deleted":false,"deletedAt":0},"data":"%s","change_event":{"event_id":"pod-create","timestamp":"1970-01-01T00:00:00.000000010Z","event_type":"CREATE","description":"CREATE event"}}]}`+"\n",
		base64.StdEncoding.EncodeToString(resourceJSON),
	)

	projection, err := ProjectionFromCheckpointStream(
		checkpointState{
			FormatVersion:  checkpointFormatVersionV1,
			HighWaterMark:  10,
			MinTimestampNs: 10,
			MaxTimestampNs: 10,
		},
		strings.NewReader(resourcesPayload),
		strings.NewReader(`{}`),
	)
	require.NoError(t, err)

	record := projection.resourcesByUID["pod-1"]
	require.NotNil(t, record)
	require.Len(t, record.versions, 1)
	require.JSONEq(t, string(resourceJSON), string(record.versions[0].data))
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
