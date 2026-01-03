package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/graph"
)

// ResourceWithDistance represents a resource in the ownership chain with its distance from the symptom
type ResourceWithDistance struct {
	Resource graph.ResourceIdentity
	Distance int
}

// ManagerData contains manager information for a resource
type ManagerData struct {
	Manager     graph.ResourceIdentity
	ManagesEdge graph.ManagesEdge
}

// RelatedResourceData contains information about a related resource
type RelatedResourceData struct {
	Resource           graph.ResourceIdentity
	RelationshipType   string
	Events             []ChangeEventInfo
	ReferenceTargetUID string // For INGRESS_REF, the UID of the Service that the Ingress references
}

// getOwnershipChain retrieves the ownership chain from the symptom resource up to 3 levels
func (a *RootCauseAnalyzer) getOwnershipChain(ctx context.Context, symptomUID string) ([]ResourceWithDistance, error) {
	// First, get the symptom resource
	symptomQuery := graph.GraphQuery{
		Timeout: 5000,
		Query: `
			MATCH (symptom:ResourceIdentity {uid: $symptomUID})
			RETURN symptom as resource, 0 as distance
		`,
		Parameters: map[string]interface{}{
			"symptomUID": symptomUID,
		},
	}

	a.logger.Debug("getOwnershipChain: getting symptom resource %s", symptomUID)
	symptomResult, err := a.graphClient.ExecuteQuery(ctx, symptomQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query symptom resource: %w", err)
	}

	chain := []ResourceWithDistance{}

	// Parse symptom
	for _, row := range symptomResult.Rows {
		if len(row) < 2 {
			continue
		}
		resourceProps, err := graph.ParseNodeFromResult(row[0])
		if err != nil || resourceProps == nil || len(resourceProps) == 0 {
			continue
		}
		resource := graph.ParseResourceIdentityFromNode(resourceProps)
		chain = append(chain, ResourceWithDistance{
			Resource: resource,
			Distance: 0,
		})
	}

	if len(chain) == 0 {
		return nil, fmt.Errorf("symptom resource not found: %s", symptomUID)
	}

	// Now get owners
	ownersQuery := graph.GraphQuery{
		Timeout: 5000,
		Query: `
			MATCH (symptom:ResourceIdentity {uid: $symptomUID})
			MATCH path = (symptom)<-[:OWNS*1..3]-(owner:ResourceIdentity)
			RETURN DISTINCT owner as resource, length(path) as distance
			ORDER BY distance ASC
		`,
		Parameters: map[string]interface{}{
			"symptomUID": symptomUID,
		},
	}

	a.logger.Debug("getOwnershipChain: getting owners for %s", symptomUID)
	ownersResult, err := a.graphClient.ExecuteQuery(ctx, ownersQuery)
	if err != nil {
		// If there are no owners, this query might fail or return empty - that's OK
		a.logger.Debug("getOwnershipChain: no owners found (or query error): %v", err)
		return chain, nil
	}

	seenUIDs := make(map[string]bool)
	seenUIDs[chain[0].Resource.UID] = true

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
		if d, ok := row[1].(int64); ok {
			distance = int(d)
		} else if d, ok := row[1].(float64); ok {
			distance = int(d)
		}

		chain = append(chain, ResourceWithDistance{
			Resource: resource,
			Distance: distance,
		})
	}

	a.logger.Debug("getOwnershipChain: found %d resources in chain", len(chain))
	return chain, nil
}

