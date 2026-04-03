package graph

import "fmt"

// FindResourceByUIDQuery returns a query to find a ResourceIdentity by UID.
func FindResourceByUIDQuery(uid string) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $uid})
			RETURN r
		`,
		Parameters: map[string]interface{}{"uid": uid},
	}
}

// FindResourceWithRelationshipsQuery returns a resource and all its related nodes and edges.
func FindResourceWithRelationshipsQuery(uid string) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (resource:ResourceIdentity {uid: $uid})
			OPTIONAL MATCH (resource)-[outRel]->(outNode)
			OPTIONAL MATCH (inNode)-[inRel]->(resource)
			RETURN resource,
			       collect(DISTINCT {
			           direction: 'outgoing',
			           type: type(outRel),
			           properties: properties(outRel),
			           node: outNode
			       }) as outgoing,
			       collect(DISTINCT {
			           direction: 'incoming',
			           type: type(inRel),
			           properties: properties(inRel),
			           node: inNode
			       }) as incoming
		`,
		Parameters: map[string]interface{}{"uid": uid},
	}
}

// FindResourceTopologyQuery returns only ResourceIdentity relationships.
func FindResourceTopologyQuery(uid string) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (resource:ResourceIdentity {uid: $uid})
			OPTIONAL MATCH (resource)-[outRel]->(outNode:ResourceIdentity)
			OPTIONAL MATCH (inNode:ResourceIdentity)-[inRel]->(resource)
			RETURN resource,
			       collect(DISTINCT {
			           direction: 'outgoing',
			           type: type(outRel),
			           properties: properties(outRel),
			           targetUID: outNode.uid,
			           targetKind: outNode.kind,
			           targetNamespace: outNode.namespace,
			           targetName: outNode.name,
			           node: outNode
			       }) as outgoing,
			       collect(DISTINCT {
			           direction: 'incoming',
			           type: type(inRel),
			           properties: properties(inRel),
			           sourceUID: inNode.uid,
			           sourceKind: inNode.kind,
			           sourceNamespace: inNode.namespace,
			           sourceName: inNode.name,
			           node: inNode
			       }) as incoming
		`,
		Parameters: map[string]interface{}{"uid": uid},
	}
}

// FindChangeEventsByResourceQuery returns all ChangeEvents for a resource.
func FindChangeEventsByResourceQuery(resourceUID string, startTime, endTime int64) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $resourceUID})-[:CHANGED]->(e:ChangeEvent)
			WHERE e.timestamp >= $startTime AND e.timestamp <= $endTime
			RETURN e
			ORDER BY e.timestamp ASC
		`,
		Parameters: map[string]interface{}{
			"resourceUID": resourceUID,
			"startTime":   startTime,
			"endTime":     endTime,
		},
	}
}

// FindRootCauseQuery traces backward from a failure to find likely root causes.
func FindRootCauseQuery(resourceUID string, failureTimestamp int64, maxDepth int, minConfidence float64) GraphQuery {
	toleranceNs := int64(300_000_000_000)
	lookbackNs := int64(600_000_000_000)

	return GraphQuery{
		Query: fmt.Sprintf(`
			MATCH (failedResource:ResourceIdentity {uid: $resourceUID})
			      -[:CHANGED]->(failureEvent:ChangeEvent)
			WHERE failureEvent.timestamp <= $failureTimestamp + $tolerance
			  AND failureEvent.timestamp >= $failureTimestamp - $tolerance

			OPTIONAL MATCH causalityPath = (failureEvent)<-[:TRIGGERED_BY*1..%d]-(causeEvent:ChangeEvent)
			WHERE ALL(rel IN relationships(causalityPath) WHERE rel.confidence >= $minConfidence)

			WITH failedResource, failureEvent, causeEvent, causalityPath
			OPTIONAL MATCH (failedResource)<-[:OWNS*1..3]-(owner:ResourceIdentity)
			WITH failedResource, failureEvent, causeEvent, causalityPath, collect(DISTINCT owner) as allOwners

			UNWIND CASE WHEN size(allOwners) > 0 THEN allOwners ELSE [null] END as owner
			OPTIONAL MATCH (manager:ResourceIdentity)-[manages:MANAGES]->(owner)
			WHERE manages.confidence >= $minConfidence
			OPTIONAL MATCH (manager)-[:CHANGED]->(managerEvent:ChangeEvent)
			WHERE managerEvent.timestamp <= $failureTimestamp
			  AND managerEvent.timestamp >= $failureTimestamp - $lookback

			WITH DISTINCT
			  CASE
			    WHEN causeEvent IS NOT NULL THEN causeEvent
			    WHEN managerEvent IS NOT NULL THEN managerEvent
			    ELSE failureEvent
			  END as finalCauseEvent,
			  CASE
			    WHEN causalityPath IS NOT NULL THEN relationships(causalityPath)
			    ELSE []
			  END as triggers,
			  CASE
			    WHEN manages IS NOT NULL THEN manages
			    ELSE null
			  END as managesRel,
			  owner as managedResource

			MATCH (finalCauseEvent)<-[:CHANGED]-(causeResource:ResourceIdentity)
			OPTIONAL MATCH (causeResource)<-[:OWNS*1..3]-(parentResource:ResourceIdentity)

			RETURN causeResource, finalCauseEvent, parentResource, triggers, managesRel
			ORDER BY finalCauseEvent.impactScore DESC, finalCauseEvent.timestamp DESC
			LIMIT 10
		`, maxDepth),
		Parameters: map[string]interface{}{
			"resourceUID":      resourceUID,
			"failureTimestamp": failureTimestamp,
			"tolerance":        toleranceNs,
			"minConfidence":    minConfidence,
			"lookback":         lookbackNs,
		},
	}
}

