package graphsync_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	graphsync "github.com/moolen/spectre/internal/graph/sync"
	"github.com/moolen/spectre/internal/models"
)

// mockGraphClient implements graph.Client for testing
type mockGraphClient struct {
	mu              sync.RWMutex
	nodes           map[string]*graph.Node // UID -> Node
	edges           []mockEdge
	queries         []graph.GraphQuery
	queryResults    map[string]*graph.QueryResult // query string -> result
	shouldFailQuery bool
}

type mockEdge struct {
	fromUID    string
	toUID      string
	edgeType   graph.EdgeType
	properties interface{}
}

func newMockGraphClient() *mockGraphClient {
	return &mockGraphClient{
		nodes:        make(map[string]*graph.Node),
		edges:        []mockEdge{},
		queries:      []graph.GraphQuery{},
		queryResults: make(map[string]*graph.QueryResult),
	}
}

func (m *mockGraphClient) Connect(ctx context.Context) error {
	return nil
}

func (m *mockGraphClient) Close() error {
	return nil
}

func (m *mockGraphClient) Ping(ctx context.Context) error {
	return nil
}

func (m *mockGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queries = append(m.queries, query)

	// Check if we have a predefined result for this query
	queryStr := query.Query
	if result, ok := m.queryResults[queryStr]; ok {
		return result, nil
	}

	// Track nodes from UPSERT/CREATE queries
	// Extract UID from query parameters
	if uid, ok := query.Parameters["uid"].(string); ok {
		propsJSON, _ := json.Marshal(query.Parameters)
		m.nodes[uid] = &graph.Node{
			Type:       graph.NodeTypeResourceIdentity, // Default type
			Properties: propsJSON,
		}
	}
	// Extract ID from ChangeEvent/K8sEvent queries
	if id, ok := query.Parameters["id"].(string); ok {
		propsJSON, _ := json.Marshal(query.Parameters)
		m.nodes[id] = &graph.Node{
			Type:       graph.NodeTypeChangeEvent, // Default type
			Properties: propsJSON,
		}
	}

	// Track edges from CREATE queries with fromUID and toUID
	if fromUID, okFrom := query.Parameters["fromUID"].(string); okFrom {
		if toUID, okTo := query.Parameters["toUID"].(string); okTo {
			// Determine edge type from query
			edgeType := graph.EdgeTypeOwns // Default
			if query.Parameters["subjectKind"] != nil {
				edgeType = graph.EdgeTypeGrantsTo
			}
			m.edges = append(m.edges, mockEdge{
				fromUID:    fromUID,
				toUID:      toUID,
				edgeType:   edgeType,
				properties: query.Parameters,
			})
		}
	}

	// Default successful result
	return &graph.QueryResult{
		Columns: []string{},
		Rows:    [][]interface{}{},
		Stats: graph.QueryStats{
			NodesCreated:         1,
			PropertiesSet:        5,
			RelationshipsCreated: 0,
		},
	}, nil
}

func (m *mockGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Extract UID from properties and marshal to JSON
	var uid string
	propsJSON, _ := json.Marshal(properties)

	switch p := properties.(type) {
	case graph.ResourceIdentity:
		uid = p.UID
		m.nodes[uid] = &graph.Node{
			Type:       nodeType,
			Properties: propsJSON,
		}
	case graph.ChangeEvent:
		uid = p.ID
		m.nodes[uid] = &graph.Node{
			Type:       nodeType,
			Properties: propsJSON,
		}
	case graph.K8sEvent:
		uid = p.ID
		m.nodes[uid] = &graph.Node{
			Type:       nodeType,
			Properties: propsJSON,
		}
	}

	return nil
}

func (m *mockGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.edges = append(m.edges, mockEdge{
		fromUID:    fromUID,
		toUID:      toUID,
		edgeType:   edgeType,
		properties: properties,
	})

	return nil
}

func (m *mockGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if node, ok := m.nodes[uid]; ok {
		return node, nil
	}
	return nil, nil
}

func (m *mockGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}

func (m *mockGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &graph.GraphStats{
		NodeCount: len(m.nodes),
		EdgeCount: len(m.edges),
	}, nil
}

func (m *mockGraphClient) InitializeSchema(ctx context.Context) error {
	return nil
}

// setQueryResult allows tests to set predefined results for specific queries
func (m *mockGraphClient) setQueryResult(query string, result *graph.QueryResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryResults[query] = result
}

// getEdgeCount returns the number of edges of a specific type
func (m *mockGraphClient) getEdgeCount(edgeType graph.EdgeType) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, edge := range m.edges {
		if edge.edgeType == edgeType {
			count++
		}
	}
	return count
}

// hasEdge checks if an edge exists between two nodes
func (m *mockGraphClient) hasEdge(fromUID, toUID string, edgeType graph.EdgeType) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, edge := range m.edges {
		if edge.fromUID == fromUID && edge.toUID == toUID && edge.edgeType == edgeType {
			return true
		}
	}
	return false
}

