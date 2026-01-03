package graph

import (
	"context"
	"encoding/json"
	"fmt"
)

// Schema provides utilities for graph schema management
type Schema struct {
	client Client
}

// NewSchema creates a new Schema manager
func NewSchema(client Client) *Schema {
	return &Schema{
		client: client,
	}
}

// Initialize sets up the graph schema with indexes and constraints
func (s *Schema) Initialize(ctx context.Context) error {
	return s.client.InitializeSchema(ctx)
}

// Query builders for common operations

// UpsertResourceIdentityQuery creates a query to insert or update a ResourceIdentity node
// Uses MERGE to provide idempotency
func UpsertResourceIdentityQuery(resource ResourceIdentity) GraphQuery {
	// Serialize labels to JSON for storage
	labelsJSON := "{}"
	if resource.Labels != nil && len(resource.Labels) > 0 {
		labelsBytes, _ := json.Marshal(resource.Labels)
		labelsJSON = string(labelsBytes)
	}

	// Build query based on whether this is a deletion or not
	// Once a resource is deleted, we don't want to un-delete it
	query := `
		MERGE (r:ResourceIdentity {uid: $uid})
		ON CREATE SET
			r.kind = $kind,
			r.apiGroup = $apiGroup,
			r.version = $version,
			r.namespace = $namespace,
			r.name = $name,
			r.labels = $labels,
			r.firstSeen = $firstSeen,
			r.lastSeen = $lastSeen,
			r.deleted = $deleted,
			r.deletedAt = $deletedAt
	`

	// Only update if this is a deletion, or if the resource isn't already deleted
	if resource.Deleted {
		// This is a deletion - always update to mark as deleted
		query += `
		ON MATCH SET
			r.labels = $labels,
			r.lastSeen = $lastSeen,
			r.deleted = true,
			r.deletedAt = $deletedAt
		`
	} else {
		// This is not a deletion - only update if not already deleted
		query += `
		ON MATCH SET
			r.labels = CASE WHEN NOT r.deleted THEN $labels ELSE r.labels END,
			r.lastSeen = CASE WHEN NOT r.deleted THEN $lastSeen ELSE r.lastSeen END
		`
	}

	return GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"uid":       resource.UID,
			"kind":      resource.Kind,
			"apiGroup":  resource.APIGroup,
			"version":   resource.Version,
			"namespace": resource.Namespace,
			"name":      resource.Name,
			"labels":    labelsJSON,
			"firstSeen": resource.FirstSeen,
			"lastSeen":  resource.LastSeen,
			"deleted":   resource.Deleted,
			"deletedAt": resource.DeletedAt,
		},
	}
}

// CreateChangeEventQuery creates a query to insert a ChangeEvent node
// Uses MERGE to ensure uniqueness by event ID
// Note: ON CREATE SET means data is only set when node is first created
// If the node already exists, data won't be updated (which is correct - events are immutable)
func CreateChangeEventQuery(event ChangeEvent) GraphQuery {
	return GraphQuery{
		Query: `
			MERGE (e:ChangeEvent {id: $id})
			ON CREATE SET
				e.timestamp = $timestamp,
				e.eventType = $eventType,
				e.status = $status,
				e.errorMessage = $errorMessage,
				e.containerIssues = $containerIssues,
				e.configChanged = $configChanged,
				e.statusChanged = $statusChanged,
				e.replicasChanged = $replicasChanged,
				e.impactScore = $impactScore,
				e.data = $data
		`,
		Parameters: map[string]interface{}{
			"id":              event.ID,
			"timestamp":       event.Timestamp,
			"eventType":       event.EventType,
			"status":          event.Status,
			"errorMessage":    event.ErrorMessage,
			"containerIssues": event.ContainerIssues,
			"configChanged":   event.ConfigChanged,
			"statusChanged":   event.StatusChanged,
			"replicasChanged": event.ReplicasChanged,
			"impactScore":     event.ImpactScore,
			"data":            event.Data,
		},
	}
}

// CreateK8sEventQuery creates a query to insert a K8sEvent node
func CreateK8sEventQuery(event K8sEvent) GraphQuery {
	return GraphQuery{
		Query: `
			MERGE (e:K8sEvent {id: $id})
			ON CREATE SET
				e.timestamp = $timestamp,
				e.reason = $reason,
				e.message = $message,
				e.type = $type,
				e.count = $count,
				e.source = $source
		`,
		Parameters: map[string]interface{}{
			"id":        event.ID,
			"timestamp": event.Timestamp,
			"reason":    event.Reason,
			"message":   event.Message,
			"type":      event.Type,
			"count":     event.Count,
			"source":    event.Source,
		},
	}
}

