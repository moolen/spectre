package extractors

import (
	"context"

	"github.com/moolen/spectre/internal/graph"
)

// MockResourceLookup provides a mock implementation of ResourceLookup for testing
type MockResourceLookup struct {
	resources   map[string]*graph.ResourceIdentity
	queryResult *graph.QueryResult
	queryError  error
}

// NewMockResourceLookup creates a new mock resource lookup
func NewMockResourceLookup() *MockResourceLookup {
	return &MockResourceLookup{
		resources: make(map[string]*graph.ResourceIdentity),
	}
}

// AddResource adds a resource to the mock lookup
func (m *MockResourceLookup) AddResource(res *graph.ResourceIdentity) {
	// Index by UID
	m.resources[res.UID] = res

	// Also index by namespace/kind/name for FindResourceByNamespace
	key := res.Namespace + "/" + res.Kind + "/" + res.Name
	m.resources[key] = res
}

// SetQueryResult sets the result to return from QueryGraph
func (m *MockResourceLookup) SetQueryResult(result *graph.QueryResult) {
	m.queryResult = result
}

// SetQueryError sets an error to return from QueryGraph
func (m *MockResourceLookup) SetQueryError(err error) {
	m.queryError = err
}

// SetOwnershipResult is a helper to set up the mock to return whether an ownership exists
func (m *MockResourceLookup) SetOwnershipResult(hasOwnership bool) {
	m.queryResult = &graph.QueryResult{
		Columns: []string{"hasOwnership"},
		Rows:    [][]interface{}{{hasOwnership}},
	}
}

func (m *MockResourceLookup) FindResourceByUID(_ context.Context, uid string) (*graph.ResourceIdentity, error) {
	if res, ok := m.resources[uid]; ok {
		return res, nil
	}
	return nil, nil
}

func (m *MockResourceLookup) FindResourceByNamespace(_ context.Context, namespace, kind, name string) (*graph.ResourceIdentity, error) {
	key := namespace + "/" + kind + "/" + name
	if res, ok := m.resources[key]; ok {
		return res, nil
	}
	return nil, nil
}

func (m *MockResourceLookup) FindRecentEvents(_ context.Context, _ string, _ int64) ([]graph.ChangeEvent, error) {
	return nil, nil
}

func (m *MockResourceLookup) QueryGraph(_ context.Context, _ graph.GraphQuery) (*graph.QueryResult, error) {
	if m.queryError != nil {
		return nil, m.queryError
	}
	return m.queryResult, nil
}
