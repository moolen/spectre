package embeddedstore

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestCheckpoint_RoundTripCompactProjectionState(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	serviceAccount := models.ResourceMetadata{
		Version:   "v1",
		Kind:      "ServiceAccount",
		Namespace: "default",
		Name:      "builder",
		UID:       "sa-1",
	}
	pod := models.ResourceMetadata{
		Version:   "v1",
		Kind:      "Pod",
		Namespace: "default",
		Name:      "pod-1",
		UID:       "pod-1",
	}

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "sa-create",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource:  serviceAccount,
			Data:      []byte(`{"apiVersion":"v1","kind":"ServiceAccount","metadata":{"name":"builder","namespace":"default","uid":"sa-1"}}`),
		},
	}))
	require.NoError(t, engine.Flush(context.Background()))

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "pod-create",
			Timestamp: 20,
			Type:      models.EventTypeCreate,
			Resource:  pod,
			Data:      []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"},"spec":{"serviceAccountName":"builder"}}`),
		},
		{
			ID:        "pod-warning",
			Timestamp: 25,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:           "v1",
				Kind:              "Event",
				Namespace:         "default",
				Name:              "pod-1.warning",
				UID:               "evt-1",
				InvolvedObjectUID: "pod-1",
			},
			Data: []byte(`{"reason":"BackOff","message":"container restart backoff","type":"Warning","count":2,"source":{"component":"kubelet"}}`),
		},
	}))

	window := analysisstore.ResourceWindow{
		FailureTimestampNs: 30,
		LookbackNs:         30,
	}
	graphQuery := analysisstore.NamespaceGraphQuery{
		Namespace:   "default",
		TimestampNs: 30,
		LookbackNs:  30,
		Limit:       10,
		MaxDepth:    2,
	}

	expectedGraph, err := engine.AnalysisStore().GetNamespaceGraph(context.Background(), graphQuery)
	require.NoError(t, err)
	expectedRelated, err := engine.AnalysisStore().GetRelatedResources(context.Background(), []string{"pod-1"}, window)
	require.NoError(t, err)
	expectedK8sEvents, err := engine.AnalysisStore().GetK8sEvents(context.Background(), []string{"pod-1"}, window)
	require.NoError(t, err)

	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.Close())

	reopened, err := OpenEngine(EngineConfig{
		DataDir:                engine.config.DataDir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, reopened.Close())
	}()

	actualGraph, err := reopened.AnalysisStore().GetNamespaceGraph(context.Background(), graphQuery)
	require.NoError(t, err)
	actualRelated, err := reopened.AnalysisStore().GetRelatedResources(context.Background(), []string{"pod-1"}, window)
	require.NoError(t, err)
	actualK8sEvents, err := reopened.AnalysisStore().GetK8sEvents(context.Background(), []string{"pod-1"}, window)
	require.NoError(t, err)

	require.Equal(t, namespaceGraphNodeKeys(expectedGraph.Graph.Nodes), namespaceGraphNodeKeys(actualGraph.Graph.Nodes))
	require.Equal(t, namespaceGraphEdgeKeys(expectedGraph.Graph.Edges), namespaceGraphEdgeKeys(actualGraph.Graph.Edges))
	require.Equal(t, relatedResourceKeys(expectedRelated["pod-1"]), relatedResourceKeys(actualRelated["pod-1"]))
	require.Equal(t, k8sEventKeys(expectedK8sEvents["pod-1"]), k8sEventKeys(actualK8sEvents["pod-1"]))
}

