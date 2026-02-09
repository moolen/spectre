package grafana

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scrapeLinkerGraphMock implements graph.Client for scrape linker testing.
// Unlike the shared mockGraphClient, this one returns results in FIFO order.
type scrapeLinkerGraphMock struct {
	queries []graph.GraphQuery
	results []*graph.QueryResult
	err     error
}

func newScrapeLinkerGraphMock() *scrapeLinkerGraphMock {
	return &scrapeLinkerGraphMock{
		queries: make([]graph.GraphQuery, 0),
		results: make([]*graph.QueryResult, 0),
	}
}

func (m *scrapeLinkerGraphMock) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)
	if m.err != nil {
		return nil, m.err
	}
	if len(m.results) > 0 {
		result := m.results[0]
		m.results = m.results[1:]
		return result, nil
	}
	return &graph.QueryResult{
		Stats: graph.QueryStats{},
	}, nil
}

func (m *scrapeLinkerGraphMock) Connect(ctx context.Context) error { return nil }
func (m *scrapeLinkerGraphMock) Close() error                      { return nil }
func (m *scrapeLinkerGraphMock) Ping(ctx context.Context) error    { return nil }
func (m *scrapeLinkerGraphMock) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *scrapeLinkerGraphMock) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *scrapeLinkerGraphMock) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *scrapeLinkerGraphMock) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *scrapeLinkerGraphMock) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *scrapeLinkerGraphMock) InitializeSchema(ctx context.Context) error { return nil }
func (m *scrapeLinkerGraphMock) DeleteGraph(ctx context.Context) error      { return nil }
func (m *scrapeLinkerGraphMock) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *scrapeLinkerGraphMock) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *scrapeLinkerGraphMock) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

func TestScrapeTargetLinkerConfig_Defaults(t *testing.T) {
	config := DefaultScrapeTargetLinkerConfig()

	assert.Equal(t, 5*time.Minute, config.SyncInterval)
	assert.Equal(t, 100*time.Millisecond, config.RateLimitInterval)
	assert.Equal(t, 7*24*time.Hour, config.StaleTTL)
}

func TestScrapeTargetLinker_ResolveByAppLabel(t *testing.T) {
	logger := logging.GetLogger("test")

	testCases := []struct {
		name          string
		labels        map[string]string
		graphResult   *graph.QueryResult
		expectedUID   string
		expectedFound bool
	}{
		{
			name: "direct app.kubernetes.io/name match",
			labels: map[string]string{
				"namespace":              "default",
				"app_kubernetes_io_name": "nginx",
			},
			graphResult: &graph.QueryResult{
				Rows: [][]interface{}{
					{"uid-123", "Deployment", "nginx"},
				},
			},
			expectedUID:   "uid-123",
			expectedFound: true,
		},
		{
			name: "fallback to app label",
			labels: map[string]string{
				"namespace": "default",
				"app":       "redis",
			},
			graphResult: &graph.QueryResult{
				Rows: [][]interface{}{
					{"uid-456", "StatefulSet", "redis"},
				},
			},
			expectedUID:   "uid-456",
			expectedFound: true,
		},
		{
			name: "no matching labels",
			labels: map[string]string{
				"namespace": "default",
			},
			graphResult: &graph.QueryResult{
				Rows: [][]interface{}{},
			},
			expectedFound: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockGraph := newScrapeLinkerGraphMock()
			mockGraph.results = []*graph.QueryResult{tc.graphResult}

			linker := &ScrapeTargetLinker{
				graphClient: mockGraph,
				logger:      logger,
				config:      DefaultScrapeTargetLinkerConfig(),
			}

			ri, err := linker.resolveByAppLabel(context.Background(), tc.labels["namespace"], tc.labels)

			require.NoError(t, err)
			if tc.expectedFound {
				require.NotNil(t, ri)
				assert.Equal(t, tc.expectedUID, ri.UID)
			} else {
				assert.Nil(t, ri)
			}
		})
	}
}

func TestScrapeTargetLinker_ResolvePodOwner(t *testing.T) {
	logger := logging.GetLogger("test")

	testCases := []struct {
		name          string
		namespace     string
		podName       string
		graphResult   *graph.QueryResult
		expectedUID   string
		expectedFound bool
	}{
		{
			name:      "found Deployment owner",
			namespace: "production",
			podName:   "nginx-abc123",
			graphResult: &graph.QueryResult{
				Rows: [][]interface{}{
					{"uid-789", "Deployment", "nginx"},
				},
			},
			expectedUID:   "uid-789",
			expectedFound: true,
		},
		{
			name:      "no owner found",
			namespace: "production",
			podName:   "standalone-pod",
			graphResult: &graph.QueryResult{
				Rows: [][]interface{}{},
			},
			expectedFound: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockGraph := newScrapeLinkerGraphMock()
			mockGraph.results = []*graph.QueryResult{tc.graphResult}

			linker := &ScrapeTargetLinker{
				graphClient: mockGraph,
				logger:      logger,
				config:      DefaultScrapeTargetLinkerConfig(),
			}

			ri, err := linker.resolvePodOwner(context.Background(), tc.namespace, tc.podName)

			require.NoError(t, err)
			if tc.expectedFound {
				require.NotNil(t, ri)
				assert.Equal(t, tc.expectedUID, ri.UID)
			} else {
				assert.Nil(t, ri)
			}
		})
	}
}

