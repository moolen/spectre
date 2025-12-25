package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/graph"
)

// buildCausalGraph constructs the causal graph from symptom to root cause
func (a *RootCauseAnalyzer) buildCausalGraph(
	ctx context.Context,
	symptom *ObservedSymptom,
	failureTimestamp int64,
	lookbackNs int64,
) (CausalGraph, error) {
	// Query to traverse ownership backward and find MANAGES relationships + additional edge types
	// OPTIMIZED: Limit ownership chain, reduce Cartesian products
	query := graph.GraphQuery{
		Timeout: 15000, // 15 second timeout for complex root cause queries
		Query: `
			MATCH (symptomResource:ResourceIdentity {uid: $symptomUID})

			// Collect owners in the ownership chain (limited to 3 levels for performance)
			// Use variable-length path with relationship array to calculate distance
			OPTIONAL MATCH ownerPath = (symptomResource)<-[:OWNS*1..3]-(owner:ResourceIdentity)
			WITH symptomResource, owner,
			     CASE WHEN owner IS NULL THEN 0 ELSE length(ownerPath) END as ownerDistance
			WITH symptomResource, collect({resource: owner, distance: ownerDistance}) as ownersWithDistance

			// Add symptom itself with distance 0
			WITH symptomResource, [{resource: symptomResource, distance: 0}] + ownersWithDistance as chainResourcesWithDistance

			// For each resource in the chain, find related resources and events
			UNWIND chainResourcesWithDistance as resourceData
			WITH symptomResource, resourceData.resource as resource, resourceData.distance as distance
			WHERE resource IS NOT NULL

			// Find manager
			OPTIONAL MATCH (manager:ResourceIdentity)-[manages:MANAGES]->(resource)
			WHERE manages.confidence >= 0.5

			// Find related resources (limited matches to avoid Cartesian explosion)
			OPTIONAL MATCH (resource)-[refSpec:REFERENCES_SPEC]->(referencedResource:ResourceIdentity)
			OPTIONAL MATCH (resource)-[scheduledOn:SCHEDULED_ON]->(node:ResourceIdentity)
			OPTIONAL MATCH (resource)-[usesSA:USES_SERVICE_ACCOUNT]->(sa:ResourceIdentity)
			OPTIONAL MATCH (rb:ResourceIdentity)-[grantsTo:GRANTS_TO]->(sa)
			OPTIONAL MATCH (service:ResourceIdentity)-[selects:SELECTS]->(resource)
			WHERE service.kind = 'Service'

			// Get change events (LIMITED to most recent 10 per resource type to avoid explosion)
			OPTIONAL MATCH (resource)-[:CHANGED]->(changeEvent:ChangeEvent)
			WHERE changeEvent.timestamp <= $failureTimestamp
			  AND changeEvent.timestamp >= $failureTimestamp - $lookback
			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, symptomResource,
			     changeEvent
			ORDER BY changeEvent.timestamp DESC
			LIMIT 10

			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, symptomResource,
			     collect(changeEvent) as changeEvents

			OPTIONAL MATCH (manager)-[:CHANGED]->(managerEvent:ChangeEvent)
			WHERE managerEvent.timestamp <= $failureTimestamp
			  AND managerEvent.timestamp >= $failureTimestamp - $lookback
			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, symptomResource, changeEvents,
			     managerEvent
			ORDER BY managerEvent.timestamp DESC
			LIMIT 10

			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, symptomResource, changeEvents,
			     collect(managerEvent) as managerEvents

			OPTIONAL MATCH (referencedResource)-[:CHANGED]->(refEvent:ChangeEvent)
			WHERE refEvent.timestamp <= $failureTimestamp AND refEvent.timestamp >= $failureTimestamp - $lookback
			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, symptomResource, changeEvents, managerEvents,
			     refEvent
			ORDER BY refEvent.timestamp DESC
			LIMIT 10

			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, symptomResource, changeEvents, managerEvents,
			     collect(refEvent) as refEvents

			OPTIONAL MATCH (node)-[:CHANGED]->(nodeEvent:ChangeEvent)
			WHERE nodeEvent.timestamp <= $failureTimestamp AND nodeEvent.timestamp >= $failureTimestamp - $lookback
			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, symptomResource, changeEvents, managerEvents, refEvents,
			     nodeEvent
			ORDER BY nodeEvent.timestamp DESC
			LIMIT 10

			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, symptomResource, changeEvents, managerEvents, refEvents,
			     collect(nodeEvent) as nodeEvents

			OPTIONAL MATCH (sa)-[:CHANGED]->(saEvent:ChangeEvent)
			WHERE saEvent.timestamp <= $failureTimestamp AND saEvent.timestamp >= $failureTimestamp - $lookback
			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, symptomResource, changeEvents, managerEvents, refEvents, nodeEvents,
			     saEvent
			ORDER BY saEvent.timestamp DESC
			LIMIT 10

			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, symptomResource, changeEvents, managerEvents, refEvents, nodeEvents,
			     collect(saEvent) as saEvents

			OPTIONAL MATCH (rb)-[:CHANGED]->(rbEvent:ChangeEvent)
			WHERE rbEvent.timestamp <= $failureTimestamp AND rbEvent.timestamp >= $failureTimestamp - $lookback
			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, symptomResource, changeEvents, managerEvents, refEvents, nodeEvents, saEvents,
			     rbEvent
			ORDER BY rbEvent.timestamp DESC
			LIMIT 10

			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, symptomResource, changeEvents, managerEvents, refEvents, nodeEvents, saEvents,
			     collect(rbEvent) as rbEvents

			OPTIONAL MATCH (service)-[:CHANGED]->(serviceEvent:ChangeEvent)
			WHERE serviceEvent.timestamp <= $failureTimestamp AND serviceEvent.timestamp >= $failureTimestamp - $lookback
			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, symptomResource, changeEvents, managerEvents, refEvents, nodeEvents, saEvents, rbEvents,
			     serviceEvent
			ORDER BY serviceEvent.timestamp DESC
			LIMIT 10

			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, symptomResource, changeEvents, managerEvents, refEvents, nodeEvents, saEvents, rbEvents,
			     collect(serviceEvent) as serviceEvents

			OPTIONAL MATCH (resource)-[:EMITTED_EVENT]->(k8sEvent:K8sEvent)
			WHERE k8sEvent.timestamp <= $failureTimestamp AND k8sEvent.timestamp >= $failureTimestamp - $lookback
			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, changeEvents, managerEvents, refEvents, nodeEvents, saEvents, rbEvents, serviceEvents,
			     k8sEvent
			ORDER BY k8sEvent.timestamp DESC
			LIMIT 20

			WITH resource, manager, manages, referencedResource, refSpec, node, scheduledOn,
			     sa, usesSA, rb, grantsTo, service, selects, distance, changeEvents, managerEvents, refEvents, nodeEvents, saEvents, rbEvents, serviceEvents,
			     collect(k8sEvent) as k8sEvents

			// Unwind the collected events back to individual rows for compatibility with existing code
			UNWIND CASE WHEN size(changeEvents) > 0 THEN changeEvents ELSE [null] END as changeEvent
			UNWIND CASE WHEN size(managerEvents) > 0 THEN managerEvents ELSE [null] END as managerEvent
			UNWIND CASE WHEN size(refEvents) > 0 THEN refEvents ELSE [null] END as refEvent
			UNWIND CASE WHEN size(nodeEvents) > 0 THEN nodeEvents ELSE [null] END as nodeEvent
			UNWIND CASE WHEN size(saEvents) > 0 THEN saEvents ELSE [null] END as saEvent
			UNWIND CASE WHEN size(rbEvents) > 0 THEN rbEvents ELSE [null] END as rbEvent
			UNWIND CASE WHEN size(serviceEvents) > 0 THEN serviceEvents ELSE [null] END as serviceEvent
			UNWIND CASE WHEN size(k8sEvents) > 0 THEN k8sEvents ELSE [null] END as k8sEvent

			RETURN DISTINCT
			  resource,
			  manager, manages,
			  referencedResource, refSpec,
			  node, scheduledOn,
			  sa, usesSA,
			  rb, grantsTo,
			  service, selects,
			  changeEvent, managerEvent, refEvent, nodeEvent, saEvent, rbEvent, serviceEvent, k8sEvent,
			  distance
			ORDER BY distance DESC, changeEvent.timestamp ASC
		`,
		Parameters: map[string]interface{}{
			"symptomUID":       symptom.Resource.UID,
			"failureTimestamp": failureTimestamp,
			"lookback":         lookbackNs,
		},
	}

	a.logger.Debug("buildCausalGraph: executing query for symptom %s with lookback %dns", symptom.Resource.UID, lookbackNs)
	queryStart := time.Now()
	result, err := a.graphClient.ExecuteQuery(ctx, query)
	queryDuration := time.Since(queryStart)

	if err != nil {
		a.logger.Error("buildCausalGraph: query failed after %v: %v", queryDuration, err)
		queryPreview := query.Query
		if len(queryPreview) > 500 {
			queryPreview = queryPreview[:500] + "..."
		}
		a.logger.Debug("buildCausalGraph: query was: %s", queryPreview)
		return CausalGraph{}, fmt.Errorf("failed to query causal chain: %w", err)
	}

	a.logger.Debug("buildCausalGraph: query returned %d rows in %v", len(result.Rows), queryDuration)

	// Collect all events per resource and related resources
	type RelatedResourceData struct {
		resource         graph.ResourceIdentity
		relationshipType string
		edge             interface{} // Edge properties (if any)
		events           []ChangeEventInfo
	}

	type ResourceData struct {
		resource         graph.ResourceIdentity
		manager          *graph.ResourceIdentity
		managesEdge      *graph.ManagesEdge
		events           []ChangeEventInfo
		managerEvents    []ChangeEventInfo
		k8sEvents        []K8sEventInfo // K8s Events emitted by this resource
		distance         int
		relatedResources map[string]*RelatedResourceData // Key: relationshipType:uid
	}

	resourceMap := make(map[string]*ResourceData)

	for _, row := range result.Rows {
		if len(row) < 22 { // Updated: now we have 22 columns (added service, selects, serviceEvent)
			continue
		}

		// Parse resource
		resourceProps, err := graph.ParseNodeFromResult(row[0])
		if err != nil {
			a.logger.Debug("Failed to parse resource: %v", err)
			continue
		}
		resource := graph.ParseResourceIdentityFromNode(resourceProps)

		// Initialize resource data if not seen
		if _, exists := resourceMap[resource.UID]; !exists {
			resourceMap[resource.UID] = &ResourceData{
				resource:         resource,
				events:           []ChangeEventInfo{},
				managerEvents:    []ChangeEventInfo{},
				k8sEvents:        []K8sEventInfo{},
				relatedResources: make(map[string]*RelatedResourceData),
			}
		}
		rd := resourceMap[resource.UID]

		// Parse manager (may be null)
		if row[1] != nil && rd.manager == nil {
			managerProps, err := graph.ParseNodeFromResult(row[1])
			if err == nil {
				mgr := graph.ParseResourceIdentityFromNode(managerProps)
				rd.manager = &mgr
			}
		}
		if row[2] != nil && rd.managesEdge == nil {
			_, edgeProps, err := graph.ParseEdgeFromResult(row[2])
			if err == nil {
				edge := graph.ParseManagesEdge(edgeProps)
				rd.managesEdge = &edge
			}
		}

		// Parse service (row[11], selects row[12])
		if row[11] != nil {
			serviceProps, err := graph.ParseNodeFromResult(row[11])
			if err == nil {
				serviceResource := graph.ParseResourceIdentityFromNode(serviceProps)
				key := fmt.Sprintf("SELECTS:%s", serviceResource.UID)
				if _, exists := rd.relatedResources[key]; !exists {
					rd.relatedResources[key] = &RelatedResourceData{
						resource:         serviceResource,
						relationshipType: "SELECTS",
						events:           []ChangeEventInfo{},
					}
				}
			}
		}

		// Parse change event for the resource (may be null) - row[13]
		if row[13] != nil {
			eventProps, err := graph.ParseNodeFromResult(row[13])
			if err == nil {
				event := graph.ParseChangeEventFromNode(eventProps)

				// Infer status from resource data
				status := analyzer.InferStatusFromResource(resource.Kind, json.RawMessage(event.Data), event.EventType)

				changeEvent := ChangeEventInfo{
					EventID:       event.ID,
					Timestamp:     time.Unix(0, event.Timestamp),
					EventType:     event.EventType,
					Status:        status,
					ConfigChanged: event.ConfigChanged,
					StatusChanged: event.StatusChanged,
					Description:   fmt.Sprintf("%s event", event.EventType),
					Data:          []byte(event.Data), // Add resource data for diff
				}
				// Avoid duplicates
				isDuplicate := false
				for _, existing := range rd.events {
					if existing.EventID == changeEvent.EventID {
						isDuplicate = true
						break
					}
				}
				if !isDuplicate {
					rd.events = append(rd.events, changeEvent)
				}
			}
		}

		// Parse change event for the manager (may be null) - row[14]
		if row[14] != nil {
			eventProps, err := graph.ParseNodeFromResult(row[14])
			if err == nil {
				event := graph.ParseChangeEventFromNode(eventProps)

				// Infer status from manager resource data
				var managerKind string
				if rd.manager != nil {
					managerKind = rd.manager.Kind
				}
				status := analyzer.InferStatusFromResource(managerKind, json.RawMessage(event.Data), event.EventType)

				managerEvent := ChangeEventInfo{
					EventID:       event.ID,
					Timestamp:     time.Unix(0, event.Timestamp),
					EventType:     event.EventType,
					Status:        status,
					ConfigChanged: event.ConfigChanged,
					StatusChanged: event.StatusChanged,
					Description:   fmt.Sprintf("%s event", event.EventType),
					Data:          []byte(event.Data), // Add resource data for diff
				}
				// Avoid duplicates
				isDuplicate := false
				for _, existing := range rd.managerEvents {
					if existing.EventID == managerEvent.EventID {
						isDuplicate = true
						break
					}
				}
				if !isDuplicate {
					rd.managerEvents = append(rd.managerEvents, managerEvent)
				}
			}
		}

		// Parse referenced resource (row[3], refSpec row[4])
		if row[3] != nil {
			refProps, err := graph.ParseNodeFromResult(row[3])
			if err == nil {
				refResource := graph.ParseResourceIdentityFromNode(refProps)
				key := fmt.Sprintf("REFERENCES_SPEC:%s", refResource.UID)
				if _, exists := rd.relatedResources[key]; !exists {
					rd.relatedResources[key] = &RelatedResourceData{
						resource:         refResource,
						relationshipType: "REFERENCES_SPEC",
						events:           []ChangeEventInfo{},
					}
				}
				// Parse ref event (row[15])
				if row[15] != nil {
					eventProps, err := graph.ParseNodeFromResult(row[15])
					if err == nil {
						event := graph.ParseChangeEventFromNode(eventProps)
						status := analyzer.InferStatusFromResource(refResource.Kind, json.RawMessage(event.Data), event.EventType)
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
						// Avoid duplicates
						isDuplicate := false
						for _, existing := range rd.relatedResources[key].events {
							if existing.EventID == changeEvent.EventID {
								isDuplicate = true
								break
							}
						}
						if !isDuplicate {
							rd.relatedResources[key].events = append(rd.relatedResources[key].events, changeEvent)
						}
					}
				}
			}
		}

		// Parse node (row[5], scheduledOn row[6])
		if row[5] != nil {
			nodeProps, err := graph.ParseNodeFromResult(row[5])
			if err == nil {
				nodeResource := graph.ParseResourceIdentityFromNode(nodeProps)
				key := fmt.Sprintf("SCHEDULED_ON:%s", nodeResource.UID)
				if _, exists := rd.relatedResources[key]; !exists {
					rd.relatedResources[key] = &RelatedResourceData{
						resource:         nodeResource,
						relationshipType: "SCHEDULED_ON",
						events:           []ChangeEventInfo{},
					}
				}
				// Parse node event (row[16])
				if row[16] != nil {
					eventProps, err := graph.ParseNodeFromResult(row[16])
					if err == nil {
						event := graph.ParseChangeEventFromNode(eventProps)
						status := analyzer.InferStatusFromResource(nodeResource.Kind, json.RawMessage(event.Data), event.EventType)
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
						// Avoid duplicates
						isDuplicate := false
						for _, existing := range rd.relatedResources[key].events {
							if existing.EventID == changeEvent.EventID {
								isDuplicate = true
								break
							}
						}
						if !isDuplicate {
							rd.relatedResources[key].events = append(rd.relatedResources[key].events, changeEvent)
						}
					}
				}
			}
		}

		// Parse service account (row[7], usesSA row[8])
		if row[7] != nil {
			saProps, err := graph.ParseNodeFromResult(row[7])
			if err == nil {
				saResource := graph.ParseResourceIdentityFromNode(saProps)
				key := fmt.Sprintf("USES_SERVICE_ACCOUNT:%s", saResource.UID)
				if _, exists := rd.relatedResources[key]; !exists {
					rd.relatedResources[key] = &RelatedResourceData{
						resource:         saResource,
						relationshipType: "USES_SERVICE_ACCOUNT",
						events:           []ChangeEventInfo{},
					}
				}
				// Parse SA event (row[17])
				if row[17] != nil {
					eventProps, err := graph.ParseNodeFromResult(row[17])
					if err == nil {
						event := graph.ParseChangeEventFromNode(eventProps)
						status := analyzer.InferStatusFromResource(saResource.Kind, json.RawMessage(event.Data), event.EventType)
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
						// Avoid duplicates
						isDuplicate := false
						for _, existing := range rd.relatedResources[key].events {
							if existing.EventID == changeEvent.EventID {
								isDuplicate = true
								break
							}
						}
						if !isDuplicate {
							rd.relatedResources[key].events = append(rd.relatedResources[key].events, changeEvent)
						}
					}
				}
			}
		}

		// Parse role binding (row[9], grantsTo row[10])
		if row[9] != nil {
			rbProps, err := graph.ParseNodeFromResult(row[9])
			if err == nil {
				rbResource := graph.ParseResourceIdentityFromNode(rbProps)
				key := fmt.Sprintf("GRANTS_TO:%s", rbResource.UID)
				if _, exists := rd.relatedResources[key]; !exists {
					rd.relatedResources[key] = &RelatedResourceData{
						resource:         rbResource,
						relationshipType: "GRANTS_TO",
						events:           []ChangeEventInfo{},
					}
				}
				// Parse RB event (row[18])
				if row[18] != nil {
					eventProps, err := graph.ParseNodeFromResult(row[18])
					if err == nil {
						event := graph.ParseChangeEventFromNode(eventProps)
						status := analyzer.InferStatusFromResource(rbResource.Kind, json.RawMessage(event.Data), event.EventType)
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
						// Avoid duplicates
						isDuplicate := false
						for _, existing := range rd.relatedResources[key].events {
							if existing.EventID == changeEvent.EventID {
								isDuplicate = true
								break
							}
						}
						if !isDuplicate {
							rd.relatedResources[key].events = append(rd.relatedResources[key].events, changeEvent)
						}
					}
				}
			}
		}

		// Parse service event (row[19])
		if row[11] != nil { // Service node
			serviceProps, _ := graph.ParseNodeFromResult(row[11])
			if serviceProps != nil {
				serviceResource := graph.ParseResourceIdentityFromNode(serviceProps)
				key := fmt.Sprintf("SELECTS:%s", serviceResource.UID)
				
				if row[19] != nil {
					eventProps, err := graph.ParseNodeFromResult(row[19])
					if err == nil {
						event := graph.ParseChangeEventFromNode(eventProps)
						status := analyzer.InferStatusFromResource(serviceResource.Kind, json.RawMessage(event.Data), event.EventType)
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
						// Avoid duplicates
						if relData, exists := rd.relatedResources[key]; exists {
							isDuplicate := false
							for _, existing := range relData.events {
								if existing.EventID == changeEvent.EventID {
									isDuplicate = true
									break
								}
							}
							if !isDuplicate {
								rd.relatedResources[key].events = append(rd.relatedResources[key].events, changeEvent)
							}
						}
					}
				}
			}
		}

		// Parse K8s Event for the resource (may be null) - row[20]
		if row[20] != nil {
			eventProps, err := graph.ParseNodeFromResult(row[20])
			if err == nil {
				k8sEvent := graph.ParseK8sEventFromNode(eventProps)
				k8sEventInfo := K8sEventInfo{
					EventID:   k8sEvent.ID,
					Timestamp: time.Unix(0, k8sEvent.Timestamp),
					Reason:    k8sEvent.Reason,
					Message:   k8sEvent.Message,
					Type:      k8sEvent.Type,
					Count:     k8sEvent.Count,
					Source:    k8sEvent.Source,
				}
				// Avoid duplicates
				isDuplicate := false
				for _, existing := range rd.k8sEvents {
					if existing.EventID == k8sEventInfo.EventID {
						isDuplicate = true
						break
					}
				}
				if !isDuplicate {
					rd.k8sEvents = append(rd.k8sEvents, k8sEventInfo)
				}
			}
		}

		// Store distance (use from first row for this resource) - row[21]
		if row[21] != nil && rd.distance == 0 {
			if distInt, ok := row[21].(int64); ok {
				rd.distance = int(distInt)
			}
		}
	}

	// Build steps from collected data, ordered by distance (DESC - furthest from symptom first)
	type sortableResource struct {
		uid      string
		distance int
	}
	var sortedUIDs []sortableResource
	for uid, rd := range resourceMap {
		sortedUIDs = append(sortedUIDs, sortableResource{uid: uid, distance: rd.distance})
	}
	// Sort by distance DESC (furthest from symptom = root cause side)
	for i := 0; i < len(sortedUIDs); i++ {
		for j := i + 1; j < len(sortedUIDs); j++ {
			if sortedUIDs[j].distance > sortedUIDs[i].distance {
				sortedUIDs[i], sortedUIDs[j] = sortedUIDs[j], sortedUIDs[i]
			}
		}
	}

	// Build graph structure: nodes and edges
	nodes := []GraphNode{}
	edges := []GraphEdge{}
	nodeMap := make(map[string]string) // resource.UID -> node.ID
	edgeSet := make(map[string]bool)   // Deduplicate edges

	seenResources := make(map[string]bool)
	stepNumber := 1

	// STEP 1: Create all SPINE nodes
	for _, sr := range sortedUIDs {
		rd := resourceMap[sr.uid]
		resource := rd.resource

		// Skip if we've already seen this resource
		if seenResources[resource.UID] {
			continue
		}
		seenResources[resource.UID] = true

		// Select primary event from collected events
		primaryEvent := selectPrimaryEvent(rd.events, failureTimestamp)

		// Determine relationship type for reasoning
		var relationshipType string
		if resource.UID != symptom.Resource.UID {
			relationshipType = "OWNS"
		} else {
			relationshipType = "SYMPTOM"
		}
		if rd.manager != nil {
			relationshipType = "MANAGED_BY"
		}

		// Generate reasoning for this node
		reasoning := generateStepReasoning(resource, rd.manager, rd.managesEdge, primaryEvent, relationshipType)

		// Create node ID and add to map
		nodeID := fmt.Sprintf("node-%s", resource.UID)
		nodeMap[resource.UID] = nodeID

		// Create SPINE node
		nodes = append(nodes, GraphNode{
			ID: nodeID,
			Resource: SymptomResource{
				UID:       resource.UID,
				Kind:      resource.Kind,
				Namespace: resource.Namespace,
				Name:      resource.Name,
			},
			ChangeEvent: primaryEvent,
			AllEvents:   rd.events,
			K8sEvents:   rd.k8sEvents,
			NodeType:    "SPINE",
			StepNumber:  stepNumber,
			Reasoning:   reasoning,
		})
		stepNumber++

		// If this resource has a manager, add the manager as a separate node
		// Include manager even if it has no recent events - it's part of the causal chain
		if rd.manager != nil && !seenResources[rd.manager.UID] {
			seenResources[rd.manager.UID] = true

			// Select primary event for manager (may be nil if no events in lookback window)
			managerPrimaryEvent := selectPrimaryEvent(rd.managerEvents, failureTimestamp)

			managerNodeID := fmt.Sprintf("node-%s", rd.manager.UID)
			nodeMap[rd.manager.UID] = managerNodeID

			nodes = append(nodes, GraphNode{
				ID: managerNodeID,
				Resource: SymptomResource{
					UID:       rd.manager.UID,
					Kind:      rd.manager.Kind,
					Namespace: rd.manager.Namespace,
					Name:      rd.manager.Name,
				},
				ChangeEvent: managerPrimaryEvent,
				AllEvents:   rd.managerEvents,
				NodeType:    "SPINE",
				StepNumber:  stepNumber,
				Reasoning: fmt.Sprintf("%s manages %s lifecycle (confidence: %.0f%%)",
					rd.manager.Kind, resource.Kind,
					rd.managesEdge.Confidence*100),
			})
			stepNumber++
		}
	}

	// STEP 2: Create SPINE edges (main chain relationships)
	// First, build a map of which resources are owned (for filtering transitive MANAGES edges)
	ownedResources := make(map[string]bool)
	for idx, sr := range sortedUIDs {
		rd := resourceMap[sr.uid]
		resource := rd.resource

		// Mark resources that are owned by finding the next resource in chain
		if resource.UID != symptom.Resource.UID {
			targetDistance := sr.distance - 1
			for nextIdx := idx + 1; nextIdx < len(sortedUIDs); nextIdx++ {
				nextRd := resourceMap[sortedUIDs[nextIdx].uid]
				if sortedUIDs[nextIdx].distance == targetDistance {
					ownedResources[nextRd.resource.UID] = true
					break
				}
			}
		}
	}

	for idx, sr := range sortedUIDs {
		rd := resourceMap[sr.uid]
		resource := rd.resource

		fromNodeID := nodeMap[resource.UID]
		if fromNodeID == "" {
			continue
		}

		// OWNS edge to next resource in chain
		if resource.UID != symptom.Resource.UID {
			targetDistance := sr.distance - 1
			for nextIdx := idx + 1; nextIdx < len(sortedUIDs); nextIdx++ {
				nextRd := resourceMap[sortedUIDs[nextIdx].uid]
				if sortedUIDs[nextIdx].distance == targetDistance {
					toNodeID := nodeMap[nextRd.resource.UID]
					if toNodeID != "" {
						edgeID := fmt.Sprintf("edge-spine-%s-%s", resource.UID, nextRd.resource.UID)
						if !edgeSet[edgeID] {
							edges = append(edges, GraphEdge{
								ID:               edgeID,
								From:             fromNodeID,
								To:               toNodeID,
								RelationshipType: "OWNS",
								EdgeType:         "SPINE",
							})
							edgeSet[edgeID] = true
						}
					}
					break
				}
			}
		}

		// MANAGES edge from manager to resource
		// Only create MANAGES edge if the resource is not already owned by another resource
		// This avoids showing transitive management (e.g., HelmRelease->Job when HelmRelease->CronJob and CronJob->Job exist)
		if rd.manager != nil && !ownedResources[resource.UID] {
			managerNodeID := nodeMap[rd.manager.UID]
			if managerNodeID != "" {
				edgeID := fmt.Sprintf("edge-spine-%s-%s", rd.manager.UID, resource.UID)
				if !edgeSet[edgeID] {
					edges = append(edges, GraphEdge{
						ID:               edgeID,
						From:             managerNodeID,
						To:               fromNodeID,
						RelationshipType: "MANAGES",
						EdgeType:         "SPINE",
					})
					edgeSet[edgeID] = true
				}
			}
		}
	}

	// STEP 3: Create RELATED nodes and ATTACHMENT edges
	for _, sr := range sortedUIDs {
		rd := resourceMap[sr.uid]
		parentNodeID := nodeMap[rd.resource.UID]
		if parentNodeID == "" {
			continue
		}

		for _, relData := range rd.relatedResources {
			hasChanges := len(relData.events) > 0
			// Always include SCHEDULED_ON nodes and GRANTS_TO (permissions) even without recent changes
			// These are structurally important for understanding the system state
			if relData.relationshipType != "SCHEDULED_ON" &&
				relData.relationshipType != "GRANTS_TO" &&
				!hasChanges {
				continue
			}

			relatedUID := relData.resource.UID

			// Create or get node for related resource
			relatedNodeID := nodeMap[relatedUID]
			if relatedNodeID == "" {
				relatedNodeID = fmt.Sprintf("node-%s", relatedUID)
				nodeMap[relatedUID] = relatedNodeID

				nodes = append(nodes, GraphNode{
					ID: relatedNodeID,
					Resource: SymptomResource{
						UID:       relData.resource.UID,
						Kind:      relData.resource.Kind,
						Namespace: relData.resource.Namespace,
						Name:      relData.resource.Name,
					},
					AllEvents: relData.events,
					NodeType:  "RELATED",
				})
			}

			// Create attachment edge
			// Special handling for GRANTS_TO: edge goes FROM RoleBinding TO ServiceAccount
			// For other relationships: edge goes FROM parent resource TO related resource
			if relData.relationshipType == "GRANTS_TO" {
				// Find the ServiceAccount node (the parent resource should use it)
				var serviceAccountNodeID string
				for _, otherRelData := range rd.relatedResources {
					if otherRelData.relationshipType == "USES_SERVICE_ACCOUNT" {
						serviceAccountNodeID = nodeMap[otherRelData.resource.UID]
						break
					}
				}

				// Create edge from RoleBinding to ServiceAccount
				if serviceAccountNodeID != "" {
					edgeID := fmt.Sprintf("edge-attach-%s-%s", relatedUID, serviceAccountNodeID)
					if !edgeSet[edgeID] {
						edges = append(edges, GraphEdge{
							ID:               edgeID,
							From:             relatedNodeID, // RoleBinding/ClusterRoleBinding
							To:               serviceAccountNodeID, // ServiceAccount
							RelationshipType: "GRANTS_TO",
							EdgeType:         "ATTACHMENT",
						})
						edgeSet[edgeID] = true
					}
				}
			} else {
				// Normal edge: from parent resource to related resource
				edgeID := fmt.Sprintf("edge-attach-%s-%s", rd.resource.UID, relatedUID)
				if !edgeSet[edgeID] {
					edges = append(edges, GraphEdge{
						ID:               edgeID,
						From:             parentNodeID,
						To:               relatedNodeID,
						RelationshipType: relData.relationshipType,
						EdgeType:         "ATTACHMENT",
					})
					edgeSet[edgeID] = true
				}
			}
		}
	}

	a.logger.Debug("Built causal graph with %d nodes and %d edges", len(nodes), len(edges))
	return CausalGraph{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// selectPrimaryEvent chooses the most relevant event from a collection
// Priority: configChanged > CREATE > statusChanged (closest to failure)
func selectPrimaryEvent(events []ChangeEventInfo, failureTimestamp int64) *ChangeEventInfo {
	if len(events) == 0 {
		return nil
	}

	// Priority 1: configChanged=true events (earliest timestamp = trigger)
	var earliestConfigChange *ChangeEventInfo
	for i := range events {
		if events[i].ConfigChanged {
			if earliestConfigChange == nil || events[i].Timestamp.Before(earliestConfigChange.Timestamp) {
				earliestConfigChange = &events[i]
			}
		}
	}
	if earliestConfigChange != nil {
		return earliestConfigChange
	}

	// Priority 2: CREATE events
	var earliestCreate *ChangeEventInfo
	for i := range events {
		if events[i].EventType == "CREATE" {
			if earliestCreate == nil || events[i].Timestamp.Before(earliestCreate.Timestamp) {
				earliestCreate = &events[i]
			}
		}
	}
	if earliestCreate != nil {
		return earliestCreate
	}

	// Priority 3: Status changes closest to failure
	var closestStatus *ChangeEventInfo
	minDelta := int64(1<<63 - 1) // Max int64
	for i := range events {
		if events[i].StatusChanged {
			delta := failureTimestamp - events[i].Timestamp.UnixNano()
			if delta < 0 {
				delta = -delta
			}
			if delta < minDelta {
				minDelta = delta
				closestStatus = &events[i]
			}
		}
	}
	if closestStatus != nil {
		return closestStatus
	}

	// Fallback: earliest event
	earliest := &events[0]
	for i := range events {
		if events[i].Timestamp.Before(earliest.Timestamp) {
			earliest = &events[i]
		}
	}
	return earliest
}

// generateStepReasoning creates a human-readable explanation for a causal step
func generateStepReasoning(
	resource graph.ResourceIdentity,
	manager *graph.ResourceIdentity,
	managesEdge *graph.ManagesEdge,
	changeEvent *ChangeEventInfo,
	relationshipType string,
) string {
	switch relationshipType {
	case "MANAGES":
		if manager != nil {
			confidence := 0.0
			if managesEdge != nil {
				confidence = managesEdge.Confidence
			}
			return fmt.Sprintf("%s manages %s lifecycle (confidence: %.0f%%)",
				manager.Kind, resource.Kind, confidence*100)
		}
		return "Lifecycle management relationship"

	case "MANAGED_BY":
		if manager != nil {
			confidence := 0.0
			if managesEdge != nil {
				confidence = managesEdge.Confidence
			}
			return fmt.Sprintf("Managed by %s (confidence: %.0f%%)",
				manager.Kind, confidence*100)
		}
		return "Managed resource"

	case "OWNS":
		return fmt.Sprintf("%s owns resources in the next layer", resource.Kind)

	case "SYMPTOM":
		if changeEvent != nil && changeEvent.ConfigChanged {
			return "Configuration change triggered the failure"
		}
		return "Observed failure symptom"

	default:
		if changeEvent != nil {
			return fmt.Sprintf("%s occurred in this resource", changeEvent.EventType)
		}
		return "Part of the causal chain"
	}
}
