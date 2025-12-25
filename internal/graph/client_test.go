package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultClientConfig(t *testing.T) {
	config := DefaultClientConfig()

	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 6379, config.Port)
	assert.Equal(t, "spectre", config.GraphName)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 10, config.PoolSize)
	assert.NotZero(t, config.DialTimeout)
	assert.NotZero(t, config.ReadTimeout)
	assert.NotZero(t, config.WriteTimeout)
}

func TestEscapeCypherString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "single quote",
			input:    "it's working",
			expected: "it\\'s working",
		},
		{
			name:     "multiple quotes",
			input:    "it's a 'test'",
			expected: "it\\'s a \\'test\\'",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeCypherString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildPropertiesString(t *testing.T) {
	tests := []struct {
		name     string
		props    map[string]interface{}
		contains []string
	}{
		{
			name:     "empty properties",
			props:    map[string]interface{}{},
			contains: []string{},
		},
		{
			name: "string property",
			props: map[string]interface{}{
				"name": "test",
			},
			contains: []string{"name:", "'test'"},
		},
		{
			name: "boolean property",
			props: map[string]interface{}{
				"active": true,
			},
			contains: []string{"active:", "true"},
		},
		{
			name: "numeric property",
			props: map[string]interface{}{
				"count": 42,
			},
			contains: []string{"count:", "42"},
		},
		{
			name: "array property",
			props: map[string]interface{}{
				"tags": []string{"foo", "bar"},
			},
			contains: []string{"tags:", "'foo'", "'bar'"},
		},
		{
			name: "mixed properties",
			props: map[string]interface{}{
				"name":   "test",
				"count":  10,
				"active": true,
			},
			contains: []string{"name:", "count:", "active:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildPropertiesString(tt.props)
			for _, expected := range tt.contains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestReplaceCypherParameters(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		params   map[string]interface{}
		expected string
	}{
		{
			name:     "string parameter",
			query:    "MATCH (n {uid: $uid}) RETURN n",
			params:   map[string]interface{}{"uid": "pod-123"},
			expected: "MATCH (n {uid: 'pod-123'}) RETURN n",
		},
		{
			name:     "numeric parameter",
			query:    "WHERE n.count > $minCount",
			params:   map[string]interface{}{"minCount": 10},
			expected: "WHERE n.count > 10",
		},
		{
			name:     "boolean parameter",
			query:    "WHERE n.active = $active",
			params:   map[string]interface{}{"active": true},
			expected: "WHERE n.active = true",
		},
		{
			name:     "multiple parameters",
			query:    "MATCH (n {uid: $uid}) WHERE n.count > $count RETURN n",
			params:   map[string]interface{}{"uid": "pod-123", "count": 5},
			expected: "MATCH (n {uid: 'pod-123'}) WHERE n.count > 5 RETURN n",
		},
		{
			name:     "string array parameter",
			query:    "WHERE n.tags IN $tags",
			params:   map[string]interface{}{"tags": []string{"foo", "bar"}},
			expected: "WHERE n.tags IN ['foo', 'bar']",
		},
		{
			name:     "parameter with quote in value",
			query:    "MATCH (n {name: $name}) RETURN n",
			params:   map[string]interface{}{"name": "it's working"},
			expected: "MATCH (n {name: 'it\\'s working'}) RETURN n",
		},
		{
			name:  "parameters with prefix collision",
			query: "SET r.deleted = $deleted, r.deletedAt = $deletedAt",
			params: map[string]interface{}{
				"deleted":   false,
				"deletedAt": 0,
			},
			expected: "SET r.deleted = false, r.deletedAt = 0",
		},
		{
			name:  "parameters with multiple prefix collisions",
			query: "SET r.name = $name, r.namespace = $namespace, r.namespacedName = $namespacedName",
			params: map[string]interface{}{
				"name":           "test",
				"namespace":      "default",
				"namespacedName": "default/test",
			},
			expected: "SET r.name = 'test', r.namespace = 'default', r.namespacedName = 'default/test'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceCypherParameters(tt.query, tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseQueryStats(t *testing.T) {
	tests := []struct {
		name     string
		stats    []interface{}
		expected QueryStats
	}{
		{
			name: "nodes created",
			stats: []interface{}{
				"Nodes created: 5",
				"Properties set: 10",
			},
			expected: QueryStats{
				NodesCreated:  5,
				PropertiesSet: 10,
			},
		},
		{
			name: "relationships created",
			stats: []interface{}{
				"Relationships created: 3",
				"Labels added: 2",
			},
			expected: QueryStats{
				RelationshipsCreated: 3,
				LabelsAdded:          2,
			},
		},
		{
			name: "deletions",
			stats: []interface{}{
				"Nodes deleted: 10",
				"Relationships deleted: 5",
			},
			expected: QueryStats{
				NodesDeleted:         10,
				RelationshipsDeleted: 5,
			},
		},
		{
			name: "mixed stats",
			stats: []interface{}{
				"Nodes created: 2",
				"Relationships created: 4",
				"Properties set: 8",
				"Labels added: 1",
			},
			expected: QueryStats{
				NodesCreated:         2,
				RelationshipsCreated: 4,
				PropertiesSet:        8,
				LabelsAdded:          1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseQueryStats(tt.stats)
			assert.Equal(t, tt.expected.NodesCreated, result.NodesCreated)
			assert.Equal(t, tt.expected.NodesDeleted, result.NodesDeleted)
			assert.Equal(t, tt.expected.RelationshipsCreated, result.RelationshipsCreated)
			assert.Equal(t, tt.expected.RelationshipsDeleted, result.RelationshipsDeleted)
			assert.Equal(t, tt.expected.PropertiesSet, result.PropertiesSet)
			assert.Equal(t, tt.expected.LabelsAdded, result.LabelsAdded)
		})
	}
}

func TestParseGraphQueryResult(t *testing.T) {
	tests := []struct {
		name          string
		result        interface{}
		expectError   bool
		expectedRows  int
		expectedCols  int
	}{
		{
			name:          "invalid result type",
			result:        "not an array",
			expectError:   true,
			expectedRows:  0,
			expectedCols:  0,
		},
		{
			name:          "empty result",
			result:        []interface{}{},
			expectError:   false,
			expectedRows:  0,
			expectedCols:  0,
		},
		{
			name: "single row result",
			result: []interface{}{
				[]interface{}{"name", "age"},                        // columns
				[]interface{}{"Alice", 30},                          // row 1
				[]interface{}{"Nodes created: 1", "Properties set: 2"}, // stats
			},
			expectError:  false,
			expectedRows: 1,
			expectedCols: 2,
		},
		{
			name: "multiple rows result",
			result: []interface{}{
				[]interface{}{"name", "age"},                        // columns
				[]interface{}{"Alice", 30},                          // row 1
				[]interface{}{"Bob", 25},                            // row 2
				[]interface{}{"Charlie", 35},                        // row 3
				[]interface{}{"Nodes created: 3", "Properties set: 6"}, // stats
			},
			expectError:  false,
			expectedRows: 3,
			expectedCols: 2,
		},
		{
			name: "result with stats only",
			result: []interface{}{
				[]interface{}{"Nodes created: 1", "Properties set: 2"}, // stats
			},
			expectError:  false,
			expectedRows: 0,
			expectedCols: 2, // The stats array becomes columns (2 elements)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseGraphQueryResult(tt.result)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Len(t, result.Rows, tt.expectedRows)

			if tt.expectedCols > 0 {
				assert.Len(t, result.Columns, tt.expectedCols)
			}
		})
	}
}