func TestScrapeTargetLinker_ResolveWorkload_Confidence(t *testing.T) {
	logger := logging.GetLogger("test")

	testCases := []struct {
		name               string
		target             ScrapeTarget
		graphResults       []*graph.QueryResult
		expectedConfidence float64
		expectedFound      bool
	}{
		{
			name: "direct label match has 1.0 confidence",
			target: ScrapeTarget{
				Labels: map[string]string{
					"namespace":              "default",
					"app_kubernetes_io_name": "nginx",
				},
				ScrapePool: "kubernetes-pods",
			},
			graphResults: []*graph.QueryResult{
				{Rows: [][]interface{}{{"uid-123", "Deployment", "nginx"}}},
			},
			expectedConfidence: 1.0,
			expectedFound:      true,
		},
		{
			name: "pod owner fallback has 0.8 confidence",
			target: ScrapeTarget{
				Labels: map[string]string{
					"namespace": "default",
					"pod":       "nginx-abc123",
					// No app labels, so resolveByAppLabel skips queries
				},
				ScrapePool: "kubernetes-pods",
			},
			graphResults: []*graph.QueryResult{
				// Only resolvePodOwner queries the graph
				{Rows: [][]interface{}{{"uid-456", "Deployment", "nginx"}}},
			},
			expectedConfidence: 0.8,
			expectedFound:      true,
		},
		{
			name: "no resolution returns 0 confidence",
			target: ScrapeTarget{
				Labels: map[string]string{
					"namespace": "default",
				},
				ScrapePool: "kubernetes-pods",
			},
			graphResults: []*graph.QueryResult{
				{Rows: [][]interface{}{}},
			},
			expectedConfidence: 0,
			expectedFound:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockGraph := newScrapeLinkerGraphMock()
			mockGraph.results = tc.graphResults

			linker := &ScrapeTargetLinker{
				graphClient: mockGraph,
				logger:      logger,
				config:      DefaultScrapeTargetLinkerConfig(),
			}

			ri, confidence, err := linker.resolveWorkload(context.Background(), tc.target)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedConfidence, confidence)
			if tc.expectedFound {
				require.NotNil(t, ri)
			} else {
				assert.Nil(t, ri)
			}
		})
	}
}

func TestScrapeTargetLinker_Status(t *testing.T) {
	logger := logging.GetLogger("test")
	mockGraph := newScrapeLinkerGraphMock()

	config := DefaultScrapeTargetLinkerConfig()
	linker := NewScrapeTargetLinker(nil, mockGraph, "test-integration", logger, config)

	// Initial status should be empty
	status := linker.Status()
	assert.True(t, status.LastSyncTime.IsZero())
	assert.Equal(t, 0, status.LinksCreated)
	assert.Equal(t, 0, status.LinksConfirmed)
	assert.Equal(t, 0, status.LinksStale)
	assert.Equal(t, 0, status.LinksDeleted)
	assert.Equal(t, "", status.LastError)
	assert.False(t, status.InProgress)

	// Simulate update
	linker.updateSyncStatus(5, 10, 2, 1, nil)

	status = linker.Status()
	assert.False(t, status.LastSyncTime.IsZero())
	assert.Equal(t, 5, status.LinksCreated)
	assert.Equal(t, 10, status.LinksConfirmed)
	assert.Equal(t, 2, status.LinksStale)
	assert.Equal(t, 1, status.LinksDeleted)
}

func TestScrapeTargetLinker_LabelKeyMapping(t *testing.T) {
	// Test that sanitized Prometheus labels are mapped back to K8s labels
	testCases := []struct {
		prometheusKey  string
		expectedK8sKey string
	}{
		{"app_kubernetes_io_name", "app.kubernetes.io/name"},
		{"app_kubernetes_io_instance", "app.kubernetes.io/instance"},
		{"app", "app"},
	}

	for _, tc := range testCases {
		t.Run(tc.prometheusKey, func(t *testing.T) {
			logger := logging.GetLogger("test")
			mockGraph := newScrapeLinkerGraphMock()
			mockGraph.results = []*graph.QueryResult{{Rows: [][]interface{}{}}}

			linker := &ScrapeTargetLinker{
				graphClient: mockGraph,
				logger:      logger,
				config:      DefaultScrapeTargetLinkerConfig(),
			}

			// Call findWorkloadByLabel which does the mapping
			_, _ = linker.findWorkloadByLabel(context.Background(), "default", tc.prometheusKey, "test")

			// Verify the query used the correct K8s label key
			require.Len(t, mockGraph.queries, 1)
			params := mockGraph.queries[0].Parameters
			assert.Equal(t, tc.expectedK8sKey, params["labelKey"])
		})
	}
}

func TestScrapeTargetLinkerStatus_ErrorTracking(t *testing.T) {
	logger := logging.GetLogger("test")
	mockGraph := newScrapeLinkerGraphMock()

	config := DefaultScrapeTargetLinkerConfig()
	linker := NewScrapeTargetLinker(nil, mockGraph, "test-integration", logger, config)

	// Set an error
	testErr := assert.AnError
	linker.setLastError(testErr)

	status := linker.Status()
	assert.Equal(t, testErr.Error(), status.LastError)

	// Clear error via update
	linker.updateSyncStatus(0, 0, 0, 0, nil)

	status = linker.Status()
	assert.Equal(t, "", status.LastError)
}

func TestResourceIdentityRef(t *testing.T) {
	ref := ResourceIdentityRef{
		UID:       "test-uid",
		Kind:      "Deployment",
		Name:      "nginx",
		Namespace: "default",
	}

	assert.Equal(t, "test-uid", ref.UID)
	assert.Equal(t, "Deployment", ref.Kind)
	assert.Equal(t, "nginx", ref.Name)
	assert.Equal(t, "default", ref.Namespace)
}