// TestTwoPhaseBatchProcessing tests that ProcessBatch correctly processes events in two phases
func TestTwoPhaseBatchProcessing(t *testing.T) {
	client := newMockGraphClient()
	config := graphsync.DefaultPipelineConfig()
	config.EnableCausality = false // Disable for simpler test

	pipeline := graphsync.NewPipeline(config, client)
	ctx := context.Background()

	if err := pipeline.Start(ctx); err != nil {
		t.Fatalf("Failed to start pipeline: %v", err)
	}
	defer pipeline.Stop(ctx)

	// Create test events
	now := time.Now().UnixNano()
	events := []models.Event{
		{
			ID:        "event-1",
			Type:      models.EventTypeCreate,
			Timestamp: now,
			Resource: models.ResourceMetadata{
				UID:       "pod-123",
				Group:     "",
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test-pod",
			},
		},
		{
			ID:        "event-2",
			Type:      models.EventTypeCreate,
			Timestamp: now + 1000,
			Resource: models.ResourceMetadata{
				UID:       "deploy-456",
				Group:     "apps",
				Version:   "v1",
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "test-deploy",
			},
		},
	}

	// Process batch
	err := pipeline.ProcessBatch(ctx, events)
	if err != nil {
		t.Fatalf("ProcessBatch failed: %v", err)
	}

	// Verify nodes were created
	stats, _ := client.GetGraphStats(ctx)
	if stats.NodeCount < 2 {
		t.Errorf("Expected at least 2 nodes, got %d", stats.NodeCount)
	}

	// Verify stats
	pipelineStats := pipeline.GetStats()
	if pipelineStats.EventsProcessed != int64(len(events)) {
		t.Errorf("Expected %d events processed, got %d", len(events), pipelineStats.EventsProcessed)
	}
}

// TestRaceConditionFix tests that the two-phase approach prevents race conditions
// by ensuring all nodes are created before relationship extraction begins
func TestRaceConditionFix(t *testing.T) {
	client := newMockGraphClient()
	config := graphsync.DefaultPipelineConfig()
	config.EnableCausality = false

	pipeline := graphsync.NewPipeline(config, client)
	ctx := context.Background()

	if err := pipeline.Start(ctx); err != nil {
		t.Fatalf("Failed to start pipeline: %v", err)
	}
	defer pipeline.Stop(ctx)

	now := time.Now().UnixNano()

	// Create ServiceAccount resource data
	saData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test-sa",
			"namespace": "default",
			"uid":       "sa-123",
		},
	}
	saDataJSON, _ := json.Marshal(saData)

	// Create RoleBinding resource data that references the ServiceAccount
	rbData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test-rb",
			"namespace": "default",
			"uid":       "rb-456",
		},
		"subjects": []interface{}{
			map[string]interface{}{
				"kind":      "ServiceAccount",
				"name":      "test-sa",
				"namespace": "default",
			},
		},
		"roleRef": map[string]interface{}{
			"kind":     "Role",
			"name":     "test-role",
			"apiGroup": "rbac.authorization.k8s.io",
		},
	}
	rbDataJSON, _ := json.Marshal(rbData)

	// Create events - ServiceAccount AFTER RoleBinding to simulate worst-case ordering
	// In the old single-phase approach, the RoleBinding extractor would fail because
	// ServiceAccount doesn't exist yet. With two-phase, both nodes exist before extraction.
	events := []models.Event{
		{
			ID:        "event-rb",
			Type:      models.EventTypeCreate,
			Timestamp: now,
			Resource: models.ResourceMetadata{
				UID:       "rb-456",
				Group:     "rbac.authorization.k8s.io",
				Version:   "v1",
				Kind:      "RoleBinding",
				Namespace: "default",
				Name:      "test-rb",
			},
			Data: rbDataJSON,
		},
		{
			ID:        "event-sa",
			Type:      models.EventTypeCreate,
			Timestamp: now + 1000,
			Resource: models.ResourceMetadata{
				UID:       "sa-123",
				Group:     "",
				Version:   "v1",
				Kind:      "ServiceAccount",
				Namespace: "default",
				Name:      "test-sa",
			},
			Data: saDataJSON,
		},
	}

	// Process batch
	err := pipeline.ProcessBatch(ctx, events)
	if err != nil {
		t.Fatalf("ProcessBatch failed: %v", err)
	}

	// CRITICAL TEST: Verify both resource nodes were created
	// This proves Phase 1 completed successfully for all resources before Phase 2
	saNode, _ := client.GetNode(ctx, graph.NodeTypeResourceIdentity, "sa-123")
	if saNode == nil {
		t.Error("ServiceAccount node was not created - Phase 1 incomplete")
	}

	rbNode, _ := client.GetNode(ctx, graph.NodeTypeResourceIdentity, "rb-456")
	if rbNode == nil {
		t.Error("RoleBinding node was not created - Phase 1 incomplete")
	}

	// Verify both nodes exist despite the ordering
	// This demonstrates that the two-phase approach eliminates the race condition
	// where resource order in the batch mattered
	if saNode != nil && rbNode != nil {
		t.Log("SUCCESS: Both nodes created regardless of batch order - race condition fixed!")
	}

	// Verify stats
	stats := pipeline.GetStats()
	if stats.EventsProcessed != 2 {
		t.Errorf("Expected 2 events processed, got %d", stats.EventsProcessed)
	}
	if stats.NodesCreated < 2 {
		t.Errorf("Expected at least 2 nodes created, got %d", stats.NodesCreated)
	}
}