func TestCheckpoint_WritesCompressedBinaryStreamsAndReopens(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)

	pod := models.ResourceMetadata{
		Version:   "v1",
		Kind:      "Pod",
		Namespace: "default",
		Name:      "pod-1",
		UID:       "pod-1",
	}

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "pod-create",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource:  pod,
			Data:      []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"}}`),
		},
		{
			ID:        "pod-k8s-event",
			Timestamp: 20,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:           "v1",
				Kind:              "Event",
				Namespace:         "default",
				Name:              "pod-1.warning",
				UID:               "evt-1",
				InvolvedObjectUID: "pod-1",
			},
			Data: []byte(`{"reason":"BackOff","message":"container restart backoff","type":"Warning","count":2}`),
		},
	}))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.Checkpoint(context.Background()))

	checkpointMeta := latestCheckpointMeta(engine.manifest.Checkpoints)
	checkpointDir := filepath.Join(embeddedRootDir(engine.config.DataDir), checkpointsDirName, checkpointMeta.ID)

	resourcesPayload, err := os.ReadFile(filepath.Join(checkpointDir, checkpointResourcesFile))
	require.NoError(t, err)
	k8sPayload, err := os.ReadFile(filepath.Join(checkpointDir, checkpointK8sEventsFile))
	require.NoError(t, err)

	require.True(t, bytes.HasPrefix(resourcesPayload, []byte{0x28, 0xb5, 0x2f, 0xfd}))
	require.True(t, bytes.HasPrefix(k8sPayload, []byte{0x28, 0xb5, 0x2f, 0xfd}))

	require.NoError(t, engine.Close())

	reopened, err := OpenEngine(EngineConfig{
		DataDir:                engine.config.DataDir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reopened.Close())
	})

	record := reopened.projection.resourcesByUID["pod-1"]
	require.NotNil(t, record)
	require.Len(t, record.versions, 1)

	k8sEvents, err := reopened.AnalysisStore().GetK8sEvents(context.Background(), []string{"pod-1"}, analysisstore.ResourceWindow{
		FailureTimestampNs: 30,
		LookbackNs:         30,
	})
	require.NoError(t, err)
	require.Len(t, k8sEvents["pod-1"], 1)
	require.Equal(t, "pod-k8s-event", k8sEvents["pod-1"][0].EventID)
}

