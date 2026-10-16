package embeddedstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func pinnedTestEvent(id, uid string, timestamp int64, eventType models.EventType, revision int) models.Event {
	event := models.Event{
		ID:        id,
		Timestamp: timestamp,
		Type:      eventType,
		Resource:  models.ResourceMetadata{Version: "v1", Kind: "ConfigMap", Namespace: "default", Name: uid, UID: uid},
	}
	if eventType != models.EventTypeDelete {
		event.Data = []byte(fmt.Sprintf(`{"kind":"ConfigMap","metadata":{"name":%q,"namespace":"default","uid":%q},"data":{"rev":"%d"}}`, uid, uid, revision))
	}
	return event
}

func TestRetention_PinsStateAtWindowStartAndDropsExpiredSegments(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	old := now.Add(-72 * time.Hour).UnixNano()
	older := now.Add(-96 * time.Hour).UnixNano()
	recent := now.Add(-time.Hour).UnixNano()

	engine, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 8, EmbeddedRetentionDays: 1, DisableAutoCompaction: true})
	require.NoError(t, err)

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		pinnedTestEvent("stable-1", "stable", older, models.EventTypeCreate, 1),
		pinnedTestEvent("stable-2", "stable", old, models.EventTypeUpdate, 2),
		pinnedTestEvent("changed-1", "changed", older, models.EventTypeCreate, 1),
		pinnedTestEvent("gone-1", "gone", older, models.EventTypeCreate, 1),
		pinnedTestEvent("gone-2", "gone", old, models.EventTypeDelete, 0),
	}))
	require.NoError(t, engine.Flush(context.Background()))
	expiredSegment := engine.manifest.ActiveSegments[0].ID
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		pinnedTestEvent("changed-2", "changed", recent, models.EventTypeUpdate, 2),
	}))
	require.NoError(t, engine.Checkpoint(context.Background()))

	_, statErr := os.Stat(filepath.Join(engine.rootDir, segmentsDirName, expiredSegment))
	require.ErrorIs(t, statErr, os.ErrNotExist, "fully expired segment is removed by the checkpoint")
	require.Len(t, engine.manifest.PinnedSegments, 1)
	require.NotContains(t, engine.projection.resourcesByUID, "gone", "deleted before the window: nothing to pin")

	assertState := func(e *Engine) {
		t.Helper()
		stable := e.projection.resourcesByUID["stable"]
		require.NotNil(t, stable)
		require.Len(t, stable.versions, 1)
		require.Equal(t, "stable-2", stable.versions[0].eventID())
		require.Contains(t, string(versionPayload(&stable.versions[0])), `"rev":"2"`)

		changed := e.projection.resourcesByUID["changed"]
		require.NotNil(t, changed)
		require.Equal(t, []string{"changed-1", "changed-2"}, []string{changed.versions[0].eventID(), changed.versions[1].eventID()})
		require.Contains(t, string(versionPayload(&changed.versions[0])), `"rev":"1"`)

		result, _, err := e.QueryExecutor().ExecutePaginated(context.Background(), &models.QueryRequest{
			StartTimestamp: now.Add(-2 * time.Hour).Unix(),
			EndTimestamp:   now.Unix(),
		}, nil)
		require.NoError(t, err)
		byID := map[string]models.Event{}
		for _, event := range result.Events {
			byID[event.ID] = event
		}
		require.True(t, byID["stable-2"].PreExisting)
		require.Contains(t, string(byID["stable-2"].Data), `"rev":"2"`)
		require.True(t, byID["changed-1"].PreExisting)
		require.Contains(t, string(byID["changed-1"].Data), `"rev":"1"`)
		require.Contains(t, string(byID["changed-2"].Data), `"rev":"2"`)

		exported, err := e.QueryExecutor().ExportTimeRange(context.Background(), &models.QueryRequest{
			StartTimestamp: time.Unix(0, older).Add(-time.Hour).Unix(),
			EndTimestamp:   now.Unix(),
		})
		require.NoError(t, err)
		exportedIDs := checkpointEventIDs(exported)
		require.Contains(t, exportedIDs, "changed-2")
		for _, pinnedID := range []string{"stable-1", "stable-2", "changed-1", "gone-1", "gone-2"} {
			require.NotContains(t, exportedIDs, pinnedID, "pinned payloads are not part of the event history")
		}
	}
	assertState(engine)
	require.NoError(t, engine.Close())

	reopened, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 8, EmbeddedRetentionDays: 1, DisableAutoCompaction: true})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, reopened.Close())
	}()
	assertState(reopened)

	// Repeated retention passes accumulate pinned segments until they are merged.
	for i := 0; i < maxPinnedSegments+2; i++ {
		require.NoError(t, reopened.ProcessBatch(context.Background(), []models.Event{
			pinnedTestEvent(fmt.Sprintf("tick-%d", i), "ticker", now.Add(-time.Duration(30-i)*time.Minute).UnixNano(), models.EventTypeUpdate, i),
		}))
		reopened.mu.Lock()
		reopened.manifest.PinnedSegments = append(reopened.manifest.PinnedSegments, reopened.manifest.PinnedSegments[0])
		reopened.mu.Unlock()
		require.NoError(t, reopened.Checkpoint(context.Background()))
		require.LessOrEqual(t, len(reopened.manifest.PinnedSegments), maxPinnedSegments)
	}
	assertState(reopened)
}

func TestRetention_KeepsStraddlingSegmentWholeAndClampsQueries(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	expired := now.Add(-30 * time.Hour).UnixNano()
	retained := now.Add(-20 * time.Hour).UnixNano()

	engine, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 8, EmbeddedRetentionDays: 1, DisableAutoCompaction: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, engine.Close()) })

	// One segment holds records on both sides of the 24h cutoff, including a core/v1
	// Event, which only segment scans return.
	k8sEvent := func(id string, ts int64) models.Event {
		return models.Event{
			ID: id, Timestamp: ts, Type: models.EventTypeCreate,
			Resource: models.ResourceMetadata{Version: "v1", Kind: "Event", Namespace: "default", Name: id, UID: id + "-uid", InvolvedObjectUID: "cm"},
			Data:     []byte(`{"reason":"Updated","message":"m","type":"Normal","count":1}`),
		}
	}
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		pinnedTestEvent("cm-1", "cm", expired, models.EventTypeCreate, 1),
		k8sEvent("ev-old", expired),
		pinnedTestEvent("cm-2", "cm", retained, models.EventTypeUpdate, 2),
		k8sEvent("ev-new", retained),
	}))
	require.NoError(t, engine.Flush(context.Background()))
	segmentID := engine.manifest.ActiveSegments[0].ID
	require.NoError(t, engine.Checkpoint(context.Background()))

	engine.mu.Lock()
	require.Len(t, engine.manifest.ActiveSegments, 1)
	require.Equal(t, segmentID, engine.manifest.ActiveSegments[0].ID, "a straddling segment is kept, not rewritten")
	engine.mu.Unlock()

	result, _, err := engine.QueryExecutor().ExecutePaginated(context.Background(), &models.QueryRequest{
		StartTimestamp: now.Add(-48 * time.Hour).Unix(),
		EndTimestamp:   now.Unix(),
		Filters:        models.QueryFilters{Kinds: []string{"Event"}},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"ev-new"}, eventIDs(result.Events), "expired records in a kept segment stay hidden")
}