// CreateOwnsEdgeQuery creates an OWNS relationship between resources
func CreateOwnsEdgeQuery(ownerUID, ownedUID string, props OwnsEdge) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (owner:ResourceIdentity {uid: $ownerUID})
			MATCH (owned:ResourceIdentity {uid: $ownedUID})
			MERGE (owner)-[r:OWNS]->(owned)
			ON CREATE SET
				r.controller = $controller,
				r.blockOwnerDeletion = $blockOwnerDeletion
		`,
		Parameters: map[string]interface{}{
			"ownerUID":           ownerUID,
			"ownedUID":           ownedUID,
			"controller":         props.Controller,
			"blockOwnerDeletion": props.BlockOwnerDeletion,
		},
	}
}

// CreateChangedEdgeQuery creates a CHANGED relationship from resource to event
func CreateChangedEdgeQuery(resourceUID, eventID string, sequenceNumber int) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $resourceUID})
			MATCH (e:ChangeEvent {id: $eventID})
			MERGE (r)-[c:CHANGED {sequenceNumber: $sequenceNumber}]->(e)
		`,
		Parameters: map[string]interface{}{
			"resourceUID":    resourceUID,
			"eventID":        eventID,
			"sequenceNumber": sequenceNumber,
		},
	}
}

// CreatePrecededByEdgeQuery creates a PRECEDED_BY temporal ordering edge
func CreatePrecededByEdgeQuery(currentEventID, previousEventID string, durationMs int64) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (current:ChangeEvent {id: $currentEventID})
			MATCH (previous:ChangeEvent {id: $previousEventID})
			MERGE (current)-[p:PRECEDED_BY {durationMs: $durationMs}]->(previous)
		`,
		Parameters: map[string]interface{}{
			"currentEventID":  currentEventID,
			"previousEventID": previousEventID,
			"durationMs":      durationMs,
		},
	}
}

// CreateTriggeredByEdgeQuery creates a TRIGGERED_BY causality edge
func CreateTriggeredByEdgeQuery(effectEventID, causeEventID string, props TriggeredByEdge) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (effect:ChangeEvent {id: $effectEventID})
			MATCH (cause:ChangeEvent {id: $causeEventID})
			MERGE (effect)-[t:TRIGGERED_BY]->(cause)
			ON CREATE SET
				t.confidence = $confidence,
				t.lagMs = $lagMs,
				t.reason = $reason
		`,
		Parameters: map[string]interface{}{
			"effectEventID": effectEventID,
			"causeEventID":  causeEventID,
			"confidence":    props.Confidence,
			"lagMs":         props.LagMs,
			"reason":        props.Reason,
		},
	}
}

// CreateEmittedEventEdgeQuery creates an EMITTED_EVENT relationship
func CreateEmittedEventEdgeQuery(resourceUID, k8sEventID string) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $resourceUID})
			MATCH (e:K8sEvent {id: $k8sEventID})
			MERGE (r)-[:EMITTED_EVENT]->(e)
		`,
		Parameters: map[string]interface{}{
			"resourceUID": resourceUID,
			"k8sEventID":  k8sEventID,
		},
	}
}

// CreateScheduledOnEdgeQuery creates a SCHEDULED_ON relationship (Pod → Node)
func CreateScheduledOnEdgeQuery(podUID, nodeUID string, props ScheduledOnEdge) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (pod:ResourceIdentity {uid: $podUID})
			MATCH (node:ResourceIdentity {uid: $nodeUID})
			MERGE (pod)-[r:SCHEDULED_ON]->(node)
			SET r.scheduledAt = $scheduledAt,
			    r.terminatedAt = $terminatedAt
		`,
		Parameters: map[string]interface{}{
			"podUID":       podUID,
			"nodeUID":      nodeUID,
			"scheduledAt":  props.ScheduledAt,
			"terminatedAt": props.TerminatedAt,
		},
	}
}