func TestCheckpoint_LoadCheckpointAcceptsLegacyUncompressedV2Streams(t *testing.T) {
	rootDir := filepath.Join(t.TempDir(), "embedded")
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, checkpointsDirName), 0o755))

	pod := models.ResourceMetadata{
		Version:   "v1",
		Kind:      "Pod",
		Namespace: "default",
		Name:      "pod-1",
		UID:       "pod-1",
	}
	events := []models.Event{
		{
			ID:        "pod-create",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource:  pod,
			Data:      []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"}}`),
		},
		{
			ID:        "pod-k8s-event",
			Timestamp: 20,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:           "v1",
				Kind:              "Event",
				Namespace:         "default",
				Name:              "pod-1.warning",
				UID:               "evt-1",
				InvolvedObjectUID: "pod-1",
			},
			Data: []byte(`{"reason":"BackOff","message":"container restart backoff","type":"Warning","count":2}`),
		},
	}

	projection, err := BuildProjection(events)
	require.NoError(t, err)

	checkpointMeta := CheckpointMeta{
		ID:            "chk-00000000000000000002-legacy",
		HighWaterMark: 2,
	}
	checkpointDir := filepath.Join(rootDir, checkpointsDirName, checkpointMeta.ID)
	require.NoError(t, os.MkdirAll(checkpointDir, 0o755))
	require.NoError(t, writeJSONFile(filepath.Join(checkpointDir, checkpointStateFile), projection.CheckpointState(checkpointMeta.HighWaterMark)))

	resourcesFile, err := os.OpenFile(filepath.Join(checkpointDir, checkpointResourcesFile), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	require.NoError(t, err)
	resourceEncoder := gob.NewEncoder(resourcesFile)
	require.NoError(t, projection.StreamCheckpointResources(func(snapshot ProjectionResourceSnapshot) error {
		return resourceEncoder.Encode(&snapshot)
	}))
	require.NoError(t, resourcesFile.Close())

	k8sFile, err := os.OpenFile(filepath.Join(checkpointDir, checkpointK8sEventsFile), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	require.NoError(t, err)
	require.NoError(t, gob.NewEncoder(k8sFile).Encode(projection.CheckpointK8sEvents()))
	require.NoError(t, k8sFile.Close())

	loaded, highWaterMark, err := loadCheckpoint(rootDir, checkpointMeta)
	require.NoError(t, err)
	require.Equal(t, checkpointMeta.HighWaterMark, highWaterMark)

	record := loaded.resourcesByUID["pod-1"]
	require.NotNil(t, record)
	require.Len(t, record.versions, 1)
	require.Equal(t, "pod-create", record.versions[0].eventID)

	k8sEvents := loaded.k8sEventsByInvolvedUID["pod-1"]
	require.Len(t, k8sEvents, 1)
	require.Equal(t, "pod-k8s-event", k8sEvents[0].EventID)
}

func TestCheckpoint_RoundTripCompactsRedundantResourceVersionsWhilePreservingHistory(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)

	pod := models.ResourceMetadata{
		Version:   "v1",
		Kind:      "Pod",
		Namespace: "default",
		Name:      "pod-1",
		UID:       "pod-1",
	}

	sameState := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"},"spec":{"containers":[{"name":"app","image":"nginx:1.29"}]}}`)
	changedState := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"},"spec":{"containers":[{"name":"app","image":"nginx:1.30"}]}}`)

	events := []models.Event{
		{ID: "pod-create-1", Timestamp: 10, Type: models.EventTypeCreate, Resource: pod, Data: sameState},
		{ID: "pod-create-2", Timestamp: 20, Type: models.EventTypeCreate, Resource: pod, Data: sameState},
		{ID: "pod-update-same", Timestamp: 30, Type: models.EventTypeUpdate, Resource: pod, Data: sameState},
		{ID: "pod-update-changed-1", Timestamp: 40, Type: models.EventTypeUpdate, Resource: pod, Data: changedState},
		{ID: "pod-update-changed-2", Timestamp: 50, Type: models.EventTypeUpdate, Resource: pod, Data: changedState},
	}

	require.NoError(t, engine.ProcessBatch(context.Background(), events))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.Close())

	reopened, err := OpenEngine(EngineConfig{
		DataDir:                engine.config.DataDir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reopened.Close())
	})

	record := reopened.projection.resourcesByUID["pod-1"]
	require.NotNil(t, record)
	require.Len(t, record.versions, 2)
	require.Equal(t, []string{"pod-create-1", "pod-update-changed-1"}, []string{
		record.versions[0].eventID,
		record.versions[1].eventID,
	})

	exported, err := reopened.QueryExecutor().ExportTimeRange(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   1_000,
	})
	require.NoError(t, err)
	require.Equal(t, []string{
		"pod-create-1",
		"pod-create-2",
		"pod-update-same",
		"pod-update-changed-1",
		"pod-update-changed-2",
	}, checkpointEventIDs(exported))
}

func TestCheckpoint_RoundTripRetainsRecentAnalysisWindow(t *testing.T) {
	now := time.Now().UTC()
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
		EmbeddedRetentionDays:  1,
	})
	require.NoError(t, err)

	pod := models.ResourceMetadata{
		Version:   "v1",
		Kind:      "Pod",
		Namespace: "default",
		Name:      "pod-1",
		UID:       "pod-1",
	}

	timestamps := []int64{
		now.Add(-36 * time.Hour).UnixNano(),
		now.Add(-12 * time.Hour).UnixNano(),
		now.Add(-6 * time.Hour).UnixNano(),
		now.Add(-1 * time.Hour).UnixNano(),
	}

	events := make([]models.Event, 0, len(timestamps))
	for i := range timestamps {
		events = append(events, models.Event{
			ID:        checkpointEventID("pod-window", i),
			Timestamp: timestamps[i],
			Type:      models.EventTypeUpdate,
			Resource:  pod,
			Data: []byte(fmt.Sprintf(
				`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"},"spec":{"containers":[{"name":"app","image":"nginx:%d"}]}}`,
				i,
			)),
		})
	}

	require.NoError(t, engine.ProcessBatch(context.Background(), events))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.Close())

	reopened, err := OpenEngine(EngineConfig{
		DataDir:                engine.config.DataDir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
		EmbeddedRetentionDays:  1,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reopened.Close())
	})

	record := reopened.projection.resourcesByUID["pod-1"]
	require.NotNil(t, record)
	require.Len(t, record.versions, 4)
	require.Equal(t, []string{
		checkpointEventID("pod-window", 0),
		checkpointEventID("pod-window", 1),
		checkpointEventID("pod-window", 2),
		checkpointEventID("pod-window", 3),
	}, []string{
		record.versions[0].eventID,
		record.versions[1].eventID,
		record.versions[2].eventID,
		record.versions[3].eventID,
	})
}

