package embeddedstore

import (
	"fmt"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestAppendOrderedEvent_AppendsMonotonicInput(t *testing.T) {
	events := []models.Event{
		{ID: "1", Timestamp: 10},
		{ID: "2", Timestamp: 20},
	}

	got := appendOrderedEvent(events, models.Event{ID: "3", Timestamp: 30})
	require.Equal(t, []string{"1", "2", "3"}, eventIDs(got))
}

func TestAppendOrderedEvent_InsertsOutOfOrderInput(t *testing.T) {
	events := []models.Event{
		{ID: "1", Timestamp: 10},
		{ID: "3", Timestamp: 30},
	}

	got := appendOrderedEvent(events, models.Event{ID: "2", Timestamp: 20})
	require.Equal(t, []string{"1", "2", "3"}, eventIDs(got))
}

func TestHotStore_QueryRecentTimeRange(t *testing.T) {
	store := newHotStore(HotStoreConfig{MaxEvents: 10, MaxResourceVersions: 4}, nil)
	store.Append([]models.Event{
		{ID: "1", Timestamp: 10, Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod"}},
		{ID: "2", Timestamp: 20, Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod"}},
	})

	got := store.ScanTimeRange(0, 15)
	require.Len(t, got, 1)
	require.Equal(t, "1", got[0].ID)
}

func TestHotStore_BoundsResourceVersionHistory(t *testing.T) {
	store := newHotStore(HotStoreConfig{MaxEvents: 100, MaxResourceVersions: 2}, nil)
	for i := 0; i < 3; i++ {
		store.Append([]models.Event{{ID: fmt.Sprintf("%d", i), Timestamp: int64(i + 1), Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod"}}})
	}

	require.Len(t, store.RecentEventsByUID("pod-1"), 2)
}

func eventIDs(events []models.Event) []string {
	ids := make([]string, 0, len(events))
	for i := range events {
		ids = append(ids, events[i].ID)
	}
	return ids
}
