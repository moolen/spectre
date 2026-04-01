package falkor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	analysispkg "github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/analysis/store"
	analyzerpkg "github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/graph"
)

const (
	queryTimeoutMs      = 120000
	maxRecentEvents     = 10
	maxK8sEvents        = 20
	defaultLimit        = 50
	maxLimit            = 500
	defaultMaxDepth     = 1
	maxMaxDepth         = 10
	defaultLookbackNs   = int64(30 * time.Minute)
	maxLookbackNs       = int64(24 * time.Hour)
	statusUnknown       = "Unknown"
	edgeTypeIngressRef  = "INGRESS_REF"
	edgeTypeReferences  = "REFERENCES_SPEC"
	defaultManagerFloor = 0.5
)

type Store struct {
	graphClient graph.Client
}

func New(graphClient graph.Client) *Store {
	return &Store{graphClient: graphClient}
}

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
			RETURN resource.uid as resourceUID, manager, manages
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
	startNs := window.FailureTimestampNs - window.LookbackNs

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
			       ingress, 'edgeTypeIngressRef' as ingressRefType,
			       role, 'BINDS_ROLE' as bindsRoleType
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs": resourceUIDs,
			"startNs":      startNs,
			"endNs":        window.FailureTimestampNs,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query related resources: %w", err)
	}

	related := make(map[string][]store.RelatedResourceData)
	for _, row := range result.Rows {
		if len(row) < 15 {
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
			res := graph.ParseResourceIdentityFromNode(props)
			for _, existing := range related[resourceUID] {
				if existing.Resource.UID == res.UID && existing.RelationshipType == relType {
					return
				}
			}
			related[resourceUID] = append(related[resourceUID], store.RelatedResourceData{
				Resource:         res,
				RelationshipType: relType,
				Events:           []store.ChangeEventInfo{},
			})
		}

		addRelated(1, edgeTypeReferences)
		addRelated(3, "SCHEDULED_ON")
		addRelated(5, "USES_SERVICE_ACCOUNT")
		addRelated(7, "SELECTS")
		addRelated(9, "GRANTS_TO")
		addRelated(13, "BINDS_ROLE")

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

			duplicate := false
			for _, existing := range related[resourceUID] {
				if existing.Resource.UID == ingress.UID && existing.RelationshipType == edgeTypeIngressRef {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}

			related[resourceUID] = append(related[resourceUID], store.RelatedResourceData{
				Resource:           ingress,
				RelationshipType:   edgeTypeIngressRef,
				Events:             []store.ChangeEventInfo{},
				ReferenceTargetUID: serviceUID,
			})
		}
	}

	return related, nil
}

