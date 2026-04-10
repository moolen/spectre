package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	namespacegraph "github.com/moolen/spectre/internal/analysis/namespace_graph"
	analysisembedded "github.com/moolen/spectre/internal/analysis/store/embedded"
	"github.com/moolen/spectre/internal/apiserver"
	"github.com/moolen/spectre/internal/embedded"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedNamespaceGraphHandler_IngressFixture(t *testing.T) {
	server := newEmbeddedFixtureServer(t, "testrootcause-ingress-samenamespace-endp.jsonl")
	timestamp, podUID, err := ExtractTimestampAndPodUIDFromFile(fixturePath(t, "testrootcause-ingress-samenamespace-endp.jsonl"))
	require.NoError(t, err)
	require.NotEmpty(t, podUID)

	events, err := LoadAuditLog(fixturePath(t, "testrootcause-ingress-samenamespace-endp.jsonl"))
	require.NoError(t, err)
	namespace := namespaceForUID(events, podUID)
	require.NotEmpty(t, namespace)

	req := httptest.NewRequest(http.MethodGet, "/v1/namespace-graph", http.NoBody)
	q := req.URL.Query()
	q.Set("namespace", namespace)
	q.Set("timestamp", time.Unix(0, timestamp).Format(time.RFC3339Nano))
	q.Set("includeAnomalies", "false")
	q.Set("maxDepth", "5")
	q.Set("limit", "200")
	req.URL.RawQuery = q.Encode()

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var response namespacegraph.NamespaceGraphResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))

	nodeKinds := make(map[string]bool)
	nodeByUID := make(map[string]string)
	for _, node := range response.Graph.Nodes {
		nodeKinds[node.Kind] = true
		nodeByUID[node.UID] = node.Kind
	}
	require.True(t, nodeKinds["Ingress"])
	require.True(t, nodeKinds["Service"])
	require.True(t, nodeKinds["Pod"])

	foundServiceSelectsPod := false
	foundIngressRefsService := false
	for _, edge := range response.Graph.Edges {
		if nodeByUID[edge.Source] == "Service" && edge.RelationshipType == "SELECTS" && nodeByUID[edge.Target] == "Pod" {
			foundServiceSelectsPod = true
		}
		if nodeByUID[edge.Source] == "Ingress" && edge.RelationshipType == "REFERENCES_SPEC" && nodeByUID[edge.Target] == "Service" {
			foundIngressRefsService = true
		}
	}
	require.True(t, foundServiceSelectsPod)
	require.True(t, foundIngressRefsService)
}

func newEmbeddedFixtureServer(t *testing.T, fixture string) *apiserver.Server {
	t.Helper()

	events, err := LoadAuditLog(fixturePath(t, fixture))
	require.NoError(t, err)

	executor, err := embedded.NewQueryExecutor(events)
	require.NoError(t, err)

	analysisStore, err := analysisembedded.New(events)
	require.NoError(t, err)

	return apiserver.NewWithStorageGraphAndPipeline(
		0,
		executor,
		analysisStore,
		nil,
		&apiserver.NoOpReadinessChecker{},
		nil,
		time.Minute,
		apiserver.NamespaceGraphCacheConfig{},
	)
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(testFile), "..", "fixtures", name)
}

func extractFixtureContext(events []models.Event) (int64, string, string) {
	var lastTimestamp int64
	var podUID string
	var namespace string
	for _, event := range events {
		if event.Timestamp > lastTimestamp {
			lastTimestamp = event.Timestamp
		}
		if event.Resource.Kind == "Pod" {
			podUID = event.Resource.UID
			namespace = event.Resource.Namespace
		}
	}
	return lastTimestamp, podUID, namespace
}

func namespaceForUID(events []models.Event, uid string) string {
	for _, event := range events {
		if event.Resource.UID == uid {
			return event.Resource.Namespace
		}
	}
	return ""
}
