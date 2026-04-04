package embeddedstore_test

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestProjection_ApplyEventUpdatesTimelineAndAnalysisViews(t *testing.T) {
	p := embeddedstore.NewProjection()
	err := p.Apply(models.Event{
		ID:        "pod-create",
		Timestamp: 10,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Kind:      "Pod",
			Version:   "v1",
			Name:      "pod-1",
			Namespace: "default",
			UID:       "pod-uid",
		},
	})
	require.NoError(t, err)

	qe := embeddedstore.NewQueryExecutor(p)
	result, err := qe.Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   20,
		Filters:        models.QueryFilters{Kinds: []string{"Pod"}},
	})
	require.NoError(t, err)
	require.Len(t, result.Events, 1)

	analysisStore := embeddedstore.NewAnalysisStore(p)
	resource, err := analysisStore.GetResource(context.Background(), "pod-uid")
	require.NoError(t, err)
	require.NotNil(t, resource)
	require.Equal(t, "Pod", resource.Kind)
	require.Equal(t, "pod-uid", resource.UID)
}

func TestProjection_BuildProjectionSnapshotMatchesLiveApply(t *testing.T) {
	events := []models.Event{
		{
			ID:        "pod-create",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "pod-1",
				Namespace: "default",
				UID:       "pod-uid",
			},
		},
		{
			ID:        "pod-delete",
			Timestamp: 30,
			Type:      models.EventTypeDelete,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "pod-1",
				Namespace: "default",
				UID:       "pod-uid",
			},
		},
	}

	live := embeddedstore.NewProjection()
	for i := range events {
		require.NoError(t, live.Apply(events[i]))
	}

	snap, err := embeddedstore.BuildProjection(events)
	require.NoError(t, err)

	qeLive := embeddedstore.NewQueryExecutor(live)
	qeSnap := embeddedstore.NewQueryExecutor(snap)

	query := &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   40,
		Filters:        models.QueryFilters{Kinds: []string{"Pod"}},
	}

	liveResult, err := qeLive.Execute(context.Background(), query)
	require.NoError(t, err)
	snapResult, err := qeSnap.Execute(context.Background(), query)
	require.NoError(t, err)

	require.Equal(t, liveResult.Events, snapResult.Events)
}

func TestProjection_BuildProjectionSupportsFallbackExport(t *testing.T) {
	events := []models.Event{
		{
			ID:        "pod-create",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "pod-1",
				Namespace: "default",
				UID:       "pod-uid",
			},
		},
		{
			ID:        "pod-update",
			Timestamp: 20,
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "pod-1",
				Namespace: "default",
				UID:       "pod-uid",
			},
		},
	}

	projection, err := embeddedstore.BuildProjection(events)
	require.NoError(t, err)

	exported, err := embeddedstore.NewQueryExecutor(projection).ExportTimeRange(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   30,
		Filters:        models.QueryFilters{Kinds: []string{"Pod"}},
	})
	require.NoError(t, err)
	require.Equal(t, events, exported)
}

func TestProjection_BuildProjectionSupportsFallbackDistinctMetadata(t *testing.T) {
	events := []models.Event{
		{
			ID:        "pod-create",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "pod-1",
				Namespace: "default",
				UID:       "pod-uid",
			},
		},
		{
			ID:        "deploy-create",
			Timestamp: 20,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Group:     "apps",
				Version:   "v1",
				Kind:      "Deployment",
				Name:      "deploy-1",
				Namespace: "apps",
				UID:       "deploy-uid",
			},
		},
	}

	projection, err := embeddedstore.BuildProjection(events)
	require.NoError(t, err)

	namespaces, kinds, minTime, maxTime, err := embeddedstore.NewQueryExecutor(projection).QueryDistinctMetadata(context.Background(), 0, 30)
	require.NoError(t, err)
	require.Equal(t, []string{"apps", "default"}, namespaces)
	require.Equal(t, []string{"Deployment", "Pod"}, kinds)
	require.Equal(t, int64(10), minTime)
	require.Equal(t, int64(20), maxTime)
}