// CalculateBlastRadiusQuery finds all resources affected by a change.
func CalculateBlastRadiusQuery(resourceUID string, changeTimestamp, timeWindowMs int64, relationshipTypes []string) GraphQuery {
	relPattern := ""
	if len(relationshipTypes) > 0 {
		relPattern = ":" + relationshipTypes[0]
		for i := 1; i < len(relationshipTypes); i++ {
			relPattern += "|" + relationshipTypes[i]
		}
	}

	timeWindowNs := timeWindowMs * 1_000_000
	query := fmt.Sprintf(`
		MATCH (triggerResource:ResourceIdentity {uid: $resourceUID})
		      -[:CHANGED]->(triggerEvent:ChangeEvent)
		WHERE triggerEvent.timestamp = $changeTimestamp

		MATCH (triggerResource)-[rel%s*1..3]->(impacted:ResourceIdentity)
		MATCH (impacted)-[:CHANGED]->(impactEvent:ChangeEvent)
		WHERE impactEvent.timestamp > $changeTimestamp
		  AND impactEvent.timestamp < $changeTimestamp + $timeWindowNs
		  AND (impactEvent.status = 'Warning' OR impactEvent.status = 'Error')

		RETURN impacted, impactEvent, type(rel) as relType, length(relationships(rel)) as distance
		ORDER BY impactEvent.timestamp
	`, relPattern)

	return GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"resourceUID":     resourceUID,
			"changeTimestamp": changeTimestamp,
			"timeWindowNs":    timeWindowNs,
		},
	}
}

// DeleteOldChangeEventsQuery removes ChangeEvent nodes older than cutoff.
func DeleteOldChangeEventsQuery(cutoffNs int64) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (e:ChangeEvent)
			WHERE e.timestamp < $cutoffNs
			DETACH DELETE e
		`,
		Parameters: map[string]interface{}{"cutoffNs": cutoffNs},
	}
}

// DeleteOldK8sEventsQuery removes K8sEvent nodes older than cutoff.
func DeleteOldK8sEventsQuery(cutoffNs int64) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (e:K8sEvent)
			WHERE e.timestamp < $cutoffNs
			DETACH DELETE e
		`,
		Parameters: map[string]interface{}{"cutoffNs": cutoffNs},
	}
}

// GetGraphStatsQuery returns comprehensive graph statistics.
func GetGraphStatsQuery() GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (n)
			WITH labels(n)[0] as nodeType, count(n) as nodeCount

			MATCH ()-[r]->()
			WITH nodeType, nodeCount, type(r) as edgeType, count(r) as edgeCount

			MATCH (e:ChangeEvent)
			WITH nodeType, nodeCount, edgeType, edgeCount,
			     min(e.timestamp) as oldestTimestamp,
			     max(e.timestamp) as newestTimestamp

			RETURN nodeType, nodeCount, edgeType, edgeCount, oldestTimestamp, newestTimestamp
		`,
		Parameters: nil,
	}
}

// FindManagedResourcesQuery finds all resources managed by a CR.
func FindManagedResourcesQuery(crUID string, minConfidence float64) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (cr:ResourceIdentity {uid: $crUID})
			      -[manages:MANAGES]->(managed:ResourceIdentity)
			WHERE manages.confidence >= $minConfidence
			  AND NOT managed.deleted
			RETURN managed, manages
			ORDER BY manages.confidence DESC
		`,
		Parameters: map[string]interface{}{
			"crUID":         crUID,
			"minConfidence": minConfidence,
		},
	}
}

// FindStaleInferredEdgesQuery finds edges needing revalidation.
func FindStaleInferredEdgesQuery(cutoffTimestamp int64) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (source)-[edge:MANAGES]->(target)
			WHERE edge.lastValidated < $cutoffTimestamp
			   OR edge.validationState = 'stale'
			RETURN source.uid as sourceUID,
			       target.uid as targetUID,
			       edge
			LIMIT 1000
		`,
		Parameters: map[string]interface{}{"cutoffTimestamp": cutoffTimestamp},
	}
}
