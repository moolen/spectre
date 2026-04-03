package graph

import "fmt"

// BatchEdgeParams represents parameters for a single edge in a batch operation.
type BatchEdgeParams struct {
	FromUID    string
	ToUID      string
	Properties map[string]interface{}
}

// BatchCreateOwnsEdgesQuery creates multiple OWNS edges in a single query.
func BatchCreateOwnsEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":            edge.FromUID,
			"toUID":              edge.ToUID,
			"controller":         edge.Properties["controller"],
			"blockOwnerDeletion": edge.Properties["blockOwnerDeletion"],
		}
	}

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

	return GraphQuery{Query: query, Parameters: nil}
}

// BatchCreateChangedEdgesQuery creates multiple CHANGED edges in a single query.
func BatchCreateChangedEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":        edge.FromUID,
			"toUID":          edge.ToUID,
			"sequenceNumber": edge.Properties["sequenceNumber"],
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

	return GraphQuery{Query: query, Parameters: nil}
}

// BatchCreateSelectsEdgesQuery creates multiple SELECTS edges in a single query.
func BatchCreateSelectsEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":   edge.FromUID,
			"toUID":     edge.ToUID,
			"selector":  edge.Properties["selector"],
			"matchType": edge.Properties["matchType"],
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

	return GraphQuery{Query: query, Parameters: nil}
}

// BatchCreateScheduledOnEdgesQuery creates multiple SCHEDULED_ON edges in a single query.
func BatchCreateScheduledOnEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":     edge.FromUID,
			"toUID":       edge.ToUID,
			"scheduledAt": edge.Properties["scheduledAt"],
			"hostIP":      edge.Properties["hostIP"],
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

	return GraphQuery{Query: query, Parameters: nil}
}

// BatchCreateMountsEdgesQuery creates multiple MOUNTS edges in a single query.
func BatchCreateMountsEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":   edge.FromUID,
			"toUID":     edge.ToUID,
			"mountPath": edge.Properties["mountPath"],
			"readOnly":  edge.Properties["readOnly"],
			"subPath":   edge.Properties["subPath"],
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

	return GraphQuery{Query: query, Parameters: nil}
}

// BatchCreateReferencesSpecEdgesQuery creates multiple REFERENCES_SPEC edges in a single query.
func BatchCreateReferencesSpecEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":       edge.FromUID,
			"toUID":         edge.ToUID,
			"referenceType": edge.Properties["referenceType"],
			"fieldPath":     edge.Properties["fieldPath"],
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

	return GraphQuery{Query: query, Parameters: nil}
}

// BatchCreateManagesEdgesQuery creates multiple MANAGES edges in a single query.
func BatchCreateManagesEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":         edge.FromUID,
			"toUID":           edge.ToUID,
			"confidence":      edge.Properties["confidence"],
			"inferredAt":      edge.Properties["inferredAt"],
			"reason":          edge.Properties["reason"],
			"validationState": edge.Properties["validationState"],
			"lastValidated":   edge.Properties["lastValidated"],
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

	return GraphQuery{Query: query, Parameters: nil}
}

// BatchCreateEmittedEventEdgesQuery creates multiple EMITTED_EVENT edges in a single query.
func BatchCreateEmittedEventEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID": edge.FromUID,
			"toUID":   edge.ToUID,
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

	return GraphQuery{Query: query, Parameters: nil}
}

// BatchCreateUsesServiceAccountEdgesQuery creates multiple USES_SERVICE_ACCOUNT edges in a single query.
func BatchCreateUsesServiceAccountEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID": edge.FromUID,
			"toUID":   edge.ToUID,
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

	return GraphQuery{Query: query, Parameters: nil}
}

// BatchCreateBindsRoleEdgesQuery creates multiple BINDS_ROLE edges in a single query.
func BatchCreateBindsRoleEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":  edge.FromUID,
			"toUID":    edge.ToUID,
			"roleKind": edge.Properties["roleKind"],
			"roleName": edge.Properties["roleName"],
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

	return GraphQuery{Query: query, Parameters: nil}
}

// BatchCreateGrantsToEdgesQuery creates multiple GRANTS_TO edges in a single query.
func BatchCreateGrantsToEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":     edge.FromUID,
			"toUID":       edge.ToUID,
			"subjectKind": edge.Properties["subjectKind"],
			"subjectName": edge.Properties["subjectName"],
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

	return GraphQuery{Query: query, Parameters: nil}
}

// BatchCreateCreatesObservedEdgesQuery creates multiple CREATES_OBSERVED edges in a single query.
func BatchCreateCreatesObservedEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":    edge.FromUID,
			"toUID":      edge.ToUID,
			"observedAt": edge.Properties["observedAt"],
			"reason":     edge.Properties["reason"],
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

	return GraphQuery{Query: query, Parameters: nil}
}

// BatchCreateTriggeredByEdgesQuery creates multiple TRIGGERED_BY edges in a single query.
func BatchCreateTriggeredByEdgesQuery(edges []BatchEdgeParams) GraphQuery {
	edgeParams := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		edgeParams[i] = map[string]interface{}{
			"fromUID":    edge.FromUID,
			"toUID":      edge.ToUID,
			"confidence": edge.Properties["confidence"],
			"lagMs":      edge.Properties["lagMs"],
			"reason":     edge.Properties["reason"],
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

	return GraphQuery{Query: query, Parameters: nil}
}
