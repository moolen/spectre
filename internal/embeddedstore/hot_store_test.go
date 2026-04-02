package embeddedstore

import (
	"fmt"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestHotStore_QueryRecentTimeRange(t *testing.T) {
	store := newHotStore(HotStoreConfig{MaxEvents: 10, MaxResourceVersions: 4})
	store.Append([]models.Event{
		{ID: "1", Timestamp: 10, Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod"}},
		{ID: "2", Timestamp: 20, Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod"}},
	})

	got := store.ScanTimeRange(0, 15)
	require.Len(t, got, 1)
	require.Equal(t, "1", got[0].ID)
}

func TestHotStore_BoundsResourceVersionHistory(t *testing.T) {
	store := newHotStore(HotStoreConfig{MaxEvents: 100, MaxResourceVersions: 2})
	for i := 0; i < 3; i++ {
		store.Append([]models.Event{{ID: fmt.Sprintf("%d", i), Timestamp: int64(i + 1), Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod"}}})
	}

	require.Len(t, store.RecentEventsByUID("pod-1"), 2)
}