// getManagers retrieves manager relationships for the given resources
func (a *RootCauseAnalyzer) getManagers(ctx context.Context, resourceUIDs []string) (map[string]*ManagerData, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string]*ManagerData), nil
	}

	query := graph.GraphQuery{
		Timeout: 5000,
		Query: `
			UNWIND $resourceUIDs as uid
			MATCH (resource:ResourceIdentity {uid: uid})
			OPTIONAL MATCH (manager:ResourceIdentity)-[manages:MANAGES]->(resource)
			WHERE manages.confidence >= 0.5
			RETURN resource.uid as resourceUID, manager, manages
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs": resourceUIDs,
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

// getRelatedResources retrieves resources related through various relationship types
func (a *RootCauseAnalyzer) getRelatedResources(ctx context.Context, resourceUIDs []string) (map[string][]RelatedResourceData, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string][]RelatedResourceData), nil
	}

	query := graph.GraphQuery{
		Timeout: 5000,
		Query: `
			// Direct relationships from resources
			UNWIND $resourceUIDs as uid
			MATCH (resource:ResourceIdentity {uid: uid})
			OPTIONAL MATCH (resource)-[refSpec:REFERENCES_SPEC]->(referencedResource:ResourceIdentity)
			OPTIONAL MATCH (resource)-[scheduledOn:SCHEDULED_ON]->(node:ResourceIdentity)
			OPTIONAL MATCH (resource)-[usesSA:USES_SERVICE_ACCOUNT]->(sa:ResourceIdentity)
			OPTIONAL MATCH (selector:ResourceIdentity)-[selects:SELECTS]->(resource)
			WHERE selector.kind IN ['Service', 'NetworkPolicy']

			// Find Ingresses that reference Services that select this resource
			OPTIONAL MATCH (ingress:ResourceIdentity)-[ref:REFERENCES_SPEC]->(selector)
			WHERE ingress.kind = 'Ingress' AND selector.kind = 'Service'

			// Get RoleBindings that grant to service accounts used by this resource
			OPTIONAL MATCH (rb:ResourceIdentity)-[grantsTo:GRANTS_TO]->(sa)
			WHERE sa IS NOT NULL

			RETURN resource.uid as resourceUID,
			       referencedResource, 'REFERENCES_SPEC' as refSpecType,
			       node, 'SCHEDULED_ON' as scheduledOnType,
			       sa, 'USES_SERVICE_ACCOUNT' as usesSAType,
			       selector, 'SELECTS' as selectsType,
			       rb, 'GRANTS_TO' as grantsToType,
			       ingress, 'INGRESS_REF' as ingressRefType
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs": resourceUIDs,
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
		if len(row) < 13 {
			a.logger.Debug("getRelatedResources: skipping row with < 13 columns: %d", len(row))
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
		addRelated(1, "REFERENCES_SPEC")      // referencedResource (outgoing from resource)
		addRelated(3, "SCHEDULED_ON")         // node
		addRelated(5, "USES_SERVICE_ACCOUNT") // sa
		addRelated(7, "SELECTS")              // selector (incoming to resource, reversed in causal_chain.go)
		addRelated(9, "GRANTS_TO")            // rb

		// Special handling for INGRESS_REF to also capture the Service UID
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
					if existing.Resource.UID == ingress.UID && existing.RelationshipType == "INGRESS_REF" {
						isDuplicate = true
						break
					}
				}

				if !isDuplicate {
					related[resourceUID] = append(related[resourceUID], RelatedResourceData{
						Resource:           ingress,
						RelationshipType:   "INGRESS_REF",
						Events:             []ChangeEventInfo{},
						ReferenceTargetUID: serviceUID,
					})
					a.logger.Debug("getRelatedResources: SUCCESS adding %s/%s (type=INGRESS_REF, target=%s) to resource %s",
						ingress.Kind, ingress.Name, serviceUID, resourceUID)
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

// getChangeEvents retrieves change events for resources within the time window
// The query ensures ALL configChanged events are included (important for root cause)
// plus the most recent events for context, avoiding truncation of important config changes.
func (a *RootCauseAnalyzer) getChangeEvents(
	ctx context.Context,
	resourceUIDs []string,
	failureTimestamp int64,
	lookbackNs int64,
) (map[string][]ChangeEventInfo, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string][]ChangeEventInfo), nil
	}

	// Query that combines:
	// 1. ALL events with configChanged=true (critical for root cause analysis)
	// 2. Up to 10 most recent events (for status context)
	// This ensures we never miss the important config change that triggered a failure,
	// even if there are many subsequent status-only events.
	query := graph.GraphQuery{
		Timeout: 5000,
		Query: `
			UNWIND $resourceUIDs as uid
			MATCH (resource:ResourceIdentity {uid: uid})
			OPTIONAL MATCH (resource)-[:CHANGED]->(event:ChangeEvent)
			WHERE event.timestamp <= $failureTimestamp
			  AND event.timestamp >= $failureTimestamp - $lookback
			WITH resource.uid as resourceUID, event
			ORDER BY event.timestamp DESC
			WITH resourceUID, collect(event) as allEvents
			WITH resourceUID,
			     [e IN allEvents WHERE e.configChanged = true] as configEvents,
			     allEvents[0..10] as recentEvents
			WITH resourceUID,
			     configEvents + [e IN recentEvents WHERE NOT e.id IN [ce IN configEvents | ce.id]] as combinedEvents
			UNWIND combinedEvents as event
			WITH resourceUID, event
			ORDER BY event.timestamp DESC
			RETURN resourceUID, collect(DISTINCT event) as events
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs":     resourceUIDs,
			"failureTimestamp": failureTimestamp,
			"lookback":         lookbackNs,
		},
	}

	a.logger.Debug("getChangeEvents: executing query for %d resources with lookback %d ns (%.1f minutes)",
		len(resourceUIDs), lookbackNs, float64(lookbackNs)/float64(time.Minute.Nanoseconds()))
	result, err := a.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query change events: %w", err)
	}

	events := make(map[string][]ChangeEventInfo)

	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}

		resourceUID, ok := row[0].(string)
		if !ok {
			continue
		}

		events[resourceUID] = []ChangeEventInfo{}

		// Parse events array
		if row[1] == nil {
			continue
		}

		eventList, ok := row[1].([]interface{})
		if !ok {
			continue
		}

		// Track seen event IDs to deduplicate (safety check)
		seenEventIDs := make(map[string]bool)

		for _, eventNode := range eventList {
			if eventNode == nil {
				continue
			}

			eventProps, err := graph.ParseNodeFromResult(eventNode)
			if err != nil || eventProps == nil || len(eventProps) == 0 {
				continue
			}

			event := graph.ParseChangeEventFromNode(eventProps)

			// Deduplicate by event ID (safety check)
			if seenEventIDs[event.ID] {
				a.logger.Debug("getChangeEvents: skipping duplicate event %s for resource %s", event.ID, resourceUID)
				continue
			}
			seenEventIDs[event.ID] = true

			// Get kind for status inference (we don't have it here, use empty)
			status := analyzer.InferStatusFromResource("", json.RawMessage(event.Data), event.EventType)

			changeEvent := ChangeEventInfo{
				EventID:       event.ID,
				Timestamp:     time.Unix(0, event.Timestamp),
				EventType:     event.EventType,
				Status:        status,
				ConfigChanged: event.ConfigChanged,
				StatusChanged: event.StatusChanged,
				Description:   fmt.Sprintf("%s event", event.EventType),
				Data:          []byte(event.Data),
			}

			events[resourceUID] = append(events[resourceUID], changeEvent)
		}
	}

	// Log event counts for debugging
	for uid, evts := range events {
		configCount := 0
		for _, e := range evts {
			if e.ConfigChanged {
				configCount++
			}
		}
		a.logger.Debug("getChangeEvents: resource %s has %d events (%d with configChanged=true)", uid, len(evts), configCount)
	}
	a.logger.Debug("getChangeEvents: found events for %d resources", len(events))
	return events, nil
}

// getK8sEvents retrieves Kubernetes events for resources within the time window
func (a *RootCauseAnalyzer) getK8sEvents(
	ctx context.Context,
	resourceUIDs []string,
	failureTimestamp int64,
	lookbackNs int64,
) (map[string][]K8sEventInfo, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string][]K8sEventInfo), nil
	}

	query := graph.GraphQuery{
		Timeout: 5000,
		Query: `
			UNWIND $resourceUIDs as uid
			MATCH (resource:ResourceIdentity {uid: uid})
			OPTIONAL MATCH (resource)-[:EMITTED_EVENT]->(k8sEvent:K8sEvent)
			WHERE k8sEvent.timestamp <= $failureTimestamp
			  AND k8sEvent.timestamp >= $failureTimestamp - $lookback
			WITH resource.uid as resourceUID, k8sEvent
			ORDER BY k8sEvent.timestamp DESC
			WITH resourceUID, collect(k8sEvent)[0..20] as events
			RETURN resourceUID, events
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs":     resourceUIDs,
			"failureTimestamp": failureTimestamp,
			"lookback":         lookbackNs,
		},
	}

	a.logger.Debug("getK8sEvents: executing query for %d resources", len(resourceUIDs))
	result, err := a.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query K8s events: %w", err)
	}

	events := make(map[string][]K8sEventInfo)

	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}

		resourceUID, ok := row[0].(string)
		if !ok {
			continue
		}

		events[resourceUID] = []K8sEventInfo{}

		// Parse events array
		if row[1] == nil {
			continue
		}

		eventList, ok := row[1].([]interface{})
		if !ok {
			continue
		}

		for _, eventNode := range eventList {
			if eventNode == nil {
				continue
			}

			eventProps, err := graph.ParseNodeFromResult(eventNode)
			if err != nil || eventProps == nil || len(eventProps) == 0 {
				continue
			}

			event := graph.ParseK8sEventFromNode(eventProps)

			k8sEventInfo := K8sEventInfo{
				EventID:   event.ID,
				Timestamp: time.Unix(0, event.Timestamp),
				Reason:    event.Reason,
				Message:   event.Message,
				Type:      event.Type,
				Count:     event.Count,
				Source:    event.Source,
			}

			events[resourceUID] = append(events[resourceUID], k8sEventInfo)
		}
	}

	a.logger.Debug("getK8sEvents: found events for %d resources", len(events))
	return events, nil
}

