package embeddedstore

import (
	"context"
	"strconv"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestTailJournal_RotateAfterCheckpointPublish(t *testing.T) {
	dir := t.TempDir()
	journal, err := openTailJournal(dir, TailJournalMeta{ID: "tail-a", BaseHighWaterMark: 10})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, journal.Close())
	})

	meta, err := journal.AppendBatch(context.Background(), 11, sampleTailEvents(3))
	require.NoError(t, err)
	require.Equal(t, uint64(13), meta.LastHighWaterMark)
	require.Equal(t, 3, meta.EventCount)

	nextMeta, err := journal.Rotate(13)
	require.NoError(t, err)
	require.Equal(t, uint64(13), nextMeta.BaseHighWaterMark)
	require.Zero(t, nextMeta.EventCount)
}

func sampleTailEvents(count int) []models.Event {
	events := make([]models.Event, 0, count)
	for i := 0; i < count; i++ {
		events = append(events, newTestEvent("tail-event-"+strconv.Itoa(i+1), int64(i+1)))
	}
	return events
}
