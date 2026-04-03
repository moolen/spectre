package falkor

import (
	"context"
	"fmt"

	"github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
)

func (s *Store) GetResource(ctx context.Context, uid string) (*graph.ResourceIdentity, error) {
	result, err := s.graphClient.ExecuteQuery(ctx, graph.FindResourceByUIDQuery(uid))
	if err != nil {
		return nil, fmt.Errorf("failed to query resource: %w", err)
	}
	if len(result.Rows) == 0 || len(result.Rows[0]) == 0 {
		return nil, nil
	}

	resourceProps, err := graph.ParseNodeFromResult(result.Rows[0][0])
	if err != nil || resourceProps == nil || len(resourceProps) == 0 {
		return nil, nil
	}

	resource := graph.ParseResourceIdentityFromNode(resourceProps)
	return &resource, nil
}

func (s *Store) GetOwnershipChain(
	ctx context.Context,
	uid string,
	atTimestampNs int64,
	maxDepth int,
) ([]store.ResourceWithDistance, error) {
	_ = atTimestampNs

	if maxDepth <= 0 {
		maxDepth = 3
	}

	symptomResult, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query: `
			MATCH (symptom:ResourceIdentity {uid: $symptomUID})
			RETURN symptom as resource, 0 as distance
		`,
		Parameters: map[string]interface{}{
			"symptomUID": uid,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query symptom resource: %w", err)
	}

	chain := []store.ResourceWithDistance{}
	for _, row := range symptomResult.Rows {
		if len(row) < 2 {
			continue
		}
		resourceProps, err := graph.ParseNodeFromResult(row[0])
		if err != nil || resourceProps == nil || len(resourceProps) == 0 {
			continue
		}
		chain = append(chain, store.ResourceWithDistance{
			Resource: graph.ParseResourceIdentityFromNode(resourceProps),
			Distance: 0,
		})
	}

	if len(chain) == 0 {
		return nil, fmt.Errorf("symptom resource not found: %s", uid)
	}

	ownersResult, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query: fmt.Sprintf(`
			MATCH (symptom:ResourceIdentity {uid: $symptomUID})
			MATCH path = (symptom)<-[:OWNS*1..%d]-(owner:ResourceIdentity)
			RETURN DISTINCT owner as resource, length(path) as distance
			ORDER BY distance ASC
		`, maxDepth),
		Parameters: map[string]interface{}{
			"symptomUID": uid,
		},
	})
	if err != nil {
		return chain, nil
	}

	seenUIDs := map[string]bool{chain[0].Resource.UID: true}
	for _, row := range ownersResult.Rows {
		if len(row) < 2 {
			continue
		}
		resourceProps, err := graph.ParseNodeFromResult(row[0])
		if err != nil || resourceProps == nil || len(resourceProps) == 0 {
			continue
		}
		resource := graph.ParseResourceIdentityFromNode(resourceProps)
		if seenUIDs[resource.UID] {
			continue
		}
		seenUIDs[resource.UID] = true

		distance := 0
		switch d := row[1].(type) {
		case int64:
			distance = int(d)
		case float64:
			distance = int(d)
		}

		chain = append(chain, store.ResourceWithDistance{
			Resource: resource,
			Distance: distance,
		})
	}

	return chain, nil
}