func (s *Store) GetChangeEvents(
	ctx context.Context,
	resourceUIDs []string,
	window store.ResourceWindow,
) (map[string][]store.ChangeEventInfo, error) {
	if len(resourceUIDs) == 0 {
		return map[string][]store.ChangeEventInfo{}, nil
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query: `
			MATCH (resource:ResourceIdentity)
			WHERE resource.uid IN $resourceUIDs
			OPTIONAL MATCH (resource)-[:CHANGED]->(event:ChangeEvent)
			WHERE event.timestamp <= $failureTimestamp
			  AND event.timestamp >= $failureTimestamp - $lookback
			WITH resource.uid as resourceUID, event
			ORDER BY event.timestamp DESC
			WITH resourceUID, collect(event) as allEvents
			WITH resourceUID,
			     [e IN allEvents WHERE e.configChanged = true] as configEvents,
			     allEvents[0..$maxEvents] as recentEvents
			WITH resourceUID,
			     configEvents + [e IN recentEvents WHERE NOT e.id IN [ce IN configEvents | ce.id]] as combinedEvents
			UNWIND CASE WHEN size(combinedEvents) > 0 THEN combinedEvents ELSE [null] END as event
			WITH resourceUID, event
			WHERE event IS NOT NULL
			WITH resourceUID, event
			ORDER BY event.timestamp DESC
			RETURN resourceUID, collect(DISTINCT event) as events
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs":     resourceUIDs,
			"failureTimestamp": window.FailureTimestampNs,
			"lookback":         window.LookbackNs,
			"maxEvents":        maxRecentEvents,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query change events: %w", err)
	}

	events := make(map[string][]store.ChangeEventInfo)
	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}

		resourceUID, ok := row[0].(string)
		if !ok {
			continue
		}
		events[resourceUID] = []store.ChangeEventInfo{}

		eventList, ok := row[1].([]interface{})
		if !ok {
			continue
		}

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
			if seenEventIDs[event.ID] {
				continue
			}
			seenEventIDs[event.ID] = true

			status := analyzerpkg.InferStatusFromResource("", json.RawMessage(event.Data), event.EventType)
			events[resourceUID] = append(events[resourceUID], store.ChangeEventInfo{
				EventID:       event.ID,
				Timestamp:     time.Unix(0, event.Timestamp),
				EventType:     event.EventType,
				Status:        status,
				ConfigChanged: event.ConfigChanged,
				StatusChanged: event.StatusChanged,
				Description:   fmt.Sprintf("%s event", event.EventType),
				Data:          []byte(event.Data),
			})
		}
	}

	return events, nil
}

func (s *Store) GetK8sEvents(
	ctx context.Context,
	resourceUIDs []string,
	window store.ResourceWindow,
) (map[string][]store.K8sEventInfo, error) {
	if len(resourceUIDs) == 0 {
		return map[string][]store.K8sEventInfo{}, nil
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query: `
			MATCH (resource:ResourceIdentity)
			WHERE resource.uid IN $resourceUIDs
			OPTIONAL MATCH (resource)-[:EMITTED_EVENT]->(k8sEvent:K8sEvent)
			WHERE k8sEvent.timestamp <= $failureTimestamp
			  AND k8sEvent.timestamp >= $failureTimestamp - $lookback
			WITH resource.uid as resourceUID, k8sEvent
			ORDER BY k8sEvent.timestamp DESC
			WITH resourceUID, collect(k8sEvent)[0..$maxEvents] as events
			RETURN resourceUID, events
		`,
		Parameters: map[string]interface{}{
			"resourceUIDs":     resourceUIDs,
			"failureTimestamp": window.FailureTimestampNs,
			"lookback":         window.LookbackNs,
			"maxEvents":        maxK8sEvents,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query K8s events: %w", err)
	}

	events := make(map[string][]store.K8sEventInfo)
	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}

		resourceUID, ok := row[0].(string)
		if !ok {
			continue
		}
		events[resourceUID] = []store.K8sEventInfo{}

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

			events[resourceUID] = append(events[resourceUID], store.K8sEventInfo{
				EventID:   event.ID,
				Timestamp: time.Unix(0, event.Timestamp),
				Reason:    event.Reason,
				Message:   event.Message,
				Type:      event.Type,
				Count:     event.Count,
				Source:    event.Source,
			})
		}
	}

	return events, nil
}

func (s *Store) GetNamespaceGraph(ctx context.Context, input store.NamespaceGraphQuery) (*store.NamespaceGraphData, error) {
	startTime := time.Now()

	normalized := input
	if normalized.Limit <= 0 {
		normalized.Limit = defaultLimit
	}
	if normalized.Limit > maxLimit {
		normalized.Limit = maxLimit
	}
	if normalized.MaxDepth <= 0 {
		normalized.MaxDepth = defaultMaxDepth
	}
	if normalized.MaxDepth > maxMaxDepth {
		normalized.MaxDepth = maxMaxDepth
	}
	if normalized.LookbackNs <= 0 {
		normalized.LookbackNs = defaultLookbackNs
	}
	if normalized.LookbackNs > maxLookbackNs {
		normalized.LookbackNs = maxLookbackNs
	}

	namespacedResources, hasMore, nextCursor, err := s.fetchNamespacedResources(
		ctx, normalized.Namespace, normalized.TimestampNs, normalized.Limit, normalized.Cursor,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch namespaced resources: %w", err)
	}

	namespacedUIDs := make([]string, len(namespacedResources))
	for i, r := range namespacedResources {
		namespacedUIDs[i] = r.UID
	}

	clusterScopedResources, err := s.fetchClusterScopedResources(
		ctx, namespacedUIDs, normalized.TimestampNs, normalized.MaxDepth,
	)
	if err != nil {
		clusterScopedResources = nil
	}

	allResources := append(namespacedResources, clusterScopedResources...)
	allUIDs := make([]string, len(allResources))
	for i, r := range allResources {
		allUIDs[i] = r.UID
	}

	latestEvents, err := s.fetchLatestEvents(ctx, allUIDs, normalized.TimestampNs)
	if err != nil {
		latestEvents = make(map[string]*store.NamespaceGraphChangeEvent)
	}

	specChanges, err := s.fetchSpecChanges(ctx, allUIDs, normalized.TimestampNs, normalized.LookbackNs)
	if err == nil {
		for uid, sc := range specChanges {
			event, ok := latestEvents[uid]
			if !ok {
				continue
			}
			diffs, diffErr := analysispkg.ComputeJSONDiff(sc.EarliestData, sc.LatestData)
			if diffErr != nil {
				continue
			}
			diffs = analysispkg.FilterSpecOnly(diffs)
			if len(diffs) > 0 {
				event.SpecChanges = analysispkg.FormatUnifiedDiff(diffs)
			}
		}
	}

	edgeResults, err := s.fetchRelationships(ctx, allUIDs)
	if err != nil {
		edgeResults = nil
	}

	nodes := s.buildNamespaceGraphNodes(allResources, latestEvents)
	edges := s.buildNamespaceGraphEdges(edgeResults)

	return &store.NamespaceGraphData{
		Graph: store.NamespaceGraph{
			Nodes: nodes,
			Edges: edges,
		},
		Metadata: store.NamespaceGraphMetadata{
			Namespace:        normalized.Namespace,
			TimestampNs:      normalized.TimestampNs,
			NodeCount:        len(nodes),
			EdgeCount:        len(edges),
			QueryExecutionMs: time.Since(startTime).Milliseconds(),
			HasMore:          hasMore,
			NextCursor:       nextCursor,
			Cached:           false,
			CacheAgeMs:       0,
		},
	}, nil
}

type resourceResult struct {
	UID       string
	Kind      string
	APIGroup  string
	Namespace string
	Name      string
	Labels    map[string]string
	FirstSeen int64
	LastSeen  int64
	Deleted   bool
	DeletedAt int64
}

type edgeResult struct {
	SourceUID        string
	TargetUID        string
	RelationshipType string
	EdgeID           string
}

type paginationCursor struct {
	LastKind string `json:"lastKind"`
	LastName string `json:"lastName"`
}

type specChangeResult struct {
	ResourceUID     string
	LatestData      []byte
	EarliestData    []byte
	LatestTimestamp int64
}

func (s *Store) fetchNamespacedResources(
	ctx context.Context,
	namespace string,
	timestamp int64,
	limit int,
	cursor string,
) ([]resourceResult, bool, string, error) {
	var lastKind, lastName string
	if cursor != "" {
		pc, err := decodeCursor(cursor)
		if err == nil {
			lastKind = pc.LastKind
			lastName = pc.LastName
		}
	}

	params := map[string]interface{}{
		"namespace": namespace,
		"timestamp": timestamp,
		"limit":     limit + 1,
	}

	query := `
		MATCH (r:ResourceIdentity)-[:CHANGED]->(:ChangeEvent)
		WHERE r.namespace = $namespace
		  AND r.firstSeen <= $timestamp
		  AND (r.deleted = false OR r.deleted IS NULL OR r.deletedAt > $timestamp)
		  AND r.kind <> 'Event'
		RETURN DISTINCT r.uid as uid, r.kind as kind, r.apiGroup as apiGroup, r.namespace as namespace,
		       r.name as name, r.labels as labels, r.firstSeen as firstSeen, r.lastSeen as lastSeen,
		       r.deleted as deleted, r.deletedAt as deletedAt
		ORDER BY r.kind, r.name
		LIMIT $limit
	`

	if lastKind != "" || lastName != "" {
		query = `
			MATCH (r:ResourceIdentity)-[:CHANGED]->(:ChangeEvent)
			WHERE r.namespace = $namespace
			  AND r.firstSeen <= $timestamp
			  AND (r.deleted = false OR r.deleted IS NULL OR r.deletedAt > $timestamp)
			  AND r.kind <> 'Event'
			  AND ((r.kind > $lastKind) OR (r.kind = $lastKind AND r.name > $lastName))
			RETURN DISTINCT r.uid as uid, r.kind as kind, r.apiGroup as apiGroup, r.namespace as namespace,
			       r.name as name, r.labels as labels, r.firstSeen as firstSeen, r.lastSeen as lastSeen,
			       r.deleted as deleted, r.deletedAt as deletedAt
			ORDER BY r.kind, r.name
			LIMIT $limit
		`
		params["lastKind"] = lastKind
		params["lastName"] = lastName
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout:    queryTimeoutMs,
		Query:      query,
		Parameters: params,
	})
	if err != nil {
		return nil, false, "", fmt.Errorf("failed to fetch namespaced resources: %w", err)
	}

	resources := parseResourceResults(result)
	hasMore := len(resources) > limit
	if hasMore {
		resources = resources[:limit]
	}

	var nextCursor string
	if hasMore && len(resources) > 0 {
		lastResource := resources[len(resources)-1]
		nextCursor = encodeCursor(paginationCursor{
			LastKind: lastResource.Kind,
			LastName: lastResource.Name,
		})
	}

	return resources, hasMore, nextCursor, nil
}

func (s *Store) fetchClusterScopedResources(
	ctx context.Context,
	namespacedUIDs []string,
	timestamp int64,
	maxDepth int,
) ([]resourceResult, error) {
	if len(namespacedUIDs) == 0 {
		return nil, nil
	}

	query := `
		MATCH (r:ResourceIdentity)-[]-(cs:ResourceIdentity)-[:CHANGED]->(:ChangeEvent)
		WHERE r.uid IN $uids
		  AND (cs.namespace = '' OR cs.namespace IS NULL)
		  AND cs.firstSeen <= $timestamp
		  AND (cs.deleted = false OR cs.deleted IS NULL OR cs.deletedAt > $timestamp)
		RETURN DISTINCT cs.uid as uid, cs.kind as kind, cs.apiGroup as apiGroup, cs.namespace as namespace,
		       cs.name as name, cs.labels as labels, cs.firstSeen as firstSeen, cs.lastSeen as lastSeen,
		       cs.deleted as deleted, cs.deletedAt as deletedAt
		LIMIT 100
	`

	if maxDepth > 1 {
		query = fmt.Sprintf(`
			MATCH (r:ResourceIdentity)-[*1..%d]-(cs:ResourceIdentity)-[:CHANGED]->(:ChangeEvent)
			WHERE r.uid IN $uids
			  AND (cs.namespace = '' OR cs.namespace IS NULL)
			  AND cs.firstSeen <= $timestamp
			  AND (cs.deleted = false OR cs.deleted IS NULL OR cs.deletedAt > $timestamp)
			RETURN DISTINCT cs.uid as uid, cs.kind as kind, cs.apiGroup as apiGroup, cs.namespace as namespace,
			       cs.name as name, cs.labels as labels, cs.firstSeen as firstSeen, cs.lastSeen as lastSeen,
			       cs.deleted as deleted, cs.deletedAt as deletedAt
			LIMIT 100
		`, maxDepth)
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query:   query,
		Parameters: map[string]interface{}{
			"uids":      namespacedUIDs,
			"timestamp": timestamp,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cluster-scoped resources: %w", err)
	}

	return parseResourceResults(result), nil
}

func (s *Store) fetchLatestEvents(
	ctx context.Context,
	resourceUIDs []string,
	timestamp int64,
) (map[string]*store.NamespaceGraphChangeEvent, error) {
	if len(resourceUIDs) == 0 {
		return map[string]*store.NamespaceGraphChangeEvent{}, nil
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query: `
			MATCH (r:ResourceIdentity)-[:CHANGED]->(e:ChangeEvent)
			WHERE r.uid IN $uids
			  AND e.timestamp <= $timestamp
			WITH r.uid as resourceUID, e
			ORDER BY e.timestamp DESC
			WITH resourceUID, collect(e)[0] as latestEvent
			WHERE latestEvent IS NOT NULL
			RETURN resourceUID,
			       latestEvent.timestamp as timestamp,
			       latestEvent.eventType as eventType,
			       latestEvent.status as status,
			       latestEvent.errorMessage as errorMessage,
			       latestEvent.containerIssues as containerIssues,
			       latestEvent.impactScore as impactScore,
			       latestEvent.data as data
		`,
		Parameters: map[string]interface{}{
			"uids":      resourceUIDs,
			"timestamp": timestamp,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest events: %w", err)
	}

	events := make(map[string]*store.NamespaceGraphChangeEvent)
	for _, row := range result.Rows {
		if len(row) < 7 {
			continue
		}

		resourceUID, _ := row[0].(string)
		if resourceUID == "" {
			continue
		}

		event := &store.NamespaceGraphChangeEvent{}
		switch ts := row[1].(type) {
		case int64:
			event.TimestampNs = ts
		case float64:
			event.TimestampNs = int64(ts)
		}
		if et, ok := row[2].(string); ok {
			event.EventType = et
		}
		if status, ok := row[3].(string); ok {
			event.Status = status
		}
		if errMsg, ok := row[4].(string); ok {
			event.ErrorMessage = errMsg
		}
		if issues, ok := row[5].([]interface{}); ok {
			for _, issue := range issues {
				if s, ok := issue.(string); ok {
					event.ContainerIssues = append(event.ContainerIssues, s)
				}
			}
		} else if issuesStr, ok := row[5].(string); ok && issuesStr != "" {
			var issues []string
			if err := json.Unmarshal([]byte(issuesStr), &issues); err == nil {
				event.ContainerIssues = issues
			}
		}
		if score, ok := row[6].(float64); ok {
			event.ImpactScore = score
		}
		if len(row) > 7 {
			if dataStr, ok := row[7].(string); ok && dataStr != "" {
				event.SpecReplicas = extractSpecReplicas(dataStr)
			}
		}

		events[resourceUID] = event
	}

	return events, nil
}

func (s *Store) fetchSpecChanges(
	ctx context.Context,
	resourceUIDs []string,
	timestamp int64,
	lookbackNs int64,
) (map[string]*specChangeResult, error) {
	if len(resourceUIDs) == 0 {
		return map[string]*specChangeResult{}, nil
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query: `
			MATCH (r:ResourceIdentity)-[:CHANGED]->(e:ChangeEvent)
			WHERE r.uid IN $uids
			  AND e.timestamp >= $startTimestamp AND e.timestamp <= $timestamp
			WITH r.uid as resourceUID, e
			ORDER BY e.timestamp ASC
			WITH resourceUID, collect(e) as events
			WHERE size(events) > 0
			RETURN resourceUID,
			       events[0].data as earliestData,
			       events[-1].data as latestData,
			       events[-1].timestamp as latestTimestamp
		`,
		Parameters: map[string]interface{}{
			"uids":           resourceUIDs,
			"timestamp":      timestamp,
			"startTimestamp": timestamp - lookbackNs,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch spec changes: %w", err)
	}

	specChanges := make(map[string]*specChangeResult)
	for _, row := range result.Rows {
		if len(row) < 4 {
			continue
		}
		resourceUID, _ := row[0].(string)
		if resourceUID == "" {
			continue
		}

		sc := &specChangeResult{ResourceUID: resourceUID}
		if data, ok := row[1].(string); ok {
			sc.EarliestData = []byte(data)
		}
		if data, ok := row[2].(string); ok {
			sc.LatestData = []byte(data)
		}
		switch ts := row[3].(type) {
		case int64:
			sc.LatestTimestamp = ts
		case float64:
			sc.LatestTimestamp = int64(ts)
		}

		if len(sc.EarliestData) > 0 && len(sc.LatestData) > 0 {
			specChanges[resourceUID] = sc
		}
	}

	return specChanges, nil
}

func (s *Store) fetchRelationships(ctx context.Context, resourceUIDs []string) ([]edgeResult, error) {
	if len(resourceUIDs) == 0 {
		return nil, nil
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query: `
			MATCH (r:ResourceIdentity)-[rel]->(target:ResourceIdentity)
			WHERE r.uid IN $uids
			  AND target.uid IN $uids
			  AND NOT type(rel) IN ['CHANGED', 'EMITTED_EVENT']
			RETURN DISTINCT r.uid as source, target.uid as target, type(rel) as relType, id(rel) as edgeId
		`,
		Parameters: map[string]interface{}{
			"uids": resourceUIDs,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch relationships: %w", err)
	}

	edges := make([]edgeResult, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 4 {
			continue
		}

		e := edgeResult{}
		if source, ok := row[0].(string); ok {
			e.SourceUID = source
		}
		if target, ok := row[1].(string); ok {
			e.TargetUID = target
		}
		if relType, ok := row[2].(string); ok {
			e.RelationshipType = relType
		}
		switch edgeID := row[3].(type) {
		case int64:
			e.EdgeID = fmt.Sprintf("%d", edgeID)
		case float64:
			e.EdgeID = fmt.Sprintf("%.0f", edgeID)
		case string:
			e.EdgeID = edgeID
		}

		if e.SourceUID != "" && e.TargetUID != "" && e.RelationshipType != "" {
			edges = append(edges, e)
		}
	}

	return edges, nil
}

func (s *Store) buildNamespaceGraphNodes(
	resources []resourceResult,
	latestEvents map[string]*store.NamespaceGraphChangeEvent,
) []store.NamespaceGraphNode {
	nodes := make([]store.NamespaceGraphNode, 0, len(resources))
	for _, r := range resources {
		if event, ok := latestEvents[r.UID]; ok && event.EventType == "DELETE" {
			continue
		}
		if r.Deleted {
			continue
		}

		node := store.NamespaceGraphNode{
			UID:         r.UID,
			Kind:        r.Kind,
			APIGroup:    r.APIGroup,
			Namespace:   r.Namespace,
			Name:        r.Name,
			Status:      statusUnknown,
			LatestEvent: latestEvents[r.UID],
			Labels:      r.Labels,
		}
		if event := latestEvents[r.UID]; event != nil && event.Status != "" {
			node.Status = event.Status
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func (s *Store) buildNamespaceGraphEdges(edgeResults []edgeResult) []store.NamespaceGraphEdge {
	edges := make([]store.NamespaceGraphEdge, 0, len(edgeResults))
	for _, e := range edgeResults {
		edges = append(edges, store.NamespaceGraphEdge{
			ID:               e.EdgeID,
			Source:           e.SourceUID,
			Target:           e.TargetUID,
			RelationshipType: e.RelationshipType,
		})
	}
	return edges
}

func parseResourceResults(result *graph.QueryResult) []resourceResult {
	resources := make([]resourceResult, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 10 {
			continue
		}

		r := resourceResult{}
		if uid, ok := row[0].(string); ok {
			r.UID = uid
		}
		if kind, ok := row[1].(string); ok {
			r.Kind = kind
		}
		if apiGroup, ok := row[2].(string); ok {
			r.APIGroup = apiGroup
		}
		if namespace, ok := row[3].(string); ok {
			r.Namespace = namespace
		}
		if name, ok := row[4].(string); ok {
			r.Name = name
		}
		if labels, ok := row[5].(map[string]interface{}); ok {
			r.Labels = make(map[string]string)
			for k, v := range labels {
				if vs, ok := v.(string); ok {
					r.Labels[k] = vs
				}
			}
		} else if labels, ok := row[5].(map[string]string); ok {
			r.Labels = labels
		}
		switch firstSeen := row[6].(type) {
		case int64:
			r.FirstSeen = firstSeen
		case float64:
			r.FirstSeen = int64(firstSeen)
		}
		switch lastSeen := row[7].(type) {
		case int64:
			r.LastSeen = lastSeen
		case float64:
			r.LastSeen = int64(lastSeen)
		}
		if deleted, ok := row[8].(bool); ok {
			r.Deleted = deleted
		}
		switch deletedAt := row[9].(type) {
		case int64:
			r.DeletedAt = deletedAt
		case float64:
			r.DeletedAt = int64(deletedAt)
		}

		if r.UID != "" {
			resources = append(resources, r)
		}
	}
	return resources
}

func encodeCursor(cursor paginationCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.StdEncoding.EncodeToString(data)
}

func decodeCursor(cursor string) (*paginationCursor, error) {
	data, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cursor: %w", err)
	}
	var pc paginationCursor
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cursor: %w", err)
	}
	return &pc, nil
}

func extractSpecReplicas(data string) *int {
	var resource map[string]interface{}
	if err := json.Unmarshal([]byte(data), &resource); err != nil {
		return nil
	}

	spec, ok := resource["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	replicas, ok := spec["replicas"]
	if !ok {
		return nil
	}

	switch v := replicas.(type) {
	case float64:
		r := int(v)
		return &r
	case int:
		return &v
	case int64:
		r := int(v)
		return &r
	}
	return nil
}
