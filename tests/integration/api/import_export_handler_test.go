package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	handlers "github.com/moolen/spectre/internal/api/handlers"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	graphfixtures "github.com/moolen/spectre/tests/integration/graph"
	"github.com/stretchr/testify/require"
)

func TestExportImportRoundTripWithPaginatedExport(t *testing.T) {
	sourceHarness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer sourceHarness.Cleanup(context.Background())

	destHarness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer destHarness.Cleanup(context.Background())

	ctx := context.Background()
	baseTime := time.Now().Add(-10 * time.Minute)

	namespaces := []string{"import-1", "import-2"}
	events := make([]models.Event, 0, 520)
	expectedByNamespace := map[string]int{}

	for i := 0; i < 520; i++ {
		ns := namespaces[i%len(namespaces)]
		expectedByNamespace[ns]++
		events = append(events, graphfixtures.CreateDeploymentEvent(
			uuid.New().String(),
			"import-deploy-"+uuid.New().String()[:8],
			ns,
			baseTime.Add(time.Duration(i)*time.Second),
			models.EventTypeCreate,
			1,
		))
	}

	err = sourceHarness.GetPipeline().ProcessBatch(ctx, events)
	require.NoError(t, err)

	exportReq := httptest.NewRequest(http.MethodGet, "/v1/storage/export?from="+
		int64ToString(baseTime.Add(-time.Minute).Unix())+"&to="+
		int64ToString(baseTime.Add(20*time.Minute).Unix()), nil)
	exportRec := httptest.NewRecorder()
	exportHandler := handlers.NewExportHandler(graph.NewQueryExecutor(sourceHarness.GetClient()), logging.GetLogger("test.export"))
	exportHandler.Handle(exportRec, exportReq)

	exportResp := exportRec.Result()
	defer exportResp.Body.Close()
	require.Equal(t, http.StatusOK, exportResp.StatusCode)

	gzipReader, err := gzip.NewReader(exportResp.Body)
	require.NoError(t, err)
	defer gzipReader.Close()

	var exportPayload map[string]any
	err = json.NewDecoder(gzipReader).Decode(&exportPayload)
	require.NoError(t, err)

	rawEvents, ok := exportPayload["events"].([]any)
	require.True(t, ok, "export payload must contain an events array")
	require.Len(t, rawEvents, len(events), "paginated export must preserve all events")

	payloadJSON, err := json.Marshal(map[string]any{"events": rawEvents})
	require.NoError(t, err)

	importReq := httptest.NewRequest(http.MethodPost, "/v1/storage/import?validate=true&overwrite=true", bytes.NewReader(payloadJSON))
	importReq.Header.Set("Content-Type", "application/vnd.spectre.events.v1+json")
	importRec := httptest.NewRecorder()
	importHandler := handlers.NewImportHandler(destHarness.GetPipeline(), logging.GetLogger("test.import"))
	importHandler.Handle(importRec, importReq)

	importResp := importRec.Result()
	defer importResp.Body.Close()
	require.Equal(t, http.StatusOK, importResp.StatusCode)

	queryExecutor := graph.NewQueryExecutor(destHarness.GetClient())
	for _, ns := range namespaces {
		result, _, queryErr := queryExecutor.ExecutePaginated(ctx, &models.QueryRequest{
			StartTimestamp: baseTime.Add(-time.Minute).Unix(),
			EndTimestamp:   baseTime.Add(20 * time.Minute).Unix(),
			Filters: models.QueryFilters{
				Namespace: ns,
				Kind:      "Deployment",
			},
		}, &models.PaginationRequest{PageSize: 500})
		require.NoError(t, queryErr)
		require.Equal(t, expectedByNamespace[ns], len(uniqueResourceUIDs(result.Events)), "namespace %s should retain all deployments after round-trip import", ns)
	}
}

func uniqueResourceUIDs(events []models.Event) map[string]struct{} {
	uids := make(map[string]struct{}, len(events))
	for _, event := range events {
		uids[event.Resource.UID] = struct{}{}
	}
	return uids
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}
