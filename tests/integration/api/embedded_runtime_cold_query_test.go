package api

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	handlers "github.com/moolen/spectre/internal/api/handlers"
	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedRuntimeExportServesFlushedColdRawEvents(t *testing.T) {
	engine, err := embeddedstore.OpenEngine(embeddedstore.EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           100,
		HotMaxResourceVersions: 4,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "pod-1",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
			Data: []byte(`{"kind":"Pod"}`),
		},
		{
			ID:        "event-1",
			Timestamp: 11,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:           "v1",
				UID:               "event-1",
				Namespace:         "default",
				Kind:              "Event",
				Name:              "event-1",
				InvolvedObjectUID: "pod-1",
			},
			Data: []byte(`{"reason":"Scheduled","message":"scheduled"}`),
		},
	}))
	require.NoError(t, engine.Flush(context.Background()))

	req := httptest.NewRequest(http.MethodGet, "/v1/storage/export", http.NoBody)
	q := req.URL.Query()
	q.Set("from", strconv.FormatInt(0, 10))
	q.Set("to", strconv.FormatInt(20, 10))
	req.URL.RawQuery = q.Encode()

	rec := httptest.NewRecorder()
	handlers.NewExportHandler(engine.QueryExecutor(), logging.GetLogger("test.export")).Handle(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	gzipReader, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	defer gzipReader.Close()

	var payload struct {
		Events []models.Event `json:"events"`
	}
	require.NoError(t, json.NewDecoder(gzipReader).Decode(&payload))
	require.Equal(t, []string{"pod-1", "event-1"}, eventIDs(payload.Events))
}

func eventIDs(events []models.Event) []string {
	ids := make([]string, 0, len(events))
	for i := range events {
		ids = append(ids, events[i].ID)
	}
	return ids
}