func TestCheckpoint_RetentionDisabledKeepsFullAnalysisHistory(t *testing.T) {
	now := time.Now().UTC()
	timestamps := []int64{
		now.Add(-72 * time.Hour).UnixNano(),
		now.Add(-36 * time.Hour).UnixNano(),
		now.Add(-12 * time.Hour).UnixNano(),
	}

	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
		EmbeddedRetentionDays:  0,
	})
	require.NoError(t, err)

	pod := models.ResourceMetadata{
		Version:   "v1",
		Kind:      "Pod",
		Namespace: "default",
		Name:      "pod-history",
		UID:       "pod-history",
	}

	events := make([]models.Event, 0, len(timestamps))
	for i := range timestamps {
		events = append(events, models.Event{
			ID:        checkpointEventID("pod-history", i),
			Timestamp: timestamps[i],
			Type:      models.EventTypeUpdate,
			Resource:  pod,
			Data: []byte(fmt.Sprintf(
				`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-history","namespace":"default","uid":"pod-history"},"spec":{"containers":[{"name":"app","image":"nginx:%d"}]}}`,
				i,
			)),
		})
	}

	require.NoError(t, engine.ProcessBatch(context.Background(), events))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.Close())

	reopened, err := OpenEngine(EngineConfig{
		DataDir:                engine.config.DataDir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
		EmbeddedRetentionDays:  0,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reopened.Close())
	})

	record := reopened.projection.resourcesByUID["pod-history"]
	require.NotNil(t, record)
	require.Len(t, record.versions, 3)
}

func checkpointEventID(prefix string, idx int) string {
	return fmt.Sprintf("%s-%d", prefix, idx)
}

func namespaceGraphNodeKeys(nodes []analysisstore.NamespaceGraphNode) []string {
	keys := make([]string, 0, len(nodes))
	for i := range nodes {
		keys = append(keys, nodes[i].Kind+"/"+nodes[i].Namespace+"/"+nodes[i].Name+"/"+nodes[i].UID)
	}
	return keys
}

func namespaceGraphEdgeKeys(edges []analysisstore.NamespaceGraphEdge) []string {
	keys := make([]string, 0, len(edges))
	for i := range edges {
		keys = append(keys, edges[i].Source+"/"+edges[i].RelationshipType+"/"+edges[i].Target)
	}
	return keys
}

func relatedResourceKeys(items []analysisstore.RelatedResourceData) []string {
	keys := make([]string, 0, len(items))
	for i := range items {
		item := items[i]
		keys = append(keys, item.RelationshipType+"/"+item.Resource.Kind+"/"+item.Resource.Namespace+"/"+item.Resource.Name+"/"+item.Resource.UID)
	}
	return keys
}

func k8sEventKeys(items []analysisstore.K8sEventInfo) []string {
	keys := make([]string, 0, len(items))
	for i := range items {
		item := items[i]
		keys = append(keys, item.EventID+"/"+item.Reason+"/"+item.Message)
	}
	return keys
}

func checkpointEventIDs(events []models.Event) []string {
	ids := make([]string, 0, len(events))
	for i := range events {
		ids = append(ids, events[i].ID)
	}
	return ids
}