func (s *Store) GetManagers(
	ctx context.Context,
	resourceUIDs []string,
	minConfidence float64,
) (map[string]*store.ManagerData, error) {
	if len(resourceUIDs) == 0 {
		return map[string]*store.ManagerData{}, nil
	}
	if minConfidence <= 0 {
		minConfidence = defaultManagerFloor
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query: `
			MATCH (resource:ResourceIdentity)
			WHERE resource.uid IN $resourceUIDs
			OPTIONAL MATCH (manager:ResourceIdentity)-[manages:MANAGES]->(resource)
			WHERE manages.confidence >= $minConfidence
			WITH resource.uid as resourceUID, manager, manages
			ORDER BY resourceUID ASC, manages.confidence DESC, manager.uid ASC
			RETURN resourceUID, manager, manages
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs":  resourceUIDs,
			"minConfidence": minConfidence,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query managers: %w", err)
	}

	managers := make(map[string]*store.ManagerData)
	for _, row := range result.Rows {
		if len(row) < 3 {
			continue
		}
		resourceUID, ok := row[0].(string)
		if !ok || row[1] == nil {
			continue
		}
		if _, exists := managers[resourceUID]; exists {
			continue
		}

		managerProps, err := graph.ParseNodeFromResult(row[1])
		if err != nil || managerProps == nil || len(managerProps) == 0 {
			continue
		}

		var managesEdge graph.ManagesEdge
		if row[2] != nil {
			_, edgeProps, err := graph.ParseEdgeFromResult(row[2])
			if err == nil {
				managesEdge = graph.ParseManagesEdge(edgeProps)
			}
		}

		managers[resourceUID] = &store.ManagerData{
			Manager:     graph.ParseResourceIdentityFromNode(managerProps),
			ManagesEdge: managesEdge,
		}
	}

	return managers, nil
}

func (s *Store) GetRelatedResources(
	ctx context.Context,
	resourceUIDs []string,
	window store.ResourceWindow,
) (map[string][]store.RelatedResourceData, error) {
	if len(resourceUIDs) == 0 {
		return map[string][]store.RelatedResourceData{}, nil
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query: `
			MATCH (resource:ResourceIdentity)
			WHERE resource.uid IN $resourceUIDs
			OPTIONAL MATCH (resource)-[refSpec:REFERENCES_SPEC]->(referencedResource:ResourceIdentity)
			WHERE coalesce(referencedResource.deleted, false) = false
			   OR (referencedResource.deletedAt >= $startNs AND referencedResource.deletedAt <= $endNs)
			OPTIONAL MATCH (resource)-[scheduledOn:SCHEDULED_ON]->(node:ResourceIdentity)
			WHERE coalesce(node.deleted, false) = false
			   OR (node.deletedAt >= $startNs AND node.deletedAt <= $endNs)
			OPTIONAL MATCH (resource)-[usesSA:USES_SERVICE_ACCOUNT]->(sa:ResourceIdentity)
			WHERE coalesce(sa.deleted, false) = false
			   OR (sa.deletedAt >= $startNs AND sa.deletedAt <= $endNs)
			OPTIONAL MATCH (selector:ResourceIdentity)-[selects:SELECTS]->(resource)
			WHERE selector.kind IN ['Service', 'NetworkPolicy']
			  AND (coalesce(selector.deleted, false) = false
			       OR (selector.deletedAt >= $startNs AND selector.deletedAt <= $endNs))
			OPTIONAL MATCH (ingress:ResourceIdentity)-[ref:REFERENCES_SPEC]->(selector)
			WHERE ingress.kind = 'Ingress' AND selector.kind = 'Service'
			  AND (coalesce(ingress.deleted, false) = false
			       OR (ingress.deletedAt >= $startNs AND ingress.deletedAt <= $endNs))
			OPTIONAL MATCH (rb:ResourceIdentity)-[grantsTo:GRANTS_TO]->(sa)
			WHERE sa IS NOT NULL
			  AND (coalesce(rb.deleted, false) = false
			       OR (rb.deletedAt >= $startNs AND rb.deletedAt <= $endNs))
			OPTIONAL MATCH (rb)-[bindsRole:BINDS_ROLE]->(role:ResourceIdentity)
			WHERE rb IS NOT NULL
			  AND (coalesce(role.deleted, false) = false
			       OR (role.deletedAt >= $startNs AND role.deletedAt <= $endNs))
			RETURN resource.uid as resourceUID,
			       referencedResource, 'REFERENCES_SPEC' as refSpecType,
			       node, 'SCHEDULED_ON' as scheduledOnType,
			       sa, 'USES_SERVICE_ACCOUNT' as usesSAType,
			       selector, 'SELECTS' as selectsType,
			       rb, 'GRANTS_TO' as grantsToType,
			       ingress,
			       role, 'BINDS_ROLE' as bindsRoleType
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs": resourceUIDs,
			"startNs":      window.Start(),
			"endNs":        window.FailureTimestampNs,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query related resources: %w", err)
	}

	related := make(map[string][]store.RelatedResourceData)
	for _, row := range result.Rows {
		if len(row) < 14 {
			continue
		}

		resourceUID, ok := row[0].(string)
		if !ok {
			continue
		}
		if _, exists := related[resourceUID]; !exists {
			related[resourceUID] = []store.RelatedResourceData{}
		}

		addRelated := func(nodeIdx int, relType string) {
			if row[nodeIdx] == nil {
				return
			}
			props, err := graph.ParseNodeFromResult(row[nodeIdx])
			if err != nil || props == nil || len(props) == 0 {
				return
			}
			appendRelatedResource(
				related,
				resourceUID,
				graph.ParseResourceIdentityFromNode(props),
				relType,
			)
		}

		addRelated(1, edgeTypeReferences)
		addRelated(3, "SCHEDULED_ON")
		addRelated(5, "USES_SERVICE_ACCOUNT")
		addRelated(7, "SELECTS")
		addRelated(9, "GRANTS_TO")
		addRelated(12, "BINDS_ROLE")

		if row[11] != nil {
			ingressProps, err := graph.ParseNodeFromResult(row[11])
			if err != nil || ingressProps == nil || len(ingressProps) == 0 {
				continue
			}
			ingress := graph.ParseResourceIdentityFromNode(ingressProps)
			var serviceUID string
			if row[7] != nil {
				serviceProps, err := graph.ParseNodeFromResult(row[7])
				if err == nil && serviceProps != nil {
					serviceUID = graph.ParseResourceIdentityFromNode(serviceProps).UID
				}
			}

			appendIngressReference(related, resourceUID, ingress, serviceUID)
		}
	}

	return related, nil
}