// CreateUsesServiceAccountEdgeQuery creates a USES_SERVICE_ACCOUNT relationship (Pod → ServiceAccount)
func CreateUsesServiceAccountEdgeQuery(podUID, serviceAccountUID string) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (pod:ResourceIdentity {uid: $podUID})
			MATCH (sa:ResourceIdentity {uid: $serviceAccountUID})
			MERGE (pod)-[:USES_SERVICE_ACCOUNT]->(sa)
		`,
		Parameters: map[string]interface{}{
			"podUID":            podUID,
			"serviceAccountUID": serviceAccountUID,
		},
	}
}

// CreateBindsRoleEdgeQuery creates a BINDS_ROLE relationship (RoleBinding/ClusterRoleBinding → Role/ClusterRole)
func CreateBindsRoleEdgeQuery(bindingUID, roleUID string, props BindsRoleEdge) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (binding:ResourceIdentity {uid: $bindingUID})
			MATCH (role:ResourceIdentity {uid: $roleUID})
			MERGE (binding)-[r:BINDS_ROLE]->(role)
			SET r.roleKind = $roleKind,
			    r.roleName = $roleName
		`,
		Parameters: map[string]interface{}{
			"bindingUID": bindingUID,
			"roleUID":    roleUID,
			"roleKind":   props.RoleKind,
			"roleName":   props.RoleName,
		},
	}
}

// CreateGrantsToEdgeQuery creates a GRANTS_TO relationship (RoleBinding/ClusterRoleBinding → Subject)
func CreateGrantsToEdgeQuery(bindingUID, subjectUID string, props GrantsToEdge) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (binding:ResourceIdentity {uid: $bindingUID})
			MATCH (subject:ResourceIdentity {uid: $subjectUID})
			MERGE (binding)-[r:GRANTS_TO]->(subject)
			SET r.subjectKind = $subjectKind,
			    r.subjectName = $subjectName,
			    r.subjectNamespace = $subjectNamespace
		`,
		Parameters: map[string]interface{}{
			"bindingUID":        bindingUID,
			"subjectUID":        subjectUID,
			"subjectKind":       props.SubjectKind,
			"subjectName":       props.SubjectName,
			"subjectNamespace":  props.SubjectNamespace,
		},
	}
}

// CreateSelectsEdgeQuery creates a SELECTS relationship (Service → Pod)
func CreateSelectsEdgeQuery(serviceUID, podUID string, props SelectsEdge) GraphQuery {
	// Serialize selectorLabels to JSON
	selectorLabelsJSON, _ := json.Marshal(props.SelectorLabels)
	
	return GraphQuery{
		Query: `
			MATCH (service:ResourceIdentity {uid: $serviceUID})
			MATCH (pod:ResourceIdentity {uid: $podUID})
			MERGE (service)-[r:SELECTS]->(pod)
			SET r.selectorLabels = $selectorLabels
		`,
		Parameters: map[string]interface{}{
			"serviceUID":     serviceUID,
			"podUID":         podUID,
			"selectorLabels": string(selectorLabelsJSON),
		},
	}
}

// FindResourceByUIDQuery returns a query to find a ResourceIdentity by UID
func FindResourceByUIDQuery(uid string) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $uid})
			RETURN r
		`,
		Parameters: map[string]interface{}{
			"uid": uid,
		},
	}
}

// FindResourceWithRelationshipsQuery returns a resource and all its related nodes and edges
// This is useful for getting the full context around a resource (e.g., Pod → ReplicaSet → Deployment, Pod → Node, etc.)
func FindResourceWithRelationshipsQuery(uid string) GraphQuery {
	return GraphQuery{
		Query: `
			// Find the target resource
			MATCH (resource:ResourceIdentity {uid: $uid})

			// Get all outgoing relationships and connected nodes
			OPTIONAL MATCH (resource)-[outRel]->(outNode)

			// Get all incoming relationships and connected nodes
			OPTIONAL MATCH (inNode)-[inRel]->(resource)

			// Return everything
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
		Parameters: map[string]interface{}{
			"uid": uid,
		},
	}
}

// FindResourceTopologyQuery returns only ResourceIdentity relationships
// This gives you the resource topology (ownership, scheduling, references, etc.) without events
func FindResourceTopologyQuery(uid string) GraphQuery {
	return GraphQuery{
		Query: `
			// Find the target resource
			MATCH (resource:ResourceIdentity {uid: $uid})

			// Get all outgoing relationships to other ResourceIdentity nodes
			OPTIONAL MATCH (resource)-[outRel]->(outNode:ResourceIdentity)

			// Get all incoming relationships from other ResourceIdentity nodes
			OPTIONAL MATCH (inNode:ResourceIdentity)-[inRel]->(resource)

			// Return the topology
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
		Parameters: map[string]interface{}{
			"uid": uid,
		},
	}
}

