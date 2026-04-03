package causalpaths

import (
	"context"
	"fmt"
	"sort"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
)

// querySelectsTargets queries the graph database to find Pods selected by a Service.
func (d *PathDiscoverer) querySelectsTargets(ctx context.Context, serviceUID string, failureTimestampNs int64) ([]selectsTarget, error) {
	service, err := d.store.GetResource(ctx, serviceUID)
	if err != nil {
		return nil, fmt.Errorf("failed to load Service resource: %w", err)
	}
	if service == nil || service.Namespace == "" {
		return nil, nil
	}

	targets := make([]selectsTarget, 0)
	seen := make(map[string]bool)
	cursor := ""

	for {
		namespaceGraph, graphErr := d.store.GetNamespaceGraph(ctx, analysisstore.NamespaceGraphQuery{
			Namespace:   service.Namespace,
			TimestampNs: failureTimestampNs,
			MaxDepth:    1,
			Limit:       500,
			Cursor:      cursor,
		})
		if graphErr != nil {
			return nil, fmt.Errorf("failed to load namespace graph for Service %s: %w", serviceUID, graphErr)
		}

		nodesByUID := make(map[string]analysisstore.NamespaceGraphNode, len(namespaceGraph.Graph.Nodes))
		for _, node := range namespaceGraph.Graph.Nodes {
			nodesByUID[node.UID] = node
		}

		for _, edge := range namespaceGraph.Graph.Edges {
			if edge.Source != serviceUID || edge.RelationshipType != "SELECTS" {
				continue
			}
			targetNode, ok := nodesByUID[edge.Target]
			if !ok || targetNode.Kind != "Pod" || seen[targetNode.UID] {
				continue
			}
			seen[targetNode.UID] = true
			targets = append(targets, selectsTarget{
				uid:       targetNode.UID,
				kind:      targetNode.Kind,
				namespace: targetNode.Namespace,
				name:      targetNode.Name,
			})
		}

		if !namespaceGraph.Metadata.HasMore || namespaceGraph.Metadata.NextCursor == "" {
			break
		}
		cursor = namespaceGraph.Metadata.NextCursor
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].uid < targets[j].uid
	})

	return targets, nil
}
