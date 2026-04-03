package graph

import (
	"context"
	"fmt"
	"strings"
)

// InitializeSchema creates indexes and constraints.
func (c *falkorClient) InitializeSchema(ctx context.Context) error {
	c.logger.Info("Initializing graph schema for graph: %s", c.config.GraphName)

	indexes := []string{
		"CREATE INDEX FOR (n:ResourceIdentity) ON (n.uid)",
		"CREATE INDEX FOR (n:ResourceIdentity) ON (n.kind)",
		"CREATE INDEX FOR (n:ResourceIdentity) ON (n.namespace)",
		"CREATE INDEX FOR (n:ResourceIdentity) ON (n.deleted)",
		"CREATE INDEX FOR (n:ResourceIdentity) ON (n.firstSeen)",
		"CREATE INDEX FOR (n:ChangeEvent) ON (n.id)",
		"CREATE INDEX FOR (n:ChangeEvent) ON (n.timestamp)",
		"CREATE INDEX FOR (n:ChangeEvent) ON (n.status)",
		"CREATE INDEX FOR (n:K8sEvent) ON (n.timestamp)",
		"CREATE INDEX FOR (n:Dashboard) ON (n.uid)",
		"CREATE INDEX FOR (n:SignalAnchor) ON (n.uid)",
		"CREATE INDEX FOR (n:SignalAnchor) ON (n.metric_name)",
		"CREATE INDEX FOR (n:SignalAnchor) ON (n.workload_namespace)",
		"CREATE INDEX FOR (n:SignalAnchor) ON (n.workload_name)",
		"CREATE INDEX FOR (n:SignalAnchor) ON (n.expires_at)",
		"CREATE INDEX FOR (n:SignalAnchor) ON (n.source_provider)",
		"CREATE INDEX FOR (n:SignalBaseline) ON (n.metric_name)",
		"CREATE INDEX FOR (n:SignalBaseline) ON (n.expires_at)",
	}

	for _, indexQuery := range indexes {
		if _, err := c.ExecuteQuery(ctx, GraphQuery{Query: indexQuery}); err != nil {
			c.logger.Warn("Failed to create index (may already exist): %v", err)
		}
	}

	c.logger.Info("Schema initialization complete")
	return nil
}

// DeleteGraph completely removes the graph.
func (c *falkorClient) DeleteGraph(ctx context.Context) error {
	if c.graph == nil {
		return fmt.Errorf("client not connected")
	}

	err := c.graph.Delete()
	if err != nil {
		if strings.Contains(err.Error(), "empty key") {
			c.logger.Debug("Graph '%s' does not exist, nothing to delete", c.config.GraphName)
		} else {
			return fmt.Errorf("failed to delete graph: %w", err)
		}
	} else {
		c.logger.Info("Graph '%s' deleted", c.config.GraphName)
	}

	c.graph = c.db.SelectGraph(c.config.GraphName)
	return nil
}

// CreateGraph creates a new named graph database.
func (c *falkorClient) CreateGraph(ctx context.Context, graphName string) error {
	if c.db == nil {
		return fmt.Errorf("client not connected")
	}

	c.logger.Info("Creating graph: %s", graphName)

	graph := c.db.SelectGraph(graphName)
	if _, err := graph.Query("RETURN 1", nil, nil); err != nil {
		return fmt.Errorf("failed to create graph %s: %w", graphName, err)
	}

	c.logger.Info("Graph '%s' created successfully", graphName)
	return nil
}

// DeleteGraphByName deletes a specific named graph database.
func (c *falkorClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	if c.db == nil {
		return fmt.Errorf("client not connected")
	}

	c.logger.Info("Deleting graph: %s", graphName)

	graph := c.db.SelectGraph(graphName)
	if err := graph.Delete(); err != nil {
		if strings.Contains(err.Error(), "empty key") {
			c.logger.Debug("Graph '%s' does not exist, nothing to delete", graphName)
			return nil
		}
		return fmt.Errorf("failed to delete graph %s: %w", graphName, err)
	}

	c.logger.Info("Graph '%s' deleted successfully", graphName)
	return nil
}

// GraphExists checks if a named graph exists.
func (c *falkorClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	if c.db == nil {
		return false, fmt.Errorf("client not connected")
	}

	result := c.db.Conn.Keys(ctx, "RedisGraph_"+graphName)
	if result.Err() != nil {
		return false, fmt.Errorf("failed to check graph existence: %w", result.Err())
	}

	keys, err := result.Result()
	if err != nil {
		return false, fmt.Errorf("failed to get keys result: %w", err)
	}

	exists := len(keys) > 0
	c.logger.Debug("Graph '%s' exists: %v", graphName, exists)
	return exists, nil
}