// getChangeEventsForRelated retrieves change events for related resources
func (a *RootCauseAnalyzer) getChangeEventsForRelated(
	ctx context.Context,
	relatedByResource map[string][]RelatedResourceData,
	failureTimestamp int64,
	lookbackNs int64,
) error {
	// Collect all related resource UIDs
	relatedUIDs := []string{}
	uidToParent := make(map[string][]struct {
		parentUID string
		index     int
	})

	for parentUID, relatedList := range relatedByResource {
		for i, rel := range relatedList {
			relatedUIDs = append(relatedUIDs, rel.Resource.UID)
			uidToParent[rel.Resource.UID] = append(uidToParent[rel.Resource.UID], struct {
				parentUID string
				index     int
			}{parentUID, i})
		}
	}

	if len(relatedUIDs) == 0 {
		return nil
	}

	// Get events for all related resources
	events, err := a.getChangeEvents(ctx, relatedUIDs, failureTimestamp, lookbackNs)
	if err != nil {
		return err
	}

	// Distribute events back to related resources
	for uid, eventList := range events {
		for _, parent := range uidToParent[uid] {
			if parent.index < len(relatedByResource[parent.parentUID]) {
				relatedByResource[parent.parentUID][parent.index].Events = eventList
			}
		}
	}

	return nil
}

// extractUIDs extracts UIDs from the ownership chain
func extractUIDs(chain []ResourceWithDistance) []string {
	uids := make([]string, len(chain))
	for i, rwd := range chain {
		uids[i] = rwd.Resource.UID
	}
	return uids
}
