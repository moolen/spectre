package sync

import (
	"context"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
)

// findNodeUIDByName queries the graph for a Node with the given name and returns its UID
func (b *graphBuilder) findNodeUIDByName(ctx context.Context, nodeName string) (string, error) {
	return b.findResourceUIDByName(ctx, "Node", nodeName, "")
}

// findPVCUIDByName queries the graph for a PVC with the given name and namespace and returns its UID
func (b *graphBuilder) findPVCUIDByName(ctx context.Context, pvcName, namespace string) (string, error) {
	return b.findResourceUIDByName(ctx, "PersistentVolumeClaim", pvcName, namespace)
}

// findServiceAccountUIDByName queries the graph for a ServiceAccount with the given name and namespace and returns its UID
func (b *graphBuilder) findServiceAccountUIDByName(ctx context.Context, saName, namespace string) (string, error) {
	return b.findResourceUIDByName(ctx, "ServiceAccount", saName, namespace)
}

// findResourceUIDByName is a generic helper to find a resource UID by kind, name, and optionally namespace
func (b *graphBuilder) findResourceUIDByName(ctx context.Context, kind, name, namespace string) (string, error) {
	var query graph.GraphQuery
	if namespace != "" {
		query = graph.GraphQuery{
			Query: `
				MATCH (n:ResourceIdentity {kind: $kind, name: $name, namespace: $namespace})
				RETURN n.uid as uid
				LIMIT 1
			`,
			Parameters: map[string]interface{}{
				"kind":      kind,
				"name":      name,
				"namespace": namespace,
			},
		}
	} else {
		query = graph.GraphQuery{
			Query: `
				MATCH (n:ResourceIdentity {kind: $kind, name: $name})
				RETURN n.uid as uid
				LIMIT 1
			`,
			Parameters: map[string]interface{}{
				"kind": kind,
				"name": name,
			},
		}
	}

	result, err := b.client.ExecuteQuery(ctx, query)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}

	if len(result.Rows) == 0 || len(result.Rows[0]) == 0 {
		return "", nil
	}

	if uid, ok := result.Rows[0][0].(string); ok {
		return uid, nil
	}

	return "", fmt.Errorf("unexpected result type: %T", result.Rows[0][0])
}