// FindChangeEventsByResourceQuery returns all ChangeEvents for a resource
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

// FindRootCauseQuery traces backward from a failure to find likely root causes
func FindRootCauseQuery(resourceUID string, failureTimestamp int64, maxDepth int, minConfidence float64) GraphQuery {
	// Allow 5 minute tolerance for timestamp matching (increased from 1 minute)
	toleranceNs := int64(300_000_000_000)
	// Look back 10 minutes for manager changes
	lookbackNs := int64(600_000_000_000)

	return GraphQuery{
		Query: fmt.Sprintf(`
			MATCH (failedResource:ResourceIdentity {uid: $resourceUID})
			      -[:CHANGED]->(failureEvent:ChangeEvent)
			WHERE failureEvent.timestamp <= $failureTimestamp + $tolerance
			  AND failureEvent.timestamp >= $failureTimestamp - $tolerance

			// Option 1: Follow causality links (TRIGGERED_BY)
			OPTIONAL MATCH causalityPath = (failureEvent)<-[:TRIGGERED_BY*1..%d]-(causeEvent:ChangeEvent)
			WHERE ALL(rel IN relationships(causalityPath) WHERE rel.confidence >= $minConfidence)

			WITH failedResource, failureEvent, causeEvent, causalityPath

			// Option 2: Check for managers (HelmRelease, etc.) via ownership + MANAGES
			// Collect all owners in the ownership chain
			OPTIONAL MATCH (failedResource)<-[:OWNS*1..3]-(owner:ResourceIdentity)
			WITH failedResource, failureEvent, causeEvent, causalityPath, collect(DISTINCT owner) as allOwners

			// For each owner, check if there's a manager
			UNWIND CASE WHEN size(allOwners) > 0 THEN allOwners ELSE [null] END as owner
			OPTIONAL MATCH (manager:ResourceIdentity)-[manages:MANAGES]->(owner)
			WHERE manages.confidence >= $minConfidence
			OPTIONAL MATCH (manager)-[:CHANGED]->(managerEvent:ChangeEvent)
			WHERE managerEvent.timestamp <= $failureTimestamp
			  AND managerEvent.timestamp >= $failureTimestamp - $lookback

			// Combine both options: prefer causality-based causes, but also include managers
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

// CalculateBlastRadiusQuery finds all resources affected by a change
func CalculateBlastRadiusQuery(resourceUID string, changeTimestamp int64, timeWindowMs int64, relationshipTypes []string) GraphQuery {
	// Convert relationship types to Cypher pattern
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

// DeleteOldChangeEventsQuery removes ChangeEvent nodes older than cutoff
func DeleteOldChangeEventsQuery(cutoffNs int64) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (e:ChangeEvent)
			WHERE e.timestamp < $cutoffNs
			DETACH DELETE e
		`,
		Parameters: map[string]interface{}{
			"cutoffNs": cutoffNs,
		},
	}
}

// DeleteOldK8sEventsQuery removes K8sEvent nodes older than cutoff
func DeleteOldK8sEventsQuery(cutoffNs int64) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (e:K8sEvent)
			WHERE e.timestamp < $cutoffNs
			DETACH DELETE e
		`,
		Parameters: map[string]interface{}{
			"cutoffNs": cutoffNs,
		},
	}
}

// GetGraphStatsQuery returns comprehensive graph statistics
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

// CreateReferencesSpecEdgeQuery creates a REFERENCES_SPEC relationship
func CreateReferencesSpecEdgeQuery(sourceUID, targetUID string, props ReferencesSpecEdge) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (source:ResourceIdentity {uid: $sourceUID})
			MATCH (target:ResourceIdentity {uid: $targetUID})
			MERGE (source)-[r:REFERENCES_SPEC]->(target)
			ON CREATE SET
				r.fieldPath = $fieldPath,
				r.refKind = $refKind,
				r.refName = $refName,
				r.refNamespace = $refNamespace
			ON MATCH SET
				r.fieldPath = $fieldPath,
				r.refKind = $refKind,
				r.refName = $refName,
				r.refNamespace = $refNamespace
		`,
		Parameters: map[string]interface{}{
			"sourceUID":    sourceUID,
			"targetUID":    targetUID,
			"fieldPath":    props.FieldPath,
			"refKind":      props.RefKind,
			"refName":      props.RefName,
			"refNamespace": props.RefNamespace,
		},
	}
}

