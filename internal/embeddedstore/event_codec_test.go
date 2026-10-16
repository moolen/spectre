package embeddedstore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestEventCodec_RoundTripsAllFields(t *testing.T) {
	event := models.Event{
		ID:        "evt-1",
		Timestamp: 1_700_000_000_123,
		Type:      models.EventTypeUpdate,
		Resource: models.ResourceMetadata{
			Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "ns", Name: "web",
			UID: "uid-1", InvolvedObjectUID: "uid-0",
		},
		Data:           json.RawMessage(`{"kind":"Deployment","spec":{"replicas":3},"pad":"` + strings.Repeat("x", 512) + `"}`),
		DataSize:       600,
		CompressedSize: 120,
		PreExisting:    true,
	}

	for _, compress := range []bool{false, true} {
		payload := appendEventRecord(nil, segmentRecordMagic, 0, &event, compress)
		decoded, err := decodeEventPayload(payload)
		require.NoError(t, err)
		require.Equal(t, event, decoded)
	}

	tailPayload := appendEventRecord(nil, tailRecordMagic, 42, &event, false)
	record, err := decodeTailJournalRecord(tailPayload)
	require.NoError(t, err)
	require.Equal(t, uint64(42), record.HighWaterMark)
	require.Equal(t, event, record.Event)
}

func TestEventCodec_RoundTripsDeleteWithoutDataAndCustomType(t *testing.T) {
	for _, event := range []models.Event{
		{ID: "d", Timestamp: -5, Type: models.EventTypeDelete, Resource: models.ResourceMetadata{UID: "u"}},
		{ID: "c", Timestamp: 7, Type: models.EventType("PATCH"), Resource: models.ResourceMetadata{UID: "u"}, Data: json.RawMessage(`{}`)},
	} {
		decoded, err := decodeEventPayload(appendEventRecord(nil, segmentRecordMagic, 0, &event, true))
		require.NoError(t, err)
		require.Equal(t, event, decoded)
	}
}

func TestEventCodec_DecodesLegacyJSONPayloads(t *testing.T) {
	event := models.Event{ID: "legacy", Timestamp: 3, Type: models.EventTypeCreate, Resource: models.ResourceMetadata{UID: "u", Kind: "Pod"}, Data: json.RawMessage(`{"a":1}`)}
	payload, err := json.Marshal(event)
	require.NoError(t, err)
	decoded, err := decodeEventPayload(payload)
	require.NoError(t, err)
	require.Equal(t, event, decoded)

	tailPayload, err := json.Marshal(tailJournalRecord{HighWaterMark: 9, Event: event})
	require.NoError(t, err)
	record, err := decodeTailJournalRecord(tailPayload)
	require.NoError(t, err)
	require.Equal(t, uint64(9), record.HighWaterMark)
	require.Equal(t, event, record.Event)
}

func TestEventCodec_FilterSkipsDataDecode(t *testing.T) {
	event := models.Event{ID: "e", Type: models.EventTypeCreate, Resource: models.ResourceMetadata{Kind: "Pod", Namespace: "a"}, Data: json.RawMessage(`{"x":1}`)}
	payload := appendEventRecord(nil, segmentRecordMagic, 0, &event, true)

	_, kept, err := decodeEventPayloadIf(payload, func(e *models.Event) bool { return e.Resource.Namespace == "b" })
	require.NoError(t, err)
	require.False(t, kept)

	decoded, kept, err := decodeEventPayloadIf(payload, func(e *models.Event) bool { return e.Resource.Namespace == "a" })
	require.NoError(t, err)
	require.True(t, kept)
	require.Equal(t, event, decoded)
}

func TestEventCodec_RejectsTruncatedRecords(t *testing.T) {
	event := models.Event{ID: "e", Resource: models.ResourceMetadata{UID: "u"}, Data: json.RawMessage(`{"x":1}`)}
	payload := appendEventRecord(nil, segmentRecordMagic, 0, &event, false)
	for cut := 1; cut < len(payload); cut++ {
		_, err := decodeEventPayload(payload[:cut])
		require.Error(t, err, "cut at %d", cut)
	}
}

func TestTailJournal_ReplaysLegacyJSONRecordsAfterUpgrade(t *testing.T) {
	root := t.TempDir()
	tail, err := openTailJournal(root, TailJournalMeta{})
	require.NoError(t, err)

	legacy := models.Event{ID: "a", Timestamp: 1, Type: models.EventTypeCreate, Resource: models.ResourceMetadata{UID: "u", Kind: "Pod"}, Data: json.RawMessage(`{"a":1}`)}
	payload, err := json.Marshal(tailJournalRecord{HighWaterMark: 1, Event: legacy})
	require.NoError(t, err)
	require.NoError(t, tail.journal.appendPayloadBatch(context.Background(), [][]byte{payload}))
	tail.meta.LastHighWaterMark = 1

	current := legacy
	current.ID = "b"
	current.Timestamp = 2
	_, err = tail.AppendBatch(context.Background(), 2, []models.Event{current})
	require.NoError(t, err)

	var replayed []models.Event
	require.NoError(t, tail.ReplaySince(context.Background(), 0, func(event models.Event, _ uint64) error {
		replayed = append(replayed, event)
		return nil
	}))
	require.Equal(t, []models.Event{legacy, current}, replayed)
	require.NoError(t, tail.Close())
}
