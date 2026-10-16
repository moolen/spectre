package embeddedstore

import (
	"fmt"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func thinningTestRecord(count int, statusAt func(int) string, configChangeAt func(int) bool) *resourceRecord {
	meta := &versionMeta{UID: "node-1", Kind: "Node", Version: "v1", Name: "node-1"}
	record := &resourceRecord{uid: "node-1"}
	for i := 0; i < count; i++ {
		eventType := models.EventTypeUpdate
		if i == 0 {
			eventType = models.EventTypeCreate
		}
		version := resourceVersion{
			timestamp:  int64(i + 1),
			meta:       meta,
			typeCode:   versionEventTypes.code(string(eventType)),
			statusCode: versionStatuses.code(statusAt(i)),
		}
		if configChangeAt(i) {
			version.flags |= versionFlagConfigChanged
		}
		version.setEventID(fmt.Sprintf("v-%d", i))
		version.setInlinePayload([]byte(fmt.Sprintf(`{"v":%d}`, i)))
		record.versions = append(record.versions, version)
	}
	return record
}

func TestResourceRecordTimelineEvents_ThinsLongStatusRunsOnly(t *testing.T) {
	statusAt := func(i int) string {
		if i >= 150 && i < 160 {
			return "NotReady"
		}
		return "Ready"
	}
	configChangeAt := func(i int) bool { return i == 50 || i == 51 }
	record := thinningTestRecord(400, statusAt, configChangeAt)

	ids := func(events []models.Event) []string {
		out := make([]string, 0, len(events))
		for _, event := range events {
			out = append(out, event.ID)
		}
		return out
	}

	// Within the budget nothing is left out.
	require.Len(t, resourceRecordTimelineEvents(record, 1, 200, 256), 200)
	require.Len(t, resourceRecordTimelineEvents(record, 1, 400, 0), 400)

	// Over the budget: create, config changes, status transitions and the last version.
	thinned := resourceRecordTimelineEvents(record, 1, 400, 256)
	require.Equal(t, []string{"v-0", "v-50", "v-51", "v-150", "v-160", "v-399"}, ids(thinned))
	require.JSONEq(t, `{"v":150}`, string(thinned[3].Data))

	// The version current at the window start leads, and the first in-window version is
	// compared against it.
	thinned = resourceRecordTimelineEvents(record, 101, 400, 256)
	require.Equal(t, []string{"v-99", "v-150", "v-160", "v-399"}, ids(thinned))
	require.True(t, thinned[0].PreExisting)
}
