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

	"github.com/moolen/spectre/internal/apiserver"
	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/moolen/spectre/internal/importexport"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

type embeddedRuntimeStorage interface {
	QueryExecutor() *embeddedstore.QueryExecutor
	AnalysisStore() *embeddedstore.Store
	IsReady() bool
}

func newEmbeddedRuntimeServer(t *testing.T, runtime embeddedRuntimeStorage) *apiserver.Server {
	t.Helper()

	return apiserver.NewWithStorageGraphAndPipeline(
		0,
		runtime.QueryExecutor(),
		runtime.AnalysisStore(),
		nil,
		runtime,
		nil,
		time.Minute,
		apiserver.NamespaceGraphCacheConfig{},
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

func requireRuntimeReady(t *testing.T, server *apiserver.Server, expected bool) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/ready", http.NoBody)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	expectedStatus := http.StatusServiceUnavailable
	if expected {
		expectedStatus = http.StatusOK
	}
	require.Equal(t, expectedStatus, recorder.Code, recorder.Body.String())

	var payload struct {
		Ready bool `json:"ready"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, expected, payload.Ready)
}

func requireImportRouteUnavailable(t *testing.T, server *apiserver.Server) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/v1/storage/import", strings.NewReader(`{"events":[]}`))
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
}

func findResource(resources []models.Resource, kind, name string) *models.Resource {
	for i := range resources {
		if resources[i].Kind == kind && resources[i].Name == name {
			return &resources[i]
		}
	}
	return nil
}

func TestEmbeddedRuntimeImportOnlyServesPersistedColdStateWithoutWatcher(t *testing.T) {
	dir := t.TempDir()

	events, err := importexport.Import(importexport.FromReader(strings.NewReader(embeddedTimelineFixture)))
	require.NoError(t, err)

	engine, err := embeddedstore.OpenEngine(embeddedstore.EngineConfig{
		DataDir:                dir,
		HotMaxEvents:           32,
		HotMaxResourceVersions: 8,
	})
	require.NoError(t, err)

	require.NoError(t, engine.ProcessBatch(context.Background(), events))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.Close())

	reopened, err := embeddedstore.OpenEngine(embeddedstore.EngineConfig{
		DataDir:                dir,
		HotMaxEvents:           32,
		HotMaxResourceVersions: 8,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = reopened.Close()
	})

	server := newEmbeddedRuntimeServer(t, reopened)
	requireRuntimeReady(t, server, true)
	requireImportRouteUnavailable(t, server)

	response := queryEmbeddedTimeline(t, server, 1700000000, 1700000010)

	target := findResource(response.Resources, "ConfigMap", "demo-config")
	require.NotNil(t, target)
	require.NotEmpty(t, target.Events)
	require.Equal(t, "Created", target.Events[0].Reason)
	require.True(t, reopened.IsReady())
}
