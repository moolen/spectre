package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// buildCypherMapLiteral builds a Cypher map literal from a Go map.
// Example output: {uid: 'abc', kind: 'Pod', deleted: false, count: 42}
func buildCypherMapLiteral(m map[string]interface{}) string {
	if len(m) == 0 {
		return "{}"
	}

	parts := make([]string, 0, len(m))
	for k, v := range m {
		var valStr string
		switch val := v.(type) {
		case string:
			valStr = "'" + escapeCypherString(val) + "'"
		case bool:
			valStr = strconv.FormatBool(val)
		case int:
			valStr = strconv.Itoa(val)
		case int64:
			valStr = strconv.FormatInt(val, 10)
		case float64:
			valStr = strconv.FormatFloat(val, 'f', -1, 64)
		case nil:
			valStr = "null"
		default:
			// Fallback: serialize to JSON string
			jsonBytes, _ := json.Marshal(val)
			valStr = "'" + escapeCypherString(string(jsonBytes)) + "'"
		}
		parts = append(parts, k+": "+valStr)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// buildCypherListLiteral builds a Cypher list literal from a slice of maps.
// Example output: [{uid: 'a'}, {uid: 'b'}]
func buildCypherListLiteral(items []map[string]interface{}) string {
	if len(items) == 0 {
		return "[]"
	}

	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = buildCypherMapLiteral(item)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

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

	// Build ON MATCH SET clause
	// Core identity properties (kind, apiGroup, version, namespace, name) are always set
	// because they are immutable for a given UID and must be populated when upgrading
	// a placeholder node (created by OWNS edge) to a full node.
	// FalkorDB may not handle "property doesn't exist" the same as "property IS NULL"
	// in CASE expressions, so we unconditionally set these immutable properties.
	if resource.Deleted {
		// This is a deletion - always update to mark as deleted
		query += `
		ON MATCH SET
			r.kind = $kind,
			r.apiGroup = $apiGroup,
			r.version = $version,
			r.namespace = $namespace,
			r.name = $name,
			r.firstSeen = CASE WHEN r.firstSeen IS NULL THEN $firstSeen ELSE r.firstSeen END,
			r.labels = $labels,
			r.lastSeen = $lastSeen,
			r.deleted = true,
			r.deletedAt = $deletedAt
		`
	} else {
		// This is not a deletion - only update if not already deleted
		// Always set r.deleted = false for placeholder nodes (created by OWNS edge)
		// that don't have deleted set yet. Without this, the Timeline query's
		// WHERE (NOT r.deleted ...) filters out nodes where r.deleted is NULL.
		query += `
		ON MATCH SET
			r.kind = $kind,
			r.apiGroup = $apiGroup,
			r.version = $version,
			r.namespace = $namespace,
			r.name = $name,
			r.deleted = CASE WHEN r.deleted IS NULL THEN false ELSE r.deleted END,
			r.firstSeen = CASE WHEN r.firstSeen IS NULL THEN $firstSeen ELSE r.firstSeen END,
			r.labels = CASE WHEN NOT COALESCE(r.deleted, false) THEN $labels ELSE r.labels END,
			r.lastSeen = CASE WHEN NOT COALESCE(r.deleted, false) THEN $lastSeen ELSE r.lastSeen END
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
	// Serialize containerIssues to JSON string (FalkorDB doesn't handle Go slices)
	containerIssuesJSON := "[]"
	if len(event.ContainerIssues) > 0 {
		issuesBytes, _ := json.Marshal(event.ContainerIssues)
		containerIssuesJSON = string(issuesBytes)
	}

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
			"containerIssues": containerIssuesJSON,
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
// Uses MERGE for both nodes to handle cases where the owner node doesn't exist yet
// (e.g., when a Pod event arrives before its owning ReplicaSet event across batches).
// The owner node will be created as a placeholder and populated later when its event arrives.
func CreateOwnsEdgeQuery(ownerUID, ownedUID string, props OwnsEdge) GraphQuery {
	return GraphQuery{
		Query: `
			MERGE (owner:ResourceIdentity {uid: $ownerUID})
			MERGE (owned:ResourceIdentity {uid: $ownedUID})
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
			"bindingUID":       bindingUID,
			"subjectUID":       subjectUID,
			"subjectKind":      props.SubjectKind,
			"subjectName":      props.SubjectName,
			"subjectNamespace": props.SubjectNamespace,
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
func CalculateBlastRadiusQuery(resourceUID string, changeTimestamp, timeWindowMs int64, relationshipTypes []string) GraphQuery {
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

// CreateMountsEdgeQuery creates a MOUNTS edge between a Pod and a PVC
func CreateMountsEdgeQuery(podUID, pvcUID string, props MountsEdge) GraphQuery {
	return GraphQuery{
		Query: `
			MATCH (pod:ResourceIdentity {uid: $podUID})
			MATCH (pvc:ResourceIdentity {uid: $pvcUID})
			MERGE (pod)-[r:MOUNTS]->(pvc)
			SET r.volumeName = $volumeName,
				r.mountPath = $mountPath
		`,
		Parameters: map[string]interface{}{
			"podUID":     podUID,
			"pvcUID":     pvcUID,
			"volumeName": props.VolumeName,
			"mountPath":  props.MountPath,
		},
	}
}

// UpsertDashboardNode creates a query to insert or update a Dashboard node
// Uses MERGE to provide idempotency based on uid
func UpsertDashboardNode(dashboard DashboardNode) GraphQuery {
	// Serialize tags to JSON for storage
	tagsJSON := "[]"
	if dashboard.Tags != nil && len(dashboard.Tags) > 0 {
		tagsBytes, _ := json.Marshal(dashboard.Tags)
		tagsJSON = string(tagsBytes)
	}

	query := `
		MERGE (d:Dashboard {uid: $uid})
		ON CREATE SET
			d.title = $title,
			d.version = $version,
			d.tags = $tags,
			d.folder = $folder,
			d.url = $url,
			d.firstSeen = $firstSeen,
			d.lastSeen = $lastSeen
		ON MATCH SET
			d.title = $title,
			d.version = $version,
			d.tags = $tags,
			d.folder = $folder,
			d.url = $url,
			d.lastSeen = $lastSeen
	`

	return GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"uid":       dashboard.UID,
			"title":     dashboard.Title,
			"version":   dashboard.Version,
			"tags":      tagsJSON,
			"folder":    dashboard.Folder,
			"url":       dashboard.URL,
			"firstSeen": dashboard.FirstSeen,
			"lastSeen":  dashboard.LastSeen,
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

// =============================================================================
// Batch Query Builders - Phase 2 Optimization
// These functions use Cypher UNWIND to batch multiple operations into single queries,
// reducing the number of database round-trips from O(n) to O(1) per batch.
// =============================================================================

// BatchUpsertResourceIdentitiesQuery creates a single query to upsert multiple ResourceIdentity nodes.
// This reduces N individual MERGE queries to a single batched operation.
// Note: This uses a simplified approach - for deletions, use the original UpsertResourceIdentityQuery
// which has special handling to prevent un-deleting resources.
func BatchUpsertResourceIdentitiesQuery(resources []ResourceIdentity) GraphQuery {
	// Build parameters list for UNWIND as inline Cypher list literal.
	// FalkorDB Go SDK doesn't support slice parameters, so we embed the data directly.
	resourceParams := make([]map[string]interface{}, len(resources))
	for i, r := range resources {
		// Serialize labels to JSON
		labelsJSON := "{}"
		if len(r.Labels) > 0 {
			labelsBytes, _ := json.Marshal(r.Labels)
			labelsJSON = string(labelsBytes)
		}
		resourceParams[i] = map[string]interface{}{
			"uid":       r.UID,
			"kind":      r.Kind,
			"apiGroup":  r.APIGroup,
			"version":   r.Version,
			"namespace": r.Namespace,
			"name":      r.Name,
			"labels":    labelsJSON,
			"firstSeen": r.FirstSeen,
			"lastSeen":  r.LastSeen,
			"deleted":   r.Deleted,
			"deletedAt": r.DeletedAt,
		}
	}

	// Build inline Cypher list literal
	resourcesLiteral := buildCypherListLiteral(resourceParams)

	// Note: This batched version doesn't handle the special case where a resource
	// might already be deleted. For deletions, use individual queries to ensure
	// the deleted flag is set correctly regardless of previous state.
	query := fmt.Sprintf(`
		UNWIND %s AS r
		MERGE (n:ResourceIdentity {uid: r.uid})
		ON CREATE SET
			n.kind = r.kind,
			n.apiGroup = r.apiGroup,
			n.version = r.version,
			n.namespace = r.namespace,
			n.name = r.name,
			n.labels = r.labels,
			n.firstSeen = r.firstSeen,
			n.lastSeen = r.lastSeen,
			n.deleted = r.deleted,
			n.deletedAt = r.deletedAt
		ON MATCH SET
			n.kind = CASE WHEN n.kind IS NULL THEN r.kind ELSE n.kind END,
			n.apiGroup = CASE WHEN n.apiGroup IS NULL THEN r.apiGroup ELSE n.apiGroup END,
			n.version = CASE WHEN n.version IS NULL THEN r.version ELSE n.version END,
			n.namespace = CASE WHEN n.namespace IS NULL THEN r.namespace ELSE n.namespace END,
			n.name = CASE WHEN n.name IS NULL THEN r.name ELSE n.name END,
			n.firstSeen = CASE WHEN n.firstSeen IS NULL THEN r.firstSeen ELSE n.firstSeen END,
			n.labels = CASE WHEN NOT n.deleted THEN r.labels ELSE n.labels END,
			n.lastSeen = CASE WHEN NOT n.deleted THEN r.lastSeen ELSE n.lastSeen END
		RETURN count(n) as upsertedCount
	`, resourcesLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchCreateChangeEventsQuery creates a single query to insert multiple ChangeEvent nodes.
func BatchCreateChangeEventsQuery(events []ChangeEvent) GraphQuery {
	eventParams := make([]map[string]interface{}, len(events))
	for i, e := range events {
		// Serialize containerIssues to JSON string (FalkorDB doesn't handle Go slices)
		containerIssuesJSON := "[]"
		if len(e.ContainerIssues) > 0 {
			issuesBytes, _ := json.Marshal(e.ContainerIssues)
			containerIssuesJSON = string(issuesBytes)
		}

		eventParams[i] = map[string]interface{}{
			"id":              e.ID,
			"timestamp":       e.Timestamp,
			"eventType":       e.EventType,
			"status":          e.Status,
			"errorMessage":    e.ErrorMessage,
			"containerIssues": containerIssuesJSON,
			"configChanged":   e.ConfigChanged,
			"statusChanged":   e.StatusChanged,
			"replicasChanged": e.ReplicasChanged,
			"impactScore":     e.ImpactScore,
			"data":            e.Data,
		}
	}

	// Build inline Cypher list literal - FalkorDB Go SDK doesn't support slice parameters
	eventsLiteral := buildCypherListLiteral(eventParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MERGE (n:ChangeEvent {id: e.id})
		ON CREATE SET
			n.timestamp = e.timestamp,
			n.eventType = e.eventType,
			n.status = e.status,
			n.errorMessage = e.errorMessage,
			n.containerIssues = e.containerIssues,
			n.configChanged = e.configChanged,
			n.statusChanged = e.statusChanged,
			n.replicasChanged = e.replicasChanged,
			n.impactScore = e.impactScore,
			n.data = e.data
		RETURN count(n) as createdCount
	`, eventsLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchCreateK8sEventsQuery creates a single query to insert multiple K8sEvent nodes.
func BatchCreateK8sEventsQuery(events []K8sEvent) GraphQuery {
	eventParams := make([]map[string]interface{}, len(events))
	for i, e := range events {
		eventParams[i] = map[string]interface{}{
			"id":        e.ID,
			"timestamp": e.Timestamp,
			"reason":    e.Reason,
			"message":   e.Message,
			"type":      e.Type,
			"count":     e.Count,
			"source":    e.Source,
		}
	}

	// Build inline Cypher list literal - FalkorDB Go SDK doesn't support slice parameters
	eventsLiteral := buildCypherListLiteral(eventParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MERGE (n:K8sEvent {id: e.id})
		ON CREATE SET
			n.timestamp = e.timestamp,
			n.reason = e.reason,
			n.message = e.message,
			n.type = e.type,
			n.count = e.count,
			n.source = e.source
		RETURN count(n) as createdCount
	`, eventsLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchEdgeParams represents parameters for a single edge in a batch operation.
type BatchEdgeParams struct {
	FromUID    string
	ToUID      string
	Properties map[string]interface{}
}

// BatchCreateOwnsEdgesQuery creates multiple OWNS edges in a single query.
func BatchCreateOwnsEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, e := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":            e.FromUID,
			"toUID":              e.ToUID,
			"controller":         e.Properties["controller"],
			"blockOwnerDeletion": e.Properties["blockOwnerDeletion"],
		}
	}

	// Build inline Cypher list literal - FalkorDB Go SDK doesn't support slice parameters
	edgesLiteral := buildCypherListLiteral(edgeParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MATCH (owner:ResourceIdentity {uid: e.fromUID})
		MATCH (owned:ResourceIdentity {uid: e.toUID})
		MERGE (owner)-[r:OWNS]->(owned)
		ON CREATE SET
			r.controller = e.controller,
			r.blockOwnerDeletion = e.blockOwnerDeletion
		ON MATCH SET
			r.controller = e.controller,
			r.blockOwnerDeletion = e.blockOwnerDeletion
		RETURN count(r) as createdCount
	`, edgesLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchCreateChangedEdgesQuery creates multiple CHANGED edges in a single query.
func BatchCreateChangedEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, e := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":        e.FromUID,
			"toUID":          e.ToUID,
			"sequenceNumber": e.Properties["sequenceNumber"],
		}
	}

	edgesLiteral := buildCypherListLiteral(edgeParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MATCH (resource:ResourceIdentity {uid: e.fromUID})
		MATCH (event:ChangeEvent {id: e.toUID})
		MERGE (resource)-[r:CHANGED]->(event)
		ON CREATE SET r.sequenceNumber = e.sequenceNumber
		ON MATCH SET r.sequenceNumber = e.sequenceNumber
		RETURN count(r) as createdCount
	`, edgesLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchCreateSelectsEdgesQuery creates multiple SELECTS edges in a single query.
func BatchCreateSelectsEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, e := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":   e.FromUID,
			"toUID":     e.ToUID,
			"selector":  e.Properties["selector"],
			"matchType": e.Properties["matchType"],
		}
	}

	edgesLiteral := buildCypherListLiteral(edgeParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MATCH (selector:ResourceIdentity {uid: e.fromUID})
		MATCH (selected:ResourceIdentity {uid: e.toUID})
		MERGE (selector)-[r:SELECTS]->(selected)
		ON CREATE SET
			r.selector = e.selector,
			r.matchType = e.matchType
		ON MATCH SET
			r.selector = e.selector,
			r.matchType = e.matchType
		RETURN count(r) as createdCount
	`, edgesLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchCreateScheduledOnEdgesQuery creates multiple SCHEDULED_ON edges in a single query.
func BatchCreateScheduledOnEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, e := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":     e.FromUID,
			"toUID":       e.ToUID,
			"scheduledAt": e.Properties["scheduledAt"],
			"hostIP":      e.Properties["hostIP"],
		}
	}

	edgesLiteral := buildCypherListLiteral(edgeParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MATCH (pod:ResourceIdentity {uid: e.fromUID})
		MATCH (node:ResourceIdentity {uid: e.toUID})
		MERGE (pod)-[r:SCHEDULED_ON]->(node)
		ON CREATE SET
			r.scheduledAt = e.scheduledAt,
			r.hostIP = e.hostIP
		ON MATCH SET
			r.scheduledAt = e.scheduledAt,
			r.hostIP = e.hostIP
		RETURN count(r) as createdCount
	`, edgesLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchCreateMountsEdgesQuery creates multiple MOUNTS edges in a single query.
func BatchCreateMountsEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, e := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":   e.FromUID,
			"toUID":     e.ToUID,
			"mountPath": e.Properties["mountPath"],
			"readOnly":  e.Properties["readOnly"],
			"subPath":   e.Properties["subPath"],
		}
	}

	edgesLiteral := buildCypherListLiteral(edgeParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MATCH (pod:ResourceIdentity {uid: e.fromUID})
		MATCH (volume:ResourceIdentity {uid: e.toUID})
		MERGE (pod)-[r:MOUNTS]->(volume)
		ON CREATE SET
			r.mountPath = e.mountPath,
			r.readOnly = e.readOnly,
			r.subPath = e.subPath
		ON MATCH SET
			r.mountPath = e.mountPath,
			r.readOnly = e.readOnly,
			r.subPath = e.subPath
		RETURN count(r) as createdCount
	`, edgesLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchCreateReferencesSpecEdgesQuery creates multiple REFERENCES_SPEC edges in a single query.
func BatchCreateReferencesSpecEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, e := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":       e.FromUID,
			"toUID":         e.ToUID,
			"referenceType": e.Properties["referenceType"],
			"fieldPath":     e.Properties["fieldPath"],
		}
	}

	edgesLiteral := buildCypherListLiteral(edgeParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MATCH (source:ResourceIdentity {uid: e.fromUID})
		MATCH (target:ResourceIdentity {uid: e.toUID})
		MERGE (source)-[r:REFERENCES_SPEC]->(target)
		ON CREATE SET
			r.referenceType = e.referenceType,
			r.fieldPath = e.fieldPath
		ON MATCH SET
			r.referenceType = e.referenceType,
			r.fieldPath = e.fieldPath
		RETURN count(r) as createdCount
	`, edgesLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchCreateManagesEdgesQuery creates multiple MANAGES edges in a single query.
func BatchCreateManagesEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, e := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":         e.FromUID,
			"toUID":           e.ToUID,
			"confidence":      e.Properties["confidence"],
			"inferredAt":      e.Properties["inferredAt"],
			"reason":          e.Properties["reason"],
			"validationState": e.Properties["validationState"],
			"lastValidated":   e.Properties["lastValidated"],
		}
	}

	edgesLiteral := buildCypherListLiteral(edgeParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MATCH (cr:ResourceIdentity {uid: e.fromUID})
		MATCH (managed:ResourceIdentity {uid: e.toUID})
		MERGE (cr)-[r:MANAGES]->(managed)
		ON CREATE SET
			r.confidence = e.confidence,
			r.inferredAt = e.inferredAt,
			r.reason = e.reason,
			r.validationState = e.validationState,
			r.lastValidated = e.lastValidated
		ON MATCH SET
			r.confidence = e.confidence,
			r.inferredAt = e.inferredAt,
			r.reason = e.reason,
			r.validationState = e.validationState,
			r.lastValidated = e.lastValidated
		RETURN count(r) as createdCount
	`, edgesLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchCreateEmittedEventEdgesQuery creates multiple EMITTED_EVENT edges in a single query.
func BatchCreateEmittedEventEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, e := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID": e.FromUID,
			"toUID":   e.ToUID,
		}
	}

	edgesLiteral := buildCypherListLiteral(edgeParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MATCH (resource:ResourceIdentity {uid: e.fromUID})
		MATCH (event:K8sEvent {id: e.toUID})
		MERGE (resource)-[r:EMITTED_EVENT]->(event)
		RETURN count(r) as createdCount
	`, edgesLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchCreateUsesServiceAccountEdgesQuery creates multiple USES_SERVICE_ACCOUNT edges in a single query.
func BatchCreateUsesServiceAccountEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, e := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID": e.FromUID,
			"toUID":   e.ToUID,
		}
	}

	edgesLiteral := buildCypherListLiteral(edgeParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MATCH (pod:ResourceIdentity {uid: e.fromUID})
		MATCH (sa:ResourceIdentity {uid: e.toUID})
		MERGE (pod)-[r:USES_SERVICE_ACCOUNT]->(sa)
		RETURN count(r) as createdCount
	`, edgesLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchCreateBindsRoleEdgesQuery creates multiple BINDS_ROLE edges in a single query.
func BatchCreateBindsRoleEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, e := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":  e.FromUID,
			"toUID":    e.ToUID,
			"roleKind": e.Properties["roleKind"],
			"roleName": e.Properties["roleName"],
		}
	}

	edgesLiteral := buildCypherListLiteral(edgeParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MATCH (binding:ResourceIdentity {uid: e.fromUID})
		MATCH (role:ResourceIdentity {uid: e.toUID})
		MERGE (binding)-[r:BINDS_ROLE]->(role)
		ON CREATE SET
			r.roleKind = e.roleKind,
			r.roleName = e.roleName
		ON MATCH SET
			r.roleKind = e.roleKind,
			r.roleName = e.roleName
		RETURN count(r) as createdCount
	`, edgesLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchCreateGrantsToEdgesQuery creates multiple GRANTS_TO edges in a single query.
func BatchCreateGrantsToEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, e := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":     e.FromUID,
			"toUID":       e.ToUID,
			"subjectKind": e.Properties["subjectKind"],
			"subjectName": e.Properties["subjectName"],
		}
	}

	edgesLiteral := buildCypherListLiteral(edgeParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MATCH (binding:ResourceIdentity {uid: e.fromUID})
		MATCH (subject:ResourceIdentity {uid: e.toUID})
		MERGE (binding)-[r:GRANTS_TO]->(subject)
		ON CREATE SET
			r.subjectKind = e.subjectKind,
			r.subjectName = e.subjectName
		ON MATCH SET
			r.subjectKind = e.subjectKind,
			r.subjectName = e.subjectName
		RETURN count(r) as createdCount
	`, edgesLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchCreateCreatesObservedEdgesQuery creates multiple CREATES_OBSERVED edges in a single query.
func BatchCreateCreatesObservedEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, e := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":    e.FromUID,
			"toUID":      e.ToUID,
			"observedAt": e.Properties["observedAt"],
			"reason":     e.Properties["reason"],
		}
	}

	edgesLiteral := buildCypherListLiteral(edgeParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MATCH (cr:ResourceIdentity {uid: e.fromUID})
		MATCH (resource:ResourceIdentity {uid: e.toUID})
		MERGE (cr)-[r:CREATES_OBSERVED]->(resource)
		ON CREATE SET
			r.observedAt = e.observedAt,
			r.reason = e.reason
		ON MATCH SET
			r.observedAt = e.observedAt,
			r.reason = e.reason
		RETURN count(r) as createdCount
	`, edgesLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}

// BatchCreateTriggeredByEdgesQuery creates multiple TRIGGERED_BY edges in a single query.
func BatchCreateTriggeredByEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, e := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":    e.FromUID,
			"toUID":      e.ToUID,
			"confidence": e.Properties["confidence"],
			"lagMs":      e.Properties["lagMs"],
			"reason":     e.Properties["reason"],
		}
	}

	edgesLiteral := buildCypherListLiteral(edgeParams)

	query := fmt.Sprintf(`
		UNWIND %s AS e
		MATCH (effect:ChangeEvent {id: e.fromUID})
		MATCH (cause:ChangeEvent {id: e.toUID})
		MERGE (effect)-[r:TRIGGERED_BY]->(cause)
		ON CREATE SET
			r.confidence = e.confidence,
			r.lagMs = e.lagMs,
			r.reason = e.reason
		ON MATCH SET
			r.confidence = e.confidence,
			r.lagMs = e.lagMs,
			r.reason = e.reason
		RETURN count(r) as createdCount
	`, edgesLiteral)

	return GraphQuery{
		Query:      query,
		Parameters: nil,
	}
}
