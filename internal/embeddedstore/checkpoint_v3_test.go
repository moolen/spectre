package embeddedstore

import (
	"context"
	"fmt"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func checkpointTestEvent(id string, timestamp int64, uid string, revision int) models.Event {
	return models.Event{
		ID:        id,
		Timestamp: timestamp,
		Type:      models.EventTypeUpdate,
		Resource:  models.ResourceMetadata{Version: "v1", Kind: "Pod", Namespace: "default", Name: uid, UID: uid},
		Data:      []byte(fmt.Sprintf(`{"kind":"Pod","metadata":{"name":%q,"labels":{"app":"x"}},"spec":{"rev":%d},"status":{"phase":"Running"}}`, uid, revision)),
	}
}

func TestCheckpointV3_CrashBeforePublishReplaysRotatedTail(t *testing.T) {
	dir := t.TempDir()
	cfg := EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 8, DisableAutoCompaction: true}
	engine, err := OpenEngine(cfg)
	require.NoError(t, err)

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{checkpointTestEvent("a", 10, "pod-1", 1)}))
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		checkpointTestEvent("b", 20, "pod-1", 2),
		checkpointTestEvent("c", 30, "pod-2", 1),
	}))

	// Phase one of a checkpoint (flush, rotate tail, snapshot), then more ingest, then a
	// crash before the checkpoint is published.
	engine.mu.Lock()
	_, _, err = engine.beginCheckpointLocked(context.Background())
	engine.mu.Unlock()
	require.NoError(t, err)
	require.Len(t, engine.manifest.PreviousTails, 1)
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{checkpointTestEvent("d", 40, "pod-2", 2)}))

	reopened, err := OpenEngine(cfg)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, reopened.Close())
	}()

	for uid, expected := range map[string][]string{"pod-1": {"a", "b"}, "pod-2": {"c", "d"}} {
		record := reopened.projection.resourcesByUID[uid]
		require.NotNil(t, record, uid)
		ids := []string{}
		for i := range record.versions {
			ids = append(ids, record.versions[i].eventID())
			require.NotEmpty(t, versionPayload(&record.versions[i]), "%s/%s payload", uid, record.versions[i].eventID())
		}
		require.Equal(t, expected, ids, uid)
	}

	require.NoError(t, reopened.Flush(context.Background()))
	exported, err := reopened.QueryExecutor().ExportTimeRange(context.Background(), &models.QueryRequest{StartTimestamp: 0, EndTimestamp: 100})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"a", "b", "c", "d"}, eventIDs(exported), "each event is stored once")

	require.NoError(t, reopened.Checkpoint(context.Background()))
	require.Empty(t, reopened.manifest.PreviousTails)
}

func TestCheckpointV3_RoundTripsVersionStateAndEventSummaries(t *testing.T) {
	dir := t.TempDir()
	cfg := EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 8}
	engine, err := OpenEngine(cfg)
	require.NoError(t, err)

	events := []models.Event{
		checkpointTestEvent("a", 10, "pod-1", 1),
		checkpointTestEvent("b", 20, "pod-1", 2),
		plannerParityK8sEvent("k1", "evt-1", "pod-1.a", "default", "pod-1", 25, "BackOff", "restarting", "Warning", 3, "kubelet"),
	}
	deleted := checkpointTestEvent("c", 30, "pod-1", 0)
	deleted.Type = models.EventTypeDelete
	deleted.Data = nil
	events = append(events, deleted)
	require.NoError(t, engine.ProcessBatch(context.Background(), events))
	before := engine.projection.resourcesByUID["pod-1"]
	beforeInfo := make([]any, 0)
	for i := range before.versions {
		version := &before.versions[i]
		beforeInfo = append(beforeInfo, version.identity(), version.changeEventInfo())
	}
	require.NoError(t, engine.Close())

	reopened, err := OpenEngine(cfg)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, reopened.Close())
	}()
	after := reopened.projection.resourcesByUID["pod-1"]
	require.NotNil(t, after)
	afterInfo := make([]any, 0)
	for i := range after.versions {
		version := &after.versions[i]
		afterInfo = append(afterInfo, version.identity(), version.changeEventInfo())
	}
	require.Equal(t, beforeInfo, afterInfo)
	require.Same(t, after.versions[0].meta, after.versions[1].meta, "identity meta is shared after restore")
	require.True(t, after.latest.known)

	infos := reopened.projection.k8sEventsByInvolvedUID["pod-1"]
	require.Len(t, infos, 1)
	require.Equal(t, "BackOff", infos[0].Reason)
	require.Equal(t, 3, infos[0].Count)
	require.Equal(t, "kubelet", infos[0].Source)
}

func TestProjection_OutOfOrderInsertMatchesInOrderHistoryAndKeepsReferences(t *testing.T) {
	ordered := []models.Event{
		checkpointTestEvent("e0", 5, "pod-1", 0),
		checkpointTestEvent("e1", 10, "pod-1", 1),
		checkpointTestEvent("e2", 20, "pod-1", 1),
		checkpointTestEvent("e3", 30, "pod-1", 2),
	}
	ordered[2].Data = []byte(`{"kind":"Pod","metadata":{"name":"pod-1","labels":{"app":"y"}},"spec":{"rev":1},"status":{"phase":"Pending"}}`)

	expected, err := BuildProjection(ordered)
	require.NoError(t, err)

	engine, err := OpenEngine(EngineConfig{DataDir: t.TempDir(), HotMaxEvents: 100, HotMaxResourceVersions: 8, DisableAutoCompaction: true})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, engine.Close())
	}()
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{ordered[1], ordered[3]}))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{ordered[2], ordered[0]}))

	describe := func(record *resourceRecord) []any {
		out := []any{}
		for i := range record.versions {
			version := &record.versions[i]
			out = append(out, version.eventID(), version.identity(), version.changeEventInfo())
		}
		return out
	}
	actual := engine.projection.resourcesByUID["pod-1"]
	require.Equal(t, describe(expected.resourcesByUID["pod-1"]), describe(actual))
	for i := range actual.versions {
		version := &actual.versions[i]
		switch version.eventID() {
		case "e1", "e3":
			require.True(t, version.ref.valid(), "%s keeps its segment reference", version.eventID())
		default:
			require.NotEmpty(t, version.inlinePayload(), "%s is unflushed", version.eventID())
		}
	}
}