// CreateManagesEdgeQuery creates a MANAGES relationship with confidence
func CreateManagesEdgeQuery(managerUID, managedUID string, props ManagesEdge) GraphQuery {
	evidenceJSON, _ := json.Marshal(props.Evidence)

	return GraphQuery{
		Query: `
			MATCH (manager:ResourceIdentity {uid: $managerUID})
			MATCH (managed:ResourceIdentity {uid: $managedUID})
			MERGE (manager)-[r:MANAGES]->(managed)
			ON CREATE SET
				r.confidence = $confidence,
				r.evidence = $evidence,
				r.firstObserved = $firstObserved,
				r.lastValidated = $lastValidated,
				r.validationState = $validationState
			ON MATCH SET
				r.confidence = $confidence,
				r.evidence = $evidence,
				r.lastValidated = $lastValidated,
				r.validationState = $validationState
		`,
		Parameters: map[string]interface{}{
			"managerUID":      managerUID,
			"managedUID":      managedUID,
			"confidence":      props.Confidence,
			"evidence":        string(evidenceJSON),
			"firstObserved":   props.FirstObserved,
			"lastValidated":   props.LastValidated,
			"validationState": string(props.ValidationState),
		},
	}
}

// CreateAnnotatesEdgeQuery creates an ANNOTATES relationship
func CreateAnnotatesEdgeQuery(sourceUID, targetUID string, props AnnotatesEdge) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (source:ResourceIdentity {uid: $sourceUID})
			MATCH (target:ResourceIdentity {uid: $targetUID})
			MERGE (source)-[r:ANNOTATES]->(target)
			ON CREATE SET
				r.annotationKey = $annotationKey,
				r.annotationValue = $annotationValue,
				r.confidence = $confidence
			ON MATCH SET
				r.annotationKey = $annotationKey,
				r.annotationValue = $annotationValue,
				r.confidence = $confidence
		`,
		Parameters: map[string]interface{}{
			"sourceUID":       sourceUID,
			"targetUID":       targetUID,
			"annotationKey":   props.AnnotationKey,
			"annotationValue": props.AnnotationValue,
			"confidence":      props.Confidence,
		},
	}
}

// CreateCreatesObservedEdgeQuery creates a CREATES_OBSERVED relationship
func CreateCreatesObservedEdgeQuery(sourceUID, targetUID string, props CreatesObservedEdge) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (source:ResourceIdentity {uid: $sourceUID})
			MATCH (target:ResourceIdentity {uid: $targetUID})
			MERGE (source)-[r:CREATES_OBSERVED]->(target)
			ON CREATE SET
				r.confidence = $confidence,
				r.observedLagMs = $observedLagMs,
				r.reconcileEventId = $reconcileEventId,
				r.evidence = $evidence
			ON MATCH SET
				r.confidence = $confidence,
				r.observedLagMs = $observedLagMs,
				r.reconcileEventId = $reconcileEventId,
				r.evidence = $evidence
		`,
		Parameters: map[string]interface{}{
			"sourceUID":        sourceUID,
			"targetUID":        targetUID,
			"confidence":       props.Confidence,
			"observedLagMs":    props.ObservedLagMs,
			"reconcileEventId": props.ReconcileEventID,
			"evidence":         props.Evidence,
		},
	}
}

// FindManagedResourcesQuery finds all resources managed by a CR
func FindManagedResourcesQuery(crUID string, minConfidence float64) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (cr:ResourceIdentity {uid: $crUID})
			      -[manages:MANAGES]->(managed:ResourceIdentity)
			WHERE manages.confidence >= $minConfidence
			  AND managed.deleted = false
			RETURN managed, manages
			ORDER BY manages.confidence DESC
		`,
		Parameters: map[string]interface{}{
			"crUID":         crUID,
			"minConfidence": minConfidence,
		},
	}
}

// FindStaleInferredEdgesQuery finds edges needing revalidation
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
		Parameters: map[string]interface{}{
			"cutoffTimestamp": cutoffTimestamp,
		},
	}
}
