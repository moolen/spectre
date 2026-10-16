package embeddedstore

import (
	"fmt"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestRecordPrefilter_AgreesWithQueryFilters(t *testing.T) {
	var events []models.Event
	for i, kind := range []string{"Pod", "Event", "Deployment"} {
		for j, namespace := range []string{"", "ns-a", "ns-b"} {
			for k, eventType := range []models.EventType{models.EventTypeCreate, models.EventType("CUSTOM")} {
				group := ""
				if kind == "Deployment" {
					group = "apps"
				}
				events = append(events, models.Event{
					ID:        fmt.Sprintf("e-%d-%d-%d", i, j, k),
					Timestamp: int64(100*i + 10*j + k),
					Type:      eventType,
					Resource:  models.ResourceMetadata{Group: group, Version: "v1", Kind: kind, Namespace: namespace, Name: "n", UID: "u"},
					Data:      []byte(`{"a":1}`),
				})
			}
		}
	}
	filterSets := []models.QueryFilters{
		{},
		{Kinds: []string{"Event"}},
		{Kind: "Pod", Namespace: "ns-a"},
		{Kinds: []string{"Pod", "Deployment"}, Namespaces: []string{"ns-a", ""}},
		{Group: "apps"},
		{Version: "v2"},
	}
	for _, magic := range []byte{segmentRecordMagic, tailRecordMagic} {
		for i := range events {
			payload := appendEventRecord(nil, magic, 7, &events[i], true)
			key, ok := peekEventRecordKey(payload)
			require.True(t, ok)
			require.Equal(t, events[i].Timestamp, key.timestamp)
			for _, filters := range filterSets {
				filters := filters
				require.Equal(t, filters.Matches(events[i].Resource), newRecordPrefilter(filters).matches(&key), "%+v %+v", filters, events[i].Resource)
			}
		}
	}
	_, ok := peekEventRecordKey([]byte(`{"id":"legacy"}`))
	require.False(t, ok)
}
