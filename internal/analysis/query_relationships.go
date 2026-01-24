package analysis

import (
	"context"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
)

// getManagers retrieves manager relationships for the given resources.
// Managers are resources connected via MANAGES edges (e.g., HelmRelease -> Deployment).
// Only managers with confidence >= MinManagerConfidence are included.
func (a *RootCauseAnalyzer) getManagers(ctx context.Context, resourceUIDs []string) (map[string]*ManagerData, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string]*ManagerData), nil
	}

	// Optimized: Use direct IN clause instead of UNWIND to avoid O(n²) complexity
	query := graph.GraphQuery{
		Timeout: DefaultQueryTimeoutMs,
		Query: `
			MATCH (resource:ResourceIdentity)
			WHERE resource.uid IN $resourceUIDs
			OPTIONAL MATCH (manager:ResourceIdentity)-[manages:MANAGES]->(resource)
			WHERE manages.confidence >= $minConfidence
			RETURN resource.uid as resourceUID, manager, manages
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs":  resourceUIDs,
			"minConfidence": MinManagerConfidence,
		},
	}

	a.logger.Debug("getManagers: executing query for %d resources", len(resourceUIDs))
	result, err := a.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query managers: %w", err)
	}

	managers := make(map[string]*ManagerData)

	for _, row := range result.Rows {
		if len(row) < 3 {
			continue
		}

		// Parse resource UID
		resourceUID, ok := row[0].(string)
		if !ok {
			continue
		}

		// Parse manager (may be null)
		if row[1] == nil {
			continue
		}

		managerProps, err := graph.ParseNodeFromResult(row[1])
		if err != nil || managerProps == nil || len(managerProps) == 0 {
			continue
		}
		manager := graph.ParseResourceIdentityFromNode(managerProps)

		// Parse manages edge
		var managesEdge graph.ManagesEdge
		if row[2] != nil {
			_, edgeProps, err := graph.ParseEdgeFromResult(row[2])
			if err == nil {
				managesEdge = graph.ParseManagesEdge(edgeProps)
			}
		}

		managers[resourceUID] = &ManagerData{
			Manager:     manager,
			ManagesEdge: managesEdge,
		}
	}

	a.logger.Debug("getManagers: found managers for %d resources", len(managers))
	return managers, nil
}

// getRelatedResources retrieves resources related through various relationship types.
// This includes:
// - REFERENCES_SPEC: Resources referenced in spec (e.g., HelmRelease -> ConfigMap)
// - SCHEDULED_ON: Pods scheduled on Nodes
// - USES_SERVICE_ACCOUNT: Pods using ServiceAccounts
// - SELECTS: Services/NetworkPolicies selecting resources
// - GRANTS_TO: RoleBindings granting permissions to ServiceAccounts
// - BINDS_ROLE: RoleBindings binding to Roles/ClusterRoles
// - edgeTypeIngressRef: Ingresses referencing Services
//
// The failureTimestamp and lookbackNs parameters are used to include deleted resources
// that were deleted within the time window (important for root cause analysis).
func (a *RootCauseAnalyzer) getRelatedResources(ctx context.Context, resourceUIDs []string, failureTimestamp, lookbackNs int64) (map[string][]RelatedResourceData, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string][]RelatedResourceData), nil
	}

	// Calculate the start of the time window
	startNs := failureTimestamp - lookbackNs

	// Optimized: Use direct IN clause instead of UNWIND to avoid O(n²) complexity
	query := graph.GraphQuery{
		Timeout: DefaultQueryTimeoutMs,
		Query: `
			// Direct relationships from resources
			MATCH (resource:ResourceIdentity)
			WHERE resource.uid IN $resourceUIDs
			// REFERENCES_SPEC: include deleted resources if deleted within time window
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

			// Find Ingresses that reference Services that select this resource
			OPTIONAL MATCH (ingress:ResourceIdentity)-[ref:REFERENCES_SPEC]->(selector)
			WHERE ingress.kind = 'Ingress' AND selector.kind = 'Service'
			  AND (coalesce(ingress.deleted, false) = false
			       OR (ingress.deletedAt >= $startNs AND ingress.deletedAt <= $endNs))

			// Get RoleBindings that grant to service accounts used by this resource
			OPTIONAL MATCH (rb:ResourceIdentity)-[grantsTo:GRANTS_TO]->(sa)
			WHERE sa IS NOT NULL
			  AND (coalesce(rb.deleted, false) = false
			       OR (rb.deletedAt >= $startNs AND rb.deletedAt <= $endNs))

			// Get the Role/ClusterRole that the RoleBinding binds to
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
			       ingress, 'edgeTypeIngressRef' as ingressRefType,
			       role, 'BINDS_ROLE' as bindsRoleType
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs": resourceUIDs,
			"startNs":      startNs,
			"endNs":        failureTimestamp,
		},
	}

	a.logger.Debug("getRelatedResources: executing query for %d resources: %v", len(resourceUIDs), resourceUIDs)
	result, err := a.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query related resources: %w", err)
	}

	a.logger.Debug("getRelatedResources: query returned %d rows", len(result.Rows))
	related := make(map[string][]RelatedResourceData)

	for i, row := range result.Rows {
		a.logger.Debug("getRelatedResources: ROW %d: len=%d", i, len(row))
		if len(row) < 15 {
			a.logger.Debug("getRelatedResources: skipping row with < 15 columns: %d", len(row))
			continue
		}

		resourceUID, ok := row[0].(string)
		if !ok {
			a.logger.Debug("getRelatedResources: skipping row - resourceUID not a string: %T", row[0])
			continue
		}

		a.logger.Debug("getRelatedResources: ROW %d for resource %s", i, resourceUID)

		if _, exists := related[resourceUID]; !exists {
			related[resourceUID] = []RelatedResourceData{}
		}

		// Helper to add related resource
		addRelated := func(nodeIdx int, relType string) {
			a.logger.Debug("getRelatedResources: checking row[%d] for %s (nil=%v)", nodeIdx, relType, row[nodeIdx] == nil)
			if row[nodeIdx] == nil {
				return
			}
			props, err := graph.ParseNodeFromResult(row[nodeIdx])
			if err != nil || props == nil || len(props) == 0 {
				a.logger.Debug("getRelatedResources: failed to parse %s node for resource %s: err=%v, props=%v",
					relType, resourceUID, err, props)
				return
			}
			res := graph.ParseResourceIdentityFromNode(props)

			a.logger.Debug("getRelatedResources: SUCCESS adding %s/%s (type=%s) to resource %s",
				res.Kind, res.Name, relType, resourceUID)

			// Check for duplicates
			for _, existing := range related[resourceUID] {
				if existing.Resource.UID == res.UID && existing.RelationshipType == relType {
					a.logger.Debug("getRelatedResources: skipping duplicate")
					return
				}
			}

			related[resourceUID] = append(related[resourceUID], RelatedResourceData{
				Resource:         res,
				RelationshipType: relType,
				Events:           []ChangeEventInfo{},
			})
		}

		// Parse each relationship type
		// Column indices: 0=resourceUID, 1=referencedResource, 2=refSpecType,
		//   3=node, 4=scheduledOnType, 5=sa, 6=usesSAType, 7=selector, 8=selectsType,
		//   9=rb, 10=grantsToType, 11=ingress, 12=ingressRefType, 13=role, 14=bindsRoleType
		addRelated(1, "REFERENCES_SPEC")      // referencedResource (outgoing from resource)
		addRelated(3, "SCHEDULED_ON")         // node
		addRelated(5, "USES_SERVICE_ACCOUNT") // sa
		addRelated(7, "SELECTS")              // selector (incoming to resource, reversed in causal_chain.go)
		addRelated(9, "GRANTS_TO")            // rb (RoleBinding)
		addRelated(13, "BINDS_ROLE")          // role (Role/ClusterRole bound by RoleBinding)

		// Special handling for edgeTypeIngressRef to also capture the Service UID
		if row[11] != nil {
			ingressProps, err := graph.ParseNodeFromResult(row[11])
			if err == nil && ingressProps != nil && len(ingressProps) > 0 {
				ingress := graph.ParseResourceIdentityFromNode(ingressProps)
				// Also get the Service UID from row[7] (selector)
				var serviceUID string
				if row[7] != nil {
					serviceProps, err := graph.ParseNodeFromResult(row[7])
					if err == nil && serviceProps != nil {
						service := graph.ParseResourceIdentityFromNode(serviceProps)
						serviceUID = service.UID
					}
				}

				// Check for duplicates
				isDuplicate := false
				for _, existing := range related[resourceUID] {
					if existing.Resource.UID == ingress.UID && existing.RelationshipType == edgeTypeIngressRef {
						isDuplicate = true
						break
					}
				}

				if !isDuplicate {
					related[resourceUID] = append(related[resourceUID], RelatedResourceData{
						Resource:           ingress,
						RelationshipType:   edgeTypeIngressRef,
						Events:             []ChangeEventInfo{},
						ReferenceTargetUID: serviceUID,
					})
					a.logger.Debug("getRelatedResources: SUCCESS adding %s/%s (type=%s, target=%s) to resource %s",
						ingress.Kind, ingress.Name, edgeTypeIngressRef, serviceUID, resourceUID)
				}
			}
		}
	}

	a.logger.Debug("getRelatedResources: found related resources for %d resources", len(related))
	for uid, relList := range related {
		a.logger.Debug("getRelatedResources: resource %s has %d related resources", uid, len(relList))
		for _, rel := range relList {
			a.logger.Debug("getRelatedResources: - %s/%s (type=%s)", rel.Resource.Kind, rel.Resource.Name, rel.RelationshipType)
			if rel.RelationshipType == "REFERENCES_SPEC" {
				a.logger.Debug("getRelatedResources: *** resource %s REFERENCES_SPEC %s/%s", uid, rel.Resource.Kind, rel.Resource.Name)
			}
		}
	}
	return related, nil
}
