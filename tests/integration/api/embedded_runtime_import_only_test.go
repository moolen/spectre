package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	spectreapi "github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/apiserver"
	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/moolen/spectre/internal/importexport"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func newEmbeddedRuntimeServer(t *testing.T, backend *embeddedstore.Backend) *apiserver.Server {
	t.Helper()

	return apiserver.NewWithStorageGraphAndPipeline(
		0,
		backend.QueryExecutor(),
		nil,
		spectreapi.TimelineQuerySourceStorage,
		nil,
		nil,
		backend.AnalysisStore(),
		backend,
		backend,
		nil,
		time.Minute,
		apiserver.NamespaceGraphCacheConfig{},
		"",
		nil,
		nil,
	)
}

func queryEmbeddedTimeline(t *testing.T, server *apiserver.Server, start, end int64) models.SearchResponse {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/v1/timeline", http.NoBody)
	q := req.URL.Query()
	q.Set("start", strconv.FormatInt(start, 10))
	q.Set("end", strconv.FormatInt(end, 10))
	req.URL.RawQuery = q.Encode()

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var response models.SearchResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func findResource(resources []models.Resource, kind, name string) *models.Resource {
	for i := range resources {
		if resources[i].Kind == kind && resources[i].Name == name {
			return &resources[i]
		}
	}
	return nil
}

func TestEmbeddedRuntimeImportOnlyServesImportedData(t *testing.T) {
	events, err := importexport.Import(importexport.FromReader(strings.NewReader(embeddedTimelineFixture)))
	require.NoError(t, err)

	backend, err := embeddedstore.Open(embeddedstore.Config{DataDir: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = backend.Close()
	})

	require.NoError(t, backend.ProcessBatch(context.Background(), events))

	server := newEmbeddedRuntimeServer(t, backend)
	response := queryEmbeddedTimeline(t, server, 1700000000, 1700000010)

	target := findResource(response.Resources, "ConfigMap", "demo-config")
	require.NotNil(t, target)
	require.NotEmpty(t, target.Events)
	require.Equal(t, "Created", target.Events[0].Reason)
	require.True(t, backend.IsReady())
}