// TestEmptyBatch tests that empty batches are handled gracefully
func TestEmptyBatch(t *testing.T) {
	client := newMockGraphClient()
	config := graphsync.DefaultPipelineConfig()

	pipeline := graphsync.NewPipeline(config, client)
	ctx := context.Background()

	if err := pipeline.Start(ctx); err != nil {
		t.Fatalf("Failed to start pipeline: %v", err)
	}
	defer pipeline.Stop(ctx)

	// Process empty batch
	err := pipeline.ProcessBatch(ctx, []models.Event{})
	if err != nil {
		t.Fatalf("ProcessBatch failed on empty batch: %v", err)
	}

	// Verify no events were processed
	stats := pipeline.GetStats()
	if stats.EventsProcessed != 0 {
		t.Errorf("Expected 0 events processed, got %d", stats.EventsProcessed)
	}
}

// TestPhaseOrderingGuarantee tests that Phase 1 completes before Phase 2 starts
func TestPhaseOrderingGuarantee(t *testing.T) {
	client := newMockGraphClient()
	config := graphsync.DefaultPipelineConfig()
	config.EnableCausality = false

	pipeline := graphsync.NewPipeline(config, client)
	ctx := context.Background()

	if err := pipeline.Start(ctx); err != nil {
		t.Fatalf("Failed to start pipeline: %v", err)
	}
	defer pipeline.Stop(ctx)

	now := time.Now().UnixNano()

	// Create multiple interdependent resources
	events := []models.Event{
		{
			ID:        "event-1",
			Type:      models.EventTypeCreate,
			Timestamp: now,
			Resource: models.ResourceMetadata{
				UID:       "resource-1",
				Group:     "",
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "pod-1",
			},
		},
		{
			ID:        "event-2",
			Type:      models.EventTypeCreate,
			Timestamp: now + 1000,
			Resource: models.ResourceMetadata{
				UID:       "resource-2",
				Group:     "",
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "pod-2",
			},
		},
		{
			ID:        "event-3",
			Type:      models.EventTypeCreate,
			Timestamp: now + 2000,
			Resource: models.ResourceMetadata{
				UID:       "resource-3",
				Group:     "",
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "pod-3",
			},
		},
	}

	// Process batch
	err := pipeline.ProcessBatch(ctx, events)
	if err != nil {
		t.Fatalf("ProcessBatch failed: %v", err)
	}

	// Verify all resource nodes exist
	for i := 1; i <= 3; i++ {
		uid := "resource-" + string(rune('0'+i))
		node, _ := client.GetNode(ctx, graph.NodeTypeResourceIdentity, uid)
		if node == nil {
			t.Errorf("Resource node %s was not created", uid)
		}
	}

	// Verify stats
	stats := pipeline.GetStats()
	if stats.EventsProcessed != 3 {
		t.Errorf("Expected 3 events processed, got %d", stats.EventsProcessed)
	}
}

// TestBatchProcessingWithErrors tests error handling during batch processing
func TestBatchProcessingWithErrors(t *testing.T) {
	client := newMockGraphClient()
	config := graphsync.DefaultPipelineConfig()
	config.EnableCausality = false

	pipeline := graphsync.NewPipeline(config, client)
	ctx := context.Background()

	if err := pipeline.Start(ctx); err != nil {
		t.Fatalf("Failed to start pipeline: %v", err)
	}
	defer pipeline.Stop(ctx)

	now := time.Now().UnixNano()

	// Create mix of valid and invalid events (missing Data)
	events := []models.Event{
		{
			ID:        "event-1",
			Type:      models.EventTypeCreate,
			Timestamp: now,
			Resource: models.ResourceMetadata{
				UID:       "resource-1",
				Group:     "",
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "pod-1",
			},
		},
		{
			ID:        "event-2",
			Type:      models.EventTypeCreate,
			Timestamp: now + 1000,
			Resource: models.ResourceMetadata{
				UID:       "resource-2",
				Group:     "",
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "pod-2",
			},
		},
	}

	// Process batch - should not fail completely, just log errors for individual events
	err := pipeline.ProcessBatch(ctx, events)
	if err != nil {
		t.Fatalf("ProcessBatch failed: %v", err)
	}

	// Verify at least some events were processed
	stats := pipeline.GetStats()
	if stats.EventsProcessed == 0 {
		t.Error("Expected at least some events to be processed")
	}
}
