package graph

import (
	"context"
	"encoding/json"
	"fmt"
)

// CreateNode creates a node in the graph.
func (c *falkorClient) CreateNode(ctx context.Context, nodeType NodeType, properties interface{}) error {
	propsJSON, err := json.Marshal(properties)
	if err != nil {
		return fmt.Errorf("failed to marshal node properties: %w", err)
	}

	var propsMap map[string]interface{}
	if err := json.Unmarshal(propsJSON, &propsMap); err != nil {
		return fmt.Errorf("failed to unmarshal properties: %w", err)
	}

	cypherQuery := fmt.Sprintf("CREATE (n:%s %s)", nodeType, buildPropertiesString(propsMap))
	_, err = c.ExecuteQuery(ctx, GraphQuery{Query: cypherQuery})

	return err
}

// CreateEdge creates an edge between two nodes.
func (c *falkorClient) CreateEdge(ctx context.Context, edgeType EdgeType, fromUID, toUID string, properties interface{}) error {
	propsJSON, err := json.Marshal(properties)
	if err != nil {
		return fmt.Errorf("failed to marshal edge properties: %w", err)
	}

	var propsMap map[string]interface{}
	if err := json.Unmarshal(propsJSON, &propsMap); err != nil {
		return fmt.Errorf("failed to unmarshal properties: %w", err)
	}

	cypherQuery := fmt.Sprintf(
		"MATCH (a {uid: '%s'}), (b {uid: '%s'}) CREATE (a)-[r:%s %s]->(b)",
		escapeCypherString(fromUID),
		escapeCypherString(toUID),
		edgeType,
		buildPropertiesString(propsMap),
	)

	_, err = c.ExecuteQuery(ctx, GraphQuery{Query: cypherQuery})
	return err
}

// GetNode retrieves a node by UID.
func (c *falkorClient) GetNode(ctx context.Context, nodeType NodeType, uid string) (*Node, error) {
	cypherQuery := fmt.Sprintf(
		"MATCH (n:%s {uid: '%s'}) RETURN n",
		nodeType,
		escapeCypherString(uid),
	)

	result, err := c.ExecuteQuery(ctx, GraphQuery{Query: cypherQuery})
	if err != nil {
		return nil, err
	}
	if len(result.Rows) == 0 {
		return nil, fmt.Errorf("node not found: %s/%s", nodeType, uid)
	}
	if len(result.Rows[0]) == 0 {
		return nil, fmt.Errorf("empty result row")
	}

	nodeProps, err := ParseNodeFromResult(result.Rows[0][0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse node: %w", err)
	}

	propsJSON, err := json.Marshal(nodeProps)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal properties: %w", err)
	}

	return &Node{
		Type:       nodeType,
		Properties: json.RawMessage(propsJSON),
	}, nil
}

// DeleteNodesByTimestamp deletes nodes older than the given timestamp.
func (c *falkorClient) DeleteNodesByTimestamp(ctx context.Context, nodeType NodeType, timestampField string, cutoffNs int64) (int, error) {
	cypherQuery := fmt.Sprintf(
		"MATCH (n:%s) WHERE n.%s < %d DETACH DELETE n",
		nodeType,
		timestampField,
		cutoffNs,
	)

	result, err := c.ExecuteQuery(ctx, GraphQuery{Query: cypherQuery})
	if err != nil {
		return 0, err
	}

	return result.Stats.NodesDeleted, nil
}

// GetGraphStats retrieves overall graph statistics.
func (c *falkorClient) GetGraphStats(ctx context.Context) (*GraphStats, error) {
	nodeResult, err := c.ExecuteQuery(ctx, GraphQuery{
		Query: `
		MATCH (n)
		RETURN labels(n)[0] as type, count(n) as count
	`,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query node counts: %w", err)
	}

	edgeResult, err := c.ExecuteQuery(ctx, GraphQuery{
		Query: `
		MATCH ()-[r]->()
		RETURN type(r) as type, count(r) as count
	`,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query edge counts: %w", err)
	}

	timestampResult, err := c.ExecuteQuery(ctx, GraphQuery{
		Query: `
		MATCH (e:ChangeEvent)
		RETURN min(e.timestamp) as oldest, max(e.timestamp) as newest
	`,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query timestamps: %w", err)
	}

	stats := &GraphStats{
		NodesByType: make(map[NodeType]int),
		EdgesByType: make(map[EdgeType]int),
	}

	for _, row := range nodeResult.Rows {
		if len(row) < 2 {
			continue
		}
		nodeType, ok := row[0].(string)
		if !ok {
			continue
		}

		switch count := row[1].(type) {
		case int64:
			stats.NodesByType[NodeType(nodeType)] = int(count)
			stats.NodeCount += int(count)
		case float64:
			stats.NodesByType[NodeType(nodeType)] = int(count)
			stats.NodeCount += int(count)
		}
	}

	for _, row := range edgeResult.Rows {
		if len(row) < 2 {
			continue
		}
		edgeType, ok := row[0].(string)
		if !ok {
			continue
		}

		switch count := row[1].(type) {
		case int64:
			stats.EdgesByType[EdgeType(edgeType)] = int(count)
			stats.EdgeCount += int(count)
		case float64:
			stats.EdgesByType[EdgeType(edgeType)] = int(count)
			stats.EdgeCount += int(count)
		}
	}

	if len(timestampResult.Rows) > 0 && len(timestampResult.Rows[0]) >= 2 {
		switch oldest := timestampResult.Rows[0][0].(type) {
		case int64:
			stats.OldestTimestamp = oldest
		case float64:
			stats.OldestTimestamp = int64(oldest)
		}

		switch newest := timestampResult.Rows[0][1].(type) {
		case int64:
			stats.NewestTimestamp = newest
		case float64:
			stats.NewestTimestamp = int64(newest)
		}
	}

	c.logger.Debug(
		"Graph stats: %d nodes, %d edges (oldest: %d, newest: %d)",
		stats.NodeCount,
		stats.EdgeCount,
		stats.OldestTimestamp,
		stats.NewestTimestamp,
	)

	return stats, nil
}
