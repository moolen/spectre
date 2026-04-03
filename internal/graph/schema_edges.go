package graph

import "encoding/json"

// CreateOwnsEdgeQuery creates an OWNS relationship between resources.
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

// CreateChangedEdgeQuery creates a CHANGED relationship from resource to event.
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

// CreatePrecededByEdgeQuery creates a PRECEDED_BY temporal ordering edge.
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

// CreateTriggeredByEdgeQuery creates a TRIGGERED_BY causality edge.
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

// CreateEmittedEventEdgeQuery creates an EMITTED_EVENT relationship.
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

// CreateScheduledOnEdgeQuery creates a SCHEDULED_ON relationship.
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

// CreateUsesServiceAccountEdgeQuery creates a USES_SERVICE_ACCOUNT relationship.
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

// CreateBindsRoleEdgeQuery creates a BINDS_ROLE relationship.
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

// CreateGrantsToEdgeQuery creates a GRANTS_TO relationship.
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

// CreateSelectsEdgeQuery creates a SELECTS relationship.
func CreateSelectsEdgeQuery(serviceUID, podUID string, props SelectsEdge) GraphQuery {
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

// CreateReferencesSpecEdgeQuery creates a REFERENCES_SPEC relationship.
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

// CreateManagesEdgeQuery creates a MANAGES relationship with confidence.
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

// CreateCreatesObservedEdgeQuery creates a CREATES_OBSERVED relationship.
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

// CreateMountsEdgeQuery creates a MOUNTS edge between a Pod and a PVC.
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
